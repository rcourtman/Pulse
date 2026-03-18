package models

import (
	"testing"
	"time"
)

// --- normalizeNodeIdentityPart ---

func TestNormalizeNodeIdentityPart(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"  PVE1  ", "pve1"},
		{"ONLINE", "online"},
		{"", ""},
		{"  ", ""},
	}
	for _, tt := range tests {
		got := normalizeNodeIdentityPart(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeNodeIdentityPart(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- normalizeIPAddress ---

func TestNormalizeIPAddress(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"192.168.1.10", "192.168.1.10"},
		{" 10.0.0.1 ", "10.0.0.1"},
		{"192.168.1.10/24", "192.168.1.10"},
		{"::1", "::1"},
		{"not-an-ip", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := normalizeIPAddress(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeIPAddress(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- extractHostEndpoint ---

func TestExtractHostEndpoint(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"https://pve1.homelab.lan:8006", "pve1.homelab.lan"},
		{"https://192.168.1.10:8006", "192.168.1.10"},
		{"http://server:9090/path", "server"},
		{"192.168.1.10:8006", "192.168.1.10"},
		{"pve1.local", "pve1.local"},
		{"", ""},
		{"  ", ""},
	}
	for _, tt := range tests {
		got := extractHostEndpoint(tt.input)
		if got != tt.expected {
			t.Errorf("extractHostEndpoint(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- shortHostname ---

func TestShortHostname(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"pve1.homelab.lan", "pve1"},
		{"PVE1.local", "pve1"},
		{"pve1", "pve1"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortHostname(tt.input)
		if got != tt.expected {
			t.Errorf("shortHostname(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- nodeLogicalKey ---

func TestNodeLogicalKey_Empty(t *testing.T) {
	got := nodeLogicalKey(Node{})
	if got != "" {
		t.Errorf("empty node should have empty key, got %q", got)
	}
}

func TestNodeLogicalKey_Cluster(t *testing.T) {
	got := nodeLogicalKey(Node{Name: "pve1", ClusterName: "mycluster"})
	if got != "cluster:mycluster:pve1" {
		t.Errorf("expected cluster key, got %q", got)
	}
}

func TestNodeLogicalKey_Instance(t *testing.T) {
	got := nodeLogicalKey(Node{Name: "pve1", Instance: "inst-a"})
	if got != "instance:inst-a:pve1" {
		t.Errorf("expected instance key, got %q", got)
	}
}

func TestNodeLogicalKey_Endpoint(t *testing.T) {
	got := nodeLogicalKey(Node{Name: "pve1", Host: "https://192.168.1.10:8006"})
	if got != "endpoint:https://192.168.1.10:8006:pve1" {
		t.Errorf("expected endpoint key, got %q", got)
	}
}

func TestNodeLogicalKey_NameOnly(t *testing.T) {
	got := nodeLogicalKey(Node{Name: "pve1"})
	if got != "name:pve1" {
		t.Errorf("expected name-only key, got %q", got)
	}
}

// --- nodeStatusRank ---

func TestNodeStatusRank(t *testing.T) {
	tests := []struct {
		status   string
		expected int
	}{
		{"online", 3},
		{"ONLINE", 3},
		{"degraded", 2},
		{"unknown", 1},
		{"offline", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := nodeStatusRank(tt.status)
		if got != tt.expected {
			t.Errorf("nodeStatusRank(%q) = %d, want %d", tt.status, got, tt.expected)
		}
	}
}

// --- nodeConnectionHealthRank ---

func TestNodeConnectionHealthRank(t *testing.T) {
	tests := []struct {
		health   string
		expected int
	}{
		{"healthy", 3},
		{"degraded", 2},
		{"unknown", 1},
		{"offline", 0},
	}
	for _, tt := range tests {
		got := nodeConnectionHealthRank(tt.health)
		if got != tt.expected {
			t.Errorf("nodeConnectionHealthRank(%q) = %d, want %d", tt.health, got, tt.expected)
		}
	}
}

// --- preferNodeForMerge ---

func TestPreferNodeForMerge_HigherStatus(t *testing.T) {
	existing := Node{Name: "pve1", Status: "unknown"}
	candidate := Node{Name: "pve1", Status: "online"}
	got := preferNodeForMerge(existing, candidate)
	if got.Status != "online" {
		t.Error("higher status node should win")
	}
}

func TestPreferNodeForMerge_LinkedAgent(t *testing.T) {
	existing := Node{Name: "pve1", Status: "online", ConnectionHealth: "healthy"}
	candidate := Node{Name: "pve1", Status: "online", ConnectionHealth: "healthy", LinkedAgentID: "agent-1"}
	got := preferNodeForMerge(existing, candidate)
	if got.LinkedAgentID != "agent-1" {
		t.Error("linked agent should break tie")
	}
}

func TestPreferNodeForMerge_LastSeenBreaksTie(t *testing.T) {
	now := time.Now()
	existing := Node{Name: "pve1", Status: "online", ConnectionHealth: "healthy", LastSeen: now.Add(-time.Hour)}
	candidate := Node{Name: "pve1", Status: "online", ConnectionHealth: "healthy", LastSeen: now}
	got := preferNodeForMerge(existing, candidate)
	if got.LastSeen != now {
		t.Error("more recent LastSeen should win tie")
	}
}

func TestPreferNodeForMerge_ExistingWinsTotalTie(t *testing.T) {
	now := time.Now()
	existing := Node{Name: "pve1", Status: "online", LastSeen: now}
	candidate := Node{Name: "pve1", Status: "online", LastSeen: now}
	got := preferNodeForMerge(existing, candidate)
	// On total tie, existing wins.
	if got.Name != existing.Name {
		t.Error("total tie should keep existing")
	}
}

// --- backupKey ---

func TestBackupKey(t *testing.T) {
	got := backupKey("pve1", 100)
	if got != "pve1-100" {
		t.Errorf("backupKey(%q, %d) = %q, want %q", "pve1", 100, got, "pve1-100")
	}
}

// --- namespaceMatchesInstance ---

func TestNamespaceMatchesInstance(t *testing.T) {
	tests := []struct {
		namespace, instance string
		expected            bool
	}{
		{"pve1", "pve1", true},                  // Exact match
		{"PVE1", "pve1", true},                  // Case-insensitive
		{"nat", "pve-nat", true},                // Namespace is suffix of normalized instance
		{"pvebackups", "pve", false},            // "pve" is a prefix not suffix of "pvebackups"
		{"", "pve1", false},                     // Empty namespace
		{"pve1", "", false},                     // Empty instance
		{"completely-different", "pve1", false}, // No match
	}
	for _, tt := range tests {
		got := namespaceMatchesInstance(tt.namespace, tt.instance)
		if got != tt.expected {
			t.Errorf("namespaceMatchesInstance(%q, %q) = %v, want %v", tt.namespace, tt.instance, got, tt.expected)
		}
	}
}

// --- nodeEndpointMergeAliases ---

func TestNodeEndpointMergeAliases_EmptyName(t *testing.T) {
	aliases := nodeEndpointMergeAliases(Node{Host: "https://192.168.1.10:8006"})
	if aliases != nil {
		t.Error("empty name should return nil")
	}
}

func TestNodeEndpointMergeAliases_EmptyHost(t *testing.T) {
	aliases := nodeEndpointMergeAliases(Node{Name: "pve1"})
	if aliases != nil {
		t.Error("empty host should return nil")
	}
}

func TestNodeEndpointMergeAliases_IPEndpoint(t *testing.T) {
	aliases := nodeEndpointMergeAliases(Node{Name: "pve1", Host: "https://192.168.1.10:8006"})
	if len(aliases) != 1 {
		t.Fatalf("expected 1 alias, got %d: %v", len(aliases), aliases)
	}
	if aliases[0] != "endpoint-ip:192.168.1.10:pve1" {
		t.Errorf("expected endpoint-ip alias, got %q", aliases[0])
	}
}

func TestNodeEndpointMergeAliases_HostnameEndpoint(t *testing.T) {
	aliases := nodeEndpointMergeAliases(Node{Name: "pve1", Host: "https://pve1.homelab.lan:8006"})
	if len(aliases) < 1 {
		t.Fatal("expected at least 1 alias")
	}
	found := false
	for _, a := range aliases {
		if a == "endpoint-host:pve1.homelab.lan:pve1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected endpoint-host alias, got %v", aliases)
	}
}

// --- registerNodeAliases ---

func TestRegisterNodeAliases_Basic(t *testing.T) {
	aliasToKey := make(map[string]string)
	ambiguous := make(map[string]struct{})
	registerNodeAliases(aliasToKey, ambiguous, []string{"alias-1"}, "key-1")
	if aliasToKey["alias-1"] != "key-1" {
		t.Error("alias should be registered")
	}
}

func TestRegisterNodeAliases_AmbiguousOnConflict(t *testing.T) {
	aliasToKey := map[string]string{"alias-1": "key-1"}
	ambiguous := make(map[string]struct{})
	registerNodeAliases(aliasToKey, ambiguous, []string{"alias-1"}, "key-2")
	if _, ok := ambiguous["alias-1"]; !ok {
		t.Error("conflicting alias should be marked ambiguous")
	}
	if _, ok := aliasToKey["alias-1"]; ok {
		t.Error("ambiguous alias should be removed from aliasToKey")
	}
}

func TestRegisterNodeAliases_SkipsAlreadyAmbiguous(t *testing.T) {
	aliasToKey := make(map[string]string)
	ambiguous := map[string]struct{}{"alias-1": {}}
	registerNodeAliases(aliasToKey, ambiguous, []string{"alias-1"}, "key-1")
	if _, ok := aliasToKey["alias-1"]; ok {
		t.Error("already ambiguous alias should not be registered")
	}
}

func TestRegisterNodeAliases_SkipsEmpty(t *testing.T) {
	aliasToKey := make(map[string]string)
	ambiguous := make(map[string]struct{})
	registerNodeAliases(aliasToKey, ambiguous, []string{""}, "key-1")
	registerNodeAliases(aliasToKey, ambiguous, []string{"alias-1"}, "")
	if len(aliasToKey) != 0 {
		t.Error("empty values should be skipped")
	}
}
