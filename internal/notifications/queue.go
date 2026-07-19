package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

// defaultQueueMaxAttempts is the default number of delivery attempts
// before a notification is moved to the dead-letter queue.
const defaultQueueMaxAttempts = 3

const (
	notificationAuditAlertIdentifiersColumn       = "alert_identifiers"
	legacyNotificationAuditAlertIdentifiersColumn = "alert_ids"
	notificationOperationalLinksColumn            = "operational_links"
	notificationQueueDirName                      = "notifications"
	notificationQueueFileName                     = "notification_queue.db"
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
	ID            string                              `json:"id"`
	Type          string                              `json:"type"` // email, webhook, apprise
	Method        string                              `json:"method,omitempty"`
	DestinationID string                              `json:"destinationId,omitempty"`
	Status        NotificationQueueStatus             `json:"status"`
	Alerts        []*alerts.Alert                     `json:"alerts"`
	Links         []operationaltrust.NotificationLink `json:"links,omitempty"`
	Config        json.RawMessage                     `json:"config"` // EmailConfig, WebhookConfig, or AppriseConfig
	Attempts      int                                 `json:"attempts"`
	MaxAttempts   int                                 `json:"maxAttempts"`
	LastAttempt   *time.Time                          `json:"lastAttempt,omitempty"`
	LastError     *string                             `json:"lastError,omitempty"`
	CreatedAt     time.Time                           `json:"createdAt"`
	NextRetryAt   *time.Time                          `json:"nextRetryAt,omitempty"`
	CompletedAt   *time.Time                          `json:"completedAt,omitempty"`
	PayloadBytes  *int                                `json:"payloadBytes,omitempty"`
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

func queueSQLiteDSN(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + url.Values{
		"_pragma": []string{
			"busy_timeout(30000)",
			"journal_mode(WAL)",
			"synchronous(NORMAL)",
			"foreign_keys(ON)",
			"cache_size(-64000)",
		},
	}.Encode()
}

func resolveNotificationQueuePath(dataDir string) (string, string, error) {
	normalizedDir := strings.TrimSpace(dataDir)
	if normalizedDir == "" {
		defaultDir, err := securityutil.JoinStorageLeaf(utils.GetDataDir(), notificationQueueDirName)
		if err != nil {
			return "", "", fmt.Errorf("resolve default notification queue dir: %w", err)
		}
		normalizedDir = defaultDir
	}

	normalizedDir, err := securityutil.NormalizeStorageDir(normalizedDir)
	if err != nil {
		return "", "", fmt.Errorf("normalize notification queue dir: %w", err)
	}

	dbPath, err := securityutil.JoinStorageLeaf(normalizedDir, notificationQueueFileName)
	if err != nil {
		return "", "", fmt.Errorf("resolve notification queue db path: %w", err)
	}

	return normalizedDir, dbPath, nil
}

func newNotificationQueueFromDSN(dbPath, dsn string) (*NotificationQueue, error) {
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
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "recover_stuck_sending").
			Str("dbPath", dbPath).
			Msg("Failed to recover stuck sending notifications")
	}

	// Start background processors
	nq.wg.Add(2)
	go nq.processQueue()
	go nq.cleanupOldEntries()

	log.Info().
		Str("dbPath", dbPath).
		Msg("notification queue initialized")

	return nq, nil
}

// NewNotificationQueue creates a new persistent notification queue.
func NewNotificationQueue(dataDir string) (*NotificationQueue, error) {
	resolvedDataDir, dbPath, err := resolveNotificationQueuePath(dataDir)
	if err != nil {
		return nil, err
	}

	// Queue data includes alert payload/context and destination configuration;
	// keep it owner-only by default.
	if err := os.MkdirAll(resolvedDataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create notification queue directory: %w", err)
	}

	return newNotificationQueueFromDSN(dbPath, queueSQLiteDSN(dbPath))
}

