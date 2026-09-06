package notifications

import (
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

// Exercise the SMTP sender, not just the classifier: its inner retry loop must
// respect structured replies before the outer queue ever sees the error.
func TestEmailRetryRespectsSMTPFailureClass(t *testing.T) {
	for _, tc := range []struct {
		name         string
		code         int
		message      string
		wantClass    NotificationFailureClass
		wantAttempts int
	}{
		{"rejected despite temporary prose", 550, "temporary timeout", NotificationFailureRejected, 1},
		{"authentication", 535, "credentials invalid", NotificationFailureAuthentication, 1},
		{"transaction rejection", 554, "transaction rejected", NotificationFailureRejected, 1},
		{"configuration", 501, "invalid parameters", NotificationFailureConfiguration, 1},
		{"transient despite authentication prose", 421, "authentication service unavailable", NotificationFailureServerError, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var servers sync.WaitGroup
			originalDial := smtpDialTimeout
			t.Cleanup(func() { smtpDialTimeout = originalDial; servers.Wait() })
			attempts := 0
			smtpDialTimeout = func(string, string, time.Duration) (net.Conn, error) {
				attempts++
				client, server := net.Pipe()
				servers.Add(1)
				go func() {
					defer servers.Done()
					defer server.Close()
					_ = server.SetDeadline(time.Now().Add(5 * time.Second))
					_, _ = fmt.Fprintf(server, "%d %s\r\n", tc.code, tc.message)
				}()
				return client, nil
			}
			manager := NewEnhancedEmailManager(EmailProviderConfig{
				EmailConfig: EmailConfig{SMTPHost: "smtp.example.test", SMTPPort: 25, From: "pulse@example.test", To: []string{"recipient@example.test"}},
				MaxRetries:  2,
			})
			err := manager.SendEmailWithRetry("test", "<p>test</p>", "test")
			if err == nil {
				t.Fatal("expected SMTP failure")
			}
			if got := ClassifyNotificationFailureError(err); got != tc.wantClass {
				t.Errorf("class = %q, want %q", got, tc.wantClass)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("email failed after %d attempts:", tc.wantAttempts)) {
				t.Errorf("incorrect attempt count in error: %v", err)
			}
			if attempts != tc.wantAttempts {
				t.Errorf("SMTP attempts = %d, want %d", attempts, tc.wantAttempts)
			}
		})
	}
}

// A provider may accept the greeting and reject the envelope or the completed
// message. Keep those wrapped replies authoritative too, and ensure a later
// successful attempt ends retries rather than exhausting the budget.
func TestEmailRetryRespectsSMTPTransactionReplies(t *testing.T) {
	for _, stage := range []string{"MAIL", "RCPT", "DATA", "message"} {
		for _, tc := range []struct {
			name         string
			code         int
			recover      bool
			wantAttempts int
		}{
			{"permanent", 550, false, 1},
			{"temporary", 451, false, 3},
			{"recovery", 451, true, 2},
		} {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				var servers sync.WaitGroup
				originalDial := smtpDialTimeout
				t.Cleanup(func() { smtpDialTimeout = originalDial; servers.Wait() })
				attempts := 0
				smtpDialTimeout = func(string, string, time.Duration) (net.Conn, error) {
					attempts++
					fail := !tc.recover || attempts == 1
					client, server := net.Pipe()
					servers.Add(1)
					go func() {
						defer servers.Done()
						defer server.Close()
						_ = server.SetDeadline(time.Now().Add(5 * time.Second))
						conn := textproto.NewConn(server)
						if err := conn.PrintfLine("220 smtp.example.test"); err != nil {
							t.Error(err)
							return
						}
						for {
							line, err := conn.ReadLine()
							if err != nil {
								t.Error(err)
								return
							}
							command := strings.Fields(line)[0]
							if fail && command == stage {
								if err := conn.PrintfLine("%d temporary authentication timeout", tc.code); err != nil {
									t.Error(err)
								}
								return
							}
							switch command {
							case "EHLO", "HELO", "MAIL", "RCPT":
								err = conn.PrintfLine("250 OK")
							case "DATA":
								if err = conn.PrintfLine("354 Send message"); err != nil {
									t.Error(err)
									return
								}
								if _, err = conn.ReadDotBytes(); err != nil {
									t.Error(err)
									return
								}
								if fail && stage == "message" {
									if err := conn.PrintfLine("%d temporary authentication timeout", tc.code); err != nil {
										t.Error(err)
									}
									return
								}
								err = conn.PrintfLine("250 Accepted")
							case "QUIT":
								if err := conn.PrintfLine("221 Bye"); err != nil {
									t.Error(err)
								}
								return
							default:
								t.Errorf("unexpected SMTP command %q", command)
								return
							}
							if err != nil {
								t.Error(err)
								return
							}
						}
					}()
					return client, nil
				}
				manager := NewEnhancedEmailManager(EmailProviderConfig{
					EmailConfig: EmailConfig{SMTPHost: "smtp.example.test", SMTPPort: 25, From: "pulse@example.test", To: []string{"recipient@example.test"}},
					MaxRetries:  2,
				})
				err := manager.SendEmailWithRetry("test", "<p>test</p>", "test")
				if tc.recover {
					if err != nil {
						t.Fatalf("recovery failed: %v", err)
					}
				} else {
					if err == nil {
						t.Fatal("expected SMTP failure")
					}
					wantClass := NotificationFailureRejected
					if tc.code == 451 {
						wantClass = NotificationFailureServerError
					}
					if got := ClassifyNotificationFailureError(err); got != wantClass {
						t.Errorf("class = %q, want %q", got, wantClass)
					}
					if !strings.Contains(err.Error(), fmt.Sprintf("email failed after %d attempts:", tc.wantAttempts)) {
						t.Errorf("incorrect attempt count: %v", err)
					}
				}
				if attempts != tc.wantAttempts {
					t.Errorf("attempts = %d, want %d", attempts, tc.wantAttempts)
				}
				servers.Wait()
			})
		}
	}
}
