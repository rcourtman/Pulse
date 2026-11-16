package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func openRehearsalManager(t *testing.T, dir, receiver string) *NotificationManager {
	t.Helper()
	m := NewNotificationManagerWithDataDir("", dir)
	require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	m.SetGroupingConfig(false, 0, false, false)
	m.SetNotifyOnResolve(true)
	if receiver != "" {
		m.AddWebhook(WebhookConfig{ID: "rehearsal-receiver", Name: "Local receiver", URL: receiver, Enabled: true, Service: "generic"})
	}
	t.Cleanup(m.Stop)
	return m
}

func TestDeliveryRehearsalRealQueueAndRecoverySurviveRestart(t *testing.T) {
	var requests atomic.Int32
	bodies := make(chan []byte, 4)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requests.Add(1)
		bodies <- body
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	dir := t.TempDir()
	m := openRehearsalManager(t, dir, server.URL)
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning, Tags: []string{"production"}}
	run, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	require.Equal(t, input.ID, run.ID)
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && run.Destinations[0].Firing.Status == "passed"
	}, 10*time.Second, 10*time.Millisecond)
	require.Contains(t, string(<-bodies), "Delivery rehearsal")
	require.Equal(t, int32(1), requests.Load())
	_, err = m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	require.Equal(t, int32(1), requests.Load(), "idempotent POST must not send again")
	log, err := m.GetDeliveryLog(time.Now().Add(-time.Minute), 20)
	require.NoError(t, err)
	require.Len(t, log, 1)
	require.Equal(t, DeliveryOutcomeSent, log[0].Outcome)
	require.Equal(t, []string{"delivery-rehearsal:" + input.ID}, log[0].AlertIDs)
	m.Stop()
	m = openRehearsalManager(t, dir, server.URL)
	run, err = m.RecoverDeliveryRehearsal(input.ID)
	require.NoError(t, err)
	require.NotNil(t, run.RecoveredAt)
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && run.Destinations[0].Recovery.Status == "passed"
	}, 10*time.Second, 10*time.Millisecond)
	require.Contains(t, string(<-bodies), "Delivery rehearsal")
	_, err = m.RecoverDeliveryRehearsal(input.ID)
	require.NoError(t, err)
	require.Equal(t, int32(2), requests.Load(), "repeated recovery must not send again")
	encoded, err := json.Marshal(run)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), server.URL)
	require.NotContains(t, string(encoded), "metadata")
	require.NotContains(t, string(encoded), "owner")
}

func TestDeliveryRehearsalKeepsMixedDestinationOutcomesSeparate(t *testing.T) {
	var accepted, rejected atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/reject" {
			rejected.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		accepted.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	m := openRehearsalManager(t, t.TempDir(), "")
	for _, hook := range []WebhookConfig{
		{ID: "accepted", Name: "Accepted", URL: server.URL + "/accept", Enabled: true, Service: "generic"},
		{ID: "rejected", Name: "Rejected", URL: server.URL + "/reject", Enabled: true, Service: "generic"},
		{ID: "filtered", Name: "Filtered", URL: server.URL + "/filtered", Enabled: true, Service: "generic", MinimumSeverity: "critical"},
	} {
		m.AddWebhook(hook)
	}
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	var run *DeliveryRehearsal
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && len(run.Destinations) == 3 && run.Destinations[0].Firing.Status == "passed" && run.Destinations[1].Firing.Status == "failed" && run.Destinations[2].Firing.Status == "suppressed"
	}, 10*time.Second, 10*time.Millisecond)
	require.Equal(t, "severity", run.Destinations[2].Firing.Reason)
	m.AddWebhook(WebhookConfig{ID: "late", Name: "Added after firing", URL: server.URL + "/late", Enabled: true, Service: "generic"})
	_, err = m.RecoverDeliveryRehearsal(input.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && len(run.Destinations) == 4 && run.Destinations[0].Recovery.Status == "passed" && run.Destinations[1].Recovery.Status == "suppressed" && run.Destinations[2].Recovery.Status == "suppressed" && run.Destinations[3].Recovery.Status == "suppressed"
	}, 10*time.Second, 10*time.Millisecond)
	require.Equal(t, int32(2), accepted.Load(), "only an accepted firing authorizes recovery")
	require.Equal(t, int32(1), rejected.Load())
	require.Equal(t, "suppressed", run.Destinations[3].Firing.Status)
	require.Equal(t, "added_after_firing", run.Destinations[3].Firing.Reason)
}

