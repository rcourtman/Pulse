package notifications

import (
	"bytes"
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// All credentials and private messages in these tests are synthetic.
const appriseSecret = "apprise-synthetic-secret"

type appriseLogBuffer struct {
	sync.Mutex
	bytes.Buffer
}

func (b *appriseLogBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}

func (b *appriseLogBuffer) String() string {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.String()
}

func captureAppriseLogs(t *testing.T) *appriseLogBuffer {
	t.Helper()
	b := &appriseLogBuffer{}
	original := log.Logger
	log.Logger = zerolog.New(b).Level(zerolog.DebugLevel)
	t.Cleanup(func() { log.Logger = original })
	return b
}

func assertAppriseConfidential(t *testing.T, message string) {
	t.Helper()
	for _, secret := range []string{appriseSecret, url.PathEscape(appriseSecret + "/config"), "provider-private-content", "private-alert-body"} {
		if strings.Contains(message, secret) {
			t.Error("Apprise diagnostic exposed a synthetic secret or private content")
		}
	}
}

func sendAppriseForConfidentiality(n *NotificationManager, cfg AppriseConfig, kind string) error {
	alert := &alerts.Alert{ID: "confidentiality", ResourceName: "fixture-node", Message: "private-alert-body", Level: alerts.AlertLevelWarning}
	switch kind {
	case "resolved":
		return n.sendResolvedApprise(cfg, []*alerts.Alert{alert}, time.Now())
	case "test":
		return n.SendTestAppriseWithConfig(cfg)
	default:
		return n.sendGroupedApprise(cfg, []*alerts.Alert{alert})
	}
}

func TestAppriseCLIConfidentiality(t *testing.T) {
	for _, kind := range []string{"firing", "resolved", "test"} {
		for _, failing := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/failure=%t", kind, failing), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				targets := []string{"tgram://" + appriseSecret + "/-1001234567890:42", "future-provider://user:" + appriseSecret + "@fixture/path"}
				cause := errors.New("provider-private-content " + appriseSecret)
				calls := 0
				n := &NotificationManager{appriseExec: func(_ context.Context, args []string) ([]byte, error) {
					calls++
					if len(args) != 4+len(targets) || args[0] != "-t" || args[2] != "-b" || !slices.Equal(args[4:], targets) {
						t.Error("CLI payload or exact target arguments changed")
					}
					if kind != "test" && !strings.Contains(args[3], "private-alert-body") {
						t.Error("alert content was removed from delivery, not just diagnostics")
					}
					if failing {
						return []byte("provider-private-content " + strings.Join(args, " ")), cause
					}
					return []byte("provider-private-content " + strings.Join(args, " ")), nil
				}}
				err := sendAppriseForConfidentiality(n, AppriseConfig{Enabled: true, Targets: targets}, kind)
				if calls != 1 || (err != nil) != failing {
					t.Fatalf("calls=%d error=%t, want one call/failure=%t", calls, err != nil, failing)
				}
				if err != nil {
					assertAppriseConfidential(t, err.Error())
					if !errors.Is(err, cause) || ClassifyNotificationFailureError(err) != NotificationFailureUnknown {
						t.Error("CLI cause or failure class changed")
					}
				}
				assertAppriseConfidential(t, captured.String())
				if !strings.Contains(captured.String(), "outputBytes") || !strings.Contains(captured.String(), "targetCount") {
					t.Error("structured CLI diagnostics were lost")
				}
			})
		}
	}
}

