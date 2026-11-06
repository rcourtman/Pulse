package notifications

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func continuityManager(t *testing.T, dir string, hooks ...WebhookConfig) *NotificationManager {
	n := deferredContinuityManager(t, dir, hooks...)
	require.NoError(t, n.StartQueueProcessingWithConfiguration(true))
	return n
}

func deferredContinuityManager(t *testing.T, dir string, hooks ...WebhookConfig) *NotificationManager {
	t.Helper()
	n := NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(n.Stop)
	require.NoError(t, n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	n.SetCooldown(30)
	n.SetGroupingConfig(false, 0, false, false)
	for _, hook := range hooks {
		n.AddWebhook(hook)
	}
	return n
}

func drainContinuity(t *testing.T, n *NotificationManager) {
	t.Helper()
	require.Eventually(t, func() bool {
		stats, err := n.GetQueueStats()
		require.NoError(t, err)
		return stats["pending"]+stats["sending"] == 0
	}, 10*time.Second, 5*time.Millisecond)
}

func TestNotificationContinuityQueueMetadataMigrationPreservesProviderBytes(t *testing.T) {
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	require.NoError(t, q.Stop())
	db, err := sql.Open("sqlite", filepath.Join(dir, notificationQueueFileName))
	require.NoError(t, err)
	_, err = db.Exec(`ALTER TABLE notification_queue DROP COLUMN receipt_epoch`)
	require.NoError(t, err)
	config := `{ "id": "legacy", "url": "https://example.test", "headers": {"X-Recipient":"original"} }`
	for _, kind := range []string{"webhook", "webhook_resolved"} {
		_, err = db.Exec(`INSERT INTO notification_queue(id,type,method,status,alerts,config,created_at) VALUES(?,?,'','pending','[]',?,?)`, kind, kind, config, time.Now().Unix())
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())
	q, err = NewNotificationQueue(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Stop() })
	rows, err := q.GetPending(10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Equal(t, config, string(row.Config), "migration rewrote immutable provider snapshot")
		if row.Type == "webhook_resolved" {
			require.NotNil(t, row.ReceiptEpoch)
			require.Empty(t, *row.ReceiptEpoch)
		} else {
			require.Nil(t, row.ReceiptEpoch)
		}
	}
}

