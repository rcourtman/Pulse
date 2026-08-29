package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
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
// before a notification is moved to the dead-letter queue. With the
// exponential backoff schedule (1s doubling, capped at 60s) eight attempts
// span roughly three minutes, so a destination that is briefly down or
// rebooting recovers deliveries instead of dead-lettering them.
const defaultQueueMaxAttempts = 8

const (
	notificationAuditAlertIdentifiersColumn       = "alert_identifiers"
	legacyNotificationAuditAlertIdentifiersColumn = "alert_ids"
	notificationOperationalLinksColumn            = "operational_links"
	notificationFailureClassColumn                = "failure_class"
	notificationAuditDestinationColumn            = "destination_id"
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

// NotificationFailureClass is a closed, content-free delivery failure bucket.
// Only these coarse values may leave the installation in telemetry.
type NotificationFailureClass string

const (
	NotificationFailureAuthentication NotificationFailureClass = "authentication"
	NotificationFailureRateLimited    NotificationFailureClass = "rate_limited"
	NotificationFailureConnectivity   NotificationFailureClass = "connectivity"
	NotificationFailureTLS            NotificationFailureClass = "tls"
	NotificationFailureConfiguration  NotificationFailureClass = "configuration"
	NotificationFailureRejected       NotificationFailureClass = "rejected"
	NotificationFailureServerError    NotificationFailureClass = "server_error"
	NotificationFailureUnknown        NotificationFailureClass = "unknown"
)

// NotificationFailureClassCounts contains only bounded aggregate counters.
type NotificationFailureClassCounts struct {
	Authentication int
	RateLimited    int
	Connectivity   int
	TLS            int
	Configuration  int
	Rejected       int
	ServerError    int
	Unknown        int
}

func (counts NotificationFailureClassCounts) AsMap() map[string]int {
	return map[string]int{
		string(NotificationFailureAuthentication): counts.Authentication,
		string(NotificationFailureRateLimited):    counts.RateLimited,
		string(NotificationFailureConnectivity):   counts.Connectivity,
		string(NotificationFailureTLS):            counts.TLS,
		string(NotificationFailureConfiguration):  counts.Configuration,
		string(NotificationFailureRejected):       counts.Rejected,
		string(NotificationFailureServerError):    counts.ServerError,
		string(NotificationFailureUnknown):        counts.Unknown,
	}
}

// ClassifyNotificationFailure maps local error text to a fixed diagnostic
// bucket. It never returns any portion of the error string.
func ClassifyNotificationFailure(errorMessage string) NotificationFailureClass {
	message := strings.ToLower(strings.TrimSpace(errorMessage))
	containsAny := func(values ...string) bool {
		for _, value := range values {
			if strings.Contains(message, value) {
				return true
			}
		}
		return false
	}

	switch {
	case containsAny(
		"unauthorized", "forbidden", "authentication", "auth failed",
		"auth negotiation",
		"invalid token", "invalid api key", "invalid credentials",
		"status 401", "http 401", "status 403", "http 403", "smtp 535", " 535 ",
	):
		return NotificationFailureAuthentication
	case containsAny("rate limit", "too many requests", "status 429", "http 429"):
		return NotificationFailureRateLimited
	case containsAny(
		"internal server error", "bad gateway", "service unavailable",
		"gateway timeout", "status 5", "http 5",
	):
		return NotificationFailureServerError
	case containsAny(
		"x509", "certificate", "tls", "ssl", "starttls",
	):
		return NotificationFailureTLS
	case containsAny(
		"timeout", "timed out", "deadline exceeded", "connection refused",
		"connection reset", "network unreachable", "no such host", "dial tcp",
		"broken pipe", "unexpected eof",
	):
		return NotificationFailureConnectivity
	case containsAny(
		"not configured", "missing ", "invalid address", "invalid url",
		"invalid webhook", "invalid configuration",
		"must start with http", "validation failed", "executable file not found",
		"no supported", "requires payload template",
	):
		return NotificationFailureConfiguration
	case containsAny(
		"bad request", "not found", "method not allowed", "gone",
		"payload too large", "unsupported media type", "unprocessable",
		"status 400", "http 400", "status 404", "http 404",
		"status 405", "http 405", "status 410", "http 410",
		"status 413", "http 413", "status 415", "http 415",
		"status 422", "http 422", "status 4", "http 4",
	):
		return NotificationFailureRejected
	default:
		return NotificationFailureUnknown
	}
}

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
	mu                    sync.RWMutex
	deliveryGateMu        sync.Mutex
	deliveryGates         map[string]*notificationDeliveryGate
	stopOnce              sync.Once
	stopErr               error
	db                    *sql.DB
	dbPath                string
	stopChan              chan struct{}
	wg                    sync.WaitGroup
	processorTicker       *time.Ticker
	cleanupTicker         *time.Ticker
	notifyChan            chan struct{}                   // Signal when new notifications are added
	processor             func(*QueuedNotification) error // Notification processor function
	deliveryHealthChanged func()                          // Reconcile the monitoring-owned delivery alert after health-changing transitions
	workerSem             chan struct{}                   // Semaphore for limiting concurrent workers
}

