package cloudcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	stripe "github.com/stripe/stripe-go/v82"
)

const (
	testMSPStarterPriceID = "price_1T5kgTBrHBocJIGHjOs15LI2"
	testMSPGrowthPriceID  = "price_1T5kgVBrHBocJIGHulNsCTb1"
	testMSPScalePriceID   = "price_1T5kgWBrHBocJIGHo40iFeRd"
)

func TestParseMSPTier(t *testing.T) {
	tests := []struct {
		input  string
		want   mspTier
		wantOK bool
	}{
		{"", mspTierStarter, true},
		{"starter", mspTierStarter, true},
		{"STARTER", mspTierStarter, true},
		{" Growth ", mspTierGrowth, true},
		{"growth", mspTierGrowth, true},
		{"scale", mspTierScale, true},
		{"SCALE", mspTierScale, true},
		{"power", "", false},
		{"max", "", false},
		{"enterprise", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseMSPTier(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseMSPTier(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("parseMSPTier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPriceIDForMSPTier(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPGrowthPriceID:  testMSPGrowthPriceID,
		CloudMSPScalePriceID:   testMSPScalePriceID,
	}, nil, nil, nil)

	tests := []struct {
		tier mspTier
		want string
	}{
		{mspTierStarter, testMSPStarterPriceID},
		{mspTierGrowth, testMSPGrowthPriceID},
		{mspTierScale, testMSPScalePriceID},
	}
	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got, ok := h.priceIDForMSPTier(tt.tier)
			if !ok || got != tt.want {
				t.Fatalf("priceIDForMSPTier(%q) = (%q,%v), want (%q,true)", tt.tier, got, ok, tt.want)
			}
		})
	}
}

func TestMSPSignupPageRendersUnavailableWhenNoTierConfigured(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/cloud/msp/signup", nil)
	rec := httptest.NewRecorder()

	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<form") {
		t.Fatal("expected no signup form when no MSP tier is configured")
	}
	if !strings.Contains(body, "not open for public self-serve purchase yet") {
		t.Fatalf("expected unavailable notice, got %q", body)
	}
}

func TestMSPSignupPageRendersFormWhenTierConfigured(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		CloudMSPStarterPriceID: testMSPStarterPriceID,
	}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/cloud/msp/signup", nil)
	rec := httptest.NewRecorder()

	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<form") {
		t.Fatal("expected signup form when an MSP tier is configured")
	}
	if !strings.Contains(body, "Request Pulse MSP Access") {
		t.Fatal("expected MSP access request heading")
	}
	if strings.Contains(strings.ToLower(body), "trial") {
		t.Fatal("MSP signup page should not advertise a trial")
	}
	if !strings.Contains(body, `action="/cloud/msp/signup"`) {
		t.Fatal("expected form to post to the canonical MSP signup path")
	}
	// Single tier configured: hidden input, no radios.
	if strings.Contains(body, `type="radio"`) {
		t.Fatal("expected no tier radios when only one MSP tier is configured")
	}
	if !strings.Contains(body, `type="hidden" name="tier" value="starter"`) {
		t.Fatal("expected hidden starter tier input when only starter is configured")
	}
}

func TestMSPSignupPageKeepsStarterSelfServeWhenMultipleTiersConfigured(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPGrowthPriceID:  testMSPGrowthPriceID,
		CloudMSPScalePriceID:   testMSPScalePriceID,
	}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/cloud/msp/signup?tier=growth", nil)
	rec := httptest.NewRecorder()

	h.HandleMSPSignupPage(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, `type="radio"`) {
		t.Fatal("expected no tier radios because only MSP Starter request path is exposed")
	}
	if !strings.Contains(body, `type="hidden" name="tier" value="starter"`) {
		t.Fatal("expected hidden starter tier input")
	}
	if !strings.Contains(body, "Growth") || !strings.Contains(body, "$249/mo") || !strings.Contains(body, "request-assisted access") {
		t.Fatal("expected assisted Growth copy")
	}
	if !strings.Contains(body, "up to 5 client workspaces") {
		t.Fatal("expected canonical MSP Starter workspace limit copy")
	}
	if !strings.Contains(body, "Scale") || !strings.Contains(body, "$399/mo") || !strings.Contains(body, "up to 40 client workspaces") {
		t.Fatal("expected assisted Scale copy")
	}
}

