package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// branchcov0723amPopulatedSnapshot builds a snapshot that carries exactly one
// entry of every top-level collection SnapshotReadState routes into the
// unified registry. Values are chosen to be distinctive so a subtest can
// assert the source value lands in the matching ReadState accessor.
func branchcov0723amPopulatedSnapshot() models.StateSnapshot {
	return models.StateSnapshot{
		Nodes: []models.Node{{
			ID:       "node-branchcov",
			Name:     "node-branchcov",
			Instance: "instance-branchcov",
			Status:   "online",
		}},
		VMs: []models.VM{{
			ID:     "vm-branchcov",
			VMID:   4242,
			Name:   "vm-branchcov",
			Status: "running",
		}},
		Containers: []models.Container{{
			ID:     "ct-branchcov",
			Name:   "ct-branchcov",
			Status: "running",
		}},
		DockerHosts: []models.DockerHost{{
			ID:       "docker-host-branchcov",
			Hostname: "docker-host-branchcov",
			Status:   "online",
			Containers: []models.DockerContainer{{
				ID:    "app-container-branchcov",
				Name:  "app-container-branchcov",
				State: "running",
			}},
		}},
		Hosts: []models.Host{{
			ID:       "host-branchcov",
			Hostname: "host-branchcov",
			Status:   "online",
		}},
		Storage: []models.Storage{{
			ID:   "storage-branchcov",
			Name: "storage-branchcov",
			Type: "lvm",
		}},
		PhysicalDisks: []models.PhysicalDisk{{
			ID:      "disk-branchcov",
			DevPath: "/dev/sda",
			Type:    "sata",
		}},
		PBSInstances: []models.PBSInstance{{
			ID:   "pbs-branchcov",
			Name: "pbs-branchcov",
		}},
		PMGInstances: []models.PMGInstance{{
			ID:   "pmg-branchcov",
			Name: "pmg-branchcov",
		}},
		KubernetesClusters: []models.KubernetesCluster{{
			ID:   "k8s-branchcov",
			Name: "k8s-branchcov",
		}},
	}
}

// findHostViewByHostname returns the first host view whose Hostname matches,
// or nil. Hosts() may synthesise linked-agent siblings, so callers locate the
// specific source host by hostname rather than by position.
func findHostViewByHostname(views []*unifiedresources.HostView, hostname string) *unifiedresources.HostView {
	for _, v := range views {
		if v != nil && v.Hostname() == hostname {
			return v
		}
	}
	return nil
}

func findPhysicalDiskViewByDevPath(views []*unifiedresources.PhysicalDiskView, devPath string) *unifiedresources.PhysicalDiskView {
	for _, v := range views {
		if v != nil && v.DevPath() == devPath {
			return v
		}
	}
	return nil
}

// TestBranchcov0723Am_SnapshotReadState_Zero drives SnapshotReadState with a
// zero-value snapshot and asserts every ReadState accessor returns an empty
// (length 0) result rather than nil-panicking.
func TestBranchcov0723Am_SnapshotReadState_Zero(t *testing.T) {
	rs := SnapshotReadState(models.StateSnapshot{})
	if rs == nil {
		t.Fatal("SnapshotReadState returned nil ReadState for zero snapshot")
	}

	cases := []struct {
		name string
		got  int
	}{
		{"VMs", len(rs.VMs())},
		{"Containers", len(rs.Containers())},
		{"Nodes", len(rs.Nodes())},
		{"Hosts", len(rs.Hosts())},
		{"DockerHosts", len(rs.DockerHosts())},
		{"DockerContainers", len(rs.DockerContainers())},
		{"StoragePools", len(rs.StoragePools())},
		{"PhysicalDisks", len(rs.PhysicalDisks())},
		{"PBSInstances", len(rs.PBSInstances())},
		{"PMGInstances", len(rs.PMGInstances())},
		{"K8sClusters", len(rs.K8sClusters())},
		{"K8sNodes", len(rs.K8sNodes())},
		{"Pods", len(rs.Pods())},
		{"K8sDeployments", len(rs.K8sDeployments())},
		{"Workloads", len(rs.Workloads())},
		{"Infrastructure", len(rs.Infrastructure())},
	}
	for _, c := range cases {
		if c.got != 0 {
			t.Errorf("%s() = %d items for zero snapshot, want 0", c.name, c.got)
		}
	}
}

