package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Exit without Stop or database Close: graceful reopen tests cannot establish
// that resolution rewrites and cancellations survive a process exit.
// This proves queue durability through local HTTP receipt, not installed
// provider delivery or power-loss recovery. An interrupted provider send
// remains at-least-once.
func TestQueueResolutionSurvivesAbruptProcessExit(t *testing.T) {
	const helperEnv = "PULSE_TEST_QUEUE_RESOLUTION_EXIT_DIR"
	const exitCode = 23
	const urlEnv = "PULSE_TEST_QUEUE_RESOLUTION_WEBHOOK_URL"
	webhook := func(url string) WebhookConfig {
		return WebhookConfig{ID: "restart-receipt", URL: url, Enabled: true, Service: "generic"}
	}
	states := []NotificationQueueStatus{QueueStatusPending, QueueStatusSending, QueueStatusFailed, QueueStatusDLQ}
	if dir := os.Getenv(helperEnv); dir != "" {
		q, err := NewNotificationQueue(dir)
		if err != nil {
			t.Fatal(err)
		}
		config, err := json.Marshal(webhook(os.Getenv(urlEnv)))
		if err != nil {
			t.Fatal(err)
		}
		enqueue := func(id, kind string, members ...string) *QueuedNotification {
			t.Helper()
			n := &QueuedNotification{ID: id, Type: kind, Status: QueueStatusPending, Config: config, MaxAttempts: 3}
			for _, member := range members {
				n.Alerts = append(n.Alerts, &alerts.Alert{ID: member})
			}
			if err := q.Enqueue(n); err != nil {
				t.Fatal(err)
			}
			return n
		}
		for _, status := range states {
			for _, grouped := range []bool{false, true} {
				id := string(status)
				members := []string{"recovered"}
				if grouped {
					id += "-group"
					members = append(members, "still-firing")
				}
				enqueue(id, "webhook", members...)
				if err := q.UpdateStatus(id, status, "destination unavailable"); err != nil {
					t.Fatal(err)
				}
			}
		}
		enqueue("recovery", "webhook_resolved", "recovered")
		if count, err := q.CancelByAlertIdentifiers([]string{"recovered"}); err != nil || count != 2 {
			t.Fatalf("resolution = %d, %v; want two pending members", count, err)
		}
		os.Exit(exitCode)
	}

	type receipt struct {
		Event  string          `json:"event"`
		Alerts []*alerts.Alert `json:"alerts"`
	}
	var receiptMu sync.Mutex
	var receipts []receipt
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var got receipt
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode webhook receipt: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receiptMu.Lock()
		receipts = append(receipts, got)
		receiptMu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	notifier := NewNotificationManagerWithDataDir("", t.TempDir())
	defer notifier.Stop()
	if err := notifier.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	notifier.AddWebhook(webhook(server.URL))

	dir := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQueueResolutionSurvivesAbruptProcessExit$")
	cmd.Env = append(os.Environ(), helperEnv+"="+dir, urlEnv+"="+server.URL)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("child timed out: %v\n%s", ctx.Err(), output)
	}
	if exited, ok := err.(*exec.ExitError); !ok || exited.ExitCode() != exitCode {
		t.Fatalf("child did not reach deliberate exit: %v\n%s", err, output)
	}
	q, err := NewNotificationQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Stop()
	if count, err := q.RetryTerminalFailures(); err != nil || count != 2 {
		t.Fatalf("terminal retry = %d, %v; want only failed/dlq surviving groups", count, err)
	}
	var mu sync.Mutex
	delivered := make(map[string][]string)
	// Exercise ordinary dispatch as well as repeated batch selection.
	q.SetProcessor(func(n *QueuedNotification) error {
		mu.Lock()
		defer mu.Unlock()
		for _, a := range n.Alerts {
			delivered[n.ID] = append(delivered[n.ID], a.ID)
		}
		return notifier.ProcessQueuedNotification(n)
	})
	q.processBatch()
	q.processBatch()
	deadline := time.Now().Add(3 * time.Second)
	for {
		stats, err := q.GetQueueStats()
		if err != nil {
			t.Fatal(err)
		}
		if stats["pending"]+stats["sending"] == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("restarted queue did not drain")
		}
		time.Sleep(time.Millisecond)
	}
	// All sends have finished before inspecting receipts; no sleep-based
	// absence assertion or mock transport is used.
	receiptMu.Lock()
	defer receiptMu.Unlock()
	if len(receipts) != 5 {
		t.Fatalf("HTTP receipts = %+v, want four firing groups and one recovery", receipts)
	}
	counts := make(map[string]int)
	for _, got := range receipts {
		if len(got.Alerts) != 1 || got.Alerts[0] == nil {
			t.Fatalf("unexpected HTTP members: %+v", got)
		}
		counts[got.Event+":"+got.Alerts[0].ID]++
	}
	if counts[":still-firing"] != 4 || counts["resolved:recovered"] != 1 {
		t.Fatalf("HTTP event/member counts = %v", counts)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 5 {
		t.Fatalf("delivered = %v; want four surviving groups and recovery", delivered)
	}
	for _, id := range []string{"pending-group", "sending-group", "failed-group", "dlq-group", "recovery"} {
		want := "still-firing"
		if id == "recovery" {
			want = "recovered"
		}
		if got := delivered[id]; len(got) != 1 || got[0] != want {
			t.Errorf("%s delivered %v, want [%s] exactly once in this run", id, got, want)
		}
	}
	for _, id := range []string{"pending", "sending", "failed", "dlq"} {
		var status string
		if err := q.db.QueryRow("SELECT status FROM notification_queue WHERE id = ?", id).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != string(QueueStatusCancelled) {
			t.Errorf("%s status = %s, want cancelled", id, status)
		}
	}
}
