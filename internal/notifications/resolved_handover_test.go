package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

const movedSummary = "Memory alert moved to pve1 (Host Agent). This is not a recovery: check the agent for the current reading."

// movedNodeMemoryAlert is a node memory alert that closed because the Pulse
// agent linked to the node took over its usage alerts, not because memory
// came back down.
func movedNodeMemoryAlert() *alerts.Alert {
	return &alerts.Alert{
		ID:           "pve1-memory",
		Type:         "memory",
		Level:        alerts.AlertLevelWarning,
		ResourceName: "pve1",
		ResourceID:   "pve1",
		Node:         "pve1",
		Instance:     "https://pve.local:8006",
		Message:      "Node memory at 95.0%",
		Value:        95,
		Threshold:    90,
		StartTime:    time.Now().Add(-10 * time.Minute),
		Resolution: &alerts.AlertResolution{
			Reason:              alerts.AlertResolutionMovedToAgent,
			SuccessorResourceID: "agent-pve1",
			SuccessorName:       "pve1 (Host Agent)",
		},
	}
}

// Exercise the normal entry points: the firing goes out, its receipt gates the
// close, and the close is persisted in the delivery queue and read back before
// it is rendered.
func TestMovedAlertCloseIsDeliveredAsMovedNotRecovered(t *testing.T) {
	type message struct{ title, body, tags string }
	received := make(chan message, 4)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read notification: %v", err)
		}
		received <- message{r.Header.Get("Title"), string(body), r.Header.Get("Tags")}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	webhook := WebhookConfig{ID: "ops", Name: "ops", URL: server.URL + "/topic", Enabled: true, Service: "ntfy"}
	m := NewNotificationManagerWithDataDir("", t.TempDir())
	t.Cleanup(m.Stop)
	m.webhookClient = server.Client()
	if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	m.SetGroupingConfig(false, 0, false, false)
	m.SetNotifyOnResolve(true)
	m.AddWebhook(webhook)

	next := func() message {
		t.Helper()
		select {
		case msg := <-received:
			return msg
		case <-time.After(10 * time.Second):
			t.Fatal("notification not received")
			return message{}
		}
	}

	alert := movedNodeMemoryAlert()
	firing := alert.Clone()
	firing.Resolution = nil
	m.SendAlert(firing)
	next()
	recoveryJob := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{alert}, WebhookConfig: &webhook}
	deadline := time.Now().Add(10 * time.Second)
	for len(m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{recoveryJob})) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("firing receipt never recorded")
		}
		time.Sleep(time.Millisecond)
	}

	m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: time.Now()})
	closing := next()
	if closing.title != "MOVED: pve1" || closing.body != movedSummary {
		t.Fatalf("close = %+v, want MOVED title and the handover summary", closing)
	}
	if strings.Contains(closing.body, "healthy") || strings.Contains(closing.tags, "white_check_mark") {
		t.Fatalf("close reads as a recovery: %+v", closing)
	}
}

// captureResolvedWebhook renders one resolved delivery through an isolated
// manager and returns what the destination received.
func captureResolvedWebhook(t *testing.T, service string, alertList []*alerts.Alert) ([]byte, http.Header) {
	t.Helper()
	type request struct {
		body   []byte
		header http.Header
	}
	received := make(chan request, 1)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- request{body: body, header: r.Header.Clone()}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	nm := NewNotificationManagerWithDataDir("https://pulse.local", t.TempDir())
	t.Cleanup(nm.Stop)
	if err := nm.UpdateAllowedPrivateCIDRs("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	webhook := WebhookConfig{Name: service, URL: server.URL + "/hook", Enabled: true, Service: service,
		CustomFields: map[string]string{"routing_key": "rk"}}
	if err := nm.sendResolvedWebhook(webhook, alertList, time.Now()); err != nil {
		t.Fatalf("sendResolvedWebhook %s: %v", service, err)
	}
	select {
	case got := <-received:
		return got.body, got.header
	case <-time.After(5 * time.Second):
		t.Fatalf("%s destination received nothing", service)
		return nil, nil
	}
}

func decodeResolvedPayload(t *testing.T, service string, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal %s payload: %v\n%s", service, err, body)
	}
	return payload
}

