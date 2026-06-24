package jwtx

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func makeJWT(header, payload string) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(header))
	p := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return h + "." + p + ".sig"
}

func TestDecodeValid(t *testing.T) {
	tok := makeJWT(`{"alg":"HS256","typ":"JWT"}`, `{"sub":"123","name":"Alice"}`)
	res, err := Decode(tok)
	if err != nil {
		t.Fatal(err)
	}
	if res.Header == "" || res.Payload == "" {
		t.Error("expected formatted header and payload")
	}
	if res.Expired != nil {
		t.Error("expected nil Expired when no exp")
	}
}

func TestDecodeExpired(t *testing.T) {
	past := time.Now().Add(-time.Hour).Unix()
	tok := makeJWT(`{"alg":"HS256"}`, fmt.Sprintf(`{"exp":%d}`, past))
	res, err := Decode(tok)
	if err != nil {
		t.Fatal(err)
	}
	if res.Expired == nil || !*res.Expired {
		t.Error("expected token to be marked expired")
	}
	if res.ExpHuman == "" {
		t.Error("expected human-readable exp")
	}
}

func TestDecodeNotExpired(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	tok := makeJWT(`{"alg":"HS256"}`, fmt.Sprintf(`{"exp":%d}`, future))
	res, err := Decode(tok)
	if err != nil {
		t.Fatal(err)
	}
	if res.Expired == nil || *res.Expired {
		t.Error("expected token to be marked not expired")
	}
}

func TestDecodeStructureError(t *testing.T) {
	if _, err := Decode("only.two"); err == nil {
		t.Error("expected structure error")
	}
	if _, err := Decode(""); err == nil {
		t.Error("expected empty error")
	}
}

func TestDecodeBadJSON(t *testing.T) {
	tok := makeJWT("not-json", "also-not-json")
	if _, err := Decode(tok); err == nil {
		t.Error("expected JSON decode error")
	}
}