func TestNotificationContinuityMigratesLandedReceiptSchema(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, notificationQueueFileName))
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE notification_delivery_receipts(alert_id TEXT NOT NULL,alert_start_nanos INTEGER NOT NULL,destination_key TEXT NOT NULL,delivered_at INTEGER NOT NULL,highest_severity INTEGER NOT NULL DEFAULT 0,delivered_nanos INTEGER NOT NULL DEFAULT 0,resolved_at INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(alert_id,alert_start_nanos,destination_key)); CREATE TABLE notification_receipt_retirements(destination_key TEXT PRIMARY KEY,retired_at INTEGER NOT NULL)`)
	require.NoError(t, err)
	start := time.Now().Add(-time.Hour)
	completed := time.Now().Add(-time.Minute).Round(0)
	for _, key := range []string{"webhook:active", "webhook:recent", "webhook:nano", "webhook:retired"} {
		resolved := int64(0)
		if key == "webhook:recent" {
			resolved = completed.Unix()
		}
		if key == "webhook:nano" {
			resolved = completed.UnixNano()
		}
		_, err = db.Exec(`INSERT INTO notification_delivery_receipts VALUES(?,?,?,?,?,?,?)`, "landed", start.UnixNano(), key, completed.Unix(), 3, completed.UnixNano(), resolved)
		require.NoError(t, err)
	}
	_, err = db.Exec(`INSERT INTO notification_receipt_retirements VALUES(?,?)`, "webhook:retired", completed.Unix())
	require.NoError(t, err)
	require.NoError(t, db.Close())
	for range 2 {
		q, err := NewNotificationQueue(dir)
		require.NoError(t, err)
		a := &alerts.Alert{ID: "landed", StartTime: start, Level: alerts.AlertLevelCritical}
		record, found, err := q.deliveryReceipt(a, "webhook:active")
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, alerts.AlertLevelCritical, record.level)
		require.Equal(t, completed, record.lastSent)
		require.NoError(t, q.recordDeliveryReceipt(a.ID, start, "webhook:active", completed.Add(-time.Hour), 2, ""))
		record, _, err = q.deliveryReceipt(a, "webhook:active")
		require.NoError(t, err)
		require.Equal(t, completed, record.lastSent)
		require.Equal(t, alerts.AlertLevelCritical, record.level)
		var resolved int64
		require.NoError(t, q.db.QueryRow(`SELECT resolved_at FROM notification_delivery_receipts WHERE destination_key='webhook:recent'`).Scan(&resolved))
		require.Equal(t, completed.Unix()*int64(time.Second), resolved, "second disposition was lost or converted twice")
		require.NoError(t, q.db.QueryRow(`SELECT resolved_at FROM notification_delivery_receipts WHERE destination_key='webhook:nano'`).Scan(&resolved))
		require.Equal(t, completed.UnixNano(), resolved, "already precise resolution was erased or multiplied")
		retired, err := q.HasDeliveryReceipt(a.ID, start, "webhook:retired")
		require.NoError(t, err)
		require.False(t, retired, "landed retirement became live legacy authority")
		q.occurrenceLifecycle = func(string, time.Time) alerts.NotificationOccurrenceLifecycle {
			return alerts.NotificationOccurrenceLifecycle{State: alerts.NotificationOccurrenceNoRecord}
		}
		q.performCleanup()
		recent, err := q.HasDeliveryReceipt(a.ID, start, "webhook:recent")
		require.NoError(t, err)
		require.True(t, recent, "recent landed resolution was interpreted as a date in 1970")
		require.NoError(t, q.Stop())
	}
}

func TestNotificationContinuityFutureTimestampNormalizesAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	start := time.Now().Add(-time.Hour)
	epoch, err := q.activateDeliveryDestination("webhook:clock", "recipient")
	require.NoError(t, err)
	future := time.Now().Add(365 * 24 * time.Hour)
	require.NoError(t, recordFixtureDeliveryReceipt(q, "clock", start, "webhook:clock", future, 3, epoch))
	var stored int64
	require.NoError(t, q.db.QueryRow(`SELECT delivered_at FROM notification_delivery_receipts`).Scan(&stored))
	require.LessOrEqual(t, stored, time.Now().Unix(), "new future evidence persisted")
	_, err = q.db.Exec(`UPDATE notification_delivery_receipts SET delivered_at=?`, future.Unix())
	require.NoError(t, err)
	require.NoError(t, q.Stop())
	q, err = NewNotificationQueue(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Stop() })
	var rank int
	var gotEpoch string
	require.NoError(t, q.db.QueryRow(`SELECT delivered_at,severity,delivery_epoch FROM notification_delivery_receipts`).Scan(&stored, &rank, &gotEpoch))
	require.LessOrEqual(t, stored, time.Now().Unix(), "reopen re-armed impossible future cooldown")
	require.Equal(t, 3, rank)
	require.Equal(t, epoch, gotEpoch)
}

func TestNotificationContinuityRecoveryGroupingKeepsGenerationsSeparate(t *testing.T) {
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	config := json.RawMessage(`{"id":"ops","url":"http://127.0.0.1:9","enabled":true}`)
	first, second := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	for i, epoch := range []*string{&first, &second, &first} {
		a := &alerts.Alert{ID: fmt.Sprintf("group-%d", i), StartTime: time.Now().Add(time.Duration(i) * time.Nanosecond)}
		require.NoError(t, q.enqueueResolvedGroup(&QueuedNotification{ID: fmt.Sprintf("row-%d", i), Type: "webhook_resolved", Alerts: []*alerts.Alert{a}, Config: config, ReceiptEpoch: epoch}, time.Hour))
	}
	require.NoError(t, q.Stop())
	q, err = NewNotificationQueue(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Stop() })
	rows := quietReplayRows(t, q)
	require.Len(t, rows, 2)
	counts := map[string]int{}
	for _, row := range rows {
		require.Equal(t, string(config), string(row.Config))
		require.NotNil(t, row.ReceiptEpoch)
		counts[*row.ReceiptEpoch] = len(row.Alerts)
	}
	require.Equal(t, map[string]int{first: 2, second: 1}, counts)
}

// Each phase is a separate native process sharing only the actual persistent
// queue and a real HTTP receiver. No successful delivery receipt is fabricated.
func TestNotificationContinuityNativeRestart(t *testing.T) {
	var mu sync.Mutex
	var got []struct {
		Event  string         `json:"event"`
		Alerts []alerts.Alert `json:"alerts"`
	}
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Event  string         `json:"event"`
			Alerts []alerts.Alert `json:"alerts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		got = append(got, payload)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	start := strconv.FormatInt(time.Now().Add(-time.Hour).UnixNano(), 10)
	for phase, want := range []int{1, 1, 2, 2, 3, 4} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestNotificationContinuityChild$", "-test.count=1")
		cmd.Env = append(os.Environ(), "PULSE_CONTINUITY_PHASE="+strconv.Itoa(phase), "PULSE_CONTINUITY_DIR="+dir, "PULSE_CONTINUITY_URL="+server.URL, "PULSE_CONTINUITY_START="+start)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "phase %d: %s", phase, output)
		mu.Lock()
		count := len(got)
		mu.Unlock()
		require.Equal(t, want, count, "phase %d", phase)
	}
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, alerts.AlertLevelWarning, got[0].Alerts[0].Level)
	require.Equal(t, alerts.AlertLevelCritical, got[1].Alerts[0].Level)
	require.NotEqual(t, got[0].Alerts[0].StartTime, got[2].Alerts[0].StartTime)
	require.Equal(t, "resolved", got[3].Event)
}

