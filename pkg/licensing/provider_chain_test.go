package licensing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// mintTestProviderMSPLicense signs an MSP license with the shared test root
// key, binding the given lease signing public key. setupTestPublicKey must be
// installed for ValidateLicense to accept it.
func mintTestProviderMSPLicense(t *testing.T, tier Tier, leaseSigningPublicKey ed25519.PublicKey, expiresIn time.Duration) string {
	t.Helper()
	testKeyPairInit()

	claims := Claims{
		LicenseID:   "lic_provider_chain_test",
		Email:       "provider@example-msp.com",
		Tier:        tier,
		IssuedAt:    time.Now().Unix(),
		PlanVersion: "msp_starter",
	}
	if expiresIn != 0 {
		claims.ExpiresAt = time.Now().Add(expiresIn).Unix()
	}
	if len(leaseSigningPublicKey) > 0 {
		claims.EntitlementSigningPublicKey = base64.StdEncoding.EncodeToString(leaseSigningPublicKey)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal provider license claims: %v", err)
	}
	return signTestJWT(t, payload, testPrivateKey)
}

func signTestProviderLease(t *testing.T, signingKey ed25519.PrivateKey, providerLicense string, capabilities []string) string {
	t.Helper()

	claims := EntitlementLeaseClaims{
		OrgID:             "default",
		InstanceHost:      "t-acme.pulse.example-msp.com",
		PlanVersion:       "msp_starter",
		SubscriptionState: SubStateActive,
		Capabilities:      capabilities,
		ProviderLicense:   providerLicense,
	}
	token, err := SignEntitlementLeaseToken(signingKey, claims)
	if err != nil {
		t.Fatalf("sign provider lease: %v", err)
	}
	return token
}

func TestEntitlementLeaseProviderChainVerifies(t *testing.T) {
	setupTestPublicKey(t)

	providerPub, providerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	license := mintTestProviderMSPLicense(t, TierMSP, providerPub, 30*24*time.Hour)
	token := signTestProviderLease(t, providerPriv, license, []string{FeatureWhiteLabel, FeatureMultiTenant})

	claims, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken() error = %v, want chain-verified lease", err)
	}
	got := strings.Join(claims.Capabilities, ",")
	if !strings.Contains(got, FeatureWhiteLabel) {
		t.Fatalf("chain-verified lease lost white_label: capabilities = %q", got)
	}
}

func TestEntitlementLeaseProviderChainCapsCapabilities(t *testing.T) {
	setupTestPublicKey(t)

	providerPub, providerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	license := mintTestProviderMSPLicense(t, TierMSP, providerPub, 30*24*time.Hour)
	// multi_user is enterprise-only and relay is Pulse-service-backed; a
	// provider-minted lease must not be able to grant either even though
	// the lease signature verifies.
	token := signTestProviderLease(t, providerPriv, license, []string{FeatureWhiteLabel, FeatureMultiUser, FeatureRelay})

	claims, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now())
	if err != nil {
		t.Fatalf("VerifyEntitlementLeaseToken() error = %v", err)
	}
	for _, capability := range claims.Capabilities {
		switch capability {
		case FeatureMultiUser:
			t.Fatalf("chained lease escaped the MSP capability ceiling: %v", claims.Capabilities)
		case FeatureRelay, FeatureMobileApp, FeaturePushNotifications:
			t.Fatalf("chained lease must not grant Pulse-service-backed capability %q: %v", capability, claims.Capabilities)
		}
	}
	if got := strings.Join(claims.Capabilities, ","); !strings.Contains(got, FeatureWhiteLabel) {
		t.Fatalf("capability cap removed white_label, which MSP leases legitimately carry: %q", got)
	}
}

func TestEntitlementLeaseProviderChainRejectsWrongSigner(t *testing.T) {
	setupTestPublicKey(t)

	providerPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate other key pair: %v", err)
	}
	license := mintTestProviderMSPLicense(t, TierMSP, providerPub, 30*24*time.Hour)
	token := signTestProviderLease(t, otherPriv, license, []string{FeatureWhiteLabel})

	if _, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now()); err == nil {
		t.Fatal("lease signed by a key the provider license does not bind must be rejected")
	}
}

func TestEntitlementLeaseProviderChainRejectsNonMSPLicense(t *testing.T) {
	setupTestPublicKey(t)

	providerPub, providerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	license := mintTestProviderMSPLicense(t, TierPro, providerPub, 30*24*time.Hour)
	token := signTestProviderLease(t, providerPriv, license, []string{FeatureWhiteLabel})

	if _, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now()); err == nil {
		t.Fatal("non-MSP license must not bind an entitlement lease signing key")
	}
}

func TestEntitlementLeaseProviderChainRejectsUnboundLicense(t *testing.T) {
	setupTestPublicKey(t)

	_, providerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	license := mintTestProviderMSPLicense(t, TierMSP, nil, 30*24*time.Hour)
	token := signTestProviderLease(t, providerPriv, license, []string{FeatureWhiteLabel})

	if _, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now()); err == nil {
		t.Fatal("license without an entitlement signing key binding must not chain")
	}
}

func TestEntitlementLeaseProviderChainRejectsForgedLicense(t *testing.T) {
	setupTestPublicKey(t)

	providerPub, providerPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate provider key pair: %v", err)
	}
	// License signed by the provider's own key instead of the Pulse root: a
	// self-issued chain must die at license validation.
	claims := Claims{
		LicenseID:                   "lic_forged",
		Email:                       "attacker@example.com",
		Tier:                        TierMSP,
		IssuedAt:                    time.Now().Unix(),
		ExpiresAt:                   time.Now().Add(30 * 24 * time.Hour).Unix(),
		PlanVersion:                 "msp_starter",
		EntitlementSigningPublicKey: base64.StdEncoding.EncodeToString(providerPub),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal forged license claims: %v", err)
	}
	forgedLicense := signTestJWT(t, payload, providerPriv)
	token := signTestProviderLease(t, providerPriv, forgedLicense, []string{FeatureWhiteLabel})

	if _, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now()); err == nil {
		t.Fatal("a provider license not signed by the Pulse root must not chain")
	}
}

func TestEntitlementLeaseWithoutProviderLicenseUsesDirectRoot(t *testing.T) {
	setupTestPublicKey(t)

	token := signTestProviderLease(t, testPrivateKey, "", []string{FeatureWhiteLabel})
	claims, err := VerifyEntitlementLeaseToken(token, testPublicKey, "t-acme.pulse.example-msp.com", time.Now())
	if err != nil {
		t.Fatalf("direct-root lease verification regressed: %v", err)
	}
	// Direct-root leases are not capability-capped; the signer is Pulse.
	if got := strings.Join(claims.Capabilities, ","); !strings.Contains(got, FeatureWhiteLabel) {
		t.Fatalf("direct-root lease capabilities = %q", got)
	}
}
