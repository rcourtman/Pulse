package alerts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

// The monitor enriches VMID lookups with remembered names after a guest is
// deleted. Those entries have a location but no live ResourceID. A synthesized
// policy identity must not turn that last-known guest into a live one.
func TestBackupOrphanPolicyWithRememberedGuestIdentity(t *testing.T) {
	for _, provider := range []recovery.Provider{recovery.ProviderProxmoxPBS, recovery.ProviderProxmoxPVE} {
		t.Run(string(provider), func(t *testing.T) {
			dir := t.TempDir()
			m := NewManagerWithDataDir(dir)
			t.Cleanup(m.Stop)
			cfg := m.GetConfig()
			cfg.Enabled = true
			cfg.ActivationState = ActivationActive
			cfg.BackupDefaults = BackupAlertConfig{Enabled: true, WarningDays: 3, CriticalDays: 5}
			m.UpdateConfig(cfg)

			remembered := GuestLookup{Name: "retired-home", Instance: "old-site", Node: "old-node", Type: "lxc", VMID: 102}
			live := GuestLookup{ResourceID: "site-a:node-a:100", Name: "database", Instance: "site-a", Node: "node-a", Type: "qemu", VMID: 100}
			byKey := map[string]GuestLookup{live.ResourceID: live}
			byVMID := map[string][]GuestLookup{"100": {live}, "102": {remembered}}
			lastBackup := time.Now().UTC().Add(-6 * 24 * time.Hour)
			rollups := []recovery.ProtectionRollup{
				{RollupID: "res-retired-home", SubjectRef: &recovery.ExternalRef{ID: "old-site:old-node:102", Name: "retired-home", Type: "proxmox-lxc"}, LastSuccessAt: &lastBackup, Providers: []recovery.Provider{provider}},
				{RollupID: "res-database", SubjectRef: &recovery.ExternalRef{ID: live.ResourceID, Name: live.Name, Type: "proxmox-vm"}, LastSuccessAt: &lastBackup, Providers: []recovery.Provider{provider}},
			}
			if err := m.CheckBackups(rollups, byKey, byVMID); err != nil {
				t.Fatal(err)
			}
			check := func(manager *Manager, label string) {
				t.Helper()
				list := manager.GetActiveAlerts()
				if len(list) != 2 {
					t.Fatalf("%s active alerts=%d, want live and orphan", label, len(list))
				}
				for _, a := range list {
					switch a.ResourceName {
					case "retired-home backup":
						if !metadataBoolValue(a.Metadata, "orphaned") {
							t.Errorf("%s: remembered-only guest is reported as live: %+v", label, a.Metadata)
						}
					case "database backup":
						if metadataBoolValue(a.Metadata, "orphaned") {
							t.Errorf("%s: live guest marked orphaned", label)
						}
					default:
						t.Fatalf("unexpected alert %+v", a)
					}
				}
			}
			check(m, "initial")
			if err := m.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			m.Stop()

			restored := NewManagerWithDataDir(dir)
			t.Cleanup(restored.Stop)
			restored.UpdateConfig(cfg)
			check(restored, "restored")
			// Capture the actual consuming payload around a config-only transition.
			// No provider poll is needed to honour a policy explicitly switched off.
			before := restored.GetActiveAlerts()
			resolved := make(chan *ResolvedAlert, 2)
			restored.SetResolvedAlertCallback(func(alert *ResolvedAlert) { resolved <- alert })
			disabled := false
			cfg.BackupDefaults.AlertOrphaned = &disabled
			restored.UpdateConfig(cfg)
			after := restored.GetActiveAlerts()
			if len(after) != 1 || after[0].ResourceID != live.ResourceID {
				t.Errorf("config save retained an orphan or cleared live warning: %+v", after)
			}
			select {
			case resolution := <-resolved:
				if resolution.Alert == nil || resolution.Alert.ResourceName != "retired-home backup" {
					t.Fatalf("configuration resolved the wrong occurrence: %+v", resolution)
				}
			case <-time.After(time.Second):
				t.Error("orphan disable did not emit the monitor's occurrence-qualified resolution callback")
			}
			projection, err := json.Marshal(map[string]any{"before": before, "after": after, "config": restored.GetConfig()})
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("BACKUP_ORPHAN_POLICY_PROJECTION=%s", projection)
			for i := 0; i < 3; i++ {
				if err := restored.CheckBackups(rollups, byKey, byVMID); err != nil {
					t.Fatal(err)
				}
			}
			if got := restored.GetActiveAlerts(); len(got) != 1 || got[0].ID != after[0].ID {
				t.Fatalf("disabled policy did not survive repeated polls: %+v", got)
			}
			if err := restored.SaveActiveAlerts(); err != nil {
				t.Fatal(err)
			}
			restored.Stop()
			reloaded := NewManagerWithDataDir(dir)
			t.Cleanup(reloaded.Stop)
			reloaded.UpdateConfig(cfg)
			if got := reloaded.GetActiveAlerts(); len(got) != 1 || got[0].ResourceID != live.ResourceID {
				t.Fatalf("resolved orphan reappeared after checkpoint: %+v", got)
			}
		})
	}
}
