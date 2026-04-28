package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func issueTrialEntitlementLease(t *testing.T, priv ed25519.PrivateKey, orgID, instanceHost, email string, now time.Time) string {
	t.Helper()
	trialState := pkglicensing.BuildTrialBillingState(now.UTC(), license.TierFeatures[license.TierPro])
	token, err := pkglicensing.SignEntitlementLeaseToken(priv, pkglicensing.EntitlementLeaseClaims{
		OrgID:             orgID,
		Email:             email,
		InstanceHost:      instanceHost,
		PlanVersion:       trialState.PlanVersion,
		SubscriptionState: trialState.SubscriptionState,
		Capabilities:      append([]string(nil), trialState.Capabilities...),
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		TrialStartedAt:    trialState.TrialStartedAt,
		TrialEndsAt:       trialState.TrialEndsAt,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Unix(*trialState.TrialEndsAt, 0).UTC()),
			Subject:   "test_trial_entitlement",
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}
	return token
}

func TestTrialStartRoute_RetiredFromSelfHostedRouter(t *testing.T) {
	rawToken := "trial-start-retired-token-123.12345678"
	record := newTokenRecord(t, rawToken, []string{config.ScopeSettingsWrite}, nil)
	router := NewRouter(newTestConfigWithTokens(t, record), nil, nil, nil, nil, "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil)
	req.Header.Set("X-API-Token", rawToken)
	rec := httptest.NewRecorder()
	router.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestTrialEntitlements_TrialDaysRemainingFromBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	orgID := "default"
	store := config.NewFileBillingStore(baseDir)
	now := time.Now()
	startedAt := now.Add(-1 * time.Hour).Unix()
	endsAt := now.Add(36 * time.Hour).Unix()
	if err := store.SaveBillingState(orgID, &entitlements.BillingState{
		Capabilities:      append([]string(nil), license.TierFeatures[license.TierPro]...),
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       "trial",
		SubscriptionState: entitlements.SubStateTrial,
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	ctx := context.WithValue(context.Background(), OrgIDContextKey, orgID)
	req := httptest.NewRequest(http.MethodGet, "/api/license/entitlements", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleEntitlements(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload EntitlementPayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.SubscriptionState != string(license.SubStateTrial) {
		t.Fatalf("subscription_state=%q, want %q", payload.SubscriptionState, license.SubStateTrial)
	}
	if payload.TrialExpiresAt == nil || payload.TrialDaysRemaining == nil {
		t.Fatalf("expected trial fields, got expires_at=%v days=%v", payload.TrialExpiresAt, payload.TrialDaysRemaining)
	}
	if *payload.TrialExpiresAt != endsAt {
		t.Fatalf("trial_expires_at=%d, want %d", *payload.TrialExpiresAt, endsAt)
	}
	// 36 hours => 2 days (ceil).
	if *payload.TrialDaysRemaining != 2 {
		t.Fatalf("trial_days_remaining=%d, want %d", *payload.TrialDaysRemaining, 2)
	}
}

func TestRefreshHostedEntitlementLeaseOnce_RenewsLeaseAndKeepsLeaseOnlyState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entitlements/refresh" {
			http.NotFound(w, r)
			return
		}
		var req hostedTrialLeaseRefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		if req.OrgID != "default" {
			t.Fatalf("req.OrgID=%q, want %q", req.OrgID, "default")
		}
		if req.InstanceHost != "pulse.example.com" {
			t.Fatalf("req.InstanceHost=%q, want %q", req.InstanceHost, "pulse.example.com")
		}
		if req.EntitlementRefreshToken != "etr_test_default" {
			t.Fatalf("req.EntitlementRefreshToken=%q, want %q", req.EntitlementRefreshToken, "etr_test_default")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hostedTrialLeaseRefreshResponse{
			EntitlementJWT: issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()),
		})
	}))
	defer refreshServer.Close()

	h := NewLicenseHandlers(mtp, false, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	store := config.NewFileBillingStore(baseDir)
	startedAt := time.Now().Add(-13 * 24 * time.Hour).Unix()
	expiredLease := issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now().Add(-15*24*time.Hour))
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		EntitlementJWT:          expiredLease,
		EntitlementRefreshToken: "etr_test_default",
		TrialStartedAt:          &startedAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	refreshed, permanent, err := h.refreshHostedEntitlementLeaseOnce("default", nil)
	if err != nil {
		t.Fatalf("refreshHostedEntitlementLeaseOnce: %v", err)
	}
	if !refreshed || permanent {
		t.Fatalf("refreshed=%v permanent=%v, want refreshed=true permanent=false", refreshed, permanent)
	}

	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.SubscriptionState != entitlements.SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateTrial)
	}

	rawData, err := os.ReadFile(filepath.Join(baseDir, "billing.json"))
	if err != nil {
		t.Fatalf("ReadFile(billing.json): %v", err)
	}
	var rawState entitlements.BillingState
	if err := json.Unmarshal(rawData, &rawState); err != nil {
		t.Fatalf("Unmarshal(raw billing.json): %v", err)
	}
	if strings.TrimSpace(rawState.EntitlementJWT) == "" {
		t.Fatal("expected entitlement_jwt ciphertext to be updated")
	}
	if rawState.EntitlementJWT == expiredLease {
		t.Fatal("expected entitlement_jwt to be encrypted at rest")
	}
	if rawState.EntitlementRefreshToken == "" {
		t.Fatal("expected persisted entitlement_refresh_token ciphertext")
	}
	if rawState.EntitlementRefreshToken == "etr_test_default" {
		t.Fatal("expected entitlement_refresh_token to be encrypted at rest")
	}
	if rawState.SubscriptionState != "" {
		t.Fatalf("raw subscription_state=%q, want empty", rawState.SubscriptionState)
	}
	if len(rawState.Capabilities) != 0 {
		t.Fatalf("raw capabilities=%v, want empty", rawState.Capabilities)
	}
}

