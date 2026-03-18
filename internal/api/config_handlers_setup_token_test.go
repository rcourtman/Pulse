package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestGenerateSetupTokenRecord(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		DataPath:   tempDir,
		ConfigPath: tempDir,
	}

	handler := newTestConfigHandlers(t, cfg)

	// Test 1: Basic generation
	token := handler.generateSetupTokenRecord()
	if len(token) != 32 { // 16 bytes hex encoded = 32 chars
		t.Errorf("expected token length 32, got %d", len(token))
	}

	// Verify uniqueness
	token2 := handler.generateSetupTokenRecord()
	if token == token2 {
		t.Error("generated setup tokens should be unique (random)")
	}

	// Storage is handled by HandleSetupScriptURL, not generateSetupTokenRecord.
	// So we don't verify storage here.
}
