package notifications

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

// Exercise the copyable templates through the real sender, not a second
// renderer or a model receiver. The input override permits an exact-parent
// documentation negative control without modifying production source.
func webhookGuide(t *testing.T) string {
	t.Helper()
	path := os.Getenv("PULSE_WEBHOOK_GUIDE_TEST_INPUT")
	if path == "" {
		path = "../../docs/WEBHOOKS.md"
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func webhookGuidePSATemplate(t *testing.T) string {
	t.Helper()
	sections := strings.SplitN(webhookGuide(t), "### Sample PSA payloads", 2)
	require.Len(t, sections, 2)
	block := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindStringSubmatch(sections[1])
	require.Len(t, block, 2)
	return block[1]
}

type guideMember struct {
	ID         string `json:"alertId"`
	StartedAt  string `json:"startedAt"`
	Severity   string `json:"severity"`
	ResourceID string `json:"resourceId"`
	Resource   string `json:"resource"`
	Summary    string `json:"summary"`
}

type guidePayload struct {
	Event      string        `json:"event"`
	TenantID   string        `json:"tenantId"`
	StartedAt  string        `json:"startedAt"`
	AlertCount int           `json:"alertCount"`
	Members    []guideMember `json:"alerts"`
}

type guideRequest struct {
	body, eventID, timestamp, signature string
}

func webhookGuideSender(t *testing.T, payloadTemplate string) (*NotificationManager, WebhookConfig, <-chan guideRequest) {
	t.Helper()
	manager := NewNotificationManager("https://pulse.example.test")
	require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.1"))
	manager.SetTenantIdentityResolver(func() (string, string) { return "synthetic-tenant", "Synthetic tenant" })
	requests := make(chan guideRequest, 16)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests <- guideRequest{string(body), r.Header.Get("X-Pulse-Event-ID"),
			r.Header.Get("X-Pulse-Timestamp"), r.Header.Get("X-Pulse-Signature")}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	webhook := WebhookConfig{ID: "synthetic-guide", Name: "Synthetic guide receiver", URL: server.URL,
		Enabled: true, Service: "generic", SigningSecret: "synthetic-guide-test-secret", Template: payloadTemplate}
	return manager, webhook, requests
}

func webhookGuideRead(t *testing.T, requests <-chan guideRequest) (guideRequest, guidePayload) {
	t.Helper()
	select {
	case request := <-requests:
		var payload guidePayload
		require.NoError(t, json.Unmarshal([]byte(request.body), &payload), "copied template must render valid JSON")
		mac := hmac.New(sha256.New, []byte("synthetic-guide-test-secret"))
		_, err := mac.Write([]byte(request.timestamp + "." + request.body))
		require.NoError(t, err)
		require.Equal(t, "v1="+hex.EncodeToString(mac.Sum(nil)), request.signature)
		return request, payload
	case <-time.After(5 * time.Second):
		t.Fatal("no local sender request received")
		return guideRequest{}, guidePayload{}
	}
}

func TestWebhookGuidePSALifecycle(t *testing.T) {
	manager, webhook, requests := webhookGuideSender(t, webhookGuidePSATemplate(t))
	start := time.Date(2026, 10, 4, 7, 0, 0, 123456789, time.UTC)
	alert := &alerts.Alert{ID: "host::metric-threshold:cpu", StartTime: start,
		Level: alerts.AlertLevelWarning, Type: "cpu", ResourceID: "synthetic-host",
		ResourceName: "quoted \"host\"\nwith é", Message: "synthetic \\ message\n\"quoted\"", Value: 95, Threshold: 90}

	require.NoError(t, manager.sendGroupedWebhook(webhook, []*alerts.Alert{alert}))
	firstRequest, first := webhookGuideRead(t, requests)
	require.Len(t, first.Members, 1)
	require.Equal(t, "alert", first.Event)
	require.Equal(t, "synthetic-tenant", first.TenantID)
	require.Equal(t, 1, first.AlertCount)
	require.Equal(t, guideMember{alert.ID, start.Format(time.RFC3339Nano), "warning",
		alert.ResourceID, alert.ResourceName, alert.Message}, first.Members[0])

	// A synthetic retry retains the member identity even when rendering anew.
	require.NoError(t, manager.sendGroupedWebhook(webhook, []*alerts.Alert{alert}))
	retryRequest, retry := webhookGuideRead(t, requests)
	require.Equal(t, first.Members, retry.Members)
	require.Equal(t, firstRequest.eventID, retryRequest.eventID)

	critical := alert.Clone()
	critical.Level = alerts.AlertLevelCritical
	require.NoError(t, manager.sendGroupedWebhook(webhook, []*alerts.Alert{critical}))
	criticalRequest, escalated := webhookGuideRead(t, requests)
	require.Equal(t, firstRequest.eventID, criticalRequest.eventID, "header cannot identify a severity change")
	require.Equal(t, first.Members[0].StartedAt, escalated.Members[0].StartedAt)
	require.Equal(t, "critical", escalated.Members[0].Severity)

	require.NoError(t, manager.sendResolvedWebhook(webhook, []*alerts.Alert{critical}, start.Add(time.Minute)))
	_, resolved := webhookGuideRead(t, requests)
	require.Equal(t, "resolved", resolved.Event)
	require.Equal(t, escalated.Members, resolved.Members, "recovery must correlate to the original occurrence")

	// A second occurrence in the SAME SECOND defeats both header-only and
	// whole-second primary-start deduplication. Member precision preserves it.
	recurrent := critical.Clone()
	recurrent.StartTime = start.Add(100 * time.Nanosecond)
	require.NoError(t, manager.sendGroupedWebhook(webhook, []*alerts.Alert{recurrent}))
	recurrentRequest, recurrence := webhookGuideRead(t, requests)
	require.Equal(t, firstRequest.eventID, recurrentRequest.eventID)
	require.Equal(t, first.StartedAt, recurrence.StartedAt)
	require.NotEqual(t, first.Members[0].StartedAt, recurrence.Members[0].StartedAt)
	require.Equal(t, recurrent.StartTime.Format(time.RFC3339Nano), recurrence.Members[0].StartedAt)
}

func TestWebhookGuidePSAGroupMembershipAndInfo(t *testing.T) {
	manager, webhook, requests := webhookGuideSender(t, webhookGuidePSATemplate(t))
	start := time.Date(2026, 10, 4, 9, 0, 0, 987654321, time.FixedZone("synthetic", 2*60*60))
	primary := &alerts.Alert{ID: "primary", StartTime: start, Level: alerts.AlertLevelWarning}
	info := &alerts.Alert{ID: "info-member", StartTime: start.Add(time.Second), Level: alerts.AlertLevelInfo}
	critical := &alerts.Alert{ID: "critical-member", StartTime: start.Add(2 * time.Second), Level: alerts.AlertLevelCritical}
	for _, members := range [][]*alerts.Alert{{primary, info}, {primary, critical}, {primary, critical, info}} {
		require.NoError(t, manager.sendGroupedWebhook(webhook, members))
		request, payload := webhookGuideRead(t, requests)
		require.Equal(t, "primary:alert", request.eventID, "changed groups can have the same correlation header")
		require.Equal(t, len(members), payload.AlertCount)
		require.Len(t, payload.Members, len(members))
		for index, alert := range members {
			require.Equal(t, alert.ID, payload.Members[index].ID)
			require.Equal(t, string(alert.Level), payload.Members[index].Severity)
			require.Equal(t, alert.StartTime.UTC().Format(time.RFC3339Nano), payload.Members[index].StartedAt)
		}
	}
	require.NoError(t, manager.sendResolvedWebhook(webhook, []*alerts.Alert{primary, critical, info}, start.Add(time.Minute)))
	_, resolved := webhookGuideRead(t, requests)
	require.Equal(t, "resolved", resolved.Event)
	require.Len(t, resolved.Members, 3)
	require.Equal(t, "info", resolved.Members[2].Severity)
}

func TestWebhookGuideShortTemplateEscapesStrings(t *testing.T) {
	section := strings.SplitN(webhookGuide(t), "### Sample PSA payloads", 2)[0]
	block := regexp.MustCompile("(?s)```http\\nPOST /api/notifications/webhooks\\nX-Pulse-Org-ID:.*?\\n\\n(.*?)\\n```").FindStringSubmatch(section)
	require.Len(t, block, 2)
	var config struct {
		Template string `json:"template"`
	}
	require.NoError(t, json.Unmarshal([]byte(block[1]), &config))
	manager, webhook, requests := webhookGuideSender(t, config.Template)
	alert := &alerts.Alert{ID: "quoted\"id", ResourceName: "host\"\\\nname", Message: "message\"\\\n",
		StartTime: time.Date(2026, 10, 4, 7, 0, 0, 123, time.UTC), Level: alerts.AlertLevelWarning}
	require.NoError(t, manager.sendGroupedWebhook(webhook, []*alerts.Alert{alert}))
	request, _ := webhookGuideRead(t, requests)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(request.body), &payload))
	require.Equal(t, alert.ID, payload["alertId"])
	require.Equal(t, "warning: "+alert.ResourceName+" "+alert.Message, payload["summary"])
	require.Equal(t, alert.StartTime.Format(time.RFC3339), payload["startedAt"])
}

