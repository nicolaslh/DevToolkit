package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

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

// InspectZip reports the encryption scheme of each entry in a ZIP archive
// without a password. The frontend uses this to tell ZipCrypto (vulnerable to
// a known-plaintext attack) apart from AES (which is not) before offering an
// attack mode.
func (s *SecurityService) InspectZip(data []byte) (crackx.ZipInfo, error) {
	return crackx.DetectZipEncryption(data)
}

// StartCrack begins an offline password-recovery job over the supplied file
// bytes and returns a job id used to poll progress. The attack runs in the
// background so the UI stays responsive. When resume is true and a matching
// checkpoint exists, the job continues from where a previous run stopped.
func (s *SecurityService) StartCrack(data []byte, filename string, opts crackx.Options, resume bool) (string, error) {
	verifier, err := crackx.NewVerifier(opts.Format, filename, data)
	if err != nil {
		return "", err
	}

	fingerprint := crackx.Fingerprint(data, opts)
	var resumeFrom int64
	if resume {
		if cp, ok := crackx.LoadCheckpoint(fingerprint); ok {
			resumeFrom = cp.Tried
		}
	} else {
		crackx.DeleteCheckpoint(fingerprint)
	}

	job := crackx.Start(verifier, opts, resumeFrom)

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

	go persistCheckpoints(job, fingerprint)
	return id, nil
}

// Resumable reports whether a saved checkpoint exists for the given file and
// options, so the UI can offer to continue a previously interrupted run.
func (s *SecurityService) Resumable(data []byte, filename string, opts crackx.Options) (crackx.ResumeInfo, error) {
	fingerprint := crackx.Fingerprint(data, opts)
	if cp, ok := crackx.LoadCheckpoint(fingerprint); ok && cp.Tried > 0 {
		return crackx.ResumeInfo{Available: true, Tried: cp.Tried, Total: cp.Total}, nil
	}
	return crackx.ResumeInfo{}, nil
}

// persistCheckpoints periodically saves the job's progress while it runs. A
// checkpoint is kept when the job is interrupted/canceled (so it can resume)
// and removed once the password is found or the whole space is exhausted.
func persistCheckpoints(job *crackx.Job, fingerprint string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		p := job.Snapshot()
		offset := job.ResumeOffset()
		if p.Done {
			if p.Canceled {
				_ = crackx.SaveCheckpoint(crackx.Checkpoint{
					Fingerprint: fingerprint, Tried: offset, Total: p.Total,
				})
			} else {
				crackx.DeleteCheckpoint(fingerprint)
			}
			return
		}
		_ = crackx.SaveCheckpoint(crackx.Checkpoint{
			Fingerprint: fingerprint, Tried: offset, Total: p.Total,
		})
	}
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
