package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// Patrol and its investigations run host commands with no caller bound. What
// those commands returned is served only to callers who could have had the
// Assistant run them; everyone else keeps the run, stream and transcript
// without the tool output.

const patrolHostOutputMarker = "root-only-log-line"

func patrolRunWithHostOutput() ai.PatrolRunRecord {
	return ai.PatrolRunRecord{
		ID:              "run-1",
		StartedAt:       time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC),
		CompletedAt:     time.Date(2026, 3, 1, 10, 1, 0, 0, time.UTC),
		Type:            "patrol",
		FindingsSummary: "ok",
		FindingIDs:      []string{},
		Status:          "healthy",
		ToolCalls: []ai.ToolCallRecord{{
			ID:       "call-1",
			ToolName: "pulse_read",
			Input:    `{"action":"file","path":"/var/log/audit.log","target_host":"app-host"}`,
			Output:   patrolHostOutputMarker,
			Success:  true,
		}},
		ToolCallCount: 1,
	}
}

func TestPatrolRunToolCallsNeedHostCommandAuthority(t *testing.T) {
	InitSessionStore(t.TempDir())
	cfg := &config.Config{AuthUser: "admin"}

	handler := createTestAIHandler(t)
	patrol := &ai.PatrolService{}
	store := ai.NewPatrolRunHistoryStore(10)
	store.Add(patrolRunWithHostOutput())
	setUnexportedField(t, patrol, "runHistoryStore", store)
	setUnexportedField(t, handler.defaultAIService, "patrolService", patrol)

	routes := map[string]http.HandlerFunc{
		"/api/ai/patrol/runs?include=tool_calls":       handler.HandleGetPatrolRunHistory,
		"/api/ai/patrol/runs/run-1?include=tool_calls": handler.HandleGetPatrolRun,
	}
	for _, caller := range assistantHostCommandCallers() {
		for target, route := range routes {
			t.Run(caller.name+" "+target, func(t *testing.T) {
				req := caller.prepare(t, httptest.NewRequest(http.MethodGet, target, nil))
				rec := httptest.NewRecorder()
				withAssistantActionAuthority(cfg, caller.authorizer, route)(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
				}
				body := rec.Body.String()
				if !strings.Contains(body, `"tool_call_count":1`) {
					t.Fatalf("run summary missing from response: %s", body)
				}
				if got := strings.Contains(body, patrolHostOutputMarker); got != caller.want {
					t.Fatalf("tool output served = %v, want %v: %s", got, caller.want, body)
				}
				if got := strings.Contains(body, `"tool_calls"`); got != caller.want {
					t.Fatalf("tool_calls served = %v, want %v: %s", got, caller.want, body)
				}
			})
		}
	}
}

// The registered run routes bind the authority themselves: a signed-in viewer
// gets the run without its tool calls, an administrator gets them.
func TestPatrolRunRoutesWithholdToolCallsFromViewerSession(t *testing.T) {
	previousAuthorizer := auth.GetAuthorizer()
	auth.SetAuthorizer(&auth.DefaultAuthorizer{})
	t.Cleanup(func() { auth.SetAuthorizer(previousAuthorizer) })

	router := newConfigTransferTestRouter(t, false, enabledConfigTransferSSO(config.SSOProviderTypeOIDC))
	manager := installTestRBACManager(t)
	const ssoAdmin = "sso:patrol-runs-admin@example.invalid"
	if err := manager.UpdateUserRoles(ssoAdmin, []string{auth.RoleAdmin}); err != nil {
		t.Fatalf("assign SSO administrator role: %v", err)
	}

	patrol := &ai.PatrolService{}
	store := ai.NewPatrolRunHistoryStore(10)
	store.Add(patrolRunWithHostOutput())
	setUnexportedField(t, patrol, "runHistoryStore", store)
	setUnexportedField(t, router.aiSettingsHandler.defaultAIService, "patrolService", patrol)

	for _, user := range []struct {
		name      string
		principal string
		want      bool
	}{
		{name: "viewer", principal: "sso:patrol-runs-viewer@example.invalid", want: false},
		{name: "administrator", principal: ssoAdmin, want: true},
	} {
		sessionToken := "patrol-runs-route-" + user.name
		GetSessionStore().CreateSession(sessionToken, time.Hour, "browser", "127.0.0.1", user.principal)
		for _, target := range []string{
			"/api/ai/patrol/runs?include=tool_calls",
			"/api/ai/patrol/runs/run-1?include=tool_calls",
		} {
			t.Run(user.name+" "+target, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, target, nil)
				req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: sessionToken})
				rec := httptest.NewRecorder()
				router.Handler().ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
				}
				body := rec.Body.String()
				if !strings.Contains(body, `"tool_call_count":1`) {
					t.Fatalf("run summary missing from response: %s", body)
				}
				if got := strings.Contains(body, patrolHostOutputMarker); got != user.want {
					t.Fatalf("tool output served = %v, want %v: %s", got, user.want, body)
				}
			})
		}
	}
}

