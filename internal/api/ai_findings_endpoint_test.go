package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// --- HandleGetInvestigation ---

func TestHandleGetInvestigation_MethodNotAllowed(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/ai/findings/f-1/investigation", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetInvestigation(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rec.Code)
		}
	}
}

func TestHandleGetInvestigation_EmptyFindingID(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings//investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty finding ID, got %d", rec.Code)
	}
	var resp APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if resp.Code != "missing_id" {
		t.Fatalf("expected code missing_id, got %q", resp.Code)
	}
}

func TestHandleGetInvestigation_FindingIDTooLong(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	longID := strings.Repeat("a", maxFindingIDLength+1)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/"+longID+"/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized finding ID, got %d", rec.Code)
	}
	var resp APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if resp.Code != "invalid_id" {
		t.Fatalf("expected code invalid_id, got %q", resp.Code)
	}
}

func TestHandleGetInvestigation_NoPatrolService(t *testing.T) {
	// newTestAISettingsHandlerLite creates an AI service without state provider,
	// so patrol won't be initialized.
	handler := newTestAISettingsHandlerLite()
	svc := handler.GetAIService(context.Background())
	if svc.GetPatrolService() != nil {
		t.Skip("patrol service unexpectedly initialized")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for nil patrol, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetInvestigation_NoAIService(t *testing.T) {
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetInvestigation(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing AI service, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if resp.Code != "not_initialized" {
		t.Fatalf("expected code not_initialized, got %q", resp.Code)
	}
}

func TestHandleGetInvestigation_NoOrchestrator(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	// Don't set orchestrator — it should be nil

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for nil orchestrator, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetInvestigation_NotFound(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	// Orchestrator with no matching session
	orchestrator := &stubInvestigationOrchestrator{session: nil}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/nonexistent/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing investigation, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if resp.Code != "not_found" {
		t.Fatalf("expected code not_found, got %q", resp.Code)
	}
}

func TestHandleGetInvestigation_FindingIDExtractionFromPath(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}

	// Normal hex finding ID — the real production format
	session := &ai.InvestigationSession{
		ID:        "inv-1",
		FindingID: "abcdef0123456789",
		SessionID: "session-1",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/abcdef0123456789/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid hex finding ID, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp ai.InvestigationSession
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.FindingID != "abcdef0123456789" {
		t.Fatalf("expected findingID abcdef0123456789, got %q", resp.FindingID)
	}
}

func TestHandleGetInvestigation_MalformedPathNoID(t *testing.T) {
	// When the path is /api/ai/findings/investigation (no actual finding ID segment),
	// TrimPrefix+TrimSuffix yields "investigation" as the ID, which is not empty
	// and passes validation. The orchestrator returns nil → 404.
	// This documents current behavior: non-crashable, returns 404.
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	orchestrator := &stubInvestigationOrchestrator{session: nil}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/investigation", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigation(rec, req)
	// Should get 404 since no finding with ID "investigation" exists
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for malformed path without real ID, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- HandleGetInvestigationMessages ---

func TestHandleGetInvestigationMessages_MethodNotAllowed(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/ai/findings/f-1/investigation/messages", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetInvestigationMessages(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected 405, got %d", method, rec.Code)
		}
	}
}

func TestHandleGetInvestigationMessages_EmptyFindingID(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings//investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty finding ID, got %d", rec.Code)
	}
}

func TestHandleGetInvestigationMessages_FindingIDTooLong(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	longID := strings.Repeat("x", maxFindingIDLength+1)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/"+longID+"/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for oversized finding ID, got %d", rec.Code)
	}
}

