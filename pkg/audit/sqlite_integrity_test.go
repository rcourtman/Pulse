package audit

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"
)

func legacyFixtureEvent(id string, timestamp time.Time, user string) Event {
	return Event{
		ID:        id,
		Timestamp: timestamp,
		EventType: "startup",
		User:      user,
		IP:        "127.0.0.1",
		Path:      "/api/audit",
		Success:   true,
		Details:   "fixture",
	}
}

func TestSQLiteSignatureCompatibilityMatrixPreservesFractionalEvidence(t *testing.T) {
	dataDir := t.TempDir()
	key := []byte("0123456789abcdef0123456789abcdef")
	signer, err := NewSignerWithKey(key)
	if err != nil {
		t.Fatalf("NewSignerWithKey: %v", err)
	}
	base := time.Date(2025, 7, 8, 9, 10, 11, 987_654_321, time.UTC)

	fractional := legacyFixtureEvent("legacy-fractional", base, "fractional")
	fractional.Signature = signer.signCanonical(signer.legacyTimeCanonicalForm(fractional))
	v2 := legacyFixtureEvent("v2", base.Add(time.Second).Truncate(time.Second), "v2")
	v2.Signature = signatureV2Prefix + hex.EncodeToString(signer.mac(signer.canonicalV2Form(v2)))
	zeroOne := legacyFixtureEvent("legacy-zero-one", base.Add(2*time.Second).Truncate(time.Second), "zero-one")
	zeroOne.Signature = signer.signCanonical(signer.legacyZeroOneCanonicalForm(zeroOne))
	unix := legacyFixtureEvent("legacy-unix", base.Add(3*time.Second).Truncate(time.Second), "unix")
	unix.Signature = signer.signCanonical(signer.legacyUnixCanonicalForm(unix))
	realTimestamp := legacyFixtureEvent("legacy-real", base.Add(4*time.Second).Truncate(time.Second), "real")
	realTimestamp.Signature = signer.signCanonical(signer.legacyUnixCanonicalForm(realTimestamp))
	invalid := legacyFixtureEvent("invalid", base.Add(5*time.Second).Truncate(time.Second), "invalid")
	invalid.Signature = strings.Repeat("0", 64)
	unknown := legacyFixtureEvent("unknown", base.Add(6*time.Second).Truncate(time.Second), "unknown")
	unknown.Signature = "v9:" + strings.Repeat("0", 64)
	unsigned := legacyFixtureEvent("unsigned", base.Add(7*time.Second).Truncate(time.Second), "unsigned")

	createLegacyAuditFixture(t, dataDir, []legacyAuditFixtureRow{
		{id: fractional.ID, timestamp: fractional.Timestamp, user: fractional.User, signature: fractional.Signature},
		{id: v2.ID, timestamp: v2.Timestamp.Unix(), user: v2.User, signature: v2.Signature},
		{id: zeroOne.ID, timestamp: zeroOne.Timestamp.Unix(), user: zeroOne.User, signature: zeroOne.Signature},
		{id: unix.ID, timestamp: unix.Timestamp.Unix(), user: unix.User, signature: unix.Signature},
		{id: realTimestamp.ID, timestamp: float64(realTimestamp.Timestamp.Unix()), user: realTimestamp.User, signature: realTimestamp.Signature},
		{id: invalid.ID, timestamp: invalid.Timestamp.Unix(), user: invalid.User, signature: invalid.Signature},
		{id: unknown.ID, timestamp: unknown.Timestamp.Unix(), user: unknown.User, signature: unknown.Signature},
		{id: unsigned.ID, timestamp: unsigned.Timestamp.Unix(), user: unsigned.User, signature: nil},
	})

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:             dataDir,
		SigningKey:          key,
		RetentionDays:       0,
		RetentionConfigured: true,
	})
	if err != nil {
		t.Fatalf("migrate logger: %v", err)
	}
	current := Event{ID: "current", Timestamp: base.Add(8 * time.Second), EventType: "startup", Success: true, Details: "fixture"}
	if err := logger.Record(current); err != nil {
		t.Fatalf("Record current: %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("close migrated logger: %v", err)
	}

	restarted, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:             dataDir,
		SigningKey:          key,
		RetentionDays:       0,
		RetentionConfigured: true,
	})
	if err != nil {
		t.Fatalf("restart logger: %v", err)
	}
	defer restarted.Close()

	events, err := restarted.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(events) != 9 {
		t.Fatalf("events = %d, want 9", len(events))
	}
	byID := make(map[string]Event, len(events))
	for _, event := range events {
		byID[event.ID] = event
	}
	if got := byID[fractional.ID].SignatureTimestamp; got != base.Format(time.RFC3339Nano) {
		t.Fatalf("fractional signature timestamp = %q, want %q", got, base.Format(time.RFC3339Nano))
	}

	expected := map[string]VerificationStatus{
		"current":        VerificationStatusStrong,
		fractional.ID:    VerificationStatusCompatibility,
		v2.ID:            VerificationStatusCompatibility,
		zeroOne.ID:       VerificationStatusCompatibility,
		unix.ID:          VerificationStatusCompatibility,
		realTimestamp.ID: VerificationStatusCompatibility,
		invalid.ID:       VerificationStatusInvalid,
		unknown.ID:       VerificationStatusUnknown,
		unsigned.ID:      VerificationStatusUnsigned,
	}
	for id, status := range expected {
		if got := restarted.VerifySignatureResult(byID[id]); got.Status != status {
			t.Errorf("%s status = %q, want %q (%+v)", id, got.Status, status, got)
		}
	}

	summary, err := NewExporter(restarted).GenerateSummary(QueryFilter{}, true)
	if err != nil {
		t.Fatalf("GenerateSummary: %v", err)
	}
	if summary.StrongSigCount != 1 || summary.CompatibilitySigCount != 5 || summary.InvalidSigCount != 1 || summary.UnknownSigCount != 1 || summary.UnsignedSigCount != 1 {
		t.Fatalf("summary assurance counts = %+v", summary)
	}
}

