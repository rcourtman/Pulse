package notifications

// A rehearsal is an identified notification producer, not an infrastructure
// incident. Its evidence lives beside the queue and is never projected into
// resource health, active alerts, Patrol or operational lifecycle records.

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

const rehearsalMetadataKey = "notificationRehearsalId"
const rehearsalRetention = 7 * 24 * time.Hour

var ErrRehearsalNotFound = errors.New("delivery rehearsal not found")
var ErrRehearsalConflict = errors.New("finish the current delivery rehearsal before starting another")
var ErrInvalidRehearsalRequest = errors.New("invalid delivery rehearsal request")

type RehearsalRequest struct {
	ID    string            `json:"id"`
	Level alerts.AlertLevel `json:"level"`
	Tags  []string          `json:"tags"`
}

// RehearsalOutcome names evidence, not inferred delivery. Deferred and pending
// work must never be flattened into either success or terminal suppression.
type RehearsalOutcome struct {
	Status       string     `json:"status"`
	Reason       string     `json:"reason"`
	Message      string     `json:"message"`
	NextAt       *time.Time `json:"nextAt,omitempty"`
	Attempts     int        `json:"attempts"`
	FailureClass string     `json:"failureClass,omitempty"`
	Diagnostic   string     `json:"diagnostic,omitempty"`
}

type RehearsalDestination struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Firing   RehearsalOutcome `json:"firing"`
	Recovery RehearsalOutcome `json:"recovery"`
}

type DeliveryRehearsal struct {
	ID           string                 `json:"id"`
	CreatedAt    time.Time              `json:"createdAt"`
	RecoveredAt  *time.Time             `json:"recoveredAt,omitempty"`
	Level        alerts.AlertLevel      `json:"level"`
	Tags         []string               `json:"tags"`
	Policy       RehearsalOutcome       `json:"policy"`
	Destinations []RehearsalDestination `json:"destinations"`
}

type storedRehearsal struct {
	DeliveryRehearsal
	Alert         *alerts.Alert `json:"alert"`
	Owner         string        `json:"owner"`
	RecoveryOwner string        `json:"recoveryOwner,omitempty"`
}

func rehearsalOutcome(status, reason, message string) RehearsalOutcome {
	return RehearsalOutcome{Status: status, Reason: reason, Message: message}
}

func rehearsalID(alert *alerts.Alert) string {
	if alert == nil || alert.Metadata == nil {
		return ""
	}
	id, _ := alert.Metadata[rehearsalMetadataKey].(string)
	if alert.ID != "delivery-rehearsal:"+id {
		return ""
	}
	if _, err := uuid.Parse(id); err != nil {
		return ""
	}
	return id
}

// partitionRehearsalAlerts preserves the ordinary grouping machinery and
// deadlines while keeping labelled rehearsals out of real incident messages.
func partitionRehearsalAlerts(batch []*alerts.Alert) [][]*alerts.Alert {
	groups := make([][]*alerts.Alert, 0, 1)
	indices := make(map[string]int)
	for _, alert := range batch {
		key := rehearsalID(alert)
		index, found := indices[key]
		if !found {
			index = len(groups)
			indices[key] = index
			groups = append(groups, nil)
		}
		groups[index] = append(groups[index], alert)
	}
	return groups
}

