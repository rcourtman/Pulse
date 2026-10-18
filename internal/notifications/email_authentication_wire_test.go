package notifications

import (
	"crypto/tls"
	"encoding/base64"
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

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/require"
)

type smtpAuthenticationCounts struct {
	connections, attempts, plain, login, credentials, accepted atomic.Int32
}

// This receiver checks the actual authentication exchange before accepting any
// envelope or message. STARTTLS advertises AUTH only after the TLS upgrade.
func smtpAuthenticationReceiver(t *testing.T, mode, mechanisms string, prompts []string, verdict int) (EmailProviderConfig, *smtpAuthenticationCounts) {
	t.Helper()
	certificate := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tlsConfig := certificate.TLS.Clone()
	certificate.Close()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require.NoError(t, err)
	counts := &smtpAuthenticationCounts{}
	var sessions sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			counts.connections.Add(1)
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
				if conn.PrintfLine("220 localhost ESMTP") != nil {
					return
				}
				upgraded, authenticated := mode == "TLS", false
				for {
					line, err := conn.ReadLine()
					if err != nil {
						return
					}
					command, argument, _ := strings.Cut(line, " ")
					switch command {
					case "EHLO", "HELO":
						switch {
						case mode == "STARTTLS" && !upgraded:
							err = conn.PrintfLine("250-localhost\r\n250 STARTTLS")
						case mechanisms != "":
							err = conn.PrintfLine("250-localhost\r\n250 AUTH %s", mechanisms)
						default:
							err = conn.PrintfLine("250 localhost")
						}
					case "STARTTLS":
						if upgraded {
							err = conn.PrintfLine("503 Already using TLS")
							break
						}
						if err = conn.PrintfLine("220 Ready for TLS"); err == nil {
							conn = textproto.NewConn(tls.Server(raw, tlsConfig))
							upgraded = true
						}
					case "AUTH":
						counts.attempts.Add(1)
						mechanism, initial, _ := strings.Cut(argument, " ")
						allowed := mechanisms == "" && mechanism == "PLAIN"
						for _, candidate := range strings.Fields(mechanisms) {
							allowed = allowed || candidate == mechanism
						}
						if !allowed || mode == "STARTTLS" && !upgraded {
							err = conn.PrintfLine("504 Unsupported mechanism")
							break
						}
						valid, cancelled := true, false
						switch mechanism {
						case "PLAIN":
							counts.plain.Add(1)
							response, decodeErr := base64.StdEncoding.DecodeString(initial)
							valid = decodeErr == nil && string(response) == "\x00synthetic-user\x00synthetic-password"
						case "LOGIN":
							counts.login.Add(1)
							for index, prompt := range prompts {
								if conn.PrintfLine("334 %s", base64.StdEncoding.EncodeToString([]byte(prompt))) != nil {
									return
								}
								encoded, readErr := conn.ReadLine()
								if readErr != nil {
									return
								}
								if encoded == "*" {
									cancelled = true
									break
								}
								counts.credentials.Add(1)
								response, decodeErr := base64.StdEncoding.DecodeString(encoded)
								want := "synthetic-user"
								if index == 1 {
									want = "synthetic-password"
								}
								valid = valid && decodeErr == nil && index < 2 && string(response) == want
							}
						default:
							valid = false
						}
						if cancelled {
							err = conn.PrintfLine("501 Authentication cancelled")
						} else if !valid {
							err = conn.PrintfLine("535 Invalid credentials")
						} else {
							authenticated = verdict == 235
							err = conn.PrintfLine("%d Authentication verdict", verdict)
						}
					case "*":
						err = conn.PrintfLine("501 Authentication cancelled")
					case "MAIL", "RCPT":
						if authenticated {
							err = conn.PrintfLine("250 OK")
						} else {
							err = conn.PrintfLine("530 Authentication required")
						}
					case "DATA":
						if !authenticated {
							err = conn.PrintfLine("530 Authentication required")
							break
						}
						if conn.PrintfLine("354 Send message") != nil {
							return
						}
						if _, err = conn.ReadDotBytes(); err != nil {
							return
						}
						counts.accepted.Add(1)
						err = conn.PrintfLine("250 Accepted")
					case "QUIT":
						_ = conn.PrintfLine("221 Bye")
						return
					default:
						t.Errorf("unexpected SMTP command %q", command)
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
	require.NoError(t, err)
	port, err := strconv.Atoi(rawPort)
	require.NoError(t, err)
	return EmailProviderConfig{EmailConfig: EmailConfig{Enabled: true, SMTPHost: "127.0.0.1", SMTPPort: port,
		From: "pulse@example.test", To: []string{"ops@example.test"}, Username: "synthetic-user", Password: "synthetic-password", TLS: mode == "TLS"},
		StartTLS: mode == "STARTTLS", SkipTLSVerify: true, MaxRetries: 2}, counts
}

func TestSMTPAuthenticationAcceptsLoginChallengeVariants(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		for _, test := range []struct {
			name, mechanisms string
			prompts          []string
		}{
			{"classic", "LOGIN", []string{"Username:", "Password:"}},
			{"legacy", "LOGIN", []string{"User Name", "Password"}},
			{"empty", "LOGIN", []string{"", ""}},
			{"exact mechanism", "PLAIN-CLIENTTOKEN LOGIN", []string{"Username:", "Password:"}},
		} {
			for _, action := range []string{"connection test", "delivery"} {
				t.Run(mode+"/"+test.name+"/"+action, func(t *testing.T) {
					config, counts := smtpAuthenticationReceiver(t, mode, test.mechanisms, test.prompts, 235)
					manager := NewEnhancedEmailManager(config)
					if action == "connection test" {
						require.NoError(t, manager.TestConnection())
						require.Zero(t, counts.accepted.Load())
					} else {
						require.NoError(t, manager.SendEmailWithRetry("synthetic authentication test", "<p>test</p>", "test"))
						require.Equal(t, int32(1), counts.accepted.Load())
					}
					require.Equal(t, int32(1), counts.connections.Load())
					require.Equal(t, int32(1), counts.login.Load())
					require.Equal(t, int32(2), counts.credentials.Load())
				})
			}
		}
	}
}

func TestSMTPAuthenticationRejectsUnsupportedMechanismsBeforeAUTH(t *testing.T) {
	for _, mechanisms := range []string{"CRAM-MD5", "XPLAIN", "XLOGIN"} {
		t.Run(mechanisms, func(t *testing.T) {
			config, counts := smtpAuthenticationReceiver(t, "plain", mechanisms, nil, 235)
			err := NewEnhancedEmailManager(config).SendEmailWithRetry("test", "<p>test</p>", "test")
			require.Error(t, err)
			require.Equal(t, NotificationFailureConfiguration, ClassifyNotificationFailureError(err))
			require.Equal(t, int32(1), counts.connections.Load(), "an unsupported mechanism must not spend transport retries")
			require.Zero(t, counts.attempts.Load(), "do not attempt an unadvertised mechanism")
			require.Zero(t, counts.accepted.Load())
		})
	}
}

func TestSMTPAuthenticationKeepsPlainPreferenceAndLegacyProbe(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		for _, mechanisms := range []string{"LOGIN PLAIN", ""} {
			t.Run(mode+"/"+mechanisms, func(t *testing.T) {
				config, counts := smtpAuthenticationReceiver(t, mode, mechanisms, nil, 235)
				require.NoError(t, NewEnhancedEmailManager(config).SendEmailWithRetry("test", "<p>test</p>", "test"))
				require.Equal(t, int32(1), counts.accepted.Load())
				require.Equal(t, int32(1), counts.plain.Load())
				require.Zero(t, counts.login.Load())
			})
		}
	}
}

