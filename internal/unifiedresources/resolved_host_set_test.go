package unifiedresources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// ---------------------------------------------------------------------------
// ResolveHosts — core dedup logic
// ---------------------------------------------------------------------------

func TestResolveHosts_Empty(t *testing.T) {
	result := ResolveHosts(nil)
	if len(result.Hosts) != 0 {
		t.Fatalf("expected 0 hosts, got %d", len(result.Hosts))
	}
}

func TestResolveHosts_NoDuplicates(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}}},
		{ID: "agent:h1", Name: "standalone", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-2", Hostnames: []string{"standalone"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(result.Hosts))
	}
}

func TestResolveHosts_MachineIDMerge(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 merged host, got %d", len(result.Hosts))
	}
	h := result.Hosts[0]
	if h.Name != "pve1" {
		t.Fatalf("expected name pve1, got %q", h.Name)
	}
	if h.PrimaryType != "proxmox-pve" {
		t.Fatalf("expected primary type proxmox-pve, got %q", h.PrimaryType)
	}
	if len(h.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(h.Sources))
	}
	if len(h.SourceLabels) != 2 {
		t.Fatalf("expected 2 source labels, got %d", len(h.SourceLabels))
	}
}

func TestResolveHosts_DMIUUIDMerge(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{DMIUUID: "uuid-1", Hostnames: []string{"pve1"}}},
		{ID: "docker:d1", Name: "pve1", Type: "docker", Source: "docker", Status: "online",
			Identity: ResourceIdentity{DMIUUID: "uuid-1", Hostnames: []string{"pve1"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 merged host, got %d", len(result.Hosts))
	}
}

func TestResolveHosts_HostnameAndMACMerge(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}, MACAddresses: []string{"00:11:22:33:44:55"}}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}, MACAddresses: []string{"00:11:22:33:44:55"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 merged host (hostname+MAC), got %d", len(result.Hosts))
	}
}

func TestResolveHosts_HostnameOnlyDoesNotMerge(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{Hostnames: []string{"pve1"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 2 {
		t.Fatalf("hostname-only match should not merge (below threshold), got %d hosts", len(result.Hosts))
	}
}

func TestResolveHosts_IPOnlyDoesNotMerge(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{IPAddresses: []string{"192.168.1.10"}}},
		{ID: "agent:h1", Name: "host1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{IPAddresses: []string{"192.168.1.10"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 2 {
		t.Fatalf("IP-only match should not merge, got %d hosts", len(result.Hosts))
	}
}

func TestResolveHosts_ThreeWayMerge(t *testing.T) {
	// PVE node + host agent + docker all on the same machine.
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
		{ID: "docker:d1", Name: "pve1", Type: "docker", Source: "docker", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 merged host from 3 sources, got %d", len(result.Hosts))
	}
	h := result.Hosts[0]
	if len(h.Sources) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(h.Sources))
	}
	if h.PrimaryType != "proxmox-pve" {
		t.Fatalf("expected PVE as primary type, got %q", h.PrimaryType)
	}
}

func TestResolveHosts_StatusPromotion(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "unknown",
			Identity: ResourceIdentity{MachineID: "machine-1"}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1"}},
	}
	result := ResolveHosts(candidates)
	if result.Hosts[0].Status != "online" {
		t.Fatalf("expected status online, got %q", result.Hosts[0].Status)
	}
}

func TestResolveHosts_ProvisionalClearedByRuntime(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "config-pve:p1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox",
			Status: "unknown", Provisional: true,
			Identity: ResourceIdentity{MachineID: "machine-1"}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent",
			Status: "online", Provisional: false,
			Identity: ResourceIdentity{MachineID: "machine-1"}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(result.Hosts))
	}
	if result.Hosts[0].Provisional {
		t.Fatalf("expected provisional=false when runtime source present")
	}
}