func TestNotificationContinuityChild(t *testing.T) {
	raw := os.Getenv("PULSE_CONTINUITY_PHASE")
	if raw == "" {
		t.Skip("native restart child only")
	}
	phase, err := strconv.Atoi(raw)
	require.NoError(t, err)
	nanos, err := strconv.ParseInt(os.Getenv("PULSE_CONTINUITY_START"), 10, 64)
	require.NoError(t, err)
	hook := WebhookConfig{ID: "ops", URL: os.Getenv("PULSE_CONTINUITY_URL"), Enabled: true, Service: "generic"}
	n := continuityManager(t, os.Getenv("PULSE_CONTINUITY_DIR"), hook)
	alert := &alerts.Alert{ID: "restart", ResourceName: "node", Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: time.Unix(0, nanos)}
	switch phase {
	case 2:
		alert.Level = alerts.AlertLevelCritical
	case 4, 5:
		alert.StartTime = alert.StartTime.Add(time.Minute)
	}
	if phase == 5 {
		n.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: time.Now()})
	} else {
		n.SendAlert(alert)
	}
	drainContinuity(t, n)
}

func TestNotificationContinuityDestinationScope(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(server.Close)
	first := WebhookConfig{ID: "first", URL: server.URL + "/first", Enabled: true, Service: "generic"}
	second := WebhookConfig{ID: "second", URL: server.URL + "/second", Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), first)
	a := &alerts.Alert{ID: "scope", StartTime: time.Now(), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	n.AddWebhook(second)
	n.SendAlert(a)
	drainContinuity(t, n)
	mu.Lock()
	require.Equal(t, map[string]int{"/first": 1, "/second": 1}, counts)
	mu.Unlock()
	// Explicitly scheduled escalation retains its independently owned cadence.
	n.SendEscalatedAlert(a, "all")
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, map[string]int{"/first": 2, "/second": 2}, counts)
}

func TestNotificationContinuityLegacyMigration(t *testing.T) {
	dir := t.TempDir()
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	alert := &alerts.Alert{ID: "legacy", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelCritical}
	destination := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	db, err := sql.Open("sqlite", filepath.Join(dir, notificationQueueFileName))
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE notification_delivery_receipts(alert_id TEXT NOT NULL,alert_start_nanos INTEGER NOT NULL,destination_key TEXT NOT NULL,delivered_at INTEGER NOT NULL,PRIMARY KEY(alert_id,alert_start_nanos,destination_key))`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO notification_delivery_receipts VALUES(?,?,?,?)`, alert.ID, alert.StartTime.UnixNano(), destination, time.Now().Unix())
	require.NoError(t, err)
	require.NoError(t, db.Close())
	n := continuityManager(t, dir, hook)
	n.SendAlert(alert)
	drainContinuity(t, n)
	n.Stop()
	n = continuityManager(t, dir, hook)
	n.SendAlert(alert)
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count, "unknown legacy severity permits new information once")
}

func TestNotificationContinuityMonotonicEvidence(t *testing.T) {
	dir := t.TempDir()
	n := NewNotificationManagerWithDeferredQueue("", dir)
	hook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true}
	destination := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	a := &alerts.Alert{ID: "order", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelCritical}
	latest := time.Now().Truncate(time.Second)
	var wg sync.WaitGroup
	errors := make(chan error, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			copy := a.Clone()
			copy.Level = alerts.AlertLevelWarning
			if i == 0 {
				copy.Level = alerts.AlertLevelCritical
			}
			errors <- recordFixtureDeliveryReceipt(n.GetQueue(), copy.ID, copy.StartTime, destination, latest.Add(-time.Duration(i)*time.Second), notificationSeverityRank(copy.Level))
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	n.Stop()
	n = NewNotificationManagerWithDeferredQueue("", dir)
	defer n.Stop()
	record, found, err := n.GetQueue().deliveryReceipt(a, destination)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, latest, record.lastSent)
	require.Equal(t, alerts.AlertLevelCritical, record.level)
}

