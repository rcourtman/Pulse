package stripe

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	stripewebhook "github.com/stripe/stripe-go/v82/webhook"
)

func TestWebhookRetriesFailedEventInsteadOfSkippingDuplicate(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := NewProvisioner(reg, tenantsDir, nil, nil, "https://cloud.example.com", nil, "", false)

	const secret = "whsec_test_secret"
	handler := NewWebhookHandler(secret, provisioner)

	eventJSON := `{"id":"evt_retry_failed_123","object":"event","type":"customer.subscription.updated","data":{"object":{"id":"sub_missing_customer","status":"active"}}}`
	req1 := signedWebhookRequest(t, secret, eventJSON)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusInternalServerError {
		t.Fatalf("first delivery status=%d, want=%d, body=%q", rec1.Code, http.StatusInternalServerError, rec1.Body.String())
	}

	// Duplicate delivery must retry processing (and fail again here), not short-circuit
	// as if the event had already been handled successfully.
	req2 := signedWebhookRequest(t, secret, eventJSON)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusInternalServerError {
		t.Fatalf("duplicate delivery status=%d, want=%d, body=%q", rec2.Code, http.StatusInternalServerError, rec2.Body.String())
	}
}

func signedWebhookRequest(t *testing.T, secret, payload string) *http.Request {
	t.Helper()

	signed := stripewebhook.GenerateTestSignedPayload(&stripewebhook.UnsignedPayload{
		Payload:   []byte(payload),
		Secret:    secret,
		Timestamp: time.Now(),
		Scheme:    "v1",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", bytes.NewReader(signed.Payload))
	req.Header.Set("Stripe-Signature", signed.Header)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func newTestRegistry(t *testing.T) *registry.TenantRegistry {
	t.Helper()
	reg, err := registry.NewTenantRegistry(t.TempDir())
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	return reg
}
