package audit

import (
	"path/filepath"
	"strings"
	"testing"
)

type stubLogger struct {
	events     []Event
	queryCalls int
	countCalls int
	closed     bool
	urls       []string
}

func (s *stubLogger) Log(event Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *stubLogger) Query(filter QueryFilter) ([]Event, error) {
	s.queryCalls++
	return []Event{{EventType: "test"}}, nil
}

func (s *stubLogger) Count(filter QueryFilter) (int, error) {
	s.countCalls++
	return 7, nil
}

func (s *stubLogger) GetWebhookURLs() []string {
	return s.urls
}

func (s *stubLogger) UpdateWebhookURLs(urls []string) error {
	s.urls = urls
	return nil
}

func (s *stubLogger) Close() error {
	s.closed = true
	return nil
}

type stubLoggerFactory struct {
	created []string
	logger  Logger
	err     error
}

func (f *stubLoggerFactory) CreateLogger(dbPath string) (Logger, error) {
	f.created = append(f.created, dbPath)
	if f.err != nil {
		return nil, f.err
	}
	return f.logger, nil
}

func TestTenantLoggerManager_GetLogger_Creates(t *testing.T) {
	logger := &stubLogger{}
	factory := &stubLoggerFactory{logger: logger}
	manager := NewTenantLoggerManager("data", factory)

	got := manager.GetLogger("org-1")
	if got != logger {
		t.Fatalf("expected factory logger")
	}

	expectedPath := filepath.Join("data", "orgs", "org-1", "audit.db")
	if len(factory.created) != 1 || factory.created[0] != expectedPath {
		t.Fatalf("expected db path %q, got %v", expectedPath, factory.created)
	}
}

func TestTenantLoggerManager_GetLogger_Default(t *testing.T) {
	manager := NewTenantLoggerManager("data", &stubLoggerFactory{logger: &stubLogger{}})
	logger := manager.GetLogger("default")
	if logger == nil {
		t.Fatalf("expected default logger")
	}
}

func TestTenantLoggerManager_GetLogger_InvalidOrgID(t *testing.T) {
	logger := &stubLogger{}
	factory := &stubLoggerFactory{logger: logger}
	manager := NewTenantLoggerManager("data", factory)

	got := manager.GetLogger("../org-1")
	if got == nil {
		t.Fatalf("expected fallback logger for invalid org ID")
	}
	if got == logger {
		t.Fatalf("expected invalid org ID to bypass tenant logger factory")
	}
	if len(factory.created) != 0 {
		t.Fatalf("expected no tenant logger creation for invalid org ID, got %v", factory.created)
	}
	if len(manager.GetAllLoggers()) != 0 {
		t.Fatalf("expected invalid org ID logger not to be cached")
	}
}

func TestTenantLoggerManager_GetLogger_InvalidOrgIDsRejected(t *testing.T) {
	logger := &stubLogger{}
	factory := &stubLoggerFactory{logger: logger}
	manager := NewTenantLoggerManager("data", factory)

	invalidIDs := []string{
		"",
		".",
		"..",
		"../org-1",
		"org/one",
		"org one",
		"org\tone",
		"org\none",
		"org\\one",
		"org:one",
		strings.Repeat("x", 65),
	}

	for _, orgID := range invalidIDs {
		got := manager.GetLogger(orgID)
		if got == nil {
			t.Fatalf("expected fallback logger for invalid orgID %q", orgID)
		}
		if got == logger {
			t.Fatalf("expected invalid orgID %q to bypass tenant logger factory", orgID)
		}
		if len(factory.created) != 0 {
			t.Fatalf("expected no tenant logger creation for invalid orgID %q, got %v", orgID, factory.created)
		}
		if len(manager.GetAllLoggers()) != 0 {
			t.Fatalf("expected invalid orgID %q logger not to be cached", orgID)
		}
	}
}

func TestTenantLoggerManager_LogQueryCount(t *testing.T) {
	logger := &stubLogger{}
	manager := NewTenantLoggerManager("data", &stubLoggerFactory{logger: logger})

	if err := manager.Log("org-1", "login", "user", "ip", "/path", true, "details"); err != nil {
		t.Fatalf("unexpected log error: %v", err)
	}
	if len(logger.events) != 1 {
		t.Fatalf("expected 1 logged event")
	}

	if _, err := manager.Query("org-1", QueryFilter{}); err != nil || logger.queryCalls != 1 {
		t.Fatalf("expected query to be called")
	}
	if _, err := manager.Count("org-1", QueryFilter{}); err != nil || logger.countCalls != 1 {
		t.Fatalf("expected count to be called")
	}
}

func TestTenantLoggerManager_CloseAndRemove(t *testing.T) {
	logger := &stubLogger{}
	manager := NewTenantLoggerManager("data", &stubLoggerFactory{logger: logger})
	manager.GetLogger("org-1")

	manager.RemoveTenantLogger("org-1")
	if !logger.closed {
		t.Fatalf("expected logger to be closed on removal")
	}
	if len(manager.GetAllLoggers()) != 0 {
		t.Fatalf("expected logger map to be empty after removal")
	}

	manager.GetLogger("org-1")
	manager.Close()
	if len(manager.GetAllLoggers()) != 0 {
		t.Fatalf("expected logger map to be cleared on close")
	}
}

func TestConsoleLogger_WebhookMethods(t *testing.T) {
	logger := NewConsoleLogger()
	if len(logger.GetWebhookURLs()) != 0 {
		t.Fatalf("expected empty webhook URLs")
	}
	if err := logger.UpdateWebhookURLs([]string{"http://example.com"}); err != nil {
		t.Fatalf("expected UpdateWebhookURLs to succeed")
	}
}
