package audit

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewSQLiteLogger(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	if logger.GetRetentionDays() != 30 {
		t.Errorf("Expected retention days 30, got %d", logger.GetRetentionDays())
	}
}

func TestNewSQLiteLoggerDefaultRetention(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   tempDir,
		CryptoMgr: newMockCryptoManager(),
		// RetentionDays not set
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	if logger.GetRetentionDays() != 90 {
		t.Errorf("Expected default retention days 90, got %d", logger.GetRetentionDays())
	}
}

func TestSQLiteLoggerLog(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	event := Event{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		EventType: "test_event",
		User:      "testuser",
		IP:        "192.168.1.1",
		Path:      "/api/test",
		Success:   true,
		Details:   "test details",
	}

	err = logger.Log(event)
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Query the event back
	events, err := logger.Query(QueryFilter{ID: event.ID})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	retrieved := events[0]
	if retrieved.ID != event.ID {
		t.Errorf("ID mismatch: expected %s, got %s", event.ID, retrieved.ID)
	}
	if retrieved.EventType != event.EventType {
		t.Errorf("EventType mismatch: expected %s, got %s", event.EventType, retrieved.EventType)
	}
	if retrieved.User != event.User {
		t.Errorf("User mismatch: expected %s, got %s", event.User, retrieved.User)
	}
	if retrieved.Success != event.Success {
		t.Errorf("Success mismatch: expected %v, got %v", event.Success, retrieved.Success)
	}

	// Event should have a signature
	if retrieved.Signature == "" {
		t.Error("Expected event to have a signature")
	}
}

func TestSQLiteLoggerQuery(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	// Log several events
	now := time.Now()
	events := []Event{
		{ID: "e1", Timestamp: now.Add(-2 * time.Hour), EventType: "login", User: "alice", Success: true},
		{ID: "e2", Timestamp: now.Add(-1 * time.Hour), EventType: "login", User: "bob", Success: true},
		{ID: "e3", Timestamp: now, EventType: "logout", User: "alice", Success: true},
		{ID: "e4", Timestamp: now, EventType: "login", User: "charlie", Success: false},
	}

	for _, e := range events {
		if err := logger.Log(e); err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// Test event type filter
	t.Run("FilterByEventType", func(t *testing.T) {
		results, err := logger.Query(QueryFilter{EventType: "login"})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(results) != 3 {
			t.Errorf("Expected 3 login events, got %d", len(results))
		}
	})

	// Test user filter
	t.Run("FilterByUser", func(t *testing.T) {
		results, err := logger.Query(QueryFilter{User: "alice"})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 events for alice, got %d", len(results))
		}
	})

	// Test success filter
	t.Run("FilterBySuccess", func(t *testing.T) {
		success := false
		results, err := logger.Query(QueryFilter{Success: &success})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 failed event, got %d", len(results))
		}
	})

	// Test time range filter
	t.Run("FilterByTimeRange", func(t *testing.T) {
		start := now.Add(-90 * time.Minute)
		end := now.Add(-30 * time.Minute)
		results, err := logger.Query(QueryFilter{StartTime: &start, EndTime: &end})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 event in time range, got %d", len(results))
		}
	})

	// Test limit and offset
	t.Run("LimitAndOffset", func(t *testing.T) {
		results, err := logger.Query(QueryFilter{Limit: 2, Offset: 1})
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("Expected 2 events with limit, got %d", len(results))
		}
	})
}

func TestSQLiteLoggerCount(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	// Log several events
	for i := 0; i < 5; i++ {
		event := Event{
			ID:        uuid.NewString(),
			Timestamp: time.Now(),
			EventType: "test",
			Success:   i%2 == 0,
		}
		if err := logger.Log(event); err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	}

	// Count all
	count, err := logger.Count(QueryFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}

	// Count successful
	success := true
	count, err = logger.Count(QueryFilter{Success: &success})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 successful events, got %d", count)
	}
}

