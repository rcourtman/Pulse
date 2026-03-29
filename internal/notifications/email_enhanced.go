package notifications

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// loginAuth implements the SMTP LOGIN authentication mechanism.
// Many mail servers (notably Microsoft 365) advertise only AUTH LOGIN
// and reject AUTH PLAIN with "504 5.7.4 Unrecognized authentication type".
type loginAuth struct {
	username, password string
}

// LoginAuth returns an Auth that implements the LOGIN authentication mechanism.
func LoginAuth(username, password string) smtp.Auth {
	return &loginAuth{username, password}
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.TrimSpace(strings.ToLower(string(fromServer)))
	switch prompt {
	case "username:":
		return []byte(a.username), nil
	case "password:":
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected LOGIN prompt: %s", fromServer)
	}
}

// plainAuth implements SMTP PLAIN authentication without requiring TLS.
// Go's built-in smtp.PlainAuth refuses to send credentials over unencrypted
// connections to non-localhost hosts. This implementation allows authenticated
// SMTP over unencrypted connections when the user has explicitly configured
// no TLS/STARTTLS — which is a conscious security decision on their part.
type plainAuth struct {
	identity, username, password string
}

func (a *plainAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	resp := []byte(a.identity + "\x00" + a.username + "\x00" + a.password)
	return "PLAIN", resp, nil
}

func (a *plainAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		return nil, fmt.Errorf("unexpected server challenge")
	}
	return nil, nil
}

func sanitizeEmailHeaderValue(value string) string {
	clean := strings.ReplaceAll(value, "\r", " ")
	clean = strings.ReplaceAll(clean, "\n", " ")
	return strings.TrimSpace(clean)
}

type resolvedEmailAddresses struct {
	from    *mail.Address
	to      []*mail.Address
	replyTo *mail.Address
}

func resolveEmailAddress(fieldName, value string) (*mail.Address, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("invalid %s address %q: %w", fieldName, value, err)
	}
	return addr, nil
}

func resolveRecipientAddresses(values []string) ([]*mail.Address, error) {
	if len(values) == 0 {
		return nil, fmt.Errorf("at least one recipient address is required")
	}

	resolved := make([]*mail.Address, 0, len(values))
	for i, value := range values {
		addr, err := resolveEmailAddress("recipient", value)
		if err != nil {
			return nil, fmt.Errorf("recipient %d: %w", i+1, err)
		}
		resolved = append(resolved, addr)
	}
	return resolved, nil
}

func (e *EnhancedEmailManager) resolveEmailAddresses() (resolvedEmailAddresses, error) {
	from, err := resolveEmailAddress("from", e.config.From)
	if err != nil {
		return resolvedEmailAddresses{}, err
	}

	to, err := resolveRecipientAddresses(e.config.To)
	if err != nil {
		return resolvedEmailAddresses{}, err
	}

	var replyTo *mail.Address
	if strings.TrimSpace(e.config.ReplyTo) != "" {
		replyTo, err = resolveEmailAddress("reply-to", e.config.ReplyTo)
		if err != nil {
			return resolvedEmailAddresses{}, err
		}
	}

	return resolvedEmailAddresses{
		from:    from,
		to:      to,
		replyTo: replyTo,
	}, nil
}

func formatHeaderAddresses(addresses []*mail.Address) string {
	if len(addresses) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		formatted = append(formatted, addr.String())
	}
	return strings.Join(formatted, ", ")
}

func envelopeRecipients(addresses []*mail.Address) []string {
	recipients := make([]string, 0, len(addresses))
	for _, addr := range addresses {
		recipients = append(recipients, addr.Address)
	}
	return recipients
}