func TestDeliveryRehearsalReportsRealSuppressionAndDeferral(t *testing.T) {
	for _, scenario := range []string{"paused", "severity", "tags", "grouping", "quiet-hours", "recovery-disabled", "no-destinations"} {
		t.Run(scenario, func(t *testing.T) {
			var requests atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusAccepted) }))
			defer server.Close()
			url := server.URL
			if scenario == "no-destinations" {
				url = ""
			}
			m := openRehearsalManager(t, t.TempDir(), url)
			if scenario == "paused" {
				m.SetEnabled(false)
			}
			if scenario == "severity" || scenario == "tags" {
				hook := m.GetWebhooks()[0]
				if scenario == "severity" {
					hook.MinimumSeverity = "critical"
				} else {
					hook.TagFilter = []string{"other"}
				}
				require.NoError(t, m.UpdateWebhook(hook.ID, hook))
			}
			if scenario == "grouping" {
				m.SetGroupingConfig(true, 600, false, false)
			}
			if scenario == "quiet-hours" {
				m.SetQuietHoursPolicyProvider(func() func(*alerts.Alert, time.Time) *time.Time {
					return func(*alerts.Alert, time.Time) *time.Time { at := time.Now().Add(time.Hour); return &at }
				})
			}
			if scenario == "recovery-disabled" {
				m.SetNotifyOnResolve(false)
			}
			input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
			run, err := m.StartDeliveryRehearsal(input)
			require.NoError(t, err)
			switch scenario {
			case "paused":
				require.Equal(t, "suppressed", run.Policy.Status)
				require.Equal(t, "notifications_disabled", run.Destinations[0].Firing.Reason)
			case "severity", "tags":
				require.Equal(t, "suppressed", run.Destinations[0].Firing.Status)
				require.Equal(t, scenario, run.Destinations[0].Firing.Reason)
			case "grouping", "quiet-hours":
				require.Equal(t, "deferred", run.Destinations[0].Firing.Status)
				require.NotNil(t, run.Destinations[0].Firing.NextAt)
			case "no-destinations":
				require.Empty(t, run.Destinations)
			case "recovery-disabled":
				require.Eventually(t, func() bool {
					run, err = m.GetDeliveryRehearsal(input.ID)
					return err == nil && run.Destinations[0].Firing.Status == "passed"
				}, 10*time.Second, 10*time.Millisecond)
			}
			run, err = m.RecoverDeliveryRehearsal(input.ID)
			require.NoError(t, err)
			if len(run.Destinations) > 0 {
				require.Equal(t, "suppressed", run.Destinations[0].Recovery.Status)
			}
			if scenario != "recovery-disabled" {
				require.Zero(t, requests.Load(), "suppression must not contact a receiver")
			}
		})
	}
}

func TestDeliveryRehearsalReportsTerminalFailureAndDoesNotInventRecovery(t *testing.T) {
	var requests atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "receiver-secret-must-not-be-returned")
	}))
	defer server.Close()
	m := openRehearsalManager(t, t.TempDir(), server.URL)
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	var run *DeliveryRehearsal
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && run.Destinations[0].Firing.Status == "failed"
	}, 10*time.Second, 10*time.Millisecond)
	require.Equal(t, int32(1), requests.Load())
	require.Equal(t, 1, run.Destinations[0].Firing.Attempts)
	require.Contains(t, run.Destinations[0].Firing.Diagnostic, "HTTP 400")
	require.NotContains(t, run.Destinations[0].Firing.Message, "process queued")
	encoded, _ := json.Marshal(run)
	require.NotContains(t, string(encoded), "receiver-secret")
	run, err = m.RecoverDeliveryRehearsal(input.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", run.Destinations[0].Firing.Status, "finishing must retain an actual provider failure")
	require.Equal(t, "suppressed", run.Destinations[0].Recovery.Status)
	require.Equal(t, int32(1), requests.Load())
}

func TestDeliveryRehearsalRetryUsesOrdinaryQueueAndKeepsAttemptEvidence(t *testing.T) {
	var requests atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	m := openRehearsalManager(t, t.TempDir(), server.URL)
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	var run *DeliveryRehearsal
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && run.Destinations[0].Firing.Attempts == 1 && run.Destinations[0].Firing.Status == "deferred"
	}, 10*time.Second, 10*time.Millisecond)
	require.Equal(t, "server_error", run.Destinations[0].Firing.FailureClass)
	require.Eventually(t, func() bool {
		run, err = m.GetDeliveryRehearsal(input.ID)
		return err == nil && run.Destinations[0].Firing.Status == "passed"
	}, 12*time.Second, 10*time.Millisecond)
	require.Equal(t, int32(2), requests.Load())
	require.Equal(t, 2, run.Destinations[0].Firing.Attempts)
	log, err := m.GetDeliveryLog(time.Now().Add(-time.Minute), 20)
	require.NoError(t, err)
	require.Len(t, log, 2)
	require.Equal(t, DeliveryOutcomeSent, log[0].Outcome)
	require.Equal(t, DeliveryOutcomeRetry, log[1].Outcome)
}

func TestDeliveryRehearsalExpirationCancelsOwnedPendingWork(t *testing.T) {
	m := openRehearsalManager(t, t.TempDir(), "https://example.invalid/hook")
	m.SetQuietHoursPolicyProvider(func() func(*alerts.Alert, time.Time) *time.Time {
		return func(*alerts.Alert, time.Time) *time.Time { at := time.Now().Add(time.Hour); return &at }
	})
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	q := m.GetQueue()
	_, err = q.db.Exec(`UPDATE notification_rehearsals SET created_at=? WHERE id=?`, time.Now().Add(-8*24*time.Hour).Unix(), input.ID)
	require.NoError(t, err)
	require.NoError(t, q.expireRehearsals())
	_, err = m.GetDeliveryRehearsal(input.ID)
	require.ErrorIs(t, err, ErrRehearsalNotFound)
	rows := quietReplayRows(t, q)
	require.Len(t, rows, 1)
	require.Equal(t, QueueStatusCancelled, rows[0].Status)
}