func TestSQLiteLoggerVerifySignature(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	event := Event{
		ID:        uuid.NewString(),
		Timestamp: time.Now(),
		EventType: "verify_test",
		User:      "testuser",
		Success:   true,
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Query the event back
	events, err := logger.Query(QueryFilter{ID: event.ID})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	// Verify should succeed
	if !logger.VerifySignature(events[0]) {
		t.Error("VerifySignature should return true for valid event")
	}

	// Tamper with event and verify should fail
	tamperedEvent := events[0]
	tamperedEvent.Details = "tampered"
	if logger.VerifySignature(tamperedEvent) {
		t.Error("VerifySignature should return false for tampered event")
	}
}

func TestSQLiteLoggerWebhooks(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	// Initially no webhooks
	urls := logger.GetWebhookURLs()
	if len(urls) != 0 {
		t.Errorf("Expected no webhooks initially, got %d", len(urls))
	}

	// Add webhooks
	testURLs := []string{"https://example.com/webhook1", "https://example.com/webhook2"}
	err = logger.UpdateWebhookURLs(testURLs)
	if err != nil {
		t.Fatalf("UpdateWebhookURLs failed: %v", err)
	}

	urls = logger.GetWebhookURLs()
	if len(urls) != 2 {
		t.Errorf("Expected 2 webhooks, got %d", len(urls))
	}

	// Clear webhooks
	err = logger.UpdateWebhookURLs([]string{})
	if err != nil {
		t.Fatalf("UpdateWebhookURLs (clear) failed: %v", err)
	}

	urls = logger.GetWebhookURLs()
	if len(urls) != 0 {
		t.Errorf("Expected 0 webhooks after clear, got %d", len(urls))
	}
}

func TestSQLiteLoggerRetention(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 1, // 1 day retention for testing
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	// Log an old event (2 days ago)
	oldEvent := Event{
		ID:        "old-event",
		Timestamp: time.Now().Add(-48 * time.Hour),
		EventType: "old",
		Success:   true,
	}
	if err := logger.Log(oldEvent); err != nil {
		t.Fatalf("Log old event failed: %v", err)
	}

	// Log a recent event
	newEvent := Event{
		ID:        "new-event",
		Timestamp: time.Now(),
		EventType: "new",
		Success:   true,
	}
	if err := logger.Log(newEvent); err != nil {
		t.Fatalf("Log new event failed: %v", err)
	}

	// Run cleanup
	logger.cleanupOldEvents()

	// Old event should be deleted
	events, err := logger.Query(QueryFilter{ID: "old-event"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 0 {
		t.Error("Old event should have been deleted")
	}

	// New event should still exist
	events, err = logger.Query(QueryFilter{ID: "new-event"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 1 {
		t.Error("New event should still exist")
	}
}

func TestSQLiteLoggerSetRetentionDays(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	logger.SetRetentionDays(60)
	if logger.GetRetentionDays() != 60 {
		t.Errorf("Expected retention days 60, got %d", logger.GetRetentionDays())
	}
}

func TestSQLiteLoggerPersistence(t *testing.T) {
	tempDir := t.TempDir()

	// Create logger and log an event
	logger1, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}

	event := Event{
		ID:        "persist-test",
		Timestamp: time.Now(),
		EventType: "persistence_test",
		User:      "testuser",
		Success:   true,
	}
	if err := logger1.Log(event); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	// Close the logger
	logger1.Close()

	// Create a new logger with same data dir
	logger2, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger (reload) failed: %v", err)
	}
	defer logger2.Close()

	// Query the event - should still exist
	events, err := logger2.Query(QueryFilter{ID: "persist-test"})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 1 {
		t.Error("Event should persist across logger restarts")
	}

	// Signature should still verify
	if len(events) > 0 && !logger2.VerifySignature(events[0]) {
		t.Error("Signature should still verify after restart")
	}
}

func TestSQLiteLoggerConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}
	defer logger.Close()

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 10; j++ {
				event := Event{
					ID:        uuid.NewString(),
					Timestamp: time.Now(),
					EventType: "concurrent_test",
					User:      "user",
					Success:   true,
				}
				if err := logger.Log(event); err != nil {
					t.Errorf("Concurrent log failed: %v", err)
				}
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify count
	count, err := logger.Count(QueryFilter{EventType: "concurrent_test"})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 100 {
		t.Errorf("Expected 100 events, got %d", count)
	}
}

func TestSQLiteLoggerCloseIdempotent(t *testing.T) {
	tempDir := t.TempDir()

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       tempDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger failed: %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}
