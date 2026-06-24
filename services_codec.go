package main

import (
	"github.com/nic/devtoolkit/internal/pkg/codecx"
	"github.com/nic/devtoolkit/internal/pkg/cryptox"
)

// CodecService exposes basic encoding/transcoding (R17) and hashing/symmetric
// encryption (R18) to the frontend. All processing is local.
type CodecService struct{}

// Encode encodes input using the given kind ("base64" | "url" | "hex").
func (s *CodecService) Encode(input string, kind string) (string, error) {
	return codecx.Encode(input, codecx.Kind(kind))
}

// Decode decodes input using the given kind ("base64" | "url" | "hex").
func (s *CodecService) Decode(input string, kind string) (string, error) {
	return codecx.Decode(input, codecx.Kind(kind))
}

// Hash returns the lowercase hex digest using algo ("md5" | "sha1" | "sha256").
func (s *CodecService) Hash(input string, algo string) (string, error) {
	return cryptox.Hash(input, cryptox.HashAlgo(algo))
}

// Encrypt performs AES-GCM encryption; the key must be 16, 24 or 32 bytes.
func (s *CodecService) Encrypt(plain string, key string) (string, error) {
	return cryptox.Encrypt(plain, key)
}

// Decrypt reverses Encrypt; a wrong key fails authentication.
func (s *CodecService) Decrypt(ciphertext string, key string) (string, error) {
	return cryptox.Decrypt(ciphertext, key)
}
