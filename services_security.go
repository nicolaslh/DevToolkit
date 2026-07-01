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

	kpaMu   sync.Mutex
	kpaJobs map[string]*crackx.KPAJob

	pwMu   sync.Mutex
	pwJobs map[string]*crackx.PasswordJob
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

// PrepareKnownPlaintext turns a user-supplied original file into the known
// plaintext the attack needs, verifying it via CRC-32 and, for Deflate entries,
// re-compressing it to match the archive. The returned Plaintext (and Offset)
// can be passed straight to StartKnownPlaintextAttack.
func (s *SecurityService) PrepareKnownPlaintext(data []byte, entryName string, rawFile []byte) (crackx.PlaintextPrep, error) {
	return crackx.PrepareKnownPlaintext(data, entryName, rawFile)
}

// StartKnownPlaintextAttack launches a Biham–Kocher known-plaintext attack
// against a ZipCrypto entry. plaintext is the known (compressed) bytes of the
// target entry starting at offset within its data. It returns a job id to poll
// with KnownPlaintextProgress; on success the recovered keys decrypt every
// ZipCrypto entry in the archive without the password. Only legacy ZipCrypto is
// vulnerable — AES entries are rejected.
func (s *SecurityService) StartKnownPlaintextAttack(data []byte, entryName string, plaintext []byte, offset int) (string, error) {
	job, err := crackx.StartKPA(data, entryName, plaintext, offset, 0)
	if err != nil {
		return "", err
	}
	id, err := newJobID()
	if err != nil {
		return "", err
	}
	s.kpaMu.Lock()
	if s.kpaJobs == nil {
		s.kpaJobs = map[string]*crackx.KPAJob{}
	}
	s.kpaJobs[id] = job
	s.kpaMu.Unlock()
	return id, nil
}

// KnownPlaintextProgress returns the latest progress snapshot for a KPA job.
func (s *SecurityService) KnownPlaintextProgress(id string) (crackx.KPAProgress, error) {
	s.kpaMu.Lock()
	job := s.kpaJobs[id]
	s.kpaMu.Unlock()
	if job == nil {
		return crackx.KPAProgress{}, apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	return job.Snapshot(), nil
}

// KnownPlaintextResult returns the recovered keys and decrypted entries once a
// KPA job has finished successfully.
func (s *SecurityService) KnownPlaintextResult(id string) ([]crackx.DecryptedEntry, error) {
	s.kpaMu.Lock()
	job := s.kpaJobs[id]
	s.kpaMu.Unlock()
	if job == nil {
		return nil, apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	_, entries, err := job.Result()
	return entries, err
}

// CancelKnownPlaintextAttack stops a running KPA job and forgets it.
func (s *SecurityService) CancelKnownPlaintextAttack(id string) error {
	s.kpaMu.Lock()
	job := s.kpaJobs[id]
	delete(s.kpaJobs, id)
	s.kpaMu.Unlock()
	if job == nil {
		return apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	job.Cancel()
	return nil
}

// StartPasswordRecovery attempts to find a password that yields the keys
// recovered by a finished known-plaintext attack (kpaId). Recovery is optional:
// decryption already works with the keys alone. It returns a separate job id to
// poll with PasswordRecoveryProgress.
func (s *SecurityService) StartPasswordRecovery(kpaID string, charset crackx.Charset, minLen, maxLen int) (string, error) {
	s.kpaMu.Lock()
	kpaJob := s.kpaJobs[kpaID]
	s.kpaMu.Unlock()
	if kpaJob == nil {
		return "", apperr.New(apperr.InvalidInput, "攻击任务不存在或已结束")
	}
	keys, ok := kpaJob.Keys()
	if !ok {
		return "", apperr.New(apperr.InvalidInput, "尚未还原出密钥，无法反推密码")
	}

	job := crackx.StartPasswordRecovery(keys, charset, minLen, maxLen)
	id, err := newJobID()
	if err != nil {
		return "", err
	}
	s.pwMu.Lock()
	if s.pwJobs == nil {
		s.pwJobs = map[string]*crackx.PasswordJob{}
	}
	s.pwJobs[id] = job
	s.pwMu.Unlock()
	return id, nil
}

// PasswordRecoveryProgress returns the latest snapshot for a password-recovery job.
func (s *SecurityService) PasswordRecoveryProgress(id string) (crackx.PasswordProgress, error) {
	s.pwMu.Lock()
	job := s.pwJobs[id]
	s.pwMu.Unlock()
	if job == nil {
		return crackx.PasswordProgress{}, apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	return job.Snapshot(), nil
}

// CancelPasswordRecovery stops a running password-recovery job and forgets it.
func (s *SecurityService) CancelPasswordRecovery(id string) error {
	s.pwMu.Lock()
	job := s.pwJobs[id]
	delete(s.pwJobs, id)
	s.pwMu.Unlock()
	if job == nil {
		return apperr.New(apperr.InvalidInput, "任务不存在或已结束")
	}
	job.Cancel()
	return nil
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
