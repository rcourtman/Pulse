package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestQuickstartStateSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)

	expiresAt := int64(1774449000)
	lastSyncedAt := int64(1774448000)
	state := QuickstartState{
		ClientInstallationID:       "community-install-1",
		QuickstartToken:            "qst_live_test_123",
		QuickstartTokenExpiresAt:   &expiresAt,
		QuickstartCreditsTotal:     25,
		QuickstartCreditsRemaining: 17,
		LastSyncedAt:               &lastSyncedAt,
	}

	if err := persistence.SaveQuickstartState(state); err != nil {
		t.Fatalf("SaveQuickstartState: %v", err)
	}

	loaded, err := persistence.LoadQuickstartState()
	if err != nil {
		t.Fatalf("LoadQuickstartState: %v", err)
	}
	if loaded.ClientInstallationID != state.ClientInstallationID {
		t.Fatalf("ClientInstallationID = %q, want %q", loaded.ClientInstallationID, state.ClientInstallationID)
	}
	if loaded.QuickstartToken != state.QuickstartToken {
		t.Fatalf("QuickstartToken = %q, want %q", loaded.QuickstartToken, state.QuickstartToken)
	}
	if loaded.QuickstartCreditsRemaining != state.QuickstartCreditsRemaining {
		t.Fatalf("QuickstartCreditsRemaining = %d, want %d", loaded.QuickstartCreditsRemaining, state.QuickstartCreditsRemaining)
	}
	if loaded.QuickstartCreditsTotal != state.QuickstartCreditsTotal {
		t.Fatalf("QuickstartCreditsTotal = %d, want %d", loaded.QuickstartCreditsTotal, state.QuickstartCreditsTotal)
	}
	if loaded.QuickstartTokenExpiresAt == nil || *loaded.QuickstartTokenExpiresAt != expiresAt {
		t.Fatalf("QuickstartTokenExpiresAt = %v, want %d", loaded.QuickstartTokenExpiresAt, expiresAt)
	}
	if loaded.LastSyncedAt == nil || *loaded.LastSyncedAt != lastSyncedAt {
		t.Fatalf("LastSyncedAt = %v, want %d", loaded.LastSyncedAt, lastSyncedAt)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "quickstart.enc"))
	if err != nil {
		t.Fatalf("ReadFile(quickstart.enc): %v", err)
	}
	if bytes.Contains(raw, []byte(state.QuickstartToken)) {
		t.Fatal("quickstart.enc should not contain the raw quickstart token")
	}
}

func TestLoadQuickstartState_MissingReturnsEmptyState(t *testing.T) {
	persistence := NewConfigPersistence(t.TempDir())

	loaded, err := persistence.LoadQuickstartState()
	if err != nil {
		t.Fatalf("LoadQuickstartState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected empty quickstart state, got nil")
	}
	if loaded.QuickstartToken != "" || loaded.QuickstartCreditsRemaining != 0 || loaded.ClientInstallationID != "" {
		t.Fatalf("expected zero-value quickstart state, got %#v", loaded)
	}
}
