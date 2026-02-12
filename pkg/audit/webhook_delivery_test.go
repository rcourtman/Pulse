package audit

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestWebhookDeliveryDeliverWithRetry(t *testing.T) {
	origResolver := resolveWebhookIPs
	origBackoff := webhookBackoff
	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	webhookBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() {
		resolveWebhookIPs = origResolver
		webhookBackoff = origBackoff
	})

	var attempts int
	event := Event{
		ID:        "evt-1",
		EventType: "login",
		Timestamp: time.Unix(123, 0),
		User:      "user",
		IP:        "10.0.0.1",
		Path:      "/api/login",
		Success:   true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("expected application/json content-type, got %s", ct)
		}
		if ua := r.Header.Get("User-Agent"); ua != "Pulse-Audit-Webhook/1.0" {
			t.Fatalf("unexpected user-agent %q", ua)
		}
		if r.Header.Get("X-Pulse-Event") != event.EventType {
			t.Fatalf("expected event header %q, got %q", event.EventType, r.Header.Get("X-Pulse-Event"))
		}
		if r.Header.Get("X-Pulse-Event-ID") != event.ID {
			t.Fatalf("expected event id header %q, got %q", event.ID, r.Header.Get("X-Pulse-Event-ID"))
		}

		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed decoding payload: %v", err)
		}
		if payload.Event != "audit."+event.EventType {
			t.Fatalf("expected payload event %q, got %q", "audit."+event.EventType, payload.Event)
		}
		if payload.Data.ID != event.ID {
			t.Fatalf("expected payload event id %q, got %q", event.ID, payload.Data.ID)
		}

		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed parsing server URL: %v", err)
	}
	targetHost := "example.com"

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, targetHost) {
				return (&net.Dialer{}).DialContext(ctx, network, serverURL.Host)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}

	delivery := NewWebhookDelivery([]string{"http://" + targetHost + "/audit"})
	delivery.client = &http.Client{Transport: transport}

	if err := delivery.deliverWithRetry("http://"+targetHost+"/audit", event); err != nil {
		t.Fatalf("expected delivery to succeed, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWebhookDeliveryDeliverWithRetryFails(t *testing.T) {
	origResolver := resolveWebhookIPs
	origBackoff := webhookBackoff
	resolveWebhookIPs = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}}, nil
	}
	webhookBackoff = []time.Duration{0, 0, 0}
	t.Cleanup(func() {
		resolveWebhookIPs = origResolver
		webhookBackoff = origBackoff
	})

	var attempts int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("failed parsing server URL: %v", err)
	}
	targetHost := "example.com"

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if strings.HasPrefix(addr, targetHost) {
				return (&net.Dialer{}).DialContext(ctx, network, serverURL.Host)
			}
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}

	delivery := NewWebhookDelivery([]string{"http://" + targetHost + "/audit"})
	delivery.client = &http.Client{Transport: transport}

	err = delivery.deliverWithRetry("http://"+targetHost+"/audit", Event{
		ID:        "evt-2",
		EventType: "logout",
		Timestamp: time.Now(),
		IP:        "10.0.0.2",
		Success:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status error, got %v", err)
	}
	if attempts != webhookMaxRetries+1 {
		t.Fatalf("expected %d attempts, got %d", webhookMaxRetries+1, attempts)
	}
}

func TestWebhookDeliveryDeliverInvalidURL(t *testing.T) {
	delivery := NewWebhookDelivery([]string{})

	err := delivery.deliver("://bad-url", Event{ID: "evt-3", EventType: "login", Timestamp: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "webhook URL blocked") {
		t.Fatalf("expected URL blocked error, got %v", err)
	}
}

func TestWebhookDeliveryStopIsIdempotent(t *testing.T) {
	delivery := NewWebhookDelivery([]string{})
	delivery.Start()

	delivery.Stop()
	delivery.Stop()
}
