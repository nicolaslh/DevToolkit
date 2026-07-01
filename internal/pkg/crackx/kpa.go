package crackx

import (
	"bytes"
	"compress/flate"
	"fmt"
	"hash/crc32"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/yeka/zip"
)

// This file is the public surface of the ZipCrypto known-plaintext attack: it
// recovers the cipher's internal keys from known plaintext and uses them to
// decrypt every ZipCrypto entry in an archive — all without the password.

// ZipKeys is the recovered 96-bit ZipCrypto internal state. Any entry encrypted
// under the same password can be decrypted with it, so the password itself is
// never needed.
type ZipKeys struct {
	Key0 uint32 `json:"key0"`
	Key1 uint32 `json:"key1"`
	Key2 uint32 `json:"key2"`
}

func (k ZipKeys) toCipher() zipCryptoKeys {
	return zipCryptoKeys{key0: k.Key0, key1: k.Key1, key2: k.Key2}
}

func zipKeysFromCipher(k zipCryptoKeys) ZipKeys {
	return ZipKeys{Key0: k.key0, Key1: k.key1, Key2: k.key2}
}

// String renders the keys in bkcrack's hexadecimal form.
func (k ZipKeys) String() string {
	return fmt.Sprintf("%08x %08x %08x", k.Key0, k.Key1, k.Key2)
}

// DecryptedEntry is one decrypted file recovered from an archive.
type DecryptedEntry struct {
	Name    string `json:"name"`
	Content []byte `json:"content"`
}

// decryptRawStream decrypts a raw ZipCrypto stream (12-byte header + body) with
// keys and returns the (still possibly-compressed) body bytes.
func decryptRawStream(raw []byte, keys ZipKeys) []byte {
	k := keys.toCipher()
	for i := 0; i < encryptionHeaderSize; i++ {
		k.decryptByte(raw[i])
	}
	body := make([]byte, len(raw)-encryptionHeaderSize)
	for i := range body {
		body[i] = k.decryptByte(raw[encryptionHeaderSize+i])
	}
	return body
}

// decryptArchiveEntries decrypts every ZipCrypto entry in the archive using
// keys. Each decrypted file's CRC-32 is verified, so a wrong key set is
// reported as an error rather than yielding garbage. AES and unencrypted
// entries are skipped.
func decryptArchiveEntries(archive []byte, keys ZipKeys) ([]DecryptedEntry, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, apperr.New(apperr.ParseError, "无法读取 ZIP 文件：可能已损坏或不是有效的压缩包")
	}

	var out []DecryptedEntry
	for _, f := range r.File {
		if f.FileInfo().IsDir() || entryEncryption(f) != ZipEncZipCrypto {
			continue
		}

		off, err := f.DataOffset()
		if err != nil {
			return nil, apperr.New(apperr.ParseError, fmt.Sprintf("无法定位条目 %q 的加密数据", f.Name))
		}
		end := off + int64(f.CompressedSize64)
		if off < 0 || end > int64(len(archive)) || end < off {
			return nil, apperr.New(apperr.ParseError, "条目数据范围越界，压缩包可能已损坏")
		}

		body := decryptRawStream(archive[off:end], keys)

		content, err := decompressEntry(f.Method, body)
		if err != nil {
			return nil, err
		}

		if crc32.ChecksumIEEE(content) != f.CRC32 {
			return nil, apperr.New(apperr.InvalidInput,
				"解密校验失败：密钥不正确，或该条目与提供的明文使用了不同的密码")
		}
		out = append(out, DecryptedEntry{Name: f.Name, Content: content})
	}

	if len(out) == 0 {
		return nil, apperr.New(apperr.Unsupported, "压缩包中没有可解密的 ZipCrypto 条目")
	}
	return out, nil
}

// decompressEntry turns a decrypted entry body into the original file content,
// inflating Deflate streams and passing Stored data through.
func decompressEntry(method uint16, body []byte) ([]byte, error) {
	switch method {
	case zip.Store:
		return body, nil
	case zip.Deflate:
		fr := flate.NewReader(bytes.NewReader(body))
		defer fr.Close()
		content, err := io.ReadAll(fr)
		if err != nil {
			return nil, apperr.New(apperr.InvalidInput,
				"解压失败：密钥不正确，或压缩数据已损坏")
		}
		return content, nil
	default:
		return nil, apperr.New(apperr.Unsupported,
			fmt.Sprintf("暂不支持的压缩方法：%d", method))
	}
}

// KPAPhase identifies which stage a known-plaintext attack job is in.
type KPAPhase string

const (
	KPAPhaseReducing   KPAPhase = "reducing"   // narrowing key2 candidates
	KPAPhaseAttacking  KPAPhase = "attacking"  // testing candidates for full keys
	KPAPhaseDecrypting KPAPhase = "decrypting" // decrypting archive with recovered keys
	KPAPhaseDone       KPAPhase = "done"
)

