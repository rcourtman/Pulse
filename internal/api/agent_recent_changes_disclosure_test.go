package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

const recentChangePrivateText = "fixture-execution-detail-not-for-monitoring"

func TestAgentRecentChangesExecutionProjectionUsesTypedFacts(t *testing.T) {
	command := unified.BuildCommandExecutionChange("vm:101", "alert-1", "operator", recentChangePrivateText, true, recentChangePrivateText, map[string]any{
		"title":                   recentChangePrivateText,
		unified.MetadataOutcome:   map[string]any{"output": recentChangePrivateText},
		unified.MetadataAutomatic: recentChangePrivateText,
		unified.MetadataRunbookID: []string{recentChangePrivateText},
	})
	runbook := unified.BuildRunbookExecutionChange("vm:101", "alert-1", "operator", "rb-1", recentChangePrivateText, "resolved", false, recentChangePrivateText, nil)
	cases := []struct {
		name     string
		change   unified.ResourceChange
		reason   string
		metadata map[string]any
	}{
		{"command_builder", *command, "Command succeeded", map[string]any{unified.MetadataAlertIdentifier: "alert-1", unified.MetadataSuccess: true}},
		{"runbook_builder", *runbook, "Runbook executed (resolved)", map[string]any{unified.MetadataAlertIdentifier: "alert-1", unified.MetadataRunbookID: "rb-1", unified.MetadataOutcome: "resolved", unified.MetadataAutomatic: false}},
		{"failed_command_untrusted_reason", unified.ResourceChange{Kind: unified.ChangeCommandExecuted, Reason: recentChangePrivateText, Metadata: map[string]any{unified.MetadataSuccess: false}}, "Command failed", map[string]any{unified.MetadataSuccess: false}},
		{"missing_command_status", unified.ResourceChange{Kind: unified.ChangeCommandExecuted, Reason: recentChangePrivateText}, "Command executed", map[string]any{}},
		{"malformed_command_status", unified.ResourceChange{Kind: unified.ChangeCommandExecuted, Reason: recentChangePrivateText, From: recentChangePrivateText, To: recentChangePrivateText, Metadata: map[string]any{unified.MetadataSuccess: "true", unified.MetadataAlertIdentifier: map[string]any{"output": recentChangePrivateText}}}, "Command executed", map[string]any{}},
		{"malformed_runbook_metadata", unified.ResourceChange{Kind: unified.ChangeRunbookExecuted, Reason: recentChangePrivateText, From: recentChangePrivateText, To: recentChangePrivateText, Metadata: map[string]any{unified.MetadataAlertIdentifier: []string{recentChangePrivateText}, unified.MetadataRunbookID: map[string]any{"output": recentChangePrivateText}, unified.MetadataAutomatic: "false", unified.MetadataOutcome: recentChangePrivateText, "title": recentChangePrivateText, unified.MetadataSuccess: true}}, "Runbook executed", map[string]any{}},
		{"empty_runbook", unified.ResourceChange{Kind: unified.ChangeRunbookExecuted}, "Runbook executed", map[string]any{}},
	}
	for _, outcome := range []string{"partial", "failed", "unknown"} {
		cases = append(cases, struct {
			name     string
			change   unified.ResourceChange
			reason   string
			metadata map[string]any
		}{outcome, unified.ResourceChange{Kind: unified.ChangeRunbookExecuted, Metadata: map[string]any{unified.MetadataOutcome: outcome, unified.MetadataAutomatic: true}}, "Runbook executed (" + outcome + ")", map[string]any{unified.MetadataOutcome: outcome, unified.MetadataAutomatic: true}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original, err := json.Marshal(tc.change)
			if err != nil {
				t.Fatal(err)
			}
			entry := AgentRecentChange{ResourceChange: tc.change, ResourceName: "db-01", ResourceType: "vm"}
			got := redactAgentRecentChangeCommand(entry)
			if got.Reason != tc.reason || !reflect.DeepEqual(got.Metadata, tc.metadata) || !got.CommandRedacted || got.From != "" || got.To != "" {
				t.Fatalf("projection differs: %+v, want reason %q and metadata %+v", got, tc.reason, tc.metadata)
			}
			body, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(body), recentChangePrivateText) {
				t.Fatal("execution detail survived the projection")
			}
			if got.ResourceName != entry.ResourceName || got.ResourceType != entry.ResourceType {
				t.Fatal("resource attribution changed")
			}
			if !reflect.DeepEqual(got, redactAgentRecentChangeCommand(got)) {
				t.Fatal("projection is not idempotent")
			}
			got.Metadata["probe"] = true
			after, err := json.Marshal(tc.change)
			if err != nil || string(after) != string(original) {
				t.Fatal("projection mutated canonical evidence")
			}
		})
	}
}

