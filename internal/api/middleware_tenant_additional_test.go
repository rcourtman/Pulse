package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestNewTenantMiddlewareWithConfig(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	checker := stubAuthorizationChecker{}

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		Persistence: persistence,
		AuthChecker: checker,
	})

	if mw.persistence != persistence {
		t.Fatalf("expected persistence to be set")
	}
	if mw.authChecker == nil {
		t.Fatalf("expected auth checker to be set")
	}
}

type stubAuthorizationChecker struct{}

func (stubAuthorizationChecker) TokenCanAccessOrg(*config.APITokenRecord, string) bool {
	return true
}

func (stubAuthorizationChecker) UserCanAccessOrg(string, string) bool {
	return true
}

func (stubAuthorizationChecker) CheckAccess(*config.APITokenRecord, string, string) AuthorizationResult {
	return AuthorizationResult{Allowed: true}
}

func TestWriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusBadRequest, "bad", "message")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "bad" || payload["message"] != "message" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestTenantMiddleware_OrgExtraction(t *testing.T) {
	mw := NewTenantMiddleware(nil)

	handler := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := GetOrgID(r.Context())
		org := GetOrganization(r.Context())
		if orgID == "" || org == nil || org.ID != orgID {
			t.Fatalf("unexpected org context: %q %+v", orgID, org)
		}
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "header-org")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "pulse_org_id", Value: "cookie-org"})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestTenantMiddleware_InvalidOrg(t *testing.T) {
	persistence := config.NewMultiTenantPersistence(t.TempDir())
	mw := NewTenantMiddleware(persistence)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "../bad")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestTenantMiddleware_MultiTenantDisabled(t *testing.T) {
	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(false)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })

	mw := NewTenantMiddleware(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-1")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestTenantMiddleware_MultiTenantLicenseRequired(t *testing.T) {
	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })

	mw := NewTenantMiddleware(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "tenant-2")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
}

// hostedTestSource implements licensing.EntitlementSource for testing hosted subscription gating.
type hostedTestSource struct {
	capabilities      []string
	subscriptionState pkglicensing.SubscriptionState
	trialStartedAt    *int64
	trialEndsAt       *int64
}

func (s hostedTestSource) Capabilities() []string   { return s.capabilities }
func (s hostedTestSource) Limits() map[string]int64 { return nil }
func (s hostedTestSource) MetersEnabled() []string  { return nil }
func (s hostedTestSource) PlanVersion() string      { return "cloud_trial" }
func (s hostedTestSource) SubscriptionState() pkglicensing.SubscriptionState {
	return s.subscriptionState
}
func (s hostedTestSource) TrialStartedAt() *int64    { return s.trialStartedAt }
func (s hostedTestSource) TrialEndsAt() *int64       { return s.trialEndsAt }
func (s hostedTestSource) OverflowGrantedAt() *int64 { return nil }

// setupHostedLicenseProvider configures a LicenseServiceProvider backed by an evaluator
// with the given subscription state and trial timestamps (no JWT license).
func setupHostedLicenseProvider(t *testing.T, subState pkglicensing.SubscriptionState, trialEndsAt *int64) {
	t.Helper()
	now := time.Now().Unix()
	source := hostedTestSource{
		capabilities:      pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil),
		subscriptionState: subState,
		trialStartedAt:    &now,
		trialEndsAt:       trialEndsAt,
	}
	eval := pkglicensing.NewEvaluator(source)
	svc := pkglicensing.NewService()
	svc.SetEvaluator(eval)
	SetLicenseServiceProvider(&staticLicenseProvider{service: svc})
	t.Cleanup(func() { SetLicenseServiceProvider(nil) })
}

func TestTenantMiddleware_HostedMode_ActiveSubscription_Allowed(t *testing.T) {
	setupHostedLicenseProvider(t, pkglicensing.SubStateActive, nil)

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		HostedMode: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "cloud-tenant-1")
	rec := httptest.NewRecorder()

	called := false
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("handler should have been called for active hosted subscription")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTenantMiddleware_HostedMode_BoundedTrial_Allowed(t *testing.T) {
	trialEnd := time.Now().Add(14 * 24 * time.Hour).Unix()
	setupHostedLicenseProvider(t, pkglicensing.SubStateTrial, &trialEnd)

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		HostedMode: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "cloud-trial-tenant")
	rec := httptest.NewRecorder()

	called := false
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("handler should have been called for bounded trial")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTenantMiddleware_HostedMode_UnboundedTrial_Blocked(t *testing.T) {
	// Trial with no TrialEndsAt should be blocked to prevent infinite free Cloud.
	setupHostedLicenseProvider(t, pkglicensing.SubStateTrial, nil)

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		HostedMode: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "cloud-unbounded-trial")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should NOT be called for unbounded trial")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload["error"] != "subscription_required" {
		t.Fatalf("expected subscription_required error code, got %q", payload["error"])
	}
}

