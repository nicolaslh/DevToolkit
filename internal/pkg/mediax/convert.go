package mediax

import (
	"bufio"
	"context"
	"io"
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
	output   string
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
	tail      []string // last few ffmpeg stderr lines, for diagnostics
}

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
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return nil, apperr.New(apperr.Unsupported, "未检测到 ffmpeg，请先安装后重试")
	}

	ctx, cancel := context.WithCancel(context.Background())
	args := buildArgs(source, output, opts)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

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
	args := []string{
		"-y", // overwrite output
		"-protocol_whitelist", "file,http,https,tcp,tls,crypto",
		"-i", source,
	}

	switch opts.Format {
	case MP3:
		// Audio-only extraction always re-encodes to MP3.
		args = append(args, "-vn", "-c:a", "libmp3lame", "-q:a", "2")
	default:
		if opts.ReEncode {
			args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-c:a", "aac")
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
		"-nostats",
		"-loglevel", "error",
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

// readStderr retains the last few error lines to explain a failure.
func (j *Job) readStderr(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		j.mu.Lock()
		j.tail = append(j.tail, line)
		if len(j.tail) > 8 {
			j.tail = j.tail[len(j.tail)-8:]
		}
		j.mu.Unlock()
	}
}

// wait blocks until ffmpeg exits and records the final outcome.
func (j *Job) wait() {
	err := j.cmd.Wait()
	j.mu.Lock()
	defer j.mu.Unlock()
	j.done = true
	if j.canceled {
		return
	}
	if err != nil {
		j.errMsg = j.failureMessage()
		return
	}
	j.success = true
	if j.duration > 0 {
		j.processed = j.duration
	}
}

// failureMessage builds a user-facing error from the retained ffmpeg output.
// Must be called with j.mu held.
func (j *Job) failureMessage() string {
	detail := strings.TrimSpace(strings.Join(j.tail, "；"))
	if detail == "" {
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
