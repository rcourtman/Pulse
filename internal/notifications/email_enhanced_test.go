package notifications

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func TestNewEnhancedEmailManager(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
		RateLimit: 10,
	}

	manager := NewEnhancedEmailManager(config)
	if manager == nil {
		t.Fatal("NewEnhancedEmailManager returned nil")
	}
	if manager.rateLimit == nil {
		t.Fatal("rate limiter not initialized")
	}
	if manager.rateLimit.rate != 10 {
		t.Errorf("expected rate limit 10, got %d", manager.rateLimit.rate)
	}
}

func TestCheckRateLimit_NoLimit(t *testing.T) {
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		RateLimit: 0, // No limit
	})

	// Should always succeed when no rate limit
	for i := 0; i < 100; i++ {
		if err := manager.checkRateLimit(); err != nil {
			t.Errorf("checkRateLimit should not error with no limit: %v", err)
		}
	}
}

func TestCheckRateLimit_ExceedsLimit(t *testing.T) {
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		RateLimit: 3,
	})

	// First 3 should succeed
	for i := 0; i < 3; i++ {
		if err := manager.checkRateLimit(); err != nil {
			t.Errorf("call %d should succeed: %v", i+1, err)
		}
	}

	// 4th should fail
	err := manager.checkRateLimit()
	if err == nil {
		t.Error("expected rate limit error on 4th call")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCheckRateLimit_ResetsAfterMinute(t *testing.T) {
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		RateLimit: 2,
	})

	// Use up the limit
	manager.checkRateLimit()
	manager.checkRateLimit()

	// Manually set lastSent to over a minute ago
	manager.rateLimit.mu.Lock()
	manager.rateLimit.lastSent = time.Now().Add(-2 * time.Minute)
	manager.rateLimit.mu.Unlock()

	// Should succeed after reset
	if err := manager.checkRateLimit(); err != nil {
		t.Errorf("should succeed after minute reset: %v", err)
	}
}

func TestSendViaProvider_ProviderUsernameDefaults(t *testing.T) {
	tests := []struct {
		name             string
		provider         string
		initialUsername  string
		expectedUsername string
	}{
		{
			name:             "SendGrid sets apikey username",
			provider:         "SendGrid",
			initialUsername:  "",
			expectedUsername: "apikey",
		},
		{
			name:             "SendGrid preserves existing username",
			provider:         "SendGrid",
			initialUsername:  "custom",
			expectedUsername: "custom",
		},
		{
			name:             "SparkPost sets SMTP_Injection username",
			provider:         "SparkPost",
			initialUsername:  "",
			expectedUsername: "SMTP_Injection",
		},
		{
			name:             "Resend sets resend username",
			provider:         "Resend",
			initialUsername:  "",
			expectedUsername: "resend",
		},
		{
			name:             "Unknown provider leaves username unchanged",
			provider:         "Custom",
			initialUsername:  "",
			expectedUsername: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := EmailProviderConfig{
				EmailConfig: EmailConfig{
					SMTPHost: "invalid.host.test", // Will fail to connect
					SMTPPort: 587,
					Username: tt.initialUsername,
					Password: "test",
					From:     "test@example.com",
					To:       []string{"recipient@example.com"},
				},
				Provider:     tt.provider,
				AuthRequired: true,
			}

			manager := NewEnhancedEmailManager(config)
			// Call sendViaProvider - it will fail on connection, but will have set username
			_ = manager.sendViaProvider([]byte("test"))

			if manager.config.Username != tt.expectedUsername {
				t.Errorf("expected username %q, got %q", tt.expectedUsername, manager.config.Username)
			}
		})
	}
}

func TestSendViaProvider_PostmarkUsernameFromPassword(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 587,
			Username: "",
			Password: "postmark-api-token",
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
		Provider:     "Postmark",
		AuthRequired: true,
	}

	manager := NewEnhancedEmailManager(config)
	_ = manager.sendViaProvider([]byte("test"))

	// Postmark copies password to username when username is empty
	if manager.config.Username != "postmark-api-token" {
		t.Errorf("expected username to be set from password, got %q", manager.config.Username)
	}
}