func (q *NotificationQueue) migrateRehearsals() error {
	q.rehearsalOwner = uuid.NewString()
	// The association is mechanical and atomic with every queue insertion or
	// payload extension, including recovery grouping, quiet-hours partitions
	// and graceful shutdown checkpoints. It does not affect dispatch policy.
	_, err := q.db.Exec(`
CREATE TABLE IF NOT EXISTS notification_rehearsals (
 id TEXT PRIMARY KEY, alert_id TEXT NOT NULL UNIQUE,
 created_at INTEGER NOT NULL, recovered_at INTEGER, data TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rehearsal_created ON notification_rehearsals(created_at);
CREATE TABLE IF NOT EXISTS notification_rehearsal_jobs (
 rehearsal_id TEXT NOT NULL REFERENCES notification_rehearsals(id) ON DELETE CASCADE,
 notification_id TEXT NOT NULL REFERENCES notification_queue(id) ON DELETE CASCADE,
 PRIMARY KEY(rehearsal_id, notification_id)
);
CREATE INDEX IF NOT EXISTS idx_rehearsal_notification ON notification_rehearsal_jobs(notification_id);
CREATE TRIGGER IF NOT EXISTS notification_rehearsal_queue_insert AFTER INSERT ON notification_queue
 WHEN instr(NEW.alerts, 'notificationRehearsalId') > 0 AND json_valid(NEW.alerts) BEGIN
 INSERT OR IGNORE INTO notification_rehearsal_jobs
 SELECT r.id, NEW.id FROM json_each(NEW.alerts) a
 JOIN notification_rehearsals r ON r.alert_id = json_extract(CASE WHEN a.type='object' THEN a.value ELSE '{}' END, '$.id')
 WHERE json_extract(CASE WHEN a.type='object' THEN a.value ELSE '{}' END, '$.metadata.notificationRehearsalId') = r.id;
END;
CREATE TRIGGER IF NOT EXISTS notification_rehearsal_queue_update AFTER UPDATE OF alerts ON notification_queue
 WHEN instr(NEW.alerts, 'notificationRehearsalId') > 0 AND json_valid(NEW.alerts) BEGIN
 INSERT OR IGNORE INTO notification_rehearsal_jobs
 SELECT r.id, NEW.id FROM json_each(NEW.alerts) a
 JOIN notification_rehearsals r ON r.alert_id = json_extract(CASE WHEN a.type='object' THEN a.value ELSE '{}' END, '$.id')
 WHERE json_extract(CASE WHEN a.type='object' THEN a.value ELSE '{}' END, '$.metadata.notificationRehearsalId') = r.id;
END;`)
	return err
}

func (q *NotificationQueue) readRehearsalLocked(id string) (*storedRehearsal, error) {
	if _, failed := q.rehearsalErrors[id]; failed {
		return nil, fmt.Errorf("rehearsal evidence is unavailable")
	}
	var raw []byte
	var recovered sql.NullInt64
	if err := q.db.QueryRow(`SELECT data, recovered_at FROM notification_rehearsals WHERE id = ? AND created_at >= ?`, id, time.Now().Add(-rehearsalRetention).Unix()).Scan(&raw, &recovered); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRehearsalNotFound
		}
		return nil, err
	}
	var run storedRehearsal
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, err
	}
	if recovered.Valid {
		at := time.Unix(0, recovered.Int64).UTC()
		run.RecoveredAt = &at
	}
	return &run, nil
}

func (q *NotificationQueue) writeRehearsalLocked(run *storedRehearsal) error {
	data, err := json.Marshal(run)
	if err == nil {
		_, err = q.db.Exec(`UPDATE notification_rehearsals SET data = ? WHERE id = ?`, string(data), run.ID)
	}
	if err != nil {
		// Never publish old admission facts as current after a failed evidence
		// write. The bounded run registry also bounds this process-local fence.
		if q.rehearsalErrors == nil {
			q.rehearsalErrors = make(map[string]struct{})
		}
		q.rehearsalErrors[run.ID] = struct{}{}
	}
	return err
}

func rehearsalDestination(job notificationDeliveryJob) RehearsalDestination {
	name := "Destination"
	switch job.Type {
	case "email":
		name = "Email"
	case "apprise":
		name = "Apprise"
	case "webhook":
		name = "Webhook"
		if job.WebhookConfig != nil && strings.TrimSpace(job.WebhookConfig.Name) != "" {
			name = strings.TrimSpace(job.WebhookConfig.Name)
		}
	}
	return RehearsalDestination{ID: notificationDestinationIDForJob(job), Name: name,
		Firing:   rehearsalOutcome("pending", "admission", "Waiting for notification policy and grouping."),
		Recovery: rehearsalOutcome("not_started", "not_started", "Send recovery after inspecting the firing result.")}
}

