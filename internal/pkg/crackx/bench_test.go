package crackx

import (
	"sync/atomic"
	"testing"
	"time"
)

// slowVerifier simulates a CPU-bound per-candidate cost (like the Office KDF)
// so the benchmark reflects parallel scaling rather than channel overhead.
type slowVerifier struct {
	calls atomic.Int64
}

func (s *slowVerifier) Verify(string) bool {
	s.calls.Add(1)
	// Busy-ish work; never matches so the whole space is enumerated.
	t := time.Now()
	for time.Since(t) < 50*time.Microsecond {
	}
	return false
}

// BenchmarkBruteThroughput measures candidates/sec across the worker pool.
func BenchmarkBruteThroughput(b *testing.B) {
	opts := Options{Mode: ModeBrute, MinLen: 3, MaxLen: 3, Charset: Charset{Lower: true}}
	for i := 0; i < b.N; i++ {
		v := &slowVerifier{}
		job := Start(v, opts, 0)
		for {
			if job.Snapshot().Done {
				break
			}
			time.Sleep(time.Millisecond)
		}
	}
}
