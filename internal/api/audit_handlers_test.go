package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/stretchr/testify/assert"
)

type verifyResponse struct {
	Available bool   `json:"available"`
	Verified  bool   `json:"verified"`
	Message   string `json:"message"`
}

type testAuditLogger struct {
	events       []audit.Event
	verifyResult bool
	queryErr     error
	updateErr    error
	countErr     error
}

func (l *testAuditLogger) Log(event audit.Event) error {
	l.events = append(l.events, event)
	return nil
}

func (l *testAuditLogger) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	if l.queryErr != nil {
		return nil, l.queryErr
	}
	if filter.ID != "" {
		for _, event := range l.events {
			if event.ID == filter.ID {
				return []audit.Event{event}, nil
			}
		}
		return []audit.Event{}, nil
	}
	return l.events, nil
}

func (l *testAuditLogger) Count(filter audit.QueryFilter) (int, error) {
	if l.countErr != nil {
		return 0, l.countErr
	}
	events, err := l.Query(filter)
	if err != nil {
		return 0, err
	}
	return len(events), nil
}

func (l *testAuditLogger) Close() error {
	return nil
}

func (l *testAuditLogger) VerifySignature(event audit.Event) bool {
	return l.verifyResult
}

func (l *testAuditLogger) GetWebhookURLs() []string {
	return []string{}
}

func (l *testAuditLogger) UpdateWebhookURLs(urls []string) error {
	return l.updateErr
}

type testAuditLoggerNoVerify struct {
	events []audit.Event
}

func (l *testAuditLoggerNoVerify) Log(event audit.Event) error {
	l.events = append(l.events, event)
	return nil
}

func (l *testAuditLoggerNoVerify) Query(filter audit.QueryFilter) ([]audit.Event, error) {
	return l.events, nil
}

func (l *testAuditLoggerNoVerify) Count(filter audit.QueryFilter) (int, error) {
	return len(l.events), nil
}

func (l *testAuditLoggerNoVerify) Close() error {
	return nil
}

func (l *testAuditLoggerNoVerify) GetWebhookURLs() []string {
	return []string{}
}

func (l *testAuditLoggerNoVerify) UpdateWebhookURLs(urls []string) error {
	return nil
}

