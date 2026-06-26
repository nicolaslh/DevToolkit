// Package dnsx resolves DNS records (A/AAAA/CNAME/MX/NS/TXT) for a host (R8).
package dnsx

import (
	"context"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

const (
	// MaxHostChars caps the input length to avoid abuse.
	MaxHostChars = 253
	// lookupTimeout bounds each resolution so the UI never hangs.
	lookupTimeout = 5 * time.Second
)

// Result groups the records resolved for a single host.
type Result struct {
	Host  string   `json:"host"`
	A     []string `json:"a"`
	AAAA  []string `json:"aaaa"`
	CNAME string   `json:"cname"`
	MX    []string `json:"mx"`
	NS    []string `json:"ns"`
	TXT   []string `json:"txt"`
}

// normalizeHost trims input and strips a scheme/path if a URL was pasted.
func normalizeHost(raw string) string {
	h := strings.TrimSpace(raw)
	if h == "" {
		return ""
	}
	// Strip scheme like "https://".
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	// Strip credentials.
	if i := strings.Index(h, "@"); i >= 0 {
		h = h[i+1:]
	}
	// Strip path/query.
	if i := strings.IndexAny(h, "/?#"); i >= 0 {
		h = h[:i]
	}
	// Strip a trailing port (host:port), but keep IPv6 literals intact.
	if !strings.Contains(h, "::") {
		if i := strings.LastIndex(h, ":"); i >= 0 {
			h = h[:i]
		}
	}
	return strings.TrimSuffix(strings.ToLower(h), ".")
}

// Resolve looks up the common record types for host and returns a grouped result.
// Individual record-type failures are tolerated; an error is only returned when
// the input is invalid or nothing at all could be resolved.
func Resolve(raw string) (Result, error) {
	host := normalizeHost(raw)
	if host == "" {
		return Result{}, apperr.New(apperr.InvalidInput, "请输入要解析的域名或主机")
	}
	if len(host) > MaxHostChars {
		return Result{}, apperr.Newf(apperr.TooLarge, "域名超出长度上限（最多 %d 个字符）", MaxHostChars)
	}

	ctx, cancel := context.WithTimeout(context.Background(), lookupTimeout)
	defer cancel()

	r := &net.Resolver{}
	res := Result{Host: host}
	var resolved bool

	if ips, err := r.LookupIP(ctx, "ip4", host); err == nil {
		for _, ip := range ips {
			res.A = append(res.A, ip.String())
		}
	}
	if ips, err := r.LookupIP(ctx, "ip6", host); err == nil {
		for _, ip := range ips {
			res.AAAA = append(res.AAAA, ip.String())
		}
	}
	if cname, err := r.LookupCNAME(ctx, host); err == nil {
		cname = strings.TrimSuffix(cname, ".")
		if !strings.EqualFold(cname, host) {
			res.CNAME = cname
		}
	}
	if mxs, err := r.LookupMX(ctx, host); err == nil {
		for _, mx := range mxs {
			res.MX = append(res.MX, strings.TrimSuffix(mx.Host, "."))
		}
	}
	if nss, err := r.LookupNS(ctx, host); err == nil {
		for _, ns := range nss {
			res.NS = append(res.NS, strings.TrimSuffix(ns.Host, "."))
		}
	}
	if txts, err := r.LookupTXT(ctx, host); err == nil {
		res.TXT = append(res.TXT, txts...)
	}

	sort.Strings(res.A)
	sort.Strings(res.AAAA)
	sort.Strings(res.MX)
	sort.Strings(res.NS)
	sort.Strings(res.TXT)

	resolved = len(res.A) > 0 || len(res.AAAA) > 0 || res.CNAME != "" ||
		len(res.MX) > 0 || len(res.NS) > 0 || len(res.TXT) > 0

	if !resolved {
		if ctx.Err() == context.DeadlineExceeded {
			return Result{}, apperr.New(apperr.Timeout, "解析超时：请检查网络连接后重试")
		}
		return Result{}, apperr.Newf(apperr.Network, "解析失败：未找到 %q 的任何 DNS 记录", host)
	}

	return res, nil
}
