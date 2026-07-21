package metrics

import (
	"testing"
	"time"
)

// A connection pool of one queued every UI history read behind metric-flush
// commits, freezing charts whenever a commit picked up a WAL checkpoint
// (#1601). Reads must proceed on their own connections while a write
// transaction holds the WAL write lock.
func TestQueriesDoNotQueueBehindWriteTransaction(t *testing.T) {
	store, err := NewStore(DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	// Let startup maintenance (checkpoint/vacuum) finish so its locks don't
	// entangle with the write transaction opened below.
	if err := store.WaitForMaintenance(10 * time.Second); err != nil {
		t.Fatalf("WaitForMaintenance: %v", err)
	}

	if got := store.db.DB.Stats().MaxOpenConnections; got < 2 {
		t.Fatalf("MaxOpenConnections = %d, want at least 2 so reads do not serialize behind writes", got)
	}

	// Hold the WAL write lock on one connection.
	tx, err := store.db.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(
		`INSERT INTO metrics (resource_type, resource_id, metric_type, value, timestamp, tier)
		 VALUES ('node', 'n1', 'cpu', 1.0, ?, 'raw')`,
		time.Now().Unix(),
	); err != nil {
		t.Fatalf("write inside transaction: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := store.Query("node", "n1", "cpu", time.Now().Add(-time.Hour), time.Now(), 60)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Query during open write transaction: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Query blocked behind the open write transaction")
	}
}
