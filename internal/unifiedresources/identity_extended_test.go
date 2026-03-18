package unifiedresources

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// --- NormalizeMAC ---

func TestNormalizeMAC(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"00:11:22:33:44:55", "00:11:22:33:44:55"},
		{"00-11-22-33-44-55", "00:11:22:33:44:55"},
		{"0011.2233.4455", "00:11:22:33:44:55"},
		{" AA:BB:CC:DD:EE:FF ", "aa:bb:cc:dd:ee:ff"},
		{"", ""},
		{"not-a-mac", "not-a-mac"}, // Falls through to lowercase
	}
	for _, tt := range tests {
		got := NormalizeMAC(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeMAC(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- NormalizeIP ---

func TestNormalizeIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"192.168.1.10", "192.168.1.10"},
		{" 10.0.0.1 ", "10.0.0.1"},
		{"192.168.1.10/24", "192.168.1.10"},
		{"::1", "::1"},
		{"fe80::1%eth0", ""}, // Scoped IPv6 not parseable
		{"not-an-ip", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeIP(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeIP(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- isNonUniqueIP ---

func TestIsNonUniqueIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"", true},
		{"127.0.0.1", true},
		{"127.0.1.1", true},
		{"::1", true},
		{"169.254.1.1", true},
		{"fe80::1", true},
		{"172.17.0.1", true},
		{"172.18.0.1", true},
		{"172.31.0.1", true},
		{"192.168.1.10", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false}, // 172.16 is NOT Docker bridge
		{"8.8.8.8", false},
	}
	for _, tt := range tests {
		got := isNonUniqueIP(tt.ip)
		if got != tt.expected {
			t.Errorf("isNonUniqueIP(%q) = %v, want %v", tt.ip, got, tt.expected)
		}
	}
}

// --- intersectIDs ---

func TestIntersectIDs_BothEmpty(t *testing.T) {
	result := intersectIDs(map[string]struct{}{}, map[string]struct{}{})
	if len(result) != 0 {
		t.Error("intersection of empty sets should be empty")
	}
}

func TestIntersectIDs_NoOverlap(t *testing.T) {
	a := map[string]struct{}{"x": {}}
	b := map[string]struct{}{"y": {}}
	result := intersectIDs(a, b)
	if len(result) != 0 {
		t.Error("non-overlapping sets should give empty intersection")
	}
}

func TestIntersectIDs_WithOverlap(t *testing.T) {
	a := map[string]struct{}{"x": {}, "y": {}}
	b := map[string]struct{}{"y": {}, "z": {}}
	result := intersectIDs(a, b)
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if _, ok := result["y"]; !ok {
		t.Error("expected 'y' in intersection")
	}
}

func TestIntersectIDs_OneEmpty(t *testing.T) {
	a := map[string]struct{}{"x": {}}
	b := map[string]struct{}{}
	result := intersectIDs(a, b)
	if len(result) != 0 {
		t.Error("intersection with empty set should be empty")
	}
}

// --- promoteCandidate ---

func TestPromoteCandidate_EmptyExisting(t *testing.T) {
	incoming := MatchCandidate{ID: "h-1", Confidence: 0.8, Reason: "test"}
	got := promoteCandidate(MatchCandidate{}, incoming)
	if got.ID != "h-1" {
		t.Error("empty existing should return incoming")
	}
}

func TestPromoteCandidate_HigherIncoming(t *testing.T) {
	existing := MatchCandidate{ID: "h-1", Confidence: 0.5, Reason: "hostname"}
	incoming := MatchCandidate{ID: "h-1", Confidence: 0.9, Reason: "hostname+mac"}
	got := promoteCandidate(existing, incoming)
	if got.Confidence != 0.9 {
		t.Errorf("expected higher incoming to win, got confidence %.2f", got.Confidence)
	}
}

func TestPromoteCandidate_LowerIncoming(t *testing.T) {
	existing := MatchCandidate{ID: "h-1", Confidence: 0.9, Reason: "hostname+mac"}
	incoming := MatchCandidate{ID: "h-1", Confidence: 0.5, Reason: "hostname"}
	got := promoteCandidate(existing, incoming)
	if got.Confidence != 0.9 {
		t.Errorf("expected existing to win, got confidence %.2f", got.Confidence)
	}
}

// --- FindCandidates: confidence ordering ---

func TestFindCandidates_SortedByConfidence(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-a", ResourceIdentity{
		Hostnames:    []string{"server1"},
		IPAddresses:  []string{"192.168.1.10"},
		MACAddresses: []string{"00:11:22:33:44:55"},
	})
	matcher.Add("host-b", ResourceIdentity{
		IPAddresses: []string{"192.168.1.20"},
	})

	candidates := matcher.FindCandidates(ResourceIdentity{
		Hostnames:    []string{"server1"},
		IPAddresses:  []string{"192.168.1.10", "192.168.1.20"},
		MACAddresses: []string{"00:11:22:33:44:55"},
	})

	if len(candidates) < 2 {
		t.Fatalf("expected at least 2 candidates, got %d", len(candidates))
	}
	if candidates[0].Confidence < candidates[1].Confidence {
		t.Error("candidates should be sorted by descending confidence")
	}
	// host-a should rank higher (hostname+mac = 0.90 vs ip only = 0.40)
	if candidates[0].ID != "host-a" {
		t.Errorf("expected host-a first, got %q", candidates[0].ID)
	}
}

