package dnsx

import "testing"

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"example.com":                         "example.com",
		"  Example.COM. ":                     "example.com",
		"https://example.com/path?q=1":        "example.com",
		"http://user:pass@example.com:8080/x": "example.com",
		"":                                    "",
		"   ":                                 "",
		"2001:db8::1":                         "2001:db8::1",
	}
	for in, want := range cases {
		if got := normalizeHost(in); got != want {
			t.Errorf("normalizeHost(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveInvalid(t *testing.T) {
	for _, s := range []string{"", "   ", "\t"} {
		if _, err := Resolve(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}
