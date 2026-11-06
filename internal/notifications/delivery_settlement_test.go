package notifications

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func TestNotificationAcknowledgedStatusFailureRetriesLocally(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := deferredContinuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	q.installProcessor(n.ProcessQueuedNotification)
	_, err := q.db.Exec(`CREATE TRIGGER reject_completion BEFORE UPDATE OF status ON notification_queue WHEN NEW.status='sent' BEGIN SELECT RAISE(ABORT,'synthetic completion persistence failure'); END`)
	require.NoError(t, err)
	n.SendAlert(&alerts.Alert{ID: "completion-pending", StartTime: time.Now(), Level: alerts.AlertLevelCritical})
	q.processBatch()
	require.Eventually(t, func() bool {
		stats, err := q.GetQueueStats()
		return err == nil && stats["pending"] == 1 && stats["settlement_pending"] == 1 && stats["sending"] == 0
	}, 5*time.Second, time.Millisecond)
	// Simulate elapsed local-repair time without another provider attempt.
	originalAttempt := time.Now().Add(-time.Hour).Unix()
	_, err = q.db.Exec(`UPDATE notification_queue SET last_attempt=?`, originalAttempt)
	require.NoError(t, err)
	for retry := 0; retry < 2; retry++ {
		retryContinuitySettlementNow(t, q)
		q.processBatch()
	}
	var attempts, audits int
	require.NoError(t, q.db.QueryRow(`SELECT attempts FROM notification_queue`).Scan(&attempts))
	require.Equal(t, 1, attempts)
	require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE success=1`).Scan(&audits))
	require.Equal(t, 1, audits)
	require.False(t, n.DeliveryHealth().Healthy)
	_, err = q.db.Exec(`DROP TRIGGER reject_completion`)
	require.NoError(t, err)
	retryContinuitySettlementNow(t, q)
	q.processBatch()
	drainContinuity(t, n)
	stats, err := q.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats["sent"])
	require.Zero(t, stats["settlement_pending"])
	var lastAttempt int64
	require.NoError(t, q.db.QueryRow(`SELECT last_attempt FROM notification_queue`).Scan(&lastAttempt))
	require.Equal(t, originalAttempt, lastAttempt, "local completion changed the recorded provider attempt time")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count, "completion settlement contacted an acknowledged provider again")
}

func TestNotificationBootstrapPreservesStorageAndConfigurationHolds(t *testing.T) {
	n := &NotificationManager{}
	n.HoldStorageAuthority(fmt.Errorf("synthetic storage obstruction"))
	n.HoldIncompleteConfiguration(fmt.Errorf("synthetic unreadable configuration"))
	require.Equal(t, NotificationReasonStorageBootstrapUnavailable, ConfigurationHoldReason(n.ConfigurationError()))
	require.ErrorContains(t, n.ConfigurationError(), "synthetic storage obstruction")
	require.ErrorContains(t, n.ConfigurationError(), "synthetic unreadable configuration")
}

func TestNotificationFindingOutsideActiveAlertInventoryStillDelivers(t *testing.T) {
	owner := alerts.NewManagerWithDataDir(t.TempDir(), alerts.WithDurableAlertStore())
	t.Cleanup(owner.Stop)
	var calls atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	n.SetOccurrenceLifecycleProvider(owner.NotificationOccurrenceLifecycle)
	finding := &alerts.Alert{ID: "patrol-finding-owned-elsewhere", StartTime: time.Now(), Level: alerts.AlertLevelWarning, Message: "A finding from an independent producer"}
	state := owner.NotificationOccurrenceLifecycle(finding.ID, finding.StartTime)
	require.Equal(t, alerts.NotificationOccurrenceUnavailable, state.State)
	require.False(t, state.CurrentInactive, "the alert owner cannot certify a finding's absence")
	n.SendAlert(finding)
	drainContinuity(t, n)
	require.Equal(t, int32(1), calls.Load(), "absence from another producer's active map discarded a new firing")
}

func TestNotificationAcknowledgedSendingRecoversAfterBothStatusWritesFail(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := deferredContinuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	q.installProcessor(n.ProcessQueuedNotification)
	_, err := q.db.Exec(`CREATE TRIGGER reject_completion_and_deferral BEFORE UPDATE OF status ON notification_queue WHEN NEW.status IN ('sent','pending') AND EXISTS(SELECT 1 FROM notification_audit a WHERE a.notification_id=OLD.id AND a.success=1) BEGIN SELECT RAISE(ABORT,'synthetic local status failure'); END`)
	require.NoError(t, err)
	n.SendAlert(&alerts.Alert{ID: "sending-local", StartTime: time.Now(), Level: alerts.AlertLevelCritical})
	q.processBatch()
	require.Eventually(t, func() bool {
		stats, err := q.GetQueueStats()
		return err == nil && stats["sending"] == 1 && stats["settlement_pending"] == 1
	}, 5*time.Second, time.Millisecond)
	require.False(t, n.DeliveryHealth().Healthy)
	// A fallback acceptance record can coexist with a failed terminal status
	// transition. It remains local settlement, including under old retention.
	_, err = q.db.Exec(`UPDATE notification_queue SET status='dlq',completed_at=?; UPDATE notification_audit SET timestamp=?`, time.Now().Add(-31*24*time.Hour).Unix(), time.Now().Add(-31*24*time.Hour).Unix())
	require.NoError(t, err)
	stats, err := q.GetQueueStats()
	require.NoError(t, err)
	require.Zero(t, stats["dlq"], "known acceptance was presented as a terminal provider failure")
	require.Equal(t, 1, stats["settlement_pending"])
	dismissed, err := q.DismissTerminalFailures()
	require.NoError(t, err)
	require.Zero(t, dismissed, "operator terminal dismissal erased accepted settlement")
	q.performCleanup()
	var retained int
	require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE success=1`).Scan(&retained))
	require.Equal(t, 1, retained)
	_, err = q.db.Exec(`DROP TRIGGER reject_completion_and_deferral`)
	require.NoError(t, err)
	q.processBatch()
	drainContinuity(t, n)
	var attempts int
	require.NoError(t, q.db.QueryRow(`SELECT attempts FROM notification_queue`).Scan(&attempts))
	require.Equal(t, 1, attempts)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count, "in-process repair replayed a provider-accepted send")
}

