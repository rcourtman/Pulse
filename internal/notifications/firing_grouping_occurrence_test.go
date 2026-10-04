package notifications

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

func pendingGroupSnapshot(id string, start, seen time.Time, level alerts.AlertLevel, message string) *alerts.Alert {
	a := quietReplayAlert(id, level, start)
	a.LastSeen = seen
	a.Message = message
	a.Metadata = map[string]interface{}{"snapshot": message}
	a.LatestTransition.ID = "transition-" + message
	return a
}

func TestPendingFiringGroupSnapshotOrdering(t *testing.T) {
	start := time.Now().UTC().Add(-time.Minute)
	first := pendingGroupSnapshot("cpu", start, start, alerts.AlertLevelWarning, "first")
	for _, tc := range []struct {
		name    string
		before  func(*alerts.Alert)
		update  func(*alerts.Alert)
		wantNew bool
		wantTwo bool
	}{
		{name: "repeat", wantNew: true},
		{name: "newer_critical", update: func(a *alerts.Alert) { a.LastSeen = start.Add(time.Second); a.Level = alerts.AlertLevelCritical }, wantNew: true},
		{name: "newer_downgrade", before: func(a *alerts.Alert) { a.Level = alerts.AlertLevelCritical }, update: func(a *alerts.Alert) { a.LastSeen = start.Add(time.Second) }, wantNew: true},
		{name: "older_critical", update: func(a *alerts.Alert) { a.LastSeen = start.Add(-time.Second); a.Level = alerts.AlertLevelCritical }},
		{name: "equal_time_critical", update: func(a *alerts.Alert) { a.Level = alerts.AlertLevelCritical }, wantNew: true},
		{name: "equal_time_downgrade", before: func(a *alerts.Alert) { a.Level = alerts.AlertLevelCritical }},
		{name: "unknown_observation", update: func(a *alerts.Alert) { a.LastSeen = time.Time{}; a.Level = alerts.AlertLevelCritical }},
		{name: "known_observation", before: func(a *alerts.Alert) { a.LastSeen = time.Time{} }, wantNew: true},
		{name: "same_instant_different_location", update: func(a *alerts.Alert) { a.StartTime = start.In(time.FixedZone("other", 3600)) }, wantNew: true},
		{name: "same_instant_without_monotonic", before: func(a *alerts.Alert) { a.StartTime = time.Now() }, update: func(a *alerts.Alert) { a.StartTime = a.StartTime.Round(0) }, wantNew: true},
		{name: "different_occurrence", update: func(a *alerts.Alert) { a.StartTime = start.Add(time.Nanosecond) }, wantTwo: true},
		{name: "different_identifier", update: func(a *alerts.Alert) { a.ID = "other-cpu" }, wantTwo: true},
		{name: "unknown_start", before: func(a *alerts.Alert) { a.StartTime = time.Time{} }, wantTwo: true},
		{name: "unknown_identifier", before: func(a *alerts.Alert) { a.ID = "" }, wantTwo: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			t.Cleanup(m.Stop)
			m.SetGroupingConfig(true, 3600, false, false)
			a := first.Clone()
			if tc.before != nil {
				tc.before(a)
			}
			m.SendAlert(a)
			m.mu.RLock()
			timer := m.groupTimer
			m.mu.RUnlock()
			incoming := pendingGroupSnapshot(a.ID, a.StartTime, first.LastSeen, alerts.AlertLevelWarning, "incoming")
			if tc.update != nil {
				tc.update(incoming)
			}
			m.SendAlert(nil)
			m.SendAlert(incoming)
			m.mu.RLock()
			defer m.mu.RUnlock()
			require.Same(t, timer, m.groupTimer, "an update must not restart the grouping deadline")
			require.Empty(t, m.lastNotified, "group admission is not delivery")
			if tc.wantTwo {
				require.Len(t, m.pendingAlerts, 2, "incomplete or distinct identities must not merge")
				require.Equal(t, a, m.pendingAlerts[0])
				require.Equal(t, incoming, m.pendingAlerts[1])
				return
			}
			require.Len(t, m.pendingAlerts, 1, "one occurrence must occupy one group slot")
			want := a
			if tc.wantNew {
				want = incoming
			}
			require.Equal(t, want, m.pendingAlerts[0], "payload and operational linkage must come from the same selected snapshot")
			incoming.Metadata["snapshot"] = "caller-mutated"
			require.NotEqual(t, incoming.Metadata, m.pendingAlerts[0].Metadata, "pending snapshot must not alias its caller")
		})
	}
}

func TestPendingFiringGroupConcurrentSnapshots(t *testing.T) {
	m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(m.Stop)
	m.SetGroupingConfig(true, 3600, false, false)
	start := time.Now().UTC().Add(-time.Minute)
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.SendAlert(pendingGroupSnapshot("cpu", start, start.Add(time.Duration(i)*time.Second), alerts.AlertLevelWarning, strconv.Itoa(i)))
		}(i)
	}
	wg.Wait()
	m.mu.RLock()
	defer m.mu.RUnlock()
	require.Len(t, m.pendingAlerts, 1)
	require.Equal(t, "63", m.pendingAlerts[0].Message)
	require.Equal(t, "transition-63", m.pendingAlerts[0].LatestTransition.ID)
	require.Empty(t, m.lastNotified)
}

