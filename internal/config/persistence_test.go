package config_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
