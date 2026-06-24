package qrx

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestGenerateAndDecodeRoundTrip(t *testing.T) {
	// Property 5: Decode(Generate(text)) == text (R14.8).
	texts := []string{"hello", "https://example.com/path?a=1", "中文测试", "1234567890"}
	for _, txt := range texts {
		dataURI, err := GeneratePNG(txt, Options{Size: 256})
		if err != nil {
			t.Fatalf("generate %q: %v", txt, err)
		}
		b64 := strings.TrimPrefix(dataURI, "data:image/png;base64,")
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Decode(raw)
		if err != nil {
			t.Fatalf("decode %q: %v", txt, err)
		}
		if got != txt {
			t.Errorf("round-trip mismatch: %q -> %q", txt, got)
		}
	}
}

func TestGenerateEmpty(t *testing.T) {
	if _, err := GeneratePNG("", Options{}); err == nil {
		t.Error("expected error for empty input")
	}
}

func TestGenerateWithColors(t *testing.T) {
	uri, err := GeneratePNG("colored", Options{Foreground: "#2f6feb", Background: "#ffffff", Size: 200})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Error("expected data URI")
	}
}

func TestDecodeNoQR(t *testing.T) {
	if _, err := Decode([]byte("not an image")); err == nil {
		t.Error("expected error for non-image bytes")
	}
}
