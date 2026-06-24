// Package svgopt optimizes SVG markup and reports size deltas (R13).
package svgopt

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/svg"
)

// Result holds the optimized output and size comparison.
type Result struct {
	Optimized    string `json:"optimized"`
	BeforeBytes  int    `json:"beforeBytes"`
	AfterBytes   int    `json:"afterBytes"`
	ReductionPct string `json:"reductionPct"` // formatted with 2 decimals
}

var svgTagRe = regexp.MustCompile(`(?is)<svg[\s>]`)

func looksLikeSVG(s string) bool {
	return svgTagRe.MatchString(s)
}

// Optimize minifies SVG and computes the size delta (R13.1, 13.4/13.5, 13.6).
func Optimize(input string) (Result, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Result{}, apperr.New(apperr.InvalidInput, "SVG 内容为空")
	}
	if !looksLikeSVG(trimmed) {
		return Result{}, apperr.New(apperr.ParseError, "不是合法的 SVG：未找到 <svg> 根元素")
	}

	m := minify.New()
	m.AddFunc("image/svg+xml", svg.Minify)
	out, err := m.String("image/svg+xml", trimmed)
	if err != nil {
		return Result{}, apperr.Newf(apperr.ParseError, "SVG 解析失败：%s", err.Error())
	}

	before := len(trimmed)
	after := len(out)
	// If minification did not reduce size, return original and 0.00% (R13.6).
	if after >= before {
		out = trimmed
		after = before
	}
	pct := 0.0
	if before > 0 {
		pct = float64(before-after) / float64(before) * 100
	}
	return Result{
		Optimized:    out,
		BeforeBytes:  before,
		AfterBytes:   after,
		ReductionPct: fmt.Sprintf("%.2f", pct),
	}, nil
}
