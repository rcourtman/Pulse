package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Sender sends transactional emails.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Message represents an email to send.
type Message struct {
	From    string
	To      string
	Subject string
	HTML    string
	Text    string
}

// ResendSender sends emails via the Resend HTTP API.
type ResendSender struct {
	apiKey     string
	httpClient *http.Client
}

// NewResendSender creates a Resend email sender.
func NewResendSender(apiKey string) *ResendSender {
	return &ResendSender{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type resendRequest struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html,omitempty"`
	Text    string `json:"text,omitempty"`
}

// Send sends an email via the Resend API.
func (r *ResendSender) Send(ctx context.Context, msg Message) error {
	payload := resendRequest{
		From:    msg.From,
		To:      msg.To,
		Subject: msg.Subject,
		HTML:    msg.HTML,
		Text:    msg.Text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create resend request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resend error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// LogSender logs emails instead of sending them. Used as fallback when no email provider is configured.
type LogSender struct {
	logFn func(to, subject, body string)
}

// NewLogSender creates a sender that logs emails.
func NewLogSender(logFn func(to, subject, body string)) *LogSender {
	return &LogSender{logFn: logFn}
}

// Send logs the email instead of sending it.
func (l *LogSender) Send(_ context.Context, msg Message) error {
	if l.logFn != nil {
		l.logFn(msg.To, msg.Subject, msg.Text)
	}
	return nil
}
