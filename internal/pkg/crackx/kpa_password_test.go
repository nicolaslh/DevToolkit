package crackx

import (
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestRecoverPasswordShort(t *testing.T) {
	charset := []byte("abcdefghijklmnopqrstuvwxyz0123456789")
	cases := []string{"a", "abc", "pass1", "secret"}
	for _, pw := range cases {
		t.Run(pw, func(t *testing.T) {
			target := keysFromPassword(pw)
			got := recoverPassword(target, charset, 1, 6, runtime.NumCPU(), false, nil, nil)
			if !slices.Contains(got, pw) {
				t.Fatalf("recoverPassword did not find %q, got %v", pw, got)
			}
		})
	}
}

func TestRecoverPasswordLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping longer password recovery in -short mode")
	}
	charset := []byte("abcdefghijklmnopqrstuvwxyz")
	pw := "secrets" // length 7 exercises the long path
	target := keysFromPassword(pw)
	got := recoverPassword(target, charset, 7, 7, runtime.NumCPU(), false, nil, nil)
	if !slices.Contains(got, pw) {
		t.Fatalf("recoverPassword did not find %q, got %v", pw, got)
	}
}

func TestStartPasswordRecoveryJob(t *testing.T) {
	target := keysFromPassword("hi42")
	job := StartPasswordRecovery(zipKeysFromCipher(target),
		Charset{Lower: true, Digits: true}, 1, 6)

	deadline := time.Now().Add(60 * time.Second)
	for {
		snap := job.Snapshot()
		if snap.Finished {
			if !slices.Contains(snap.Passwords, "hi42") {
				t.Fatalf("job did not recover password, got %v", snap.Passwords)
			}
			return
		}
		if time.Now().After(deadline) {
			job.Cancel()
			t.Fatal("password recovery did not finish in time")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
