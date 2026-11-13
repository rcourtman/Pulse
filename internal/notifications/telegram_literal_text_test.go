package notifications

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

type telegramLiteralPayload struct {
	ChatID         string `json:"chat_id"`
	Text           string `json:"text"`
	ParseMode      string `json:"parse_mode"`
	DisablePreview bool   `json:"disable_web_page_preview"`
}

type telegramLiteralContent struct {
	text  string
	links []string
	bold  int
}

// Parse the HTML subset emitted by the built-in templates, independently of
// their Go escaping pipeline. This is a local wire-format oracle, not a full
// Telegram implementation or proof of acceptance by the external provider.
func parseTelegramLiteralContent(payload telegramLiteralPayload) (telegramLiteralContent, error) {
	var content telegramLiteralContent
	if payload.ParseMode != "HTML" {
		return content, fmt.Errorf("parse_mode = %q, want HTML", payload.ParseMode)
	}
	decoder := xml.NewDecoder(strings.NewReader("<root>" + payload.Text + "</root>"))
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			content.text = text.String()
			return content, nil
		}
		if err != nil {
			return content, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Space != "" {
				return content, fmt.Errorf("unexpected namespace")
			}
			switch token.Name.Local {
			case "root":
			case "b":
				content.bold++
			case "a":
				if len(token.Attr) != 1 || token.Attr[0].Name != (xml.Name{Local: "href"}) {
					return content, fmt.Errorf("unexpected link attributes")
				}
				if token.Attr[0].Value == "" {
					return content, fmt.Errorf("empty HTML link target")
				}
				content.links = append(content.links, token.Attr[0].Value)
				continue
			default:
				return content, fmt.Errorf("unexpected element %s", token.Name.Local)
			}
			if len(token.Attr) != 0 {
				return content, fmt.Errorf("unexpected attributes on %s", token.Name.Local)
			}
		case xml.CharData:
			text.Write(token)
		case xml.EndElement:
		default:
			return content, fmt.Errorf("unexpected HTML token %T", token)
		}
	}
}

func TestTelegramTemplatesKeepDynamicFieldsLiteral(t *testing.T) {
	fixture := `host_*[disk](path)` + "`" + ` <b>not markup</b> <img src="x"> & 'quoted' "double" \` + "\nsecond line 🖥"
	base := WebhookPayloadData{Level: "warning", ChatID: "-1001234", Type: "cpu", Value: 91.2, Threshold: 80,
		Message: "ordinary message", ResourceName: "ordinary host", Node: "pve1", Duration: "1m",
		ResolvedAt: "2026-11-13T02:00:00Z", AlertURL: "https://pulse.example.test/alerts/history?alert=a&view=b"}
	template, ok := builtInWebhookServiceTemplate("telegram")
	require.True(t, ok)
	for mode, source := range map[string]string{"firing": template.PayloadTemplate, "resolved": template.ResolvedPayloadTemplate} {
		for _, field := range []string{"Message", "ResourceName", "Node", "Type", "Duration", "ResolvedAt", "AlertURL"} {
			if mode == "firing" && field == "ResolvedAt" || mode == "resolved" && field == "AlertURL" {
				continue
			}
			t.Run(mode+"/"+field, func(t *testing.T) {
				data := base
				wantLiteral := fixture
				if field == "Type" {
					// The existing title helper lowercases types on every service.
					wantLiteral = "Host" + strings.ToLower(fixture[4:])
				}
				if field == "AlertURL" {
					wantLiteral = `https://pulse.example.test/alerts?alert=a&name="quoted"&tag=<b>&path=one_two`
				}
				switch field {
				case "Message":
					data.Message = fixture
				case "ResourceName":
					data.ResourceName = fixture
				case "Node":
					data.Node = fixture
				case "Type":
					data.Type = fixture
				case "Duration":
					data.Duration = fixture
				case "ResolvedAt":
					data.ResolvedAt = fixture
				case "AlertURL":
					data.AlertURL = wantLiteral
				}
				body, err := (&NotificationManager{}).generatePayloadFromTemplateWithService(source, data, "telegram")
				require.NoError(t, err)
				var payload telegramLiteralPayload
				require.NoError(t, json.Unmarshal(body, &payload))
				require.Equal(t, data.ChatID, payload.ChatID, "chat routing is not HTML text")
				require.True(t, payload.DisablePreview)
				content, err := parseTelegramLiteralContent(payload)
				require.NoError(t, err)
				require.Equal(t, 2, content.bold, "only the fixed heading and Details may be markup")
				if mode == "firing" {
					require.Equal(t, []string{data.AlertURL}, content.links, "HTML attribute decoding must preserve the exact Pulse link")
					require.Contains(t, content.text, "Pulse Alert: Warning\n\n")
					require.Contains(t, content.text, "• Value: 91.2%\n• Threshold: 80%")
				} else {
					require.Empty(t, content.links)
					require.Contains(t, content.text, "Resolved: "+data.ResourceName+"\n\n")
				}
				if field != "AlertURL" {
					require.Contains(t, content.text, wantLiteral, "dynamic text must survive HTML and JSON decoding unchanged")
				}
			})
		}
	}
}

