package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// NotificationQueueStatus represents the status of a queued notification
type NotificationQueueStatus string

const (
	QueueStatusPending   NotificationQueueStatus = "pending"
	QueueStatusSending   NotificationQueueStatus = "sending"
	QueueStatusSent      NotificationQueueStatus = "sent"
	QueueStatusFailed    NotificationQueueStatus = "failed"
	QueueStatusDLQ       NotificationQueueStatus = "dlq"
	QueueStatusCancelled NotificationQueueStatus = "cancelled"
)

// QueuedNotification represents a notification in the persistent queue
type QueuedNotification struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"` // email, webhook, apprise
	Method       string                  `json:"method,omitempty"`
	Status       NotificationQueueStatus `json:"status"`
	Alerts       []*alerts.Alert         `json:"alerts"`
	Config       json.RawMessage         `json:"config"` // EmailConfig, WebhookConfig, or AppriseConfig
	Attempts     int                     `json:"attempts"`
	MaxAttempts  int                     `json:"maxAttempts"`
	LastAttempt  *time.Time              `json:"lastAttempt,omitempty"`
	LastError    *string                 `json:"lastError,omitempty"`
	CreatedAt    time.Time               `json:"createdAt"`
	NextRetryAt  *time.Time              `json:"nextRetryAt,omitempty"`
	CompletedAt  *time.Time              `json:"completedAt,omitempty"`
	PayloadBytes *int                    `json:"payloadBytes,omitempty"`
}

// NotificationQueue manages persistent notification delivery with retries and DLQ
type NotificationQueue struct {
	mu              sync.RWMutex
	stopOnce        sync.Once
	stopErr         error
	db              *sql.DB
	dbPath          string
	stopChan        chan struct{}
	wg              sync.WaitGroup
	processorTicker *time.Ticker
	cleanupTicker   *time.Ticker
	notifyChan      chan struct{}                   // Signal when new notifications are added
	processor       func(*QueuedNotification) error // Notification processor function
	workerSem       chan struct{}                   // Semaphore for limiting concurrent workers
}

// NewNotificationQueue creates a new persistent notification queue
func NewNotificationQueue(dataDir string) (*NotificationQueue, error) {
	if dataDir == "" {
		dataDir = filepath.Join(utils.GetDataDir(), "notifications")
	}

	// Ensure directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create notification queue directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "notification_queue.db")

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
		return nil, fmt.Errorf("failed to open notification queue database: %w", err)
	}

	// SQLite works best with a single writer connection
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	nq := &NotificationQueue{
		db:              db,
		dbPath:          dbPath,
		stopChan:        make(chan struct{}),
		processorTicker: time.NewTicker(5 * time.Second),
		cleanupTicker:   time.NewTicker(1 * time.Hour),
		notifyChan:      make(chan struct{}, 100),
		workerSem:       make(chan struct{}, 5), // Allow 5 concurrent workers
	}

	if err := nq.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	// Reset any stuck "sending" items to "pending" (crash recovery)
	if _, err := nq.db.Exec(`UPDATE notification_queue SET status = 'pending' WHERE status = 'sending'`); err != nil {
		log.Error().Err(err).Msg("Failed to recover stuck sending notifications")
	}

	// Start background processors
	nq.wg.Add(2)
	go nq.processQueue()
	go nq.cleanupOldEntries()

	log.Info().
		Str("dbPath", dbPath).
		Msg("Notification queue initialized")

	return nq, nil
}

