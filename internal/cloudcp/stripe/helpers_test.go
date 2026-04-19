package stripe

import (
	"context"
	"net/url"
	"strings"
	"testing"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	cpemail "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

type captureProvisionerEmailSender struct {
	calls int
	msg   cpemail.Message
}

func (c *captureProvisionerEmailSender) Send(_ context.Context, msg cpemail.Message) error {
	c.calls++
	c.msg = msg
	return nil
}

func extractMagicLinkToken(t *testing.T, body string) string {
	t.Helper()

	start := strings.Index(body, "http")
	if start < 0 {
		t.Fatalf("expected magic link URL in body %q", body)
	}

	rawURL := body[start:]
	if end := strings.IndexAny(rawURL, " \r\n\t<>\""); end >= 0 {
		rawURL = rawURL[:end]
	}

	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		t.Fatalf("parse magic link URL: %v", err)
	}

	token := strings.TrimSpace(parsed.Query().Get("token"))
	if token == "" {
		t.Fatalf("expected token in magic link URL %q", rawURL)
	}
	return token
}

func TestMapSubscriptionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  entitlements.SubscriptionState
	}{
		{"active", entitlements.SubStateActive},
		{"Active", entitlements.SubStateActive},
		{"trialing", entitlements.SubStateTrial},
		{"past_due", entitlements.SubStateGrace},
		{"unpaid", entitlements.SubStateGrace},
		{"canceled", entitlements.SubStateCanceled},
		{"paused", entitlements.SubStateSuspended},
		{"incomplete", entitlements.SubStateExpired},
		{"incomplete_expired", entitlements.SubStateExpired},
		{"unknown_status", entitlements.SubStateExpired},
		{"", entitlements.SubStateExpired},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := MapSubscriptionStatus(tt.input)
			if got != tt.want {
				t.Errorf("MapSubscriptionStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestShouldGrantCapabilities(t *testing.T) {
	tests := []struct {
		state entitlements.SubscriptionState
		want  bool
	}{
		{entitlements.SubStateActive, true},
		{entitlements.SubStateTrial, true},
		{entitlements.SubStateGrace, true},
		{entitlements.SubStateCanceled, false},
		{entitlements.SubStateSuspended, false},
		{entitlements.SubStateExpired, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := ShouldGrantCapabilities(tt.state)
			if got != tt.want {
				t.Errorf("ShouldGrantCapabilities(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestDerivePlanVersion(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		priceID  string
		want     string
	}{
		{"legacy cloud alias canonicalizes", map[string]string{"plan_version": "cloud-v1"}, "", "cloud_starter"},
		{"cloud shorthand canonicalizes", map[string]string{"plan": "max"}, "", "cloud_max"},
		{"plan_version in metadata", map[string]string{"plan_version": "v2"}, "", "v2"},
		{"plan in metadata", map[string]string{"plan": "pro"}, "", "pro"},
		{"plan_version takes priority", map[string]string{"plan_version": "v3", "plan": "pro"}, "", "v3"},
		{"price ID fallback", nil, "price_123", "stripe_price:price_123"},
		{"generic fallback", nil, "", "stripe"},
		{"nil metadata with price", nil, "price_abc", "stripe_price:price_abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DerivePlanVersion(tt.metadata, tt.priceID)
			if got != tt.want {
				t.Errorf("DerivePlanVersion = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPlanVersionFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		fallback string
		want     string
	}{
		{"canonicalizes legacy cloud alias", map[string]string{"plan_version": "cloud-v1"}, "msp_starter", "cloud_starter"},
		{"uses canonicalized shorthand", map[string]string{"plan": "max"}, "msp_starter", "cloud_max"},
		{"falls back when metadata missing", nil, "msp_growth", "msp_growth"},
		{"canonicalizes legacy fallback alias", nil, "cloud_v1", "cloud_starter"},
		{"canonicalizes legacy msp fallback alias", nil, "msp_hosted_v1", "msp_starter"},
		{"falls back when metadata resolves generic stripe", nil, "msp_starter", "msp_starter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := planVersionFromMetadata(tt.metadata, tt.fallback)
			if got != tt.want {
				t.Errorf("planVersionFromMetadata = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCanonicalizeProvisionedPlanVersion(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{" cloud_v1 ", "cloud_starter"},
		{"starter", "cloud_starter"},
		{" msp_hosted_v1 ", "msp_starter"},
		{"msp_growth", "msp_growth"},
		{"stripe_price:price_123", "stripe_price:price_123"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := canonicalizeProvisionedPlanVersion(tt.input)
			if got != tt.want {
				t.Fatalf("canonicalizeProvisionedPlanVersion(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsSafeStripeID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"cus_test123", true},
		{"sub_abc-def", true},
		{"evt_12345678901234567890", true},
		{"", false},
		{"ab", false},
		{"cus_../etc/passwd", false},
		{"cus test", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			got := IsSafeStripeID(tt.id)
			if got != tt.want {
				t.Errorf("IsSafeStripeID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}

func TestRedactMagicLinkURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "removes query token",
			in:   "https://tenant.cloud.example.com/auth/magic-link/verify?token=abc123&foo=bar",
			want: "https://tenant.cloud.example.com/auth/magic-link/verify",
		},
		{
			name: "invalid URL returns empty",
			in:   "not a url",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactMagicLinkURL(tt.in)
			if got != tt.want {
				t.Fatalf("redactMagicLinkURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGenerateAndLogPortalMagicLinkIssuesPortalTargetedToken(t *testing.T) {
	svc, err := cpauth.NewService(t.TempDir())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)

	emailSender := &captureProvisionerEmailSender{}
	provisioner := &Provisioner{
		magicLinks:  svc,
		baseURL:     "https://cloud.example.com",
		emailSender: emailSender,
		emailFrom:   "noreply@pulserelay.pro",
	}

	provisioner.generateAndLogPortalMagicLink("Owner@Example.com", "t_hosted_1")

	if emailSender.calls != 1 {
		t.Fatalf("email sender calls=%d, want 1", emailSender.calls)
	}

	token := extractMagicLinkToken(t, emailSender.msg.Text)
	validated, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if validated.Target != cpauth.MagicLinkTargetPortal {
		t.Fatalf("token target=%q, want %q", validated.Target, cpauth.MagicLinkTargetPortal)
	}
	if validated.TenantID != "t_hosted_1" {
		t.Fatalf("token tenantID=%q, want %q", validated.TenantID, "t_hosted_1")
	}
	if validated.Email != "owner@example.com" {
		t.Fatalf("token email=%q, want %q", validated.Email, "owner@example.com")
	}
}
