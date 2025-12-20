package config_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"golang.org/x/crypto/pbkdf2"
)

func TestSaveAlertConfig_PreservesStorageOverrideHysteresis(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		Enabled:          true,
		StorageDefault:   alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		HysteresisMargin: 5.0,
		Overrides: map[string]alerts.ThresholdConfig{
			"storage-123": {
				Usage: &alerts.HysteresisThreshold{Trigger: 90, Clear: 0},
			},
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	override, ok := loaded.Overrides["storage-123"]
	if !ok {
		t.Fatalf("storage override missing after load: %+v", loaded.Overrides)
	}
	if override.Usage == nil {
		t.Fatalf("usage threshold nil after load")
	}
	if got, want := override.Usage.Trigger, 90.0; got != want {
		t.Fatalf("trigger mismatch: got %v want %v", got, want)
	}
	if got, want := override.Usage.Clear, 85.0; got != want {
		t.Fatalf("clear threshold mismatch: got %v want %v", got, want)
	}
}

func TestSaveAlertConfig_DoesNotOverwriteExistingClear(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 5.0,
		StorageDefault:   alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		Overrides: map[string]alerts.ThresholdConfig{
			"storage-456": {
				Usage: &alerts.HysteresisThreshold{Trigger: 92, Clear: 88},
			},
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	override := loaded.Overrides["storage-456"]
	if override.Usage == nil {
		t.Fatalf("usage threshold nil")
	}
	if got, want := override.Usage.Clear, 88.0; got != want {
		t.Fatalf("clear threshold changed unexpectedly: got %v want %v", got, want)
	}
}

func TestSaveAlertConfig_NormalizesHostDefaults(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Config with nil/zero HostDefaults - should get defaults
	cfg := alerts.AlertConfig{
		Enabled:        true,
		StorageDefault: alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		HostDefaults:   alerts.ThresholdConfig{}, // Empty - needs defaults
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	// Verify host defaults were applied
	if loaded.HostDefaults.CPU == nil {
		t.Fatal("CPU defaults should be set")
	}
	if loaded.HostDefaults.CPU.Trigger != 80 {
		t.Errorf("CPU trigger = %v, want 80", loaded.HostDefaults.CPU.Trigger)
	}
	if loaded.HostDefaults.Memory == nil {
		t.Fatal("Memory defaults should be set")
	}
	if loaded.HostDefaults.Memory.Trigger != 85 {
		t.Errorf("Memory trigger = %v, want 85", loaded.HostDefaults.Memory.Trigger)
	}
	if loaded.HostDefaults.Disk == nil {
		t.Fatal("Disk defaults should be set")
	}
	if loaded.HostDefaults.Disk.Trigger != 90 {
		t.Errorf("Disk trigger = %v, want 90", loaded.HostDefaults.Disk.Trigger)
	}
}

func TestSaveAlertConfig_NormalizesHostDefaultsClear(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Config with trigger set but clear=0 - should compute clear
	cfg := alerts.AlertConfig{
		Enabled:        true,
		StorageDefault: alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		HostDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 90, Clear: 0},
			Memory: &alerts.HysteresisThreshold{Trigger: 95, Clear: 0},
			Disk:   &alerts.HysteresisThreshold{Trigger: 92, Clear: 0},
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	// Clear should be trigger - 5
	if loaded.HostDefaults.CPU.Clear != 85 {
		t.Errorf("CPU clear = %v, want 85", loaded.HostDefaults.CPU.Clear)
	}
	if loaded.HostDefaults.Memory.Clear != 90 {
		t.Errorf("Memory clear = %v, want 90", loaded.HostDefaults.Memory.Clear)
	}
	if loaded.HostDefaults.Disk.Clear != 87 {
		t.Errorf("Disk clear = %v, want 87", loaded.HostDefaults.Disk.Clear)
	}
}

// TestSaveAlertConfig_HostDefaultsZeroDisablesAlerting verifies that setting
// Host Agent thresholds to 0 is preserved (fixes GitHub issue #864).
// Setting a threshold to 0 should disable alerting for that metric.
func TestSaveAlertConfig_HostDefaultsZeroDisablesAlerting(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Config with Memory=0 to disable memory alerting for host agents
	cfg := alerts.AlertConfig{
		Enabled:        true,
		StorageDefault: alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		HostDefaults: alerts.ThresholdConfig{
			CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Memory: &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}, // Disabled
			Disk:   &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	// Memory threshold should remain at 0 (disabled), not reset to default
	if loaded.HostDefaults.Memory == nil {
		t.Fatal("Memory defaults should be preserved (not nil)")
	}
	if loaded.HostDefaults.Memory.Trigger != 0 {
		t.Errorf("Memory trigger = %v, want 0 (disabled)", loaded.HostDefaults.Memory.Trigger)
	}
	if loaded.HostDefaults.Memory.Clear != 0 {
		t.Errorf("Memory clear = %v, want 0 (disabled)", loaded.HostDefaults.Memory.Clear)
	}

	// CPU and Disk should still have their values
	if loaded.HostDefaults.CPU.Trigger != 80 {
		t.Errorf("CPU trigger = %v, want 80", loaded.HostDefaults.CPU.Trigger)
	}
	if loaded.HostDefaults.Disk.Trigger != 90 {
		t.Errorf("Disk trigger = %v, want 90", loaded.HostDefaults.Disk.Trigger)
	}
}

func TestAlertConfigPersistenceNormalizesDockerIgnoredPrefixes(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := alerts.AlertConfig{
		Enabled:        true,
		StorageDefault: alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
		DockerIgnoredContainerPrefixes: []string{
			"  Foo ",
			"foo",
			"Bar",
			" bar ",
		},
	}

	if err := cp.SaveAlertConfig(cfg); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	expected := []string{"Foo", "Bar"}
	if !reflect.DeepEqual(loaded.DockerIgnoredContainerPrefixes, expected) {
		t.Fatalf("unexpected prefixes: got %v want %v", loaded.DockerIgnoredContainerPrefixes, expected)
	}
}

func TestLoadAlertConfigAppliesDefaults(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	raw := alerts.AlertConfig{
		Enabled:                        false,
		TimeThreshold:                  0,
		TimeThresholds:                 map[string]int{"guest": 0, "node": 0},
		DockerIgnoredContainerPrefixes: []string{" Runner "},
		SnapshotDefaults: alerts.SnapshotAlertConfig{
			Enabled:         true,
			WarningDays:     20,
			CriticalDays:    10,
			WarningSizeGiB:  15,
			CriticalSizeGiB: 8,
		},
		BackupDefaults: alerts.BackupAlertConfig{
			Enabled:      true,
			WarningDays:  12,
			CriticalDays: 8,
		},
		NodeDefaults: alerts.ThresholdConfig{
			Temperature: &alerts.HysteresisThreshold{Trigger: 0, Clear: 0},
		},
	}

	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tempDir, "alerts.json"), data, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := cp.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}

	if loaded.TimeThreshold != 5 {
		t.Fatalf("expected time threshold default 5, got %d", loaded.TimeThreshold)
	}
	if got := loaded.TimeThresholds["guest"]; got != 5 {
		t.Fatalf("expected guest threshold default 5, got %d", got)
	}
	if got := loaded.TimeThresholds["node"]; got != 5 {
		t.Fatalf("expected node threshold default 5, got %d", got)
	}
	if loaded.NodeDefaults.Temperature == nil {
		t.Fatalf("expected node temperature defaults to be set")
	}
	if loaded.NodeDefaults.Temperature.Trigger != 80 || loaded.NodeDefaults.Temperature.Clear != 75 {
		t.Fatalf("expected temperature defaults 80/75, got %+v", loaded.NodeDefaults.Temperature)
	}
	if !loaded.BackupDefaults.Enabled {
		t.Fatalf("expected backup defaults to remain enabled")
	}
	if loaded.BackupDefaults.WarningDays != 8 {
		t.Fatalf("expected backup warning normalized to 8, got %d", loaded.BackupDefaults.WarningDays)
	}
	if loaded.BackupDefaults.CriticalDays != 8 {
		t.Fatalf("expected backup critical normalized to 8, got %d", loaded.BackupDefaults.CriticalDays)
	}
	expectedPrefixes := []string{"Runner"}
	if !reflect.DeepEqual(loaded.DockerIgnoredContainerPrefixes, expectedPrefixes) {
		t.Fatalf("expected normalized prefixes %v, got %v", expectedPrefixes, loaded.DockerIgnoredContainerPrefixes)
	}
	if loaded.SnapshotDefaults.Enabled != true {
		t.Fatalf("expected snapshot defaults to preserve enabled state")
	}
	if loaded.SnapshotDefaults.WarningDays != 10 {
		t.Fatalf("expected snapshot warning days normalized to critical, got %d", loaded.SnapshotDefaults.WarningDays)
	}
	if loaded.SnapshotDefaults.CriticalDays != 10 {
		t.Fatalf("expected snapshot critical days preserved at 10, got %d", loaded.SnapshotDefaults.CriticalDays)
	}
	if loaded.SnapshotDefaults.WarningSizeGiB != 8 {
		t.Fatalf("expected snapshot warning size normalized to 8, got %.1f", loaded.SnapshotDefaults.WarningSizeGiB)
	}
	if loaded.SnapshotDefaults.CriticalSizeGiB != 8 {
		t.Fatalf("expected snapshot critical size preserved at 8, got %.1f", loaded.SnapshotDefaults.CriticalSizeGiB)
	}
}

func TestAppriseConfigPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	cfg := notifications.AppriseConfig{
		Enabled:        true,
		Targets:        []string{"  discord://token  ", "", "mailto://alerts@example.com"},
		CLIPath:        " /usr/local/bin/apprise ",
		TimeoutSeconds: 3,
	}

	if err := cp.SaveAppriseConfig(cfg); err != nil {
		t.Fatalf("SaveAppriseConfig: %v", err)
	}

	loaded, err := cp.LoadAppriseConfig()
	if err != nil {
		t.Fatalf("LoadAppriseConfig: %v", err)
	}

	if !loaded.Enabled {
		t.Fatalf("expected config to remain enabled")
	}

	expectedTargets := []string{"discord://token", "mailto://alerts@example.com"}
	if !reflect.DeepEqual(loaded.Targets, expectedTargets) {
		t.Fatalf("unexpected targets: got %v want %v", loaded.Targets, expectedTargets)
	}

	if loaded.CLIPath != "/usr/local/bin/apprise" {
		t.Fatalf("expected CLI path to be trimmed, got %q", loaded.CLIPath)
	}

	if loaded.TimeoutSeconds != 5 {
		t.Fatalf("expected timeout normalized to minimum 5 seconds, got %d", loaded.TimeoutSeconds)
	}

	// Clearing targets should disable the config on next load
	if err := cp.SaveAppriseConfig(notifications.AppriseConfig{Enabled: true}); err != nil {
		t.Fatalf("SaveAppriseConfig empty: %v", err)
	}

	empty, err := cp.LoadAppriseConfig()
	if err != nil {
		t.Fatalf("LoadAppriseConfig empty: %v", err)
	}
	if empty.Enabled {
		t.Fatalf("expected disabled configuration when no targets stored")
	}
}

func TestExportConfigIncludesAPITokens(t *testing.T) {
	t.Setenv("PULSE_DATA_DIR", t.TempDir())

	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	createdAt := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	tokens := []config.APITokenRecord{
		{
			ID:        "token-1",
			Name:      "automation",
			Hash:      "hash-1",
			Prefix:    "hash-1",
			Suffix:    "-0001",
			CreatedAt: createdAt,
			Scopes:    []string{config.ScopeWildcard},
		},
		{
			ID:        "token-2",
			Name:      "metrics",
			Hash:      "hash-2",
			Prefix:    "hash-2",
			Suffix:    "-0002",
			CreatedAt: createdAt.Add(time.Hour),
			Scopes:    []string{config.ScopeMonitoringRead},
		},
	}

	if err := cp.SaveAPITokens(tokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	passphrase := "strong-passphrase"
	exported, err := cp.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}

	decoded := mustDecodeExport(t, exported, passphrase)

	if decoded.Version != "4.1" {
		t.Fatalf("expected export version 4.1, got %q", decoded.Version)
	}

	assertJSONEqual(t, decoded.APITokens, tokens, "api tokens")
}

func TestImportConfigTransactionalSuccess(t *testing.T) {
	const passphrase = "import-success"

	sourceDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", sourceDataDir)

	sourceConfigDir := t.TempDir()
	source := config.NewConfigPersistence(sourceConfigDir)
	if err := source.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	newNodes := []config.PVEInstance{
		{
			Name:           "pve-new",
			Host:           "https://pve-new.example:8006",
			User:           "root@pam",
			MonitorVMs:     true,
			MonitorStorage: true,
		},
	}
	newPBS := []config.PBSInstance{
		{
			Name:           "pbs-new",
			Host:           "https://pbs-new.example:8007",
			User:           "pbs@pam",
			MonitorBackups: true,
		},
	}
	if err := source.SaveNodesConfig(newNodes, newPBS, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	newAlerts := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 3.5,
		StorageDefault: alerts.HysteresisThreshold{
			Trigger: 70,
			Clear:   65,
		},
		TimeThreshold: 10,
		TimeThresholds: map[string]int{
			"guest":   10,
			"node":    10,
			"storage": 10,
			"pbs":     10,
		},
		Overrides: map[string]alerts.ThresholdConfig{
			"node/pve-new": {
				CPU: &alerts.HysteresisThreshold{Trigger: 80, Clear: 72},
			},
		},
	}
	if err := source.SaveAlertConfig(newAlerts); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	newSystem := config.SystemSettings{
		PBSPollingInterval: 45,
		PMGPollingInterval: 50,
		AutoUpdateEnabled:  true,
		DiscoveryEnabled:   false,
		DiscoverySubnet:    "192.168.10.0/24",
		DiscoveryConfig:    config.DefaultDiscoveryConfig(),
		Theme:              "dark",
		AllowEmbedding:     true,
	}
	if err := source.SaveSystemSettings(newSystem); err != nil {
		t.Fatalf("SaveSystemSettings: %v", err)
	}

	newTokens := []config.APITokenRecord{
		{
			ID:        "token-new-1",
			Name:      "automation",
			Hash:      "hash-new-1",
			Prefix:    "hashn1",
			Suffix:    "n1",
			CreatedAt: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
			Scopes:    []string{config.ScopeMonitoringRead, config.ScopeMonitoringWrite},
		},
	}
	if err := source.SaveAPITokens(newTokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	exported, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}
	exportedData := mustDecodeExport(t, exported, passphrase)

	targetDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", targetDataDir)

	targetConfigDir := t.TempDir()
	target := config.NewConfigPersistence(targetConfigDir)
	if err := target.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	oldNodes := []config.PVEInstance{
		{
			Name: "pve-old",
			Host: "https://pve-old.example:8006",
			User: "root@pam",
		},
	}
	if err := target.SaveNodesConfig(oldNodes, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig baseline: %v", err)
	}

	oldAlerts := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 5,
		StorageDefault: alerts.HysteresisThreshold{
			Trigger: 85,
			Clear:   80,
		},
		Overrides: map[string]alerts.ThresholdConfig{},
	}
	if err := target.SaveAlertConfig(oldAlerts); err != nil {
		t.Fatalf("SaveAlertConfig baseline: %v", err)
	}

	oldSystem := config.SystemSettings{
		PBSPollingInterval: 120,
		PMGPollingInterval: 120,
		AutoUpdateEnabled:  false,
		DiscoveryEnabled:   true,
		DiscoverySubnet:    "auto",
		DiscoveryConfig:    config.DefaultDiscoveryConfig(),
		Theme:              "light",
	}
	if err := target.SaveSystemSettings(oldSystem); err != nil {
		t.Fatalf("SaveSystemSettings baseline: %v", err)
	}

	oldTokens := []config.APITokenRecord{
		{
			ID:        "token-old-1",
			Name:      "legacy",
			Hash:      "hash-old-1",
			Prefix:    "hasho1",
			Suffix:    "o1",
			CreatedAt: time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
			Scopes:    []string{config.ScopeWildcard},
		},
	}
	if err := target.SaveAPITokens(oldTokens); err != nil {
		t.Fatalf("SaveAPITokens baseline: %v", err)
	}

	if err := target.ImportConfig(exported, passphrase); err != nil {
		t.Fatalf("ImportConfig: %v", err)
	}

	nodesAfter, err := target.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}
	assertJSONEqual(t, nodesAfter, exportedData.Nodes, "nodes")

	alertsAfter, err := target.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	assertJSONEqual(t, alertsAfter, exportedData.Alerts, "alerts")

	systemAfter, err := target.LoadSystemSettings()
	if err != nil {
		t.Fatalf("LoadSystemSettings: %v", err)
	}
	if systemAfter == nil {
		t.Fatal("expected system settings after import")
	}
	assertJSONEqual(t, systemAfter, exportedData.System, "system settings")

	tokensAfter, err := target.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	assertJSONEqual(t, tokensAfter, exportedData.APITokens, "api tokens")

	tmpFiles, err := filepath.Glob(filepath.Join(targetConfigDir, "*.tmp"))
	if err != nil {
		t.Fatalf("Glob tmp files: %v", err)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("expected no tmp files after import, found %v", tmpFiles)
	}
}

