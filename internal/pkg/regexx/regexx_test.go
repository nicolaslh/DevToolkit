package regexx

import "testing"

func TestBasicMatch(t *testing.T) {
	r, err := Test(`\d+`, "a1b22c333", Flags{Global: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count != 3 {
		t.Fatalf("got %d matches, want 3", r.Count)
	}
	if r.Matches[2].Text != "333" || r.Matches[2].Start != 6 {
		t.Errorf("unexpected match: %+v", r.Matches[2])
	}
}

func TestGroups(t *testing.T) {
	r, err := Test(`(\w+)@(\w+)`, "user@host", Flags{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Count != 1 || len(r.Matches[0].Groups) != 2 {
		t.Fatalf("unexpected: %+v", r)
	}
	if r.Matches[0].Groups[0] != "user" || r.Matches[0].Groups[1] != "host" {
		t.Errorf("groups: %v", r.Matches[0].Groups)
	}
}

func TestNonGlobalStopsAtFirst(t *testing.T) {
	r, _ := Test(`\d`, "123", Flags{Global: false})
	if r.Count != 1 {
		t.Errorf("expected 1 match, got %d", r.Count)
	}
}

func TestIgnoreCase(t *testing.T) {
	r, _ := Test(`abc`, "ABC", Flags{IgnoreCase: true, Global: true})
	if r.Count != 1 {
		t.Errorf("expected case-insensitive match")
	}
}

func TestInvalidPattern(t *testing.T) {
	if _, err := Test(`(unclosed`, "x", Flags{}); err == nil {
		t.Error("expected syntax error")
	}
	if _, err := Test(``, "x", Flags{}); err == nil {
		t.Error("expected empty-pattern error")
	}
}

func TestLibraryNotEmpty(t *testing.T) {
	lib := Library()
	if len(lib) < 5 {
		t.Errorf("expected at least 5 presets, got %d", len(lib))
	}
}
