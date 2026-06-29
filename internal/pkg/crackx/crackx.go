// Package crackx implements local, offline password recovery (brute-force and
// dictionary attacks) for encrypted ZIP, PDF and Office (docx/xlsx/pptx) files.
//
// It is intended to help users recover passwords for files they own. All work
// happens on the local machine; no data leaves the process. Candidate
// passwords are never logged.
package crackx

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// Format identifies the container type being attacked.
type Format string

const (
	FormatAuto   Format = "auto"
	FormatZIP    Format = "zip"
	FormatPDF    Format = "pdf"
	FormatOffice Format = "office" // docx / xlsx / pptx (OOXML, OLE-encrypted)
)

// Mode selects the candidate-generation strategy.
type Mode string

const (
	ModeBrute Mode = "brute" // charset + length range
	ModeDict  Mode = "dict"  // user-supplied wordlist
)

// Charset describes which character classes feed the brute-force generator.
type Charset struct {
	Lower   bool   `json:"lower"`   // a-z
	Upper   bool   `json:"upper"`   // A-Z
	Digits  bool   `json:"digits"`  // 0-9
	Symbols bool   `json:"symbols"` // common ASCII symbols
	Custom  string `json:"custom"`  // extra characters to include
}

const symbolChars = "!@#$%^&*()-_=+[]{};:,.?/"

