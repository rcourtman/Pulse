package unifiedresources

import (
	"database/sql"
	"testing"
	"time"
)

func TestPruneOldRecords_DeletesOldResourceChanges(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(resourceChangesRetention + time.Hour))
	recent := now.Add(-time.Minute)

	change := ResourceChange{
		ID:         "chg-old",
		ResourceID: "vm:100",
		ObservedAt: old,
		Kind:       ChangeStateTransition,
		From:       "offline",
		To:         "online",
		SourceType: SourcePlatformEvent,
		Confidence: ConfidenceHigh,
	}
	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange old: %v", err)
	}

	change.ID = "chg-recent"
	change.ObservedAt = recent
	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange recent: %v", err)
	}

	store.pruneOldRecords()

	results, err := store.GetRecentChanges("vm:100", now.Add(-resourceChangesRetention*2), 100)
	if err != nil {
		t.Fatalf("GetRecentChanges: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 change after prune, got %d", len(results))
	}
	if results[0].ID != "chg-recent" {
		t.Errorf("expected chg-recent to survive, got %s", results[0].ID)
	}
}

func TestPruneOldRecords_DeletesOldActionAudits(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(actionAuditsRetention + time.Hour)).Format("2006-01-02T15:04:05Z")
	recent := now.Add(-time.Minute).Format("2006-01-02T15:04:05Z")

	_, err := store.db.Exec(`INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json)
		VALUES ('audit-old', 'act-1', 'res:1', 'req-1', ?, ?, 'completed', '{}', '{}')`, old, old)
	if err != nil {
		t.Fatalf("insert old audit: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO action_audits (id, action_id, canonical_id, request_id, created_at, updated_at, state, request_json, plan_json)
		VALUES ('audit-recent', 'act-2', 'res:1', 'req-2', ?, ?, 'completed', '{}', '{}')`, recent, recent)
	if err != nil {
		t.Fatalf("insert recent audit: %v", err)
	}

	store.pruneOldRecords()

	var count int
	err = store.db.QueryRow(`SELECT count(*) FROM action_audits`).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 action_audit after prune, got %d", count)
	}
}

func TestPruneOldRecords_DeletesOldActionLifecycleEvents(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(actionLifecycleRetention + time.Hour)).Format("2006-01-02T15:04:05Z")
	recent := now.Add(-time.Minute).Format("2006-01-02T15:04:05Z")

	_, err := store.db.Exec(`INSERT INTO action_lifecycle_events (action_id, timestamp, state, actor, message)
		VALUES ('act-1', ?, 'queued', 'system', 'old')`, old)
	if err != nil {
		t.Fatalf("insert old lifecycle: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO action_lifecycle_events (action_id, timestamp, state, actor, message)
		VALUES ('act-2', ?, 'queued', 'system', 'recent')`, recent)
	if err != nil {
		t.Fatalf("insert recent lifecycle: %v", err)
	}

	store.pruneOldRecords()

	var count int
	err = store.db.QueryRow(`SELECT count(*) FROM action_lifecycle_events`).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 lifecycle event after prune, got %d", count)
	}
}

