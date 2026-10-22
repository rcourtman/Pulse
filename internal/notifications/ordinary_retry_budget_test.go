package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercise manager admission, not the queue's independently correct default.
// No processor is installed: these controls never contact any destination.
func TestOrdinaryNotificationRetryBudget(t *testing.T) {
	for _, event := range []notificationEvent{eventAlert, eventResolved} {
		for _, mode := range []string{"immediate", "grouped", "quiet", "grouped_quiet"} {
			t.Run(string(event)+"/"+mode, func(t *testing.T) {
				now := time.Now().UTC().Truncate(time.Second)
				q := newQuietReplayQueue(t, t.TempDir(), &now)
				m := &NotificationManager{enabled: true, notifyOnResolve: true, queue: q,
					emailConfig:   EmailConfig{Enabled: true, SMTPHost: "smtp.example.test", From: "pulse@example.test", To: []string{"ops@example.test"}},
					webhooks:      []WebhookConfig{{ID: "ops", Enabled: true, URL: "https://receiver.example.test/hook"}},
					appriseConfig: AppriseConfig{Enabled: true, Mode: AppriseModeCLI, Targets: []string{"mailto://ops@example.test"}},
					lastNotified:  make(map[string]notificationRecord), deliveryReceipts: make(map[string]struct{}),
					groupingEnabled: strings.HasPrefix(mode, "grouped"), groupWindow: time.Hour,
				}
				t.Cleanup(func() {
					m.mu.Lock()
					defer m.mu.Unlock()
					if m.groupTimer != nil {
						m.groupTimer.Stop()
					}
					if m.resolvedGroupTimer != nil {
						m.resolvedGroupTimer.Stop()
					}
				})
				a := quietReplayAlert("synthetic-retry-budget", alerts.AlertLevelWarning, now.Add(-time.Minute))
				if strings.Contains(mode, "quiet") {
					a.Metadata = map[string]interface{}{alerts.MetadataQuietHoursReplayAt: now.Add(2 * time.Hour).Format(time.RFC3339)}
				}
				if event == eventResolved {
					// Fixture precondition: only destinations with a firing receipt can clear.
					for _, job := range buildNotificationDeliveryJobs(m.emailConfig, m.webhooks, m.appriseConfig, []*alerts.Alert{a}, eventAlert, time.Time{}) {
						m.recordSuccessfulDelivery(job, now)
					}
					m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: now})
				} else {
					m.SendAlert(a)
					if m.groupingEnabled {
						m.sendGroupedAlerts()
					}
				}
				rows := quietReplayRows(t, q)
				require.Len(t, rows, 3)
				seen := map[string]bool{}
				for _, row := range rows {
					kind, gotEvent := normalizeQueueType(row.Type)
					seen[kind] = true
					require.Equal(t, event, gotEvent)
					require.Equal(t, defaultQueueMaxAttempts, row.MaxAttempts, "ordinary %s admission must use the queue default", row.Type)
					require.Equal(t, QueueStatusPending, row.Status)
					require.Zero(t, row.Attempts)
					require.Len(t, row.Alerts, 1)
					require.Equal(t, a.ID, row.Alerts[0].ID)
					require.True(t, row.Alerts[0].StartTime.Equal(a.StartTime))
					require.Len(t, row.Links, 1)
					require.Equal(t, "record-"+a.ID, row.Links[0].OperationalRecordID)
					require.Equal(t, "transition-"+a.ID, row.Links[0].TransitionID)
					if strings.Contains(mode, "quiet") || event == eventResolved && m.groupingEnabled {
						require.NotNil(t, row.NextRetryAt)
						require.True(t, row.NextRetryAt.After(now))
					}
					job := buildNotificationDeliveryJobs(m.emailConfig, m.webhooks, m.appriseConfig, []*alerts.Alert{a}, event, now)
					for _, expected := range job {
						if expected.Type == kind {
							config, err := configJSONForNotificationDeliveryJob(expected)
							require.NoError(t, err)
							require.JSONEq(t, string(config), string(row.Config))
						}
					}
				}
				require.Len(t, seen, 3)
				log, err := q.GetDeliveryLog(now.Add(-time.Hour), 200)
				require.NoError(t, err)
				require.Empty(t, log, "admission and holds are not provider attempts")
			})
		}
	}
}

