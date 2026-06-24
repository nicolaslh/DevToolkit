// Package cryptox implements hashing and symmetric AES-GCM encryption (R18).
package cryptox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// MaxInputBytes bounds hash/encrypt input (R18.1/18.2).
const MaxInputBytes = 10485760

// HashAlgo enumerates supported hash algorithms.
type HashAlgo string

const (
	MD5    HashAlgo = "md5"
	SHA1   HashAlgo = "sha1"
	SHA256 HashAlgo = "sha256"
)

// Hash returns the lowercase hex digest of input using the given algorithm (R18.1).
func Hash(input string, algo HashAlgo) (string, error) {
	if len(input) > MaxInputBytes {
		return "", apperr.Newf(apperr.TooLarge, "输入超出长度上限（最多 %d 字节）", MaxInputBytes)
	}
	switch algo {
	case MD5:
		sum := md5.Sum([]byte(input))
		return hex.EncodeToString(sum[:]), nil
	case SHA1:
		sum := sha1.Sum([]byte(input))
		return hex.EncodeToString(sum[:]), nil
	case SHA256:
		sum := sha256.Sum256([]byte(input))
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", apperr.New(apperr.InvalidInput, "不支持的哈希算法")
	}
}

func validKeyLen(n int) bool { return n == 16 || n == 24 || n == 32 }

// Encrypt performs AES-GCM encryption. The nonce is prepended to the ciphertext
// and the whole blob is Base64-encoded (R18.2).
func Encrypt(plain, key string) (string, error) {
	if !validKeyLen(len(key)) {
		return "", apperr.New(apperr.InvalidInput, "密钥长度无效：AES 密钥须为 16、24 或 32 字节")
	}
	if len(plain) > MaxInputBytes {
		return "", apperr.Newf(apperr.TooLarge, "明文超出长度上限（最多 %d 字节）", MaxInputBytes)
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "无法初始化加密：密钥无效")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "无法初始化加密模式")
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", apperr.New(apperr.InvalidInput, "无法生成随机 nonce")
	}
	out := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// Decrypt reverses Encrypt. A wrong key fails GCM authentication (R18.3/18.5).
func Decrypt(ciphertext, key string) (string, error) {
	if !validKeyLen(len(key)) {
		return "", apperr.New(apperr.InvalidInput, "密钥长度无效：AES 密钥须为 16、24 或 32 字节")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", apperr.New(apperr.ParseError, "密文不是合法的 Base64")
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "无法初始化解密：密钥无效")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "无法初始化解密模式")
	}
	if len(raw) < gcm.NonceSize() {
		return "", apperr.New(apperr.ParseError, "密文格式不正确")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "解密失败：密钥不正确或密文已被篡改")
	}
	return string(plain), nil
}
