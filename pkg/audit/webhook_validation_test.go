package audit

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestValidateWebhookURL(t *testing.T) {
	origResolver := resolveWebhookIPs
	defer func() { resolveWebhookIPs = origResolver }()

	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}

	if err := validateWebhookURL(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty URL")
	}
	if err := validateWebhookURL(context.Background(), "not a url"); err == nil {
		t.Fatalf("expected error for invalid URL")
	}
	if err := validateWebhookURL(context.Background(), "ftp://example.com"); err == nil {
		t.Fatalf("expected error for invalid scheme")
	}
	if err := validateWebhookURL(context.Background(), "http://"); err == nil {
		t.Fatalf("expected error for missing host")
	}
	if err := validateWebhookURL(context.Background(), "http://localhost"); err == nil {
		t.Fatalf("expected error for localhost")
	}
	if err := validateWebhookURL(context.Background(), "http://127.0.0.1"); err == nil {
		t.Fatalf("expected error for loopback")
	}
	if err := validateWebhookURL(context.Background(), "http://[::1]"); err == nil {
		t.Fatalf("expected error for ipv6 loopback")
	}
	if err := validateWebhookURL(context.Background(), "http://192.168.1.5"); err == nil {
		t.Fatalf("expected error for private IP")
	}
	if err := validateWebhookURL(context.Background(), "http://metadata.google.internal"); err == nil {
		t.Fatalf("expected error for blocked hostname")
	}
	if err := validateWebhookURL(context.Background(), "http://example.local"); err == nil {
		t.Fatalf("expected error for .local hostname")
	}
	if err := validateWebhookURL(context.Background(), "http://internal.example.com"); err == nil {
		t.Fatalf("expected error for internal hostname")
	}

	if err := validateWebhookURL(context.Background(), "https://example.com"); err != nil {
		t.Fatalf("expected valid URL, got %v", err)
	}
	if err := validateWebhookURL(nil, "https://example.com"); err != nil {
		t.Fatalf("expected valid URL with nil context, got %v", err)
	}

	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return nil, context.DeadlineExceeded
	}
	if err := validateWebhookURL(context.Background(), "https://example.com"); err == nil {
		t.Fatalf("expected resolution error")
	}

	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("10.0.0.2")}}, nil
	}
	if err := validateWebhookURL(context.Background(), "https://example.com"); err == nil {
		t.Fatalf("expected private IP resolution error")
	}
}

func TestIsPrivateOrReservedIP(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":   true,
		"10.0.0.1":    true,
		"169.254.1.1": true,
		"0.0.0.0":     true,
		"8.8.8.8":     false,
	}
	for ipStr, expected := range cases {
		if got := isPrivateOrReservedIP(net.ParseIP(ipStr)); got != expected {
			t.Fatalf("ip %s expected %v, got %v", ipStr, expected, got)
		}
	}
}

func TestWebhookDelivery_QueueAndURLs(t *testing.T) {
	delivery := NewWebhookDelivery([]string{"http://example.com"})
	if delivery.QueueLength() != 0 {
		t.Fatalf("expected empty queue")
	}

	delivery.Enqueue(Event{ID: "e1", EventType: "login", Timestamp: time.Now()})
	if delivery.QueueLength() != 1 {
		t.Fatalf("expected queued event")
	}

	delivery.UpdateURLs([]string{"http://new.example.com"})
	urls := delivery.GetURLs()
	if len(urls) != 1 || urls[0] != "http://new.example.com" {
		t.Fatalf("expected updated URLs")
	}

	urls[0] = "mutated"
	if delivery.GetURLs()[0] != "http://new.example.com" {
		t.Fatalf("expected URLs to be copied defensively")
	}
}

func TestWebhookDeliveryEnqueueDropsWhenFull(t *testing.T) {
	delivery := &WebhookDelivery{
		queue: make(chan Event, 1),
	}

	delivery.Enqueue(Event{ID: "first", EventType: "login", Timestamp: time.Now()})
	delivery.Enqueue(Event{ID: "second", EventType: "login", Timestamp: time.Now()})

	if delivery.QueueLength() != 1 {
		t.Fatalf("expected queue to stay at capacity, got %d", delivery.QueueLength())
	}
}
