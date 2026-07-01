package crackx

import (
	"sync"
	"sync/atomic"
)

// This file implements the Biham–Kocher known-plaintext attack on the legacy
// ZipCrypto cipher: given enough known plaintext, it recovers the cipher's
// 96-bit internal state (key0/key1/key2), after which the whole archive can be
// decrypted without the password.
//
// The algorithm and its lookup-table optimizations are ported from bkcrack by
// Joachim Hotonnier (https://github.com/kimci86/bkcrack), used under the
// zlib/libpng license. This is an independent Go reimplementation; the original
// is not modified or redistributed.

// Bit masks and difference bounds, named after bkcrack's mask<begin,end> and
// maxdiff<x> helpers.
const (
	maskZ2to32 = 0xFFFFFFFC // mask<2,32>: discard the 2 least significant bits
	maskHigh8  = 0xFF000000 // mask<24,32>
	maxDiff24  = 0x010000FE // maxdiff<24> = mask<0,24> + 0xff
	maxDiff26  = 0x040000FE // maxdiff<26> = mask<0,26> + 0xff
)

// attackContiguousSize is the number of contiguous keystream bytes the core
// attack consumes (bkcrack's Attack::contiguousSize).
const attackContiguousSize = 8

// --- keystream lookup tables (bkcrack KeystreamTab) ---

type keystreamTables struct {
	tab    [1 << 14]byte     // Zi[2,16) -> keystream byte
	filter [256][64][]uint32 // (k, Zi[10,16)) -> Zi[2,16) candidates
	exists [256][64]bool     // whether the corresponding filter list is non-empty
}

var ksTab = makeKeystreamTables()

func makeKeystreamTables() *keystreamTables {
	t := &keystreamTables{}
	for z := uint32(0); z < (1 << 16); z += 4 {
		k := byte(((z | 2) * (z | 3)) >> 8)
		t.tab[z>>2] = k
		t.filter[k][z>>10] = append(t.filter[k][z>>10], z)
		t.exists[k][z>>10] = true
	}
	return t
}

// keystreamByte returns the keystream byte derived from a key2 (Zi) value;
// only Zi[2,16) is used.
func keystreamByte(z uint32) byte { return ksTab.tab[(z&0xffff)>>2] }

// getZi2to16 returns the Zi[2,16) values consistent with keystream byte ki and
// the given Zi[10,16) bits. The slice is owned by the table; do not mutate it.
func getZi2to16(ki byte, zi10to16 uint32) []uint32 {
	return ksTab.filter[ki][(zi10to16&0xffff)>>10]
}

// hasZi2to16 reports whether getZi2to16 would return a non-empty slice.
func hasZi2to16(ki byte, zi10to16 uint32) bool {
	return ksTab.exists[ki][(zi10to16&0xffff)>>10]
}

// --- multiplication lookup tables (bkcrack MultTab) ---

type multTables struct {
	fiber2 [256][]byte
	fiber3 [256][]byte
}

var multTab = makeMultTables()

func makeMultTables() *multTables {
	t := &multTables{}
	prodInv := uint32(0)
	for x := 0; x < 256; x++ {
		m := byte(prodInv >> 24)
		t.fiber2[m] = append(t.fiber2[m], byte(x))
		t.fiber2[byte(m+1)] = append(t.fiber2[byte(m+1)], byte(x))
		t.fiber3[byte(m+255)] = append(t.fiber3[byte(m+255)], byte(x))
		t.fiber3[m] = append(t.fiber3[m], byte(x))
		t.fiber3[byte(m+1)] = append(t.fiber3[byte(m+1)], byte(x))
		prodInv += zipCryptoMultInv
	}
	return t
}

// --- CRC32-based helpers on key2 (Zi) values (bkcrack Crc32Tab) ---

// getZim1_10to32 derives Z{i-1}[10,32) from Zi[2,32) via CRC32^-1.
func getZim1_10to32(zi2to32 uint32) uint32 {
	return crc32ByteInv(zi2to32, 0) & 0xFFFFFC00 // mask<10,32>
}

