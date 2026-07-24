package audit

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type legacyAuditFixtureRow struct {
	id        string
	timestamp any
	user      any
	signature any
}

func createLegacyAuditFixture(
	t *testing.T,
	dataDir string,
	rows []legacyAuditFixtureRow,
) string {
	t.Helper()

	auditDir := filepath.Join(dataDir, "audit")
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		t.Fatalf("create legacy audit directory: %v", err)
	}
	dbPath := filepath.Join(auditDir, "audit.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy audit database: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`
		CREATE TABLE audit_events (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			event_type TEXT NOT NULL,
			user TEXT,
			ip TEXT,
			path TEXT,
			success INTEGER NOT NULL,
			details TEXT,
			signature TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_audit_timestamp ON audit_events(timestamp);
	`); err != nil {
		t.Fatalf("create legacy audit schema: %v", err)
	}
	for _, row := range rows {
		if _, err := db.Exec(`
			INSERT INTO audit_events
				(id, timestamp, event_type, user, ip, path, success, details, signature)
			VALUES (?, ?, 'startup', ?, '127.0.0.1', '/api/audit', 1, 'fixture', ?)`,
			row.id,
			row.timestamp,
			row.user,
			row.signature,
		); err != nil {
			t.Fatalf("insert legacy row %q: %v", row.id, err)
		}
	}
	return dbPath
}

func TestSQLiteLoggerMigratesLegacySchemaAndQueryContract(t *testing.T) {
	dataDir := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	old := now.AddDate(0, 0, -120)
	recent := now.Add(-time.Hour)
	sameSecond := now.Add(-30 * time.Minute)
	dbPath := createLegacyAuditFixture(t, dataDir, []legacyAuditFixtureRow{
		{
			id:        "legacy-old",
			timestamp: old.Format("2006-01-02 15:04:05 -0700 MST") + " m=+0.009025344",
			user:      nil,
			signature: nil,
		},
		{id: "legacy-recent", timestamp: recent, user: "alice", signature: "legacy-signature"},
		{id: "same-a", timestamp: sameSecond, user: "alice", signature: "legacy-signature"},
		{id: "same-z", timestamp: sameSecond, user: "alice", signature: "legacy-signature"},
	})

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       dataDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("migrate legacy logger: %v", err)
	}

	start := now.Add(-2 * time.Hour)
	events, total, err := logger.QueryPage(QueryFilter{
		StartTime: &start,
		User:      "alice",
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("query migrated page: %v", err)
	}
	if total != 3 || len(events) != 3 {
		t.Fatalf("migrated page = %d events, total %d, want 3/3", len(events), total)
	}
	if events[0].ID != "same-z" || events[1].ID != "same-a" || events[2].ID != "legacy-recent" {
		t.Fatalf("deterministic order = %v", []string{events[0].ID, events[1].ID, events[2].ID})
	}

	var timestampType string
	var timestampNotNull int
	rows, err := logger.db.Query(`PRAGMA table_info(audit_events)`)
	if err != nil {
		t.Fatalf("read migrated schema: %v", err)
	}
	for rows.Next() {
		var position, primaryKey int
		var name, columnType string
		var notNull int
		var defaultValue any
		if err := rows.Scan(
			&position,
			&name,
			&columnType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			t.Fatalf("scan migrated schema: %v", err)
		}
		if name == "timestamp" {
			timestampType = columnType
			timestampNotNull = notNull
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close schema rows: %v", err)
	}
	if timestampType != "INTEGER" || timestampNotNull != 1 {
		t.Fatalf("timestamp schema = %q/%d, want INTEGER/1", timestampType, timestampNotNull)
	}
	var nonInteger int
	if err := logger.db.QueryRow(
		`SELECT COUNT(*) FROM audit_events WHERE typeof(timestamp) != 'integer'`,
	).Scan(&nonInteger); err != nil {
		t.Fatalf("count non-integer timestamps: %v", err)
	}
	if nonInteger != 0 {
		t.Fatalf("non-integer timestamps = %d, want 0", nonInteger)
	}
	var schemaVersion int
	if err := logger.db.QueryRow(`SELECT MAX(version) FROM schema_version`).Scan(&schemaVersion); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if schemaVersion != auditSchemaVersion {
		t.Fatalf("schema version = %d, want %d", schemaVersion, auditSchemaVersion)
	}

	logger.cleanupOldEvents()
	oldEvents, err := logger.Query(QueryFilter{ID: "legacy-old"})
	if err != nil {
		t.Fatalf("query retained legacy row: %v", err)
	}
	if len(oldEvents) != 0 {
		t.Fatal("legacy row older than retention window was not removed")
	}
	cleanupEvents, err := logger.Query(QueryFilter{EventType: "audit_cleanup", Limit: 1})
	if err != nil || len(cleanupEvents) != 1 {
		t.Fatalf("cleanup event = %d, err %v", len(cleanupEvents), err)
	}
	if !logger.VerifySignature(cleanupEvents[0]) {
		t.Fatal("cleanup event must carry a valid signature")
	}

	if err := logger.Close(); err != nil {
		t.Fatalf("close migrated logger: %v", err)
	}
	restarted, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:       dataDir,
		CryptoMgr:     newMockCryptoManager(),
		RetentionDays: 30,
	})
	if err != nil {
		t.Fatalf("restart migrated logger: %v", err)
	}
	defer restarted.Close()
	if got := restarted.dbPath; got != dbPath {
		t.Fatalf("restart database path = %q, want %q", got, dbPath)
	}
}

func TestSQLiteLoggerEmptyQueryReturnsArrayShape(t *testing.T) {
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   t.TempDir(),
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer logger.Close()
	events, total, err := logger.QueryPage(QueryFilter{Limit: 100})
	if err != nil {
		t.Fatalf("QueryPage: %v", err)
	}
	if events == nil || len(events) != 0 || total != 0 {
		t.Fatalf("empty page = %#v, total %d, want non-nil empty/0", events, total)
	}
}

func TestSQLiteLoggerLegacyMigrationRollsBackMalformedRows(t *testing.T) {
	dataDir := t.TempDir()
	const sensitiveValue = "customer-secret-invalid-timestamp"
	dbPath := createLegacyAuditFixture(t, dataDir, []legacyAuditFixtureRow{
		{id: "valid", timestamp: time.Now().UTC(), user: "alice", signature: nil},
		{id: "invalid", timestamp: sensitiveValue, user: "bob", signature: nil},
	})

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   dataDir,
		CryptoMgr: newMockCryptoManager(),
	})
	if logger != nil || err == nil {
		t.Fatal("malformed legacy row must fail logger initialization")
	}
	if !IsStoreUnavailableError(err) {
		t.Fatalf("malformed row error must be store unavailable: %v", err)
	}
	if strings.Contains(err.Error(), sensitiveValue) {
		t.Fatalf("malformed row error leaked stored value: %v", err)
	}

	db, openErr := sql.Open("sqlite", dbPath)
	if openErr != nil {
		t.Fatalf("reopen rolled-back database: %v", openErr)
	}
	defer db.Close()
	var tableType string
	if err := db.QueryRow(`
		SELECT type FROM pragma_table_info('audit_events') WHERE name = 'timestamp'
	`).Scan(&tableType); err != nil {
		t.Fatalf("read rolled-back schema: %v", err)
	}
	if tableType != "DATETIME" {
		t.Fatalf("migration was partially applied, timestamp type = %q", tableType)
	}
	var scratchTables int
	if err := db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'audit_events_v2'
	`).Scan(&scratchTables); err != nil {
		t.Fatalf("check migration scratch table: %v", err)
	}
	if scratchTables != 0 {
		t.Fatal("migration scratch table survived rollback")
	}
}

