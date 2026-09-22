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
	for _, stage := range []string{"same_process", "after_firing", "pending_recovery"} {
		t.Run(stage, func(t *testing.T) {
			type payload struct {
				Event  string          `json:"event"`
				Alerts []*alerts.Alert `json:"alerts"`
			}
			received := make(chan payload, 64)
			server := newResolvedContractServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var p payload
				if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
					t.Error(err)
				}
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
			for i, a := range batch {
				if m.CancelAlert(a.ID) {
					t.Fatal("delivered occurrence cancelled as unannounced")
				}
				m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: resolvedAt.Add(time.Duration(i) * time.Second)})
			}
			m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: &alerts.Alert{ID: "unannounced", ResourceName: "unannounced", StartTime: time.Now()}, ResolvedTime: resolvedAt})
			if stage == "pending_recovery" {
				m.Stop()
				m = open()
				m.AddWebhook(unseenHook)
			}
			select {
			case p := <-received:
				t.Fatalf("resolved delivery bypassed grouping window: event=%s alerts=%d", p.Event, len(p.Alerts))
			case <-time.After(150 * time.Millisecond):
			}
			var recovery payload
			select {
			case recovery = <-received:
			case <-time.After(12 * time.Second):
				// The queue processes on its own ticker; the wait must exceed that
				// interval so a grouped recovery is not raced by the poll period.
				t.Fatal("no grouped recovery")
			}
			if recovery.Event != "resolved" || len(recovery.Alerts) != 15 {
				t.Fatalf("recovery event=%s alerts=%d", recovery.Event, len(recovery.Alerts))
			}
			for i, a := range recovery.Alerts {
				if a.ID != batch[i].ID || a.Metadata[metadataResolvedAt] != resolvedAt.Add(time.Duration(i)*time.Second).Format(time.RFC3339) {
					t.Fatalf("lost occurrence/resolution metadata: %+v", a)
				}
			}
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

// Disabled grouping remains immediate. With the same 15-alert workload this
// deliberately hits the unchanged internal 10/minute destination limit; grouped
// recovery above sends one request instead and creates no dead letters.
func TestResolvedGroupingDisabledBurstRetainsRateLimit(t *testing.T) {
	server := newResolvedContractServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	m := NewNotificationManagerWithDataDir("", t.TempDir())
	defer m.Stop()
	m.webhookClient = server.Client()
	if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	hook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL, Enabled: true}
	m.AddWebhook(hook)
	m.SetNotifyOnResolve(true)
	m.SetGroupingConfig(true, 1, false, false)
	batch := make([]*alerts.Alert, 15)
	for i := range batch {
		batch[i] = &alerts.Alert{ID: fmt.Sprintf("limit-%d", i), ResourceName: fmt.Sprintf("limit-%d", i), StartTime: time.Now().Add(-time.Minute), Level: alerts.AlertLevelWarning}
		m.SendAlert(batch[i])
	}
	waitResolvedContract(t, func() bool { stats, err := m.GetQueue().GetQueueStats(); return err == nil && stats["sent"] == 1 })
	m.SetGroupingConfig(false, 1, false, false)
	for _, a := range batch {
		m.CancelAlert(a.ID)
		m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: time.Now()})
	}
	waitResolvedContract(t, func() bool {
		stats, err := m.GetQueue().GetQueueStats()
		return err == nil && stats["dlq"] == 6 && stats["sent"] == 10
	})
	dlq, err := m.GetQueue().GetDLQ(20)
	if err != nil {
		t.Fatal(err)
	}
	if len(dlq) != 6 {
		t.Fatalf("dead letters=%d", len(dlq))
	}
	for _, job := range dlq {
		if job.Attempts != 3 || (job.LastError == nil || !strings.Contains(*job.LastError, "rate limit exceeded")) {
			t.Errorf("wrong retry outcome: %+v", job)
		}
	}
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
	m := NewNotificationManagerWithDataDir("", dir)
	defer m.Stop()
	m.webhookClient = server.Client()
	if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	// A premature startup wake must not consume the due job before the
	// destination configuration is restored.
	time.Sleep(250 * time.Millisecond)
	queue := m.GetQueue()
	if queue == nil {
		t.Fatal("queue unavailable")
	}
	var status string
	if err := queue.db.QueryRow(`SELECT status FROM notification_queue WHERE id = 'restart-due'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != string(QueueStatusPending) {
		t.Fatalf("due delivery status before configuration = %s, want pending", status)
	}

	m.AddWebhook(hook)
	queue.processBatch()
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