func TestSQLiteCanonicalStorageAndRawMutationFailClosed(t *testing.T) {
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:             t.TempDir(),
		SigningKey:          []byte("0123456789abcdef0123456789abcdef"),
		RetentionDays:       0,
		RetentionConfigured: true,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer logger.Close()
	event := Event{ID: "canonical", Timestamp: time.Unix(1_750_000_000, 0), EventType: "test", User: "operator", Success: false}
	if err := logger.Record(event); err != nil {
		t.Fatalf("Record: %v", err)
	}

	var storageTypes [10]string
	if err := logger.db.QueryRow(`SELECT typeof(id), typeof(timestamp), typeof(event_type), typeof(user), typeof(ip), typeof(path), typeof(success), typeof(details), typeof(signature), typeof(signature_timestamp) FROM audit_events WHERE id = ?`, event.ID).Scan(
		&storageTypes[0], &storageTypes[1], &storageTypes[2], &storageTypes[3], &storageTypes[4],
		&storageTypes[5], &storageTypes[6], &storageTypes[7], &storageTypes[8], &storageTypes[9],
	); err != nil {
		t.Fatalf("read storage classes: %v", err)
	}
	for i, got := range storageTypes {
		want := "text"
		if i == 1 || i == 6 {
			want = "integer"
		}
		if got != want {
			t.Fatalf("storage type %d = %q, want %q", i, got, want)
		}
	}

	for _, mutation := range []string{
		`UPDATE audit_events SET success = 2 WHERE id = 'canonical'`,
		`UPDATE audit_events SET user = NULL WHERE id = 'canonical'`,
		`UPDATE audit_events SET ip = x'00' WHERE id = 'canonical'`,
		`UPDATE audit_events SET timestamp = 1.5 WHERE id = 'canonical'`,
	} {
		if _, err := logger.db.Exec(mutation); err == nil {
			t.Fatalf("noncanonical mutation succeeded: %s", mutation)
		}
	}

	// SQLite losslessly coerces an integer into canonical TEXT storage. That is
	// still a semantic mutation and must invalidate, rather than preserve, the
	// signature through a lossy Go projection.
	if _, err := logger.db.Exec(`UPDATE audit_events SET user = 42 WHERE id = ?`, event.ID); err != nil {
		t.Fatalf("coerced text mutation: %v", err)
	}
	mutated, err := logger.Query(QueryFilter{ID: event.ID})
	if err != nil || len(mutated) != 1 {
		t.Fatalf("query coerced mutation: %v / %d", err, len(mutated))
	}
	if result := logger.VerifySignatureResult(mutated[0]); result.Status != VerificationStatusInvalid {
		t.Fatalf("coerced mutation result = %+v", result)
	}
	if _, err := logger.db.Exec(`UPDATE audit_events SET user = 'operator' WHERE id = ?`, event.ID); err != nil {
		t.Fatalf("restore user: %v", err)
	}

	// Even if a privileged connection disables CHECK constraints, decoding the
	// raw storage class/value rejects the historic success=2 projection flaw.
	if _, err := logger.db.Exec(`PRAGMA ignore_check_constraints = ON`); err != nil {
		t.Fatalf("enable constraint bypass: %v", err)
	}
	if _, err := logger.db.Exec(`UPDATE audit_events SET success = 2 WHERE id = ?`, event.ID); err != nil {
		t.Fatalf("inject noncanonical success: %v", err)
	}
	if _, err := logger.Query(QueryFilter{ID: event.ID}); err == nil {
		t.Fatal("query accepted noncanonical success=2")
	}
	if _, err := logger.db.Exec(`UPDATE audit_events SET success = 0, signature_timestamp = '2025-06-15T15:06:40+00:00' WHERE id = ?`, event.ID); err != nil {
		t.Fatalf("inject alternate timestamp encoding: %v", err)
	}
	if _, err := logger.Query(QueryFilter{ID: event.ID}); err == nil {
		t.Fatal("query accepted alternate signature timestamp encoding")
	}
}

