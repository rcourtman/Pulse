package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func issueTrialSignupInitiationToken(t *testing.T, h *LicenseHandlers, orgID, returnURL string) string {
	t.Helper()
	if h == nil || h.trialInitiations == nil {
		t.Fatal("trial initiation store is not configured")
	}
	token, err := h.trialInitiations.issue(orgID, returnURL, time.Now().UTC().Add(trialSignupInitiationTTL))
	if err != nil {
		t.Fatalf("issue trial initiation token: %v", err)
	}
	return token
}

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

func hostedTrialRedemptionWithLease(lease string) *hostedTrialRedemptionResponse {
	return &hostedTrialRedemptionResponse{
		EntitlementJWT:          lease,
		EntitlementRefreshToken: "refresh_test_token",
	}
}

func issueTrialRedemptionResponse(t *testing.T, priv ed25519.PrivateKey, orgID, instanceHost, email string, now time.Time) *hostedTrialRedemptionResponse {
	t.Helper()
	return &hostedTrialRedemptionResponse{
		EntitlementJWT:          issueTrialEntitlementLease(t, priv, orgID, instanceHost, email, now),
		EntitlementRefreshToken: "etr_test_" + orgID,
	}
}

func TestTrialStart_DefaultOrgReturnsHostedSignupRedirect(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: "https://billing.example.com/start-pro-trial?source=test",
	})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")

	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleStartTrial(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	var resp APIError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "trial_signup_required" {
		t.Fatalf("code=%q, want %q", resp.Code, "trial_signup_required")
	}
	actionURL := resp.Details["action_url"]
	if actionURL == "" {
		t.Fatal("expected action_url in trial signup response")
	}
	parsed, err := url.Parse(actionURL)
	if err != nil {
		t.Fatalf("parse action_url: %v", err)
	}
	if got := parsed.Scheme + "://" + parsed.Host + parsed.Path; got != "https://billing.example.com/start-pro-trial" {
		t.Fatalf("action_url base=%q, want %q", got, "https://billing.example.com/start-pro-trial")
	}
	if got := parsed.Query().Get("source"); got != "test" {
		t.Fatalf("action_url source=%q, want %q", got, "test")
	}
	if got := parsed.Query().Get("org_id"); got != "default" {
		t.Fatalf("action_url org_id=%q, want %q", got, "default")
	}
	if got := parsed.Query().Get("return_url"); got != "https://pulse.example.com/auth/trial-activate" {
		t.Fatalf("action_url return_url=%q, want %q", got, "https://pulse.example.com/auth/trial-activate")
	}
	if got := strings.TrimSpace(parsed.Query().Get("instance_token")); got == "" {
		t.Fatal("expected instance_token in action_url")
	}

	billingPath := filepath.Join(baseDir, "billing.json")
	if _, err := os.Stat(billingPath); !os.IsNotExist(err) {
		t.Fatalf("expected no billing.json to be written, stat err=%v", err)
	}
}

func TestTrialStart_FailsClosedWithoutCallbackURL(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{
		ProTrialSignupURL: "https://billing.example.com/start-pro-trial",
	})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	req.Host = ""
	rec := httptest.NewRecorder()
	h.HandleStartTrial(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	var resp APIError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "trial_signup_unavailable" {
		t.Fatalf("code=%q, want %q", resp.Code, "trial_signup_unavailable")
	}
}