func TestImportConfigRollbackOnFailure(t *testing.T) {
	const passphrase = "import-rollback"

	sourceDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", sourceDataDir)

	sourceConfigDir := t.TempDir()
	source := config.NewConfigPersistence(sourceConfigDir)
	if err := source.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	newNodes := []config.PVEInstance{
		{
			Name: "pve-new",
			Host: "https://pve-new.example:8006",
			User: "root@pam",
		},
	}
	if err := source.SaveNodesConfig(newNodes, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	newAlerts := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 4,
		StorageDefault: alerts.HysteresisThreshold{
			Trigger: 65,
			Clear:   60,
		},
		Overrides: map[string]alerts.ThresholdConfig{},
	}
	if err := source.SaveAlertConfig(newAlerts); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	newSystem := config.SystemSettings{
		PBSPollingInterval: 30,
		PMGPollingInterval: 30,
		AutoUpdateEnabled:  true,
		DiscoveryEnabled:   false,
		DiscoverySubnet:    "10.20.0.0/24",
		DiscoveryConfig:    config.DefaultDiscoveryConfig(),
	}
	if err := source.SaveSystemSettings(newSystem); err != nil {
		t.Fatalf("SaveSystemSettings: %v", err)
	}

	newTokens := []config.APITokenRecord{
		{
			ID:        "token-new",
			Name:      "new",
			Hash:      "hash-new",
			Prefix:    "hashn",
			Suffix:    "-n",
			CreatedAt: time.Date(2024, 2, 2, 12, 0, 0, 0, time.UTC),
			Scopes:    []string{config.ScopeDockerReport},
		},
	}
	if err := source.SaveAPITokens(newTokens); err != nil {
		t.Fatalf("SaveAPITokens: %v", err)
	}

	exported, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}

	targetDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", targetDataDir)

	targetConfigDir := t.TempDir()
	target := config.NewConfigPersistence(targetConfigDir)
	if err := target.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	baselineNodes := []config.PVEInstance{
		{
			Name: "pve-original",
			Host: "https://pve-original.example:8006",
			User: "root@pam",
		},
	}
	if err := target.SaveNodesConfig(baselineNodes, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig baseline: %v", err)
	}
	baselineAlerts := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 5,
		StorageDefault: alerts.HysteresisThreshold{
			Trigger: 90,
			Clear:   85,
		},
		Overrides: map[string]alerts.ThresholdConfig{},
	}
	if err := target.SaveAlertConfig(baselineAlerts); err != nil {
		t.Fatalf("SaveAlertConfig baseline: %v", err)
	}
	baselineTokens := []config.APITokenRecord{
		{
			ID:        "token-original",
			Name:      "original",
			Hash:      "hash-original",
			Prefix:    "hasho",
			Suffix:    "-o",
			CreatedAt: time.Date(2023, 3, 3, 12, 0, 0, 0, time.UTC),
			Scopes:    []string{config.ScopeWildcard},
		},
	}
	if err := target.SaveAPITokens(baselineTokens); err != nil {
		t.Fatalf("SaveAPITokens baseline: %v", err)
	}

	originalNodes, err := target.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}
	originalNodesJSON := mustMarshalJSON(t, originalNodes)

	originalAlerts, err := target.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	originalAlertsJSON := mustMarshalJSON(t, originalAlerts)

	originalTokens, err := target.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	originalTokensJSON := mustMarshalJSON(t, originalTokens)

	if err := os.Mkdir(filepath.Join(targetConfigDir, "system.json"), 0o700); err != nil {
		t.Fatalf("creating obstacle directory: %v", err)
	}

	if err := target.ImportConfig(exported, passphrase); err == nil {
		t.Fatal("expected import to fail, but it succeeded")
	}

	nodesAfter, err := target.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig after failure: %v", err)
	}
	if !bytes.Equal(mustMarshalJSON(t, nodesAfter), originalNodesJSON) {
		t.Fatalf("nodes changed despite rollback:\noriginal: %s\ncurrent:  %s",
			originalNodesJSON, mustMarshalJSON(t, nodesAfter))
	}

	alertsAfter, err := target.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig after failure: %v", err)
	}
	if !bytes.Equal(mustMarshalJSON(t, alertsAfter), originalAlertsJSON) {
		t.Fatalf("alerts changed despite rollback:\noriginal: %s\ncurrent:  %s",
			originalAlertsJSON, mustMarshalJSON(t, alertsAfter))
	}

	tokensAfter, err := target.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens after failure: %v", err)
	}
	if !bytes.Equal(mustMarshalJSON(t, tokensAfter), originalTokensJSON) {
		t.Fatalf("api tokens changed despite rollback:\noriginal: %s\ncurrent:  %s",
			originalTokensJSON, mustMarshalJSON(t, tokensAfter))
	}

	tmpFiles, err := filepath.Glob(filepath.Join(targetConfigDir, "*.tmp"))
	if err != nil {
		t.Fatalf("Glob tmp files: %v", err)
	}
	if len(tmpFiles) != 0 {
		t.Fatalf("expected tmp files cleaned up after rollback, found %v", tmpFiles)
	}
}