// KPAProgress is an immutable snapshot of a known-plaintext attack job.
type KPAProgress struct {
	Phase     KPAPhase `json:"phase"`
	Done      int64    `json:"done"`
	Total     int64    `json:"total"`
	Elapsed   float64  `json:"elapsed"` // seconds
	Finished  bool     `json:"finished"`
	Found     bool     `json:"found"`
	Keys      string   `json:"keys"`      // hex keys when found
	FileCount int      `json:"fileCount"` // decrypted entries when finished
	Canceled  bool     `json:"canceled"`
	Error     string   `json:"error"`
}

// KPAJob is a running or finished known-plaintext attack.
type KPAJob struct {
	start      time.Time
	cancelCh   chan struct{}
	cancelOnce sync.Once

	phase atomic.Pointer[KPAPhase]
	done  atomic.Int64
	total atomic.Int64

	mu       sync.Mutex
	finished bool
	found    bool
	keys     ZipKeys
	entries  []DecryptedEntry
	canceled bool
	errMsg   string
}

// StartKPA validates the inputs and launches a known-plaintext attack against
// entryName in the archive, using plaintext known to start at the given offset
// within that entry's (compressed) data. It returns immediately; poll Snapshot
// for progress. workers <= 0 uses all CPUs.
func StartKPA(archive []byte, entryName string, plaintext []byte, offset, workers int) (*KPAJob, error) {
	raw, enc, err := rawEncryptedData(archive, entryName)
	if err != nil {
		return nil, err
	}
	if enc != ZipEncZipCrypto {
		return nil, apperr.New(apperr.Unsupported,
			"该条目不是 ZipCrypto 加密（可能是 AES），已知明文攻击不适用")
	}
	d, err := newAttackData(raw, plaintext, offset)
	if err != nil {
		return nil, err
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	j := &KPAJob{start: time.Now(), cancelCh: make(chan struct{})}
	j.setPhase(KPAPhaseReducing)
	go j.run(archive, d, workers)
	return j, nil
}

func (j *KPAJob) setPhase(p KPAPhase) { j.phase.Store(&p) }

func (j *KPAJob) run(archive []byte, d attackData, workers int) {
	report := func(phase string, done, total int) {
		j.setPhase(KPAPhase(phase))
		j.done.Store(int64(done))
		j.total.Store(int64(total))
	}

	keys := recoverKeys(d, attackOptions{
		workers: workers,
		cancel:  j.cancelCh,
		report:  report,
	})

	if j.stopped() {
		j.finish(false, ZipKeys{}, nil, "", true)
		return
	}
	if len(keys) == 0 {
		j.finish(false, ZipKeys{}, nil,
			"未能还原密钥：请确认已知明文确实属于该条目，且为压缩后的字节（偏移正确）", false)
		return
	}

	found := zipKeysFromCipher(keys[0])
	j.setPhase(KPAPhaseDecrypting)
	entries, err := decryptArchiveEntries(archive, found)
	if err != nil {
		// Keys were found even if decrypting some entry failed; surface both.
		j.finish(true, found, nil, err.Error(), false)
		return
	}
	j.finish(true, found, entries, "", false)
}

// Cancel asks the job to stop as soon as possible.
func (j *KPAJob) Cancel() { j.cancelOnce.Do(func() { close(j.cancelCh) }) }

func (j *KPAJob) stopped() bool {
	select {
	case <-j.cancelCh:
		return true
	default:
		return false
	}
}

func (j *KPAJob) finish(found bool, keys ZipKeys, entries []DecryptedEntry, errMsg string, canceled bool) {
	j.mu.Lock()
	j.finished = true
	j.found = found
	j.keys = keys
	j.entries = entries
	j.errMsg = errMsg
	j.canceled = canceled
	j.mu.Unlock()
	j.setPhase(KPAPhaseDone)
}

// Snapshot returns the job's current progress.
func (j *KPAJob) Snapshot() KPAProgress {
	j.mu.Lock()
	defer j.mu.Unlock()

	phase := KPAPhaseReducing
	if p := j.phase.Load(); p != nil {
		phase = *p
	}
	p := KPAProgress{
		Phase:     phase,
		Done:      j.done.Load(),
		Total:     j.total.Load(),
		Elapsed:   time.Since(j.start).Seconds(),
		Finished:  j.finished,
		Found:     j.found,
		FileCount: len(j.entries),
		Canceled:  j.canceled,
		Error:     j.errMsg,
	}
	if j.found {
		p.Keys = j.keys.String()
	}
	return p
}

// Result returns the recovered keys and decrypted entries once the job has
// finished successfully. It errors if the job is still running or failed.
func (j *KPAJob) Result() (ZipKeys, []DecryptedEntry, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.finished {
		return ZipKeys{}, nil, apperr.New(apperr.InvalidInput, "攻击尚未完成")
	}
	if !j.found {
		msg := j.errMsg
		if msg == "" {
			msg = "未找到密钥"
		}
		return ZipKeys{}, nil, apperr.New(apperr.InvalidInput, msg)
	}
	return j.keys, j.entries, nil
}

// Keys returns the recovered internal keys and whether they are available.
func (j *KPAJob) Keys() (ZipKeys, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.keys, j.found
}
