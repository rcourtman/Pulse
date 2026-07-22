package audit

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	sqlite "modernc.org/sqlite"
)

// SQLiteLoggerConfig configures the SQLite audit logger.
type SQLiteLoggerConfig struct {
	DataDir       string          // Directory for audit.db
	CryptoMgr     CryptoEncryptor // For encrypting the signing key (optional)
	RetentionDays int             // Days to keep events (default: 90, 0 = forever)
}

// SQLiteLogger implements Logger with persistent SQLite storage and HMAC signing.
type SQLiteLogger struct {
	mu              sync.RWMutex
	db              *sql.DB
	dbPath          string
	signer          *Signer
	webhookDelivery *WebhookDelivery
	retentionDays   int
	stopChan        chan struct{}
	wg              sync.WaitGroup
	closeOnce       sync.Once
	closeErr        error
}

var retentionStartupCleanupDelay = 5 * time.Minute
var retentionStartupCleanup = func(l *SQLiteLogger) {
	l.cleanupOldEvents()
}

const (
	sqliteCodeBusy              = 5
	sqliteCodeLocked            = 6
	sqliteCodeReadonly          = 8
	sqliteCodeIOErr             = 10
	sqliteCodeCorrupt           = 11
	sqliteCodeCantOpen          = 14
	sqliteCodeNotADB            = 26
	sqliteCodeBusyRecovery      = sqliteCodeBusy | (1 << 8)
	sqliteCodeBusySnapshot      = sqliteCodeBusy | (2 << 8)
	sqliteCodeBusyTimeout       = sqliteCodeBusy | (3 << 8)
	sqliteCodeLockedSharedCache = sqliteCodeLocked | (1 << 8)
	sqliteCodeLockedVTab        = sqliteCodeLocked | (2 << 8)
)

var auditSQLiteRetryDelays = []time.Duration{25 * time.Millisecond, 75 * time.Millisecond}
var auditSQLiteRetrySleep = time.Sleep

// IsStoreBusyError reports whether an audit store error is a transient SQLite lock.
func IsStoreBusyError(err error) bool {
	if err == nil {
		return false
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqliteCodeBusy,
			sqliteCodeLocked,
			sqliteCodeBusyRecovery,
			sqliteCodeBusySnapshot,
			sqliteCodeBusyTimeout,
			sqliteCodeLockedSharedCache,
			sqliteCodeLockedVTab:
			return true
		}
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "sqlite_busy") ||
		strings.Contains(message, "sqlite_locked") ||
		strings.Contains(message, "database is locked") ||
		strings.Contains(message, "database table is locked")
}

// IsStoreUnavailableError reports whether an audit store error means the
// backing database is missing, corrupt, unreadable, or not initialized.
func IsStoreUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	if IsStoreBusyError(err) {
		return false
	}

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqliteCodeReadonly, sqliteCodeIOErr, sqliteCodeCorrupt, sqliteCodeCantOpen, sqliteCodeNotADB:
			return true
		}
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "database disk image is malformed") ||
		strings.Contains(message, "file is not a database") ||
		strings.Contains(message, "unable to open database file") ||
		strings.Contains(message, "attempt to write a readonly database") ||
		strings.Contains(message, "no such table: audit_events") ||
		strings.Contains(message, "no such table: schema_version")
}

func withSQLiteRetry(operation string, run func() error) error {
	var err error
	for attempt := 0; ; attempt++ {
		err = run()
		if err == nil || !IsStoreBusyError(err) || attempt >= len(auditSQLiteRetryDelays) {
			return err
		}

		delay := auditSQLiteRetryDelays[attempt]
		log.Warn().
			Err(err).
			Str("operation", operation).
			Int("attempt", attempt+1).
			Dur("retryIn", delay).
			Msg("Audit SQLite store busy; retrying")
		auditSQLiteRetrySleep(delay)
	}
}

