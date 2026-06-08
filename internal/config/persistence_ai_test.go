package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
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
		InvestigationRecord: &aicontracts.InvestigationRecord{
			ID:        "investigation-1",
			FindingID: "id1",
			Status:    aicontracts.InvestigationStatusCompleted,
			Outcome:   aicontracts.OutcomeFixQueued,
			Evidence:  []aicontracts.InvestigationRecordEvidence{},
			ToolsUsed: []string{},
		},
	}
	data.Findings["id1"] = record

	err = p.SaveAIFindings(data.Findings) // SaveAIFindings takes map[string]*AIFindingRecord
	require.NoError(t, err)

	// Reload
	loaded, err := p.LoadAIFindings()
	require.NoError(t, err)
	assert.Len(t, loaded.Findings, 1)
	assert.Equal(t, "analysis", loaded.Findings["id1"].Description)
	require.NotNil(t, loaded.Findings["id1"].InvestigationRecord)
	assert.Equal(t, "investigation-1", loaded.Findings["id1"].InvestigationRecord.ID)

	// Test Corrupt file (Unmarshal error)
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai_findings.json"), []byte("{invalid"), 0644))

	loaded, err = p.LoadAIFindings()
	// Should return empty structure on unmarshal error, not fail completely (as per code)
	require.NoError(t, err)
	assert.Empty(t, loaded.Findings)
}

func TestAIFindingRecordJSONCanonicalOutput(t *testing.T) {
	record := AIFindingRecord{
		ID:              "finding-1",
		Description:     "analysis",
		AlertIdentifier: "instance:node:100::metric/cpu",
		DetectedAt:      time.Now(),
		LastSeenAt:      time.Now(),
		InvestigationRecord: &aicontracts.InvestigationRecord{
			ID:        "investigation-1",
			FindingID: "finding-1",
			Status:    aicontracts.InvestigationStatusCompleted,
			Evidence:  []aicontracts.InvestigationRecordEvidence{},
			ToolsUsed: []string{},
		},
	}

	raw, err := json.Marshal(record)
	require.NoError(t, err)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &payload))
	assert.Equal(t, "instance:node:100::metric/cpu", payload["alert_identifier"])
	require.Contains(t, payload, "investigation_record")
	_, hasLegacy := payload["alert_id"]
	assert.False(t, hasLegacy)

	var decoded AIFindingRecord
	require.NoError(t, json.Unmarshal([]byte(`{
		"id":"finding-1",
		"description":"analysis",
		"detected_at":"2026-03-11T00:00:00Z",
		"last_seen_at":"2026-03-11T00:00:00Z",
		"alert_identifier":"instance:node:100::metric/cpu"
	}`), &decoded))
	assert.Equal(t, "instance:node:100::metric/cpu", decoded.AlertIdentifier)
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

func TestPersistence_AIConfig_OllamaKeepAliveDefaultsAndExplicitServerDefault(t *testing.T) {
	t.Run("missing field loads pulse default", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)

		legacy := map[string]interface{}{
			"enabled":         true,
			"model":           "ollama:llama3",
			"ollama_base_url": "http://127.0.0.1:11434",
		}
		data, err := json.Marshal(legacy)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "ai.enc"), data, 0o600))

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		require.Equal(t, DefaultOllamaKeepAlive, loaded.OllamaKeepAlive)
	})

	t.Run("empty field survives reload as server default", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)

		cfg := NewDefaultAIConfig()
		cfg.OllamaKeepAlive = ""
		require.NoError(t, p.SaveAIConfig(*cfg))

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		require.Empty(t, loaded.OllamaKeepAlive)
	})
}

