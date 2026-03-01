package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ========================================
// HandleGetGuestKnowledge — happy path & edge cases
// ========================================

func TestHandleGetGuestKnowledge_HappyPath(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// Save a note first
	if err := handler.legacyAIService.SaveGuestNote("vm-200", "Web Server", "vm", "service", "Nginx", "Running on port 80"); err != nil {
		t.Fatalf("save note: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge?guest_id=vm-200", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var gk knowledge.GuestKnowledge
	if err := json.NewDecoder(rec.Body).Decode(&gk); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(gk.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(gk.Notes))
	}
	if gk.Notes[0].Title != "Nginx" {
		t.Fatalf("expected title 'Nginx', got %q", gk.Notes[0].Title)
	}
}

func TestHandleGetGuestKnowledge_EmptyResult(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge?guest_id=nonexistent-vm", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleGetGuestKnowledge_GuestIDTooLong(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	longID := strings.Repeat("a", 257)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge?guest_id="+longID, nil)
	rec := httptest.NewRecorder()
	handler.HandleGetGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d for oversized guest_id, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleGetGuestKnowledge_GuestIDExactlyAtLimit(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// 256 chars should be accepted (boundary test)
	exactID := strings.Repeat("b", 256)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge?guest_id="+exactID, nil)
	rec := httptest.NewRecorder()
	handler.HandleGetGuestKnowledge(rec, req)

	// Should not be rejected — 200 or any non-400 response is acceptable
	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected guest_id of exactly 256 chars to be accepted, got 400")
	}
}

// ========================================
// HandleSaveGuestNote — happy path & edge cases
// ========================================

func TestHandleSaveGuestNote_HappyPath(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	body := `{"guest_id":"vm-300","guest_name":"DB Server","guest_type":"vm","category":"credential","title":"MySQL","content":"root:pass123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true")
	}

	// Verify the note was actually saved
	gk, err := handler.legacyAIService.GetGuestKnowledge("vm-300")
	if err != nil {
		t.Fatalf("get knowledge: %v", err)
	}
	if len(gk.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(gk.Notes))
	}
}

func TestHandleSaveGuestNote_GuestIDTooLong(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	longID := strings.Repeat("x", 257)
	body, _ := json.Marshal(map[string]string{
		"guest_id": longID, "guest_name": "X", "guest_type": "vm",
		"category": "service", "title": "T", "content": "C",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleSaveGuestNote_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", strings.NewReader("{bad"))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d for invalid JSON, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleSaveGuestNote_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	tests := []struct {
		name string
		body map[string]string
	}{
		{
			name: "missing_guest_id",
			body: map[string]string{"category": "ops", "title": "T", "content": "C"},
		},
		{
			name: "missing_category",
			body: map[string]string{"guest_id": "vm-1", "title": "T", "content": "C"},
		},
		{
			name: "missing_title",
			body: map[string]string{"guest_id": "vm-1", "category": "ops", "content": "C"},
		},
		{
			name: "missing_content",
			body: map[string]string{"guest_id": "vm-1", "category": "ops", "title": "T"},
		},
		{
			name: "all_empty",
			body: map[string]string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			handler.HandleSaveGuestNote(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected %d for %s, got %d: %s", http.StatusBadRequest, tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestHandleSaveGuestNote_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/ai/knowledge/save", nil)
			rec := httptest.NewRecorder()
			handler.HandleSaveGuestNote(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected %d for %s, got %d", http.StatusMethodNotAllowed, method, rec.Code)
			}
		})
	}
}

func TestHandleSaveGuestNote_VerifySavedFields(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	body := `{"guest_id":"vm-verify","guest_name":"Test VM","guest_type":"container","category":"service","title":"Nginx Config","content":"proxy_pass http://backend:8080"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	gk, err := handler.legacyAIService.GetGuestKnowledge("vm-verify")
	if err != nil {
		t.Fatalf("get knowledge: %v", err)
	}
	if len(gk.Notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(gk.Notes))
	}
	note := gk.Notes[0]
	if note.Category != "service" {
		t.Errorf("expected category 'service', got %q", note.Category)
	}
	if note.Title != "Nginx Config" {
		t.Errorf("expected title 'Nginx Config', got %q", note.Title)
	}
	if note.Content != "proxy_pass http://backend:8080" {
		t.Errorf("expected content 'proxy_pass http://backend:8080', got %q", note.Content)
	}
	if note.ID == "" {
		t.Error("expected note ID to be populated")
	}
	if gk.GuestName != "Test VM" {
		t.Errorf("expected guest name 'Test VM', got %q", gk.GuestName)
	}
	if gk.GuestType != "container" {
		t.Errorf("expected guest type 'container', got %q", gk.GuestType)
	}
}