func setAuditLogger(t *testing.T, logger audit.Logger) {
	prev := audit.GetLogger()
	audit.SetLogger(logger)
	t.Cleanup(func() {
		audit.SetLogger(prev)
	})
}
func TestHandleVerifyAuditEvent_InvalidPath(t *testing.T) {
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/verify", nil)
	rec := httptest.NewRecorder()

	handler.HandleVerifyAuditEvent(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestHandleVerifyAuditEvent_NotPersistent(t *testing.T) {
	setAuditLogger(t, audit.NewConsoleLogger())
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.HandleVerifyAuditEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp verifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Available {
		t.Fatalf("expected available to be false")
	}
	if resp.Message == "" {
		t.Fatalf("expected message to be set")
	}
}

func TestHandleVerifyAuditEvent_NoVerifier(t *testing.T) {
	setAuditLogger(t, &testAuditLoggerNoVerify{})
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.HandleVerifyAuditEvent(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}

func TestHandleVerifyAuditEvent_NotFound(t *testing.T) {
	setAuditLogger(t, &testAuditLogger{})
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.HandleVerifyAuditEvent(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
	}
}

func TestHandleVerifyAuditEvent_Verified(t *testing.T) {
	setAuditLogger(t, &testAuditLogger{
		events:       []audit.Event{{ID: "abc"}},
		verifyResult: true,
	})
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.HandleVerifyAuditEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp verifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Available {
		t.Fatalf("expected available to be true")
	}
	if !resp.Verified {
		t.Fatalf("expected verified to be true")
	}
}

func TestHandleVerifyAuditEvent_Failed(t *testing.T) {
	setAuditLogger(t, &testAuditLogger{
		events:       []audit.Event{{ID: "abc"}},
		verifyResult: false,
	})
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
	req.SetPathValue("id", "abc")
	rec := httptest.NewRecorder()

	handler.HandleVerifyAuditEvent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp verifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if !resp.Available {
		t.Fatalf("expected available to be true")
	}
	if resp.Verified {
		t.Fatalf("expected verified to be false")
	}

	t.Run("Event not found", func(t *testing.T) {
		setAuditLogger(t, &testAuditLogger{
			events: []audit.Event{},
		})
		req := httptest.NewRequest(http.MethodGet, "/api/audit/missing/verify", nil)
		req.SetPathValue("id", "missing")
		rec := httptest.NewRecorder()
		handler.HandleVerifyAuditEvent(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		var resp APIError
		json.Unmarshal(rec.Body.Bytes(), &resp)
		assert.Equal(t, "Audit event not found", resp.ErrorMessage)
	})

	t.Run("Not persistent logger", func(t *testing.T) {
		setAuditLogger(t, audit.NewConsoleLogger())
		req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
		req.SetPathValue("id", "abc")
		rec := httptest.NewRecorder()
		handler.HandleVerifyAuditEvent(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code) // Console logger returns 200 with available: false
	})

	t.Run("Missing ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/audit//verify", nil)
		// Don't set path value
		rec := httptest.NewRecorder()
		handler.HandleVerifyAuditEvent(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Query error", func(t *testing.T) {
		setAuditLogger(t, &testAuditLogger{
			queryErr: fmt.Errorf("query error"),
		})
		req := httptest.NewRequest(http.MethodGet, "/api/audit/abc/verify", nil)
		req.SetPathValue("id", "abc")
		rec := httptest.NewRecorder()
		handler.HandleVerifyAuditEvent(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleListAuditEvents(t *testing.T) {
	setAuditLogger(t, &testAuditLogger{
		events: []audit.Event{{ID: "1", EventType: "login"}, {ID: "2", EventType: "logout"}},
	})
	handler := NewAuditHandlers()

	// Test success
	req := httptest.NewRequest(http.MethodGet, "/api/audit?event=login", nil)
	rec := httptest.NewRecorder()
	handler.HandleListAuditEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["total"].(float64) != 2 {
		t.Errorf("expected total 2, got %v", resp["total"])
	}

	// Test parse error for startTime/endTime
	req = httptest.NewRequest(http.MethodGet, "/api/audit?startTime=invalid&endTime=invalid", nil)
	rec = httptest.NewRecorder()
	handler.HandleListAuditEvents(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code) // It just ignores invalid times

	// Test method not allowed
	req = httptest.NewRequest(http.MethodPost, "/api/audit", nil)
	rec = httptest.NewRecorder()
	handler.HandleListAuditEvents(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleGetWebhooks(t *testing.T) {
	handler := NewAuditHandlers()

	// Test success
	req := httptest.NewRequest(http.MethodGet, "/api/audit/webhooks", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetWebhooks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, ok := resp["urls"]; !ok {
		t.Error("expected urls field in response")
	}

	// Test method not allowed
	req = httptest.NewRequest(http.MethodPost, "/api/audit/webhooks", nil)
	rec = httptest.NewRecorder()
	handler.HandleGetWebhooks(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleUpdateWebhooks(t *testing.T) {
	handler := NewAuditHandlers()

	// Test success
	body := `{"urls": ["https://example.com/webhook"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/audit/webhooks", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateWebhooks(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	// Test invalid URL (loopback)
	body = `{"urls": ["http://127.0.0.1/webhook"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/audit/webhooks", strings.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleUpdateWebhooks(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d for loopback URL, got %d", http.StatusBadRequest, rec.Code)
	}

	// Test invalid JSON
	req = httptest.NewRequest(http.MethodPost, "/api/audit/webhooks", strings.NewReader("invalid"))
	rec = httptest.NewRecorder()
	handler.HandleUpdateWebhooks(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	// Test method not allowed
	req = httptest.NewRequest(http.MethodGet, "/api/audit/webhooks", nil)
	rec = httptest.NewRecorder()
	handler.HandleUpdateWebhooks(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}

	// Test update error
	setAuditLogger(t, &testAuditLogger{
		updateErr: fmt.Errorf("update failed"),
	})
	body = `{"urls": ["https://example.com/webhook"]}`
	req = httptest.NewRequest(http.MethodPost, "/api/audit/webhooks", strings.NewReader(body))
	rec = httptest.NewRecorder()
	handler.HandleUpdateWebhooks(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}

func TestHandleExportAuditEvents_NotPersistent(t *testing.T) {
	setAuditLogger(t, audit.NewConsoleLogger())
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/export", nil)
	rec := httptest.NewRecorder()
	handler.HandleExportAuditEvents(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}

func TestHandleAuditSummary_NotPersistent(t *testing.T) {
	setAuditLogger(t, audit.NewConsoleLogger())
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit/summary", nil)
	rec := httptest.NewRecorder()
	handler.HandleAuditSummary(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected status %d, got %d", http.StatusNotImplemented, rec.Code)
	}
}

func TestIsPrivateOrReservedIP(t *testing.T) {
	testCases := []struct {
		ip       string
		reserved bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"8.8.8.8", false},
		{"169.254.1.1", true},
		{"224.0.0.1", true},
		{"0.0.0.0", true},
		{"0.255.255.255", true},
		{"::1", true},
		{"fe80::1", true},
		{"ff02::1", true},
	}

	for _, tc := range testCases {
		ip := net.ParseIP(tc.ip)
		if got := isPrivateOrReservedIP(ip); got != tc.reserved {
			t.Errorf("isPrivateOrReservedIP(%s) = %v, want %v", tc.ip, got, tc.reserved)
		}
	}
}

func TestValidateWebhookURL(t *testing.T) {
	origResolver := resolveWebhookIPs
	resolveWebhookIPs = func(_ context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
	}
	t.Cleanup(func() {
		resolveWebhookIPs = origResolver
	})

	testCases := []struct {
		url     string
		wantErr bool
	}{
		{"https://example.com", false},
		{"http://test.com/hook", false},
		{"", true},
		{"   ", true},
		{"://", true},
		{"ftp://example.com", true},
		{"https://", true},
		{"https://localhost", true},
		{"http://127.0.0.1", true},
		{"https://192.168.1.100", true},
		{"https://metadata.google", true},
		{"https://internal.site", true},
		{"http://example.local", true},
		{"https://example.com/path\x7f", true},
	}

	for _, tc := range testCases {
		err := validateWebhookURL(tc.url)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateWebhookURL(%s) error = %v, wantErr %v", tc.url, err, tc.wantErr)
		}
	}
}

func TestValidateWebhookURLBlocksPrivateResolution(t *testing.T) {
	origResolver := resolveWebhookIPs
	resolveWebhookIPs = func(_ context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.1")}}, nil
	}
	t.Cleanup(func() {
		resolveWebhookIPs = origResolver
	})

	if err := validateWebhookURL("https://example.com"); err == nil {
		t.Fatalf("expected hostname resolving to private IP to be rejected")
	}
}

func TestHandleListAuditEvents_Filters(t *testing.T) {
	setAuditLogger(t, &testAuditLogger{
		events: []audit.Event{{ID: "1", EventType: "login", Success: true}},
	})
	handler := NewAuditHandlers()

	// Test with various filters
	req := httptest.NewRequest(http.MethodGet, "/api/audit?limit=10&offset=0&startTime=2023-01-01T00:00:00Z&endTime=2024-01-01T00:00:00Z&success=true", nil)
	rec := httptest.NewRecorder()
	handler.HandleListAuditEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestHandleListAuditEvents_QueryError(t *testing.T) {
	setAuditLogger(t, &testAuditLogger{
		queryErr: fmt.Errorf("db error"),
	})
	handler := NewAuditHandlers()

	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec := httptest.NewRecorder()
	handler.HandleListAuditEvents(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	// Test Count error
	setAuditLogger(t, &testAuditLogger{
		countErr: fmt.Errorf("count error"),
	})
	req = httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	rec = httptest.NewRecorder()
	handler.HandleListAuditEvents(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected count error status %d, got %d", http.StatusInternalServerError, rec.Code)
	}
}
func TestHandleExportAuditEvents(t *testing.T) {
	oldLogger := audit.GetLogger()
	defer audit.SetLogger(oldLogger)

	logger := &testAuditLogger{
		events: []audit.Event{
			{ID: "1", EventType: "test", Success: true, Timestamp: time.Now()},
		},
	}
	audit.SetLogger(logger)

	h := NewAuditHandlers()

	t.Run("JSON format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit/export?format=json", nil)
		w := httptest.NewRecorder()
		h.HandleExportAuditEvents(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=audit-log-")
		assert.Equal(t, "1", w.Header().Get("X-Event-Count"))
	})

	t.Run("CSV format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit/export?format=csv", nil)
		w := httptest.NewRecorder()
		h.HandleExportAuditEvents(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment; filename=audit-log-")
	})

	t.Run("With filters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit/export?event=test&user=admin&startTime=2026-01-01T00:00:00Z&endTime=2026-12-31T23:59:59Z&success=true&verify=true", nil)
		w := httptest.NewRecorder()
		h.HandleExportAuditEvents(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/audit/export", nil)
		w := httptest.NewRecorder()
		h.HandleExportAuditEvents(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Export error", func(t *testing.T) {
		setAuditLogger(t, &testAuditLogger{
			queryErr: fmt.Errorf("query error"),
		})
		req := httptest.NewRequest("GET", "/api/audit/export", nil)
		w := httptest.NewRecorder()
		h.HandleExportAuditEvents(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

func TestHandleAuditSummary(t *testing.T) {
	oldLogger := audit.GetLogger()
	defer audit.SetLogger(oldLogger)

	logger := &testAuditLogger{
		events: []audit.Event{
			{ID: "1", EventType: "login", Success: true, Timestamp: time.Now()},
			{ID: "2", EventType: "login", Success: false, Timestamp: time.Now()},
		},
	}
	audit.SetLogger(logger)

	h := NewAuditHandlers()

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit/summary?verify=true", nil)
		w := httptest.NewRecorder()
		h.HandleAuditSummary(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var summary audit.ExportSummary
		err := json.NewDecoder(w.Body).Decode(&summary)
		assert.NoError(t, err)
		assert.Equal(t, 2, summary.TotalEvents)
		assert.Equal(t, 1, summary.SuccessCount)
		assert.Equal(t, 1, summary.FailureCount)
	})

	t.Run("With filters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/audit/summary?event=login&user=admin&startTime=2026-01-01T00:00:00Z&endTime=2026-12-31T23:59:59Z", nil)
		w := httptest.NewRecorder()
		h.HandleAuditSummary(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("Method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/audit/summary", nil)
		w := httptest.NewRecorder()
		h.HandleAuditSummary(w, req)
		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("Summary error", func(t *testing.T) {
		setAuditLogger(t, &testAuditLogger{
			queryErr: fmt.Errorf("query error"),
		})
		req := httptest.NewRequest("GET", "/api/audit/summary", nil)
		w := httptest.NewRecorder()
		h.HandleAuditSummary(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
