package mediax

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// Progress is an immutable snapshot of a conversion job, polled by the frontend.
type Progress struct {
	// Percent is 0-100, or -1 when the total duration is unknown (e.g. a master
	// playlist we couldn't probe, or a live stream).
	Percent float64 `json:"percent"`
	// Processed is how many seconds of media ffmpeg has written so far.
	Processed float64 `json:"processed"`
	// Duration is the total media length in seconds (0 when unknown).
	Duration float64 `json:"duration"`
	// Speed is the encoding speed relative to realtime (e.g. 12.0 = 12x).
	Speed float64 `json:"speed"`
	// Elapsed is the wall-clock time since the job started, in seconds.
	Elapsed  float64 `json:"elapsed"`
	Done     bool    `json:"done"`
	Canceled bool    `json:"canceled"`
	// Success is true when ffmpeg finished and wrote the output file.
	Success bool   `json:"success"`
	Error   string `json:"error"`
	// Output is the path of the produced file (set on success).
	Output string `json:"output"`
}

// Job is a running or finished conversion.
type Job struct {
	output   string // final destination chosen by the user
	tempOut  string // temp file ffmpeg writes to; moved to output on success
	duration float64
	start    time.Time

	cmd        *exec.Cmd
	cancel     context.CancelFunc
	cancelOnce sync.Once

	mu        sync.Mutex
	processed float64
	speed     float64
	done      bool
	canceled  bool
	success   bool
	errMsg    string
	exitErr   string   // ffmpeg's process exit error (e.g. "exit status 1")
	logLines  []string // rolling ffmpeg log (stderr), for display + diagnostics
}

// maxLogLines caps the retained ffmpeg log so a long-running or chatty
// conversion can't grow memory without bound.
const maxLogLines = 500

// Start validates options, builds the ffmpeg command and runs it in the
// background. duration (from Probe) is optional; pass 0 when unknown to get an
// indeterminate progress bar. It returns immediately with a Job to poll.
func Start(source, output string, opts Options, duration float64) (*Job, error) {
	source = strings.TrimSpace(source)
	output = strings.TrimSpace(output)
	if source == "" {
		return nil, apperr.New(apperr.InvalidInput, "请填写 m3u8 地址或选择本地文件")
	}
	if output == "" {
		return nil, apperr.New(apperr.InvalidInput, "请选择输出文件位置")
	}
	if !opts.Format.valid() {
		return nil, apperr.Newf(apperr.Unsupported, "不支持的输出格式：%s", opts.Format)
	}
	ffmpegBin := resolveFFmpeg()
	if ffmpegBin == "" {
		return nil, apperr.New(apperr.Unsupported, "未检测到 ffmpeg，请先安装后重试")
	}

	// Write to a temp file first, then move it to the user's destination from
	// the app process. A packaged macOS GUI app can grant itself access to
	// TCC-protected folders (Desktop/Documents/Downloads), but the ffmpeg child
	// process cannot — so letting ffmpeg write straight to such a folder can
	// fail with "Operation not permitted". The temp dir is never protected.
	tempOut, err := tempOutputPath(opts.Format.Ext())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	args := buildArgs(source, tempOut, opts)
	cmd := exec.CommandContext(ctx, ffmpegBin, args...)
	// Run from a writable temp dir (a Finder-launched app's CWD is "/", which
	// isn't writable) and give the child a usable PATH (the app inherits only a
	// minimal PATH from launchd).
	cmd.Dir = os.TempDir()
	cmd.Env = augmentedEnv(ffmpegBin)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, apperr.Newf(apperr.Unsupported, "无法启动 ffmpeg：%v", err)
	}
	// ffmpeg writes machine-readable progress to stdout via "-progress pipe:1".
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, apperr.Newf(apperr.Unsupported, "无法启动 ffmpeg：%v", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, apperr.Newf(apperr.Unsupported, "无法启动 ffmpeg：%v", err)
	}

	j := &Job{
		output:   output,
		tempOut:  tempOut,
		duration: duration,
		start:    time.Now(),
		cmd:      cmd,
		cancel:   cancel,
	}
	go j.readProgress(stdout)
	go j.readStderr(stderr)
	go j.wait()
	return j, nil
}

