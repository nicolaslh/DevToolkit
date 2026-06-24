// Package jwtx decodes (without signature verification) and formats JWTs (R15).
package jwtx

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// Result holds the decoded, formatted JWT parts.
type Result struct {
	Header   string `json:"header"`
	Payload  string `json:"payload"`
	ExpHuman string `json:"expHuman"` // empty when no exp
	Expired  *bool  `json:"expired"`  // nil when no exp
}

func decodeSegment(seg string) ([]byte, error) {
	// JWT uses base64url without padding.
	return base64.RawURLEncoding.DecodeString(seg)
}

func prettyJSON(raw []byte) (string, bool) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return "", false
	}
	return buf.String(), true
}

// Decode parses a JWT into formatted header/payload and computes exp status.
// It does not verify the signature.
func Decode(token string) (Result, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Result{}, apperr.New(apperr.InvalidInput, "JWT 为空：请粘贴一个 JWT")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Result{}, apperr.New(apperr.InvalidInput, "JWT 结构错误：应由点号分隔为三段（Header.Payload.Signature）")
	}

	headerRaw, err := decodeSegment(parts[0])
	if err != nil {
		return Result{}, apperr.New(apperr.ParseError, "Header 段解码失败：不是合法的 Base64Url 编码")
	}
	payloadRaw, err := decodeSegment(parts[1])
	if err != nil {
		return Result{}, apperr.New(apperr.ParseError, "Payload 段解码失败：不是合法的 Base64Url 编码")
	}

	headerStr, ok := prettyJSON(headerRaw)
	if !ok {
		return Result{}, apperr.New(apperr.ParseError, "Header 段解码后不是合法 JSON")
	}
	payloadStr, ok := prettyJSON(payloadRaw)
	if !ok {
		return Result{}, apperr.New(apperr.ParseError, "Payload 段解码后不是合法 JSON")
	}

	res := Result{Header: headerStr, Payload: payloadStr}

	// Extract exp if present and numeric.
	var claims map[string]any
	if err := json.Unmarshal(payloadRaw, &claims); err == nil {
		if expVal, exists := claims["exp"]; exists {
			if expFloat, ok := expVal.(float64); ok {
				expTime := time.Unix(int64(expFloat), 0)
				res.ExpHuman = expTime.Local().Format("2006-01-02 15:04:05")
				expired := !time.Now().Before(expTime) // now >= exp
				res.Expired = &expired
			}
		}
	}
	return res, nil
}
