package main

import (
	"github.com/nic/devtoolkit/internal/pkg/colorx"
	"github.com/nic/devtoolkit/internal/pkg/qrx"
	"github.com/nic/devtoolkit/internal/pkg/svgopt"
)

// FrontendService exposes color conversion (R12), SVG optimization (R13) and
// QR code generation/decoding (R14). All processing is local.
type FrontendService struct{}

// ConvertColor parses value in the given format ("hex" | "rgb" | "rgba" | "hsl")
// and returns all four representations.
func (s *FrontendService) ConvertColor(value string, format string) (colorx.ColorSet, error) {
	return colorx.Convert(value, colorx.Format(format))
}

// OptimizeSVG minifies SVG markup and reports the size delta.
func (s *FrontendService) OptimizeSVG(input string) (svgopt.Result, error) {
	return svgopt.Optimize(input)
}

// GenerateQR returns a base64 PNG data URI for the given text.
func (s *FrontendService) GenerateQR(text string, opts qrx.Options) (string, error) {
	return qrx.GeneratePNG(text, opts)
}

// DecodeQR decodes a QR code from raw PNG/JPEG image bytes.
func (s *FrontendService) DecodeQR(imageBytes []byte) (string, error) {
	return qrx.Decode(imageBytes)
}
