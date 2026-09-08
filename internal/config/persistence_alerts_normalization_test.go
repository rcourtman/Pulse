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
				assert.Equal(t, 5, cfg.Schedule.Cooldown)
				assert.Equal(t, 10, cfg.Schedule.MaxAlertsHour)
				assert.Equal(t, "all", cfg.Schedule.InitialNotify)
				assert.True(t, cfg.Schedule.NotifyOnResolve)
				assert.True(t, cfg.Schedule.Grouping.Enabled)
				assert.Equal(t, 30, cfg.Schedule.Grouping.Window)
				assert.True(t, cfg.Schedule.Grouping.ByNode)
				assert.False(t, cfg.Schedule.Grouping.ByGuest)
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
			name: "Schedule missing initialNotify defaults to all",
			input: map[string]interface{}{
				"schedule": map[string]interface{}{},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, "all", cfg.Schedule.InitialNotify)
			},
		},
		{
			name: "Schedule explicit initialNotify is preserved",
			input: map[string]interface{}{
				"schedule": map[string]interface{}{
					"initialNotify": "apprise",
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, "apprise", cfg.Schedule.InitialNotify)
			},
		},
		{
			name: "Schedule missing defaults notifyOnResolve to true",
			input: map[string]interface{}{
				"enabled": true,
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.True(t, cfg.Schedule.NotifyOnResolve)
			},
		},
		{
			name: "Schedule present without notifyOnResolve defaults to true",
			input: map[string]interface{}{
				"schedule": map[string]interface{}{},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.True(t, cfg.Schedule.NotifyOnResolve)
			},
		},
		{
			name: "Schedule explicit notifyOnResolve false is preserved",
			input: map[string]interface{}{
				"schedule": map[string]interface{}{
					"notifyOnResolve": false,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.False(t, cfg.Schedule.NotifyOnResolve)
			},
		},
		{
			name: "Legacy schedule missing cooldown and grouping defaults",
			input: map[string]interface{}{
				"schedule": map[string]interface{}{
					"notifyOnResolve": false,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 5, cfg.Schedule.Cooldown)
				assert.Equal(t, 10, cfg.Schedule.MaxAlertsHour)
				assert.False(t, cfg.Schedule.NotifyOnResolve)
				assert.True(t, cfg.Schedule.Grouping.Enabled)
				assert.Equal(t, 30, cfg.Schedule.Grouping.Window)
				assert.True(t, cfg.Schedule.Grouping.ByNode)
				assert.False(t, cfg.Schedule.Grouping.ByGuest)
			},
		},
		{
			name: "Explicit zero cooldown and grouping preserved",
			input: map[string]interface{}{
				"schedule": map[string]interface{}{
					"cooldown":        0,
					"notifyOnResolve": true,
					"grouping": map[string]interface{}{
						"enabled": false,
						"window":  0,
						"byNode":  false,
						"byGuest": true,
					},
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0, cfg.Schedule.Cooldown)
				assert.Equal(t, 10, cfg.Schedule.MaxAlertsHour)
				assert.True(t, cfg.Schedule.NotifyOnResolve)
				assert.False(t, cfg.Schedule.Grouping.Enabled)
				assert.Equal(t, 0, cfg.Schedule.Grouping.Window)
				assert.False(t, cfg.Schedule.Grouping.ByNode)
				assert.True(t, cfg.Schedule.Grouping.ByGuest)
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
			name: "AgentDefaults CPU negative",
			input: map[string]interface{}{
				"agentDefaults": map[string]interface{}{"cpu": map[string]interface{}{"trigger": -1}},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 80.0, cfg.AgentDefaults.CPU.Trigger)
			},
		},
		{
			name: "AgentDefaults CPU zero",
			input: map[string]interface{}{
				"agentDefaults": map[string]interface{}{"cpu": map[string]interface{}{"trigger": 0}},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
				assert.Equal(t, 0.0, cfg.AgentDefaults.CPU.Trigger)
				assert.Equal(t, 0.0, cfg.AgentDefaults.CPU.Clear)
			},
		},
		{
			name: "TimeThresholds defaults",
			input: map[string]interface{}{
				"timeThreshold": 0,
				"timeThresholds": map[string]interface{}{
					"guest": 0,
					"all":   0,
				},
			},
			verify: func(t *testing.T, cfg *alerts.AlertConfig) {
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

// Exercise the real file boundary, not just JSON reconstruction. A differing
// legacy copy cannot safely be distinguished from an intentional guest override.
func TestBackupOverridesSurvivePersistenceReopen(t *testing.T) {
	for _, tc := range []struct {
		name   string
		backup *alerts.BackupAlertConfig
	}{
		{name: "global only"},
		{name: "toggle only", backup: &alerts.BackupAlertConfig{Enabled: true}},
		{name: "explicit thresholds", backup: &alerts.BackupAlertConfig{
			Enabled: true, WarningDays: 7, CriticalDays: 14, FreshHours: 24, StaleHours: 72,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range []string{"inst:node:100", "inst:100"} {
				t.Run(key, func(t *testing.T) {
					dir := t.TempDir()
					cfg := alerts.AlertConfig{
						Enabled:        true,
						BackupDefaults: alerts.BackupAlertConfig{Enabled: true, WarningDays: 33, CriticalDays: 34, FreshHours: 745, StaleHours: 746},
						Overrides:      map[string]alerts.ThresholdConfig{},
					}
					if tc.backup != nil {
						copy := *tc.backup
						cfg.Overrides[key] = alerts.ThresholdConfig{Backup: &copy}
					}
					for cycle := 0; cycle < 3; cycle++ {
						wantDefaults := cfg.BackupDefaults
						// Load supplies the documented default for omitted orphan policy.
						orphaned := true
						wantDefaults.AlertOrphaned = &orphaned
						require.NoError(t, NewConfigPersistence(dir).SaveAlertConfig(cfg))
						loaded, err := NewConfigPersistence(dir).LoadAlertConfig()
						require.NoError(t, err)
						require.Equal(t, wantDefaults, loaded.BackupDefaults)
						if tc.backup == nil {
							require.Empty(t, loaded.Overrides)
						} else {
							require.Len(t, loaded.Overrides, 1)
							require.Equal(t, tc.backup, loaded.Overrides[key].Backup)
						}
						cfg = *loaded
						// A subsequent global edit must neither freeze sparse values
						// nor erase explicit values on the next save and reopen.
						cfg.BackupDefaults.WarningDays++
						cfg.BackupDefaults.CriticalDays++
					}
				})
			}
		})
	}
}
