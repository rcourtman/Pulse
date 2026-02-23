package cloudcp

import (
	"crypto/ed25519"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	stripe "github.com/stripe/stripe-go/v82"
)

func TestTrialSignupHandleStartProTrialRendersForm(t *testing.T) {
	h := NewTrialSignupHandlers(&CPConfig{})
	req := httptest.NewRequest(http.MethodGet, "/start-pro-trial?org_id=default&return_url=https://pulse.example.com/auth/trial-activate", nil)
	rec := httptest.NewRecorder()

	h.HandleStartProTrial(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<form") || !strings.Contains(body, "Continue To Secure Checkout") {
		t.Fatalf("expected form markup in response body")
	}
}

func TestTrialSignupHandleCheckoutRejectsInvalidInput(t *testing.T) {
	h := NewTrialSignupHandlers(&CPConfig{})
	form := url.Values{
		"org_id":     {"default"},
		"return_url": {"not-a-url"},
		"name":       {"Test User"},
		"email":      {"owner@example.com"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/checkout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleCheckout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "valid return URL") {
		t.Fatalf("expected validation message, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleCheckoutRedirectsToStripe(t *testing.T) {
	capturedSuccessURL := ""
	h := NewTrialSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_test_123",
	})
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		if params == nil {
			t.Fatal("expected params")
		}
		if got := strings.TrimSpace(params.Metadata["org_id"]); got != "default" {
			t.Fatalf("metadata org_id=%q, want %q", got, "default")
		}
		capturedSuccessURL = strings.TrimSpace(stripe.StringValue(params.SuccessURL))
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	form := url.Values{
		"org_id":     {"default"},
		"return_url": {"https://pulse.example.com/auth/trial-activate"},
		"name":       {"Test User"},
		"email":      {"owner@example.com"},
		"company":    {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/checkout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleCheckout(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://checkout.stripe.com/c/pay/cs_test" {
		t.Fatalf("location=%q, want stripe checkout URL", loc)
	}
	if !strings.Contains(capturedSuccessURL, "session_id={CHECKOUT_SESSION_ID}") {
		t.Fatalf("success_url=%q, expected unescaped Stripe session placeholder", capturedSuccessURL)
	}
}

func TestTrialSignupHandleCompleteRedirectsWithActivationToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	h := NewTrialSignupHandlers(&CPConfig{
		StripeAPIKey:              "sk_test_123",
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv),
	})
	h.now = func() time.Time { return time.Unix(1710000000, 0).UTC() }
	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID:            id,
			Status:        stripe.CheckoutSessionStatusComplete,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":     "default",
				"return_url": "https://pulse.example.com/auth/trial-activate",
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_123", nil)
	rec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusSeeOther, rec.Body.String())
	}

	loc := rec.Header().Get("Location")
	parsed, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	token := strings.TrimSpace(parsed.Query().Get("token"))
	if token == "" {
		t.Fatalf("expected activation token in redirect URL")
	}

	claims, err := pkglicensing.VerifyTrialActivationToken(token, pub, "pulse.example.com", h.now().UTC())
	if err != nil {
		t.Fatalf("VerifyTrialActivationToken: %v", err)
	}
	if claims.OrgID != "default" {
		t.Fatalf("claims.OrgID=%q, want %q", claims.OrgID, "default")
	}
}
