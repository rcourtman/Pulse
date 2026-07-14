package qualification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestTriggerAndWaitAssociatesExactNewScopedRun(t *testing.T) {
	var triggered atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/ai/patrol/run":
			triggered.Store(true)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"success":true}`))
		case "/api/ai/patrol/runs":
			if !triggered.Load() {
				_, _ = w.Write([]byte(`[{"id":"old","started_at":"2026-01-01T00:00:00Z","completed_at":"2026-01-01T00:00:01Z"}]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"new","started_at":"2099-01-01T00:00:00Z","completed_at":"2099-01-01T00:00:01Z","scope_resource_ids":["r1"]},{"id":"old","started_at":"2026-01-01T00:00:00Z","completed_at":"2026-01-01T00:00:01Z"}]`))
		case "/api/ai/patrol/runs/new":
			_, _ = w.Write([]byte(`{"id":"new","started_at":"2099-01-01T00:00:00Z","completed_at":"2099-01-01T00:00:01Z","scope_resource_ids":["r1"],"tool_calls":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := NewPulseClient(ClientConfig{BaseURL: server.URL, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	run, err := client.TriggerAndWait(context.Background(), []string{"r1"}, "test", 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if run.ID != "new" {
		t.Fatalf("run id = %q", run.ID)
	}
}

func TestNewPulseClientRejectsCredentialsInReportableBaseURL(t *testing.T) {
	if _, err := NewPulseClient(ClientConfig{BaseURL: "https://admin:secret@example.test"}); err == nil {
		t.Fatal("base URL credentials must be rejected rather than persisted in reports")
	}
}

func TestDecideActionBindsExactPlanHash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/actions/action-1/decision" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		payload, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			t.Fatal(err)
		}
		if body["outcome"] != "rejected" || body["planHash"] != "sha256:exact" {
			t.Fatalf("decision body = %s", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"actionId":"action-1","state":"rejected","audit":{"id":"action-1","state":"rejected","plan":{"planHash":"sha256:exact"}}}`))
	}))
	defer server.Close()
	client, err := NewPulseClient(ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	audit, err := client.DecideAction(context.Background(), "action-1", "rejected", "operator rejected", "sha256:exact")
	if err != nil {
		t.Fatal(err)
	}
	if audit.ID != "action-1" || audit.Plan.PlanHash != "sha256:exact" {
		t.Fatalf("audit = %+v", audit)
	}
}

func TestFilterRunFindingsUsesRunIDsOrFreshExactResources(t *testing.T) {
	now := time.Now().UTC()
	before := []Finding{{ID: "updated", ResourceID: "r1", LastSeenAt: now.Add(-time.Minute)}}
	after := []Finding{
		{ID: "run-owned", ResourceID: "other", LastSeenAt: now},
		{ID: "updated", ResourceID: "r1", LastSeenAt: now},
		{ID: "stale", ResourceID: "r1", LastSeenAt: now.Add(-2 * time.Minute)},
	}
	got := filterRunFindings(before, after, PatrolRun{FindingIDs: []string{"run-owned"}}, map[string]Resource{"target": {ID: "r1"}}, now.Add(-time.Second))
	encoded, _ := json.Marshal(got)
	if len(got) != 2 || got[0].ID != "run-owned" || got[1].ID != "updated" {
		t.Fatalf("findings = %s", encoded)
	}
}
