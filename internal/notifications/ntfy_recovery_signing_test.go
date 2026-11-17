package notifications

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Verify what a receiver gets, independently of the producer's signing helper.
// All credentials and destinations are synthetic and confined to loopback.
func ntfyRecoveryRequestVerified(r *http.Request, body []byte, secret string) bool {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return false
	}
	if len(r.Header.Values("X-Pulse-Timestamp")) != 1 || len(r.Header.Values("X-Pulse-Signature")) != 1 {
		return false
	}
	timestamp := r.Header.Get("X-Pulse-Timestamp")
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(seconds, 0)).Abs() > 5*time.Minute {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + "."))
	_, _ = mac.Write(body)
	return hmac.Equal([]byte("v1="+hex.EncodeToString(mac.Sum(nil))), []byte(r.Header.Get("X-Pulse-Signature")))
}

func TestNtfyRecoverySigningContract(t *testing.T) {
	for _, tc := range []struct {
		name, secret                         string
		grouped, leadingNil, unicode, shadow bool
	}{
		{name: "single", secret: "synthetic-ntfy-key"},
		{name: "grouped", secret: "synthetic-ntfy-key", grouped: true},
		{name: "first-non-nil", secret: "synthetic-ntfy-key", leadingNil: true},
		{name: "unicode-bytes", secret: "synthetic-ntfy-key", unicode: true},
		{name: "trimmed-secret", secret: "  synthetic-ntfy-key\t"},
		{name: "custom-integrity-headers", secret: "synthetic-ntfy-key", shadow: true},
		{name: "unsigned"},
		{name: "empty-trimmed-secret", secret: " \t "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			primary := &alerts.Alert{ID: "ntfy-recovery", Type: "cpu", ResourceName: "database", Message: "CPU above threshold", Level: alerts.AlertLevelWarning}
			if tc.unicode {
				primary.ResourceName = "nœud-東京-🚨"
			}
			batch := []*alerts.Alert{primary}
			if tc.grouped {
				batch = append(batch, &alerts.Alert{ID: "ntfy-second", Type: "memory", ResourceName: "worker", Level: alerts.AlertLevelCritical})
			}
			if tc.leadingNil {
				batch = append([]*alerts.Alert{nil}, batch...)
			}
			before := primary.Clone()
			headers := map[string]string{"X-Static": "preserved", "Authorization": "Bearer synthetic-ntfy-auth"}
			if tc.shadow {
				headers["x-pulse-event-id"] = "custom-event"
				headers["x-pulse-timestamp"] = "1"
				headers["x-pulse-signature"] = "v1=custom"
			}
			originalHeaders := make(map[string]string, len(headers))
			for key, value := range headers {
				originalHeaders[key] = value
			}
			resolvedAt := time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC)
			var requests atomic.Int32
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if got := r.Header.Values("X-Pulse-Event-ID"); len(got) != 1 || got[0] != "ntfy-recovery:resolved" {
					t.Error("recovery lacks its primary alert's correlation header")
				}
				if strings.TrimSpace(tc.secret) == "" {
					if len(r.Header.Values("X-Pulse-Signature")) != 0 || len(r.Header.Values("X-Pulse-Timestamp")) != 0 {
						t.Error("empty signing secret generated signing headers")
					}
				} else if !ntfyRecoveryRequestVerified(r, body, tc.secret) {
					t.Error("recovery failed independent verification of its actual wire bytes")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				for key, want := range map[string]string{"Content-Type": "text/plain", "Priority": "default", "Tags": "white_check_mark,pulse,resolved", "X-Static": "preserved", "Authorization": headers["Authorization"]} {
					if r.Header.Get(key) != want {
						t.Errorf("recovery changed %s", key)
					}
				}
				wantTitle := "RESOLVED: " + primary.ResourceName
				wantBody := "Resolved: " + primary.ResourceName + " on " + primary.Node + " is now healthy"
				if len(batch) > 1 {
					count := strconv.Itoa(len(batch))
					wantTitle = "RESOLVED: " + count + " alerts"
					wantBody = count + " alerts resolved at " + resolvedAt.Format(time.RFC822) + ":\n"
					for _, alert := range batch {
						if alert != nil {
							wantBody += "- " + alert.ResourceName + " on " + alert.Node + "\n"
						}
					}
				}
				if r.Header.Get("Title") != wantTitle || string(body) != wantBody {
					t.Errorf("recovery content = %q / %q; want %q / %q", r.Header.Get("Title"), body, wantTitle, wantBody)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			defer server.Close()
			manager := &NotificationManager{webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit)}
			if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			hook := WebhookConfig{ID: "ops", Enabled: true, Service: "ntfy", URL: server.URL + "/topic", SigningSecret: tc.secret, Headers: headers}
			if err := manager.sendResolvedWebhook(hook, batch, resolvedAt); err != nil {
				t.Error(err)
			}
			if requests.Load() != 1 {
				t.Errorf("HTTP requests = %d; want exactly one", requests.Load())
			}
			if !reflect.DeepEqual(primary, before) || !reflect.DeepEqual(headers, originalHeaders) {
				t.Error("delivery mutated the alert or saved custom headers")
			}
		})
	}
}

