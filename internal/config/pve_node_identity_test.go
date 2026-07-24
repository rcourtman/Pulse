package config

import (
	"strings"
	"testing"
)

func TestEnsurePVEClusterNodeIdentitiesMigratesLegacyIDsWithoutChurn(t *testing.T) {
	instances := []PVEInstance{{
		Name:        "api-entry",
		IsCluster:   true,
		ClusterName: "production",
		ClusterEndpoints: []ClusterEndpoint{{
			NodeID:       "node/pve1",
			NativeNodeID: 1,
			NodeName:     "pve1",
		}},
	}}

	if !EnsurePVEClusterNodeIdentities(instances) {
		t.Fatal("expected legacy configuration to be migrated")
	}
	endpoint := instances[0].ClusterEndpoints[0]
	if endpoint.NodeIdentity != "production-pve1" {
		t.Fatalf("identity = %q, want historical ID %q", endpoint.NodeIdentity, "production-pve1")
	}
	if EnsurePVEClusterNodeIdentities(instances) {
		t.Fatal("second normalization should be stable")
	}
}

func TestEnsurePVEClusterNodeIdentitiesScopesSameNamedClustersByConnection(t *testing.T) {
	instances := []PVEInstance{
		{
			Name: "site-a", IsCluster: true, ClusterName: "production",
			ClusterEndpoints: []ClusterEndpoint{{NodeName: "pve1", NativeNodeID: 1}},
		},
		{
			Name: "site-b", IsCluster: true, ClusterName: "production",
			ClusterEndpoints: []ClusterEndpoint{{NodeName: "pve1", NativeNodeID: 1}},
		},
	}

	EnsurePVEClusterNodeIdentities(instances)
	left := instances[0].ClusterEndpoints[0].NodeIdentity
	right := instances[1].ClusterEndpoints[0].NodeIdentity
	if left != "site-a-pve1" || right != "site-b-pve1" || left == right {
		t.Fatalf("same-name cluster identities were not connection scoped: %q, %q", left, right)
	}
}

func TestEnsurePVEClusterNodeIdentitiesSurvivesRemovalRenameAndReIP(t *testing.T) {
	instances := []PVEInstance{{
		Name: "edge", IsCluster: true, ClusterName: "edge-cluster",
		ClusterEndpoints: []ClusterEndpoint{{
			NodeName: "pve-old", NativeNodeID: 7, IP: "10.0.0.7",
		}},
	}}
	EnsurePVEClusterNodeIdentities(instances)
	identityID := instances[0].ClusterEndpoints[0].NodeIdentity
	instances[0].ClusterNodeIdentities[0].DisplayName = "Render host"

	instances[0].ClusterEndpoints = nil
	EnsurePVEClusterNodeIdentities(instances)
	if len(instances[0].ClusterNodeIdentities) != 1 {
		t.Fatal("confirmed membership removal must retain presentation identity metadata")
	}

	instances[0].ClusterEndpoints = []ClusterEndpoint{{
		NodeName: "pve-new", NativeNodeID: 7, IP: "10.20.0.7",
	}}
	EnsurePVEClusterNodeIdentities(instances)
	endpoint := instances[0].ClusterEndpoints[0]
	if endpoint.NodeIdentity != identityID {
		t.Fatalf("reappearing renamed member identity = %q, want %q", endpoint.NodeIdentity, identityID)
	}
	_, displayName := PVEClusterNodePresentation(&instances[0], "pve-new")
	if displayName != "Render host" {
		t.Fatalf("display override = %q, want retained override", displayName)
	}
	if instances[0].ClusterNodeIdentities[0].NativeName != "pve-new" {
		t.Fatalf("native diagnostic name was not refreshed: %+v", instances[0].ClusterNodeIdentities[0])
	}
	if len(instances[0].ClusterNodeIdentities[0].NativeAliases) != 1 ||
		instances[0].ClusterNodeIdentities[0].NativeAliases[0] != "pve-old" {
		t.Fatalf("previous native name was not retained as a diagnostic/history alias: %+v", instances[0].ClusterNodeIdentities[0])
	}
}