func TestMovedAlertCloseRendersHonestlyOnEveryChannel(t *testing.T) {
	recovered := &alerts.Alert{ID: "vm-cpu", Type: "cpu", ResourceName: "web-vm-01", Node: "pve1", StartTime: time.Now().Add(-time.Minute)}

	t.Run("discord says moved without the recovery colour", func(t *testing.T) {
		body, _ := captureResolvedWebhook(t, "discord", []*alerts.Alert{movedNodeMemoryAlert()})
		embed := decodeResolvedPayload(t, "discord", body)["embeds"].([]any)[0].(map[string]any)
		description, _ := embed["description"].(string)
		if !strings.Contains(description, movedSummary) || strings.Contains(description, "healthy") {
			t.Fatalf("description = %q, want the handover summary", description)
		}
		if embed["color"] != float64(3447003) {
			t.Fatalf("embed color = %v, want the neutral 3447003, not recovery green", embed["color"])
		}
	})

	t.Run("teams cards drop recovery green", func(t *testing.T) {
		body, _ := captureResolvedWebhook(t, "teams", []*alerts.Alert{movedNodeMemoryAlert()})
		if got := decodeResolvedPayload(t, "teams", body)["themeColor"]; got != "0076D7" {
			t.Fatalf("teams themeColor = %v, want 0076D7", got)
		}
		body, _ = captureResolvedWebhook(t, "teams-adaptive", []*alerts.Alert{movedNodeMemoryAlert()})
		if !strings.Contains(string(body), `"color": "Accent"`) || strings.Contains(string(body), `"color": "Good"`) {
			t.Fatalf("teams adaptive card = %s, want the Accent title colour", body)
		}
	})

	t.Run("mattermost drops the check mark", func(t *testing.T) {
		body, _ := captureResolvedWebhook(t, "mattermost", []*alerts.Alert{movedNodeMemoryAlert()})
		text, _ := decodeResolvedPayload(t, "mattermost", body)["text"].(string)
		if strings.Contains(text, ":white_check_mark:") || !strings.Contains(text, ":arrow_right: **RESOLVED**") || !strings.Contains(text, movedSummary) {
			t.Fatalf("mattermost text = %q", text)
		}
	})

	t.Run("pagerduty still closes the incident", func(t *testing.T) {
		body, _ := captureResolvedWebhook(t, "pagerduty", []*alerts.Alert{movedNodeMemoryAlert()})
		payload := decodeResolvedPayload(t, "pagerduty", body)
		if payload["event_action"] != "resolve" || payload["dedup_key"] != "pve1-memory" {
			t.Fatalf("payload = %v, want a resolve for the node alert's incident", payload)
		}
	})

	t.Run("grouped list names the move", func(t *testing.T) {
		body, _ := captureResolvedWebhook(t, "slack", []*alerts.Alert{movedNodeMemoryAlert(), recovered})
		if !strings.Contains(string(body), "pve1 on pve1, moved to pve1 (Host Agent)") || !strings.Contains(string(body), "web-vm-01 on pve1") {
			t.Fatalf("grouped body = %s, want the moved alert marked and the recovery listed", body)
		}
	})

	t.Run("grouped ntfy batch drops the check mark", func(t *testing.T) {
		body, header := captureResolvedWebhook(t, "ntfy", []*alerts.Alert{movedNodeMemoryAlert(), recovered})
		if tags := header.Get("Tags"); strings.Contains(tags, "white_check_mark") || !strings.Contains(tags, "arrow_right") {
			t.Fatalf("ntfy tags = %q, want no check mark on a batch holding a move", tags)
		}
		if !strings.Contains(string(body), "- pve1 on pve1, moved to pve1 (Host Agent)\n") {
			t.Fatalf("ntfy body = %q", body)
		}
	})

	t.Run("email and apprise say moved", func(t *testing.T) {
		title, htmlBody, textBody := buildResolvedNotificationContent([]*alerts.Alert{movedNodeMemoryAlert()}, time.Now(), "")
		if title != "Pulse alert moved: pve1" {
			t.Fatalf("title = %q", title)
		}
		if !strings.Contains(textBody, "Node memory at 95.0%\n"+movedSummary+"\n") {
			t.Fatalf("text body = %q, want the breach followed by the handover summary", textBody)
		}
		if !strings.Contains(htmlBody, "This is not a recovery: check the agent for the current reading.") {
			t.Fatalf("html body = %q", htmlBody)
		}
	})

	t.Run("ordinary recovery keeps its wording and visuals", func(t *testing.T) {
		plain := movedNodeMemoryAlert()
		plain.Resolution = nil
		if got := resolvedAlertMessage(plain); got != "pve1 on pve1 is now healthy" {
			t.Fatalf("resolvedAlertMessage() = %q", got)
		}
		if title, _, _ := buildResolvedNotificationContent([]*alerts.Alert{plain}, time.Now(), ""); title != "Pulse alert resolved: pve1" {
			t.Fatalf("title = %q", title)
		}
		body, _ := captureResolvedWebhook(t, "discord", []*alerts.Alert{plain})
		if embed := decodeResolvedPayload(t, "discord", body)["embeds"].([]any)[0].(map[string]any); embed["color"] != float64(3066993) {
			t.Fatalf("ordinary recovery embed color = %v, want green 3066993", embed["color"])
		}
		body, _ = captureResolvedWebhook(t, "mattermost", []*alerts.Alert{plain})
		if text, _ := decodeResolvedPayload(t, "mattermost", body)["text"].(string); !strings.Contains(text, ":white_check_mark: **RESOLVED**") {
			t.Fatalf("ordinary mattermost text = %q", text)
		}
		_, header := captureResolvedWebhook(t, "ntfy", []*alerts.Alert{plain})
		if tags := header.Get("Tags"); tags != "white_check_mark,pulse,resolved" {
			t.Fatalf("ordinary ntfy tags = %q", tags)
		}
	})
}
