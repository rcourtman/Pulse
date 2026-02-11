package email

import (
	"context"
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

func TestPostmarkSender_New(t *testing.T) {
	sender := NewPostmarkSender("test-token")
	if sender == nil {
		t.Fatal("expected non-nil sender")
	}
	if sender.serverToken != "test-token" {
		t.Errorf("expected token=test-token, got %s", sender.serverToken)
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		From:    "sender@example.com",
		To:      "recipient@example.com",
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
}
