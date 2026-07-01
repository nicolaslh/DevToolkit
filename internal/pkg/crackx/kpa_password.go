package crackx

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// This file recovers a password that produces a given set of ZipCrypto internal
// keys. It is optional: decryption only needs the keys. The algorithm is the
// charset-based recovery from bkcrack's password.cpp (Joachim Hotonnier,
// zlib/libpng license), reimplemented in Go. The mask-based variant is omitted.
//
// At its heart is a meet-in-the-middle that recovers the last 6 password
// characters for a given start state and target keys; passwords longer than 6
// are handled by brute-forcing the leading characters and recovering the last
// six for each prefix.

// pwRecovery holds the state for one password search. The lookup-derived
// bitsets and the solution sink are shared across clones; the x/y/z/p scratch
// and prefix are per-clone.
type pwRecovery struct {
	charset    []byte
	charsetSet [256]bool

	// Precomputed feasibility filters keyed on the target keys + charset.
	z0_16_32  []bool    // 1<<16 entries: possible Z0[16,32)
	zm1_24_32 [256]bool // possible Z{-1}[24,32)

	// Per-search scratch.
	x, y        [7]uint32
	z           [7]uint32
	p           [6]byte
	candidateX0 uint32
	prefix      []byte
	length      int

	// Shared output and control.
	mu         *sync.Mutex
	solutions  *[]string
	seen       map[string]bool
	found      *atomic.Bool
	exhaustive bool
	cancel     <-chan struct{}
}

func (r *pwRecovery) stop() bool {
	if !r.exhaustive && r.found != nil && r.found.Load() {
		return true
	}
	return canceledChan(r.cancel)
}

// setTarget precomputes the feasibility filters for the target keys, mirroring
// bkcrack's SixCharactersRecovery::setTarget.
func (r *pwRecovery) setTarget(target zipCryptoKeys, charset5 []byte) {
	r.x[6] = target.key0
	r.y[6] = target.key1
	r.z[6] = target.key2

	r.y[5] = (r.y[6]-1)*zipCryptoMultInv - uint32(byte(r.x[6]))
	for i := 6; i > 1; i-- {
		r.z[i-1] = crc32ByteInv(r.z[i], byte(r.y[i]>>24))
	}

	if r.z0_16_32 == nil {
		r.z0_16_32 = make([]bool, 1<<16)
	} else {
		for i := range r.z0_16_32 {
			r.z0_16_32[i] = false
		}
	}
	r.zm1_24_32 = [256]bool{}

	for _, p5 := range charset5 {
		r.x[5] = crc32ByteInv(r.x[6], p5)
		r.y[4] = (r.y[5]-1)*zipCryptoMultInv - uint32(byte(r.x[5]))
		r.z[3] = crc32ByteInv(r.z[4], byte(r.y[4]>>24))

		y3 := (r.y[4] - 1) * zipCryptoMultInv
		msbMin := byte((y3 - 255) >> 24)
		msbMax := byte((y3 - 0) >> 24)

		r.z[2] = crc32ByteInv(r.z[3], msbMin)
		r.z[1] = crc32ByteInv(r.z[2], 0)
		r.z[0] = crc32ByteInv(r.z[1], 0)
		r.z0_16_32[r.z[0]>>16] = true
		r.zm1_24_32[crc32ByteInv(r.z[0], 0)>>24] = true

		if msbMax != msbMin {
			r.z[2] = crc32ByteInv(r.z[3], msbMax)
			r.z[1] = crc32ByteInv(r.z[2], 0)
			r.z[0] = crc32ByteInv(r.z[1], 0)
			r.z0_16_32[r.z[0]>>16] = true
			r.zm1_24_32[crc32ByteInv(r.z[0], 0)>>24] = true
		}
	}
}

// sixSearch recovers the last 6 characters between the initial state and the
// target, mirroring SixCharactersRecovery::search.
func (r *pwRecovery) sixSearch(initial zipCryptoKeys) {
	if !r.z0_16_32[initial.key2>>16] {
		return
	}
	r.x[0] = initial.key0
	r.candidateX0 = initial.key0
	r.y[0] = initial.key1
	r.z[0] = initial.key2
	for i := 1; i <= 4; i++ {
		r.y[i] = getYi_24to32(r.z[i], r.z[i-1])
		r.z[i] = crc32Byte(r.z[i-1], byte(r.y[i]>>24))
	}
	r.searchRecursive(5)
}