func TestSMTPAuthenticationRetainsServerVerdicts(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		for _, verdict := range []int{535, 454} {
			t.Run(mode+"/"+strconv.Itoa(verdict), func(t *testing.T) {
				config, counts := smtpAuthenticationReceiver(t, mode, "LOGIN", []string{"User Name", "Password"}, verdict)
				err := NewEnhancedEmailManager(config).SendEmailWithRetry("test", "<p>test</p>", "test")
				require.Error(t, err)
				var reply *textproto.Error
				require.ErrorAs(t, err, &reply)
				require.Equal(t, verdict, reply.Code)
				attempts, class := int32(1), NotificationFailureAuthentication
				if verdict == 454 {
					attempts, class = 3, NotificationFailureServerError
				}
				require.Equal(t, class, ClassifyNotificationFailureError(err))
				require.Equal(t, attempts, counts.connections.Load())
				require.Equal(t, attempts*2, counts.credentials.Load())
				require.Zero(t, counts.accepted.Load())
			})
		}
	}
}

func TestSMTPAuthenticationRejectsMalformedLoginExchange(t *testing.T) {
	for _, prompts := range [][]string{nil, {"Username:"}, {"Username:", "Password:", "receiver-controlled-private-prompt"}} {
		t.Run(strconv.Itoa(len(prompts)), func(t *testing.T) {
			config, counts := smtpAuthenticationReceiver(t, "plain", "LOGIN", prompts, 235)
			err := NewEnhancedEmailManager(config).SendEmailWithRetry("test", "<p>test</p>", "test")
			require.Error(t, err)
			require.Equal(t, NotificationFailureAuthentication, ClassifyNotificationFailureError(err))
			require.NotContains(t, err.Error(), "receiver-controlled-private-prompt")
			require.Equal(t, int32(1), counts.connections.Load())
			require.LessOrEqual(t, counts.credentials.Load(), int32(2))
			require.Zero(t, counts.accepted.Load())
		})
	}
}

