package main

import (
	"github.com/nic/devtoolkit/internal/pkg/jwtx"
	"github.com/nic/devtoolkit/internal/pkg/pwdgen"
)

// SecurityService exposes JWT decoding (R15) and password/bcrypt generation (R16).
// All processing is local; plaintext and generated secrets are never logged.
type SecurityService struct{}

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