func (r *pwRecovery) searchRecursive(i int) {
	if i != 1 {
		fy := (r.y[i] - 1) * zipCryptoMultInv
		ffy := (fy - 1) * zipCryptoMultInv
		diff := ffy - (r.y[i-2] & maskHigh8)
		for _, xi := range multTab.fiber2[byte(diff>>24)] {
			yim1 := fy - uint32(xi)
			if ffy-zipCryptoMultInv*uint32(xi)-(r.y[i-2]&maskHigh8) <= maxDiff24 &&
				byte(yim1>>24) == byte(r.y[i-1]>>24) {
				r.y[i-1] = yim1
				r.x[i] = uint32(xi)
				r.searchRecursive(i - 1)
			}
		}
		return
	}

	x1 := (r.y[1]-1)*zipCryptoMultInv - r.y[0]
	if x1 > 0xff {
		return
	}
	r.x[1] = x1
	for j := 5; j >= 0; j-- {
		xXorP := crc32ByteInv(r.x[j+1], 0)
		r.p[j] = byte(xXorP ^ r.x[j])
		r.x[j] = xXorP ^ uint32(r.p[j])
	}
	if r.x[0] == r.candidateX0 {
		r.onSolution()
	}
}

func (r *pwRecovery) onSolution() {
	full := make([]byte, 0, len(r.prefix)+6)
	full = append(full, r.prefix...)
	full = append(full, r.p[:]...)
	if len(full) < r.length {
		return
	}
	pw := full[len(full)-r.length:]
	for _, c := range pw {
		if !r.charsetSet[c] {
			return
		}
	}
	s := string(pw)
	r.mu.Lock()
	if !r.seen[s] {
		r.seen[s] = true
		*r.solutions = append(*r.solutions, s)
	}
	r.mu.Unlock()
	if !r.exhaustive {
		r.found.Store(true)
	}
}

// searchShort handles passwords of length 0..6.
func (r *pwRecovery) searchShort(length int) {
	init := newZipCryptoKeys()
	for i := length; i < 6; i++ {
		init.updateBackwardPlaintext(r.charset[0])
	}
	r.length = length
	r.prefix = r.prefix[:0]
	r.sixSearch(init)
}

// searchLongRecursive handles passwords of length >= 7 by brute-forcing the
// leading characters and recovering the last six for each prefix.
func (r *pwRecovery) searchLongRecursive(initial zipCryptoKeys) {
	if len(r.prefix)+7 == r.length {
		if !r.zm1_24_32[initial.key2>>24] {
			return
		}
		r.prefix = append(r.prefix, r.charset[0])
		x0p := crc32Byte(initial.key0, 0)
		y0p := initial.key1*zipCryptoMult + 1
		z0p := crc32Byte(initial.key2, 0)
		for _, pi := range r.charset {
			x0 := x0p ^ crc32Tab[pi]
			y0 := y0p + zipCryptoMult*uint32(byte(x0))
			z0 := z0p ^ crc32Tab[byte(y0>>24)]
			if !r.z0_16_32[z0>>16] {
				continue
			}
			r.prefix[len(r.prefix)-1] = pi
			r.x[0] = x0
			r.candidateX0 = x0
			r.y[0] = y0
			r.z[0] = z0
			r.y[1] = getYi_24to32(r.z[1], r.z[0])
			r.z[1] = crc32Byte(r.z[0], byte(r.y[1]>>24))
			r.y[2] = getYi_24to32(r.z[2], r.z[1])
			r.z[2] = crc32Byte(r.z[1], byte(r.y[2]>>24))
			r.y[3] = getYi_24to32(r.z[3], r.z[2])
			r.z[3] = crc32Byte(r.z[2], byte(r.y[3]>>24))
			r.y[4] = getYi_24to32(r.z[4], r.z[3])
			r.searchRecursive(5)
			if r.stop() {
				break
			}
		}
		r.prefix = r.prefix[:len(r.prefix)-1]
		return
	}

	r.prefix = append(r.prefix, r.charset[0])
	for _, pi := range r.charset {
		init := initial
		init.update(pi)
		r.prefix[len(r.prefix)-1] = pi
		r.searchLongRecursive(init)
		if r.stop() {
			break
		}
	}
	r.prefix = r.prefix[:len(r.prefix)-1]
}

// clone returns an independent worker that shares the read-only filters and the
// solution sink but has its own scratch and prefix buffer.
func (r *pwRecovery) clone() *pwRecovery {
	c := *r
	c.prefix = make([]byte, 0, 32)
	return &c
}

