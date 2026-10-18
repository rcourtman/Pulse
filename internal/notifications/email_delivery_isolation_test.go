package notifications

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

type isolatedSMTPReceipt struct {
	from       string
	recipients []string
}

func awaitIsolatedSMTPReceipt(t *testing.T, receipts <-chan isolatedSMTPReceipt) isolatedSMTPReceipt {
	t.Helper()
	select {
	case receipt := <-receipts:
		return receipt
	case <-time.After(5 * time.Second):
		t.Fatal("missing accepted SMTP envelope")
		return isolatedSMTPReceipt{}
	}
}

// Only synthetic mail reaches this loopback receiver. It requires the exact
// AUTH PLAIN exchange when credentials are supplied, and records envelopes
// only after accepting DATA so configuration isolation is tested on the wire.
func isolatedSMTPReceiver(t *testing.T, username, password string) (EmailConfig, <-chan isolatedSMTPReceipt) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	receipts := make(chan isolatedSMTPReceipt, 16)
	done := make(chan struct{})
	var sessions sync.WaitGroup
	go func() {
		defer close(done)
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				defer raw.Close()
				_ = raw.SetDeadline(time.Now().Add(5 * time.Second))
				conn := textproto.NewConn(raw)
				if conn.PrintfLine("220 localhost ESMTP") != nil {
					return
				}
				authenticated := username == ""
				var receipt isolatedSMTPReceipt
				for {
					line, err := conn.ReadLine()
					if err != nil {
						return
					}
					command, argument, _ := strings.Cut(line, " ")
					switch command {
					case "EHLO", "HELO":
						err = conn.PrintfLine("250-localhost\r\n250 AUTH PLAIN")
					case "AUTH":
						want := "PLAIN " + base64.StdEncoding.EncodeToString([]byte("\x00"+username+"\x00"+password))
						if argument == want {
							authenticated = true
							err = conn.PrintfLine("235 Authenticated")
						} else {
							err = conn.PrintfLine("535 Invalid credentials")
						}
					case "MAIL":
						if !authenticated {
							err = conn.PrintfLine("530 Authentication required")
						} else {
							receipt = isolatedSMTPReceipt{from: argument}
							err = conn.PrintfLine("250 OK")
						}
					case "RCPT":
						receipt.recipients = append(receipt.recipients, argument)
						err = conn.PrintfLine("250 OK")
					case "DATA":
						if conn.PrintfLine("354 Send message") != nil {
							return
						}
						if _, err = conn.ReadDotBytes(); err != nil {
							return
						}
						if err = conn.PrintfLine("250 Accepted"); err == nil {
							receipts <- receipt
						}
					case "QUIT":
						_ = conn.PrintfLine("221 Bye")
						return
					default:
						err = conn.PrintfLine("500 Unsupported command")
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
	require.NoError(t, err)
	port, err := strconv.Atoi(rawPort)
	require.NoError(t, err)
	return EmailConfig{Enabled: true, SMTPHost: "127.0.0.1", SMTPPort: port,
		From: "pulse@example.test", To: []string{"ops@example.test"}, RateLimit: 100}, receipts
}

func TestEmailDeliveryAuthenticatesProviderDefaultUsername(t *testing.T) {
	const password = "synthetic-provider-key"
	for _, tc := range []struct{ provider, username string }{
		{"SendGrid", "apikey"}, {"Postmark", password},
		{"SparkPost", "SMTP_Injection"}, {"Resend", "resend"},
	} {
		for _, saved := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/saved=%t", tc.provider, saved), func(t *testing.T) {
				cfg, receipts := isolatedSMTPReceiver(t, tc.username, password)
				cfg.Provider, cfg.Password = tc.provider, password
				var delivery *EnhancedEmailManager
				if saved {
					manager := NewNotificationManagerWithDataDir("", t.TempDir())
					t.Cleanup(manager.Stop)
					manager.SetEmailConfig(cfg)
					delivery, _ = manager.emailDeliveryManager(cfg)
				} else {
					delivery = newEmailDeliveryManager(cfg, cfg.To)
				}
				require.NoError(t, delivery.ValidateConfiguration())
				require.NoError(t, delivery.SendEmailWithRetry("synthetic test", "<p>test</p>", "test"))
				require.Len(t, receipts, 1, "valid provider credentials must produce an accepted message")
				require.Empty(t, cfg.Username, "provider resolution must not mutate saved input")
			})
		}
	}
}

func TestEmailDeliveryKeepsIndependentEnvelopeSnapshots(t *testing.T) {
	cfg, receipts := isolatedSMTPReceiver(t, "", "")
	manager := NewNotificationManagerWithDataDir("", t.TempDir())
	t.Cleanup(manager.Stop)
	manager.SetEmailConfig(cfg)
	saved := manager.GetEmailConfig()
	firstConfig, secondConfig := copyEmailConfig(cfg), copyEmailConfig(cfg)
	firstConfig.From, firstConfig.To = "first@example.test", []string{"first-ops@example.test"}
	secondConfig.From, secondConfig.To = "second@example.test", []string{"second-ops@example.test"}
	first, _ := manager.emailDeliveryManager(firstConfig)
	second, _ := manager.emailDeliveryManager(secondConfig)
	// Both configurations are acquired before either is sent, as can happen
	// with independent queue workers or a scheduled report and an alert.
	require.NoError(t, first.SendEmailWithRetry("first", "<p>first</p>", "first"))
	require.NoError(t, second.SendEmailWithRetry("second", "<p>second</p>", "second"))
	require.Len(t, receipts, 2)
	require.Equal(t, isolatedSMTPReceipt{"FROM:<first@example.test>", []string{"TO:<first-ops@example.test>"}}, <-receipts)
	require.Equal(t, isolatedSMTPReceipt{"FROM:<second@example.test>", []string{"TO:<second-ops@example.test>"}}, <-receipts)
	require.Equal(t, saved, manager.GetEmailConfig())
}

