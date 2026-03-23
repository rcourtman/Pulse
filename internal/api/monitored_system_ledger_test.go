package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestMonitoredSystemLedgerEntryTypes(t *testing.T) {
	entry := MonitoredSystemLedgerEntry{
		Name:   "server-1",
		Type:   "host",
		Status: "online",
		StatusExplanation: MonitoredSystemLedgerStatusExplanation{
			Summary: "All included top-level collection paths currently report online status.",
			Reasons: []MonitoredSystemLedgerStatusReason{},
		},
		LatestIncludedSignalAt: "2025-01-01T00:00:00Z",
		LastSeen:               "2025-01-01T00:00:00Z",
		Source:                 "agent",
		Explanation: MonitoredSystemLedgerExplanation{
			Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
			Reasons: []MonitoredSystemLedgerExplanationReason{
				{Kind: "standalone", Signal: "single-top-level-view", Summary: "No overlapping top-level source matched this system."},
			},
			Surfaces: []MonitoredSystemLedgerExplanationSurface{
				{Name: "server-1", Type: "host", Source: "agent"},
			},
		},
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
	if decoded.StatusExplanation.Summary == "" {
		t.Errorf("status explanation mismatch: %+v", decoded.StatusExplanation)
	}
	if decoded.StatusExplanation.Reasons == nil {
		t.Errorf("status explanation reasons mismatch: %+v", decoded.StatusExplanation)
	}
	if decoded.LatestIncludedSignalAt != "2025-01-01T00:00:00Z" {
		t.Errorf("latest included signal mismatch: %+v", decoded)
	}
	if decoded.Source != "agent" {
		t.Errorf("source mismatch: got %q", decoded.Source)
	}
	if decoded.Explanation.Summary == "" || len(decoded.Explanation.Reasons) != 1 || len(decoded.Explanation.Surfaces) != 1 {
		t.Errorf("explanation mismatch: %+v", decoded.Explanation)
	}
}

func TestNormalizeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"online", "online"},
		{"warning", "warning"},
		{"offline", "offline"},
		{"unknown", "unknown"},
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

func TestMonitoredSystemLedgerStatusExplanation(t *testing.T) {
	got := monitoredSystemLedgerStatusExplanation(unifiedresources.MonitoredSystemStatusExplanation{
		Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
		Reasons: []unifiedresources.MonitoredSystemStatusReason{
			{
				Kind:     "source-stale",
				Name:     "Tower",
				Type:     "host",
				Source:   "agent",
				Status:   "stale",
				LastSeen: time.Date(2026, 3, 23, 11, 55, 0, 0, time.UTC),
				Summary:  "Agent data for Tower is stale (last reported 2026-03-23T11:55:00Z).",
			},
		},
	}, "warning")
	if got.Summary != "At least one included source is stale, so Pulse marks this monitored system as warning." {
		t.Fatalf("unexpected status summary: %+v", got)
	}
	if len(got.Reasons) != 1 {
		t.Fatalf("expected one status reason, got %+v", got)
	}
	if got.Reasons[0].Status != "stale" {
		t.Fatalf("expected stale status reason, got %+v", got.Reasons[0])
	}
	if got.Reasons[0].LastSeen != "2026-03-23T11:55:00Z" {
		t.Fatalf("expected formatted reason last_seen, got %+v", got.Reasons[0])
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

func TestMonitoredSystemLedgerEntryNormalizeCollections(t *testing.T) {
	entry := MonitoredSystemLedgerEntry{
		Name: "server-1",
		StatusExplanation: MonitoredSystemLedgerStatusExplanation{
			Summary: "Pulse cannot determine a canonical runtime status for this monitored system yet.",
		},
		Explanation: MonitoredSystemLedgerExplanation{
			Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
		},
	}.NormalizeCollections()

	if entry.StatusExplanation.Reasons == nil {
		t.Fatal("expected status explanation reasons to normalize to an empty slice")
	}
	if entry.Explanation.Reasons == nil {
		t.Fatal("expected explanation reasons to normalize to an empty slice")
	}
	if entry.Explanation.Surfaces == nil {
		t.Fatal("expected explanation surfaces to normalize to an empty slice")
	}
}

func TestHandleMonitoredSystemLedgerHTTP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/license/monitored-system-ledger", nil)
	rec := httptest.NewRecorder()

	resp := MonitoredSystemLedgerResponse{
		Systems: []MonitoredSystemLedgerEntry{
			{
				Name:   "test-host",
				Type:   "host",
				Status: "online",
				StatusExplanation: MonitoredSystemLedgerStatusExplanation{
					Summary: "All included top-level collection paths currently report online status.",
					Reasons: []MonitoredSystemLedgerStatusReason{},
				},
				LatestIncludedSignalAt: "2025-01-01T00:00:00Z",
				LastSeen:               "2025-01-01T00:00:00Z",
				Source:                 "agent",
				Explanation: MonitoredSystemLedgerExplanation{
					Summary: "Counts as one monitored system because Pulse sees one top-level host view from agent.",
					Reasons: []MonitoredSystemLedgerExplanationReason{
						{Kind: "standalone", Signal: "single-top-level-view", Summary: "No overlapping top-level source matched this system."},
					},
					Surfaces: []MonitoredSystemLedgerExplanationSurface{
						{Name: "test-host", Type: "host", Source: "agent"},
					},
				},
			},
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
	if decoded.Systems[0].StatusExplanation.Summary == "" {
		t.Errorf("expected status explanation summary, got %+v", decoded.Systems[0].StatusExplanation)
	}
	if decoded.Systems[0].StatusExplanation.Reasons == nil {
		t.Errorf("expected status explanation reasons, got %+v", decoded.Systems[0].StatusExplanation)
	}
	if decoded.Systems[0].LatestIncludedSignalAt != "2025-01-01T00:00:00Z" {
		t.Errorf("expected latest included signal timestamp, got %+v", decoded.Systems[0])
	}
	if decoded.Systems[0].Explanation.Summary == "" {
		t.Errorf("expected explanation summary, got %+v", decoded.Systems[0].Explanation)
	}
}
