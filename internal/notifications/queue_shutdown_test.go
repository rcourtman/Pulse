package notifications

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

// Shutdown is a restart boundary, not an operator decision to stop delivery.
// Five real sends occupy every queue worker while a sixth admitted job waits
// for a slot, which is where shutdown finds an ordinary backlog. The waiting
// job must survive for the next start rather than be cancelled as "delivery
// disabled", and sends completing during shutdown must keep durable firing
// receipts so their recoveries are still delivered after restart.
func TestManagerStopKeepsQueuedDeliveryForRestart(t *testing.T) {
	type message struct {
		Event  string          `json:"event"`
		Alerts []*alerts.Alert `json:"alerts"`
	}
	received := make(chan message, 16)
	release := make(chan struct{})
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var msg message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil || len(msg.Alerts) != 1 {
			t.Errorf("decode delivery: %v (%d alerts)", err, len(msg.Alerts))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		select {
		case received <- msg:
		default:
			t.Error("unexpected excess deliveries")
		}
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	var releaseOnce sync.Once
	releaseSends := func() { releaseOnce.Do(func() { close(release) }) }

	dir := t.TempDir()
	hook := WebhookConfig{ID: "ops", URL: server.URL, Enabled: true, Service: "generic"}
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
	// Registered after the manager, so held sends are released before its Stop.
	t.Cleanup(releaseSends)

	start := time.Now().UTC().Add(-time.Minute)
	admitted := map[string]*alerts.Alert{}
	for i := range 6 {
		alert := quietReplayAlert(fmt.Sprintf("shutdown-%d", i), alerts.AlertLevelWarning, start)
		admitted[alert.ID] = alert
		m.SendAlert(alert)
	}
	pending, err := m.GetQueue().GetPending(20)
	require.NoError(t, err)
	require.Len(t, pending, 6, "every job must be admitted before workers start")

	q := m.GetQueue()
	m.StartQueueProcessing()
	inFlight := map[string]bool{}
	for range 5 {
		select {
		case msg := <-received:
			inFlight[msg.Alerts[0].ID] = true
		case <-time.After(10 * time.Second):
			t.Fatal("five queued sends did not occupy the worker pool")
		}
	}

	stopped := make(chan struct{})
	go func() {
		m.Stop()
		close(stopped)
	}()
	// Either half of shutdown may come first; release the held sends only
	// once the manager has begun to stop.
	stopBegun := func() bool {
		select {
		case <-q.stopChan:
			return true
		default:
			return !m.IsEnabled()
		}
	}
	deadline := time.Now().Add(10 * time.Second)
	for !stopBegun() {
		if time.Now().After(deadline) {
			t.Fatal("manager shutdown did not begin")
		}
		time.Sleep(time.Millisecond)
	}
	releaseSends()
	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		t.Fatal("manager shutdown did not finish its in-flight sends")
	}
	select {
	case extra := <-received:
		t.Fatalf("shutdown started a new provider attempt: %+v", extra)
	default:
	}

	m = open()
	q = m.GetQueue()
	var waiting *QueuedNotification
	for _, row := range quietReplayRows(t, q) {
		require.Len(t, row.Alerts, 1)
		if inFlight[row.Alerts[0].ID] {
			require.Equal(t, QueueStatusSent, row.Status, "a send completed during shutdown keeps its outcome")
			require.Equal(t, 1, row.Attempts)
			continue
		}
		require.Nil(t, waiting, "exactly one job waited for a worker")
		waiting = row
	}
	require.NotNil(t, waiting)

	delivered := make([]string, 0, len(inFlight))
	for id := range inFlight {
		delivered = append(delivered, id)
	}
	sort.Strings(delivered)
	for _, id := range delivered {
		jobs := buildNotificationDeliveryJobs(EmailConfig{}, []WebhookConfig{hook}, AppriseConfig{}, []*alerts.Alert{admitted[id]}, eventResolved, time.Now())
		if len(m.filterResolvedJobsByDeliveryReceipt(jobs)) != 1 {
			t.Errorf("SHUTDOWN_RECEIPT_LOST: %s reached the receiver during shutdown, but its recovery is suppressed after restart", id)
		}
	}
	if waiting.Status != QueueStatusPending || waiting.Attempts != 0 {
		lastError := ""
		if waiting.LastError != nil {
			lastError = *waiting.LastError
		}
		t.Fatalf("SHUTDOWN_CANCELLED_UNCLAIMED: job for %s that waited for a worker became %s after %d attempt(s) (%q); it must stay pending for the next start",
			waiting.Alerts[0].ID, waiting.Status, waiting.Attempts, lastError)
	}

	m.StartQueueProcessing()
	select {
	case msg := <-received:
		require.Equal(t, waiting.Alerts[0].ID, msg.Alerts[0].ID)
	case <-time.After(10 * time.Second):
		t.Fatal("the job kept across shutdown was not delivered after restart")
	}
	deadline = time.Now().Add(10 * time.Second)
	for loadQuietReplay(t, q, waiting.ID).Status != QueueStatusSent {
		if time.Now().After(deadline) {
			t.Fatal("the job kept across shutdown was not recorded as sent")
		}
		time.Sleep(time.Millisecond)
	}

	// The user-visible end of a receipt: a recovery after restart still
	// reaches the destination that received the firing during shutdown.
	recovered := delivered[0]
	m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: admitted[recovered], ResolvedTime: time.Now()})
	select {
	case msg := <-received:
		require.Equal(t, "resolved", msg.Event)
		require.Equal(t, recovered, msg.Alerts[0].ID)
	case <-time.After(10 * time.Second):
		t.Fatalf("recovery for %s was not delivered after restart", recovered)
	}
	select {
	case extra := <-received:
		t.Fatalf("duplicate provider message: %+v", extra)
	default:
	}
}
