package curlconv

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	req, err := Parse(`curl https://api.example.com/users`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "GET" || req.URL != "https://api.example.com/users" {
		t.Errorf("unexpected: %+v", req)
	}
}

func TestParseWithHeadersAndBody(t *testing.T) {
	cmd := `curl -X POST https://api.example.com/login -H "Content-Type: application/json" -H "Accept: */*" -d '{"u":"a"}'`
	req, err := Parse(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Errorf("method = %q", req.Method)
	}
	if len(req.Headers) != 2 || req.Headers[0].Key != "Content-Type" {
		t.Errorf("headers = %+v", req.Headers)
	}
	if req.Body != `{"u":"a"}` {
		t.Errorf("body = %q", req.Body)
	}
}

func TestParseImplicitPost(t *testing.T) {
	req, err := Parse(`curl https://x.test -d hello`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Method != "POST" {
		t.Errorf("expected implicit POST, got %q", req.Method)
	}
}

func TestParseInvalid(t *testing.T) {
	if _, err := Parse(`wget https://x.test`); err == nil {
		t.Error("expected error for non-curl command")
	}
	if _, err := Parse(`curl -X POST`); err == nil {
		t.Error("expected error when no URL")
	}
}

func TestConvertAllLanguages(t *testing.T) {
	cmd := `curl -X POST https://api.example.com/login -H "Content-Type: application/json" -d '{"u":"a"}'`
	for _, lang := range []string{"python", "javascript", "go", "java"} {
		code, err := Convert(cmd, lang)
		if err != nil {
			t.Fatalf("convert %s: %v", lang, err)
		}
		if !strings.Contains(code, "api.example.com/login") {
			t.Errorf("%s output missing URL", lang)
		}
		if !strings.Contains(code, "application/json") {
			t.Errorf("%s output missing header", lang)
		}
	}
}

func TestConvertUnsupported(t *testing.T) {
	if _, err := Convert(`curl https://x.test`, "ruby"); err == nil {
		t.Error("expected error for unsupported language")
	}
}
