package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// The actual MCP stdio consumer must not regain execution details through
// either text content or structuredContent. Only the collected history is a
// fixture; discovery, auth, scope, store read and projection are production code.
func TestServer_RecentChangesDisclosureThroughRealHandler(t *testing.T) {
	t.Setenv("PULSE_AUTH_BYPASS", "false")
	const private = "fixture-execution-detail-not-for-monitoring"
	for _, tc := range []struct {
		name    string
		scopes  []string
		private bool
	}{
		{"monitoring", []string{config.ScopeMonitoringRead}, false},
		{"executor", []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const token = "mcp-history-fixture-token-123.12345678"
			record, err := config.NewAPITokenRecord(token, "history fixture", tc.scopes)
			if err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{DataPath: t.TempDir(), APITokens: []config.APITokenRecord{*record}, AuthUser: "admin", AuthPass: "fixture-nonempty-hash"}
			resources := api.NewResourceHandlers(cfg)
			defer resources.CloseStores()
			resources.SetStateProvider(mcpFleetSnapshot{resources: []unified.Resource{{ID: "vm:101", Type: "vm", Name: "db-01"}}})
			store, err := resources.Store("default")
			if err != nil {
				t.Fatal(err)
			}
			for _, change := range []*unified.ResourceChange{
				unified.BuildCommandExecutionChange("vm:101", "alert-1", "operator", private, true, private, map[string]any{"title": private, unified.MetadataAutomatic: map[string]any{"output": private}}),
				unified.BuildRunbookExecutionChange("vm:101", "alert-1", "operator", "rb-1", private, "resolved", false, private, nil),
			} {
				if err := store.RecordChange(*change); err != nil {
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
			handler := api.NewAgentContextHandler(resources)
			manifest := agentcapabilities.CanonicalManifest()
			protected := api.RequireAuth(cfg, api.RequireScope(config.ScopeMonitoringRead, handler.HandleRecentChanges))
			var calls atomic.Int32
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == agentcapabilities.AgentCapabilitiesPath {
					_ = json.NewEncoder(w).Encode(manifest)
					return
				}
				if r.Method != http.MethodGet || r.URL.Path != agentcapabilities.RecentChangesCapabilityPath || r.URL.RawQuery != "kind=command_executed%2Crunbook_executed&limit=10" {
					t.Errorf("unexpected history request %s %s", r.Method, r.URL.String())
					http.Error(w, "unexpected request", http.StatusNotFound)
					return
				}
				calls.Add(1)
				if r.Header.Get(agentcapabilities.AgentSurfaceHeader) != agentcapabilities.AgentSurfacePulseMCP {
					t.Error("missing MCP surface header")
				}
				protected(w, r)
			}))
			defer backend.Close()
			fetched, err := agentcapabilities.FetchManifest(context.Background(), backend.Client(), backend.URL)
			if err != nil {
				t.Fatal(err)
			}
			server := &mcpServer{baseURL: backend.URL, token: token, manifest: fetched, http: backend.Client()}
			var stdout bytes.Buffer
			server.serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_recent_changes","arguments":{"kind":"command_executed,runbook_executed","limit":10}}}`+"\n"), &stdout)
			var reply struct {
				ID     int
				Result agentcapabilities.MCPToolResult
				Error  any
			}
			decoder := json.NewDecoder(&stdout)
			if err := decoder.Decode(&reply); err != nil {
				t.Fatal(err)
			}
			if reply.ID != 1 || reply.Error != nil || reply.Result.IsError || len(reply.Result.Content) != 1 {
				t.Fatalf("tools/call: %+v", reply)
			}
			var text api.AgentRecentChanges
			if err := json.Unmarshal([]byte(reply.Result.Content[0].Text), &text); err != nil {
				t.Fatal(err)
			}
			structured, err := json.Marshal(reply.Result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			var parsed api.AgentRecentChanges
			if err := json.Unmarshal(structured, &parsed); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(text, parsed) {
				t.Fatal("MCP text and structured content differ")
			}
			if text.Count != 2 || len(text.Changes) != 2 || text.Truncated || text.Limit != 10 {
				t.Fatalf("lost history envelope: %+v", text)
			}
			for _, body := range []string{reply.Result.Content[0].Text, string(structured)} {
				if strings.Contains(body, private) != tc.private {
					t.Error("MCP execution disclosure differs from authority")
				}
			}
			for _, change := range text.Changes {
				if change.CommandRedacted == tc.private || change.Metadata[unified.MetadataAlertIdentifier] != "alert-1" || change.ResourceName != "db-01" {
					t.Errorf("lost scoped history facts: %+v", change)
				}
			}
			if calls.Load() != 1 {
				t.Errorf("history calls = %d, want one read", calls.Load())
			}
			var extra any
			if err := decoder.Decode(&extra); err != io.EOF {
				t.Fatalf("extra stdout: %v", err)
			}
			after, err := store.GetRecentChangesFiltered("", time.Time{}, 50, unified.ResourceChangeFilters{})
			if err != nil {
				t.Fatal(err)
			}
			stored, err := json.Marshal(after)
			if err != nil || string(stored) != string(original) {
				t.Fatal("MCP read rewrote canonical history")
			}
		})
	}
}