func TestRefreshHostedEntitlementLeaseOnce_PermanentFailureClearsLocalEntitlement(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entitlements/refresh" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "invalid entitlement refresh token", http.StatusUnauthorized)
	}))
	defer refreshServer.Close()

	h := NewLicenseHandlers(mtp, false, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	store := config.NewFileBillingStore(baseDir)
	startedAt := time.Now().Add(-2 * time.Hour).Unix()
	activeLease := issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now())
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		EntitlementJWT:          activeLease,
		EntitlementRefreshToken: "etr_test_default",
		TrialStartedAt:          &startedAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	refreshed, permanent, err := h.refreshHostedEntitlementLeaseOnce("default", nil)
	if err == nil {
		t.Fatal("expected refreshHostedEntitlementLeaseOnce to return an error")
	}
	if refreshed || !permanent {
		t.Fatalf("refreshed=%v permanent=%v, want refreshed=false permanent=true", refreshed, permanent)
	}

	loaded, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected billing state to remain after permanent refresh failure")
	}
	if loaded.SubscriptionState != entitlements.SubStateExpired {
		t.Fatalf("subscription_state=%q, want %q", loaded.SubscriptionState, entitlements.SubStateExpired)
	}

	rawData, err := os.ReadFile(filepath.Join(baseDir, "billing.json"))
	if err != nil {
		t.Fatalf("ReadFile(billing.json): %v", err)
	}
	var rawState entitlements.BillingState
	if err := json.Unmarshal(rawData, &rawState); err != nil {
		t.Fatalf("Unmarshal(raw billing.json): %v", err)
	}
	if rawState.EntitlementJWT != "" {
		t.Fatalf("raw entitlement_jwt=%q, want empty", rawState.EntitlementJWT)
	}
	if rawState.EntitlementRefreshToken != "" {
		t.Fatalf("raw entitlement_refresh_token=%q, want empty", rawState.EntitlementRefreshToken)
	}
	if rawState.TrialStartedAt == nil || *rawState.TrialStartedAt <= 0 {
		t.Fatalf("raw trial_started_at=%v, want non-nil positive timestamp", rawState.TrialStartedAt)
	}
}

func TestRefreshHostedEntitlementLeaseOnce_HostMismatchLeavesStateUnchanged(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entitlements/refresh" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hostedTrialLeaseRefreshResponse{
			EntitlementJWT: issueTrialEntitlementLease(t, priv, "default", "pulse-b.example.com", "owner@example.com", time.Now()),
		})
	}))
	defer refreshServer.Close()

	h := NewLicenseHandlers(mtp, false, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	store := config.NewFileBillingStore(baseDir)
	startedAt := time.Now().Add(-13 * 24 * time.Hour).Unix()
	originalLease := issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now().Add(-15*24*time.Hour))
	if err := store.SaveBillingState("default", &entitlements.BillingState{
		EntitlementJWT:          originalLease,
		EntitlementRefreshToken: "etr_test_default",
		TrialStartedAt:          &startedAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	refreshed, permanent, err := h.refreshHostedEntitlementLeaseOnce("default", nil)
	if err == nil {
		t.Fatal("expected refreshHostedEntitlementLeaseOnce to fail on host mismatch")
	}
	if refreshed || permanent {
		t.Fatalf("refreshed=%v permanent=%v, want refreshed=false permanent=false", refreshed, permanent)
	}

	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil {
		t.Fatal("expected billing state to remain present")
	}
	if state.EntitlementJWT != originalLease {
		t.Fatal("expected original entitlement_jwt to remain unchanged after host mismatch")
	}
}
