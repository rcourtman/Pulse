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
	h := NewTrialSignupHandlers(&CPConfig{}, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/start-pro-trial?org_id=default&return_url=https://pulse.example.com:7655/auth/trial-activate&instance_token=tsi_test", nil)
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
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Test User"},
		"email":          {"owner@example.com"},
		"company":        {"Pulse Labs"},
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

func TestTrialSignupHandleRequestVerificationRejectsConsumerEmailDomain(t *testing.T) {
	h, _, sender := newTrialSignupTestHandler(t)
	form := url.Values{
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Test User"},
		"email":          {"owner@gmail.com"},
		"company":        {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleRequestVerification(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if sender.calls != 0 {
		t.Fatalf("email sender calls=%d, want 0", sender.calls)
	}
	if !strings.Contains(rec.Body.String(), "Consumer email addresses are not eligible") {
		t.Fatalf("expected consumer email rejection message, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleRequestVerificationRejectsPendingVerificationResend(t *testing.T) {
	h, _, sender := newTrialSignupTestHandler(t)
	form := url.Values{
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Test User"},
		"email":          {"owner@example.com"},
		"company":        {"Pulse Labs"},
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(form.Encode()))
	firstReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	firstRec := httptest.NewRecorder()
	h.HandleRequestVerification(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status=%d, want %d body=%q", firstRec.Code, http.StatusOK, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(form.Encode()))
	secondReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	secondRec := httptest.NewRecorder()
	h.HandleRequestVerification(secondRec, secondReq)

	if secondRec.Code != http.StatusTooManyRequests {
		t.Fatalf("second status=%d, want %d body=%q", secondRec.Code, http.StatusTooManyRequests, secondRec.Body.String())
	}
	if sender.calls != 1 {
		t.Fatalf("email sender calls=%d, want 1", sender.calls)
	}
	if !strings.Contains(secondRec.Body.String(), "verification email was already sent recently") {
		t.Fatalf("expected pending verification message, got %q", secondRec.Body.String())
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
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Test User"},
		"email":          {"owner@example.com"},
		"company":        {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleRequestVerification(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "organization has already used a Pulse Pro trial") {
		t.Fatalf("expected duplicate trial message, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleRequestVerificationRejectsCorporateDomainReuse(t *testing.T) {
	h, store, sender := newTrialSignupTestHandler(t)

	firstForm := url.Values{
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Alice Admin"},
		"email":          {"alice@acme.com"},
		"company":        {"Acme Inc."},
	}
	firstReq := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(firstForm.Encode()))
	firstReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	firstRec := httptest.NewRecorder()
	h.HandleRequestVerification(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first status=%d, want %d body=%q", firstRec.Code, http.StatusOK, firstRec.Body.String())
	}
	firstRawToken := extractQueryValueFromTextURL(t, sender.msg.Text, "token")
	firstVerifiedToken := verifyTrialRequest(t, h, firstRawToken)
	firstRequestID := parseVerifiedTokenRequestID(t, h, firstVerifiedToken)
	if err := store.MarkTrialIssued(firstRequestID, h.now().UTC()); err != nil {
		t.Fatalf("MarkTrialIssued(first): %v", err)
	}

	secondForm := url.Values{
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Bob Builder"},
		"email":          {"bob@acme.com"},
		"company":        {"Acme Holdings"},
	}
	secondReq := httptest.NewRequest(http.MethodPost, "/api/trial-signup/request-verification", strings.NewReader(secondForm.Encode()))
	secondReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	secondRec := httptest.NewRecorder()
	h.HandleRequestVerification(secondRec, secondReq)

	if secondRec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d body=%q", secondRec.Code, http.StatusConflict, secondRec.Body.String())
	}
	if !strings.Contains(secondRec.Body.String(), "organization has already used a Pulse Pro trial") {
		t.Fatalf("expected organization duplicate trial message, got %q", secondRec.Body.String())
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
		return &stripe.CheckoutSession{ID: "cs_test_new", URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
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
	if record.CheckoutSessionID != "cs_test_new" {
		t.Fatalf("checkout_session_id=%q, want %q", record.CheckoutSessionID, "cs_test_new")
	}
}

func TestTrialSignupHandleCheckoutReusesExistingOpenSession(t *testing.T) {
	h, store, sender := newTrialSignupTestHandler(t)
	h.cfg.StripeAPIKey = "sk_test_123"
	h.cfg.TrialSignupPriceID = "price_test_123"

	rawToken := requestTrialVerification(t, h, sender)
	verifiedToken := verifyTrialRequest(t, h, rawToken)
	recordID := parseVerifiedTokenRequestID(t, h, verifiedToken)
	if err := store.MarkCheckoutStarted(recordID, "cs_existing_open", h.now().UTC()); err != nil {
		t.Fatalf("MarkCheckoutStarted: %v", err)
	}

	createCalls := 0
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		createCalls++
		return &stripe.CheckoutSession{ID: "cs_should_not_exist", URL: "https://checkout.stripe.com/c/pay/cs_should_not_exist"}, nil
	}
	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		if id != "cs_existing_open" {
			t.Fatalf("getCheckoutSession id=%q, want %q", id, "cs_existing_open")
		}
		return &stripe.CheckoutSession{
			ID:     id,
			Status: stripe.CheckoutSessionStatusOpen,
			URL:    "https://checkout.stripe.com/c/pay/cs_existing_open",
		}, nil
	}

	form := url.Values{"verified_token": {verifiedToken}}
	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/checkout", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleCheckout(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://checkout.stripe.com/c/pay/cs_existing_open" {
		t.Fatalf("location=%q, want existing checkout URL", loc)
	}
	if createCalls != 0 {
		t.Fatalf("createCheckoutSession calls=%d, want 0", createCalls)
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
		InstanceToken:         "tsi_test",
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
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": record.ID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
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
	if claims.ReturnURL != "https://pulse.example.com/auth/trial-activate" {
		t.Fatalf("claims.ReturnURL=%q, want %q", claims.ReturnURL, "https://pulse.example.com/auth/trial-activate")
	}
	updatedRecord, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord(updated): %v", err)
	}
	if updatedRecord.CheckoutSessionID != "cs_test_123" {
		t.Fatalf("checkout_session_id=%q, want %q", updatedRecord.CheckoutSessionID, "cs_test_123")
	}
	if updatedRecord.CheckoutCompletedAt.IsZero() {
		t.Fatalf("expected checkout_completed_at to be recorded")
	}
	if updatedRecord.ActivationIssuedAt.IsZero() {
		t.Fatalf("expected activation_issued_at to be recorded")
	}
	if strings.TrimSpace(updatedRecord.ActivationToken) == "" {
		t.Fatalf("expected activation_token to be persisted")
	}
	if updatedRecord.ActivationToken != token {
		t.Fatalf("activation_token=%q, want issued redirect token", updatedRecord.ActivationToken)
	}
}

func TestTrialSignupHandleCompleteReusesStoredActivationToken(t *testing.T) {
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
		InstanceToken:         "tsi_test",
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
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": record.ID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
			},
		}, nil
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_reuse", nil)
	firstRec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(firstRec, firstReq)
	if firstRec.Code != http.StatusSeeOther {
		t.Fatalf("first status=%d, want %d body=%q", firstRec.Code, http.StatusSeeOther, firstRec.Body.String())
	}
	firstLocation := firstRec.Header().Get("Location")
	firstParsed, err := url.Parse(firstLocation)
	if err != nil {
		t.Fatalf("parse first redirect: %v", err)
	}
	firstIssuedToken := strings.TrimSpace(firstParsed.Query().Get("token"))
	if firstIssuedToken == "" {
		t.Fatal("expected first activation token")
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_reuse", nil)
	secondRec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(secondRec, secondReq)
	if secondRec.Code != http.StatusSeeOther {
		t.Fatalf("second status=%d, want %d body=%q", secondRec.Code, http.StatusSeeOther, secondRec.Body.String())
	}
	secondLocation := secondRec.Header().Get("Location")
	secondParsed, err := url.Parse(secondLocation)
	if err != nil {
		t.Fatalf("parse second redirect: %v", err)
	}
	secondIssuedToken := strings.TrimSpace(secondParsed.Query().Get("token"))
	if secondIssuedToken == "" {
		t.Fatal("expected second activation token")
	}
	if secondIssuedToken != firstIssuedToken {
		t.Fatalf("second token=%q, want first token=%q", secondIssuedToken, firstIssuedToken)
	}

	claims, err := pkglicensing.VerifyTrialActivationToken(secondIssuedToken, pub, "pulse.example.com", h.now().UTC())
	if err != nil {
		t.Fatalf("VerifyTrialActivationToken: %v", err)
	}
	if claims.Subject != "cs_test_reuse" {
		t.Fatalf("claims.Subject=%q, want %q", claims.Subject, "cs_test_reuse")
	}
	if claims.ReturnURL != "https://pulse.example.com/auth/trial-activate" {
		t.Fatalf("claims.ReturnURL=%q, want %q", claims.ReturnURL, "https://pulse.example.com/auth/trial-activate")
	}
}

func TestTrialSignupHandleCompleteRotatesExpiredActivationToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	baseNow := time.Unix(1710000000, 0).UTC()
	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Test User",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             baseNow,
		VerificationExpiresAt: baseNow.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, baseNow)
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}

	now := baseNow
	h := NewTrialSignupHandlers(&CPConfig{
		StripeAPIKey:              "sk_test_123",
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv),
	}, nil, store)
	h.now = func() time.Time { return now }
	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID:            id,
			Status:        stripe.CheckoutSessionStatusComplete,
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": record.ID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
			},
		}, nil
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_rotate", nil)
	firstRec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(firstRec, firstReq)
	if firstRec.Code != http.StatusSeeOther {
		t.Fatalf("first status=%d, want %d body=%q", firstRec.Code, http.StatusSeeOther, firstRec.Body.String())
	}
	firstLocation := firstRec.Header().Get("Location")
	firstParsed, err := url.Parse(firstLocation)
	if err != nil {
		t.Fatalf("parse first redirect: %v", err)
	}
	firstIssuedToken := strings.TrimSpace(firstParsed.Query().Get("token"))
	if firstIssuedToken == "" {
		t.Fatal("expected first activation token")
	}

	now = now.Add(trialSignupActivationTokenTTL + time.Minute)

	secondReq := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_rotate", nil)
	secondRec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(secondRec, secondReq)
	if secondRec.Code != http.StatusSeeOther {
		t.Fatalf("second status=%d, want %d body=%q", secondRec.Code, http.StatusSeeOther, secondRec.Body.String())
	}
	secondLocation := secondRec.Header().Get("Location")
	secondParsed, err := url.Parse(secondLocation)
	if err != nil {
		t.Fatalf("parse second redirect: %v", err)
	}
	secondIssuedToken := strings.TrimSpace(secondParsed.Query().Get("token"))
	if secondIssuedToken == "" {
		t.Fatal("expected second activation token")
	}
	if secondIssuedToken == firstIssuedToken {
		t.Fatal("expected expired activation token to be rotated")
	}

	claims, err := pkglicensing.VerifyTrialActivationToken(secondIssuedToken, pub, "pulse.example.com", now)
	if err != nil {
		t.Fatalf("VerifyTrialActivationToken: %v", err)
	}
	if claims.Subject != "cs_test_rotate" {
		t.Fatalf("claims.Subject=%q, want %q", claims.Subject, "cs_test_rotate")
	}
	if claims.ReturnURL != "https://pulse.example.com/auth/trial-activate" {
		t.Fatalf("claims.ReturnURL=%q, want %q", claims.ReturnURL, "https://pulse.example.com/auth/trial-activate")
	}
}

func TestTrialSignupHandleRedeemRecordsRedemption(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	rawToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Test User",
		Email:                 "owner@example.com",
		Company:               "Pulse Labs",
		CreatedAt:             now,
		VerificationExpiresAt: now.Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification: %v", err)
	}
	record, err := store.ConsumeVerification(rawToken, now)
	if err != nil {
		t.Fatalf("ConsumeVerification: %v", err)
	}
	if err := store.MarkCheckoutCompleted(record.ID, "cs_test_redeem", now); err != nil {
		t.Fatalf("MarkCheckoutCompleted: %v", err)
	}

	activationToken, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: "tsi_test",
		ReturnURL:     "https://pulse.example.com/auth/trial-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_test_redeem",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(trialSignupActivationTokenTTL)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}
	if _, _, err := store.StoreOrRotateActivationToken(record.ID, activationToken, now, trialSignupActivationTokenTTL); err != nil {
		t.Fatalf("StoreOrRotateActivationToken(actual): %v", err)
	}

	h := NewTrialSignupHandlers(&CPConfig{
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv),
	}, nil, store)
	h.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/redeem", strings.NewReader(`{"token":"`+activationToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleTrialSignupRedeem(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	updatedRecord, err := store.GetRecord(record.ID)
	if err != nil {
		t.Fatalf("GetRecord(updated): %v", err)
	}
	if updatedRecord.RedemptionRecordedAt.IsZero() {
		t.Fatalf("expected redemption_recorded_at to be set")
	}

	claims, err := pkglicensing.VerifyTrialActivationToken(activationToken, pub, "pulse.example.com", now)
	if err != nil {
		t.Fatalf("VerifyTrialActivationToken: %v", err)
	}
	if claims.ReturnURL != "https://pulse.example.com/auth/trial-activate" {
		t.Fatalf("claims.ReturnURL=%q, want %q", claims.ReturnURL, "https://pulse.example.com/auth/trial-activate")
	}
}

func TestTrialSignupHandleRedeemRejectsHostMismatch(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	store, err := NewTrialSignupStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewTrialSignupStore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	now := time.Unix(1710000000, 0).UTC()
	activationToken, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "wrong.example.com",
		InstanceToken: "tsi_test",
		ReturnURL:     "https://pulse.example.com/auth/trial-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_bad_host",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(trialSignupActivationTokenTTL)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	h := NewTrialSignupHandlers(&CPConfig{
		TrialActivationPrivateKey: base64.StdEncoding.EncodeToString(priv),
	}, nil, store)
	h.now = func() time.Time { return now }

	req := httptest.NewRequest(http.MethodPost, "/api/trial-signup/redeem", strings.NewReader(`{"token":"`+activationToken+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleTrialSignupRedeem(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "host mismatch") {
		t.Fatalf("expected host mismatch error, got %q", rec.Body.String())
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
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": secondRequestID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
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

func TestTrialSignupHandleCompleteRejectsDuplicateCorporateDomainIssuedEmail(t *testing.T) {
	h, store, _ := newTrialSignupTestHandler(t)
	h.cfg.StripeAPIKey = "sk_test_123"

	firstToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Alice Admin",
		Email:                 "alice@acme.com",
		Company:               "Acme Inc.",
		CreatedAt:             h.now().UTC(),
		VerificationExpiresAt: h.now().UTC().Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification(first): %v", err)
	}
	firstRecord, err := store.ConsumeVerification(firstToken, h.now().UTC())
	if err != nil {
		t.Fatalf("ConsumeVerification(first): %v", err)
	}
	if err := store.MarkTrialIssued(firstRecord.ID, h.now().UTC()); err != nil {
		t.Fatalf("MarkTrialIssued(first): %v", err)
	}

	secondToken, err := store.CreateVerification(&TrialSignupRecord{
		OrgID:                 "default",
		ReturnURL:             "https://pulse.example.com/auth/trial-activate",
		InstanceToken:         "tsi_test",
		Name:                  "Bob Builder",
		Email:                 "bob@acme.com",
		Company:               "Acme Holdings",
		CreatedAt:             h.now().UTC(),
		VerificationExpiresAt: h.now().UTC().Add(trialSignupVerificationTTL),
	})
	if err != nil {
		t.Fatalf("CreateVerification(second): %v", err)
	}
	secondRecord, err := store.ConsumeVerification(secondToken, h.now().UTC())
	if err != nil {
		t.Fatalf("ConsumeVerification(second): %v", err)
	}

	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID:            id,
			Status:        stripe.CheckoutSessionStatusComplete,
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "bob@acme.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": secondRecord.ID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_domain_dupe", nil)
	rec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusConflict, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "trial already used for this organization") {
		t.Fatalf("expected organization duplicate issuance error, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleCompleteRejectsCheckoutSessionMismatch(t *testing.T) {
	h, store, sender := newTrialSignupTestHandler(t)
	h.cfg.StripeAPIKey = "sk_test_123"

	rawToken := requestTrialVerification(t, h, sender)
	verifiedToken := verifyTrialRequest(t, h, rawToken)
	requestID := parseVerifiedTokenRequestID(t, h, verifiedToken)
	if err := store.MarkCheckoutStarted(requestID, "cs_bound_original", h.now().UTC()); err != nil {
		t.Fatalf("MarkCheckoutStarted: %v", err)
	}

	h.getCheckoutSession = func(id string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{
			ID:            id,
			Status:        stripe.CheckoutSessionStatusComplete,
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": requestID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_other_session", nil)
	rec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "checkout session mismatch") {
		t.Fatalf("expected checkout session mismatch, got %q", rec.Body.String())
	}
}

func TestTrialSignupHandleCompleteRejectsNonTrialCheckoutSession(t *testing.T) {
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
		InstanceToken:         "tsi_test",
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
			Mode:          stripe.CheckoutSessionModePayment,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": record.ID,
				"signup_source":    "pulse_pro_trial",
				"email_verified":   "true",
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_non_trial", nil)
	rec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid checkout session mode") {
		t.Fatalf("expected invalid mode error, got %q", rec.Body.String())
	}

	_ = pub
}

