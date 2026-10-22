package notifications

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func waitResolvedContract(t *testing.T, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for !predicate() {
		if time.Now().After(deadline) {
			t.Fatal("notification condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Ordinary firing establishes real receipts. Restart reloads those receipts;
// no receipt or delivery job is manufactured by this test.
func TestResolvedGroupingOrdinaryRestart(t *testing.T) {
	for _, stage := range []string{"same_process", "after_firing", "pending_recovery", "late_arrival"} {
		t.Run(stage, func(t *testing.T) {
			type payload struct {
				Event      string          `json:"event"`
				Alerts     []*alerts.Alert `json:"alerts"`
				receivedAt time.Time
			}
			received := make(chan payload, 64)
			server := newResolvedContractServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var p payload
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Error(err)
				}
				p.receivedAt = time.Now()
				received <- p
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			dir := t.TempDir()
			hook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL, Enabled: true}
			open := func() *NotificationManager {
				m := NewNotificationManagerWithDataDir("", dir)
				m.webhookClient = server.Client()
				if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				m.AddWebhook(hook)
				m.SetNotifyOnResolve(true)
				m.SetGroupingConfig(true, 1, false, false)
				t.Cleanup(m.Stop)
				return m
			}
			m := open()
			batch := make([]*alerts.Alert, 15)
			for i := range batch {
				batch[i] = &alerts.Alert{ID: fmt.Sprintf("burst-%02d", i), ResourceName: fmt.Sprintf("guest-%02d", i), Type: "cpu", Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute)}
				m.SendAlert(batch[i])
			}
			job := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: batch, WebhookConfig: &hook}
			waitResolvedContract(t, func() bool {
				jobs := m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{job})
				return len(jobs) == 1 && len(jobs[0].Alerts) == 15
			})
			firing := <-received
			if len(firing.Alerts) != 15 {
				t.Fatalf("firing batch = %d", len(firing.Alerts))
			}
			if stage == "after_firing" {
				m.Stop()
				m = open()
			}
			// A newly configured destination must not receive recoveries for firings
			// delivered only to ops, even when sharing its endpoint.
			unseenHook := hook
			unseenHook.ID = "never-delivered"
			m.AddWebhook(unseenHook)
			resolvedAt := time.Now().Truncate(time.Second)
			submittedAt := make(map[string]time.Time, len(batch))
			for i, a := range batch {
				if m.CancelAlert(a.ID) {
					t.Fatal("delivered occurrence cancelled as unannounced")
				}
				submittedAt[a.ID] = time.Now()
				m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: resolvedAt.Add(time.Duration(i) * time.Second)})
				if stage == "late_arrival" && i == 0 {
					// A completed enqueue has at most one extra second of rounding.
					// Arrivals after its deadline belong to a new fixed window, even
					// when the ordinary dispatcher has not yet claimed the first group.
					time.Sleep(2100 * time.Millisecond)
				}
			}
			burstFitsWindow := time.Since(submittedAt[batch[0].ID]) < time.Second
			m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: &alerts.Alert{ID: "unannounced", ResourceName: "unannounced", StartTime: time.Now()}, ResolvedTime: resolvedAt})
			if stage == "pending_recovery" {
				m.Stop()
				m = open()
				m.AddWebhook(unseenHook)
			}
			// Check receiver time against admission, not against the end of the
			// loop/restart: slow emission can legitimately span fixed windows.
			// A fast burst must still be one group, and all occurrences must be
			// delivered exactly once with their original recovery metadata.
			seen := make(map[string]bool, len(batch))
			groups := 0
			timer := time.NewTimer(12 * time.Second)
			defer timer.Stop()
			for len(seen) < len(batch) {
				var recovery payload
				select {
				case recovery = <-received:
				case <-timer.C:
					t.Fatalf("grouped recovery missing occurrences: delivered=%d expected=%d", len(seen), len(batch))
				}
				if recovery.Event != "resolved" || len(recovery.Alerts) == 0 {
					t.Fatalf("recovery event=%s alerts=%d", recovery.Event, len(recovery.Alerts))
				}
				if burstFitsWindow && len(recovery.Alerts) != len(batch) {
					t.Fatalf("burst admitted within one window was split: alerts=%d", len(recovery.Alerts))
				}
				if stage == "late_arrival" && len(seen) == 0 && len(recovery.Alerts) != 1 {
					t.Fatal("late arrivals extended the expired first group")
				}
				firstAdmission := submittedAt[recovery.Alerts[0].ID]
				if firstAdmission.IsZero() || recovery.receivedAt.Before(firstAdmission.Add(time.Second)) {
					t.Fatal("resolved delivery bypassed the first occurrence's grouping window")
				}
				groups++
				previousIndex := -1
				for _, a := range recovery.Alerts {
					i := -1
					for j, original := range batch {
						if a.ID == original.ID {
							i = j
							break
						}
					}
					if i < 0 || i <= previousIndex || seen[a.ID] || !a.StartTime.Equal(batch[i].StartTime) || a.Metadata[metadataResolvedAt] != resolvedAt.Add(time.Duration(i)*time.Second).Format(time.RFC3339) {
						t.Fatalf("duplicate, unannounced or altered recovery occurrence: id=%s", a.ID)
					}
					seen[a.ID] = true
					previousIndex = i
				}
			}
			t.Logf("ordinary recovery: stage=%s groups=%d occurrences=%d burstFitsWindow=%t", stage, groups, len(seen), burstFitsWindow)
			waitResolvedContract(t, func() bool { return len(m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{job})) == 0 })
			stats, err := m.GetQueue().GetQueueStats()
			if err != nil {
				t.Fatal(err)
			}
			if stats["dlq"] != 0 || stats["failed"] != 0 {
				t.Fatalf("unexpected delivery failures: %v", stats)
			}
			m.Stop()
			select {
			case p := <-received:
				t.Fatalf("extra recovery: %+v", p)
			default:
			}
		})
	}
}

