// Package codecx implements basic encoding/decoding: Base64, URL, Hex (R17).
package codecx

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// MaxInputChars is the maximum accepted input length (R17.1/17.6).
const MaxInputChars = 1048576

// Kind enumerates supported codec types.
type Kind string

const (
	Base64  Kind = "base64"
	URL     Kind = "url"
	Hex     Kind = "hex"
	Unicode Kind = "unicode"
)

func validKind(k Kind) bool {
	switch k {
	case Base64, URL, Hex, Unicode:
		return true
	default:
		return false
	}
}

func checkSize(s string) error {
	if len([]rune(s)) > MaxInputChars {
		return apperr.Newf(apperr.TooLarge, "输入超出长度上限（最多 %d 个字符）", MaxInputChars)
	}
	return nil
}

// Encode encodes plaintext according to kind. Empty input yields empty output (R17.4).
func Encode(input string, kind Kind) (string, error) {
	if !validKind(kind) {
		return "", apperr.New(apperr.InvalidInput, "不支持的编码类型")
	}
	if err := checkSize(input); err != nil {
		return "", err
	}
	switch kind {
	case Base64:
		return base64.StdEncoding.EncodeToString([]byte(input)), nil
	case URL:
		return url.QueryEscape(input), nil
	case Hex:
		return hex.EncodeToString([]byte(input)), nil
	case Unicode:
		return unicodeEscape(input), nil
	}
	return "", nil
}

// Decode decodes encoded content according to kind. Empty input yields empty output (R17.4).
func Decode(input string, kind Kind) (string, error) {
	if !validKind(kind) {
		return "", apperr.New(apperr.InvalidInput, "不支持的编码类型")
	}
	if err := checkSize(input); err != nil {
		return "", err
	}
	if input == "" {
		return "", nil
	}
	switch kind {
	case Base64:
		// Tolerate both standard and raw (no padding) base64.
		b, err := base64.StdEncoding.DecodeString(input)
		if err != nil {
			if b2, err2 := base64.RawStdEncoding.DecodeString(strings.TrimRight(input, "=")); err2 == nil {
				return string(b2), nil
			}
			return "", apperr.New(apperr.ParseError, "无法按 Base64 解码：内容不是合法的 Base64")
		}
		return string(b), nil
	case URL:
		s, err := url.QueryUnescape(input)
		if err != nil {
			return "", apperr.New(apperr.ParseError, "无法按 URL 编码解码：内容包含非法转义序列")
		}
		return s, nil
	case Hex:
		b, err := hex.DecodeString(input)
		if err != nil {
			return "", apperr.New(apperr.ParseError, "无法按十六进制解码：内容不是合法的 Hex 字符串")
		}
		return string(b), nil
	case Unicode:
		return unicodeUnescape(input), nil
	}
	return "", nil
}

// unicodeEscape converts text into \uXXXX escape sequences. Characters outside
// the Basic Multilingual Plane are emitted as UTF-16 surrogate pairs so the
// result is compatible with JSON/JavaScript style escapes.
func unicodeEscape(input string) string {
	var b strings.Builder
	for _, r := range input {
		if r > 0xFFFF {
			r1, r2 := utf16.EncodeRune(r)
			fmt.Fprintf(&b, "\\u%04x\\u%04x", r1, r2)
		} else {
			fmt.Fprintf(&b, "\\u%04x", r)
		}
	}
	return b.String()
}

// unicodeUnescape converts \uXXXX escape sequences back into text. Any content
// that is not a valid escape sequence is preserved as-is so the function is
// tolerant of mixed input.
func unicodeUnescape(input string) string {
	var b strings.Builder
	var units []uint16
	runes := []rune(input)

	flush := func() {
		if len(units) > 0 {
			b.WriteString(string(utf16.Decode(units)))
			units = units[:0]
		}
	}

	for i := 0; i < len(runes); {
		if runes[i] == '\\' && i+5 < len(runes) && (runes[i+1] == 'u' || runes[i+1] == 'U') {
			if v, err := strconv.ParseUint(string(runes[i+2:i+6]), 16, 32); err == nil {
				units = append(units, uint16(v))
				i += 6
				continue
			}
		}
		flush()
		b.WriteRune(runes[i])
		i++
	}
	flush()
	return b.String()
}