// negotiateAuth queries the server for supported AUTH mechanisms after EHLO
// and returns the best smtp.Auth to use. Prefers PLAIN, falls back to LOGIN.
// Returns nil if auth is not configured.
func (e *EnhancedEmailManager) negotiateAuth(client *smtp.Client) (smtp.Auth, error) {
	if !e.config.AuthRequired || e.config.Username == "" || e.config.Password == "" {
		return nil, nil
	}

	// Check what the server advertises after EHLO.
	// Extension returns (ok bool, params string).
	hasAuth, mechanismsRaw := client.Extension("AUTH")
	mechanisms := strings.ToUpper(mechanismsRaw)

	if strings.Contains(mechanisms, "PLAIN") {
		return &plainAuth{"", e.config.Username, e.config.Password}, nil
	}
	if strings.Contains(mechanisms, "LOGIN") {
		return LoginAuth(e.config.Username, e.config.Password), nil
	}

	// Server didn't advertise AUTH at all — try PLAIN as a default since
	// it's the most widely supported. This handles servers that don't
	// properly advertise their capabilities.
	if !hasAuth || mechanisms == "" {
		return &plainAuth{"", e.config.Username, e.config.Password}, nil
	}

	return nil, fmt.Errorf("server advertises AUTH mechanisms [%s] but none are supported (PLAIN, LOGIN)", mechanisms)
}

// EnhancedEmailManager extends email functionality with provider support
type EnhancedEmailManager struct {
	config    EmailProviderConfig
	rateLimit *RateLimiter
}

var (
	smtpDialTimeout       = net.DialTimeout
	smtpTLSDialWithDialer = tls.DialWithDialer
)

// RateLimiter implements a simple rate limiter
type RateLimiter struct {
	mu        sync.Mutex
	rate      int
	lastSent  time.Time
	sentCount int
}

// NewEnhancedEmailManager creates an enhanced email manager
func NewEnhancedEmailManager(config EmailProviderConfig) *EnhancedEmailManager {
	return &EnhancedEmailManager{
		config: config,
		rateLimit: &RateLimiter{
			rate: config.RateLimit,
		},
	}
}

// SendEmailWithRetry sends email with retry logic
// Note: When used with the persistent queue, retry behavior is layered:
// - Transport retries (this function): up to MaxRetries attempts with RetryDelay between
// - Queue retries: up to MaxAttempts (default 3) with exponential backoff
// Total attempts = MaxRetries * MaxAttempts (e.g., 3 * 3 = 9 SMTP calls for a single notification)
// This ensures delivery even during transient failures at either layer.
func (e *EnhancedEmailManager) SendEmailWithRetry(subject, htmlBody, textBody string) error {
	var lastErr error

	for attempt := 0; attempt <= e.config.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(e.config.RetryDelay) * time.Second
			log.Debug().
				Int("attempt", attempt).
				Dur("delay", delay).
				Msg("retrying email send after delay")
			time.Sleep(delay)
		}

		// Check rate limit
		if err := e.checkRateLimit(); err != nil {
			lastErr = err
			continue
		}

		// Try to send
		err := e.sendEmailOnce(subject, htmlBody, textBody)
		if err == nil {
			if attempt > 0 {
				log.Info().
					Int("attempt", attempt).
					Msg("email sent successfully after retry")
			}
			return nil
		}

		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Str("provider", e.config.Provider).
			Msg("email send attempt failed")
	}

	return fmt.Errorf("email failed after %d attempts: %w", e.config.MaxRetries+1, lastErr)
}

// checkRateLimit enforces rate limiting
func (e *EnhancedEmailManager) checkRateLimit() error {
	if e.config.RateLimit <= 0 {
		return nil // No rate limit
	}

	e.rateLimit.mu.Lock()
	defer e.rateLimit.mu.Unlock()

	now := time.Now()
	if now.Sub(e.rateLimit.lastSent) >= time.Minute {
		// Reset counter after a minute
		e.rateLimit.sentCount = 0
		e.rateLimit.lastSent = now
	}

	if e.rateLimit.sentCount >= e.config.RateLimit {
		return fmt.Errorf("rate limit exceeded: %d emails per minute", e.config.RateLimit)
	}

	e.rateLimit.sentCount++
	return nil
}