func TestPersistence_HasAIConfig(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	if p.HasAIConfig() {
		t.Fatal("expected HasAIConfig() to be false before ai config is saved")
	}

	cfg := NewDefaultAIConfig()
	cfg.Enabled = true
	if err := p.SaveAIConfig(*cfg); err != nil {
		t.Fatalf("SaveAIConfig(): %v", err)
	}

	if !p.HasAIConfig() {
		t.Fatal("expected HasAIConfig() to be true after ai config is saved")
	}
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

func TestPersistence_AIConfig_MigratesLegacyPatrolEventTriggerToggle(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	legacy := map[string]interface{}{
		"enabled":                       true,
		"patrol_event_triggers_enabled": false,
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
	assert.False(t, loaded.PatrolAlertTriggersEnabled)
	assert.False(t, loaded.PatrolAnomalyTriggersEnabled)
	assert.False(t, loaded.PatrolEventTriggersEnabled)
}

func TestPersistence_AIConfig_MigratesCloudContextPrivacyFromLegacyShareToggle(t *testing.T) {
	writeLegacy := func(t *testing.T, p *ConfigPersistence, dir string, legacy map[string]interface{}) {
		t.Helper()
		data, err := json.Marshal(legacy)
		require.NoError(t, err)
		if p.crypto != nil {
			data, err = p.crypto.Encrypt(data)
			require.NoError(t, err)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "ai.enc"), data, 0o600))
	}

	t.Run("legacy share on derives full and leaves the legacy flag untouched", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)
		writeLegacy(t, p, tempDir, map[string]interface{}{
			"enabled":                              true,
			"share_operational_context_with_cloud": true,
		})

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		assert.Equal(t, CloudContextPrivacyFull, loaded.CloudContextPrivacy)
		// The redaction seam still reads the legacy boolean until increment 2, so
		// migration must preserve it byte-for-byte.
		assert.True(t, loaded.ShareOperationalContextWithCloud)
	})

	t.Run("legacy share absent derives redacted preserving current behavior", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)
		writeLegacy(t, p, tempDir, map[string]interface{}{"enabled": true})

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		assert.Equal(t, CloudContextPrivacyRedacted, loaded.CloudContextPrivacy)
		assert.False(t, loaded.ShareOperationalContextWithCloud)
	})

	t.Run("legacy share explicit off derives redacted", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)
		writeLegacy(t, p, tempDir, map[string]interface{}{
			"enabled":                              true,
			"share_operational_context_with_cloud": false,
		})

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		assert.Equal(t, CloudContextPrivacyRedacted, loaded.CloudContextPrivacy)
	})

	t.Run("explicit dial value is preserved and not re-derived", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)
		writeLegacy(t, p, tempDir, map[string]interface{}{
			"enabled":                              true,
			"share_operational_context_with_cloud": true, // would imply "full" if re-derived
			"cloud_context_privacy":                CloudContextPrivacyLocalOnly,
		})

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		assert.Equal(t, CloudContextPrivacyLocalOnly, loaded.CloudContextPrivacy)
	})

	t.Run("fresh install with no config file defaults to full", func(t *testing.T) {
		tempDir := t.TempDir()
		p := NewConfigPersistence(tempDir)

		loaded, err := p.LoadAIConfig()
		require.NoError(t, err)
		assert.Equal(t, CloudContextPrivacyFull, loaded.CloudContextPrivacy)
	})
}

func TestPersistence_AIConfig_NormalizesGranularPatrolTriggerSettingsOnSave(t *testing.T) {
	tempDir := t.TempDir()
	p := NewConfigPersistence(tempDir)

	cfg := NewDefaultAIConfig()
	cfg.SetPatrolEventTriggerSettings(true, false)
	cfg.PatrolEventTriggersEnabled = false
	require.NoError(t, p.SaveAIConfig(*cfg))

	loaded, err := p.LoadAIConfig()
	require.NoError(t, err)
	assert.True(t, loaded.PatrolAlertTriggersEnabled)
	assert.False(t, loaded.PatrolAnomalyTriggersEnabled)
	assert.True(t, loaded.PatrolEventTriggersEnabled)
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