func TestTrialStart_RejectsAlreadyUsedTrial(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false, &config.Config{PublicURL: "https://pulse.example.com"})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "default")
	now := time.Now()
	startedAt := now.Add(-2 * time.Hour).Unix()
	endsAt := now.Add(12 * time.Hour).Unix()
	store := config.NewFileBillingStore(baseDir)
	if err := store.SaveBillingState("default", &entitlements.BillingState{
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

	req := httptest.NewRequest(http.MethodPost, "/api/license/trial/start", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	h.HandleStartTrial(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	var resp APIError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != "trial_already_used" {
		t.Fatalf("code=%q, want %q", resp.Code, "trial_already_used")
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

func TestTrialActivation_SignedTokenStartsTrial(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))
	h.trialRedeemer = func(string) (*hostedTrialRedemptionResponse, error) {
		return issueTrialRedemptionResponse(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()), nil
	}
	returnURL := "https://pulse.example.com/auth/trial-activate"
	instanceToken := issueTrialSignupInitiationToken(t, h, "default", returnURL)

	token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()
	h.HandleTrialActivation(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d: %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/settings/system-pro?trial=activated" {
		t.Fatalf("redirect=%q, want %q", got, "/settings/system-pro?trial=activated")
	}

	store := config.NewFileBillingStore(baseDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.SubscriptionState != entitlements.SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateTrial)
	}
	if strings.TrimSpace(state.EntitlementJWT) == "" {
		t.Fatal("expected entitlement_jwt to be stored")
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
		t.Fatal("expected raw entitlement_jwt to be persisted")
	}
	if strings.TrimSpace(rawState.EntitlementRefreshToken) == "" {
		t.Fatal("expected raw entitlement_refresh_token to be persisted")
	}
	if rawState.SubscriptionState != "" {
		t.Fatalf("raw subscription_state=%q, want empty", rawState.SubscriptionState)
	}
	if rawState.PlanVersion != "" {
		t.Fatalf("raw plan_version=%q, want empty", rawState.PlanVersion)
	}
	if len(rawState.Capabilities) != 0 {
		t.Fatalf("raw capabilities=%v, want empty", rawState.Capabilities)
	}
	if len(rawState.Limits) != 0 {
		t.Fatalf("raw limits=%v, want empty", rawState.Limits)
	}
	if rawState.TrialStartedAt == nil {
		t.Fatal("expected raw trial_started_at to be preserved")
	}
	if rawState.TrialEndsAt != nil {
		t.Fatalf("raw trial_ends_at=%v, want nil", rawState.TrialEndsAt)
	}
}

func TestTrialActivation_ReplayTokenRejected(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))
	h.trialRedeemer = func(string) (*hostedTrialRedemptionResponse, error) {
		return issueTrialRedemptionResponse(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()), nil
	}
	returnURL := "https://pulse.example.com/auth/trial-activate"
	instanceToken := issueTrialSignupInitiationToken(t, h, "default", returnURL)

	token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil)
	firstReq.Host = "pulse.example.com"
	firstRec := httptest.NewRecorder()
	h.HandleTrialActivation(firstRec, firstReq)
	if firstRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("first status=%d, want %d", firstRec.Code, http.StatusTemporaryRedirect)
	}

	replayReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil)
	replayReq.Host = "pulse.example.com"
	replayRec := httptest.NewRecorder()
	h.HandleTrialActivation(replayRec, replayReq)

	if replayRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("replay status=%d, want %d", replayRec.Code, http.StatusTemporaryRedirect)
	}
	if got := replayRec.Header().Get("Location"); got != "/settings/system-pro?trial=replayed" {
		t.Fatalf("replay redirect=%q, want %q", got, "/settings/system-pro?trial=replayed")
	}
}

func TestTrialActivation_ReissuedTokenForSameSessionRejected(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))
	h.trialRedeemer = func(string) (*hostedTrialRedemptionResponse, error) {
		return issueTrialRedemptionResponse(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()), nil
	}
	returnURL := "https://pulse.example.com/auth/trial-activate"
	instanceToken := issueTrialSignupInitiationToken(t, h, "default", returnURL)

	expiresAt := jwt.NewNumericDate(time.Now().Add(10 * time.Minute))
	firstToken, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_same_session",
			ExpiresAt: expiresAt,
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken(first): %v", err)
	}
	secondToken, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_same_session",
			ExpiresAt: expiresAt,
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken(second): %v", err)
	}
	if firstToken == secondToken {
		t.Fatalf("expected distinct signed tokens for same session subject")
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(firstToken), nil)
	firstReq.Host = "pulse.example.com"
	firstRec := httptest.NewRecorder()
	h.HandleTrialActivation(firstRec, firstReq)
	if firstRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("first status=%d, want %d", firstRec.Code, http.StatusTemporaryRedirect)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(secondToken), nil)
	secondReq.Host = "pulse.example.com"
	secondRec := httptest.NewRecorder()
	h.HandleTrialActivation(secondRec, secondReq)

	if secondRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("second status=%d, want %d", secondRec.Code, http.StatusTemporaryRedirect)
	}
	if got := secondRec.Header().Get("Location"); got != "/settings/system-pro?trial=replayed" {
		t.Fatalf("second redirect=%q, want %q", got, "/settings/system-pro?trial=replayed")
	}
}

func TestTrialActivation_RequiresPendingInitiationToken(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: "tsi_missing",
		ReturnURL:     "https://pulse.example.com/auth/trial-activate",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil)
	req.Host = "pulse.example.com"
	rec := httptest.NewRecorder()
	h.HandleTrialActivation(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status=%d, want %d", rec.Code, http.StatusTemporaryRedirect)
	}
	if got := rec.Header().Get("Location"); got != "/settings/system-pro?trial=invalid" {
		t.Fatalf("redirect=%q, want %q", got, "/settings/system-pro?trial=invalid")
	}
}

