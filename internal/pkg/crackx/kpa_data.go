package crackx

import (
	"bytes"
	"fmt"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/yeka/zip"
)

// This file prepares the inputs for the Biham–Kocher known-plaintext attack:
// it pulls the raw ZipCrypto-encrypted bytes out of an archive entry and aligns
// them with the caller-supplied known plaintext, yielding the keystream bytes
// the attack consumes.

// encryptionHeaderSize is the number of random bytes ZipCrypto prepends to the
// (compressed) data of every encrypted entry.
const encryptionHeaderSize = 12

// minKnownPlaintext is the minimum number of contiguous known plaintext bytes
// the attack needs. More plaintext yields fewer key candidates and a faster
// run; this is the hard floor below which the attack cannot proceed.
const minKnownPlaintext = 12

// attackData bundles the attack inputs. ciphertext is the full ZipCrypto stream
// for the target entry (12-byte encryption header followed by the encrypted,
// possibly-compressed data). plaintext is the known data. offset is the
// absolute index into ciphertext at which plaintext[0] aligns; it already
// includes the encryption header, so the first data byte is at offset 12.
type attackData struct {
	ciphertext []byte
	plaintext  []byte
	offset     int
}

// keystream returns cipher XOR plaintext for every known position. keystream[j]
// is the cipher's keystream output at absolute ciphertext index offset+j, which
// is exactly what the attack reasons about.
func (d attackData) keystream() []byte {
	ks := make([]byte, len(d.plaintext))
	for i := range d.plaintext {
		ks[i] = d.ciphertext[d.offset+i] ^ d.plaintext[i]
	}
	return ks
}

// newAttackData aligns plaintext against ciphertext. dataOffset is the position
// of the known plaintext relative to the start of the entry's (compressed)
// data, i.e. after the 12-byte encryption header; pass 0 when the plaintext is
// the beginning of the data.
func newAttackData(ciphertext, plaintext []byte, dataOffset int) (attackData, error) {
	if len(plaintext) < minKnownPlaintext {
		return attackData{}, apperr.New(apperr.InvalidInput,
			fmt.Sprintf("已知明文至少需要 %d 字节，当前 %d 字节", minKnownPlaintext, len(plaintext)))
	}
	if dataOffset < 0 {
		return attackData{}, apperr.New(apperr.InvalidInput, "明文偏移不能为负")
	}
	abs := encryptionHeaderSize + dataOffset
	if abs+len(plaintext) > len(ciphertext) {
		return attackData{}, apperr.New(apperr.InvalidInput,
			"已知明文超出密文范围：偏移或长度不正确")
	}
	return attackData{ciphertext: ciphertext, plaintext: plaintext, offset: abs}, nil
}

// rawEncryptedData returns the raw encrypted bytes of the named entry (the
// 12-byte header plus encrypted data) along with the entry's encryption scheme,
// without needing a password. The attack only applies to ZipCrypto entries; the
// caller is responsible for rejecting AES.
func rawEncryptedData(archive []byte, entryName string) ([]byte, ZipEncryption, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, ZipEncNone, apperr.New(apperr.ParseError,
			"无法读取 ZIP 文件：可能已损坏或不是有效的压缩包")
	}
	for _, f := range r.File {
		if f.Name != entryName {
			continue
		}
		enc := entryEncryption(f)
		off, err := f.DataOffset()
		if err != nil {
			return nil, enc, apperr.New(apperr.ParseError, "无法定位条目的加密数据")
		}
		end := off + int64(f.CompressedSize64)
		if off < 0 || end > int64(len(archive)) || end < off {
			return nil, enc, apperr.New(apperr.ParseError, "条目数据范围越界，压缩包可能已损坏")
		}
		return archive[off:end], enc, nil
	}
	return nil, ZipEncNone, apperr.New(apperr.InvalidInput,
		fmt.Sprintf("压缩包中找不到条目 %q", entryName))
}