func TestNotificationSendingRepairDoesNotReclaimAnOwnedAttempt(t *testing.T) {
	var deliveries atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := deferredContinuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	entered, unblock := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(unblock) })
	t.Cleanup(release)
	var processors atomic.Int32
	q.installProcessor(func(row *QueuedNotification) error {
		call := processors.Add(1)
		err := n.ProcessQueuedNotification(row)
		if call == 1 {
			if err != nil {
				t.Error(err)
			}
			close(entered)
			<-unblock
		}
		return err
	})
	n.SendAlert(&alerts.Alert{ID: "owned-settlement", StartTime: time.Now(), Level: alerts.AlertLevelCritical})
	// Enqueue wakes the real background owner. A separate manual sweep must
	// not reclaim that owner's accepted attempt while it is still blocked.
	awaitShutdownSignal(t, "accepted attempt still owned by its worker", entered)
	require.True(t, n.DeliveryHealth().Healthy, "live accepted completion was reported as a storage failure")
	q.processBatch()
	require.Equal(t, int32(1), processors.Load(), "repair reclaimed a live accepted attempt")
	var status string
	require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue`).Scan(&status))
	require.Equal(t, string(QueueStatusSending), status)
	release()
	drainContinuity(t, n)
	require.Equal(t, int32(1), processors.Load(), "completion replayed an accepted attempt")
	require.Equal(t, int32(1), deliveries.Load())
	require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue`).Scan(&status))
	require.Equal(t, string(QueueStatusSent), status)
}

