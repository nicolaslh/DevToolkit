package crackx

import (
	"bytes"
	"testing"

	"github.com/yeka/zip"
)

// buildZip creates an in-memory ZIP with one entry encrypted using enc (or no
// encryption when enc is 0).
func buildZip(t *testing.T, name string, enc zip.EncryptionMethod, encrypt bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	var (
		w interface {
			Write([]byte) (int, error)
		}
		err error
	)
	if encrypt {
		w, err = zw.Encrypt(name, "secret", enc)
	} else {
		w, err = zw.Create(name)
	}
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if _, err := w.Write([]byte("hello known plaintext content")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

func TestDetectZipEncryption(t *testing.T) {
	cases := []struct {
		name    string
		enc     zip.EncryptionMethod
		encrypt bool
		want    ZipEncryption
		kpa     bool
	}{
		{"zipcrypto", zip.StandardEncryption, true, ZipEncZipCrypto, true},
		{"aes128", zip.AES128Encryption, true, ZipEncAES128, false},
		{"aes192", zip.AES192Encryption, true, ZipEncAES192, false},
		{"aes256", zip.AES256Encryption, true, ZipEncAES256, false},
		{"plain", 0, false, ZipEncNone, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := buildZip(t, "file.txt", tc.enc, tc.encrypt)
			info, err := DetectZipEncryption(data)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if len(info.Entries) != 1 {
				t.Fatalf("entries = %d, want 1", len(info.Entries))
			}
			if got := info.Entries[0].Encryption; got != tc.want {
				t.Fatalf("encryption = %q, want %q", got, tc.want)
			}
			if info.KnownPlaintextViable() != tc.kpa {
				t.Fatalf("KnownPlaintextViable = %v, want %v", info.KnownPlaintextViable(), tc.kpa)
			}
		})
	}
}