func rehearsalDestinations(email EmailConfig, webhooks []WebhookConfig, apprise AppriseConfig) []RehearsalDestination {
	result := []RehearsalDestination{}
	add := func(job notificationDeliveryJob) {
		result = append(result, rehearsalDestination(job))
	}
	if email.Enabled || email.SMTPHost != "" || len(email.To) > 0 {
		add(notificationDeliveryJob{Type: "email", EmailConfig: &email})
	}
	for _, webhook := range webhooks {
		add(notificationDeliveryJob{Type: "webhook", WebhookConfig: &webhook})
	}
	if apprise.Enabled || len(apprise.Targets) > 0 {
		add(notificationDeliveryJob{Type: "apprise", AppriseConfig: &apprise})
	}
	return result
}

// StartDeliveryRehearsal admits one durable, idempotent producer occurrence.
// Retrying a request with the same ID never resends an uncertain operation.
func (n *NotificationManager) StartDeliveryRehearsal(request RehearsalRequest) (*DeliveryRehearsal, error) {
	if parsed, err := uuid.Parse(request.ID); err != nil || parsed.String() != request.ID {
		return nil, fmt.Errorf("%w: a canonical UUID is required", ErrInvalidRehearsalRequest)
	}
	if request.Level != alerts.AlertLevelWarning && request.Level != alerts.AlertLevelCritical && request.Level != alerts.AlertLevelInfo {
		return nil, fmt.Errorf("%w: choose info, warning or critical", ErrInvalidRehearsalRequest)
	}
	if len(request.Tags) > 32 {
		return nil, fmt.Errorf("%w: use at most 32 tags", ErrInvalidRehearsalRequest)
	}
	for _, tag := range request.Tags {
		if len(tag) > 64 {
			return nil, fmt.Errorf("%w: each tag must be at most 64 bytes", ErrInvalidRehearsalRequest)
		}
	}
	request.Tags = normalizeNotificationTagFilter(request.Tags)
	if request.Tags == nil {
		request.Tags = []string{}
	}
	n.mu.RLock()
	queue := n.queue
	email, webhooks, apprise := copyEmailConfig(n.emailConfig), copyWebhookConfigs(n.webhooks), copyAppriseConfig(n.appriseConfig)
	admitted := n.lifecycle.admit()
	n.mu.RUnlock()
	if !admitted {
		return nil, fmt.Errorf("notification manager is stopping")
	}
	defer n.lifecycle.release()
	if queue == nil || queue.dbPath == ":memory:" {
		return nil, fmt.Errorf("persistent notification queue is unavailable")
	}
	queue.rehearsalAdmissionMu.Lock()
	defer queue.rehearsalAdmissionMu.Unlock()
	now := time.Now().UTC()
	alert := &alerts.Alert{ID: "delivery-rehearsal:" + request.ID, Type: "notification-rehearsal", Level: request.Level,
		ResourceID: "delivery-rehearsal:" + request.ID, ResourceName: "[Delivery rehearsal] Sample alert", Node: "Pulse delivery rehearsal",
		Message:   "[Delivery rehearsal] This is a labelled notification rehearsal. No infrastructure problem was detected.",
		StartTime: now, LastSeen: now, Metadata: map[string]interface{}{rehearsalMetadataKey: request.ID, "tags": request.Tags}}
	// Apply the canonical schedule now as ordinary alerts do at admission. The
	// worker still revalidates its current policy immediately before sending.
	queue.mu.RLock()
	policyProvider := queue.quietHoursPolicy
	queue.mu.RUnlock()
	if policyProvider != nil {
		if policy := policyProvider(); policy != nil {
			if at := policy(alert, now); at != nil {
				alert.Metadata[alerts.MetadataQuietHoursReplayAt] = at.UTC().Format(time.RFC3339)
			}
		}
	}
	run := &storedRehearsal{DeliveryRehearsal: DeliveryRehearsal{ID: request.ID, CreatedAt: now, Level: request.Level, Tags: request.Tags,
		Policy: rehearsalOutcome("pending", "admission", "Waiting for notification admission."), Destinations: rehearsalDestinations(email, webhooks, apprise)}, Alert: alert, Owner: queue.rehearsalOwner}
	err := queue.expireRehearsals()
	queue.mu.Lock()
	if err == nil {
		if previous, readErr := queue.readRehearsalLocked(request.ID); readErr == nil {
			queue.mu.Unlock()
			if previous.Level != request.Level || strings.Join(previous.Tags, "\x00") != strings.Join(request.Tags, "\x00") {
				return nil, fmt.Errorf("%w: rehearsal ID belongs to different inputs", ErrInvalidRehearsalRequest)
			}
			return n.GetDeliveryRehearsal(request.ID)
		} else if !errors.Is(readErr, ErrRehearsalNotFound) {
			err = readErr
		}
	}
	if err == nil {
		var active int
		err = queue.db.QueryRow(`SELECT count(*) FROM notification_rehearsals WHERE recovered_at IS NULL`).Scan(&active)
		if err == nil && active > 0 {
			err = ErrRehearsalConflict
		}
	}
	if err == nil {
		var count int
		err = queue.db.QueryRow(`SELECT count(*) FROM notification_rehearsals`).Scan(&count)
		if err == nil && count >= 100 {
			err = fmt.Errorf("the seven-day rehearsal history is full")
		}
	}
	if err == nil {
		var data []byte
		data, err = json.Marshal(run)
		if err == nil {
			_, err = queue.db.Exec(`INSERT INTO notification_rehearsals(id,alert_id,created_at,data) VALUES(?,?,?,?)`, run.ID, alert.ID, now.Unix(), string(data))
		}
	}
	queue.mu.Unlock()
	if err != nil {
		return nil, err
	}
	n.SendAlert(alert)
	return n.GetDeliveryRehearsal(run.ID)
}

