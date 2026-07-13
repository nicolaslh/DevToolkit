package mediax

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// tempOutputPath returns a unique path in the system temp directory with the
// given extension. ffmpeg writes here first; the app then moves the result to
// the user's chosen destination (see moveFile), which sidesteps macOS TCC
// restrictions on the ffmpeg child process writing to protected folders.
func tempOutputPath(ext string) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", apperr.New(apperr.Unsupported, "无法创建临时文件")
	}
	name := "devtoolkit-" + hex.EncodeToString(b[:]) + "." + ext
	return filepath.Join(os.TempDir(), name), nil
}

// moveFile moves src to dst, creating dst's directory if needed. It first tries
// a rename (fast, same-volume) and falls back to copy+remove across volumes.
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	// Cross-device or permission on rename: copy then remove the source.
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

// augmentedEnv returns the current environment with PATH extended to include
// ffmpeg's own directory, common install locations and the login-shell PATH.
// A Finder-launched macOS app inherits only a minimal PATH from launchd; this
// ensures ffmpeg (and anything it may exec) resolves as it would in a terminal.
func augmentedEnv(ffmpegBin string) []string {
	var extra []string
	if ffmpegBin != "" {
		extra = append(extra, filepath.Dir(ffmpegBin))
	}
	extra = append(extra, ffmpegSearchDirs()...)
	extra = append(extra, cachedLoginShellDirs()...)

	sep := string(os.PathListSeparator)
	seen := map[string]bool{}
	var parts []string
	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		parts = append(parts, dir)
	}
	for _, d := range extra {
		add(d)
	}
	for _, d := range strings.Split(os.Getenv("PATH"), sep) {
		add(d)
	}
	newPath := strings.Join(parts, sep)

	env := os.Environ()
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			out = append(out, "PATH="+newPath)
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, "PATH="+newPath)
	}
	return out
}
