package config

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/relay"
)

func TestConfigPersistenceLoadRelayConfigMissingFileReturnsDefault(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())

	cfg, err := cp.LoadRelayConfig()
	if err != nil {
		t.Fatalf("LoadRelayConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadRelayConfig() returned nil config")
	}
	if cfg.Enabled {
		t.Fatalf("LoadRelayConfig() enabled = true, want false")
	}
	if cfg.ServerURL != relay.DefaultServerURL {
		t.Fatalf("LoadRelayConfig() server_url = %q, want %q", cfg.ServerURL, relay.DefaultServerURL)
	}
}

func TestConfigPersistenceLoadRelayConfigMigratesPlaintextFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)

	plaintext := &relay.Config{
		Enabled:             true,
		ServerURL:           "wss://relay.example.com/ws/instance",
		InstanceSecret:      "relay-instance-secret",
		IdentityPrivateKey:  "private-key",
		IdentityPublicKey:   "public-key",
		IdentityFingerprint: "fingerprint",
	}
	raw, err := json.Marshal(plaintext)
	if err != nil {
		t.Fatalf("marshal plaintext relay config: %v", err)
	}

	filePath := filepath.Join(tempDir, "relay.enc")
	if err := os.WriteFile(filePath, raw, 0o600); err != nil {
		t.Fatalf("write plaintext relay config: %v", err)
	}

	cfg, err := cp.LoadRelayConfig()
	if err != nil {
		t.Fatalf("LoadRelayConfig() error = %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadRelayConfig() returned nil config")
	}
	if cfg.InstanceSecret != plaintext.InstanceSecret {
		t.Fatalf("LoadRelayConfig() instance_secret = %q, want %q", cfg.InstanceSecret, plaintext.InstanceSecret)
	}
	if cfg.IdentityPrivateKey != plaintext.IdentityPrivateKey {
		t.Fatalf("LoadRelayConfig() identity_private_key = %q, want %q", cfg.IdentityPrivateKey, plaintext.IdentityPrivateKey)
	}

	rewritten, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read rewritten relay config: %v", err)
	}
	if bytes.Equal(rewritten, raw) {
		t.Fatal("expected plaintext relay config to be rewritten encrypted")
	}
}
