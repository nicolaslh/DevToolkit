// Package pathx splits, dedups and compares PATH-style variables (R11).
package pathx

import (
	"runtime"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

const MaxLen = 32767

// Separator returns the OS path-list separator (R11.1).
func Separator() string {
	if runtime.GOOS == "windows" {
		return ";"
	}
	return ":"
}

func caseFold(s string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(s)
	}
	return s
}

// Split breaks a PATH string into ordered, non-empty entries (R11.1/11.2).
func Split(raw string) ([]string, error) {
	if len(raw) > MaxLen {
		return nil, apperr.Newf(apperr.TooLarge, "PATH 超出长度上限（最多 %d 个字符）", MaxLen)
	}
	parts := strings.Split(raw, Separator())
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// Dedup removes duplicate entries keeping first occurrence (R11.3).
func Dedup(entries []string) []string {
	seen := make(map[string]struct{}, len(entries))
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		key := caseFold(strings.TrimSpace(e))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	return out
}

// DiffEntry labels an entry with its comparison state.
type DiffEntry struct {
	Path  string `json:"path"`
	State string `json:"state"` // "left" | "right" | "both"
}

// Compare classifies each entry as left-only, right-only, or shared (R11.4).
func Compare(a, b string) ([]DiffEntry, error) {
	ea, err := Split(a)
	if err != nil {
		return nil, err
	}
	eb, err := Split(b)
	if err != nil {
		return nil, err
	}
	setA := toSet(ea)
	setB := toSet(eb)

	var out []DiffEntry
	emitted := make(map[string]struct{})
	for _, e := range ea {
		key := caseFold(strings.TrimSpace(e))
		if _, dup := emitted[key]; dup {
			continue
		}
		emitted[key] = struct{}{}
		if _, inB := setB[key]; inB {
			out = append(out, DiffEntry{Path: e, State: "both"})
		} else {
			out = append(out, DiffEntry{Path: e, State: "left"})
		}
	}
	for _, e := range eb {
		key := caseFold(strings.TrimSpace(e))
		if _, inA := setA[key]; inA {
			continue // already emitted as "both"
		}
		if _, dup := emitted[key]; dup {
			continue
		}
		emitted[key] = struct{}{}
		out = append(out, DiffEntry{Path: e, State: "right"})
	}
	return out, nil
}

func toSet(entries []string) map[string]struct{} {
	m := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		m[caseFold(strings.TrimSpace(e))] = struct{}{}
	}
	return m
}

// Join recombines entries using the OS separator (R11.5).
func Join(entries []string) string {
	return strings.Join(entries, Separator())
}
