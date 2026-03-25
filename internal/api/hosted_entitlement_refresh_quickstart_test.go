package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	entitlements "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

func TestRefreshHostedEntitlementLeaseOnce_GrantsQuickstartCredits(t *testing.T) {
	baseDir := t.TempDir()
	mtp := config.NewMultiTenantPersistence(baseDir)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	t.Setenv(entitlements.TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	refreshServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/entitlements/refresh" {
			http.NotFound(w, r)
			return
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
	if state == nil {
		t.Fatal("expected billing state after hosted entitlement refresh")
	}
	if !state.QuickstartCreditsGranted {
		t.Fatal("expected hosted entitlement refresh to grant quickstart credits")
	}
	if state.QuickstartCreditsGrantedAt == nil {
		t.Fatal("expected hosted entitlement refresh to record quickstart grant timestamp")
	}
}
