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

func TestEnsurePVEClusterNodeIdentitiesKeepsOtherConnectionsNodeIdentities(t *testing.T) {
	// A standalone connection "pve-b" reports its node "pve-b" as
	// "pve-b-pve-b". The cluster name and node names are what the other
	// endpoint says about itself and must not spell that identity.
	for _, tc := range []struct {
		name, clusterName, nodeName string
	}{
		{"cluster named after the connection", "pve-b", "pve-b"},
		{"same name in another case", "PVE-B", "pve-b"},
		{"split at an earlier hyphen", "pve", "b-pve-b"},
		{"split at a later hyphen", "pve-b-pve", "b"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instances := []PVEInstance{
				{Name: "pve-b"},
				{
					Name: "lab", IsCluster: true, ClusterName: tc.clusterName,
					ClusterEndpoints: []ClusterEndpoint{
						{NodeName: "lab1", NativeNodeID: 1},
						{NodeName: tc.nodeName, NativeNodeID: 2},
					},
				},
			}
			EnsurePVEClusterNodeIdentities(instances)
			for _, endpoint := range instances[1].ClusterEndpoints {
				if strings.HasPrefix(strings.ToLower(endpoint.NodeIdentity), "pve-b-") {
					t.Fatalf("node %q got identity %q in connection pve-b's namespace", endpoint.NodeName, endpoint.NodeIdentity)
				}
			}
			if got, want := instances[1].ClusterEndpoints[1].NodeIdentity, "lab-"+tc.nodeName; got != want {
				t.Fatalf("identity = %q, want connection-scoped %q", got, want)
			}
			if EnsurePVEClusterNodeIdentities(instances) {
				t.Fatal("second normalization should be stable")
			}
		})
	}
}

func TestEnsurePVEClusterNodeIdentitiesRejectsIdentityRecordedByOtherConnection(t *testing.T) {
	instances := []PVEInstance{
		{
			Name: "site-a", IsCluster: true, ClusterName: "prod",
			ClusterEndpoints:      []ClusterEndpoint{{NodeName: "dr-1", NativeNodeID: 1, NodeIdentity: "prod-dr-1"}},
			ClusterNodeIdentities: []PVEClusterNodeIdentity{{ID: "prod-dr-1", NativeNodeID: 1, NativeName: "dr-1"}},
		},
		{
			Name: "site-b", IsCluster: true, ClusterName: "prod-dr",
			ClusterEndpoints: []ClusterEndpoint{{NodeName: "1", NativeNodeID: 1}},
		},
	}
	EnsurePVEClusterNodeIdentities(instances)
	if got := instances[0].ClusterEndpoints[0].NodeIdentity; got != "prod-dr-1" {
		t.Fatalf("recorded identity changed to %q", got)
	}
	if got := instances[1].ClusterEndpoints[0].NodeIdentity; got != "site-b-1" {
		t.Fatalf("identity = %q, want connection-scoped %q", got, "site-b-1")
	}
}

func TestDerivePVENodeIdentity(t *testing.T) {
	instances := []PVEInstance{
		{Name: "pve"},
		{Name: "pve-b"},
		{
			Name: "lab", IsCluster: true, ClusterName: "homelab",
			ClusterNodeIdentities: []PVEClusterNodeIdentity{{ID: "homelab-n1", NativeNodeID: 1, NativeName: "n1"}},
		},
		{Name: "homelab"},
	}
	for _, tc := range []struct {
		instance, scope, node, want string
	}{
		{"pve-b", "pve-b", "pve-b", "pve-b-pve-b"},
		{"pve", "pve", "pve", "pve-pve"},
		{"lab", "homelab", "n2", "lab-n2"},
		{"lab", "pve-b", "pve-b", "lab-pve-b"},
	} {
		if got := DerivePVENodeIdentity(instances, tc.instance, tc.scope, tc.node); got != tc.want {
			t.Errorf("DerivePVENodeIdentity(%q, %q, %q) = %q, want %q", tc.instance, tc.scope, tc.node, got, tc.want)
		}
	}
	// A node name that reaches into a longer connection name's namespace, or
	// spells an identity another connection records, gets an opaque identity.
	for _, tc := range []struct {
		instance, node, taken string
	}{
		{"pve", "b-pve-b", "pve-b-pve-b"},
		{"homelab", "n1", "homelab-n1"},
	} {
		got := DerivePVENodeIdentity(instances, tc.instance, tc.instance, tc.node)
		if strings.EqualFold(got, tc.taken) || !strings.HasPrefix(got, tc.instance+"-node-") {
			t.Errorf("DerivePVENodeIdentity(%q, %q) = %q, want an opaque identity other than %q", tc.instance, tc.node, got, tc.taken)
		}
		if again := DerivePVENodeIdentity(instances, tc.instance, tc.instance, tc.node); again != got {
			t.Errorf("DerivePVENodeIdentity(%q, %q) is not stable: %q then %q", tc.instance, tc.node, got, again)
		}
	}
}
