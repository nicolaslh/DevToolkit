// Package colorx converts between HEX, RGB, RGBA and HSL color formats (R12).
package colorx

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// ColorSet holds all four representations of a single color.
type ColorSet struct {
	HEX  string `json:"hex"`
	RGB  string `json:"rgb"`
	RGBA string `json:"rgba"`
	HSL  string `json:"hsl"`
}

type rgba struct {
	R, G, B int
	A       float64
}

// Format enumerates the accepted input formats.
type Format string

const (
	HEX  Format = "hex"
	RGB  Format = "rgb"
	RGBA Format = "rgba"
	HSL  Format = "hsl"
)

var (
	hexRe  = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	rgbRe  = regexp.MustCompile(`^rgb\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*\)$`)
	rgbaRe = regexp.MustCompile(`^rgba\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*([0-9]*\.?[0-9]+)\s*\)$`)
	hslRe  = regexp.MustCompile(`^hsl\(\s*(\d{1,3})\s*,\s*(\d{1,3})%\s*,\s*(\d{1,3})%\s*\)$`)
)

// Convert parses value in the given format and returns all representations (R12.1–12.3).
func Convert(value string, format Format) (ColorSet, error) {
	value = strings.TrimSpace(value)
	var c rgba
	var err error
	switch format {
	case HEX:
		c, err = parseHex(value)
	case RGB:
		c, err = parseRGB(value)
	case RGBA:
		c, err = parseRGBA(value)
	case HSL:
		c, err = parseHSL(value)
	default:
		return ColorSet{}, apperr.New(apperr.InvalidInput, "不支持的颜色格式")
	}
	if err != nil {
		return ColorSet{}, err
	}
	return c.toSet(), nil
}

func parseHex(s string) (rgba, error) {
	m := hexRe.FindStringSubmatch(s)
	if m == nil {
		return rgba{}, apperr.New(apperr.InvalidInput, "HEX 颜色格式不合法（应为 3、6 或 8 位十六进制）")
	}
	h := m[1]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	r, _ := strconv.ParseInt(h[0:2], 16, 0)
	g, _ := strconv.ParseInt(h[2:4], 16, 0)
	b, _ := strconv.ParseInt(h[4:6], 16, 0)
	a := 1.0
	if len(h) == 8 {
		av, _ := strconv.ParseInt(h[6:8], 16, 0)
		a = math.Round(float64(av)/255*100) / 100
	}
	return rgba{int(r), int(g), int(b), a}, nil
}

func parseRGB(s string) (rgba, error) {
	m := rgbRe.FindStringSubmatch(s)
	if m == nil {
		return rgba{}, apperr.New(apperr.InvalidInput, "RGB 颜色格式不合法（应为 rgb(0-255,0-255,0-255)）")
	}
	r, g, b := atoi(m[1]), atoi(m[2]), atoi(m[3])
	if !in255(r) || !in255(g) || !in255(b) {
		return rgba{}, apperr.New(apperr.InvalidInput, "RGB 各通道取值须为 0 至 255")
	}
	return rgba{r, g, b, 1}, nil
}

func parseRGBA(s string) (rgba, error) {
	m := rgbaRe.FindStringSubmatch(s)
	if m == nil {
		return rgba{}, apperr.New(apperr.InvalidInput, "RGBA 颜色格式不合法")
	}
	r, g, b := atoi(m[1]), atoi(m[2]), atoi(m[3])
	a, _ := strconv.ParseFloat(m[4], 64)
	if !in255(r) || !in255(g) || !in255(b) {
		return rgba{}, apperr.New(apperr.InvalidInput, "RGBA 各通道取值须为 0 至 255")
	}
	if a < 0 || a > 1 {
		return rgba{}, apperr.New(apperr.InvalidInput, "Alpha 取值须为 0 至 1")
	}
	return rgba{r, g, b, a}, nil
}

func parseHSL(s string) (rgba, error) {
	m := hslRe.FindStringSubmatch(s)
	if m == nil {
		return rgba{}, apperr.New(apperr.InvalidInput, "HSL 颜色格式不合法（应为 hsl(0-360,0-100%,0-100%)）")
	}
	h, sl, l := atoi(m[1]), atoi(m[2]), atoi(m[3])
	if h < 0 || h > 360 || sl < 0 || sl > 100 || l < 0 || l > 100 {
		return rgba{}, apperr.New(apperr.InvalidInput, "HSL 取值范围：色相 0-360，饱和度/亮度 0-100%")
	}
	r, g, b := hslToRGB(float64(h), float64(sl)/100, float64(l)/100)
	return rgba{r, g, b, 1}, nil
}

func (c rgba) toSet() ColorSet {
	h, s, l := rgbToHSL(c.R, c.G, c.B)
	return ColorSet{
		HEX:  fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B),
		RGB:  fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B),
		RGBA: fmt.Sprintf("rgba(%d, %d, %d, %s)", c.R, c.G, c.B, trimFloat(c.A)),
		HSL:  fmt.Sprintf("hsl(%d, %d%%, %d%%)", h, s, l),
	}
}

func hslToRGB(h, s, l float64) (int, int, int) {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return int(math.Round((r + m) * 255)), int(math.Round((g + m) * 255)), int(math.Round((b + m) * 255))
}

func rgbToHSL(ri, gi, bi int) (int, int, int) {
	r, g, b := float64(ri)/255, float64(gi)/255, float64(bi)/255
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	l := (max + min) / 2
	var h, s float64
	if max == min {
		h, s = 0, 0
	} else {
		d := max - min
		if l > 0.5 {
			s = d / (2 - max - min)
		} else {
			s = d / (max + min)
		}
		switch max {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		case b:
			h = (r-g)/d + 4
		}
		h *= 60
	}
	return int(math.Round(h)), int(math.Round(s * 100)), int(math.Round(l * 100))
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }
func in255(n int) bool  { return n >= 0 && n <= 255 }
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
