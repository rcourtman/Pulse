package pulsecli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPassphraseFromEnvAndConfigDeps(t *testing.T) {
	t.Setenv("PULSE_PASSPHRASE", "from-env")
	flagValue := "from-flag"
	config := &ConfigDeps{Passphrase: &flagValue}
	if got := GetPassphrase(config, "ignored", false); got != "from-env" {
		t.Fatalf("GetPassphrase() = %q, want from-env", got)
	}

	t.Setenv("PULSE_PASSPHRASE", "")
	if got := GetPassphrase(config, "ignored", false); got != "from-flag" {
		t.Fatalf("GetPassphrase() = %q, want from-flag", got)
	}
}

func TestReadBoundedRegularFileRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.enc")
	if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	link := filepath.Join(dir, "config.enc")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	if _, err := ReadBoundedRegularFile(link, 1024); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("ReadBoundedRegularFile() error = %v, want regular file rejection", err)
	}
}

func TestReadBoundedHTTPBodyRejectsOversizedStream(t *testing.T) {
	_, err := ReadBoundedHTTPBody(bytes.NewReader([]byte("0123456789")), -1, 8, "configuration response")
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("ReadBoundedHTTPBody() error = %v, want exceeds error", err)
	}
}

func TestGetPassphraseInteractiveScenarios(t *testing.T) {
	readCalls := 0
	deps := &ConfigDeps{
		ReadPassword: func(fd int) ([]byte, error) {
			readCalls++
			return []byte("interactive"), nil
		},
	}

	if got := GetPassphrase(deps, "ignored", false); got != "interactive" {
		t.Fatalf("GetPassphrase() = %q, want interactive", got)
	}

	readCalls = 0
	deps.ReadPassword = func(fd int) ([]byte, error) {
		readCalls++
		return []byte("match"), nil
	}
	if got := GetPassphrase(deps, "ignored", true); got != "match" {
		t.Fatalf("GetPassphrase(confirm) = %q, want match", got)
	}
	if readCalls != 2 {
		t.Fatalf("readCalls = %d, want 2", readCalls)
	}

	readCalls = 0
	deps.ReadPassword = func(fd int) ([]byte, error) {
		readCalls++
		if readCalls == 1 {
			return []byte("first"), nil
		}
		return []byte("second"), nil
	}
	if got := GetPassphrase(deps, "ignored", true); got != "" {
		t.Fatalf("GetPassphrase(mismatch) = %q, want empty", got)
	}

	deps.ReadPassword = func(fd int) ([]byte, error) {
		return nil, fmt.Errorf("read error")
	}
	if got := GetPassphrase(deps, "ignored", false); got != "" {
		t.Fatalf("GetPassphrase(error) = %q, want empty", got)
	}
}
