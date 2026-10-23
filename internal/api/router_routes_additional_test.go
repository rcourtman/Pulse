package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/stretchr/testify/mock"
)

func TestRouteAISessions_NoSessionID(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/sessions/", nil)
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRouteAISessions_UnknownSubresource(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/sessions/session-1/unknown", nil)
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

// The web chat window that used these session sub-resources is retired, and
// Pulse Mobile never called them; they fall to the unknown-subresource 404.
func TestRouteAISessions_RetiredWebChatSubresourcesAreNotFound(t *testing.T) {
	for _, sub := range []string{"summarize", "fork", "undo", "redo", "steer", "diff", "revert", "unrevert"} {
		router := &Router{}
		req := httptest.NewRequest(http.MethodPost, "/api/ai/sessions/session-1/"+sub, nil)
		rec := httptest.NewRecorder()

		router.routeAISessions(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("POST %s: expected status %d, got %d", sub, http.StatusNotFound, rec.Code)
		}
	}
}

func TestRouteAISessions_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/sessions/session-1", nil)
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestRouteAISessions_Messages(t *testing.T) {
	mockSvc := &MockAIService{}
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("GetMessages", mock.Anything, "session-1").Return([]chat.Message{}, nil)

	handler := &AIHandler{}
	setUnexportedField(t, handler, "defaultService", mockSvc)

	router := &Router{aiHandler: handler}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/sessions/session-1/messages", nil)
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var payload []chat.Message
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	mockSvc.AssertExpectations(t)
}

func TestRouteAISessions_DeleteSession(t *testing.T) {
	mockSvc := &MockAIService{}
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("DeleteSession", mock.Anything, "session-1").Return(nil)

	handler := &AIHandler{}
	setUnexportedField(t, handler, "defaultService", mockSvc)

	router := &Router{aiHandler: handler}
	req := httptest.NewRequest(http.MethodDelete, "/api/ai/sessions/session-1", nil)
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, rec.Code)
	}
	mockSvc.AssertExpectations(t)
}

func TestRouteAISessions_RenameSession(t *testing.T) {
	mockSvc := &MockAIService{}
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("RenameSession", mock.Anything, "session-1", "Renamed session").Return(&chat.Session{
		ID:    "session-1",
		Title: "Renamed session",
	}, nil)

	handler := &AIHandler{}
	setUnexportedField(t, handler, "defaultService", mockSvc)

	router := &Router{aiHandler: handler}
	req := httptest.NewRequest(http.MethodPatch, "/api/ai/sessions/session-1", strings.NewReader(`{"title":"Renamed session"}`))
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	mockSvc.AssertExpectations(t)
}

func TestRouteAISessions_Abort(t *testing.T) {
	mockSvc := &MockAIService{}
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("AbortSession", mock.Anything, "session-1").Return(nil)

	handler := &AIHandler{}
	setUnexportedField(t, handler, "defaultService", mockSvc)

	router := &Router{aiHandler: handler}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/sessions/session-1/abort", nil)
	rec := httptest.NewRecorder()

	router.routeAISessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
	mockSvc.AssertExpectations(t)
}

func TestRouteApprovals_MissingID(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/api/ai/approvals/", nil)
	rec := httptest.NewRecorder()

	router.routeApprovals(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestRouteApprovals_UnknownAction(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/approvals/abc/unknown", nil)
	rec := httptest.NewRecorder()

	router.routeApprovals(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestRouteApprovals_MethodNotAllowed(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/ai/approvals/abc", nil)
	rec := httptest.NewRecorder()

	router.routeApprovals(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleAgentWebSocket_Unavailable(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodGet, "/ws/agent", nil)
	rec := httptest.NewRecorder()

	router.handleAgentWebSocket(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleVerifyTemperatureSSH_ServiceUnavailable(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh", nil)
	rec := httptest.NewRecorder()

	router.handleVerifyTemperatureSSH(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleVerifyTemperatureSSH_Unauthorized(t *testing.T) {
	hash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	router := &Router{
		config:         &config.Config{AuthUser: "admin", AuthPass: hash},
		configHandlers: &ConfigHandlers{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/system/verify-temperature-ssh", nil)
	rec := httptest.NewRecorder()

	router.handleVerifyTemperatureSSH(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestHandleSSHConfig_ServiceUnavailable(t *testing.T) {
	router := &Router{}
	req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config", nil)
	rec := httptest.NewRecorder()

	router.handleSSHConfig(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleSSHConfig_Unauthorized(t *testing.T) {
	hash, err := auth.HashPassword("secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	router := &Router{
		config:                &config.Config{AuthUser: "admin", AuthPass: hash},
		systemSettingsHandler: &SystemSettingsHandler{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/system/ssh-config", nil)
	rec := httptest.NewRecorder()

	router.handleSSHConfig(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}
}

func TestRouterAIChatControl(t *testing.T) {
	t.Run("nil handler", func(t *testing.T) {
		router := &Router{}
		router.StopAIChat(context.Background())
		router.RestartAIChat(context.Background())
	})

	t.Run("calls through to ai handler", func(t *testing.T) {
		mockSvc := &MockAIService{}
		mockSvc.On("Stop", mock.Anything).Return(nil)
		mockSvc.On("Restart", mock.Anything, mock.Anything).Return(nil)
		mockSvc.On("IsRunning").Return(true)

		handler := &AIHandler{}
		setUnexportedField(t, handler, "defaultService", mockSvc)

		router := &Router{aiHandler: handler}
		router.StopAIChat(context.Background())
		router.RestartAIChat(context.Background())

		mockSvc.AssertExpectations(t)
	})
}
