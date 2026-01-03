package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAlertConfig_Normalization(t *testing.T) {
	tempDir := t.TempDir()
	cp := NewConfigPersistence(tempDir)
	alertFile := filepath.Join(tempDir, "alerts.json")

	tests := []struct {
		name   string
		input  interface{}
		verify func(*testing.T, *alerts.AlertConfig)
	}{
		{
			name:  "Empty JSON enabling by default",
			input: map[string]interface{}{},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.True(t, cfg.Enabled)
			},
		},
		{
			name: "StorageDefault negative trigger",
			input: map[string]interface{}{
				"storageDefault": map[string]interface{}{"trigger": -1},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 85.0, cfg.StorageDefault.Trigger)
				assert.Equal(t, 80.0, cfg.StorageDefault.Clear)
			},
		},
		{
			name: "StorageDefault zero trigger",
			input: map[string]interface{}{
				"storageDefault": map[string]interface{}{"trigger": 0},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0.0, cfg.StorageDefault.Trigger)
				assert.Equal(t, 0.0, cfg.StorageDefault.Clear)
			},
		},
		{
			name: "StorageDefault missing clear",
			input: map[string]interface{}{
				"storageDefault": map[string]interface{}{"trigger": 50},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 50.0, cfg.StorageDefault.Trigger)
				assert.Equal(t, 45.0, cfg.StorageDefault.Clear)
			},
		},
		{
			name: "MinimumDelta zero",
			input: map[string]interface{}{
				"minimumDelta": 0,
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 2.0, cfg.MinimumDelta)
			},
		},
		{
			name: "SuppressionWindow zero",
			input: map[string]interface{}{
				"suppressionWindow": 0,
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 5, cfg.SuppressionWindow)
			},
		},
		{
			name: "HysteresisMargin zero",
			input: map[string]interface{}{
				"hysteresisMargin": 0,
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 5.0, cfg.HysteresisMargin)
			},
		},
		{
			name: "NodeDefaults Temperature nil",
			input: map[string]interface{}{
				"nodeDefaults": map[string]interface{}{},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.NotNil(t, cfg.NodeDefaults.Temperature)
				assert.Equal(t, 80.0, cfg.NodeDefaults.Temperature.Trigger)
			},
		},
		{
			name: "HostDefaults CPU negative",
			input: map[string]interface{}{
				"hostDefaults": map[string]interface{}{"cpu": map[string]interface{}{"trigger": -1}},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 80.0, cfg.HostDefaults.CPU.Trigger)
			},
		},
		{
			name: "HostDefaults CPU zero",
			input: map[string]interface{}{
				"hostDefaults": map[string]interface{}{"cpu": map[string]interface{}{"trigger": 0}},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0.0, cfg.HostDefaults.CPU.Trigger)
				assert.Equal(t, 0.0, cfg.HostDefaults.CPU.Clear)
			},
		},
		{
			name: "TimeThreshold and TimeThresholds",
			input: map[string]interface{}{
				"timeThreshold": 0,
				"timeThresholds": map[string]interface{}{
					"guest": 0,
					"all":   0,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 5, cfg.TimeThreshold)
				assert.Equal(t, 5, cfg.TimeThresholds["guest"])
				assert.Equal(t, 5, cfg.TimeThresholds["all"])
			},
		},
		{
			name: "SnapshotDefaults negative days and size",
			input: map[string]interface{}{
				"snapshotDefaults": map[string]interface{}{
					"warningDays":     -1,
					"criticalDays":    10,
					"warningSizeGiB":  20,
					"criticalSizeGiB": 10,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0, cfg.SnapshotDefaults.WarningDays)
				assert.Equal(t, 10.0, cfg.SnapshotDefaults.WarningSizeGiB)
			},
		},
		{
			name: "SnapshotDefaults critical size zero warning size positive",
			input: map[string]interface{}{
				"snapshotDefaults": map[string]interface{}{
					"warningSizeGiB":  10,
					"criticalSizeGiB": 0,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 10.0, cfg.SnapshotDefaults.CriticalSizeGiB)
			},
		},
		{
			name: "BackupDefaults negative and stale < fresh",
			input: map[string]interface{}{
				"backupDefaults": map[string]interface{}{
					"warningDays": -1,
					"freshHours":  48,
					"staleHours":  24,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0, cfg.BackupDefaults.WarningDays)
				assert.Equal(t, 48, cfg.BackupDefaults.StaleHours)
			},
		},
		{
			name: "GuestDefaults migration",
			input: map[string]interface{}{
				"guestDefaults": map[string]interface{}{
					"diskRead":   map[string]interface{}{"trigger": 150},
					"diskWrite":  map[string]interface{}{"trigger": 150},
					"networkIn":  map[string]interface{}{"trigger": 200},
					"networkOut": map[string]interface{}{"trigger": 200},
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0.0, cfg.GuestDefaults.DiskRead.Trigger)
				assert.Equal(t, 0.0, cfg.GuestDefaults.DiskWrite.Trigger)
				assert.Equal(t, 0.0, cfg.GuestDefaults.NetworkIn.Trigger)
				assert.Equal(t, 0.0, cfg.GuestDefaults.NetworkOut.Trigger)
			},
		},
		{
			name: "TimeThresholds normalization",
			input: map[string]interface{}{
				"timeThresholds": map[string]interface{}{
					"guest": -1,
					"pbs":   0,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 5, cfg.TimeThresholds["guest"])
				assert.Equal(t, 5, cfg.TimeThresholds["pbs"])
			},
		},
		{
			name: "BackupDefaults warning > critical",
			input: map[string]interface{}{
				"backupDefaults": map[string]interface{}{
					"warningDays":  20,
					"criticalDays": 10,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 10, cfg.BackupDefaults.WarningDays)
			},
		},
		{
			name: "BackupDefaults critical negative",
			input: map[string]interface{}{
				"backupDefaults": map[string]interface{}{
					"criticalDays": -5,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0, cfg.BackupDefaults.CriticalDays)
			},
		},
		{
			name: "BackupDefaults fresh/stale zero/negative",
			input: map[string]interface{}{
				"backupDefaults": map[string]interface{}{
					"freshHours": 0,
					"staleHours": -1,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 24, cfg.BackupDefaults.FreshHours)
				assert.Equal(t, 72, cfg.BackupDefaults.StaleHours) // 72 >= 24
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.input)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(alertFile, data, 0644))

			cfg, err := cp.LoadAlertConfig()
			require.NoError(t, err)
			tt.verify(t, cfg)
		})
	}
}
