package crackx

import (
	"bytes"
	"io"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/yeka/zip"
)

// zipVerifier validates passwords against an encrypted ZIP archive
// (ZipCrypto or AES). It keeps the raw bytes and re-opens the smallest
// encrypted entry per attempt, reading it fully so the CRC/auth tag is checked.
type zipVerifier struct {
	data   []byte
	target string // name of the entry used for verification
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
	return &zipVerifier{data: data, target: target}, nil
}

func (v *zipVerifier) Verify(password string) bool {
	r, err := zip.NewReader(bytes.NewReader(v.data), int64(len(v.data)))
	if err != nil {
		return false
	}
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
