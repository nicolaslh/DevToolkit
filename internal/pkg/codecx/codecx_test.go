package codecx

import (
	"strings"
	"testing"
	"testing/quick"
)

func TestEncodeDecodeEmpty(t *testing.T) {
	for _, k := range []Kind{Base64, URL, Hex} {
		enc, err := Encode("", k)
		if err != nil || enc != "" {
			t.Fatalf("Encode empty %s = %q, %v; want empty", k, enc, err)
		}
		dec, err := Decode("", k)
		if err != nil || dec != "" {
			t.Fatalf("Decode empty %s = %q, %v; want empty", k, dec, err)
		}
	}
}

func TestDecodeInvalid(t *testing.T) {
	if _, err := Decode("not base64!!!", Base64); err == nil {
		t.Error("expected error for invalid base64")
	}
	if _, err := Decode("zz", Hex); err == nil {
		t.Error("expected error for invalid hex")
	}
}

func TestTooLarge(t *testing.T) {
	big := strings.Repeat("a", MaxInputChars+1)
	if _, err := Encode(big, Base64); err == nil {
		t.Error("expected TooLarge error")
	}
}

// Property 1: Decode(Encode(x)) == x for all inputs (R17.7).
func TestRoundTripProperty(t *testing.T) {
	for _, k := range []Kind{Base64, URL, Hex} {
		kind := k
		f := func(s string) bool {
			enc, err := Encode(s, kind)
			if err != nil {
				return false
			}
			dec, err := Decode(enc, kind)
			if err != nil {
				return false
			}
			return dec == s
		}
		if err := quick.Check(f, &quick.Config{MaxCount: 2000}); err != nil {
			t.Errorf("round-trip failed for %s: %v", kind, err)
		}
	}
}
