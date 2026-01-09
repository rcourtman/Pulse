package main

import (
	"encoding/base64"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/server"
)

func TestNormalizeImportPayload_Base64Passthrough(t *testing.T) {
	raw := []byte(" Zm9vYmFy \n") // base64("foobar")
	out, err := server.NormalizeImportPayload(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != "Zm9vYmFy" {
		t.Fatalf("out = %q", out)
	}
}

func TestNormalizeImportPayload_Base64OfBase64Unwrap(t *testing.T) {
	inner := "Zm9vYmFy" // base64("foobar")
	outer := base64.StdEncoding.EncodeToString([]byte(inner))
	out, err := server.NormalizeImportPayload([]byte(outer))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if out != inner {
		t.Fatalf("out = %q, want %q", out, inner)
	}
}

func TestNormalizeImportPayload_RawBytesGetEncoded(t *testing.T) {
	raw := []byte{0x00, 0x01, 0x02, 0xff, 0x10}
	out, err := server.NormalizeImportPayload(raw)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("DecodeString: %v", err)
	}
	if string(decoded) != string(raw) {
		t.Fatalf("roundtrip mismatch")
	}
}

func TestLooksLikeBase64(t *testing.T) {
	if server.LooksLikeBase64("") {
		t.Fatalf("empty should be false")
	}
	if !server.LooksLikeBase64("Zm9vYmFy") {
		t.Fatalf("expected base64 true")
	}
	if server.LooksLikeBase64("not-base64!!!") {
		t.Fatalf("expected base64 false")
	}
}