func TestDeliveryRehearsalNeverUsesDirectQueueFailureFallback(t *testing.T) {
	var requests atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusAccepted) }))
	defer server.Close()
	m := openRehearsalManager(t, t.TempDir(), server.URL)
	_, err := m.GetQueue().db.Exec(`CREATE TRIGGER refuse_rehearsal_enqueue BEFORE INSERT ON notification_queue BEGIN SELECT RAISE(ABORT,'test queue write failure'); END`)
	require.NoError(t, err)
	run, err := m.StartDeliveryRehearsal(RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning})
	require.NoError(t, err)
	require.Equal(t, "unavailable", run.Policy.Status)
	require.Equal(t, "unavailable", run.Destinations[0].Firing.Status)
	require.Zero(t, requests.Load())
}

func TestDeliveryRehearsalEvidenceFailureFencesAdmission(t *testing.T) {
	var requests atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(http.StatusAccepted) }))
	defer server.Close()
	m := openRehearsalManager(t, t.TempDir(), server.URL)
	_, err := m.GetQueue().db.Exec(`CREATE TRIGGER refuse_rehearsal_evidence BEFORE UPDATE ON notification_rehearsals BEGIN SELECT RAISE(ABORT,'test evidence write failure'); END`)
	require.NoError(t, err)
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err = m.StartDeliveryRehearsal(input)
	require.Error(t, err)
	_, err = m.GetDeliveryRehearsal(input.ID)
	require.Error(t, err, "stale admission evidence must never be published as current")
	_, err = m.StartDeliveryRehearsal(input)
	require.Error(t, err, "an uncertain request must never be resent")
	require.Empty(t, quietReplayRows(t, m.GetQueue()))
	require.Zero(t, requests.Load())
}

func TestDeliveryRehearsalInterruptedPartialAdmissionIsPerDestination(t *testing.T) {
	m := openRehearsalManager(t, t.TempDir(), "https://example.invalid/one")
	m.AddWebhook(WebhookConfig{ID: "second", Name: "Second", URL: "https://example.invalid/two", Enabled: true, Service: "generic"})
	m.SetQuietHoursPolicyProvider(func() func(*alerts.Alert, time.Time) *time.Time {
		return func(*alerts.Alert, time.Time) *time.Time { at := time.Now().Add(time.Hour); return &at }
	})
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	q := m.GetQueue()
	rows := quietReplayRows(t, q)
	require.Len(t, rows, 2)
	job, err := retainedDeliveryJob(rows[0].Type, rows[0].Config)
	require.NoError(t, err)
	missingDestination := notificationDestinationIDForJob(job)
	q.mu.Lock()
	_, err = q.db.Exec(`DELETE FROM notification_queue WHERE id=?`, rows[0].ID)
	require.NoError(t, err)
	saved, err := q.readRehearsalLocked(input.ID)
	require.NoError(t, err)
	saved.Owner = "previous-process"
	require.NoError(t, q.writeRehearsalLocked(saved))
	q.mu.Unlock()
	run, err := m.GetDeliveryRehearsal(input.ID)
	require.NoError(t, err)
	require.Equal(t, "unavailable", run.Policy.Status)
	for _, destination := range run.Destinations {
		if destination.ID == missingDestination {
			require.Equal(t, "unavailable", destination.Firing.Status)
		} else {
			require.Equal(t, "deferred", destination.Firing.Status, "retained work keeps its actual queue evidence")
		}
	}
}

func TestDeliveryRehearsalCheckpointAndMessageIsolation(t *testing.T) {
	dir := t.TempDir()
	m := openRehearsalManager(t, dir, "https://example.invalid/hook")
	m.SetGroupingConfig(true, 600, false, false)
	input := RehearsalRequest{ID: uuid.NewString(), Level: alerts.AlertLevelWarning}
	_, err := m.StartDeliveryRehearsal(input)
	require.NoError(t, err)
	m.SendAlert(&alerts.Alert{ID: "ordinary", Type: "cpu", ResourceName: "Real guest", Level: alerts.AlertLevelWarning, StartTime: time.Now()})
	m.Stop()
	m = openRehearsalManager(t, dir, "https://example.invalid/hook")
	run, err := m.GetDeliveryRehearsal(input.ID)
	require.NoError(t, err)
	require.Equal(t, "deferred", run.Destinations[0].Firing.Status)
	rows := quietReplayRows(t, m.GetQueue())
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.Len(t, row.Alerts, 1, "rehearsal must never share an incident message")
	}
	other := input
	other.ID = uuid.NewString()
	_, err = m.StartDeliveryRehearsal(other)
	require.ErrorIs(t, err, ErrRehearsalConflict)
}
