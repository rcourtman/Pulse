package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type mcpDisclosureFlusher struct {
	http.ResponseWriter
	ready chan struct{}
	once  sync.Once
}

func (w *mcpDisclosureFlusher) Flush() {
	w.ResponseWriter.(http.Flusher).Flush()
	w.once.Do(func() { close(w.ready) })
}

type mcpDisclosureWriter struct {
	bytes.Buffer
	received chan struct{}
	once     sync.Once
}

func (w *mcpDisclosureWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	if bytes.Contains(w.Bytes(), []byte(`"method":"notifications/action.completed"`)) {
		w.once.Do(func() { close(w.received) })
	}
	return n, err
}

// Exercise both consuming MCP paths against Pulse's real authenticated HTTP
// handlers. No adapter-side redactor may substitute for server enforcement:
// text and structured tools/call output and live JSON-RPC params must agree.
func TestServer_ActionDisclosureThroughRealAgentHandlers(t *testing.T) {
	const privateDetail = "mcp-synthetic-command-output"
	for _, restricted := range []bool{true, false} {
		name := "action_capable"
		if restricted {
			name = "monitoring_read"
		}
		t.Run(name, func(t *testing.T) {
			scopes := []string{config.ScopeMonitoringRead}
			if !restricted {
				scopes = append(scopes, config.ScopeAIExecute)
			}
			token, err := config.NewAPITokenRecord("fixture-mcp-token", "fixture", scopes)
			if err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{DataPath: t.TempDir(), APITokens: []config.APITokenRecord{*token}}
			resources := api.NewResourceHandlers(cfg)
			defer resources.CloseStores()
			resources.SetStateProvider(mcpFleetSnapshot{resources: []unified.Resource{{ID: "vm:101", Type: "vm", Name: "fixture-vm"}}})
			handler := api.NewAgentContextHandler(resources)
			store, err := resources.Store("default")
			if err != nil {
				t.Fatal(err)
			}
			now := time.Now().UTC()
			truth := unified.ActionResultV2{
				Version:      unified.ActionResultV2Version,
				Execution:    unified.ActionExecutionTruth{Status: unified.ActionExecutionFailed, ReasonCode: "execution_failed", Summary: privateDetail},
				Verification: unified.ActionVerificationTruth{Status: unified.ActionVerificationNotAttempted, EvidenceClass: unified.ActionEvidenceNone},
				Compensation: unified.ActionCompensationTruth{Support: unified.ActionCompensationUnavailable, Status: unified.ActionCompensationNotAvailable},
			}
			record := unified.ActionAuditRecord{
				ID: "mcp-action", CreatedAt: now, UpdatedAt: now, State: unified.ActionStateFailed,
				Request: unified.ActionRequest{ResourceID: "vm:101", CapabilityName: "restart", Params: map[string]any{"command": "printf " + privateDetail}},
				Result:  &unified.ExecutionResult{ErrorMessage: "plan_drift: " + privateDetail, ActionResultV2: &truth},
			}
			if err := store.RecordActionAudit(record); err != nil {
				t.Fatal(err)
			}
			b := api.NewAgentEventBroadcaster()
			ready := make(chan struct{})
			mux := http.NewServeMux()
			mux.HandleFunc(agentcapabilities.AgentEventsPath, api.RequireAuth(cfg, api.RequireScope(config.ScopeMonitoringRead, func(w http.ResponseWriter, r *http.Request) {
				b.HandleAgentEvents(&mcpDisclosureFlusher{ResponseWriter: w, ready: ready}, r)
			})))
			mux.HandleFunc("/api/agent/resource-context/", api.RequireAuth(cfg, api.RequireScope(config.ScopeMonitoringRead, handler.HandleResourceContext)))
			backend := httptest.NewServer(mux)
			defer backend.Close()
			manifest := agentcapabilities.CanonicalManifest()
			server := &mcpServer{baseURL: backend.URL, token: "fixture-mcp-token", http: backend.Client(), manifest: &manifest}
			var stdout bytes.Buffer
			server.serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_resource_context","arguments":{"resourceId":"vm:101"}}}`+"\n"), &stdout)
			var reply struct {
				Result agentcapabilities.MCPToolResult
				Error  any
			}
			if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
				t.Fatal(err)
			}
			if reply.Error != nil || reply.Result.IsError || len(reply.Result.Content) != 1 {
				t.Fatalf("MCP tools/call failed: %s", stdout.String())
			}
			structured, err := json.Marshal(reply.Result.StructuredContent)
			if err != nil {
				t.Fatal(err)
			}
			check := func(body []byte) {
				t.Helper()
				if restricted {
					if bytes.Contains(body, []byte(privateDetail)) || bytes.Contains(body, []byte(`"actionResultV2"`)) || !bytes.Contains(body, []byte(`"detailsRedacted":true`)) || !bytes.Contains(body, []byte(`"errorMessage":"plan_drift:"`)) {
						t.Errorf("MCP disclosed restricted action detail or lost safe outcome: %s", body)
					}
				} else if !bytes.Contains(body, []byte(privateDetail)) || !bytes.Contains(body, []byte(`"actionResultV2"`)) || bytes.Contains(body, []byte(`"actionResultSummary"`)) {
					t.Errorf("authorized MCP detail changed: %s", body)
				}
			}
			check([]byte(reply.Result.Content[0].Text))
			check(structured)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			out := &mcpDisclosureWriter{received: make(chan struct{})}
			server.out = out
			done := make(chan error, 1)
			go func() { done <- server.streamSSEOnce(ctx, agentcapabilities.AgentEventsPath) }()
			select {
			case <-ready:
			case <-ctx.Done():
				t.Fatal("MCP stream never subscribed")
			}
			b.PublishActionCompletedRecord(record)
			select {
			case <-out.received:
			case <-ctx.Done():
				t.Fatal("MCP notification never arrived")
			}
			cancel()
			<-done
			var notification agentcapabilities.JSONRPCRequest
			if err := json.Unmarshal(out.Bytes(), &notification); err != nil {
				t.Fatal(err)
			}
			if notification.Method != "notifications/action.completed" {
				t.Fatalf("wrong MCP event: %s", out.String())
			}
			check(notification.Params)
		})
	}
}
