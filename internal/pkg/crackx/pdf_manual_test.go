package crackx

import (
	"bytes"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// minimalPDF is a tiny single-page PDF used to build an encrypted fixture.
const minimalPDF = `%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj
xref
0 4
0000000000 65535 f 
0000000009 00000 n 
0000000052 00000 n 
0000000101 00000 n 
trailer<</Size 4/Root 1 0 R>>
startxref
164
%%EOF`

func TestPDFCrack(t *testing.T) {
	// Validate the base PDF parses.
	conf := model.NewDefaultConfiguration()
	if err := api.Validate(bytes.NewReader([]byte(minimalPDF)), conf); err != nil {
		t.Skipf("base PDF not accepted by pdfcpu, skipping: %v", err)
	}

	// Encrypt it with a user password.
	encConf := model.NewDefaultConfiguration()
	encConf.UserPW = "x9"
	encConf.OwnerPW = "x9"
	var enc bytes.Buffer
	if err := api.Encrypt(bytes.NewReader([]byte(minimalPDF)), &enc, encConf); err != nil {
		t.Skipf("encrypt failed, skipping: %v", err)
	}

	v, err := NewVerifier(FormatPDF, "doc.pdf", enc.Bytes())
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	job := Start(v, Options{Mode: ModeBrute, MinLen: 2, MaxLen: 2, Charset: Charset{Lower: true, Digits: true}}, 0)
	p := waitDone(t, job)
	if !p.Found || p.Password != "x9" {
		t.Fatalf("expected to crack x9, got %+v", p)
	}
}
