package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func recentChangesFixture(t *testing.T, resources []unified.Resource, changes []unified.ResourceChange) *AgentContextHandler {
	t.Helper()
	h, _, _ := fleetFixtureHandlers(t, resources)
	req := httptest.NewRequest(http.MethodGet, agentcapabilities.RecentChangesCapabilityPath, nil)
	store, err := h.resources.getStore(GetOrgID(req.Context()))
	if err != nil {
		t.Fatalf("getStore: %v", err)
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange(%s): %v", change.ID, err)
		}
	}
	return h
}

func recentChange(id, resourceID string, kind unified.ChangeKind, observedAt time.Time) unified.ResourceChange {
	return unified.ResourceChange{
		ID:         id,
		ObservedAt: observedAt,
		ResourceID: resourceID,
		Kind:       kind,
		SourceType: unified.SourcePulseDiff,
		Confidence: unified.ConfidenceHigh,
	}
}

func getRecentChanges(t *testing.T, h *AgentContextHandler, rawQuery string, token *config.APITokenRecord) (*httptest.ResponseRecorder, AgentRecentChanges) {
	t.Helper()
	target := agentcapabilities.RecentChangesCapabilityPath
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if token != nil {
		attachAPITokenRecord(req, token)
	}
	rec := httptest.NewRecorder()
	h.HandleRecentChanges(rec, req)
	var out AgentRecentChanges
	if rec.Code == http.StatusOK {
		if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return rec, out
}

func TestHandleAgentRecentChanges_DefaultWindowNamesAndOrder(t *testing.T) {
	now := time.Now().UTC()
	h := recentChangesFixture(t,
		[]unified.Resource{{ID: "vm:101", Type: "vm", Name: "db-01"}},
		[]unified.ResourceChange{
			recentChange("old", "vm:101", unified.ChangeRestart, now.Add(-30*time.Hour)),
			recentChange("earlier", "vm:101", unified.ChangeStateTransition, now.Add(-2*time.Hour)),
			recentChange("latest", "vm:101", unified.ChangeRestart, now.Add(-time.Hour)),
			recentChange("gone", "vm:999", unified.ChangeStateTransition, now.Add(-90*time.Minute)),
		})

	rec, out := getRecentChanges(t, h, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	ids := make([]string, 0, len(out.Changes))
	for _, change := range out.Changes {
		ids = append(ids, change.ID)
	}
	if got := strings.Join(ids, ","); got != "latest,gone,earlier" {
		t.Fatalf("changes = %s; want newest first inside the default 24h window", got)
	}
	if out.Count != 3 || out.Truncated || out.Limit != agentcapabilities.RecentChangesDefaultLimit {
		t.Fatalf("count=%d truncated=%v limit=%d", out.Count, out.Truncated, out.Limit)
	}
	if out.Changes[0].ResourceName != "db-01" || out.Changes[0].ResourceType != "vm" {
		t.Fatalf("known resource not named: %+v", out.Changes[0])
	}
	if out.Changes[1].ResourceName != "" {
		t.Fatalf("history for a resource no longer known must stay readable without a name: %+v", out.Changes[1])
	}
}

func TestHandleAgentRecentChanges_FiltersWindowResourceAndKind(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	h := recentChangesFixture(t,
		[]unified.Resource{{ID: "vm:101", Type: "vm", Name: "db-01"}, {ID: "vm:102", Type: "vm", Name: "web-01"}},
		[]unified.ResourceChange{
			recentChange("a", "vm:101", unified.ChangeRestart, now.Add(-5*time.Hour)),
			recentChange("b", "vm:101", unified.ChangeAlertFired, now.Add(-4*time.Hour)),
			recentChange("c", "vm:102", unified.ChangeRestart, now.Add(-3*time.Hour)),
			recentChange("d", "vm:101", unified.ChangeRestart, now.Add(-time.Hour)),
		})

	since := now.Add(-6 * time.Hour).Format(time.RFC3339)
	before := now.Add(-2 * time.Hour).Format(time.RFC3339)
	rec, out := getRecentChanges(t, h, "since="+since+"&before="+before+"&resourceId=vm:101&kind=restart", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if len(out.Changes) != 1 || out.Changes[0].ID != "a" || out.Count != 1 {
		t.Fatalf("filtered changes = %+v (count %d); want only a", out.Changes, out.Count)
	}
	if out.Before == nil || !out.Before.Equal(now.Add(-2*time.Hour)) {
		t.Fatalf("effective before = %v", out.Before)
	}

	_, out = getRecentChanges(t, h, "since="+since+"&kind=restart,alert_fired", nil)
	if out.Count != 4 {
		t.Fatalf("comma-separated kinds count = %d; want 4", out.Count)
	}
}

func TestHandleAgentRecentChanges_CapsLimitAndReportsTruncation(t *testing.T) {
	now := time.Now().UTC()
	h := recentChangesFixture(t, nil, []unified.ResourceChange{
		recentChange("a", "vm:101", unified.ChangeRestart, now.Add(-3*time.Hour)),
		recentChange("b", "vm:101", unified.ChangeRestart, now.Add(-2*time.Hour)),
		recentChange("c", "vm:101", unified.ChangeRestart, now.Add(-time.Hour)),
	})

	_, out := getRecentChanges(t, h, "limit=2", nil)
	if len(out.Changes) != 2 || out.Count != 3 || !out.Truncated {
		t.Fatalf("limit=2: len=%d count=%d truncated=%v", len(out.Changes), out.Count, out.Truncated)
	}
	_, out = getRecentChanges(t, h, "limit=100000", nil)
	if out.Limit != agentcapabilities.RecentChangesMaxLimit || out.Truncated {
		t.Fatalf("oversized limit: effective=%d truncated=%v", out.Limit, out.Truncated)
	}
}

func TestHandleAgentRecentChanges_RejectsMalformedFilters(t *testing.T) {
	h := recentChangesFixture(t, nil, nil)
	for _, query := range []string{
		"since=yesterday",
		"before=soon",
		"limit=0",
		"limit=ten",
		"kind=reboot",
		"sourceType=rumour",
		"since=2026-10-28T10:00:00Z&before=2026-10-28T09:00:00Z",
	} {
		rec, _ := getRecentChanges(t, h, query, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d; want 400", query, rec.Code)
		}
	}
}

func TestHandleAgentRecentChanges_RedactsCommandsWithoutAIExecute(t *testing.T) {
	command := unified.BuildCommandExecutionChange("vm:101", "alert-1", "patrol", "systemctl restart nginx", true, "secret output", map[string]any{"script": "cat /etc/shadow"})
	runbook := unified.BuildRunbookExecutionChange("vm:101", "alert-1", "operator", "rb-1", "Restart web", "succeeded", false, "ran systemctl restart nginx", nil)
	h := recentChangesFixture(t, []unified.Resource{{ID: "vm:101", Type: "vm", Name: "db-01"}}, []unified.ResourceChange{*command, *runbook})

	_, readOnly := getRecentChanges(t, h, "", &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead}})
	if len(readOnly.Changes) != 2 {
		t.Fatalf("changes = %d; want 2", len(readOnly.Changes))
	}
	body, _ := json.Marshal(readOnly)
	for _, leaked := range []string{"systemctl", "secret output", "/etc/shadow"} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("monitoring-read response leaked %q: %s", leaked, body)
		}
	}
	for _, change := range readOnly.Changes {
		if !change.CommandRedacted {
			t.Fatalf("%s not marked redacted: %+v", change.Kind, change)
		}
		if change.Metadata[unified.MetadataAlertIdentifier] != "alert-1" {
			t.Fatalf("%s lost safe metadata: %+v", change.Kind, change.Metadata)
		}
		if change.Kind == unified.ChangeCommandExecuted && change.Reason != "Command succeeded" {
			t.Fatalf("command reason = %q", change.Reason)
		}
	}

	_, executor := getRecentChanges(t, h, "", &config.APITokenRecord{Scopes: []string{config.ScopeMonitoringRead, config.ScopeAIExecute}})
	body, _ = json.Marshal(executor)
	if !strings.Contains(string(body), "systemctl restart nginx") || strings.Contains(string(body), `"commandRedacted":true`) {
		t.Fatalf("ai:execute token must read command payloads: %s", body)
	}
}

func TestRecentChangeKindsParseThroughSharedFilterContract(t *testing.T) {
	for _, kind := range agentcapabilities.RecentChangeKinds {
		filters, err := unified.ParseResourceChangeFilters([]string{kind}, nil, nil)
		if err != nil || len(filters.Kinds) != 1 || string(filters.Kinds[0]) != kind {
			t.Errorf("documented kind %q does not parse: %v %+v", kind, err, filters)
		}
	}
}
