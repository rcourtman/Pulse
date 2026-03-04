package cloudcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	cpemail "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	stripe "github.com/stripe/stripe-go/v82"
)

type captureMagicLinkGenerator struct {
	calls    int
	email    string
	tenantID string
	token    string
	err      error
}

func (c *captureMagicLinkGenerator) GenerateToken(email, tenantID string) (string, error) {
	c.calls++
	c.email = email
	c.tenantID = tenantID
	if c.err != nil {
		return "", c.err
	}
	return c.token, nil
}

type captureEmailSender struct {
	calls int
	msg   cpemail.Message
}

func (c *captureEmailSender) Send(_ context.Context, msg cpemail.Message) error {
	c.calls++
	c.msg = msg
	return nil
}

func TestPublicCloudSignupHandleSignupPageRendersForm(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rec := httptest.NewRecorder()

	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<form") || !strings.Contains(body, "Start Pulse Cloud") {
		t.Fatalf("expected signup form markup in response body")
	}
}

func TestPublicCloudSignupHandleSignupPagePostValidRedirectsToStripe(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_test_123",
	}, nil, nil, nil)
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		if params == nil {
			t.Fatal("expected checkout params")
		}
		if got := strings.TrimSpace(params.Metadata["account_display_name"]); got != "Pulse Labs" {
			t.Fatalf("metadata account_display_name=%q, want %q", got, "Pulse Labs")
		}
		if got := strings.TrimSpace(params.Metadata["signup_source"]); got != "public_cloud_signup" {
			t.Fatalf("metadata signup_source=%q, want %q", got, "public_cloud_signup")
		}
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://checkout.stripe.com/c/pay/cs_test" {
		t.Fatalf("location=%q, want stripe checkout URL", loc)
	}
}

func TestPublicCloudSignupHandleSignupPagePostInvalidEmail(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{}, nil, nil, nil)
	form := url.Values{
		"email":    {"invalid"},
		"org_name": {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "A valid email address is required.") {
		t.Fatalf("expected validation message, got %q", rec.Body.String())
	}
}

func TestPublicCloudSignupHandlePublicSignupMethodAndValidation(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{}, nil, nil, nil)

	methodReq := httptest.NewRequest(http.MethodGet, "/api/public/signup", nil)
	methodRec := httptest.NewRecorder()
	h.HandlePublicSignup(methodRec, methodReq)
	if methodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status=%d, want %d", methodRec.Code, http.StatusMethodNotAllowed)
	}

	invalidReq := httptest.NewRequest(http.MethodPost, "/api/public/signup", strings.NewReader(`{"email":"owner@example.com","org_name":"ab"}`))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidRec := httptest.NewRecorder()
	h.HandlePublicSignup(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid payload status=%d, want %d", invalidRec.Code, http.StatusBadRequest)
	}
}

