package notifications

import (
	"bufio"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func stubSMTPDialSuccess(t *testing.T, deliveries *int32) {
	t.Helper()

	origDial := smtpDialTimeout
	smtpDialTimeout = func(network, addr string, timeout time.Duration) (net.Conn, error) {
		clientConn, serverConn := net.Pipe()

		go func() {
			defer serverConn.Close()

			writer := bufio.NewWriter(serverConn)
			reader := textproto.NewReader(bufio.NewReader(serverConn))

			fmt.Fprint(writer, "220 smtp.example.com ESMTP\r\n")
			_ = writer.Flush()

			for {
				line, err := reader.ReadLine()
				if err != nil {
					return
				}

				switch {
				case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
					fmt.Fprint(writer, "250-smtp.example.com\r\n250 AUTH PLAIN LOGIN\r\n")
				case strings.HasPrefix(line, "AUTH"):
					fmt.Fprint(writer, "235 2.7.0 Authentication successful\r\n")
				case strings.HasPrefix(line, "MAIL FROM:"):
					fmt.Fprint(writer, "250 2.1.0 OK\r\n")
				case strings.HasPrefix(line, "RCPT TO:"):
					fmt.Fprint(writer, "250 2.1.5 OK\r\n")
				case strings.HasPrefix(line, "DATA"):
					fmt.Fprint(writer, "354 End data with <CR><LF>.<CR><LF>\r\n")
					_ = writer.Flush()

					for {
						dataLine, readErr := reader.ReadLine()
						if readErr != nil {
							return
						}
						if dataLine == "." {
							break
						}
					}
					if deliveries != nil {
						atomic.AddInt32(deliveries, 1)
					}
					fmt.Fprint(writer, "250 2.0.0 queued\r\n")
				case strings.HasPrefix(line, "QUIT"):
					fmt.Fprint(writer, "221 2.0.0 Bye\r\n")
					_ = writer.Flush()
					return
				default:
					fmt.Fprint(writer, "250 OK\r\n")
				}

				_ = writer.Flush()
			}
		}()

		return clientConn, nil
	}

	t.Cleanup(func() {
		smtpDialTimeout = origDial
	})
}

func newSMTPClientWithEHLO(t *testing.T, ehloLines []string) *smtp.Client {
	t.Helper()

	clientConn, serverConn := net.Pipe()

	go func() {
		defer serverConn.Close()

		writer := bufio.NewWriter(serverConn)
		reader := textproto.NewReader(bufio.NewReader(serverConn))

		fmt.Fprint(writer, "220 smtp.example.com ESMTP\r\n")
		_ = writer.Flush()

		for {
			line, err := reader.ReadLine()
			if err != nil {
				return
			}

			switch {
			case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
				for _, response := range ehloLines {
					fmt.Fprintf(writer, "%s\r\n", response)
				}
				_ = writer.Flush()
			case strings.HasPrefix(line, "QUIT"):
				fmt.Fprint(writer, "221 2.0.0 Bye\r\n")
				_ = writer.Flush()
				return
			default:
				fmt.Fprint(writer, "250 OK\r\n")
				_ = writer.Flush()
			}
		}
	}()

	client, err := smtp.NewClient(clientConn, "smtp.example.com")
	if err != nil {
		t.Fatalf("failed to create SMTP client: %v", err)
	}

	t.Cleanup(func() {
		_ = client.Close()
	})

	return client
}

func TestLoginAuth_StartAndNext(t *testing.T) {
	auth := LoginAuth("user-a", "pass-a")
	login, ok := auth.(*loginAuth)
	if !ok {
		t.Fatalf("expected *loginAuth, got %T", auth)
	}

	mech, initialResp, err := login.Start(&smtp.ServerInfo{Name: "smtp.example.com"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if mech != "LOGIN" {
		t.Fatalf("expected LOGIN mechanism, got %q", mech)
	}
	if initialResp != nil {
		t.Fatalf("expected nil initial response, got %q", string(initialResp))
	}

	usernameResp, err := login.Next([]byte("Username:"), true)
	if err != nil {
		t.Fatalf("Next username prompt returned error: %v", err)
	}
	if string(usernameResp) != "user-a" {
		t.Fatalf("expected username response user-a, got %q", string(usernameResp))
	}

	passwordResp, err := login.Next([]byte("Password:"), true)
	if err != nil {
		t.Fatalf("Next password prompt returned error: %v", err)
	}
	if string(passwordResp) != "pass-a" {
		t.Fatalf("expected password response pass-a, got %q", string(passwordResp))
	}

	_, err = login.Next([]byte("Token:"), true)
	if err == nil {
		t.Fatal("expected error for unexpected LOGIN prompt")
	}

	finalResp, err := login.Next(nil, false)
	if err != nil {
		t.Fatalf("Next with more=false returned error: %v", err)
	}
	if finalResp != nil {
		t.Fatalf("expected nil final response, got %q", string(finalResp))
	}
}

func TestPlainAuth_StartAndNext(t *testing.T) {
	auth := &plainAuth{
		identity: "",
		username: "plain-user",
		password: "plain-pass",
	}

	mech, resp, err := auth.Start(&smtp.ServerInfo{Name: "smtp.example.com"})
	if err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if mech != "PLAIN" {
		t.Fatalf("expected PLAIN mechanism, got %q", mech)
	}
	if string(resp) != "\x00plain-user\x00plain-pass" {
		t.Fatalf("unexpected PLAIN payload: %q", string(resp))
	}

	finalResp, err := auth.Next(nil, false)
	if err != nil {
		t.Fatalf("Next with more=false returned error: %v", err)
	}
	if finalResp != nil {
		t.Fatalf("expected nil final response, got %q", string(finalResp))
	}

	_, err = auth.Next([]byte("challenge"), true)
	if err == nil {
		t.Fatal("expected challenge error when more=true")
	}
}

func TestNegotiateAuth_NoAuthConfigured(t *testing.T) {
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		EmailConfig: EmailConfig{
			Username: "",
			Password: "",
		},
		AuthRequired: false,
	})

	auth, err := manager.negotiateAuth(nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if auth != nil {
		t.Fatalf("expected nil auth when not configured, got %T", auth)
	}
}

func TestNegotiateAuth_ServerMechanisms(t *testing.T) {
	tests := []struct {
		name      string
		ehloLines []string
		wantType  string
		wantErr   string
	}{
		{
			name:      "prefers plain when advertised",
			ehloLines: []string{"250-smtp.example.com", "250-AUTH PLAIN LOGIN", "250 SIZE 35882577"},
			wantType:  "plain",
		},
		{
			name:      "falls back to login when plain unavailable",
			ehloLines: []string{"250-smtp.example.com", "250-AUTH LOGIN", "250 SIZE 35882577"},
			wantType:  "login",
		},
		{
			name:      "defaults to plain when auth not advertised",
			ehloLines: []string{"250-smtp.example.com", "250 SIZE 35882577"},
			wantType:  "plain",
		},
		{
			name:      "errors when unsupported mechanisms only",
			ehloLines: []string{"250-smtp.example.com", "250-AUTH CRAM-MD5", "250 SIZE 35882577"},
			wantErr:   "none are supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newSMTPClientWithEHLO(t, tt.ehloLines)
			defer func() {
				_ = client.Quit()
			}()

			manager := NewEnhancedEmailManager(EmailProviderConfig{
				EmailConfig: EmailConfig{
					Username: "auth-user",
					Password: "auth-pass",
				},
				AuthRequired: true,
			})

			auth, err := manager.negotiateAuth(client)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			switch tt.wantType {
			case "plain":
				if _, ok := auth.(*plainAuth); !ok {
					t.Fatalf("expected *plainAuth, got %T", auth)
				}
			case "login":
				if _, ok := auth.(*loginAuth); !ok {
					t.Fatalf("expected *loginAuth, got %T", auth)
				}
			default:
				t.Fatalf("unknown expected auth type %q", tt.wantType)
			}
		})
	}
}

func TestSendGroupedEmail_UsesFromAsRecipient(t *testing.T) {
	var deliveries int32
	stubSMTPDialSuccess(t, &deliveries)

	manager := NewEnhancedEmailManager(EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "smtp.example.com",
			SMTPPort: 25,
		},
		MaxRetries: 0,
		RetryDelay: 0,
		RateLimit:  0,
	})

	nm := &NotificationManager{
		emailManager: manager,
	}

	config := EmailConfig{
		From: "sender@example.com",
		To:   nil,
	}

	alertList := []*alerts.Alert{
		{
			ID:           "alert-1",
			ResourceName: "node-1",
			Level:        alerts.AlertLevelWarning,
			Message:      "CPU utilization high",
			StartTime:    time.Now().Add(-5 * time.Minute),
		},
	}

	if err := nm.sendGroupedEmail(config, alertList); err != nil {
		t.Fatalf("expected grouped email send to succeed, got %v", err)
	}

	if atomic.LoadInt32(&deliveries) != 1 {
		t.Fatalf("expected 1 delivered email, got %d", deliveries)
	}

	expectedTo := []string{"sender@example.com"}
	if !reflect.DeepEqual(manager.config.EmailConfig.To, expectedTo) {
		t.Fatalf("expected manager recipients %v, got %v", expectedTo, manager.config.EmailConfig.To)
	}
}

func TestSendEmail_LegacyPathDeliversMessage(t *testing.T) {
	var deliveries int32
	stubSMTPDialSuccess(t, &deliveries)

	nm := &NotificationManager{
		emailConfig: EmailConfig{
			From:     "sender@example.com",
			To:       []string{"recipient@example.com"},
			SMTPHost: "smtp.example.com",
			SMTPPort: 25,
		},
	}

	alert := &alerts.Alert{
		ID:           "alert-legacy-email",
		ResourceName: "vm-legacy",
		Level:        alerts.AlertLevelCritical,
		Message:      "Disk usage critical",
		StartTime:    time.Now().Add(-10 * time.Minute),
	}

	nm.sendEmail(alert)

	if atomic.LoadInt32(&deliveries) != 1 {
		t.Fatalf("expected legacy sendEmail path to deliver 1 message, got %d", deliveries)
	}
}
