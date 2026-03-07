package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// createTestHandlerWithDir creates a LicenseHandlers and returns the base data
// directory so callers can create billing stores against the same path.
func createTestHandlerWithDir(t *testing.T) (*LicenseHandlers, string) {
	t.Helper()
	tempDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(tempDir)
	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	return NewLicenseHandlers(mtp, false), tempDir
}

// TestExpiredLicenseBlocksProFeature verifies that when a Pro license expires
// (and the grace period has also elapsed), the RequireLicenseFeature middleware
// returns 402 Payment Required for a Pro-only feature. This is the critical
// HTTP-level integration test for acceptance checklist item 6.3:
// "After license expires, Pro features are gated again."
func TestExpiredLicenseBlocksProFeature(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)

	// Activate a Pro license with a 24h expiry.
	licenseKey, err := license.GenerateLicenseForTesting("expiry@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	lic, err := handler.Service(context.Background()).Activate(licenseKey)
	if err != nil {
		t.Fatalf("activate test license: %v", err)
	}

	// Sanity: Pro feature works while license is active.
	handlerCalled := false
	wrapped := RequireLicenseFeature(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with active license, got %d", rec.Code)
	}
	if !handlerCalled {
		t.Fatal("handler must be called with active license")
	}

	// Expire the license by moving ExpiresAt into the past, beyond the
	// 7-day grace period. In dev mode, Activate returns the mutable license
	// pointer (see service.go:186-190).
	lic.Claims.ExpiresAt = time.Now().Add(-8 * 24 * time.Hour).Unix()
	// Clear any cached grace period end so it gets recalculated from the
	// new ExpiresAt value.
	lic.GracePeriodEnd = nil

	// Now the middleware must block with 402.
	handlerCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402 after license expired past grace period, got %d", rec.Code)
	}
	if handlerCalled {
		t.Fatal("handler must NOT be called when license is expired past grace period")
	}
}

// TestGracePeriodAllowsProFeature verifies that during the 7-day grace period
// after license expiry, Pro features remain accessible. This covers acceptance
// checklist item 6.3: "Grace period applies (features work for 7 days after expiry)."
func TestGracePeriodAllowsProFeature(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)

	licenseKey, err := license.GenerateLicenseForTesting("grace@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	lic, err := handler.Service(context.Background()).Activate(licenseKey)
	if err != nil {
		t.Fatalf("activate test license: %v", err)
	}

	// Expire the license, but still within the 7-day grace window.
	// License expired 3 days ago → grace period end is 4 days in the future.
	lic.Claims.ExpiresAt = time.Now().Add(-3 * 24 * time.Hour).Unix()
	lic.GracePeriodEnd = nil // Let ensureGracePeriodEnd recalculate

	handlerCalled := false
	wrapped := RequireLicenseFeature(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 during grace period, got %d", rec.Code)
	}
	if !handlerCalled {
		t.Fatal("handler must be called during grace period")
	}
}

// TestExpiredLicenseFeaturesEndpoint verifies that the /api/license/features
// endpoint correctly reports only free-tier features after a Pro license expires
// past its grace period. This ensures the frontend receives accurate feature
// flags to render upgrade prompts.
func TestExpiredLicenseFeaturesEndpoint(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)

	licenseKey, err := license.GenerateLicenseForTesting("features@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	lic, err := handler.Service(context.Background()).Activate(licenseKey)
	if err != nil {
		t.Fatalf("activate test license: %v", err)
	}

	// Sanity: features endpoint shows Pro features while active.
	req := httptest.NewRequest(http.MethodGet, "/api/license/features", nil)
	rec := httptest.NewRecorder()
	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp licenseFeaturesResponseDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Features[license.FeatureAIAutoFix] {
		t.Fatal("ai_autofix must be true with active Pro license")
	}
	if resp.LicenseStatus != string(license.LicenseStateActive) {
		t.Fatalf("expected license_status %q, got %q", license.LicenseStateActive, resp.LicenseStatus)
	}

	// Expire past grace period.
	lic.Claims.ExpiresAt = time.Now().Add(-8 * 24 * time.Hour).Unix()
	lic.GracePeriodEnd = nil

	req = httptest.NewRequest(http.MethodGet, "/api/license/features", nil)
	rec = httptest.NewRecorder()
	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var expiredResp licenseFeaturesResponseDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &expiredResp); err != nil {
		t.Fatalf("unmarshal expired response: %v", err)
	}

	// License status must reflect expiration.
	if expiredResp.LicenseStatus != string(license.LicenseStateExpired) {
		t.Fatalf("expected license_status %q after expiry, got %q", license.LicenseStateExpired, expiredResp.LicenseStatus)
	}

	// Pro-only features must now be false.
	proOnlyFeatures := []string{
		license.FeatureAIAutoFix,
		license.FeatureAIAlerts,
		license.FeatureKubernetesAI,
	}
	for _, feat := range proOnlyFeatures {
		if expiredResp.Features[feat] {
			t.Errorf("feature %q must be false after license expires past grace period", feat)
		}
	}

	// Free-tier features must still be true.
	if !expiredResp.Features[license.FeatureAIPatrol] {
		t.Error("free-tier feature ai_patrol must remain true after license expires")
	}
}

