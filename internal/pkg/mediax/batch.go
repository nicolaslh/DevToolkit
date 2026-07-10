package mediax

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

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
	ETA     float64 `json:"eta"`
	Done    bool    `json:"done"`
	Success bool    `json:"success"`
	Error   string  `json:"error"`
}

// BatchProgress is an immutable snapshot of a batch conversion.
type BatchProgress struct {
	Total     int               `json:"total"`
	Completed int               `json:"completed"` // items finished (success or failure)
	Succeeded int               `json:"succeeded"`
	Failed    int               `json:"failed"`
	Current   int               `json:"current"` // index of the running item, -1 when none
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
	done      bool
	success   bool
	errMsg    string
}

func (it *batchItem) setDuration(d float64) {
	it.mu.Lock()
	it.duration = d
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

// eta estimates remaining seconds for this item from its speed and how much
// media is left. Returns -1 when it can't be estimated. Caller holds it.mu.
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
		Done:     it.done,
		Success:  it.success,
		Error:    it.errMsg,
	}
	return st, it.duration, it.processed, it.speed
}

// BatchJob converts several m3u8 sources sequentially, writing each result into
// a shared output directory. Output names reuse the source's base name with the
// target format's extension.
type BatchJob struct {
	items []*batchItem
	opts  Options
	start time.Time

	mu       sync.Mutex
	current  int
	done     bool
	canceled bool

	cancelCh   chan struct{}
	cancelOnce sync.Once

	runMu   sync.Mutex
	running *Job
}

// StartBatch validates inputs, computes per-source output paths and launches a
// sequential conversion in the background. It returns a job to poll.
func StartBatch(sources []string, outputDir string, opts Options) (*BatchJob, error) {
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
		items:    items,
		opts:     opts,
		start:    time.Now(),
		current:  -1,
		cancelCh: make(chan struct{}),
	}
	go b.run()
	return b, nil
}

func (b *BatchJob) run() {
	// Probe every source's duration up front (best-effort) so the overall
	// progress and ETA can be computed across the whole batch, not just the
	// item currently converting.
	for _, it := range b.items {
		if b.stopped() {
			break
		}
		if res, err := Probe(it.source); err == nil {
			it.setDuration(res.Duration)
		}
	}

	for i, it := range b.items {
		if b.stopped() {
			break
		}
		b.setCurrent(i)
		b.convertOne(it)
	}
	b.setCurrent(-1)
	b.mu.Lock()
	b.done = true
	b.mu.Unlock()
}

func (b *BatchJob) convertOne(it *batchItem) {
	it.mu.Lock()
	duration := it.duration
	it.mu.Unlock()

	job, err := Start(it.source, it.output, b.opts, duration)
	if err != nil {
		it.fail(err.Error())
		return
	}
	b.setRunning(job)
	defer b.clearRunning()

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

func (b *BatchJob) setCurrent(i int) {
	b.mu.Lock()
	b.current = i
	b.mu.Unlock()
}

func (b *BatchJob) setRunning(j *Job) {
	b.runMu.Lock()
	b.running = j
	b.runMu.Unlock()
}

func (b *BatchJob) clearRunning() {
	b.runMu.Lock()
	b.running = nil
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

// Cancel stops the batch and the item currently converting.
func (b *BatchJob) Cancel() {
	b.cancelOnce.Do(func() {
		b.mu.Lock()
		b.canceled = true
		b.mu.Unlock()
		close(b.cancelCh)
		b.runMu.Lock()
		if b.running != nil {
			b.running.Cancel()
		}
		b.runMu.Unlock()
	})
}

// Snapshot returns the current batch progress, including an overall percentage
// and estimated time remaining.
func (b *BatchJob) Snapshot() BatchProgress {
	items := make([]BatchItemStatus, len(b.items))
	completed, succeeded, failed := 0, 0, 0

	var totalDur, processedDur, curSpeed float64
	allKnown := true // every item's duration is known → duration-weighted overall

	for i, it := range b.items {
		st, dur, proc, speed := it.snapshot()
		items[i] = st

		if dur > 0 {
			totalDur += dur
		} else {
			allKnown = false
		}
		if st.Done {
			completed++
			if st.Success {
				succeeded++
				processedDur += dur // finished items contribute their full length
			} else {
				failed++
			}
		} else if !st.Done && proc > 0 {
			processedDur += proc // partial progress of the running item
			if speed > curSpeed {
				curSpeed = speed
			}
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

	// Overall ETA: only when all durations are known and something is encoding.
	eta := -1.0
	if !b.done && allKnown && totalDur > 0 && curSpeed > 0 {
		remaining := totalDur - processedDur
		if remaining < 0 {
			remaining = 0
		}
		eta = remaining / curSpeed
	}

	return BatchProgress{
		Total:     len(b.items),
		Completed: completed,
		Succeeded: succeeded,
		Failed:    failed,
		Current:   b.current,
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
