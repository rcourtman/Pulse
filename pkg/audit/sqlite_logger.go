package audit

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
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

	_, err := l.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Record schema version
	_, err = l.db.Exec(`INSERT OR IGNORE INTO schema_version (version, applied_at) VALUES (1, ?)`,
		time.Now().Unix())
	return err
}

// Log records an audit event with HMAC signature.
func (l *SQLiteLogger) Log(event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Sign the event
	event.Signature = l.signer.Sign(event)

	// Insert into database
	success := 0
	if event.Success {
		success = 1
	}

	_, err := l.db.Exec(`
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

	if err != nil {
		return fmt.Errorf("failed to insert audit event: %w", err)
	}

	// Also log to zerolog for real-time visibility
	logEvent := log.With().
		Str("audit_id", event.ID).
		Str("event", event.EventType).
		Str("user", event.User).
		Str("ip", event.IP).
		Str("path", event.Path).
		Time("timestamp", event.Timestamp).
		Str("details", event.Details).
		Logger()

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
	l.mu.RLock()
	defer l.mu.RUnlock()

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
		var timestamp int64
		var success int
		var user, ip, path, details, signature sql.NullString

		err := rows.Scan(&e.ID, &timestamp, &e.EventType, &user, &ip, &path, &success, &details, &signature)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit event: %w", err)
		}

		e.Timestamp = time.Unix(timestamp, 0)
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

// Count returns the number of events matching the filter.
func (l *SQLiteLogger) Count(filter QueryFilter) (int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

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
	l.mu.Lock()
	defer l.mu.Unlock()

	// Save to config table
	value := strings.Join(urls, ",")
	_, err := l.db.Exec(`
		INSERT INTO audit_config (key, value, updated_at) VALUES ('webhook_urls', ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		value, time.Now().Unix())
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
	close(l.stopChan)

	if l.webhookDelivery != nil {
		l.webhookDelivery.Stop()
	}

	l.wg.Wait()

	if err := l.db.Close(); err != nil {
		return fmt.Errorf("failed to close audit database: %w", err)
	}

	log.Info().Msg("SQLite audit logger closed")
	return nil
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

	// Also run once at startup after a short delay
	time.AfterFunc(5*time.Minute, func() {
		l.cleanupOldEvents()
	})

	for {
		select {
		case <-l.stopChan:
			return
		case <-ticker.C:
			l.cleanupOldEvents()
		}
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
