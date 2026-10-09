package notifications

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

// Secret-free loopback receivers: none of these tests sends a real notification.
const redirectPrivateHeader = "synthetic-redirect-authentication"
const redirectPrivateBody = "synthetic-redirect-private-alert"
const redirectPrivateLocation = "synthetic-redirect-private-location"

type notificationRedirectRequest struct {
	method, body string
	headers      http.Header
}

type notificationRedirectCapture struct {
	sync.Mutex
	requests []notificationRedirectRequest
}

func (c *notificationRedirectCapture) record(t *testing.T, r *http.Request) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Error("fixture could not read request body")
	}
	c.Lock()
	c.requests = append(c.requests, notificationRedirectRequest{r.Method, string(body), r.Header.Clone()})
	c.Unlock()
}

func (c *notificationRedirectCapture) snapshot() []notificationRedirectRequest {
	c.Lock()
	defer c.Unlock()
	return append([]notificationRedirectRequest(nil), c.requests...)
}

func notificationRedirectFixture(t *testing.T, status int, sameOrigin, hostnameAlias bool) (string, *notificationRedirectCapture, *notificationRedirectCapture) {
	t.Helper()
	initial, final := &notificationRedirectCapture{}, &notificationRedirectCapture{}
	sink := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		final.record(t, r)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(sink.Close)
	origin := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/accepted/"+redirectPrivateLocation {
			final.record(t, r)
			w.WriteHeader(http.StatusAccepted)
			return
		}
		initial.record(t, r)
		target := sink.URL + "/accepted/" + redirectPrivateLocation
		if sameOrigin {
			target = "/accepted/" + redirectPrivateLocation
		} else if hostnameAlias {
			// Same port and resolved IP are not the configured hostname.
			target = strings.Replace("http://"+r.Host, "127.0.0.1", "localhost", 1) + "/accepted/" + redirectPrivateLocation
		}
		http.Redirect(w, r, target, status)
	}))
	t.Cleanup(origin.Close)
	return origin.URL, initial, final
}

func notificationRedirectManager(t *testing.T) *NotificationManager {
	t.Helper()
	n := &NotificationManager{enabled: true, webhookRateLimits: make(map[string]*webhookRateLimit)}
	if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	n.webhookClient = n.createSecureWebhookClient(WebhookTimeout)
	t.Cleanup(n.webhookClient.CloseIdleConnections)
	return n
}

func assertNotificationRedirectRefused(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "redirects must stay on the configured origin") || !strings.Contains(err.Error(), "use the final destination URL") {
		t.Error("foreign redirect was not rejected with actionable destination guidance")
		return
	}
	if ClassifyNotificationFailureError(err) != NotificationFailureConfiguration || isRetryableWebhookError(err) {
		t.Error("local origin refusal lost its terminal configuration class")
	}
	assertNotificationRedirectPrivate(t, err.Error())
}

func assertNotificationRedirectPrivate(t *testing.T, diagnostic string) {
	t.Helper()
	for _, private := range []string{redirectPrivateHeader, redirectPrivateBody, redirectPrivateLocation} {
		if strings.Contains(diagnostic, private) {
			t.Error("receiver-controlled redirect data escaped into diagnostics")
		}
	}
}

// All five standard redirects are fenced, even when Go would change POST to
// GET but still copy custom headers (or Authorization to another port).
func TestNotificationRedirectOriginTransport(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, alias := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/hostname-alias=%t", status, alias), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				base, initial, final := notificationRedirectFixture(t, status, false, alias)
				n := notificationRedirectManager(t)
				webhook := WebhookConfig{URL: base + "/entry", Headers: map[string]string{
					"Authorization": "Bearer " + redirectPrivateHeader, "X-Receiver-Key": redirectPrivateHeader,
				}}
				_, err := n.executeWebhookRequest(webhook, []byte(redirectPrivateBody), webhookRequestOptions{validateURL: true})
				assertNotificationRedirectRefused(t, err)
				first := initial.snapshot()
				if len(first) != 1 || first[0].method != http.MethodPost || first[0].body != redirectPrivateBody || first[0].headers.Get("X-Receiver-Key") != redirectPrivateHeader {
					t.Error("origin admission changed the original configured request")
				}
				if len(final.snapshot()) != 0 {
					t.Error("foreign origin received notification data")
				}
				assertNotificationRedirectPrivate(t, captured.String())
			})
		}
	}
}

