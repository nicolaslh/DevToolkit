package cronx

import "testing"

func TestBuildDefaults(t *testing.T) {
	if got := Build(Fields{}); got != "* * * * *" {
		t.Errorf("got %q, want '* * * * *'", got)
	}
	if got := Build(Fields{Minute: "0", Hour: "9"}); got != "0 9 * * *" {
		t.Errorf("got %q", got)
	}
}

func TestParseValid(t *testing.T) {
	info, err := Parse("0 9 * * 1")
	if err != nil {
		t.Fatal(err)
	}
	if len(info.NextRuns) != 5 {
		t.Errorf("expected 5 next runs, got %d", len(info.NextRuns))
	}
	if info.Description == "" {
		t.Error("expected a description")
	}
}

func TestParseInvalid(t *testing.T) {
	for _, e := range []string{"", "* * *", "60 * * * *", "* * * * 9 9 9"} {
		if _, err := Parse(e); err == nil {
			t.Errorf("expected error for %q", e)
		}
	}
}

// Property 7: Build(Parse fields) semantic equivalence — building from a parsed
// expression's fields yields a schedule with identical next-run sequence (R9.6).
func TestRoundTripSemantic(t *testing.T) {
	exprs := []string{"0 9 * * 1", "*/15 * * * *", "0 0 1 * *", "30 6 * * 1-5"}
	for _, e := range exprs {
		i1, err := Parse(e)
		if err != nil {
			t.Fatalf("parse %q: %v", e, err)
		}
		// Rebuild from the same field tokens, then parse again.
		i2, err := Parse(Build(splitFields(e)))
		if err != nil {
			t.Fatalf("re-parse %q: %v", e, err)
		}
		for j := range i1.NextRuns {
			if i1.NextRuns[j] != i2.NextRuns[j] {
				t.Errorf("run %d mismatch for %q: %s vs %s", j, e, i1.NextRuns[j], i2.NextRuns[j])
			}
		}
	}
}

func splitFields(expr string) Fields {
	p := []string{"*", "*", "*", "*", "*"}
	fields := []string{}
	cur := ""
	for _, c := range expr {
		if c == ' ' {
			if cur != "" {
				fields = append(fields, cur)
				cur = ""
			}
			continue
		}
		cur += string(c)
	}
	if cur != "" {
		fields = append(fields, cur)
	}
	for i := 0; i < len(fields) && i < 5; i++ {
		p[i] = fields[i]
	}
	return Fields{Minute: p[0], Hour: p[1], Day: p[2], Month: p[3], Weekday: p[4]}
}
