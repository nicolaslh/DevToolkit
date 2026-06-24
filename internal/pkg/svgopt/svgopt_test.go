package svgopt

import "testing"

func TestOptimizeReduces(t *testing.T) {
	in := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
    <!-- a comment -->
    <rect x="0" y="0" width="100" height="100" fill="#ff0000" />
</svg>`
	r, err := Optimize(in)
	if err != nil {
		t.Fatal(err)
	}
	if r.AfterBytes > r.BeforeBytes {
		t.Errorf("expected size not to grow: %d -> %d", r.BeforeBytes, r.AfterBytes)
	}
	if r.Optimized == "" {
		t.Error("expected optimized output")
	}
	if r.ReductionPct == "" {
		t.Error("expected reduction percentage")
	}
}

func TestOptimizeInvalid(t *testing.T) {
	if _, err := Optimize("not svg at all"); err == nil {
		t.Error("expected error for non-SVG input")
	}
	if _, err := Optimize(""); err == nil {
		t.Error("expected error for empty input")
	}
}
