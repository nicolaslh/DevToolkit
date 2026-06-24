package chmod

import (
	"fmt"
	"testing"
)

func TestFromNumeric(t *testing.T) {
	r, err := FromNumeric("755")
	if err != nil {
		t.Fatal(err)
	}
	if r.Symbolic != "rwxr-xr-x" {
		t.Errorf("got %q, want rwxr-xr-x", r.Symbolic)
	}
	r, _ = FromNumeric("644")
	if r.Symbolic != "rw-r--r--" {
		t.Errorf("got %q, want rw-r--r--", r.Symbolic)
	}
}

func TestFromNumericPadding(t *testing.T) {
	r, err := FromNumeric("7")
	if err != nil {
		t.Fatal(err)
	}
	if r.Numeric != "007" || r.Symbolic != "------rwx" {
		t.Errorf("got %q/%q", r.Numeric, r.Symbolic)
	}
}

func TestFromNumericErrors(t *testing.T) {
	for _, s := range []string{"", "8", "999", "1234", "7a"} {
		if _, err := FromNumeric(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

// Property 3: numeric -> symbolic/perms -> numeric round-trips for all 000-777 (R10.4).
func TestRoundTrip(t *testing.T) {
	for o := 0; o <= 7; o++ {
		for g := 0; g <= 7; g++ {
			for ot := 0; ot <= 7; ot++ {
				num := fmt.Sprintf("%d%d%d", o, g, ot)
				r, err := FromNumeric(num)
				if err != nil {
					t.Fatalf("FromNumeric(%s): %v", num, err)
				}
				back := FromPerms(r.Perms)
				if back.Numeric != num {
					t.Errorf("round-trip mismatch: %s -> %s", num, back.Numeric)
				}
			}
		}
	}
}