// getYi_24to32 derives Yi[24,32) from Zi and Z{i-1} via CRC32^-1.
func getYi_24to32(zi, zim1 uint32) uint32 {
	return (crc32ByteInv(zi, 0) ^ zim1) << 24
}

// --- Z reduction (bkcrack Zreduction) ---
//
// zreduction narrows the set of possible Zi[2,32) values for the last attacked
// index by propagating keystream constraints backward through the known
// plaintext.

type zreduction struct {
	keystream  []byte
	index      int
	candidates []uint32 // Zi[10,32) values during reduce; Zi[2,32) after generate
}

func newZreduction(keystream []byte) *zreduction {
	zr := &zreduction{keystream: keystream}
	zr.index = len(keystream) - 1
	for s := uint32(0); s < (1 << 22); s++ {
		v := s << 10
		if hasZi2to16(keystream[zr.index], v) {
			zr.candidates = append(zr.candidates, v)
		}
	}
	return zr
}

// reduce walks indices backward, keeping at each step only the Z{i-1}[10,32)
// values compatible with the keystream, and remembers the index with the
// fewest candidates. report and canceled may be nil.
func (zr *zreduction) reduce(report func(done, total int), canceled func() bool) {
	const trackSizeThreshold = 1 << 16
	const waitSizeThreshold = 1 << 8

	tracking := false
	var bestCopy []uint32
	bestIndex := zr.index
	bestSize := trackSizeThreshold
	waiting := false
	wait := 0

	set := make([]bool, 1<<22)
	var touched []uint32
	var next []uint32

	total := len(zr.keystream) - attackContiguousSize
	done := 0

	for i := zr.index; i >= attackContiguousSize; i-- {
		for _, idx := range touched {
			set[idx] = false
		}
		touched = touched[:0]
		next = next[:0]
		numberOfZim1 := 0

		for _, zi10to32 := range zr.candidates {
			for _, zi2to16 := range getZi2to16(zr.keystream[i], zi10to32) {
				zim1 := getZim1_10to32(zi10to32 | zi2to16)
				bi := zim1 >> 10
				if !set[bi] && hasZi2to16(zr.keystream[i-1], zim1) {
					next = append(next, zim1)
					set[bi] = true
					touched = append(touched, bi)
					numberOfZim1 += len(getZi2to16(zr.keystream[i-1], zim1))
				}
			}
		}

		if numberOfZim1 <= bestSize {
			tracking = true
			bestIndex = i - 1
			bestSize = numberOfZim1
			waiting = false
		} else if tracking {
			if bestIndex == i {
				bestCopy, zr.candidates = zr.candidates, bestCopy
				if bestSize <= waitSizeThreshold {
					waiting = true
					wait = bestSize * 4
				}
			}
			if waiting {
				wait--
				if wait == 0 {
					break
				}
			}
		}

		zr.candidates, next = next, zr.candidates
		done++
		if report != nil {
			report(done, total)
		}
		if canceled != nil && canceled() {
			break
		}
	}

	if tracking {
		if bestIndex != attackContiguousSize-1 {
			zr.candidates, bestCopy = bestCopy, zr.candidates
		}
		zr.index = bestIndex
	} else {
		zr.index = attackContiguousSize - 1
	}
}

// generate expands the surviving Zi[10,32) values into full Zi[2,32) candidates
// using the keystream byte at the reduced index.
func (zr *zreduction) generate() {
	n := len(zr.candidates)
	for i := 0; i < n; i++ {
		v := getZi2to16(zr.keystream[zr.index], zr.candidates[i])
		for _, zi2to16 := range v[1:] {
			zr.candidates = append(zr.candidates, zr.candidates[i]|zi2to16)
		}
		zr.candidates[i] |= v[0]
	}
}

// --- core attack (bkcrack Attack) ---