func TestSQLiteCanonicalStoragePreservesEmptySignedStrings(t *testing.T) {
	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir:             t.TempDir(),
		SigningKey:          []byte("0123456789abcdef0123456789abcdef"),
		RetentionDays:       0,
		RetentionConfigured: true,
	})
	if err != nil {
		t.Fatalf("NewSQLiteLogger: %v", err)
	}
	defer logger.Close()

	// Logger.Record has historically accepted empty ID and event type strings.
	// They remain one canonical TEXT representation even though NULL and other
	// SQLite storage classes fail closed.
	event := Event{Timestamp: time.Unix(1_750_000_000, 0), Success: true}
	if err := logger.Record(event); err != nil {
		t.Fatalf("Record empty signed strings: %v", err)
	}
	events, err := logger.Query(QueryFilter{Limit: 1})
	if err != nil || len(events) != 1 {
		t.Fatalf("Query empty signed strings: %v / %d", err, len(events))
	}
	if events[0].ID != "" || events[0].EventType != "" {
		t.Fatalf("empty signed strings changed: %+v", events[0])
	}
	if result := logger.VerifySignatureResult(events[0]); result.Status != VerificationStatusStrong {
		t.Fatalf("empty signed string assurance = %+v", result)
	}
}

func TestSQLiteMigrationInjectedInterruptionRollsBackExactly(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := createLegacyAuditFixture(t, dataDir, []legacyAuditFixtureRow{
		{id: "one", timestamp: "2025-01-01T00:00:00.123456789Z", user: nil, signature: nil},
		{id: "two", timestamp: int64(1_735_689_601), user: "operator", signature: ""},
	})

	previousHook := auditMigrationBeforeInsert
	inserted := 0
	auditMigrationBeforeInsert = func(canonicalAuditRow) error {
		inserted++
		if inserted == 2 {
			return errors.New("synthetic migration interruption")
		}
		return nil
	}
	t.Cleanup(func() { auditMigrationBeforeInsert = previousHook })

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{DataDir: dataDir, SigningKey: []byte("0123456789abcdef0123456789abcdef")})
	if logger != nil || err == nil || !strings.Contains(err.Error(), "synthetic migration interruption") {
		t.Fatalf("interrupted migration = logger %v, err %v", logger, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen original: %v", err)
	}
	defer db.Close()
	var columnType string
	if err := db.QueryRow(`SELECT type FROM pragma_table_info('audit_events') WHERE name = 'timestamp'`).Scan(&columnType); err != nil {
		t.Fatalf("read original schema: %v", err)
	}
	if columnType != "DATETIME" {
		t.Fatalf("partial schema publication: timestamp type %q", columnType)
	}
	var count, scratch int
	if err := db.QueryRow(`SELECT COUNT(*) FROM audit_events`).Scan(&count); err != nil {
		t.Fatalf("count original rows: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_events_v3'`).Scan(&scratch); err != nil {
		t.Fatalf("count scratch tables: %v", err)
	}
	if count != 2 || scratch != 0 {
		t.Fatalf("rollback state rows=%d scratch=%d", count, scratch)
	}
}

func TestSQLiteMigrationPreservesMinimumRowID(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := createLegacyAuditFixture(t, dataDir, []legacyAuditFixtureRow{{
		id: "minimum-rowid", timestamp: int64(1_735_689_600), user: "operator", signature: nil,
	}})
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy fixture: %v", err)
	}
	if _, err := db.Exec(`UPDATE audit_events SET rowid = ? WHERE id = ?`, int64(-1<<63), "minimum-rowid"); err != nil {
		_ = db.Close()
		t.Fatalf("assign minimum rowid: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy fixture: %v", err)
	}

	logger, err := NewSQLiteLogger(SQLiteLoggerConfig{
		DataDir: dataDir, SigningKey: []byte("0123456789abcdef0123456789abcdef"),
		RetentionDays: 0, RetentionConfigured: true,
	})
	if err != nil {
		t.Fatalf("migrate minimum rowid: %v", err)
	}
	defer logger.Close()
	events, err := logger.Query(QueryFilter{})
	if err != nil || len(events) != 1 || events[0].ID != "minimum-rowid" {
		t.Fatalf("migrated rows = %+v, err %v", events, err)
	}
}
