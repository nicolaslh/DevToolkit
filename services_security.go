package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"github.com/nic/devtoolkit/internal/pkg/crackx"
	"github.com/nic/devtoolkit/internal/pkg/jwtx"
	"github.com/nic/devtoolkit/internal/pkg/pwdgen"
)

// SecurityService exposes JWT decoding (R15), password/bcrypt generation (R16)
// and offline password recovery for encrypted ZIP/PDF/Office files (R17).
// All processing is local; plaintext and generated secrets are never logged.
type SecurityService struct {
	crackMu   sync.Mutex
	crackJobs map[string]*crackx.Job
}

// DecodeJWT decodes (without signature verification) and formats a JWT.
func (s *SecurityService) DecodeJWT(token string) (jwtx.Result, error) {
	return jwtx.Decode(token)
}

// GeneratePasswords generates Count strong passwords per the given options.
func (s *SecurityService) GeneratePasswords(opts pwdgen.Options) ([]string, error) {
	return pwdgen.Generate(opts)
}

// BcryptHash returns the bcrypt hash of plain.
func (s *SecurityService) BcryptHash(plain string) (string, error) {
	return pwdgen.Bcrypt(plain)
}

// StartCrack begins an offline password-recovery job over the supplied file
// bytes and returns a job id used to poll progress. The attack runs in the
// background so the UI stays responsive.
func (s *SecurityService) StartCrack(data []byte, filename string, opts crackx.Options) (string, error) {
	verifier, err := crackx.NewVerifier(opts.Format, filename, data)
	if err != nil {
		return "", err
	}
	job := crackx.Start(verifier, opts)

	id, err := newJobID()
	if err != nil {
		return "", err
	}
	s.crackMu.Lock()
	if s.crackJobs == nil {
		s.crackJobs = map[string]*crackx.Job{}
	}
	s.crackJobs[id] = job
	s.crackMu.Unlock()
	return id, nil
}

// CrackProgress returns the latest progress snapshot for a job.
func (s *SecurityService) CrackProgress(id string) (crackx.Progress, error) {
	s.crackMu.Lock()
	job := s.crackJobs[id]
	s.crackMu.Unlock()
	if job == nil {
		return crackx.Progress{}, apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	return job.Snapshot(), nil
}

// CancelCrack stops a running job and forgets it.
func (s *SecurityService) CancelCrack(id string) error {
	s.crackMu.Lock()
	job := s.crackJobs[id]
	delete(s.crackJobs, id)
	s.crackMu.Unlock()
	if job == nil {
		return apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	job.Cancel()
	return nil
}

func newJobID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", apperr.New(apperr.InvalidInput, "无法创建任务")
	}
	return hex.EncodeToString(b[:]), nil
}
