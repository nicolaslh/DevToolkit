package qrx

import (
	"bytes"
	"image"
	_ "image/jpeg" // register JPEG decoder
	_ "image/png"  // register PNG decoder
)

// decodeImage decodes PNG or JPEG bytes into an image.Image.
func decodeImage(data []byte) (image.Image, string, error) {
	return image.Decode(bytes.NewReader(data))
}
