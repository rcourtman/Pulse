package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

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

	token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:        "default",
		Email:        "owner@example.com",
		InstanceHost: "pulse.example.com",
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
	if got := rec.Header().Get("Location"); got != "/settings?trial=activated" {
		t.Fatalf("redirect=%q, want %q", got, "/settings?trial=activated")
	}

	store := config.NewFileBillingStore(baseDir)
	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState: %v", err)
	}
	if state == nil || state.SubscriptionState != entitlements.SubStateTrial {
		t.Fatalf("subscription_state=%q, want %q", state.SubscriptionState, entitlements.SubStateTrial)
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

	token, err := pkglicensing.SignTrialActivationToken(priv, pkglicensing.TrialActivationClaims{
		OrgID:        "default",
		Email:        "owner@example.com",
		InstanceHost: "pulse.example.com",
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
	if got := replayRec.Header().Get("Location"); got != "/settings?trial=replayed" {
		t.Fatalf("replay redirect=%q, want %q", got, "/settings?trial=replayed")
	}
}