func TestTenantMiddleware_HostedMode_ExpiredSubscription_Blocked(t *testing.T) {
	setupHostedLicenseProvider(t, pkglicensing.SubStateExpired, nil)

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		HostedMode: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "cloud-expired-tenant")
	rec := httptest.NewRecorder()

	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("handler should NOT be called for expired subscription")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", rec.Code)
	}
}

func TestTenantMiddleware_HostedMode_BypassesMultiTenantFlag(t *testing.T) {
	// Hosted mode should NOT require PULSE_MULTI_TENANT_ENABLED=true.
	// Even with the flag disabled, hosted tenants with active subscriptions should work.
	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(false)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })

	setupHostedLicenseProvider(t, pkglicensing.SubStateActive, nil)

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		HostedMode: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Pulse-Org-ID", "cloud-no-flag-tenant")
	rec := httptest.NewRecorder()

	called := false
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("handler should have been called — hosted mode bypasses multi-tenant flag")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestTenantMiddleware_HostedMode_DefaultOrg_AlwaysAllowed(t *testing.T) {
	// Default org should still be allowed in hosted mode even without any subscription.
	SetLicenseServiceProvider(nil)
	t.Cleanup(func() { SetLicenseServiceProvider(nil) })

	mw := NewTenantMiddlewareWithConfig(TenantMiddlewareConfig{
		HostedMode: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No X-Pulse-Org-ID header → defaults to "default"
	rec := httptest.NewRecorder()

	called := false
	mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if got := GetOrgID(r.Context()); got != "default" {
			t.Fatalf("expected default org, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("handler should have been called for default org")
	}
}

func TestWebSocketChecker_HostedMode_ActiveSubscription_Allowed(t *testing.T) {
	setupHostedLicenseProvider(t, pkglicensing.SubStateActive, nil)

	checker := NewMultiTenantChecker(true) // hosted mode
	result := checker.CheckMultiTenant(context.Background(), "cloud-ws-tenant")

	if !result.Allowed {
		t.Fatalf("expected allowed for active hosted subscription, got reason: %s", result.Reason)
	}
	if !result.FeatureEnabled {
		t.Fatalf("expected feature_enabled=true in hosted mode")
	}
}

func TestWebSocketChecker_HostedMode_ExpiredSubscription_Blocked(t *testing.T) {
	setupHostedLicenseProvider(t, pkglicensing.SubStateExpired, nil)

	checker := NewMultiTenantChecker(true) // hosted mode
	result := checker.CheckMultiTenant(context.Background(), "cloud-ws-expired")

	if result.Allowed {
		t.Fatalf("expected blocked for expired hosted subscription")
	}
	if result.Reason == "" {
		t.Fatalf("expected non-empty reason for blocked subscription")
	}
}

func TestWebSocketChecker_SelfHosted_StillRequiresEnterprise(t *testing.T) {
	// Self-hosted mode should NOT be affected by hosted mode changes.
	orig := IsMultiTenantEnabled()
	SetMultiTenantEnabled(true)
	t.Cleanup(func() { SetMultiTenantEnabled(orig) })

	// No enterprise license → should be blocked
	SetLicenseServiceProvider(nil)
	t.Cleanup(func() { SetLicenseServiceProvider(nil) })

	checker := NewMultiTenantChecker(false) // self-hosted mode
	result := checker.CheckMultiTenant(context.Background(), "self-hosted-tenant")

	if result.Allowed {
		t.Fatalf("expected blocked for self-hosted tenant without Enterprise license")
	}
	if !result.FeatureEnabled {
		t.Fatalf("expected feature_enabled=true (flag is on)")
	}
	if result.Licensed {
		t.Fatalf("expected licensed=false without Enterprise")
	}
}