// searchLongParallel distributes the first leading character across workers for
// lengths >= 8.
func (r *pwRecovery) searchLongParallel(workers int) {
	cs := r.charset
	var (
		wg   sync.WaitGroup
		next atomic.Int64
	)
	spawn := func() {
		defer wg.Done()
		c := r.clone()
		for {
			i := int(next.Add(1)) - 1
			if i >= len(cs) || r.stop() {
				return
			}
			pi := cs[i]
			c.prefix = c.prefix[:0]
			c.prefix = append(c.prefix, pi)
			init := newZipCryptoKeys()
			init.update(pi)
			c.searchLongRecursive(init)
		}
	}
	if workers < 1 {
		workers = 1
	}
	if workers > len(cs) {
		workers = len(cs)
	}
	for n := 0; n < workers; n++ {
		wg.Add(1)
		go spawn()
	}
	wg.Wait()
}

// recoverPassword searches for passwords made of charset characters with length
// in [minLen, maxLen] that yield target. onLength, if non-nil, is called as each
// length is started.
func recoverPassword(target zipCryptoKeys, charset []byte, minLen, maxLen, workers int,
	exhaustive bool, cancel <-chan struct{}, onLength func(length int)) []string {
	if workers < 1 {
		workers = runtime.NumCPU()
	}
	if minLen < 0 {
		minLen = 0
	}

	var (
		sols  []string
		mu    sync.Mutex
		found atomic.Bool
	)
	base := &pwRecovery{
		charset:    charset,
		mu:         &mu,
		solutions:  &sols,
		seen:       map[string]bool{},
		found:      &found,
		exhaustive: exhaustive,
		cancel:     cancel,
	}
	for _, c := range charset {
		base.charsetSet[c] = true
	}
	base.setTarget(target, charset)

	for length := minLen; length <= maxLen; length++ {
		if base.stop() {
			break
		}
		if onLength != nil {
			onLength(length)
		}
		switch {
		case length <= 6:
			base.searchShort(length)
		case length == 7:
			base.length = 7
			base.prefix = base.prefix[:0]
			base.searchLongRecursive(newZipCryptoKeys())
		default:
			base.length = length
			base.searchLongParallel(workers)
		}
	}
	return sols
}

// --- async password-recovery job ---

// PasswordProgress is a snapshot of a password-recovery job.
type PasswordProgress struct {
	CurrentLength int      `json:"currentLength"`
	Passwords     []string `json:"passwords"`
	Elapsed       float64  `json:"elapsed"`
	Finished      bool     `json:"finished"`
	Canceled      bool     `json:"canceled"`
}

// PasswordJob is a running or finished password-recovery task.
type PasswordJob struct {
	start      time.Time
	cancelCh   chan struct{}
	cancelOnce sync.Once

	curLen atomic.Int64

	mu        sync.Mutex
	passwords []string
	finished  bool
	canceled  bool
}

// StartPasswordRecovery launches a background search for a password that yields
// keys, trying charset combinations of length in [minLen, maxLen].
func StartPasswordRecovery(keys ZipKeys, charset Charset, minLen, maxLen int) *PasswordJob {
	alphabet := []byte(charset.alphabet())
	j := &PasswordJob{start: time.Now(), cancelCh: make(chan struct{})}
	go func() {
		pw := recoverPassword(keys.toCipher(), alphabet, minLen, maxLen, runtime.NumCPU(), false,
			j.cancelCh, func(length int) { j.curLen.Store(int64(length)) })
		j.mu.Lock()
		j.passwords = pw
		j.finished = true
		j.canceled = j.stopped() && len(pw) == 0
		j.mu.Unlock()
	}()
	return j
}

// Cancel asks the job to stop.
func (j *PasswordJob) Cancel() { j.cancelOnce.Do(func() { close(j.cancelCh) }) }

func (j *PasswordJob) stopped() bool {
	select {
	case <-j.cancelCh:
		return true
	default:
		return false
	}
}

// Snapshot returns the current progress.
func (j *PasswordJob) Snapshot() PasswordProgress {
	j.mu.Lock()
	defer j.mu.Unlock()
	return PasswordProgress{
		CurrentLength: int(j.curLen.Load()),
		Passwords:     append([]string(nil), j.passwords...),
		Elapsed:       time.Since(j.start).Seconds(),
		Finished:      j.finished,
		Canceled:      j.canceled,
	}
}