func (n *NotificationManager) ListDeliveryRehearsals() ([]DeliveryRehearsal, error) {
	q := n.GetQueue()
	if q == nil {
		return nil, fmt.Errorf("notification queue unavailable")
	}
	q.mu.RLock()
	rows, err := q.db.Query(`SELECT id FROM notification_rehearsals WHERE created_at >= ? ORDER BY created_at DESC, id DESC LIMIT 20`, time.Now().Add(-rehearsalRetention).Unix())
	if err != nil {
		q.mu.RUnlock()
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			break
		}
		ids = append(ids, id)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	q.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	result := make([]DeliveryRehearsal, 0, len(ids))
	for _, id := range ids {
		run, err := n.GetDeliveryRehearsal(id)
		if err != nil {
			return nil, err
		}
		result = append(result, *run)
	}
	return result, nil
}

// RecoverDeliveryRehearsal uses ordinary occurrence cancellation and recovery
// receipt admission. The recorded request makes repeated POSTs idempotent.
func (n *NotificationManager) RecoverDeliveryRehearsal(id string) (*DeliveryRehearsal, error) {
	q := n.GetQueue()
	if q == nil {
		return nil, fmt.Errorf("notification queue unavailable")
	}
	q.rehearsalAdmissionMu.Lock()
	defer q.rehearsalAdmissionMu.Unlock()
	q.mu.Lock()
	run, err := q.readRehearsalLocked(id)
	if err != nil {
		q.mu.Unlock()
		return nil, err
	}
	if run.RecoveredAt != nil {
		q.mu.Unlock()
		return n.GetDeliveryRehearsal(id)
	}
	now := time.Now().UTC()
	run.RecoveryOwner = q.rehearsalOwner
	data, err := json.Marshal(run)
	if err == nil {
		_, err = q.db.Exec(`UPDATE notification_rehearsals SET recovered_at = ?, data = ? WHERE id = ? AND recovered_at IS NULL`, now.UnixNano(), string(data), id)
	}
	q.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if n.CancelResolvedAlert(run.Alert) {
		q.mu.Lock()
		if saved, readErr := q.readRehearsalLocked(id); readErr == nil {
			for i := range saved.Destinations {
				if saved.Destinations[i].Firing.Status == "pending" || saved.Destinations[i].Firing.Status == "deferred" {
					saved.Destinations[i].Firing = rehearsalOutcome("suppressed", "resolved_before_delivery", "The rehearsal ended before its firing notification was delivered.")
				}
			}
			_ = q.writeRehearsalLocked(saved)
		}
		q.mu.Unlock()
	}
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: run.Alert, ResolvedTime: now})
	return n.GetDeliveryRehearsal(id)
}