func TestNotificationRedirectOriginSendPaths(t *testing.T) {
	for _, path := range []string{"single", "grouped", "resolved", "ntfy-resolved", "enhanced-retry", "test", "enhanced-test", "apprise", "apprise-resolved", "apprise-test"} {
		for _, same := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/same-origin=%t", path, same), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				base, initial, final := notificationRedirectFixture(t, http.StatusTemporaryRedirect, same, false)
				n := notificationRedirectManager(t)
				webhook := WebhookConfig{Name: "redirect fixture", URL: base + "/entry", Enabled: true,
					Headers: map[string]string{"X-Receiver-Key": redirectPrivateHeader}, SigningSecret: "synthetic-signing-key"}
				alert := &alerts.Alert{ID: "redirect-fixture", Type: "cpu", Message: redirectPrivateBody, ResourceName: "fixture-node",
					Level: alerts.AlertLevelWarning, Value: 90, Threshold: 80, StartTime: time.Unix(100, 0)}
				cfg := AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: base, ConfigKey: "fixture-config",
					APIKey: redirectPrivateHeader, APIKeyHeader: "X-Receiver-Key", Targets: []string{"tgram://" + redirectPrivateHeader + "/fixture"}, TimeoutSeconds: 5}
				var err error
				switch path {
				case "single":
					err = n.sendGroupedWebhook(webhook, []*alerts.Alert{alert})
				case "grouped":
					err = n.sendGroupedWebhook(webhook, []*alerts.Alert{alert, alert.Clone()})
				case "resolved":
					err = n.sendResolvedWebhook(webhook, []*alerts.Alert{alert}, time.Unix(200, 0))
				case "ntfy-resolved":
					webhook.Service = "ntfy"
					err = n.sendResolvedWebhook(webhook, []*alerts.Alert{alert}, time.Unix(200, 0))
				case "enhanced-retry":
					err = n.sendWebhookWithRetry(EnhancedWebhookConfig{WebhookConfig: webhook, RetryCount: 3}, []byte(redirectPrivateBody), "fixture:alert")
				case "test":
					err = n.SendTestWebhook(webhook)
				case "enhanced-test":
					_, _, err = n.TestEnhancedWebhook(BuildEnhancedWebhookTestConfig(webhook, "generic"))
				case "apprise":
					err = n.sendGroupedApprise(cfg, []*alerts.Alert{alert})
				case "apprise-resolved":
					err = n.sendResolvedApprise(cfg, []*alerts.Alert{alert}, time.Unix(200, 0))
				case "apprise-test":
					err = n.SendTestAppriseWithConfig(cfg)
				}
				if same {
					if err != nil {
						t.Error("same-origin notification redirect stopped working")
					}
				} else {
					assertNotificationRedirectRefused(t, err)
				}
				first, last := initial.snapshot(), final.snapshot()
				if len(first) != 1 || first[0].headers.Get("X-Receiver-Key") != redirectPrivateHeader {
					t.Error("configured receiver did not get exactly one authenticated attempt")
				}
				if !same && len(last) != 0 {
					t.Error("foreign receiver got a firing, recovery or Test notification")
				}
				if same && (len(last) != 1 || len(first) != 1 || last[0].body != first[0].body || last[0].method != first[0].method || last[0].headers.Get("X-Receiver-Key") != redirectPrivateHeader || last[0].headers.Get("X-Pulse-Signature") != first[0].headers.Get("X-Pulse-Signature")) {
					t.Error("same-origin redirect changed payload, method, credentials or signature")
				}
				if !same {
					assertNotificationRedirectPrivate(t, captured.String())
				}
				if path == "enhanced-retry" {
					history := n.GetWebhookHistory()
					if len(history) != 1 || history[0].Success != same || history[0].RetryAttempts != 0 {
						t.Error("redirect refusal was retried or marked delivered")
					}
					if len(history) == 1 && !same {
						assertNotificationRedirectPrivate(t, history[0].ErrorMessage)
					}
				}
			})
		}
	}
}

func TestNotificationRedirectOriginTLSDowngrade(t *testing.T) {
	for _, skipTLS := range []bool{false, true} {
		t.Run(fmt.Sprintf("skip-tls=%t", skipTLS), func(t *testing.T) {
			initial, final := &notificationRedirectCapture{}, &notificationRedirectCapture{}
			sink := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				final.record(t, r)
				w.WriteHeader(http.StatusAccepted)
			}))
			defer sink.Close()
			origin := newIPv4TLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				initial.record(t, r)
				http.Redirect(w, r, sink.URL+"/"+redirectPrivateLocation, http.StatusPermanentRedirect)
			}))
			defer origin.Close()
			n := notificationRedirectManager(t)
			n.webhookClient = n.createSecureWebhookClientWithTLS(WebhookTimeout, skipTLS)
			defer n.webhookClient.CloseIdleConnections()
			if !skipTLS {
				roots := x509.NewCertPool()
				roots.AddCert(origin.Certificate())
				n.webhookClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
			}
			_, err := n.executeWebhookRequest(WebhookConfig{URL: origin.URL, Headers: map[string]string{"X-Receiver-Key": redirectPrivateHeader}}, []byte(redirectPrivateBody), webhookRequestOptions{validateURL: true})
			assertNotificationRedirectRefused(t, err)
			if len(initial.snapshot()) != 1 || len(final.snapshot()) != 0 {
				t.Error("HTTPS origin failed authentication or downgraded private delivery")
			}
		})
	}
}

