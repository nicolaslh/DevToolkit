package crackx

import (
	"bytes"
	"runtime"
	"testing"
	"time"

	"github.com/yeka/zip"
)

// addEntry writes one entry to zw, optionally encrypted.
func addEntry(t *testing.T, zw *zip.Writer, name, password string, enc zip.EncryptionMethod, method uint16, content []byte) {
	t.Helper()
	fh := &zip.FileHeader{Name: name, Method: method}
	var (
		w   interface{ Write([]byte) (int, error) }
		err error
	)
	if password != "" {
		fh.SetPassword(password)
		fh.SetEncryptionMethod(enc)
	}
	w, err = zw.CreateHeader(fh)
	if err != nil {
		t.Fatalf("create %s: %v", name, err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestDecryptArchiveEntries validates the decryption pipeline independently of
// the attack, by deriving keys straight from the password. It exercises Store
// and Deflate entries, CRC verification, and skipping AES entries.
func TestDecryptArchiveEntries(t *testing.T) {
	const pw = "shared-pass"
	storeContent := []byte("stored content, uncompressed, recovered verbatim")
	deflateContent := bytes.Repeat([]byte("deflate me! "), 200) // compressible

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addEntry(t, zw, "a_store.txt", pw, zip.StandardEncryption, zip.Store, storeContent)
	addEntry(t, zw, "b_deflate.txt", pw, zip.StandardEncryption, zip.Deflate, deflateContent)
	// AES entry under a different password: must be skipped, not fail.
	addEntry(t, zw, "c_aes.txt", "other-pass", zip.AES256Encryption, zip.Deflate, []byte("aes secret"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	archive := buf.Bytes()

	keys := zipKeysFromCipher(keysFromPassword(pw))
	entries, err := decryptArchiveEntries(archive, keys)
	if err != nil {
		t.Fatalf("decryptArchiveEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (AES should be skipped)", len(entries))
	}
	got := map[string][]byte{}
	for _, e := range entries {
		got[e.Name] = e.Content
	}
	if !bytes.Equal(got["a_store.txt"], storeContent) {
		t.Errorf("store content mismatch")
	}
	if !bytes.Equal(got["b_deflate.txt"], deflateContent) {
		t.Errorf("deflate content mismatch")
	}

	// Wrong keys must be rejected via CRC, not return garbage.
	bad := ZipKeys{Key0: 1, Key1: 2, Key2: 3}
	if _, err := decryptArchiveEntries(archive, bad); err == nil {
		t.Fatal("expected error for wrong keys, got nil")
	}
}

// TestStartKPAEndToEnd runs the full async job: attack a ZipCrypto entry, then
// decrypt the archive with the recovered keys.
func TestStartKPAEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full known-plaintext attack in -short mode")
	}
	const (
		pw   = "j0b-Test!"
		name = "doc.txt"
	)
	content := []byte("end-to-end known-plaintext attack job test. " +
		"padding padding padding 0123456789 abcdefghijklmnopqrstuvwxyz " +
		"the rain in spain falls mainly on the plain.")
	archive := makeZipCryptoArchive(t, name, pw, content)

	job, err := StartKPA(archive, name, content, 0, runtime.NumCPU())
	if err != nil {
		t.Fatalf("StartKPA: %v", err)
	}

	deadline := time.Now().Add(120 * time.Second)
	for {
		snap := job.Snapshot()
		if snap.Finished {
			break
		}
		if time.Now().After(deadline) {
			job.Cancel()
			t.Fatal("attack did not finish within deadline")
		}
		time.Sleep(50 * time.Millisecond)
	}

	keys, entries, err := job.Result()
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if keys != zipKeysFromCipher(keysFromPassword(pw)) {
		t.Fatalf("recovered keys %v != password-derived", keys)
	}
	if len(entries) != 1 || !bytes.Equal(entries[0].Content, content) {
		t.Fatalf("decrypted entries incorrect: %+v", entries)
	}
}

// TestStartKPARejectsAES ensures AES entries are refused up front.
func TestStartKPARejectsAES(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	addEntry(t, zw, "aes.txt", "pw", zip.AES256Encryption, zip.Deflate, []byte("some content here"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := StartKPA(buf.Bytes(), "aes.txt", []byte("some content here"), 0, 1); err == nil {
		t.Fatal("expected AES entry to be rejected, got nil error")
	}
}