// NewSQLiteLogger creates a new SQLite-backed audit logger.
func NewSQLiteLogger(cfg SQLiteLoggerConfig) (*SQLiteLogger, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}

	// Ensure directory exists
	auditDir := filepath.Join(cfg.DataDir, "audit")
	if err := os.MkdirAll(auditDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create audit directory: %w", err)
	}

	dbPath := filepath.Join(auditDir, "audit.db")

	// Open database with pragmas in DSN so every pool connection is configured
	dsn := dbPath + "?" + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
			"cache_size(-64000)",
		},
	}.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit database: %w", err)
	}

	// SQLite works best with a single writer connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Initialize signer
	signer, err := NewSigner(auditDir, cfg.CryptoMgr)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize audit signer: %w", err)
	}

	retentionDays := cfg.RetentionDays
	if retentionDays == 0 {
		retentionDays = 90 // Default
	}

	l := &SQLiteLogger{
		db:            db,
		dbPath:        dbPath,
		signer:        signer,
		retentionDays: retentionDays,
		stopChan:      make(chan struct{}),
	}

	// Initialize schema
	if err := l.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	l.migrateAutoVacuum()

	// Load webhook URLs from config table
	urls := l.loadWebhookURLs()
	if len(urls) > 0 {
		l.webhookDelivery = NewWebhookDelivery(urls)
		l.webhookDelivery.Start()
	}

	// Start retention worker if retention is enabled
	if retentionDays > 0 {
		l.wg.Add(1)
		go l.retentionWorker()
	}

	log.Info().
		Str("dbPath", dbPath).
		Int("retentionDays", retentionDays).
		Bool("signingEnabled", signer.SigningEnabled()).
		Msg("SQLite audit logger initialized")

	return l, nil
}

// IsPersistentAuditLogger reports that SQLite provides queryable audit storage.
func (l *SQLiteLogger) IsPersistentAuditLogger() bool {
	return l != nil
}

// initSchema creates the database tables and runs migrations.
func (l *SQLiteLogger) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS audit_events (
		id TEXT PRIMARY KEY,
		timestamp INTEGER NOT NULL,
		event_type TEXT NOT NULL,
		user TEXT,
		ip TEXT,
		path TEXT,
		success INTEGER NOT NULL,
		details TEXT,
		signature TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_event_type ON audit_events(event_type);
	CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_events(user) WHERE user != '';
	CREATE INDEX IF NOT EXISTS idx_audit_success ON audit_events(success);

	CREATE TABLE IF NOT EXISTS audit_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	);
	`

	var err error
	err = withSQLiteRetry("init_schema", func() error {
		_, err = l.db.Exec(schema)
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Record schema version
	err = withSQLiteRetry("record_schema_version", func() error {
		_, err = l.db.Exec(`INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (1, ?)`,
			time.Now().Unix())
		return err
	})
	return err
}

// Log records an audit event with HMAC signature.
func (l *SQLiteLogger) Record(event Event) error {
	// Sign the event
	event.Signature = l.signer.Sign(event)

	// Insert into database
	success := 0
	if event.Success {
		success = 1
	}

	var err error
	err = withSQLiteRetry("insert_audit_event", func() error {
		l.mu.Lock()
		defer l.mu.Unlock()

		_, err = l.db.Exec(`
			INSERT INTO audit_events (id, timestamp, event_type, user, ip, path, success, details, signature)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			event.ID,
			event.Timestamp.Unix(),
			event.EventType,
			event.User,
			event.IP,
			event.Path,
			success,
			event.Details,
			event.Signature,
		)
		return err
	})

	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}

	// Also log to zerolog for real-time visibility
	logContext := log.With().
		Str("audit_id", event.ID).
		Str("event", event.EventType).
		Time("timestamp", event.Timestamp)
	logContext = appendRealtimeAuditIdentityFields(logContext, event)
	logEvent := appendRealtimeAuditDetailFields(logContext, event.Details).Logger()

	if event.Success {
		logEvent.Info().Msg("Audit event")
	} else {
		logEvent.Warn().Msg("Audit event - FAILED")
	}

	// Send to webhooks if configured
	if l.webhookDelivery != nil {
		l.webhookDelivery.Enqueue(event)
	}

	return nil
}

