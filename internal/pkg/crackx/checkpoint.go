package crackx

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Checkpoint records how far a job progressed so an interrupted run can resume.
// Checkpoints are stored on disk keyed by a fingerprint of the file + options,
// so reopening the same file with the same settings finds the saved progress.
type Checkpoint struct {
	Fingerprint string    `json:"fingerprint"`
	Tried       int64     `json:"tried"`
	Total       int64     `json:"total"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ckptMu serializes checkpoint file writes across jobs.
var ckptMu sync.Mutex

// Fingerprint derives a stable key from the file contents and the parameters
// that determine candidate ordering. Changing the file or any of these options
// produces a different key, so progress is never reused across mismatched runs.
func Fingerprint(data []byte, opts Options) string {
	h := sha256.New()
	h.Write(data)
	fmt.Fprintf(h, "\x00mode=%s\x00min=%d\x00max=%d\x00alpha=%s\x00wl=",
		opts.Mode, opts.MinLen, opts.MaxLen, opts.Charset.alphabet())
	if opts.Mode == ModeDict {
		h.Write([]byte(opts.Wordlist))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func checkpointDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "DevToolkit", "crack-checkpoints")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func checkpointPath(fingerprint string) (string, error) {
	dir, err := checkpointDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fingerprint+".json"), nil
}

// SaveCheckpoint writes (or overwrites) the checkpoint for a fingerprint.
func SaveCheckpoint(cp Checkpoint) error {
	cp.UpdatedAt = time.Now()
	path, err := checkpointPath(cp.Fingerprint)
	if err != nil {
		return err
	}
	blob, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	ckptMu.Lock()
	defer ckptMu.Unlock()
	// Write atomically so a crash mid-write can't corrupt the checkpoint.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadCheckpoint returns the saved checkpoint for a fingerprint, if any.
func LoadCheckpoint(fingerprint string) (Checkpoint, bool) {
	path, err := checkpointPath(fingerprint)
	if err != nil {
		return Checkpoint{}, false
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		return Checkpoint{}, false
	}
	var cp Checkpoint
	if err := json.Unmarshal(blob, &cp); err != nil {
		return Checkpoint{}, false
	}
	return cp, true
}

// DeleteCheckpoint removes the checkpoint for a fingerprint (no error if absent).
func DeleteCheckpoint(fingerprint string) {
	if path, err := checkpointPath(fingerprint); err == nil {
		_ = os.Remove(path)
	}
}
