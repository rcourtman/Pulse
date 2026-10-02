package notifications

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
)

// Use the real persistent schema and delivery processor, without a background
// ticker racing these controlled replay instants. The monitor constructor test
// covers installing this policy on the autonomous worker after saved-config load.
func newQuietReplayQueue(t *testing.T, dir string, now *time.Time) *NotificationQueue {
	t.Helper()
	db, err := sql.Open("sqlite", queueSQLiteDSN(filepath.Join(dir, "replay.db")))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	q := &NotificationQueue{db: db, deliveryGates: make(map[string]*notificationDeliveryGate),
		notifyChan: make(chan struct{}, 100), now: func() time.Time { return *now }}
	if err := q.initSchema(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return q
}

func quietReplaySchedule(start, end, zone string) alerts.QuietHours {
	return alerts.QuietHours{Enabled: true, Start: start, End: end, Timezone: zone,
		Days: map[string]bool{"sunday": true, "monday": true, "tuesday": true,
			"wednesday": true, "thursday": true, "friday": true, "saturday": true}}
}

func quietReplayAlert(id string, level alerts.AlertLevel, start time.Time) *alerts.Alert {
	return &alerts.Alert{ID: id, Type: "cpu", Level: level, StartTime: start,
		ResourceID: id, ResourceName: id,
		OperationalRecord: &operationaltrust.OperationalRecord{ID: "record-" + id},
		LatestTransition: &operationaltrust.LifecycleTransition{ID: "transition-" + id,
			To: operationaltrust.OperationalOpen, CauseKey: "cause-" + id}}
}

type quietReplayReceipt struct {
	Event  string          `json:"event"`
	Alerts []*alerts.Alert `json:"alerts"`
}

type quietReplaySink struct {
	mu       sync.Mutex
	receipts []quietReplayReceipt
}

func (s *quietReplaySink) all() []quietReplayReceipt {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]quietReplayReceipt(nil), s.receipts...)
}

