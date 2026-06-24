// Package urlparse splits and rebuilds URLs with editable query params (R5).
package urlparse

import (
	"net/url"
	"sort"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

const (
	MaxURLChars = 8192
	MaxParams   = 500
)

// QueryParam is a single editable query key/value pair.
type QueryParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Parts is the decomposed URL.
type Parts struct {
	Protocol string       `json:"protocol"`
	Host     string       `json:"host"`
	Port     string       `json:"port"`
	Path     string       `json:"path"`
	Query    []QueryParam `json:"query"`
}

// Parse decomposes a URL into parts (R5.1–5.3, 5.7).
func Parse(raw string) (Parts, error) {
	if strings.TrimSpace(raw) == "" {
		return Parts{}, apperr.New(apperr.InvalidInput, "URL 为空：请输入一个 URL")
	}
	if len([]rune(raw)) > MaxURLChars {
		return Parts{}, apperr.Newf(apperr.TooLarge, "URL 超出长度上限（最多 %d 个字符）", MaxURLChars)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return Parts{}, apperr.New(apperr.ParseError, "解析失败：不是合法的 URL（需包含协议与主机）")
	}

	var params []QueryParam
	// Preserve order using RawQuery splitting rather than url.Values (which sorts).
	if u.RawQuery != "" {
		for _, pair := range strings.Split(u.RawQuery, "&") {
			if pair == "" {
				continue
			}
			kv := strings.SplitN(pair, "=", 2)
			k, _ := url.QueryUnescape(kv[0])
			v := ""
			if len(kv) == 2 {
				v, _ = url.QueryUnescape(kv[1])
			}
			params = append(params, QueryParam{Key: k, Value: v})
			if len(params) > MaxParams {
				return Parts{}, apperr.Newf(apperr.TooLarge, "查询参数过多（最多 %d 个）", MaxParams)
			}
		}
	}

	return Parts{
		Protocol: u.Scheme,
		Host:     u.Hostname(),
		Port:     u.Port(),
		Path:     u.Path,
		Query:    params,
	}, nil
}

// Build reassembles parts into a URL (R5.4).
func Build(p Parts) (string, error) {
	if p.Protocol == "" || p.Host == "" {
		return "", apperr.New(apperr.InvalidInput, "协议与主机不能为空")
	}
	host := p.Host
	if p.Port != "" {
		host = host + ":" + p.Port
	}
	u := url.URL{
		Scheme: p.Protocol,
		Host:   host,
		Path:   p.Path,
	}
	if len(p.Query) > 0 {
		var parts []string
		for _, q := range p.Query {
			parts = append(parts, url.QueryEscape(q.Key)+"="+url.QueryEscape(q.Value))
		}
		u.RawQuery = strings.Join(parts, "&")
	}
	return u.String(), nil
}

// canonicalQuery returns a sorted, normalized representation for equivalence checks.
func canonicalQuery(params []QueryParam) string {
	pairs := make([]string, 0, len(params))
	for _, q := range params {
		pairs = append(pairs, url.QueryEscape(q.Key)+"="+url.QueryEscape(q.Value))
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

// Equivalent reports whether two Parts are semantically equal (R5.8).
func Equivalent(a, b Parts) bool {
	return a.Protocol == b.Protocol &&
		a.Host == b.Host &&
		a.Port == b.Port &&
		a.Path == b.Path &&
		canonicalQuery(a.Query) == canonicalQuery(b.Query)
}