type retryHorizonBrowserSnapshot struct {
	Entries []DeliveryLogEntry `json:"entries"`
	Queue   map[string]int     `json:"queue"`
	Health  DeliveryHealth     `json:"health"`
}

func retryBudgetSnapshot(t *testing.T, m *NotificationManager) retryHorizonBrowserSnapshot {
	t.Helper()
	entries, err := m.queue.GetDeliveryLog(time.Now().Add(-time.Hour), 200)
	require.NoError(t, err)
	queue, err := m.queue.GetQueueStats()
	require.NoError(t, err)
	return retryHorizonBrowserSnapshot{entries, queue, m.DeliveryHealth()}
}

// Use the real due-row selector and unchanged backoff on a local HTTP receiver.
// The old caller reaches a dead letter after rejection three, before recovery.
func TestOrdinaryNotificationTransientRetryHorizon(t *testing.T) {
	for _, service := range []string{"generic", "ntfy"} {
		for _, event := range []notificationEvent{eventAlert, eventResolved} {
			t.Run(service+"/"+string(event), func(t *testing.T) {
				now := time.Now().UTC()
				q := newQuietReplayQueue(t, t.TempDir(), &now)
				var attempts atomic.Int32
				eventID := ""
				server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					attempt := attempts.Add(1)
					body, err := io.ReadAll(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, "/topic", r.URL.Path)
					assert.Equal(t, "POST", r.Method)
					assert.Contains(t, string(body), "Synthetic retry host")
					if service == "generic" || event == eventAlert {
						if attempt == 1 {
							eventID = r.Header.Get("X-Pulse-Event-ID")
						}
						assert.NotEmpty(t, eventID)
						assert.Equal(t, eventID, r.Header.Get("X-Pulse-Event-ID"))
					}
					if attempt <= 3 {
						w.WriteHeader(http.StatusServiceUnavailable)
					} else {
						w.WriteHeader(http.StatusNoContent)
					}
				}))
				defer server.Close()
				hook := WebhookConfig{ID: "ops", Name: "Synthetic operations", Enabled: true, Service: service, URL: server.URL + "/topic"}
				m := &NotificationManager{enabled: true, notifyOnResolve: true, queue: q, webhooks: []WebhookConfig{hook},
					webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit),
					lastNotified: make(map[string]notificationRecord), deliveryReceipts: make(map[string]struct{})}
				require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
				q.SetProcessor(m.ProcessQueuedNotification)
				a := quietReplayAlert("synthetic-retry-host", alerts.AlertLevelWarning, now.Add(-time.Minute))
				a.ResourceName = "Synthetic retry host"
				a.Message = "Synthetic retry host CPU warning"
				if event == eventResolved {
					m.recordSuccessfulDelivery(notificationDeliveryJob{Type: "webhook", Event: eventAlert, Alerts: []*alerts.Alert{a}, WebhookConfig: &hook}, now)
					m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: now})
				} else {
					m.SendAlert(a)
				}
				rows := quietReplayRows(t, q)
				require.Len(t, rows, 1)
				id := rows[0].ID
				var retry, sent retryHorizonBrowserSnapshot
				started := time.Now()
				deadline := started.Add(22 * time.Second)
				for {
					due, err := q.GetPending(10)
					require.NoError(t, err)
					for _, row := range due {
						q.processNotification(row)
						persisted := loadQuietReplay(t, q, id)
						if attempts.Load() == 3 {
							require.Equal(t, QueueStatusPending, persisted.Status, "a transient third failure must remain eligible for recovery")
							require.Equal(t, 3, persisted.Attempts)
							require.NotNil(t, persisted.NextRetryAt)
							// Whole-second storage may round down, but must retain the 8s backoff.
							delta := persisted.NextRetryAt.Sub(*persisted.LastAttempt)
							require.GreaterOrEqual(t, delta, 7*time.Second)
							require.LessOrEqual(t, delta, 8*time.Second)
							retry = retryBudgetSnapshot(t, m)
							require.Len(t, retry.Entries, 3)
							for _, entry := range retry.Entries {
								require.Equal(t, DeliveryOutcomeRetry, entry.Outcome)
								require.False(t, entry.Success)
								require.Equal(t, string(NotificationFailureServerError), entry.FailureClass)
							}
						}
					}
					persisted := loadQuietReplay(t, q, id)
					if persisted.Status == QueueStatusSent {
						require.Equal(t, int32(4), attempts.Load())
						require.Equal(t, 4, persisted.Attempts)
						sent = retryBudgetSnapshot(t, m)
						require.Len(t, sent.Entries, 4)
						require.Equal(t, DeliveryOutcomeSent, sent.Entries[0].Outcome)
						require.True(t, sent.Entries[0].Success)
						stats, err := q.GetTelemetryStats(now.Add(-time.Hour))
						require.NoError(t, err)
						require.Equal(t, 4, stats.Attempts)
						require.Equal(t, 1, stats.Deliveries)
						require.Zero(t, stats.Failures)
						key := notificationDeliveryDestinationKey(notificationDeliveryJob{Type: "webhook", WebhookConfig: &hook})
						receipt, err := q.HasDeliveryReceipt(a.ID, a.StartTime, key)
						require.NoError(t, err)
						require.Equal(t, event == eventAlert, receipt, "only accepted firing creates, and accepted recovery consumes, its receipt")
						q.processNotification(persisted)
						require.Equal(t, int32(4), attempts.Load(), "completed work cannot be claimed again")
						break
					}
					require.NotEqual(t, QueueStatusDLQ, persisted.Status)
					require.True(t, time.Now().Before(deadline), "retry horizon timed out: %s/%d", persisted.Status, persisted.Attempts)
					time.Sleep(20 * time.Millisecond)
				}
				require.NotEmpty(t, retry.Entries)
				t.Logf("actual due-row HTTP recovery: service=%s event=%s attempts=%d elapsed=%s", service, event, attempts.Load(), time.Since(started))
				if service == "generic" && event == eventAlert {
					fixture, err := json.Marshal(map[string]retryHorizonBrowserSnapshot{"retrying": retry, "recovered": sent})
					require.NoError(t, err)
					t.Logf("RETRY_BROWSER_FIXTURE=%s", fixture)
				}
			})
		}
	}
}

