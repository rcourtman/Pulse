// Package db provides instrumented database wrappers for slow query
// detection, logging, and Prometheus metrics.
package db

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

// SlowQueryThreshold is the default duration above which a query is
// considered slow and logged at WARN level.
const SlowQueryThreshold = 100 * time.Millisecond

var metricsOnce sync.Once

var (
	dbQueryDuration *prometheus.HistogramVec
	dbSlowQueries   *prometheus.CounterVec
)

func ensureMetrics() {
	metricsOnce.Do(func() {
		dbQueryDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "pulse",
				Subsystem: "db",
				Name:      "query_duration_seconds",
				Help:      "Database query execution time in seconds.",
				Buckets:   []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 5, 10},
			},
			[]string{"database", "operation"},
		)

		dbSlowQueries = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "pulse",
				Subsystem: "db",
				Name:      "slow_queries_total",
				Help:      "Total number of database queries exceeding the slow query threshold.",
			},
			[]string{"database", "operation"},
		)

		prometheus.MustRegister(dbQueryDuration, dbSlowQueries)
	})
}

// observe records duration metrics and logs slow queries.
// Safe to call even if Wrap was bypassed — metrics init lazily.
func observe(name, op, query string, start time.Time) {
	ensureMetrics()

	elapsed := time.Since(start)
	dbQueryDuration.WithLabelValues(name, op).Observe(elapsed.Seconds())

	if elapsed >= SlowQueryThreshold {
		dbSlowQueries.WithLabelValues(name, op).Inc()

		// Truncate the query for logging to avoid huge log lines.
		logQuery := query
		if len(logQuery) > 200 {
			logQuery = logQuery[:200] + "..."
		}

		log.Warn().
			Str("database", name).
			Str("operation", op).
			Dur("duration", elapsed).
			Str("query", logQuery).
			Msg("Slow database query")
	}
}

// InstrumentedDB wraps a *sql.DB and logs queries that exceed
// SlowQueryThreshold. It also records Prometheus histogram/counter
// metrics for every operation.
type InstrumentedDB struct {
	DB   *sql.DB
	name string // identifies this database in logs/metrics (e.g. "metrics", "audit")
}

// Wrap returns an InstrumentedDB that wraps db with the given logical name.
// The name appears in log messages and as the "database" Prometheus label.
func Wrap(db *sql.DB, name string) *InstrumentedDB {
	ensureMetrics()
	return &InstrumentedDB{DB: db, name: name}
}

// Exec executes a query without returning any rows.
func (d *InstrumentedDB) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := d.DB.Exec(query, args...)
	observe(d.name, "exec", query, start)
	return result, err
}

// ExecContext executes a query without returning any rows.
func (d *InstrumentedDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := d.DB.ExecContext(ctx, query, args...)
	observe(d.name, "exec", query, start)
	return result, err
}

// Query executes a query that returns rows.
func (d *InstrumentedDB) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.DB.Query(query, args...)
	observe(d.name, "query", query, start)
	return rows, err
}

// QueryContext executes a query that returns rows.
func (d *InstrumentedDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := d.DB.QueryContext(ctx, query, args...)
	observe(d.name, "query", query, start)
	return rows, err
}

// QueryRow executes a query that returns at most one row.
func (d *InstrumentedDB) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.DB.QueryRow(query, args...)
	observe(d.name, "query_row", query, start)
	return row
}

// QueryRowContext executes a query that returns at most one row.
func (d *InstrumentedDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	start := time.Now()
	row := d.DB.QueryRowContext(ctx, query, args...)
	observe(d.name, "query_row", query, start)
	return row
}

// InstrumentedTx wraps a *sql.Tx with slow query instrumentation.
type InstrumentedTx struct {
	Tx   *sql.Tx
	name string
}

// Begin starts a transaction and returns an InstrumentedTx.
func (d *InstrumentedDB) Begin() (*InstrumentedTx, error) {
	start := time.Now()
	tx, err := d.DB.Begin()
	observe(d.name, "begin", "BEGIN", start)
	if err != nil {
		return nil, err
	}
	return &InstrumentedTx{Tx: tx, name: d.name}, nil
}

