// Package migration contains integration tests verifying that a v6 Pulse binary
// can safely start against a v5 data directory and load all configurations
// without data loss.
package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildV5DataDir creates a temporary directory that mimics a v5 Pulse data
// directory. It generates a real AES-256 encryption key and encrypts
// nodes.enc and ai.enc using the same crypto path the production binary uses.
// alerts.json and system.json are written as plaintext JSON (matching v5 format).
//
// The returned directory contains:
//
//	.encryption.key  – base64-encoded 32-byte AES key
//	nodes.enc        – AES-GCM encrypted node config
//	ai.enc           – AES-GCM encrypted AI config
//	alerts.json      – plaintext alert config
//	system.json      – plaintext system settings
func buildV5DataDir(t *testing.T) (dataDir string, nodesJSON []byte, aiJSON []byte, alertsJSON []byte, systemJSON []byte) {
	t.Helper()
	dataDir = t.TempDir()

	// 1. Create encryption key via CryptoManager (generates .encryption.key)
	cm, err := crypto.NewCryptoManagerAt(dataDir)
	require.NoError(t, err, "CryptoManager should initialize and create .encryption.key")

	// Verify encryption key file exists
	keyPath := filepath.Join(dataDir, ".encryption.key")
	_, err = os.Stat(keyPath)
	require.NoError(t, err, ".encryption.key must exist")

	// 2. Build a realistic v5 nodes config (3 PVE nodes, 1 PBS)
	nodes := config.NodesConfig{
		PVEInstances: []config.PVEInstance{
			{
				Name:              "pve-node-1",
				Host:              "https://192.168.1.10:8006",
				User:              "root@pam",
				TokenName:         "pulse",
				TokenValue:        "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
				VerifySSL:         true,
				MonitorVMs:        true,
				MonitorContainers: true,
				MonitorStorage:    true,
				MonitorBackups:    true,
			},
			{
				Name:        "pve-node-2",
				Host:        "https://192.168.1.11:8006",
				User:        "root@pam",
				Password:    "supersecret",
				VerifySSL:   false,
				MonitorVMs:  true,
				IsCluster:   true,
				ClusterName: "dc1",
			},
			{
				Name:       "pve-node-3",
				Host:       "https://10.0.0.5:8006",
				User:       "admin@pve",
				TokenName:  "monitor",
				TokenValue: "11111111-2222-3333-4444-555555555555",
				VerifySSL:  true,
				MonitorVMs: true,
			},
		},
		PBSInstances: []config.PBSInstance{
			{
				Name:      "pbs-backup-1",
				Host:      "https://192.168.1.20:8007",
				User:      "backup@pbs",
				TokenName: "pulse-read",
			},
		},
		PMGInstances: []config.PMGInstance{},
	}
	nodesJSON, err = json.Marshal(nodes)
	require.NoError(t, err)
	encryptedNodes, err := cm.Encrypt(nodesJSON)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "nodes.enc"), encryptedNodes, 0o600))

	// 3. Build a v5-era AI config (single provider, no multi-provider fields)
	aiCfg := map[string]interface{}{
		"enabled":               true,
		"provider":              "anthropic",
		"api_key":               "sk-ant-v5-test-key-placeholder",
		"model":                 "anthropic:claude-3-5-sonnet-20241022",
		"autonomous_mode":       false,
		"custom_context":        "3-node Proxmox cluster running production workloads",
		"patrol_enabled":        true,
		"patrol_analyze_nodes":  true,
		"patrol_analyze_guests": true,
	}
	aiJSON, err = json.Marshal(aiCfg)
	require.NoError(t, err)
	encryptedAI, err := cm.Encrypt(aiJSON)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "ai.enc"), encryptedAI, 0o600))

	// 4. Build a v5-era alert config
	alertCfg := alerts.AlertConfig{
		Enabled: true,
		GuestDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:   &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
		},
		NodeDefaults: alerts.ThresholdConfig{
			CPU:         &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory:      &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
			Disk:        &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
			Temperature: &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
		},
		Overrides: map[string]alerts.ThresholdConfig{
			"vm-100": {
				CPU:    &alerts.HysteresisThreshold{Trigger: 95, Clear: 90},
				Memory: &alerts.HysteresisThreshold{Trigger: 95, Clear: 90},
			},
		},
	}
	alertsJSON, err = json.Marshal(alertCfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "alerts.json"), alertsJSON, 0o644))

	// 5. Build a v5-era system.json
	systemCfg := map[string]interface{}{
		"pvePollingInterval":           10,
		"pbsPollingInterval":           60,
		"pmgPollingInterval":           60,
		"connectionTimeout":            10,
		"autoUpdateEnabled":            false,
		"discoveryEnabled":             true,
		"discoverySubnet":              "auto",
		"theme":                        "dark",
		"temperatureMonitoringEnabled": true,
	}
	systemJSON, err = json.Marshal(systemCfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "system.json"), systemJSON, 0o644))

	return dataDir, nodesJSON, aiJSON, alertsJSON, systemJSON
}