func (q *NotificationQueue) rehearsalGate(alert *alerts.Alert, phase, status, reason, message string) {
	id := rehearsalID(alert)
	if id == "" || q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	run, err := q.readRehearsalLocked(id)
	if err != nil {
		return
	}
	outcome := rehearsalOutcome(status, reason, message)
	if phase == "firing" {
		run.Policy = outcome
	}
	if status == "suppressed" || status == "failed" || status == "unavailable" {
		for i := range run.Destinations {
			if phase == "firing" {
				run.Destinations[i].Firing = outcome
			} else {
				run.Destinations[i].Recovery = outcome
			}
		}
	}
	_ = q.writeRehearsalLocked(run)
}

func (n *NotificationManager) rehearsalGate(alert *alerts.Alert, phase, status, reason, message string) {
	if rehearsalID(alert) == "" {
		return
	}
	if q := n.GetQueue(); q != nil {
		q.rehearsalGate(alert, phase, status, reason, message)
	}
}

func rehearsalPhase(event notificationEvent) string {
	if event == eventResolved {
		return "recovery"
	}
	return "firing"
}

func (q *NotificationQueue) rehearsalGrouping(alert *alerts.Alert, deadline time.Time) {
	id := rehearsalID(alert)
	if q == nil || id == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	run, err := q.readRehearsalLocked(id)
	if err != nil {
		return
	}
	for i := range run.Destinations {
		if run.Destinations[i].Firing.Reason == "admission" {
			out := rehearsalOutcome("deferred", "grouping", "Waiting for the configured grouping window.")
			out.NextAt = &deadline
			run.Destinations[i].Firing = out
		}
	}
	_ = q.writeRehearsalLocked(run)
}

