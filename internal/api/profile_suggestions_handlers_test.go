package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/stretchr/testify/mock"
)

func TestProfileSuggestionHandler_MethodNotAllowed(t *testing.T) {
	handler := NewProfileSuggestionHandler(config.NewConfigPersistence(t.TempDir()), &AIHandler{})
	req := httptest.NewRequest(http.MethodGet, "/api/admin/profiles/suggestions", nil)
	rr := httptest.NewRecorder()

	handler.HandleSuggestProfile(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestProfileSuggestionHandler_ServiceUnavailable(t *testing.T) {
	mockSvc := new(MockAIService)
	mockSvc.On("IsRunning").Return(false)
	aiHandler := &AIHandler{legacyService: mockSvc}

	handler := NewProfileSuggestionHandler(config.NewConfigPersistence(t.TempDir()), aiHandler)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/profiles/suggestions", bytes.NewReader([]byte(`{"prompt":"test"}`)))
	rr := httptest.NewRecorder()

	handler.HandleSuggestProfile(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rr.Code)
	}
}

func TestProfileSuggestionHandler_InvalidRequest(t *testing.T) {
	mockSvc := new(MockAIService)
	mockSvc.On("IsRunning").Return(true)
	aiHandler := &AIHandler{legacyService: mockSvc}
	handler := NewProfileSuggestionHandler(config.NewConfigPersistence(t.TempDir()), aiHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profiles/suggestions", bytes.NewReader([]byte("{bad")))
	rr := httptest.NewRecorder()
	handler.HandleSuggestProfile(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/profiles/suggestions", bytes.NewReader([]byte(`{"prompt":"   "}`)))
	rr = httptest.NewRecorder()
	handler.HandleSuggestProfile(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestProfileSuggestionHandler_SuccessAndParseFailure(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveAgentProfiles([]models.AgentProfile{{Name: "Existing"}}); err != nil {
		t.Fatalf("save profiles: %v", err)
	}

	mockSvc := new(MockAIService)
	mockSvc.On("IsRunning").Return(true)
	mockSvc.On("Execute", mock.Anything, mock.Anything).Return(map[string]interface{}{
		"content": `{"name":"Suggested","description":"desc","config":{"interval":"30s"},"rationale":["reason"]}`,
	}, nil).Once()

	aiHandler := &AIHandler{legacyService: mockSvc}
	handler := NewProfileSuggestionHandler(persistence, aiHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/profiles/suggestions", bytes.NewReader([]byte(`{"prompt":"build a profile"}`)))
	rr := httptest.NewRecorder()
	handler.HandleSuggestProfile(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var resp ProfileSuggestion
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Name != "Suggested" {
		t.Fatalf("unexpected name: %q", resp.Name)
	}
	if resp.Config["interval"] == "" {
		t.Fatalf("expected config in response")
	}

	mockSvc.On("Execute", mock.Anything, mock.Anything).Return(map[string]interface{}{
		"content": "not json",
	}, nil).Once()

	req = httptest.NewRequest(http.MethodPost, "/api/admin/profiles/suggestions", bytes.NewReader([]byte(`{"prompt":"bad response"}`)))
	rr = httptest.NewRecorder()
	handler.HandleSuggestProfile(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rr.Code)
	}
}

// Ensure MockAIService implements the interface methods used by the handler.
var _ interface {
	IsRunning() bool
	Execute(ctx context.Context, req chat.ExecuteRequest) (map[string]interface{}, error)
} = (*MockAIService)(nil)
