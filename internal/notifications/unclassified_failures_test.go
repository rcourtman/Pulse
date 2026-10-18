package notifications

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"text/template"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnclassifiedTLSAlertRetainsTLSVerdict(t *testing.T) {
	listener, err := tls.Listen("tcp4", "127.0.0.1:0", &tls.Config{GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
		return nil, errors.New("synthetic handshake refusal")
	}})
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		_ = conn.(*tls.Conn).Handshake()
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done })
	client, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp4", listener.Addr().String(), &tls.Config{InsecureSkipVerify: true})
	if client != nil {
		defer client.Close()
	}
	require.Error(t, err)
	var operation *net.OpError
	require.ErrorAs(t, err, &operation)
	require.Equal(t, NotificationFailureTLS, ClassifyNotificationFailureError(err))
}

func TestUnclassifiedWireErrorsUseStructuralEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want NotificationFailureClass
	}{
		{"plain EOF", fmt.Errorf("send failed: %w", io.EOF), NotificationFailureConnectivity},
		{"authentication stage EOF", fmt.Errorf("SMTP auth failed: %w", io.EOF), NotificationFailureConnectivity},
		{"TLS stage EOF", fmt.Errorf("TLS dial failed: %w", io.EOF), NotificationFailureConnectivity},
		{"truncation with misleading context", fmt.Errorf("authentication failed: %w", io.ErrUnexpectedEOF), NotificationFailureConnectivity},
		{"opaque network operation", &net.OpError{Op: "read", Net: "tcp", Err: errors.New("opaque wire interruption")}, NotificationFailureConnectivity},
		{"declared rejection wins", FailWithClass(NotificationFailureRejected, io.EOF), NotificationFailureRejected},
		{"unconfirmed SMTP acceptance remains unknown", &EmailAcceptanceUnconfirmedError{Err: io.EOF}, NotificationFailureUnknown},
		{"SMTP reply wins", &textproto.Error{Code: 535, Msg: "receiver disconnected"}, NotificationFailureAuthentication},
		{"certificate wins", &net.OpError{Op: "dial", Net: "tcp", Err: x509.UnknownAuthorityError{}}, NotificationFailureTLS},
		{"opaque error remains unknown", errors.New("opaque delivery failure"), NotificationFailureUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ClassifyNotificationFailureError(tc.err))
		})
	}
}

func TestUnclassifiedClosedHTTPConnection(t *testing.T) {
	for _, kind := range []string{"webhook", "apprise"} {
		t.Run(kind, func(t *testing.T) {
			server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, _, err := w.(http.Hijacker).Hijack()
				if err != nil {
					t.Error(err)
					return
				}
				_ = conn.Close()
			}))
			defer server.Close()
			manager := &NotificationManager{webhookClient: server.Client(), webhookRateLimits: make(map[string]*webhookRateLimit)}
			require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
			var err error
			if kind == "webhook" {
				err = manager.sendWebhookRequest(WebhookConfig{URL: server.URL}, []byte(`{"synthetic":true}`), "test", "synthetic-event")
			} else {
				err = manager.SendTestAppriseWithConfig(AppriseConfig{Enabled: true, Mode: AppriseModeHTTP, ServerURL: server.URL})
			}
			require.ErrorIs(t, err, io.EOF)
			require.Equal(t, NotificationFailureConnectivity, ClassifyNotificationFailureError(err))
		})
	}
}

