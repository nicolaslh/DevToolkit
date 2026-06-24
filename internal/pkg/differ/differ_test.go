package differ

import "testing"

func TestIdentical(t *testing.T) {
	r, err := Diff("same\ntext", "same\ntext", Options{Mode: "line"})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Identical {
		t.Error("expected identical")
	}
}

func TestLineDiff(t *testing.T) {
	r, err := Diff("a\nb\nc\n", "a\nB\nc\n", Options{Mode: "line"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Identical {
		t.Error("expected differences")
	}
	hasDelete, hasInsert := false, false
	for _, s := range r.Left {
		if s.Type == "delete" {
			hasDelete = true
		}
	}
	for _, s := range r.Right {
		if s.Type == "insert" {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected delete on left and insert on right; left=%+v right=%+v", r.Left, r.Right)
	}
}

func TestEmptyInput(t *testing.T) {
	if _, err := Diff("", "x", Options{}); err == nil {
		t.Error("expected error for empty side")
	}
}

func TestJSONMode(t *testing.T) {
	// Same data, different key order/whitespace should be identical after normalization.
	left := `{"b":2,"a":1}`
	right := `{
  "a": 1,
  "b": 2
}`
	r, err := Diff(left, right, Options{JSONMode: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Identical {
		t.Errorf("expected identical after JSON normalization; left=%+v", r.Left)
	}
}

func TestJSONModeInvalid(t *testing.T) {
	if _, err := Diff("not json", "{}", Options{JSONMode: true}); err == nil {
		t.Error("expected parse error for invalid JSON")
	}
}