// initSchema creates the database tables
func (nq *NotificationQueue) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS notification_queue (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		method TEXT,
		status TEXT NOT NULL,
		alerts TEXT NOT NULL,
		config TEXT NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 0,
		max_attempts INTEGER NOT NULL DEFAULT 3,
		last_attempt INTEGER,
		last_error TEXT,
		created_at INTEGER NOT NULL,
		next_retry_at INTEGER,
		completed_at INTEGER,
		payload_bytes INTEGER
	);

	CREATE INDEX IF NOT EXISTS idx_status ON notification_queue(status);
	CREATE INDEX IF NOT EXISTS idx_next_retry ON notification_queue(next_retry_at) WHERE status = 'pending';
	CREATE INDEX IF NOT EXISTS idx_created_at ON notification_queue(created_at);

	CREATE TABLE IF NOT EXISTS notification_audit (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		notification_id TEXT NOT NULL,
		type TEXT NOT NULL,
		method TEXT,
		status TEXT NOT NULL,
		alert_ids TEXT,
		alert_count INTEGER,
		attempts INTEGER,
		success BOOLEAN,
		error_message TEXT,
		payload_size INTEGER,
		timestamp INTEGER NOT NULL,
		FOREIGN KEY (notification_id) REFERENCES notification_queue(id)
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON notification_audit(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_notification_id ON notification_audit(notification_id);
	CREATE INDEX IF NOT EXISTS idx_audit_status ON notification_audit(status);
	`

	_, err := nq.db.Exec(schema)
	return err
}

// Enqueue adds a notification to the queue
func (nq *NotificationQueue) Enqueue(notif *QueuedNotification) error {
	if notif.ID == "" {
		notif.ID = fmt.Sprintf("%s-%d", notif.Type, time.Now().UnixNano())
	}
	if notif.Status == "" {
		notif.Status = QueueStatusPending
	}
	if notif.MaxAttempts == 0 {
		notif.MaxAttempts = 3
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}

	alertsJSON, err := json.Marshal(notif.Alerts)
	if err != nil {
		return fmt.Errorf("failed to marshal alerts: %w", err)
	}

	nq.mu.Lock()
	defer nq.mu.Unlock()

	query := `
		INSERT INTO notification_queue
		(id, type, method, status, alerts, config, attempts, max_attempts, created_at, next_retry_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	var nextRetryAt *int64
	if notif.NextRetryAt != nil {
		ts := notif.NextRetryAt.Unix()
		nextRetryAt = &ts
	}

	_, err = nq.db.Exec(query,
		notif.ID,
		notif.Type,
		notif.Method,
		notif.Status,
		string(alertsJSON),
		string(notif.Config),
		notif.Attempts,
		notif.MaxAttempts,
		notif.CreatedAt.Unix(),
		nextRetryAt,
	)

	if err != nil {
		return fmt.Errorf("failed to enqueue notification: %w", err)
	}

	log.Debug().
		Str("id", notif.ID).
		Str("type", notif.Type).
		Int("alertCount", len(notif.Alerts)).
		Msg("Notification enqueued")

	// Signal processor that new work is available
	select {
	case nq.notifyChan <- struct{}{}:
	default:
	}

	return nil
}

// UpdateStatus updates the status of a queued notification without incrementing attempts
func (nq *NotificationQueue) UpdateStatus(id string, status NotificationQueueStatus, errorMsg string) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	now := time.Now().Unix()
	var completedAt *int64
	if status == QueueStatusSent || status == QueueStatusFailed || status == QueueStatusDLQ || status == QueueStatusCancelled {
		completedAt = &now
	}

	query := `
		UPDATE notification_queue
		SET status = ?, last_attempt = ?, last_error = ?, completed_at = ?
		WHERE id = ?
	`

	result, err := nq.db.Exec(query, status, now, errorMsg, completedAt, id)
	if err != nil {
		return fmt.Errorf("failed to update notification status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}

	return nil
}

// IncrementAttempt increments the attempt counter for a notification
func (nq *NotificationQueue) IncrementAttempt(id string) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	query := `UPDATE notification_queue SET attempts = attempts + 1 WHERE id = ?`
	_, err := nq.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to increment attempt counter: %w", err)
	}
	return nil
}

// IncrementAttemptAndSetStatus atomically increments attempt counter and sets status in a single operation
func (nq *NotificationQueue) IncrementAttemptAndSetStatus(id string, status NotificationQueueStatus) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	query := `
		UPDATE notification_queue
		SET attempts = attempts + 1,
		    status = ?,
		    last_attempt = ?
		WHERE id = ?
	`
	_, err := nq.db.Exec(query, status, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("failed to increment attempt and set status: %w", err)
	}
	return nil
}

// GetPending returns notifications ready for processing
func (nq *NotificationQueue) GetPending(limit int) ([]*QueuedNotification, error) {
	nq.mu.RLock()
	defer nq.mu.RUnlock()

	query := `
		SELECT id, type, method, status, alerts, config, attempts, max_attempts,
		       last_attempt, last_error, created_at, next_retry_at, completed_at, payload_bytes
		FROM notification_queue
		WHERE status = 'pending'
		  AND (next_retry_at IS NULL OR next_retry_at <= ?)
		ORDER BY created_at ASC
		LIMIT ?
	`

	rows, err := nq.db.Query(query, time.Now().Unix(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending notifications: %w", err)
	}
	defer rows.Close()

	var notifications []*QueuedNotification
	for rows.Next() {
		notif, err := nq.scanNotification(rows)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan notification row")
			continue
		}
		notifications = append(notifications, notif)
	}

	return notifications, rows.Err()
}

