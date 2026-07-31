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
	if err := validateWebhookURL(context.Background(), "https://example.com"); err != nil {
		t.Fatalf("expected valid URL with background context, got %v", err)
	}

	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return nil, context.DeadlineExceeded
	}
	if err := validateWebhookURL(context.Background(), "https://example.com"); err == nil {
		t.Fatalf("expected resolution error")
	}

	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{}, nil
	}
	if err := validateWebhookURL(context.Background(), "https://example.com"); err == nil {
		t.Fatalf("expected empty resolution error")
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
		"::1":         true,
		"8.8.8.8":     false,
	}
	for ipStr, expected := range cases {
		if got := isPrivateOrReservedIP(net.ParseIP(ipStr)); got != expected {
			t.Fatalf("ip %s expected %v, got %v", ipStr, expected, got)
		}
	}
}

// TestIsPrivateOrReservedIPRejectsIPv6TransitionAddresses covers the SSRF
// bypass where a NAT64, 6to4, Teredo, ISATAP or IPv4-compatible address routes
// to an internal IPv4 destination while every net.IP predicate reports it as an
// ordinary public address.
func TestIsPrivateOrReservedIPRejectsIPv6TransitionAddresses(t *testing.T) {
	cases := map[string]bool{
		"64:ff9b::a9fe:a9fe":       true,  // NAT64 -> 169.254.169.254
		"64:ff9b::7f00:1":          true,  // NAT64 -> 127.0.0.1
		"64:ff9b::a00:1":           true,  // NAT64 -> 10.0.0.1
		"64:ff9b:1::c0a8:1":        true,  // NAT64 local-use -> 192.168.0.1
		"64:ff9b:1:a9fe:a9:fe00::": true,  // NAT64 local-use /48 -> 169.254.169.254
		"2002:ac10:1::":            true,  // 6to4 -> 172.16.0.1
		"2002:a9fe:a9fe::":         true,  // 6to4 -> 169.254.169.254
		"2001:0:a9fe:a9fe::":       true,  // Teredo server -> 169.254.169.254
		"2001:db8::5efe:c0a8:101":  true,  // ISATAP -> 192.168.1.1
		"::192.168.1.1":            true,  // IPv4-compatible -> 192.168.1.1
		"::ffff:0:10.0.0.1":        true,  // IPv4-translated -> 10.0.0.1
		"64:ff9b::808:808":         false, // NAT64 -> 8.8.8.8
		"2002:808:808::":           false, // 6to4 -> 8.8.8.8
		"2001:db8::5efe:808:808":   false, // ISATAP -> 8.8.8.8
		"2606:4700:4700::1111":     false, // ordinary global unicast
	}
	for ipStr, expected := range cases {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			t.Fatalf("failed to parse %q", ipStr)
		}
		if got := isPrivateOrReservedIP(ip); got != expected {
			t.Fatalf("ip %s expected %v, got %v", ipStr, expected, got)
		}
	}
}

func TestValidateWebhookURLRejectsIPv6TransitionLiterals(t *testing.T) {
	blocked := []string{
		"https://[64:ff9b::a9fe:a9fe]/latest/meta-data/",
		"https://[2002:ac10:1::]/hook",
		"https://[2001:db8::5efe:c0a8:101]/hook",
	}
	for _, raw := range blocked {
		if err := validateWebhookURL(context.Background(), raw); err == nil {
			t.Fatalf("expected %s to be rejected", raw)
		}
	}
}

func TestValidateWebhookURLRejectsResolvedIPv6TransitionAddresses(t *testing.T) {
	origResolver := resolveWebhookIPs
	defer func() { resolveWebhookIPs = origResolver }()

	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("64:ff9b::a9fe:a9fe")}}, nil
	}

	if err := validateWebhookURL(context.Background(), "https://webhook.example.com/hook"); err == nil {
		t.Fatalf("expected hostname resolving to a NAT64 metadata address to be rejected")
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