func TestPendingFiringGroupCancellationAndFlush(t *testing.T) {
	for _, target := range []string{"all", "webhook"} {
		for _, flush := range []string{"disable", "zero_window", "disable_notifications"} {
			t.Run(target+"/"+flush, func(t *testing.T) {
				m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
				t.Cleanup(m.Stop)
				m.SetGroupingConfig(true, 3600, false, false)
				m.SetInitialNotifyTarget(target)
				m.SetEmailConfig(EmailConfig{Enabled: true, SMTPHost: "smtp.example.test", SMTPPort: 587, From: "pulse@example.test", To: []string{"ops@example.test"}})
				m.SetAppriseConfig(AppriseConfig{Enabled: true, Targets: []string{"ntfy://example.test/pulse"}})
				m.AddWebhook(WebhookConfig{ID: "ops", URL: "https://ops.example.test", Enabled: true})
				start := time.Now().UTC().Add(-time.Minute)
				old := pendingGroupSnapshot("cpu", start, start, alerts.AlertLevelCritical, "old")
				current := pendingGroupSnapshot("cpu", start.Add(time.Nanosecond), start, alerts.AlertLevelWarning, "current")
				current.OperationalRecord.ID += "-new"
				peer := pendingGroupSnapshot("memory", start, start, alerts.AlertLevelWarning, "peer")
				for _, a := range []*alerts.Alert{old, current, old, peer, current} {
					m.SendAlert(a)
				}
				require.True(t, m.CancelResolvedAlert(old))
				// Rebuild after cancellation; a stale index must neither overwrite
				// the peer nor append the current occurrence again.
				current.LastSeen = start.Add(time.Second)
				current.Message = "updated-current"
				current.LatestTransition.ID = "updated-transition"
				m.SendAlert(current)
				m.mu.RLock()
				pending := append([]*alerts.Alert(nil), m.pendingAlerts...)
				m.mu.RUnlock()
				require.Equal(t, []*alerts.Alert{current, peer}, pending)
				switch flush {
				case "disable":
					m.SetGroupingConfig(false, 30, false, false)
				case "zero_window":
					m.SetGroupingConfig(true, 0, false, false)
				case "disable_notifications":
					m.SetEnabled(false)
					m.SetEnabled(true)
					m.SendAlert(peer)
					m.SetGroupingConfig(false, 0, false, false)
					pending = []*alerts.Alert{peer}
				}
				rows := quietReplayRows(t, m.GetQueue())
				destinations := 3
				if target == "webhook" {
					destinations = 1
				}
				require.Len(t, rows, destinations*len(pending))
				counts := map[string]int{}
				for _, row := range rows {
					require.Equal(t, QueueStatusPending, row.Status)
					require.Zero(t, row.Attempts)
					require.Len(t, row.Alerts, 1)
					require.Len(t, row.Links, 1)
					a := row.Alerts[0]
					require.Equal(t, a.LatestTransition.ID, row.Links[0].TransitionID)
					require.False(t, old.ID == a.ID && old.StartTime.Equal(a.StartTime), "cancelled occurrence must not reach the queue")
					counts[row.Type+"/"+a.ID]++
				}
				for _, count := range counts {
					require.Equal(t, 1, count, "flush duplicated an occurrence/destination")
				}
				m.mu.RLock()
				require.Empty(t, m.pendingAlerts)
				require.Nil(t, m.groupTimer)
				require.Empty(t, m.lastNotified, "persisting pending work is not a successful delivery")
				m.mu.RUnlock()
			})
		}
	}
}