// TestBranchcov0723Am_SnapshotReadState_Populated drives SnapshotReadState
// with one entry per top-level collection and asserts each lands in the
// matching accessor with its source identity intact. Canonical registry IDs
// are generated, so identity is asserted through the source carriers the
// registry preserves verbatim (Name / Hostname / VMID / DevPath) plus the
// exact accessor count, which proves a 1:1 map with no cross-collection
// leakage.
func TestBranchcov0723Am_SnapshotReadState_Populated(t *testing.T) {
	rs := SnapshotReadState(branchcov0723amPopulatedSnapshot())
	if rs == nil {
		t.Fatal("SnapshotReadState returned nil ReadState")
	}

	t.Run("VM", func(t *testing.T) {
		vms := rs.VMs()
		if len(vms) != 1 {
			t.Fatalf("VMs() = %d views, want 1", len(vms))
		}
		v := vms[0]
		if v == nil {
			t.Fatal("VM view is nil")
		}
		if v.Name() != "vm-branchcov" {
			t.Errorf("VM Name = %q, want vm-branchcov", v.Name())
		}
		if v.VMID() != 4242 {
			t.Errorf("VM VMID = %d, want 4242", v.VMID())
		}
		if v.Status() == "" {
			t.Error("VM Status empty; source snapshot Status was \"running\"")
		}
	})

	t.Run("Node", func(t *testing.T) {
		nodes := rs.Nodes()
		if len(nodes) != 1 {
			t.Fatalf("Nodes() = %d views, want 1", len(nodes))
		}
		n := nodes[0]
		if n == nil {
			t.Fatal("Node view is nil")
		}
		if n.Name() != "node-branchcov" {
			t.Errorf("Node Name = %q, want node-branchcov", n.Name())
		}
	})

	t.Run("Container", func(t *testing.T) {
		cts := rs.Containers()
		if len(cts) != 1 {
			t.Fatalf("Containers() = %d views, want 1", len(cts))
		}
		c := cts[0]
		if c == nil {
			t.Fatal("Container view is nil")
		}
		if c.Name() != "ct-branchcov" {
			t.Errorf("Container Name = %q, want ct-branchcov", c.Name())
		}
	})

	t.Run("DockerHost", func(t *testing.T) {
		dhs := rs.DockerHosts()
		if len(dhs) != 1 {
			t.Fatalf("DockerHosts() = %d views, want 1", len(dhs))
		}
		dh := dhs[0]
		if dh == nil {
			t.Fatal("DockerHost view is nil")
		}
		if dh.Hostname() != "docker-host-branchcov" {
			t.Errorf("DockerHost Hostname = %q, want docker-host-branchcov", dh.Hostname())
		}
	})

	t.Run("DockerContainer", func(t *testing.T) {
		dcs := rs.DockerContainers()
		if len(dcs) != 1 {
			t.Fatalf("DockerContainers() = %d views, want 1", len(dcs))
		}
		dc := dcs[0]
		if dc == nil {
			t.Fatal("DockerContainer view is nil")
		}
		if dc.Name() != "app-container-branchcov" {
			t.Errorf("DockerContainer Name = %q, want app-container-branchcov", dc.Name())
		}
	})

	t.Run("Host", func(t *testing.T) {
		h := findHostViewByHostname(rs.Hosts(), "host-branchcov")
		if h == nil {
			t.Fatalf("Hosts() did not surface host-branchcov (got %d views)", len(rs.Hosts()))
		}
		if h.ID() == "" {
			t.Error("Host canonical ID empty")
		}
	})

	t.Run("StoragePool", func(t *testing.T) {
		sps := rs.StoragePools()
		if len(sps) != 1 {
			t.Fatalf("StoragePools() = %d views, want 1", len(sps))
		}
		s := sps[0]
		if s == nil {
			t.Fatal("StoragePool view is nil")
		}
		if s.Name() != "storage-branchcov" {
			t.Errorf("StoragePool Name = %q, want storage-branchcov", s.Name())
		}
	})

	t.Run("PhysicalDisk", func(t *testing.T) {
		pds := rs.PhysicalDisks()
		if len(pds) != 1 {
			t.Fatalf("PhysicalDisks() = %d views, want 1", len(pds))
		}
		d := findPhysicalDiskViewByDevPath(pds, "/dev/sda")
		if d == nil {
			t.Fatalf("PhysicalDisks() did not surface DevPath /dev/sda")
		}
		if d.DevPath() != "/dev/sda" {
			t.Errorf("PhysicalDisk DevPath = %q, want /dev/sda", d.DevPath())
		}
	})

	t.Run("PBSInstance", func(t *testing.T) {
		pbs := rs.PBSInstances()
		if len(pbs) != 1 {
			t.Fatalf("PBSInstances() = %d views, want 1", len(pbs))
		}
		p := pbs[0]
		if p == nil {
			t.Fatal("PBSInstance view is nil")
		}
		if p.Name() != "pbs-branchcov" {
			t.Errorf("PBSInstance Name = %q, want pbs-branchcov", p.Name())
		}
	})

	t.Run("PMGInstance", func(t *testing.T) {
		pmgs := rs.PMGInstances()
		if len(pmgs) != 1 {
			t.Fatalf("PMGInstances() = %d views, want 1", len(pmgs))
		}
		p := pmgs[0]
		if p == nil {
			t.Fatal("PMGInstance view is nil")
		}
		if p.Name() != "pmg-branchcov" {
			t.Errorf("PMGInstance Name = %q, want pmg-branchcov", p.Name())
		}
	})

	t.Run("K8sCluster", func(t *testing.T) {
		clusters := rs.K8sClusters()
		if len(clusters) != 1 {
			t.Fatalf("K8sClusters() = %d views, want 1", len(clusters))
		}
		k := clusters[0]
		if k == nil {
			t.Fatal("K8sCluster view is nil")
		}
		if k.Name() != "k8s-branchcov" {
			t.Errorf("K8sCluster Name = %q, want k8s-branchcov", k.Name())
		}
	})
}

