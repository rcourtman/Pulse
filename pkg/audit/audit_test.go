package audit

import (
	"testing"
	"time"
)

func TestConsoleLogger_Log(t *testing.T) {
	logger := NewConsoleLogger()

	event := Event{
		ID:        "test-id-123",
		Timestamp: time.Now(),
		EventType: "login",
		User:      "testuser",
		IP:        "192.168.1.100",
		Path:      "/api/login",
		Success:   true,
		Details:   "Basic auth login",
	}

	err := logger.Log(event)
	if err != nil {
		t.Errorf("ConsoleLogger.Log() returned error: %v", err)
	}
}

func TestConsoleLogger_Log_Failed(t *testing.T) {
	logger := NewConsoleLogger()

	event := Event{
		ID:        "test-id-456",
		Timestamp: time.Now(),
		EventType: "login",
		User:      "baduser",
		IP:        "10.0.0.5",
		Path:      "/api/login",
		Success:   false,
		Details:   "Invalid credentials",
	}

	err := logger.Log(event)
	if err != nil {
		t.Errorf("ConsoleLogger.Log() returned error: %v", err)
	}
}

func TestConsoleLogger_Query(t *testing.T) {
	logger := NewConsoleLogger()

	events, err := logger.Query(QueryFilter{})
	if err != nil {
		t.Errorf("ConsoleLogger.Query() returned error: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("ConsoleLogger.Query() should return empty slice, got %d events", len(events))
	}
}

func TestConsoleLogger_Count(t *testing.T) {
	logger := NewConsoleLogger()

	count, err := logger.Count(QueryFilter{})
	if err != nil {
		t.Errorf("ConsoleLogger.Count() returned error: %v", err)
	}

	if count != 0 {
		t.Errorf("ConsoleLogger.Count() should return 0, got %d", count)
	}
}

func TestConsoleLogger_Close(t *testing.T) {
	logger := NewConsoleLogger()

	err := logger.Close()
	if err != nil {
		t.Errorf("ConsoleLogger.Close() returned error: %v", err)
	}
}

func TestConsoleLogger_Webhooks(t *testing.T) {
	logger := NewConsoleLogger()

	if urls := logger.GetWebhookURLs(); len(urls) != 0 {
		t.Fatalf("expected no webhook URLs, got %v", urls)
	}

	if err := logger.UpdateWebhookURLs([]string{"https://example.com"}); err != nil {
		t.Fatalf("UpdateWebhookURLs returned error: %v", err)
	}
}

func TestSetLogger_GetLogger(t *testing.T) {
	// Create a custom logger for testing
	customLogger := NewConsoleLogger()

	SetLogger(customLogger)

	got := GetLogger()
	if got != customLogger {
		t.Error("GetLogger() did not return the logger set by SetLogger()")
	}
}

func TestLog_ConvenienceFunction(t *testing.T) {
	// Set a console logger
	SetLogger(NewConsoleLogger())

	// This should not panic
	Log("test_event", "user", "127.0.0.1", "/test", true, "test details")
}

func TestGetLogger_DefaultsToConsole(t *testing.T) {
	// Reset global state for this test
	loggerMu.Lock()
	globalLogger = nil
	loggerMu.Unlock()

	logger := GetLogger()
	if logger == nil {
		t.Error("GetLogger() returned nil")
	}

	_, ok := logger.(*ConsoleLogger)
	if !ok {
		t.Error("GetLogger() should return ConsoleLogger by default")
	}
}
