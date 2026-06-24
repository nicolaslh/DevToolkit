// Package chmod converts between numeric and symbolic Linux permissions (R10).
package chmod

import (
	"strconv"
	"strings"

	"github.com/nic/devtoolkit/internal/pkg/apperr"
)

// Perms represents the three permission triads.
type Perms struct {
	Owner Triad `json:"owner"`
	Group Triad `json:"group"`
	Other Triad `json:"other"`
}

// Triad is a single read/write/execute group.
type Triad struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
	Exec  bool `json:"exec"`
}

// Result bundles all representations.
type Result struct {
	Numeric  string `json:"numeric"`  // e.g. "755"
	Symbolic string `json:"symbolic"` // e.g. "rwxr-xr-x"
	Perms    Perms  `json:"perms"`
}

func triadDigit(t Triad) int {
	n := 0
	if t.Read {
		n += 4
	}
	if t.Write {
		n += 2
	}
	if t.Exec {
		n += 1
	}
	return n
}

func digitTriad(d int) Triad {
	return Triad{Read: d&4 != 0, Write: d&2 != 0, Exec: d&1 != 0}
}

func triadSymbol(t Triad) string {
	var b strings.Builder
	if t.Read {
		b.WriteByte('r')
	} else {
		b.WriteByte('-')
	}
	if t.Write {
		b.WriteByte('w')
	} else {
		b.WriteByte('-')
	}
	if t.Exec {
		b.WriteByte('x')
	} else {
		b.WriteByte('-')
	}
	return b.String()
}

// FromPerms builds a Result from checkbox state (R10.1).
func FromPerms(p Perms) Result {
	num := strconv.Itoa(triadDigit(p.Owner)) +
		strconv.Itoa(triadDigit(p.Group)) +
		strconv.Itoa(triadDigit(p.Other))
	sym := triadSymbol(p.Owner) + triadSymbol(p.Group) + triadSymbol(p.Other)
	return Result{Numeric: num, Symbolic: sym, Perms: p}
}

// FromNumeric parses a 1–3 digit octal permission value (R10.2/10.3).
func FromNumeric(numeric string) (Result, error) {
	numeric = strings.TrimSpace(numeric)
	if numeric == "" {
		return Result{}, apperr.New(apperr.InvalidInput, "权限值为空：请输入 1 至 3 位八进制数字")
	}
	if len(numeric) > 3 {
		return Result{}, apperr.New(apperr.InvalidInput, "权限值超出范围：最多 3 位八进制数字")
	}
	for _, c := range numeric {
		if c < '0' || c > '7' {
			return Result{}, apperr.New(apperr.InvalidInput, "包含非八进制数字：每位取值须为 0 至 7")
		}
	}
	// Left-pad to 3 digits.
	padded := strings.Repeat("0", 3-len(numeric)) + numeric
	o := int(padded[0] - '0')
	g := int(padded[1] - '0')
	ot := int(padded[2] - '0')
	p := Perms{Owner: digitTriad(o), Group: digitTriad(g), Other: digitTriad(ot)}
	return FromPerms(p), nil
}
