package alerts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPlanAlertIdentityMigrationDryRunAndApply(t *testing.T) {
	guestOverride := ThresholdConfig{CPU: &HysteresisThreshold{Trigger: 91, Clear: 86}}
	diskOverride := ThresholdConfig{Disk: &HysteresisThreshold{Trigger: 93, Clear: 88}}
	storageOverride := ThresholdConfig{Usage: &HysteresisThreshold{Trigger: 94, Clear: 89}}
	unknownOverride := ThresholdConfig{Disabled: true}
	config := AlertConfig{Overrides: map[string]ThresholdConfig{
		"cluster-a-node-1-100":                      guestOverride,
		"guest-disk:cluster-a-node-2-101/disk:root": diskOverride,
		"agent:cluster-a-ceph-pool-data":            storageOverride,
		"temporarily-offline-resource":              unknownOverride,
	}}
	original := cloneThresholdOverrides(config.Overrides)
	resources := []unifiedresources.Resource{
		{
			ID:   "vm-aaaaaaaaaaaaaaaa",
			Type: unifiedresources.ResourceTypeVM,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "cluster-a",
				NodeName: "node-1",
				VMID:     100,
			},
		},
		{
			ID:   "vm-bbbbbbbbbbbbbbbb",
			Type: unifiedresources.ResourceTypeVM,
			Proxmox: &unifiedresources.ProxmoxData{
				Instance: "cluster-a",
				NodeName: "node-2",
				VMID:     101,
			},
		},
		{
			ID:   "storage-cccccccccccccccc",
			Type: unifiedresources.ResourceTypeStorage,
			MetricsTarget: &unifiedresources.MetricsTarget{
				ResourceType: "storage",
				ResourceID:   "cluster-a-ceph-pool-data",
			},
			Storage: &unifiedresources.StorageMeta{
				AliasIDs: []string{"agent:cluster-a-ceph-pool-data"},
			},
		},
	}

	plan := PlanAlertIdentityMigration(config, resources)
	if !plan.Changed() || plan.FromVersion != 0 || plan.ToVersion != CurrentAlertIdentitySchemaVersion {
		t.Fatalf("unexpected migration plan: %+v", plan)
	}
	if !reflect.DeepEqual(config.Overrides, original) || config.IdentitySchemaVersion != 0 {
		t.Fatalf("dry-run mutated source config: %+v", config)
	}

	if !ApplyAlertIdentityMigration(&config, plan) {
		t.Fatal("expected migration plan to apply")
	}
	if config.IdentitySchemaVersion != CurrentAlertIdentitySchemaVersion {
		t.Fatalf("identity schema version = %d", config.IdentitySchemaVersion)
	}
	for _, removed := range []string{
		"cluster-a-node-1-100",
		"guest-disk:cluster-a-node-2-101/disk:root",
		"agent:cluster-a-ceph-pool-data",
	} {
		if _, exists := config.Overrides[removed]; exists {
			t.Fatalf("legacy override %q remained after migration", removed)
		}
	}
	if got := config.Overrides["guest:cluster-a:100"]; got.CPU == nil || got.CPU.Trigger != 91 {
		t.Fatalf("guest override was not migrated: %+v", got)
	}
	if got := config.Overrides["guest-disk:guest:cluster-a:101/disk:root"]; got.Disk == nil || got.Disk.Trigger != 93 {
		t.Fatalf("guest disk override was not migrated: %+v", got)
	}
	if got := config.Overrides["cluster-a-ceph-pool-data"]; got.Usage == nil || got.Usage.Trigger != 94 {
		t.Fatalf("storage override was not migrated: %+v", got)
	}
	if got, exists := config.Overrides["temporarily-offline-resource"]; !exists || !got.Disabled {
		t.Fatalf("unknown override was not preserved: %+v", config.Overrides)
	}

	second := PlanAlertIdentityMigration(config, resources)
	if second.Changed() {
		t.Fatalf("migration was not idempotent: %+v", second)
	}
}

func TestPlanAlertIdentityMigrationFailsClosedOnConflictingLegacyRows(t *testing.T) {
	config := AlertConfig{Overrides: map[string]ThresholdConfig{
		"cluster-a-100":        {CPU: &HysteresisThreshold{Trigger: 90, Clear: 85}},
		"cluster-a-node-1-100": {CPU: &HysteresisThreshold{Trigger: 95, Clear: 90}},
	}}
	resources := []unifiedresources.Resource{{
		ID:   "vm-aaaaaaaaaaaaaaaa",
		Type: unifiedresources.ResourceTypeVM,
		Proxmox: &unifiedresources.ProxmoxData{
			Instance: "cluster-a",
			NodeName: "node-1",
			VMID:     100,
		},
	}}

	plan := PlanAlertIdentityMigration(config, resources)
	if len(plan.Deferred) != 2 {
		t.Fatalf("deferred rows = %+v, want both conflicting aliases", plan.Deferred)
	}
	if !ApplyAlertIdentityMigration(&config, plan) {
		t.Fatal("schema-only portion of plan should still apply")
	}
	if len(config.Overrides) != 2 {
		t.Fatalf("conflicting rows were changed: %+v", config.Overrides)
	}
}

func TestPlanAlertIdentityMigrationRejectsNewerSchema(t *testing.T) {
	config := AlertConfig{
		IdentitySchemaVersion: CurrentAlertIdentitySchemaVersion + 1,
		Overrides:             map[string]ThresholdConfig{"legacy": {Disabled: true}},
	}
	plan := PlanAlertIdentityMigration(config, nil)
	if !plan.UnsupportedVersion || plan.Changed() {
		t.Fatalf("newer schema plan = %+v", plan)
	}
	if ApplyAlertIdentityMigration(&config, plan) {
		t.Fatal("newer schema must not be modified")
	}
}

func TestLoadActiveAlertsRewritesCanonicalPersistedIDAndPreservesAck(t *testing.T) {
	m := newTestManager(t)
	now := time.Now().UTC()
	legacy := []*Alert{{
		ID:           "node-offline-pve-a",
		Type:         "connectivity",
		Level:        AlertLevelWarning,
		ResourceName: "pve-a",
		Node:         "pve-a",
		Message:      "node offline",
		StartTime:    now.Add(-time.Minute),
		LastSeen:     now,
		Acknowledged: true,
		AckTime:      &now,
		AckUser:      "operator",
	}}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	alertsDir := m.getAlertsDir()
	if err := os.MkdirAll(alertsDir, alertsDirPerm); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(alertsDir, "active-alerts.json")
	if err := os.WriteFile(path, data, alertsFilePerm); err != nil {
		t.Fatal(err)
	}

	if err := m.LoadActiveAlerts(); err != nil {
		t.Fatalf("LoadActiveAlerts() error = %v", err)
	}
	wantID := "pve-a::pve-a-connectivity"
	deadline := time.Now().Add(2 * time.Second)
	for {
		persistedData, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		var persisted []*Alert
		if err := json.Unmarshal(persistedData, &persisted); err != nil {
			t.Fatal(err)
		}
		if len(persisted) == 1 && persisted[0].ID == wantID {
			if !persisted[0].Acknowledged || persisted[0].AckUser != "operator" || persisted[0].AckTime == nil {
				t.Fatalf("ack fields changed during rewrite: %+v", persisted[0])
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("active alert file was not canonically rewritten: %+v", persisted)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
