// Package cronx parses/generates 5-field cron expressions and predicts runs (R9).
package cronx

import (
	"fmt"
	"strings"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/robfig/cron/v3"
)

// Fields holds the five standard cron fields. Empty string defaults to "*".
type Fields struct {
	Minute  string `json:"minute"`
	Hour    string `json:"hour"`
	Day     string `json:"day"`
	Month   string `json:"month"`
	Weekday string `json:"weekday"`
}

// Info bundles parse results.
type Info struct {
	Expression  string   `json:"expression"`
	Description string   `json:"description"`
	NextRuns    []string `json:"nextRuns"`
}

var standardParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
)

func orStar(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "*"
	}
	return s
}

// Build assembles a 5-field expression from visual selections (R9.1/9.2).
func Build(f Fields) string {
	return strings.Join([]string{
		orStar(f.Minute),
		orStar(f.Hour),
		orStar(f.Day),
		orStar(f.Month),
		orStar(f.Weekday),
	}, " ")
}

// Parse validates an expression and computes description + next 5 runs (R9.3–9.5).
func Parse(expr string) (Info, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return Info{}, apperr.New(apperr.InvalidInput, "Cron 表达式为空")
	}
	if len(strings.Fields(expr)) != 5 {
		return Info{}, apperr.New(apperr.InvalidInput, "Cron 字段数量错误：标准 Cron 应为 5 个字段（分 时 日 月 周）")
	}
	sched, err := standardParser.Parse(expr)
	if err != nil {
		return Info{}, apperr.Newf(apperr.ParseError, "Cron 语法错误：%s", err.Error())
	}

	next := NextRuns(sched, time.Now(), 5)
	runs := make([]string, len(next))
	for i, t := range next {
		runs[i] = t.Format("2006-01-02 15:04:05")
	}
	return Info{
		Expression:  expr,
		Description: describe(expr),
		NextRuns:    runs,
	}, nil
}

// NextRuns returns the next n activation times after from.
func NextRuns(sched cron.Schedule, from time.Time, n int) []time.Time {
	out := make([]time.Time, 0, n)
	t := from
	for i := 0; i < n; i++ {
		t = sched.Next(t)
		out = append(out, t)
	}
	return out
}

// describe produces a concise natural-language summary of the schedule.
func describe(expr string) string {
	parts := strings.Fields(expr)
	min, hour, dom, mon, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	var b strings.Builder
	if min == "*" && hour == "*" {
		b.WriteString("每分钟")
	} else if hour == "*" {
		b.WriteString(fmt.Sprintf("每小时的第 %s 分钟", min))
	} else if min != "*" && hour != "*" {
		b.WriteString(fmt.Sprintf("每天 %s:%s", pad(hour), pad(min)))
	} else {
		b.WriteString(fmt.Sprintf("分=%s 时=%s", min, hour))
	}
	if dom != "*" {
		b.WriteString(fmt.Sprintf("，每月第 %s 日", dom))
	}
	if mon != "*" {
		b.WriteString(fmt.Sprintf("，%s 月", mon))
	}
	if dow != "*" {
		b.WriteString(fmt.Sprintf("，星期 %s", dow))
	}
	b.WriteString("执行")
	return b.String()
}

func pad(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}