func TestNotificationDelayedFiringRequiresPositiveReactivation(t *testing.T) {
	for _, initialState := range []alerts.NotificationOccurrenceState{alerts.NotificationOccurrenceResolved, alerts.NotificationOccurrenceUnavailable, alerts.NotificationOccurrenceActive} {
		t.Run(string(initialState), func(t *testing.T) {
			var deliveries atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(200) }))
			t.Cleanup(server.Close)
			hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
			n := deferredContinuityManager(t, t.TempDir(), hook)
			q := n.GetQueue()
			q.installProcessor(n.ProcessQueuedNotification)
			a := &alerts.Alert{ID: "delayed-firing", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
			n.SendAlert(a)
			q.processBatch()
			drainContinuity(t, n)
			n.CancelResolvedAlert(a)
			// Admit an actual recovery and keep it beyond this controlled replay.
			n.SetGroupingConfig(true, 3600, false, false)
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
			n.SetGroupingConfig(false, 0, false, false)
			n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
				return alerts.NotificationOccurrenceLifecycle{State: initialState, ResolvedAt: time.Now()}
			})
			n.SendAlert(a)
			q.processBatch()
			var admittedFirings int
			require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE type='webhook'`).Scan(&admittedFirings))
			var recoveryStatus string
			require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE type='webhook_resolved'`).Scan(&recoveryStatus))
			if initialState == alerts.NotificationOccurrenceActive {
				require.Equal(t, 2, admittedFirings)
				require.Equal(t, string(QueueStatusCancelled), recoveryStatus)
				drainContinuity(t, n)
				require.Equal(t, int32(2), deliveries.Load())
				return
			}
			require.Equal(t, 1, admittedFirings, "inactive or unavailable authority admitted a fresh replay")
			require.Equal(t, string(QueueStatusPending), recoveryStatus, "stale or uncertain firing cancelled owed recovery")
			require.Equal(t, int32(1), deliveries.Load())
			// Also exercise a stale firing that was already admitted to the queue.
			config, err := json.Marshal(hook)
			require.NoError(t, err)
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
			epoch, err := fixtureDeliveryEpoch(q, key)
			require.NoError(t, err)
			require.NoError(t, q.Enqueue(&QueuedNotification{ID: "delayed-admitted-firing", Type: "webhook", Alerts: []*alerts.Alert{a.Clone()}, Config: config, ReceiptEpoch: &epoch}))
			q.processBatch()
			// A manual sweep can lose the row to the background worker. Wait
			// for that worker's durable deferral or cancellation and release
			// of ownership before checking delivery or changing authority.
			require.Eventually(t, func() bool {
				var settled bool
				if initialState == alerts.NotificationOccurrenceUnavailable {
					stats, err := q.GetQueueStats()
					settled = err == nil && stats["occurrence_pending"] == 1
				} else {
					var status string
					err := q.db.QueryRow(`SELECT status FROM notification_queue WHERE id='delayed-admitted-firing'`).Scan(&status)
					settled = err == nil && status == string(QueueStatusCancelled)
				}
				q.mu.RLock()
				_, owned := q.processingNotifications["delayed-admitted-firing"]
				q.mu.RUnlock()
				return settled && !owned
			}, 10*time.Second, 5*time.Millisecond)
			require.Equal(t, int32(1), deliveries.Load(), "stale admitted firing reached the provider")
			if initialState == alerts.NotificationOccurrenceUnavailable {
				stats, err := q.GetQueueStats()
				require.NoError(t, err)
				require.Equal(t, 1, stats["occurrence_pending"])
				require.Zero(t, stats["settlement_pending"])
				require.Contains(t, n.DeliveryHealth().ReasonCodes, NotificationReasonOccurrenceUnavailable)
				require.NotContains(t, n.DeliveryHealth().ReasonCodes, "notification_storage_unsettled")
			}
			require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE type='webhook_resolved'`).Scan(&recoveryStatus))
			require.Equal(t, string(QueueStatusPending), recoveryStatus)
			n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
				return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceResolved, ResolvedAt: time.Now()}
			})
			_, err = q.db.Exec(`UPDATE notification_queue SET next_retry_at=0 WHERE status='pending'`)
			require.NoError(t, err)
			q.processBatch()
			drainContinuity(t, n)
			require.Equal(t, int32(2), deliveries.Load(), "owed recovery did not reach its original receiver")
			require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE type='webhook_resolved'`).Scan(&recoveryStatus))
			require.Equal(t, string(QueueStatusSent), recoveryStatus, "a stale firing substituted for the owed recovery")
			var staleStatus string
			require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE id='delayed-admitted-firing'`).Scan(&staleStatus))
			require.Equal(t, string(QueueStatusCancelled), staleStatus)
		})
	}
}

func TestNotificationMainWebhookReceiptKeyMigrationGolden(t *testing.T) {
	// This fixture contains actual unchanged-main writer output, schemas and
	// inputs. Seed its three-column table before invoking the candidate migration;
	// the candidate's receipt writer cannot manufacture the compatibility proof.
	type capturedReceipt struct {
		ID          string `json:"alert_id"`
		Start       int64  `json:"alert_start_nanos"`
		Destination string `json:"destination_key"`
		At          int64  `json:"delivered_at"`
		Nanos       int64  `json:"delivered_nanos"`
		Severity    int    `json:"highest_severity"`
		Resolved    int64  `json:"resolved_at"`
	}
	var fixture struct {
		Source          string          `json:"source"`
		Inputs          WebhookConfig   `json:"inputs"`
		ReceiptSchema   string          `json:"receipt_schema"`
		QueueSchema     string          `json:"queue_schema"`
		Receipt         capturedReceipt `json:"receipt"`
		ResolvedReceipt capturedReceipt `json:"resolved_receipt"`
		ResolutionInput time.Time       `json:"resolution_input"`
		Queue           struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Method  string `json:"method"`
			Status  string `json:"status"`
			Alerts  string `json:"alerts"`
			Config  string `json:"config"`
			Created int64  `json:"created_at"`
		} `json:"queue"`
	}
	raw, err := os.ReadFile("testdata/main-notification-continuity.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.Equal(t, "cf9354876e0482a8edc8386557e17e06a99c3aec", fixture.Source)
	require.Equal(t, " ops ", fixture.Inputs.ID)
	require.Equal(t, " https://hooks.example.test/notify ", fixture.Inputs.URL)
	require.Equal(t, "webhook:2ce734b77b65620d03d81b7bfe881c6a", fixture.Receipt.Destination)
	require.NotContains(t, fixture.ReceiptSchema, "delivery_epoch")
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "notification_queue.db"))
	require.NoError(t, err)
	_, err = db.Exec(fixture.ReceiptSchema)
	require.NoError(t, err)
	_, err = db.Exec(fixture.QueueSchema)
	require.NoError(t, err)
	r := fixture.Receipt
	require.NotZero(t, fixture.ResolvedReceipt.Resolved)
	require.Equal(t, fixture.ResolutionInput.Unix(), fixture.ResolvedReceipt.Resolved, "unchanged main resolution writer's actual precision changed")
	for _, captured := range []capturedReceipt{r, fixture.ResolvedReceipt} {
		_, err = db.Exec(`INSERT INTO notification_delivery_receipts(alert_id,alert_start_nanos,destination_key,delivered_at,delivered_nanos,highest_severity,resolved_at) VALUES(?,?,?,?,?,?,?)`, captured.ID, captured.Start, captured.Destination, captured.At, captured.Nanos, captured.Severity, captured.Resolved)
		require.NoError(t, err)
	}
	row := fixture.Queue
	_, err = db.Exec(`INSERT INTO notification_queue(id,type,method,status,alerts,config,created_at) VALUES(?,?,?,?,?,?,?)`, row.ID, row.Type, row.Method, row.Status, row.Alerts, row.Config, row.Created)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	var hook WebhookConfig
	require.NoError(t, json.Unmarshal([]byte(row.Config), &hook))
	hook.Headers["X-Blank"] = "" // benign saved-editor drift must not hold the endpoint
	n := deferredContinuityManager(t, dir, hook)
	q := n.GetQueue()
	a := &alerts.Alert{ID: r.ID, StartTime: time.Unix(0, r.Start), Level: alerts.AlertLevelCritical}
	require.NoError(t, n.reconcileDeliveryDestinations())
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	record, found, err := q.deliveryReceipt(a, key)
	require.NoError(t, err)
	require.True(t, found, "real main key was stranded by receiver-key migration")
	require.Equal(t, r.Severity, notificationSeverityRank(record.level))
	require.Equal(t, r.Nanos, record.lastSent.UnixNano())
	resolved := fixture.ResolvedReceipt
	resolvedAlert := &alerts.Alert{ID: resolved.ID, StartTime: time.Unix(0, resolved.Start), Level: alerts.AlertLevelCritical}
	resolvedRecord, found, err := q.deliveryReceipt(resolvedAlert, key)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, resolvedRecord.resolved, "actual main nonzero resolution was erased during migration")
	var migratedResolution int64
	require.NoError(t, q.db.QueryRow(`SELECT resolved_at FROM notification_delivery_receipts WHERE alert_id=?`, resolved.ID).Scan(&migratedResolution))
	require.Equal(t, resolved.Resolved*int64(time.Second), migratedResolution)
	require.Equal(t, resolved.Nanos, resolvedRecord.lastSent.UnixNano())

	var retainedConfig, retainedAlerts string
	require.NoError(t, q.db.QueryRow(`SELECT config,alerts FROM notification_queue WHERE id=?`, row.ID).Scan(&retainedConfig, &retainedAlerts))
	require.Equal(t, row.Config, retainedConfig)
	require.Equal(t, row.Alerts, retainedAlerts)
}

func TestNotificationReopenCannotCancelANewerResolution(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(fmt.Sprintf("queued_%t", queued), func(t *testing.T) {
			var delivered, newerDelivered atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Event  string         `json:"event"`
					Alerts []alerts.Alert `json:"alerts"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				delivered.Add(1)
				if body.Event == "resolved" && len(body.Alerts) > 0 && body.Alerts[0].Message == "newer resolution proof" {
					newerDelivered.Add(1)
				}
				w.WriteHeader(200)
			}))
			t.Cleanup(server.Close)
			hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
			n := deferredContinuityManager(t, t.TempDir(), hook)
			q := n.GetQueue()
			q.installProcessor(n.ProcessQueuedNotification)
			a := &alerts.Alert{ID: "resolution-race", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
			n.SendAlert(a)
			q.processBatch()
			drainContinuity(t, n)
			n.CancelResolvedAlert(a)
			n.SetGroupingConfig(true, 3600, false, false)
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
			n.SetGroupingConfig(false, 0, false, false)
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
			epoch, err := fixtureDeliveryEpoch(q, key)
			require.NoError(t, err)
			config, err := json.Marshal(hook)
			require.NoError(t, err)
			var advanced atomic.Bool
			n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
				if advanced.CompareAndSwap(false, true) {
					// A second resolution and its admitted recovery arrive after the
					// Active snapshot was taken. Its wall clock moved backwards.
					at := time.Now().Add(-time.Hour)
					if err := q.markReceiptOccurrenceResolved(a.ID, a.StartTime, at); err != nil {
						t.Error(err)
						return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
					}
					resolved := a.Clone()
					resolved.Message = "newer resolution proof"
					annotateResolvedMetadata(resolved, at)
					due := time.Now().Add(time.Hour)
					if err := q.Enqueue(&QueuedNotification{ID: "newer-resolution", Type: "webhook_resolved", Alerts: []*alerts.Alert{resolved}, Config: config, ReceiptEpoch: &epoch, NextRetryAt: &due}); err != nil {
						t.Error(err)
						return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
					}
					return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
				}
				return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceResolved, CurrentInactive: true, ResolvedAt: time.Now()}
			})
			if queued {
				require.NoError(t, q.Enqueue(&QueuedNotification{ID: "stale-firing", Type: "webhook", Alerts: []*alerts.Alert{a.Clone()}, Config: config, ReceiptEpoch: &epoch}))
				q.processBatch()
				// The queue's normal wakeup can own this row before our explicit
				// batch. Wait for its guarded disposition, not merely our batch's
				// return while that other worker is still inside the provider.
				require.Eventually(t, func() bool {
					var status string
					var reason sql.NullString
					if err := q.db.QueryRow(`SELECT status,last_error FROM notification_queue WHERE id='stale-firing'`).Scan(&status, &reason); err != nil {
						return false
					}
					return status == "cancelled" || (status == "pending" && reason.String == notificationOccurrencePendingMessage)
				}, 5*time.Second, time.Millisecond)
			} else {
				n.SendAlert(a)
			}
			var pending int
			require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE type='webhook_resolved' AND status='pending'`).Scan(&pending))
			require.Equal(t, 2, pending, "an earlier Active snapshot cancelled a later admitted resolution")
			require.Equal(t, int32(1), delivered.Load(), "stale firing reached the provider after the later resolution")
			_, err = q.db.Exec(`UPDATE notification_queue SET next_retry_at=0 WHERE status='pending'`)
			require.NoError(t, err)
			q.processBatch()
			drainContinuity(t, n)
			require.Equal(t, int32(1), newerDelivered.Load(), "the newer admitted recovery never reached its receiver")
			var final string
			require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE id='newer-resolution'`).Scan(&final))
			require.Equal(t, string(QueueStatusSent), final)
			if queued {
				require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE id='stale-firing'`).Scan(&final))
				require.Equal(t, string(QueueStatusCancelled), final)
			}
		})
	}
}

func TestNotificationReopenSurvivesCompactedResolutionFacts(t *testing.T) {
	var calls atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	a := &alerts.Alert{ID: "facts-compaction", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelWarning}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.CancelResolvedAlert(a)
	// Only the accepted audit's fact now carries the old disposition. Deleting
	// that terminal audit exercises the real ON DELETE CASCADE during preflight.
	_, err := q.db.Exec(`UPDATE notification_delivery_receipts SET resolved_at=0,resolution_order=0 WHERE alert_id=?`, a.ID)
	require.NoError(t, err)
	var once atomic.Bool
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		if once.CompareAndSwap(false, true) {
			_, err := q.db.Exec(`DELETE FROM notification_audit WHERE notification_id IN (SELECT id FROM notification_queue WHERE status='sent')`)
			if err != nil {
				t.Error(err)
			}
		}
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	})
	refire := a.Clone()
	refire.Level = alerts.AlertLevelCritical
	n.SendAlert(refire)
	drainContinuity(t, n)
	require.True(t, once.Load())
	require.Equal(t, int32(2), calls.Load(), "fact compaction discarded a legitimate severity increase")
}

func TestNotificationLegacyRetainedReceiverCannotDonateAuthorityToCurrentAccount(t *testing.T) {
	for _, tc := range []struct {
		status         NotificationQueueStatus
		unidentifiable bool
	}{{QueueStatusPending, false}, {QueueStatusSent, false}, {QueueStatusSent, true}} {
		retainedStatus := tc.status
		t.Run(fmt.Sprintf("%s_unidentifiable_%t", retainedStatus, tc.unidentifiable), func(t *testing.T) {
			var mu sync.Mutex
			counts := make(map[string]int)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Event string `json:"event"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				mu.Lock()
				counts[r.Header.Get("X-Account")+":"+body.Event]++
				mu.Unlock()
				w.WriteHeader(200)
			}))
			t.Cleanup(server.Close)
			oldHook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic", Headers: map[string]string{"X-Account": "A"}}
			current := copyWebhookConfig(oldHook)
			current.Headers["X-Account"] = "B"
			n := deferredContinuityManager(t, t.TempDir(), current)
			q := n.GetQueue()
			legacy, _ := legacyWebhookDestination(oldHook)
			_, err := q.db.Exec(`INSERT INTO notification_destination_state(destination_key,epoch) VALUES(?,'')`, legacy)
			require.NoError(t, err)
			first := &alerts.Alert{ID: "legacy-first", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelCritical}
			second := first.Clone()
			second.ID = "legacy-second"
			for _, a := range []*alerts.Alert{first, second} {
				require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, legacy, time.Now(), 3, ""))
			}
			config, err := json.Marshal(oldHook)
			require.NoError(t, err)
			if tc.unidentifiable {
				config = []byte("null")
			}
			kind := "webhook"
			var bound *string
			var due *time.Time
			if retainedStatus == QueueStatusPending {
				kind = "webhook_resolved"
				epoch := ""
				bound = &epoch
				at := time.Now().Add(time.Hour)
				due = &at
				annotateResolvedMetadata(first, time.Now())
			}
			// This is an explicit pre-upgrade persisted fixture. Its immutable
			// receiver contradicts the sole current account; no prior HTTP is
			// manufactured or counted as evidence by this test.
			require.NoError(t, q.Enqueue(&QueuedNotification{ID: "legacy-retained", Type: kind, Status: retainedStatus, Alerts: []*alerts.Alert{first.Clone()}, Config: config, ReceiptEpoch: bound, NextRetryAt: due}))
			require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
			n.SendAlert(second)
			require.Eventually(t, func() bool {
				var sent int
				return q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE id<>'legacy-retained' AND type='webhook' AND status='sent'`).Scan(&sent) == nil && sent == 1
			}, 5*time.Second, time.Millisecond)
			n.CancelResolvedAlert(second)
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: second.Clone(), ResolvedTime: time.Now()})
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: first.Clone(), ResolvedTime: time.Now()})
			require.Eventually(t, func() bool {
				var sent int
				return q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE id<>'legacy-retained' AND type='webhook_resolved' AND status='sent'`).Scan(&sent) == nil && sent == 1
			}, 5*time.Second, time.Millisecond)
			if retainedStatus == QueueStatusPending {
				oldKey := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &oldHook})
				q.performCleanup()
				var pinned int
				require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE destination_key=? AND alert_id=?`, oldKey, first.ID).Scan(&pinned))
				require.Equal(t, 1, pinned, "admitted old recovery lost its retired receiver receipt")
				_, err = q.db.Exec(`UPDATE notification_queue SET next_retry_at=0 WHERE id='legacy-retained'`)
				require.NoError(t, err)
				q.processBatch()
				drainContinuity(t, n)
				require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts WHERE destination_key=? AND alert_id=?`, oldKey, first.ID).Scan(&pinned))
				require.Zero(t, pinned, "old recovery did not consume its exact retired receiver receipt")
			}
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, 1, counts["B:"], "A's historical delivery suppressed B's first firing")
			require.Equal(t, 1, counts["B:resolved"], "B received recovery for A's other occurrence")
			if retainedStatus == QueueStatusPending {
				require.Equal(t, 1, counts["A:resolved"])
			}
		})
	}
}