// TestBranchcov0723Am_SnapshotReadState_SourceIndependence proves the registry
// built by SnapshotReadState copies snapshot data rather than aliasing the
// caller's slice elements: after the read state is built, mutating the source
// snapshot must not change the already-materialized views. (The reverse
// direction is structurally impossible: views reference registry-owned
// Resources, never the caller's snapshot, so no view mutation can reach the
// source.)
func TestBranchcov0723Am_SnapshotReadState_SourceIndependence(t *testing.T) {
	snap := branchcov0723amPopulatedSnapshot()
	rs := SnapshotReadState(snap)

	vms := rs.VMs()
	if len(vms) != 1 || vms[0] == nil {
		t.Fatalf("expected one VM view before mutation; got %d", len(vms))
	}
	beforeName := vms[0].Name()
	if beforeName != "vm-branchcov" {
		t.Fatalf("VM Name before mutation = %q, want vm-branchcov", beforeName)
	}

	// Mutate the caller's snapshot in place. SnapshotReadState received (and
	// IngestSnapshot received) the snapshot by value and the registry copied
	// field values into its own Resources, so this must not leak through.
	snap.VMs[0].Name = "mutated-after-read"
	snap.VMs[0].VMID = 9999

	// The caller's own snapshot reflects the mutation (sanity), but the
	// already-built ReadState must be isolated from it.
	if snap.VMs[0].Name != "mutated-after-read" {
		t.Fatalf("test harness sanity check failed: caller snapshot Name = %q", snap.VMs[0].Name)
	}
	after := rs.VMs()
	if len(after) != 1 || after[0] == nil {
		t.Fatalf("after mutating source snapshot, ReadState has %d VMs, want 1", len(after))
	}
	if after[0].Name() != beforeName {
		t.Errorf("ReadState VM Name = %q after source mutation, want %q (source aliasing)", after[0].Name(), beforeName)
	}
	if after[0].VMID() != 4242 {
		t.Errorf("ReadState VM VMID = %d after source mutation, want 4242 (source aliasing)", after[0].VMID())
	}
}