func TestSMTPAuthenticationThreadedAndAttachmentDelivery(t *testing.T) {
	for _, mode := range []string{"plain", "TLS", "STARTTLS"} {
		t.Run(mode, func(t *testing.T) {
			config, counts := smtpAuthenticationReceiver(t, mode, "LOGIN", []string{"User Name", "Password"}, 235)
			manager := NewEnhancedEmailManager(config)
			require.NoError(t, manager.SendEmailThreaded("test", "<p>test</p>", "test", "synthetic-thread"))
			require.NoError(t, manager.SendEmailWithAttachments("test", "<p>test</p>", "test", []EmailAttachment{{
				Filename: "synthetic-report.csv", ContentType: "text/csv", Data: []byte("resource,status\nsynthetic,ok\n"),
			}}))
			require.Equal(t, int32(2), counts.accepted.Load())
			require.Equal(t, int32(2), counts.login.Load())
		})
	}
}

func TestSMTPAuthenticationKeepsCertificateVerification(t *testing.T) {
	for _, mode := range []string{"TLS", "STARTTLS"} {
		t.Run(mode, func(t *testing.T) {
			config, counts := smtpAuthenticationReceiver(t, mode, "LOGIN", []string{"User Name", "Password"}, 235)
			config.SkipTLSVerify = false
			err := NewEnhancedEmailManager(config).SendEmailWithRetry("test", "<p>test</p>", "test")
			require.Error(t, err)
			require.Equal(t, NotificationFailureTLS, ClassifyNotificationFailureError(err))
			require.Zero(t, counts.attempts.Load())
			require.Zero(t, counts.accepted.Load())
		})
	}
}

func TestSMTPConnectionTestUsesImplicitTLSForPort465(t *testing.T) {
	config, counts := smtpAuthenticationReceiver(t, "TLS", "LOGIN", []string{"Username:", "Password:"}, 235)
	receiver := net.JoinHostPort(config.SMTPHost, strconv.Itoa(config.SMTPPort))
	original := smtpTLSDialWithDialer
	t.Cleanup(func() { smtpTLSDialWithDialer = original })
	smtpTLSDialWithDialer = func(dialer *net.Dialer, network, address string, policy *tls.Config) (*tls.Conn, error) {
		require.Equal(t, net.JoinHostPort(config.SMTPHost, "465"), address)
		return original(dialer, network, receiver, policy)
	}
	// Map only the destination port to the owned ephemeral receiver. The real
	// TLS handshake, configured policy and SMTP session all remain in use.
	config.SMTPPort, config.TLS, config.StartTLS = 465, false, true
	manager := NewEnhancedEmailManager(config)
	require.NoError(t, manager.TestConnection())
	require.NoError(t, manager.SendEmailWithRetry("test", "<p>test</p>", "test"))
	require.Equal(t, int32(2), counts.login.Load())
	require.Equal(t, int32(1), counts.accepted.Load())
}

func TestQueuedSMTPLoginAuthenticationVerdictSurvivesRestart(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(strconv.FormatBool(valid), func(t *testing.T) {
			config, counts := smtpAuthenticationReceiver(t, "plain", "LOGIN", []string{"User Name", "Password"}, 235)
			if !valid {
				config.Password = "synthetic-invalid-password"
			}
			dir := t.TempDir()
			manager := NewNotificationManagerWithDeferredQueue("", dir)
			t.Cleanup(manager.Stop)
			manager.SetEmailConfig(config.EmailConfig)
			manager.SetGroupingConfig(false, 0, false, false)
			manager.SendAlert(&alerts.Alert{ID: "smtp-login-test", ResourceName: "synthetic-node", Type: "cpu",
				Level: alerts.AlertLevelWarning, Message: "synthetic alert", StartTime: time.Now()})
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
			require.Equal(t, int32(1), counts.connections.Load())
			require.Equal(t, int32(2), counts.credentials.Load(), "the receiver must check both credentials before its verdict")
			if valid {
				require.Equal(t, string(QueueStatusSent), status)
				require.Equal(t, 1, success)
				require.Empty(t, failureClass)
				require.Equal(t, 1, stats.Deliveries)
				require.Zero(t, stats.Failures)
				require.Equal(t, int32(1), counts.accepted.Load())
			} else {
				require.Equal(t, string(QueueStatusDLQ), status)
				require.Zero(t, success)
				require.Equal(t, string(NotificationFailureAuthentication), failureClass)
				require.Zero(t, stats.Deliveries)
				require.Equal(t, 1, stats.Failures)
				require.Equal(t, 1, stats.FailureClasses.Authentication)
				require.Zero(t, counts.accepted.Load())
			}
		})
	}
}