func TestResolveHosts_SortOrder(t *testing.T) {
	candidates := []HostCandidate{
		{ID: "agent:h1", Name: "zulu", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{MachineID: "m1"}},
		{ID: "docker:d1", Name: "alpha", Type: "docker", Source: "docker", Status: "online",
			Identity: ResourceIdentity{MachineID: "m2"}},
		{ID: "pve:n1", Name: "bravo", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{MachineID: "m3"}},
	}
	result := ResolveHosts(candidates)
	// Sorted by type then name: agent:zulu, docker:alpha, proxmox-pve:bravo
	if result.Hosts[0].PrimaryType != "agent" {
		t.Fatalf("expected agent first, got %q", result.Hosts[0].PrimaryType)
	}
	if result.Hosts[1].PrimaryType != "docker" {
		t.Fatalf("expected docker second, got %q", result.Hosts[1].PrimaryType)
	}
	if result.Hosts[2].PrimaryType != "proxmox-pve" {
		t.Fatalf("expected proxmox-pve third, got %q", result.Hosts[2].PrimaryType)
	}
}

// ---------------------------------------------------------------------------
// CollectHostCandidates
// ---------------------------------------------------------------------------

func TestCollectHostCandidates_PVENodesFromRuntime(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "pve1", Host: "https://pve1.local:8006", Status: "online",
				LastSeen: time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)},
			{ID: "n2", Name: "pve2", Host: "https://pve2.local:8006", Status: "offline"},
		},
	}
	candidates := CollectHostCandidates(state, nil, nil, nil, nil)
	if len(candidates) != 2 {
		t.Fatalf("expected 2 PVE candidates, got %d", len(candidates))
	}
	if candidates[0].Type != "proxmox-pve" {
		t.Fatalf("expected proxmox-pve type, got %q", candidates[0].Type)
	}
	if candidates[0].LastSeen == "" {
		t.Fatalf("expected non-empty LastSeen for online node")
	}
}

func TestCollectHostCandidates_PVENodeEndpointIPBecomesIdentityIP(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "minipc", Host: "https://10.0.0.5:8006", Status: "online"},
		},
	}

	candidates := CollectHostCandidates(state, nil, nil, nil, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if len(candidates[0].Identity.Hostnames) != 1 || candidates[0].Identity.Hostnames[0] != "minipc" {
		t.Fatalf("Hostnames = %v, want [minipc]", candidates[0].Identity.Hostnames)
	}
	if len(candidates[0].Identity.IPAddresses) != 1 || candidates[0].Identity.IPAddresses[0] != "10.0.0.5" {
		t.Fatalf("IPAddresses = %v, want [10.0.0.5]", candidates[0].Identity.IPAddresses)
	}
}

func TestCollectHostCandidates_PVEFallbackToConfig(t *testing.T) {
	state := models.StateSnapshot{}
	configPVE := []ConfigEntry{{ID: "p1", Name: "my-pve", Host: "https://pve.local:8006"}}
	candidates := CollectHostCandidates(state, configPVE, nil, nil, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 PVE config candidate, got %d", len(candidates))
	}
	if !candidates[0].Provisional {
		t.Fatalf("config-only PVE should be provisional")
	}
}

func TestCollectHostCandidates_AllSourceTypes(t *testing.T) {
	state := models.StateSnapshot{
		Nodes:       []models.Node{{ID: "n1", Name: "pve1", Status: "online"}},
		Hosts:       []models.Host{{ID: "h1", Hostname: "host1", Status: "online", MachineID: "m1"}},
		DockerHosts: []models.DockerHost{{ID: "d1", Hostname: "docker1", Status: "online", MachineID: "m2"}},
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "k8s-prod", Status: "online",
				Nodes: []models.KubernetesNode{{UID: "kn1", Name: "k8s-node1", Ready: true}}},
		},
	}
	configPBS := []ConfigEntry{{ID: "pbs1", Name: "backup-srv", Host: "https://pbs.local:8007"}}
	configPMG := []ConfigEntry{{ID: "pmg1", Name: "mail-gw", Host: "https://pmg.local:8006"}}
	configTrueNAS := []ConfigEntry{{ID: "tn1", Name: "nas1", Host: "https://nas.local"}}

	candidates := CollectHostCandidates(state, nil, configPBS, configPMG, configTrueNAS)
	// 1 PVE + 1 PBS + 1 PMG + 1 TrueNAS + 1 host + 1 docker + 1 k8s node = 7
	if len(candidates) != 7 {
		t.Fatalf("expected 7 candidates, got %d", len(candidates))
	}

	types := make(map[string]int)
	for _, c := range candidates {
		types[c.Type]++
	}
	expected := map[string]int{
		"proxmox-pve": 1,
		"proxmox-pbs": 1,
		"proxmox-pmg": 1,
		"truenas":     1,
		"agent":       1,
		"docker":      1,
		"kubernetes":  1,
	}
	for typ, want := range expected {
		if types[typ] != want {
			t.Errorf("type %q: expected %d, got %d", typ, want, types[typ])
		}
	}
}

