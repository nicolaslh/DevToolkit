package crackx

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/yeka/zip"
)

// waitDone polls a job until it finishes or the timeout elapses.
func waitDone(t *testing.T, job *Job) Progress {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		p := job.Snapshot()
		if p.Done {
			return p
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not finish in time")
	return Progress{}
}

func makeEncryptedZip(t *testing.T, password string, enc zip.EncryptionMethod) []byte {
	t.Helper()
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	fw, err := w.Encrypt("secret.txt", password, enc)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if _, err := io.Copy(fw, strings.NewReader("the quick brown fox")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return buf.Bytes()
}

func TestZipDictionaryAES(t *testing.T) {
	data := makeEncryptedZip(t, "ab2", zip.AES256Encryption)
	v, err := NewVerifier(FormatAuto, "a.zip", data)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	job := Start(v, Options{
		Mode:     ModeDict,
		Wordlist: "wrong\nnope\nab2\nextra",
	})
	p := waitDone(t, job)
	if !p.Found || p.Password != "ab2" {
		t.Fatalf("expected to find ab2, got %+v", p)
	}
}

func TestZipBruteZipCrypto(t *testing.T) {
	data := makeEncryptedZip(t, "7a", zip.StandardEncryption)
	v, err := NewVerifier(FormatZIP, "", data)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	job := Start(v, Options{
		Mode:    ModeBrute,
		MinLen:  1,
		MaxLen:  2,
		Charset: Charset{Lower: true, Digits: true},
	})
	p := waitDone(t, job)
	if !p.Found || p.Password != "7a" {
		t.Fatalf("expected to find 7a, got %+v", p)
	}
}

func TestZipWrongPasswordExhausts(t *testing.T) {
	data := makeEncryptedZip(t, "zz", zip.AES256Encryption)
	v, _ := NewVerifier(FormatZIP, "", data)
	job := Start(v, Options{Mode: ModeDict, Wordlist: "a\nb\nc"})
	p := waitDone(t, job)
	if p.Found {
		t.Fatalf("did not expect a match, got %+v", p)
	}
	if p.Error == "" {
		t.Fatalf("expected exhaustion error")
	}
}

func TestCancel(t *testing.T) {
	data := makeEncryptedZip(t, "longpassword", zip.AES256Encryption)
	v, _ := NewVerifier(FormatZIP, "", data)
	job := Start(v, Options{
		Mode:    ModeBrute,
		MinLen:  1,
		MaxLen:  8,
		Charset: Charset{Lower: true},
	})
	time.Sleep(20 * time.Millisecond)
	job.Cancel()
	p := waitDone(t, job)
	if !p.Canceled {
		t.Fatalf("expected canceled, got %+v", p)
	}
}

func TestCountSpace(t *testing.T) {
	got := countSpace(Options{Mode: ModeBrute, MinLen: 1, MaxLen: 2, Charset: Charset{Digits: true}})
	if got != 10+100 {
		t.Fatalf("want 110, got %d", got)
	}
	got = countSpace(Options{Mode: ModeDict, Wordlist: "a\n\nb\nc\n"})
	if got != 3 {
		t.Fatalf("want 3, got %d", got)
	}
}

func TestAlphabetDedup(t *testing.T) {
	a := Charset{Lower: true, Custom: "a?!"}.alphabet()
	if strings.Count(a, "a") != 1 {
		t.Fatalf("expected deduped alphabet, got %q", a)
	}
}