// TestTrialExpiryReverts verifies that after a trial expires, the license
// features endpoint reverts to free-tier capabilities. This covers acceptance
// checklist item 6.4: "Trial expiry reverts to Community."
func TestTrialExpiryReverts(t *testing.T) {
	handler, dataDir := createTestHandlerWithDir(t)

	billingStore := config.NewFileBillingStore(dataDir)

	// Create an expired trial: started 15 days ago, ended 1 day ago.
	trialStarted := time.Now().Add(-15 * 24 * time.Hour).Unix()
	trialEnded := time.Now().Add(-1 * 24 * time.Hour).Unix()
	state := &pkglicensing.BillingState{
		Capabilities:      append([]string(nil), pkglicensing.TierFeatures[pkglicensing.TierPro]...),
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       "trial",
		SubscriptionState: pkglicensing.SubStateTrial,
		TrialStartedAt:    &trialStarted,
		TrialEndsAt:       &trialEnded,
	}
	if err := billingStore.SaveBillingState("default", state); err != nil {
		t.Fatalf("save billing state: %v", err)
	}

	// Wire up the evaluator so the service reads billing state.
	// The DatabaseSource's normalizeTrialExpiry will detect that the trial
	// has expired and set SubscriptionState to "expired" with nil capabilities.
	svc := handler.Service(context.Background())
	eval := newLicenseEvaluatorForBillingStoreFromLicensing(billingStore, "default", 0, "")
	svc.SetEvaluator(eval)

	// Features endpoint must show free-tier only.
	req := httptest.NewRequest(http.MethodGet, "/api/license/features", nil)
	rec := httptest.NewRecorder()
	handler.HandleLicenseFeatures(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp licenseFeaturesResponseDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	// License status must not be "active" or "trial" after trial expires.
	if resp.LicenseStatus == string(license.LicenseStateActive) {
		t.Error("license_status must not be 'active' after trial expires")
	}

	proOnlyFeatures := []string{
		license.FeatureAIAutoFix,
		license.FeatureAIAlerts,
		license.FeatureKubernetesAI,
	}
	for _, feat := range proOnlyFeatures {
		if resp.Features[feat] {
			t.Errorf("feature %q must be false after trial expires", feat)
		}
	}

	// Free-tier features must remain.
	if !resp.Features[license.FeatureAIPatrol] {
		t.Error("free-tier feature ai_patrol must remain true after trial expires")
	}
}

// TestLicenseGatedEmptyResponseOnExpiry verifies that the LicenseGatedEmptyResponse
// middleware returns an empty array with X-License-Required header when a license
// expires, rather than real data. This ensures frontend Promise.all patterns work.
func TestLicenseGatedEmptyResponseOnExpiry(t *testing.T) {
	t.Setenv("PULSE_LICENSE_DEV_MODE", "true")

	handler := createTestHandler(t)

	licenseKey, err := license.GenerateLicenseForTesting("gated@example.com", license.TierPro, 24*time.Hour)
	if err != nil {
		t.Fatalf("generate test license: %v", err)
	}
	lic, err := handler.Service(context.Background()).Activate(licenseKey)
	if err != nil {
		t.Fatalf("activate test license: %v", err)
	}

	// Sanity: gated endpoint returns real data with active license.
	innerCalled := false
	wrapped := LicenseGatedEmptyResponse(handler, license.FeatureAIAutoFix, func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"finding":"real-data"}]`))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with active license, got %d", rec.Code)
	}
	if !innerCalled {
		t.Fatal("inner handler must be called with active license")
	}
	if rec.Header().Get("X-License-Required") != "" {
		t.Fatal("X-License-Required must not be set with active license")
	}

	// Expire past grace period.
	lic.Claims.ExpiresAt = time.Now().Add(-8 * 24 * time.Hour).Unix()
	lic.GracePeriodEnd = nil

	innerCalled = false
	req = httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec = httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (empty array, not 402) from gated middleware, got %d", rec.Code)
	}
	if innerCalled {
		t.Fatal("inner handler must NOT be called after license expires")
	}
	if rec.Header().Get("X-License-Required") != "true" {
		t.Fatal("X-License-Required header must be set when license is expired")
	}
	if rec.Header().Get("X-License-Feature") != license.FeatureAIAutoFix {
		t.Fatalf("expected X-License-Feature=%q, got %q", license.FeatureAIAutoFix, rec.Header().Get("X-License-Feature"))
	}
	if rec.Body.String() != "[]" {
		t.Fatalf("expected empty array response, got %s", rec.Body.String())
	}
}