func TestSQLiteLoggerMigratesLargeLegacyHistory(t *testing.T) {
	dataDir := t.TempDir()
	const rowCount = auditTimestampMigrationBatchSize*4 + 17
	rows := make([]legacyAuditFixtureRow, 0, rowCount)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < rowCount; i++ {
		rows = append(rows, legacyAuditFixtureRow{
			id:        fmt.Sprintf("legacy-%05d", i),
			timestamp: base.Add(time.Duration(i) * time.Second),
			user:      "operator",
			signature: nil,
		})
	}
	createLegacyAuditFixture(t, dataDir, rows)

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:             dataDir,
		CryptoMgr:           newMockCryptoManager(),
		RetentionDays:       0,
		RetentionConfigured: true,
	})
	if err != nil {
		t.Fatalf("migrate large legacy history: %v", err)
	}
	defer logger.Close()
	events, total, err := logger.QueryPage(QueryFilter{Limit: 100, Offset: rowCount - 100})
	if err != nil {
		t.Fatalf("query deep page: %v", err)
	}
	if len(events) != 100 || total != rowCount {
		t.Fatalf("deep page = %d events, total %d, want 100/%d", len(events), total, rowCount)
	}
}

func TestSQLiteLoggerRealBusyLockFailsClosedAndRecovers(t *testing.T) {
	dataDir := t.TempDir()
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   dataDir,
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer logger.Close()
	if _, err := logger.db.Exec(`PRAGMA busy_timeout = 1`); err != nil {
		t.Fatalf("set short busy timeout: %v", err)
	}

	blockerDSN := logger.dbPath + "?" + url.Values{
		"_pragma": []string{"busy_timeout(1)", "journal_mode(WAL)"},
	}.Encode()
	blocker, err := sql.Open("sqlite", blockerDSN)
	if err != nil {
		t.Fatalf("open blocking connection: %v", err)
	}
	defer blocker.Close()
	tx, err := blocker.Begin()
	if err != nil {
		t.Fatalf("begin blocking transaction: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO audit_config (key, value, updated_at) VALUES ('lock', '1', 1)`); err != nil {
		t.Fatalf("acquire write lock: %v", err)
	}

	previousSleep := auditSQLiteRetrySleep
	auditSQLiteRetrySleep = func(time.Duration) {}
	t.Cleanup(func() { auditSQLiteRetrySleep = previousSleep })
	event := Event{ID: "busy", Timestamp: time.Now(), EventType: "test", Success: true}
	err = logger.Record(event)
	if !IsStoreBusyError(err) {
		t.Fatalf("write-lock error = %v, want store busy", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("release write lock: %v", err)
	}
	if err := logger.Record(event); err != nil {
		t.Fatalf("write did not recover after lock release: %v", err)
	}
}

func TestSQLiteLoggerConcurrentPageSnapshots(t *testing.T) {
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   t.TempDir(),
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer logger.Close()

	var writers sync.WaitGroup
	for writer := 0; writer < 4; writer++ {
		writer := writer
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := 0; i < 50; i++ {
				_ = logger.Record(Event{
					ID:        fmt.Sprintf("%d-%03d", writer, i),
					Timestamp: time.Unix(1_800_000_000+int64(i), 0),
					EventType: "concurrent",
					Success:   true,
				})
			}
		}()
	}
	for i := 0; i < 40; i++ {
		events, total, err := logger.QueryPage(QueryFilter{EventType: "concurrent", Limit: 25})
		if err != nil {
			t.Fatalf("concurrent QueryPage: %v", err)
		}
		if total < len(events) {
			t.Fatalf("snapshot total %d smaller than page %d", total, len(events))
		}
		seen := make(map[string]struct{}, len(events))
		for _, event := range events {
			if _, exists := seen[event.ID]; exists {
				t.Fatalf("duplicate event %q in snapshot page", event.ID)
			}
			seen[event.ID] = struct{}{}
		}
	}
	writers.Wait()
}

func TestSQLiteLoggerOfflineBackupRestore(t *testing.T) {
	sourceDir := t.TempDir()
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   sourceDir,
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	event := Event{
		ID:        "backup-event",
		Timestamp: time.Now().UTC(),
		EventType: "backup",
		Success:   true,
	}
	if err := logger.Record(event); err != nil {
		t.Fatalf("record backup event: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close source logger: %v", err)
	}

	restoreDir := t.TempDir()
	restoreAuditDir := filepath.Join(restoreDir, "audit")
	if err := os.MkdirAll(restoreAuditDir, 0o700); err != nil {
		t.Fatalf("create restore directory: %v", err)
	}
	for _, name := range []string{"audit.db", ".audit-signing.key"} {
		source := filepath.Join(sourceDir, "audit", name)
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read backup member %q: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(restoreAuditDir, name), data, 0o600); err != nil {
			t.Fatalf("restore backup member %q: %v", name, err)
		}
	}

	restored, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   restoreDir,
		CryptoMgr: newMockCryptoManager(),
	})
	if err != nil {
		t.Fatalf("open restored logger: %v", err)
	}
	defer restored.Close()
	events, err := restored.Query(QueryFilter{ID: event.ID})
	if err != nil || len(events) != 1 {
		t.Fatalf("restored events = %d, err %v", len(events), err)
	}
	if !restored.VerifySignature(events[0]) {
		t.Fatal("restored event signature did not verify")
	}
}

func TestSQLiteLoggerCorruptStorageFailsUnavailable(t *testing.T) {
	dataDir := t.TempDir()
	auditDir := filepath.Join(dataDir, "audit")
	if err := os.MkdirAll(auditDir, 0o700); err != nil {
		t.Fatalf("create audit directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(auditDir, "audit.db"), []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatalf("write corrupt audit database: %v", err)
	}
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:   dataDir,
		CryptoMgr: newMockCryptoManager(),
	})
	if logger != nil || err == nil {
		t.Fatal("corrupt database must fail logger initialization")
	}
	if !IsStoreUnavailableError(err) && !errors.Is(err, os.ErrInvalid) {
		t.Fatalf("corrupt database error was not unavailable: %v", err)
	}
}
