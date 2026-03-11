package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSignAndVerifyEntitlementLeaseToken(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	now := time.Unix(1710000000, 0).UTC()
	startedAt, endsAt := TrialWindow(now, DefaultTrialDuration)

	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		Email:             "owner@example.com",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       string(SubStateTrial),
		SubscriptionState: SubStateTrial,
		Capabilities:      []string{"ai_autofix"},
		Limits:            map[string]int64{"max_agents": 25},
		MetersEnabled:     []string{"agents"},
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(time.Unix(endsAt, 0).UTC()),
			Subject:   "trial_req_123",
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	claims, err := VerifyEntitlementLeaseToken(token, pub, "pulse.example.com", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.OrgID != "default" {
		t.Fatalf("claims.OrgID=%q, want %q", claims.OrgID, "default")
	}
	if claims.InstanceHost != "pulse.example.com" {
		t.Fatalf("claims.InstanceHost=%q, want %q", claims.InstanceHost, "pulse.example.com")
	}
	if claims.SubscriptionState != SubStateTrial {
		t.Fatalf("claims.SubscriptionState=%q, want %q", claims.SubscriptionState, SubStateTrial)
	}
	if got := claims.Limits["max_agents"]; got != 25 {
		t.Fatalf("claims.Limits[max_agents]=%d, want %d", got, 25)
	}
}

func TestResolveEntitlementLeaseBillingStateExpired(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	embeddedBefore := EmbeddedPublicKey
	EmbeddedPublicKey = ""
	t.Cleanup(func() { EmbeddedPublicKey = embeddedBefore })
	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	start := time.Unix(1710000000, 0).UTC().Add(-15 * 24 * time.Hour)
	startedAt, endsAt := TrialWindow(start, DefaultTrialDuration)
	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       string(SubStateTrial),
		SubscriptionState: SubStateTrial,
		Capabilities:      []string{"ai_autofix"},
		TrialStartedAt:    &startedAt,
		TrialEndsAt:       &endsAt,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(start),
			ExpiresAt: jwt.NewNumericDate(time.Unix(endsAt, 0).UTC()),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	resolved := ResolveEntitlementLeaseBillingState(BillingState{
		EntitlementJWT: token,
	}, "pulse.example.com", time.Now().UTC())
	if resolved.SubscriptionState != SubStateExpired {
		t.Fatalf("resolved.SubscriptionState=%q, want %q", resolved.SubscriptionState, SubStateExpired)
	}
	if len(resolved.Capabilities) != 0 {
		t.Fatalf("resolved.Capabilities=%v, want none", resolved.Capabilities)
	}
	if resolved.TrialStartedAt == nil || *resolved.TrialStartedAt != startedAt {
		t.Fatalf("resolved.TrialStartedAt=%v, want %d", resolved.TrialStartedAt, startedAt)
	}
}

func TestVerifyEntitlementLeaseTokenHostMismatch(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse-a.example.com",
		SubscriptionState: SubStateTrial,
		PlanVersion:       string(SubStateTrial),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	_, err = VerifyEntitlementLeaseToken(token, pub, "pulse-b.example.com", time.Now())
	if !errors.Is(err, ErrTrialActivationHostMismatch) {
		t.Fatalf("VerifyEntitlementLeaseToken() error=%v, want %v", err, ErrTrialActivationHostMismatch)
	}
}

func TestEntitlementLeaseCanonicalizesCloudPlanVersionAndLimits(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       " cloud_v1 ",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_agents": 999},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	claims, err := VerifyEntitlementLeaseToken(token, pub, "pulse.example.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.PlanVersion != "cloud_starter" {
		t.Fatalf("claims.PlanVersion=%q, want %q", claims.PlanVersion, "cloud_starter")
	}
	if got := claims.Limits["max_agents"]; got != 10 {
		t.Fatalf("claims.Limits[max_agents]=%d, want %d", got, 10)
	}
}

func TestEntitlementLeasePreservesNonCloudLimits(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       "pro-v2",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_agents": 42},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	claims, err := VerifyEntitlementLeaseToken(token, pub, "pulse.example.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.PlanVersion != "pro-v2" {
		t.Fatalf("claims.PlanVersion=%q, want %q", claims.PlanVersion, "pro-v2")
	}
	if got := claims.Limits["max_agents"]; got != 42 {
		t.Fatalf("claims.Limits[max_agents]=%d, want %d", got, 42)
	}
}

func TestEntitlementLeasePreservesMissingPlanVersion(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       "   ",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_agents": 42},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	claims, err := VerifyEntitlementLeaseToken(token, pub, "pulse.example.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken: %v", err)
	}
	if claims.PlanVersion != "" {
		t.Fatalf("claims.PlanVersion=%q, want empty", claims.PlanVersion)
	}
	if claims.SubscriptionState != SubStateActive {
		t.Fatalf("claims.SubscriptionState=%q, want %q", claims.SubscriptionState, SubStateActive)
	}
	if got := claims.Limits["max_agents"]; got != 42 {
		t.Fatalf("claims.Limits[max_agents]=%d, want %d", got, 42)
	}
}

func TestResolveEntitlementLeaseBillingStatePreservesMissingPlanVersion(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	embeddedBefore := EmbeddedPublicKey
	EmbeddedPublicKey = ""
	t.Cleanup(func() { EmbeddedPublicKey = embeddedBefore })
	t.Setenv(TrialActivationPublicKeyEnvVar, base64.StdEncoding.EncodeToString(pub))

	token, err := SignEntitlementLeaseToken(priv, EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "pulse.example.com",
		PlanVersion:       "   ",
		SubscriptionState: SubStateActive,
		Limits:            map[string]int64{"max_agents": 42},
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
		},
	})
	if err != nil {
		t.Fatalf("SignEntitlementLeaseToken: %v", err)
	}

	resolved := ResolveEntitlementLeaseBillingState(BillingState{
		EntitlementJWT: token,
	}, "pulse.example.com", time.Now().UTC())
	if resolved.PlanVersion != "" {
		t.Fatalf("resolved.PlanVersion=%q, want empty", resolved.PlanVersion)
	}
	if resolved.SubscriptionState != SubStateActive {
		t.Fatalf("resolved.SubscriptionState=%q, want %q", resolved.SubscriptionState, SubStateActive)
	}
	if got := resolved.Limits["max_agents"]; got != 42 {
		t.Fatalf("resolved.Limits[max_agents]=%d, want %d", got, 42)
	}
}
