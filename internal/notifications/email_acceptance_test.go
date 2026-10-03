package notifications

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Real guest-local SMTP connections cover all three transports. A message is
// accepted only after the final DATA reply, not when its bytes arrive. QUIT
// errors deliberately contain private-looking prose: cleanup diagnostics must
// neither resend the accepted message nor copy that prose into a new log.
func acceptanceSMTPServer(t *testing.T, mode string, dataCode int, quitReply string) (*EnhancedEmailManager, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	certServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tlsConfig := certServer.TLS.Clone()
	certServer.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var sessions sync.WaitGroup
	connections, accepted := &atomic.Int32{}, &atomic.Int32{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				defer raw.Close()
				_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
				wire := raw
				if mode == "TLS" {
					wire = tls.Server(raw, tlsConfig)
				}
				conn := textproto.NewConn(wire)
				if err := conn.PrintfLine("220 localhost ESMTP"); err != nil {
					return
				}
				upgraded := mode == "TLS"
				for {
					line, err := conn.ReadLine()
					if err != nil {
						return
					}
					switch strings.Fields(line)[0] {
					case "EHLO", "HELO":
						if mode == "STARTTLS" && !upgraded {
							err = conn.PrintfLine("250-localhost\r\n250 STARTTLS")
						} else {
							err = conn.PrintfLine("250 localhost")
						}
					case "STARTTLS":
						if err = conn.PrintfLine("220 Ready for TLS"); err == nil {
							conn = textproto.NewConn(tls.Server(raw, tlsConfig))
							upgraded = true
						}
					case "MAIL", "RCPT":
						err = conn.PrintfLine("250 OK")
					case "DATA":
						if err = conn.PrintfLine("354 Send message"); err != nil {
							return
						}
						if _, err = conn.ReadDotBytes(); err != nil {
							return
						}
						if dataCode == 0 { // No acknowledgement: acceptance is unknown.
							return
						}
						err = conn.PrintfLine("%d transaction verdict", dataCode)
						if err == nil && dataCode == 250 {
							accepted.Add(1)
						}
					case "QUIT":
						if quitReply != "" {
							_ = conn.PrintfLine("%s", quitReply)
						}
						return
					default:
						t.Errorf("unexpected SMTP command %q", line)
						return
					}
					if err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done; sessions.Wait() })
	_, rawPort, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		EmailConfig: EmailConfig{SMTPHost: "127.0.0.1", SMTPPort: port,
			From: "pulse@example.test", To: []string{"recipient@example.test"}, TLS: mode == "TLS"},
		StartTLS: mode == "STARTTLS", SkipTLSVerify: true, // Ephemeral fixture certificate only.
		MaxRetries: 2, RetryDelay: 0,
	})
	return manager, connections, accepted
}

func TestSMTPDeliveryAcceptanceSurvivesCleanup(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		for _, tc := range []struct {
			name, quit string
			dataCode   int
			class      NotificationFailureClass
			attempts   int32
		}{
			{"clean", "221 Bye", 250, "", 1},
			{"closed at QUIT", "", 250, "", 1},
			{"rejected QUIT", "500 private-cleanup-token", 250, "", 1},
			{"auth prose at QUIT", "535 private-cleanup-token", 250, "", 1},
			{"message rejected", "221 Bye", 550, NotificationFailureRejected, 1},
			{"temporary message failure", "221 Bye", 451, NotificationFailureServerError, 3},
			{"missing acceptance", "221 Bye", 0, NotificationFailureUnknown, 3},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				manager, connections, accepted := acceptanceSMTPServer(t, mode, tc.dataCode, tc.quit)
				captured := captureNotificationQueueLogs(t)
				err := manager.SendEmailWithRetry("test", "<p>test</p>", "test")
				if (err == nil) != (tc.class == "") {
					t.Errorf("delivery error = %v; want class %q", err, tc.class)
				}
				if tc.class != "" && ClassifyNotificationFailureError(err) != tc.class {
					t.Errorf("failure class = %q; want %q", ClassifyNotificationFailureError(err), tc.class)
				}
				if got := connections.Load(); got != tc.attempts {
					t.Errorf("connections = %d; want %d", got, tc.attempts)
				}
				wantAccepted := int32(0)
				if tc.dataCode == 250 {
					wantAccepted = 1
				}
				if got := accepted.Load(); got != wantAccepted {
					t.Errorf("accepted messages = %d; want %d", got, wantAccepted)
				}
				if strings.Contains(captured.String(), "private-cleanup-token") {
					t.Error("cleanup diagnostics exposed response text")
				}
			})
		}
	}
}

