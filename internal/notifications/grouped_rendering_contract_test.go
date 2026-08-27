package notifications

// The grouped-rendering half of the lifecycle contract suite
// (docs/ALERT_ENGINE_EVOLUTION.md): a batch of N grouped alerts must name
// every alert on every delivery surface — the #1683 class, where built-in
// service payloads captured the primary alert's message before grouping
// enriched it and silently dropped N−1 of N alerts. The engine-side
// contracts live in internal/alerts/lifecycle_contract_test.go.

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// contractGroupedAlerts builds a batch whose resource names are distinctive
// enough that "does the payload name this alert" cannot false-positive.
func contractGroupedAlerts() []*alerts.Alert {
	names := []string{
		"contract-guest-alpha",
		"contract-guest-bravo",
		"contract-guest-charlie",
		"contract-guest-delta",
		"contract-guest-echo",
	}
	now := time.Now()
	batch := make([]*alerts.Alert, 0, len(names))
	for i, name := range names {
		level := alerts.AlertLevelWarning
		if i == 0 {
			level = alerts.AlertLevelCritical
		}
		batch = append(batch, &alerts.Alert{
			ID:           "contract-" + name,
			Type:         "cpu",
			Level:        level,
			ResourceID:   name,
			ResourceName: name,
			Node:         "node-1",
			Instance:     "inst-1",
			Message:      name + " cpu above threshold",
			Value:        90 + float64(i),
			Threshold:    80,
			StartTime:    now.Add(-time.Duration(i+1) * time.Minute),
		})
	}
	return batch
}

func assertEveryAlertNamed(t *testing.T, surface, rendered string, batch []*alerts.Alert) {
	t.Helper()
	var missing []string
	for _, alert := range batch {
		if !strings.Contains(rendered, alert.ResourceName) {
			missing = append(missing, alert.ResourceName)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s rendered %d of %d grouped alerts; missing %v\nrendered: %.2000s",
			surface, len(batch)-len(missing), len(batch), missing, rendered)
	}
}

func TestContractGroupedEmailRendersEveryAlert(t *testing.T) {
	batch := contractGroupedAlerts()
	subject, htmlBody, textBody := groupedAlertTemplate(batch)
	if subject == "" {
		t.Fatal("grouped email rendered an empty subject")
	}
	assertEveryAlertNamed(t, "grouped email (html)", htmlBody, batch)
	assertEveryAlertNamed(t, "grouped email (text)", textBody, batch)
}

func TestContractGroupedAppriseRendersEveryAlert(t *testing.T) {
	batch := contractGroupedAlerts()
	title, body, _ := buildApprisePayload(batch, "")
	if title == "" && body == "" {
		t.Fatal("grouped apprise payload rendered empty")
	}
	assertEveryAlertNamed(t, "grouped apprise", title+"\n"+body, batch)
}

// TestContractGroupedWebhookRendersEveryAlertPerService drives the real
// grouped send path for every built-in service template plus the generic
// fallback and a custom template, capturing what would go over the wire.
func TestContractGroupedWebhookRendersEveryAlertPerService(t *testing.T) {
	services := []string{
		"", // generic fallback payload (full alert array)
		"generic",
		"discord",
		"slack",
		"telegram",
		"teams",
		"teams-adaptive",
		"pagerduty",
		"pushover",
		"gotify",
		"ntfy",
		"mattermost",
	}

	for _, service := range services {
		label := service
		if label == "" {
			label = "fallback"
		}
		t.Run(label, func(t *testing.T) {
			batch := contractGroupedAlerts()

			var gotBody []byte
			var gotHeaders http.Header
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				gotBody = body
				gotHeaders = r.Header.Clone()
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			manager := NewNotificationManager("")
			defer manager.Stop()
			manager.webhookClient = server.Client()
			if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatalf("allowlist: %v", err)
			}

			url := server.URL + "/hook"
			if service == "telegram" {
				// Telegram webhook URLs carry the chat as a query
				// parameter; the send path extracts it.
				url += "?chat_id=1234"
			}
			webhook := WebhookConfig{
				Name:    "contract-" + label,
				URL:     url,
				Service: service,
				Enabled: true,
			}

			if err := manager.sendGroupedWebhook(webhook, batch); err != nil {
				t.Fatalf("sendGroupedWebhook(%s): %v", label, err)
			}

			rendered := string(gotBody)
			if service == "ntfy" {
				// ntfy carries the title/tags in headers and the body in
				// plain text; both belong to the rendered surface.
				for _, values := range gotHeaders {
					rendered += "\n" + strings.Join(values, "\n")
				}
			}
			assertEveryAlertNamed(t, "grouped webhook ("+label+")", rendered, batch)
		})
	}
}

func TestContractGroupedCustomTemplateRendersEveryAlert(t *testing.T) {
	batch := contractGroupedAlerts()

	var gotBody []byte
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.webhookClient = server.Client()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatalf("allowlist: %v", err)
	}

	webhook := WebhookConfig{
		Name:     "contract-custom",
		URL:      server.URL + "/hook",
		Enabled:  true,
		Template: `{"text": "{{.Message}}"}`,
	}

	if err := manager.sendGroupedWebhook(webhook, batch); err != nil {
		t.Fatalf("sendGroupedWebhook(custom template): %v", err)
	}
	assertEveryAlertNamed(t, "grouped webhook (custom template)", string(gotBody), batch)
}

// TestContractCancelBeforeDeliverySuppressesRecovery pins the
// notifications-side half of "a suppressed firing must not produce a
// recovery": cancelling an alert still inside the grouping window reports
// that the firing was never delivered, and the monitor uses that answer to
// hold the recovery notification (#1553 class).
func TestContractCancelBeforeDeliverySuppressesRecovery(t *testing.T) {
	manager := NewNotificationManager("")
	defer manager.Stop()
	manager.SetGroupingConfig(true, 60, false, false)

	batch := contractGroupedAlerts()
	pending := batch[0]
	manager.SendAlert(pending)

	if never := manager.CancelAlert(pending.ID); !never {
		t.Fatal("cancelling an alert still inside the grouping window must report the firing as never delivered")
	}

	// A firing that was actually delivered must not be reported as
	// undelivered when the alert later resolves.
	delivered := batch[1]
	manager.mu.Lock()
	manager.lastNotified[delivered.ID] = notificationRecord{lastSent: time.Now()}
	manager.mu.Unlock()
	if never := manager.CancelAlert(delivered.ID); never {
		t.Fatal("a delivered firing must not be reported as never-delivered on resolve")
	}
}
