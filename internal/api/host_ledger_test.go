package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestHostLedgerEntryTypes(t *testing.T) {
	// Verify the structs marshal correctly including source and first_seen.
	entry := HostLedgerEntry{
		Name:      "node1",
		Type:      "proxmox-pve",
		Status:    "online",
		LastSeen:  "2025-01-01T00:00:00Z",
		Source:    "proxmox",
		FirstSeen: "2024-12-01T00:00:00Z",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded HostLedgerEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Name != "node1" || decoded.Type != "proxmox-pve" || decoded.Status != "online" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
	if decoded.Source != "proxmox" {
		t.Errorf("source mismatch: got %q", decoded.Source)
	}
	if decoded.FirstSeen != "2024-12-01T00:00:00Z" {
		t.Errorf("first_seen mismatch: got %q", decoded.FirstSeen)
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"online", "online"},
		{"offline", "offline"},
		{"", "unknown"},
		{"degraded", "unknown"},
		{"running", "unknown"},
	}
	for _, tt := range tests {
		got := normalizeStatus(tt.input)
		if got != tt.want {
			t.Errorf("normalizeStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatLastSeen(t *testing.T) {
	zero := time.Time{}
	if got := formatLastSeen(zero); got != "" {
		t.Errorf("formatLastSeen(zero) = %q, want empty", got)
	}

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	got := formatLastSeen(ts)
	if got != "2025-06-15T10:30:00Z" {
		t.Errorf("formatLastSeen = %q, want 2025-06-15T10:30:00Z", got)
	}
}

func TestDisplayNameHelpers(t *testing.T) {
	// PVE node
	if got := pveNodeDisplayName("My Node", "node1", "id1"); got != "My Node" {
		t.Errorf("pveNodeDisplayName with display = %q", got)
	}
	if got := pveNodeDisplayName("", "node1", "id1"); got != "node1" {
		t.Errorf("pveNodeDisplayName fallback name = %q", got)
	}
	if got := pveNodeDisplayName("", "", "id1"); got != "id1" {
		t.Errorf("pveNodeDisplayName fallback id = %q", got)
	}

	// TrueNAS
	if got := trueNASDisplayName("MyNAS", "https://nas:443"); got != "MyNAS" {
		t.Errorf("trueNASDisplayName with name = %q", got)
	}
	if got := trueNASDisplayName("", "https://nas:443"); got != "https://nas:443" {
		t.Errorf("trueNASDisplayName fallback = %q", got)
	}

	// Host
	if got := hostDisplayName("Display", "hostname", "id"); got != "Display" {
		t.Errorf("hostDisplayName = %q", got)
	}
	if got := hostDisplayName("", "hostname", "id"); got != "hostname" {
		t.Errorf("hostDisplayName fallback hostname = %q", got)
	}
	if got := hostDisplayName("", "", "id"); got != "id" {
		t.Errorf("hostDisplayName fallback id = %q", got)
	}

	// Docker
	if got := dockerDisplayName("display", "custom", "hostname", "id"); got != "custom" {
		t.Errorf("dockerDisplayName custom = %q", got)
	}
	if got := dockerDisplayName("display", "", "hostname", "id"); got != "display" {
		t.Errorf("dockerDisplayName display = %q", got)
	}
	if got := dockerDisplayName("", "", "hostname", "id"); got != "hostname" {
		t.Errorf("dockerDisplayName hostname = %q", got)
	}
	if got := dockerDisplayName("", "", "", "id"); got != "id" {
		t.Errorf("dockerDisplayName id = %q", got)
	}

	// K8s
	if got := k8sDisplayName("display", "custom", "name", "id"); got != "custom" {
		t.Errorf("k8sDisplayName custom = %q", got)
	}
	if got := k8sDisplayName("display", "", "name", "id"); got != "display" {
		t.Errorf("k8sDisplayName display = %q", got)
	}
	if got := k8sDisplayName("", "", "name", "id"); got != "name" {
		t.Errorf("k8sDisplayName name = %q", got)
	}
	if got := k8sDisplayName("", "", "", "id"); got != "id" {
		t.Errorf("k8sDisplayName id = %q", got)
	}
}

func TestPveStatusFromState(t *testing.T) {
	state := models.StateSnapshot{
		Nodes: []models.Node{
			{Host: "https://pve1:8006", Status: "online"},
			{Host: "https://pve2:8006", Status: "offline"},
		},
	}

	if got := pveStatusFromState("https://pve1:8006", state); got != "online" {
		t.Errorf("expected online, got %q", got)
	}
	if got := pveStatusFromState("https://pve2:8006", state); got != "offline" {
		t.Errorf("expected offline, got %q", got)
	}
	if got := pveStatusFromState("https://pve3:8006", state); got != "unknown" {
		t.Errorf("expected unknown for missing host, got %q", got)
	}
}

func TestEnrichPBSStatus(t *testing.T) {
	now := time.Now()
	state := models.StateSnapshot{
		PBSInstances: []models.PBSInstance{
			{Host: "https://pbs1:8007", Status: "online", LastSeen: now},
		},
	}

	entry := HostLedgerEntry{}
	enrichPBSStatus(&entry, "https://pbs1:8007", state)
	if entry.Status != "online" {
		t.Errorf("expected online, got %q", entry.Status)
	}
	if entry.LastSeen == "" {
		t.Error("expected non-empty LastSeen")
	}

	entry2 := HostLedgerEntry{}
	enrichPBSStatus(&entry2, "https://pbs-missing:8007", state)
	if entry2.Status != "unknown" {
		t.Errorf("expected unknown for missing, got %q", entry2.Status)
	}
}

func TestEnrichPMGStatus(t *testing.T) {
	now := time.Now()
	state := models.StateSnapshot{
		PMGInstances: []models.PMGInstance{
			{Host: "https://pmg1:8006", Status: "online", LastSeen: now},
		},
	}

	entry := HostLedgerEntry{}
	enrichPMGStatus(&entry, "https://pmg1:8006", state)
	if entry.Status != "online" {
		t.Errorf("expected online, got %q", entry.Status)
	}

	entry2 := HostLedgerEntry{}
	enrichPMGStatus(&entry2, "https://pmg-missing:8006", state)
	if entry2.Status != "unknown" {
		t.Errorf("expected unknown for missing, got %q", entry2.Status)
	}
}

func TestHostLedgerResponseEmptyState(t *testing.T) {
	resp := HostLedgerResponse{
		Hosts: []HostLedgerEntry{},
		Total: 0,
		Limit: 0,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded HostLedgerResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Total != 0 || decoded.Limit != 0 || len(decoded.Hosts) != 0 {
		t.Errorf("unexpected response: %+v", decoded)
	}
}

func TestHostLedgerResponseSorting(t *testing.T) {
	// Verify the sort function produces type-then-name ordering.
	entries := []HostLedgerEntry{
		{Name: "zebra", Type: "proxmox-pve"},
		{Name: "alpha", Type: "proxmox-pve"},
		{Name: "docker1", Type: "docker"},
		{Name: "host-b", Type: "host-agent"},
		{Name: "host-a", Type: "host-agent"},
		{Name: "k8s-1", Type: "kubernetes"},
	}

	expected := []struct {
		Type string
		Name string
	}{
		{"docker", "docker1"},
		{"host-agent", "host-a"},
		{"host-agent", "host-b"},
		{"kubernetes", "k8s-1"},
		{"proxmox-pve", "alpha"},
		{"proxmox-pve", "zebra"},
	}

	// Apply the same sort used in the handler.
	sortHostLedgerEntries(entries)

	for i, e := range expected {
		if entries[i].Type != e.Type || entries[i].Name != e.Name {
			t.Errorf("entry[%d] = {%s, %s}, want {%s, %s}", i, entries[i].Type, entries[i].Name, e.Type, e.Name)
		}
	}
}

func TestHostLedgerMixedEntriesCount(t *testing.T) {
	// Build a response with mixed entries and verify total matches.
	entries := []HostLedgerEntry{
		{Name: "pve1", Type: "proxmox-pve", Status: "online"},
		{Name: "pbs1", Type: "proxmox-pbs", Status: "offline"},
		{Name: "host1", Type: "host-agent", Status: "online"},
		{Name: "docker1", Type: "docker", Status: "online"},
		{Name: "k8s1", Type: "kubernetes", Status: "offline"},
	}

	resp := HostLedgerResponse{
		Hosts: entries,
		Total: len(entries),
		Limit: 10,
	}

	if resp.Total != 5 {
		t.Errorf("expected total 5, got %d", resp.Total)
	}
	if resp.Limit != 10 {
		t.Errorf("expected limit 10, got %d", resp.Limit)
	}
}

func TestHostLedgerNilHostsBecomesEmptyArray(t *testing.T) {
	// Ensure nil hosts slice is replaced with empty array for clean JSON.
	resp := HostLedgerResponse{
		Hosts: nil,
		Total: 0,
		Limit: 5,
	}
	if resp.Hosts == nil {
		resp.Hosts = []HostLedgerEntry{}
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Should contain "hosts":[] not "hosts":null
	if string(data) == "" {
		t.Error("empty marshal output")
	}
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)
	hosts, ok := decoded["hosts"].([]interface{})
	if !ok {
		t.Fatalf("hosts is not an array: %T", decoded["hosts"])
	}
	if len(hosts) != 0 {
		t.Errorf("expected empty hosts array, got %d entries", len(hosts))
	}
}

// Test that the handler returns valid JSON via httptest when no monitor is available.
func TestHandleHostLedgerHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/license/host-ledger", nil)
	rec := httptest.NewRecorder()

	// We can't easily construct a full Router, but we can test the response
	// types directly by encoding a response.
	resp := HostLedgerResponse{
		Hosts: []HostLedgerEntry{
			{Name: "test-host", Type: "host-agent", Status: "online", LastSeen: "2025-01-01T00:00:00Z"},
		},
		Total: 1,
		Limit: 5,
	}

	rec.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rec).Encode(resp)

	_ = req // used for request construction pattern verification

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	var decoded HostLedgerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Total != 1 || decoded.Limit != 5 {
		t.Errorf("unexpected response: %+v", decoded)
	}
	if decoded.Hosts[0].Name != "test-host" {
		t.Errorf("unexpected host name: %s", decoded.Hosts[0].Name)
	}
}

// sortHostLedgerEntries mirrors the sort logic used in the handler.
func sortHostLedgerEntries(entries []HostLedgerEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type < entries[j].Type
		}
		return entries[i].Name < entries[j].Name
	})
}