func TestNotificationCleanupRetainsObservedResolutionBeforeCompaction(t *testing.T) {
	n := deferredContinuityManager(t, t.TempDir())
	q := n.GetQueue()
	a := &alerts.Alert{ID: "resolved-before-compaction", StartTime: time.Now().Add(-48 * time.Hour), Level: alerts.AlertLevelCritical}
	const key = "webhook:compacted-history"
	require.NoError(t, recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now(), 3))
	resolvedAt := time.Now().Add(-24 * time.Hour)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceResolved, ResolvedAt: resolvedAt}
	})
	q.cleanupDeliveryReceipts()
	var retained int64
	require.NoError(t, q.db.QueryRow(`SELECT resolved_at FROM notification_delivery_receipts`).Scan(&retained))
	require.Equal(t, resolvedAt.UnixNano(), retained)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceNoRecord}
	})
	_, err := q.db.Exec(`UPDATE notification_delivery_receipts SET resolved_at=?`, time.Now().Add(-31*24*time.Hour).UnixNano())
	require.NoError(t, err)
	q.cleanupDeliveryReceipts()
	var remaining int
	require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts`).Scan(&remaining))
	require.Zero(t, remaining, "history compaction turned a known resolution into an indefinite hold")
}

func TestNotificationLegacyUnidentifiableAcceptanceCannotBecomeSettled(t *testing.T) {
	var deliveries atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { deliveries.Add(1); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	row := &QueuedNotification{ID: "legacy-unknown-receiver", Type: "webhook", Alerts: []*alerts.Alert{{ID: "known-occurrence", StartTime: time.Now(), Level: alerts.AlertLevelCritical}}, Config: []byte("null")}
	require.NoError(t, q.Enqueue(row))
	// Explicit legacy persisted acceptance fixture: a pre-ledger audit survives,
	// but its retained provider configuration is no longer identifiable.
	require.NoError(t, q.RecordAuditError(row, true, nil))
	_, err = q.db.Exec(`DROP TABLE notification_delivery_facts; ALTER TABLE notification_audit DROP COLUMN receipt_facts_complete; UPDATE notification_queue SET receipt_epoch=''`)
	require.NoError(t, err)
	require.NoError(t, q.Stop())
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, dir, hook)
	q = n.GetQueue()
	q.processBatch()
	var complete bool
	require.NoError(t, q.db.QueryRow(`SELECT receipt_facts_complete FROM notification_audit WHERE success=1`).Scan(&complete))
	require.False(t, complete, "unknown retained receiver was declared reconstructed")
	stats, err := q.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats["settlement_pending"])
	require.Zero(t, stats["sent"])
	require.Equal(t, int32(0), deliveries.Load(), "unidentifiable acceptance caused a new provider attempt")
}

func TestNotificationDeferredOwnerDoesNotActivateVolatileFallbackOnDiskFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(path, []byte("owned path obstruction"), 0600))
	n := NewNotificationManagerWithDeferredQueue("", path)
	t.Cleanup(n.Stop)
	require.ErrorIs(t, n.ConfigurationError(), ErrNotificationStorageAuthority)
	require.Nil(t, n.GetQueue(), "persistent bootstrap silently activated an unrelated volatile queue")
	require.Error(t, n.StartQueueProcessingWithConfiguration(true))
	require.Contains(t, n.DeliveryHealth().ReasonCodes, NotificationReasonStorageBootstrapUnavailable)
}

func TestNotificationDelayedResolutionDispatchCannotRelabelRefire(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "delayed-resolution", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	oldResolution := &alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()}
	n.CancelResolvedAlert(a)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	})
	n.SendAlert(a)
	drainContinuity(t, n)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	})
	// The monitor's asynchronous dispatch can arrive after a same-start refire.
	n.SendResolvedAlert(oldResolution)
	drainContinuity(t, n)
	n.SendAlert(a)
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, count, "late old resolution broke the current firing cooldown")
}

func TestNotificationAcknowledgedRecoverySurvivesRefireCancellation(t *testing.T) {
	var mu sync.Mutex
	var events []string
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Event string `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, body.Event)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "acknowledged-recovery", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	q := n.GetQueue()
	_, err := q.db.Exec(`CREATE TRIGGER reject_recovery_projection BEFORE DELETE ON notification_delivery_receipts BEGIN SELECT RAISE(ABORT,'synthetic recovery projection failure'); END`)
	require.NoError(t, err)
	n.CancelResolvedAlert(a)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	require.Eventually(t, func() bool {
		stats, err := q.GetQueueStats()
		return err == nil && stats["settlement_pending"] == 1 && stats["sending"] == 0
	}, 5*time.Second, time.Millisecond)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	})
	n.SendAlert(a)
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(events) == 3 }, 5*time.Second, time.Millisecond)
	var status string
	require.NoError(t, q.db.QueryRow(`SELECT status FROM notification_queue WHERE type='webhook_resolved'`).Scan(&status))
	require.Equal(t, string(QueueStatusPending), status, "provider-accepted recovery was cancelled before local settlement")
	_, err = q.db.Exec(`DROP TRIGGER reject_recovery_projection`)
	require.NoError(t, err)
	retryContinuitySettlementNow(t, q)
	drainContinuity(t, n)
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	current, found, err := q.deliveryReceipt(a, key)
	require.NoError(t, err)
	require.True(t, found, "old recovery consumed the newer firing")
	require.False(t, current.resolved)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"", "resolved", ""}, events)
}

