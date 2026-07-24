package audit

import (
	"testing"
	"time"
)

func TestAsyncLoggerPreservesPersistentReadAndVerificationContracts(t *testing.T) {
	backend, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   t.TempDir(),
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	event := Event{
		ID:        "async-contract",
		Timestamp: time.Now().UTC(),
		EventType: "test",
		Success:   true,
	}
	if err := backend.Record(event); err != nil {
		t.Fatalf("record event: %v", err)
	}

	logger := NewAsyncLogger(backend, AsyncLoggerConfig{BufferSize: 8})
	defer logger.Close()
	events, total, err := logger.QueryPage(QueryFilter{ID: event.ID, Limit: 1})
	if err != nil {
		t.Fatalf("QueryPage: %v", err)
	}
	if len(events) != 1 || total != 1 {
		t.Fatalf("QueryPage = %d events, total %d, want 1/1", len(events), total)
	}
	if !logger.VerifySignature(events[0]) {
		t.Fatal("async wrapper did not delegate signature verification")
	}
	if _, ok := any(logger).(PersistentLogger); !ok {
		t.Fatal("async persistent logger no longer satisfies export contract")
	}
}
