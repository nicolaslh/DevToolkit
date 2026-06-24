package colorx

import (
	"strings"
	"testing"
)

func TestConvertHex(t *testing.T) {
	set, err := Convert("#ff0000", HEX)
	if err != nil {
		t.Fatal(err)
	}
	if set.RGB != "rgb(255, 0, 0)" {
		t.Errorf("got %q", set.RGB)
	}
	if set.HSL != "hsl(0, 100%, 50%)" {
		t.Errorf("got %q", set.HSL)
	}
}

func TestConvertShortHex(t *testing.T) {
	set, err := Convert("#0f0", HEX)
	if err != nil {
		t.Fatal(err)
	}
	if set.RGB != "rgb(0, 255, 0)" {
		t.Errorf("got %q", set.RGB)
	}
}

func TestConvertRGBAndHSL(t *testing.T) {
	if _, err := Convert("rgb(10, 20, 30)", RGB); err != nil {
		t.Error(err)
	}
	if _, err := Convert("hsl(120, 100%, 50%)", HSL); err != nil {
		t.Error(err)
	}
	if _, err := Convert("rgba(1,2,3,0.5)", RGBA); err != nil {
		t.Error(err)
	}
}

func TestConvertInvalid(t *testing.T) {
	cases := []struct {
		v string
		f Format
	}{
		{"#xyz", HEX},
		{"rgb(300,0,0)", RGB},
		{"hsl(400,0%,0%)", HSL},
		{"rgba(0,0,0,2)", RGBA},
		{"garbage", HEX},
	}
	for _, c := range cases {
		if _, err := Convert(c.v, c.f); err == nil {
			t.Errorf("expected error for %q (%s)", c.v, c.f)
		}
	}
}

// Property 6: HexToRGB then RGBToHex round-trips (ignoring case) (R12.4).
func TestHexRoundTrip(t *testing.T) {
	hexes := []string{"#000000", "#FFFFFF", "#123456", "#abcdef", "#FF8800"}
	for _, h := range hexes {
		set, err := Convert(h, HEX)
		if err != nil {
			t.Fatal(err)
		}
		// set.HEX is uppercase; compare case-insensitively.
		if !strings.EqualFold(set.HEX, h) {
			t.Errorf("round-trip mismatch: %q -> %q", h, set.HEX)
		}
		// Now go via RGB back to a set and ensure HEX stable.
		set2, err := Convert(set.RGB, RGB)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(set2.HEX, h) {
			t.Errorf("rgb round-trip mismatch: %q -> %q", h, set2.HEX)
		}
	}
}
