package pwdgen

import (
	"strings"
	"testing"
)

func TestGenerateRespectsLengthAndSets(t *testing.T) {
	opts := Options{Upper: true, Lower: true, Digits: true, Symbol: true, Length: 16, Count: 50}
	pwds, err := Generate(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(pwds) != 50 {
		t.Fatalf("got %d passwords, want 50", len(pwds))
	}
	for _, p := range pwds {
		if len([]byte(p)) != 16 {
			t.Errorf("password length = %d, want 16", len(p))
		}
		if !strings.ContainsAny(p, upperChars) || !strings.ContainsAny(p, lowerChars) ||
			!strings.ContainsAny(p, digitChars) || !strings.ContainsAny(p, symChars) {
			t.Errorf("password %q missing a required char set", p)
		}
	}
}

func TestGenerateUnique(t *testing.T) {
	pwds, err := Generate(Options{Lower: true, Upper: true, Digits: true, Length: 12, Count: 200})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, p := range pwds {
		if seen[p] {
			t.Errorf("duplicate password %q", p)
		}
		seen[p] = true
	}
}

func TestGenerateErrors(t *testing.T) {
	if _, err := Generate(Options{Length: 10, Count: 1}); err == nil {
		t.Error("expected error when no charset selected")
	}
	if _, err := Generate(Options{Lower: true, Length: 2, Count: 1}); err == nil {
		t.Error("expected error for length below min")
	}
	if _, err := Generate(Options{Lower: true, Length: 10, Count: 0}); err == nil {
		t.Error("expected error for count below 1")
	}
	if _, err := Generate(Options{Lower: true, Length: 10, Count: MaxBatch + 1}); err == nil {
		t.Error("expected error for count above max")
	}
}

func TestBcrypt(t *testing.T) {
	h, err := Bcrypt("hunter2")
	if err != nil || h == "" {
		t.Fatalf("Bcrypt failed: %v", err)
	}
	if _, err := Bcrypt(""); err == nil {
		t.Error("expected error for empty plaintext")
	}
	if _, err := Bcrypt(strings.Repeat("a", 73)); err == nil {
		t.Error("expected error for plaintext over 72 bytes")
	}
}
