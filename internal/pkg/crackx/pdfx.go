package crackx

import (
	"bytes"
	"errors"
	"io"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

func init() {
	// Never touch the user's pdfcpu config directory.
	model.ConfigPath = "disable"
}

// pdfVerifier validates passwords against an encrypted PDF. pdfcpu decrypts
// while reading, so a successful validation means the password is correct.
type pdfVerifier struct {
	data []byte
}

func newPDFVerifier(data []byte) (Verifier, error) {
	// Probe with an empty password to confirm the document is encrypted.
	err := validatePDF(data, "")
	if err == nil {
		return nil, apperr.New(apperr.Unsupported, "该 PDF 未加密或使用空口令，无需破解")
	}
	if !errors.Is(err, pdfcpu.ErrWrongPassword) {
		return nil, apperr.New(apperr.ParseError, "无法读取 PDF 文件：可能已损坏或不是有效的 PDF")
	}
	return &pdfVerifier{data: data}, nil
}

func (v *pdfVerifier) Verify(password string) bool {
	return validatePDF(v.data, password) == nil
}

func validatePDF(data []byte, password string) error {
	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password
	conf.ValidationMode = model.ValidationRelaxed
	var rs io.ReadSeeker = bytes.NewReader(data)
	return api.Validate(rs, conf)
}