// scanNotification scans a database row into a QueuedNotification
func (nq *NotificationQueue) scanNotification(rows *sql.Rows) (*QueuedNotification, error) {
	var notif QueuedNotification
	var alertsJSON, configJSON string
	var lastAttempt, nextRetryAt, completedAt *int64
	var createdAtUnix int64

	err := rows.Scan(
		&notif.ID,
		&notif.Type,
		&notif.Method,
		&notif.Status,
		&alertsJSON,
		&configJSON,
		&notif.Attempts,
		&notif.MaxAttempts,
		&lastAttempt,
		&notif.LastError,
		&createdAtUnix,
		&nextRetryAt,
		&completedAt,
		&notif.PayloadBytes,
	)
	if err != nil {
		return nil, err
	}

	notif.CreatedAt = time.Unix(createdAtUnix, 0)

	if lastAttempt != nil {
		t := time.Unix(*lastAttempt, 0)
		notif.LastAttempt = &t
	}
	if nextRetryAt != nil {
		t := time.Unix(*nextRetryAt, 0)
		notif.NextRetryAt = &t
	}
	if completedAt != nil {
		t := time.Unix(*completedAt, 0)
		notif.CompletedAt = &t
	}

	if err := json.Unmarshal([]byte(alertsJSON), &notif.Alerts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alerts: %w", err)
	}

	notif.Config = json.RawMessage(configJSON)

	return &notif, nil
}

// ScheduleRetry schedules a notification for retry with exponential backoff
func (nq *NotificationQueue) ScheduleRetry(id string, attempt int) error {
	backoff := calculateBackoff(attempt)
	nextRetry := time.Now().Add(backoff)

	nq.mu.Lock()
	defer nq.mu.Unlock()

	query := `
		UPDATE notification_queue
		SET status = 'pending', next_retry_at = ?, last_attempt = ?
		WHERE id = ?
	`

	_, err := nq.db.Exec(query, nextRetry.Unix(), time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("failed to schedule retry: %w", err)
	}

	log.Debug().
		Str("id", id).
		Int("attempt", attempt).
		Dur("backoff", backoff).
		Time("nextRetry", nextRetry).
		Msg("Notification retry scheduled")

	return nil
}

// MoveToDLQ moves a notification to the dead letter queue
func (nq *NotificationQueue) MoveToDLQ(id string, reason string) error {
	return nq.UpdateStatus(id, QueueStatusDLQ, reason)
}

// GetDLQ returns all notifications in the dead letter queue
func (nq *NotificationQueue) GetDLQ(limit int) ([]*QueuedNotification, error) {
	nq.mu.RLock()
	defer nq.mu.RUnlock()

	query := `
		SELECT id, type, method, status, alerts, config, attempts, max_attempts,
		       last_attempt, last_error, created_at, next_retry_at, completed_at, payload_bytes
		FROM notification_queue
		WHERE status = 'dlq'
		ORDER BY completed_at DESC
		LIMIT ?
	`

	rows, err := nq.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query DLQ: %w", err)
	}
	defer rows.Close()

	var notifications []*QueuedNotification
	for rows.Next() {
		notif, err := nq.scanNotification(rows)
		if err != nil {
			log.Error().Err(err).Msg("Failed to scan DLQ notification")
			continue
		}
		notifications = append(notifications, notif)
	}

	return notifications, rows.Err()
}

