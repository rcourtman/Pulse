package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistence_AIFindings(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	// Default load (empty)
	data, err := p.LoadAIFindings()
	require.NoError(t, err)
	assert.Empty(t, data.Findings)

	// Save
	record := &AIFindingRecord{
		ID:          "id1",
		Description: "analysis",
		DetectedAt:  time.Now(),
	}
	data.Findings["id1"] = record

	err = p.SaveAIFindings(data.Findings) // SaveAIFindings takes map[string]*AIFindingRecord
	require.NoError(t, err)

	// Reload
	loaded, err := p.LoadAIFindings()
	require.NoError(t, err)
	assert.Len(t, loaded.Findings, 1)
	assert.Equal(t, "analysis", loaded.Findings["id1"].Description)

	// Test Corrupt file (Unmarshal error)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai_findings.json"), []byte("{invalid"), 0644))

	loaded, err = p.LoadAIFindings()
	// Should return empty structure on unmarshal error, not fail completely (as per code)
	require.NoError(t, err)
	assert.Empty(t, loaded.Findings)
}

func TestAIFindingRecordJSONCanonicalAndLegacyCompatibility(t *testing.T) {
	record := AIFindingRecord{
		ID:              "finding-1",
		Description:     "analysis",
		AlertIdentifier: "instance:node:100::metric/cpu",
		DetectedAt:      time.Now(),
		LastSeenAt:      time.Now(),
	}

	raw, err := json.Marshal(record)
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, "instance:node:100::metric/cpu", payload["alert_identifier"])
	_, hasLegacy := payload["alert_id"]
	assert.False(t, hasLegacy)

	var decodedCanonical AIFindingRecord
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"finding-1",
		"description":"analysis",
		"detected_at":"2026-03-11T00:00:00Z",
		"last_seen_at":"2026-03-11T00:00:00Z",
		"alert_identifier":"instance:node:100::metric/cpu"
	}`), &decodedCanonical))
	assert.Equal(t, "instance:node:100::metric/cpu", decodedCanonical.AlertIdentifier)

	var decodedLegacy AIFindingRecord
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"finding-1",
		"description":"analysis",
		"detected_at":"2026-03-11T00:00:00Z",
		"last_seen_at":"2026-03-11T00:00:00Z",
		"alert_id":"legacy-alert-id"
	}`), &decodedLegacy))
	assert.Equal(t, "legacy-alert-id", decodedLegacy.AlertIdentifier)
}

