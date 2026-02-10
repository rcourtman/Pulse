package audit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSQLiteLoggerFactory_CreateLogger_PersistsAndSigns(t *testing.T) {
	baseDir := t.TempDir()

	factory := &SQLiteLoggerFactory{
		CryptoMgr: newMockCryptoManager(),
	}

	dbPath := filepath.Join(baseDir, "orgs", "org-a", "audit.db")
	logger, err := factory.CreateLogger(dbPath)
	if err != nil {
		t.Fatalf("CreateLogger failed: %v", err)
	}
	defer logger.Close()

	// Ensure the expected DB path exists (NewSQLiteLogger always creates <DataDir>/audit/audit.db).
	expectedDB := filepath.Join(filepath.Dir(dbPath), "audit", "audit.db")
	if _, err := os.Stat(expectedDB); err != nil {
		t.Fatalf("expected audit db to exist at %q: %v", expectedDB, err)
	}

	event := Event{
		ID:        uuid.NewString(),
		Timestamp: time.Now().UTC(),
		EventType: "factory_test",
		User:      "tester",
		IP:        "127.0.0.1",
		Path:      "/api/test",
		Success:   true,
		Details:   "hello",
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	events, err := logger.Query(QueryFilter{ID: event.ID, Limit: 1})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Signature == "" {
		t.Fatalf("expected signature to be present (signing enabled)")
	}

	verifier, ok := logger.(interface {
		VerifySignature(Event) bool
	})
	if !ok {
		t.Fatalf("expected factory logger to support signature verification")
	}
	if !verifier.VerifySignature(events[0]) {
		t.Fatalf("expected persisted event signature to verify")
	}
}

func TestSQLiteLoggerFactory_TenantIsolation(t *testing.T) {
	baseDir := t.TempDir()

	factory := &SQLiteLoggerFactory{
		CryptoMgr: newMockCryptoManager(),
	}

	mgr := NewTenantLoggerManager(baseDir, factory)
	defer mgr.Close()

	// Write to org-a.
	if err := mgr.Log("org-a", "login", "alice", "10.0.0.1", "/api/login", true, "ok"); err != nil {
		t.Fatalf("org-a Log failed: %v", err)
	}

	// Ensure org-b can't see org-a's events.
	bEvents, err := mgr.Query("org-b", QueryFilter{Limit: 100})
	if err != nil {
		t.Fatalf("org-b Query failed: %v", err)
	}
	if len(bEvents) != 0 {
		t.Fatalf("expected org-b to have 0 events, got %d", len(bEvents))
	}

	// Write to org-b.
	if err := mgr.Log("org-b", "login", "bob", "10.0.0.2", "/api/login", true, "ok"); err != nil {
		t.Fatalf("org-b Log failed: %v", err)
	}

	aEvents, err := mgr.Query("org-a", QueryFilter{Limit: 100})
	if err != nil {
		t.Fatalf("org-a Query failed: %v", err)
	}
	if len(aEvents) != 1 {
		t.Fatalf("expected org-a to have 1 event, got %d", len(aEvents))
	}
	if aEvents[0].User != "alice" {
		t.Fatalf("expected org-a user %q, got %q", "alice", aEvents[0].User)
	}

	bEvents, err = mgr.Query("org-b", QueryFilter{Limit: 100})
	if err != nil {
		t.Fatalf("org-b Query failed: %v", err)
	}
	if len(bEvents) != 1 {
		t.Fatalf("expected org-b to have 1 event, got %d", len(bEvents))
	}
	if bEvents[0].User != "bob" {
		t.Fatalf("expected org-b user %q, got %q", "bob", bEvents[0].User)
	}
}