func TestCollectHostCandidates_K8sNoNodesCountsAsOne(t *testing.T) {
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "k8s-prod", Status: "online", Nodes: nil},
		},
	}
	candidates := CollectHostCandidates(state, nil, nil, nil, nil)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 K8s cluster candidate, got %d", len(candidates))
	}
	if !candidates[0].Provisional {
		t.Fatalf("K8s cluster with no nodes should be provisional")
	}
}

func TestCollectHostCandidates_K8sMultipleNodes(t *testing.T) {
	state := models.StateSnapshot{
		KubernetesClusters: []models.KubernetesCluster{
			{ID: "k1", Name: "k8s-prod", Status: "online", Nodes: []models.KubernetesNode{
				{UID: "kn1", Name: "node1", Ready: true},
				{UID: "kn2", Name: "node2", Ready: true},
				{UID: "kn3", Name: "node3", Ready: false},
			}},
		},
	}
	candidates := CollectHostCandidates(state, nil, nil, nil, nil)
	if len(candidates) != 3 {
		t.Fatalf("expected 3 K8s node candidates, got %d", len(candidates))
	}
}

// ---------------------------------------------------------------------------
// K8s node identity enrichment
// ---------------------------------------------------------------------------

func TestEnrichK8sNodeIdentity_MatchesHostAgent(t *testing.T) {
	hosts := []models.Host{
		{ID: "h1", Hostname: "k8s-node1", MachineID: "machine-abc",
			NetworkInterfaces: []models.HostNetworkInterface{
				{Name: "eth0", MAC: "00:11:22:33:44:55"},
			}},
	}
	identity := ResourceIdentity{Hostnames: []string{"k8s-node1"}}
	enrichK8sNodeIdentity(&identity, "k8s-node1", hosts)

	if identity.MachineID != "machine-abc" {
		t.Fatalf("expected machine-id enrichment, got %q", identity.MachineID)
	}
	if len(identity.MACAddresses) != 1 || identity.MACAddresses[0] != "00:11:22:33:44:55" {
		t.Fatalf("expected MAC enrichment, got %v", identity.MACAddresses)
	}
}

func TestEnrichK8sNodeIdentity_NoMatch(t *testing.T) {
	hosts := []models.Host{
		{ID: "h1", Hostname: "other-host", MachineID: "machine-abc"},
	}
	identity := ResourceIdentity{Hostnames: []string{"k8s-node1"}}
	enrichK8sNodeIdentity(&identity, "k8s-node1", hosts)

	if identity.MachineID != "" {
		t.Fatalf("expected no machine-id enrichment, got %q", identity.MachineID)
	}
}

func TestEnrichK8sNodeIdentity_NormalizesHostname(t *testing.T) {
	hosts := []models.Host{
		{ID: "h1", Hostname: "K8S-NODE1.local", MachineID: "machine-abc"},
	}
	identity := ResourceIdentity{Hostnames: []string{"k8s-node1"}}
	enrichK8sNodeIdentity(&identity, "k8s-node1", hosts)

	if identity.MachineID != "machine-abc" {
		t.Fatalf("expected match via normalized hostname, got %q", identity.MachineID)
	}
}