func TestUnclassifiedClosedSMTPConnection(t *testing.T) {
	for _, stage := range []string{"greeting", "authentication"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			require.NoError(t, err)
			var sessions sync.WaitGroup
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					sessions.Add(1)
					go func() {
						defer sessions.Done()
						defer conn.Close()
						if stage == "greeting" {
							return
						}
						_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
						wire := textproto.NewConn(conn)
						if wire.PrintfLine("220 localhost ESMTP") != nil {
							return
						}
						if _, err := wire.ReadLine(); err != nil {
							return
						}
						if wire.PrintfLine("250-localhost\r\n250 AUTH LOGIN") != nil {
							return
						}
						_, _ = wire.ReadLine() // Close before answering AUTH, with no credential verdict.
					}()
				}
			}()
			t.Cleanup(func() { _ = listener.Close(); <-done; sessions.Wait() })
			_, port, err := net.SplitHostPort(listener.Addr().String())
			require.NoError(t, err)
			p, err := strconv.Atoi(port)
			require.NoError(t, err)
			manager := NewEnhancedEmailManager(EmailProviderConfig{EmailConfig: EmailConfig{
				SMTPHost: "127.0.0.1", SMTPPort: p, From: "pulse@example.test", To: []string{"ops@example.test"},
				Username: "synthetic-user", Password: "synthetic-password",
			}})
			err = manager.SendEmailWithRetry("synthetic", "<p>synthetic</p>", "synthetic")
			require.ErrorIs(t, err, io.EOF)
			require.Equal(t, NotificationFailureConnectivity, ClassifyNotificationFailureError(err))
		})
	}
}

func TestUnclassifiedSMTPAuthDisconnectRecoversWithTransportRetry(t *testing.T) {
	config, receiver := smtpAuthenticationReceiver(t, "plain", "LOGIN", []string{"User Name", "Password"}, 235)
	upstreamAddress := net.JoinHostPort(config.SMTPHost, strconv.Itoa(config.SMTPPort))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	var connections atomic.Int32
	var sessions sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			attempt := connections.Add(1)
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				if attempt == 1 {
					wire := textproto.NewConn(conn)
					if wire.PrintfLine("220 localhost ESMTP") != nil {
						return
					}
					if _, err := wire.ReadLine(); err != nil {
						return
					}
					if wire.PrintfLine("250-localhost\r\n250 AUTH LOGIN") != nil {
						return
					}
					_, _ = wire.ReadLine() // First AUTH exchange loses its connection.
					return
				}
				// Later connections reach the real credential-checking receiver
				// behind the same configured endpoint, without replacing SMTP.
				upstream, err := net.DialTimeout("tcp4", upstreamAddress, time.Second)
				if err != nil {
					t.Error(err)
					return
				}
				defer upstream.Close()
				_ = upstream.SetDeadline(time.Now().Add(5 * time.Second))
				copied := make(chan struct{})
				go func() { _, _ = io.Copy(upstream, conn); close(copied) }()
				_, _ = io.Copy(conn, upstream)
				_ = conn.Close()
				_ = upstream.Close()
				<-copied
			}()
		}
	}()
	t.Cleanup(func() { _ = listener.Close(); <-done; sessions.Wait() })
	_, port, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	config.SMTPPort, err = strconv.Atoi(port)
	require.NoError(t, err)
	config.MaxRetries = 1
	require.NoError(t, NewEnhancedEmailManager(config).SendEmailWithRetry("synthetic", "<p>synthetic</p>", "synthetic"))
	require.Equal(t, int32(2), connections.Load())
	require.Equal(t, int32(1), receiver.accepted.Load())
	require.Equal(t, int32(2), receiver.credentials.Load())
}

