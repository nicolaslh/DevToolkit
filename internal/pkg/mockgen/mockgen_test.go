package mockgen

import (
	"regexp"
	"testing"
)

func TestGenerateCount(t *testing.T) {
	out, err := Generate(Name, 100)
	if err != nil || len(out) != 100 {
		t.Fatalf("got %d, %v", len(out), err)
	}
}

func TestGenerateCountBounds(t *testing.T) {
	if _, err := Generate(Name, 0); err == nil {
		t.Error("expected error for 0")
	}
	if _, err := Generate(Name, MaxCount+1); err == nil {
		t.Error("expected error for over max")
	}
}

func TestGenerateUnknownKind(t *testing.T) {
	if _, err := Generate(Kind("nope"), 1); err == nil {
		t.Error("expected error for unknown kind")
	}
}

func TestPhoneFormat(t *testing.T) {
	re := regexp.MustCompile(`^1\d{10}$`)
	out, _ := Generate(Phone, 200)
	for _, p := range out {
		if !re.MatchString(p) {
			t.Errorf("invalid phone: %q", p)
		}
	}
}

func TestEmailFormat(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+@[a-z0-9.]+\.[a-z]{2,}$`)
	out, _ := Generate(Email, 100)
	for _, e := range out {
		if !re.MatchString(e) {
			t.Errorf("invalid email: %q", e)
		}
	}
}

func TestBankCardLuhn(t *testing.T) {
	out, _ := Generate(BankCard, 300)
	for _, c := range out {
		if len(c) != 16 {
			t.Errorf("card not 16 digits: %q", c)
		}
		if !LuhnValid(c) {
			t.Errorf("card fails Luhn: %q", c)
		}
	}
}
