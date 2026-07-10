package mediax

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// maxPlaylistBytes caps how much of a playlist we read while probing, guarding
// against a hostile or mislabeled endpoint streaming an unbounded body.
const maxPlaylistBytes = 8 << 20 // 8 MiB

// httpGet fetches a playlist over HTTP(S) with a short timeout.
func httpGet(rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", apperr.Newf(apperr.InvalidInput, "无效的地址：%v", err)
	}
	req.Header.Set("User-Agent", "DevToolkit/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", apperr.Newf(apperr.Network, "无法访问 m3u8 地址：%v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", apperr.Newf(apperr.Network, "服务器返回状态码 %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPlaylistBytes))
	if err != nil {
		return "", apperr.Newf(apperr.Network, "读取播放列表失败：%v", err)
	}
	return string(data), nil
}

// resolveRef resolves a (possibly relative) playlist reference against the base
// playlist location. Remote bases use URL resolution; local bases join paths.
func resolveRef(base, ref string) (string, error) {
	if isRemote(ref) {
		return ref, nil
	}
	if isRemote(base) {
		b, err := url.Parse(base)
		if err != nil {
			return "", err
		}
		r, err := url.Parse(ref)
		if err != nil {
			return "", err
		}
		return b.ResolveReference(r).String(), nil
	}
	// Local base: resolve relative to its directory.
	if strings.HasPrefix(ref, "/") {
		return ref, nil
	}
	return path.Join(path.Dir(base), ref), nil
}
