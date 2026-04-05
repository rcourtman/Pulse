package api

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestRefreshHostedEntitlementLeaseOnce_HostedNonDefaultFallsBackToDefaultBillingState(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshCalled := false
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
		refreshCalled = true
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hostedTrialLeaseRefreshResponse{
			EntitlementJWT: issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()),
		})
	}))
	defer refreshServer.Close()

	h := NewLicenseHandlers(mtp, true, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	store := config.NewFileBillingStore(baseDir)
	startedAt := time.Now().Add(-13 * 24 * time.Hour).Unix()
	expiredLease := issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now().Add(-15*24*time.Hour))
	if err := store.SaveBillingState("default", &pkglicensing.BillingState{
		EntitlementJWT:          expiredLease,
		EntitlementRefreshToken: "etr_test_default",
		TrialStartedAt:          &startedAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	refreshed, permanent, err := h.refreshHostedEntitlementLeaseOnce("t-tenant", nil)
	if err != nil {
		t.Fatalf("refreshHostedEntitlementLeaseOnce: %v", err)
	}
	if !refreshed || permanent {
		t.Fatalf("refreshed=%v permanent=%v, want refreshed=true permanent=false", refreshed, permanent)
	}
	if !refreshCalled {
		t.Fatal("expected hosted entitlement refresh request to use the default-org fallback state")
	}

	state, err := store.GetBillingState("default")
	if err != nil {
		t.Fatalf("GetBillingState(default): %v", err)
	}
	if state == nil || state.SubscriptionState != pkglicensing.SubStateTrial {
		t.Fatalf("default subscription_state=%q, want %q", state.SubscriptionState, pkglicensing.SubStateTrial)
	}
	if !containsHostedRefreshCapability(state.Capabilities, pkglicensing.FeatureAIAutoFix) {
		t.Fatalf("default capabilities=%v, want ai_autofix after refresh", state.Capabilities)
	}
}

func TestLicenseHandlersService_HostedNonDefaultRefreshesDefaultFallbackLease(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	if _, err := mtp.GetPersistence("default"); err != nil {
		t.Fatalf("init default persistence: %v", err)
	}
	if _, err := mtp.GetPersistence("t-tenant"); err != nil {
		t.Fatalf("init tenant persistence: %v", err)
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(pkglicensing.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshCalls := 0
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
		refreshCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hostedTrialLeaseRefreshResponse{
			EntitlementJWT: issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now()),
		})
	}))
	defer refreshServer.Close()

	store := config.NewFileBillingStore(baseDir)
	startedAt := time.Now().Add(-13 * 24 * time.Hour).Unix()
	expiredLease := issueTrialEntitlementLease(t, priv, "default", "pulse.example.com", "owner@example.com", time.Now().Add(-15*24*time.Hour))
	if err := store.SaveBillingState("default", &pkglicensing.BillingState{
		EntitlementJWT:          expiredLease,
		EntitlementRefreshToken: "etr_test_default",
		TrialStartedAt:          &startedAt,
	}); err != nil {
		t.Fatalf("SaveBillingState: %v", err)
	}

	handlers := NewLicenseHandlers(mtp, true, &config.Config{
		PublicURL:         "https://pulse.example.com",
		ProTrialSignupURL: refreshServer.URL + "/start-pro-trial",
	})

	ctx := context.WithValue(context.Background(), OrgIDContextKey, "t-tenant")
	service := handlers.Service(ctx)
	if service == nil {
		t.Fatal("service is nil")
	}
	if refreshCalls != 1 {
		t.Fatalf("refreshCalls=%d, want 1", refreshCalls)
	}
	if !service.HasFeature(pkglicensing.FeatureAIAutoFix) {
		t.Fatalf("expected hosted non-default org to regain ai_autofix from the default-org entitlement refresh")
	}

	handlers.stopHostedEntitlementRefreshLoop("default")
}

func containsHostedRefreshCapability(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