func TestInvestigationMessagesToolOutputNeedsHostCommandAuthority(t *testing.T) {
	InitSessionStore(t.TempDir())
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp, AuthUser: "admin"}
	handler := newTestAISettingsHandler(cfg, config.NewConfigPersistence(tmp), nil)

	succeeded := true
	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	// The stub hands out its own slice, so each caller reads a fresh transcript.
	transcript := func() []ai.ChatMessage {
		return []ai.ChatMessage{
			{ID: "msg-1", Role: "assistant", Content: "reading the log", ToolCalls: []ai.ChatToolCall{{
				ID:      "call-1",
				Name:    "pulse_read",
				Input:   map[string]interface{}{"action": "file", "path": "/var/log/audit.log"},
				Output:  patrolHostOutputMarker,
				Success: &succeeded,
			}}},
			{ID: "msg-2", Role: "user", ToolResult: &ai.ChatToolResult{ToolUseID: "call-1", Content: patrolHostOutputMarker}},
		}
	}
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	patrol.SetInvestigationOrchestrator(&stubInvestigationOrchestrator{session: &ai.InvestigationSession{
		ID:        "inv-1",
		FindingID: "finding-1",
		SessionID: "session-1",
		Status:    "completed",
		StartedAt: time.Now(),
	}})

	for _, caller := range assistantHostCommandCallers() {
		t.Run(caller.name, func(t *testing.T) {
			svc.SetChatService(&stubChatService{messages: transcript()})
			req := caller.prepare(t, newLoopbackRequest(http.MethodGet, "/api/ai/findings/finding-1/investigation/messages", nil))
			rec := httptest.NewRecorder()
			withAssistantActionAuthority(cfg, caller.authorizer, handler.HandleGetInvestigationMessages)(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, kept := range []string{`"content":"reading the log"`, `"name":"pulse_read"`, `"tool_use_id":"call-1"`} {
				if !strings.Contains(body, kept) {
					t.Fatalf("transcript lost %s: %s", kept, body)
				}
			}
			if got := strings.Contains(body, patrolHostOutputMarker); got != caller.want {
				t.Fatalf("tool output served = %v, want %v: %s", got, caller.want, body)
			}
		})
	}
}

func TestPatrolStreamToolOutputNeedsHostCommandAuthority(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		tmp := t.TempDir()
		cfg := &config.Config{DataPath: tmp}
		handler := newTestAISettingsHandler(cfg, config.NewConfigPersistence(tmp), nil)
		handler.defaultAIService.SetStateProvider(&stubStateProvider{})
		patrol := handler.defaultAIService.GetPatrolService()
		if patrol == nil {
			t.Fatalf("expected patrol service to be initialized")
		}

		// Buffer a phase event and a tool_end event, then resume after the
		// first so the handler replays the tool_end to this subscriber.
		patrol.DebugResetStreamForRun("run-1")
		patrol.DebugSetStreamPhase("analyzing")
		success := true
		patrol.DebugBroadcastStreamEvent(ai.PatrolStreamEvent{
			Type:        "tool_end",
			ToolID:      "call-1",
			ToolName:    "pulse_read",
			ToolInput:   `{"action":"file","path":"/var/log/audit.log"}`,
			ToolOutput:  patrolHostOutputMarker,
			ToolSuccess: &success,
		})

		srv := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.HandlePatrolStream(w, r.WithContext(chat.WithHostCommandAuthority(r.Context(), allowed)))
		}))

		reqCtx, cancel := context.WithCancel(context.Background())
		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, srv.URL+"/api/ai/patrol/stream?last_event_id=1", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}

		reader := bufio.NewReader(resp.Body)
		var toolEnd map[string]any
		deadline := time.Now().Add(2 * time.Second)
		for toolEnd == nil && time.Now().Before(deadline) {
			ctxRead, cancelRead := context.WithTimeout(context.Background(), 2*time.Second)
			msg, err := readNextSSEMessage(ctxRead, reader)
			cancelRead()
			if err != nil {
				t.Fatalf("allowed=%v: read sse: %v", allowed, err)
			}
			if msg.Data == "" {
				continue
			}
			var payload map[string]any
			if err := json.Unmarshal([]byte(msg.Data), &payload); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if payload["type"] == "tool_end" {
				toolEnd = payload
			}
		}
		_ = resp.Body.Close()
		cancel()
		srv.Close()

		if toolEnd == nil {
			t.Fatalf("allowed=%v: did not observe the tool_end event", allowed)
		}
		if toolEnd["tool_name"] != "pulse_read" || toolEnd["tool_id"] != "call-1" {
			t.Fatalf("allowed=%v: tool_end lost its identity: %#v", allowed, toolEnd)
		}
		output, served := toolEnd["tool_output"]
		if served != allowed || (allowed && output != patrolHostOutputMarker) {
			t.Fatalf("allowed=%v: tool_output = %#v (served=%v)", allowed, output, served)
		}
	}
}