func TestNotificationContinuityCacheMissShutdown(t *testing.T) {
	f := newShutdownAdmissionFixture(t)
	n := f.open(nil)
	a := &alerts.Alert{ID: "cache-miss", Level: alerts.AlertLevelCritical, StartTime: time.Now()}
	q := n.GetQueue()
	destination := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &f.hook})
	require.NoError(t, recordFixtureDeliveryReceipt(q, a.ID, a.StartTime, destination, time.Now(), 3))
	// Observe the existing admission lease while the authoritative read is held,
	// then close admission. Checkpoint serialization also waits for this read;
	// the explicit lease-count assertion verifies its independent queue ownership.
	q.mu.Lock()
	sent := make(chan struct{})
	go func() { n.SendAlert(a); close(sent) }()
	require.Eventually(t, func() bool { n.lifecycle.mu.Lock(); defer n.lifecycle.mu.Unlock(); return n.lifecycle.leases > 0 }, time.Second, time.Millisecond)
	stopped := make(chan struct{})
	go func() { n.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("shutdown passed an admitted receipt read")
	case <-time.After(20 * time.Millisecond):
	}
	q.mu.Unlock()
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("receipt read did not finish")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not drain")
	}
	require.Empty(t, f.spawned)
}

func TestNotificationContinuityGroupedAdmissionFinishesBeforeShutdownCheckpoint(t *testing.T) {
	f := newShutdownAdmissionFixture(t)
	n := f.open(func(n *NotificationManager) { n.SetGroupingConfig(true, 3600, false, false) })
	a := &alerts.Alert{ID: "grouped-cooldown-read", Level: alerts.AlertLevelCritical, StartTime: time.Now()}
	q := n.GetQueue()
	q.mu.Lock()
	release := sync.OnceFunc(q.mu.Unlock)
	t.Cleanup(release)
	sent := make(chan struct{})
	go func() { n.SendAlert(a); close(sent) }()
	require.Eventually(t, func() bool { n.lifecycle.mu.Lock(); defer n.lifecycle.mu.Unlock(); return n.lifecycle.leases > 0 }, time.Second, time.Millisecond)
	stopped := make(chan struct{})
	go func() { n.Stop(); close(stopped) }()
	require.Eventually(t, func() bool { return !n.lifecycle.accepting() }, time.Second, time.Millisecond)
	release()
	awaitShutdownSignal(t, "grouped admission", sent)
	awaitShutdownSignal(t, "grouped checkpoint", stopped)
	reopened, err := NewNotificationQueue(f.dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Stop()) })
	rows, err := reopened.GetPending(10)
	require.NoError(t, err)
	// Future-due rows are intentionally absent from the processor's due query.
	require.Empty(t, rows)
	var id string
	var due int64
	require.NoError(t, reopened.db.QueryRow(`SELECT json_extract(alerts,'$[0].id'),next_retry_at FROM notification_queue WHERE status='pending'`).Scan(&id, &due))
	require.Equal(t, a.ID, id)
	require.Greater(t, due, time.Now().Add(59*time.Minute).Unix())
	require.Empty(t, f.spawned)
}

type continuityRoundTripper func(*http.Request) (*http.Response, error)

