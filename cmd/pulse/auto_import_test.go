package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestShouldAutoImport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)

	// No env vars => false.
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")
	if shouldAutoImport() {
		t.Fatalf("expected false when no env vars set")
	}

	// Env var set => true.
	t.Setenv("PULSE_INIT_CONFIG_DATA", "anything")
	if !shouldAutoImport() {
		t.Fatalf("expected true when init config env var set")
	}

	// Existing config should disable auto-import.
	if err := os.WriteFile(filepath.Join(dir, "nodes.enc"), []byte("exists"), 0o600); err != nil {
		t.Fatalf("write nodes.enc: %v", err)
	}
	if shouldAutoImport() {
		t.Fatalf("expected false when nodes.enc exists")
	}
}

func TestPerformAutoImport_ValidPayload(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)

	pass := "test-passphrase"
	cp := config.NewConfigPersistence(dir)
	exported, err := cp.ExportConfig(pass)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}

	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", pass)
	t.Setenv("PULSE_INIT_CONFIG_DATA", exported)
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")

	if err := performAutoImport(); err != nil {
		t.Fatalf("performAutoImport: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "nodes.enc")); err != nil {
		t.Fatalf("expected nodes.enc to exist: %v", err)
	}
}

func TestPerformAutoImport_MissingPassphrase(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "")
	t.Setenv("PULSE_INIT_CONFIG_DATA", "data")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")

	if err := performAutoImport(); err == nil {
		t.Fatalf("expected error")
	}
}

func TestPerformAutoImport_MissingData(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dir)
	t.Setenv("PULSE_INIT_CONFIG_PASSPHRASE", "pass")
	t.Setenv("PULSE_INIT_CONFIG_DATA", "")
	t.Setenv("PULSE_INIT_CONFIG_FILE", "")

	if err := performAutoImport(); err == nil {
		t.Fatalf("expected error")
	}
}