func TestAppriseHTTPConfidentiality(t *testing.T) {
	for _, kind := range []string{"firing", "resolved", "test"} {
		for _, status := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("%s/%d/attempts=%d", kind, status, maxAttempts), func(t *testing.T) {
				captured := captureAppriseLogs(t)
				var calls atomic.Int32
				configKey := appriseSecret + "/config"
				target := "tgram://" + appriseSecret + "/-1001234567890:42"
				server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					var payload struct {
						Body, Title, Type string
						URLs              []string
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error("invalid HTTP payload")
					}
					wantType := "warning"
					if kind == "resolved" {
						wantType = "info"
					}
					if r.Method != http.MethodPost || r.URL.EscapedPath() != "/mounted/"+appriseSecret+"/notify/"+url.PathEscape(configKey) || r.Header.Get("X-API-KEY") != appriseSecret || !slices.Equal(payload.URLs, []string{target}) || payload.Type != wantType {
						t.Error("HTTP endpoint, credentials, targets or event type changed")
					}
					if kind != "test" && !strings.Contains(payload.Body, "private-alert-body") {
						t.Error("HTTP alert content was removed")
					}
					w.WriteHeader(status)
					fmt.Fprint(w, "provider-private-content ", appriseSecret, " ", r.URL.String())
				}))
				defer server.Close()
				n := &NotificationManager{}
				if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
					t.Fatal(err)
				}
				err := sendAppriseForConfidentiality(n, AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL + "/mounted/" + appriseSecret, ConfigKey: configKey, APIKey: appriseSecret, Targets: []string{target}}, kind)
				if calls.Load() != 1 || (err != nil) != (status != http.StatusOK) {
					t.Fatalf("calls=%d error=%t, want one call/status=%d", calls.Load(), err != nil, status)
				}
				if err != nil {
					assertAppriseConfidential(t, err.Error())
					if !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", status)) || ClassifyNotificationFailureError(err) != ClassFromHTTPStatus(status) {
						t.Error("structured HTTP status/class was lost")
					}
				}
				assertAppriseConfidential(t, captured.String())
				if status == http.StatusOK && !strings.Contains(captured.String(), "responseBytes") {
					t.Error("HTTP response observation was lost")
				}
			})
		}
	}
}

func TestAppriseValidationAndRedirectConfidentiality(t *testing.T) {
	for _, raw := range []string{
		"ftp://fixture/" + appriseSecret,
		"http://fixture/%zz" + appriseSecret,
		"http://" + appriseSecret + ":password@fixture",
		"http://fixture/path?key=" + appriseSecret,
		"http://fixture/#" + appriseSecret,
	} {
		t.Run(fmt.Sprintf("invalid-%d", len(raw)), func(t *testing.T) {
			captured := captureAppriseLogs(t)
			err := (&NotificationManager{}).SendTestAppriseWithConfig(AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: raw})
			if err == nil {
				t.Fatal("unsafe base URL was accepted")
			}
			assertAppriseConfidential(t, err.Error())
			assertAppriseConfidential(t, captured.String())
			if ClassifyNotificationFailureError(err) != NotificationFailureConfiguration {
				t.Error("validation retry class changed")
			}
		})
	}
	for _, allowed := range []bool{false, true} {
		t.Run(fmt.Sprintf("redirect-allowed=%t", allowed), func(t *testing.T) {
			captured := captureAppriseLogs(t)
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasPrefix(r.URL.Path, "/notify") {
					target := "http://169.254.169.254/" + appriseSecret
					if allowed {
						target = "/accepted/" + appriseSecret
					}
					http.Redirect(w, r, target, http.StatusTemporaryRedirect)
					return
				}
				fmt.Fprint(w, "provider-private-content ", appriseSecret)
			}))
			defer server.Close()
			n := &NotificationManager{}
			if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
				t.Fatal(err)
			}
			err := n.SendTestAppriseWithConfig(AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL, ConfigKey: appriseSecret})
			if (err == nil) != allowed {
				t.Fatal("redirect security behaviour changed")
			}
			if err != nil {
				assertAppriseConfidential(t, err.Error())
				if !strings.Contains(err.Error(), "link-local addresses are not allowed") {
					t.Error("actionable redirect refusal was lost")
				}
			}
			assertAppriseConfidential(t, captured.String())
		})
	}
}

func TestAppriseTransportConfidentiality(t *testing.T) {
	captured := captureAppriseLogs(t)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		conn.Close()
	}))
	defer server.Close()
	n := &NotificationManager{}
	if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	err := n.SendTestAppriseWithConfig(AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL + "/" + appriseSecret, ConfigKey: appriseSecret})
	if err == nil {
		t.Fatal("broken HTTP connection was treated as delivered")
	}
	assertAppriseConfidential(t, err.Error())
	assertAppriseConfidential(t, captured.String())
	var transportErr *url.Error
	if !errors.As(err, &transportErr) {
		t.Error("typed transport cause was lost")
	}
}

func TestAppriseSafeErrorPreservesClassification(t *testing.T) {
	for _, cause := range []error{
		context.DeadlineExceeded,
		&net.DNSError{Name: appriseSecret, Err: "private DNS error"},
		x509.HostnameError{Host: appriseSecret, Certificate: &x509.Certificate{}},
		FailfWithClass(NotificationFailureAuthentication, "%s", appriseSecret),
		errors.New("rate limit " + appriseSecret),
	} {
		err := safeAppriseError("operation", cause)
		assertAppriseConfidential(t, err.Error())
		if !errors.Is(err, cause) || ClassifyNotificationFailureError(err) != ClassifyNotificationFailureError(cause) {
			t.Error("original cause or retry classification changed")
		}
	}
	cmd := exec.Command("sh", "-c", "exit 7")
	cause := cmd.Run()
	err := safeAppriseError("execute apprise CLI", cause)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || !strings.Contains(err.Error(), "exit status 7") {
		t.Error("structured CLI exit was lost")
	}
	if safeAppriseError("operation", nil) != nil {
		t.Error("nil cause became a failure")
	}
}