// sendEmailOnce sends a single email
func (e *EnhancedEmailManager) sendEmailOnce(subject, htmlBody, textBody string) error {
	addresses, err := e.resolveEmailAddresses()
	if err != nil {
		return err
	}

	// Build message with enhanced headers
	boundary := fmt.Sprintf("===============%d==", time.Now().UnixNano())

	msg := fmt.Sprintf("From: %s\r\n", addresses.from.String())
	msg += fmt.Sprintf("To: %s\r\n", formatHeaderAddresses(addresses.to))
	if addresses.replyTo != nil {
		msg += fmt.Sprintf("Reply-To: %s\r\n", addresses.replyTo.String())
	}
	msg += fmt.Sprintf("Subject: %s\r\n", sanitizeEmailHeaderValue(subject))
	msg += fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	msg += fmt.Sprintf("Message-ID: <%d@pulse-monitoring>\r\n", time.Now().UnixNano())
	msg += "MIME-Version: 1.0\r\n"
	msg += fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	msg += "X-Mailer: Pulse Monitoring System\r\n"
	msg += "\r\n"

	// Text part
	msg += fmt.Sprintf("--%s\r\n", boundary)
	msg += "Content-Type: text/plain; charset=\"UTF-8\"\r\n"
	msg += "Content-Transfer-Encoding: 7bit\r\n"
	msg += "\r\n"
	msg += textBody + "\r\n"

	// HTML part
	msg += fmt.Sprintf("--%s\r\n", boundary)
	msg += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
	msg += "Content-Transfer-Encoding: 7bit\r\n"
	msg += "\r\n"
	msg += htmlBody + "\r\n"

	// End boundary
	msg += fmt.Sprintf("--%s--\r\n", boundary)

	// Send based on provider configuration
	return e.sendViaProviderWithAddresses([]byte(msg), addresses)
}

// sendViaProvider sends email using provider-specific settings
func (e *EnhancedEmailManager) sendViaProvider(msg []byte) error {
	addresses, err := e.resolveEmailAddresses()
	if err != nil {
		return err
	}
	return e.sendViaProviderWithAddresses(msg, addresses)
}

func (e *EnhancedEmailManager) sendViaProviderWithAddresses(msg []byte, addresses resolvedEmailAddresses) error {
	addr := net.JoinHostPort(e.config.SMTPHost, strconv.Itoa(e.config.SMTPPort))

	// Special handling for specific providers
	switch e.config.Provider {
	case "SendGrid":
		// SendGrid uses "apikey" as username
		if e.config.Username == "" {
			e.config.Username = "apikey"
		}
	case "Postmark":
		// Postmark uses API token for both username and password
		if e.config.Password != "" && e.config.Username == "" {
			e.config.Username = e.config.Password
		}
	case "SparkPost":
		// SparkPost uses specific username
		if e.config.Username == "" {
			e.config.Username = "SMTP_Injection"
		}
	case "Resend":
		// Resend uses "resend" as username
		if e.config.Username == "" {
			e.config.Username = "resend"
		}
	}

	// Send with TLS configuration — auth is negotiated after connection
	if e.config.TLS || e.config.SMTPPort == 465 {
		return e.sendTLS(addr, msg, addresses)
	} else if e.config.StartTLS {
		return e.sendStartTLS(addr, msg, addresses)
	} else {
		return e.sendPlain(addr, msg, addresses)
	}
}