func TestEnsurePVEClusterNodeIdentitiesRepairsConflictingCaseNames(t *testing.T) {
	instances := []PVEInstance{{
		Name: "case-cluster", IsCluster: true,
		ClusterEndpoints: []ClusterEndpoint{
			{NodeName: "PVE", NativeNodeID: 1},
			{NodeName: "pve", NativeNodeID: 2},
		},
	}}
	EnsurePVEClusterNodeIdentities(instances)
	left := instances[0].ClusterEndpoints[0].NodeIdentity
	right := instances[0].ClusterEndpoints[1].NodeIdentity
	if left == "" || right == "" || left == right {
		t.Fatalf("case-conflicting native names need distinct deterministic identities: %q, %q", left, right)
	}
}

func TestEnsurePVEClusterNodeIdentitiesFailsClosedOnConflictingNumericLedger(t *testing.T) {
	instances := []PVEInstance{{
		Name: "conflict", IsCluster: true,
		ClusterEndpoints: []ClusterEndpoint{{
			NodeName: "pve-new", NativeNodeID: 7,
		}},
		ClusterNodeIdentities: []PVEClusterNodeIdentity{
			{ID: "conflict-a", NativeNodeID: 7, NativeName: "pve-a", DisplayName: "A"},
			{ID: "conflict-b", NativeNodeID: 7, NativeName: "pve-b", DisplayName: "B"},
		},
	}}

	EnsurePVEClusterNodeIdentities(instances)
	got := instances[0].ClusterEndpoints[0].NodeIdentity
	if got == "conflict-a" || got == "conflict-b" || got == "" {
		t.Fatalf("ambiguous numeric ledger must mint an isolated identity, got %q", got)
	}
}

func TestNormalizePVEClusterNodeDisplayName(t *testing.T) {
	if got, err := NormalizePVEClusterNodeDisplayName("  Rendu Étage  "); err != nil || got != "Rendu Étage" {
		t.Fatalf("Unicode display name = %q, %v", got, err)
	}
	for _, invalid := range []string{"bad\nname", string([]byte{0xff}), strings.Repeat("界", 129)} {
		if _, err := NormalizePVEClusterNodeDisplayName(invalid); err == nil {
			t.Fatalf("expected invalid display name %q to fail", invalid)
		}
	}
}

func TestConfigDeepCopyIsolatesPVEClusterIdentityMetadata(t *testing.T) {
	original := &Config{PVEInstances: []PVEInstance{{
		Name: "cluster", IsCluster: true,
		ClusterEndpoints:      []ClusterEndpoint{{NodeName: "pve1", NodeIdentity: "cluster-pve1"}},
		ClusterNodeIdentities: []PVEClusterNodeIdentity{{ID: "cluster-pve1", NativeName: "pve1", DisplayName: "Compute"}},
	}}}
	original.PVEInstances[0].ClusterNodeIdentities[0].NativeAliases = []string{"pve-old"}
	clone := original.DeepCopy()
	clone.PVEInstances[0].ClusterEndpoints[0].NodeName = "changed"
	clone.PVEInstances[0].ClusterNodeIdentities[0].DisplayName = "Changed"
	clone.PVEInstances[0].ClusterNodeIdentities[0].NativeAliases[0] = "changed"

	if original.PVEInstances[0].ClusterEndpoints[0].NodeName != "pve1" ||
		original.PVEInstances[0].ClusterNodeIdentities[0].DisplayName != "Compute" ||
		original.PVEInstances[0].ClusterNodeIdentities[0].NativeAliases[0] != "pve-old" {
		t.Fatal("tenant-scoped deep copy leaked nested Proxmox cluster metadata")
	}
}
