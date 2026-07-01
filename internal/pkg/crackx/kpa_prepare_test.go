package crackx

import (
	"bytes"
	"testing"

	"github.com/yeka/zip"
)

func TestPrepareKnownPlaintextStore(t *testing.T) {
	content := []byte("readme content that is known to the attacker verbatim here")
	archive := makeZipCryptoArchive(t, "readme.txt", "pw", content)

	prep, err := PrepareKnownPlaintext(archive, "readme.txt", content)
	if err != nil {
		t.Fatalf("PrepareKnownPlaintext: %v", err)
	}
	if !prep.Stored || !prep.CRCMatch || !prep.LengthMatch || !prep.Ready {
		t.Fatalf("expected ready store prep, got %+v", prep)
	}
	if !bytes.Equal(prep.Plaintext, content) {
		t.Fatalf("store plaintext should equal original content")
	}

	// Wrong file: CRC must not match and prep must not be ready.
	bad, err := PrepareKnownPlaintext(archive, "readme.txt", []byte("totally different content of same-ish length!!"))
	if err != nil {
		t.Fatalf("PrepareKnownPlaintext(bad): %v", err)
	}
	if bad.CRCMatch || bad.Ready {
		t.Fatalf("expected CRC mismatch for wrong file, got %+v", bad)
	}
}

func TestPrepareKnownPlaintextDeflate(t *testing.T) {
	// Build a Deflate ZipCrypto entry using Go's flate so re-compression matches.
	content := bytes.Repeat([]byte("known plaintext deflate content 0123456789 "), 40)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fh := &zip.FileHeader{Name: "data.bin", Method: zip.Deflate}
	fh.SetPassword("pw")
	fh.SetEncryptionMethod(zip.StandardEncryption)
	w, err := zw.CreateHeader(fh)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	prep, err := PrepareKnownPlaintext(buf.Bytes(), "data.bin", content)
	if err != nil {
		t.Fatalf("PrepareKnownPlaintext: %v", err)
	}
	if !prep.CRCMatch {
		t.Fatalf("CRC should match for correct original file")
	}
	if prep.Stored {
		t.Fatalf("Deflate entry should not be marked stored")
	}
	// Re-compressed plaintext must be usable to build attack data.
	raw, _, err := rawEncryptedData(buf.Bytes(), "data.bin")
	if err != nil {
		t.Fatalf("rawEncryptedData: %v", err)
	}
	if _, err := newAttackData(raw, prep.Plaintext, prep.Offset); err != nil {
		t.Fatalf("prepared plaintext not usable: %v", err)
	}

	// Decrypt the real stored stream with the known password and compare: this
	// tells us whether the auto re-compression byte-matches this archive.
	trueCompressed := decryptRawStream(raw, zipKeysFromCipher(keysFromPassword("pw")))
	t.Logf("lengthMatch=%v, want=%d got=%d, byteMatch=%v",
		prep.LengthMatch, len(trueCompressed), len(prep.Plaintext),
		bytes.Equal(prep.Plaintext, trueCompressed))
}
