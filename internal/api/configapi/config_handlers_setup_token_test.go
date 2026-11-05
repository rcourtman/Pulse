package configapi

import (
	"testing"
	"time"

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

func TestConfigHandlersStopEndsSetupTokenCleanup(t *testing.T) {
	(&ConfigHandlers{}).Stop() // Handlers built without NewConfigHandlers have no worker.

	h := NewConfigHandlers(nil, nil, nil, nil, nil, nil, false)
	stopped := make(chan struct{})
	go func() {
		h.Stop()
		h.Stop() // A second owner shutting down must not block or panic.
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not end the setup-token cleanup worker")
	}
}

func TestRemoveExpiredSetupTokensKeepsLiveTokens(t *testing.T) {
	h := NewConfigHandlers(nil, nil, nil, nil, nil, nil, false)
	t.Cleanup(h.Stop)
	now := time.Now()
	h.setupTokens["expired"] = &SetupTokenRecord{ExpiresAt: now.Add(-time.Minute)}
	h.setupTokens["used"] = &SetupTokenRecord{ExpiresAt: now.Add(time.Hour), Used: true}
	h.setupTokens["live"] = &SetupTokenRecord{ExpiresAt: now.Add(time.Hour)}
	h.recentSetupTokens["recent-expired"] = RecentSetupTokenRecord{ExpiresAt: now.Add(-time.Minute)}
	h.recentSetupTokens["recent-live"] = RecentSetupTokenRecord{ExpiresAt: now.Add(time.Hour)}

	h.removeExpiredSetupTokens(now)

	if _, ok := h.setupTokens["live"]; !ok || len(h.setupTokens) != 1 {
		t.Fatalf("setup tokens after cleanup = %v, want only live", h.setupTokens)
	}
	if _, ok := h.recentSetupTokens["recent-live"]; !ok || len(h.recentSetupTokens) != 1 {
		t.Fatalf("recent setup tokens after cleanup = %v, want only recent-live", h.recentSetupTokens)
	}
}
