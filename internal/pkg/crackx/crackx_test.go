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
	}, 0)
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
	}, 0)
	p := waitDone(t, job)
	if !p.Found || p.Password != "7a" {
		t.Fatalf("expected to find 7a, got %+v", p)
	}
}

func TestZipWrongPasswordExhausts(t *testing.T) {
	data := makeEncryptedZip(t, "zz", zip.AES256Encryption)
	v, _ := NewVerifier(FormatZIP, "", data)
	job := Start(v, Options{Mode: ModeDict, Wordlist: "a\nb\nc"}, 0)
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
	}, 0)
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

func TestLocateMatchesSequentialEnumeration(t *testing.T) {
	// Build the full ordered sequence for a small space, then verify locate +
	// decompose reproduces the candidate at every offset.
	opts := Options{Mode: ModeBrute, MinLen: 1, MaxLen: 3, Charset: Charset{Lower: true, Digits: true}}
	alphabet := []rune(opts.Charset.alphabet())
	base := int64(len(alphabet))

	var seq []string
	for length := 1; length <= 3; length++ {
		idx := make([]int, length)
		for {
			buf := make([]rune, length)
			for i := range idx {
				buf[i] = alphabet[idx[i]]
			}
			seq = append(seq, string(buf))
			pos := length - 1
			for pos >= 0 {
				idx[pos]++
				if idx[pos] < len(alphabet) {
					break
				}
				idx[pos] = 0
				pos--
			}
			if pos < 0 {
				break
			}
		}
	}

	for off := 0; off < len(seq); off++ {
		length, idx, ok := locate(int64(off), base, 1, 3)
		if !ok {
			t.Fatalf("offset %d should be locatable", off)
		}
		buf := make([]rune, length)
		for i := range idx {
			buf[i] = alphabet[idx[i]]
		}
		if got := string(buf); got != seq[off] {
			t.Fatalf("offset %d: locate gave %q, want %q", off, got, seq[off])
		}
	}
	if _, _, ok := locate(int64(len(seq)), base, 1, 3); ok {
		t.Fatalf("offset past the space should not be locatable")
	}
}

func TestResumeSkipsToCandidate(t *testing.T) {
	// "az" is the 36th candidate of length 2 (a*36 + z). Resuming just before it
	// must still find it; resuming just after must miss it.
	data := makeEncryptedZip(t, "az", zip.AES256Encryption)
	opts := Options{Mode: ModeBrute, MinLen: 2, MaxLen: 2, Charset: Charset{Lower: true, Digits: true}}

	v, _ := NewVerifier(FormatZIP, "", data)
	// length-2 index of "az": a->0, z->25, base 36 => 0*36 + 25 = 25.
	job := Start(v, opts, 25)
	p := waitDone(t, job)
	if !p.Found || p.Password != "az" {
		t.Fatalf("resume just before target should find az, got %+v", p)
	}
	if p.ResumedFrom != 25 {
		t.Fatalf("expected ResumedFrom=25, got %d", p.ResumedFrom)
	}

	v2, _ := NewVerifier(FormatZIP, "", data)
	job2 := Start(v2, opts, 26)
	p2 := waitDone(t, job2)
	if p2.Found {
		t.Fatalf("resume past target should not find az, got %+v", p2)
	}
}
