// Package screenpick implements a live desktop eyedropper: it reads the color
// under the mouse cursor at any position on screen, with no prior screenshot
// required (R12 extension: 取色器).
package screenpick

import (
	"bytes"
	"encoding/base64"
	"image/png"

	"github.com/kbinani/screenshot"
	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// Pick is the color sampled under the cursor plus a small zoomed region used by
// the frontend magnifier loupe.
type Pick struct {
	Hex     string `json:"hex"`
	R       int    `json:"r"`
	G       int    `json:"g"`
	B       int    `json:"b"`
	X       int    `json:"x"`       // cursor position (display points)
	Y       int    `json:"y"`       //
	Region  int    `json:"region"`  // side length (px) of the loupe region
	DataURI string `json:"dataUri"` // PNG of the region for magnification
}

const hexDigits = "0123456789abcdef"

// EnsureAccess reports whether the app can capture other apps' windows. If
// permission is missing it triggers the OS prompt (macOS Screen Recording) and
// returns the current grant state; the user typically must restart afterwards.
func EnsureAccess() bool {
	if hasScreenAccess() {
		return true
	}
	requestScreenAccess()
	return hasScreenAccess()
}

func toHex(r, g, b int) string {
	buf := []byte{'#', 0, 0, 0, 0, 0, 0}
	vals := []int{r, g, b}
	for i, v := range vals {
		buf[1+i*2] = hexDigits[(v>>4)&0xf]
		buf[2+i*2] = hexDigits[v&0xf]
	}
	return string(buf)
}

// AtCursor samples the color under the mouse cursor and returns a zoomed region
// of the given odd side length for the loupe (R12.4, R12.5). On macOS this
// requires the "Screen Recording" privacy permission.
func AtCursor(region int) (Pick, error) {
	if region <= 0 {
		region = 15
	}
	if region > 129 {
		region = 129
	}
	if region%2 == 0 {
		region++ // keep a true center pixel
	}

	if !hasScreenAccess() {
		return Pick{}, apperr.New(apperr.Unsupported, "缺少屏幕录制权限：取色只能看到壁纸，无法读取其他应用窗口")
	}

	x, y, err := cursorPos()
	if err != nil {
		return Pick{}, err
	}

	half := region / 2
	img, err := screenshot.Capture(x-half, y-half, region, region)
	if err != nil {
		return Pick{}, apperr.New(apperr.Unsupported, "屏幕取色失败：请确认已授予屏幕录制权限")
	}

	b := img.Bounds()
	c := img.RGBAAt(b.Min.X+half, b.Min.Y+half)
	r, g, bl := int(c.R), int(c.G), int(c.B)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return Pick{}, apperr.New(apperr.ParseError, "编码取色区域失败")
	}

	return Pick{
		Hex:     toHex(r, g, bl),
		R:       r,
		G:       g,
		B:       bl,
		X:       x,
		Y:       y,
		Region:  region,
		DataURI: "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()),
	}, nil
}