func TestImportAcceptsVersion40Bundle(t *testing.T) {
	const passphrase = "import-legacy"

	sourceDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", sourceDataDir)

	sourceConfigDir := t.TempDir()
	source := config.NewConfigPersistence(sourceConfigDir)
	if err := source.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	newNodes := []config.PVEInstance{
		{
			Name: "pve-legacy",
			Host: "https://pve-legacy.example:8006",
			User: "root@pam",
		},
	}
	if err := source.SaveNodesConfig(newNodes, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	newAlerts := alerts.AlertConfig{
		Enabled:          true,
		HysteresisMargin: 4,
		StorageDefault: alerts.HysteresisThreshold{
			Trigger: 75,
			Clear:   70,
		},
		Overrides: map[string]alerts.ThresholdConfig{},
	}
	if err := source.SaveAlertConfig(newAlerts); err != nil {
		t.Fatalf("SaveAlertConfig: %v", err)
	}

	newSystem := config.SystemSettings{
		PBSPollingInterval: 80,
		PMGPollingInterval: 90,
		AutoUpdateEnabled:  true,
		DiscoveryEnabled:   true,
		DiscoverySubnet:    "172.16.0.0/24",
		DiscoveryConfig:    config.DefaultDiscoveryConfig(),
	}
	if err := source.SaveSystemSettings(newSystem); err != nil {
		t.Fatalf("SaveSystemSettings: %v", err)
	}

	exported, err := source.ExportConfig(passphrase)
	if err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}

	exportData := mustDecodeExport(t, exported, passphrase)
	exportData.Version = "4.0"
	exportData.APITokens = nil

	legacyPayload := mustEncodeExport(t, exportData, passphrase)

	targetDataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", targetDataDir)

	targetConfigDir := t.TempDir()
	target := config.NewConfigPersistence(targetConfigDir)
	if err := target.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	baselineTokens := []config.APITokenRecord{
		{
			ID:        "token-legacy",
			Name:      "keep-me",
			Hash:      "hash-keep",
			Prefix:    "hashk",
			Suffix:    "-k",
			CreatedAt: time.Date(2022, 4, 4, 12, 0, 0, 0, time.UTC),
			Scopes:    []string{config.ScopeWildcard},
		},
	}
	if err := target.SaveAPITokens(baselineTokens); err != nil {
		t.Fatalf("SaveAPITokens baseline: %v", err)
	}

	if err := target.ImportConfig(legacyPayload, passphrase); err != nil {
		t.Fatalf("ImportConfig (legacy 4.0): %v", err)
	}

	nodesAfter, err := target.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}
	assertJSONEqual(t, nodesAfter, exportData.Nodes, "nodes (4.0 import)")

	alertsAfter, err := target.LoadAlertConfig()
	if err != nil {
		t.Fatalf("LoadAlertConfig: %v", err)
	}
	assertJSONEqual(t, alertsAfter, exportData.Alerts, "alerts (4.0 import)")

	systemAfter, err := target.LoadSystemSettings()
	if err != nil {
		t.Fatalf("LoadSystemSettings: %v", err)
	}
	if systemAfter == nil {
		t.Fatal("expected system settings after legacy import")
	}
	assertJSONEqual(t, systemAfter, exportData.System, "system settings (4.0 import)")

	tokensAfter, err := target.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens: %v", err)
	}
	assertJSONEqual(t, tokensAfter, baselineTokens, "api tokens unchanged for 4.0 import")
}