func TestNtfySignedOrdinaryRecovery(t *testing.T) {
	for _, restart := range []bool{false, true} {
		name := "same-process"
		if restart {
			name = "manager-restart"
		}
		t.Run(name, func(t *testing.T) {
			const secret = "synthetic-ordinary-ntfy-key"
			type request struct {
				event    string
				verified bool
			}
			received := make(chan request, 4)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				verified := ntfyRecoveryRequestVerified(r, body, secret)
				received <- request{r.Header.Get("X-Pulse-Event-ID"), verified}
				if !verified {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.WriteHeader(http.StatusAccepted)
			}))
			defer server.Close()
			dir := t.TempDir()
			hook := WebhookConfig{ID: "signed-ops", Name: "Signed ntfy receiver", Enabled: true, Service: "ntfy", URL: server.URL + "/topic", SigningSecret: secret}
			open := func() *NotificationManager {
				m := NewNotificationManagerWithDataDir("", dir)
				m.webhookClient = server.Client()
				if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				m.SetGroupingConfig(false, 0, false, false)
				m.SetNotifyOnResolve(true)
				m.AddWebhook(hook)
				t.Cleanup(m.Stop)
				return m
			}
			m := open()
			alert := &alerts.Alert{ID: "signed-ordinary-cpu", Type: "cpu", ResourceID: "fixture-node", ResourceName: "database", Level: alerts.AlertLevelCritical, Message: "CPU above threshold", StartTime: time.Now().Add(-time.Minute)}
			job := notificationDeliveryJob{Type: "webhook", Event: eventResolved, Alerts: []*alerts.Alert{alert}, WebhookConfig: &hook}
			waitReceipt := func(want bool) {
				t.Helper()
				deadline := time.Now().Add(5 * time.Second)
				for (len(m.filterResolvedJobsByDeliveryReceipt([]notificationDeliveryJob{job})) == 1) != want {
					if time.Now().After(deadline) {
						t.Fatal("occurrence delivery receipt did not settle")
					}
					time.Sleep(time.Millisecond)
				}
			}
			next := func(event string) {
				t.Helper()
				select {
				case request := <-received:
					if !request.verified || request.event != alert.ID+":"+event {
						t.Fatalf("ordinary %s did not pass the signing-enforcing receiver", event)
					}
				case <-time.After(5 * time.Second):
					t.Fatalf("ordinary %s did not reach the receiver", event)
				}
			}
			m.SendAlert(alert)
			next("alert")
			waitReceipt(true)
			if restart {
				m.Stop()
				m = open()
				waitReceipt(true)
			}
			if m.CancelAlert(alert.ID) {
				t.Fatal("delivered firing was treated as never sent")
			}
			m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: alert, ResolvedTime: time.Now()})
			next("resolved")
			waitReceipt(false)
			var entries []DeliveryLogEntry
			deadline := time.Now().Add(5 * time.Second)
			for {
				var err error
				entries, err = m.GetDeliveryLog(time.Now().Add(-time.Hour), 10)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) == 2 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("delivery log entries = %d; want two committed attempts", len(entries))
				}
				time.Sleep(time.Millisecond)
			}
			for _, entry := range entries {
				if !entry.Success || entry.Outcome != DeliveryOutcomeSent || entry.Attempts != 1 {
					t.Error("signed delivery did not retain one successful attempt")
				}
			}
			m.Stop()
			select {
			case <-received:
				t.Error("duplicate signed delivery after draining")
			default:
			}
		})
	}
}