func TestNotificationSubsecondResolutionDoesNotCancelRecovery(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	fired := time.Now().Add(-time.Minute).Truncate(time.Second).Add(300 * time.Millisecond)
	resolved := fired.Truncate(time.Second).Add(800 * time.Millisecond)
	a := &alerts.Alert{ID: "subsecond", StartTime: fired, Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceResolved, ResolvedAt: resolved, LatestActivationAt: fired}
	})
	n.CancelResolvedAlert(a)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: resolved})
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, count, "original subsecond activation was mistaken for a later refire")
	require.False(t, activationAfterRetainedResolution(fired, resolved.Truncate(time.Second)), "legacy whole-second snapshot invented later activity")
	require.True(t, activationAfterRetainedResolution(resolved.Add(time.Second), resolved.Truncate(time.Second)))
}

func TestNotificationFiringStripsCallerSettlementMetadata(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "metadata", StartTime: time.Now(), Level: alerts.AlertLevelCritical, Metadata: map[string]interface{}{metadataResolvedAt: 17, metadataReceiptRecovered: true}}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.SendAlert(a)
	drainContinuity(t, n)
	require.Equal(t, 17, a.Metadata[metadataResolvedAt], "firing admission mutated caller metadata")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count)
}

func TestNotificationMalformedAcknowledgementIsolatedByDestination(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(n.Stop)
	first := WebhookConfig{ID: "first", URL: "http://127.0.0.1:9/first", Enabled: true}
	second := WebhookConfig{ID: "second", URL: "http://127.0.0.1:9/second", Enabled: true}
	n.AddWebhook(first)
	n.AddWebhook(second)
	q := n.GetQueue()
	a := &alerts.Alert{ID: "isolated", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &first})
	epoch, err := fixtureDeliveryEpoch(q, key)
	require.NoError(t, err)
	require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, key, time.Now(), 3, epoch))
	otherKey := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &second})
	otherEpoch, err := fixtureDeliveryEpoch(q, otherKey)
	require.NoError(t, err)
	healthy := a.Clone()
	healthy.ID = "healthy-same-destination"
	require.NoError(t, q.recordDeliveryReceipt(healthy.ID, healthy.StartTime, otherKey, time.Now(), 3, otherEpoch))
	config, err := json.Marshal(second)
	require.NoError(t, err)
	row := &QueuedNotification{ID: "malformed-ack", Type: "webhook", Alerts: []*alerts.Alert{a}, Config: config, ReceiptEpoch: &otherEpoch}
	require.NoError(t, q.Enqueue(row))
	require.NoError(t, q.RecordAuditError(row, true, nil))
	_, err = q.db.Exec(`UPDATE notification_queue SET alerts='invalid',config='invalid' WHERE id=?`, row.ID)
	require.NoError(t, err)
	_, found, err := q.deliveryReceipt(a, key)
	require.NoError(t, err)
	require.True(t, found)
	_, found, err = q.deliveryReceipt(healthy, otherKey)
	require.NoError(t, err)
	require.True(t, found, "one corrupt queue snapshot poisoned another occurrence at the same destination")
	require.NoError(t, q.markReceiptOccurrenceResolved(a.ID, a.StartTime, time.Now()))
	require.NoError(t, q.deleteDeliveryReceiptEpoch(a.ID, a.StartTime, key, epoch))
	found, err = q.HasDeliveryReceipt(a.ID, a.StartTime, key)
	require.NoError(t, err)
	require.False(t, found)
	stats, err := q.GetQueueStats()
	require.NoError(t, err)
	require.Equal(t, 1, stats["settlement_pending"], "malformed obligation silently disappeared")
	row.Config = []byte("invalid")
	q.SetProcessor(n.ProcessQueuedNotification)
	q.processNotification(row)
	stats, err = q.GetQueueStats()
	require.NoError(t, err)
	require.Zero(t, stats["settlement_pending"])
	require.Equal(t, 1, stats["sent"], "accepted occurrence facts could not settle a corrupt provider snapshot")
	require.Zero(t, stats["dlq"], "accepted-provider configuration corruption became a provider rejection")
}

