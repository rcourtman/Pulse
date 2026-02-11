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

// PostmarkSender sends emails via the Postmark HTTP API.
type PostmarkSender struct {
	serverToken string
	httpClient  *http.Client
}

// NewPostmarkSender creates a Postmark email sender.
func NewPostmarkSender(serverToken string) *PostmarkSender {
	return &PostmarkSender{
		serverToken: serverToken,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type postmarkRequest struct {
	From     string `json:"From"`
	To       string `json:"To"`
	Subject  string `json:"Subject"`
	HtmlBody string `json:"HtmlBody,omitempty"`
	TextBody string `json:"TextBody,omitempty"`
}

type postmarkResponse struct {
	ErrorCode int    `json:"ErrorCode"`
	Message   string `json:"Message"`
}

// Send sends an email via the Postmark API.
func (p *PostmarkSender) Send(ctx context.Context, msg Message) error {
	payload := postmarkRequest{
		From:     msg.From,
		To:       msg.To,
		Subject:  msg.Subject,
		HtmlBody: msg.HTML,
		TextBody: msg.Text,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal postmark request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.postmarkapp.com/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create postmark request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", p.serverToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("postmark request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	if resp.StatusCode != http.StatusOK {
		var pmResp postmarkResponse
		_ = json.Unmarshal(respBody, &pmResp)
		return fmt.Errorf("postmark error (HTTP %d): code=%d message=%s", resp.StatusCode, pmResp.ErrorCode, pmResp.Message)
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
