package notifications

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolvedNtfyTruncatedResponseRetainsVerdict(t *testing.T) {
	for _, code := range []int{200, 401, 403, 421, 422, 423, 425, 429, 503} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "100")
				w.WriteHeader(code)
				_, _ = fmt.Fprint(w, "short")
			}))
			defer server.Close()
			manager := &NotificationManager{webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit)}
			if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			err := manager.sendResolvedWebhookNtfy(WebhookConfig{URL: server.URL, Service: "ntfy"}, nil, time.Now())
			if !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatalf("response read error lost: %v", err)
			}
			want := ClassFromHTTPStatus(code)
			if code == 200 { // Do not manufacture success for an incomplete 2xx response.
				want = NotificationFailureConnectivity
			}
			if got := ClassifyNotificationFailureError(err); got != want {
				t.Errorf("class = %s; want %s", got, want)
			}
		})
	}
}

func TestResolvedNtfyRejectionAuditAndOperatorRetry(t *testing.T) {
	var repaired atomic.Bool
	var requests atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if repaired.Load() {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, "short")
	}))
	defer server.Close()
	manager := &NotificationManager{webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit)}
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	open := func() *NotificationQueue {
		q, err := NewNotificationQueue(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = q.Stop() })
		return q
	}
	processor := func(*QueuedNotification) error {
		return manager.sendResolvedWebhookNtfy(WebhookConfig{URL: server.URL, Service: "ntfy"}, nil, time.Now())
	}
	q := open()
	q.SetProcessor(processor)
	if err := q.Enqueue(&QueuedNotification{ID: "rejected", Type: "webhook_resolved", Config: []byte(`{}`), MaxAttempts: 3}); err != nil {
		t.Fatal(err)
	}
	waitVerdictAudit(t, q, "rejected")
	if err := q.Stop(); err != nil {
		t.Fatal(err)
	}
	q = open()
	var status, class string
	var attempts int
	if err := q.db.QueryRow(`SELECT status, attempts, failure_class FROM notification_audit WHERE notification_id = 'rejected'`).Scan(&status, &attempts, &class); err != nil {
		t.Fatal(err)
	}
	stats, err := q.GetTelemetryStats(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if status != string(QueueStatusDLQ) || attempts != 1 || class != string(NotificationFailureAuthentication) ||
		stats.Attempts != 1 || stats.Failures != 1 || stats.Deliveries != 0 || stats.FailureClasses.Authentication != 1 || requests.Load() != 1 {
		t.Fatalf("rejection lost: audit=%s/%d/%s stats=%+v HTTP requests=%d", status, attempts, class, stats, requests.Load())
	}
	// Only an explicit operator retry after the destination is repaired resumes
	// delivery. It retains the original terminal failure and the successful send.
	repaired.Store(true)
	if count, err := q.RetryTerminalFailures(); err != nil || count != 1 {
		t.Fatalf("operator retry = %d / %v; want one retained failure", count, err)
	}
	q.SetProcessor(processor)
	deadline := time.Now().Add(5 * time.Second)
	for {
		stats, err = q.GetTelemetryStats(time.Now().Add(-time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		if stats.Deliveries == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("operator retry did not deliver")
		}
		time.Sleep(time.Millisecond)
	}
	if stats.Attempts != 2 || stats.Failures != 1 || stats.FailureClasses.Authentication != 1 || requests.Load() != 2 {
		t.Errorf("operator retry erased history or duplicated delivery: stats=%+v HTTP requests=%d", stats, requests.Load())
	}
}