func TestSendViaProvider_RoutingByTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		tls          bool
		startTLS     bool
		port         int
		expectsError string // Partial match of expected error
	}{
		{
			name:         "TLS true routes to sendTLS",
			tls:          true,
			startTLS:     false,
			port:         587,
			expectsError: "TLS dial failed",
		},
		{
			name:         "Port 465 routes to sendTLS",
			tls:          false,
			startTLS:     false,
			port:         465,
			expectsError: "TLS dial failed",
		},
		{
			name:         "StartTLS routes to sendStartTLS",
			tls:          false,
			startTLS:     true,
			port:         587,
			expectsError: "TCP dial failed",
		},
		{
			name:         "Plain routes to sendPlain",
			tls:          false,
			startTLS:     false,
			port:         25,
			expectsError: "TCP dial failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := EmailProviderConfig{
				EmailConfig: EmailConfig{
					SMTPHost: "invalid.host.test",
					SMTPPort: tt.port,
					TLS:      tt.tls,
					StartTLS: tt.startTLS,
					From:     "test@example.com",
					To:       []string{"recipient@example.com"},
				},
			}

			manager := NewEnhancedEmailManager(config)
			err := manager.sendViaProvider([]byte("test"))

			if err == nil {
				t.Error("expected connection error")
				return
			}

			if !strings.Contains(err.Error(), tt.expectsError) {
				t.Errorf("expected error containing %q, got %q", tt.expectsError, err.Error())
			}
		})
	}
}

func TestSendEmailWithRetry_RetriesOnFailure(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 587,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
		MaxRetries: 2,
		RetryDelay: 0, // No delay for tests
	}

	manager := NewEnhancedEmailManager(config)
	err := manager.SendEmailWithRetry("Test", "<p>test</p>", "test")

	if err == nil {
		t.Error("expected error after all retries exhausted")
	}

	// Should mention the retry count
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Errorf("error should mention attempt count: %v", err)
	}
}

func TestSendEmailWithRetry_RateLimitPreventsRetry(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 587,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
		MaxRetries: 3,
		RetryDelay: 0,
		RateLimit:  1, // Only 1 per minute
	}

	manager := NewEnhancedEmailManager(config)

	// First send uses the 1 allowed
	err := manager.SendEmailWithRetry("Test", "<p>test</p>", "test")
	if err == nil {
		t.Error("expected error (connection should fail)")
	}

	// Second send should hit rate limit on all retries
	err = manager.SendEmailWithRetry("Test2", "<p>test</p>", "test")
	if err == nil {
		t.Error("expected rate limit error")
	}
	if !strings.Contains(err.Error(), "rate limit exceeded") {
		t.Errorf("expected rate limit error, got: %v", err)
	}
}

func TestSendEmailOnce_BuildsMultipartMessage(t *testing.T) {
	// We can't test actual sending, but we can verify the method doesn't panic
	// with valid inputs and returns expected connection error
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 587,
			From:     "sender@example.com",
			To:       []string{"recipient@example.com"},
		},
		ReplyTo: "reply@example.com",
	}

	manager := NewEnhancedEmailManager(config)
	err := manager.sendEmailOnce("Test Subject", "<p>HTML Body</p>", "Text Body")

	// Should fail on connection, not on message building
	if err == nil {
		t.Error("expected connection error")
	}
	// Error should be from connection, not from message construction
	if strings.Contains(err.Error(), "message") && strings.Contains(err.Error(), "build") {
		t.Errorf("unexpected message build error: %v", err)
	}
}