// alphabet returns the de-duplicated, ordered set of characters to try.
func (c Charset) alphabet() string {
	var b strings.Builder
	seen := map[rune]bool{}
	add := func(s string) {
		for _, r := range s {
			if !seen[r] {
				seen[r] = true
				b.WriteRune(r)
			}
		}
	}
	if c.Lower {
		add("abcdefghijklmnopqrstuvwxyz")
	}
	if c.Upper {
		add("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	}
	if c.Digits {
		add("0123456789")
	}
	if c.Symbols {
		add(symbolChars)
	}
	add(c.Custom)
	return b.String()
}

// Options configures a cracking job.
type Options struct {
	Format   Format  `json:"format"`
	Mode     Mode    `json:"mode"`
	MinLen   int     `json:"minLen"`
	MaxLen   int     `json:"maxLen"`
	Charset  Charset `json:"charset"`
	Wordlist string  `json:"wordlist"` // newline-separated, for ModeDict
}

// Progress is an immutable snapshot of a job's state, returned to the frontend.
type Progress struct {
	Tried       int64   `json:"tried"`
	Total       int64   `json:"total"` // -1 when the space is unbounded/too large to count
	Current     string  `json:"current"`
	Found       bool    `json:"found"`
	Password    string  `json:"password"`
	Done        bool    `json:"done"`
	Canceled    bool    `json:"canceled"`
	Error       string  `json:"error"`
	Elapsed     float64 `json:"elapsed"`     // seconds
	Rate        float64 `json:"rate"`        // attempts per second
	ETA         float64 `json:"eta"`         // estimated seconds remaining; -1 when unknown
	ResumedFrom int64   `json:"resumedFrom"` // candidates skipped because the job resumed a checkpoint
}

// ResumeInfo describes a previously-saved checkpoint that a new job can continue.
type ResumeInfo struct {
	Available bool  `json:"available"`
	Tried     int64 `json:"tried"`
	Total     int64 `json:"total"`
}

// Verifier reports whether a candidate password opens the encrypted container.
type Verifier interface {
	// Verify returns true when password is correct.
	Verify(password string) bool
}

// NewVerifier inspects data, resolves the format and returns a Verifier.
// It returns an error when the file is not encrypted or is unsupported.
func NewVerifier(format Format, filename string, data []byte) (Verifier, error) {
	resolved := resolveFormat(format, filename, data)
	switch resolved {
	case FormatZIP:
		return newZipVerifier(data)
	case FormatPDF:
		return newPDFVerifier(data)
	case FormatOffice:
		return newOfficeVerifier(data)
	default:
		return nil, apperr.New(apperr.Unsupported, "无法识别文件类型，请手动选择格式（zip / pdf / office）")
	}
}

// Job is a running or finished cracking task.
type Job struct {
	tried   atomic.Int64
	total   int64
	start   time.Time
	current atomic.Pointer[string]

	mu       sync.Mutex
	found    bool
	password string
	done     bool
	canceled bool
	errMsg   string

	cancelCh   chan struct{}
	cancelOnce sync.Once

	resumedFrom int64

	// Watermark bookkeeping: the largest contiguous prefix of dispatched
	// candidates that has finished verifying. Used as the safe resume offset
	// under parallel execution (completions can land out of order).
	wmMu   sync.Mutex
	wmNext int64
	wmDone map[int64]bool
}

// Start launches the attack in a background goroutine and returns immediately.
// resumeFrom is the number of candidates already tried in a previous run; the
// engine skips that many before resuming. Pass 0 to start from scratch.
func Start(v Verifier, opts Options, resumeFrom int64) *Job {
	if resumeFrom < 0 {
		resumeFrom = 0
	}
	j := &Job{
		start:       time.Now(),
		cancelCh:    make(chan struct{}),
		resumedFrom: resumeFrom,
		wmDone:      map[int64]bool{},
	}
	j.total = countSpace(opts)
	j.tried.Store(resumeFrom)
	go j.run(v, opts, resumeFrom)
	return j
}

// Cancel requests the job to stop as soon as possible.
func (j *Job) Cancel() {
	j.cancelOnce.Do(func() { close(j.cancelCh) })
}

func (j *Job) stopped() bool {
	select {
	case <-j.cancelCh:
		return true
	default:
		return false
	}
}

// Snapshot returns the current progress.
func (j *Job) Snapshot() Progress {
	current := ""
	if p := j.current.Load(); p != nil {
		current = *p
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	elapsed := time.Since(j.start).Seconds()
	tried := j.tried.Load()
	rate := 0.0
	if elapsed > 0 {
		rate = float64(tried) / elapsed
	}
	// Estimate remaining time from the current rate and the candidates left.
	// Unknown (-1) when the space is unbounded or there is no rate yet.
	eta := -1.0
	if !j.done && j.total >= 0 && rate > 0 {
		remaining := float64(j.total - tried)
		if remaining < 0 {
			remaining = 0
		}
		eta = remaining / rate
	}
	return Progress{
		Tried:       tried,
		Total:       j.total,
		Current:     current,
		Found:       j.found,
		Password:    j.password,
		Done:        j.done,
		Canceled:    j.canceled,
		Error:       j.errMsg,
		Elapsed:     elapsed,
		Rate:        rate,
		ETA:         eta,
		ResumedFrom: j.resumedFrom,
	}
}

// ResumeOffset returns the safe candidate offset to resume from: the previous
// resume base plus the largest fully-completed contiguous prefix of this run.
func (j *Job) ResumeOffset() int64 {
	j.wmMu.Lock()
	defer j.wmMu.Unlock()
	return j.resumedFrom + j.wmNext
}

func (j *Job) setCurrent(pw string) {
	j.current.Store(&pw)
}

// markDone records that the candidate with the given sequence number finished
// verifying, advancing the contiguous-completion watermark.
func (j *Job) markDone(seq int64) {
	j.wmMu.Lock()
	if seq == j.wmNext {
		j.wmNext++
		for j.wmDone[j.wmNext] {
			delete(j.wmDone, j.wmNext)
			j.wmNext++
		}
	} else {
		j.wmDone[seq] = true
	}
	j.wmMu.Unlock()
}

func (j *Job) finish(found bool, password, errMsg string, canceled bool) {
	j.mu.Lock()
	j.found = found
	j.password = password
	j.errMsg = errMsg
	j.canceled = canceled
	j.done = true
	j.mu.Unlock()
}

func (j *Job) run(v Verifier, opts Options, resumeFrom int64) {
	// Fail fast on an empty brute-force charset.
	if opts.Mode != ModeDict && len(opts.Charset.alphabet()) == 0 {
		j.finish(false, "", "请至少选择一种字符集", false)
		return
	}

	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}

	type candidate struct {
		seq int64
		pw  string
	}
	ch := make(chan candidate, workers*4)

	// Producer: emit candidates in deterministic order, numbered from 0, until
	// the space is exhausted or the job is canceled.
	go func() {
		defer close(ch)
		var seq int64
		emit := func(pw string) bool {
			select {
			case ch <- candidate{seq: seq, pw: pw}:
				seq++
				return true
			case <-j.cancelCh:
				return false
			}
		}
		if opts.Mode == ModeDict {
			produceDict(opts, resumeFrom, emit)
		} else {
			produceBrute(opts, resumeFrom, emit)
		}
	}()

	// Workers verify candidates concurrently. The first match cancels the rest.
	var (
		foundOnce sync.Once
		foundPW   string
		found     atomic.Bool
		wg        sync.WaitGroup
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c := range ch {
				if found.Load() || j.stopped() {
					return
				}
				ok := v.Verify(c.pw)
				j.tried.Add(1)
				j.setCurrent(c.pw)
				j.markDone(c.seq)
				if ok {
					foundOnce.Do(func() {
						foundPW = c.pw
						found.Store(true)
					})
					j.Cancel() // stop the producer and peer workers
					return
				}
			}
		}()
	}
	wg.Wait()

	switch {
	case found.Load():
		j.finish(true, foundPW, "", false)
	case j.stopped():
		j.finish(false, "", "", true)
	default:
		j.finish(false, "", "已尝试全部候选口令，未找到匹配项", false)
	}
}

// produceDict walks the wordlist line by line, skipping the first `skip`
// candidates so an interrupted run can continue where it stopped.
func produceDict(opts Options, skip int64, emit func(string) bool) {
	var seen int64
	for _, line := range strings.Split(opts.Wordlist, "\n") {
		pw := strings.TrimRight(line, "\r")
		if pw == "" {
			continue
		}
		if seen < skip {
			seen++
			continue
		}
		if !emit(pw) {
			return
		}
	}
}

// produceBrute enumerates every combination of the alphabet for each length in
// the configured range, shortest first. When skip > 0 it jumps directly to the
// matching position so an interrupted run can resume.
func produceBrute(opts Options, skip int64, emit func(string) bool) {
	alphabet := []rune(opts.Charset.alphabet())
	if len(alphabet) == 0 {
		return
	}
	minLen, maxLen := normalizeLen(opts.MinLen, opts.MaxLen)

	startLen, startIdx, ok := locate(skip, int64(len(alphabet)), minLen, maxLen)
	if !ok {
		// The skip offset is past the entire search space; nothing left to do.
		return
	}

	for length := startLen; length <= maxLen; length++ {
		idx := make([]int, length)
		if length == startLen {
			copy(idx, startIdx)
		}
		buf := make([]rune, length)
		for {
			for i := 0; i < length; i++ {
				buf[i] = alphabet[idx[i]]
			}
			if !emit(string(buf)) {
				return
			}
			// Increment the odometer (least-significant position first).
			pos := length - 1
			for pos >= 0 {
				idx[pos]++
				if idx[pos] < len(alphabet) {
					break
				}
				idx[pos] = 0
				pos--
			}
			if pos < 0 {
				break // exhausted this length
			}
		}
	}
}

// locate maps a linear candidate offset to the (length, odometer) position
// where brute-force enumeration should resume. ok is false when the offset is
// beyond the whole search space.
func locate(skip, base int64, minLen, maxLen int) (int, []int, bool) {
	if skip < 0 {
		skip = 0
	}
	remaining := skip
	for length := minLen; length <= maxLen; length++ {
		count, overflow := powGuard(base, length)
		if overflow || remaining < count {
			return length, decompose(remaining, base, length), true
		}
		remaining -= count
	}
	return 0, nil, false
}

// decompose writes index as a base-`base` number across `length` digits, with
// position 0 holding the most-significant digit (matching the odometer order).
func decompose(index, base int64, length int) []int {
	idx := make([]int, length)
	for i := length - 1; i >= 0; i-- {
		idx[i] = int(index % base)
		index /= base
	}
	return idx
}

// powGuard returns base**length, flagging overflow past ~2^62.
func powGuard(base int64, length int) (int64, bool) {
	const limit = int64(1) << 62
	res := int64(1)
	for i := 0; i < length; i++ {
		res *= base
		if res > limit {
			return 0, true
		}
	}
	return res, false
}

func normalizeLen(minLen, maxLen int) (int, int) {
	if minLen < 1 {
		minLen = 1
	}
	if maxLen < minLen {
		maxLen = minLen
	}
	return minLen, maxLen
}

// countSpace estimates the total number of candidates, returning -1 when the
// value is unbounded or larger than int64 can safely hold.
func countSpace(opts Options) int64 {
	if opts.Mode == ModeDict {
		var n int64
		for _, line := range strings.Split(opts.Wordlist, "\n") {
			if strings.TrimRight(line, "\r") != "" {
				n++
			}
		}
		return n
	}
	base := int64(len([]rune(opts.Charset.alphabet())))
	if base == 0 {
		return 0
	}
	minLen, maxLen := normalizeLen(opts.MinLen, opts.MaxLen)
	const maxCount = int64(1) << 62
	var total int64
	for length := minLen; length <= maxLen; length++ {
		term := int64(1)
		for i := 0; i < length; i++ {
			term *= base
			if term > maxCount {
				return -1
			}
		}
		total += term
		if total > maxCount {
			return -1
		}
	}
	return total
}

// resolveFormat maps an explicit/auto format plus filename/magic bytes to a
// concrete Format.
func resolveFormat(format Format, filename string, data []byte) Format {
	switch format {
	case FormatZIP, FormatPDF, FormatOffice:
		return format
	}
	name := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(name, ".pdf"):
		return FormatPDF
	case strings.HasSuffix(name, ".docx"), strings.HasSuffix(name, ".xlsx"),
		strings.HasSuffix(name, ".pptx"), strings.HasSuffix(name, ".docm"),
		strings.HasSuffix(name, ".xlsm"), strings.HasSuffix(name, ".pptm"):
		return FormatOffice
	case strings.HasSuffix(name, ".zip"):
		return FormatZIP
	}
	// Fall back to magic bytes.
	switch {
	case len(data) >= 5 && string(data[:5]) == "%PDF-":
		return FormatPDF
	case len(data) >= 8 && data[0] == 0xD0 && data[1] == 0xCF && data[2] == 0x11 && data[3] == 0xE0:
		return FormatOffice // OLE compound file (encrypted OOXML)
	case len(data) >= 4 && data[0] == 'P' && data[1] == 'K':
		return FormatZIP
	}
	return FormatAuto
}

// String renders a human-friendly progress summary (used in tests/debugging).
func (p Progress) String() string {
	return fmt.Sprintf("tried=%d total=%d found=%v done=%v", p.Tried, p.Total, p.Found, p.Done)
}