// TestV5DataDir_V6ConfigPersistenceLoadsAll verifies that a v6
// ConfigPersistence can initialize against a v5 data directory and load
// every config file without error or data loss.
func TestV5DataDir_V6ConfigPersistenceLoadsAll(t *testing.T) {
	dataDir, _, _, _, _ := buildV5DataDir(t)

	// v6 ConfigPersistence creates a CryptoManager that reads .encryption.key
	cp := config.NewConfigPersistence(dataDir)
	require.NotNil(t, cp, "ConfigPersistence should initialize")
	assert.Equal(t, dataDir, cp.DataDir())

	// --- Nodes ---
	nodesCfg, err := cp.LoadNodesConfig()
	require.NoError(t, err, "LoadNodesConfig must succeed against v5 data")
	require.NotNil(t, nodesCfg)
	assert.Len(t, nodesCfg.PVEInstances, 3, "should load all 3 PVE nodes")
	assert.Len(t, nodesCfg.PBSInstances, 1, "should load 1 PBS instance")
	assert.Len(t, nodesCfg.PMGInstances, 0, "PMG should be empty")

	// Verify specific node data survived decryption roundtrip
	assert.Equal(t, "pve-node-1", nodesCfg.PVEInstances[0].Name)
	assert.Equal(t, "https://192.168.1.10:8006", nodesCfg.PVEInstances[0].Host)
	assert.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", nodesCfg.PVEInstances[0].TokenValue)
	assert.Equal(t, "supersecret", nodesCfg.PVEInstances[1].Password)
	assert.True(t, nodesCfg.PVEInstances[1].IsCluster)
	assert.Equal(t, "dc1", nodesCfg.PVEInstances[1].ClusterName)
	assert.Equal(t, "pbs-backup-1", nodesCfg.PBSInstances[0].Name)

	// --- AI ---
	aiCfg, err := cp.LoadAIConfig()
	require.NoError(t, err, "LoadAIConfig must succeed against v5 data")
	require.NotNil(t, aiCfg)
	assert.True(t, aiCfg.Enabled)
	assert.Equal(t, "anthropic:claude-3-5-sonnet-20241022", aiCfg.Model)
	assert.Equal(t, "sk-ant-v5-test-key-placeholder", aiCfg.AnthropicAPIKey)
	assert.True(t, aiCfg.PatrolEnabled)
	assert.True(t, aiCfg.PatrolAnalyzeNodes)

	// --- Alerts ---
	alertCfg, err := cp.LoadAlertConfig()
	require.NoError(t, err, "LoadAlertConfig must succeed against v5 data")
	require.NotNil(t, alertCfg)
	assert.True(t, alertCfg.Enabled)
	assert.Equal(t, float64(80), alertCfg.GuestDefaults.CPU.Trigger)
	assert.Equal(t, float64(80), alertCfg.NodeDefaults.Temperature.Trigger)
	require.Contains(t, alertCfg.Overrides, "vm-100")
	assert.Equal(t, float64(95), alertCfg.Overrides["vm-100"].CPU.Trigger)

	// --- System ---
	sysCfg, err := cp.LoadSystemSettings()
	require.NoError(t, err, "LoadSystemSettings must succeed against v5 data")
	require.NotNil(t, sysCfg)
	assert.Equal(t, 10, sysCfg.PVEPollingInterval)
	assert.Equal(t, 60, sysCfg.PBSPollingInterval)
}