func (q *NotificationQueue) rehearsalReceiptUnavailable(alert *alerts.Alert, destination string) {
	id := rehearsalID(alert)
	if q == nil || id == "" {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	run, err := q.readRehearsalLocked(id)
	if err != nil {
		return
	}
	for i := range run.Destinations {
		if run.Destinations[i].ID == destination {
			run.Destinations[i].Recovery = rehearsalOutcome("unavailable", "receipt_unavailable", "The firing receipt could not be read. Recovery was not authorized.")
		}
	}
	_ = q.writeRehearsalLocked(run)
}

func rehearsalJobContains(jobs []notificationDeliveryJob, alertID, destination string) bool {
	for _, job := range jobs {
		if notificationDestinationIDForJob(job) == destination {
			for _, alert := range job.Alerts {
				if alert != nil && alert.ID == alertID {
					return true
				}
			}
		}
	}
	return false
}

// Observe the actual planner inputs and retained jobs. Shared routing helpers
// explain exclusions; no rehearsal-owned policy can authorize a send.
func (q *NotificationQueue) rehearsalRouting(batch []*alerts.Alert, email EmailConfig, webhooks []WebhookConfig, apprise AppriseConfig, target notificationDeliveryTarget, before, after []notificationDeliveryJob, phase string) {
	if q == nil {
		return
	}
	for _, alert := range batch {
		id := rehearsalID(alert)
		if id == "" {
			continue
		}
		q.mu.Lock()
		run, err := q.readRehearsalLocked(id)
		if err != nil {
			q.mu.Unlock()
			continue
		}
		current := rehearsalDestinations(email, webhooks, apprise)
		for _, d := range current {
			found := false
			for _, saved := range run.Destinations {
				if saved.ID == d.ID {
					found = true
					break
				}
			}
			if !found {
				if phase == "recovery" {
					d.Firing = rehearsalOutcome("suppressed", "added_after_firing", "This destination was added after firing admission. No firing was sent to it.")
				}
				run.Destinations = append(run.Destinations, d)
			}
		}
		for i := range run.Destinations {
			d := &run.Destinations[i]
			out := rehearsalOutcome("pending", "queue", "Waiting for durable queue evidence.")
			if !rehearsalJobContains(after, alert.ID, d.ID) {
				out = rehearsalOutcome("suppressed", "destination_unavailable", "Destination is disabled, removed or outside the configured initial route.")
				if rehearsalJobContains(before, alert.ID, d.ID) {
					if phase == "recovery" {
						out = rehearsalOutcome("suppressed", "firing_not_delivered", "Recovery requires a successful firing receipt for this destination and occurrence.")
					} else {
						out = rehearsalOutcome("suppressed", "cooldown", "This destination is in its delivery cooldown.")
					}
				} else {
					var filter []string
					var mode, severity string
					enabled := false
					selected := true
					for _, wh := range webhooks {
						job := notificationDeliveryJob{Type: "webhook", WebhookConfig: &wh}
						if notificationDestinationIDForJob(job) == d.ID {
							enabled = wh.Enabled
							filter = wh.TagFilter
							mode = wh.TagMode
							severity = wh.MinimumSeverity
							selected = target == "" || target == notificationDeliveryTargetAll || target == notificationDeliveryTargetWebhook
						}
					}
					if d.ID == notificationDestinationIDForJob(notificationDeliveryJob{Type: "email", EmailConfig: &email}) {
						enabled = email.Enabled
						filter = email.TagFilter
						mode = email.TagMode
						severity = email.MinimumSeverity
						selected = target == "" || target == notificationDeliveryTargetAll || target == notificationDeliveryTargetEmail
					}
					if d.ID == notificationDestinationIDForJob(notificationDeliveryJob{Type: "apprise", AppriseConfig: &apprise}) {
						enabled = apprise.Enabled
						severity = apprise.MinimumSeverity
						selected = target == "" || target == notificationDeliveryTargetAll || target == notificationDeliveryTargetApprise
					}
					if enabled && selected {
						if !notificationAlertMeetsMinimumSeverity(alert, severity) && phase != "recovery" {
							out = rehearsalOutcome("suppressed", "severity", "The rehearsal level does not meet this destination's minimum severity.")
						} else if !notificationAlertMatchesTags(alert, filter, mode) {
							out = rehearsalOutcome("suppressed", "tags", "The rehearsal tags do not match this destination's tag filter.")
						}
					}
				}
			}
			if phase == "firing" {
				d.Firing = out
			} else if d.Recovery.Status != "unavailable" {
				d.Recovery = out
			}
		}
		_ = q.writeRehearsalLocked(run)
		q.mu.Unlock()
	}
}

func (n *NotificationManager) GetDeliveryRehearsal(id string) (*DeliveryRehearsal, error) {
	q := n.GetQueue()
	if q == nil {
		return nil, fmt.Errorf("notification queue unavailable")
	}
	q.mu.RLock()
	run, err := q.readRehearsalLocked(id)
	if err != nil {
		q.mu.RUnlock()
		return nil, err
	}
	rows, err := q.db.Query(`SELECT n.id,n.type,n.method,n.status,n.alerts,n.config,n.attempts,n.max_attempts,n.last_attempt,n.last_error,n.created_at,n.next_retry_at,n.completed_at,n.payload_bytes,n.operational_links,n.receipt_epoch
 FROM notification_rehearsal_jobs j JOIN notification_queue n ON n.id=j.notification_id WHERE j.rehearsal_id=? ORDER BY n.created_at,n.id`, id)
	if err != nil {
		q.mu.RUnlock()
		return nil, err
	}
	var jobs []*QueuedNotification
	for rows.Next() {
		var row *QueuedNotification
		row, err = q.scanNotification(rows)
		if err != nil {
			break
		}
		jobs = append(jobs, row)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	provider := q.quietHoursPolicy
	type auditEvidence struct {
		status                NotificationQueueStatus
		success               bool
		failureClass, message string
	}
	audits := make(map[string]auditEvidence, len(jobs))
	if err == nil {
		for _, row := range jobs {
			var evidence auditEvidence
			auditErr := q.db.QueryRow(`SELECT status,success,COALESCE(failure_class,''),COALESCE(error_message,'') FROM notification_audit WHERE notification_id=? ORDER BY id DESC LIMIT 1`, row.ID).Scan(&evidence.status, &evidence.success, &evidence.failureClass, &evidence.message)
			if auditErr != nil && !errors.Is(auditErr, sql.ErrNoRows) {
				err = auditErr
				break
			}
			if auditErr == nil {
				audits[row.ID] = evidence
			}
		}
	}
	q.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	var policy func(*alerts.Alert, time.Time) *time.Time
	if provider != nil {
		policy = provider()
	}
	firingEvidence, recoveryEvidence := make(map[string]bool), make(map[string]bool)
	destinationIndices := make(map[string]int, len(run.Destinations))
	for i, destination := range run.Destinations {
		destinationIndices[destination.ID] = i
	}
	for _, row := range jobs {
		job, decodeErr := retainedDeliveryJob(row.Type, row.Config)
		if decodeErr != nil {
			return nil, fmt.Errorf("rehearsal delivery evidence is unreadable")
		}
		_, event := normalizeQueueType(row.Type)
		destination := notificationDestinationIDForJob(job)
		index, known := destinationIndices[destination]
		if !known {
			// A checkpoint can commit queue work before its report projection.
			// Retained queue evidence must remain visible after that write fails.
			retained := rehearsalDestination(job)
			retained.Firing = rehearsalOutcome("unavailable", "admission_unknown", "Retained firing delivery evidence is unavailable for this destination.")
			index = len(run.Destinations)
			run.Destinations = append(run.Destinations, retained)
			destinationIndices[destination] = index
		}
		if event == eventResolved {
			recoveryEvidence[destination] = true
		} else {
			firingEvidence[destination] = true
		}
		out := rehearsalOutcome("pending", "queue", "Waiting for queue processing.")
		out.Attempts = row.Attempts
		switch row.Status {
		case QueueStatusSent:
			out.Status = "passed"
			out.Reason = "accepted"
			out.Message = "Accepted by the destination. Confirm receipt in your inbox or service."
		case QueueStatusFailed, QueueStatusDLQ:
			out.Status = "failed"
			out.Reason = "delivery_failed"
			out.Message = "The destination did not accept this delivery. Inspect recent delivery activity before retrying."
		case QueueStatusCancelled:
			out.Status = "suppressed"
			out.Reason = "cancelled"
			out.Message = "Cancelled by delivery policy or because the rehearsal ended before delivery."
		case QueueStatusSending:
			out.Reason = "sending"
			out.Message = "A delivery attempt is in progress."
		case QueueStatusPending:
			if row.Attempts > 0 {
				out.Reason = "retry"
				out.Message = "An attempt failed. The ordinary queue will retry."
			}
			if row.NextRetryAt != nil {
				out.NextAt = row.NextRetryAt
				if row.NextRetryAt.After(time.Now()) {
					out.Status = "deferred"
					out.Reason = "scheduled"
					out.Message = "Waiting for the configured grouping, quiet-hours or retry deadline."
				}
			}
			if policy != nil {
				if at := policy(run.Alert, time.Now()); at != nil && at.After(time.Now()) {
					out.Status = "deferred"
					out.Reason = "quiet_hours"
					out.Message = "Quiet hours are holding this delivery. The queue rechecks current policy before sending."
					out.NextAt = at
				}
			}
		default:
			out = rehearsalOutcome("unavailable", "queue_state_unknown", "The retained queue status is not recognized. No delivery outcome can be established.")
		}
		if row.LastError != nil && row.Status != QueueStatusSent && row.Status != QueueStatusCancelled {
			out.Diagnostic = RedactWebhookDiagnosticSecrets(*row.LastError)
		}
		if audit, found := audits[row.ID]; found {
			out.FailureClass = audit.failureClass
			// Finishing a rehearsal cancels retryable/terminal queue work, but
			// cannot erase an actual failed provider attempt from its results.
			if row.Status == QueueStatusCancelled && !audit.success {
				if audit.status == QueueStatusDLQ || audit.status == QueueStatusFailed {
					out.Status = "failed"
					out.Reason = "delivery_failed"
				}
				out.Message = "A delivery attempt failed. Further attempts were cancelled when the rehearsal ended."
				if audit.message != "" {
					out.Diagnostic = RedactWebhookDiagnosticSecrets(audit.message)
				}
			}
		}
		if event == eventResolved {
			run.Destinations[index].Recovery = out
		} else {
			run.Destinations[index].Firing = out
		}
	}
	interrupted := false
	for i := range run.Destinations {
		destination := &run.Destinations[i]
		if run.Owner != q.rehearsalOwner && !firingEvidence[destination.ID] && (destination.Firing.Status == "pending" || destination.Firing.Status == "deferred") {
			destination.Firing = rehearsalOutcome("unavailable", "interrupted", "The notification runtime restarted without retained queue admission for this destination. Its firing outcome is unknown.")
			interrupted = true
		}
		if run.RecoveredAt != nil && destination.Recovery.Status == "not_started" {
			destination.Recovery = rehearsalOutcome("pending", "recovery_admission", "The recovery request is recorded. Waiting for its admission evidence.")
		}
		if run.RecoveredAt != nil && run.RecoveryOwner != q.rehearsalOwner && !recoveryEvidence[destination.ID] && (destination.Recovery.Status == "pending" || destination.Recovery.Status == "deferred") {
			destination.Recovery = rehearsalOutcome("unavailable", "recovery_interrupted", "The recovery request was recorded before a notification runtime restart, but no recovery admission was retained. Its outcome is unknown.")
		}
	}
	if interrupted {
		run.Policy = rehearsalOutcome("unavailable", "interrupted", "The notification runtime restarted without complete destination admission evidence. Inspect each result and finish this rehearsal before starting another.")
	}
	if run.Policy.Status == "pending" {
		run.Policy = rehearsalOutcome("unavailable", "admission_unknown", "No completed notification admission was recorded. Refresh to reconcile this request before starting another rehearsal.")
	}
	result := run.DeliveryRehearsal
	return &result, nil
}

func rehearsalExpired(batch []*alerts.Alert, now time.Time) bool {
	for _, alert := range batch {
		if rehearsalID(alert) != "" && now.Sub(alert.StartTime) >= rehearsalRetention {
			return true
		}
	}
	return false
}

func (q *NotificationQueue) rehearsalEvidenceUnavailable(batch []*alerts.Alert) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	for _, alert := range batch {
		if _, failed := q.rehearsalErrors[rehearsalID(alert)]; failed {
			return true
		}
	}
	return false
}

// Expiration retires this producer's work through the queue's existing
// cancellation gates before removing history. Accepted attempts retain their
// normal settlement authority. No provider request or recovery is fabricated.
func (q *NotificationQueue) expireRehearsals() error {
	q.mu.RLock()
	rows, err := q.db.Query(`SELECT id,data FROM notification_rehearsals WHERE created_at < ?`, time.Now().Add(-rehearsalRetention).Unix())
	if err != nil {
		q.mu.RUnlock()
		return err
	}
	var expired []storedRehearsal
	for rows.Next() {
		var id string
		var raw []byte
		if err = rows.Scan(&id, &raw); err != nil {
			break
		}
		var run storedRehearsal
		if err = json.Unmarshal(raw, &run); err != nil {
			break
		}
		if run.Alert == nil || run.Alert.ID != "delivery-rehearsal:"+id {
			err = fmt.Errorf("expired rehearsal identity is unreadable")
			break
		}
		expired = append(expired, run)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	q.mu.RUnlock()
	if err != nil {
		return err
	}
	for _, run := range expired {
		if _, err = q.CancelByAlertOccurrence(run.Alert.ID, run.Alert.StartTime); err != nil {
			return err
		}
		if _, err = q.cancelOccurrenceNotifications([]string{run.Alert.ID}, func(a *alerts.Alert) bool { return a.ID == run.Alert.ID && a.StartTime.Equal(run.Alert.StartTime) }, eventResolved); err != nil {
			return err
		}
		q.mu.Lock()
		_, err = q.db.Exec(`DELETE FROM notification_rehearsals WHERE id=?`, run.ID)
		if err == nil {
			delete(q.rehearsalErrors, run.ID)
		}
		q.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}
