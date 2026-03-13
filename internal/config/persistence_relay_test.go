package config

import (
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
