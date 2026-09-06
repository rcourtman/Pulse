package notifications

import (
	"fmt"
	"net"
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
