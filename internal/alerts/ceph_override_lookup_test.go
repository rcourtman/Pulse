package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// #1341: alert evaluation collapses a dual-source Ceph cluster to the Proxmox
// API pool identity (e.g. "pve5-ceph-pool-data_replication"). An existing
// per-pool override may have been saved under the legacy host-agent identity
// ("agent:pve5-ceph-pool-data_replication"); findStorageOverride must still
// honor it so the threshold keeps firing without manual re-entry.
func TestFindStorageOverrideHonorsLegacyAgentCephPoolKey(t *testing.T) {
	usage := HysteresisThreshold{Trigger: 50, Clear: 45}
	overrides := map[string]ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {Usage: &usage},
	}

	apiPool := models.Storage{
		ID:     "pve5-ceph-pool-data_replication",
		Name:   "data_replication",
		Type:   "ceph-pool",
		Shared: true,
	}

	override, ok, key := findStorageOverride(overrides, apiPool)
	if !ok {
		t.Fatalf("expected legacy agent-keyed override to be found for API pool identity")
	}
	if key != "agent:pve5-ceph-pool-data_replication" {
		t.Errorf("expected match on legacy agent key, got %q", key)
	}
	if override.Usage == nil || override.Usage.Trigger != 50 {
		t.Errorf("expected override trigger 50, got %+v", override.Usage)
	}
}

// An override saved under the canonical (non-agent) Ceph pool ID must continue
// to match directly, and an agent-sourced pool must not gain a double "agent:"
// prefix.
func TestFindStorageOverrideCephPoolDirectAndAgentSource(t *testing.T) {
	usage := HysteresisThreshold{Trigger: 70, Clear: 65}

	direct := models.Storage{ID: "pve5-ceph-pool-data_replication", Name: "data_replication", Type: "ceph-pool", Shared: true}
	if _, ok, key := findStorageOverride(map[string]ThresholdConfig{
		"pve5-ceph-pool-data_replication": {Usage: &usage},
	}, direct); !ok || key != "pve5-ceph-pool-data_replication" {
		t.Errorf("expected direct canonical key match, ok=%v key=%q", ok, key)
	}

	agentPool := models.Storage{ID: "agent:pve5-ceph-pool-data_replication", Name: "data_replication", Type: "ceph-pool", Shared: true}
	keys := storageOverrideLookupKeys(agentPool)
	for _, k := range keys {
		if k == "agent:agent:pve5-ceph-pool-data_replication" {
			t.Errorf("agent-sourced pool must not gain a doubled agent: prefix; keys=%v", keys)
		}
	}
}
