package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// QuickstartState is the machine-owned local cache for the server-authoritative
// quickstart bootstrap contract.
type QuickstartState struct {
	QuickstartToken            string `json:"quickstart_token,omitempty"`
	QuickstartTokenExpiresAt   *int64 `json:"quickstart_token_expires_at,omitempty"`
	QuickstartCreditsTotal     int    `json:"quickstart_credits_total,omitempty"`
	QuickstartCreditsRemaining int    `json:"quickstart_credits_remaining,omitempty"`
	LastSyncedAt               *int64 `json:"last_synced_at,omitempty"`
}

// NormalizeQuickstartState returns a deep-cloned, normalized quickstart state.
func NormalizeQuickstartState(state *QuickstartState) *QuickstartState {
	if state == nil {
		return &QuickstartState{}
	}

	cp := *state
	normalized := &cp
	normalized.QuickstartToken = strings.TrimSpace(normalized.QuickstartToken)
	normalized.QuickstartTokenExpiresAt = cloneQuickstartInt64Ptr(state.QuickstartTokenExpiresAt)
	normalized.LastSyncedAt = cloneQuickstartInt64Ptr(state.LastSyncedAt)
	if normalized.QuickstartCreditsTotal < 0 {
		normalized.QuickstartCreditsTotal = 0
	}
	if normalized.QuickstartCreditsRemaining < 0 {
		normalized.QuickstartCreditsRemaining = 0
	}
	if normalized.QuickstartCreditsTotal > 0 && normalized.QuickstartCreditsRemaining > normalized.QuickstartCreditsTotal {
		normalized.QuickstartCreditsRemaining = normalized.QuickstartCreditsTotal
	}
	return normalized
}

func cloneQuickstartInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// TokenExpired reports whether the cached quickstart token is expired at the
// supplied timestamp.
func (s *QuickstartState) TokenExpired(now time.Time) bool {
	if s == nil || s.QuickstartTokenExpiresAt == nil || *s.QuickstartTokenExpiresAt <= 0 {
		return false
	}
	return now.Unix() >= *s.QuickstartTokenExpiresAt
}

// SaveQuickstartState persists the machine-owned quickstart cache.
func (c *ConfigPersistence) SaveQuickstartState(state QuickstartState) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return fmt.Errorf("prepare config directory for quickstart state: %w", err)
	}

	normalized := NormalizeQuickstartState(&state)
	data, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("marshal quickstart state: %w", err)
	}

	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return fmt.Errorf("encrypt quickstart state: %w", err)
		}
		data = encrypted
	}

	if err := c.writeConfigFileLocked(c.quickstartFile, data, 0o600); err != nil {
		return fmt.Errorf("persist quickstart state: %w", err)
	}

	return nil
}

// LoadQuickstartState retrieves the cached quickstart bootstrap state.
func (c *ConfigPersistence) LoadQuickstartState() (*QuickstartState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var state QuickstartState
	migratedPlaintext, _, err := loadEncryptedJSONLocked(c, c.quickstartFile, &state, "quickstart state")
	if err != nil {
		if os.IsNotExist(err) {
			return &QuickstartState{}, nil
		}
		return nil, err
	}

	normalized := NormalizeQuickstartState(&state)
	if migratedPlaintext {
		jsonData, err := json.Marshal(normalized)
		if err != nil {
			return nil, fmt.Errorf("marshal quickstart state migration rewrite: %w", err)
		}
		if err := rewriteEncryptedJSONLocked(c, c.quickstartFile, jsonData, "quickstart state migration rewrite"); err != nil {
			return nil, err
		}
	}

	return normalized, nil
}

// HasQuickstartState reports whether quickstart.enc exists on disk.
func (c *ConfigPersistence) HasQuickstartState() bool {
	if c == nil || c.fs == nil {
		return false
	}
	_, err := c.fs.Stat(c.quickstartFile)
	return err == nil
}
