package cryptox

import (
	"testing"
	"testing/quick"
)

func TestHashKnownVectors(t *testing.T) {
	cases := []struct {
		algo HashAlgo
		in   string
		want string
	}{
		{MD5, "abc", "900150983cd24fb0d6963f7d28e17f72"},
		{SHA1, "abc", "a9993e364706816aba3e25717850c26c9cd0d89d"},
		{SHA256, "abc", "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"},
	}
	for _, c := range cases {
		got, err := Hash(c.in, c.algo)
		if err != nil || got != c.want {
			t.Errorf("Hash(%q,%s) = %q,%v; want %q", c.in, c.algo, got, err, c.want)
		}
	}
}

func TestEncryptInvalidKeyLen(t *testing.T) {
	if _, err := Encrypt("hi", "shortkey"); err == nil {
		t.Error("expected invalid key length error")
	}
}

// Property 9: wrong key must fail decryption (R18.5).
func TestWrongKeyFails(t *testing.T) {
	ct, err := Encrypt("secret message", "0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decrypt(ct, "fedcba9876543210"); err == nil {
		t.Error("expected decryption failure with wrong key")
	}
}

// Property 2: Decrypt(Encrypt(x, k), k) == x (R18.6).
func TestEncryptDecryptRoundTrip(t *testing.T) {
	keys := []string{"0123456789abcdef", "0123456789abcdef01234567", "0123456789abcdef0123456789abcdef"}
	for _, key := range keys {
		k := key
		f := func(plain string) bool {
			ct, err := Encrypt(plain, k)
			if err != nil {
				return false
			}
			pt, err := Decrypt(ct, k)
			if err != nil {
				return false
			}
			return pt == plain
		}
		if err := quick.Check(f, &quick.Config{MaxCount: 1000}); err != nil {
			t.Errorf("round-trip failed for key len %d: %v", len(k), err)
		}
	}
}
