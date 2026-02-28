package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestWrap(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "testdb")
	if idb.DB != raw {
		t.Fatal("Wrap should store the original *sql.DB")
	}
	if idb.name != "testdb" {
		t.Fatalf("expected name=testdb, got %q", idb.name)
	}
}

func TestExecAndQuery(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "exec_query_db")

	// Snapshot counters before operations.
	execBefore := histogramSampleCount(t, "exec_query_db", "exec")

	// Create a table and insert a row.
	_, err := idb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("Exec CREATE: %v", err)
	}
	_, err = idb.Exec("INSERT INTO t (val) VALUES (?)", "hello")
	if err != nil {
		t.Fatalf("Exec INSERT: %v", err)
	}

	// Query the row back.
	rows, err := idb.Query("SELECT val FROM t WHERE id = ?", 1)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal("expected one row")
	}
	var val string
	if err := rows.Scan(&val); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got %q", val)
	}

	// QueryRow.
	var val2 string
	if err := idb.QueryRow("SELECT val FROM t WHERE id = ?", 1).Scan(&val2); err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if val2 != "hello" {
		t.Fatalf("expected 'hello', got %q", val2)
	}

	// Verify the exec histogram grew by at least 2 (CREATE + INSERT).
	execAfter := histogramSampleCount(t, "exec_query_db", "exec")
	if execAfter-execBefore < 2 {
		t.Fatalf("expected at least 2 new exec histogram samples, got %d", execAfter-execBefore)
	}
}

func TestTransactionInstrumentation(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "tx_instr_db")

	beginBefore := histogramSampleCount(t, "tx_instr_db", "begin")
	commitBefore := histogramSampleCount(t, "tx_instr_db", "commit")

	_, err := idb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	tx, err := idb.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	_, err = tx.Exec("INSERT INTO t (val) VALUES (?)", "world")
	if err != nil {
		t.Fatalf("tx.Exec: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify begin and commit were recorded.
	if histogramSampleCount(t, "tx_instr_db", "begin")-beginBefore < 1 {
		t.Fatal("expected at least 1 new begin histogram sample")
	}
	if histogramSampleCount(t, "tx_instr_db", "commit")-commitBefore < 1 {
		t.Fatal("expected at least 1 new commit histogram sample")
	}
}

func TestPreparedStatementInstrumentation(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "stmt_instr_db")

	_, err := idb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	stmtExecBefore := histogramSampleCount(t, "stmt_instr_db", "stmt_exec")

	tx, err := idb.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	stmt, err := tx.Prepare("INSERT INTO t (val) VALUES (?)")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer stmt.Close()

	for i := 0; i < 5; i++ {
		_, err = stmt.Exec("val")
		if err != nil {
			t.Fatalf("stmt.Exec: %v", err)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify all 5 prepared statement executions were instrumented.
	stmtExecAfter := histogramSampleCount(t, "stmt_instr_db", "stmt_exec")
	delta := stmtExecAfter - stmtExecBefore
	if delta < 5 {
		t.Fatalf("expected at least 5 new stmt_exec histogram samples, got %d", delta)
	}
}

func TestSlowQueryDetection(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping timing-sensitive test in CI")
	}

	raw := openTestDB(t)
	idb := Wrap(raw, "slow_detect_db")

	slowBefore := slowQueryCount(t, "slow_detect_db", "exec")

	// Directly call observe with a timestamp in the past to simulate a slow query.
	past := time.Now().Add(-150 * time.Millisecond)
	observe(idb.name, "exec", "SELECT slow_query()", past)

	// Check the slow query counter grew by 1.
	slowAfter := slowQueryCount(t, "slow_detect_db", "exec")
	if slowAfter-slowBefore != 1 {
		t.Fatalf("expected 1 new slow query count, got %v", slowAfter-slowBefore)
	}

	// A fast observe should NOT increment the slow counter.
	fastBefore := slowQueryCount(t, "slow_detect_db", "query")
	observe(idb.name, "query", "SELECT 1", time.Now())
	fastAfter := slowQueryCount(t, "slow_detect_db", "query")
	if fastAfter-fastBefore != 0 {
		t.Fatalf("expected 0 new slow query count for fast query, got %v", fastAfter-fastBefore)
	}
}

func TestQueryTruncation(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "trunc_db")

	slowBefore := slowQueryCount(t, "trunc_db", "exec")

	// Build a long query string (>200 chars).
	longQuery := "SELECT " + string(make([]byte, 300))

	// Simulate a slow query — just verifying it doesn't panic.
	past := time.Now().Add(-200 * time.Millisecond)
	observe(idb.name, "exec", longQuery, past)

	// If we got here without panic, truncation worked.
	slowAfter := slowQueryCount(t, "trunc_db", "exec")
	if slowAfter-slowBefore != 1 {
		t.Fatalf("expected 1 new slow query, got %v", slowAfter-slowBefore)
	}
}

func TestRollback(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "rb_db")

	_, err := idb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY)")
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	tx, err := idb.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}

	_, err = tx.Exec("INSERT INTO t (id) VALUES (1)")
	if err != nil {
		t.Fatalf("tx.Exec: %v", err)
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	// Verify the row was NOT committed.
	var count int
	if err := idb.QueryRow("SELECT COUNT(*) FROM t").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after rollback, got %d", count)
	}
}

func TestClose(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "close_db")

	if err := idb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Further operations should fail on a closed DB.
	_, err := idb.Exec("SELECT 1")
	if err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestDBPrepare(t *testing.T) {
	raw := openTestDB(t)
	idb := Wrap(raw, "prep_db")

	_, err := idb.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)")
	if err != nil {
		t.Fatalf("CREATE: %v", err)
	}

	stmt, err := idb.Prepare("INSERT INTO t (val) VALUES (?)")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec("prepared_val")
	if err != nil {
		t.Fatalf("stmt.Exec: %v", err)
	}

	var val string
	if err := idb.QueryRow("SELECT val FROM t WHERE id = 1").Scan(&val); err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	if val != "prepared_val" {
		t.Fatalf("expected 'prepared_val', got %q", val)
	}
}

func TestSlowQueryThresholdConstant(t *testing.T) {
	if SlowQueryThreshold != 100*time.Millisecond {
		t.Fatalf("SlowQueryThreshold should be 100ms, got %v", SlowQueryThreshold)
	}
}

// histogramSampleCount returns the sample count from the duration histogram.
func histogramSampleCount(t *testing.T, database, operation string) uint64 {
	t.Helper()
	ensureMetrics()
	obs, err := dbQueryDuration.GetMetricWithLabelValues(database, operation)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%q, %q): %v", database, operation, err)
	}
	m := &dto.Metric{}
	if err := obs.(prometheus.Metric).Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Histogram == nil {
		return 0
	}
	return m.Histogram.GetSampleCount()
}

// slowQueryCount returns the current value of the slow query counter.
func slowQueryCount(t *testing.T, database, operation string) float64 {
	t.Helper()
	ensureMetrics()
	counter, err := dbSlowQueries.GetMetricWithLabelValues(database, operation)
	if err != nil {
		t.Fatalf("GetMetricWithLabelValues(%q, %q): %v", database, operation, err)
	}
	m := &dto.Metric{}
	if err := counter.Write(m); err != nil {
		t.Fatalf("Write metric: %v", err)
	}
	if m.Counter == nil {
		return 0
	}
	return m.Counter.GetValue()
}
