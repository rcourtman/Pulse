package pulsecli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPassphraseFromEnvAndRuntime(t *testing.T) {
	t.Setenv("PULSE_PASSPHRASE", "from-env")
	flagValue := "from-flag"
	runtime := &Runtime{Config: ConfigRuntime{Passphrase: &flagValue}}
	if got := GetPassphrase(runtime, "ignored", false); got != "from-env" {
		t.Fatalf("GetPassphrase() = %q, want from-env", got)
	}

	t.Setenv("PULSE_PASSPHRASE", "")
	if got := GetPassphrase(runtime, "ignored", false); got != "from-flag" {
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
