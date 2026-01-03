package main

import (
	"bytes"
	"errors"
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

func TestRunError(t *testing.T) {
	oldHashPassword := hashPassword
	defer func() { hashPassword = oldHashPassword }()

	hashPassword = func(password string) (string, error) {
		return "", errors.New("forced error")
	}

	var out bytes.Buffer
	code := run([]string{"hashpw", "password"}, &out)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(out.String(), "Error: forced error") {
		t.Fatalf("expected error message, got %q", out.String())
	}
}

func TestMain(t *testing.T) {
	oldOsExit := osExit
	oldOsArgs := osArgs
	oldStdout := stdout
	defer func() {
		osExit = oldOsExit
		osArgs = oldOsArgs
		stdout = oldStdout
	}()

	var exitCode int
	osExit = func(code int) {
		exitCode = code
	}
	osArgs = []string{"hashpw", "test-password"}
	var out bytes.Buffer
	stdout = &out

	main()

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if !auth.CheckPasswordHash("test-password", strings.TrimSpace(out.String())) {
		t.Fatalf("expected valid hash in output")
	}
}
