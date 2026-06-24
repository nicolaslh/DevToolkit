// Package qrx generates and decodes QR codes (R14).
package qrx

import (
	"bytes"
	"encoding/base64"
	"image/color"
	"image/png"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/nic/devtoolkit/internal/pkg/apperr"
	qrcodegen "github.com/skip2/go-qrcode"
)

// MaxTextLen is the practical upper bound for QR text content (R14.1).
const MaxTextLen = 2953

// Options configures generation.
type Options struct {
	Foreground string `json:"foreground"` // hex like #000000
	Background string `json:"background"` // hex like #ffffff
	Size       int    `json:"size"`       // pixel dimensions
}

func parseHexColor(s string, def color.Color) color.Color {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return def
	}
	var r, g, b uint8
	_, err := hexByte(s[0:2], &r)
	if err != nil {
		return def
	}
	if _, err := hexByte(s[2:4], &g); err != nil {
		return def
	}
	if _, err := hexByte(s[4:6], &b); err != nil {
		return def
	}
	return color.RGBA{R: r, G: g, B: b, A: 255}
}

func hexByte(s string, out *uint8) (int, error) {
	var v int
	for _, c := range s {
		var d int
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'f':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = int(c-'A') + 10
		default:
			return 0, apperr.New(apperr.InvalidInput, "颜色值不合法")
		}
		v = v*16 + d
	}
	*out = uint8(v)
	return 1, nil
}

// GeneratePNG returns a base64-encoded PNG data URI for the given text (R14.1–14.3).
func GeneratePNG(text string, opts Options) (string, error) {
	if text == "" {
		return "", apperr.New(apperr.InvalidInput, "内容为空：请输入文本或链接")
	}
	if len([]rune(text)) > MaxTextLen {
		return "", apperr.Newf(apperr.TooLarge, "内容超出长度上限（最多 %d 个字符）", MaxTextLen)
	}
	size := opts.Size
	if size <= 0 {
		size = 256
	}
	qr, err := qrcodegen.New(text, qrcodegen.Medium)
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "生成二维码失败")
	}
	qr.ForegroundColor = parseHexColor(opts.Foreground, color.Black)
	qr.BackgroundColor = parseHexColor(opts.Background, color.White)

	img := qr.Image(size)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", apperr.New(apperr.InvalidInput, "编码 PNG 失败")
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// Decode reads a QR code from raw PNG/JPEG image bytes (R14.6, 14.7).
func Decode(imageBytes []byte) (string, error) {
	if len(imageBytes) == 0 {
		return "", apperr.New(apperr.InvalidInput, "图像为空")
	}
	img, _, err := decodeImage(imageBytes)
	if err != nil {
		return "", apperr.New(apperr.ParseError, "无法解析图像（仅支持 PNG / JPEG）")
	}
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", apperr.New(apperr.ParseError, "无法处理图像")
	}
	reader := qrcode.NewQRCodeReader()
	result, err := reader.Decode(bmp, nil)
	if err != nil {
		return "", apperr.New(apperr.ParseError, "未识别到二维码")
	}
	return result.GetText(), nil
}