// notificationDeliveryGate orders delivery and cancellation for a single
// alert identifier. References are counted so high-cardinality alert IDs do
// not accumulate in the queue for the lifetime of the process.
type notificationDeliveryGate struct {
	mu   sync.RWMutex
	refs int
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
		deliveryGates:   make(map[string]*notificationDeliveryGate),
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

func normalizeAlertIdentifiers(alertIdentifiers []string) []string {
	seen := make(map[string]struct{}, len(alertIdentifiers))
	normalized := make([]string, 0, len(alertIdentifiers))
	for _, identifier := range alertIdentifiers {
		identifier = strings.TrimSpace(identifier)
		if identifier == "" {
			continue
		}
		if _, exists := seen[identifier]; exists {
			continue
		}
		seen[identifier] = struct{}{}
		normalized = append(normalized, identifier)
	}
	sort.Strings(normalized)
	return normalized
}

func alertIdentifiersFromAlerts(alertList []*alerts.Alert) []string {
	identifiers := make([]string, 0, len(alertList))
	for _, alert := range alertList {
		if alert != nil {
			identifiers = append(identifiers, alert.ID)
		}
	}
	return normalizeAlertIdentifiers(identifiers)
}

// acquireAlertDeliveryGates coordinates a firing delivery with resolution
// cancellation for only the alert IDs involved in that operation. Acquiring
// IDs in sorted order prevents grouped notifications from deadlocking when
// different alerts resolve concurrently.
func (nq *NotificationQueue) acquireAlertDeliveryGates(alertIdentifiers []string, write bool) func() {
	identifiers := normalizeAlertIdentifiers(alertIdentifiers)
	if len(identifiers) == 0 {
		return func() {}
	}

	nq.deliveryGateMu.Lock()
	gates := make([]*notificationDeliveryGate, 0, len(identifiers))
	for _, identifier := range identifiers {
		gate := nq.deliveryGates[identifier]
		if gate == nil {
			gate = &notificationDeliveryGate{}
			nq.deliveryGates[identifier] = gate
		}
		gate.refs++
		gates = append(gates, gate)
	}
	nq.deliveryGateMu.Unlock()

	for _, gate := range gates {
		if write {
			gate.mu.Lock()
		} else {
			gate.mu.RLock()
		}
	}

	return func() {
		for i := len(gates) - 1; i >= 0; i-- {
			if write {
				gates[i].mu.Unlock()
			} else {
				gates[i].mu.RUnlock()
			}
		}

		nq.deliveryGateMu.Lock()
		for i, identifier := range identifiers {
			gate := gates[i]
			gate.refs--
			if gate.refs == 0 && nq.deliveryGates[identifier] == gate {
				delete(nq.deliveryGates, identifier)
			}
		}
		nq.deliveryGateMu.Unlock()
	}
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
		failure_class TEXT NOT NULL DEFAULT '',
		destination_id TEXT NOT NULL DEFAULT '',
		payload_size INTEGER,
		timestamp INTEGER NOT NULL,
		FOREIGN KEY (notification_id) REFERENCES notification_queue(id)
	);

	CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON notification_audit(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_notification_id ON notification_audit(notification_id);
	CREATE INDEX IF NOT EXISTS idx_audit_status ON notification_audit(status);

	CREATE TABLE IF NOT EXISTS notification_delivery_receipts (
		alert_id TEXT NOT NULL,
		alert_start_nanos INTEGER NOT NULL,
		destination_key TEXT NOT NULL,
		delivered_at INTEGER NOT NULL,
		PRIMARY KEY (alert_id, alert_start_nanos, destination_key)
	);

	CREATE INDEX IF NOT EXISTS idx_delivery_receipts_delivered_at
		ON notification_delivery_receipts(delivered_at);
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
	if err := nq.ensureJSONColumn(
		"notification_audit",
		notificationOperationalLinksColumn,
	); err != nil {
		return err
	}
	if err := nq.ensureTextColumn(
		"notification_audit",
		notificationFailureClassColumn,
	); err != nil {
		return err
	}
	return nq.ensureTextColumn(
		"notification_audit",
		notificationAuditDestinationColumn,
	)
}

func (nq *NotificationQueue) RecordDeliveryReceipt(alertID string, alertStart time.Time, destinationKey string, deliveredAt time.Time) error {
	if nq == nil || strings.TrimSpace(alertID) == "" || strings.TrimSpace(destinationKey) == "" || alertStart.IsZero() {
		return fmt.Errorf("alert occurrence and destination are required for a delivery receipt")
	}
	if deliveredAt.IsZero() {
		deliveredAt = time.Now()
	}
	nq.mu.Lock()
	defer nq.mu.Unlock()
	_, err := nq.db.Exec(`
		INSERT INTO notification_delivery_receipts
			(alert_id, alert_start_nanos, destination_key, delivered_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(alert_id, alert_start_nanos, destination_key)
		DO UPDATE SET delivered_at = excluded.delivered_at
	`, alertID, alertStart.UnixNano(), destinationKey, deliveredAt.Unix())
	if err != nil {
		return fmt.Errorf("record notification delivery receipt: %w", err)
	}
	return nil
}

func (nq *NotificationQueue) HasDeliveryReceipt(alertID string, alertStart time.Time, destinationKey string) (bool, error) {
	if nq == nil || strings.TrimSpace(alertID) == "" || strings.TrimSpace(destinationKey) == "" || alertStart.IsZero() {
		return false, nil
	}
	nq.mu.RLock()
	defer nq.mu.RUnlock()
	var marker int
	err := nq.db.QueryRow(`
		SELECT 1 FROM notification_delivery_receipts
		WHERE alert_id = ? AND alert_start_nanos = ? AND destination_key = ?
	`, alertID, alertStart.UnixNano(), destinationKey).Scan(&marker)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read notification delivery receipt: %w", err)
	}
	return true, nil
}