func TestSMTPAcceptedThreadedAndAttachmentDelivery(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		for _, variant := range []string{"threaded", "attachment"} {
			t.Run(mode+"/"+variant, func(t *testing.T) {
				manager, connections, accepted := acceptanceSMTPServer(t, mode, 250, "")
				var err error
				if variant == "threaded" {
					err = manager.SendEmailThreaded("test", "<p>test</p>", "test", "<incident@example.test>")
				} else {
					err = manager.SendEmailWithAttachments("test", "<p>test</p>", "test",
						[]EmailAttachment{{Filename: "test.txt", ContentType: "text/plain", Data: []byte("test")}})
				}
				if err != nil || connections.Load() != 1 || accepted.Load() != 1 {
					t.Errorf("error=%v connections=%d accepted=%d; want one accepted message", err, connections.Load(), accepted.Load())
				}
			})
		}
	}
}

func TestSMTPConnectionCheckAndCertificateFailureRemainFailures(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		t.Run(mode, func(t *testing.T) {
			manager, connections, accepted := acceptanceSMTPServer(t, mode, 250, "500 cleanup rejected")
			if err := manager.TestConnection(); err == nil {
				t.Error("connection test without DATA reported success after rejected QUIT")
			}
			if accepted.Load() != 0 || connections.Load() != 1 {
				t.Errorf("connection check sent a message or retried: %d / %d", accepted.Load(), connections.Load())
			}
			if mode != "plain" {
				manager.config.SkipTLSVerify = false
				manager.config.MaxRetries = 0
				err := manager.SendEmailWithRetry("test", "test", "test")
				if err == nil || ClassifyNotificationFailureError(err) != NotificationFailureTLS || accepted.Load() != 0 {
					t.Errorf("certificate failure was hidden: error=%v accepted=%d", err, accepted.Load())
				}
			}
		})
	}
}

func TestSMTPAcceptedMessageQueueAuditAcrossRestart(t *testing.T) {
	manager, connections, accepted := acceptanceSMTPServer(t, "plain", 250, "")
	dir := t.TempDir()
	q, err := NewNotificationQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = q.Stop() })
	q.SetProcessor(func(*QueuedNotification) error { return manager.SendEmailWithRetry("test", "test", "test") })
	if err := q.Enqueue(&QueuedNotification{ID: "accepted", Type: "email", Config: []byte(`{}`), MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}
	waitVerdictAudit(t, q, "accepted")
	if err := q.Stop(); err != nil {
		t.Fatal(err)
	}
	q, err = NewNotificationQueue(dir)
	if err != nil {
		t.Fatal(err)
	}
	var status, class string
	var attempts, success int
	if err := q.db.QueryRow(`SELECT status, attempts, success, failure_class FROM notification_audit WHERE notification_id = 'accepted'`).Scan(&status, &attempts, &success, &class); err != nil {
		t.Fatal(err)
	}
	stats, err := q.GetTelemetryStats(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if status != string(QueueStatusSent) || attempts != 1 || success != 1 || class != "" ||
		stats.Attempts != 1 || stats.Deliveries != 1 || stats.Failures != 0 || connections.Load() != 1 || accepted.Load() != 1 {
		t.Errorf("accepted message became duplicate/failure: audit=%s/%d/%d/%s stats=%+v connections=%d accepted=%d",
			status, attempts, success, class, stats, connections.Load(), accepted.Load())
	}
	if count, err := q.RetryTerminalFailures(); err != nil || count != 0 {
		t.Errorf("accepted message available for operator retry: %d / %v", count, err)
	}
}

func waitVerdictAudit(t *testing.T, q *NotificationQueue, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var count int
		if err := q.db.QueryRow(`SELECT count(*) FROM notification_audit WHERE notification_id = ?`, id).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal(fmt.Sprintf("missing delivery audit for %s", id))
		}
		time.Sleep(time.Millisecond)
	}
}
