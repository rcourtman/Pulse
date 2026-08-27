package config_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
)

func TestDeadManConfigEncryptedRoundTripAndRemoval(t *testing.T) {
	dir := t.TempDir()
	cp := config.NewConfigPersistence(dir)
	const endpoint = "https://watchdog.example.com/ping/credential-token"

	if err := cp.SaveDeadManConfig(notifications.DeadManConfig{PingURL: endpoint}); err != nil {
		t.Fatalf("SaveDeadManConfig: %v", err)
	}
	loaded, err := cp.LoadDeadManConfig()
	if err != nil {
		t.Fatalf("LoadDeadManConfig: %v", err)
	}
	if loaded.PingURL != endpoint {
		t.Fatalf("loaded ping URL = %q, want %q", loaded.PingURL, endpoint)
	}

	stored, err := os.ReadFile(filepath.Join(dir, "deadman.enc"))
	if err != nil {
		t.Fatalf("ReadFile deadman.enc: %v", err)
	}
	if bytes.Contains(stored, []byte(endpoint)) || bytes.Contains(stored, []byte("credential-token")) {
		t.Fatal("deadman.enc exposes the credential-bearing ping URL")
	}
	info, err := os.Stat(filepath.Join(dir, "deadman.enc"))
	if err != nil {
		t.Fatalf("Stat deadman.enc: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("deadman.enc permissions = %o, want 600", info.Mode().Perm())
	}

	if err := cp.SaveDeadManConfig(notifications.DeadManConfig{}); err != nil {
		t.Fatalf("clear dead-man config: %v", err)
	}
	cleared, err := cp.LoadDeadManConfig()
	if err != nil {
		t.Fatalf("load cleared dead-man config: %v", err)
	}
	if cleared.PingURL != "" {
		t.Fatalf("cleared ping URL = %q", cleared.PingURL)
	}
}

func TestDeadManConfigMigratesPlaintextStorage(t *testing.T) {
	dir := t.TempDir()
	cp := config.NewConfigPersistence(dir)
	const endpoint = "https://watchdog.example.com/ping/plaintext-token"
	plain, err := json.Marshal(notifications.DeadManConfig{PingURL: endpoint})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	path := filepath.Join(dir, "deadman.enc")
	if err := os.WriteFile(path, plain, 0o600); err != nil {
		t.Fatalf("WriteFile plaintext deadman.enc: %v", err)
	}

	loaded, err := cp.LoadDeadManConfig()
	if err != nil {
		t.Fatalf("LoadDeadManConfig: %v", err)
	}
	if loaded.PingURL != endpoint {
		t.Fatalf("loaded ping URL = %q, want %q", loaded.PingURL, endpoint)
	}
	rewritten, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile rewritten deadman.enc: %v", err)
	}
	if bytes.Equal(rewritten, plain) || bytes.Contains(rewritten, []byte("plaintext-token")) {
		t.Fatal("plaintext dead-man configuration was not rewritten encrypted")
	}
}

func TestDeadManConfigRejectsInvalidDestinationWithoutReplacingStoredValue(t *testing.T) {
	dir := t.TempDir()
	cp := config.NewConfigPersistence(dir)
	const endpoint = "https://watchdog.example.com/ping/original-token"
	if err := cp.SaveDeadManConfig(notifications.DeadManConfig{PingURL: endpoint}); err != nil {
		t.Fatalf("SaveDeadManConfig: %v", err)
	}
	if err := cp.SaveDeadManConfig(notifications.DeadManConfig{PingURL: "http://127.0.0.1/ping/token"}); err == nil {
		t.Fatal("expected same-host dead-man destination to be rejected")
	}
	loaded, err := cp.LoadDeadManConfig()
	if err != nil {
		t.Fatalf("LoadDeadManConfig: %v", err)
	}
	if loaded.PingURL != endpoint {
		t.Fatalf("invalid update replaced stored value with %q", loaded.PingURL)
	}
}

func TestExportImportIncludesDeadManConfig(t *testing.T) {
	sourceDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", sourceDir)
	source := config.NewConfigPersistence(sourceDir)
	const endpoint = "https://watchdog.example.com/ping/export-token"
	if err := source.SaveDeadManConfig(notifications.DeadManConfig{PingURL: endpoint}); err != nil {
		t.Fatalf("SaveDeadManConfig: %v", err)
	}

	const passphrase = "dead-man-export-round-trip"
	bundle, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}
	decoded := mustDecodeExport(t, bundle, passphrase)
	if decoded.Version != "4.4" || decoded.DeadMan.PingURL != endpoint {
		t.Fatalf("export metadata = version %q deadMan %#v", decoded.Version, decoded.DeadMan)
	}

	destinationDir := t.TempDir()
	destination := config.NewConfigPersistence(destinationDir)
	if err := destination.ImportConfig(bundle, passphrase); err != nil {
		t.Fatalf("ImportConfig: %v", err)
	}
	loaded, err := destination.LoadDeadManConfig()
	if err != nil {
		t.Fatalf("LoadDeadManConfig imported: %v", err)
	}
	if loaded.PingURL != endpoint {
		t.Fatalf("imported ping URL = %q, want %q", loaded.PingURL, endpoint)
	}
}
