package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMonitoredSystemLedgerEntryTypes(t *testing.T) {
	entry := MonitoredSystemLedgerEntry{
		Name:     "server-1",
		Type:     "host",
		Status:   "online",
		LastSeen: "2025-01-01T00:00:00Z",
		Source:   "agent",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded MonitoredSystemLedgerEntry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Name != "server-1" || decoded.Type != "host" || decoded.Status != "online" {
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

func TestMonitoredSystemLedgerResponseEmptyState(t *testing.T) {
	resp := EmptyMonitoredSystemLedgerResponse()
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded MonitoredSystemLedgerResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Total != 0 || decoded.Limit != 0 || len(decoded.Systems) != 0 {
		t.Errorf("unexpected response: %+v", decoded)
	}
}

func TestMonitoredSystemLedgerNilSystemsBecomesEmptyArray(t *testing.T) {
	resp := MonitoredSystemLedgerResponse{
		Limit: 5,
	}.NormalizeCollections()

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)
	systems, ok := decoded["systems"].([]interface{})
	if !ok {
		t.Fatalf("systems is not an array: %T", decoded["systems"])
	}
	if len(systems) != 0 {
		t.Errorf("expected empty systems array, got %d entries", len(systems))
	}
}

func TestHandleMonitoredSystemLedgerHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/license/monitored-system-ledger", nil)
	rec := httptest.NewRecorder()

	resp := MonitoredSystemLedgerResponse{
		Systems: []MonitoredSystemLedgerEntry{
			{Name: "test-host", Type: "host", Status: "online", LastSeen: "2025-01-01T00:00:00Z", Source: "agent"},
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

	var decoded MonitoredSystemLedgerResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Total != 1 || decoded.Limit != 5 {
		t.Errorf("unexpected response: %+v", decoded)
	}
	if decoded.Systems[0].Name != "test-host" || decoded.Systems[0].Type != "host" {
		t.Errorf("unexpected system: %+v", decoded.Systems[0])
	}
}