// NewInMemoryNotificationQueue creates a non-persistent in-memory queue with the
// same runtime behavior as the persistent queue. It exists as a fallback owner
// when on-disk queue bootstrap fails, so notification delivery still routes
// through the canonical queue processor.
func NewInMemoryNotificationQueue() (*NotificationQueue, error) {
	return newNotificationQueueFromDSN(":memory:", queueSQLiteDSN("file::memory:?mode=memory"))
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
			operational_links TEXT NOT NULL DEFAULT '[]',
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
	CREATE INDEX IF NOT EXISTS idx_status_completed ON notification_queue(status, completed_at);

	CREATE TABLE IF NOT EXISTS notification_audit (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		notification_id TEXT NOT NULL,
		type TEXT NOT NULL,
		method TEXT,
			status TEXT NOT NULL,
			alert_identifiers TEXT,
			alert_count INTEGER,
			operational_links TEXT NOT NULL DEFAULT '[]',
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

	if _, err := nq.db.Exec(schema); err != nil {
		return err
	}

	if err := nq.migrateAlertIdentifierColumns(); err != nil {
		return err
	}
	if err := nq.ensureJSONColumn(
		"notification_queue",
		notificationOperationalLinksColumn,
	); err != nil {
		return err
	}
	return nq.ensureJSONColumn(
		"notification_audit",
		notificationOperationalLinksColumn,
	)
}

func (nq *NotificationQueue) migrateAlertIdentifierColumns() error {
	columns, err := nq.tableColumns("notification_audit")
	if err != nil {
		return err
	}
	if columns[notificationAuditAlertIdentifiersColumn] || !columns[legacyNotificationAuditAlertIdentifiersColumn] {
		return nil
	}

	if _, err := nq.db.Exec(
		`ALTER TABLE notification_audit RENAME COLUMN ` +
			legacyNotificationAuditAlertIdentifiersColumn +
			` TO ` +
			notificationAuditAlertIdentifiersColumn,
	); err != nil {
		return fmt.Errorf(
			"rename notification_audit.%s to %s: %w",
			legacyNotificationAuditAlertIdentifiersColumn,
			notificationAuditAlertIdentifiersColumn,
			err,
		)
	}

	return nil
}

func (nq *NotificationQueue) tableColumns(table string) (map[string]bool, error) {
	rows, err := nq.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, fmt.Errorf("inspect columns for %s: %w", table, err)
	}
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return nil, fmt.Errorf("scan column for %s: %w", table, err)
		}
		columns[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns for %s: %w", table, err)
	}

	return columns, nil
}

