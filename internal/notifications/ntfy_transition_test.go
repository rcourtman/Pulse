package notifications

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Exercise the shared destination across transitions: generated firing headers
// must not leak into the stored configuration and override recovery metadata.
func TestNtfySeverityRecoveryTransition(t *testing.T) {
	type receipt struct {
		header http.Header
		body   string
	}
	receipts := make(chan receipt, 4)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		receipts <- receipt{r.Header.Clone(), string(body)}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	webhook := WebhookConfig{Name: "transition", URL: server.URL + "/topic", Enabled: true, Service: "ntfy",
		Headers: map[string]string{"X-Static": "preserved"}}
	originalHeaders := map[string]string{"X-Static": "preserved"}
	alert := &alerts.Alert{ID: "transition", Type: "cpu", ResourceID: "vm-1", ResourceName: "database", Node: "node-a",
		Message: "CPU above threshold", Value: 99, Threshold: 90, StartTime: time.Now().Add(-time.Minute)}
	for _, step := range []struct {
		name                  string
		level                 alerts.AlertLevel
		resolved              bool
		priority, title, tags string
	}{
		{"warning", alerts.AlertLevelWarning, false, "high", "WARNING: database on node-a", "warning,pulse,cpu"},
		{"critical", alerts.AlertLevelCritical, false, "urgent", "CRITICAL: database on node-a", "rotating_light,pulse,cpu"},
		{"resolved", alerts.AlertLevelCritical, true, "default", "RESOLVED: database", "white_check_mark,pulse,resolved"},
		{"warning again", alerts.AlertLevelWarning, false, "high", "WARNING: database on node-a", "warning,pulse,cpu"},
	} {
		t.Run(step.name, func(t *testing.T) {
			alert.Level = step.level
			before := *alert
			var err error
			if step.resolved {
				err = manager.sendResolvedWebhook(webhook, []*alerts.Alert{alert}, time.Now())
			} else {
				err = manager.sendGroupedWebhook(webhook, []*alerts.Alert{alert})
			}
			if err != nil {
				t.Fatal(err)
			}
			var got receipt
			select {
			case got = <-receipts:
			case <-time.After(time.Second):
				t.Fatal("successful send produced no HTTP receipt")
			}
			for key, want := range map[string]string{"Priority": step.priority, "Title": step.title, "Tags": step.tags, "X-Static": "preserved", "Content-Type": "text/plain"} {
				if got.header.Get(key) != want {
					t.Errorf("%s = %q, want %q", key, got.header.Get(key), want)
				}
			}
			if step.resolved {
				if !strings.Contains(got.body, "is now healthy") || strings.Contains(got.body, alert.Message) {
					t.Errorf("unexpected recovery body: %q", got.body)
				}
			} else if !strings.Contains(got.body, alert.Message) {
				t.Errorf("missing trigger context: %q", got.body)
			}
			if !reflect.DeepEqual(webhook.Headers, originalHeaders) {
				t.Error("destination headers mutated")
			}
			if !reflect.DeepEqual(*alert, before) {
				t.Error("source alert mutated")
			}
		})
	}
}
