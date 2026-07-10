// Package mediax converts HLS (m3u8) streams into regular media containers
// such as MP4/MKV/MOV using a locally installed ffmpeg binary.
//
// The m3u8 source may be a remote URL or a local playlist file. Conversion is
// delegated to ffmpeg: by default the audio/video streams are copied (fast,
// lossless remux); an optional re-encode mode produces H.264/AAC for maximum
// compatibility. All work happens on the local machine.
package mediax

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// FFmpegInfo reports whether ffmpeg is available and, if so, its location and
// version banner. The frontend uses this to guide the user through installing
// ffmpeg when it is missing.
type FFmpegInfo struct {
	Available bool   `json:"available"`
	Path      string `json:"path"`
	Version   string `json:"version"`
	// OS is the current platform (runtime.GOOS), so the UI can highlight the
	// matching install instructions.
	OS string `json:"os"`
	// InstallCmd is the recommended one-line install command for this platform.
	InstallCmd string `json:"installCmd"`
	// DownloadURL points to the official ffmpeg download page as a fallback.
	DownloadURL string `json:"downloadURL"`
	// HWEncoder is the detected, validated hardware H.264 encoder (e.g.
	// "h264_videotoolbox"); empty when none is usable. Only meaningful when
	// Available is true. The UI uses it to offer hardware-accelerated encoding.
	HWEncoder string `json:"hwEncoder"`
}

// installCmdFor returns the recommended install command for a given GOOS.
func installCmdFor(goos string) string {
	switch goos {
	case "darwin":
		return "brew install ffmpeg"
	case "windows":
		return "winget install Gyan.FFmpeg"
	case "linux":
		return "sudo apt install ffmpeg"
	default:
		return ""
	}
}

// Container identifies the target output format.
type Container string

const (
	MP4 Container = "mp4"
	MKV Container = "mkv"
	MOV Container = "mov"
	TS  Container = "ts"
	MP3 Container = "mp3" // audio-only extraction
)

// Ext returns the file extension (without dot) for the container.
func (c Container) Ext() string { return string(c) }

// valid reports whether c is a supported container.
func (c Container) valid() bool {
	switch c {
	case MP4, MKV, MOV, TS, MP3:
		return true
	default:
		return false
	}
}

// Options configures a conversion job.
type Options struct {
	// Format is the target container (mp4/mkv/mov/ts/mp3).
	Format Container `json:"format"`
	// ReEncode transcodes to H.264/AAC instead of copying streams. Slower, but
	// resolves codec/container incompatibilities. Ignored for audio-only output.
	ReEncode bool `json:"reEncode"`
	// HWAccel uses a hardware video encoder (VideoToolbox/NVENC/QSV/AMF) when
	// re-encoding, which is much faster and offloads the CPU. Ignored unless
	// ReEncode is set; falls back to software when no hardware encoder is usable.
	HWAccel bool `json:"hwAccel"`
}

// ProbeResult summarizes an m3u8 playlist before conversion.
type ProbeResult struct {
	// Master is true when the playlist lists variant streams rather than segments.
	Master bool `json:"master"`
	// Duration is the total media length in seconds (0 when unknown, e.g. for
	// master playlists or live streams).
	Duration float64 `json:"duration"`
	// Segments counts the media segments in the (resolved) media playlist.
	Segments int `json:"segments"`
	// Encrypted is true when the playlist declares AES-128 segment encryption.
	Encrypted bool `json:"encrypted"`
	// Live is true when the playlist has no #EXT-X-ENDLIST marker.
	Live bool `json:"live"`
}

// ffmpegExeName is the platform-specific binary name.
func ffmpegExeName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

// ffmpegSearchDirs lists directories to probe for ffmpeg beyond PATH. This is
// essential on macOS: GUI apps launched from Finder/Dock/Applications do NOT
// inherit the shell PATH, so Homebrew (/opt/homebrew, /usr/local) and MacPorts
// locations are invisible to exec.LookPath and must be checked explicitly.
func ffmpegSearchDirs() []string {
	var dirs []string
	// Alongside our own executable first, so a bundled ffmpeg wins.
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		dirs = append(dirs, exeDir, filepath.Join(exeDir, "..", "Resources"))
	}
	switch runtime.GOOS {
	case "darwin":
		dirs = append(dirs,
			"/opt/homebrew/bin", // Apple Silicon Homebrew
			"/usr/local/bin",    // Intel Homebrew
			"/opt/local/bin",    // MacPorts
			"/usr/bin",
		)
	case "linux":
		dirs = append(dirs, "/usr/bin", "/usr/local/bin", "/bin", "/snap/bin")
	case "windows":
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			dirs = append(dirs, filepath.Join(la, "Microsoft", "WinGet", "Links"))
		}
		if pf := os.Getenv("ProgramFiles"); pf != "" {
			dirs = append(dirs, filepath.Join(pf, "ffmpeg", "bin"))
		}
		dirs = append(dirs, `C:\ffmpeg\bin`)
	}
	return dirs
}

var (
	loginDirsOnce sync.Once
	loginDirs     []string
)

// cachedLoginShellDirs memoizes loginShellDirs for the app's lifetime. The
// login shell's PATH doesn't change while the app runs, and spawning an
// interactive shell is comparatively slow, so this runs at most once.
func cachedLoginShellDirs() []string {
	loginDirsOnce.Do(func() { loginDirs = loginShellDirs() })
	return loginDirs
}

