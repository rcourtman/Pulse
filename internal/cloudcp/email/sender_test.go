package email

import (
	"context"
	"encoding/json"
	"testing"
)

func TestLogSender_Send(t *testing.T) {
	var called bool
	var gotTo, gotSubject string

	sender := NewLogSender(func(to, subject, body string) {
		called = true
		gotTo = to
		gotSubject = subject
		_ = body
	})

	err := sender.Send(context.Background(), Message{
		To:      "test@example.com",
		Subject: "Test Subject",
		Text:    "Hello",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("log function was not called")
	}
	if gotTo != "test@example.com" {
		t.Errorf("expected to=test@example.com, got %s", gotTo)
	}
	if gotSubject != "Test Subject" {
		t.Errorf("expected subject=Test Subject, got %s", gotSubject)
	}
}

func TestResendSender_New(t *testing.T) {
	sender := NewResendSender("re_test_key", "support@example.com")
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}
	if sender.apiKey != "re_test_key" {
		t.Errorf("expected apiKey=re_test_key, got %s", sender.apiKey)
	}
	if sender.defaultReplyTo != "support@example.com" {
		t.Errorf("expected defaultReplyTo=support@example.com, got %s", sender.defaultReplyTo)
	}
}

func TestResendSender_RequestUsesDefaultReplyTo(t *testing.T) {
	sender := NewResendSender("re_test_key", " support@example.com ")

	payload := sender.requestForMessage(Message{
		From:    "Pulse <noreply@example.com>",
		To:      "customer@example.com",
		Subject: "Sign in to Pulse",
		Text:    "Open this link.",
	})

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !json.Valid(body) {
		t.Fatalf("payload is not valid JSON: %s", body)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got["reply_to"] != "support@example.com" {
		t.Fatalf("reply_to = %q, want support@example.com", got["reply_to"])
	}
}

func TestResendSender_RequestPrefersMessageReplyTo(t *testing.T) {
	sender := NewResendSender("re_test_key", "support@example.com")

	payload := sender.requestForMessage(Message{
		From:    "Pulse <noreply@example.com>",
		To:      "customer@example.com",
		ReplyTo: " owner@example.com ",
		Subject: "Sign in to Pulse",
		Text:    "Open this link.",
	})

	if payload.ReplyTo != "owner@example.com" {
		t.Fatalf("ReplyTo = %q, want owner@example.com", payload.ReplyTo)
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		From:    "sender@example.com",
		To:      "recipient@example.com",
		ReplyTo: "support@example.com",
		Subject: "Hello",
		HTML:    "<h1>Hello</h1>",
		Text:    "Hello",
	}

	if msg.From != "sender@example.com" {
		t.Errorf("unexpected From: %s", msg.From)
	}
	if msg.To != "recipient@example.com" {
		t.Errorf("unexpected To: %s", msg.To)
	}
	if msg.ReplyTo != "support@example.com" {
		t.Errorf("unexpected ReplyTo: %s", msg.ReplyTo)
	}
}