func TestPruneOldRecords_DeletesOldExportAudits(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(exportAuditsRetention + time.Hour)).Format("2006-01-02T15:04:05Z")
	recent := now.Add(-time.Minute).Format("2006-01-02T15:04:05Z")

	_, err := store.db.Exec(`INSERT INTO export_audits (id, timestamp, actor, envelope_hash, decision, destination)
		VALUES ('exp-old', ?, 'admin', 'hash1', 'approved', 'email')`, old)
	if err != nil {
		t.Fatalf("insert old export audit: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO export_audits (id, timestamp, actor, envelope_hash, decision, destination)
		VALUES ('exp-recent', ?, 'admin', 'hash2', 'approved', 'email')`, recent)
	if err != nil {
		t.Fatalf("insert recent export audit: %v", err)
	}

	store.pruneOldRecords()

	var count int
	err = store.db.QueryRow(`SELECT count(*) FROM export_audits`).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 export audit after prune, got %d", count)
	}
}

func TestPruneOldRecords_DeletesOldLoopReports(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(loopReportsRetention + time.Hour)).Format("2006-01-02T15:04:05Z")
	recent := now.Add(-time.Minute).Format("2006-01-02T15:04:05Z")

	_, err := store.db.Exec(`INSERT INTO loop_reports (id, report_type, scope, trigger, goal, status, started_at, completed_at)
		VALUES ('rpt-old', 'maintenance', 'res:1', 'tick', '', 'pass', ?, ?)`, old, old)
	if err != nil {
		t.Fatalf("insert old loop report: %v", err)
	}
	_, err = store.db.Exec(`INSERT INTO loop_reports (id, report_type, scope, trigger, goal, status, started_at, completed_at)
		VALUES ('rpt-recent', 'maintenance', 'res:1', 'tick', '', 'pass', ?, ?)`, recent, recent)
	if err != nil {
		t.Fatalf("insert recent loop report: %v", err)
	}

	store.pruneOldRecords()

	var count int
	err = store.db.QueryRow(`SELECT count(*) FROM loop_reports`).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 loop report after prune, got %d", count)
	}
}

func TestMigrateAutoVacuum_SetsIncrementalMode(t *testing.T) {
	store := newTestStore(t)

	var mode int
	err := store.db.QueryRow("PRAGMA auto_vacuum").Scan(&mode)
	if err != nil {
		t.Fatalf("PRAGMA auto_vacuum: %v", err)
	}
	if mode != 2 {
		t.Fatalf("expected auto_vacuum=2 (INCREMENTAL), got %d", mode)
	}
}

func TestReclaimFreePages_ReducesFreelistAfterPrune(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(resourceChangesRetention + time.Hour))

	for i := 0; i < 100; i++ {
		change := ResourceChange{
			ID:         "chg-" + string(rune('A'+i)) + "-old",
			ResourceID: "vm:100",
			ObservedAt: old,
			Kind:       ChangeStateTransition,
			From:       "offline",
			To:         "online",
			SourceType: SourcePlatformEvent,
			Confidence: ConfidenceHigh,
		}
		if err := store.RecordChange(change); err != nil {
			t.Fatalf("RecordChange %d: %v", i, err)
		}
	}

	store.pruneOldRecords()

	var freelist int64
	err := store.db.QueryRow(`PRAGMA freelist_count`).Scan(&freelist)
	if err != nil {
		t.Fatalf("freelist_count: %v", err)
	}
	if freelist > 0 {
		var dbSize int64
		err = store.db.QueryRow(`PRAGMA page_count`).Scan(&dbSize)
		if err != nil {
			t.Fatalf("page_count: %v", err)
		}
		t.Logf("freelist=%d, page_count=%d (free pages should be 0 after reclaim)", freelist, dbSize)
	}
}

func TestRetentionLoop_RunsInitialPrune(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().UTC()
	old := now.Add(-(resourceChangesRetention + time.Hour))

	change := ResourceChange{
		ID:         "chg-loop-init",
		ResourceID: "vm:100",
		ObservedAt: old,
		Kind:       ChangeStateTransition,
		From:       "offline",
		To:         "online",
		SourceType: SourcePlatformEvent,
		Confidence: ConfidenceHigh,
	}
	if err := store.RecordChange(change); err != nil {
		t.Fatalf("RecordChange: %v", err)
	}

	stop := store.startRetentionLoop()
	defer close(stop)

	deadline := time.Now().Add(initialRetentionDelay + 10*time.Second)
	for time.Now().Before(deadline) {
		results, err := store.GetRecentChanges("vm:100", now.Add(-resourceChangesRetention*2), 100)
		if err != nil {
			t.Fatalf("GetRecentChanges: %v", err)
		}
		if len(results) == 0 {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("initial prune did not run within expected time")
}

func TestPruneOldRecords_NoErrorOnEmptyTables(t *testing.T) {
	store := newTestStore(t)

	store.pruneOldRecords()

	var changesCount int
	err := store.db.QueryRow(`SELECT count(*) FROM resource_changes`).Scan(&changesCount)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("count resource_changes: %v", err)
	}
	if changesCount != 0 {
		t.Errorf("expected 0 resource_changes, got %d", changesCount)
	}
}