func (nq *NotificationQueue) ensureJSONColumn(table, column string) error {
	columns, err := nq.tableColumns(table)
	if err != nil {
		return err
	}
	if columns[column] {
		return nil
	}
	if _, err := nq.db.Exec(
		`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` TEXT NOT NULL DEFAULT '[]'`,
	); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

func notificationLinksForAlerts(
	alertsToLink []*alerts.Alert,
	destinationID string,
) []operationaltrust.NotificationLink {
	destinationID = strings.TrimSpace(destinationID)
	if destinationID == "" {
		return nil
	}
	links := make([]operationaltrust.NotificationLink, 0, len(alertsToLink))
	for _, alert := range alertsToLink {
		if alert == nil ||
			alert.OperationalRecord == nil ||
			alert.LatestTransition == nil {
			continue
		}
		links = append(links, operationaltrust.NotificationLink{
			OperationalRecordID: alert.OperationalRecord.ID,
			TransitionID:        alert.LatestTransition.ID,
			LifecycleState:      alert.LatestTransition.To,
			CauseKey:            alert.LatestTransition.CauseKey,
			DestinationID:       destinationID,
			DeliveryState:       operationaltrust.NotificationQueued,
		})
	}
	return links
}

func notificationTransitionIDs(
	links []operationaltrust.NotificationLink,
) []string {
	ids := make([]string, 0, len(links))
	for _, link := range links {
		if id := strings.TrimSpace(link.TransitionID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func normalizeNotificationLinks(
	notificationID string,
	destinationID string,
	links []operationaltrust.NotificationLink,
	state operationaltrust.NotificationDeliveryState,
	attemptedAt *time.Time,
	completedAt *time.Time,
) ([]operationaltrust.NotificationLink, error) {
	if len(links) == 0 {
		return nil, nil
	}
	normalized := make([]operationaltrust.NotificationLink, 0, len(links))
	seen := make(map[string]struct{}, len(links))
	for _, link := range links {
		link = link.Clone()
		link.NotificationID = notificationID
		if strings.TrimSpace(link.DestinationID) == "" {
			link.DestinationID = strings.TrimSpace(destinationID)
		}
		link.DeliveryState = state
		link.AttemptedAt = attemptedAt
		link.CompletedAt = completedAt
		if err := link.Validate(); err != nil {
			return nil, err
		}
		key := link.OperationalRecordID + "\x00" +
			link.TransitionID + "\x00" +
			link.DestinationID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, link)
	}
	return normalized, nil
}

// Enqueue adds a notification to the queue
func (nq *NotificationQueue) Enqueue(notif *QueuedNotification) error {
	if notif == nil {
		return fmt.Errorf("notification cannot be nil")
	}

	notif.Type = strings.TrimSpace(notif.Type)
	if notif.Type == "" {
		return fmt.Errorf("notification type cannot be empty")
	}

	notif.Method = strings.TrimSpace(notif.Method)

	if len(notif.Config) == 0 {
		return fmt.Errorf("notification config cannot be empty")
	}

	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}
	if len(notif.Links) == 0 {
		notif.Links = notificationLinksForAlerts(
			notif.Alerts,
			notif.DestinationID,
		)
	}
	if strings.TrimSpace(notif.DestinationID) == "" && len(notif.Links) > 0 {
		notif.DestinationID = strings.TrimSpace(notif.Links[0].DestinationID)
	}
	if notif.ID == "" {
		transitionIDs := notificationTransitionIDs(notif.Links)
		if len(transitionIDs) > 0 && strings.TrimSpace(notif.DestinationID) != "" {
			id, err := operationaltrust.NewNotificationID(
				notif.DestinationID,
				notif.Type,
				notif.CreatedAt,
				transitionIDs,
			)
			if err != nil {
				return fmt.Errorf("build notification id: %w", err)
			}
			notif.ID = id
		} else {
			notif.ID = fmt.Sprintf("%s-%d", notif.Type, notif.CreatedAt.UnixNano())
		}
	}
	if notif.Status == "" {
		notif.Status = QueueStatusPending
	}
	if notif.MaxAttempts <= 0 {
		notif.MaxAttempts = defaultQueueMaxAttempts
	}
	if notif.Attempts < 0 {
		notif.Attempts = 0
	}
	var err error
	notif.Links, err = normalizeNotificationLinks(
		notif.ID,
		notif.DestinationID,
		notif.Links,
		notificationDeliveryStateForQueueStatus(notif.Status),
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("normalize notification links: %w", err)
	}

	alertsJSON, err := json.Marshal(notif.Alerts)
	if err != nil {
		return fmt.Errorf("failed to marshal alerts: %w", err)
	}
	linksJSON, err := json.Marshal(notif.Links)
	if err != nil {
		return fmt.Errorf("failed to marshal operational links: %w", err)
	}

	nq.mu.Lock()
	defer nq.mu.Unlock()

	query := `
		INSERT INTO notification_queue
		(id, type, method, status, alerts, operational_links, config, attempts, max_attempts, created_at, next_retry_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		string(linksJSON),
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
		Msg("notification enqueued")
	metrics := operationaltrust.GetMetrics()
	metrics.ObserveNotificationDelivery("queued")
	for _, alert := range notif.Alerts {
		if alert == nil ||
			alert.LatestTransition == nil ||
			alert.LatestTransition.To != operationaltrust.OperationalOpen {
			continue
		}
		metrics.ObserveOpenToNotification(
			alert.LatestTransition.At,
			notif.CreatedAt,
		)
	}

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

	return nq.updateNotificationStatusNoLock(id, status, errorMsg, time.Now())
}

func (nq *NotificationQueue) updateNotificationStatusNoLock(
	id string,
	status NotificationQueueStatus,
	errorMsg string,
	now time.Time,
) error {
	links, err := nq.readNotificationLinksNoLock(id)
	if err != nil {
		return err
	}
	links = transitionNotificationLinks(
		links,
		notificationDeliveryStateForQueueStatus(status),
		now,
	)
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal notification links for status update: %w", err)
	}
	nowUnix := now.Unix()
	var completedAt *int64
	if status == QueueStatusSent ||
		status == QueueStatusFailed ||
		status == QueueStatusDLQ ||
		status == QueueStatusCancelled {
		completedAt = &nowUnix
	}
	query := `
		UPDATE notification_queue
		SET status = ?, last_attempt = ?, last_error = ?, completed_at = ?,
		    operational_links = ?,
		    next_retry_at = CASE WHEN ? THEN NULL ELSE next_retry_at END
		WHERE id = ?
	`

	result, err := nq.db.Exec(
		query,
		status,
		nowUnix,
		errorMsg,
		completedAt,
		string(linksJSON),
		completedAt != nil,
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to update notification status: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("notification not found: %s", id)
	}

	return nil
}

func (nq *NotificationQueue) readNotificationLinksNoLock(
	id string,
) ([]operationaltrust.NotificationLink, error) {
	var raw string
	if err := nq.db.QueryRow(
		`SELECT operational_links FROM notification_queue WHERE id = ?`,
		id,
	).Scan(&raw); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("notification not found: %s", id)
		}
		return nil, fmt.Errorf("read notification links: %w", err)
	}
	var links []operationaltrust.NotificationLink
	if err := json.Unmarshal([]byte(raw), &links); err != nil {
		return nil, fmt.Errorf("unmarshal notification links: %w", err)
	}
	return links, nil
}

func (nq *NotificationQueue) getNotificationLinks(
	id string,
) ([]operationaltrust.NotificationLink, error) {
	nq.mu.RLock()
	defer nq.mu.RUnlock()
	return nq.readNotificationLinksNoLock(id)
}

func notificationDeliveryStateForQueueStatus(
	status NotificationQueueStatus,
) operationaltrust.NotificationDeliveryState {
	switch status {
	case QueueStatusSending:
		return operationaltrust.NotificationDelivering
	case QueueStatusSent:
		return operationaltrust.NotificationDelivered
	case QueueStatusFailed:
		return operationaltrust.NotificationFailed
	case QueueStatusDLQ:
		return operationaltrust.NotificationDeadLetter
	case QueueStatusCancelled:
		return operationaltrust.NotificationCancelled
	default:
		return operationaltrust.NotificationQueued
	}
}