func TestMSPSignupPostValidRedirectsToStripeWithMSPMetadata(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testMSPStarterPriceID,
	}, nil, nil, nil)

	var captured *stripe.CheckoutSessionParams
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		captured = params
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_msp"}, nil
	}

	form := url.Values{"email": {"owner@example.com"}, "org_name": {"Quesys MSP"}}
	req := httptest.NewRequest(http.MethodPost, "/cloud/msp/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://checkout.stripe.com/c/pay/cs_msp" {
		t.Fatalf("location=%q, want stripe URL", loc)
	}
	if captured == nil || captured.Metadata == nil {
		t.Fatal("expected checkout metadata")
	}
	if captured.SubscriptionData != nil {
		t.Fatalf("MSP checkout should not set trial subscription data, got %+v", captured.SubscriptionData)
	}
	meta := captured.Metadata
	if got := meta["account_kind"]; got != "msp" {
		t.Fatalf("account_kind=%q, want %q", got, "msp")
	}
	if got := meta["plan_version"]; got != "msp_starter" {
		t.Fatalf("plan_version=%q, want %q", got, "msp_starter")
	}
	if got := meta["signup_source"]; got != "public_msp_signup" {
		t.Fatalf("signup_source=%q, want %q", got, "public_msp_signup")
	}
	if got := meta["account_display_name"]; got != "Quesys MSP" {
		t.Fatalf("account_display_name=%q, want %q", got, "Quesys MSP")
	}
	if got := meta[checkoutBillingModeMetadataKey]; got != checkoutBillingModeImmediate {
		t.Fatalf("checkout_billing_mode=%q, want %q", got, checkoutBillingModeImmediate)
	}
}

func TestMSPSignupPostTierSelectionPicksPrice(t *testing.T) {
	cfg := &CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPGrowthPriceID:  testMSPGrowthPriceID,
		CloudMSPScalePriceID:   testMSPScalePriceID,
	}
	tests := []struct {
		tier        string
		wantPriceID string
	}{
		{"", testMSPStarterPriceID},
		{"starter", testMSPStarterPriceID},
	}
	for _, tt := range tests {
		t.Run("tier="+tt.tier, func(t *testing.T) {
			h := NewPublicCloudSignupHandlers(cfg, nil, nil, nil)
			var priceID string
			h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
				if len(params.LineItems) > 0 && params.LineItems[0].Price != nil {
					priceID = *params.LineItems[0].Price
				}
				return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
			}
			form := url.Values{"email": {"o@example.com"}, "org_name": {"Quesys MSP"}, "tier": {tt.tier}}
			req := httptest.NewRequest(http.MethodPost, "/cloud/msp/signup", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.HandleMSPSignupPage(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
			}
			if priceID != tt.wantPriceID {
				t.Fatalf("price_id=%q, want %q", priceID, tt.wantPriceID)
			}
		})
	}
}

func TestMSPSignupPostAssistedTierReturns400EvenWhenConfigured(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPGrowthPriceID:  testMSPGrowthPriceID,
		CloudMSPScalePriceID:   testMSPScalePriceID,
	}, nil, nil, nil)

	form := url.Values{"email": {"o@example.com"}, "org_name": {"Quesys MSP"}, "tier": {"growth"}}
	req := httptest.NewRequest(http.MethodPost, "/cloud/msp/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "request-assisted") {
		t.Fatalf("expected assisted-tier message, got %q", rec.Body.String())
	}
}

func TestMSPSignupPostUnconfiguredStarterReturns400(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:      "https://cloud.example.com",
		StripeAPIKey: "sk_test_123",
	}, nil, nil, nil)

	form := url.Values{"email": {"o@example.com"}, "org_name": {"Quesys MSP"}, "tier": {"starter"}}
	req := httptest.NewRequest(http.MethodPost, "/cloud/msp/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Starter access is not currently available") {
		t.Fatalf("expected tier-unavailable message, got %q", rec.Body.String())
	}
}