// sendTLS sends email over TLS connection
func (e *EnhancedEmailManager) sendTLS(addr string, msg []byte, addresses resolvedEmailAddresses) error {
	tlsConfig := &tls.Config{
		ServerName:         e.config.SMTPHost,
		InsecureSkipVerify: e.config.SkipTLSVerify,
	}

	// Use DialWithDialer with timeout
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
	}
	conn, err := smtpTLSDialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS dial failed: %w", err)
	}
	defer conn.Close()

	// Set overall connection timeout
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, e.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	auth, err := e.negotiateAuth(client)
	if err != nil {
		return fmt.Errorf("SMTP auth negotiation failed: %w", err)
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err = client.Mail(addresses.from.Address); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, to := range envelopeRecipients(addresses.to) {
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("RCPT TO failed for %s: %w", to, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("message write failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("message close failed: %w", err)
	}

	return client.Quit()
}

// sendStartTLS sends email using STARTTLS
func (e *EnhancedEmailManager) sendStartTLS(addr string, msg []byte, addresses resolvedEmailAddresses) error {
	// Use DialTimeout to prevent hanging on unreachable servers
	conn, err := smtpDialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("TCP dial failed: %w", err)
	}
	defer conn.Close()

	// Set overall connection timeout
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, e.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	// STARTTLS
	tlsConfig := &tls.Config{
		ServerName:         e.config.SMTPHost,
		InsecureSkipVerify: e.config.SkipTLSVerify,
	}

	if err = client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS failed: %w", err)
	}

	// Negotiate auth after STARTTLS — the server re-advertises capabilities
	// and we can now see what AUTH mechanisms are available
	auth, err := e.negotiateAuth(client)
	if err != nil {
		return fmt.Errorf("SMTP auth negotiation failed: %w", err)
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err = client.Mail(addresses.from.Address); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, to := range envelopeRecipients(addresses.to) {
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("RCPT TO failed for %s: %w", to, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("message write failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("message close failed: %w", err)
	}

	return client.Quit()
}

// TestConnection tests the email server connection
func (e *EnhancedEmailManager) TestConnection() error {
	addr := net.JoinHostPort(e.config.SMTPHost, strconv.Itoa(e.config.SMTPPort))

	// Try to connect
	var conn net.Conn
	var err error

	if e.config.TLS || e.config.SMTPPort == 465 {
		tlsConfig := &tls.Config{
			ServerName:         e.config.SMTPHost,
			InsecureSkipVerify: e.config.SkipTLSVerify,
		}
		dialer := &net.Dialer{
			Timeout: 10 * time.Second,
		}
		conn, err = smtpTLSDialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = net.DialTimeout("tcp", addr, 10*time.Second)
	}

	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close()

	// Bound handshake/auth commands so TestConnection cannot hang indefinitely.
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, e.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("SMTP handshake failed: %w", err)
	}
	defer client.Close()

	// Test STARTTLS if configured
	if e.config.StartTLS && !e.config.TLS {
		tlsConfig := &tls.Config{
			ServerName:         e.config.SMTPHost,
			InsecureSkipVerify: e.config.SkipTLSVerify,
		}
		if err = client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("STARTTLS failed: %w", err)
		}
	}

	// Test authentication if configured
	auth, authErr := e.negotiateAuth(client)
	if authErr != nil {
		return fmt.Errorf("auth negotiation failed: %w", authErr)
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	return client.Quit()
}

// sendPlain sends email over plain SMTP connection with timeout
func (e *EnhancedEmailManager) sendPlain(addr string, msg []byte, addresses resolvedEmailAddresses) error {
	// Use DialTimeout to prevent hanging on unreachable servers
	conn, err := smtpDialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("TCP dial failed: %w", err)
	}
	defer conn.Close()

	// Set overall connection timeout
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return fmt.Errorf("failed to set connection deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, e.config.SMTPHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	auth, err := e.negotiateAuth(client)
	if err != nil {
		return fmt.Errorf("SMTP auth negotiation failed: %w", err)
	}
	if auth != nil {
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP auth failed: %w", err)
		}
	}

	if err = client.Mail(addresses.from.Address); err != nil {
		return fmt.Errorf("MAIL FROM failed: %w", err)
	}

	for _, to := range envelopeRecipients(addresses.to) {
		if err = client.Rcpt(to); err != nil {
			return fmt.Errorf("RCPT TO failed for %s: %w", to, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA command failed: %w", err)
	}

	_, err = w.Write(msg)
	if err != nil {
		return fmt.Errorf("message write failed: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("message close failed: %w", err)
	}

	return client.Quit()
}
