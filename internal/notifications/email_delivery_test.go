package notifications

import (
	"errors"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEmailDeliveryConfigurationCheck(t *testing.T) {
	valid := EmailProviderConfig{EmailConfig: EmailConfig{
		SMTPHost: "smtp.example.test", SMTPPort: 25, From: "pulse@example.test", To: []string{"ops@example.test"},
	}}
	for _, tc := range []struct {
		name   string
		change func(*EmailProviderConfig)
	}{
		{"host", func(c *EmailProviderConfig) { c.SMTPHost = "" }},
		{"port", func(c *EmailProviderConfig) { c.SMTPPort = 65536 }},
		{"sender", func(c *EmailProviderConfig) { c.From = "invalid" }},
		{"recipient", func(c *EmailProviderConfig) { c.To = []string{"invalid"} }},
		{"no recipient", func(c *EmailProviderConfig) { c.To = nil }},
		{"partial credentials", func(c *EmailProviderConfig) { c.Username = "user" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.change(&cfg)
			err := NewEnhancedEmailManager(cfg).ValidateConfiguration()
			require.Error(t, err)
			require.Equal(t, NotificationFailureConfiguration, ClassifyNotificationFailureError(err))
		})
	}
	require.NoError(t, NewEnhancedEmailManager(valid).ValidateConfiguration())
	valid.Provider = "SendGrid"
	valid.Password = "test-provider-credential"
	valid.AuthRequired = true
	require.NoError(t, NewEnhancedEmailManager(valid).ValidateConfiguration())
}

func TestEmailDoesNotRepeatUnconfirmedCompletedMessage(t *testing.T) {
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
			conn := textproto.NewConn(server)
			if conn.PrintfLine("220 smtp.example.test") != nil {
				return
			}
			for {
				line, err := conn.ReadLine()
				if err != nil {
					return
				}
				switch strings.Fields(line)[0] {
				case "EHLO", "HELO", "MAIL", "RCPT":
					if conn.PrintfLine("250 OK") != nil {
						return
					}
				case "DATA":
					if conn.PrintfLine("354 Send message") != nil {
						return
					}
					_, _ = conn.ReadDotBytes()
					// The server has the message but drops before its final verdict.
					return
				default:
					t.Errorf("unexpected command %q", line)
					return
				}
			}
		}()
		return client, nil
	}
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		EmailConfig: EmailConfig{SMTPHost: "smtp.example.test", SMTPPort: 25, From: "pulse@example.test", To: []string{"ops@example.test"}},
		MaxRetries:  2,
	})
	err := manager.SendEmailWithAttachments("test", "<p>test</p>", "test", []EmailAttachment{{Filename: "report.csv", Data: []byte("cpu\n42")}})
	var unconfirmed *EmailAcceptanceUnconfirmedError
	require.True(t, errors.As(err, &unconfirmed), "expected an explicit uncertain acceptance result, got %v", err)
	require.Equal(t, 1, attempts, "repeating this message could create duplicates")
}
