package mediax

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// hwEncoder describes a hardware H.264 encoder and the codec arguments that
// select a good quality/speed operating point for it.
type hwEncoder struct {
	// Name is the ffmpeg encoder id (e.g. "h264_videotoolbox").
	Name string
	// Args are the extra codec flags appended after "-c:v <Name>".
	Args []string
}

// hwCandidates lists hardware H.264 encoders to try, most-preferred first, for
// the current platform. Quality flags differ per encoder family.
func hwCandidates() []hwEncoder {
	switch runtime.GOOS {
	case "darwin":
		// Apple Silicon/Intel media engine. -q:v is constant-quality (1-100).
		return []hwEncoder{{Name: "h264_videotoolbox", Args: []string{"-q:v", "60"}}}
	case "windows":
		return []hwEncoder{
			{Name: "h264_nvenc", Args: []string{"-preset", "p4", "-cq", "23"}},
			{Name: "h264_qsv", Args: []string{"-global_quality", "23"}},
			{Name: "h264_amf", Args: []string{"-quality", "balanced"}},
		}
	case "linux":
		return []hwEncoder{
			{Name: "h264_nvenc", Args: []string{"-preset", "p4", "-cq", "23"}},
			{Name: "h264_qsv", Args: []string{"-global_quality", "23"}},
		}
	default:
		return nil
	}
}

var (
	hwOnce sync.Once
	hwEnc  *hwEncoder
)

// bestHWEncoder returns the fastest usable hardware encoder, or nil when none
// is available. Detection is expensive (it runs a tiny probe encode to confirm
// the hardware actually works, not just that ffmpeg lists the encoder), so the
// result is cached for the app's lifetime.
func bestHWEncoder() *hwEncoder {
	hwOnce.Do(func() {
		bin := resolveFFmpeg()
		if bin == "" {
			return
		}
		listed := listedEncoders(bin)
		for _, c := range hwCandidates() {
			if !listed[c.Name] {
				continue
			}
			if probeEncoder(bin, c) {
				enc := c
				hwEnc = &enc
				return
			}
		}
	})
	return hwEnc
}

// listedEncoders returns the set of encoder ids ffmpeg was built with.
func listedEncoders(bin string) map[string]bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "-hide_banner", "-encoders").Output()
	if err != nil {
		return nil
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		// Lines look like: " V..... h264_videotoolbox   VideoToolbox H.264 ..."
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.HasPrefix(fields[0], "V") {
			set[fields[1]] = true
		}
	}
	return set
}

// probeEncoder confirms an encoder actually works by encoding a few synthetic
// frames to null. This weeds out encoders that are listed but unusable (e.g.
// NVENC with no NVIDIA GPU present).
func probeEncoder(bin string, e hwEncoder) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=320x240:r=15", "-frames:v", "10",
		"-c:v", e.Name,
	}
	args = append(args, e.Args...)
	args = append(args, "-f", "null", "-")
	return exec.CommandContext(ctx, bin, args...).Run() == nil
}

// videoEncodeArgs returns the "-c:v ..." arguments for a re-encode, preferring a
// validated hardware encoder when opts.HWAccel is set and falling back to a
// multi-threaded libx264 otherwise.
func videoEncodeArgs(opts Options) []string {
	if opts.HWAccel {
		if e := bestHWEncoder(); e != nil {
			return append([]string{"-c:v", e.Name}, e.Args...)
		}
	}
	return []string{"-c:v", "libx264", "-preset", "veryfast", "-threads", "0"}
}