func transitionNotificationLinks(
	links []operationaltrust.NotificationLink,
	state operationaltrust.NotificationDeliveryState,
	at time.Time,
) []operationaltrust.NotificationLink {
	transitioned := make([]operationaltrust.NotificationLink, len(links))
	for index := range links {
		transitioned[index] = links[index].Clone()
		transitioned[index].DeliveryState = state
		switch state {
		case operationaltrust.NotificationQueued:
			transitioned[index].AttemptedAt = nil
			transitioned[index].CompletedAt = nil
		case operationaltrust.NotificationDelivering:
			attemptedAt := at
			transitioned[index].AttemptedAt = &attemptedAt
			transitioned[index].CompletedAt = nil
		case operationaltrust.NotificationRetrying:
			transitioned[index].CompletedAt = nil
		case operationaltrust.NotificationDelivered,
			operationaltrust.NotificationFailed,
			operationaltrust.NotificationDeadLetter,
			operationaltrust.NotificationCancelled:
			if transitioned[index].AttemptedAt != nil {
				completedAt := at
				transitioned[index].CompletedAt = &completedAt
			}
		}
	}
	return transitioned
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

	now := time.Now()
	links, err := nq.readNotificationLinksNoLock(id)
	if err != nil {
		return err
	}
	links = transitionNotificationLinks(
		links,
		notificationDeliveryStateForQueueStatus(status),
		now,
	)
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal notification links for attempt: %w", err)
	}
	query := `
		UPDATE notification_queue
		SET attempts = attempts + 1,
		    status = ?,
		    last_attempt = ?,
		    operational_links = ?
		WHERE id = ?
	`
	_, err = nq.db.Exec(query, status, now.Unix(), string(linksJSON), id)
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
		       last_attempt, last_error, created_at, next_retry_at, completed_at, payload_bytes,
		       operational_links
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
			log.Error().
				Err(err).
				Str("component", "notification_queue").
				Str("action", "scan_pending_row").
				Str("queueStatus", string(QueueStatusPending)).
				Int("batchLimit", limit).
				Str("dbPath", nq.dbPath).
				Msg("Failed to scan notification row")
			continue
		}
		notifications = append(notifications, notif)
	}

	return notifications, rows.Err()
}

// scanNotification scans a database row into a QueuedNotification
func (nq *NotificationQueue) scanNotification(rows *sql.Rows) (*QueuedNotification, error) {
	var notif QueuedNotification
	var alertsJSON, configJSON, linksJSON string
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
		&linksJSON,
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
	if err := json.Unmarshal([]byte(linksJSON), &notif.Links); err != nil {
		return nil, fmt.Errorf("failed to unmarshal operational links: %w", err)
	}
	if len(notif.Links) > 0 {
		notif.DestinationID = notif.Links[0].DestinationID
	}

	return &notif, nil
}

