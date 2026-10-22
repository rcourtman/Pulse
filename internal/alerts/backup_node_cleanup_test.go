package alerts

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// Retained backups can outlive their Proxmox node or have no node at all.
// A node inventory sweep is not evidence that their backup condition recovered.
func TestBackupAlertsSurviveNodeCleanupAndRestart(t *testing.T) {
	for _, provider := range []recovery.Provider{recovery.ProviderProxmoxPBS, recovery.ProviderProxmoxPVE} {
		for _, remembered := range []bool{false, true} {
			name := string(provider) + "/unattributed"
			if remembered {
				name = string(provider) + "/remembered"
			}
			t.Run(name, func(t *testing.T) {
				dir := t.TempDir()
				m := NewManagerWithDataDir(dir)
				t.Cleanup(m.Stop)
				cfg := m.GetConfig()
				cfg.Enabled = true
				cfg.ActivationState = ActivationActive
				cfg.BackupDefaults = BackupAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5}
				m.UpdateConfig(cfg)

				live := GuestLookup{ResourceID: "site-a:node-a:100", Name: "database", Instance: "site-a", Node: "node-a", Type: "qemu", VMID: 100}
				byKey := map[string]GuestLookup{live.ResourceID: live}
				byVMID := map[string][]GuestLookup{"100": {live}}
				if remembered {
					byVMID["102"] = []GuestLookup{{Name: "retired-home", Instance: "old-site", Node: "old-node", Type: "lxc", VMID: 102}}
				}
				old := time.Now().UTC().Add(-6 * 24 * time.Hour)
				rollups := []recovery.ProtectionRollup{
					{RollupID: "res:system-container-retired-home", SubjectRef: &recovery.ExternalRef{ID: "102", Name: "retired-home", Type: "proxmox-lxc"}, LastSuccessAt: &old, Providers: []recovery.Provider{provider}},
					{RollupID: "res:database", SubjectRef: &recovery.ExternalRef{ID: live.ResourceID, Name: live.Name, Type: "proxmox-vm"}, LastSuccessAt: &old, Providers: []recovery.Provider{provider}},
				}
				if err := m.CheckBackups(rollups, byKey, byVMID); err != nil {
					t.Fatal(err)
				}
				initial := m.GetActiveAlerts()
				if len(initial) != 2 {
					t.Fatalf("initial alerts=%d, want live and orphan", len(initial))
				}
				var orphan Alert
				for _, a := range initial {
					if a.ResourceName == "retired-home backup" {
						orphan = a
					}
				}
				if orphan.ID == "" || !metadataBoolValue(orphan.Metadata, "orphaned") {
					t.Fatal("fixture did not create an orphan backup warning")
				}
				if err := m.AcknowledgeAlert(orphan.ID, "operator"); err != nil {
					t.Fatal(err)
				}
				check := func(manager *Manager, phase string) {
					t.Helper()
					active := manager.GetActiveAlerts()
					if len(active) != len(initial) {
						t.Fatalf("%s: node cleanup hid a backup warning: got %d, want %d", phase, len(active), len(initial))
					}
					for _, before := range initial {
						found := false
						for _, after := range active {
							if after.ID == before.ID && after.StartTime.Equal(before.StartTime) {
								found = true
								if after.ID == orphan.ID && (!after.Acknowledged || after.AckUser != "operator") {
									t.Fatalf("%s: orphan acknowledgement lost", phase)
								}
							}
						}
						if !found {
							t.Fatalf("%s: backup occurrence identity changed for %s", phase, before.ID)
						}
					}
				}
				for i := 0; i < 3; i++ {
					m.CleanupAlertsForNodes(map[string]bool{"node-a": true})
					check(m, "after node cleanup")
					if err := m.CheckBackups(rollups, byKey, byVMID); err != nil {
						t.Fatal(err)
					}
					check(m, "after backup evaluation")
				}
				if err := m.SaveActiveAlerts(); err != nil {
					t.Fatal(err)
				}
				m.Stop()
				restored := NewManagerWithDataDir(dir)
				t.Cleanup(restored.Stop)
				restored.UpdateConfig(cfg)
				restored.CleanupAlertsForNodes(map[string]bool{"node-a": true})
				check(restored, "after restart and node cleanup")

				// The backup owner can still resolve the retained orphan by policy,
				// and resolve the live warning from fresh backup evidence.
				disabled := false
				cfg.BackupDefaults.AlertOrphaned = &disabled
				restored.UpdateConfig(cfg)
				if got := restored.GetActiveAlerts(); len(got) != 1 || got[0].ResourceID != live.ResourceID {
					t.Fatalf("orphan-off retained an orphan or cleared live warning: %+v", got)
				}
				fresh := time.Now().UTC()
				rollups[1].LastSuccessAt = &fresh
				if err := restored.CheckBackups(rollups, byKey, byVMID); err != nil {
					t.Fatal(err)
				}
				if got := restored.GetActiveAlerts(); len(got) != 0 {
					t.Fatalf("fresh backup did not clear retained warning: %+v", got)
				}
			})
		}
	}
}
