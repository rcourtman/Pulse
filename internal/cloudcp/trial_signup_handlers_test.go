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

	"github.com/golang-jwt/jwt/v5"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	stripe "github.com/stripe/stripe-go/v82"
)

func TestTrialSignupHandleStartProTrialRendersVerificationForm(t *testing.T) {
	h := NewTrialSignupHandlers(&CPConfig{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/start-pro-trial?org_id=default&return_url=https://pulse.example.com:7655/auth/trial-activate", nil)
	rec := httptest.NewRecorder()

	h.HandleStartProTrial(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Verify your work email") {
		t.Fatalf("expected verification CTA in response body")
	}
	if !strings.Contains(body, "pulse.example.com:7655") {
		t.Fatalf("expected instance host in response body")
	}
	if strings.Contains(body, "Activation Return URL") {
		t.Fatalf("raw activation callback should not be shown")
	}
}

func TestTrialSignupHandleRequestVerificationSendsEmail(t *testing.T) {
	h, _ := newTrialSignupTestHandler(t)
	form := url.Values{
		"org_id":     {"default"},
		"return_url": {"https://pulse.example.com/auth/trial-activate"},
		"name":       {"Test User"},
		"email":      {"owner@example.com"},
		"company":    {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleRequestVerification(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	if h.emailSender.(*captureEmailSender).calls != 1 {
		t.Fatalf("email sender calls=%d, want 1", h.emailSender.(*captureEmailSender).calls)
	}
	if !strings.Contains(rec.Body.String(), "Check owner@example.com for a verification link") {
		t.Fatalf("expected verification sent message")
	}
	if !strings.Contains(h.emailSender.(*captureEmailSender).msg.Text, "/trial-signup/verify?token=") {
		t.Fatalf("expected verification email to contain verify URL")
	}
}

func TestTrialSignupHandleVerifyEmailRendersVerifiedState(t *testing.T) {
	h, priv := newTrialSignupTestHandler(t)
	token := mustSignTrialVerificationToken(t, h, priv, trialSignupVerificationClaims{
		OrgID:            "default",
		ReturnURL:        "https://pulse.example.com/auth/trial-activate",
		Name:             "Test User",
		Email:            "owner@example.com",
		Company:          "Pulse Labs",
		RegisteredClaims: registeredClaimsFor(h.now().UTC()),
	})

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/verify?token="+url.QueryEscape(token), nil)
	rec := httptest.NewRecorder()

	h.HandleVerifyEmail(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Email verified") || !strings.Contains(body, "Continue To Secure Setup") {
		t.Fatalf("expected verified state in response body")
	}
}

func TestTrialSignupHandleCheckoutRejectsUnverifiedRequest(t *testing.T) {
	h, _ := newTrialSignupTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/checkout", strings.NewReader(url.Values{}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleCheckout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Please verify your work email") {
		t.Fatalf("expected verification error, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleCheckoutRedirectsToStripe(t *testing.T) {
	h, priv := newTrialSignupTestHandler(t)
	capturedSuccessURL := ""
	capturedCancelURL := ""
	h.cfg.StripeAPIKey = "sk_test_123"
	h.cfg.TrialSignupPriceID = "price_test_123"
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		if params == nil {
			t.Fatal("expected params")
		}
		if got := strings.TrimSpace(params.Metadata["org_id"]); got != "default" {
			t.Fatalf("metadata org_id=%q, want %q", got, "default")
		}
		if got := strings.TrimSpace(params.Metadata["email_verified"]); got != "true" {
			t.Fatalf("metadata email_verified=%q, want %q", got, "true")
		}
		capturedSuccessURL = strings.TrimSpace(stripe.StringValue(params.SuccessURL))
		capturedCancelURL = strings.TrimSpace(stripe.StringValue(params.CancelURL))
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	verifiedToken := mustSignTrialVerificationToken(t, h, priv, trialSignupVerificationClaims{
		OrgID:            "default",
		ReturnURL:        "https://pulse.example.com/auth/trial-activate",
		Name:             "Test User",
		Email:            "owner@example.com",
		Company:          "Pulse Labs",
		RegisteredClaims: registeredClaimsFor(h.now().UTC()),
	})

	form := url.Values{
		"verified_token": {verifiedToken},
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
	if !strings.Contains(capturedCancelURL, "/trial-signup/verify?") || !strings.Contains(capturedCancelURL, "cancelled=1") {
		t.Fatalf("cancel_url=%q, expected verify page return", capturedCancelURL)
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
	}, nil)
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

func newTrialSignupTestHandler(t *testing.T) (*TrialSignupHandlers, ed25519.PrivateKey) {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	sender := &captureEmailSender{}
	h := NewTrialSignupHandlers(&CPConfig{
		BaseURL:                   "https://cloud.example.com",
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv),
		EmailFrom:                 "noreply@pulserelay.pro",
	}, sender)
	h.now = func() time.Time { return time.Unix(1710000000, 0).UTC() }
	return h, priv
}

func mustSignTrialVerificationToken(t *testing.T, h *TrialSignupHandlers, privateKey ed25519.PrivateKey, claims trialSignupVerificationClaims) string {
	t.Helper()

	token, err := h.signTrialSignupVerificationToken(privateKey, claims)
	if err != nil {
		t.Fatalf("signTrialSignupVerificationToken: %v", err)
	}
	return token
}

func registeredClaimsFor(now time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(trialSignupVerificationTTL)),
		Subject:   "owner@example.com",
	}
}