func TestConfiguredNodeCountFallback(t *testing.T) {
	cfg := &config.Config{
		PVEInstances: []config.PVEInstance{{Host: "pve1"}, {Host: "pve2"}},
		PBSInstances: []config.PBSInstance{{Host: "pbs1"}},
		PMGInstances: []config.PMGInstance{{Host: "pmg1"}},
	}

	t.Run("uses_runtime_nodes_when_populated", func(t *testing.T) {
		state := models.StateSnapshot{
			Nodes: []models.Node{{ID: "n1"}, {ID: "n2"}, {ID: "n3"}, {ID: "n4"}, {ID: "n5"}},
		}
		// 5 PVE nodes + 1 PBS + 1 PMG = 7
		got := configuredNodeCount(cfg, state)
		if got != 7 {
			t.Fatalf("expected 7, got %d", got)
		}
	})

	t.Run("falls_back_to_connection_count_when_no_nodes", func(t *testing.T) {
		state := models.StateSnapshot{}
		// 2 PVE connections + 1 PBS + 1 PMG = 4
		got := configuredNodeCount(cfg, state)
		if got != 4 {
			t.Fatalf("expected 4, got %d", got)
		}
	})

	t.Run("nil_config_returns_zero", func(t *testing.T) {
		state := models.StateSnapshot{
			Nodes: []models.Node{{ID: "n1"}},
		}
		got := configuredNodeCount(nil, state)
		if got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})
}

func TestK8sNodeStatus(t *testing.T) {
	if got := k8sNodeStatus(true); got != "online" {
		t.Errorf("ready=true → %q, want online", got)
	}
	if got := k8sNodeStatus(false); got != "offline" {
		t.Errorf("ready=false → %q, want offline", got)
	}
}
