package notifications

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

// admissionBarrier holds the first serialisation of the alert that carries it.
// Enqueue serialises an alert's payload after the admission path has read the
// queue and before it writes the row, so the barrier pauses admitted work at
// the point where Stop could otherwise close storage underneath it.
type admissionBarrier struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func newAdmissionBarrier() *admissionBarrier {
	return &admissionBarrier{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *admissionBarrier) MarshalJSON() ([]byte, error) {
	b.once.Do(func() {
		close(b.entered)
		<-b.release
	})
	return []byte(`"released"`), nil
}

// shutdownAdmissionFixture is a deferred manager on persistent storage whose
// queue is never activated, so only the work under test takes the queue lock
// while Stop runs. Direct delivery is captured, never run.
type shutdownAdmissionFixture struct {
	t       *testing.T
	dir     string
	hook    WebhookConfig
	spawned chan func()
}

func newShutdownAdmissionFixture(t *testing.T) *shutdownAdmissionFixture {
	t.Helper()
	f := &shutdownAdmissionFixture{
		t:       t,
		dir:     t.TempDir(),
		hook:    WebhookConfig{ID: "ops", URL: "http://127.0.0.1:9/never-contacted", Enabled: true, Service: "generic"},
		spawned: make(chan func(), 8),
	}
	origSpawn := spawnAsync
	spawnAsync = func(deliver func()) { f.spawned <- deliver }
	t.Cleanup(func() { spawnAsync = origSpawn })
	return f
}

func (f *shutdownAdmissionFixture) open(configure func(*NotificationManager)) *NotificationManager {
	f.t.Helper()
	m := NewNotificationManagerWithDeferredQueue("", f.dir)
	f.t.Cleanup(m.Stop)
	// No wall-clock batch may contend for the queue lock in this window.
	m.GetQueue().processorTicker.Stop()
	m.SetGroupingConfig(false, 0, false, false)
	m.AddWebhook(f.hook)
	if configure != nil {
		configure(m)
	}
	return m
}

// stopPassedAdmission reports that Stop has gone as far as it can while
// admitted work is held: it closed storage without waiting for that work, or
// it stopped the queue workers and is waiting with delivery still permitted.
func stopPassedAdmission(m *NotificationManager, q *NotificationQueue) bool {
	if q.db.Ping() != nil {
		return true
	}
	select {
	case <-q.stopChan:
		return m.IsEnabled()
	default:
		return false
	}
}

func awaitShutdownCondition(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func awaitShutdownSignal(t *testing.T, what string, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// An alert, recovery or timer group admitted before Stop must be written
// before Stop closes storage, and an attempt made once Stop has begun must be
// refused rather than wait on the queue or fall back to direct delivery. Two
// concurrent Stop calls share one shutdown.
func TestStopWaitsForAdmittedNotificationWrites(t *testing.T) {
	for _, path := range []string{"firing", "recovery", "group-timer"} {
		t.Run(path, func(t *testing.T) {
			f := newShutdownAdmissionFixture(t)
			m := f.open(func(m *NotificationManager) {
				if path == "group-timer" {
					// sendGroupedAlerts is the timer callback; a long window keeps
					// the real timer from also running it.
					m.SetGroupingConfig(true, 3600, false, false)
				}
			})
			q := m.GetQueue()
			barrier := newAdmissionBarrier()
			alert := quietReplayAlert("admitted-"+path, alerts.AlertLevelWarning, time.Now().UTC().Add(-time.Minute))
			alert.Metadata = map[string]interface{}{"barrier": barrier}

			admitted := make(chan struct{})
			switch path {
			case "firing":
				go func() {
					defer close(admitted)
					m.SendAlert(alert)
				}()
			case "recovery":
				m.recordSuccessfulDelivery(notificationDeliveryJob{Type: "webhook", Event: eventAlert,
					Alerts: []*alerts.Alert{alert}, WebhookConfig: &f.hook}, time.Now())
				go func() {
					defer close(admitted)
					m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: time.Now()})
				}()
			case "group-timer":
				m.SendAlert(alert)
				go func() {
					defer close(admitted)
					m.sendGroupedAlerts()
				}()
			}
			awaitShutdownSignal(t, "admitted work to reach queue serialisation", barrier.entered)

			stopped := make(chan struct{}, 2)
			for range 2 {
				go func() {
					m.Stop()
					stopped <- struct{}{}
				}()
			}
			awaitShutdownCondition(t, "Stop to pass the admitted work", func() bool { return stopPassedAdmission(m, q) })

			late := quietReplayAlert("late-"+path, alerts.AlertLevelWarning, time.Now().UTC())
			refused := make(chan struct{})
			go func() {
				defer close(refused)
				m.SendAlert(late)
				m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: late, ResolvedTime: time.Now()})
			}()
			awaitShutdownSignal(t, "an attempt made during Stop to be refused", refused)

			close(barrier.release)
			awaitShutdownSignal(t, "admitted work to finish", admitted)
			for range 2 {
				select {
				case <-stopped:
				case <-time.After(10 * time.Second):
					t.Fatal("Stop did not finish after admitted work completed")
				}
			}
			select {
			case <-f.spawned:
				t.Errorf("SHUTDOWN_ADMISSION_DIRECT_SEND: %s admitted before Stop could not be written because Stop closed the queue under it, and went to direct delivery after shutdown", path)
			default:
			}

			rows := quietReplayRows(t, f.open(nil).GetQueue())
			if len(rows) != 1 || len(rows[0].Alerts) != 1 || rows[0].Alerts[0].ID != alert.ID ||
				rows[0].Status != QueueStatusPending || rows[0].Attempts != 0 {
				t.Fatalf("SHUTDOWN_ADMISSION_LOST: %s admitted before Stop is not pending after restart: %+v", path, rows)
			}
		})
	}
}

