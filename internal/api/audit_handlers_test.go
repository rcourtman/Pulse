package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
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
	return nil
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
}
