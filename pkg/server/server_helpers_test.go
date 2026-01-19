package server

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestShouldAutoImport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")

	if ShouldAutoImport() {
		t.Fatal("expected auto-import false without config")
	}

	t.Setenv("PULSE_INIT_CONFIG_DATA", "payload")
	if !ShouldAutoImport() {
		t.Fatal("expected auto-import true with data")
	}

	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "/tmp/file")
	if !ShouldAutoImport() {
		t.Fatal("expected auto-import true with file")
	}

	file := filepath.Join(dir, "nodes.enc")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatalf("write nodes: %v", err)
	}
	if ShouldAutoImport() {
		t.Fatal("expected auto-import false when nodes.enc exists")
	}
}

func TestNormalizeImportPayload(t *testing.T) {
	if _, err := NormalizeImportPayload([]byte("")); err == nil {
		t.Fatal("expected error for empty payload")
	}

	raw := []byte("hello")
	encoded := base64.StdEncoding.EncodeToString(raw)
	out, err := NormalizeImportPayload([]byte(encoded))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if out != encoded {
		t.Fatalf("unexpected output: %s", out)
	}

	double := base64.StdEncoding.EncodeToString([]byte(encoded))
	out, err = NormalizeImportPayload([]byte(double))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if out != encoded {
		t.Fatalf("unexpected output: %s", out)
	}

	out, err = NormalizeImportPayload([]byte("not-base64"))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if out == "not-base64" {
		t.Fatal("expected payload to be encoded")
	}
}

func TestLooksLikeBase64(t *testing.T) {
	if LooksLikeBase64("") {
		t.Fatal("expected false for empty")
	}
	if !LooksLikeBase64("aGVsbG8=") {
		t.Fatal("expected true for base64")
	}
	if LooksLikeBase64("nope!!") {
		t.Fatal("expected false for invalid")
	}
}

func TestPerformAutoImportErrors(t *testing.T) {
	t.Setenv("PULSE_INIT_CONFIG_DATA", "data")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "")
	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error without passphrase")
	}

	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "pass")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "/tmp/missing-file")
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error for missing file")
	}

	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	if err := PerformAutoImport(); err == nil {
		t.Fatal("expected error for missing data")
	}
}