// Bound the finite budget without waiting three minutes. These controlled
// claims retain and check the production backoff; they are not a scheduler test.
func TestOrdinaryNotificationRetryBudgetBoundaries(t *testing.T) {
	for _, mode := range []string{"exhausted", "authentication", "configuration", "disabled", "resolved-before-retry"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now().UTC()
			q := newQuietReplayQueue(t, t.TempDir(), &now)
			hook := WebhookConfig{ID: "ops", Enabled: true, URL: "https://receiver.example.test/hook"}
			m := &NotificationManager{enabled: true, queue: q, webhooks: []WebhookConfig{hook}, lastNotified: make(map[string]notificationRecord)}
			a := quietReplayAlert("synthetic-finite-retry", alerts.AlertLevelWarning, now.Add(-time.Minute))
			m.SendAlert(a)
			rows := quietReplayRows(t, q)
			require.Len(t, rows, 1)
			id := rows[0].ID
			calls := 0
			q.SetProcessor(func(*QueuedNotification) error {
				calls++
				switch mode {
				case "authentication":
					return FailfWithClass(NotificationFailureAuthentication, "synthetic authentication refusal")
				case "configuration":
					return FailfWithClass(NotificationFailureConfiguration, "synthetic invalid configuration")
				default:
					return FailfWithClass(NotificationFailureConnectivity, "synthetic connection refusal")
				}
			})
			stop := defaultQueueMaxAttempts
			if mode != "exhausted" {
				stop = 1
			}
			for attempt := 1; attempt <= stop; attempt++ {
				row := loadQuietReplay(t, q, id)
				require.Equal(t, QueueStatusPending, row.Status)
				q.processNotification(row)
				require.Equal(t, attempt, loadQuietReplay(t, q, id).Attempts)
			}
			switch mode {
			case "disabled":
				m.SetEnabled(false)
			case "resolved-before-retry":
				require.True(t, m.CancelResolvedAlert(a))
			}
			final := loadQuietReplay(t, q, id)
			expected := QueueStatusDLQ
			if mode == "disabled" || mode == "resolved-before-retry" {
				expected = QueueStatusCancelled
			}
			require.Equal(t, expected, final.Status)
			require.Equal(t, stop, final.Attempts)
			q.processNotification(final)
			require.Equal(t, stop, calls, "terminal work must not reach the processor")
			log, err := q.GetDeliveryLog(now.Add(-time.Hour), 200)
			require.NoError(t, err)
			require.Len(t, log, stop)
			if expected == QueueStatusDLQ {
				require.Equal(t, DeliveryOutcomeDeadLetter, log[0].Outcome)
			}
			t.Logf("finite admitted budget: %s providerCalls=%d final=%s audits=%d", mode, calls, final.Status, len(log))
		})
	}
}

