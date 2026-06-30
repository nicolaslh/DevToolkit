package crackx

import (
	"bytes"
	"encoding/binary"
	"io"
	"sync"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/yeka/zip"
)

// winzipAESExtraID is the header ID of the WinZip AES extra field found in the
// central directory of AES-encrypted entries.
const winzipAESExtraID = 0x9901

// ZipEncryption identifies the scheme protecting a single ZIP entry.
type ZipEncryption string

const (
	ZipEncNone ZipEncryption = "none"
	// ZipEncZipCrypto is traditional PKWARE (ZipCrypto) encryption. It is the
	// only scheme vulnerable to the Biham–Kocher known-plaintext attack.
	ZipEncZipCrypto ZipEncryption = "zipcrypto"
	ZipEncAES128    ZipEncryption = "aes128"
	ZipEncAES192    ZipEncryption = "aes192"
	ZipEncAES256    ZipEncryption = "aes256"
	// ZipEncAESOther is a WinZip AES entry whose strength byte is unrecognized.
	ZipEncAESOther ZipEncryption = "aes"
)

// ZipEntryInfo describes one entry's encryption.
type ZipEntryInfo struct {
	Name       string        `json:"name"`
	Encrypted  bool          `json:"encrypted"`
	Encryption ZipEncryption `json:"encryption"`
}

// ZipInfo summarizes the encryption used across a ZIP archive.
type ZipInfo struct {
	Entries      []ZipEntryInfo `json:"entries"`
	Encrypted    bool           `json:"encrypted"`    // at least one encrypted entry
	HasZipCrypto bool           `json:"hasZipCrypto"` // at least one ZipCrypto entry
	HasAES       bool           `json:"hasAES"`       // at least one AES entry
}

// KnownPlaintextViable reports whether the archive contains at least one
// ZipCrypto entry, the precondition for a known-plaintext (Biham–Kocher)
// attack. AES entries are immune to it.
func (zi ZipInfo) KnownPlaintextViable() bool { return zi.HasZipCrypto }

// DetectZipEncryption parses the central directory and reports the encryption
// scheme of every entry. It needs no password and never decrypts data.
func DetectZipEncryption(data []byte) (ZipInfo, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ZipInfo{}, apperr.New(apperr.ParseError, "无法读取 ZIP 文件：可能已损坏或不是有效的压缩包")
	}
	var info ZipInfo
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		enc := entryEncryption(f)
		info.Entries = append(info.Entries, ZipEntryInfo{
			Name:       f.Name,
			Encrypted:  enc != ZipEncNone,
			Encryption: enc,
		})
		switch enc {
		case ZipEncNone:
		case ZipEncZipCrypto:
			info.Encrypted = true
			info.HasZipCrypto = true
		default:
			info.Encrypted = true
			info.HasAES = true
		}
	}
	return info, nil
}

// entryEncryption classifies a single entry. AES entries carry a WinZip AES
// extra field (0x9901); any other encrypted entry is ZipCrypto. Note that the
// reader rewrites Method from 99 to the real compression method, so the extra
// field — not Method — is the reliable AES signal.
func entryEncryption(f *zip.File) ZipEncryption {
	if !f.IsEncrypted() {
		return ZipEncNone
	}
	if strength, ok := aesStrength(f.Extra); ok {
		switch strength {
		case 1:
			return ZipEncAES128
		case 2:
			return ZipEncAES192
		case 3:
			return ZipEncAES256
		default:
			return ZipEncAESOther
		}
	}
	return ZipEncZipCrypto
}

// aesStrength scans a central-directory extra field for the WinZip AES record
// (0x9901) and returns its strength byte (1=AES-128, 2=AES-192, 3=AES-256).
// The record layout after the 4-byte tag/size header is:
// AE version(2) + vendor "AE"(2) + strength(1) + original method(2).
func aesStrength(extra []byte) (byte, bool) {
	b := extra
	for len(b) >= 4 {
		tag := binary.LittleEndian.Uint16(b)
		size := int(binary.LittleEndian.Uint16(b[2:]))
		b = b[4:]
		if size > len(b) {
			return 0, false
		}
		if tag == winzipAESExtraID && size >= 7 {
			return b[4], true
		}
		b = b[size:]
	}
	return 0, false
}

// zipVerifier validates passwords against an encrypted ZIP archive
// (ZipCrypto or AES). A sync.Pool of parsed readers avoids re-parsing the
// central directory on every attempt and keeps Verify safe for concurrent use
// (each goroutine borrows its own reader before mutating its password).
type zipVerifier struct {
	data   []byte
	target string // name of the entry used for verification
	pool   sync.Pool
}

func newZipVerifier(data []byte) (Verifier, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, apperr.New(apperr.ParseError, "无法读取 ZIP 文件：可能已损坏或不是有效的压缩包")
	}

	var target string
	var smallest uint64
	for _, f := range r.File {
		if f.FileInfo().IsDir() || !f.IsEncrypted() {
			continue
		}
		if target == "" || f.UncompressedSize64 < smallest {
			target = f.Name
			smallest = f.UncompressedSize64
		}
	}
	if target == "" {
		return nil, apperr.New(apperr.Unsupported, "该 ZIP 没有加密条目，无需破解")
	}
	v := &zipVerifier{data: data, target: target}
	v.pool.New = func() any {
		pr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil
		}
		return pr
	}
	return v, nil
}

func (v *zipVerifier) Verify(password string) bool {
	r, _ := v.pool.Get().(*zip.Reader)
	if r == nil {
		return false
	}
	defer v.pool.Put(r)

	for _, f := range r.File {
		if f.Name != v.target {
			continue
		}
		f.SetPassword(password)
		rc, err := f.Open()
		if err != nil {
			return false
		}
		// Reading to EOF forces CRC (ZipCrypto) / GCM-tag (AES) verification.
		_, err = io.Copy(io.Discard, rc)
		rc.Close()
		return err == nil
	}
	return false
}
