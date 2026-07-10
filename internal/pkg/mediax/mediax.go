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
	"runtime"
	"strconv"
	"strings"
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

// CheckFFmpeg locates the ffmpeg binary and reads its version banner. Install
// guidance is filled in regardless of availability so the UI can show it.
func CheckFFmpeg() FFmpegInfo {
	base := FFmpegInfo{
		OS:          runtime.GOOS,
		InstallCmd:  installCmdFor(runtime.GOOS),
		DownloadURL: "https://ffmpeg.org/download.html",
	}

	path, err := exec.LookPath("ffmpeg")
	if err != nil {
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