func TestOrdinaryNotificationRetryBudgetKeepsRetainedWork(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	q := newQuietReplayQueue(t, t.TempDir(), &now)
	future := now.Add(time.Hour)
	old := &QueuedNotification{ID: "already-admitted", Type: "webhook", Status: QueueStatusPending,
		Attempts: 2, MaxAttempts: 3, NextRetryAt: &future, Config: []byte(`{"enabled":true,"id":"ops","url":"https://receiver.example.test/old"}`),
		Alerts: []*alerts.Alert{quietReplayAlert("old-occurrence", alerts.AlertLevelWarning, now.Add(-time.Hour))}}
	require.NoError(t, q.Enqueue(old))
	before := loadQuietReplay(t, q, old.ID)
	terminal := &QueuedNotification{ID: "already-terminal", Type: "webhook", Status: QueueStatusDLQ, Attempts: 3, MaxAttempts: 3,
		Config: []byte(`{"enabled":true,"id":"ops"}`), Alerts: []*alerts.Alert{quietReplayAlert("terminal-occurrence", alerts.AlertLevelWarning, now.Add(-time.Hour))}}
	require.NoError(t, q.Enqueue(terminal))
	terminalBefore := loadQuietReplay(t, q, terminal.ID)
	m := &NotificationManager{enabled: true, queue: q, lastNotified: make(map[string]notificationRecord),
		webhooks: []WebhookConfig{{ID: "ops", Enabled: true, URL: "https://receiver.example.test/new"}}}
	m.SendAlert(quietReplayAlert("new-occurrence", alerts.AlertLevelWarning, now))
	rows := quietReplayRows(t, q)
	require.Len(t, rows, 3)
	require.Equal(t, before, loadQuietReplay(t, q, old.ID), "new admission must not change an older budget, delay, payload or attempts")
	require.Equal(t, terminalBefore, loadQuietReplay(t, q, terminal.ID), "new admission must not replay or rewrite terminal work")
	for _, row := range rows {
		if row.ID != old.ID && row.ID != terminal.ID {
			require.Equal(t, defaultQueueMaxAttempts, row.MaxAttempts)
			require.Zero(t, row.Attempts)
			require.Equal(t, QueueStatusPending, row.Status)
		}
	}
}