func (f continuityRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A real process dies after a real HTTP acknowledgement and before receipt
// commit. The pending row must replay, honestly allowing a second provider send.
func TestNotificationContinuityAcknowledgementCrashBoundary(t *testing.T) {
	var mu sync.Mutex
	deliveries := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); deliveries++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	start := strconv.FormatInt(time.Now().Add(-time.Hour).UnixNano(), 10)
	cmd := exec.Command(os.Args[0], "-test.run=^TestNotificationContinuityCrashChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), "PULSE_CONTINUITY_CRASH=1", "PULSE_CONTINUITY_DIR="+dir, "PULSE_CONTINUITY_URL="+server.URL, "PULSE_CONTINUITY_START="+start)
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	require.NoError(t, cmd.Start())
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	acknowledged := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			if scanner.Text() == "notification-ack-before-receipt" {
				close(acknowledged)
				return
			}
		}
	}()
	select {
	case <-acknowledged:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not reach provider acknowledgement boundary")
	}
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, cmd.Wait(), "child must have died before graceful Stop")
	reaped = true
	mu.Lock()
	count := deliveries
	mu.Unlock()
	require.Equal(t, 1, count)
	// Observe the pre-replay persisted boundary before installing an autonomous
	// processor: sending survives, but no successful firing receipt exists.
	db, err := sql.Open("sqlite", filepath.Join(dir, notificationQueueFileName))
	require.NoError(t, err)
	var status string
	var receipts int
	require.NoError(t, db.QueryRow(`SELECT status FROM notification_queue`).Scan(&status))
	require.Equal(t, string(QueueStatusSending), status)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts`).Scan(&receipts))
	require.Zero(t, receipts)
	require.NoError(t, db.Close())
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, dir, hook)
	drainContinuity(t, n)
	nanos, err := strconv.ParseInt(start, 10, 64)
	require.NoError(t, err)
	a := &alerts.Alert{ID: "crash-boundary", StartTime: time.Unix(0, nanos), Level: alerts.AlertLevelCritical}
	n.SendAlert(a)
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 2, deliveries, "crash replay is at least once; later cooldown uses committed evidence")
}

// Once the actual provider acceptance and scalar occurrence facts commit,
// abrupt process death must leave only local settlement work after reopen.
func TestNotificationContinuityAcceptedFactsSurviveProcessDeath(t *testing.T) {
	var mu sync.Mutex
	deliveries := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); deliveries++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	start := time.Now().Add(-time.Hour)
	cmd := exec.Command(os.Args[0], "-test.run=^TestNotificationContinuityCrashChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), "PULSE_CONTINUITY_CRASH=accepted", "PULSE_CONTINUITY_DIR="+dir, "PULSE_CONTINUITY_URL="+server.URL, "PULSE_CONTINUITY_START="+strconv.FormatInt(start.UnixNano(), 10))
	output, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	require.NoError(t, cmd.Start())
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	accepted := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			if scanner.Text() == "notification-accepted-before-projection" {
				close(accepted)
				return
			}
		}
	}()
	select {
	case <-accepted:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not reach durable accepted-facts boundary")
	}
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, cmd.Wait())
	reaped = true
	db, err := sql.Open("sqlite", filepath.Join(dir, notificationQueueFileName))
	require.NoError(t, err)
	var facts, receipts, audits int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_facts`).Scan(&facts))
	require.Equal(t, 1, facts)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE success=1 AND receipt_facts_complete=1`).Scan(&audits))
	require.Equal(t, 1, audits)
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM notification_delivery_receipts`).Scan(&receipts))
	require.Zero(t, receipts)
	_, err = db.Exec(`DROP TRIGGER reject_crash_projection; UPDATE notification_queue SET next_retry_at=0`)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, dir, hook)
	drainContinuity(t, n)
	a := &alerts.Alert{ID: "crash-boundary", StartTime: start, Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	_, found, err := n.GetQueue().deliveryReceipt(a, key)
	require.NoError(t, err)
	require.True(t, found)
	n.SendAlert(a)
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, deliveries, "durably accepted work was sent again after process death")
}

func TestNotificationContinuityCrashChild(t *testing.T) {
	if os.Getenv("PULSE_CONTINUITY_CRASH") == "" {
		t.Skip("crash boundary child only")
	}
	nanos, err := strconv.ParseInt(os.Getenv("PULSE_CONTINUITY_START"), 10, 64)
	require.NoError(t, err)
	hook := WebhookConfig{ID: "ops", URL: os.Getenv("PULSE_CONTINUITY_URL"), Enabled: true, Service: "generic"}
	n := continuityManager(t, os.Getenv("PULSE_CONTINUITY_DIR"), hook)
	q := n.GetQueue()
	if os.Getenv("PULSE_CONTINUITY_CRASH") == "accepted" {
		_, err = q.db.Exec(`CREATE TRIGGER reject_crash_projection BEFORE INSERT ON notification_delivery_receipts BEGIN SELECT RAISE(ABORT,'synthetic crash projection failure'); END`)
		require.NoError(t, err)
		n.SendAlert(&alerts.Alert{ID: "crash-boundary", StartTime: time.Unix(0, nanos), Level: alerts.AlertLevelCritical})
		require.Eventually(t, func() bool {
			stats, err := q.GetQueueStats()
			return err == nil && stats["settlement_pending"] == 1 && stats["pending"] == 1 && stats["sending"] == 0
		}, 5*time.Second, time.Millisecond)
		fmt.Println("notification-accepted-before-projection")
		select {}
	}
	n.webhookClient = &http.Client{Transport: continuityRoundTripper(func(r *http.Request) (*http.Response, error) {
		response, err := http.DefaultTransport.RoundTrip(r)
		if err == nil && response.StatusCode == 200 {
			q.mu.Lock() // The parent kills this owned process before any receipt write.
			fmt.Println("notification-ack-before-receipt")
		}
		return response, err
	})}
	n.SendAlert(&alerts.Alert{ID: "crash-boundary", StartTime: time.Unix(0, nanos), Level: alerts.AlertLevelCritical})
	select {} // Parent must kill and reap this process, never run deferred Stop.
}

