// Package crackx implements local, offline password recovery (brute-force and
// dictionary attacks) for encrypted ZIP, PDF and Office (docx/xlsx/pptx) files.
//
// It is intended to help users recover passwords for files they own. All work
// happens on the local machine; no data leaves the process. Candidate
// passwords are never logged.
package crackx

import (
	"fmt"
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
	Tried    int64   `json:"tried"`
	Total    int64   `json:"total"` // -1 when the space is unbounded/too large to count
	Current  string  `json:"current"`
	Found    bool    `json:"found"`
	Password string  `json:"password"`
	Done     bool    `json:"done"`
	Canceled bool    `json:"canceled"`
	Error    string  `json:"error"`
	Elapsed  float64 `json:"elapsed"` // seconds
	Rate     float64 `json:"rate"`    // attempts per second
	ETA      float64 `json:"eta"`     // estimated seconds remaining; -1 when unknown
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
	tried atomic.Int64
	total int64
	start time.Time

	mu       sync.Mutex
	current  string
	found    bool
	password string
	done     bool
	canceled bool
	errMsg   string

	cancelCh   chan struct{}
	cancelOnce sync.Once
}

// Start launches the attack in a background goroutine and returns immediately.
func Start(v Verifier, opts Options) *Job {
	j := &Job{
		start:    time.Now(),
		cancelCh: make(chan struct{}),
	}
	j.total = countSpace(opts)
	go j.run(v, opts)
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
		Tried:    tried,
		Total:    j.total,
		Current:  j.current,
		Found:    j.found,
		Password: j.password,
		Done:     j.done,
		Canceled: j.canceled,
		Error:    j.errMsg,
		Elapsed:  elapsed,
		Rate:     rate,
		ETA:      eta,
	}
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

func (j *Job) run(v Verifier, opts Options) {
	hit := func(pw string) bool {
		j.mu.Lock()
		j.current = pw
		j.mu.Unlock()
		j.tried.Add(1)
		return v.Verify(pw)
	}

	var (
		found bool
		pw    string
	)
	switch opts.Mode {
	case ModeDict:
		found, pw = j.runDict(opts, hit)
	default:
		found, pw = j.runBrute(opts, hit)
	}

	if j.stopped() && !found {
		j.finish(false, "", "", true)
		return
	}
	if found {
		j.finish(true, pw, "", false)
		return
	}
	j.finish(false, "", "已尝试全部候选口令，未找到匹配项", false)
}

// runDict walks the wordlist line by line.
func (j *Job) runDict(opts Options, hit func(string) bool) (bool, string) {
	for _, line := range strings.Split(opts.Wordlist, "\n") {
		if j.stopped() {
			return false, ""
		}
		pw := strings.TrimRight(line, "\r")
		if pw == "" {
			continue
		}
		if hit(pw) {
			return true, pw
		}
	}
	return false, ""
}

// runBrute enumerates every combination of the alphabet for each length in the
// configured range, shortest first.
func (j *Job) runBrute(opts Options, hit func(string) bool) (bool, string) {
	alphabet := []rune(opts.Charset.alphabet())
	if len(alphabet) == 0 {
		j.finish(false, "", "请至少选择一种字符集", false)
		return false, ""
	}
	minLen, maxLen := normalizeLen(opts.MinLen, opts.MaxLen)

	for length := minLen; length <= maxLen; length++ {
		idx := make([]int, length)
		buf := make([]rune, length)
		for {
			if j.stopped() {
				return false, ""
			}
			for i := 0; i < length; i++ {
				buf[i] = alphabet[idx[i]]
			}
			if hit(string(buf)) {
				return true, string(buf)
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
	return false, ""
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
