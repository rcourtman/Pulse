package notifications

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestWebhookHeaderValue(t *testing.T) {
	for _, tc := range []struct{ name, value, want string }{
		{"ordinary id", "docker:edge/4f0cd2a9b1c30::metric-threshold:memory:alert", "docker:edge/4f0cd2a9b1c30::metric-threshold:memory:alert"},
		{"non-ascii name", "WARNING: nœud-東京-🚨", "WARNING: nœud-東京-🚨"},
		{"tab and percent", "a\tb 100%", "a\tb 100%"},
		{"control bytes", "4f0c\x01d2\r\nX-Injected: 1\x00\x7f", "4f0c%01d2%0D%0AX-Injected: 1%00%7F"},
		{"empty", "", ""},
	} {
		if got := webhookHeaderValue(tc.value); got != tc.want {
			t.Errorf("%s: webhookHeaderValue(%q) = %q, want %q", tc.name, tc.value, got, tc.want)
		}
	}
	oversized := "docker:edge/" + strings.Repeat("a", 70_000) + ":alert"
	got := webhookHeaderValue(oversized)
	if len(got) > maxWebhookHeaderValueBytes {
		t.Errorf("oversized webhookHeaderValue length = %d, want <= %d", len(got), maxWebhookHeaderValueBytes)
	}
	if got != webhookHeaderValue(oversized) {
		t.Error("oversized webhookHeaderValue must be deterministic across retries")
	}
}

// A container ID or name is whatever an agent reports. One holding a control
// byte must not stop the request that also carries other hosts' alerts.
func TestWebhookDeliversAlertsWithControlBytesInHeaderValues(t *testing.T) {
	hostile := &alerts.Alert{
		ID:           "docker:edge/4f0c\x01d2a9b1c30::metric-threshold:memory",
		Type:         "memory",
		Level:        alerts.AlertLevelWarning,
		ResourceID:   "docker:edge/4f0c\x01d2a9b1c30",
		ResourceName: "cache\x01\r\nX-Injected: 1",
		Node:         "edge",
		Message:      "Memory is high",
		Value:        99,
		Threshold:    85,
		StartTime:    time.Now().Add(-time.Minute),
	}
	bystander := &alerts.Alert{
		ID:           "agent:nas02/raid:md0::agent:nas02/raid:md0-health",
		Type:         "raid",
		Level:        alerts.AlertLevelCritical,
		ResourceID:   "agent:nas02/raid:md0",
		ResourceName: "nas02 - /dev/md0 (raid1)",
		Node:         "nas02",
		Message:      "RAID array degraded",
		StartTime:    time.Now().Add(-time.Minute),
	}
	const wantID = "docker:edge/4f0c%01d2a9b1c30::metric-threshold:memory"
	const wantName = "cache%01%0D%0AX-Injected: 1"

	for _, tc := range []struct {
		name, service        string
		batch                []*alerts.Alert
		resolved             bool
		wantEvent, wantTitle string
	}{
		{name: "generic group", service: "generic", batch: []*alerts.Alert{hostile, bystander}, wantEvent: wantID + ":alert"},
		{name: "generic recovery", service: "generic", batch: []*alerts.Alert{hostile, bystander}, resolved: true, wantEvent: wantID + ":resolved"},
		{name: "ntfy single", service: "ntfy", batch: []*alerts.Alert{hostile}, wantEvent: wantID + ":alert", wantTitle: "WARNING: " + wantName + " on edge"},
		{name: "ntfy group", service: "ntfy", batch: []*alerts.Alert{hostile, bystander}, wantEvent: wantID + ":alert", wantTitle: "CRITICAL: 2 alerts"},
		{name: "ntfy recovery", service: "ntfy", batch: []*alerts.Alert{hostile}, resolved: true, wantEvent: wantID + ":resolved", wantTitle: "RESOLVED: " + wantName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []http.Header
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = append(got, r.Header.Clone())
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			manager := NewNotificationManager("")
			defer manager.Stop()
			manager.webhookClient = server.Client()
			if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatalf("allowlist: %v", err)
			}
			webhook := WebhookConfig{Name: "ops", URL: server.URL + "/hook", Enabled: true, Service: tc.service}

			var err error
			if tc.resolved {
				err = manager.sendResolvedWebhook(webhook, tc.batch, time.Now())
			} else {
				err = manager.sendGroupedWebhook(webhook, tc.batch)
			}
			if err != nil {
				t.Fatalf("delivery failed: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("receiver got %d requests, want 1", len(got))
			}
			if event := got[0].Values("X-Pulse-Event-ID"); len(event) != 1 || event[0] != tc.wantEvent {
				t.Errorf("X-Pulse-Event-ID = %q, want %q", event, tc.wantEvent)
			}
			if title := got[0].Get("Title"); title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", title, tc.wantTitle)
			}
			if len(got[0].Values("X-Injected")) != 0 {
				t.Error("alert data added a request header")
			}
		})
	}
}
