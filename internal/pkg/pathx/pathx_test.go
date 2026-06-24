package pathx

import (
	"strings"
	"testing"
)

func TestSplitAndJoin(t *testing.T) {
	sep := Separator()
	raw := strings.Join([]string{"/usr/bin", "/bin", "/usr/local/bin"}, sep)
	entries, err := Split(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries", len(entries))
	}
	if Join(entries) != raw {
		t.Errorf("join mismatch: %q", Join(entries))
	}
}

func TestSplitSkipsEmpty(t *testing.T) {
	sep := Separator()
	raw := sep + "/usr/bin" + sep + sep + "/bin" + sep
	entries, _ := Split(raw)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(entries), entries)
	}
}

func TestDedup(t *testing.T) {
	entries := []string{"/usr/bin", "/bin", "/usr/bin", "/usr/local/bin", "/bin"}
	out := Dedup(entries)
	if len(out) != 3 {
		t.Errorf("expected 3 unique, got %d: %v", len(out), out)
	}
	if out[0] != "/usr/bin" || out[1] != "/bin" || out[2] != "/usr/local/bin" {
		t.Errorf("order not preserved: %v", out)
	}
}

func TestCompare(t *testing.T) {
	sep := Separator()
	a := strings.Join([]string{"/usr/bin", "/bin"}, sep)
	b := strings.Join([]string{"/bin", "/opt/bin"}, sep)
	diff, err := Compare(a, b)
	if err != nil {
		t.Fatal(err)
	}
	states := map[string]string{}
	for _, d := range diff {
		states[d.Path] = d.State
	}
	if states["/usr/bin"] != "left" || states["/bin"] != "both" || states["/opt/bin"] != "right" {
		t.Errorf("unexpected states: %+v", states)
	}
}
