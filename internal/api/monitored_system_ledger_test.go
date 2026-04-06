package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
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
		LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
			Name:   "server-1",
			Type:   "host",
			Source: "agent",
			At:     "2025-01-01T00:00:00Z",
		},
		Source: "agent",
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
	if decoded.LatestIncludedSignal.Name != "server-1" || decoded.LatestIncludedSignal.Type != "host" || decoded.LatestIncludedSignal.At != "2025-01-01T00:00:00Z" {
		t.Errorf("latest included signal payload mismatch: %+v", decoded.LatestIncludedSignal)
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

func TestFormatMonitoredSystemTime(t *testing.T) {
	zero := time.Time{}
	if got := formatMonitoredSystemTime(zero); got != "" {
		t.Errorf("formatMonitoredSystemTime(zero) = %q, want empty", got)
	}

	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	got := formatMonitoredSystemTime(ts)
	if got != "2025-06-15T10:30:00Z" {
		t.Errorf("formatMonitoredSystemTime = %q, want 2025-06-15T10:30:00Z", got)
	}
}

func TestMonitoredSystemLedgerStatusExplanation(t *testing.T) {
	got := monitoredSystemLedgerStatusExplanation(unifiedresources.MonitoredSystemStatusExplanation{
		Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
		Reasons: []unifiedresources.MonitoredSystemStatusReason{
			{
				Kind:       "source-stale",
				Name:       "Tower",
				Type:       "host",
				Source:     "agent",
				Status:     "stale",
				ReportedAt: time.Date(2026, 3, 23, 11, 55, 0, 0, time.UTC),
				Summary:    "Agent data for Tower is stale (last reported 2026-03-23T11:55:00Z).",
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
	if got.Reasons[0].ReportedAt != "2026-03-23T11:55:00Z" {
		t.Fatalf("expected formatted reason reported_at, got %+v", got.Reasons[0])
	}
}

func TestMonitoredSystemLedgerEntryDoesNotEmitCompatibilityAliases(t *testing.T) {
	got := monitoredSystemLedgerEntry(unifiedresources.MonitoredSystemRecord{
		Name:   "Tower",
		Type:   "host",
		Status: unifiedresources.StatusWarning,
		StatusExplanation: unifiedresources.MonitoredSystemStatusExplanation{
			Summary: "At least one included source is stale, so Pulse marks this monitored system as warning.",
			Reasons: []unifiedresources.MonitoredSystemStatusReason{
				{
					Kind:       "source-stale",
					Name:       "Tower",
					Type:       "host",
					Source:     "agent",
					Status:     "stale",
					ReportedAt: time.Date(2026, 3, 23, 11, 55, 0, 0, time.UTC),
					Summary:    "Agent data for Tower is stale (last reported 2026-03-23T11:55:00Z).",
				},
			},
		},
		LastSeen: time.Date(2026, 3, 23, 12, 5, 0, 0, time.UTC),
		LatestIncludedSignal: unifiedresources.MonitoredSystemLatestSignal{
			Name:   "tower.local",
			Type:   "docker-host",
			Source: "docker",
			At:     time.Date(2026, 3, 23, 12, 0, 0, 0, time.UTC),
		},
		Source: "multiple",
		Explanation: unifiedresources.MonitoredSystemGroupingExplanation{
			Summary:  "Counts as one monitored system because Pulse merged 2 top-level views into one canonical system using shared machine identity.",
			Reasons:  []unifiedresources.MonitoredSystemGroupingReason{},
			Surfaces: []unifiedresources.MonitoredSystemGroupingSurface{},
		},
	})

	if got.LatestIncludedSignal.At != "2026-03-23T12:00:00Z" {
		t.Fatalf("expected canonical latest signal timestamp, got %+v", got.LatestIncludedSignal)
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := decoded["latest_included_signal_at"]; ok {
		t.Fatalf("expected latest_included_signal_at to be absent, got %+v", decoded)
	}
	if _, ok := decoded["latest_included_signal_source"]; ok {
		t.Fatalf("expected latest_included_signal_source to be absent, got %+v", decoded)
	}
	if _, ok := decoded["last_seen"]; ok {
		t.Fatalf("expected last_seen to be absent, got %+v", decoded)
	}
	statusExplanation, ok := decoded["status_explanation"].(map[string]any)
	if !ok {
		t.Fatalf("expected status_explanation object, got %+v", decoded)
	}
	reasons, ok := statusExplanation["reasons"].([]any)
	if !ok || len(reasons) != 1 {
		t.Fatalf("expected one status reason, got %+v", statusExplanation)
	}
	reason, ok := reasons[0].(map[string]any)
	if !ok {
		t.Fatalf("expected status reason object, got %+v", reasons[0])
	}
	if _, ok := reason["last_seen"]; ok {
		t.Fatalf("expected nested reason last_seen to be absent, got %+v", reason)
	}
	if reason["reported_at"] != "2026-03-23T11:55:00Z" {
		t.Fatalf("expected nested reason reported_at, got %+v", reason)
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
				LatestIncludedSignal: MonitoredSystemLedgerLatestSignal{
					Name:   "test-host",
					Type:   "host",
					Source: "agent",
					At:     "2025-01-01T00:00:00Z",
				},
				Source: "agent",
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
	if decoded.Systems[0].LatestIncludedSignal.Source != "agent" {
		t.Errorf("expected latest included signal payload, got %+v", decoded.Systems[0].LatestIncludedSignal)
	}
	if decoded.Systems[0].Explanation.Summary == "" {
		t.Errorf("expected explanation summary, got %+v", decoded.Systems[0].Explanation)
	}
}

func TestHandleMonitoredSystemLedger_DemoModeReturnsNotFound(t *testing.T) {
	router := &Router{config: &config.Config{DemoMode: true}}
	req := httptest.NewRequest(http.MethodGet, "/api/license/monitored-system-ledger", nil)
	rec := httptest.NewRecorder()

	router.handleMonitoredSystemLedger(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}
