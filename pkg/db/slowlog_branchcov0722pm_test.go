package db

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// TestBranchcov0722PM raises branch coverage on the Context/BeginTx/setter
// wrappers in slowlog.go that slowlog_test.go does not reach:
//
//   - InstrumentedDB:   ExecContext, QueryContext, QueryRowContext, BeginTx,
//     SetMaxOpenConns, SetMaxIdleConns, SetConnMaxLifetime
//   - InstrumentedTx:   ExecContext, Query, QueryContext, QueryRow
//   - InstrumentedStmt: ExecContext, Query, QueryRow
//
// Each wrapper is driven through both its success arm (asserting the concrete
// return value AND that the per-operation Prometheus histogram sample count
// grew, proving observe() instrumentation actually ran) and its error arm
// (invalid SQL, wrong arg count, or a cancelled context — asserting the
// error is propagated to the caller). Unique db names per subtest isolate
// the global Prometheus counters.
//
// TIMING NOTE: SlowQueryThreshold is 100ms but the spec caps test sleeps at
// 50ms, so no wrapper call here can deterministically exceed the threshold
// without a >50ms sleep. The slow-query branch of observe() is already 100%
// covered by TestSlowQueryDetection / TestQueryTruncation in slowlog_test.go
// (which call observe() directly with a backdated start time). These tests
// therefore cover pass-through + instrumentation-counter behaviour + error
// propagation only; slow-query detection via a wrapper call is intentionally
// not asserted here.
func TestBranchcov0722PM(t *testing.T) {
	ctx := context.Background()

	// ---------------- InstrumentedDB.ExecContext --------------------------
	t.Run("DB_ExecContext", func(t *testing.T) {
		const name = "branchcov0722pm_db_execctx"
		idb := Wrap(openTestDB(t), name)
		before := histogramSampleCount(t, name, "exec")

		res, err := idb.ExecContext(ctx,
			"CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)")
		if err != nil {
			t.Fatalf("ExecContext CREATE: unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("ExecContext: expected non-nil Result, got nil")
		}

		// Error path: invalid SQL. observe() runs unconditionally, so the
		// histogram must still advance.
		_, err = idb.ExecContext(ctx,
			"INSERT INTO no_such_table (val) VALUES (?)", "x")
		if err == nil {
			t.Fatal("ExecContext: expected error for invalid SQL, got nil")
		}
		if !strings.Contains(strings.ToLower(err.Error()), "no such table") {
			t.Fatalf("ExecContext: expected 'no such table' in error, got %q", err.Error())
		}

		if got := histogramSampleCount(t, name, "exec") - before; got != 2 {
			t.Fatalf("exec histogram delta = %d, want 2", got)
		}
	})

	// ---------------- InstrumentedDB.QueryContext -------------------------
	t.Run("DB_QueryContext", func(t *testing.T) {
		const name = "branchcov0722pm_db_queryctx"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "ctx")

		before := histogramSampleCount(t, name, "query")

		rows, err := idb.QueryContext(ctx, "SELECT val FROM t")
		if err != nil {
			t.Fatalf("QueryContext: unexpected error: %v", err)
		}
		var vals []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				t.Fatalf("Scan: %v", err)
			}
			vals = append(vals, v)
		}
		rows.Close()
		if len(vals) != 1 || vals[0] != "ctx" {
			t.Fatalf("QueryContext: rows = %v, want [ctx]", vals)
		}

		// Error path.
		_, err = idb.QueryContext(ctx, "SELECT * FROM no_such_table")
		if err == nil {
			t.Fatal("QueryContext: expected error for invalid SQL, got nil")
		}

		if got := histogramSampleCount(t, name, "query") - before; got != 2 {
			t.Fatalf("query histogram delta = %d, want 2", got)
		}
	})

	// ---------------- InstrumentedDB.QueryRowContext ----------------------
	t.Run("DB_QueryRowContext", func(t *testing.T) {
		const name = "branchcov0722pm_db_queryrowctx"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "rowctx")

		before := histogramSampleCount(t, name, "query_row")

		var val string
		if err := idb.QueryRowContext(ctx,
			"SELECT val FROM t WHERE id = ?", 1).Scan(&val); err != nil {
			t.Fatalf("QueryRowContext.Scan: unexpected error: %v", err)
		}
		if val != "rowctx" {
			t.Fatalf("QueryRowContext: val = %q, want rowctx", val)
		}

		// Error path: no matching row -> sql.ErrNoRows from Scan.
		var missing string
		err := idb.QueryRowContext(ctx,
			"SELECT val FROM t WHERE id = ?", 9999).Scan(&missing)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("QueryRowContext: expected sql.ErrNoRows, got %v", err)
		}

		if got := histogramSampleCount(t, name, "query_row") - before; got != 2 {
			t.Fatalf("query_row histogram delta = %d, want 2", got)
		}
	})

	// ---------------- InstrumentedDB.BeginTx ------------------------------
	t.Run("DB_BeginTx", func(t *testing.T) {
		const name = "branchcov0722pm_db_begintx"
		idb := Wrap(openTestDB(t), name)

		// Success path: returns a fully-populated InstrumentedTx and bumps
		// the "begin" histogram.
		before := histogramSampleCount(t, name, "begin")
		tx, err := idb.BeginTx(ctx, nil)
		if err != nil {
			t.Fatalf("BeginTx: unexpected error: %v", err)
		}
		if tx == nil || tx.Tx == nil {
			t.Fatal("BeginTx: expected non-nil InstrumentedTx wrapping a *sql.Tx")
		}
		if tx.name != name {
			t.Fatalf("BeginTx: tx.name = %q, want %q", tx.name, name)
		}
		if got := histogramSampleCount(t, name, "begin") - before; got != 1 {
			t.Fatalf("begin histogram delta = %d, want 1", got)
		}
		if err := tx.Rollback(); err != nil {
			t.Fatalf("Rollback: %v", err)
		}

		// Error path: a pre-cancelled context must surface as
		// "context canceled" and a nil tx (the wrapper's
		// `if err != nil { return nil, err }` arm).
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		ntx, err := idb.BeginTx(cctx, nil)
		if err == nil {
			t.Fatal("BeginTx: expected error from cancelled context, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("BeginTx: expected context.Canceled, got %v", err)
		}
		if ntx != nil {
			t.Fatalf("BeginTx: expected nil tx on error, got %T", ntx)
		}
	})

	// ---------------- InstrumentedDB setters ------------------------------
	t.Run("DB_Setters", func(t *testing.T) {
		idb := Wrap(openTestDB(t), "branchcov0722pm_db_setters")

		// SetMaxOpenConns: database/sql exposes the configured value via
		// Stats().MaxOpenConnections, so assert it round-trips — both the
		// bounded value and 0 (unlimited).
		idb.SetMaxOpenConns(7)
		if got := idb.DB.Stats().MaxOpenConnections; got != 7 {
			t.Fatalf("SetMaxOpenConns(7): Stats().MaxOpenConnections = %d, want 7", got)
		}
		idb.SetMaxOpenConns(0)
		if got := idb.DB.Stats().MaxOpenConnections; got != 0 {
			t.Fatalf("SetMaxOpenConns(0): Stats().MaxOpenConnections = %d, want 0", got)
		}

		// SetMaxIdleConns / SetConnMaxLifetime: database/sql exposes no
		// getter for the configured value, so the only observable behaviour
		// is that the delegation returns without panicking. Exercise both
		// with a positive value and with zero to cover the call sites.
		idb.SetMaxIdleConns(4)
		idb.SetMaxIdleConns(0)
		idb.SetConnMaxLifetime(5 * time.Second)
		idb.SetConnMaxLifetime(0)
	})

	// ---------------- InstrumentedTx.ExecContext --------------------------
	t.Run("Tx_ExecContext", func(t *testing.T) {
		const name = "branchcov0722pm_tx_execctx"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "")

		tx := beginTx(t, idb)
		defer rollbackQuiet(tx)
		before := histogramSampleCount(t, name, "tx_exec")

		res, err := tx.ExecContext(ctx,
			"INSERT INTO t (val) VALUES (?)", "a")
		if err != nil {
			t.Fatalf("tx.ExecContext: unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("tx.ExecContext: expected non-nil Result")
		}

		// Error path.
		_, err = tx.ExecContext(ctx,
			"INSERT INTO no_such_table (val) VALUES (?)", "a")
		if err == nil {
			t.Fatal("tx.ExecContext: expected error for invalid SQL, got nil")
		}

		if got := histogramSampleCount(t, name, "tx_exec") - before; got != 2 {
			t.Fatalf("tx_exec histogram delta = %d, want 2", got)
		}
	})

	// ---------------- InstrumentedTx.Query --------------------------------
	t.Run("Tx_Query", func(t *testing.T) {
		const name = "branchcov0722pm_tx_query"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "tq")

		tx := beginTx(t, idb)
		defer rollbackQuiet(tx)
		before := histogramSampleCount(t, name, "tx_query")

		rows, err := tx.Query("SELECT val FROM t")
		if err != nil {
			t.Fatalf("tx.Query: unexpected error: %v", err)
		}
		var got []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				t.Fatalf("Scan: %v", err)
			}
			got = append(got, v)
		}
		rows.Close()
		if len(got) != 1 || got[0] != "tq" {
			t.Fatalf("tx.Query: rows = %v, want [tq]", got)
		}

		// Error path.
		_, err = tx.Query("SELECT * FROM no_such_table")
		if err == nil {
			t.Fatal("tx.Query: expected error for invalid SQL, got nil")
		}

		if delta := histogramSampleCount(t, name, "tx_query") - before; delta != 2 {
			t.Fatalf("tx_query histogram delta = %d, want 2", delta)
		}
	})

	// ---------------- InstrumentedTx.QueryContext -------------------------
	t.Run("Tx_QueryContext", func(t *testing.T) {
		const name = "branchcov0722pm_tx_queryctx"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "tqc")

		tx := beginTx(t, idb)
		defer rollbackQuiet(tx)
		before := histogramSampleCount(t, name, "tx_query")

		rows, err := tx.QueryContext(ctx, "SELECT val FROM t WHERE id = ?", 1)
		if err != nil {
			t.Fatalf("tx.QueryContext: unexpected error: %v", err)
		}
		if !rows.Next() {
			rows.Close()
			t.Fatal("tx.QueryContext: expected one row")
		}
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			t.Fatalf("Scan: %v", err)
		}
		rows.Close()
		if v != "tqc" {
			t.Fatalf("tx.QueryContext: val = %q, want tqc", v)
		}

		// Error path.
		_, err = tx.QueryContext(ctx, "SELECT * FROM no_such_table")
		if err == nil {
			t.Fatal("tx.QueryContext: expected error for invalid SQL, got nil")
		}

		if delta := histogramSampleCount(t, name, "tx_query") - before; delta != 2 {
			t.Fatalf("tx_query histogram delta = %d, want 2", delta)
		}
	})

	// ---------------- InstrumentedTx.QueryRow -----------------------------
	t.Run("Tx_QueryRow", func(t *testing.T) {
		const name = "branchcov0722pm_tx_queryrow"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "tqr")

		tx := beginTx(t, idb)
		defer rollbackQuiet(tx)
		before := histogramSampleCount(t, name, "tx_query_row")

		var v string
		if err := tx.QueryRow("SELECT val FROM t WHERE id = ?", 1).Scan(&v); err != nil {
			t.Fatalf("tx.QueryRow.Scan: unexpected error: %v", err)
		}
		if v != "tqr" {
			t.Fatalf("tx.QueryRow: val = %q, want tqr", v)
		}

		// Error path: no matching row -> sql.ErrNoRows.
		var missing string
		err := tx.QueryRow("SELECT val FROM t WHERE id = ?", 9999).Scan(&missing)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("tx.QueryRow: expected sql.ErrNoRows, got %v", err)
		}

		if delta := histogramSampleCount(t, name, "tx_query_row") - before; delta != 2 {
			t.Fatalf("tx_query_row histogram delta = %d, want 2", delta)
		}
	})

	// ---------------- InstrumentedStmt.ExecContext ------------------------
	t.Run("Stmt_ExecContext", func(t *testing.T) {
		const name = "branchcov0722pm_stmt_execctx"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "")

		stmt, err := idb.Prepare("INSERT INTO t (val) VALUES (?)")
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		defer stmt.Close()
		before := histogramSampleCount(t, name, "stmt_exec")

		res, err := stmt.ExecContext(ctx, "x")
		if err != nil {
			t.Fatalf("stmt.ExecContext: unexpected error: %v", err)
		}
		if res == nil {
			t.Fatal("stmt.ExecContext: expected non-nil Result")
		}

		// Error path: the statement was prepared with one placeholder, so
		// calling it with no args is rejected by the driver at exec time.
		_, err = stmt.ExecContext(ctx)
		if err == nil {
			t.Fatal("stmt.ExecContext: expected error for wrong arg count, got nil")
		}

		if delta := histogramSampleCount(t, name, "stmt_exec") - before; delta != 2 {
			t.Fatalf("stmt_exec histogram delta = %d, want 2", delta)
		}
	})

	// ---------------- InstrumentedStmt.Query ------------------------------
	t.Run("Stmt_Query", func(t *testing.T) {
		const name = "branchcov0722pm_stmt_query"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "sq")

		stmt, err := idb.Prepare("SELECT val FROM t WHERE id = ?")
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		defer stmt.Close()
		before := histogramSampleCount(t, name, "stmt_query")

		rows, err := stmt.Query(1)
		if err != nil {
			t.Fatalf("stmt.Query: unexpected error: %v", err)
		}
		if !rows.Next() {
			rows.Close()
			t.Fatal("stmt.Query: expected one row")
		}
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			t.Fatalf("Scan: %v", err)
		}
		rows.Close()
		if v != "sq" {
			t.Fatalf("stmt.Query: val = %q, want sq", v)
		}

		// Error path: wrong arg count (prepared statement needs 1, given 0).
		r2, err := stmt.Query()
		if err == nil {
			if r2 != nil {
				r2.Close()
			}
			t.Fatal("stmt.Query: expected error for wrong arg count, got nil")
		}
		if r2 != nil {
			t.Fatalf("stmt.Query: expected nil rows on error, got non-nil")
		}

		if delta := histogramSampleCount(t, name, "stmt_query") - before; delta != 2 {
			t.Fatalf("stmt_query histogram delta = %d, want 2", delta)
		}
	})

	// ---------------- InstrumentedStmt.QueryRow ---------------------------
	t.Run("Stmt_QueryRow", func(t *testing.T) {
		const name = "branchcov0722pm_stmt_queryrow"
		idb := Wrap(openTestDB(t), name)
		setupTable(t, idb.DB, "sqr")

		stmt, err := idb.Prepare("SELECT val FROM t WHERE id = ?")
		if err != nil {
			t.Fatalf("Prepare: %v", err)
		}
		defer stmt.Close()
		before := histogramSampleCount(t, name, "stmt_query_row")

		// Success: Scan a real value.
		var v string
		if err := stmt.QueryRow(1).Scan(&v); err != nil {
			t.Fatalf("stmt.QueryRow.Scan: unexpected error: %v", err)
		}
		if v != "sqr" {
			t.Fatalf("stmt.QueryRow: val = %q, want sqr", v)
		}

		// Error path: no matching row -> sql.ErrNoRows from Scan. QueryRow
		// has no error-return arm itself; this is its real failure mode.
		var missing string
		err = stmt.QueryRow(9999).Scan(&missing)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("stmt.QueryRow: expected sql.ErrNoRows, got %v", err)
		}

		if delta := histogramSampleCount(t, name, "stmt_query_row") - before; delta != 2 {
			t.Fatalf("stmt_query_row histogram delta = %d, want 2", delta)
		}
	})
}

// setupTable creates table t(id INTEGER PRIMARY KEY, val TEXT) and, when val
// is non-empty, inserts one row with that val — the common precondition for
// the query/queryrow subtests.
func setupTable(t *testing.T, db *sql.DB, val string) {
	t.Helper()
	if _, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, val TEXT)"); err != nil {
		t.Fatalf("setup CREATE TABLE: %v", err)
	}
	if val != "" {
		if _, err := db.Exec("INSERT INTO t (val) VALUES (?)", val); err != nil {
			t.Fatalf("setup INSERT: %v", err)
		}
	}
}

// beginTx starts an InstrumentedTx (via the already-covered Begin) for the
// Tx/Stmt subtests. Kept separate from the BeginTx target test so those
// subtests do not depend on the function under test.
func beginTx(t *testing.T, idb *InstrumentedDB) *InstrumentedTx {
	t.Helper()
	tx, err := idb.Begin()
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	return tx
}

// rollbackQuiet rolls the tx back, ignoring the (benign) error that arises
// when the subtest already committed/rolled back. Used only in deferred
// cleanup; real assertions go through the wrappers under test.
func rollbackQuiet(tx *InstrumentedTx) {
	_ = tx.Rollback()
}