func TestTrialActivation_ConsumedInitiationTokenCannotBeReused(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))
	h.trialRedeemer = func(string) (*hostedTrialRedemptionResponse, error) {
		return issueTrialRedemptionResponse(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()), nil
	}

	returnURL := "https://pulse.example.com/auth/trial-activate"
	instanceToken := issueTrialSignupInitiationToken(t, h, "default", returnURL)

	firstToken, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_first",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken(first): %v", err)
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(firstToken), nil)
	firstReq.Host = "pulse.example.com"
	firstRec := httptest.NewRecorder()
	h.HandleTrialActivation(firstRec, firstReq)
	if firstRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("first status=%d, want %d", firstRec.Code, http.StatusTemporaryRedirect)
	}
	if got := firstRec.Header().Get("Location"); got != "/settings/system-pro?trial=activated" {
		t.Fatalf("first redirect=%q, want %q", got, "/settings/system-pro?trial=activated")
	}

	billingStore := config.NewFileBillingStore(baseDir)
	if err := billingStore.SaveBillingState("default", &entitlements.BillingState{}); err != nil {
		t.Fatalf("SaveBillingState(reset): %v", err)
	}

	secondToken, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_second",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken(second): %v", err)
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(secondToken), nil)
	secondReq.Host = "pulse.example.com"
	secondRec := httptest.NewRecorder()
	h.HandleTrialActivation(secondRec, secondReq)

	if secondRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("second status=%d, want %d", secondRec.Code, http.StatusTemporaryRedirect)
	}
	if got := secondRec.Header().Get("Location"); got != "/settings/system-pro?trial=invalid" {
		t.Fatalf("second redirect=%q, want %q", got, "/settings/system-pro?trial=invalid")
	}
}

func TestTrialActivation_RedeemerFailureReturnsUnavailableAndAllowsRetry(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)
	h := NewLicenseHandlers(mtp, false)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	returnURL := "https://pulse.example.com/auth/trial-activate"
	instanceToken := issueTrialSignupInitiationToken(t, h, "default", returnURL)
	token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:         "default",
		Email:         "owner@example.com",
		InstanceHost:  "pulse.example.com",
		InstanceToken: instanceToken,
		ReturnURL:     returnURL,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "cs_retryable_redeem",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignTrialActivationToken: %v", err)
	}

	h.trialRedeemer = func(string) (*hostedTrialRedemptionResponse, error) {
		return nil, errors.New("control plane unavailable")
	}

	firstReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil)
	firstReq.Host = "pulse.example.com"
	firstRec := httptest.NewRecorder()
	h.HandleTrialActivation(firstRec, firstReq)

	if firstRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("first status=%d, want %d", firstRec.Code, http.StatusTemporaryRedirect)
	}
	if got := firstRec.Header().Get("Location"); got != "/settings/system-pro?trial=unavailable" {
		t.Fatalf("first redirect=%q, want %q", got, "/settings/system-pro?trial=unavailable")
	}

	store := config.NewFileBillingStore(baseDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state != nil && state.SubscriptionState == entitlements.SubStateTrial {
		t.Fatalf("trial state should not be written when redemption fails")
	}

	h.trialRedeemer = func(string) (*hostedTrialRedemptionResponse, error) {
		return issueTrialRedemptionResponse(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()), nil
	}

	secondReq := httptest.NewRequest(http.MethodGet, "/auth/trial-activate?token="+url.QueryEscape(token), nil)
	secondReq.Host = "pulse.example.com"
	secondRec := httptest.NewRecorder()
	h.HandleTrialActivation(secondRec, secondReq)

	if secondRec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("second status=%d, want %d", secondRec.Code, http.StatusTemporaryRedirect)
	}
	if got := secondRec.Header().Get("Location"); got != "/settings/system-pro?trial=activated" {
		t.Fatalf("second redirect=%q, want %q", got, "/settings/system-pro?trial=activated")
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
		if r.URL.Path != "/api/trial-signup/refresh" {
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
		t.Fatal("expected raw entitlement_jwt to be updated")
	}
	if rawState.EntitlementRefreshToken != "etr_test_default" {
		t.Fatalf("raw entitlement_refresh_token=%q, want %q", rawState.EntitlementRefreshToken, "etr_test_default")
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
		if r.URL.Path != "/api/trial-signup/refresh" {
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
	if rawState.TrialStartedAt == nil || *rawState.TrialStartedAt != startedAt {
		t.Fatalf("raw trial_started_at=%v, want %d", rawState.TrialStartedAt, startedAt)
	}
}
