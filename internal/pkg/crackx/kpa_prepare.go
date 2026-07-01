package crackx

import (
	"bytes"
	"compress/flate"
	"fmt"
	"hash/crc32"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/yeka/zip"
)

// This file turns a user-supplied original file into the "known plaintext" the
// attack needs. ZipCrypto encrypts the compressed form of a file, so for
// Deflate entries the raw file must be re-compressed to match; this helper
// automates that and verifies the file is the right one via its CRC-32.

// PlaintextPrep is the outcome of preparing known plaintext from an original
// file. Plaintext is the (compressed) bytes to feed the attack.
type PlaintextPrep struct {
	Plaintext   []byte `json:"plaintext"`
	Offset      int    `json:"offset"`
	Method      uint16 `json:"method"`      // 0 = Store, 8 = Deflate
	Stored      bool   `json:"stored"`      // true when the entry is uncompressed
	CRCMatch    bool   `json:"crcMatch"`    // the original file's CRC-32 matches the entry
	LengthMatch bool   `json:"lengthMatch"` // the (re)compressed length matches the entry
	Ready       bool   `json:"ready"`       // safe to launch the attack directly
	Message     string `json:"message"`     // human-readable guidance
}

// deflateLevels are tried in turn when re-compressing a Deflate entry. Level 6
// (zlib/most tools' default) comes first as the most likely match.
var deflateLevels = []int{6, 9, 5, 7, 8, 4, 3, 2, 1, flate.HuffmanOnly}

func rawDeflate(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// PrepareKnownPlaintext prepares the known plaintext for entryName from the
// original (decompressed) file the user provides. It verifies the file via
// CRC-32 and, for Deflate entries, re-compresses it to match the archive.
func PrepareKnownPlaintext(archive []byte, entryName string, rawFile []byte) (PlaintextPrep, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return PlaintextPrep{}, apperr.New(apperr.ParseError, "无法读取 ZIP 文件：可能已损坏或不是有效的压缩包")
	}

	var f *zip.File
	for _, e := range r.File {
		if e.Name == entryName {
			f = e
			break
		}
	}
	if f == nil {
		return PlaintextPrep{}, apperr.New(apperr.InvalidInput, fmt.Sprintf("压缩包中找不到条目 %q", entryName))
	}
	if entryEncryption(f) != ZipEncZipCrypto {
		return PlaintextPrep{}, apperr.New(apperr.Unsupported,
			"该条目不是 ZipCrypto 加密（可能是 AES），已知明文攻击不适用")
	}

	prep := PlaintextPrep{Method: f.Method}
	prep.CRCMatch = crc32.ChecksumIEEE(rawFile) == f.CRC32

	// Compressed data length = stored compressed size minus the 12-byte header.
	compLen := int(f.CompressedSize64) - encryptionHeaderSize

	switch f.Method {
	case zip.Store:
		prep.Stored = true
		prep.Plaintext = rawFile
		prep.LengthMatch = len(rawFile) == compLen
		prep.Ready = prep.CRCMatch && prep.LengthMatch && len(rawFile) >= minKnownPlaintext
		prep.Message = storeMessage(prep, len(rawFile), compLen)
		return prep, nil

	case zip.Deflate:
		best, matched, err := bestDeflate(rawFile, compLen)
		if err != nil {
			return PlaintextPrep{}, apperr.New(apperr.InvalidInput, "压缩已知明文时出错")
		}
		prep.Plaintext = best
		prep.LengthMatch = matched
		prep.Ready = prep.CRCMatch && matched && len(best) >= minKnownPlaintext
		prep.Message = deflateMessage(prep, len(best), compLen)
		return prep, nil

	default:
		return PlaintextPrep{}, apperr.New(apperr.Unsupported,
			fmt.Sprintf("暂不支持的压缩方法：%d", f.Method))
	}
}

// bestDeflate re-compresses data at several levels and returns the candidate
// whose length matches wantLen, preferring an exact length match. When none
// match, it returns the default-level output with matched=false.
func bestDeflate(data []byte, wantLen int) (best []byte, matched bool, err error) {
	for _, lvl := range deflateLevels {
		out, e := rawDeflate(data, lvl)
		if e != nil {
			return nil, false, e
		}
		if best == nil {
			best = out // fallback: first (level 6 / default)
		}
		if len(out) == wantLen {
			return out, true, nil
		}
	}
	return best, false, nil
}

func storeMessage(p PlaintextPrep, gotLen, wantLen int) string {
	if !p.CRCMatch {
		return "⚠️ 提供的文件 CRC-32 与该条目不一致，可能不是同一个文件。仍可尝试，但很可能无法还原密钥。"
	}
	if gotLen < minKnownPlaintext {
		return fmt.Sprintf("文件校验一致，但内容仅 %d 字节，攻击至少需要 %d 字节。", gotLen, minKnownPlaintext)
	}
	return "✅ 文件校验一致，且为未压缩（Store）条目，可直接开始攻击。"
}

func deflateMessage(p PlaintextPrep, gotLen, wantLen int) string {
	switch {
	case !p.CRCMatch:
		return "⚠️ 提供的文件 CRC-32 与该条目不一致，可能不是同一个文件。请确认选对了原始文件。"
	case p.LengthMatch:
		return "✅ 文件校验一致，且自动压缩后长度与包内一致，可直接开始攻击。"
	default:
		return fmt.Sprintf(
			"文件校验一致，但自动压缩结果（%d 字节）与包内压缩数据（%d 字节）长度不符——"+
				"原包可能用了不同的压缩实现。仍可尝试攻击；若失败，请改用「手动」方式提供与原包相同方式压缩后的字节。",
			gotLen, wantLen)
	}
}
