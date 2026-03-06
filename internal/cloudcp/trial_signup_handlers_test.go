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

func TestTrialSignupHandleStartProTrialRendersVerificationForm(t *testing.T) {
	h := NewTrialSignupHandlers(&CPConfig{}, nil, nil)
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
	h, _, sender := newTrialSignupTestHandler(t)
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
	if sender.calls != 1 {
		t.Fatalf("email sender calls=%d, want 1", sender.calls)
	}
	if !strings.Contains(rec.Body.String(), "Check owner@example.com for a verification link") {
		t.Fatalf("expected verification sent message")
	}
	if !strings.Contains(sender.msg.Text, "/trial-signup/verify?token=") {
		t.Fatalf("expected verification email to contain verify URL")
	}
}

func TestTrialSignupHandleRequestVerificationRejectsEmailThatAlreadyUsedTrial(t *testing.T) {
	h, store, sender := newTrialSignupTestHandler(t)
	rawToken := requestTrialVerification(t, h, sender)
	verifiedToken := verifyTrialRequest(t, h, rawToken)
	recordID := parseVerifiedTokenRequestID(t, h, verifiedToken)
	if err := store.MarkTrialIssued(recordID, h.now().UTC()); err != nil {
		t.Fatalf("MarkTrialIssued: %v", err)
	}

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

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "already used a Pulse Pro trial") {
		t.Fatalf("expected duplicate trial message, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleVerifyEmailConsumesSingleUseToken(t *testing.T) {
	h, _, sender := newTrialSignupTestHandler(t)
	rawToken := requestTrialVerification(t, h, sender)

	firstReq := httptest.NewRequest(http.MethodGet, "/trial-signup/verify?token="+url.QueryEscape(rawToken), nil)
	firstRec := httptest.NewRecorder()
	h.HandleVerifyEmail(firstRec, firstReq)
	if firstRec.Code != http.StatusSeeOther {
		t.Fatalf("first verify status=%d, want %d body=%q", firstRec.Code, http.StatusSeeOther, firstRec.Body.String())
	}
	location := firstRec.Header().Get("Location")
	if !strings.Contains(location, "/trial-signup/verify?verified=") {
		t.Fatalf("location=%q, want verified redirect", location)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/trial-signup/verify?token="+url.QueryEscape(rawToken), nil)
	secondRec := httptest.NewRecorder()
	h.HandleVerifyEmail(secondRec, secondReq)
	if secondRec.Code != http.StatusBadRequest {
		t.Fatalf("second verify status=%d, want %d body=%q", secondRec.Code, http.StatusBadRequest, secondRec.Body.String())
	}
	if !strings.Contains(secondRec.Body.String(), "invalid or expired") {
		t.Fatalf("expected invalid link message, got %q", secondRec.Body.String())
	}
}

func TestTrialSignupHandleVerifyEmailRendersVerifiedState(t *testing.T) {
	h, _, sender := newTrialSignupTestHandler(t)
	rawToken := requestTrialVerification(t, h, sender)
	verifiedToken := verifyTrialRequest(t, h, rawToken)

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/verify?verified="+url.QueryEscape(verifiedToken), nil)
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
	h, _, _ := newTrialSignupTestHandler(t)
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
	h, store, sender := newTrialSignupTestHandler(t)
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
		if strings.TrimSpace(params.Metadata["trial_request_id"]) == "" {
			t.Fatalf("expected trial_request_id metadata")
		}
		capturedSuccessURL = strings.TrimSpace(stripe.StringValue(params.SuccessURL))
		capturedCancelURL = strings.TrimSpace(stripe.StringValue(params.CancelURL))
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	rawToken := requestTrialVerification(t, h, sender)
	verifiedToken := verifyTrialRequest(t, h, rawToken)

	form := url.Values{"verified_token": {verifiedToken}}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/checkout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleCheckout(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://checkout.stripe.com/c/pay/cs_test" {
		t.Fatalf("location=%q, want stripe checkout URL", loc)
	}
	if !strings.Contains(capturedSuccessURL, "session_id={CHECKOUT_SESSION_ID}") {
		t.Fatalf("success_url=%q, expected unescaped Stripe session placeholder", capturedSuccessURL)
	}
	if !strings.Contains(capturedCancelURL, "/trial-signup/verify?") || !strings.Contains(capturedCancelURL, "verified=") || !strings.Contains(capturedCancelURL, "cancelled=1") {
		t.Fatalf("cancel_url=%q, expected verified page return", capturedCancelURL)
	}

	recordID := parseVerifiedTokenRequestID(t, h, verifiedToken)
	record, err := store.GetRecord(recordID)
	if err != nil {
		t.Fatalf("GetRecord: %v", err)
	}
	if record.CheckoutStartedAt.IsZero() {
		t.Fatalf("expected checkout_started_at to be recorded")
	}
}

func TestTrialSignupHandleCompleteRedirectsWithActivationToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		Name:                  "Test User",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             time.Unix(1710000000, 0).UTC(),
		VerificationExpiresAt: time.Unix(1710000000, 0).UTC().Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, time.Unix(1710000000, 0).UTC())
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}

	h := NewTrialSignupHandlers(&CPConfig{
		StripeAPIKey:              "sk_test_123",
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv),
	}, nil, store)
	h.now = func() time.Time { return time.Unix(1710000000, 0).UTC() }
	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID:            id,
			Status:        stripe.CheckoutSessionStatusComplete,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": record.ID,
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

