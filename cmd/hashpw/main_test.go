package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/auth"
)

func TestRunUsage(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"hashpw"}, &out)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(out.String(), "Usage: hashpw <password>") {
		t.Fatalf("expected usage output, got %q", out.String())
	}
}

func TestRunHashesPassword(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"hashpw", "this-is-a-test-password"}, &out)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (output: %q)", code, out.String())
	}

	hash := strings.TrimSpace(out.String())
	if hash == "" {
		t.Fatalf("expected hash output, got empty string")
	}
	if !auth.CheckPasswordHash("this-is-a-test-password", hash) {
		t.Fatalf("expected hash to validate against original password")
	}
}