func TestLoadNodesConfigNormalizesPVEHostsAndClusterEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	pveNodes := []config.PVEInstance{
		{
			Name: "pve-cluster",
			Host: "https://pve.local",
			ClusterEndpoints: []config.ClusterEndpoint{
				{NodeName: "pve1", Host: "https://pve1.local"},
				{NodeName: "pve2", Host: "pve2.local"},
			},
		},
	}

	if err := cp.SaveNodesConfig(pveNodes, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	loaded, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}

	if len(loaded.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(loaded.PVEInstances))
	}

	pve := loaded.PVEInstances[0]
	if pve.Host != "https://pve.local:8006" {
		t.Fatalf("expected primary host normalized with default port, got %q", pve.Host)
	}

	if len(pve.ClusterEndpoints) != 2 {
		t.Fatalf("expected 2 cluster endpoints, got %d", len(pve.ClusterEndpoints))
	}

	if pve.ClusterEndpoints[0].Host != "https://pve1.local:8006" {
		t.Fatalf("expected endpoint host normalized, got %q", pve.ClusterEndpoints[0].Host)
	}
	if pve.ClusterEndpoints[1].Host != "https://pve2.local:8006" {
		t.Fatalf("expected endpoint host normalized, got %q", pve.ClusterEndpoints[1].Host)
	}

	// Second load should keep normalized values and not panic on migration.
	loadedAgain, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig second read: %v", err)
	}
	if loadedAgain.PVEInstances[0].Host != "https://pve.local:8006" {
		t.Fatalf("expected normalized host persisted, got %q", loadedAgain.PVEInstances[0].Host)
	}
}

func mustDecodeExport(t *testing.T, payload, passphrase string) config.ExportData {
	t.Helper()

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}

	plaintext, err := decryptExportPayload(raw, passphrase)
	if err != nil {
		t.Fatalf("decrypt export: %v", err)
	}

	var data config.ExportData
	if err := json.Unmarshal(plaintext, &data); err != nil {
		t.Fatalf("unmarshal export data: %v", err)
	}
	return data
}

func mustEncodeExport(t *testing.T, data config.ExportData, passphrase string) string {
	t.Helper()

	plaintext, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal export data: %v", err)
	}

	ciphertext, err := encryptExportPayload(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encrypt export data: %v", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext)
}

func encryptExportPayload(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	result := append(salt, ciphertext...)
	return result, nil
}

