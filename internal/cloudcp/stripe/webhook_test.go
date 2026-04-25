package stripe

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	stripelib "github.com/stripe/stripe-go/v82"
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

func TestWebhookIgnoresSelfHostedProTrialCheckoutSessions(t *testing.T) {
	reg := newTestRegistry(t)
	tenantsDir := t.TempDir()
	provisioner := NewProvisioner(reg, tenantsDir, nil, nil, "https://cloud.example.com", nil, "", false)

	const secret = "whsec_test_secret"
	handler := NewWebhookHandler(secret, provisioner)

	eventJSON := `{
		"id":"evt_self_hosted_trial_123",
		"object":"event",
		"type":"checkout.session.completed",
		"data":{
			"object":{
				"id":"cs_test_self_hosted_pro_trial",
				"mode":"subscription",
				"customer":"cus_selfhost_trial",
				"subscription":"sub_selfhost_trial",
				"customer_email":"trial@business.example",
				"customer_details":{"email":"trial@business.example"},
				"metadata":{
					"signup_source":"pulse_pro_trial",
					"org_id":"default",
					"return_url":"https://pulse.example.com/auth/trial-activate",
					"instance_token":"tsi_test"
				}
			}
		}
	}`
	req := signedWebhookRequest(t, secret, eventJSON)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delivery status=%d, want=%d, body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	tenants, err := reg.List()
	if err != nil {
		t.Fatalf("List tenants: %v", err)
	}
	if len(tenants) != 0 {
		t.Fatalf("tenant count=%d, want 0 for self-hosted Pro trial checkout", len(tenants))
	}
	stripeAccount, err := reg.GetStripeAccountByCustomerID("cus_selfhost_trial")
	if err != nil {
		t.Fatalf("GetStripeAccountByCustomerID: %v", err)
	}
	if stripeAccount != nil {
		t.Fatalf("Stripe account mapping was created for self-hosted Pro trial checkout: %#v", stripeAccount)
	}
}

func TestWebhookEventContext_DetachesCheckoutFromRequestContext(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", nil)
	ctx, cancelReq := context.WithCancel(req.Context())
	cancelReq()
	req = req.WithContext(ctx)

	gotCtx, cancel := webhookEventContext(req, stripelib.EventType("checkout.session.completed"))
	defer cancel()

	if err := gotCtx.Err(); err != nil {
		t.Fatalf("checkout context should not inherit request cancellation: %v", err)
	}
	deadline, ok := gotCtx.Deadline()
	if !ok {
		t.Fatal("checkout context should carry a timeout deadline")
	}
	if remaining := time.Until(deadline); remaining <= time.Minute || remaining > checkoutProvisioningTimeout {
		t.Fatalf("checkout context deadline window=%v, want within (1m,%v]", remaining, checkoutProvisioningTimeout)
	}
}

func TestWebhookEventContext_PreservesRequestContextForNonCheckout(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", nil)
	ctx, cancelReq := context.WithCancel(req.Context())
	cancelReq()
	req = req.WithContext(ctx)

	gotCtx, cancel := webhookEventContext(req, stripelib.EventType("customer.subscription.updated"))
	defer cancel()

	if err := gotCtx.Err(); err == nil {
		t.Fatal("non-checkout context should preserve request cancellation")
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