func TestTrialSignupHandleCompleteRejectsMissingVerifiedTrialMetadata(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(nil)
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
		InstanceToken:         "tsi_test",
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
			Mode:          stripe.CheckoutSessionModeSubscription,
			CustomerEmail: "owner@example.com",
			Metadata: map[string]string{
				"org_id":           "default",
				"return_url":       "https://pulse.example.com/auth/trial-activate",
				"trial_request_id": record.ID,
				"signup_source":    "something_else",
				"email_verified":   "false",
			},
		}, nil
	}

	req := httptest.NewRequest(http.MethodGet, "/trial-signup/complete?session_id=cs_test_bad_metadata", nil)
	rec := httptest.NewRecorder()
	h.HandleTrialSignupComplete(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid trial signup source") {
		t.Fatalf("expected invalid signup source error, got %q", rec.Body.String())
	}
}

func newTrialSignupTestHandler(t *testing.T) (*TrialSignupHandlers, *TrialSignupStore, *captureEmailSender) {
	t.Helper()

	_, activationPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey activation: %v", err)
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
		EmailFrom:                 "noreply@pulserelay.pro",
	}, sender, store)
	h.now = func() time.Time { return time.Unix(1710000000, 0).UTC() }
	return h, store, sender
}

func requestTrialVerification(t *testing.T, h *TrialSignupHandlers, sender *captureEmailSender) string {
	t.Helper()

	form := url.Values{
		"org_id":         {"default"},
		"return_url":     {"https://pulse.example.com/auth/trial-activate"},
		"instance_token": {"tsi_test"},
		"name":           {"Test User"},
		"email":          {"owner@example.com"},
		"company":        {"Pulse Labs"},
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
	record, err := h.lookupVerifiedTrialSignupRecord(verifiedToken)
	if err != nil {
		t.Fatalf("lookupVerifiedTrialSignupRecord: %v", err)
	}
	return record.ID
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

func TestIsValidTrialReturnURL(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "https callback", raw: "https://pulse.example.com/auth/trial-activate", want: true},
		{name: "private http callback", raw: "http://192.168.0.98:7655/auth/trial-activate", want: true},
		{name: "localhost http callback", raw: "http://localhost:7655/auth/trial-activate", want: true},
		{name: "local dns http callback", raw: "http://pulse.local:7655/auth/trial-activate", want: true},
		{name: "wrong path", raw: "https://pulse.example.com/settings", want: false},
		{name: "query not allowed", raw: "https://pulse.example.com/auth/trial-activate?next=1", want: false},
		{name: "fragment not allowed", raw: "https://pulse.example.com/auth/trial-activate#token=x", want: false},
		{name: "public http not allowed", raw: "http://pulse.example.com/auth/trial-activate", want: false},
		{name: "arbitrary https path under callback prefix not allowed", raw: "https://pulse.example.com/auth/trial-activate/extra", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidTrialReturnURL(tt.raw); got != tt.want {
				t.Fatalf("isValidTrialReturnURL(%q)=%v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
