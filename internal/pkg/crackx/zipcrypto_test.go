package crackx

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/yeka/zip"
)

// TestCRC32TabMatchesStdlib confirms our reflected CRC-32 table equals the
// IEEE table the standard library uses, guarding against a wrong polynomial.
func TestCRC32TabMatchesStdlib(t *testing.T) {
	std := crc32.MakeTable(crc32.IEEE)
	for i := range crc32Tab {
		if crc32Tab[i] != std[i] {
			t.Fatalf("crc32Tab[%d] = %#08x, want %#08x", i, crc32Tab[i], std[i])
		}
	}
}

// TestEncryptDecryptRoundTrip checks the cipher is internally consistent: a
// freshly keyed encryptor and decryptor (same password) are inverses.
func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("The quick brown fox jumps over the lazy dog. 0123456789")

	var enc zipCryptoKeys
	enc.initWithPassword("hunter2")
	cipher := make([]byte, len(plaintext))
	for i, p := range plaintext {
		cipher[i] = enc.encryptByte(p)
	}

	var dec zipCryptoKeys
	dec.initWithPassword("hunter2")
	got := make([]byte, len(cipher))
	for i, c := range cipher {
		got[i] = dec.decryptByte(c)
	}

	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round trip mismatch:\n got = %q\nwant = %q", got, plaintext)
	}
}

// TestDecryptRealZipCrypto is the golden test: it asks yeka/zip to produce a
// real ZipCrypto-encrypted entry, then decrypts the raw bytes with our cipher
// using only the password. Success proves update/streamByte/decryptByte match
// a real-world ZipCrypto stream, not just each other.
func TestDecryptRealZipCrypto(t *testing.T) {
	const (
		password = "correct horse"
		name     = "secret.txt"
	)
	plaintext := []byte("known plaintext attack target — repeated content content content")

	// Build an archive with a single STORE (uncompressed) ZipCrypto entry, so
	// the encrypted data is exactly the 12-byte header + encrypted plaintext.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: name, Method: zip.Store}
	fh.SetPassword(password)
	fh.SetEncryptionMethod(zip.StandardEncryption)
	w, err := zw.CreateHeader(fh)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	archive := buf.Bytes()

	// Locate the raw encrypted bytes for the entry.
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}
	f := r.File[0]
	if f.Method != zip.Store {
		t.Fatalf("expected STORE method, got %d", f.Method)
	}
	off, err := f.DataOffset()
	if err != nil {
		t.Fatalf("data offset: %v", err)
	}
	raw := archive[off : off+int64(f.CompressedSize64)]
	if len(raw) != 12+len(plaintext) {
		t.Fatalf("raw len = %d, want %d", len(raw), 12+len(plaintext))
	}

	// Decrypt with our cipher: 12-byte encryption header first, then the body.
	var keys zipCryptoKeys
	keys.initWithPassword(password)
	header := make([]byte, 12)
	for i := range header {
		header[i] = keys.decryptByte(raw[i])
	}
	got := make([]byte, len(plaintext))
	for i := range got {
		got[i] = keys.decryptByte(raw[12+i])
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("decrypt mismatch:\n got = %q\nwant = %q", got, plaintext)
	}

	// The final header byte must equal the high byte of the entry's CRC-32,
	// the standard ZipCrypto password check.
	wantCheck := byte(f.CRC32 >> 24)
	if header[11] != wantCheck {
		// Some writers use the high byte of the MS-DOS time instead; accept
		// the CRC form which yeka/zip uses.
		t.Fatalf("header check byte = %#02x, want %#02x (crc32=%#08x)",
			header[11], wantCheck, f.CRC32)
	}
}

// sanity: ensure binary import stays used if the golden test changes.
var _ = binary.LittleEndian

// sanity: ensure binary import stays used if the golden test changes.
var _ = binary.LittleEndian