func (nq *NotificationQueue) DeleteDeliveryReceipt(alertID string, alertStart time.Time, destinationKey string) error {
	if nq == nil || strings.TrimSpace(alertID) == "" || strings.TrimSpace(destinationKey) == "" || alertStart.IsZero() {
		return nil
	}
	nq.mu.Lock()
	defer nq.mu.Unlock()
	if _, err := nq.db.Exec(`
		DELETE FROM notification_delivery_receipts
		WHERE alert_id = ? AND alert_start_nanos = ? AND destination_key = ?
	`, alertID, alertStart.UnixNano(), destinationKey); err != nil {
		return fmt.Errorf("delete notification delivery receipt: %w", err)
	}
	return nil
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

func (nq *NotificationQueue) ensureTextColumn(table, column string) error {
	columns, err := nq.tableColumns(table)
	if err != nil {
		return err
	}
	if columns[column] {
		return nil
	}
	if _, err := nq.db.Exec(
		`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` TEXT NOT NULL DEFAULT ''`,
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
	err := nq.updateNotificationStatusNoLock(id, status, errorMsg, time.Now())
	nq.mu.Unlock()

	if err == nil && notificationStatusChangesDeliveryHealth(status) {
		nq.notifyDeliveryHealthChanged()
	}
	return err
}

func notificationStatusChangesDeliveryHealth(status NotificationQueueStatus) bool {
	switch status {
	case QueueStatusFailed, QueueStatusDLQ, QueueStatusCancelled:
		return true
	default:
		return false
	}
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

// IncrementAttemptAndSetStatus updates a queue row and its operational links
// atomically. Delivery workers use claimPendingForDelivery so a concurrent
// resolution can cancel a pending row before it is claimed; this method
// remains available for explicit lifecycle transitions and compatibility.
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
	if _, err := nq.db.Exec(query, status, now.Unix(), string(linksJSON), id); err != nil {
		return fmt.Errorf("failed to increment attempt and set status: %w", err)
	}
	return nil
}

// claimPendingForDelivery atomically claims a pending queue row and refreshes
// its alert list. The refresh is required because resolution cancellation may
// have rewritten a grouped row after GetPending returned its snapshot.
func (nq *NotificationQueue) claimPendingForDelivery(notif *QueuedNotification) (bool, error) {
	nq.mu.Lock()
	defer nq.mu.Unlock()

	now := time.Now()
	links, err := nq.readNotificationLinksNoLock(notif.ID)
	if err != nil {
		return false, err
	}
	links = transitionNotificationLinks(
		links,
		notificationDeliveryStateForQueueStatus(QueueStatusSending),
		now,
	)
	linksJSON, err := json.Marshal(links)
	if err != nil {
		return false, fmt.Errorf("marshal notification links for attempt: %w", err)
	}
	query := `
		UPDATE notification_queue
		SET attempts = attempts + 1,
		    status = ?,
		    last_attempt = ?,
		    operational_links = ?
		WHERE id = ? AND status = 'pending'
	`
	result, err := nq.db.Exec(
		query,
		QueueStatusSending,
		now.Unix(),
		string(linksJSON),
		notif.ID,
	)
	if err != nil {
		return false, fmt.Errorf("failed to claim pending notification: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read claimed notification row count: %w", err)
	}
	if rows == 0 {
		return false, nil
	}

	var alertsJSON []byte
	if err := nq.db.QueryRow(`SELECT alerts FROM notification_queue WHERE id = ?`, notif.ID).Scan(&alertsJSON); err != nil {
		return false, fmt.Errorf("reload claimed notification alerts: %w", err)
	}
	var refreshedAlerts []*alerts.Alert
	if err := json.Unmarshal(alertsJSON, &refreshedAlerts); err != nil {
		return false, fmt.Errorf("decode claimed notification alerts: %w", err)
	}
	notif.Alerts = refreshedAlerts
	notif.Attempts++
	notif.Status = QueueStatusSending
	notif.LastAttempt = &now
	notif.Links = links
	return true, nil
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

	links, err := nq.readNotificationLinksNoLock(id)
	if err != nil {
		nq.mu.Unlock()
		return err
	}
	links = transitionNotificationLinks(
		links,
		operationaltrust.NotificationRetrying,
		time.Now(),
	)
	linksJSON, err := json.Marshal(links)
	if err != nil {
		nq.mu.Unlock()
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
		nq.mu.Unlock()
		return fmt.Errorf("failed to schedule retry: %w", err)
	}
	nq.mu.Unlock()

	log.Debug().
		Str("id", id).
		Int("attempt", attempt).
		Dur("backoff", backoff).
		Time("nextRetry", nextRetry).
		Msg("notification retry scheduled")

	nq.notifyDeliveryHealthChanged()
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

// RetryTerminalFailures returns every retained failed or dead-lettered
// delivery to the pending queue with a fresh retry budget. The per-attempt
// audit rows remain intact, so this operator recovery never rewrites delivery
// history or makes an earlier failure look successful.
func (nq *NotificationQueue) RetryTerminalFailures() (int, error) {
	return nq.resolveTerminalFailures(true)
}

// DismissTerminalFailures marks every retained failed or dead-lettered
// delivery as cancelled. This clears the active queue-health warning while
// retaining the immutable delivery audit instead of asking operators to
// delete the queue database.
func (nq *NotificationQueue) DismissTerminalFailures() (int, error) {
	return nq.resolveTerminalFailures(false)
}

func (nq *NotificationQueue) resolveTerminalFailures(retry bool) (int, error) {
	if nq == nil || nq.db == nil {
		return 0, fmt.Errorf("notification queue not initialized")
	}

	nq.mu.Lock()
	affected, err := nq.resolveTerminalFailuresNoLock(retry)
	nq.mu.Unlock()
	if err != nil {
		return 0, err
	}

	// Reconcile even when no rows were affected. A prior process may already
	// have repaired the queue while leaving a persisted system alert active.
	nq.notifyDeliveryHealthChanged()
	return affected, nil
}

func (nq *NotificationQueue) resolveTerminalFailuresNoLock(retry bool) (int, error) {
	rows, err := nq.db.Query(`
		SELECT id, operational_links
		FROM notification_queue
		WHERE status IN ('failed', 'dlq')
		ORDER BY completed_at, id
	`)
	if err != nil {
		return 0, fmt.Errorf("read retained terminal notifications: %w", err)
	}
	type terminalNotification struct {
		id    string
		links []operationaltrust.NotificationLink
	}
	terminal := make([]terminalNotification, 0)
	for rows.Next() {
		var id, rawLinks string
		if err := rows.Scan(&id, &rawLinks); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan retained terminal notification: %w", err)
		}
		var links []operationaltrust.NotificationLink
		if err := json.Unmarshal([]byte(rawLinks), &links); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("decode retained terminal notification links: %w", err)
		}
		terminal = append(terminal, terminalNotification{id: id, links: links})
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close retained terminal notification rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate retained terminal notifications: %w", err)
	}
	if len(terminal) == 0 {
		return 0, nil
	}

	tx, err := nq.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin terminal notification recovery: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now()
	affected := 0
	for _, notification := range terminal {
		state := operationaltrust.NotificationCancelled
		if retry {
			state = operationaltrust.NotificationRetrying
		}
		linksJSON, err := json.Marshal(transitionNotificationLinks(notification.links, state, now))
		if err != nil {
			return 0, fmt.Errorf("encode terminal notification links: %w", err)
		}

		var result sql.Result
		if retry {
			result, err = tx.Exec(`
				UPDATE notification_queue
				SET status = 'pending', attempts = 0, last_attempt = NULL,
				    last_error = NULL, next_retry_at = ?, completed_at = NULL,
				    operational_links = ?
				WHERE id = ? AND status IN ('failed', 'dlq')
			`, now.Unix(), string(linksJSON), notification.id)
		} else {
			result, err = tx.Exec(`
				UPDATE notification_queue
				SET status = 'cancelled', last_attempt = ?,
				    last_error = 'Dismissed by operator', next_retry_at = NULL,
				    completed_at = ?, operational_links = ?
				WHERE id = ? AND status IN ('failed', 'dlq')
			`, now.Unix(), now.Unix(), string(linksJSON), notification.id)
		}
		if err != nil {
			return 0, fmt.Errorf("resolve terminal notification %s: %w", notification.id, err)
		}
		if count, countErr := result.RowsAffected(); countErr == nil {
			affected += int(count)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit terminal notification recovery: %w", err)
	}
	if retry && affected > 0 {
		select {
		case nq.notifyChan <- struct{}{}:
		default:
		}
	}
	return affected, nil
}

// SetDeliveryHealthChangedCallback installs the monitoring-owned reconciliation
// hook. Notification delivery owns queue truth; monitoring owns its system
// alert projection, so the queue only announces that the verdict may have
// changed after committing and releasing its database lock.
func (nq *NotificationQueue) SetDeliveryHealthChangedCallback(callback func()) {
	if nq == nil {
		return
	}
	nq.mu.Lock()
	nq.deliveryHealthChanged = callback
	nq.mu.Unlock()
}

func (nq *NotificationQueue) notifyDeliveryHealthChanged() {
	if nq == nil {
		return
	}
	nq.mu.RLock()
	callback := nq.deliveryHealthChanged
	nq.mu.RUnlock()
	if callback != nil {
		callback()
	}
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
		(notification_id, type, method, status, alert_identifiers, alert_count, operational_links, attempts, success, error_message, failure_class, destination_id, payload_size, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	failureClass := ""
	if !success {
		failureClass = string(ClassifyNotificationFailure(errorMsg))
	}
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
		failureClass,
		strings.TrimSpace(notif.DestinationID),
		notif.PayloadBytes,
		time.Now().Unix(),
	)
	if err == nil {
		operationaltrust.GetMetrics().ObserveNotificationDelivery(
			deliveryOutcome(notif.Status, success, notif.Attempts),
		)
	}
	return err
}

// GetQueueStats returns counts for the rows currently retained by the queue.
// Sent, failed, and cancelled rows are retained for seven days; dead-letter
// rows are retained for 30 days; pending and sending rows remain until they
// reach a terminal state. Callers must not present these mixed retention
// windows as a rate or as lifetime delivery history.
func (nq *NotificationQueue) GetQueueStats() (map[string]int, error) {
	nq.mu.RLock()
	defer nq.mu.RUnlock()

	query := `
		SELECT status, COUNT(*) as count
		FROM notification_queue
		GROUP BY status
	`

	rows, err := nq.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan notification queue statistics: %w", err)
		}
		stats[status] = count
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notification queue statistics: %w", err)
	}
	return stats, nil
}

// TelemetryStats is a content-free delivery outcome aggregate. It deliberately
// excludes notification, destination, tenant, alert, and resource identities.
type TelemetryStats struct {
	Attempts   int
	Deliveries int
	// Failures counts terminal delivery failures only. Recoverable failed
	// attempts that returned to pending for retry are included in Attempts but
	// not in Failures.
	Failures       int
	FailureClasses NotificationFailureClassCounts
}

// GetTelemetryStats returns aggregate delivery activity from locally retained
// per-attempt audit rows recorded on or after since. Attempts includes retries;
// Deliveries counts successful terminal sends; Failures counts only failed or
// dead-letter terminal outcomes. Callers must not interpret a window longer
// than the queue's completed-row retention as complete.
func (nq *NotificationQueue) GetTelemetryStats(since time.Time) (TelemetryStats, error) {
	if nq == nil {
		return TelemetryStats{}, nil
	}
	if since.IsZero() {
		since = time.Now().Add(-7 * 24 * time.Hour)
	}

	nq.mu.RLock()
	defer nq.mu.RUnlock()

	var stats TelemetryStats
	err := nq.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE
				WHEN success = 0 AND status IN ('failed', 'dlq') THEN 1
				ELSE 0
			END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'authentication' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'rate_limited' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'connectivity' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'tls' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'configuration' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'rejected' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN success = 0 AND status IN ('failed', 'dlq') AND failure_class = 'server_error' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE
				WHEN success = 0
					AND status IN ('failed', 'dlq')
					AND COALESCE(failure_class, '') NOT IN ('authentication', 'rate_limited', 'connectivity', 'tls', 'configuration', 'rejected', 'server_error')
				THEN 1 ELSE 0
			END), 0)
		FROM notification_audit
		WHERE timestamp >= ?
	`, since.UTC().Unix()).Scan(
		&stats.Attempts,
		&stats.Deliveries,
		&stats.Failures,
		&stats.FailureClasses.Authentication,
		&stats.FailureClasses.RateLimited,
		&stats.FailureClasses.Connectivity,
		&stats.FailureClasses.TLS,
		&stats.FailureClasses.Configuration,
		&stats.FailureClasses.Rejected,
		&stats.FailureClasses.ServerError,
		&stats.FailureClasses.Unknown,
	)
	if err != nil {
		return TelemetryStats{}, fmt.Errorf("read notification telemetry aggregates: %w", err)
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
	releaseDeliveryGates := nq.acquireAlertDeliveryGates(alertIdentifiersFromAlerts(notif.Alerts), false)
	defer releaseDeliveryGates()

	// Atomically claim the pending row. A concurrent resolution may have
	// cancelled it while it was waiting for its per-alert delivery gate.
	claimed, err := nq.claimPendingForDelivery(notif)
	if err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "increment_attempt_set_status").
			Str("id", notif.ID).
			Str("type", notif.Type).
			Int("attempt", notif.Attempts+1).
			Int("maxAttempts", notif.MaxAttempts).
			Msg("Failed to claim pending notification")
		return
	}
	if !claimed {
		log.Debug().
			Str("component", "notification_queue").
			Str("action", "skip_unclaimed").
			Str("id", notif.ID).
			Str("type", notif.Type).
			Msg("Skipping notification because it is no longer pending")
		return
	}
	// Call processor if set
	nq.mu.RLock()
	processor := nq.processor
	nq.mu.RUnlock()
	if processor == nil {
		return
	}

	err = processor(notif)

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
	receiptCutoff := dlqCutoff

	// Receipts are only needed to correlate a later recovery with a firing
	// occurrence. Bound their lifetime so installations with recovery delivery
	// disabled do not grow this side table indefinitely.
	if result, err := nq.db.Exec(`DELETE FROM notification_delivery_receipts WHERE delivered_at < ?`, receiptCutoff); err != nil {
		log.Error().
			Err(err).
			Str("component", "notification_queue").
			Str("action", "cleanup_delivery_receipts").
			Int64("receiptCutoff", receiptCutoff).
			Msg("Failed to cleanup old notification delivery receipts")
	} else if rows, _ := result.RowsAffected(); rows > 0 {
		log.Debug().
			Str("component", "notification_queue").
			Str("action", "cleanup_delivery_receipts").
			Int64("count", rows).
			Int64("receiptCutoff", receiptCutoff).
			Msg("Cleaned up old notification delivery receipts")
	}

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
	alertIdentifiers = normalizeAlertIdentifiers(alertIdentifiers)
	if len(alertIdentifiers) == 0 {
		return 0, nil
	}
	releaseDeliveryGates := nq.acquireAlertDeliveryGates(alertIdentifiers, true)
	defer releaseDeliveryGates()

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
		alertIdentifierSet[id] = struct{}{}
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