func newQuietReplayNotifier(t *testing.T, q *NotificationQueue, policyOwner *alerts.Manager) (*NotificationManager, WebhookConfig, *quietReplaySink) {
	t.Helper()
	sink := &quietReplaySink{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var receipt quietReplayReceipt
		if err := json.NewDecoder(r.Body).Decode(&receipt); err != nil {
			t.Errorf("decode local delivery: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sink.mu.Lock()
		sink.receipts = append(sink.receipts, receipt)
		sink.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "admitted-ops", Enabled: true, Service: "generic", URL: server.URL}
	n := &NotificationManager{enabled: true, queue: q, webhooks: []WebhookConfig{hook},
		webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit),
		lastNotified: make(map[string]notificationRecord), deliveryReceipts: make(map[string]struct{})}
	if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	n.SetQuietHoursPolicyProvider(policyOwner.QuietHoursNotificationPolicy)
	q.processor = n.ProcessQueuedNotification
	return n, hook, sink
}

func loadQuietReplay(t *testing.T, q *NotificationQueue, id string) *QueuedNotification {
	t.Helper()
	n, err := q.scanNotification(q.db.QueryRow(`SELECT id, type, method, status, alerts, config,
  attempts, max_attempts, last_attempt, last_error, created_at, next_retry_at,
  completed_at, payload_bytes, operational_links FROM notification_queue WHERE id = ?`, id))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func quietReplayRows(t *testing.T, q *NotificationQueue) []*QueuedNotification {
	t.Helper()
	rows, err := q.db.Query(`SELECT id, type, method, status, alerts, config,
  attempts, max_attempts, last_attempt, last_error, created_at, next_retry_at,
  completed_at, payload_bytes, operational_links FROM notification_queue ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []*QueuedNotification
	for rows.Next() {
		n, err := q.scanNotification(rows)
		if err != nil {
			t.Fatal(err)
		}
		result = append(result, n)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func seedQuietReplay(t *testing.T, q *NotificationQueue, hook WebhookConfig, now time.Time, batch ...*alerts.Alert) *QueuedNotification {
	t.Helper()
	config, err := json.Marshal(hook)
	if err != nil {
		t.Fatal(err)
	}
	due := now.Add(-time.Hour)
	n := &QueuedNotification{ID: "admitted-batch", Type: "webhook", DestinationID: "webhook:" + hook.ID,
		Alerts: batch, Config: config, MaxAttempts: 3, CreatedAt: now.Add(-24 * time.Hour), NextRetryAt: &due}
	if err := q.Enqueue(n); err != nil {
		t.Fatal(err)
	}
	return n
}

func setQuietReplaySchedule(m *alerts.Manager, quiet alerts.QuietHours) {
	config := m.GetConfig()
	config.Schedule.QuietHours = quiet
	m.UpdateConfig(config)
}

func TestQueuedQuietHoursRevalidatesCurrentSchedule(t *testing.T) {
	for _, tc := range []struct {
		name, now, start, end, zone string
	}{
		{"continuous_midnight", "2026-10-02T00:00:00Z", "00:00", "23:59", "UTC"},
		{"continuous_offset_boundary", "2026-10-02T12:00:00Z", "12:00", "11:59", "UTC"},
		{"late_replay_next_night", "2026-10-02T22:30:00Z", "22:00", "06:00", "UTC"},
		{"edited_longer_window", "2026-10-02T07:00:00Z", "22:00", "08:00", "UTC"},
		{"repeated_end_minute", "2026-10-25T01:45:00Z", "01:30", "02:00", "Europe/London"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.now)
			if err != nil {
				t.Fatal(err)
			}
			owner := alerts.NewManagerWithDataDir(t.TempDir())
			defer owner.Stop()
			setQuietReplaySchedule(owner, quietReplaySchedule(tc.start, tc.end, tc.zone))
			q := newQuietReplayQueue(t, t.TempDir(), &now)
			_, hook, sink := newQuietReplayNotifier(t, q, owner)
			alert := quietReplayAlert("warning", alerts.AlertLevelWarning, now.Add(-2*time.Hour))
			alert.Metadata = map[string]interface{}{alerts.MetadataQuietHoursReplayAt: now.Add(-time.Hour).Format(time.RFC3339)}
			n := seedQuietReplay(t, q, hook, now, alert)
			before := loadQuietReplay(t, q, n.ID)
			stale := *before
			q.processNotification(n)
			after := loadQuietReplay(t, q, before.ID)
			if got := sink.all(); len(got) != 0 {
				t.Fatalf("QUIET_REPLAY_LEAK: held notification reached provider: %+v", got)
			}
			if after.Status != QueueStatusPending || after.Attempts != 0 || after.LastAttempt != nil ||
				after.NextRetryAt == nil || !after.NextRetryAt.After(now) {
				t.Fatalf("hold counted as delivery or did not move replay: %+v", after)
			}
			if !reflect.DeepEqual(before.Alerts, after.Alerts) || string(before.Config) != string(after.Config) ||
				!reflect.DeepEqual(before.Links, after.Links) {
				t.Fatal("quiet hold changed occurrence, destination or operational identity")
			}
			for range 3 {
				copy := stale
				q.processNotification(&copy)
			}
			if repeated := loadQuietReplay(t, q, before.ID); !reflect.DeepEqual(after, repeated) {
				t.Fatal("stale worker snapshot consumed an attempt or moved a held row again")
			}
			logs, err := q.GetDeliveryLog(before.CreatedAt, 10)
			if err != nil || len(logs) != 0 {
				t.Fatalf("quiet hold manufactured delivery audit: %+v, %v", logs, err)
			}
			// A continuous schedule never gets an invented open minute. Once the
			// operator disables it, the next admitted wake reuses the same row.
			setQuietReplaySchedule(owner, alerts.QuietHours{})
			now = after.NextRetryAt.Add(time.Second)
			q.processNotification(&stale)
			if got := sink.all(); len(got) != 1 || len(got[0].Alerts) != 1 || got[0].Alerts[0].ID != alert.ID ||
				!got[0].Alerts[0].StartTime.Equal(alert.StartTime) {
				t.Fatalf("eligible replay did not preserve occurrence: %+v", got)
			}
			if sent := loadQuietReplay(t, q, before.ID); sent.Status != QueueStatusSent || sent.Attempts != 1 {
				t.Fatalf("eligible replay attempt: %+v", sent)
			}
		})
	}
}

func TestQueuedQuietHoursMixedBatchPreservesDeliveryAndBudget(t *testing.T) {
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	owner := alerts.NewManagerWithDataDir(t.TempDir())
	defer owner.Stop()
	setQuietReplaySchedule(owner, quietReplaySchedule("22:00", "06:00", "UTC"))
	q := newQuietReplayQueue(t, t.TempDir(), &now)
	notifier, hook, sink := newQuietReplayNotifier(t, q, owner)
	warning := quietReplayAlert("warning", alerts.AlertLevelWarning, now.Add(-time.Hour))
	critical := quietReplayAlert("critical", alerts.AlertLevelCritical, now.Add(-time.Minute))
	n := seedQuietReplay(t, q, hook, now, warning, critical)
	for range 2 {
		if err := q.IncrementAttemptAndSetStatus(n.ID, QueueStatusSending); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.ScheduleRetry(n.ID, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := q.db.Exec(`UPDATE notification_queue SET next_retry_at = ?, last_error = 'destination unavailable' WHERE id = ?`, now.Add(-time.Second).Unix(), n.ID); err != nil {
		t.Fatal(err)
	}
	before := loadQuietReplay(t, q, n.ID)
	if err := q.RecordAudit(before, false, "destination unavailable"); err != nil {
		t.Fatal(err)
	}
	originalID := n.ID
	q.processNotification(n)
	got := sink.all()
	if len(got) != 1 || len(got[0].Alerts) != 1 || got[0].Alerts[0].ID != critical.ID {
		t.Fatalf("QUIET_MIXED_BATCH: immediately eligible subset was delayed or held member leaked: %+v", got)
	}
	rows := quietReplayRows(t, q)
	if len(rows) != 2 {
		t.Fatalf("mixed batch rows=%d, want held original plus ready child", len(rows))
	}
	held := loadQuietReplay(t, q, originalID)
	if len(held.Alerts) != 1 || held.Alerts[0].ID != warning.ID || held.Status != QueueStatusPending ||
		held.Attempts != 2 || held.MaxAttempts != 3 || !reflect.DeepEqual(held.LastAttempt, before.LastAttempt) ||
		!reflect.DeepEqual(held.LastError, before.LastError) {
		t.Fatalf("held partition lost its admission/attempt history: %+v", held)
	}
	if held.NextRetryAt == nil || !held.NextRetryAt.Equal(time.Date(2026, 10, 2, 6, 1, 0, 0, time.UTC)) {
		t.Fatalf("held replay deadline=%v, want inclusive end at 06:01", held.NextRetryAt)
	}
	child := loadQuietReplay(t, q, n.ID)
	if child.ID == originalID || child.Attempts != 3 || child.Status != QueueStatusSent || child.MaxAttempts != 3 {
		t.Fatalf("ready partition reset budget or did not complete: %+v", child)
	}
	for _, row := range []*QueuedNotification{held, child} {
		if string(row.Config) != string(before.Config) || row.DestinationID != before.DestinationID || !row.CreatedAt.Equal(before.CreatedAt) || len(row.Links) != 1 {
			t.Fatalf("partition lost admitted destination, time or links: %+v", row)
		}
		link := row.Links[0]
		if err := link.Validate(); err != nil || link.NotificationID != row.ID ||
			link.OperationalRecordID != row.Alerts[0].OperationalRecord.ID ||
			link.TransitionID != row.Alerts[0].LatestTransition.ID || link.DestinationID != before.DestinationID {
			t.Fatalf("partition misattributed operational receipt: %+v, %v", link, err)
		}
	}
	if held.Links[0].DeliveryState != operationaltrust.NotificationRetrying || child.Links[0].DeliveryState != operationaltrust.NotificationDelivered {
		t.Fatal("held link gained a delivery or lost its existing retry evidence")
	}
	jobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{hook}, AppriseConfig{}, []*alerts.Alert{warning, critical}, eventResolved, now)
	filtered := notifier.filterResolvedJobsByDeliveryReceipt(jobs)
	if len(filtered) != 1 || len(filtered[0].Alerts) != 1 || filtered[0].Alerts[0].ID != critical.ID {
		t.Fatalf("held member gained a firing receipt: %+v", filtered)
	}
	// Release at its real schedule boundary: only the held occurrence gets its
	// third provider attempt, with no repetition of the already sent child.
	now = *held.NextRetryAt
	q.processNotification(held)
	if got := sink.all(); len(got) != 2 || got[1].Alerts[0].ID != warning.ID || len(got[1].Alerts) != 1 {
		t.Fatalf("released partition delivery: %+v", got)
	}
	if done := loadQuietReplay(t, q, originalID); done.Attempts != 3 || done.Status != QueueStatusSent {
		t.Fatalf("held retry budget after release: %+v", done)
	}
}

func TestQueuedQuietHoursPartitionRollbackAndRestart(t *testing.T) {
	for _, failure := range []string{"ready_insert", "held_rewrite"} {
		t.Run(failure, func(t *testing.T) {
			now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
			owner := alerts.NewManagerWithDataDir(t.TempDir())
			defer owner.Stop()
			setQuietReplaySchedule(owner, quietReplaySchedule("22:00", "06:00", "UTC"))
			dir := t.TempDir()
			q := newQuietReplayQueue(t, dir, &now)
			_, hook, sink := newQuietReplayNotifier(t, q, owner)
			n := seedQuietReplay(t, q, hook, now,
				quietReplayAlert("warning", alerts.AlertLevelWarning, now.Add(-time.Hour)),
				quietReplayAlert("critical", alerts.AlertLevelCritical, now.Add(-time.Minute)))
			before := loadQuietReplay(t, q, n.ID)
			trigger := `CREATE TRIGGER reject_partition BEFORE INSERT ON notification_queue WHEN NEW.id != 'admitted-batch' BEGIN SELECT RAISE(ABORT, 'fixture partition persistence failed'); END`
			if failure == "held_rewrite" {
				trigger = `CREATE TRIGGER reject_partition BEFORE UPDATE OF alerts ON notification_queue BEGIN SELECT RAISE(ABORT, 'fixture partition persistence failed'); END`
			}
			if _, err := q.db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			q.processNotification(n)
			if got := sink.all(); len(got) != 0 {
				t.Fatalf("QUIET_PARTITION_FAILURE: persistence failure still attempted delivery: %+v", got)
			}
			if rows := quietReplayRows(t, q); len(rows) != 1 || !reflect.DeepEqual(rows[0], before) {
				t.Fatalf("failed transaction did not retain original complete batch: %+v", rows)
			}
			if _, err := q.db.Exec("DROP TRIGGER reject_partition"); err != nil {
				t.Fatal(err)
			}
			q.processNotification(n)
			if got := sink.all(); len(got) != 1 || len(got[0].Alerts) != 1 || got[0].Alerts[0].ID != "critical" {
				t.Fatalf("successful partition: %+v", got)
			}
			if err := q.db.Close(); err != nil {
				t.Fatal(err)
			}
			q = newQuietReplayQueue(t, dir, &now)
			_, _, restartedSink := newQuietReplayNotifier(t, q, owner)
			rows := quietReplayRows(t, q)
			if len(rows) != 2 {
				t.Fatalf("reopen lost partition: %+v", rows)
			}
			held := loadQuietReplay(t, q, before.ID)
			if count, err := q.CancelByAlertIdentifiers([]string{"warning"}); err != nil || count != 1 {
				t.Fatalf("cancellation of held member: %d, %v", count, err)
			}
			now = held.NextRetryAt.Add(time.Second)
			q.processNotification(held)
			if got := restartedSink.all(); len(got) != 0 {
				t.Fatalf("cancelled held member resurrected after reopen: %+v", got)
			}
			if cancelled := loadQuietReplay(t, q, before.ID); cancelled.Status != QueueStatusCancelled || cancelled.Attempts != 0 {
				t.Fatalf("cancelled member acquired attempt: %+v", cancelled)
			}
		})
	}
}

func TestQueuedQuietHoursConcurrentSnapshotsAndCancellation(t *testing.T) {
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	owner := alerts.NewManagerWithDataDir(t.TempDir())
	defer owner.Stop()
	setQuietReplaySchedule(owner, quietReplaySchedule("22:00", "06:00", "UTC"))
	q := newQuietReplayQueue(t, t.TempDir(), &now)
	_, hook, sink := newQuietReplayNotifier(t, q, owner)
	n := seedQuietReplay(t, q, hook, now,
		quietReplayAlert("cancelled", alerts.AlertLevelWarning, now.Add(-time.Hour)),
		quietReplayAlert("held", alerts.AlertLevelWarning, now.Add(-time.Hour)),
		quietReplayAlert("ready", alerts.AlertLevelCritical, now.Add(-time.Minute)))
	snapshotTaken := make(chan struct{})
	continueDelivery := make(chan struct{})
	q.quietHoursPolicy = func() func(*alerts.Alert, time.Time) *time.Time {
		policy := owner.QuietHoursNotificationPolicy()
		close(snapshotTaken)
		<-continueDelivery
		return policy
	}
	done := make(chan struct{})
	go func() { q.processNotification(n); close(done) }()
	<-snapshotTaken
	if count, err := q.CancelByAlertIdentifiers([]string{"cancelled"}); err != nil || count != 1 {
		t.Fatalf("cancellation between snapshot and claim: %d, %v", count, err)
	}
	close(continueDelivery)
	<-done
	q.quietHoursPolicy = owner.QuietHoursNotificationPolicy
	held := loadQuietReplay(t, q, "admitted-batch")
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := *held
			q.processNotification(&copy)
		}()
	}
	wg.Wait()
	if got := sink.all(); len(got) != 1 || len(got[0].Alerts) != 1 || got[0].Alerts[0].ID != "ready" {
		t.Fatalf("QUIET_CANCELLATION: stale snapshots lost cancellation or duplicated delivery: %+v", got)
	}
	if len(held.Alerts) != 1 || held.Alerts[0].ID != "held" || held.Attempts != 0 || len(held.Links) != 1 || held.Links[0].TransitionID != "transition-held" {
		t.Fatalf("held remainder resurrected removed member or consumed budget: %+v", held)
	}
	if count, err := q.CancelByAlertIdentifiers([]string{"held"}); err != nil || count != 1 {
		t.Fatalf("post-partition cancellation: %d, %v", count, err)
	}
	now = held.NextRetryAt.Add(time.Second)
	q.processNotification(held)
	if got := sink.all(); len(got) != 1 {
		t.Fatalf("post-partition cancellation leaked delivery: %+v", got)
	}
}

func TestQueuedQuietHoursResolvedKeepsUndeliveredReceipt(t *testing.T) {
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	owner := alerts.NewManagerWithDataDir(t.TempDir())
	defer owner.Stop()
	setQuietReplaySchedule(owner, quietReplaySchedule("22:00", "06:00", "UTC"))
	q := newQuietReplayQueue(t, t.TempDir(), &now)
	notifier, hook, sink := newQuietReplayNotifier(t, q, owner)
	warning := quietReplayAlert("warning", alerts.AlertLevelWarning, now.Add(-2*time.Hour))
	critical := quietReplayAlert("critical", alerts.AlertLevelCritical, now.Add(-time.Hour))
	batch := []*alerts.Alert{warning, critical}
	firing := notificationDeliveryJob{Type: "webhook", Event: eventAlert, Alerts: batch, WebhookConfig: &hook}
	notifier.recordSuccessfulDelivery(firing, now.Add(-time.Hour))
	for _, alert := range batch {
		alert.LatestTransition.To = operationaltrust.OperationalResolved
		annotateResolvedMetadata(alert, now.Add(-time.Minute))
	}
	n := seedQuietReplay(t, q, hook, now, batch...)
	if _, err := q.db.Exec(`UPDATE notification_queue SET type = 'webhook_resolved', method = 'resolved-group' WHERE id = ?`, n.ID); err != nil {
		t.Fatal(err)
	}
	q.processNotification(n)
	if got := sink.all(); len(got) != 1 || got[0].Event != "resolved" || len(got[0].Alerts) != 1 || got[0].Alerts[0].ID != critical.ID {
		t.Fatalf("QUIET_RECOVERY: grouped recovery leaked held member or lost ready member: %+v", got)
	}
	held := loadQuietReplay(t, q, "admitted-batch")
	if held.Method != "quiet-hours-replay" || held.Attempts != 0 {
		t.Fatalf("postponed recovery reopened grouping window or consumed attempt: %+v", held)
	}
	resolved := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: batch, WebhookConfig: &hook}
	if eligible := notifier.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(eligible) != 1 || len(eligible[0].Alerts) != 1 || eligible[0].Alerts[0].ID != warning.ID {
		t.Fatalf("held recovery lost the original occurrence receipt: %+v", eligible)
	}
	if count, err := q.CancelByAlertIdentifiers([]string{warning.ID}); err != nil || count != 0 {
		t.Fatalf("firing cancellation consumed admitted recovery: %d, %v", count, err)
	}
	now = *held.NextRetryAt
	q.processNotification(held)
	if got := sink.all(); len(got) != 2 || got[1].Event != "resolved" || len(got[1].Alerts) != 1 || got[1].Alerts[0].ID != warning.ID {
		t.Fatalf("held recovery did not complete once: %+v", got)
	}
	if eligible := notifier.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{resolved}); len(eligible) != 0 {
		t.Fatalf("delivered recovery did not consume original receipts: %+v", eligible)
	}
}

func TestQueuedQuietHoursProviderFamiliesAndLegacyRows(t *testing.T) {
	for _, kind := range []string{"email", "webhook", "apprise", "email_resolved", "webhook_resolved", "apprise_resolved"} {
		t.Run(kind, func(t *testing.T) {
			now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			owner := alerts.NewManagerWithDataDir(t.TempDir())
			defer owner.Stop()
			setQuietReplaySchedule(owner, quietReplaySchedule("00:00", "23:59", "UTC"))
			q := newQuietReplayQueue(t, t.TempDir(), &now)
			notifier, hook, sink := newQuietReplayNotifier(t, q, owner)
			notifier.emailConfig.Enabled = true
			notifier.appriseConfig.Enabled = true
			n := seedQuietReplay(t, q, hook, now, quietReplayAlert("legacy-warning", alerts.AlertLevelWarning, now.Add(-time.Hour)))
			// Legacy payloads have no operational links or quiet replay metadata;
			// they are still governed by the current policy, not a schema marker.
			if _, err := q.db.Exec(`UPDATE notification_queue SET type = ?, config = '{}', operational_links = '[]' WHERE id = ?`, kind, n.ID); err != nil {
				t.Fatal(err)
			}
			q.processNotification(n)
			after := loadQuietReplay(t, q, n.ID)
			if after.Status != QueueStatusPending || after.Attempts != 0 || after.NextRetryAt == nil || !after.NextRetryAt.After(now) {
				t.Fatalf("QUIET_PROVIDER_FAMILY: %s bypassed quiet revalidation: %+v", kind, after)
			}
			if got := sink.all(); len(got) != 0 {
				t.Fatalf("legacy quiet row leaked: %+v", got)
			}
		})
	}
}

func TestQueuedQuietHoursRejectsAmbiguousLinks(t *testing.T) {
	now := time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC)
	owner := alerts.NewManagerWithDataDir(t.TempDir())
	defer owner.Stop()
	setQuietReplaySchedule(owner, quietReplaySchedule("22:00", "06:00", "UTC"))
	q := newQuietReplayQueue(t, t.TempDir(), &now)
	_, hook, sink := newQuietReplayNotifier(t, q, owner)
	n := seedQuietReplay(t, q, hook, now,
		quietReplayAlert("held", alerts.AlertLevelWarning, now.Add(-time.Hour)),
		quietReplayAlert("ready", alerts.AlertLevelCritical, now.Add(-time.Minute)))
	links := append([]operationaltrust.NotificationLink(nil), n.Links...)
	links[0].OperationalRecordID = "orphaned-record"
	encoded, err := json.Marshal(links)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.db.Exec(`UPDATE notification_queue SET operational_links = ? WHERE id = ?`, string(encoded), n.ID); err != nil {
		t.Fatal(err)
	}
	before := loadQuietReplay(t, q, n.ID)
	ready, err := q.prepareQuietHoursDelivery(n, owner.QuietHoursNotificationPolicy())
	if ready || err == nil || !strings.Contains(err.Error(), "ambiguous operational link") {
		t.Fatalf("orphaned operational link was silently discarded: ready=%t err=%v", ready, err)
	}
	if got := sink.all(); len(got) != 0 || !reflect.DeepEqual(before, loadQuietReplay(t, q, before.ID)) {
		t.Fatal("rejected linkage still changed batch or attempted delivery")
	}
}