// TestV5DataDir_EncryptedConfigRoundtrip verifies that encrypted data written
// with a v5-era encryption key can be read back, modified, and re-written by
// v6 without corrupting the encryption.
func TestV5DataDir_EncryptedConfigRoundtrip(t *testing.T) {
	dataDir, _, _, _, _ := buildV5DataDir(t)

	cp := config.NewConfigPersistence(dataDir)

	// Load existing nodes
	nodesCfg, err := cp.LoadNodesConfig()
	require.NoError(t, err)
	require.Len(t, nodesCfg.PVEInstances, 3)

	// Simulate v6 adding a new node and saving
	nodesCfg.PVEInstances = append(nodesCfg.PVEInstances, config.PVEInstance{
		Name:       "pve-node-4-v6-added",
		Host:       "https://10.0.0.100:8006",
		User:       "root@pam",
		TokenName:  "pulse-v6",
		TokenValue: "v6-token-value",
		VerifySSL:  true,
		MonitorVMs: true,
	})

	err = cp.SaveNodesConfig(
		nodesCfg.PVEInstances,
		nodesCfg.PBSInstances,
		nodesCfg.PMGInstances,
	)
	require.NoError(t, err, "SaveNodesConfig must succeed for v6 re-write")

	// Re-load and verify both old and new data survived
	nodesCfg2, err := cp.LoadNodesConfig()
	require.NoError(t, err)
	require.Len(t, nodesCfg2.PVEInstances, 4)
	assert.Equal(t, "pve-node-1", nodesCfg2.PVEInstances[0].Name)
	assert.Equal(t, "pve-node-4-v6-added", nodesCfg2.PVEInstances[3].Name)
	assert.Equal(t, "v6-token-value", nodesCfg2.PVEInstances[3].TokenValue)

	// Verify a fresh ConfigPersistence against the same dir can read it
	cp2 := config.NewConfigPersistence(dataDir)
	nodesCfg3, err := cp2.LoadNodesConfig()
	require.NoError(t, err)
	assert.Len(t, nodesCfg3.PVEInstances, 4)
}

// TestV5DataDir_EmptyDataDir verifies that v6 starts cleanly against an
// empty data directory (brand new installation).
func TestV5DataDir_EmptyDataDir(t *testing.T) {
	dataDir := t.TempDir()

	cp := config.NewConfigPersistence(dataDir)
	require.NotNil(t, cp)

	// All loads should return defaults, not errors
	nodesCfg, err := cp.LoadNodesConfig()
	require.NoError(t, err)
	assert.Empty(t, nodesCfg.PVEInstances)

	aiCfg, err := cp.LoadAIConfig()
	require.NoError(t, err)
	assert.False(t, aiCfg.Enabled, "default AI config should be disabled")

	alertCfg, err := cp.LoadAlertConfig()
	require.NoError(t, err)
	assert.True(t, alertCfg.Enabled, "default alerts should be enabled")

	sysCfg, err := cp.LoadSystemSettings()
	require.NoError(t, err)
	assert.Nil(t, sysCfg, "missing system.json should return nil, not defaults")
}