func decryptExportPayload(ciphertext []byte, passphrase string) ([]byte, error) {
	if len(ciphertext) < 32 {
		return nil, io.ErrUnexpectedEOF
	}

	salt := ciphertext[:32]
	cipherbody := ciphertext[32:]

	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(cipherbody) < gcm.NonceSize() {
		return nil, io.ErrUnexpectedEOF
	}

	nonce := cipherbody[:gcm.NonceSize()]
	payload := cipherbody[gcm.NonceSize():]

	return gcm.Open(nil, nonce, payload, nil)
}

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func assertJSONEqual(t *testing.T, got interface{}, want interface{}, context string) {
	t.Helper()

	gotJSON := mustMarshalJSON(t, got)
	wantJSON := mustMarshalJSON(t, want)

	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("%s mismatch:\n got: %s\nwant: %s", context, gotJSON, wantJSON)
	}
}

// ============================================================================
// Error path and edge case tests for persistence functions
// ============================================================================

func TestLoadAPITokensErrorInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid JSON to the api_tokens.json file
	tokensFile := filepath.Join(tempDir, "api_tokens.json")
	if err := os.WriteFile(tokensFile, []byte(`{invalid json content`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadAPITokens()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadAPITokensEmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write empty file
	tokensFile := filepath.Join(tempDir, "api_tokens.json")
	if err := os.WriteFile(tokensFile, []byte{}, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tokens, err := cp.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens returned error for empty file: %v", err)
	}

	if len(tokens) != 0 {
		t.Fatalf("expected empty slice for empty file, got %d tokens", len(tokens))
	}
}

func TestLoadAPITokensFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test that non-existent file returns empty slice
	tokens, err := cp.LoadAPITokens()
	if err != nil {
		t.Fatalf("LoadAPITokens returned error for non-existent file: %v", err)
	}

	if len(tokens) != 0 {
		t.Fatalf("expected empty slice for non-existent file, got %d tokens", len(tokens))
	}
}

func TestLoadEmailConfigErrorInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid JSON to the email.enc file (unencrypted for test without crypto)
	emailFile := filepath.Join(tempDir, "email.enc")
	if err := os.WriteFile(emailFile, []byte(`not valid json {{{{`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadEmailConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON/decryption failure, got nil")
	}
}

func TestLoadEmailConfigFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test default config is returned
	cfg, err := cp.LoadEmailConfig()
	if err != nil {
		t.Fatalf("LoadEmailConfig returned error for non-existent file: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected default config, got nil")
	}
	if cfg.Enabled {
		t.Fatal("expected Enabled=false for default config")
	}
	if cfg.SMTPPort != 587 {
		t.Fatalf("expected default SMTPPort=587, got %d", cfg.SMTPPort)
	}
}

func TestLoadEmailConfig_EncryptedRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Create a config with all fields populated
	original := notifications.EmailConfig{
		Enabled:  true,
		Provider: "smtp",
		SMTPHost: "mail.example.com",
		SMTPPort: 465,
		Username: "user@example.com",
		Password: "secret-password",
		From:     "alerts@example.com",
		To:       []string{"admin@example.com", "ops@example.com"},
		TLS:      true,
		StartTLS: false,
	}

	// Save (encrypts) then Load (decrypts)
	if err := cp.SaveEmailConfig(original); err != nil {
		t.Fatalf("SaveEmailConfig: %v", err)
	}

	loaded, err := cp.LoadEmailConfig()
	if err != nil {
		t.Fatalf("LoadEmailConfig: %v", err)
	}

	// Verify round-trip preserved all fields
	if loaded.Enabled != original.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", loaded.Enabled, original.Enabled)
	}
	if loaded.Provider != original.Provider {
		t.Errorf("Provider mismatch: got %v, want %v", loaded.Provider, original.Provider)
	}
	if loaded.SMTPHost != original.SMTPHost {
		t.Errorf("SMTPHost mismatch: got %v, want %v", loaded.SMTPHost, original.SMTPHost)
	}
	if loaded.SMTPPort != original.SMTPPort {
		t.Errorf("SMTPPort mismatch: got %v, want %v", loaded.SMTPPort, original.SMTPPort)
	}
	if loaded.Username != original.Username {
		t.Errorf("Username mismatch: got %v, want %v", loaded.Username, original.Username)
	}
	if loaded.Password != original.Password {
		t.Errorf("Password mismatch: got %v, want %v", loaded.Password, original.Password)
	}
	if loaded.From != original.From {
		t.Errorf("From mismatch: got %v, want %v", loaded.From, original.From)
	}
	if len(loaded.To) != len(original.To) {
		t.Errorf("To length mismatch: got %d, want %d", len(loaded.To), len(original.To))
	}
	if loaded.TLS != original.TLS {
		t.Errorf("TLS mismatch: got %v, want %v", loaded.TLS, original.TLS)
	}
	if loaded.StartTLS != original.StartTLS {
		t.Errorf("StartTLS mismatch: got %v, want %v", loaded.StartTLS, original.StartTLS)
	}
}

func TestLoadWebhooksErrorInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid JSON to the webhooks.enc file
	webhooksFile := filepath.Join(tempDir, "webhooks.enc")
	if err := os.WriteFile(webhooksFile, []byte(`[{"broken`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadWebhooks()
	if err == nil {
		t.Fatal("expected error for invalid JSON/decryption failure, got nil")
	}
}

func TestLoadWebhooksFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test empty slice is returned
	webhooks, err := cp.LoadWebhooks()
	if err != nil {
		t.Fatalf("LoadWebhooks returned error for non-existent file: %v", err)
	}

	if len(webhooks) != 0 {
		t.Fatalf("expected empty slice for non-existent file, got %d webhooks", len(webhooks))
	}
}

func TestLoadWebhooksMigrationFromLegacyFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Create legacy webhooks.json file (unencrypted)
	legacyWebhooks := []notifications.WebhookConfig{
		{
			ID:      "webhook-1",
			Name:    "test-webhook",
			URL:     "https://example.com/hook",
			Method:  "POST",
			Enabled: true,
		},
	}
	legacyData, err := json.Marshal(legacyWebhooks)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	legacyFile := filepath.Join(tempDir, "webhooks.json")
	if err := os.WriteFile(legacyFile, legacyData, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// LoadWebhooks should find and parse the legacy file
	webhooks, err := cp.LoadWebhooks()
	if err != nil {
		t.Fatalf("LoadWebhooks returned error: %v", err)
	}

	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook from legacy file, got %d", len(webhooks))
	}

	if webhooks[0].ID != "webhook-1" {
		t.Fatalf("expected webhook ID 'webhook-1', got %q", webhooks[0].ID)
	}
}