// attackWorker carries out the attack for individual Zi[2,32) candidates. Each
// goroutine owns one worker so the zlist/ylist/xlist scratch arrays are not
// shared.
type attackWorker struct {
	keystream  []byte
	plaintext  []byte
	ciphertext []byte
	offset     int
	index      int // starting index of the attacked window within keystream/plaintext
	exhaustive bool

	zlist [attackContiguousSize]uint32
	ylist [attackContiguousSize]uint32 // first two entries unused
	xlist [attackContiguousSize]uint32 // first four entries unused

	found []zipCryptoKeys
}

func newAttackWorker(d attackData, keystream []byte, zrIndex int, exhaustive bool) *attackWorker {
	return &attackWorker{
		keystream:  keystream,
		plaintext:  d.plaintext,
		ciphertext: d.ciphertext,
		offset:     d.offset,
		index:      zrIndex + 1 - attackContiguousSize,
		exhaustive: exhaustive,
	}
}

// carryout runs the attack for one Zi[2,32) candidate, returning any keys found.
func (w *attackWorker) carryout(z7 uint32) []zipCryptoKeys {
	w.found = w.found[:0]
	w.zlist[7] = z7
	w.exploreZlists(7)
	return w.found
}

func (w *attackWorker) exploreZlists(i int) {
	if i != 0 {
		zim110to32 := getZim1_10to32(w.zlist[i])
		for _, zim12to16 := range getZi2to16(w.keystream[w.index+i-1], zim110to32) {
			w.zlist[i-1] = zim110to32 | zim12to16

			// recover Zi[0,2) from CRC32^-1 and Z{i-1}
			w.zlist[i] &= maskZ2to32
			w.zlist[i] |= (crc32ByteInv(w.zlist[i], 0) ^ w.zlist[i-1]) >> 8

			if i < 7 {
				w.ylist[i+1] = getYi_24to32(w.zlist[i+1], w.zlist[i])
			}
			w.exploreZlists(i - 1)
		}
		return
	}

	// Z-list complete: iterate over possible Y7 values.
	multInv := uint32(zipCryptoMultInv)
	msbY7 := uint32(byte(w.ylist[7] >> 24))
	prod := (zipCryptoMultInv*msbY7)<<24 - zipCryptoMultInv
	step := multInv << 8
	for y7High := uint32(0); y7High < (1 << 24); y7High += 1 << 8 {
		target := byte(w.ylist[6]>>24) - byte(prod>>24)
		for _, y7Low := range multTab.fiber3[target] {
			if prod+zipCryptoMultInv*uint32(y7Low)-(w.ylist[6]&maskHigh8) <= maxDiff24 {
				w.ylist[7] = uint32(y7Low) | y7High | (w.ylist[7] & maskHigh8)
				w.exploreYlists(7)
			}
		}
		prod += step
	}
}

func (w *attackWorker) exploreYlists(i int) {
	if i != 3 {
		fy := (w.ylist[i] - 1) * zipCryptoMultInv
		ffy := (fy - 1) * zipCryptoMultInv
		diff := ffy - (w.ylist[i-2] & maskHigh8)
		for _, xi := range multTab.fiber2[byte(diff>>24)] {
			yim1 := fy - uint32(xi)
			if ffy-zipCryptoMultInv*uint32(xi)-(w.ylist[i-2]&maskHigh8) <= maxDiff24 &&
				byte(yim1>>24) == byte(w.ylist[i-1]>>24) {
				w.ylist[i-1] = yim1
				w.xlist[i] = uint32(xi)
				w.exploreYlists(i - 1)
			}
		}
		return
	}
	w.testXlist()
}

