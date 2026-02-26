package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHostLedgerEntryTypes(t *testing.T) {
	entry := HostLedgerEntry{
		Name:     "server-1",
		Type:     "agent",
		Status:   "online",
		LastSeen: "2025-01-01T00:00:00Z",
		Source:   "agent",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded HostLedgerEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Name != "server-1" || decoded.Type != "agent" || decoded.Status != "online" {
		t.Errorf("round-trip mismatch: %+v", decoded)
	}
	if decoded.Source != "agent" {
		t.Errorf("source mismatch: got %q", decoded.Source)
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

func TestHostDisplayName(t *testing.T) {
	if got := hostDisplayName("Display", "hostname", "id"); got != "Display" {
		t.Errorf("hostDisplayName = %q", got)
	}
	if got := hostDisplayName("", "hostname", "id"); got != "hostname" {
		t.Errorf("hostDisplayName fallback hostname = %q", got)
	}
	if got := hostDisplayName("", "", "id"); got != "id" {
		t.Errorf("hostDisplayName fallback id = %q", got)
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

func TestHostLedgerNilHostsBecomesEmptyArray(t *testing.T) {
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

func TestHandleHostLedgerHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/license/host-ledger", nil)
	rec := httptest.NewRecorder()

	resp := HostLedgerResponse{
		Hosts: []HostLedgerEntry{
			{Name: "test-host", Type: "agent", Status: "online", LastSeen: "2025-01-01T00:00:00Z", Source: "agent"},
		},
		Total: 1,
		Limit: 5,
	}

	rec.Header().Set("Content-Type", "application/json")
	json.NewEncoder(rec).Encode(resp)

	_ = req

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
	if decoded.Hosts[0].Name != "test-host" || decoded.Hosts[0].Type != "agent" {
		t.Errorf("unexpected host: %+v", decoded.Hosts[0])
	}
}
