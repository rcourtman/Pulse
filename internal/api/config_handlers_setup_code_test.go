package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGenerateSetupCode(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	// Test 1: Basic generation
	code := handler.generateSetupCode()
	if len(code) != 32 { // 16 bytes hex encoded = 32 chars
		t.Errorf("expected code length 32, got %d", len(code))
	}

	// Verify uniqueness
	code2 := handler.generateSetupCode()
	if code == code2 {
		t.Error("generated codes should be unique (random)")
	}

	// Storage is handled by HandleSetupScriptURL, not generateSetupCode.
	// So we don't verify storage here.
}
