// Package pwdgen generates strong passwords and bcrypt hashes (R16).
package pwdgen

import (
	"crypto/rand"
	"math/big"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
	"golang.org/x/crypto/bcrypt"
)

const (
	MinLen     = 4
	MaxLen     = 128
	MaxBatch   = 1000
	MaxBcrypt  = 72 // bcrypt input byte limit
	lowerChars = "abcdefghijklmnopqrstuvwxyz"
	upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars = "0123456789"
	symChars   = "!@#$%^&*()-_=+[]{};:,.<>?"
)

// Options configures password generation.
type Options struct {
	Upper  bool `json:"upper"`
	Lower  bool `json:"lower"`
	Digits bool `json:"digits"`
	Symbol bool `json:"symbol"`
	Length int  `json:"length"`
	Count  int  `json:"count"`
}

func selectedSets(o Options) []string {
	var sets []string
	if o.Upper {
		sets = append(sets, upperChars)
	}
	if o.Lower {
		sets = append(sets, lowerChars)
	}
	if o.Digits {
		sets = append(sets, digitChars)
	}
	if o.Symbol {
		sets = append(sets, symChars)
	}
	return sets
}

func randInt(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func shuffle(b []byte) error {
	for i := len(b) - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return err
		}
		b[i], b[j] = b[j], b[i]
	}
	return nil
}

// generateOne creates a single password guaranteeing at least one char per selected set.
func generateOne(sets []string, length int) (string, error) {
	pwd := make([]byte, 0, length)
	// Guarantee one character from each selected set.
	for _, set := range sets {
		idx, err := randInt(len(set))
		if err != nil {
			return "", err
		}
		pwd = append(pwd, set[idx])
	}
	// Fill the rest from the union of all selected sets.
	var all string
	for _, s := range sets {
		all += s
	}
	for len(pwd) < length {
		idx, err := randInt(len(all))
		if err != nil {
			return "", err
		}
		pwd = append(pwd, all[idx])
	}
	if err := shuffle(pwd); err != nil {
		return "", err
	}
	return string(pwd), nil
}

// Generate produces Count passwords per Options (R16.1–16.4).
func Generate(o Options) ([]string, error) {
	sets := selectedSets(o)
	if len(sets) == 0 {
		return nil, apperr.New(apperr.InvalidInput, "请至少启用一种字符集")
	}
	if o.Length < MinLen || o.Length > MaxLen {
		return nil, apperr.Newf(apperr.InvalidInput, "密码长度须介于 %d 至 %d 之间", MinLen, MaxLen)
	}
	if o.Length < len(sets) {
		return nil, apperr.Newf(apperr.InvalidInput, "密码长度须不小于所选字符集数量（%d）", len(sets))
	}
	if o.Count < 1 || o.Count > MaxBatch {
		return nil, apperr.Newf(apperr.InvalidInput, "生成数量须介于 1 至 %d 之间", MaxBatch)
	}

	seen := make(map[string]struct{}, o.Count)
	out := make([]string, 0, o.Count)
	// Bounded retries to ensure uniqueness without infinite loops.
	attempts := 0
	maxAttempts := o.Count*4 + 100
	for len(out) < o.Count && attempts < maxAttempts {
		attempts++
		p, err := generateOne(sets, o.Length)
		if err != nil {
			return nil, apperr.New(apperr.InvalidInput, "生成密码时发生错误")
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	if len(out) < o.Count {
		// Extremely small keyspace (e.g. length 4, single tiny set) — return what we have is wrong;
		// signal the caller instead.
		return nil, apperr.New(apperr.InvalidInput, "可用字符空间过小，无法生成足够多互不相同的密码，请增大长度或字符集")
	}
	return out, nil
}

// Bcrypt returns the bcrypt hash of plain (R16.5/16.6).
func Bcrypt(plain string) (string, error) {
	if len(plain) < 1 || len(plain) > MaxBcrypt {
		return "", apperr.Newf(apperr.InvalidInput, "明文长度须介于 1 至 %d 字节之间", MaxBcrypt)
	}
	h, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", apperr.New(apperr.InvalidInput, "生成 Bcrypt 哈希失败")
	}
	return string(h), nil
}