func TestNotificationLegacyAnonymousWebhookInventoryRetainsAuthority(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(n.Stop)
	hooks := []WebhookConfig{{URL: "http://127.0.0.1:9/first", Enabled: true}, {URL: "http://127.0.0.1:9/second", Enabled: true}}
	a := &alerts.Alert{ID: "legacy-inventory", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	for _, hook := range hooks {
		n.AddWebhook(hook)
		key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
		require.NoError(t, recordFixtureDeliveryReceipt(n.GetQueue(), a.ID, a.StartTime, key, time.Now(), 3))
	}
	require.NoError(t, n.ReplaceWebhooks(hooks))
	for _, hook := range hooks {
		key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
		found, err := n.GetQueue().HasDeliveryReceipt(a.ID, a.StartTime, key)
		require.NoError(t, err)
		require.True(t, found, "unchanged anonymous legacy inventory was retired")
	}
	require.Error(t, ValidateWebhookInventory([]WebhookConfig{{ID: "same"}, {ID: "same"}}))
}

func TestNotificationRelayReceiverUpgradePreservesOnlyKnownReceiverScope(t *testing.T) {
	cases := []struct {
		id       string
		accounts []string
	}{{"", []string{"alpha"}}, {"", []string{"alpha", "beta"}}, {"ops", []string{"alpha"}}, {"ops", []string{"alpha", "beta"}}}
	for _, tc := range cases {
		accounts := tc.accounts
		t.Run(tc.id+"_"+accounts[len(accounts)-1], func(t *testing.T) {
			var mu sync.Mutex
			counts := make(map[string]int)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Event string `json:"event"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					w.WriteHeader(400)
					return
				}
				mu.Lock()
				counts[r.Header.Get("X-Account")+":"+body.Event]++
				mu.Unlock()
				w.WriteHeader(200)
			}))
			t.Cleanup(server.Close)
			var hooks []WebhookConfig
			for _, account := range accounts {
				hooks = append(hooks, WebhookConfig{ID: tc.id, URL: server.URL, Enabled: true, Service: "generic", Headers: map[string]string{"X-Account": account}})
			}
			dir := t.TempDir()
			n := deferredContinuityManager(t, dir, hooks...)
			q := n.GetQueue()
			a := &alerts.Alert{ID: "anonymous-upgrade", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
			legacy, _ := legacyWebhookDestination(hooks[0])
			_, err := q.db.Exec(`INSERT INTO notification_destination_state(destination_key,epoch,fingerprint) VALUES(?,'','')`, legacy)
			require.NoError(t, err)
			require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, legacy, time.Now(), 3, ""))
			require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
			n.SendAlert(a)
			drainContinuity(t, n)
			n.CancelResolvedAlert(a)
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
			drainContinuity(t, n)
			mu.Lock()
			observed := make(map[string]int)
			for key, value := range counts {
				observed[key] = value
			}
			mu.Unlock()
			for _, account := range accounts {
				wantFiring := 0
				if len(accounts) > 1 {
					wantFiring = 1
				}
				require.Equal(t, wantFiring, observed[account+":"], "old ambiguous endpoint evidence suppressed a current receiver")
				require.Equal(t, 1, observed[account+":resolved"])
			}
			if len(accounts) == 1 {
				return
			}
			n.Stop()
			n = continuityManager(t, dir, hooks[0])
			q = n.GetQueue()
			current := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hooks[0]})
			found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, current)
			require.NoError(t, err)
			require.False(t, found, "removing one possible receiver made legacy uncertainty precise")
			var uncertain bool
			require.NoError(t, q.db.QueryRow(`SELECT legacy_uncertain FROM notification_destination_state WHERE destination_key=?`, legacy).Scan(&uncertain))
			require.True(t, uncertain)
			n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
			drainContinuity(t, n)
			mu.Lock()
			resolutions := counts["alpha:resolved"]
			mu.Unlock()
			require.Equal(t, 1, resolutions, "legacy ambiguity authorized a second recovery")
			require.NoError(t, n.ReplaceWebhooks(nil))
			found, err = q.HasDeliveryReceipt(a.ID, a.StartTime, legacy)
			require.NoError(t, err)
			require.False(t, found, "removing the last receiver kept its legacy endpoint authority")
		})
	}
}

func TestNotificationAcknowledgedFiringCanRecoverBeforeReceiptProjection(t *testing.T) {
	var mu sync.Mutex
	var events []string
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value struct {
			Event string `json:"event"`
		}
		if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		events = append(events, value.Event)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	_, err := q.db.Exec(`CREATE TRIGGER reject_receipt BEFORE INSERT ON notification_delivery_receipts BEGIN SELECT RAISE(ABORT,'synthetic receipt projection failure'); END`)
	require.NoError(t, err)
	a := &alerts.Alert{ID: "ack-recovery", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	require.Eventually(t, func() bool {
		stats, err := q.GetQueueStats()
		return err == nil && stats["settlement_pending"] == 1 && stats["sending"] == 0
	}, 5*time.Second, time.Millisecond)
	n.CancelResolvedAlert(a)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return len(events) == 2 }, 5*time.Second, time.Millisecond)
	retryContinuitySettlementNow(t, q)
	drainContinuity(t, n)
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
	require.NoError(t, err)
	require.False(t, found, "late firing projection resurrected consumed recovery authority")
	var audits int
	require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE success=1`).Scan(&audits))
	require.Equal(t, 2, audits)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"", "resolved"}, events, "bookkeeping retry contacted a provider again")
}