func telegramReportedAlerts() []*alerts.Alert {
	start := time.Now().Add(-5 * time.Minute)
	return []*alerts.Alert{
		{ID: "pve-connection", ResourceID: "pve:pve_one", ResourceName: "pve_one", Node: "pve_one", Type: "connection-degraded",
			Level: alerts.AlertLevelWarning, StartTime: start,
			Message: `Connection 'pve_one' is unreachable: poll_nodes failed on pve_one: Get "https://pve_one:8006/api2/json/nodes": context deadline exceeded`},
		{ID: "pbs-connection", ResourceID: "pbs:pbs_two", ResourceName: "pbs_two", Node: "pbs_two", Type: "connection-degraded",
			Level: alerts.AlertLevelWarning, StartTime: start, Message: "get_pbs_version failed <timeout> & retry"},
		{ID: "guest-one", ResourceID: "vm:101", ResourceName: "guest_[one]", Node: "pve_one", Type: "powered-off",
			Level: alerts.AlertLevelWarning, StartTime: start, Message: "guest_[one] powered off *unexpectedly"},
		{ID: "guest-two", ResourceID: "vm:102", ResourceName: "guest_`two", Node: "pve_one", Type: "powered-off",
			Level: alerts.AlertLevelWarning, StartTime: start, Message: `guest_` + "`" + `two powered off <b>not markup</b>`},
	}
}

func TestTelegramDeliveryPathsKeepReportedTextLiteral(t *testing.T) {
	for _, path := range []string{"single", "grouped", "resolved", "grouped-resolved", "enhanced", "test-button"} {
		t.Run(path, func(t *testing.T) {
			url, captured := newWebhookCaptureServer(t)
			m := NewNotificationManagerWithDeferredQueue("https://pulse.example.test", t.TempDir())
			t.Cleanup(m.Stop)
			require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			webhook := WebhookConfig{ID: "telegram", Name: "Synthetic Telegram", Service: "telegram", Enabled: true,
				URL: url + "/bot123:synthetic/sendMessage?chat_id=-1001234"}
			batch := telegramReportedAlerts()
			original := batch[0].Clone()
			var err error
			switch path {
			case "single":
				err = m.sendGroupedWebhook(webhook, batch[:1])
			case "grouped":
				err = m.sendGroupedWebhook(webhook, batch)
			case "resolved":
				err = m.sendResolvedWebhook(webhook, batch[:1], time.Now())
			case "grouped-resolved":
				err = m.sendResolvedWebhook(webhook, batch, time.Now())
			case "enhanced":
				err = m.SendEnhancedWebhook(BuildEnhancedWebhookTestConfig(webhook, "telegram"), batch[0])
			case "test-button":
				err = m.SendTestWebhook(webhook)
			}
			require.NoError(t, err)
			require.Equal(t, original, batch[0], "group formatting must not rewrite the alert")
			requests := captured()
			require.Len(t, requests, 1, "format repair must not introduce retries or extra sends")
			require.Equal(t, "application/json", requests[0].headers.Get("Content-Type"))
			var payload telegramLiteralPayload
			require.NoError(t, json.Unmarshal(requests[0].body, &payload))
			require.Equal(t, "-1001234", payload.ChatID)
			content, err := parseTelegramLiteralContent(payload)
			require.NoError(t, err)
			require.Equal(t, 2, content.bold)
			if path != "test-button" {
				require.Contains(t, content.text, batch[0].ResourceName)
			}
			if path == "single" || path == "grouped" || path == "enhanced" {
				require.Contains(t, content.text, batch[0].Message)
			}
			if path == "grouped" || path == "grouped-resolved" {
				for _, alert := range batch {
					require.Contains(t, content.text, alert.ResourceName)
					if path == "grouped" {
						require.Contains(t, content.text, alert.Message)
					}
				}
			}
		})
	}
}

