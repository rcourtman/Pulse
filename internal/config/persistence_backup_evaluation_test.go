package config

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/stretchr/testify/require"
)

// Exercise persistence and the public evaluator together: preserving opaque
// keys on disk alone does not prove an override applies to its intended guest.
func TestPersistedBackupOverridesDriveGuestEvaluation(t *testing.T) {
	for _, key := range []string{"inst:node:100", "guest:inst:100"} {
		for _, kind := range []string{"global", "sparse", "explicit", "disabled"} {
			t.Run(key+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				cfg := alerts.AlertConfig{
					Enabled:        true,
					BackupDefaults: alerts.BackupAlertConfig{Enabled: true, WarningDays: 33, CriticalDays: 34, FreshHours: 745, StaleHours: 746},
					Overrides:      map[string]alerts.ThresholdConfig{},
				}
				switch kind {
				case "sparse":
					cfg.Overrides[key] = alerts.ThresholdConfig{Backup: &alerts.BackupAlertConfig{Enabled: true}}
				case "explicit":
					cfg.Overrides[key] = alerts.ThresholdConfig{Backup: &alerts.BackupAlertConfig{Enabled: true, WarningDays: 7, CriticalDays: 14, FreshHours: 24, StaleHours: 72}}
				case "disabled":
					cfg.Overrides[key] = alerts.ThresholdConfig{Backup: &alerts.BackupAlertConfig{Enabled: false}}
				}
				for cycle := 0; cycle < 2; cycle++ {
					require.NoError(t, NewConfigPersistence(dir).SaveAlertConfig(cfg))
					loaded, err := NewConfigPersistence(dir).LoadAlertConfig()
					require.NoError(t, err)
					for _, age := range []int{8, 35} {
						// Fresh managers isolate each firing decision from existing incidents.
						// This is not an installed-process restart or notification delivery test.
						func() {
							m := alerts.NewManagerWithDataDir(t.TempDir())
							defer m.Stop()
							m.UpdateConfig(*loaded)
							lastSuccess := time.Now().Add(-time.Duration(age) * 24 * time.Hour)
							guest := alerts.GuestLookup{ResourceID: "inst:node:100", Name: "fixture", Instance: "inst", Node: "node", Type: "qemu", VMID: 100}
							m.CheckBackups([]recovery.ProtectionRollup{{
								RollupID:      "res:vm:proxmox:inst:node:100",
								SubjectRef:    &recovery.ExternalRef{Type: "proxmox-vm", Namespace: "inst", Name: "fixture", ID: guest.ResourceID, Class: "node"},
								LastSuccessAt: &lastSuccess, LastOutcome: recovery.OutcomeSuccess,
								Providers: []recovery.Provider{recovery.ProviderProxmoxPVE},
							}}, map[string]alerts.GuestLookup{alerts.BuildGuestKey("inst", "node", 100): guest}, map[string][]alerts.GuestLookup{"100": {guest}})
							active := m.GetActiveAlerts()
							wantThreshold := 0
							wantLevel := alerts.AlertLevelCritical
							if kind == "explicit" {
								wantThreshold = 14
								if age == 8 {
									wantThreshold = 7
									wantLevel = alerts.AlertLevelWarning
								}
							} else if kind != "disabled" && cycle == 0 && age == 35 {
								wantThreshold = 34
							}
							if wantThreshold == 0 {
								require.Empty(t, active, "cycle=%d age=%d", cycle, age)
							} else {
								require.Len(t, active, 1, "cycle=%d age=%d", cycle, age)
								require.Equal(t, float64(wantThreshold), active[0].Threshold)
								require.Equal(t, wantLevel, active[0].Level)
							}
						}()
					}
					cfg = *loaded
					cfg.BackupDefaults.WarningDays = 40
					cfg.BackupDefaults.CriticalDays = 41
				}
			})
		}
	}
}