// TestV5DataDir_MissingOptionalFiles verifies that v6 handles a data
// directory where some optional files are absent (e.g., ai.enc never
// configured, no alerts customized).
func TestV5DataDir_MissingOptionalFiles(t *testing.T) {
	dataDir := t.TempDir()

	// Only create encryption key and nodes.enc — skip ai.enc, alerts.json, system.json
	cm, err := crypto.NewCryptoManagerAt(dataDir)
	require.NoError(t, err)

	nodes := config.NodesConfig{
		PVEInstances: []config.PVEInstance{
			{Name: "solo-node", Host: "https://10.0.0.1:8006", User: "root@pam"},
		},
	}
	nodesJSON, err := json.Marshal(nodes)
	require.NoError(t, err)
	encrypted, err := cm.Encrypt(nodesJSON)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "nodes.enc"), encrypted, 0o600))

	cp := config.NewConfigPersistence(dataDir)

	// Nodes should load fine
	nodesCfg, err := cp.LoadNodesConfig()
	require.NoError(t, err)
	assert.Len(t, nodesCfg.PVEInstances, 1)
	assert.Equal(t, "solo-node", nodesCfg.PVEInstances[0].Name)

	// Missing AI config returns defaults
	aiCfg, err := cp.LoadAIConfig()
	require.NoError(t, err)
	assert.False(t, aiCfg.Enabled)

	// Missing alerts returns defaults
	alertCfg, err := cp.LoadAlertConfig()
	require.NoError(t, err)
	assert.True(t, alertCfg.Enabled)
	assert.NotNil(t, alertCfg.GuestDefaults.CPU)

	// Missing system.json returns nil (not an error)
	sysCfg, err := cp.LoadSystemSettings()
	require.NoError(t, err)
	assert.Nil(t, sysCfg)
}

// TestV5DataDir_CorruptEncryptionKey verifies that v6 refuses to generate a
// new encryption key when encrypted data already exists with a different key.
// This prevents silent data loss from key mismatch.
func TestV5DataDir_CorruptEncryptionKey(t *testing.T) {
	dataDir, _, _, _, _ := buildV5DataDir(t)

	// Isolate from host's /etc/pulse/.encryption.key to prevent false negatives
	// if a valid legacy key happens to exist on the dev machine.
	t.Setenv("PULSE_LEGACY_KEY_PATH", filepath.Join(dataDir, "nonexistent-legacy-key"))

	// Corrupt the encryption key by overwriting with invalid content
	keyPath := filepath.Join(dataDir, ".encryption.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("not-a-valid-base64-key!!!"), 0o600))

	// ConfigPersistence should fail (Fatal in production, error via internal)
	// We test the underlying newConfigPersistence indirectly — NewConfigPersistence
	// calls log.Fatal on error, so we test via CryptoManager directly.
	_, err := crypto.NewCryptoManagerAt(dataDir)
	require.Error(t, err, "CryptoManager must reject invalid encryption key when enc files exist")
	require.ErrorContains(t, err, "encrypted data exists", "should refuse to generate new key when enc files are present")
}

// TestV5DataDir_MultiTenantMigration verifies that the multi-tenant migration
// (orgs/default/) correctly moves v5 files and creates backward-compatible
// symlinks, and that ConfigPersistence can still load configs through symlinks.
func TestV5DataDir_MultiTenantMigration(t *testing.T) {
	dataDir, _, _, _, _ := buildV5DataDir(t)

	// Before migration, verify files exist in root
	_, err := os.Stat(filepath.Join(dataDir, "nodes.enc"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dataDir, "alerts.json"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dataDir, "system.json"))
	require.NoError(t, err)

	// Run multi-tenant migration
	require.NoError(t, config.RunMigrationIfNeeded(dataDir))

	// After migration: files should be in orgs/default/
	defaultOrgDir := filepath.Join(dataDir, "orgs", "default")
	_, err = os.Stat(filepath.Join(defaultOrgDir, "nodes.enc"))
	require.NoError(t, err, "nodes.enc should exist in orgs/default/")
	_, err = os.Stat(filepath.Join(defaultOrgDir, "alerts.json"))
	require.NoError(t, err, "alerts.json should exist in orgs/default/")
	_, err = os.Stat(filepath.Join(defaultOrgDir, "system.json"))
	require.NoError(t, err, "system.json should exist in orgs/default/")

	// Original paths should be symlinks
	info, err := os.Lstat(filepath.Join(dataDir, "nodes.enc"))
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, info.Mode()&os.ModeSymlink, "nodes.enc should be a symlink")

	// ConfigPersistence should still load everything through symlinks
	cp := config.NewConfigPersistence(dataDir)
	nodesCfg, err := cp.LoadNodesConfig()
	require.NoError(t, err)
	assert.Len(t, nodesCfg.PVEInstances, 3, "should load all 3 PVE nodes through symlink")
	assert.Equal(t, "pve-node-1", nodesCfg.PVEInstances[0].Name)

	alertCfg, err := cp.LoadAlertConfig()
	require.NoError(t, err)
	assert.True(t, alertCfg.Enabled)
	// Assert fixture-specific values to ensure we're reading the migrated file, not defaults
	require.Contains(t, alertCfg.Overrides, "vm-100", "fixture override must survive migration")
	assert.Equal(t, float64(95), alertCfg.Overrides["vm-100"].CPU.Trigger)

	sysCfg, err := cp.LoadSystemSettings()
	require.NoError(t, err)
	require.NotNil(t, sysCfg)
	assert.Equal(t, 10, sysCfg.PVEPollingInterval)
	assert.Equal(t, 60, sysCfg.PBSPollingInterval, "fixture PBS interval must survive migration")

	// Migration should be idempotent
	assert.False(t, config.IsMigrationNeeded(dataDir))
}