func TestNotificationRecoveryRevalidatesPositiveActiveLifecycleWithoutFiringCallback(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "quiet-resume", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.CancelResolvedAlert(a)
	n.SetGroupingConfig(true, 3600, false, false)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	// The alerts owner can resume without choosing to notify again. Its positive
	// active fact must still prevent an obsolete queued recovery.
	n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
		return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
	})
	q := n.GetQueue()
	_, err := q.db.Exec(`UPDATE notification_queue SET next_retry_at=? WHERE status='pending'`, time.Now().Add(-time.Second).Unix())
	require.NoError(t, err)
	q.processBatch()
	drainContinuity(t, n)
	var cancelled int
	require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE type='webhook_resolved' AND status='cancelled'`).Scan(&cancelled))
	require.Equal(t, 1, cancelled)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count)
}

func TestNotificationIdenticalWebhookRestorePreservesDeliveryGeneration(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "unchanged-restore", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	require.NoError(t, n.ReplaceWebhooks([]WebhookConfig{hook}))
	n.SendAlert(a)
	drainContinuity(t, n)
	n.CancelResolvedAlert(a)
	n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a.Clone(), ResolvedTime: time.Now()})
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, count, "unchanged restore lost cooldown or recovery authority")
}

func TestNotificationMalformedRecoveryDispositionDoesNotInventStorageHold(t *testing.T) {
	for _, active := range []bool{false, true} {
		t.Run(fmt.Sprintf("active_%t", active), func(t *testing.T) {
			var calls atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(200) }))
			t.Cleanup(server.Close)
			hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
			n := deferredContinuityManager(t, t.TempDir(), hook)
			q := n.GetQueue()
			a := &alerts.Alert{ID: "legacy-malformed-resolution", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelWarning, Metadata: map[string]interface{}{metadataResolvedAt: 42}}
			key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
			require.NoError(t, recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now(), 2))
			n.SetOccurrenceLifecycleProvider(func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
				if active {
					return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceActive}
				}
				return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceUnavailable}
			})
			config, err := json.Marshal(hook)
			require.NoError(t, err)
			require.NoError(t, q.Enqueue(&QueuedNotification{ID: "malformed-recovery", Type: "webhook_resolved", Status: QueueStatusPending, Alerts: []*alerts.Alert{a}, Config: config}))
			require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
			drainContinuity(t, n)
			var status, lastError string
			require.NoError(t, q.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM notification_queue WHERE id='malformed-recovery'`).Scan(&status, &lastError))
			if active {
				require.Equal(t, "cancelled", status)
				require.Zero(t, calls.Load())
			} else {
				require.Equal(t, "sent", status)
				require.Equal(t, int32(1), calls.Load())
			}
			require.NotContains(t, lastError, notificationSettlementPendingMessage)
		})
	}
}