func TestMSPSignupRejectsCloudPriceMisconfiguration(t *testing.T) {
	// MSP starter price slot misconfigured with a cloud_starter price ID.
	// Must fail closed before calling Stripe: the MSP path only accepts
	// msp_* plan versions.
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testCloudStarterPriceID, // cloud_starter, not msp_*
	}, nil, nil, nil)

	stripeCalled := false
	h.createCheckoutSession = func(_ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		stripeCalled = true
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
	}

	form := url.Values{"email": {"o@example.com"}, "org_name": {"Quesys MSP"}}
	req := httptest.NewRequest(http.MethodPost, "/cloud/msp/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusBadGateway)
	}
	if stripeCalled {
		t.Fatal("expected cloud-price misconfiguration to fail closed before calling Stripe")
	}
}

func TestMSPSignupAPICreatesCheckout(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPGrowthPriceID:  testMSPGrowthPriceID,
	}, nil, nil, nil)

	var captured *stripe.CheckoutSessionParams
	var priceID string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		captured = params
		if len(params.LineItems) > 0 && params.LineItems[0].Price != nil {
			priceID = *params.LineItems[0].Price
		}
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_api"}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/public/msp/signup", strings.NewReader(`{"email":"o@example.com","org_name":"Quesys MSP","tier":"starter"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleMSPPublicSignup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if captured == nil {
		t.Fatal("expected checkout session params")
	}
	if captured.SubscriptionData != nil {
		t.Fatalf("MSP API checkout should not set trial subscription data, got %+v", captured.SubscriptionData)
	}
	if got := captured.Metadata[checkoutBillingModeMetadataKey]; got != checkoutBillingModeImmediate {
		t.Fatalf("checkout_billing_mode=%q, want %q", got, checkoutBillingModeImmediate)
	}
	if priceID != testMSPStarterPriceID {
		t.Fatalf("price_id=%q, want %q", priceID, testMSPStarterPriceID)
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := strings.TrimSpace(asString(payload["checkout_url"])); got != "https://checkout.stripe.com/c/pay/cs_api" {
		t.Fatalf("checkout_url=%q, want stripe URL", got)
	}
}

func TestMSPSignupAPIAssistedTierReturns400(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPScalePriceID:   testMSPScalePriceID,
	}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/public/msp/signup", strings.NewReader(`{"email":"o@example.com","org_name":"Quesys MSP","tier":"scale"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandleMSPPublicSignup(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := asString(payload["code"]); got != "tier_unavailable" {
		t.Fatalf("code=%q, want %q", got, "tier_unavailable")
	}
}

func TestMSPSignupCompleteRendersHandoff(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/cloud/msp/signup/complete", nil)
	rec := httptest.NewRecorder()

	h.HandleMSPSignupComplete(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Request received") {
		t.Fatal("expected completion heading")
	}
	if strings.Contains(strings.ToLower(body), "trial") {
		t.Fatal("MSP access completion should not advertise a trial")
	}
	if !strings.Contains(body, "signed provider license") || !strings.Contains(body, "client workspace cap") {
		t.Fatal("expected assisted access setup copy")
	}
}

func TestMSPSignupCancelURLPreservesTier(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:                "https://cloud.example.com",
		StripeAPIKey:           "sk_test_123",
		CloudMSPStarterPriceID: testMSPStarterPriceID,
		CloudMSPGrowthPriceID:  testMSPGrowthPriceID,
	}, nil, nil, nil)

	var cancelURL string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		if params.CancelURL != nil {
			cancelURL = *params.CancelURL
		}
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
	}

	form := url.Values{"email": {"o@example.com"}, "org_name": {"Quesys MSP"}, "tier": {"starter"}}
	req := httptest.NewRequest(http.MethodPost, "/cloud/msp/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleMSPSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusSeeOther)
	}
	parsed, err := url.Parse(cancelURL)
	if err != nil {
		t.Fatalf("parse cancel URL: %v", err)
	}
	if got := parsed.Query().Get("tier"); got != "starter" {
		t.Fatalf("cancel URL tier=%q, want %q (URL: %s)", got, "starter", cancelURL)
	}
	if !strings.HasPrefix(parsed.Path, "/cloud/msp/signup") {
		t.Fatalf("cancel URL path=%q, want MSP signup path", parsed.Path)
	}
}
