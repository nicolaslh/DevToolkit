package mediax

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// maxConcurrency caps how many conversions may run at once, protecting against
// oversubscribing CPU/network no matter what the caller requests.
const maxConcurrency = 6

// BatchItemStatus is the per-source state within a batch conversion.
type BatchItemStatus struct {
	Source string `json:"source"`
	Output string `json:"output"`
	// Name is the display base name (without directory) of the output file.
	Name string `json:"name"`
	// Percent is 0-100 for the item, or -1 when its duration is unknown.
	Percent float64 `json:"percent"`
	// Duration is the item's media length in seconds (0 when unknown).
	Duration float64 `json:"duration"`
	// Speed is the current encoding speed relative to realtime (e.g. 12 = 12x).
	Speed float64 `json:"speed"`
	// ETA is the estimated seconds remaining for this item; -1 when unknown.
	ETA float64 `json:"eta"`
	// Started is true once the item's conversion has begun (it may run in
	// parallel with others).
	Started bool   `json:"started"`
	Done    bool   `json:"done"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// BatchProgress is an immutable snapshot of a batch conversion.
type BatchProgress struct {
	Total     int               `json:"total"`
	Completed int               `json:"completed"` // items finished (success or failure)
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Running   int               `json:"running"` // items currently converting in parallel
	Items     []BatchItemStatus `json:"items"`
	// Percent is the overall progress 0-100. It is duration-weighted when every
	// item's length is known, otherwise it falls back to a per-item count.
	Percent float64 `json:"percent"`
	// ETA is the estimated seconds remaining for the whole batch; -1 when unknown.
	ETA float64 `json:"eta"`
	// Elapsed is the wall-clock seconds since the batch started.
	Elapsed  float64 `json:"elapsed"`
	Done     bool    `json:"done"`
	Canceled bool    `json:"canceled"`
}

type batchItem struct {
	source string
	output string
	name   string

	mu        sync.Mutex
	percent   float64
	duration  float64 // probed media length in seconds (0 when unknown)
	processed float64 // seconds of media written so far
	speed     float64 // encoding speed relative to realtime
	started   bool
	done      bool
	success   bool
	errMsg    string
	job       *Job // the underlying conversion, kept so its log stays reachable
}

func (it *batchItem) setDuration(d float64) {
	it.mu.Lock()
	it.duration = d
	it.mu.Unlock()
}

func (it *batchItem) setJob(j *Job) {
	it.mu.Lock()
	it.job = j
	it.mu.Unlock()
}

// log returns the ffmpeg output captured for this item, or "" before it starts.
func (it *batchItem) log() string {
	it.mu.Lock()
	j := it.job
	it.mu.Unlock()
	if j == nil {
		return ""
	}
	return j.Log()
}

func (it *batchItem) markStarted() {
	it.mu.Lock()
	it.started = true
	it.mu.Unlock()
}

func (it *batchItem) update(percent, processed, speed float64) {
	it.mu.Lock()
	it.percent = percent
	it.processed = processed
	it.speed = speed
	it.mu.Unlock()
}

func (it *batchItem) succeed() {
	it.mu.Lock()
	it.percent = 100
	it.processed = it.duration
	it.done = true
	it.success = true
	it.mu.Unlock()
}

func (it *batchItem) fail(msg string) {
	it.mu.Lock()
	it.done = true
	it.success = false
	it.errMsg = msg
	it.mu.Unlock()
}

// etaLocked estimates remaining seconds for this item from its speed and how
// much media is left. Returns -1 when it can't be estimated. Caller holds it.mu.
func (it *batchItem) etaLocked() float64 {
	if it.done || it.duration <= 0 || it.speed <= 0 {
		return -1
	}
	remaining := it.duration - it.processed
	if remaining < 0 {
		remaining = 0
	}
	return remaining / it.speed
}

// snapshot returns the item status plus the raw values Snapshot needs to
// aggregate overall progress/ETA, taking the lock only once.
func (it *batchItem) snapshot() (st BatchItemStatus, duration, processed, speed float64) {
	it.mu.Lock()
	defer it.mu.Unlock()
	st = BatchItemStatus{
		Source:   it.source,
		Output:   it.output,
		Name:     it.name,
		Percent:  it.percent,
		Duration: it.duration,
		Speed:    it.speed,
		ETA:      it.etaLocked(),
		Started:  it.started,
		Done:     it.done,
		Success:  it.success,
		Error:    it.errMsg,
	}
	return st, it.duration, it.processed, it.speed
}

// BatchJob converts several m3u8 sources into a shared output directory, up to
// Concurrency at a time. Output names reuse the source's base name with the
// target format's extension.
type BatchJob struct {
	items       []*batchItem
	opts        Options
	concurrency int
	start       time.Time

	mu       sync.Mutex
	done     bool
	canceled bool

	cancelCh   chan struct{}
	cancelOnce sync.Once

	runMu   sync.Mutex
	running map[*Job]struct{}
}

// StartBatch validates inputs, computes per-source output paths and launches a
// concurrent conversion in the background. concurrency is clamped to
// [1, maxConcurrency]; values <= 0 default to 2. It returns a job to poll.
func StartBatch(sources []string, outputDir string, opts Options, concurrency int) (*BatchJob, error) {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return nil, apperr.New(apperr.InvalidInput, "请选择输出文件夹")
	}
	if !opts.Format.valid() {
		return nil, apperr.Newf(apperr.Unsupported, "不支持的输出格式：%s", opts.Format)
	}

	cleaned := make([]string, 0, len(sources))
	for _, s := range sources {
		if s = strings.TrimSpace(s); s != "" {
			cleaned = append(cleaned, s)
		}
	}
	if len(cleaned) == 0 {
		return nil, apperr.New(apperr.InvalidInput, "请至少添加一个 m3u8 地址或文件")
	}

	if concurrency <= 0 {
		concurrency = 2
	}
	if concurrency > maxConcurrency {
		concurrency = maxConcurrency
	}
	if concurrency > len(cleaned) {
		concurrency = len(cleaned)
	}

	ext := opts.Format.Ext()
	used := map[string]bool{}
	items := make([]*batchItem, 0, len(cleaned))
	for _, s := range cleaned {
		name := uniqueName(baseName(s), ext, used)
		items = append(items, &batchItem{
			source:  s,
			output:  filepath.Join(outputDir, name),
			name:    name,
			percent: -1,
		})
	}

	b := &BatchJob{
		items:       items,
		opts:        opts,
		concurrency: concurrency,
		start:       time.Now(),
		cancelCh:    make(chan struct{}),
		running:     map[*Job]struct{}{},
	}
	go b.run()
	return b, nil
}

func (b *BatchJob) run() {
	// Probe every source's duration up front (in parallel, best-effort) so the
	// overall progress and ETA span the whole batch, not just running items.
	b.forEach(b.concurrency, func(it *batchItem) {
		if res, err := Probe(it.source); err == nil {
			it.setDuration(res.Duration)
		}
	})

	// Convert items with a bounded worker pool.
	b.forEach(b.concurrency, b.convertOne)

	b.mu.Lock()
	b.done = true
	b.mu.Unlock()
}

// forEach runs fn over every item using `workers` goroutines, stopping early
// when the job is canceled.
func (b *BatchJob) forEach(workers int, fn func(*batchItem)) {
	if workers < 1 {
		workers = 1
	}
	ch := make(chan *batchItem)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for it := range ch {
				if b.stopped() {
					continue // drain remaining without work
				}
				fn(it)
			}
		}()
	}
	for _, it := range b.items {
		if b.stopped() {
			break
		}
		ch <- it
	}
	close(ch)
	wg.Wait()
}

func (b *BatchJob) convertOne(it *batchItem) {
	it.markStarted()
	it.mu.Lock()
	duration := it.duration
	it.mu.Unlock()

	job, err := Start(it.source, it.output, b.opts, duration)
	if err != nil {
		it.fail(err.Error())
		return
	}
	it.setJob(job)
	b.addRunning(job)
	defer b.removeRunning(job)

	for {
		if b.stopped() {
			job.Cancel()
		}
		p := job.Snapshot()
		it.update(p.Percent, p.Processed, p.Speed)
		if p.Done {
			switch {
			case p.Success:
				it.succeed()
			case p.Canceled:
				it.fail("已取消")
			default:
				it.fail(p.Error)
			}
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (b *BatchJob) addRunning(j *Job) {
	b.runMu.Lock()
	b.running[j] = struct{}{}
	b.runMu.Unlock()
}

func (b *BatchJob) removeRunning(j *Job) {
	b.runMu.Lock()
	delete(b.running, j)
	b.runMu.Unlock()
}

func (b *BatchJob) stopped() bool {
	select {
	case <-b.cancelCh:
		return true
	default:
		return false
	}
}

// Cancel stops the batch and every item currently converting.
func (b *BatchJob) Cancel() {
	b.cancelOnce.Do(func() {
		b.mu.Lock()
		b.canceled = true
		b.mu.Unlock()
		close(b.cancelCh)
		b.runMu.Lock()
		for j := range b.running {
			j.Cancel()
		}
		b.runMu.Unlock()
	})
}

// ItemLog returns the captured ffmpeg log for the item at index, or "" if the
// index is out of range or the item hasn't started yet.
func (b *BatchJob) ItemLog(index int) string {
	if index < 0 || index >= len(b.items) {
		return ""
	}
	return b.items[index].log()
}

// Snapshot returns the current batch progress, including an overall percentage
// and estimated time remaining.
func (b *BatchJob) Snapshot() BatchProgress {
	items := make([]BatchItemStatus, len(b.items))
	completed, succeeded, failed, running := 0, 0, 0, 0

	var totalDur, processedDur, aggSpeed float64
	allKnown := true // every item's duration is known → duration-weighted overall

	for i, it := range b.items {
		st, dur, proc, speed := it.snapshot()
		items[i] = st

		if dur > 0 {
			totalDur += dur
		} else {
			allKnown = false
		}
		switch {
		case st.Done:
			completed++
			if st.Success {
				succeeded++
				processedDur += dur // finished items contribute their full length
			} else {
				failed++
			}
		case st.Started:
			running++
			processedDur += proc // partial progress of a running item
			aggSpeed += speed    // parallel items add throughput
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Overall percent: duration-weighted when possible, else item-count based.
	percent := 0.0
	if b.done {
		percent = 100
	} else if allKnown && totalDur > 0 {
		percent = processedDur / totalDur * 100
	} else if len(b.items) > 0 {
		percent = float64(completed) / float64(len(b.items)) * 100
	}
	if percent > 100 {
		percent = 100
	}

	// Overall ETA: remaining media divided by the combined throughput of all
	// items currently converting. Only when every duration is known.
	eta := -1.0
	if !b.done && allKnown && totalDur > 0 && aggSpeed > 0 {
		remaining := totalDur - processedDur
		if remaining < 0 {
			remaining = 0
		}
		eta = remaining / aggSpeed
	}

	return BatchProgress{
		Total:     len(b.items),
		Completed: completed,
		Succeeded: succeeded,
		Failed:    failed,
		Running:   running,
		Items:     items,
		Percent:   percent,
		ETA:       eta,
		Elapsed:   time.Since(b.start).Seconds(),
		Done:      b.done,
		Canceled:  b.canceled,
	}
}

// baseName extracts the file stem from a URL or local path, dropping any query
// string, fragment, directory and extension.
func baseName(source string) string {
	s := strings.TrimSpace(source)
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "/\\")
	if i := strings.LastIndexAny(s, "/\\"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.LastIndexByte(s, '.'); i > 0 {
		s = s[:i]
	}
	if s == "" {
		return "output"
	}
	return s
}

// uniqueName returns "stem.ext", appending "-2", "-3"… when that name is already
// taken within the batch so outputs don't overwrite one another.
func uniqueName(stem, ext string, used map[string]bool) string {
	name := stem + "." + ext
	for i := 2; used[strings.ToLower(name)]; i++ {
		name = stem + "-" + strconv.Itoa(i) + "." + ext
	}
	used[strings.ToLower(name)] = true
	return name
}