func TestResolvedGroupedWebhookNamesEveryAlert(t *testing.T) {
	for _, service := range []string{"generic", "discord", "slack", "telegram", "teams", "teams-adaptive", "pushover", "gotify", "ntfy", "mattermost", "custom"} {
		t.Run(service, func(t *testing.T) {
			var body string
			server := newResolvedContractServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				body = string(b)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			m := NewNotificationManagerWithDataDir("", t.TempDir())
			defer m.Stop()
			m.webhookClient = server.Client()
			if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			hook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL + "?chat_id=1234", Enabled: true, Service: service}
			if service == "custom" {
				hook.Service = "generic"
				hook.Template = `{"message":"{{.Message}}","count":{{.AlertCount}}}`
			}
			batch := contractGroupedAlerts()
			if err := m.sendResolvedWebhook(hook, batch, time.Now()); err != nil {
				t.Fatal(err)
			}
			for _, a := range batch {
				if !strings.Contains(body, a.ResourceName) {
					t.Errorf("%s missing %s: %s", service, a.ResourceName, body)
				}
			}
		})
	}
}

// Exercise HTTP payloads without a listener or external network. Persistent
// queue operations remain real; this adapter replaces only provider transport.
type resolvedContractTransport struct{ handler http.Handler }

func (tr resolvedContractTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	tr.handler.ServeHTTP(recorder, r)
	return recorder.Result(), nil
}

type resolvedContractServer struct {
	URL    string
	client *http.Client
}

func newResolvedContractServer(handler http.Handler) *resolvedContractServer {
	return &resolvedContractServer{URL: "http://127.0.0.1:12345", client: &http.Client{Transport: resolvedContractTransport{handler}}}
}
func (s *resolvedContractServer) Client() *http.Client { return s.client }
func (s *resolvedContractServer) Close()               {}