// ---------------------------------------------------------------------------
// End-to-end dedup scenario
// ---------------------------------------------------------------------------

func TestResolveHosts_EndToEnd_PVEWithAgent(t *testing.T) {
	// Simulates the common case: PVE node with a host agent on the same machine.
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "n1", Name: "pve1", Host: "https://192.168.1.10:8006", Status: "online",
				LinkedAgentID: "h1",
				LastSeen:      time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)},
		},
		Hosts: []models.Host{
			{ID: "h1", Hostname: "pve1", MachineID: "machine-1", Status: "online",
				LastSeen: time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"192.168.1.10/24"}},
				}},
		},
		DockerHosts: []models.DockerHost{
			{ID: "d1", Hostname: "pve1", MachineID: "machine-1", Status: "online",
				LastSeen: time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC),
				NetworkInterfaces: []models.HostNetworkInterface{
					{Name: "eth0", MAC: "00:11:22:33:44:55"},
				}},
		},
	}

	candidates := CollectHostCandidates(state, nil, nil, nil, nil)
	result := ResolveHosts(candidates)

	if len(result.Hosts) != 1 {
		t.Fatalf("expected 1 resolved host after linked host identity propagation, got %d", len(result.Hosts))
	}
	if result.Hosts[0].PrimaryType != "proxmox-pve" {
		t.Fatalf("expected proxmox-pve primary type, got %q", result.Hosts[0].PrimaryType)
	}
	if len(result.Hosts[0].Sources) != 3 {
		t.Fatalf("expected 3 merged sources, got %d", len(result.Hosts[0].Sources))
	}
	if result.Hosts[0].Identity.MachineID != "machine-1" {
		t.Fatalf("expected propagated machine-id, got %q", result.Hosts[0].Identity.MachineID)
	}
}

func TestResolveHosts_EndToEnd_MachineIDPropagation(t *testing.T) {
	// When PVE node and host agent both report the same machine-id, they merge.
	candidates := []HostCandidate{
		{ID: "pve:n1", Name: "pve1", Type: "proxmox-pve", Source: "proxmox", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
		{ID: "agent:h1", Name: "pve1", Type: "agent", Source: "agent", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"},
				MACAddresses: []string{"00:11:22:33:44:55"}}},
		{ID: "docker:d1", Name: "pve1", Type: "docker", Source: "docker", Status: "online",
			Identity: ResourceIdentity{MachineID: "machine-1", Hostnames: []string{"pve1"}}},
	}
	result := ResolveHosts(candidates)
	if len(result.Hosts) != 1 {
		t.Fatalf("expected all 3 to merge via machine-id, got %d", len(result.Hosts))
	}
	h := result.Hosts[0]
	if h.PrimaryType != "proxmox-pve" {
		t.Fatalf("PVE should be primary, got %q", h.PrimaryType)
	}
	if len(h.SourceLabels) != 3 {
		t.Fatalf("expected 3 source labels, got %v", h.SourceLabels)
	}
}

// ---------------------------------------------------------------------------
// betterStatus
// ---------------------------------------------------------------------------

func TestBetterStatus(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"unknown", "offline", "offline"},
		{"offline", "online", "online"},
		{"online", "unknown", "online"},
		{"online", "online", "online"},
		{"unknown", "unknown", "unknown"},
	}
	for _, tc := range cases {
		if got := betterStatus(tc.a, tc.b); got != tc.want {
			t.Errorf("betterStatus(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// resolvedFormatTime
// ---------------------------------------------------------------------------

func TestResolvedFormatTime(t *testing.T) {
	if got := resolvedFormatTime(time.Time{}); got != "" {
		t.Fatalf("zero time should return empty, got %q", got)
	}
	ts := time.Date(2026, 2, 25, 10, 30, 0, 0, time.UTC)
	got := resolvedFormatTime(ts)
	if got != "2026-02-25T10:30:00Z" {
		t.Fatalf("expected RFC3339, got %q", got)
	}
}