// BeginTx starts a transaction and returns an InstrumentedTx.
func (d *InstrumentedDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*InstrumentedTx, error) {
	start := time.Now()
	tx, err := d.DB.BeginTx(ctx, opts)
	observe(d.name, "begin", "BEGIN", start)
	if err != nil {
		return nil, err
	}
	return &InstrumentedTx{Tx: tx, name: d.name}, nil
}

// Prepare creates an instrumented prepared statement on the underlying DB.
func (d *InstrumentedDB) Prepare(query string) (*InstrumentedStmt, error) {
	stmt, err := d.DB.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &InstrumentedStmt{Stmt: stmt, name: d.name, query: query}, nil
}

// Close closes the underlying database.
func (d *InstrumentedDB) Close() error {
	return d.DB.Close()
}

// SetMaxOpenConns delegates to the underlying DB.
func (d *InstrumentedDB) SetMaxOpenConns(n int) {
	d.DB.SetMaxOpenConns(n)
}

// SetMaxIdleConns delegates to the underlying DB.
func (d *InstrumentedDB) SetMaxIdleConns(n int) {
	d.DB.SetMaxIdleConns(n)
}

// SetConnMaxLifetime delegates to the underlying DB.
func (d *InstrumentedDB) SetConnMaxLifetime(dur time.Duration) {
	d.DB.SetConnMaxLifetime(dur)
}

// Exec executes a query within the transaction.
func (t *InstrumentedTx) Exec(query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := t.Tx.Exec(query, args...)
	observe(t.name, "tx_exec", query, start)
	return result, err
}

// ExecContext executes a query within the transaction.
func (t *InstrumentedTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := t.Tx.ExecContext(ctx, query, args...)
	observe(t.name, "tx_exec", query, start)
	return result, err
}

// Query executes a query within the transaction.
func (t *InstrumentedTx) Query(query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := t.Tx.Query(query, args...)
	observe(t.name, "tx_query", query, start)
	return rows, err
}

// QueryContext executes a query within the transaction.
func (t *InstrumentedTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := t.Tx.QueryContext(ctx, query, args...)
	observe(t.name, "tx_query", query, start)
	return rows, err
}

// QueryRow executes a query within the transaction that returns at most one row.
func (t *InstrumentedTx) QueryRow(query string, args ...any) *sql.Row {
	start := time.Now()
	row := t.Tx.QueryRow(query, args...)
	observe(t.name, "tx_query_row", query, start)
	return row
}

// Prepare creates an instrumented prepared statement within the transaction.
func (t *InstrumentedTx) Prepare(query string) (*InstrumentedStmt, error) {
	stmt, err := t.Tx.Prepare(query)
	if err != nil {
		return nil, err
	}
	return &InstrumentedStmt{Stmt: stmt, name: t.name, query: query}, nil
}

// Commit commits the transaction.
func (t *InstrumentedTx) Commit() error {
	start := time.Now()
	err := t.Tx.Commit()
	observe(t.name, "commit", "COMMIT", start)
	return err
}

// Rollback aborts the transaction.
func (t *InstrumentedTx) Rollback() error {
	return t.Tx.Rollback()
}

// InstrumentedStmt wraps a *sql.Stmt with slow query instrumentation.
type InstrumentedStmt struct {
	Stmt  *sql.Stmt
	name  string
	query string // original query text for logging
}

// Exec executes the prepared statement.
func (s *InstrumentedStmt) Exec(args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := s.Stmt.Exec(args...)
	observe(s.name, "stmt_exec", s.query, start)
	return result, err
}

// ExecContext executes the prepared statement.
func (s *InstrumentedStmt) ExecContext(ctx context.Context, args ...any) (sql.Result, error) {
	start := time.Now()
	result, err := s.Stmt.ExecContext(ctx, args...)
	observe(s.name, "stmt_exec", s.query, start)
	return result, err
}

// Query executes the prepared statement.
func (s *InstrumentedStmt) Query(args ...any) (*sql.Rows, error) {
	start := time.Now()
	rows, err := s.Stmt.Query(args...)
	observe(s.name, "stmt_query", s.query, start)
	return rows, err
}

// QueryRow executes the prepared statement.
func (s *InstrumentedStmt) QueryRow(args ...any) *sql.Row {
	start := time.Now()
	row := s.Stmt.QueryRow(args...)
	observe(s.name, "stmt_query_row", s.query, start)
	return row
}

// Close closes the prepared statement.
func (s *InstrumentedStmt) Close() error {
	return s.Stmt.Close()
}