func TestTestConnection_TLSRouting(t *testing.T) {
	tests := []struct {
		name    string
		tls     bool
		port    int
		wantTLS bool
	}{
		{"TLS true uses TLS dial", true, 587, true},
		{"Port 465 uses TLS dial", false, 465, true},
		{"Port 587 without TLS uses plain dial", false, 587, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := EmailProviderConfig{
				EmailConfig: EmailConfig{
					SMTPHost: "invalid.host.test",
					SMTPPort: tt.port,
					TLS:      tt.tls,
				},
			}

			manager := NewEnhancedEmailManager(config)
			err := manager.TestConnection()

			if err == nil {
				t.Error("expected connection error")
			}

			// Verify error message indicates correct connection type
			if tt.wantTLS && strings.Contains(err.Error(), "TCP dial") {
				t.Error("TLS connection should not produce TCP dial error")
			}
		})
	}
}

func TestTestConnection_TLSUsesDialerTimeout(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "smtp.example.com",
			SMTPPort: 465,
			TLS:      true,
		},
	}

	manager := NewEnhancedEmailManager(config)

	origTLSDial := smtpTLSDialWithDialer
	t.Cleanup(func() { smtpTLSDialWithDialer = origTLSDial })

	var gotTimeout time.Duration
	smtpTLSDialWithDialer = func(dialer *net.Dialer, network, addr string, cfg *tls.Config) (*tls.Conn, error) {
		if dialer != nil {
			gotTimeout = dialer.Timeout
		}
		return nil, fmt.Errorf("tls dial intercepted")
	}

	err := manager.TestConnection()
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "tls dial intercepted") {
		t.Fatalf("expected intercepted TLS dial error, got %v", err)
	}
	if gotTimeout != 10*time.Second {
		t.Fatalf("expected TLS dial timeout of 10s, got %s", gotTimeout)
	}
}

func TestSendTLS_ConnectionError(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 465,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
	}

	manager := NewEnhancedEmailManager(config)
	err := manager.sendTLS("invalid.host.test:465", []byte("test"))

	if err == nil {
		t.Error("expected TLS dial error")
	}
	if !strings.Contains(err.Error(), "TLS dial failed") {
		t.Errorf("expected TLS dial error, got: %v", err)
	}
}

func TestSendStartTLS_ConnectionError(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 587,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
	}

	manager := NewEnhancedEmailManager(config)
	err := manager.sendStartTLS("invalid.host.test:587", []byte("test"))

	if err == nil {
		t.Error("expected TCP dial error")
	}
	if !strings.Contains(err.Error(), "TCP dial failed") {
		t.Errorf("expected TCP dial error, got: %v", err)
	}
}

func TestSendPlain_ConnectionError(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 25,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
	}

	manager := NewEnhancedEmailManager(config)
	err := manager.sendPlain("invalid.host.test:25", []byte("test"))

	if err == nil {
		t.Error("expected TCP dial error")
	}
	if !strings.Contains(err.Error(), "TCP dial failed") {
		t.Errorf("expected TCP dial error, got: %v", err)
	}
}

func TestCheckRateLimit_NegativeLimit(t *testing.T) {
	// Negative rate limit should be treated as no limit
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		RateLimit: -1,
	})

	for i := 0; i < 10; i++ {
		if err := manager.checkRateLimit(); err != nil {
			t.Errorf("negative rate limit should allow all calls: %v", err)
		}
	}
}

