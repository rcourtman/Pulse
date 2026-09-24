package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestNormalizeBillingStatePreservesMissingPlanVersionAndScrubsRetiredMonitoringLimit(t *testing.T) {
	state := &BillingState{
		PlanVersion:       "   ",
		Limits:            map[string]int64{"max_monitored_systems": 42, "max_guests": 7},
		SubscriptionState: SubscriptionState(" ACTIVE "),
	}

	normalized := NormalizeBillingState(state)
	if normalized.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", normalized.PlanVersion)
	}
	if normalized.SubscriptionState != SubStateActive {
		t.Fatalf("subscription_state=%q, want %q", normalized.SubscriptionState, SubStateActive)
	}
	if _, ok := normalized.Limits["max_monitored_systems"]; ok {
		t.Fatalf("limits retained retired max_monitored_systems: %v", normalized.Limits)
	}
	if got := normalized.Limits["max_guests"]; got != 7 {
		t.Fatalf("limits[max_guests]=%d, want %d", got, 7)
	}
}

func TestNormalizeEntitlementLeaseClaimsPreservesMissingPlanVersionAndScrubsRetiredMonitoringLimit(t *testing.T) {
	claims := &EntitlementLeaseClaims{
		PlanVersion:       "   ",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_monitored_systems": 42, "max_guests": 7},
	}

	normalizeEntitlementLeaseClaims(claims)
	if claims.PlanVersion != "" {
		t.Fatalf("plan_version=%q, want empty", claims.PlanVersion)
	}
	if _, ok := claims.Limits["max_monitored_systems"]; ok {
		t.Fatalf("limits retained retired max_monitored_systems: %v", claims.Limits)
	}
	if got := claims.Limits["max_guests"]; got != 7 {
		t.Fatalf("limits[max_guests]=%d, want %d", got, 7)
	}
}

func TestClaimsPreserveMissingPlanVersionAndScrubRetiredMonitoringLimit(t *testing.T) {
	claims := &Claims{
		Tier:        TierCloud,
		PlanVersion: "   ",
		Limits:      map[string]int64{"max_monitored_systems": 42, "max_guests": 7},
	}

	if got := claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	limits := claims.EffectiveLimits()
	if _, ok := limits["max_monitored_systems"]; ok {
		t.Fatalf("EffectiveLimits retained retired max_monitored_systems: %v", limits)
	}
	if got := limits["max_guests"]; got != 7 {
		t.Fatalf("EffectiveLimits()[max_guests]=%d, want %d", got, 7)
	}
}

func TestTokenSourcePreservesMissingPlanVersionContract(t *testing.T) {
	source := NewTokenSource(stubTokenClaims{
		planVersion:       "",
		subscriptionState: SubStateActive,
		limits:            map[string]int64{"max_guests": 7},
	})

	if got := source.PlanVersion(); got != "" {
		t.Fatalf("PlanVersion()=%q, want empty", got)
	}
	if got := source.SubscriptionState(); got != SubStateActive {
		t.Fatalf("SubscriptionState()=%q, want %q", got, SubStateActive)
	}
	if got := source.Limits()["max_guests"]; got != 7 {
		t.Fatalf("Limits()[max_guests]=%d, want %d", got, 7)
	}
}

func TestCloudClaimsMissingPlanVersionDoesNotReintroduceMonitoringLimit(t *testing.T) {
	claims := &Claims{
		Tier:        TierCloud,
		PlanVersion: "   ",
	}

	if got := claims.EntitlementPlanVersion(); got != "" {
		t.Fatalf("EntitlementPlanVersion()=%q, want empty", got)
	}
	if _, ok := claims.EffectiveLimits()["max_monitored_systems"]; ok {
		t.Fatalf("EffectiveLimits retained retired max_monitored_systems: %v", claims.EffectiveLimits())
	}
}

// A provider control plane must keep starting on a lapsed licence so its
// portal can sell the renewal, but the licence must still be authentic, and
// client runtimes must still refuse to take MSP capabilities from it.
func TestValidateLicenseAllowingLapseReturnsLapsedButStillAuthenticLicence(t *testing.T) {
	setupTestPublicKey(t)
	providerPub, providerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	lapsed := mintTestProviderMSPLicense(t, TierMSP, providerPub, -30*24*time.Hour)

	if _, err := ValidateLicense(lapsed); !errors.Is(err, ErrExpiredLicense) {
		t.Fatalf("ValidateLicense(lapsed) error = %v, want ErrExpiredLicense", err)
	}
	license, err := ValidateLicenseAllowingLapse(lapsed)
	if err != nil || license == nil {
		t.Fatalf("ValidateLicenseAllowingLapse(lapsed) = %v, %v; want the licence", license, err)
	}
	if !license.IsExpired() || license.GracePeriodEnd == nil || !license.GracePeriodEnd.Before(time.Now()) {
		t.Fatalf("lapsed licence expired=%v graceEnd=%v; want expired with a grace end in the past", license.IsExpired(), license.GracePeriodEnd)
	}

	// Authenticity is not relaxed: a licence not signed by the Pulse root is
	// still refused.
	claims := Claims{LicenseID: "lic_forged", Email: "attacker@example.com", Tier: TierMSP, IssuedAt: time.Now().Unix(),
		ExpiresAt: time.Now().Add(-30 * 24 * time.Hour).Unix(), PlanVersion: "msp_starter",
		EntitlementSigningPublicKey: base64.StdEncoding.EncodeToString(providerPub)}
	payload, _ := json.Marshal(claims)
	if _, err := ValidateLicenseAllowingLapse(signTestJWT(t, payload, providerPriv)); err == nil {
		t.Fatal("ValidateLicenseAllowingLapse accepted a licence not signed by the Pulse root")
	}

	// Enforcement stays with the runtime: a lease chained to the lapsed
	// licence does not verify.
	token := signTestProviderLease(t, providerPriv, lapsed, []string{FeatureWhiteLabel, FeatureMultiTenant})
	if _, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now()); err == nil {
		t.Fatal("a lease chained to a lapsed provider licence must not verify")
	}
}