// The real worker persists only safe summaries to retry/DLQ, attempt audits and
// the delivery-log projection. Admitted config bytes remain private queue data.
func TestAppriseQueueConfidentiality(t *testing.T) {
	for _, kind := range []string{"apprise", "apprise_resolved"} {
		for _, status := range []int{http.StatusUnauthorized, http.StatusTooManyRequests, http.StatusInternalServerError} {
			for _, maxAttempts := range []int{1, 3} {
				t.Run(fmt.Sprintf("%s/%d/attempts=%d", kind, status, maxAttempts), func(t *testing.T) {
					captured := captureAppriseLogs(t)
					server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(status)
						fmt.Fprint(w, "provider-private-content ", appriseSecret)
					}))
					defer server.Close()
					q, err := NewNotificationQueue(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					defer q.Stop()
					cfg := AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL + "/" + appriseSecret, ConfigKey: appriseSecret, APIKey: appriseSecret, TimeoutSeconds: 5}
					n := &NotificationManager{enabled: true, queue: q, appriseConfig: cfg}
					if err := n.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
						t.Fatal(err)
					}
					config, _ := json.Marshal(cfg)
					notif := &QueuedNotification{ID: "confidentiality", Type: kind, Status: QueueStatusPending, Config: config, MaxAttempts: maxAttempts,
						Alerts: []*alerts.Alert{{ID: "fixture", StartTime: time.Now(), Message: "private-alert-body"}}}
					if err := q.Enqueue(notif); err != nil {
						t.Fatal(err)
					}
					q.SetProcessor(n.ProcessQueuedNotification)
					q.processBatch()
					// The autonomous worker can win the atomic claim. Its committed
					// attempt audit, not the return from our stale snapshot, is proof
					// the send completed. Stop processing before the retry wake.
					deadline := time.Now().Add(3 * time.Second)
					for {
						var count int
						if err := q.db.QueryRow("SELECT count(*) FROM notification_audit WHERE notification_id = ?", notif.ID).Scan(&count); err != nil {
							t.Fatal(err)
						}
						if count > 0 {
							break
						}
						if time.Now().After(deadline) {
							t.Fatal("queue attempt did not complete")
						}
						time.Sleep(time.Millisecond)
					}
					q.SetProcessor(nil)
					var gotStatus, rawConfig string
					// Retry scheduling leaves last_error nullable; the attempt audit
					// holds the failure text. DLQ also retains it on the queue row.
					var lastError sql.NullString
					var attempts int
					if err := q.db.QueryRow("SELECT status, attempts, last_error, config FROM notification_queue WHERE id = ?", notif.ID).Scan(&gotStatus, &attempts, &lastError, &rawConfig); err != nil {
						t.Fatal(err)
					}
					wantStatus := QueueStatusPending
					if maxAttempts == 1 {
						wantStatus = QueueStatusDLQ
					}
					if gotStatus != string(wantStatus) || attempts != 1 || rawConfig != string(config) {
						t.Error("queue lifecycle, attempt budget or admitted credentials changed")
					}
					if maxAttempts == 1 && (!lastError.Valid || !strings.Contains(lastError.String, fmt.Sprintf("HTTP %d", status))) {
						t.Error("DLQ error observation was lost")
					}
					assertAppriseConfidential(t, lastError.String)
					var auditError, class string
					if err := q.db.QueryRow("SELECT error_message, failure_class FROM notification_audit WHERE notification_id = ?", notif.ID).Scan(&auditError, &class); err != nil {
						t.Fatal(err)
					}
					if class != string(ClassFromHTTPStatus(status)) || !strings.Contains(auditError, fmt.Sprintf("HTTP %d", status)) {
						t.Error("audit HTTP status/class was lost")
					}
					assertAppriseConfidential(t, auditError)
					entries, err := q.GetDeliveryLog(time.Time{}, 10)
					if err != nil || len(entries) != 1 {
						t.Fatalf("delivery log entries=%d error=%v", len(entries), err)
					}
					projection, _ := json.Marshal(entries)
					assertAppriseConfidential(t, string(projection))
					assertAppriseConfidential(t, captured.String())
				})
			}
		}
	}
}
