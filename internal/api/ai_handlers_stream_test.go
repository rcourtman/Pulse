package api

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"net/http"
	"net/http/httptest"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type stubLicenseChecker struct {
	allow bool
}

func (s stubLicenseChecker) HasFeature(feature string) bool {
	return s.allow
}

func (s stubLicenseChecker) GetLicenseStateString() (string, bool) {
	if s.allow {
		return "active", true
	}
	return "expired", false
}

func withEnv(t *testing.T, key, value string, fn func()) {
	t.Helper()
	old := os.Getenv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	defer func() {
		_ = os.Setenv(key, old)
	}()
	fn()
}

func newTestAISettingsHandlerWithService(t *testing.T) *AISettingsHandler {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := NewAISettingsHandler(nil, nil, nil)
	handler.legacyConfig = cfg
	handler.legacyPersistence = persistence
	handler.legacyAIService = ai.NewService(persistence, nil)
	return handler
}

func TestHandleExecuteStream_LicenseRequired(t *testing.T) {
	withEnv(t, "PULSE_MOCK_MODE", "true", func() {
		handler := newTestAISettingsHandlerWithService(t)
		handler.legacyAIService.SetLicenseChecker(stubLicenseChecker{allow: false})

		body := `{"prompt":"hi","use_case":"autofix"}`
		req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleExecuteStream(rec, req)

		if rec.Code != http.StatusPaymentRequired {
			t.Fatalf("expected payment required, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "license_required") {
			t.Fatalf("expected license error body")
		}
	})
}

func TestHandleExecuteStream_PromptRequired(t *testing.T) {
	withEnv(t, "PULSE_MOCK_MODE", "true", func() {
		handler := newTestAISettingsHandlerWithService(t)

		body := `{"prompt":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleExecuteStream(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected bad request, got %d", rec.Code)
		}
	})
}

func TestHandleExecuteStream_Success(t *testing.T) {
	withEnv(t, "PULSE_MOCK_MODE", "true", func() {
		handler := newTestAISettingsHandlerWithService(t)

		body := `{"prompt":"hello"}`
		req := httptest.NewRequest(http.MethodPost, "/api/ai/execute/stream", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleExecuteStream(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected OK, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatalf("expected SSE content type")
		}
	})
}

func TestHandleExportGuestKnowledge(t *testing.T) {
	handler := newTestAISettingsHandlerWithService(t)
	if err := handler.legacyAIService.SaveGuestNote("guest-1", "VM 1", "vm", "ops", "Note", "Content"); err != nil {
		t.Fatalf("save note error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/export?guest_id=guest-1", nil)
	rec := httptest.NewRecorder()
	handler.HandleExportGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "guest-1") {
		t.Fatalf("expected content disposition with guest id")
	}

	var exported knowledge.GuestKnowledge
	if err := json.NewDecoder(rec.Body).Decode(&exported); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(exported.Notes) == 0 {
		t.Fatalf("expected exported notes")
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/export", nil)
	missingRec := httptest.NewRecorder()
	handler.HandleExportGuestKnowledge(missingRec, missingReq)
	if missingRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for missing guest id")
	}
}

func TestHandleImportGuestKnowledge(t *testing.T) {
	handler := newTestAISettingsHandlerWithService(t)
	if err := handler.legacyAIService.SaveGuestNote("guest-1", "VM 1", "vm", "ops", "Old", "Old content"); err != nil {
		t.Fatalf("save note error: %v", err)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/import", strings.NewReader("{bad"))
	invalidRec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid json")
	}

	methodReq := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/import", nil)
	methodRec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(methodRec, methodReq)
	if methodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected method not allowed")
	}

	emptyReq := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/import", strings.NewReader(`{"guest_id":""}`))
	emptyRec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(emptyRec, emptyReq)
	if emptyRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for missing guest id")
	}

	noNotesReq := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/import", strings.NewReader(`{"guest_id":"guest-1","notes":[]}`))
	noNotesRec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(noNotesRec, noNotesReq)
	if noNotesRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for empty notes")
	}

	payload := map[string]interface{}{
		"guest_id":   "guest-1",
		"guest_name": "VM 1",
		"guest_type": "vm",
		"merge":      false,
		"notes": []map[string]string{
			{"category": "ops", "title": "New", "content": "New content"},
			{"category": "", "title": "Skip", "content": "Bad"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "\"imported\":1") {
		t.Fatalf("expected import count")
	}

	knowledge, err := handler.legacyAIService.GetGuestKnowledge("guest-1")
	if err != nil {
		t.Fatalf("get knowledge error: %v", err)
	}
	if len(knowledge.Notes) != 1 {
		t.Fatalf("expected only imported notes, got %d", len(knowledge.Notes))
	}
}