func TestLoadWebhooksMigrationFromUnencryptedEncFile(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write plain JSON to webhooks.enc (migration scenario where file
	// was written before encryption was enabled)
	plainWebhooks := []notifications.WebhookConfig{
		{
			ID:      "unencrypted-webhook",
			Name:    "plain-webhook",
			URL:     "https://example.com/plain",
			Method:  "POST",
			Enabled: true,
		},
	}
	plainData, err := json.Marshal(plainWebhooks)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	encFile := filepath.Join(tempDir, "webhooks.enc")
	if err := os.WriteFile(encFile, plainData, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// LoadWebhooks should fall back to parsing as plain JSON when decryption fails
	webhooks, err := cp.LoadWebhooks()
	if err != nil {
		t.Fatalf("LoadWebhooks returned error: %v", err)
	}

	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(webhooks))
	}

	if webhooks[0].ID != "unencrypted-webhook" {
		t.Fatalf("expected ID 'unencrypted-webhook', got %q", webhooks[0].ID)
	}
}

func TestLoadWebhooksLegacyFileInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Create legacy file with invalid JSON - should be ignored, return empty
	legacyFile := filepath.Join(tempDir, "webhooks.json")
	if err := os.WriteFile(legacyFile, []byte(`{invalid json`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Should return empty slice since legacy file is invalid
	webhooks, err := cp.LoadWebhooks()
	if err != nil {
		t.Fatalf("LoadWebhooks returned error: %v", err)
	}

	if len(webhooks) != 0 {
		t.Fatalf("expected empty slice for invalid legacy file, got %d webhooks", len(webhooks))
	}
}

func TestLoadNodesConfigEmptyArrays(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Use SaveNodesConfigAllowEmpty with empty slices to properly encrypt the data
	if err := cp.SaveNodesConfigAllowEmpty([]config.PVEInstance{}, []config.PBSInstance{}, []config.PMGInstance{}); err != nil {
		t.Fatalf("SaveNodesConfigAllowEmpty: %v", err)
	}

	cfg, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig returned error: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected empty PVEInstances, got %d", len(cfg.PVEInstances))
	}
	if len(cfg.PBSInstances) != 0 {
		t.Fatalf("expected empty PBSInstances, got %d", len(cfg.PBSInstances))
	}
	if len(cfg.PMGInstances) != 0 {
		t.Fatalf("expected empty PMGInstances, got %d", len(cfg.PMGInstances))
	}
}

func TestLoadNodesConfigMissingFields(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Save config with only PVE, no PBS and PMG
	// When we load it back, PBS and PMG should be initialized to empty slices
	pveInstances := []config.PVEInstance{
		{
			Name: "pve-test",
			Host: "https://pve.local:8006",
			User: "root@pam",
		},
	}

	// Save only PVE, pass nil for PBS and PMG
	if err := cp.SaveNodesConfig(pveInstances, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	cfg, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig returned error: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(cfg.PVEInstances))
	}
	// PBS and PMG should be initialized to empty slices
	if cfg.PBSInstances == nil {
		t.Fatal("expected PBSInstances to be initialized (not nil)")
	}
	if cfg.PMGInstances == nil {
		t.Fatal("expected PMGInstances to be initialized (not nil)")
	}
}

func TestLoadNodesConfigCorruptedRecoversWithEmptyConfig(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write corrupted data (invalid encrypted content)
	// This tests the recovery behavior: when nodes.enc is corrupted and no backup exists,
	// LoadNodesConfig returns an empty config instead of an error to allow system startup
	nodesFile := filepath.Join(tempDir, "nodes.enc")
	if err := os.WriteFile(nodesFile, []byte(`{broken json`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig should recover gracefully from corruption, got error: %v", err)
	}

	// Verify we got an empty config (recovery behavior)
	if cfg == nil {
		t.Fatal("expected empty config on recovery, got nil")
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected empty PVEInstances on recovery, got %d", len(cfg.PVEInstances))
	}
}

func TestLoadNodesConfigFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test that empty config is returned
	cfg, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig returned error for non-existent file: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected empty config, got nil")
	}
	if len(cfg.PVEInstances) != 0 {
		t.Fatalf("expected empty PVEInstances, got %d", len(cfg.PVEInstances))
	}
}

func TestLoadNodesConfig_PBSTokenClearing(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Save PBS instance with both Password and TokenName (buggy config)
	pbsInstances := []config.PBSInstance{
		{
			Name:           "pbs-buggy",
			Host:           "https://pbs.local:8007",
			User:           "root@pam",
			Password:       "secret-password",
			TokenName:      "should-be-cleared",
			TokenValue:     "also-cleared",
			MonitorBackups: true, // Already set so no migration needed for this
		},
	}

	if err := cp.SaveNodesConfig(nil, pbsInstances, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	loaded, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}

	if len(loaded.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(loaded.PBSInstances))
	}

	pbs := loaded.PBSInstances[0]
	// TokenName/TokenValue should be cleared since Password is set
	if pbs.TokenName != "" {
		t.Errorf("expected TokenName cleared, got %q", pbs.TokenName)
	}
	if pbs.TokenValue != "" {
		t.Errorf("expected TokenValue cleared, got %q", pbs.TokenValue)
	}
	// Password should remain
	if pbs.Password != "secret-password" {
		t.Errorf("expected Password preserved, got %q", pbs.Password)
	}
}

func TestLoadNodesConfig_PBSHostNormalization(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Save PBS instance without port (should be normalized to :8007)
	pbsInstances := []config.PBSInstance{
		{
			Name:           "pbs-noport",
			Host:           "https://pbs.local",
			User:           "root@pam",
			Password:       "pass",
			MonitorBackups: true,
		},
	}

	if err := cp.SaveNodesConfig(nil, pbsInstances, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	loaded, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}

	if len(loaded.PBSInstances) != 1 {
		t.Fatalf("expected 1 PBS instance, got %d", len(loaded.PBSInstances))
	}

	pbs := loaded.PBSInstances[0]
	if pbs.Host != "https://pbs.local:8007" {
		t.Errorf("expected PBS host normalized with port, got %q", pbs.Host)
	}
}

func TestLoadNodesConfig_PMGTokenClearing(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Save PMG instance with both Password and TokenName (buggy config)
	pmgInstances := []config.PMGInstance{
		{
			Name:       "pmg-buggy",
			Host:       "https://pmg.local:8006",
			User:       "root@pam",
			Password:   "secret-password",
			TokenName:  "should-be-cleared",
			TokenValue: "also-cleared",
		},
	}

	if err := cp.SaveNodesConfig(nil, nil, pmgInstances); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	loaded, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}

	if len(loaded.PMGInstances) != 1 {
		t.Fatalf("expected 1 PMG instance, got %d", len(loaded.PMGInstances))
	}

	pmg := loaded.PMGInstances[0]
	// TokenName/TokenValue should be cleared since Password is set
	if pmg.TokenName != "" {
		t.Errorf("expected TokenName cleared, got %q", pmg.TokenName)
	}
	if pmg.TokenValue != "" {
		t.Errorf("expected TokenValue cleared, got %q", pmg.TokenValue)
	}
	// Password should remain
	if pmg.Password != "secret-password" {
		t.Errorf("expected Password preserved, got %q", pmg.Password)
	}
}