func TestNotificationWebhookRestoreRetirementIsAtomic(t *testing.T) {
	first := WebhookConfig{ID: "a", URL: "http://127.0.0.1:9/a", Enabled: true}
	second := WebhookConfig{ID: "b", URL: "http://127.0.0.1:9/b", Enabled: true}
	n := continuityManager(t, t.TempDir(), first, second)
	q := n.GetQueue()
	previous := n.GetWebhooks()
	a := &alerts.Alert{ID: "restore-atomic", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	keys := []string{}
	for _, hook := range []WebhookConfig{first, second} {
		key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
		keys = append(keys, key)
		require.NoError(t, recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, key, time.Now(), 3))
	}
	_, err := q.db.Exec(`CREATE TRIGGER reject_second_retirement BEFORE UPDATE OF retired_at ON notification_delivery_receipts WHEN OLD.destination_key='` + keys[1] + `' BEGIN SELECT RAISE(ABORT,'synthetic second retirement failure'); END`)
	require.NoError(t, err)
	changedFirst, changedSecond := first, second
	changedFirst.URL += "-new"
	changedSecond.URL += "-new"
	require.Error(t, n.ReplaceWebhooks([]WebhookConfig{changedFirst, changedSecond}))
	require.Equal(t, previous, n.GetWebhooks())
	for _, key := range keys {
		found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
		require.NoError(t, err)
		require.True(t, found, "partial retirement escaped failed restore")
	}
}

func TestNotificationAcknowledgementStorageFailureKeepsProviderBudgetBounded(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	q := n.GetQueue()
	_, err := q.db.Exec(`CREATE TRIGGER reject_success_audit BEFORE INSERT ON notification_audit WHEN NEW.success=1 BEGIN SELECT RAISE(ABORT,'synthetic acknowledgement persistence failure'); END`)
	require.NoError(t, err)
	n.SendAlert(&alerts.Alert{ID: "ack-unpersisted", StartTime: time.Now(), Level: alerts.AlertLevelCritical})
	for want := 1; want <= 3; want++ {
		require.Eventually(t, func() bool {
			mu.Lock()
			observed := count
			mu.Unlock()
			stats, err := q.GetQueueStats()
			if want == 3 {
				return err == nil && observed == want && stats["dlq"] == 1 && stats["sending"] == 0
			}
			return err == nil && observed == want && stats["pending"] == 1 && stats["sending"] == 0
		}, 5*time.Second, time.Millisecond)
		if want < 3 {
			_, err := q.db.Exec(`UPDATE notification_queue SET next_retry_at=? WHERE status='pending'`, time.Now().Add(-time.Second).Unix())
			require.NoError(t, err)
			q.processBatch()
		}
	}
	q.processBatch()
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 3, count, "unpersisted acknowledgement escaped ordinary provider budget")
	var audits int
	require.NoError(t, q.db.QueryRow(`SELECT COUNT(*) FROM notification_audit`).Scan(&audits))
	require.Zero(t, audits, "failed acknowledgement storage fabricated a provider rejection")
}

func TestNotificationOlderRecoveryCannotConsumeNewerFiringInSameGeneration(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(n.Stop)
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true}
	n.AddWebhook(hook)
	q := n.GetQueue()
	a := &alerts.Alert{ID: "same-generation", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	epoch, err := fixtureDeliveryEpoch(q, key)
	require.NoError(t, err)
	require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, key, time.Now(), 3, epoch))
	config, err := json.Marshal(hook)
	require.NoError(t, err)
	oldRecovery := &QueuedNotification{ID: "old-recovery-ack", Type: "webhook_resolved", Status: QueueStatusSent, Alerts: []*alerts.Alert{a}, Attempts: 1, Config: config, ReceiptEpoch: &epoch}
	require.NoError(t, q.Enqueue(oldRecovery))
	require.NoError(t, q.RecordAuditError(oldRecovery, true, nil))
	require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, key, time.Now(), 3, epoch))
	job := notificationDeliveryJob{QueueID: oldRecovery.ID, Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{a}, WebhookConfig: &hook, ReceiptEpoch: epoch, ReceiptEpochBound: true}
	require.NoError(t, n.recordSuccessfulDelivery(job, time.Now()))
	found, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
	require.NoError(t, err)
	require.True(t, found, "old recovery settlement deleted newer firing authority")
}

func TestNotificationOlderFiringSettlementCannotRestoreEndedCooldown(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(n.Stop)
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true}
	n.AddWebhook(hook)
	q := n.GetQueue()
	a := &alerts.Alert{ID: "settlement-order", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	epoch, err := fixtureDeliveryEpoch(q, key)
	require.NoError(t, err)
	config, err := json.Marshal(hook)
	require.NoError(t, err)
	old := a.Clone()
	annotateResolvedMetadata(old, time.Now())
	for _, row := range []*QueuedNotification{
		{ID: "old-ack", Type: "webhook", Alerts: []*alerts.Alert{old}, Config: config, ReceiptEpoch: &epoch},
		{ID: "fresh-ack", Type: "webhook", Alerts: []*alerts.Alert{a}, Config: config, ReceiptEpoch: &epoch},
	} {
		require.NoError(t, q.Enqueue(row))
		require.NoError(t, q.RecordAuditError(row, true, nil))
		if row.ID == "old-ack" {
			require.NoError(t, q.markReceiptOccurrenceResolved(a.ID, a.StartTime, time.Now()))
		}
		require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, key, time.Now(), 3, epoch, row.ID))
	}
	require.NoError(t, q.recordDeliveryReceipt(a.ID, a.StartTime, key, time.Now(), 3, epoch, "old-ack"))
	record, found, err := q.deliveryReceipt(a, key)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, record.resolved, "late old settlement restored prior resolution over fresh firing")
	require.True(t, receiptInCooldown(record, a, 30*time.Minute))
}