func TestNotificationSameOriginSignedReplay(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			requests := &notificationRedirectCapture{}
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.record(t, r)
				switch r.URL.Path {
				case "/first":
					http.Redirect(w, r, "/middle", status)
				case "/middle":
					http.Redirect(w, r, "/final?kept=fixture", status)
				default:
					w.WriteHeader(http.StatusAccepted)
				}
			}))
			defer server.Close()
			n := notificationRedirectManager(t)
			webhook := WebhookConfig{URL: server.URL + "/first", SigningSecret: "synthetic-signing-key",
				Headers: map[string]string{"Authorization": "Bearer " + redirectPrivateHeader, "X-Receiver-Key": redirectPrivateHeader}}
			if err := n.sendWebhookRequest(webhook, []byte(redirectPrivateBody), "alert", "fixture:alert"); err != nil {
				t.Fatal("same-origin signed delivery did not complete")
			}
			got := requests.snapshot()
			if len(got) != WebhookMaxRedirects {
				t.Fatal("same-origin chain did not reach the receiver within the unchanged limit")
			}
			for _, request := range got {
				timestamp := request.headers.Get("X-Pulse-Timestamp")
				if request.method != http.MethodPost || request.body != redirectPrivateBody || request.headers.Get("Authorization") != "Bearer "+redirectPrivateHeader || request.headers.Get("X-Receiver-Key") != redirectPrivateHeader || request.headers.Get("X-Pulse-Event-ID") != "fixture:alert" || timestamp == "" || request.headers.Get("X-Pulse-Signature") != "v1="+signWebhookPayload(webhook.SigningSecret, timestamp, []byte(redirectPrivateBody)) || timestamp != got[0].headers.Get("X-Pulse-Timestamp") {
					t.Error("same-origin replay altered authentication, event identity or signed body")
				}
			}
		})
	}
}

func TestNotificationRedirectOriginQueue(t *testing.T) {
	for _, kind := range []string{"webhook", "webhook_resolved", "apprise", "apprise_resolved"} {
		for _, same := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/same-origin=%t", kind, same), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				base, initial, final := notificationRedirectFixture(t, http.StatusTemporaryRedirect, same, false)
				q, err := NewNotificationQueue(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer q.Stop()
				n := notificationRedirectManager(t)
				n.queue = q
				webhook := WebhookConfig{ID: "redirect-destination", Name: "redirect fixture", Enabled: true, URL: base + "/entry",
					Headers: map[string]string{"X-Receiver-Key": redirectPrivateHeader}}
				cfg := AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: base, APIKey: redirectPrivateHeader, APIKeyHeader: "X-Receiver-Key", TimeoutSeconds: 5}
				n.webhooks, n.appriseConfig = []WebhookConfig{webhook}, cfg
				alert := &alerts.Alert{ID: "redirect-occurrence", StartTime: time.Unix(100, 0), Message: redirectPrivateBody}
				job := notificationDeliveryJob{Type: "webhook", Event: eventAlert, WebhookConfig: &webhook, Alerts: []*alerts.Alert{alert}}
				config, err := json.Marshal(webhook)
				if strings.HasPrefix(kind, "apprise") {
					job.Type, job.AppriseConfig = "apprise", &cfg
					config, err = json.Marshal(cfg)
				}
				if err != nil {
					t.Fatal(err)
				}
				resolved := strings.HasSuffix(kind, "_resolved")
				destination := notificationDeliveryDestinationKey(job)
				if resolved {
					if err := q.RecordDeliveryReceipt(alert.ID, alert.StartTime, destination, time.Unix(150, 0)); err != nil {
						t.Fatal(err)
					}
				}
				notif := &QueuedNotification{ID: "redirect-job", Type: kind, Status: QueueStatusPending, Config: config, Alerts: job.Alerts, MaxAttempts: 3}
				if err := q.Enqueue(notif); err != nil {
					t.Fatal(err)
				}
				q.SetProcessor(n.ProcessQueuedNotification)
				// Observe the real dispatcher; do not race it with a manual batch.
				deadline := time.Now().Add(5 * time.Second)
				for {
					var count int
					if err := q.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = ?", notif.ID).Scan(&count); err != nil {
						t.Fatal(err)
					}
					if count != 0 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("notification queue did not finish its attempt")
					}
					time.Sleep(time.Millisecond)
				}
				q.SetProcessor(nil)
				var status, rawConfig string
				var attempts int
				var lastError sql.NullString
				if err := q.db.QueryRow("SELECT status, attempts, last_error, config FROM notification_queue WHERE id = ?", notif.ID).Scan(&status, &attempts, &lastError, &rawConfig); err != nil {
					t.Fatal(err)
				}
				wantStatus := QueueStatusDLQ
				if same {
					wantStatus = QueueStatusSent
				}
				if status != string(wantStatus) || attempts != 1 || rawConfig != string(config) || len(initial.snapshot()) != 1 || (!same && len(final.snapshot()) != 0) || (same && len(final.snapshot()) != 1) {
					t.Error("queue origin verdict, attempt budget or private config snapshot changed")
				}
				var auditError, class string
				var success bool
				if err := q.db.QueryRow("SELECT error_message, failure_class, success FROM notification_audit WHERE notification_id = ?", notif.ID).Scan(&auditError, &class, &success); err != nil {
					t.Fatal(err)
				}
				if success != same || (!same && (class != string(NotificationFailureConfiguration) || !strings.Contains(lastError.String, "configured origin"))) {
					t.Error("origin refusal was reported as delivered or lost from the dead letter/audit")
				}
				if !same {
					assertNotificationRedirectPrivate(t, lastError.String)
					assertNotificationRedirectPrivate(t, auditError)
					entries, err := q.GetDeliveryLog(time.Time{}, 10)
					if err != nil || len(entries) != 1 {
						t.Fatal("delivery-log projection did not retain exactly one failed attempt")
					}
					projection, err := json.Marshal(entries)
					if err != nil {
						t.Fatal(err)
					}
					assertNotificationRedirectPrivate(t, string(projection))
					assertNotificationRedirectPrivate(t, captured.String())
				}
				hasReceipt, err := q.HasDeliveryReceipt(alert.ID, alert.StartTime, destination)
				if err != nil || hasReceipt != (same != resolved) {
					t.Error("refused redirect invented a firing receipt or erased a recovery receipt")
				}
			})
		}
	}
}

