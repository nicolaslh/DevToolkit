package urlparse

import "testing"

func TestParseBasic(t *testing.T) {
	p, err := Parse("https://example.com:8443/api/v1?a=1&b=two")
	if err != nil {
		t.Fatal(err)
	}
	if p.Protocol != "https" || p.Host != "example.com" || p.Port != "8443" || p.Path != "/api/v1" {
		t.Errorf("unexpected parts: %+v", p)
	}
	if len(p.Query) != 2 || p.Query[0].Key != "a" || p.Query[1].Value != "two" {
		t.Errorf("unexpected query: %+v", p.Query)
	}
}

func TestParseNoPort(t *testing.T) {
	p, err := Parse("http://localhost/path")
	if err != nil {
		t.Fatal(err)
	}
	if p.Port != "" {
		t.Errorf("expected empty port, got %q", p.Port)
	}
}

func TestParseInvalid(t *testing.T) {
	for _, s := range []string{"", "not a url", "/just/a/path", "example.com"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("expected error for %q", s)
		}
	}
}

// Property 4: Build(Parse(u)) is equivalent to u (R5.8).
func TestRoundTripEquivalent(t *testing.T) {
	urls := []string{
		"https://example.com:8443/api/v1?a=1&b=two",
		"http://localhost/path",
		"https://host.example/p/q?x=hello%20world&y=%26%3D",
		"https://a.b.c/?single=value",
		"https://a.b.c/nopath",
	}
	for _, raw := range urls {
		p1, err := Parse(raw)
		if err != nil {
			t.Fatalf("parse %q: %v", raw, err)
		}
		rebuilt, err := Build(p1)
		if err != nil {
			t.Fatalf("build %q: %v", raw, err)
		}
		p2, err := Parse(rebuilt)
		if err != nil {
			t.Fatalf("re-parse %q: %v", rebuilt, err)
		}
		if !Equivalent(p1, p2) {
			t.Errorf("not equivalent: %q -> %q", raw, rebuilt)
		}
	}
}
