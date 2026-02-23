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

func asString(v any) string {
	s, _ := v.(string)
	return s
}