// TestV5DataDir_AlertForwardCompat verifies that a v5 alert config with only
// basic fields loads correctly in v6 which may have added new fields with
// defaults.
func TestV5DataDir_AlertForwardCompat(t *testing.T) {
	dataDir := t.TempDir()

	// Minimal v5-era alert config (no PMG, no PBS, no docker, no schedule)
	minimalAlert := map[string]interface{}{
		"enabled": true,
		"guestDefaults": map[string]interface{}{
			"cpu":    map[string]interface{}{"trigger": 80, "clear": 75},
			"memory": map[string]interface{}{"trigger": 85, "clear": 80},
		},
		"nodeDefaults": map[string]interface{}{
			"cpu":    map[string]interface{}{"trigger": 80, "clear": 75},
			"memory": map[string]interface{}{"trigger": 85, "clear": 80},
		},
	}
	data, err := json.Marshal(minimalAlert)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "alerts.json"), data, 0o644))

	cp := config.NewConfigPersistence(dataDir)
	alertCfg, err := cp.LoadAlertConfig()
	require.NoError(t, err)
	assert.True(t, alertCfg.Enabled)
	assert.Equal(t, float64(80), alertCfg.GuestDefaults.CPU.Trigger)
	// New v6 fields should have zero-value defaults, not cause errors
	assert.Empty(t, alertCfg.CustomRules)
}

// TestV5DataDir_SystemSettingsForwardCompat verifies that a v5 system.json
// with fewer fields loads into v6 SystemSettings without error. New v6 fields
// like metrics retention should get defaults.
func TestV5DataDir_SystemSettingsForwardCompat(t *testing.T) {
	dataDir := t.TempDir()

	// Provide encryption key so ConfigPersistence doesn't fail
	_, err := crypto.NewCryptoManagerAt(dataDir)
	require.NoError(t, err)

	// Minimal v5 system.json (no metrics retention, no discovery config)
	v5System := map[string]interface{}{
		"pvePollingInterval": 15,
		"pbsPollingInterval": 120,
		"theme":              "dark",
	}
	data, err := json.Marshal(v5System)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "system.json"), data, 0o644))

	cp := config.NewConfigPersistence(dataDir)
	sysCfg, err := cp.LoadSystemSettings()
	require.NoError(t, err)
	require.NotNil(t, sysCfg)
	assert.Equal(t, 15, sysCfg.PVEPollingInterval)
	assert.Equal(t, 120, sysCfg.PBSPollingInterval)
}