// Query retrieves audit events matching the filter.
func (l *SQLiteLogger) Query(filter QueryFilter) ([]Event, error) {
	var events []Event
	err := withSQLiteRetry("query_audit_events", func() error {
		l.mu.RLock()
		defer l.mu.RUnlock()

		result, err := l.queryLocked(filter)
		if err != nil {
			return err
		}
		events = result
		return nil
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (l *SQLiteLogger) queryLocked(filter QueryFilter) ([]Event, error) {
	query := "SELECT id, timestamp, event_type, user, ip, path, success, details, signature FROM audit_events WHERE 1=1"
	args := []interface{}{}

	if filter.ID != "" {
		query += " AND id = ?"
		args = append(args, filter.ID)
	}
	if filter.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime.Unix())
	}
	if filter.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime.Unix())
	}
	if filter.EventType != "" {
		query += " AND event_type = ?"
		args = append(args, filter.EventType)
	}
	if filter.User != "" {
		query += " AND user = ?"
		args = append(args, filter.User)
	}
	if filter.Success != nil {
		success := 0
		if *filter.Success {
			success = 1
		}
		query += " AND success = ?"
		args = append(args, success)
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		// SQLite requires LIMIT when OFFSET is present.
		if filter.Limit <= 0 {
			query += " LIMIT -1"
		}
		query += " OFFSET ?"
		args = append(args, filter.Offset)
	}

	rows, err := l.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit events: %w", err)
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var timestampValue any
		var success int
		var user, ip, path, details, signature sql.NullString

		err := rows.Scan(&e.ID, &timestampValue, &e.EventType, &user, &ip, &path, &success, &details, &signature)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}
		timestamp, err := parseAuditTimestamp(timestampValue)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}

		e.Timestamp = timestamp
		e.Success = success == 1
		e.User = user.String
		e.IP = ip.String
		e.Path = path.String
		e.Details = details.String
		e.Signature = signature.String

		events = append(events, e)
	}

	return events, rows.Err()
}

func parseAuditTimestamp(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case int64:
		return time.Unix(v, 0), nil
	case int:
		return time.Unix(int64(v), 0), nil
	case int32:
		return time.Unix(int64(v), 0), nil
	case float64:
		return time.Unix(int64(v), 0), nil
	case []byte:
		return parseAuditTimestampString(string(v))
	case string:
		return parseAuditTimestampString(v)
	case nil:
		return time.Time{}, errors.New("timestamp is null")
	default:
		return time.Time{}, fmt.Errorf("unsupported timestamp type %T", value)
	}
}

func parseAuditTimestampString(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, errors.New("timestamp is empty")
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(unix, 0), nil
	}
	if monotonicIndex := strings.LastIndex(raw, " m="); monotonicIndex > 0 && monotonicIndex+3 < len(raw) {
		sign := raw[monotonicIndex+3]
		if sign == '+' || sign == '-' {
			raw = strings.TrimSpace(raw[:monotonicIndex])
		}
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if timestamp, err := time.Parse(layout, raw); err == nil {
			return timestamp, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported timestamp value %q", raw)
}

// Count returns the number of events matching the filter.
func (l *SQLiteLogger) Count(filter QueryFilter) (int, error) {
	var count int
	err := withSQLiteRetry("count_audit_events", func() error {
		l.mu.RLock()
		defer l.mu.RUnlock()

		result, err := l.countLocked(filter)
		if err != nil {
			return err
		}
		count = result
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (l *SQLiteLogger) countLocked(filter QueryFilter) (int, error) {
	query := "SELECT COUNT(*) FROM audit_events WHERE 1=1"
	args := []interface{}{}

	if filter.ID != "" {
		query += " AND id = ?"
		args = append(args, filter.ID)
	}
	if filter.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartTime.Unix())
	}
	if filter.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndTime.Unix())
	}
	if filter.EventType != "" {
		query += " AND event_type = ?"
		args = append(args, filter.EventType)
	}
	if filter.User != "" {
		query += " AND user = ?"
		args = append(args, filter.User)
	}
	if filter.Success != nil {
		success := 0
		if *filter.Success {
			success = 1
		}
		query += " AND success = ?"
		args = append(args, success)
	}

	var count int
	err := l.db.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count audit events: %w", err)
	}

	return count, nil
}