func TestHandleGetInvestigationMessages_NoPatrolService(t *testing.T) {
	handler := newTestAISettingsHandlerLite()
	svc := handler.GetAIService(context.Background())
	if svc.GetPatrolService() != nil {
		t.Skip("patrol service unexpectedly initialized")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestHandleGetInvestigationMessages_NoAIService(t *testing.T) {
	handler := &AISettingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation/messages", nil)
	rec := httptest.NewRecorder()

	handler.HandleGetInvestigationMessages(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for missing AI service, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp APIError
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if resp.Code != "not_initialized" {
		t.Fatalf("expected code not_initialized, got %q", resp.Code)
	}
}

func TestHandleGetInvestigationMessages_NoOrchestrator(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetInvestigationMessages_InvestigationNotFound(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	orchestrator := &stubInvestigationOrchestrator{session: nil}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/nonexistent/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetInvestigationMessages_NoChatService(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	// Don't set chat service - leave it nil
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	session := &ai.InvestigationSession{
		ID:        "inv-1",
		FindingID: "f-1",
		SessionID: "session-1",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for nil chat service, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleGetInvestigationMessages_EmptyMessageList(t *testing.T) {
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	svc := handler.GetAIService(context.Background())
	svc.SetStateProvider(&MockStateProvider{})
	svc.SetChatService(&stubChatService{messages: []ai.ChatMessage{}})
	patrol := svc.GetPatrolService()
	if patrol == nil {
		t.Fatalf("expected patrol service")
	}
	session := &ai.InvestigationSession{
		ID:        "inv-1",
		FindingID: "f-1",
		SessionID: "session-1",
		Status:    "completed",
		StartedAt: time.Now(),
	}
	orchestrator := &stubInvestigationOrchestrator{session: session}
	patrol.SetInvestigationOrchestrator(orchestrator)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/investigation/messages", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetInvestigationMessages(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["investigation_id"] != "inv-1" {
		t.Fatalf("expected investigation_id inv-1, got %v", resp["investigation_id"])
	}
	msgs := resp["messages"].([]interface{})
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages, got %d", len(msgs))
	}
}

// --- Router-level dispatch ---

func TestFindingsRouterDispatch_UnknownSubpath(t *testing.T) {
	rawToken := "ai-findings-dispatch-test.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/ai/findings/f-1/unknown", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown sub-path, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFindingsRouterDispatch_RequiresAIExecuteScope(t *testing.T) {
	// Token with only ai:chat scope — should be denied ai:execute endpoints
	rawToken := "ai-findings-scope-test.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIChat}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	paths := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/ai/findings/f-1/investigation"},
		{http.MethodGet, "/api/ai/findings/f-1/investigation/messages"},
		{http.MethodPost, "/api/ai/findings/f-1/reinvestigate"},
		{http.MethodPost, "/api/ai/findings/f-1/reapprove"},
	}

	for _, tc := range paths {
		var body *strings.Reader
		if tc.method == http.MethodPost {
			body = strings.NewReader(`{}`)
		}
		var req *http.Request
		if body != nil {
			req = httptest.NewRequest(tc.method, tc.path, body)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		}
		req.Header.Set("X-API-Token", rawToken)
		rec := httptest.NewRecorder()
		router.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected 403 for wrong scope, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestFindingsRouterDispatch_CommunityReinvestigateReturns402(t *testing.T) {
	rawToken := "ai-findings-community-reinv.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/f-1/reinvestigate", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for Community reinvestigate, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFindingsRouterDispatch_CommunityReapproveReturns402(t *testing.T) {
	rawToken := "ai-findings-community-reapp.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeAIExecute}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/ai/findings/f-1/reapprove", strings.NewReader(`{}`))
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 for Community reapprove, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- maxFindingIDLength constant ---

func TestMaxFindingIDLength_IsReasonable(t *testing.T) {
	// Verify the constant is set to a reasonable value.
	// Real finding IDs are 16 hex chars; the limit should be generous but bounded.
	if maxFindingIDLength < 16 {
		t.Fatalf("maxFindingIDLength=%d is too small for real finding IDs (16 chars)", maxFindingIDLength)
	}
	if maxFindingIDLength > 1024 {
		t.Fatalf("maxFindingIDLength=%d is unreasonably large", maxFindingIDLength)
	}
}
