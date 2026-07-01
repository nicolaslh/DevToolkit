package crackx

import (
	"bytes"
	"runtime"
	"testing"
)

// keysFromPassword returns the internal cipher state derived from a password,
// i.e. the value the attack is expected to recover.
func keysFromPassword(pw string) zipCryptoKeys {
	var k zipCryptoKeys
	k.initWithPassword(pw)
	return k
}

func TestRecoverKeysGolden(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full known-plaintext attack in -short mode")
	}
	const (
		password = "S3cr3t-P@ss"
		name     = "payload.bin"
	)
	// Long, varied known plaintext so the Z reduction converges quickly.
	plaintext := []byte("DevToolkit known-plaintext attack self-test payload. " +
		"The quick brown fox jumps over the lazy dog 0123456789. " +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do.")

	archive := makeZipCryptoArchive(t, name, password, plaintext)
	raw, enc, err := rawEncryptedData(archive, name)
	if err != nil {
		t.Fatalf("rawEncryptedData: %v", err)
	}
	if enc != ZipEncZipCrypto {
		t.Fatalf("enc = %q, want zipcrypto", enc)
	}

	d, err := newAttackData(raw, plaintext, 0)
	if err != nil {
		t.Fatalf("newAttackData: %v", err)
	}

	keys := recoverKeys(d, attackOptions{workers: runtime.NumCPU()})
	if len(keys) == 0 {
		t.Fatal("attack found no keys")
	}

	want := keysFromPassword(password)
	var matched bool
	for _, k := range keys {
		if k == want {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("recovered keys %+v do not include password-derived %+v", keys, want)
	}

	// The recovered keys must decrypt the archive without the password.
	dec := keys[0]
	out := make([]byte, len(plaintext))
	// Skip the 12-byte encryption header, then decrypt the body.
	for i := 0; i < encryptionHeaderSize; i++ {
		dec.decryptByte(raw[i])
	}
	for i := range out {
		out[i] = dec.decryptByte(raw[encryptionHeaderSize+i])
	}
	if !bytes.Equal(out, plaintext) {
		t.Fatalf("decryption with recovered keys mismatch:\n got=%q\nwant=%q", out, plaintext)
	}
}