func TestLoadNodesConfig_PMGHostNormalization(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Save PMG instance without port (should be normalized to :8006)
	pmgInstances := []config.PMGInstance{
		{
			Name:     "pmg-noport",
			Host:     "https://pmg.local",
			User:     "root@pam",
			Password: "pass",
		},
	}

	if err := cp.SaveNodesConfig(nil, nil, pmgInstances); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	loaded, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}

	if len(loaded.PMGInstances) != 1 {
		t.Fatalf("expected 1 PMG instance, got %d", len(loaded.PMGInstances))
	}

	pmg := loaded.PMGInstances[0]
	if pmg.Host != "https://pmg.local:8006" {
		t.Errorf("expected PMG host normalized with port, got %q", pmg.Host)
	}
}

// NOTE: TestSaveNodesConfig_BlocksEmptyWhenNodesExist is not included because
// the saveNodesConfig function has a deadlock bug - it holds c.mu.Lock() while
// calling LoadNodesConfig() which tries to acquire c.mu.RLock(). This makes
// the empty config protection path untestable without first fixing the bug.

func TestSaveNodesConfigAllowEmpty_PermitsEmptyConfig(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// First save a valid config with nodes
	pveInstances := []config.PVEInstance{
		{
			Name: "pve-test",
			Host: "https://pve.local:8006",
			User: "root@pam",
		},
	}

	if err := cp.SaveNodesConfig(pveInstances, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig initial: %v", err)
	}

	// SaveNodesConfigAllowEmpty should permit deleting all nodes
	err := cp.SaveNodesConfigAllowEmpty(nil, nil, nil)
	if err != nil {
		t.Fatalf("SaveNodesConfigAllowEmpty should permit empty config, got: %v", err)
	}

	// Verify nodes are now empty
	loaded, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}
	if len(loaded.PVEInstances) != 0 {
		t.Errorf("expected 0 PVE instances after AllowEmpty save, got %d", len(loaded.PVEInstances))
	}
}

func TestCleanupOldBackupsNonExistentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Save a valid config so we can trigger cleanup
	pveInstances := []config.PVEInstance{
		{
			Name: "pve-test",
			Host: "https://pve.local:8006",
			User: "root@pam",
		},
	}

	// First save to create the file
	if err := cp.SaveNodesConfig(pveInstances, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	// Cleanup with non-existent pattern should not error
	// The pattern won't match anything, but it shouldn't panic or error
	// This is implicitly tested by the SaveNodesConfig which calls cleanupOldBackups
	// We just verify no panic occurred and the config was saved

	cfg, err := cp.LoadNodesConfig()
	if err != nil {
		t.Fatalf("LoadNodesConfig: %v", err)
	}
	if len(cfg.PVEInstances) != 1 {
		t.Fatalf("expected 1 PVE instance, got %d", len(cfg.PVEInstances))
	}
}

func TestCleanupOldBackupsMultipleFiles(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	nodesFile := filepath.Join(tempDir, "nodes.enc")

	// Create initial file
	initialData := []byte(`{"pveInstances":[],"pbsInstances":[],"pmgInstances":[]}`)
	if err := os.WriteFile(nodesFile, initialData, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Create 15 timestamped backup files (more than the 10 limit)
	baseTime := time.Now()
	for i := 0; i < 15; i++ {
		backupTime := baseTime.Add(time.Duration(-i) * time.Hour)
		backupFile := fmt.Sprintf("%s.backup-%s", nodesFile, backupTime.Format("20060102-150405"))
		content := fmt.Sprintf(`{"backup": %d}`, i)
		if err := os.WriteFile(backupFile, []byte(content), 0600); err != nil {
			t.Fatalf("WriteFile backup %d: %v", i, err)
		}
		// Set modification time to simulate different ages
		if err := os.Chtimes(backupFile, backupTime, backupTime); err != nil {
			t.Fatalf("Chtimes: %v", err)
		}
	}

	// Verify 15 backups exist
	matches, err := filepath.Glob(nodesFile + ".backup-*")
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) != 15 {
		t.Fatalf("expected 15 backup files, got %d", len(matches))
	}

	// Now save a new config, which should trigger cleanup
	pveInstances := []config.PVEInstance{
		{
			Name: "pve-test",
			Host: "https://pve.local:8006",
			User: "root@pam",
		},
	}
	if err := cp.SaveNodesConfig(pveInstances, nil, nil); err != nil {
		t.Fatalf("SaveNodesConfig: %v", err)
	}

	// Verify that old backups were cleaned up (should have at most 10 + 1 new = 11)
	// Actually the cleanup runs before the new backup is created, so we should have 10 old + 1 new = 11
	matches, err = filepath.Glob(nodesFile + ".backup-*")
	if err != nil {
		t.Fatalf("Glob after cleanup: %v", err)
	}

	// After cleanup, we should have max 10 old backups + 1 new backup = 11
	if len(matches) > 11 {
		t.Fatalf("expected at most 11 backup files after cleanup, got %d", len(matches))
	}
}

func TestLoadAppriseConfigErrorInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid JSON to the apprise.enc file
	appriseFile := filepath.Join(tempDir, "apprise.enc")
	if err := os.WriteFile(appriseFile, []byte(`{not valid}`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadAppriseConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON/decryption failure, got nil")
	}
}

func TestLoadAppriseConfigFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test default config is returned
	cfg, err := cp.LoadAppriseConfig()
	if err != nil {
		t.Fatalf("LoadAppriseConfig returned error for non-existent file: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected default config, got nil")
	}
	if cfg.Enabled {
		t.Fatal("expected Enabled=false for default config")
	}
	if cfg.TimeoutSeconds != 15 {
		t.Fatalf("expected default TimeoutSeconds=15, got %d", cfg.TimeoutSeconds)
	}
}

func TestLoadAlertConfigErrorInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid JSON to the alerts.json file
	alertsFile := filepath.Join(tempDir, "alerts.json")
	if err := os.WriteFile(alertsFile, []byte(`{"broken": `), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadAlertConfig()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadSystemSettingsErrorInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid JSON to the system.json file
	systemFile := filepath.Join(tempDir, "system.json")
	if err := os.WriteFile(systemFile, []byte(`not json at all`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadSystemSettings()
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadSystemSettingsFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test nil is returned (env vars take precedence)
	settings, err := cp.LoadSystemSettings()
	if err != nil {
		t.Fatalf("LoadSystemSettings returned error for non-existent file: %v", err)
	}

	if settings != nil {
		t.Fatal("expected nil for non-existent system settings file")
	}
}

// ============================================================================
// LoadOIDCConfig error paths and success cases
// ============================================================================

func TestLoadOIDCConfigFileNotExist(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Don't create the file - test that nil, nil is returned
	cfg, err := cp.LoadOIDCConfig()
	if err != nil {
		t.Fatalf("LoadOIDCConfig returned error for non-existent file: %v", err)
	}

	if cfg != nil {
		t.Fatal("expected nil config for non-existent file, got non-nil")
	}
}

func TestLoadOIDCConfigFileReadError(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Create oidc.enc as a directory to trigger a read error (not IsNotExist)
	oidcFile := filepath.Join(tempDir, "oidc.enc")
	if err := os.Mkdir(oidcFile, 0700); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	_, err := cp.LoadOIDCConfig()
	if err == nil {
		t.Fatal("expected error when reading directory as file, got nil")
	}
}

func TestLoadOIDCConfigValidJSONWithoutEncryption(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Use SaveOIDCConfig to properly save (and encrypt) the config,
	// then LoadOIDCConfig should be able to load it back
	expected := config.OIDCConfig{
		Enabled:       true,
		IssuerURL:     "https://auth.example.com",
		ClientID:      "test-client-id",
		ClientSecret:  "test-client-secret",
		RedirectURL:   "https://app.example.com/callback",
		Scopes:        []string{"openid", "email", "profile"},
		UsernameClaim: "preferred_username",
	}

	if err := cp.SaveOIDCConfig(expected); err != nil {
		t.Fatalf("SaveOIDCConfig: %v", err)
	}

	loaded, err := cp.LoadOIDCConfig()
	if err != nil {
		t.Fatalf("LoadOIDCConfig: %v", err)
	}

	if loaded == nil {
		t.Fatal("expected non-nil config, got nil")
	}
	if loaded.Enabled != expected.Enabled {
		t.Fatalf("Enabled mismatch: got %v want %v", loaded.Enabled, expected.Enabled)
	}
	if loaded.IssuerURL != expected.IssuerURL {
		t.Fatalf("IssuerURL mismatch: got %q want %q", loaded.IssuerURL, expected.IssuerURL)
	}
	if loaded.ClientID != expected.ClientID {
		t.Fatalf("ClientID mismatch: got %q want %q", loaded.ClientID, expected.ClientID)
	}
	if loaded.ClientSecret != expected.ClientSecret {
		t.Fatalf("ClientSecret mismatch: got %q want %q", loaded.ClientSecret, expected.ClientSecret)
	}
	if loaded.RedirectURL != expected.RedirectURL {
		t.Fatalf("RedirectURL mismatch: got %q want %q", loaded.RedirectURL, expected.RedirectURL)
	}
	if loaded.UsernameClaim != expected.UsernameClaim {
		t.Fatalf("UsernameClaim mismatch: got %q want %q", loaded.UsernameClaim, expected.UsernameClaim)
	}
	if !reflect.DeepEqual(loaded.Scopes, expected.Scopes) {
		t.Fatalf("Scopes mismatch: got %v want %v", loaded.Scopes, expected.Scopes)
	}
}

func TestLoadOIDCConfigInvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write invalid data to oidc.enc file
	// Since crypto is enabled, writing raw JSON will cause a decryption error first
	// To test JSON unmarshal error, we need to bypass encryption or test the path
	// where crypto is nil. Writing garbage data will trigger decrypt error which
	// covers the decrypt error path.
	oidcFile := filepath.Join(tempDir, "oidc.enc")
	if err := os.WriteFile(oidcFile, []byte(`{invalid json content`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadOIDCConfig()
	if err == nil {
		t.Fatal("expected error for invalid data, got nil")
	}
}

func TestLoadOIDCConfigDecryptionError(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Write data that will fail decryption (not valid encrypted format)
	oidcFile := filepath.Join(tempDir, "oidc.enc")
	if err := os.WriteFile(oidcFile, []byte(`corrupted encrypted data`), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := cp.LoadOIDCConfig()
	if err == nil {
		t.Fatal("expected decryption error, got nil")
	}
}

func TestLoadOIDCConfigRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	cp := config.NewConfigPersistence(tempDir)
	if err := cp.EnsureConfigDir(); err != nil {
		t.Fatalf("EnsureConfigDir: %v", err)
	}

	// Test full round-trip with encryption
	original := config.OIDCConfig{
		Enabled:        true,
		IssuerURL:      "https://idp.example.org/realms/myrealm",
		ClientID:       "my-app",
		ClientSecret:   "super-secret-value",
		RedirectURL:    "https://myapp.example.org/auth/callback",
		LogoutURL:      "https://idp.example.org/realms/myrealm/protocol/openid-connect/logout",
		Scopes:         []string{"openid", "email", "profile", "groups"},
		UsernameClaim:  "preferred_username",
		EmailClaim:     "email",
		GroupsClaim:    "groups",
		AllowedGroups:  []string{"admin", "users"},
		AllowedDomains: []string{"example.org"},
		AllowedEmails:  []string{"admin@example.org"},
		CABundle:       "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	}

	if err := cp.SaveOIDCConfig(original); err != nil {
		t.Fatalf("SaveOIDCConfig: %v", err)
	}

	loaded, err := cp.LoadOIDCConfig()
	if err != nil {
		t.Fatalf("LoadOIDCConfig: %v", err)
	}

	if loaded == nil {
		t.Fatal("expected config, got nil")
	}

	// Verify all fields are preserved through the round-trip
	if loaded.Enabled != original.Enabled {
		t.Errorf("Enabled: got %v want %v", loaded.Enabled, original.Enabled)
	}
	if loaded.IssuerURL != original.IssuerURL {
		t.Errorf("IssuerURL: got %q want %q", loaded.IssuerURL, original.IssuerURL)
	}
	if loaded.ClientID != original.ClientID {
		t.Errorf("ClientID: got %q want %q", loaded.ClientID, original.ClientID)
	}
	if loaded.ClientSecret != original.ClientSecret {
		t.Errorf("ClientSecret: got %q want %q", loaded.ClientSecret, original.ClientSecret)
	}
	if loaded.RedirectURL != original.RedirectURL {
		t.Errorf("RedirectURL: got %q want %q", loaded.RedirectURL, original.RedirectURL)
	}
	if loaded.LogoutURL != original.LogoutURL {
		t.Errorf("LogoutURL: got %q want %q", loaded.LogoutURL, original.LogoutURL)
	}
	if !reflect.DeepEqual(loaded.Scopes, original.Scopes) {
		t.Errorf("Scopes: got %v want %v", loaded.Scopes, original.Scopes)
	}
	if loaded.UsernameClaim != original.UsernameClaim {
		t.Errorf("UsernameClaim: got %q want %q", loaded.UsernameClaim, original.UsernameClaim)
	}
	if loaded.EmailClaim != original.EmailClaim {
		t.Errorf("EmailClaim: got %q want %q", loaded.EmailClaim, original.EmailClaim)
	}
	if loaded.GroupsClaim != original.GroupsClaim {
		t.Errorf("GroupsClaim: got %q want %q", loaded.GroupsClaim, original.GroupsClaim)
	}
	if !reflect.DeepEqual(loaded.AllowedGroups, original.AllowedGroups) {
		t.Errorf("AllowedGroups: got %v want %v", loaded.AllowedGroups, original.AllowedGroups)
	}
	if !reflect.DeepEqual(loaded.AllowedDomains, original.AllowedDomains) {
		t.Errorf("AllowedDomains: got %v want %v", loaded.AllowedDomains, original.AllowedDomains)
	}
	if !reflect.DeepEqual(loaded.AllowedEmails, original.AllowedEmails) {
		t.Errorf("AllowedEmails: got %v want %v", loaded.AllowedEmails, original.AllowedEmails)
	}
	if loaded.CABundle != original.CABundle {
		t.Errorf("CABundle: got %q want %q", loaded.CABundle, original.CABundle)
	}
}