func TestHandleSaveGuestNote_MultipleNotes(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// Save two notes for the same guest
	for i, note := range []struct{ title, content string }{
		{"Note One", "Content one"},
		{"Note Two", "Content two"},
	} {
		body, _ := json.Marshal(map[string]string{
			"guest_id": "vm-multi", "guest_name": "Multi VM", "guest_type": "vm",
			"category": "learning", "title": note.title, "content": note.content,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleSaveGuestNote(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("save %d: expected %d, got %d: %s", i, http.StatusOK, rec.Code, rec.Body.String())
		}
	}

	gk, err := handler.legacyAIService.GetGuestKnowledge("vm-multi")
	if err != nil {
		t.Fatalf("get knowledge: %v", err)
	}
	if len(gk.Notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(gk.Notes))
	}
}

func TestHandleSaveGuestNote_GuestIDExactlyAtLimit(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// 256 chars passes validation but may fail on the filesystem (.enc suffix
	// pushes total filename past OS limits). The important assertion is that
	// the handler does NOT reject it as 400 — the validation boundary is 256.
	exactID := strings.Repeat("s", 256)
	body, _ := json.Marshal(map[string]string{
		"guest_id": exactID, "category": "ops", "title": "Boundary", "content": "Boundary content",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/save", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleSaveGuestNote(rec, req)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected 256-char guest_id to pass validation, got 400: %s", rec.Body.String())
	}
}

// ========================================
// HandleDeleteGuestNote — happy path
// ========================================

func TestHandleDeleteGuestNote_HappyPath(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// Save a note first
	if err := handler.legacyAIService.SaveGuestNote("vm-400", "VM", "vm", "service", "Redis", "Port 6379"); err != nil {
		t.Fatalf("save note: %v", err)
	}

	gk, err := handler.legacyAIService.GetGuestKnowledge("vm-400")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(gk.Notes) == 0 {
		t.Fatalf("expected at least 1 note")
	}
	noteID := gk.Notes[0].ID

	body, _ := json.Marshal(map[string]string{"guest_id": "vm-400", "note_id": noteID})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/delete", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleDeleteGuestNote(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteGuestNote_GuestIDTooLong(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	longID := strings.Repeat("d", 257)
	body, _ := json.Marshal(map[string]string{"guest_id": longID, "note_id": "n-1"})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/delete", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleDeleteGuestNote(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleExportGuestKnowledge — sanitized filename
// ========================================

func TestHandleExportGuestKnowledge_MaliciousGuestID(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// Save a note with a guest ID containing injection characters
	maliciousID := `vm"inject`
	if err := handler.legacyAIService.SaveGuestNote(maliciousID, "VM", "vm", "ops", "Note", "Content"); err != nil {
		t.Fatalf("save: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/export?guest_id="+maliciousID, nil)
	rec := httptest.NewRecorder()
	handler.HandleExportGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	disp := rec.Header().Get("Content-Disposition")
	// The sanitized filename should NOT contain quotes from the guest ID
	if strings.Contains(disp, `vm"inject`) {
		t.Fatalf("Content-Disposition contains unsanitized guest_id: %q", disp)
	}
	// It should contain the sanitized version
	if !strings.Contains(disp, "vminject") {
		t.Fatalf("Content-Disposition missing sanitized guest_id: %q", disp)
	}
}

func TestHandleExportGuestKnowledge_SanitizedFilename(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	if err := handler.legacyAIService.SaveGuestNote("vm-500", "VM", "vm", "ops", "Note", "Content"); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Normal ID — filename should contain the ID
	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/export?guest_id=vm-500", nil)
	rec := httptest.NewRecorder()
	handler.HandleExportGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, rec.Code)
	}
	disp := rec.Header().Get("Content-Disposition")
	if !strings.Contains(disp, "vm-500") {
		t.Fatalf("expected Content-Disposition to contain 'vm-500', got %q", disp)
	}
}

func TestHandleExportGuestKnowledge_GuestIDTooLong(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	longID := strings.Repeat("z", 257)
	req := httptest.NewRequest(http.MethodGet, "/api/ai/knowledge/export?guest_id="+longID, nil)
	rec := httptest.NewRecorder()
	handler.HandleExportGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleClearGuestKnowledge — confirm=false & happy path
// ========================================

func TestHandleClearGuestKnowledge_ConfirmFalse(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	body := `{"guest_id":"vm-600","confirm":false}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/clear", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleClearGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d for confirm=false, got %d", http.StatusBadRequest, rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "confirm must be true") {
		t.Fatalf("expected confirm error, got %q", rec.Body.String())
	}
}

func TestHandleClearGuestKnowledge_HappyPath(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// Save notes
	if err := handler.legacyAIService.SaveGuestNote("vm-700", "VM", "vm", "ops", "A", "A content"); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := handler.legacyAIService.SaveGuestNote("vm-700", "VM", "vm", "ops", "B", "B content"); err != nil {
		t.Fatalf("save: %v", err)
	}

	body := `{"guest_id":"vm-700","confirm":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/clear", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleClearGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("expected success=true")
	}
	deleted, ok := resp["deleted"].(float64)
	if !ok || deleted < 1 {
		t.Fatalf("expected deleted >= 1, got %v", resp["deleted"])
	}
}

func TestHandleClearGuestKnowledge_InvalidBody(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/clear", strings.NewReader("{bad"))
	rec := httptest.NewRecorder()
	handler.HandleClearGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleClearGuestKnowledge_GuestIDTooLong(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	longID := strings.Repeat("c", 257)
	body, _ := json.Marshal(map[string]interface{}{"guest_id": longID, "confirm": true})
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/clear", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleClearGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// ========================================
// HandleImportGuestKnowledge — merge mode & edge cases
// ========================================

func TestHandleImportGuestKnowledge_GuestIDTooLong(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	longID := strings.Repeat("i", 257)
	payload := map[string]interface{}{
		"guest_id": longID,
		"notes":    []map[string]string{{"category": "ops", "title": "T", "content": "C"}},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleImportGuestKnowledge_MergeMode(t *testing.T) {
	t.Parallel()
	handler := newTestAISettingsHandlerWithService(t)

	// Save an existing note
	if err := handler.legacyAIService.SaveGuestNote("vm-800", "VM", "vm", "ops", "Existing", "Existing content"); err != nil {
		t.Fatalf("save: %v", err)
	}

	payload := map[string]interface{}{
		"guest_id":   "vm-800",
		"guest_name": "VM",
		"guest_type": "vm",
		"merge":      true,
		"notes": []map[string]string{
			{"category": "ops", "title": "New", "content": "New content"},
		},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/ai/knowledge/import", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleImportGuestKnowledge(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	// With merge=true, both old and new notes should exist
	gk, err := handler.legacyAIService.GetGuestKnowledge("vm-800")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(gk.Notes) < 2 {
		t.Fatalf("expected at least 2 notes with merge, got %d", len(gk.Notes))
	}
}

// ========================================
// sanitizeFilenameComponent
// ========================================

func TestSanitizeFilenameComponent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "normal_id", input: "vm-100", want: "vm-100"},
		{name: "with_dots", input: "host.local", want: "host.local"},
		{name: "with_underscores", input: "ct_200", want: "ct_200"},
		{name: "strips_quotes", input: `vm"inject`, want: "vminject"},
		{name: "strips_newlines", input: "vm\r\ninjection", want: "vminjection"},
		{name: "strips_slashes", input: "../../../etc/passwd", want: "......etcpasswd"},
		{name: "empty_after_strip", input: "///", want: "export"},
		{name: "all_special", input: "!@#$%^&*()", want: "export"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeFilenameComponent(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeFilenameComponent(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ========================================
// Route-level integration: auth + scope enforcement
// ========================================

func TestKnowledgeEndpoints_RequireAuth(t *testing.T) {
	t.Parallel()

	// Config with auth enabled — requests without credentials should be rejected
	cfg := &config.Config{
		DataPath:   t.TempDir(),
		ConfigPath: t.TempDir(),
		AuthUser:   "admin",
		AuthPass:   "$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ012", // bcrypt hash placeholder
	}
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/ai/knowledge?guest_id=vm-1", ""},
		{http.MethodPost, "/api/ai/knowledge/save", `{"guest_id":"vm-1","category":"ops","title":"T","content":"C"}`},
		{http.MethodPost, "/api/ai/knowledge/delete", `{"guest_id":"vm-1","note_id":"n-1"}`},
		{http.MethodGet, "/api/ai/knowledge/export?guest_id=vm-1", ""},
		{http.MethodPost, "/api/ai/knowledge/import", `{"guest_id":"vm-1","notes":[{"category":"ops","title":"T","content":"C"}]}`},
		{http.MethodPost, "/api/ai/knowledge/clear", `{"guest_id":"vm-1","confirm":true}`},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.body != "" {
				req = httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("expected %d for unauthenticated request, got %d", http.StatusUnauthorized, rec.Code)
			}
		})
	}
}

func TestKnowledgeEndpoints_RequireAIChatScope(t *testing.T) {
	t.Parallel()

	// Create a token with monitoring:read scope (NOT ai:chat)
	rawToken := "knowledge-scope-test-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeMonitoringRead}, nil)
	cfg := newTestConfigWithTokens(t, record)
	router := NewRouter(cfg, nil, nil, nil, nil, "1.0.0")

	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/api/ai/knowledge?guest_id=vm-1", ""},
		{http.MethodPost, "/api/ai/knowledge/save", `{}`},
		{http.MethodPost, "/api/ai/knowledge/delete", `{}`},
		{http.MethodGet, "/api/ai/knowledge/export?guest_id=vm-1", ""},
		{http.MethodPost, "/api/ai/knowledge/import", `{}`},
		{http.MethodPost, "/api/ai/knowledge/clear", `{}`},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			var req *http.Request
			if ep.body != "" {
				req = httptest.NewRequest(ep.method, ep.path, strings.NewReader(ep.body))
			} else {
				req = httptest.NewRequest(ep.method, ep.path, nil)
			}
			req.Header.Set("X-API-Token", rawToken)
			rec := httptest.NewRecorder()
			router.Handler().ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Fatalf("expected %d for wrong scope, got %d: %s", http.StatusForbidden, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), config.ScopeAIChat) {
				t.Fatalf("expected error to mention %q scope, got %q", config.ScopeAIChat, rec.Body.String())
			}
		})
	}
}
