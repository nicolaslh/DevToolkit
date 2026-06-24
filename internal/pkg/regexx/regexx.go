// Package regexx implements regex testing with a built-in library (R3).
// Uses Go's RE2 engine (regexp); note RE2 does not support backreferences.
package regexx

import (
	"regexp"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

const MaxTextChars = 100000

// Flags toggles regex modifiers.
type Flags struct {
	Global     bool `json:"global"`
	IgnoreCase bool `json:"ignoreCase"`
	Multiline  bool `json:"multiline"`
}

// Match describes a single match and its capture groups.
type Match struct {
	Text   string   `json:"text"`
	Start  int      `json:"start"`
	End    int      `json:"end"`
	Groups []string `json:"groups"`
}

// Result holds all matches.
type Result struct {
	Matches []Match `json:"matches"`
	Count   int     `json:"count"`
}

// Preset is a named entry from the common regex library.
type Preset struct {
	Name    string `json:"name"`
	Pattern string `json:"pattern"`
}

// Library returns the built-in common regex presets (R3.6).
func Library() []Preset {
	return []Preset{
		{Name: "邮箱", Pattern: `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`},
		{Name: "身份证号（18 位）", Pattern: `\d{17}[\dXx]`},
		{Name: "手机号（中国大陆）", Pattern: `1[3-9]\d{9}`},
		{Name: "IPv4", Pattern: `\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d?\d)\b`},
		{Name: "IPv6", Pattern: `(?:[A-Fa-f0-9]{1,4}:){7}[A-Fa-f0-9]{1,4}`},
		{Name: "URL", Pattern: `https?://[^\s]+`},
		{Name: "日期 YYYY-MM-DD", Pattern: `\d{4}-\d{2}-\d{2}`},
	}
}

func buildPattern(pattern string, f Flags) string {
	prefix := ""
	if f.IgnoreCase {
		prefix += "i"
	}
	if f.Multiline {
		prefix += "m"
	}
	if prefix != "" {
		return "(?" + prefix + ")" + pattern
	}
	return pattern
}

// Test runs pattern against text and returns highlighted matches (R3.1–3.5, 3.8).
func Test(pattern, text string, f Flags) (Result, error) {
	if pattern == "" {
		return Result{}, apperr.New(apperr.InvalidInput, "正则表达式为空：请输入正则")
	}
	if len([]rune(text)) > MaxTextChars {
		return Result{}, apperr.Newf(apperr.TooLarge, "待测试文本超出上限（最多 %d 个字符）", MaxTextChars)
	}
	re, err := regexp.Compile(buildPattern(pattern, f))
	if err != nil {
		return Result{}, apperr.Newf(apperr.ParseError, "正则语法错误：%s", err.Error())
	}

	limit := -1 // all matches
	if !f.Global {
		limit = 1
	}
	idxs := re.FindAllStringSubmatchIndex(text, limit)
	res := Result{Matches: []Match{}}
	for _, loc := range idxs {
		start, end := loc[0], loc[1]
		var groups []string
		for g := 2; g < len(loc); g += 2 {
			if loc[g] >= 0 {
				groups = append(groups, text[loc[g]:loc[g+1]])
			} else {
				groups = append(groups, "")
			}
		}
		res.Matches = append(res.Matches, Match{
			Text:   text[start:end],
			Start:  start,
			End:    end,
			Groups: groups,
		})
	}
	res.Count = len(res.Matches)
	return res, nil
}