func TestPublicCloudSignupHandlePublicSignupCreatesCheckout(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_test_123",
	}, nil, nil, nil)
	h.createCheckoutSession = func(_ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_live"}, nil
	}

	req := httptest.NewRequest(http.MethodPost, "/api/public/signup", strings.NewReader(`{"email":"owner@example.com","org_name":"Pulse Labs"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandlePublicSignup(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := strings.TrimSpace(asString(payload["checkout_url"])); got != "https://checkout.stripe.com/c/pay/cs_live" {
		t.Fatalf("checkout_url=%q, want stripe URL", got)
	}
}

func TestPublicCloudSignupHandlePublicMagicLinkRequestAlwaysOpaqueWhenUnknown(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{}, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/public/magic-link/request", strings.NewReader(`{"email":"owner@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandlePublicMagicLinkRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := strings.TrimSpace(asString(payload["message"])); got == "" {
		t.Fatalf("expected opaque success message")
	}
}

func TestPublicCloudSignupHandlePublicMagicLinkRequestKnownTenantSendsEmail(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewTenantRegistry(dir)
	if err != nil {
		t.Fatalf("NewTenantRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })

	if err := reg.Create(&registry.Tenant{
		ID:          "t-test-1",
		AccountID:   "a-test-1",
		Email:       "owner@example.com",
		DisplayName: "Pulse Labs",
		State:       registry.TenantStateActive,
	}); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	magic := &captureMagicLinkGenerator{token: "ml_test_123"}
	emailSender := &captureEmailSender{}
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:   "https://cloud.example.com",
		EmailFrom: "noreply@pulserelay.pro",
	}, reg, magic, emailSender)

	req := httptest.NewRequest(http.MethodPost, "/api/public/magic-link/request", strings.NewReader(`{"email":"OWNER@EXAMPLE.COM"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.HandlePublicMagicLinkRequest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}
	if magic.calls != 1 {
		t.Fatalf("magic GenerateToken calls=%d, want 1", magic.calls)
	}
	if magic.tenantID != "t-test-1" {
		t.Fatalf("magic tenantID=%q, want %q", magic.tenantID, "t-test-1")
	}
	if emailSender.calls != 1 {
		t.Fatalf("email sender calls=%d, want 1", emailSender.calls)
	}
	if emailSender.msg.To != "OWNER@EXAMPLE.COM" {
		t.Fatalf("email to=%q, want original request email", emailSender.msg.To)
	}
	if !strings.Contains(emailSender.msg.Text, "/auth/magic-link/verify?token=") {
		t.Fatalf("email body missing verify URL")
	}
}

func TestPublicCloudSignupCheckoutMetadataIncludesPlanVersion(t *testing.T) {
	// Use a real Stripe price ID from PriceIDToPlanVersion so plan_version is set.
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_1T5kflBrHBocJIGHUqPv1dzV", // cloud_starter
	}, nil, nil, nil)

	var capturedMeta map[string]string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		capturedMeta = params.Metadata
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if capturedMeta == nil {
		t.Fatal("expected checkout session params with metadata")
	}
	if got := capturedMeta["plan_version"]; got != "cloud_starter" {
		t.Fatalf("metadata plan_version=%q, want %q", got, "cloud_starter")
	}
	if got := capturedMeta["account_kind"]; got != "individual" {
		t.Fatalf("metadata account_kind=%q, want %q", got, "individual")
	}
}

func TestPublicCloudSignupCheckoutMetadataRejectsMSPPlanForPublicSignup(t *testing.T) {
	// MSP price IDs should NOT set plan_version on public individual signup.
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_1T5kgTBrHBocJIGHjOs15LI2", // msp_starter
	}, nil, nil, nil)

	var capturedMeta map[string]string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		capturedMeta = params.Metadata
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	form := url.Values{"email": {"o@example.com"}, "org_name": {"Pulse Labs"}}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusSeeOther)
	}
	if _, exists := capturedMeta["plan_version"]; exists {
		t.Fatalf("MSP price should NOT set plan_version on public signup, got %q", capturedMeta["plan_version"])
	}
}

func TestPublicCloudSignupCheckoutMetadataOmitsPlanVersionForUnknownPrice(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_unknown_test",
	}, nil, nil, nil)

	var capturedMeta map[string]string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		capturedMeta = params.Metadata
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/c/pay/cs_test"}, nil
	}

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusSeeOther)
	}
	if _, exists := capturedMeta["plan_version"]; exists {
		t.Fatalf("metadata should NOT contain plan_version for unknown price, got %q", capturedMeta["plan_version"])
	}
}

func TestParseCloudTier(t *testing.T) {
	tests := []struct {
		input  string
		want   cloudTier
		wantOK bool
	}{
		{"", cloudTierStarter, true},
		{"starter", cloudTierStarter, true},
		{"STARTER", cloudTierStarter, true},
		{" Starter ", cloudTierStarter, true},
		{"power", cloudTierPower, true},
		{"Power", cloudTierPower, true},
		{"max", cloudTierMax, true},
		{"MAX", cloudTierMax, true},
		{"enterprise", "", false},
		{"invalid", "", false},
		{"pro", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseCloudTier(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseCloudTier(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("parseCloudTier(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPriceIDForTier(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		TrialSignupPriceID: "price_starter",
		CloudPowerPriceID:  "price_power",
		CloudMaxPriceID:    "price_max",
	}, nil, nil, nil)

	tests := []struct {
		tier   cloudTier
		want   string
		wantOK bool
	}{
		{cloudTierStarter, "price_starter", true},
		{cloudTierPower, "price_power", true},
		{cloudTierMax, "price_max", true},
	}
	for _, tt := range tests {
		t.Run(string(tt.tier), func(t *testing.T) {
			got, ok := h.priceIDForTier(tt.tier)
			if ok != tt.wantOK {
				t.Fatalf("priceIDForTier(%q) ok=%v, want %v", tt.tier, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Fatalf("priceIDForTier(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestPriceIDForTierUnconfiguredReturnsNotOK(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		TrialSignupPriceID: "price_starter",
		// Power and Max not configured
	}, nil, nil, nil)

	if _, ok := h.priceIDForTier(cloudTierPower); ok {
		t.Fatal("expected not-ok for unconfigured Power tier")
	}
	if _, ok := h.priceIDForTier(cloudTierMax); ok {
		t.Fatal("expected not-ok for unconfigured Max tier")
	}
}

func TestPublicCloudSignupTierSelectionFormPost(t *testing.T) {
	cfg := &CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_starter",
		CloudPowerPriceID:  "price_power",
		CloudMaxPriceID:    "price_max",
	}

	tests := []struct {
		tier        string
		wantPriceID string
	}{
		{"", "price_starter"}, // default
		{"starter", "price_starter"},
		{"power", "price_power"},
		{"max", "price_max"},
	}

	for _, tt := range tests {
		t.Run("tier="+tt.tier, func(t *testing.T) {
			h := NewPublicCloudSignupHandlers(cfg, nil, nil, nil)
			var capturedPriceID string
			h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
				if len(params.LineItems) > 0 && params.LineItems[0].Price != nil {
					capturedPriceID = *params.LineItems[0].Price
				}
				return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
			}

			form := url.Values{
				"email":    {"owner@example.com"},
				"org_name": {"Pulse Labs"},
				"tier":     {tt.tier},
			}
			req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			h.HandleSignupPage(rec, req)

			if rec.Code != http.StatusSeeOther {
				t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
			}
			if capturedPriceID != tt.wantPriceID {
				t.Fatalf("price_id=%q, want %q", capturedPriceID, tt.wantPriceID)
			}
		})
	}
}

func TestPublicCloudSignupInvalidTierFormPost(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		TrialSignupPriceID: "price_starter",
	}, nil, nil, nil)

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
		"tier":     {"enterprise"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Invalid plan tier") {
		t.Fatalf("expected tier validation message, got %q", rec.Body.String())
	}
}

func TestPublicCloudSignupUnconfiguredTierFormPost(t *testing.T) {
	// Power tier price not configured — should fail with 400 (not 502).
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_starter",
		// CloudPowerPriceID intentionally empty
	}, nil, nil, nil)

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
		"tier":     {"power"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "not currently available") {
		t.Fatalf("expected tier unavailable message, got %q", rec.Body.String())
	}
}

func TestPublicCloudSignupAPIUnconfiguredTierReturns400(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_starter",
		// CloudPowerPriceID intentionally empty
	}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/public/signup", strings.NewReader(`{"email":"o@example.com","org_name":"Pulse Labs","tier":"power"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.HandlePublicSignup(rec, req)

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

func TestPublicCloudSignupAPITierSelection(t *testing.T) {
	cfg := &CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_starter",
		CloudPowerPriceID:  "price_power",
		CloudMaxPriceID:    "price_max",
	}

	tests := []struct {
		body        string
		wantStatus  int
		wantPriceID string
	}{
		// Default tier (starter)
		{`{"email":"o@example.com","org_name":"Pulse Labs"}`, http.StatusCreated, "price_starter"},
		// Explicit starter
		{`{"email":"o@example.com","org_name":"Pulse Labs","tier":"starter"}`, http.StatusCreated, "price_starter"},
		// Power tier
		{`{"email":"o@example.com","org_name":"Pulse Labs","tier":"power"}`, http.StatusCreated, "price_power"},
		// Max tier
		{`{"email":"o@example.com","org_name":"Pulse Labs","tier":"max"}`, http.StatusCreated, "price_max"},
		// Invalid tier
		{`{"email":"o@example.com","org_name":"Pulse Labs","tier":"enterprise"}`, http.StatusBadRequest, ""},
	}

	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			h := NewPublicCloudSignupHandlers(cfg, nil, nil, nil)
			var capturedPriceID string
			h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
				if len(params.LineItems) > 0 && params.LineItems[0].Price != nil {
					capturedPriceID = *params.LineItems[0].Price
				}
				return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
			}

			req := httptest.NewRequest(http.MethodPost, "/api/public/signup", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.HandlePublicSignup(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status=%d, want %d body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if tt.wantPriceID != "" && capturedPriceID != tt.wantPriceID {
				t.Fatalf("price_id=%q, want %q", capturedPriceID, tt.wantPriceID)
			}
		})
	}
}

func TestPublicCloudSignupTierPreservedInCancelURL(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_starter",
		CloudPowerPriceID:  "price_power",
	}, nil, nil, nil)

	var capturedCancelURL string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		if params.CancelURL != nil {
			capturedCancelURL = *params.CancelURL
		}
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
	}

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
		"tier":     {"power"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusSeeOther)
	}
	parsed, err := url.Parse(capturedCancelURL)
	if err != nil {
		t.Fatalf("parse cancel URL: %v", err)
	}
	if got := parsed.Query().Get("tier"); got != "power" {
		t.Fatalf("cancel URL tier=%q, want %q (URL: %s)", got, "power", capturedCancelURL)
	}
}