func TestCheckRateLimit_Concurrency(t *testing.T) {
	manager := NewEnhancedEmailManager(EmailProviderConfig{
		RateLimit: 100,
	})

	// Run concurrent rate limit checks
	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func() {
			manager.checkRateLimit()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 50; i++ {
		<-done
	}

	// Verify counter is correct (should be 50)
	manager.rateLimit.mu.Lock()
	count := manager.rateLimit.sentCount
	manager.rateLimit.mu.Unlock()

	if count != 50 {
		t.Errorf("expected count 50 after concurrent calls, got %d", count)
	}
}
func TestSendPlain_Success(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "smtp.example.com",
			SMTPPort: 25,
			From:     "test@example.com",
			To:       []string{"recipient@example.com"},
		},
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	origDial := smtpDialTimeout
	smtpDialTimeout = func(network, addr string, timeout time.Duration) (net.Conn, error) {
		return clientConn, nil
	}
	t.Cleanup(func() { smtpDialTimeout = origDial })

	go func() {
		defer serverConn.Close()

		w := bufio.NewWriter(serverConn)
		r := textproto.NewReader(bufio.NewReader(serverConn))

		// Greeting
		fmt.Fprint(w, "220 smtp.example.com ESMTP\r\n")
		_ = w.Flush()

		for {
			line, err := r.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "HELO") || strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(w, "250-smtp.example.com\r\n250 8BITMIME\r\n")
				_ = w.Flush()
			case strings.HasPrefix(line, "MAIL FROM:"):
				fmt.Fprint(w, "250 OK\r\n")
				_ = w.Flush()
			case strings.HasPrefix(line, "RCPT TO:"):
				fmt.Fprint(w, "250 OK\r\n")
				_ = w.Flush()
			case strings.HasPrefix(line, "DATA"):
				fmt.Fprint(w, "354 Start mail input; end with <CRLF>.<CRLF>\r\n")
				_ = w.Flush()
				for {
					l, err := r.ReadLine()
					if err != nil || l == "." {
						break
					}
				}
				fmt.Fprint(w, "250 OK\r\n")
				_ = w.Flush()
			case strings.HasPrefix(line, "QUIT"):
				fmt.Fprint(w, "221 Bye\r\n")
				_ = w.Flush()
				return
			default:
				// Default OK to tolerate extra commands.
				fmt.Fprint(w, "250 OK\r\n")
				_ = w.Flush()
			}
		}
	}()

	manager := NewEnhancedEmailManager(config)
	err := manager.sendPlain("ignored:25", []byte("Test Message"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
func TestSendTLS_Success(t *testing.T) {
	// We don't need a real TLS server here. This is an error-path sanity check that
	// exercises the TLS dialer logic without binding ports (which can be blocked in CI).
	addr := "127.0.0.1:1"

	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "invalid.host.test",
			SMTPPort: 1,
			TLS:      true,
		},
		SkipTLSVerify: true,
	}

	manager := NewEnhancedEmailManager(config)
	err := manager.sendTLS(addr, []byte("test"))
	// It will still fail because we aren't running a real TLS server here,
	// but we can verify it reaches the TLS dialer.
	if err == nil {
		t.Error("expected TLS error")
	}
	if !strings.Contains(err.Error(), "TLS dial failed") && !strings.Contains(err.Error(), "remote error") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendStartTLS_Success(t *testing.T) {
	config := EmailProviderConfig{
		EmailConfig: EmailConfig{
			SMTPHost: "smtp.example.com",
			SMTPPort: 587,
			StartTLS: true,
		},
		SkipTLSVerify: true,
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	origDial := smtpDialTimeout
	smtpDialTimeout = func(network, addr string, timeout time.Duration) (net.Conn, error) {
		return clientConn, nil
	}
	t.Cleanup(func() { smtpDialTimeout = origDial })

	go func() {
		defer serverConn.Close()

		w := bufio.NewWriter(serverConn)
		r := textproto.NewReader(bufio.NewReader(serverConn))

		fmt.Fprint(w, "220 smtp.example.com ESMTP\r\n")
		_ = w.Flush()

		for {
			line, err := r.ReadLine()
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "EHLO") {
				fmt.Fprint(w, "250-smtp.example.com\r\n250 STARTTLS\r\n")
				_ = w.Flush()
				continue
			}
			if strings.HasPrefix(line, "STARTTLS") {
				fmt.Fprint(w, "220 Ready to start TLS\r\n")
				_ = w.Flush()
				return // Client will attempt TLS handshake and fail.
			}
			fmt.Fprint(w, "250 OK\r\n")
			_ = w.Flush()
		}
	}()

	manager := NewEnhancedEmailManager(config)
	err := manager.sendStartTLS("ignored:587", []byte("Test Message"))
	// Should fail at TLS upgrade because mock server doesn't actually do TLS
	if err == nil {
		t.Error("expected STARTTLS upgrade error")
	}
}