func TestTrialSignupHandleCompleteRejectsDuplicateIssuedEmail(t *testing.T) {
	h, store, sender := newTrialSignupTestHandler(t)
	h.cfg.StripeAPIKey = "sk_test_123"

	firstRawToken := requestTrialVerification(t, h, sender)
	firstVerifiedToken := verifyTrialRequest(t, h, firstRawToken)
	firstRequestID := parseVerifiedTokenRequestID(t, h, firstVerifiedToken)

	secondRawToken := requestTrialVerification(t, h, sender)
	secondVerifiedToken := verifyTrialRequest(t, h, secondRawToken)
	secondRequestID := parseVerifiedTokenRequestID(t, h, secondVerifiedToken)
	if err := store.MarkTrialIssued(firstRequestID, h.now().UTC()); err != nil {
		t.Fatalf("MarkTrialIssued(first): %v", err)
	}

	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID:            id,
			Status:        stripe.CheckoutSessionStatusComplete,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": secondRequestID,
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_dupe", nil)
	rec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "trial already used for this email") {
		t.Fatalf("expected duplicate issuance error, got %q", rec.Body.String())
	}
}

func newTrialSignupTestHandler(t *testing.T) (*TrialSignupHandlers, *TrialSignupStore, *captureEmailSender) {
	t.Helper()

	_, activationPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey activation: %v", err)
	}
	_, checkoutPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey checkout: %v", err)
	}
	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	sender := &captureEmailSender{}
	h := NewTrialSignupHandlers(&CPConfig{
		BaseURL:                   "https://cloud.example.com",
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(activationPriv),
		TrialCheckoutPrivateKey:   base64.StdEncoding.EncodeToString(checkoutPriv),
		EmailFrom:                 "noreply@pulserelay.pro",
	}, sender, store)
	h.now = func() time.Time { return time.Unix(1710000000, 0).UTC() }
	return h, store, sender
}

func requestTrialVerification(t *testing.T, h *TrialSignupHandlers, sender *captureEmailSender) string {
	t.Helper()

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
		t.Fatalf("request verification status=%d body=%q", rec.Code, rec.Body.String())
	}

	rawToken := extractQueryValueFromTextURL(t, sender.msg.Text, "token")
	if rawToken == "" {
		t.Fatalf("expected token in verification email body")
	}
	return rawToken
}

func verifyTrialRequest(t *testing.T, h *TrialSignupHandlers, rawToken string) string {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/verify?token="+url.QueryEscape(rawToken), nil)
	rec := httptest.NewRecorder()
	h.HandleVerifyEmail(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("verify request status=%d body=%q", rec.Code, rec.Body.String())
	}
	verifiedToken := extractQueryValueFromURL(t, rec.Header().Get("Location"), "verified")
	if verifiedToken == "" {
		t.Fatalf("expected verified token in redirect URL")
	}
	return verifiedToken
}

func parseVerifiedTokenRequestID(t *testing.T, h *TrialSignupHandlers, verifiedToken string) string {
	t.Helper()
	claims, err := h.verifyTrialSignupCheckoutToken(verifiedToken)
	if err != nil {
		t.Fatalf("verifyTrialSignupCheckoutToken: %v", err)
	}
	return claims.RequestID
}

func extractQueryValueFromTextURL(t *testing.T, text, key string) string {
	t.Helper()
	start := strings.Index(text, "https://")
	if start == -1 {
		t.Fatalf("no URL found in %q", text)
	}
	rawURL := strings.Fields(text[start:])[0]
	return extractQueryValueFromURL(t, rawURL, key)
}

func extractQueryValueFromURL(t *testing.T, rawURL, key string) string {
	t.Helper()
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		t.Fatalf("Parse URL %q: %v", rawURL, err)
	}
	return strings.TrimSpace(parsed.Query().Get(key))
}