// ScheduleRetry schedules a notification for retry with exponential backoff
func (nq *NotificationQueue) ScheduleRetry(id string, attempt int) error {
	backoff := calculateBackoff(attempt)
	nextRetry := time.Now().Add(backoff)

	nq.mu.Lock()
	defer nq.mu.Unlock()

	links, err := nq.readNotificationLinksNoLock(id)
	if err != nil {
		return err
	}
	links = transitionNotificationLinks(
		links,
		operationaltrust.NotificationRetrying,
		time.Now(),
	)
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return fmt.Errorf("marshal notification links for retry: %w", err)
	}
	query := `
		UPDATE notification_queue
		SET status = 'pending', next_retry_at = ?, last_attempt = ?,
		    operational_links = ?, completed_at = NULL, last_error = NULL
		WHERE id = ?
	`

	_, err = nq.db.Exec(
		query,
		nextRetry.Unix(),
		time.Now().Unix(),
		string(linksJSON),
		id,
	)
	if err != nil {
		return fmt.Errorf("failed to schedule retry: %w", err)
	}

	log.Debug().
		Str("id", id).
		Int("attempt", attempt).
		Dur("backoff", backoff).
		Time("nextRetry", nextRetry).
		Msg("notification retry scheduled")

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
		       last_attempt, last_error, created_at, next_retry_at, completed_at, payload_bytes,
		       operational_links
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
			log.Error().
				Err(err).
				Str("component", "notification_queue").
				Str("action", "scan_dlq_row").
				Str("queueStatus", string(QueueStatusDLQ)).
				Int("batchLimit", limit).
				Str("dbPath", nq.dbPath).
				Msg("Failed to scan DLQ notification")
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

	alertIdentifiers := make([]string, len(notif.Alerts))
	for i, alert := range notif.Alerts {
		alertIdentifiers[i] = alert.ID
	}
	alertIdentifiersJSON, _ := json.Marshal(alertIdentifiers)
	linksJSON, err := json.Marshal(notif.Links)
	if err != nil {
		return fmt.Errorf("failed to marshal notification links for audit: %w", err)
	}

	query := `
		INSERT INTO notification_audit
		(notification_id, type, method, status, alert_identifiers, alert_count, operational_links, attempts, success, error_message, payload_size, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = nq.db.Exec(query,
		notif.ID,
		notif.Type,
		notif.Method,
		notif.Status,
		string(alertIdentifiersJSON),
		len(notif.Alerts),
		string(linksJSON),
		notif.Attempts,
		success,
		errorMsg,
		notif.PayloadBytes,
		time.Now().Unix(),
	)
	if err == nil {
		outcome := "failed"
		switch {
		case success && notif.Status == QueueStatusSent:
			outcome = "sent"
		case notif.Status == QueueStatusDLQ:
			outcome = "dead_letter"
		case notif.Status == QueueStatusCancelled:
			outcome = "cancelled"
		case notif.Status == QueueStatusPending && notif.Attempts > 0:
			outcome = "retry"
		}
		operationaltrust.GetMetrics().ObserveNotificationDelivery(outcome)
	}
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
	nq.processor = processor
	nq.mu.Unlock()

	if processor != nil {
		select {
		case nq.notifyChan <- struct{}{}:
		default:
		}
	}
}

// processBatch processes a batch of pending notifications concurrently
func (nq *NotificationQueue) processBatch() {
	const batchLimit = 20

	nq.mu.RLock()
	processorConfigured := nq.processor != nil
	nq.mu.RUnlock()
	if !processorConfigured {
		return
	}

	pending, err := nq.GetPending(batchLimit) // Increased batch size for concurrency
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "get_pending_batch").
			Int("batchLimit", batchLimit).
			Msg("Failed to get pending notifications")
		return
	}

	if len(pending) == 0 {
		return
	}

	log.Debug().
		Str("component", "notification_queue").
		Int("count", len(pending)).
		Int("batchLimit", batchLimit).
		Msg("Processing notification batch")

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
		log.Debug().
			Str("component", "notification_queue").
			Str("action", "skip_cancelled").
			Str("id", notif.ID).
			Str("type", notif.Type).
			Str("status", string(notif.Status)).
			Msg("Skipping cancelled notification")
		return
	}

	// Atomically increment attempt counter and set status to sending
	if err := nq.IncrementAttemptAndSetStatus(notif.ID, QueueStatusSending); err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "increment_attempt_set_status").
			Str("id", notif.ID).
			Str("type", notif.Type).
			Int("attempt", notif.Attempts+1).
			Int("maxAttempts", notif.MaxAttempts).
			Msg("Failed to increment attempt and set status")
		return
	}
	attemptedAt := time.Now()
	notif.Attempts++
	notif.Status = QueueStatusSending
	notif.LastAttempt = &attemptedAt
	notif.Links = transitionNotificationLinks(
		notif.Links,
		operationaltrust.NotificationDelivering,
		attemptedAt,
	)

	// Call processor if set
	nq.mu.RLock()
	processor := nq.processor
	nq.mu.RUnlock()
	if processor == nil {
		return
	}

	err := processor(notif)

	success := err == nil
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
	}

	if success {
		// Mark as sent
		if err := nq.UpdateStatus(notif.ID, QueueStatusSent, ""); err != nil {
			log.Error().
				Err(err).
				Str("component", "notification_queue").
				Str("action", "mark_sent").
				Str("id", notif.ID).
				Str("type", notif.Type).
				Int("attempt", notif.Attempts).
				Msg("Failed to update notification status to sent")
		} else {
			completedAt := time.Now()
			notif.Status = QueueStatusSent
			notif.CompletedAt = &completedAt
			notif.Links = transitionNotificationLinks(
				notif.Links,
				operationaltrust.NotificationDelivered,
				completedAt,
			)
		}
		log.Info().
			Str("component", "notification_queue").
			Str("action", "send_success").
			Str("id", notif.ID).
			Str("type", notif.Type).
			Int("attempt", notif.Attempts).
			Int("maxAttempts", notif.MaxAttempts).
			Msg("Notification sent successfully")
	} else {
		// Check if we should retry or move to DLQ
		if notif.Attempts >= notif.MaxAttempts {
			// Move to DLQ
			if dlqErr := nq.MoveToDLQ(notif.ID, errorMsg); dlqErr != nil {
				log.Error().
					Err(dlqErr).
					Str("component", "notification_queue").
					Str("action", "move_to_dlq").
					Str("id", notif.ID).
					Str("type", notif.Type).
					Int("attempt", notif.Attempts).
					Int("maxAttempts", notif.MaxAttempts).
					Msg("Failed to move notification to DLQ")
			} else {
				completedAt := time.Now()
				notif.Status = QueueStatusDLQ
				notif.CompletedAt = &completedAt
				notif.Links = transitionNotificationLinks(
					notif.Links,
					operationaltrust.NotificationDeadLetter,
					completedAt,
				)
				log.Warn().
					Str("component", "notification_queue").
					Str("action", "move_to_dlq").
					Str("id", notif.ID).
					Str("type", notif.Type).
					Int("attempts", notif.Attempts).
					Int("maxAttempts", notif.MaxAttempts).
					Str("error", errorMsg).
					Msg("notification moved to DLQ after max retries")
			}
		} else {
			// Schedule retry
			if retryErr := nq.ScheduleRetry(notif.ID, notif.Attempts); retryErr != nil {
				log.Error().
					Err(retryErr).
					Str("component", "notification_queue").
					Str("action", "schedule_retry").
					Str("id", notif.ID).
					Str("type", notif.Type).
					Int("attempt", notif.Attempts).
					Int("maxAttempts", notif.MaxAttempts).
					Msg("Failed to schedule retry")
			} else {
				notif.Status = QueueStatusPending
				notif.Links = transitionNotificationLinks(
					notif.Links,
					operationaltrust.NotificationRetrying,
					time.Now(),
				)
				log.Warn().
					Str("component", "notification_queue").
					Str("action", "schedule_retry").
					Str("id", notif.ID).
					Str("type", notif.Type).
					Int("attempt", notif.Attempts).
					Int("maxAttempts", notif.MaxAttempts).
					Str("error", errorMsg).
					Msg("notification failed, scheduled for retry")
			}
		}
	}

	if persistedLinks, linksErr := nq.getNotificationLinks(notif.ID); linksErr != nil {
		log.Error().
			Err(linksErr).
			Str("component", "notification_queue").
			Str("action", "read_links_for_audit").
			Str("id", notif.ID).
			Msg("Failed to read persisted notification links for audit")
	} else {
		notif.Links = persistedLinks
	}
	if auditErr := nq.RecordAudit(notif, success, errorMsg); auditErr != nil {
		log.Error().
			Err(auditErr).
			Str("component", "notification_queue").
			Str("action", "record_audit").
			Str("id", notif.ID).
			Str("type", notif.Type).
			Int("attempt", notif.Attempts).
			Int("maxAttempts", notif.MaxAttempts).
			Msg("Failed to record audit")
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

	// Delete audit records for notifications about to be cleaned up (FK constraint)
	auditCleanup := `DELETE FROM notification_audit WHERE notification_id IN (
		SELECT id FROM notification_queue WHERE status IN ('sent', 'failed', 'cancelled') AND completed_at < ?
	)`
	if _, err := nq.db.Exec(auditCleanup, completedCutoff); err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "cleanup_audit_for_completed").
			Msg("Failed to cleanup audit records for completed notifications")
	}

	// Clean completed/sent/failed/cancelled
	query := `DELETE FROM notification_queue WHERE status IN ('sent', 'failed', 'cancelled') AND completed_at < ?`
	result, err := nq.db.Exec(query, completedCutoff)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "cleanup_completed_notifications").
			Int64("completedCutoff", completedCutoff).
			Msg("Failed to cleanup old notifications")
	} else {
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Info().
				Str("component", "notification_queue").
				Str("action", "cleanup_completed_notifications").
				Int64("count", rows).
				Int64("completedCutoff", completedCutoff).
				Msg("Cleaned up old completed notifications")
		}
	}

	// Delete audit records for DLQ notifications about to be cleaned up (FK constraint)
	auditCleanup = `DELETE FROM notification_audit WHERE notification_id IN (
		SELECT id FROM notification_queue WHERE status = 'dlq' AND completed_at < ?
	)`
	if _, err = nq.db.Exec(auditCleanup, dlqCutoff); err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "cleanup_audit_for_dlq").
			Msg("Failed to cleanup audit records for DLQ notifications")
	}

	// Clean old DLQ entries
	query = `DELETE FROM notification_queue WHERE status = 'dlq' AND completed_at < ?`
	result, err = nq.db.Exec(query, dlqCutoff)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "cleanup_dlq_entries").
			Int64("dlqCutoff", dlqCutoff).
			Msg("Failed to cleanup old DLQ entries")
	} else {
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Info().
				Str("component", "notification_queue").
				Str("action", "cleanup_dlq_entries").
				Int64("count", rows).
				Int64("dlqCutoff", dlqCutoff).
				Msg("Cleaned up old DLQ entries")
		}
	}

	// Clean old audit logs (keep 30 days)
	auditCutoff := time.Now().Add(-30 * 24 * time.Hour).Unix()
	query = `DELETE FROM notification_audit WHERE timestamp < ?`
	result, err = nq.db.Exec(query, auditCutoff)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "cleanup_audit_logs").
			Int64("auditCutoff", auditCutoff).
			Msg("Failed to cleanup old audit logs")
	} else {
		if rows, _ := result.RowsAffected(); rows > 0 {
			log.Debug().
				Str("component", "notification_queue").
				Str("action", "cleanup_audit_logs").
				Int64("count", rows).
				Int64("auditCutoff", auditCutoff).
				Msg("Cleaned up old audit logs")
		}
	}
}

// Stop gracefully stops the queue processor
func (nq *NotificationQueue) Stop() error {
	nq.stopOnce.Do(func() {
		close(nq.stopChan)
		nq.wg.Wait()

		nq.processorTicker.Stop()
		nq.cleanupTicker.Stop()

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
	if attempt <= 0 {
		return 1 * time.Second
	}
	if attempt >= 6 {
		return 60 * time.Second
	}

	// 1s, 2s, 4s, 8s, 16s, 32s, 60s (capped)
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if backoff > 60*time.Second {
		backoff = 60 * time.Second
	}
	return backoff
}

// CancelByAlertIdentifiers suppresses queued firing notifications for resolved
// alerts while preserving unrelated alerts in the same grouped queue row. It
// returns the number of matched firing-alert entries removed from rows that
// were still waiting for delivery ('pending'). Entries in rows already
// mid-send ('sending') are cancelled best-effort but not counted, because
// their delivery may still complete.
func (nq *NotificationQueue) CancelByAlertIdentifiers(alertIdentifiers []string) (int, error) {
	if len(alertIdentifiers) == 0 {
		return 0, nil
	}

	nq.mu.Lock()
	defer nq.mu.Unlock()

	query := `
		SELECT id, type, status, alerts, operational_links
		FROM notification_queue
		WHERE status IN ('pending', 'sending')
	`

	rows, err := nq.db.Query(query)
	if err != nil {
		return 0, fmt.Errorf("failed to query notifications for cancellation: %w", err)
	}

	alertIdentifierSet := make(map[string]struct{})
	for _, id := range alertIdentifiers {
		id = strings.TrimSpace(id)
		if id != "" {
			alertIdentifierSet[id] = struct{}{}
		}
	}
	if len(alertIdentifierSet) == 0 {
		if err := rows.Close(); err != nil {
			return 0, fmt.Errorf("failed to close cancellation query: %w", err)
		}
		return 0, nil
	}

	type queuedAlertCancellation struct {
		notificationID string
		remaining      []*alerts.Alert
		remainingLinks []operationaltrust.NotificationLink
	}

	var toCancelIDs []string
	var toRewrite []queuedAlertCancellation
	suppressedAlertCount := 0
	suppressedPendingAlertCount := 0

	for rows.Next() {
		var notifID string
		var notifType string
		var notifStatus string
		var alertsJSON []byte
		var linksJSON []byte
		if err := rows.Scan(
			&notifID,
			&notifType,
			&notifStatus,
			&alertsJSON,
			&linksJSON,
		); err != nil {
			log.Error().
				Err(err).
				Str("component", "notification_queue").
				Str("action", "cancel_scan_notification").
				Msg("Failed to scan notification for cancellation")
			continue
		}
		if !queueTypeCancelableOnAlertResolution(notifType) {
			continue
		}

		var queuedAlerts []*alerts.Alert
		if err := json.Unmarshal(alertsJSON, &queuedAlerts); err != nil {
			log.Error().
				Err(err).
				Str("component", "notification_queue").
				Str("action", "cancel_unmarshal_alerts").
				Str("notifID", notifID).
				Msg("Failed to unmarshal alerts for cancellation check")
			continue
		}
		var links []operationaltrust.NotificationLink
		if err := json.Unmarshal(linksJSON, &links); err != nil {
			log.Error().
				Err(err).
				Str("component", "notification_queue").
				Str("action", "cancel_unmarshal_links").
				Str("notifID", notifID).
				Msg("Failed to unmarshal operational links for cancellation check")
			continue
		}

		remainingAlerts := make([]*alerts.Alert, 0, len(queuedAlerts))
		removedLinkKeys := make(map[string]struct{})
		matchedAlertCount := 0
		for _, alert := range queuedAlerts {
			if alert == nil {
				remainingAlerts = append(remainingAlerts, alert)
				continue
			}
			if _, exists := alertIdentifierSet[alert.ID]; exists {
				matchedAlertCount++
				if alert.OperationalRecord != nil && alert.LatestTransition != nil {
					removedLinkKeys[alert.OperationalRecord.ID+"\x00"+alert.LatestTransition.ID] = struct{}{}
				}
				continue
			}
			remainingAlerts = append(remainingAlerts, alert)
		}
		if matchedAlertCount == 0 {
			continue
		}

		suppressedAlertCount += matchedAlertCount
		if notifStatus == string(QueueStatusPending) {
			suppressedPendingAlertCount += matchedAlertCount
		}
		if len(remainingAlerts) == 0 {
			toCancelIDs = append(toCancelIDs, notifID)
			continue
		}
		remainingLinks := make([]operationaltrust.NotificationLink, 0, len(links))
		for _, link := range links {
			key := link.OperationalRecordID + "\x00" + link.TransitionID
			if _, removed := removedLinkKeys[key]; removed {
				continue
			}
			remainingLinks = append(remainingLinks, link)
		}
		toRewrite = append(toRewrite, queuedAlertCancellation{
			notificationID: notifID,
			remaining:      remainingAlerts,
			remainingLinks: remainingLinks,
		})
	}

	rowsErr := rows.Err()
	rows.Close() // Release connection before executing updates
	if rowsErr != nil {
		return 0, fmt.Errorf("error iterating notifications for cancellation: %w", rowsErr)
	}

	if len(toRewrite) > 0 {
		updateAlertsQuery := `
			UPDATE notification_queue
			SET alerts = ?, operational_links = ?
			WHERE id = ?
		`
		for _, rewrite := range toRewrite {
			alertsJSON, err := json.Marshal(rewrite.remaining)
			if err != nil {
				return 0, fmt.Errorf("failed to marshal remaining alerts for %s: %w", rewrite.notificationID, err)
			}
			linksJSON, err := json.Marshal(rewrite.remainingLinks)
			if err != nil {
				return 0, fmt.Errorf("failed to marshal remaining links for %s: %w", rewrite.notificationID, err)
			}
			if _, err := nq.db.Exec(
				updateAlertsQuery,
				string(alertsJSON),
				string(linksJSON),
				rewrite.notificationID,
			); err != nil {
				return 0, fmt.Errorf("failed to rewrite queued notification %s after alert resolution: %w", rewrite.notificationID, err)
			}
		}
	}

	if len(toCancelIDs) > 0 {
		for _, notifID := range toCancelIDs {
			if err := nq.updateNotificationStatusNoLock(
				notifID,
				QueueStatusCancelled,
				"Alert resolved",
				time.Now(),
			); err != nil {
				log.Error().
					Err(err).
					Str("component", "notification_queue").
					Str("action", "cancel_mark_notification").
					Str("notifID", notifID).
					Msg("Failed to mark notification as cancelled")
			}
		}
	}

	if suppressedAlertCount > 0 {
		log.Info().
			Str("component", "notification_queue").
			Str("action", "cancel_alert_identifiers").
			Int("suppressedAlertCount", suppressedAlertCount).
			Int("suppressedPendingAlertCount", suppressedPendingAlertCount).
			Int("cancelledRows", len(toCancelIDs)).
			Int("rewrittenRows", len(toRewrite)).
			Strs("alertIdentifiers", alertIdentifiers).
			Msg("suppressed resolved alerts in queued notifications")
	}

	return suppressedPendingAlertCount, nil
}

func queueTypeCancelableOnAlertResolution(notifType string) bool {
	_, event := normalizeQueueType(notifType)
	return event != eventResolved
}

// CancelByTypes marks queued notifications of the given types as cancelled.
func (nq *NotificationQueue) CancelByTypes(types []string, reason string) error {
	if len(types) == 0 {
		return nil
	}
	if strings.TrimSpace(reason) == "" {
		reason = "Notification destination disabled"
	}

	nq.mu.Lock()
	defer nq.mu.Unlock()

	placeholders := make([]string, len(types))
	args := make([]any, 0, len(types))
	for i, notifType := range types {
		placeholders[i] = "?"
		args = append(args, notifType)
	}

	query := fmt.Sprintf(`
		SELECT id
		FROM notification_queue
		WHERE status IN ('pending', 'sending')
		  AND type IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := nq.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("failed to query notifications by type: %w", err)
	}
	var notificationIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan notification by type: %w", err)
		}
		notificationIDs = append(notificationIDs, id)
	}
	rowsErr := rows.Err()
	_ = rows.Close()
	if rowsErr != nil {
		return fmt.Errorf("iterate notifications by type: %w", rowsErr)
	}

	for _, id := range notificationIDs {
		if err := nq.updateNotificationStatusNoLock(
			id,
			QueueStatusCancelled,
			reason,
			time.Now(),
		); err != nil {
			return fmt.Errorf("cancel notification %s by type: %w", id, err)
		}
	}

	if len(notificationIDs) > 0 {
		log.Info().
			Str("component", "notification_queue").
			Str("action", "cancel_types").
			Int("count", len(notificationIDs)).
			Strs("types", types).
			Str("reason", reason).
			Msg("cancelled queued notifications by type")
	}

	return nil
}
