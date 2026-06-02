package alerts

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// #1341: a Ceph pool is known by a different ID per reporting source. The pool
// target carries those alternate IDs in AliasIDs, and findStorageOverride must
// honor a per-pool override saved under any of them, so the threshold does not
// fall back to the default when a different source wins.
func TestFindStorageOverrideHonorsCephPoolAliasID(t *testing.T) {
	usage := HysteresisThreshold{Trigger: 50, Clear: 45}
	overrides := map[string]ThresholdConfig{
		// Saved under the host-agent node identity.
		"agent:pve5-ceph-pool-data_replication": {Usage: &usage},
	}

	// The winning identity is the Proxmox-API CLUSTER name, which differs from
	// the agent's node hostname (the real-world clustered case). The override
	// must still resolve via the alias.
	apiPool := models.Storage{
		ID:       "prodcluster-ceph-pool-data_replication",
		AliasIDs: []string{"agent:pve5-ceph-pool-data_replication", "pve5-ceph-pool-data_replication"},
		Name:     "data_replication",
		Type:     "ceph-pool",
		Shared:   true,
	}

	override, ok, key := findStorageOverride(overrides, apiPool)
	if !ok {
		t.Fatalf("expected the override to resolve via a pool alias ID")
	}
	if key != "agent:pve5-ceph-pool-data_replication" {
		t.Errorf("expected match on the agent alias key, got %q", key)
	}
	if override.Usage == nil || override.Usage.Trigger != 50 {
		t.Errorf("expected override trigger 50, got %+v", override.Usage)
	}
}

// A direct (primary-ID) override still matches, and the lookup keys include the
// pool's alias IDs.
func TestStorageOverrideLookupKeysIncludeCephPoolAliases(t *testing.T) {
	usage := HysteresisThreshold{Trigger: 70, Clear: 65}

	direct := models.Storage{ID: "pve5-ceph-pool-data_replication", Name: "data_replication", Type: "ceph-pool", Shared: true}
	if _, ok, key := findStorageOverride(map[string]ThresholdConfig{
		"pve5-ceph-pool-data_replication": {Usage: &usage},
	}, direct); !ok || key != "pve5-ceph-pool-data_replication" {
		t.Errorf("expected direct canonical key match, ok=%v key=%q", ok, key)
	}

	pool := models.Storage{
		ID:       "prodcluster-ceph-pool-data_replication",
		AliasIDs: []string{"agent:pve5-ceph-pool-data_replication"},
		Name:     "data_replication",
		Type:     "ceph-pool",
		Shared:   true,
	}
	keys := storageOverrideLookupKeys(pool)
	want := map[string]bool{
		"prodcluster-ceph-pool-data_replication": false,
		"agent:pve5-ceph-pool-data_replication":  false,
	}
	for _, k := range keys {
		if _, ok := want[k]; ok {
			want[k] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("expected lookup keys to include %q; got %v", k, keys)
		}
	}
}