func TestTelegramGroupedLiteralTextSettlesWithoutDeadLetter(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadRequest} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := make(chan telegramLiteralPayload, 2)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload telegramLiteralPayload
				err := json.NewDecoder(r.Body).Decode(&payload)
				if err == nil {
					_, err = parseTelegramLiteralContent(payload)
				}
				requests <- payload
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				w.WriteHeader(status)
			}))
			t.Cleanup(server.Close)
			dir := t.TempDir()
			m := NewNotificationManagerWithDeferredQueue("https://pulse.example.test", dir)
			t.Cleanup(m.Stop)
			require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			m.AddWebhook(WebhookConfig{ID: "telegram", Name: "Synthetic Telegram", Service: "telegram", Enabled: true,
				URL: server.URL + "/bot123:synthetic/sendMessage?chat_id=-1001234"})
			m.SetGroupingConfig(true, 60, false, false)
			batch := telegramReportedAlerts()
			for _, alert := range batch {
				m.SendAlert(alert)
			}
			m.sendGroupedAlerts()
			pending, err := m.queue.GetPending(10)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			require.Len(t, pending[0].Alerts, len(batch))
			m.queue.processNotification(pending[0])
			m.Stop()

			require.Len(t, requests, 1, "a rejected provider request is not split or replayed")
			payload := <-requests
			content, err := parseTelegramLiteralContent(payload)
			require.NoError(t, err)
			for _, alert := range batch {
				require.Contains(t, content.text, alert.Message)
			}
			reopened := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(reopened.Stop)
			row := loadQuietReplay(t, reopened.queue, pending[0].ID)
			require.Equal(t, 1, row.Attempts)
			var auditCount, successes int
			require.NoError(t, reopened.queue.db.QueryRow(`SELECT count(*), coalesce(sum(success),0) FROM notification_audit WHERE notification_id = ?`, row.ID).Scan(&auditCount, &successes))
			require.Equal(t, 1, auditCount)
			if status == http.StatusOK {
				require.Equal(t, QueueStatusSent, row.Status)
				require.Equal(t, 1, successes)
				require.True(t, reopened.DeliveryHealth().Healthy)
			} else {
				require.Equal(t, QueueStatusDLQ, row.Status, "real provider rejection remains terminal, not false success")
				require.Zero(t, successes)
				require.False(t, reopened.DeliveryHealth().Healthy)
			}
		})
	}
}

func TestTelegramCustomTemplateStillOwnsParseMode(t *testing.T) {
	url, captured := newWebhookCaptureServer(t)
	m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
	t.Cleanup(m.Stop)
	require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	webhook := WebhookConfig{Service: "telegram", Enabled: true, URL: url + "/sendMessage?chat_id=-1001234",
		Template: `{"chat_id":"{{.ChatID | jsonString}}","text":"{{.Message | jsonString}}","parse_mode":"MarkdownV2"}`}
	alert := telegramReportedAlerts()[0]
	require.NoError(t, m.sendGroupedWebhook(webhook, []*alerts.Alert{alert}))
	requests := captured()
	require.Len(t, requests, 1)
	var payload telegramLiteralPayload
	require.NoError(t, json.Unmarshal(requests[0].body, &payload))
	require.Equal(t, "MarkdownV2", payload.ParseMode)
	require.Equal(t, alert.Message, payload.Text, "built-in escaping must not rewrite a caller-owned template")
}

// An unset public URL is supported. HTML must not turn the existing dashboard
// pointer into a malformed empty link; a full instance URL still supplies it.
func TestTelegramFiringWithoutPublicURLKeepsDashboardPointer(t *testing.T) {
	for _, instance := range []string{"", "https://pve.example.test:8006"} {
		name := "unset"
		if instance != "" {
			name = "instance-fallback"
		}
		t.Run(name, func(t *testing.T) {
			url, captured := newWebhookCaptureServer(t)
			m := NewNotificationManagerWithDeferredQueue("", t.TempDir())
			t.Cleanup(m.Stop)
			require.NoError(t, m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			webhook := WebhookConfig{Service: "telegram", Enabled: true, URL: url + "/sendMessage?chat_id=-1001234"}
			alert := telegramReportedAlerts()[0]
			alert.Instance = instance
			require.NoError(t, m.sendGroupedWebhook(webhook, []*alerts.Alert{alert}))
			requests := captured()
			require.Len(t, requests, 1)
			var payload telegramLiteralPayload
			require.NoError(t, json.Unmarshal(requests[0].body, &payload))
			content, err := parseTelegramLiteralContent(payload)
			require.NoError(t, err)
			require.Contains(t, content.text, alert.Message)
			require.Contains(t, content.text, "View in Pulse")
			if instance == "" {
				require.Empty(t, content.links)
			} else {
				require.Equal(t, []string{instance}, content.links)
			}
		})
	}
}