func TestEmailDeliverySnapshotsShareRateLimit(t *testing.T) {
	cfg, receipts := isolatedSMTPReceiver(t, "", "")
	cfg.RateLimit = 1
	manager := NewNotificationManagerWithDataDir("", t.TempDir())
	t.Cleanup(manager.Stop)
	manager.SetEmailConfig(cfg)
	first, _ := manager.emailDeliveryManager(cfg)
	second, _ := manager.emailDeliveryManager(cfg)
	// Avoid sleeping through transport retries for a deterministic local limit.
	first.config.MaxRetries, second.config.MaxRetries = 0, 0
	require.NoError(t, first.SendEmailWithRetry("first", "<p>test</p>", "test"))
	err := second.SendEmailWithRetry("second", "<p>test</p>", "test")
	require.Error(t, err)
	require.Equal(t, NotificationFailureRateLimited, ClassifyNotificationFailureError(err))
	require.Len(t, receipts, 1, "isolating a delivery must not reset the destination's rate budget")
}

func TestEmailDeliveryConcurrentEnvelopes(t *testing.T) {
	cfg, receipts := isolatedSMTPReceiver(t, "", "")
	manager := NewNotificationManagerWithDataDir("", t.TempDir())
	t.Cleanup(manager.Stop)
	manager.SetEmailConfig(cfg)
	const workers = 8
	saved := manager.GetEmailConfig()
	start := make(chan struct{})
	results := make(chan error, workers)
	for index := 0; index < workers; index++ {
		go func(index int) {
			own := copyEmailConfig(cfg)
			own.From = fmt.Sprintf("sender-%d@example.test", index)
			own.To = []string{fmt.Sprintf("recipient-%d@example.test", index)}
			<-start
			results <- manager.sendHTMLEmailWithError("synthetic test", "<p>test</p>", "test", own)
		}(index)
	}
	close(start)
	for index := 0; index < workers; index++ {
		require.NoError(t, <-results)
	}
	require.Len(t, receipts, workers)
	seen := make(map[string][]string)
	for index := 0; index < workers; index++ {
		receipt := <-receipts
		seen[receipt.from] = receipt.recipients
	}
	for index := 0; index < workers; index++ {
		require.Equal(t, []string{fmt.Sprintf("TO:<recipient-%d@example.test>", index)},
			seen[fmt.Sprintf("FROM:<sender-%d@example.test>", index)])
	}
	require.Equal(t, saved, manager.GetEmailConfig())
}

func TestQueuedEmailProviderAuthenticationVerdictSurvivesRestart(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprintf("valid=%t", valid), func(t *testing.T) {
			cfg, receipts := isolatedSMTPReceiver(t, "apikey", "synthetic-provider-key")
			cfg.Provider, cfg.Password = "SendGrid", "synthetic-provider-key"
			if !valid {
				cfg.Password = "synthetic-invalid-key"
			}
			dir := t.TempDir()
			manager := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(manager.Stop)
			manager.SetEmailConfig(cfg)
			manager.SetGroupingConfig(false, 0, false, false)
			manager.SendAlert(&alerts.Alert{ID: "smtp-auth-test", ResourceName: "synthetic-node",
				Type: "cpu", Level: alerts.AlertLevelWarning, Message: "synthetic alert", StartTime: time.Now()})
			pending, err := manager.queue.GetPending(10)
			require.NoError(t, err)
			require.Len(t, pending, 1)
			id := pending[0].ID
			manager.StartQueueProcessing()
			waitVerdictAudit(t, manager.queue, id)
			manager.Stop()
			reopened := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(reopened.Stop)
			var status, failureClass string
			var attempts, success int
			require.NoError(t, reopened.queue.db.QueryRow(`SELECT status, attempts FROM notification_queue WHERE id = ?`, id).Scan(&status, &attempts))
			require.NoError(t, reopened.queue.db.QueryRow(`SELECT success, failure_class FROM notification_audit WHERE notification_id = ?`, id).Scan(&success, &failureClass))
			stats, err := reopened.GetTelemetryStats(time.Now().Add(-time.Hour))
			require.NoError(t, err)
			require.Equal(t, 1, attempts)
			require.Equal(t, 1, stats.Attempts)
			if valid {
				require.Equal(t, string(QueueStatusSent), status)
				require.Equal(t, 1, success)
				require.Empty(t, failureClass)
				require.Equal(t, 1, stats.Deliveries)
				require.Zero(t, stats.Failures)
				require.Len(t, receipts, 1)
				require.True(t, reopened.DeliveryHealth().Healthy)
			} else {
				require.Equal(t, string(QueueStatusDLQ), status)
				require.Zero(t, success)
				require.Equal(t, string(NotificationFailureAuthentication), failureClass)
				require.Zero(t, stats.Deliveries)
				require.Equal(t, 1, stats.Failures)
				require.Equal(t, 1, stats.FailureClasses.Authentication)
				require.Empty(t, receipts)
				require.False(t, reopened.DeliveryHealth().Healthy)
			}
		})
	}
}