// RecordAudit records a notification delivery attempt in the audit log
func (nq *NotificationQueue) RecordAudit(notif *QueuedNotification, success bool, errorMsg string) error {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	alertIDs := make([]string, len(notif.Alerts))
	for i, alert := range notif.Alerts {
		alertIDs[i] = alert.ID
	}
	alertIDsJSON, _ := json.Marshal(alertIDs)

	query := `
		INSERT INTO notification_audit
		(notification_id, type, method, status, alert_ids, alert_count, attempts, success, error_message, payload_size, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := nq.db.Exec(query,
		notif.ID,
		notif.Type,
		notif.Method,
		notif.Status,
		string(alertIDsJSON),
		len(notif.Alerts),
		notif.Attempts,
		success,
		errorMsg,
		notif.PayloadBytes,
		time.Now().Unix(),
	)

	return err
}

// GetQueueStats returns statistics about the notification queue
func (nq *NotificationQueue) GetQueueStats() (map[string]int, error) {
	nq.mu.RLock()
	defer nq.mu.RUnlock()

	query := `
		SELECT status, COUNT(*) as count
		FROM notification_queue
		WHERE completed_at IS NULL OR completed_at > ?
		GROUP BY status
	`

	// Include last 24 hours of completed
	since := time.Now().Add(-24 * time.Hour).Unix()

	rows, err := nq.db.Query(query, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats[status] = count
	}

	return stats, nil
}

// processQueue runs in background to process pending notifications
func (nq *NotificationQueue) processQueue() {
	defer nq.wg.Done()

	for {
		select {
		case <-nq.stopChan:
			return
		case <-nq.processorTicker.C:
			nq.processBatch()
		case <-nq.notifyChan:
			// Process immediately when notified
			nq.processBatch()
		}
	}
}

// SetProcessor sets the notification processor function
func (nq *NotificationQueue) SetProcessor(processor func(*QueuedNotification) error) {
	nq.mu.Lock()
	defer nq.mu.Unlock()
	nq.processor = processor
}

// processBatch processes a batch of pending notifications concurrently
func (nq *NotificationQueue) processBatch() {
	pending, err := nq.GetPending(20) // Increased batch size for concurrency
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pending notifications")
		return
	}

	if len(pending) == 0 {
		return
	}

	log.Debug().Int("count", len(pending)).Msg("Processing notification batch")

	// Process notifications concurrently with semaphore limiting
	var wg sync.WaitGroup
	for _, notif := range pending {
		wg.Add(1)
		go func(n *QueuedNotification) {
			defer wg.Done()

			// Acquire semaphore slot
			nq.workerSem <- struct{}{}
			defer func() { <-nq.workerSem }()

			nq.processNotification(n)
		}(notif)
	}
	wg.Wait()
}

// processNotification processes a single notification
func (nq *NotificationQueue) processNotification(notif *QueuedNotification) {
	// Skip cancelled notifications
	if notif.Status == QueueStatusCancelled {
		log.Debug().Str("id", notif.ID).Msg("Skipping cancelled notification")
		return
	}

	// Atomically increment attempt counter and set status to sending
	if err := nq.IncrementAttemptAndSetStatus(notif.ID, QueueStatusSending); err != nil {
		log.Error().Err(err).Str("id", notif.ID).Msg("Failed to increment attempt and set status")
		return
	}

	// Call processor if set
	nq.mu.RLock()
	processor := nq.processor
	nq.mu.RUnlock()

	var err error
	if processor != nil {
		err = processor(notif)
	} else {
		err = fmt.Errorf("no processor configured")
	}

	// Record audit
	success := err == nil
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	if auditErr := nq.RecordAudit(notif, success, errorMsg); auditErr != nil {
		log.Error().Err(auditErr).Str("id", notif.ID).Msg("Failed to record audit")
	}

	if success {
		// Mark as sent
		if err := nq.UpdateStatus(notif.ID, QueueStatusSent, ""); err != nil {
			log.Error().Err(err).Str("id", notif.ID).Msg("Failed to update notification status to sent")
		}
		log.Info().Str("id", notif.ID).Str("type", notif.Type).Msg("Notification sent successfully")
	} else {
		// Check if we should retry or move to DLQ
		if notif.Attempts+1 >= notif.MaxAttempts {
			// Move to DLQ
			if dlqErr := nq.MoveToDLQ(notif.ID, errorMsg); dlqErr != nil {
				log.Error().Err(dlqErr).Str("id", notif.ID).Msg("Failed to move notification to DLQ")
			} else {
				log.Warn().
					Str("id", notif.ID).
					Str("type", notif.Type).
					Int("attempts", notif.Attempts+1).
					Str("error", errorMsg).
					Msg("Notification moved to DLQ after max retries")
			}
		} else {
			// Schedule retry
			if retryErr := nq.ScheduleRetry(notif.ID, notif.Attempts+1); retryErr != nil {
				log.Error().Err(retryErr).Str("id", notif.ID).Msg("Failed to schedule retry")
			} else {
				log.Warn().
					Str("id", notif.ID).
					Str("type", notif.Type).
					Int("attempt", notif.Attempts+1).
					Str("error", errorMsg).
					Msg("Notification failed, scheduled for retry")
			}
		}
	}
}

// cleanupOldEntries removes old completed notifications
func (nq *NotificationQueue) cleanupOldEntries() {
	defer nq.wg.Done()

	for {
		select {
		case <-nq.stopChan:
			return
		case <-nq.cleanupTicker.C:
			nq.performCleanup()
		}
	}
}

// performCleanup removes notifications older than retention period
func (nq *NotificationQueue) performCleanup() {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	// Keep completed/failed for 7 days, DLQ for 30 days
	completedCutoff := time.Now().Add(-7 * 24 * time.Hour).Unix()
	dlqCutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()

	// Clean completed/sent/failed/cancelled
	query := `DELETE FROM notification_queue WHERE status IN ('sent', 'failed', 'cancelled') AND completed_at < ?`
	result, err := nq.db.Exec(query, completedCutoff)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup old notifications")
	} else {
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Info().Int64("count", rows).Msg("Cleaned up old completed notifications")
		}
	}

	// Clean old DLQ entries
	query = `DELETE FROM notification_queue WHERE status = 'dlq' AND completed_at < ?`
	result, err = nq.db.Exec(query, dlqCutoff)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup old DLQ entries")
	} else {
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Info().Int64("count", rows).Msg("Cleaned up old DLQ entries")
		}
	}

	// Clean old audit logs (keep 30 days)
	auditCutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	query = `DELETE FROM notification_audit WHERE timestamp < ?`
	result, err = nq.db.Exec(query, auditCutoff)
	if err != nil {
		log.Error().Err(err).Msg("Failed to cleanup old audit logs")
	} else {
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Debug().Int64("count", rows).Msg("Cleaned up old audit logs")
		}
	}
}

// Stop gracefully stops the queue processor
func (nq *NotificationQueue) Stop() error {
	nq.stopOnce.Do(func() {
		close(nq.stopChan)
		nq.processorTicker.Stop()
		nq.cleanupTicker.Stop()
		nq.wg.Wait()

		if err := nq.db.Close(); err != nil {
			nq.stopErr = fmt.Errorf("failed to close database: %w", err)
			return
		}

		log.Info().Msg("Notification queue stopped")
	})

	return nq.stopErr
}

// calculateBackoff calculates exponential backoff duration
func calculateBackoff(attempt int) time.Duration {
	// 1s, 2s, 4s, 8s, 16s, 32s, 60s (capped)
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}
	return backoff
}

// CancelByAlertIDs marks all queued notifications containing any of the given alert IDs as cancelled
func (nq *NotificationQueue) CancelByAlertIDs(alertIDs []string) error {
	if len(alertIDs) == 0 {
		return nil
	}

	nq.mu.Lock()
	defer nq.mu.Unlock()

	// Query pending/sending notifications
	query := `
		SELECT id, alerts
		FROM notification_queue
		WHERE status IN ('pending', 'sending')
	`

	rows, err := nq.db.Query(query)
	if err != nil {
		return fmt.Errorf("failed to query notifications for cancellation: %w", err)
	}

	var toCancelIDs []string
	alertIDSet := make(map[string]struct{})
	for _, id := range alertIDs {
		alertIDSet[id] = struct{}{}
	}

	for rows.Next() {
		var notifID string
		var alertsJSON []byte
		if err := rows.Scan(&notifID, &alertsJSON); err != nil {
			log.Error().Err(err).Msg("Failed to scan notification for cancellation")
			continue
		}

		var alerts []*alerts.Alert
		if err := json.Unmarshal(alertsJSON, &alerts); err != nil {
			log.Error().Err(err).Str("notifID", notifID).Msg("Failed to unmarshal alerts for cancellation check")
			continue
		}

		// Check if any alert in this notification matches
		for _, alert := range alerts {
			if _, exists := alertIDSet[alert.ID]; exists {
				toCancelIDs = append(toCancelIDs, notifID)
				break
			}
		}
	}

	rowsErr := rows.Err()
	rows.Close() // Release connection before executing updates
	if rowsErr != nil {
		return fmt.Errorf("error iterating notifications for cancellation: %w", rowsErr)
	}

	// Cancel the matched notifications (using direct SQL since we already hold the lock)
	if len(toCancelIDs) > 0 {
		now := time.Now().Unix()
		updateQuery := `
			UPDATE notification_queue
			SET status = ?, last_attempt = ?, last_error = ?
			WHERE id = ?
		`
		for _, notifID := range toCancelIDs {
			if _, err := nq.db.Exec(updateQuery, QueueStatusCancelled, now, "Alert resolved", notifID); err != nil {
				log.Error().Err(err).Str("notifID", notifID).Msg("Failed to mark notification as cancelled")
			}
		}

		log.Info().
			Int("count", len(toCancelIDs)).
			Strs("alertIDs", alertIDs).
			Msg("Cancelled queued notifications for resolved alerts")
	}

	return nil
}