func TestPersistence_AIUsageHistory(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	// Default load
	data, err := p.LoadAIUsageHistory()
	require.NoError(t, err)
	assert.Empty(t, data.Events)

	// Save
	record := AIUsageEventRecord{
		Timestamp:    time.Now(),
		RequestModel: "gpt-4",
		InputTokens:  10,
		OutputTokens: 20,
	}
	data.Events = append(data.Events, record)

	err = p.SaveAIUsageHistory(data.Events)
	require.NoError(t, err)

	// Reload
	loaded, err := p.LoadAIUsageHistory()
	require.NoError(t, err)
	assert.Len(t, loaded.Events, 1)
	assert.Equal(t, 10, loaded.Events[0].InputTokens)

	// Test Corrupt file
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai_usage_history.json"), []byte("{invalid"), 0644))

	loaded, err = p.LoadAIUsageHistory()
	// Should return empty structure
	require.NoError(t, err)
	assert.Empty(t, loaded.Events)
}

func TestPersistence_AIConfig(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	// Load default (empty/nil config if file missing? LoadAIConfig returns NewDefaultAIConfig logic inside persistence??)
	// Let's check logic: LoadAIConfig reads file, if missing returns default?
	loaded, err := p.LoadAIConfig()
	require.NoError(t, err)
	assert.NotNil(t, loaded) // Default config

	// Save
	cfg := NewDefaultAIConfig()
	cfg.AnthropicAPIKey = "testkey"
	err = p.SaveAIConfig(*cfg)
	require.NoError(t, err)

	// Reload
	loaded, err = p.LoadAIConfig()
	require.NoError(t, err)
	assert.Equal(t, "testkey", loaded.AnthropicAPIKey)

	// Corrupt file
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai.enc"), []byte("{invalid"), 0644))
	// Without encryption, it's just json
	// If encryption enabled (not in this test), behaviour changes.

	// If crypto nil:
	// Persistence.LoadAIConfig attempts decrypt if crypto != nil.

	// If Unmarshal fails:
	_, err = p.LoadAIConfig()
	assert.Error(t, err)
}

func TestPersistence_AIConfig_MigratesSuggestControlLevel(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	cfg := NewDefaultAIConfig()
	cfg.ControlLevel = "suggest"
	require.NoError(t, p.SaveAIConfig(*cfg))

	loaded, err := p.LoadAIConfig()
	require.NoError(t, err)
	assert.Equal(t, ControlLevelControlled, loaded.ControlLevel)

	updatedRaw, err := os.ReadFile(filepath.Join(tempDir, "ai.enc"))
	require.NoError(t, err)
	if p.crypto != nil {
		decoded, err := p.crypto.Decrypt(updatedRaw)
		require.NoError(t, err)
		updatedRaw = decoded
	}

	var saved AIConfig
	require.NoError(t, json.Unmarshal(updatedRaw, &saved))
	assert.Equal(t, ControlLevelControlled, saved.ControlLevel)
}

func TestPersistence_AIConfig_MigratesLegacyProviderAndAPIKey(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	legacy := map[string]interface{}{
		"enabled":              true,
		"provider":             "anthropic",
		"api_key":              "sk-ant-legacy",
		"model":                "anthropic:claude-3-5-sonnet-20241022",
		"autonomous_mode":      true,
		"custom_context":       "legacy config",
		"patrol_enabled":       true,
		"patrol_analyze_nodes": true,
	}

	data, err := json.Marshal(legacy)
	require.NoError(t, err)
	if p.crypto != nil {
		data, err = p.crypto.Encrypt(data)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai.enc"), data, 0o600))

	loaded, err := p.LoadAIConfig()
	require.NoError(t, err)
	assert.Equal(t, "anthropic:claude-3-5-sonnet-20241022", loaded.Model)
	assert.Equal(t, "sk-ant-legacy", loaded.AnthropicAPIKey)
	assert.Equal(t, ControlLevelAutonomous, loaded.ControlLevel)
	assert.Equal(t, "legacy config", loaded.CustomContext)

	savedRaw, err := os.ReadFile(filepath.Join(tempDir, "ai.enc"))
	require.NoError(t, err)
	if p.crypto != nil {
		savedRaw, err = p.crypto.Decrypt(savedRaw)
		require.NoError(t, err)
	}

	var saved AIConfig
	require.NoError(t, json.Unmarshal(savedRaw, &saved))
	assert.Equal(t, "sk-ant-legacy", saved.AnthropicAPIKey)
	assert.Equal(t, ControlLevelAutonomous, saved.ControlLevel)
}

func TestPersistence_PatrolRunHistory(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	hist, err := p.LoadPatrolRunHistory()
	require.NoError(t, err)
	assert.Empty(t, hist.Runs)

	hist.Runs = append(hist.Runs, PatrolRunRecord{ID: "run1"})
	err = p.SavePatrolRunHistory(hist.Runs)
	require.NoError(t, err)

	loaded, err := p.LoadPatrolRunHistory()
	require.NoError(t, err)
	assert.Len(t, loaded.Runs, 1)

	// Corrupt
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai_patrol_runs.json"), []byte("{invalid"), 0644))
	loaded, err = p.LoadPatrolRunHistory()
	require.NoError(t, err) // Returns empty on error
	assert.Empty(t, loaded.Runs)
}
