package api

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

func TestExtractReportNetworkIDs(t *testing.T) {
	ifaces := []agentshost.NetworkInterface{
		{Name: "eth0", MAC: "aa:bb:cc:dd:ee:01", Addresses: []string{"192.168.1.10/24", "fe80::1/64"}},
		{Name: "lo", Addresses: []string{"127.0.0.1/8"}},
		{Name: "eth1", MAC: "aa:bb:cc:dd:ee:02"},
	}

	ips, macs := extractReportNetworkIDs(ifaces)

	wantIPs := []string{"192.168.1.10", "fe80::1", "127.0.0.1"}
	wantMACs := []string{"aa:bb:cc:dd:ee:01", "aa:bb:cc:dd:ee:02"}

	if len(ips) != len(wantIPs) {
		t.Fatalf("IPs: got %d, want %d: %v", len(ips), len(wantIPs), ips)
	}
	for i, ip := range ips {
		if ip != wantIPs[i] {
			t.Errorf("IP[%d]: got %q, want %q", i, ip, wantIPs[i])
		}
	}

	if len(macs) != len(wantMACs) {
		t.Fatalf("MACs: got %d, want %d: %v", len(macs), len(wantMACs), macs)
	}
	for i, mac := range macs {
		if mac != wantMACs[i] {
			t.Errorf("MAC[%d]: got %q, want %q", i, mac, wantMACs[i])
		}
	}
}

func TestHostReportCandidates_IncludesNetworkIdentity(t *testing.T) {
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:        "host-1",
			Hostname:  "server-a",
			MachineID: "mid-123",
			ReportIP:  "10.0.0.5",
		},
		Network: []agentshost.NetworkInterface{
			{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff", Addresses: []string{"192.168.1.10/24"}},
		},
	}

	candidates := hostReportCandidates(report)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Identity.MachineID != "mid-123" {
		t.Errorf("MachineID: got %q, want %q", c.Identity.MachineID, "mid-123")
	}
	if len(c.Identity.MACAddresses) != 1 || c.Identity.MACAddresses[0] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MACAddresses: got %v, want [aa:bb:cc:dd:ee:ff]", c.Identity.MACAddresses)
	}
	// Should include both the interface IP and the ReportIP.
	if len(c.Identity.IPAddresses) != 2 {
		t.Errorf("IPAddresses: got %v, want 2 entries", c.Identity.IPAddresses)
	}
}

func TestDockerReportCandidates_IncludesNetworkIdentity(t *testing.T) {
	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-host",
			MachineID: "mid-456",
			Network: []agentsdocker.NetworkInterface{
				{Name: "eth0", MAC: "11:22:33:44:55:66", Addresses: []string{"172.16.0.2/16"}},
			},
		},
	}

	candidates := dockerReportCandidates(report)
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	c := candidates[0]
	if c.Identity.MachineID != "mid-456" {
		t.Errorf("MachineID: got %q", c.Identity.MachineID)
	}
	if len(c.Identity.MACAddresses) != 1 || c.Identity.MACAddresses[0] != "11:22:33:44:55:66" {
		t.Errorf("MACAddresses: got %v", c.Identity.MACAddresses)
	}
	if len(c.Identity.IPAddresses) != 1 || c.Identity.IPAddresses[0] != "172.16.0.2" {
		t.Errorf("IPAddresses: got %v", c.Identity.IPAddresses)
	}
}

func TestK8sReportCandidates_NodeIDCollisionProtection(t *testing.T) {
	report := agentsk8s.Report{
		Cluster: agentsk8s.ClusterInfo{Name: "prod"},
		Nodes: []agentsk8s.Node{
			{Name: "node-1"},
			{Name: "node-2"},
			{}, // empty — should get idx-2
		},
	}

	candidates := k8sReportCandidates(report)
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	ids := make(map[string]bool)
	for _, c := range candidates {
		if ids[c.ID] {
			t.Errorf("duplicate ID: %s", c.ID)
		}
		ids[c.ID] = true
	}

	// Third candidate should use idx-2 fallback.
	if candidates[2].ID != "report-k8s-node:prod:idx-2" {
		t.Errorf("third candidate ID: got %q, want report-k8s-node:prod:idx-2", candidates[2].ID)
	}
}

// TestReportDedupViaMAC verifies that a host report with matching MAC address
// deduplicates with an existing host-agent candidate (the scenario that was
// previously broken when report candidates lacked network identity).
func TestReportDedupViaMAC(t *testing.T) {
	// Existing host agent candidate (from live state).
	existing := unifiedresources.HostCandidate{
		ID:     "host:existing-1",
		Name:   "server-a",
		Type:   "host-agent",
		Source: "agent",
		Status: "online",
		Identity: unifiedresources.ResourceIdentity{
			Hostnames:    []string{"server-a"},
			MACAddresses: []string{"aa:bb:cc:dd:ee:ff"},
		},
	}

	// New report candidate — same hostname and MAC, no machine-id.
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:       "new-agent",
			Hostname: "server-a",
		},
		Network: []agentshost.NetworkInterface{
			{Name: "eth0", MAC: "aa:bb:cc:dd:ee:ff"},
		},
	}
	reportCandidates := hostReportCandidates(report)

	// Project dedup.
	all := append([]unifiedresources.HostCandidate{existing}, reportCandidates...)
	resolved := unifiedresources.ResolveHosts(all)

	// Should dedup into 1 host (hostname + MAC >= 0.90 confidence).
	if len(resolved.Hosts) != 1 {
		t.Errorf("expected 1 deduped host, got %d", len(resolved.Hosts))
		for _, h := range resolved.Hosts {
			t.Logf("  host: %s (sources: %v)", h.Name, h.SourceLabels)
		}
	}
}
