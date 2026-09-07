package notifications

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Bounded Delivery assessment. All credentials are synthetic; no network sends.
func TestDeliveryEncodedQueryConfidentiality(t *testing.T) {
	for _, key := range []string{"token", "apikey", "api_key", "key", "secret", "password"} {
		t.Run(key, func(t *testing.T) {
			encoded := "%" + "74" + key[1:]
			// Encode the first byte of each already-supported query key.
			switch key[0] {
			case 'a':
				encoded = "%61" + key[1:]
			case 'k':
				encoded = "%6b" + key[1:]
			case 's':
				encoded = "%73" + key[1:]
			case 'p':
				encoded = "%70" + key[1:]
			}
			target := "https://example.test/hook?" + encoded + "=delivery-fixture-secret&channel=ops"
			parsed, err := url.Parse(target)
			if err != nil || parsed.Query().Get(key) != "delivery-fixture-secret" {
				t.Fatal("invalid fixture")
			}
			if strings.Contains(RedactWebhookURLSecrets(target), "delivery-fixture-secret") {
				t.Error("URL helper exposes encoded-key credential")
			}
			if strings.Contains(RedactWebhookDiagnosticSecrets("Post "+target+" failed"), "delivery-fixture-secret") {
				t.Error("diagnostic helper exposes encoded-key credential")
			}
			var captured bytes.Buffer
			original := log.Logger
			log.Logger = zerolog.New(&captured)
			defer func() { log.Logger = original }()
			manager := &NotificationManager{webhookRateLimits: make(map[string]*webhookRateLimit)}
			for range WebhookRateLimitMax + 2 {
				manager.checkWebhookRateLimit(target)
			}
			if !strings.Contains(captured.String(), "rate limit exceeded") {
				t.Fatal("log path not reached")
			}
			if strings.Contains(captured.String(), "delivery-fixture-secret") {
				t.Error("rate-limit log exposes encoded-key credential")
			}
		})
	}
}

type deliveryFailTransport struct{ cause error }

func (d deliveryFailTransport) RoundTrip(*http.Request) (*http.Response, error) { return nil, d.cause }
func TestDeliveryResolvedNtfyConfidentiality(t *testing.T) {
	for _, key := range []string{"token", "apikey", "api_key", "key", "secret", "password"} {
		t.Run(key, func(t *testing.T) { testResolvedNtfyConfidentiality(t, key) })
	}
}
func testResolvedNtfyConfidentiality(t *testing.T, key string) {
	manager := &NotificationManager{webhookRateLimits: make(map[string]*webhookRateLimit)}
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32"); err != nil {
		t.Fatal(err)
	}
	cause := errors.New("synthetic transport failure")
	manager.webhookClient = &http.Client{Transport: deliveryFailTransport{cause}}
	var captured bytes.Buffer
	original := log.Logger
	log.Logger = zerolog.New(&captured)
	defer func() { log.Logger = original }()
	err := manager.sendResolvedWebhookNtfy(WebhookConfig{Name: "fixture", Service: "ntfy", URL: fmt.Sprintf("http://127.0.0.1/topic?%s=delivery-fixture-secret&%%%02x%s=delivery-fixture-secret", key, key[0], key[1:])}, nil, time.Now())
	if !errors.Is(err, cause) {
		t.Fatalf("transport not reached: %v", err)
	}
	if strings.Contains(err.Error(), "delivery-fixture-secret") {
		t.Error("resolved ntfy transport error exposes literal token credential")
	}
	if !strings.Contains(captured.String(), "failed to send resolved ntfy webhook") {
		t.Fatal("error log not reached")
	}
	if strings.Contains(captured.String(), "delivery-fixture-secret") {
		t.Error("resolved ntfy error log exposes literal token credential")
	}
}

// A finite cross-sink matrix: no network sends or persistent queue.
func TestWebhookConfidentialityCallerMatrix(t *testing.T) {
	targets := []string{
		"https://fixture-user:fixture-secret@example.test/hook",
		"https://hooks.slack.com/services/team/id/fixture-secret",
		"https://hooks.slack-gov.com/legacy/fixture-secret",
		"https://discord.com/api/v10/webhooks/id/fixture-secret",
		"https://discordapp.com/api/webhooks/id/fixture-secret",
		"https://api.telegram.org/%62otfixture-secret/sendMessage",
		"http://127.0.0.1:8081/botfixture-secret/sendMessage",
	}
	for _, key := range []string{"token", "apikey", "api_key", "key", "secret", "password"} {
		encoded := fmt.Sprintf("%%%02x%s", key[0], key[1:])
		targets = append(targets, "https://example.test/hook?"+key+"=fixture-secret&"+encoded+"=fixture-secret&channel=ops")
	}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			want := RedactWebhookURLSecrets(target)
			if strings.Contains(want, "fixture-secret") || strings.Contains(want, "fixture-user") {
				t.Fatal("unsafe URL")
			}
			if got := RedactWebhookDiagnosticSecrets("Post " + target + " failed"); got != "Post "+want+" failed" {
				t.Fatalf("context lost: %s", got)
			}
			var captured bytes.Buffer
			original := log.Logger
			log.Logger = zerolog.New(&captured)
			defer func() { log.Logger = original }()
			manager := &NotificationManager{webhookRateLimits: make(map[string]*webhookRateLimit)}
			for range WebhookRateLimitMax + 2 {
				manager.checkWebhookRateLimit(target)
			}
			if strings.Contains(captured.String(), "fixture-secret") || !strings.Contains(captured.String(), "rate limit exceeded") {
				t.Fatal("unsafe/missing rate-limit log")
			}
			cause := errors.New("synthetic transport failure")
			payload := []byte(`{"event":"unchanged"}`)
			sent := false
			manager.webhookClient = &http.Client{Transport: confidentialityTransport(func(req *http.Request) (*http.Response, error) {
				sent = true
				body, err := io.ReadAll(req.Body)
				if err != nil || !bytes.Equal(body, payload) || req.URL.String() != target || req.Header.Get("X-Pulse-Event-ID") != "event-1" {
					t.Error("request identity changed")
				}
				return nil, cause
			})}
			_, err := manager.executeWebhookRequest(WebhookConfig{URL: target}, payload, webhookRequestOptions{eventID: "event-1"})
			if strings.Contains(target, "fixture-user") {
				if sent || err == nil || !strings.Contains(err.Error(), "URL userinfo is not allowed") || strings.Contains(err.Error(), "fixture-secret") {
					t.Fatalf("userinfo must be safely rejected: %v", err)
				}
				return
			}
			if !sent || !errors.Is(err, cause) || strings.Contains(err.Error(), "fixture-secret") || !strings.Contains(err.Error(), "synthetic transport failure") {
				t.Fatalf("unsafe/missing transport failure: %v", err)
			}
		})
	}
}

type confidentialityTransport func(*http.Request) (*http.Response, error)

func (f confidentialityTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