func TestNotificationContinuityFutureReceiptHasBoundedCooldown(t *testing.T) {
	var mu sync.Mutex
	count := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); count++; mu.Unlock(); w.WriteHeader(200) }))
	t.Cleanup(server.Close)
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
	n := continuityManager(t, t.TempDir(), hook)
	a := &alerts.Alert{ID: "clock", StartTime: time.Now().Add(-time.Hour), Level: alerts.AlertLevelCritical}
	key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
	epoch, err := n.GetQueue().activateDeliveryDestination(key, notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook}))
	require.NoError(t, err)
	require.NoError(t, recordFixtureDeliveryReceipt(n.GetQueue(), a.ID, a.StartTime, key, time.Now().Add(365*24*time.Hour), 3, epoch))
	// Simulate an already-corrupt clock value arriving after startup to exercise
	// the read guard independently of write/startup normalization.
	_, err = n.GetQueue().db.Exec(`UPDATE notification_delivery_receipts SET delivered_at=?`, time.Now().Add(365*24*time.Hour).Unix())
	require.NoError(t, err)
	n.SendAlert(a)
	drainContinuity(t, n)
	mu.Lock()
	before := count
	mu.Unlock()
	require.Zero(t, before)
	var repaired int64
	require.NoError(t, n.GetQueue().db.QueryRow(`SELECT delivered_at FROM notification_delivery_receipts`).Scan(&repaired))
	require.LessOrEqual(t, repaired, time.Now().Unix())
	aged := time.Now().Add(-31 * time.Minute)
	_, err = n.GetQueue().db.Exec(`UPDATE notification_delivery_receipts SET delivered_at=?,delivered_nanos=?`, aged.Unix(), aged.UnixNano())
	require.NoError(t, err)
	n.SendAlert(a)
	drainContinuity(t, n)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, 1, count, "a corrected wall clock must not pin cooldown to the bad future timestamp")
}

func TestNotificationConfigurationFingerprintsScopeRecipients(t *testing.T) {
	webhook := WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9", Enabled: true, Service: "pagerduty", Headers: map[string]string{"routing_key": "A"}}
	changed := copyWebhookConfig(webhook)
	changed.Headers["routing_key"] = "B"
	require.NotEqual(t, notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "webhook", WebhookConfig: &webhook}), notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "webhook", WebhookConfig: &changed}))
	email := EmailConfig{Enabled: true, SMTPHost: "relay-a", From: "pulse@example.test", To: []string{"ops@example.test"}}
	other := email
	other.SMTPHost = "relay-b"
	require.NotEqual(t, notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "email", EmailConfig: &email}), notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "email", EmailConfig: &other}))
	apprise := AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: "http://127.0.0.1:9", Targets: []string{"a", "b"}, APIKey: "account-a"}
	next := copyAppriseConfig(apprise)
	next.APIKey = "account-b"
	require.NotEqual(t, notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "apprise", AppriseConfig: &apprise}), notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "apprise", AppriseConfig: &next}))
	next = copyAppriseConfig(apprise)
	next.TimeoutSeconds = 90
	next.Targets = []string{"b", "a"}
	require.Equal(t, notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "apprise", AppriseConfig: &apprise}), notificationDeliveryConfigurationFingerprint(notificationDeliveryJob{Type: "apprise", AppriseConfig: &next}))
}

func TestNotificationNoDestinationDoesNotInventDelivery(t *testing.T) {
	n := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	defer n.Stop()
	n.SetGroupingConfig(false, 0, false, false)
	n.SendAlert(&alerts.Alert{ID: "no-destination", StartTime: time.Now(), Level: alerts.AlertLevelCritical})
	n.mu.RLock()
	defer n.mu.RUnlock()
	require.Empty(t, n.lastNotified)
	require.Empty(t, n.deliveryCooldown)
}
