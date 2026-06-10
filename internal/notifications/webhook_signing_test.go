package notifications

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignWebhookPayloadDeterministic(t *testing.T) {
	sig := signWebhookPayload("secret", "1700000000", []byte(`{"a":1}`))

	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte("1700000000.{\"a\":1}"))
	assert.Equal(t, hex.EncodeToString(mac.Sum(nil)), sig)
}

func TestWebhookEventID(t *testing.T) {
	assert.Equal(t, "alert-1:alert", webhookEventID("alert-1", "alert"))
	assert.Equal(t, "alert-1:resolved", webhookEventID("alert-1", "resolved"))
	assert.Equal(t, "alert-1:alert", webhookEventID("alert-1", ""))
	assert.Equal(t, "", webhookEventID("", "alert"))
}

func TestWebhookDeliveryCarriesSignatureAndEventID(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	var gotSignature, gotTimestamp, gotEventID string
	var gotBody []byte
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-Pulse-Signature")
		gotTimestamp = r.Header.Get("X-Pulse-Timestamp")
		gotEventID = r.Header.Get("X-Pulse-Event-ID")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := WebhookConfig{
		Name:          "Signed Webhook",
		URL:           server.URL,
		Enabled:       true,
		Service:       "generic",
		SigningSecret: "topsecret",
	}

	err := nm.sendWebhookRequest(webhook, []byte(`{"hello":"world"}`), "alert", "alert-42:alert")
	require.NoError(t, err)

	assert.Equal(t, "alert-42:alert", gotEventID)
	require.NotEmpty(t, gotTimestamp)
	require.True(t, strings.HasPrefix(gotSignature, "v1="), "signature %q missing v1= prefix", gotSignature)
	expected := signWebhookPayload("topsecret", gotTimestamp, gotBody)
	assert.Equal(t, "v1="+expected, gotSignature)
}

func TestWebhookDeliveryWithoutSecretOmitsSignature(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	var sawSignature, sawTimestamp bool
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawSignature = r.Header["X-Pulse-Signature"]
		_, sawTimestamp = r.Header["X-Pulse-Timestamp"]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := WebhookConfig{
		Name:    "Unsigned Webhook",
		URL:     server.URL,
		Enabled: true,
		Service: "generic",
	}

	err := nm.sendWebhookRequest(webhook, []byte(`{}`), "alert", "")
	require.NoError(t, err)
	assert.False(t, sawSignature)
	assert.False(t, sawTimestamp)
}

// Custom header maps must not be able to shadow the integrity headers.
func TestWebhookSigningHeadersWinOverCustomHeaders(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	var gotSignature string
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSignature = r.Header.Get("X-Pulse-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := WebhookConfig{
		Name:          "Shadow Attempt",
		URL:           server.URL,
		Enabled:       true,
		Service:       "generic",
		Headers:       map[string]string{"X-Pulse-Signature": "v1=spoofed"},
		SigningSecret: "topsecret",
	}

	err := nm.sendWebhookRequest(webhook, []byte(`{}`), "alert", "")
	require.NoError(t, err)
	assert.NotEqual(t, "v1=spoofed", gotSignature)
	assert.True(t, strings.HasPrefix(gotSignature, "v1="))
}