func TestUnclassifiedPayloadTemplateFailuresAreConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"parse", `{"message":"{{synthetic_private_function}}"}`},
		{"execution", `{"message":"{{.Absent}}"}`},
		{"JSON", `{"message":"{{.Message}}"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			captured := captureAppriseLogs(t)
			_, err := (&NotificationManager{}).generatePayloadFromTemplateWithService(tc.source,
				WebhookPayloadData{Message: "synthetic-private-message\"invalid"}, "generic")
			require.Error(t, err)
			assert.Equal(t, NotificationFailureConfiguration, ClassifyNotificationFailureError(err))
			require.NotContains(t, err.Error(), "synthetic_private_function")
			require.NotContains(t, captured.String(), "synthetic-private-message")
		})
	}
}

func TestUnclassifiedEnhancedNtfyUsesCanonicalTemplateVerdict(t *testing.T) {
	for _, action := range []string{"send", "test"} {
		t.Run(action, func(t *testing.T) {
			manager := &NotificationManager{}
			config := EnhancedWebhookConfig{WebhookConfig: WebhookConfig{URL: "http://127.0.0.1:1"}, Service: "ntfy", PayloadTemplate: `{{.Absent}}`}
			var err error
			if action == "send" {
				err = manager.SendEnhancedWebhook(config, &alerts.Alert{ID: "synthetic", StartTime: time.Now()})
			} else {
				_, _, err = manager.TestEnhancedWebhook(config)
			}
			require.Error(t, err)
			require.Equal(t, NotificationFailureConfiguration, ClassifyNotificationFailureError(err))
			var execution template.ExecError
			require.ErrorAs(t, err, &execution)
			require.NotContains(t, err.Error(), "Absent")
		})
	}
}

func TestUnclassifiedQueuedTemplateStopsWithoutSending(t *testing.T) {
	var requests atomic.Int32
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(204) }))
	defer server.Close()
	dir := t.TempDir()
	manager := NewNotificationManagerWithDeferredQueue("", dir)
	t.Cleanup(manager.Stop)
	require.NoError(t, manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"))
	manager.SetGroupingConfig(false, 0, false, false)
	manager.AddWebhook(WebhookConfig{ID: "synthetic-template", Enabled: true, URL: server.URL, Template: `{"message":"{{.Absent}}"}`})
	manager.SendAlert(&alerts.Alert{ID: "template-failure", ResourceName: "synthetic-node", Level: alerts.AlertLevelWarning, StartTime: time.Now()})
	pending, err := manager.queue.GetPending(10)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	manager.StartQueueProcessing()
	waitVerdictAudit(t, manager.queue, pending[0].ID)
	manager.Stop()
	queue, err := NewNotificationQueue(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = queue.Stop() })
	var status, class, message string
	var attempts int
	require.NoError(t, queue.db.QueryRow(`SELECT status, attempts, failure_class, error_message FROM notification_audit WHERE notification_id = ?`, pending[0].ID).Scan(&status, &attempts, &class, &message))
	require.Equal(t, string(QueueStatusDLQ), status)
	require.Equal(t, 1, attempts)
	require.Equal(t, string(NotificationFailureConfiguration), class)
	require.True(t, strings.Contains(message, "template execution failed"))
	stats, err := queue.GetTelemetryStats(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, stats.Failures)
	require.Equal(t, 1, stats.FailureClasses.Configuration)
	require.Zero(t, stats.FailureClasses.Unknown)
	require.Zero(t, stats.Deliveries)
	require.Zero(t, requests.Load())
}

func TestUnclassifiedRetainedConfigurationStopsWithoutRetry(t *testing.T) {
	for _, kind := range []string{"email", "webhook", "apprise", "unsupported"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			manager := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(manager.Stop)
			config := []byte(`{"synthetic-private-field":`)
			if kind == "unsupported" {
				config = []byte(`{}`)
			}
			notif := &QueuedNotification{ID: "invalid-retained-config", Type: kind, Config: config, MaxAttempts: 3}
			err := manager.ProcessQueuedNotification(notif)
			require.Error(t, err)
			assert.Equal(t, NotificationFailureConfiguration, ClassifyNotificationFailureError(err))
			if kind != "unsupported" {
				var syntax *json.SyntaxError
				require.ErrorAs(t, err, &syntax)
			}
			require.NoError(t, manager.queue.Enqueue(notif))
			manager.StartQueueProcessing()
			waitVerdictAudit(t, manager.queue, notif.ID)
			manager.Stop()
			queue, err := NewNotificationQueue(dir)
			require.NoError(t, err)
			t.Cleanup(func() { _ = queue.Stop() })
			var status, class string
			var attempts int
			require.NoError(t, queue.db.QueryRow(`SELECT status, attempts, failure_class FROM notification_audit WHERE notification_id = ?`, notif.ID).Scan(&status, &attempts, &class))
			require.Equal(t, string(QueueStatusDLQ), status)
			require.Equal(t, 1, attempts)
			require.Equal(t, string(NotificationFailureConfiguration), class)
		})
	}
}
