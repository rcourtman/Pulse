package notifications

// Built-in service bodies carry their template's headers on every delivery
// path (#2540). A Telegram webhook whose stored Content-Type had been blanked
// passed the Test button, which always applied the template headers, while
// every real alert went out without a JSON content type. Telegram ignores
// such a body and rejects the request with "message text is empty".

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

type capturedWebhookRequest struct {
	headers http.Header
	body    []byte
}

func newWebhookCaptureServer(t *testing.T) (string, func() []capturedWebhookRequest) {
	t.Helper()
	var (
		mu       sync.Mutex
		requests []capturedWebhookRequest
	)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		requests = append(requests, capturedWebhookRequest{headers: r.Header.Clone(), body: body})
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server.URL, func() []capturedWebhookRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]capturedWebhookRequest(nil), requests...)
	}
}

// Stored header states an edit can leave behind for the template's
// Content-Type: a cleared value, the API's masked value saved as typed, a
// replacement, and the same name in other spellings.
var damagedContentTypeHeaders = map[string]map[string]string{
	"blank":     {"Content-Type": ""},
	"masked":    {"Content-Type": "***REDACTED***"},
	"replaced":  {"Content-Type": "text/plain"},
	"lowercase": {"content-type": ""},
	"duplicate": {"Content-Type": "application/json", "content-type": "text/plain"},
}

func TestRealTelegramAlertWithDamagedContentTypeStillSendsText(t *testing.T) {
	for name, headers := range damagedContentTypeHeaders {
		t.Run(name, func(t *testing.T) {
			url, captured := newWebhookCaptureServer(t)
			m := NewNotificationManagerWithDataDir("", t.TempDir())
			t.Cleanup(m.Stop)
			if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			m.SetGroupingConfig(true, 1, true, false)
			m.AddWebhook(WebhookConfig{
				ID:      "telegram",
				Name:    "Telegram Bot",
				URL:     url + "/bot123:token/sendMessage?chat_id=-1001234",
				Method:  "POST",
				Enabled: true,
				Service: "telegram",
				Headers: headers,
			})

			m.SendAlert(&alerts.Alert{
				ID:           "pve-cluster-synology-backups::metric-threshold:usage",
				Type:         "usage",
				Level:        alerts.AlertLevelWarning,
				ResourceID:   "pve-cluster-synology-backups",
				ResourceName: "synology-backups",
				Node:         "pve",
				Message:      "Storage usage at 91%",
				Value:        91.2,
				Threshold:    90,
				StartTime:    time.Now().Add(-time.Minute),
				LastSeen:     time.Now(),
			})

			deadline := time.Now().Add(15 * time.Second)
			for len(captured()) == 0 && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			requests := captured()
			if len(requests) != 1 {
				t.Fatalf("Telegram received %d requests, want 1", len(requests))
			}
			got := requests[0]
			if contentType := got.headers.Values("Content-Type"); len(contentType) != 1 || contentType[0] != "application/json" {
				t.Fatalf("Content-Type = %q, want [application/json]", contentType)
			}
			var payload map[string]any
			if err := json.Unmarshal(got.body, &payload); err != nil {
				t.Fatalf("body is not JSON: %v\n%s", err, got.body)
			}
			text, _ := payload["text"].(string)
			if !strings.Contains(text, "synology-backups") || payload["chat_id"] != "-1001234" {
				t.Fatalf("Telegram payload lacks the alert text or chat: %s", got.body)
			}
		})
	}
}

// Test and real deliveries must agree on headers for every built-in service,
// so a passing Test predicts real delivery.
func TestBuiltInServiceDeliveriesSendTemplateHeadersLikeTest(t *testing.T) {
	for _, tmpl := range GetWebhookTemplates() {
		if tmpl.Service == "generic" {
			continue
		}
		for name, headers := range damagedContentTypeHeaders {
			t.Run(tmpl.Service+"/"+name, func(t *testing.T) {
				url, captured := newWebhookCaptureServer(t)
				m := NewNotificationManager("")
				t.Cleanup(m.Stop)
				if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				webhook := WebhookConfig{
					Name:    "contract-" + tmpl.Service,
					URL:     url + "/hook?chat_id=1234",
					Enabled: true,
					Service: tmpl.Service,
					Headers: map[string]string{"Authorization": "Bearer stored"},
				}
				for key, value := range headers {
					webhook.Headers[key] = value
				}
				if tmpl.Service == "pagerduty" {
					webhook.Headers["routing_key"] = "routing-key"
				}
				batch := contractGroupedAlerts()

				if err := m.SendTestWebhook(webhook); err != nil {
					t.Fatalf("test send: %v", err)
				}
				m.webhookClient = nil // real sends below use the delivery client
				if err := m.sendGroupedWebhook(webhook, batch); err != nil {
					t.Fatalf("grouped send: %v", err)
				}
				resolvedSends := 0
				if tmpl.Service != "ntfy" { // ntfy recoveries own their plain-text request
					if err := m.sendResolvedWebhook(webhook, batch, time.Now()); err != nil {
						t.Fatalf("resolved send: %v", err)
					}
					resolvedSends = 1
				}

				requests := captured()
				if len(requests) != 2+resolvedSends {
					t.Fatalf("captured %d requests, want %d", len(requests), 2+resolvedSends)
				}
				for i, req := range requests {
					for key, want := range tmpl.Headers {
						if got := req.headers.Values(key); len(got) != 1 || got[0] != want {
							t.Errorf("request %d %s = %q, want [%s]", i, key, got, want)
						}
					}
					if got := req.headers.Get("Authorization"); got != "Bearer stored" {
						t.Errorf("request %d dropped an unrelated stored header: Authorization = %q", i, got)
					}
				}
			})
		}
	}
}

// A custom template means the user owns the body, so their headers stand.
func TestCustomTemplateDeliveryKeepsStoredContentType(t *testing.T) {
	url, captured := newWebhookCaptureServer(t)
	m := NewNotificationManager("")
	t.Cleanup(m.Stop)
	if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	m.webhookClient = nil
	webhook := WebhookConfig{
		Name:     "custom",
		URL:      url + "/hook?chat_id=1234",
		Enabled:  true,
		Service:  "telegram",
		Template: `{"text": "{{.ResourceName | jsonString}}"}`,
		Headers:  map[string]string{"Content-Type": "application/vnd.custom+json"},
	}
	if err := m.sendGroupedWebhook(webhook, contractGroupedAlerts()); err != nil {
		t.Fatalf("grouped send: %v", err)
	}
	requests := captured()
	if len(requests) != 1 {
		t.Fatalf("captured %d requests, want 1", len(requests))
	}
	if got := requests[0].headers.Get("Content-Type"); got != "application/vnd.custom+json" {
		t.Fatalf("Content-Type = %q, want the stored custom value", got)
	}
}