// Operator pause and channel disable keep their meaning while Stop drains:
// they cancel queued work, including an alert admitted just before Stop.
func TestStopDrainKeepsPauseAndChannelDisable(t *testing.T) {
	email := EmailConfig{Enabled: true, SMTPHost: "127.0.0.1", SMTPPort: 9,
		From: "pulse@example.test", To: []string{"ops@example.test"}}
	for _, tc := range []struct {
		name      string
		configure func(*NotificationManager)
		act       func(*NotificationManager)
		visible   func(*NotificationManager) bool
		reason    string
	}{
		{
			name:    "pause",
			act:     func(m *NotificationManager) { m.SetEnabled(false) },
			visible: func(m *NotificationManager) bool { return !m.IsEnabled() },
			reason:  "Notifications disabled",
		},
		{
			name: "email-disabled",
			configure: func(m *NotificationManager) {
				m.SetEmailConfig(email)
				_ = m.DeleteWebhook("ops")
			},
			act: func(m *NotificationManager) {
				disabled := email
				disabled.Enabled = false
				m.SetEmailConfig(disabled)
			},
			visible: func(m *NotificationManager) bool { return !m.GetEmailConfig().Enabled },
			reason:  "Email notifications disabled",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newShutdownAdmissionFixture(t)
			m := f.open(tc.configure)
			q := m.GetQueue()
			barrier := newAdmissionBarrier()
			alert := quietReplayAlert("paused-"+tc.name, alerts.AlertLevelWarning, time.Now().UTC().Add(-time.Minute))
			alert.Metadata = map[string]interface{}{"barrier": barrier}
			admitted := make(chan struct{})
			go func() {
				defer close(admitted)
				m.SendAlert(alert)
			}()
			awaitShutdownSignal(t, "admitted work to reach queue serialisation", barrier.entered)

			stopped := make(chan struct{})
			go func() {
				m.Stop()
				close(stopped)
			}()
			awaitShutdownCondition(t, "Stop to pass the admitted work", func() bool { return stopPassedAdmission(m, q) })

			// The change takes its lease in the critical section that publishes
			// it, so once it is visible its cancellation is owed to the queue.
			acted := make(chan struct{})
			go func() {
				defer close(acted)
				tc.act(m)
			}()
			awaitShutdownCondition(t, tc.name+" to take effect", func() bool { return tc.visible(m) })
			close(barrier.release)
			awaitShutdownSignal(t, "admitted work to finish", admitted)
			awaitShutdownSignal(t, tc.name+" to finish", acted)
			awaitShutdownSignal(t, "Stop to finish", stopped)

			rows := quietReplayRows(t, f.open(nil).GetQueue())
			if len(rows) != 1 || rows[0].Status != QueueStatusCancelled ||
				rows[0].LastError == nil || *rows[0].LastError != tc.reason {
				t.Fatalf("SHUTDOWN_POLICY_LOST: %s during the drain did not cancel the alert admitted before Stop: %+v", tc.name, rows)
			}
		})
	}
}