// TestBranchcov0723Am_APITokenUsage_NormalizeCollections covers both arms of
// (u APITokenUsage).NormalizeCollections: nil Agents normalised to a non-nil
// empty slice, and a populated Agents slice preserved. Because the method has
// a value receiver, the caller's struct must be unchanged regardless of input.
func TestBranchcov0723Am_APITokenUsage_NormalizeCollections(t *testing.T) {
	cases := []struct {
		name       string
		input      APITokenUsage
		wantAgents int
		wantNil    bool
	}{
		{
			name:       "nil_agents_normalized_to_empty",
			input:      APITokenUsage{TokenID: "tok-1", AgentCount: 0, Agents: nil},
			wantAgents: 0,
			wantNil:    false,
		},
		{
			name:       "empty_slice_preserved_non_nil",
			input:      APITokenUsage{TokenID: "tok-2", Agents: []string{}},
			wantAgents: 0,
			wantNil:    false,
		},
		{
			name:       "populated_agents_preserved",
			input:      APITokenUsage{TokenID: "tok-3", AgentCount: 2, Agents: []string{"agent-a", "agent-b"}},
			wantAgents: 2,
			wantNil:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.input
			got := tc.input.NormalizeCollections()

			if (got.Agents == nil) != tc.wantNil {
				t.Errorf("Agents nil = %v, want %v", got.Agents == nil, tc.wantNil)
			}
			if len(got.Agents) != tc.wantAgents {
				t.Errorf("len(Agents) = %d, want %d", len(got.Agents), tc.wantAgents)
			}
			if got.TokenID != original.TokenID {
				t.Errorf("TokenID = %q, want %q", got.TokenID, original.TokenID)
			}
			if got.AgentCount != original.AgentCount {
				t.Errorf("AgentCount = %d, want %d", got.AgentCount, original.AgentCount)
			}
		})
	}
}

// TestBranchcov0723Am_DockerAgentAttention_NormalizeCollections covers both
// arms of (a DockerAgentAttention).NormalizeCollections: nil Issues normalised
// to a non-nil empty slice, and a populated Issues slice preserved. Value
// receiver => caller's struct is unchanged.
func TestBranchcov0723Am_DockerAgentAttention_NormalizeCollections(t *testing.T) {
	cases := []struct {
		name       string
		input      DockerAgentAttention
		wantIssues int
		wantNil    bool
	}{
		{
			name:       "nil_issues_normalized_to_empty",
			input:      DockerAgentAttention{AgentID: "agent-1", Issues: nil},
			wantIssues: 0,
			wantNil:    false,
		},
		{
			name:       "empty_slice_preserved_non_nil",
			input:      DockerAgentAttention{AgentID: "agent-2", Issues: []string{}},
			wantIssues: 0,
			wantNil:    false,
		},
		{
			name:       "populated_issues_preserved",
			input:      DockerAgentAttention{AgentID: "agent-3", Issues: []string{"stale", "no-token"}},
			wantIssues: 2,
			wantNil:    false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := tc.input
			got := tc.input.NormalizeCollections()

			if (got.Issues == nil) != tc.wantNil {
				t.Errorf("Issues nil = %v, want %v", got.Issues == nil, tc.wantNil)
			}
			if len(got.Issues) != tc.wantIssues {
				t.Errorf("len(Issues) = %d, want %d", len(got.Issues), tc.wantIssues)
			}
			if got.AgentID != original.AgentID {
				t.Errorf("AgentID = %q, want %q", got.AgentID, original.AgentID)
			}
			if got.Name != original.Name || got.Status != original.Status {
				t.Errorf("scalar fields changed: Name=%q Status=%q", got.Name, got.Status)
			}
		})
	}
}