// GetWebhookURLs returns the configured webhook URLs.
func (l *SQLiteLogger) GetWebhookURLs() []string {
	if l.webhookDelivery != nil {
		return l.webhookDelivery.GetURLs()
	}
	return l.loadWebhookURLs()
}

// UpdateWebhookURLs updates the webhook configuration.
func (l *SQLiteLogger) UpdateWebhookURLs(urls []string) error {
	// Save to config table
	value := strings.Join(urls, ",")
	var err error
	err = withSQLiteRetry("update_webhook_urls", func() error {
		l.mu.Lock()
		defer l.mu.Unlock()

		_, err = l.db.Exec(`
			INSERT INTO audit_config (key, value, updated_at) VALUES ('webhook_urls', ?, ?)
			ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			value, time.Now().Unix())
		return err
	})
	if err != nil {
		return fmt.Errorf("failed to save webhook URLs: %w", err)
	}

	// Update delivery worker
	if len(urls) > 0 {
		if l.webhookDelivery == nil {
			l.webhookDelivery = NewWebhookDelivery(urls)
			l.webhookDelivery.Start()
		} else {
			l.webhookDelivery.UpdateURLs(urls)
		}
	} else if l.webhookDelivery != nil {
		l.webhookDelivery.Stop()
		l.webhookDelivery = nil
	}

	return nil
}

// VerifySignature checks if an event's signature is valid.
func (l *SQLiteLogger) VerifySignature(event Event) bool {
	return l.signer.Verify(event)
}

// Close gracefully shuts down the logger.
func (l *SQLiteLogger) Close() error {
	l.closeOnce.Do(func() {
		close(l.stopChan)

		if l.webhookDelivery != nil {
			l.webhookDelivery.Stop()
		}

		l.wg.Wait()

		if err := l.db.Close(); err != nil {
			l.closeErr = fmt.Errorf("failed to close audit database: %w", err)
			return
		}

		log.Info().Msg("SQLite audit logger closed")
	})

	return l.closeErr
}

// loadWebhookURLs loads webhook URLs from the config table.
func (l *SQLiteLogger) loadWebhookURLs() []string {
	var value string
	err := l.db.QueryRow(`SELECT value FROM audit_config WHERE key = 'webhook_urls'`).Scan(&value)
	if err != nil || value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

// retentionWorker runs periodically to clean up old events.
func (l *SQLiteLogger) retentionWorker() {
	defer l.wg.Done()

	// Run at 3 AM daily
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Also run once at startup after a short delay. Keep timer stoppable
	// so Close() can cancel it and avoid cleanup callbacks after shutdown.
	startupTimer := time.NewTimer(retentionStartupCleanupDelay)
	startupTimerCh := startupTimer.C
	defer func() {
		if !startupTimer.Stop() {
			select {
			case <-startupTimerCh:
			default:
			}
		}
	}()

	for {
		select {
		case <-l.stopChan:
			return
		case <-startupTimerCh:
			retentionStartupCleanup(l)
			startupTimerCh = nil
		case <-ticker.C:
			l.cleanupOldEvents()
		}
	}
}

// maxAuditReclaimPages caps how many freelist pages a single retention cycle
// returns to the OS, so a large backlog drains over successive daily cycles
// instead of holding the write lock in one long incremental_vacuum.
const maxAuditReclaimPages = 50000

// migrateAutoVacuum ensures the database uses incremental auto-vacuum so
// retention deletes can return pages to the OS. Without it the deletes free
// pages internally but audit.db never shrinks, the same bloat class fixed for
// unified_resources.db under #1496. Set once at startup rather than in the
// DSN: as a per-connection pragma it replays as a header write on every new
// pool connection (#1601).
func (l *SQLiteLogger) migrateAutoVacuum() {
	var mode int
	if err := l.db.QueryRow("PRAGMA auto_vacuum").Scan(&mode); err != nil {
		log.Error().Err(err).Msg("Failed to check audit auto_vacuum mode")
		return
	}
	if mode == 2 {
		return
	}

	log.Info().Msg("Converting audit database to incremental auto-vacuum (one-time migration)")
	start := time.Now()

	if _, err := l.db.Exec("PRAGMA auto_vacuum = INCREMENTAL"); err != nil {
		log.Error().Err(err).Msg("Failed to set audit auto_vacuum mode")
		return
	}
	if _, err := l.db.Exec("VACUUM"); err != nil {
		log.Error().Err(err).Msg("Audit auto-vacuum migration VACUUM failed (will retry next restart)")
		return
	}

	log.Info().Dur("took", time.Since(start)).Msg("Audit auto-vacuum migration complete")
}

// reclaimFreePages returns freed pages to the OS after retention deletes.
// Callers must hold l.mu.
func (l *SQLiteLogger) reclaimFreePages() {
	var freelist int64
	if err := l.db.QueryRow(`PRAGMA freelist_count`).Scan(&freelist); err != nil {
		log.Error().Err(err).Msg("Failed to read audit freelist_count")
		return
	}
	if freelist == 0 {
		return
	}
	pages := freelist
	if pages > maxAuditReclaimPages {
		pages = maxAuditReclaimPages
	}
	if _, err := l.db.Exec(fmt.Sprintf(`PRAGMA incremental_vacuum(%d)`, pages)); err != nil {
		log.Error().Err(err).Msg("Audit incremental vacuum failed")
		return
	}
	if _, err := l.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		log.Error().Err(err).Msg("Audit WAL checkpoint failed")
	}
}

// cleanupOldEvents deletes events older than the retention period.
func (l *SQLiteLogger) cleanupOldEvents() {
	if l.retentionDays <= 0 {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -l.retentionDays).Unix()

	result, err := l.db.Exec(`DELETE FROM audit_events WHERE timestamp < ?`, cutoff)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup old audit events")
		return
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		log.Info().
			Int64("deleted", deleted).
			Int("retentionDays", l.retentionDays).
			Msg("Cleaned up old audit events")

		// Log the cleanup as an audit event (without recursion - direct insert)
		_, _ = l.db.Exec(`
			INSERT INTO audit_events (id, timestamp, event_type, user, ip, path, success, details, signature)
			VALUES (?, ?, 'audit_cleanup', 'system', '', '', 1, ?, '')`,
			fmt.Sprintf("cleanup-%d", time.Now().Unix()),
			time.Now().Unix(),
			fmt.Sprintf("Deleted %d events older than %d days", deleted, l.retentionDays),
		)
	}

	l.reclaimFreePages()
}

// GetRetentionDays returns the current retention period.
func (l *SQLiteLogger) GetRetentionDays() int {
	return l.retentionDays
}

// SetRetentionDays updates the retention period.
func (l *SQLiteLogger) SetRetentionDays(days int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.retentionDays = days

	// Save to config
	_, _ = l.db.Exec(`
		INSERT INTO audit_config (key, value, updated_at) VALUES ('retention_days', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		fmt.Sprintf("%d", days), time.Now().Unix())
}