// The real timer admits one snapshot per occurrence into the persistent queue.
// Bootstrap restart, autonomous processing and recovery use normal entry points;
// no synthetic successful receipt, direct dispatcher or edited queue clock.
func TestPendingFiringGroupHTTPAcrossRestart(t *testing.T) {
	type message struct {
		destination string
		Event       string          `json:"event"`
		Grouped     bool            `json:"grouped"`
		Alerts      []*alerts.Alert `json:"alerts"`
		Count       int             `json:"count"`
	}
	received := make(chan message, 16)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var msg message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			t.Errorf("decode grouped delivery: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		msg.destination = r.URL.Path
		select {
		case received <- msg:
		default:
			t.Error("unexpected excess grouped deliveries")
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	dir := t.TempDir()
	hooks := []WebhookConfig{
		{ID: "left", URL: server.URL + "/left", Enabled: true, Service: "generic"},
		{ID: "right", URL: server.URL + "/right", Enabled: true, Service: "generic"},
	}
	open := func() *NotificationManager {
		m := NewNotificationManagerWithDeferredQueue("", dir)
		t.Cleanup(m.Stop)
		m.webhookClient = server.Client()
		require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
		m.SetCooldown(0)
		m.SetGroupingConfig(true, 1, false, false)
		for _, hook := range hooks {
			m.AddWebhook(hook)
		}
		return m
	}
	wait := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for !check() {
			if time.Now().After(deadline) {
				t.Fatal("grouped delivery did not complete")
			}
			time.Sleep(time.Millisecond)
		}
	}
	m := open()
	start := time.Now().UTC().Add(-time.Minute)
	current := pendingGroupSnapshot("cpu", start, start, alerts.AlertLevelWarning, "warning")
	peer := pendingGroupSnapshot("memory", start, start, alerts.AlertLevelWarning, "peer")
	for range 32 {
		m.SendAlert(current)
	}
	m.SendAlert(peer)
	current.LastSeen = time.Now().UTC()
	current.Level = alerts.AlertLevelCritical
	current.Message = "current critical"
	current.LatestTransition.ID = "current-critical-transition"
	m.SendAlert(current)
	// Actual timer expiry, not a call to the private flush method.
	wait(func() bool { return len(quietReplayRows(t, m.GetQueue())) == 2 })
	for _, row := range quietReplayRows(t, m.GetQueue()) {
		require.Len(t, row.Alerts, 2, "callback count leaked into persisted payload")
		require.Equal(t, current, row.Alerts[0])
		require.Equal(t, peer, row.Alerts[1])
		require.Len(t, row.Links, 2)
		require.Equal(t, current.LatestTransition.ID, row.Links[0].TransitionID)
		require.Zero(t, row.Attempts)
	}
	m.Stop()
	m = open()
	m.StartQueueProcessing()
	wait(func() bool {
		stats, err := m.GetQueue().GetQueueStats()
		require.NoError(t, err)
		return stats["sent"] == 2
	})
	firingDestinations := map[string]bool{}
	for range 2 {
		select {
		case msg := <-received:
			require.True(t, msg.Grouped)
			require.Equal(t, 2, msg.Count)
			require.Len(t, msg.Alerts, 2)
			require.Equal(t, current.Message, msg.Alerts[0].Message)
			require.Equal(t, current.Level, msg.Alerts[0].Level)
			require.Equal(t, peer.ID, msg.Alerts[1].ID)
			require.Contains(t, []string{"/left", "/right"}, msg.destination)
			require.False(t, firingDestinations[msg.destination], "one destination cannot stand in for another")
			firingDestinations[msg.destination] = true
		default:
			t.Fatal("successful queued firing has no receiver evidence")
		}
	}
	m.SendAlert(current)
	m.mu.RLock()
	require.Empty(t, m.pendingAlerts, "delivered occurrence must keep its ordinary cooldown")
	m.mu.RUnlock()
	require.False(t, m.CancelResolvedAlert(current))
	m.SetGroupingConfig(false, 0, false, false)
	m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: current, ResolvedTime: time.Now()})
	wait(func() bool {
		stats, err := m.GetQueue().GetQueueStats()
		require.NoError(t, err)
		return stats["sent"] == 4
	})
	recoveryDestinations := map[string]bool{}
	for range 2 {
		select {
		case msg := <-received:
			require.Equal(t, "resolved", msg.Event)
			require.Len(t, msg.Alerts, 1)
			require.Equal(t, current.ID, msg.Alerts[0].ID)
			require.True(t, firingDestinations[msg.destination])
			require.False(t, recoveryDestinations[msg.destination])
			recoveryDestinations[msg.destination] = true
		default:
			t.Fatal("successful queued recovery has no receiver evidence")
		}
	}
	m.Stop()
	m = open()
	for _, hook := range hooks {
		for _, tc := range []struct {
			a    *alerts.Alert
			want int
		}{{current, 0}, {peer, 1}} {
			jobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{hook}, AppriseConfig{}, []*alerts.Alert{tc.a}, eventResolved, time.Now())
			require.Len(t, m.filterResolvedJobsByDeliveryReceipt(jobs), tc.want, "reopen must preserve each destination/occurrence receipt independently")
		}
	}
	select {
	case extra := <-received:
		t.Fatalf("duplicate provider message: %+v", extra)
	default:
	}
	t.Log("34 callbacks become two occurrence snapshots per destination; actual timer, persisted restart, two HTTP 200 firing deliveries and two matching recoveries pass, without duplicate receiver messages.")
}

func TestPendingFiringGroupStopCancelsTimer(t *testing.T) {
	received := make(chan struct{}, 4)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- struct{}{}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(m.Stop)
	m.webhookClient = server.Client()
	require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	m.AddWebhook(WebhookConfig{ID: "ops", URL: server.URL, Enabled: true})
	m.SetGroupingConfig(true, 1, false, false)
	start := time.Now().UTC()
	m.SendAlert(pendingGroupSnapshot("cpu", start, start, alerts.AlertLevelWarning, "pending at stop"))
	m.Stop()
	// An abandoned timer must not dispatch directly after Stop clears the queue.
	select {
	case <-received:
		t.Fatal("stopped manager delivered its abandoned firing group")
	case <-time.After(1500 * time.Millisecond):
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	require.Empty(t, m.pendingAlerts)
	require.Nil(t, m.groupTimer)
	require.Empty(t, m.lastNotified)
}