// loginShellDirs returns the PATH entries from the user's login shell. GUI apps
// on macOS/Linux start with a minimal PATH; the interactive login shell loads
// the full profile (.zprofile AND .zshrc, .bash_profile/.bashrc) where package
// managers like Homebrew, MacPorts, asdf, nvm or conda add their bin dirs.
// Empty on Windows or on failure.
//
// The PATH is wrapped in sentinels so shell-startup output (banners, echoes
// from rc files) doesn't pollute the parsed value. Stdin is left nil (so it
// reads from /dev/null and never blocks) and a timeout guards against hangs.
func loginShellDirs() []string {
	if runtime.GOOS == "windows" {
		return nil
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	const begin, end = "__DTK_PATH_BEGIN__", "__DTK_PATH_END__"
	script := "printf '%s%s%s' " + begin + " \"$PATH\" " + end
	// -i (interactive) sources .zshrc/.bashrc; -l (login) sources the profile.
	out, err := exec.CommandContext(ctx, shell, "-ilc", script).Output()
	if err != nil && len(out) == 0 {
		return nil
	}
	s := string(out)
	i := strings.Index(s, begin)
	j := strings.Index(s, end)
	if i < 0 || j < 0 || j < i {
		return nil
	}
	pathValue := s[i+len(begin) : j]

	var dirs []string
	for _, d := range strings.Split(pathValue, string(os.PathListSeparator)) {
		if d = strings.TrimSpace(d); d != "" {
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// resolveFFmpeg returns the path to a usable ffmpeg binary, or "" when none is
// found. It checks PATH first, then well-known install locations, then the
// login shell's PATH — so a packaged GUI app (which lacks the shell PATH) can
// still find a Homebrew/MacPorts/conda/custom install.
func resolveFFmpeg() string {
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	name := ffmpegExeName()
	dirs := append(ffmpegSearchDirs(), cachedLoginShellDirs()...)
	for _, dir := range dirs {
		cand := filepath.Join(dir, name)
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand
		}
	}
	return ""
}

// CheckFFmpeg locates the ffmpeg binary and reads its version banner. Install
// guidance is filled in regardless of availability so the UI can show it.
func CheckFFmpeg() FFmpegInfo {
	base := FFmpegInfo{
		OS:          runtime.GOOS,
		InstallCmd:  installCmdFor(runtime.GOOS),
		DownloadURL: "https://ffmpeg.org/download.html",
	}

	path := resolveFFmpeg()
	if path == "" {
		base.Available = false
		return base
	}
	info := base
	info.Available = true
	info.Path = path

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-version").Output()
	if err == nil {
		if line, _, ok := strings.Cut(string(out), "\n"); ok {
			info.Version = strings.TrimSpace(line)
		} else {
			info.Version = strings.TrimSpace(string(out))
		}
	}
	if e := bestHWEncoder(); e != nil {
		info.HWEncoder = e.Name
	}
	return info
}

// isRemote reports whether source is an http(s) URL.
func isRemote(source string) bool {
	s := strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// Probe parses an m3u8 playlist (URL or local file) and summarizes it. Failures
// to reach or parse the playlist are surfaced as AppErrors so the frontend can
// warn the user before starting a conversion.
func Probe(source string) (ProbeResult, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return ProbeResult{}, apperr.New(apperr.InvalidInput, "请填写 m3u8 地址或选择本地文件")
	}

	body, err := fetchPlaylist(source)
	if err != nil {
		return ProbeResult{}, err
	}
	if !strings.Contains(body, "#EXTM3U") {
		return ProbeResult{}, apperr.New(apperr.ParseError, "内容不是有效的 m3u8 播放列表")
	}

	// Master playlists point to variant media playlists; resolve the first one
	// so the reported duration/segment count reflects real media.
	if strings.Contains(body, "#EXT-X-STREAM-INF") {
		variant := firstVariantURI(body)
		if variant != "" {
			if resolved, rerr := resolveRef(source, variant); rerr == nil {
				if vbody, verr := fetchPlaylist(resolved); verr == nil {
					res := parseMediaPlaylist(vbody)
					res.Master = true
					return res, nil
				}
			}
		}
		return ProbeResult{Master: true}, nil
	}

	return parseMediaPlaylist(body), nil
}

// parseMediaPlaylist walks a media playlist, summing segment durations and
// detecting AES-128 encryption and the live/VOD marker.
func parseMediaPlaylist(body string) ProbeResult {
	var res ProbeResult
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sawEndList := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "#EXTINF:"):
			res.Segments++
			res.Duration += parseExtinf(line)
		case strings.HasPrefix(line, "#EXT-X-KEY:"):
			if !strings.Contains(strings.ToUpper(line), "METHOD=NONE") {
				res.Encrypted = true
			}
		case line == "#EXT-X-ENDLIST":
			sawEndList = true
		}
	}
	res.Live = !sawEndList
	return res
}

// parseExtinf extracts the duration from an "#EXTINF:12.5,title" line.
func parseExtinf(line string) float64 {
	rest := strings.TrimPrefix(line, "#EXTINF:")
	if comma := strings.IndexByte(rest, ','); comma >= 0 {
		rest = rest[:comma]
	}
	d, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
	if err != nil {
		return 0
	}
	return d
}

// firstVariantURI returns the URI on the line following the first
// #EXT-X-STREAM-INF tag in a master playlist.
func firstVariantURI(body string) string {
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	pending := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if pending && !strings.HasPrefix(line, "#") {
			return line
		}
		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			pending = true
		}
	}
	return ""
}

// fetchPlaylist reads a playlist from an http(s) URL or a local file path.
func fetchPlaylist(source string) (string, error) {
	if isRemote(source) {
		return httpGet(source)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return "", apperr.Newf(apperr.InvalidInput, "无法读取文件：%v", err)
	}
	return string(data), nil
}