// buildArgs assembles the ffmpeg argument list for the requested conversion.
func buildArgs(source, output string, opts Options) []string {
	args := []string{"-y"} // overwrite output
	if isRemote(source) {
		// Reuse TCP connections and fetch segments over multiple/pipelined
		// requests instead of one-at-a-time — a big speedup for HLS streams
		// made of many small segments (the usual bottleneck in copy mode).
		args = append(args,
			"-http_persistent", "1",
			"-http_multiple", "1",
			// Recover from transient network drops instead of aborting, and don't
			// stall forever on a dead segment — both waste conversion time.
			"-reconnect", "1",
			"-reconnect_streamed", "1",
			"-reconnect_delay_max", "5",
			"-rw_timeout", "15000000", // 15s I/O timeout (microseconds)
		)
	}
	args = append(args,
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-i", source,
	)

	switch opts.Format {
	case MP3:
		// Audio-only extraction always re-encodes to MP3.
		args = append(args, "-vn", "-c:a", "libmp3lame", "-q:a", "2")
	default:
		if opts.ReEncode {
			args = append(args, videoEncodeArgs(opts)...)
			args = append(args, "-c:a", "aac")
		} else {
			args = append(args, "-c", "copy")
			// TS segments typically carry ADTS AAC; MP4/MOV need it repackaged.
			if opts.Format == MP4 || opts.Format == MOV {
				args = append(args, "-bsf:a", "aac_adtstoasc")
			}
		}
	}

	args = append(args,
		"-progress", "pipe:1",
		"-nostats", // suppress the per-frame stats spam (progress comes via pipe:1)
		"-hide_banner",
		"-loglevel", "info", // stream mapping + warnings/errors: a useful, non-noisy log
		output,
	)
	return args
}

// readProgress consumes ffmpeg's "-progress" key=value stream.
func (j *Job) readProgress(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, val, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_us", "out_time_ms":
			// Despite the name, ffmpeg emits microseconds for both keys.
			if us, err := strconv.ParseFloat(val, 64); err == nil && us >= 0 {
				j.mu.Lock()
				j.processed = us / 1_000_000
				j.mu.Unlock()
			}
		case "speed":
			v := strings.TrimSpace(strings.TrimSuffix(val, "x"))
			if s, err := strconv.ParseFloat(v, 64); err == nil {
				j.mu.Lock()
				j.speed = s
				j.mu.Unlock()
			}
		}
	}
}

// readStderr retains ffmpeg's log lines (bounded) for display and diagnostics.
func (j *Job) readStderr(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		j.mu.Lock()
		j.logLines = append(j.logLines, line)
		if len(j.logLines) > maxLogLines {
			j.logLines = j.logLines[len(j.logLines)-maxLogLines:]
		}
		j.mu.Unlock()
	}
}

// Log returns the retained ffmpeg output as a single string.
func (j *Job) Log() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return strings.Join(j.logLines, "\n")
}

// wait blocks until ffmpeg exits, moves the finished temp file to its
// destination and records the final outcome.
func (j *Job) wait() {
	err := j.cmd.Wait()

	if j.stoppedByCancel() {
		_ = os.Remove(j.tempOut)
		j.mu.Lock()
		j.done = true
		j.mu.Unlock()
		return
	}

	if err != nil {
		_ = os.Remove(j.tempOut)
		j.mu.Lock()
		j.exitErr = err.Error()
		j.errMsg = j.failureMessage()
		j.done = true
		j.mu.Unlock()
		return
	}

	// ffmpeg succeeded; move the temp file to the user's chosen destination.
	// The app process does the move, so it works even for TCC-protected folders.
	if moveErr := moveFile(j.tempOut, j.output); moveErr != nil {
		_ = os.Remove(j.tempOut)
		j.mu.Lock()
		j.errMsg = "转换完成但无法写入目标文件夹：" + moveErr.Error()
		j.done = true
		j.mu.Unlock()
		return
	}

	j.mu.Lock()
	j.success = true
	if j.duration > 0 {
		j.processed = j.duration
	}
	j.done = true
	j.mu.Unlock()
}

// stoppedByCancel reports whether the job was canceled by the user.
func (j *Job) stoppedByCancel() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.canceled
}

// failureMessage builds a user-facing error from the tail of the ffmpeg log.
// Must be called with j.mu held.
func (j *Job) failureMessage() string {
	tail := j.logLines
	if len(tail) > 12 {
		tail = tail[len(tail)-12:]
	}
	detail := strings.TrimSpace(strings.Join(tail, "；"))
	if detail == "" {
		if j.exitErr != "" {
			return "转换失败（ffmpeg " + j.exitErr + "），请确认地址可访问、输出文件夹可写入"
		}
		return "转换失败，请检查地址或 ffmpeg 是否正常"
	}
	return "转换失败：" + detail
}

// Cancel stops ffmpeg as soon as possible.
func (j *Job) Cancel() {
	j.cancelOnce.Do(func() {
		j.mu.Lock()
		j.canceled = true
		j.mu.Unlock()
		j.cancel()
	})
}

// Snapshot returns the current progress.
func (j *Job) Snapshot() Progress {
	j.mu.Lock()
	defer j.mu.Unlock()

	percent := -1.0
	if j.duration > 0 {
		percent = j.processed / j.duration * 100
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
	}
	if j.success {
		percent = 100
	}

	return Progress{
		Percent:   percent,
		Processed: j.processed,
		Duration:  j.duration,
		Speed:     j.speed,
		Elapsed:   time.Since(j.start).Seconds(),
		Done:      j.done,
		Canceled:  j.canceled,
		Success:   j.success,
		Error:     j.errMsg,
		Output:    outputIf(j.success, j.output),
	}
}

func outputIf(ok bool, path string) string {
	if ok {
		return path
	}
	return ""
}