// The count limit must not hide an origin refusal: net/http attaches the final
// Location to its error even when it never requests that receiver.
func TestNotificationRedirectOriginAtLimit(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		for _, path := range []string{"request", "enhanced-retry", "apprise"} {
			t.Run(fmt.Sprintf("%d/%s", status, path), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				initial, final := &notificationRedirectCapture{}, &notificationRedirectCapture{}
				sink := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					final.record(t, r)
					w.WriteHeader(http.StatusAccepted)
				}))
				defer sink.Close()
				origin := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					initial.record(t, r)
					target := sink.URL + "/" + redirectPrivateLocation
					switch r.URL.Path {
					case "/same-origin-1":
						target = "/same-origin-2"
					case "/same-origin-2":
					default:
						target = "/same-origin-1"
					}
					http.Redirect(w, r, target, status)
				}))
				defer origin.Close()
				n := notificationRedirectManager(t)
				webhook := WebhookConfig{Name: "capped redirect fixture", URL: origin.URL + "/entry",
					Headers: map[string]string{"X-Receiver-Key": redirectPrivateHeader}}
				var err error
				switch path {
				case "request":
					_, err = n.executeWebhookRequest(webhook, []byte(redirectPrivateBody), webhookRequestOptions{validateURL: true})
				case "enhanced-retry":
					err = n.sendWebhookWithRetry(EnhancedWebhookConfig{WebhookConfig: webhook, RetryCount: 1}, []byte(redirectPrivateBody), "fixture:alert")
				case "apprise":
					cfg := AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: origin.URL, APIKey: redirectPrivateHeader, APIKeyHeader: "X-Receiver-Key", TimeoutSeconds: 5}
					err = n.sendGroupedApprise(cfg, []*alerts.Alert{{ID: "capped-redirect", Message: redirectPrivateBody}})
				}
				assertNotificationRedirectRefused(t, err)
				if len(initial.snapshot()) != WebhookMaxRedirects || len(final.snapshot()) != 0 {
					t.Error("capped foreign redirect was retried or contacted its receiver")
				}
				assertNotificationRedirectPrivate(t, captured.String())
				if path == "enhanced-retry" {
					history := n.GetWebhookHistory()
					if len(history) != 1 || history[0].Success || history[0].RetryAttempts != 0 {
						t.Error("capped foreign redirect lost its one-attempt failure state")
					}
					if len(history) == 1 {
						assertNotificationRedirectPrivate(t, history[0].ErrorMessage)
					}
				}
			})
		}
	}
}