func webhookGuideExamplePayload(t *testing.T) string {
	t.Helper()
	sections := strings.SplitN(webhookGuide(t), "**Example Payload:**", 2)
	require.Len(t, sections, 2)
	block := regexp.MustCompile("(?s)```json\\n(.*?)\\n```").FindStringSubmatch(sections[1])
	require.Len(t, block, 2)
	return block[1]
}

func TestWebhookGuideExamplePayloadEscapesAlertText(t *testing.T) {
	for _, message := range []struct {
		name, text string
		value      float64
	}{
		{"simple-test", "A simple test notification", 0},
		{"quoted-name", `Resource "synthetic-host" is unavailable`, 95.25},
		{"backslash-path", `Synthetic path C:\backup\reports`, 12.5},
		{"multiline", "Synthetic first line\nsecond line\r\nthird\tcolumn", 1},
		{"unicode", "Synthetic é host — 警告", 0.0286},
		{"member-injection", `Synthetic text", "injected": true, "other": "value`, 99},
	} {
		t.Run(message.name, func(t *testing.T) {
			manager, webhook, requests := webhookGuideSender(t, webhookGuideExamplePayload(t))
			alert := &alerts.Alert{ID: "synthetic-example", ResourceName: "synthetic-host",
				Level: alerts.AlertLevelWarning, Message: message.text, Value: message.value}
			require.NoError(t, manager.sendGroupedWebhook(webhook, []*alerts.Alert{alert}))
			request, _ := webhookGuideRead(t, requests)
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(request.body), &payload))
			require.Len(t, payload, 2, "alert text must not add JSON members")
			require.Equal(t, "Alert: warning - "+message.text, payload["text"])
			require.Equal(t, message.value, payload["value"], "numbers must remain numeric")
		})
	}
}

func TestWebhookGuideExamplePayloadEscapesRecoveryText(t *testing.T) {
	manager, webhook, requests := webhookGuideSender(t, webhookGuideExamplePayload(t))
	alert := &alerts.Alert{ID: "synthetic-example", ResourceName: "synthetic \"host\"\nwith \\ path",
		Node: "synthetic-node", Level: alerts.AlertLevelWarning, Value: 0,
		StartTime: time.Date(2026, 10, 6, 4, 0, 0, 0, time.UTC)}
	require.NoError(t, manager.sendResolvedWebhook(webhook, []*alerts.Alert{alert}, alert.StartTime.Add(time.Minute)))
	request, _ := webhookGuideRead(t, requests)
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(request.body), &payload))
	require.Len(t, payload, 2)
	require.Equal(t, "Alert: warning - "+alert.ResourceName+" on "+alert.Node+" is now healthy", payload["text"])
	require.Equal(t, float64(0), payload["value"])
}