// A deferred queue that never started keeps admitted rows for the next start,
// and Stop still discards a firing group that was only buffered in memory.
func TestStopNeverStartedQueueAndBufferedGroup(t *testing.T) {
	f := newShutdownAdmissionFixture(t)
	m := f.open(nil)
	start := time.Now().UTC().Add(-time.Minute)
	m.SendAlert(quietReplayAlert("queued-1", alerts.AlertLevelWarning, start))
	m.SendAlert(quietReplayAlert("queued-2", alerts.AlertLevelWarning, start))
	m.SetGroupingConfig(true, 3600, false, false)
	m.SendAlert(quietReplayAlert("buffered", alerts.AlertLevelWarning, start))

	stopped := make(chan struct{})
	go func() {
		m.Stop()
		close(stopped)
	}()
	awaitShutdownSignal(t, "Stop of a never-started queue", stopped)
	// A timer that runs after Stop finds nothing to admit.
	m.sendGroupedAlerts()

	select {
	case <-f.spawned:
		t.Fatal("stopped manager handed work to direct delivery")
	default:
	}
	statuses := map[string]NotificationQueueStatus{}
	rows := quietReplayRows(t, f.open(nil).GetQueue())
	for _, row := range rows {
		if row.Attempts != 0 {
			t.Errorf("never-started queue consumed an attempt: %+v", row)
		}
		for _, alert := range row.Alerts {
			statuses[alert.ID] = row.Status
		}
	}
	if len(rows) != 2 || statuses["queued-1"] != QueueStatusPending || statuses["queued-2"] != QueueStatusPending {
		t.Fatalf("never-started queue lost admitted rows across Stop: %+v", rows)
	}
	if _, persisted := statuses["buffered"]; persisted {
		t.Fatal("Stop persisted a buffered firing group; buffered groups are discarded at shutdown")
	}
}

// Direct delivery started before Stop is owned: Stop waits for it, so its
// firing receipt is durable and the recovery still reaches the destination
// after restart. Direct delivery cannot start once Stop has finished.
func TestStopOwnsDirectFallbackDelivery(t *testing.T) {
	received := make(chan struct{}, 4)
	release := make(chan struct{})
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	var releaseOnce sync.Once
	releaseDelivery := func() { releaseOnce.Do(func() { close(release) }) }

	dir := t.TempDir()
	hook := WebhookConfig{ID: "ops", URL: server.URL + "/direct", Enabled: true, Service: "generic"}
	open := func() *NotificationManager {
		m := NewNotificationManagerWithDeferredQueue("", dir)
		t.Cleanup(m.Stop)
		m.webhookClient = server.Client()
		require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
		m.SetGroupingConfig(false, 0, false, false)
		m.AddWebhook(hook)
		return m
	}
	m := open()
	// Registered after the manager, so a held delivery is released before Stop.
	t.Cleanup(releaseDelivery)
	q := m.GetQueue()
	// A storage failure, not shutdown, sends this alert down the direct path.
	_, err := q.db.Exec(`CREATE TRIGGER reject_admission BEFORE INSERT ON notification_queue
  BEGIN SELECT RAISE(ABORT, 'fixture queue write failed'); END`)
	require.NoError(t, err)

	alert := quietReplayAlert("direct", alerts.AlertLevelWarning, time.Now().UTC().Add(-time.Minute))
	m.SendAlert(alert)
	select {
	case <-received:
	case <-time.After(10 * time.Second):
		t.Fatal("enqueue failure did not start direct delivery")
	}

	stopped := make(chan struct{})
	go func() {
		m.Stop()
		close(stopped)
	}()
	awaitShutdownCondition(t, "Stop to pass the direct delivery", func() bool { return stopPassedAdmission(m, q) })
	releaseDelivery()
	awaitShutdownSignal(t, "Stop to finish", stopped)

	spawned := 0
	origSpawn := spawnAsync
	spawnAsync = func(func()) { spawned++ }
	m.dispatchNotificationJobAsync(notificationDeliveryJob{Type: "webhook", Event: eventAlert,
		Alerts: []*alerts.Alert{alert}, WebhookConfig: &hook}, "direct delivery after Stop")
	spawnAsync = origSpawn
	if spawned != 0 {
		t.Error("FALLBACK_AFTER_SHUTDOWN: direct delivery started after Stop returned")
	}

	jobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{hook}, AppriseConfig{}, []*alerts.Alert{alert}, eventResolved, time.Now())
	if len(open().filterResolvedJobsByDeliveryReceipt(jobs)) != 1 {
		t.Fatal("FALLBACK_RECEIPT_LOST: direct delivery accepted during shutdown left no durable firing receipt, so its recovery is suppressed after restart")
	}
}