func TestAgentRecentChangesNonExecutionEvidenceIsUnchanged(t *testing.T) {
	entry := AgentRecentChange{ResourceChange: unified.ResourceChange{Kind: unified.ChangeRestart, Reason: "uptime reset", From: "3600", To: "10", Metadata: map[string]any{"previousUptime": 3600}}, ResourceName: "db-01"}
	if got := redactAgentRecentChangeCommand(entry); !reflect.DeepEqual(got, entry) {
		t.Fatal("non-execution evidence changed")
	}
}

// Exercise the registered route, real token authentication, scope enforcement,
// persistent store and JSON consumer together. Concurrent readers must neither
// disclose details to monitoring nor redact the stored executor's evidence.
func TestAgentRecentChangesHTTPPreservesAuthorityAndCanonicalEvidence(t *testing.T) {
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	const readToken = "history-read-fixture-token-123.12345678"
	const execToken = "history-exec-fixture-token-123.12345678"
	const otherToken = "history-other-fixture-token-123.12345678"
	cfg := newTestConfigWithTokens(t,
		newTokenRecord(t, readToken, []string{config.ScopeMonitoringRead}, nil),
		newTokenRecord(t, execToken, []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil),
		newTokenRecord(t, otherToken, []string{config.ScopeSettingsRead}, nil),
	)
	cfg.AuthUser = "admin"
	cfg.AuthPass = "fixture-nonempty-hash"
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	t.Cleanup(func() { _ = router.resourceHandlers.CloseStores() })
	store, err := router.resourceHandlers.getStore("default")
	if err != nil {
		t.Fatal(err)
	}
	changes := []unified.ResourceChange{
		*unified.BuildCommandExecutionChange("vm:101", "alert-1", "operator", recentChangePrivateText, false, recentChangePrivateText, map[string]any{"title": recentChangePrivateText, unified.MetadataOutcome: map[string]any{"output": recentChangePrivateText}}),
		*unified.BuildRunbookExecutionChange("vm:101", "alert-1", "operator", "rb-1", recentChangePrivateText, "partial", true, recentChangePrivateText, nil),
	}
	for _, change := range changes {
		if err := store.RecordChange(change); err != nil {
			t.Fatal(err)
		}
	}
	before, err := store.GetRecentChangesFiltered("", time.Time{}, 50, unified.ResourceChangeFilters{})
	if err != nil {
		t.Fatal(err)
	}
	original, err := json.Marshal(before)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(router.Handler())
	defer server.Close()
	read := func(token string, want int, private bool) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, server.URL+agentcapabilities.RecentChangesCapabilityPath, nil)
		if err != nil {
			t.Error(err)
			return
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
			return
		}
		if resp.StatusCode != want {
			t.Errorf("status %d, want %d: %s", resp.StatusCode, want, body)
			return
		}
		if want != http.StatusOK {
			return
		}
		var got AgentRecentChanges
		if err := json.Unmarshal(body, &got); err != nil {
			t.Error(err)
			return
		}
		if got.Count != 2 || len(got.Changes) != 2 || got.Truncated {
			t.Errorf("lost history envelope: %+v", got)
		}
		if strings.Contains(string(body), recentChangePrivateText) != private {
			t.Error("execution detail visibility differs from token authority")
		}
		for _, change := range got.Changes {
			if change.CommandRedacted == private {
				t.Error("redaction marker differs from token authority")
			}
			if change.Metadata[unified.MetadataAlertIdentifier] != "alert-1" {
				t.Error("lost safe identity")
			}
		}
	}
	read("", http.StatusUnauthorized, false)
	read(otherToken, http.StatusForbidden, false)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); read(readToken, http.StatusOK, false) }()
		go func() { defer wg.Done(); read(execToken, http.StatusOK, true) }()
	}
	wg.Wait()
	after, err := store.GetRecentChangesFiltered("", time.Time{}, 50, unified.ResourceChangeFilters{})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := json.Marshal(after)
	if err != nil || string(stored) != string(original) {
		t.Fatal("HTTP reads rewrote canonical history")
	}
}
