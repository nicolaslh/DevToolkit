// Package differ computes side-by-side text/JSON diffs (R1).
package differ

import (
	"encoding/json"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/sergi/go-diff/diffmatchpatch"
)

const MaxChars = 1000000

// Segment is a contiguous run tagged by its diff type.
type Segment struct {
	Type string `json:"type"` // "equal" | "insert" | "delete"
	Text string `json:"text"`
}

// Result holds side-by-side segments. Left carries equal+delete; Right carries equal+insert.
type Result struct {
	Left      []Segment `json:"left"`
	Right     []Segment `json:"right"`
	Identical bool      `json:"identical"`
}

// Options configures the diff.
type Options struct {
	Mode     string `json:"mode"`     // "line" (default) | "char"
	JSONMode bool   `json:"jsonMode"` // normalize both sides as JSON first
}

func normalizeJSON(s string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return "", err
	}
	// MarshalIndent sorts object keys deterministically.
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Diff compares left and right (R1.1–1.9).
func Diff(left, right string, opts Options) (Result, error) {
	if left == "" || right == "" {
		return Result{}, apperr.New(apperr.InvalidInput, "请提供左右两侧内容后再比对")
	}
	if len([]rune(left)) > MaxChars || len([]rune(right)) > MaxChars {
		return Result{}, apperr.Newf(apperr.TooLarge, "输入超出长度上限（每侧最多 %d 个字符）", MaxChars)
	}

	if opts.JSONMode {
		nl, err := normalizeJSON(left)
		if err != nil {
			return Result{}, apperr.New(apperr.ParseError, "左侧不是合法 JSON，无法进行结构比对")
		}
		nr, err := normalizeJSON(right)
		if err != nil {
			return Result{}, apperr.New(apperr.ParseError, "右侧不是合法 JSON，无法进行结构比对")
		}
		left, right = nl, nr
	}

	if left == right {
		return Result{Left: []Segment{{Type: "equal", Text: left}}, Right: []Segment{{Type: "equal", Text: right}}, Identical: true}, nil
	}

	dmp := diffmatchpatch.New()
	var diffs []diffmatchpatch.Diff
	if opts.Mode == "char" {
		diffs = dmp.DiffMain(left, right, false)
	} else {
		a, b, lineArray := dmp.DiffLinesToChars(left, right)
		diffs = dmp.DiffMain(a, b, false)
		diffs = dmp.DiffCharsToLines(diffs, lineArray)
	}

	res := Result{Left: []Segment{}, Right: []Segment{}}
	for _, d := range diffs {
		switch d.Type {
		case diffmatchpatch.DiffEqual:
			res.Left = append(res.Left, Segment{Type: "equal", Text: d.Text})
			res.Right = append(res.Right, Segment{Type: "equal", Text: d.Text})
		case diffmatchpatch.DiffDelete:
			res.Left = append(res.Left, Segment{Type: "delete", Text: d.Text})
		case diffmatchpatch.DiffInsert:
			res.Right = append(res.Right, Segment{Type: "insert", Text: d.Text})
		}
	}
	return res, nil
}
