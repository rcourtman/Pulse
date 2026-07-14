package qualification

import (
	"context"
	"encoding/json"
	"errors"
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

func TestValidateCollectedScenarioProjectionUsesFaultOracle(t *testing.T) {
	manifest := validTestManifest()
	manifest.Faults = []FaultSpec{
		{ID: "stopped", Target: "dependency", Oracle: []Predicate{{Probe: "docker.running", Target: "dependency", Operator: "eq", Value: json.RawMessage("false")}}},
		{ID: "unhealthy", Target: "client", Oracle: []Predicate{{Probe: "docker.health", Target: "client", Operator: "eq", Value: json.RawMessage(`"unhealthy"`)}}},
		{ID: "restart-loop", Target: "worker", Oracle: []Predicate{{Probe: "docker.restart_count", Target: "worker", Operator: "gte", Value: json.RawMessage("2")}}},
	}
	resources := map[string]Resource{
		"dependency": {Docker: &DockerResource{ContainerState: "running"}},
		"client":     {Docker: &DockerResource{Health: "healthy"}},
		"worker":     {Docker: &DockerResource{RestartCount: 1}},
	}
	if err := validateCollectedScenarioProjection(manifest, resources); err == nil {
		t.Fatal("expected pre-fault collected projection to be rejected")
	}
	resources["dependency"] = Resource{Docker: &DockerResource{ContainerState: "exited"}}
	resources["client"] = Resource{Docker: &DockerResource{Health: "unhealthy"}}
	resources["worker"] = Resource{Docker: &DockerResource{RestartCount: 2}}
	if err := validateCollectedScenarioProjection(manifest, resources); err != nil {
		t.Fatalf("expected collected projection to satisfy scenario-owned oracles: %v", err)
	}
}

func TestValidateCollectedScenarioProjectionRequiresNegativeControlBaseline(t *testing.T) {
	manifest := validTestManifest()
	manifest.Faults = nil
	manifest.NegativeControls = []NegativeControl{{Resource: "healthy", Reason: "healthy control"}}
	manifest.Baseline = []Predicate{{Probe: "docker.health", Target: "healthy", Operator: "eq", Value: json.RawMessage(`"healthy"`)}}
	resources := map[string]Resource{"healthy": {Docker: &DockerResource{ContainerState: "running", Health: "starting"}}}

	if err := validateCollectedScenarioProjection(manifest, resources); err == nil {
		t.Fatal("expected stale starting projection to be rejected for a healthy negative control")
	}
	resources["healthy"] = Resource{Docker: &DockerResource{ContainerState: "running", Health: "healthy"}}
	if err := validateCollectedScenarioProjection(manifest, resources); err != nil {
		t.Fatalf("expected collected negative control to satisfy scenario-owned baseline: %v", err)
	}
}

func TestWaitForResourcesMatchingPollsPastStaleState(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		call := calls.Add(1)
		health := "healthy"
		if call >= 2 {
			health = "unhealthy"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{
			"id": "r1", "type": "app-container", "name": "fixture", "docker": map[string]any{"health": health},
		}}})
	}))
	defer server.Close()
	client, err := NewPulseClient(ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	resources, err := client.WaitForResourcesMatching(context.Background(), map[string]string{"target": "fixture"}, time.Second, time.Millisecond, func(resources map[string]Resource) error {
		if resources["target"].Docker == nil || resources["target"].Docker.Health != "unhealthy" {
			return errors.New("fault state is not collected yet")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() < 2 || resources["target"].Docker.Health != "unhealthy" {
		t.Fatalf("collection returned before fault projection converged: calls=%d resources=%+v", calls.Load(), resources)
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