// Disabled grouping remains individual and hits the unchanged 10/minute
// destination limit. The extra deliveries now retain the shared retry horizon
// rather than exhausting a caller-local three-attempt budget. Drive claims and
// the limiter's next-window state explicitly; real due-time/backoff acceptance
// is covered by TestOrdinaryNotificationTransientRetryHorizon.
func TestResolvedGroupingDisabledBurstRetainsRateLimit(t *testing.T) {
	now := time.Now()
	q := newQuietReplayQueue(t, t.TempDir(), &now)
	requests := 0
	server := newResolvedContractServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	hook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL, Enabled: true}
	m := &NotificationManager{enabled: true, notifyOnResolve: true, queue: q,
		webhookClient: server.Client(), webhooks: []WebhookConfig{hook},
		webhookRateLimits: make(map[string]*webhookRateLimit),
		lastNotified:      make(map[string]notificationRecord), deliveryReceipts: make(map[string]struct{}),
		groupingEnabled: true, groupWindow: time.Hour}
	t.Cleanup(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.groupTimer != nil {
			m.groupTimer.Stop()
		}
	})
	if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	q.SetProcessor(m.ProcessQueuedNotification)
	batch := make([]*alerts.Alert, 15)
	for i := range batch {
		batch[i] = &alerts.Alert{ID: fmt.Sprintf("limit-%d", i), ResourceName: fmt.Sprintf("limit-%d", i), StartTime: now.Add(-time.Minute), Level: alerts.AlertLevelWarning}
		m.SendAlert(batch[i])
	}
	m.sendGroupedAlerts()
	rows := quietReplayRows(t, q)
	if len(rows) != 1 {
		t.Fatalf("firing groups=%d, want 1", len(rows))
	}
	q.processNotification(rows[0])
	m.SetGroupingConfig(false, 1, false, false)
	for _, a := range batch {
		m.CancelAlert(a.ID)
		m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: now})
	}
	for attempt := 1; attempt <= 3; attempt++ {
		for _, row := range quietReplayRows(t, q) {
			if row.Status == QueueStatusPending {
				q.processNotification(row)
			}
		}
		stats, err := q.GetQueueStats()
		if err != nil || stats["pending"] != 6 || stats["sent"] != 10 || stats["dlq"] != 0 || requests != 10 {
			t.Fatalf("attempt %d: rows=%v requests=%d error=%v; want six pending, ten sent and ten requests", attempt, stats, requests, err)
		}
	}
	for _, row := range quietReplayRows(t, q) {
		if row.Status != QueueStatusPending {
			continue
		}
		if row.Attempts != 3 || row.MaxAttempts != defaultQueueMaxAttempts {
			t.Fatalf("retry lost shared budget: %+v", row)
		}
		entries, err := q.GetDeliveryLog(now.Add(-time.Hour), 200)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range entries {
			if entry.NotificationID == row.ID {
				count++
				if entry.Outcome != DeliveryOutcomeRetry || entry.Success || entry.FailureClass != string(NotificationFailureRateLimited) {
					t.Fatalf("wrong rate-limited outcome: %+v", entry)
				}
			}
		}
		if count != 3 {
			t.Fatalf("retry audits=%d, want 3", count)
		}
	}
	// Expire this test receiver's real limiter state, without changing its
	// count/window constants or retry deadlines. No background workers exist.
	m.webhookRateMu.Lock()
	m.webhookRateLimits[hook.URL].lastSent = time.Now().Add(-WebhookRateLimitWindow - time.Second)
	m.webhookRateMu.Unlock()
	for _, row := range quietReplayRows(t, q) {
		if row.Status == QueueStatusPending {
			q.processNotification(row)
		}
	}
	stats, err := q.GetQueueStats()
	if err != nil || stats["pending"] != 0 || stats["sent"] != 16 || stats["dlq"] != 0 || requests != 16 {
		t.Fatalf("next-window recovery: rows=%v requests=%d error=%v; want sixteen sent", stats, requests, err)
	}
	telemetry, err := q.GetTelemetryStats(now.Add(-time.Hour))
	if err != nil || telemetry.Attempts != 34 || telemetry.Deliveries != 16 || telemetry.Failures != 0 {
		t.Fatalf("recovery accounting: %+v error=%v", telemetry, err)
	}
	t.Logf("ungrouped burst: six rate-limited jobs remain pending after three failures; next-window acceptance gives %d requests, %d audited attempts, no dead letters", requests, telemetry.Attempts)
}

// A restart must not terminally cancel a due persisted delivery before the
// owner has applied saved destination configuration. Manager construction
// happens before saved webhooks/email/Apprise are loaded in the monitor
// startup path; the worker must not treat the still-empty destination list as
// a permanent policy decision.
func TestRestartKeepsDueDeliveryPendingUntilConfigured(t *testing.T) {
	received := make(chan struct{}, 4)
	server := newResolvedContractServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	hook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL, Enabled: true}
	configJSON, err := json.Marshal(hook)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := NewNotificationQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	due := time.Now().Add(-time.Second)
	if err := seed.Enqueue(&QueuedNotification{
		ID:          "restart-due",
		Type:        "webhook_resolved",
		Status:      QueueStatusPending,
		Config:      configJSON,
		Alerts:      []*alerts.Alert{{ID: "restart-due", ResourceName: "restart-due", StartTime: time.Now().Add(-time.Minute)}},
		MaxAttempts: 3,
		NextRetryAt: &due,
	}); err != nil {
		t.Fatal(err)
	}
	if err := seed.Stop(); err != nil {
		t.Fatal(err)
	}

	// Production order: construct the manager, then apply saved configuration.
	m := NewNotificationManagerWithDeferredQueue("", dir)
	defer m.Stop()
	m.webhookClient = server.Client()
	if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	// Force the same batch path the five-second ticker uses while saved
	// configuration is still absent. It must leave the due row pending.
	queue := m.GetQueue()
	if queue == nil {
		t.Fatal("queue unavailable")
	}
	queue.processBatch()
	var status string
	if err := queue.db.QueryRow(`SELECT status FROM notification_queue WHERE id = 'restart-due'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(QueueStatusPending) {
		t.Fatalf("due delivery status before configuration = %s, want pending", status)
	}

	m.AddWebhook(hook)
	m.StartQueueProcessing()
	select {
	case <-received:
	case <-time.After(5 * time.Second):
		stats, _ := queue.GetQueueStats()
		t.Fatalf("configured due delivery not sent: %v", stats)
	}
}

func TestResolvedGroupingPagerDutyKeepsIndividualKeys(t *testing.T) {
	hook := WebhookConfig{ID: "pd", Service: "pagerduty", Enabled: true}
	batch := contractGroupedAlerts()
	jobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{hook}, AppriseConfig{}, batch, eventResolved, time.Now())
	if len(jobs) != len(batch) {
		t.Fatalf("PagerDuty jobs=%d, want %d independent dedup keys", len(jobs), len(batch))
	}
	for i, job := range jobs {
		if len(job.Alerts) != 1 || job.Alerts[0].ID != batch[i].ID {
			t.Fatalf("wrong PagerDuty occurrence: %+v", job)
		}
	}
}
