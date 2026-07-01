package crackx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yeka/zip"
)

// makeZipCryptoArchive builds a single-entry, STORE-method ZipCrypto archive
// and returns the archive bytes plus the original plaintext content.
func makeZipCryptoArchive(t *testing.T, name, password string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: name, Method: zip.Store}
	fh.SetPassword(password)
	fh.SetEncryptionMethod(zip.StandardEncryption)
	w, err := zw.CreateHeader(fh)
	if err != nil {
		t.Fatalf("create header: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

// expectedKeystream runs the real cipher (knowing the password) to produce the
// keystream byte at every absolute position of the encrypted stream.
func expectedKeystream(t *testing.T, raw []byte, password string) []byte {
	t.Helper()
	var keys zipCryptoKeys
	keys.initWithPassword(password)
	ks := make([]byte, len(raw))
	for i := range raw {
		ks[i] = keys.streamByte()
		keys.update(raw[i] ^ ks[i]) // advance with the recovered plaintext byte
	}
	return ks
}

func TestRawEncryptedData(t *testing.T) {
	content := []byte("known plaintext attack target — STORE method, no compression here")
	archive := makeZipCryptoArchive(t, "secret.txt", "pw-correct", content)

	raw, enc, err := rawEncryptedData(archive, "secret.txt")
	if err != nil {
		t.Fatalf("rawEncryptedData: %v", err)
	}
	if enc != ZipEncZipCrypto {
		t.Fatalf("enc = %q, want %q", enc, ZipEncZipCrypto)
	}
	if len(raw) != encryptionHeaderSize+len(content) {
		t.Fatalf("raw len = %d, want %d", len(raw), encryptionHeaderSize+len(content))
	}

	if _, _, err := rawEncryptedData(archive, "missing.txt"); err == nil {
		t.Fatal("expected error for missing entry, got nil")
	}
}

func TestNewAttackDataKeystream(t *testing.T) {
	content := []byte("the more known plaintext we have, the faster the attack converges!!")
	const pw = "secret-pass"
	archive := makeZipCryptoArchive(t, "f.bin", pw, content)
	raw, _, err := rawEncryptedData(archive, "f.bin")
	if err != nil {
		t.Fatalf("rawEncryptedData: %v", err)
	}
	wantKS := expectedKeystream(t, raw, pw)

	cases := []struct{ dataOffset, plen int }{
		{0, len(content)}, // whole data
		{0, minKnownPlaintext},
		{5, 20}, // interior window
		{len(content) - minKnownPlaintext, minKnownPlaintext}, // tail
	}
	for _, tc := range cases {
		plain := content[tc.dataOffset : tc.dataOffset+tc.plen]
		d, err := newAttackData(raw, plain, tc.dataOffset)
		if err != nil {
			t.Fatalf("newAttackData(off=%d,len=%d): %v", tc.dataOffset, tc.plen, err)
		}
		got := d.keystream()
		want := wantKS[encryptionHeaderSize+tc.dataOffset : encryptionHeaderSize+tc.dataOffset+tc.plen]
		if !bytes.Equal(got, want) {
			t.Fatalf("keystream mismatch at off=%d len=%d:\n got=%x\nwant=%x",
				tc.dataOffset, tc.plen, got, want)
		}
	}
}

func TestNewAttackDataValidation(t *testing.T) {
	cipher := make([]byte, 64)
	cases := []struct {
		name      string
		plaintext []byte
		offset    int
		wantErr   string
	}{
		{"too short", make([]byte, minKnownPlaintext-1), 0, "至少需要"},
		{"negative offset", make([]byte, minKnownPlaintext), -1, "不能为负"},
		{"out of range", make([]byte, 40), 20, "超出密文范围"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newAttackData(cipher, tc.plaintext, tc.offset)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tc.wantErr)
			}
		})
	}
}