func TestFindCandidates_EmptyIdentity(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-a", ResourceIdentity{Hostnames: []string{"pve1"}})
	candidates := matcher.FindCandidates(ResourceIdentity{})
	if len(candidates) != 0 {
		t.Errorf("empty identity should find no candidates, got %d", len(candidates))
	}
}

func TestFindCandidates_SkipsNonUniqueIPs(t *testing.T) {
	matcher := NewIdentityMatcher()
	matcher.Add("host-a", ResourceIdentity{IPAddresses: []string{"172.17.0.1"}})
	candidates := matcher.FindCandidates(ResourceIdentity{IPAddresses: []string{"172.17.0.1"}})
	if len(candidates) != 0 {
		t.Error("Docker bridge IPs should be skipped")
	}
}

// --- proxmoxNodeCorroboratesHost ---

func TestProxmoxNodeCorroboratesHost_HostnameMatch(t *testing.T) {
	node := models.Node{Name: "pve1.homelab.lan"}
	host := models.Host{Hostname: "PVE1.local"}
	if !proxmoxNodeCorroboratesHost(node, host) {
		t.Error("should corroborate on matching hostname after normalization")
	}
}

func TestProxmoxNodeCorroboratesHost_IPMatch(t *testing.T) {
	node := models.Node{Name: "pve1", Host: "https://192.168.1.10:8006"}
	host := models.Host{Hostname: "different", ReportIP: "192.168.1.10"}
	if !proxmoxNodeCorroboratesHost(node, host) {
		t.Error("should corroborate on matching endpoint IP")
	}
}

func TestProxmoxNodeCorroboratesHost_InterfaceIPMatch(t *testing.T) {
	node := models.Node{Name: "pve1", Host: "https://192.168.1.10:8006"}
	host := models.Host{
		Hostname: "different",
		ReportIP: "10.0.0.1",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Addresses: []string{"192.168.1.10"}},
		},
	}
	if !proxmoxNodeCorroboratesHost(node, host) {
		t.Error("should corroborate on matching interface IP")
	}
}

func TestProxmoxNodeCorroboratesHost_NoMatch(t *testing.T) {
	node := models.Node{Name: "pve1", Host: "https://192.168.1.10:8006"}
	host := models.Host{Hostname: "pve2", ReportIP: "192.168.1.20"}
	if proxmoxNodeCorroboratesHost(node, host) {
		t.Error("should not corroborate with different hostname and IP")
	}
}

func TestProxmoxNodeCorroboratesHost_HostnameEndpointMatch(t *testing.T) {
	node := models.Node{Name: "pve1", Host: "https://pve1.local:8006"}
	host := models.Host{Hostname: "pve1.homelab.lan"}
	if !proxmoxNodeCorroboratesHost(node, host) {
		t.Error("should corroborate when endpoint hostname normalizes to match host hostname")
	}
}

// --- trustedProxmoxNodeHostLink ---

func TestTrustedProxmoxNodeHostLink_MutualLinkedIDs(t *testing.T) {
	node := models.Node{ID: "node-1", Name: "pve1"}
	host := models.Host{ID: "host-1", LinkedNodeID: "node-1"}
	if !trustedProxmoxNodeHostLink(node, host) {
		t.Error("mutual linked IDs should be trusted")
	}
}

func TestTrustedProxmoxNodeHostLink_NoLink(t *testing.T) {
	node := models.Node{ID: "node-1", Name: "pve1", Host: "https://192.168.1.10:8006"}
	host := models.Host{ID: "host-1", Hostname: "different-host", ReportIP: "10.0.0.1"}
	if trustedProxmoxNodeHostLink(node, host) {
		t.Error("unrelated node and host should not be trusted")
	}
}

// --- proxmoxNodeLinkKeys ---

func TestProxmoxNodeLinkKeys_EmptyName(t *testing.T) {
	node := models.Node{Name: ""}
	keys := proxmoxNodeLinkKeys(node)
	if keys != nil {
		t.Error("empty name should return nil keys")
	}
}

func TestProxmoxNodeLinkKeys_WithCluster(t *testing.T) {
	node := models.Node{Name: "pve1", ClusterName: "mycluster"}
	keys := proxmoxNodeLinkKeys(node)
	found := false
	for _, k := range keys {
		if k == "cluster:mycluster:pve1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cluster key, got %v", keys)
	}
}

func TestProxmoxNodeLinkKeys_WithEndpointHost(t *testing.T) {
	node := models.Node{Name: "pve1", Host: "https://pve1.homelab.lan:8006"}
	keys := proxmoxNodeLinkKeys(node)
	if len(keys) == 0 {
		t.Fatal("expected at least one key")
	}
}

func TestProxmoxNodeLinkKeys_WithIPEndpoint(t *testing.T) {
	node := models.Node{Name: "pve1", Host: "https://192.168.1.10:8006"}
	keys := proxmoxNodeLinkKeys(node)
	found := false
	for _, k := range keys {
		if k == "endpoint-ip:192.168.1.10:pve1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected endpoint-ip key, got %v", keys)
	}
}