func (w *attackWorker) testXlist() {
	// reconstruct X5..X7 high bytes from plaintext
	for i := 5; i <= 7; i++ {
		w.xlist[i] = (crc32Byte(w.xlist[i-1], w.plaintext[w.index+i-1]) & 0xFFFFFF00) |
			(w.xlist[i] & 0xff)
	}

	// compute X3 by inverting the CRC chain
	x := w.xlist[7]
	for i := 6; i >= 3; i-- {
		x = crc32ByteInv(x, w.plaintext[w.index+i])
	}

	// check that X3 is consistent with Y1[26,32)
	y126to32 := getYi_24to32(w.zlist[1], w.zlist[0]) & 0xFC000000 // mask<26,32>
	if ((w.ylist[3]-1)*zipCryptoMultInv-uint32(byte(x))-1)*zipCryptoMultInv-y126to32 > maxDiff26 {
		return
	}

	// filter forward over the remaining contiguous plaintext
	kf := zipCryptoKeys{key0: w.xlist[7], key1: w.ylist[7], key2: w.zlist[7]}
	kf.update(w.plaintext[w.index+7])
	for j := w.index + 8; j < len(w.plaintext); j++ {
		c := w.ciphertext[w.offset+j]
		if c^kf.streamByte() != w.plaintext[j] {
			return
		}
		kf.update(w.plaintext[j])
	}

	// and backward
	kb := zipCryptoKeys{key0: x, key1: w.ylist[3], key2: w.zlist[3]}
	for j := w.index + 2; j >= 0; j-- {
		c := w.ciphertext[w.offset+j]
		kb.updateBackward(c)
		if c^kb.streamByte() != w.plaintext[j] {
			return
		}
	}

	// all filters passed: recover the initial (password-derived) keys
	kb.updateBackwardRange(w.ciphertext, w.offset, 0)
	w.found = append(w.found, kb)
}

// --- driver ---

// attackOptions configures recoverKeys.
type attackOptions struct {
	workers    int
	exhaustive bool                                // find all keys vs. stop at the first
	cancel     <-chan struct{}                     // closed to abort early; may be nil
	report     func(phase string, done, total int) // progress callback; may be nil
}

func canceledChan(ch <-chan struct{}) bool {
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// recoverKeys runs the known-plaintext attack and returns the recovered
// internal keys. With exhaustive=false it returns as soon as one solution is
// found. The returned slice is empty when no keys match the plaintext.
func recoverKeys(d attackData, opts attackOptions) []zipCryptoKeys {
	keystream := d.keystream()

	zr := newZreduction(keystream)
	if len(keystream) > attackContiguousSize {
		zr.reduce(func(done, total int) {
			if opts.report != nil {
				opts.report("reducing", done, total)
			}
		}, func() bool { return canceledChan(opts.cancel) })
	}
	if canceledChan(opts.cancel) {
		return nil
	}
	zr.generate()

	candidates := zr.candidates
	zrIndex := zr.index
	total := len(candidates)

	workers := opts.workers
	if workers < 1 {
		workers = 1
	}
	if workers > total {
		workers = total
	}
	if workers < 1 {
		return nil
	}

	var (
		mu        sync.Mutex
		solutions []zipCryptoKeys
		found     atomic.Bool
		nextIdx   atomic.Int64
		tried     atomic.Int64
		wg        sync.WaitGroup
	)

	run := func() {
		defer wg.Done()
		w := newAttackWorker(d, keystream, zrIndex, opts.exhaustive)
		for {
			i := int(nextIdx.Add(1)) - 1
			if i >= total {
				return
			}
			if !opts.exhaustive && found.Load() {
				return
			}
			if canceledChan(opts.cancel) {
				return
			}
			if sols := w.carryout(candidates[i]); len(sols) > 0 {
				mu.Lock()
				solutions = append(solutions, sols...)
				mu.Unlock()
				if !opts.exhaustive {
					found.Store(true)
				}
			}
			t := tried.Add(1)
			if opts.report != nil {
				opts.report("attacking", int(t), total)
			}
		}
	}

	for n := 0; n < workers; n++ {
		wg.Add(1)
		go run()
	}
	wg.Wait()
	return solutions
}