func TestPublicCloudSignupPowerTierMetadataIncludesPlanVersion(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		BaseURL:            "https://cloud.example.com",
		StripeAPIKey:       "sk_test_123",
		TrialSignupPriceID: "price_1T5kflBrHBocJIGHUqPv1dzV", // cloud_starter
		CloudPowerPriceID:  "price_1T5kg2BrHBocJIGHmkoF0zXY", // cloud_power
	}, nil, nil, nil)

	var capturedMeta map[string]string
	h.createCheckoutSession = func(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
		capturedMeta = params.Metadata
		return &stripe.CheckoutSession{URL: "https://checkout.stripe.com/test"}, nil
	}

	form := url.Values{
		"email":    {"owner@example.com"},
		"org_name": {"Pulse Labs"},
		"tier":     {"power"},
	}
	req := httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want %d body=%q", rec.Code, http.StatusSeeOther, rec.Body.String())
	}
	if capturedMeta == nil {
		t.Fatal("expected checkout metadata")
	}
	if got := capturedMeta["plan_version"]; got != "cloud_power" {
		t.Fatalf("plan_version=%q, want %q", got, "cloud_power")
	}
}

func TestPublicCloudSignupGETUnconfiguredTierFallsBackToStarter(t *testing.T) {
	// Power configured, Max not — ?tier=max should fall back to starter.
	h := NewPublicCloudSignupHandlers(&CPConfig{
		TrialSignupPriceID: "price_starter",
		CloudPowerPriceID:  "price_power",
		// CloudMaxPriceID intentionally empty
	}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/signup?tier=max", nil)
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	// Starter radio should be checked (fallback), not max.
	if !strings.Contains(body, `value="starter" checked`) {
		t.Fatal("expected starter radio to be checked as fallback for unconfigured max tier")
	}
}

func TestPublicCloudSignupFormShowsTierRadiosWhenConfigured(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		CloudPowerPriceID: "price_power",
		CloudMaxPriceID:   "price_max",
	}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/signup?tier=power", nil)
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `value="starter"`) {
		t.Fatal("expected starter radio option")
	}
	if !strings.Contains(body, `value="power"`) {
		t.Fatal("expected power radio option")
	}
	if !strings.Contains(body, `value="max"`) {
		t.Fatal("expected max radio option")
	}
}

func TestPublicCloudSignupFormHidesTierRadiosWhenSingleTier(t *testing.T) {
	h := NewPublicCloudSignupHandlers(&CPConfig{
		TrialSignupPriceID: "price_starter",
		// No Power or Max configured
	}, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rec := httptest.NewRecorder()
	h.HandleSignupPage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	// Radio inputs should NOT be present when only starter is configured.
	// Note: "tier-group" appears in the CSS <style> block regardless,
	// so check for actual radio inputs instead.
	if strings.Contains(body, `type="radio"`) {
		t.Fatal("expected no tier radio inputs when only starter is configured")
	}
	if !strings.Contains(body, `type="hidden" name="tier" value="starter"`) {
		t.Fatal("expected hidden tier input when only starter is configured")
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}
