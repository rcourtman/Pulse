package licensing

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// HostedEntitlementLeaseIssuer is the JWT issuer for hosted entitlement cache leases.
	HostedEntitlementLeaseIssuer = "pulse-pro-entitlement-lease"

	// HostedEntitlementLeaseAudience is the JWT audience for hosted entitlement cache leases.
	HostedEntitlementLeaseAudience = "pulse-pro-entitlement-cache"

	// TrialEntitlementLeaseIssuer is the legacy exported name for hosted entitlement leases.
	TrialEntitlementLeaseIssuer = HostedEntitlementLeaseIssuer

	// TrialEntitlementLeaseAudience is the legacy exported name for hosted entitlement leases.
	TrialEntitlementLeaseAudience = HostedEntitlementLeaseAudience
)

var (
	ErrEntitlementLeasePrivateKeyInvalid = errors.New("invalid entitlement lease private key")
	ErrEntitlementLeasePublicKeyInvalid  = errors.New("invalid entitlement lease public key")
	ErrEntitlementLeaseOrgIDMissing      = errors.New("entitlement lease org_id is required")
	ErrEntitlementLeaseInstanceMissing   = errors.New("entitlement lease instance host is required")
)

// EntitlementLeaseClaims is a signed hosted entitlement snapshot cached on the
// Pulse instance. It is the authority for hosted trial/pro entitlement when no
// activation-key license is active locally.
type EntitlementLeaseClaims struct {
	OrgID             string            `json:"org_id"`
	Email             string            `json:"email,omitempty"`
	InstanceHost      string            `json:"instance_host"`
	PlanVersion       string            `json:"plan_version"`
	SubscriptionState SubscriptionState `json:"subscription_state"`
	Capabilities      []string          `json:"capabilities,omitempty"`
	Limits            map[string]int64  `json:"limits,omitempty"`
	MetersEnabled     []string          `json:"meters_enabled,omitempty"`
	TrialStartedAt    *int64            `json:"trial_started_at,omitempty"`
	TrialEndsAt       *int64            `json:"trial_ends_at,omitempty"`

	// ProviderLicense carries the Pulse-signed provider MSP license whose
	// entitlement_signing_public_key claim binds the key this lease was
	// signed with. Release builds verify the chain (embedded Pulse root →
	// provider license → lease signature), so provider-hosted control
	// planes can mint verifiable leases for the client runtimes they
	// operate without holding Pulse's signing key.
	ProviderLicense string `json:"provider_license,omitempty"`

	jwt.RegisteredClaims
}

func normalizeEntitlementLeaseClaims(claims *EntitlementLeaseClaims) {
	if claims == nil {
		return
	}

	claims.PlanVersion = CanonicalizePlanVersion(strings.TrimSpace(claims.PlanVersion))
	claims.Capabilities = cloneStringSlice(claims.Capabilities)
	claims.Limits = NormalizeMonitoredSystemLimits(claims.Limits)
	stripSelfHostedCommercialVolumeCaps(claims.Limits, claims.PlanVersion, "", false)
	claims.MetersEnabled = cloneStringSlice(claims.MetersEnabled)
	claims.TrialStartedAt = cloneInt64Ptr(claims.TrialStartedAt)
	claims.TrialEndsAt = cloneInt64Ptr(claims.TrialEndsAt)
}

// SignEntitlementLeaseToken signs a hosted entitlement lease JWT.
func SignEntitlementLeaseToken(privateKey ed25519.PrivateKey, claims EntitlementLeaseClaims) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", ErrEntitlementLeasePrivateKeyInvalid
	}
	claims.OrgID = strings.TrimSpace(claims.OrgID)
	if claims.OrgID == "" {
		return "", ErrEntitlementLeaseOrgIDMissing
	}
	claims.InstanceHost = normalizeHost(claims.InstanceHost)
	if claims.InstanceHost == "" {
		return "", ErrEntitlementLeaseInstanceMissing
	}
	normalizeEntitlementLeaseClaims(&claims)

	now := time.Now().UTC()
	if claims.IssuedAt == nil {
		claims.IssuedAt = jwt.NewNumericDate(now)
	}
	if claims.ExpiresAt == nil {
		expiresAt := now.Add(DefaultTrialDuration)
		if claims.TrialEndsAt != nil && *claims.TrialEndsAt > 0 {
			expiresAt = time.Unix(*claims.TrialEndsAt, 0).UTC()
		}
		claims.ExpiresAt = jwt.NewNumericDate(expiresAt)
	}
	if strings.TrimSpace(claims.Issuer) == "" {
		claims.Issuer = HostedEntitlementLeaseIssuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = jwt.ClaimStrings{HostedEntitlementLeaseAudience}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	signed, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("sign entitlement lease token: %w", err)
	}
	return signed, nil
}

// VerifyEntitlementLeaseToken verifies a hosted entitlement lease JWT, including expiry.
func VerifyEntitlementLeaseToken(token string, publicKey ed25519.PublicKey, expectedInstanceHost string, now time.Time) (*EntitlementLeaseClaims, error) {
	return parseEntitlementLeaseToken(token, publicKey, expectedInstanceHost, now, false)
}

// ParseEntitlementLeaseToken verifies signature and host binding without enforcing expiry.
func ParseEntitlementLeaseToken(token string, publicKey ed25519.PublicKey, expectedInstanceHost string) (*EntitlementLeaseClaims, error) {
	return parseEntitlementLeaseToken(token, publicKey, expectedInstanceHost, time.Time{}, true)
}

func parseEntitlementLeaseToken(token string, publicKey ed25519.PublicKey, expectedInstanceHost string, now time.Time, skipTimeValidation bool) (*EntitlementLeaseClaims, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrEntitlementLeasePublicKeyInvalid
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("entitlement lease token is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	claims := &EntitlementLeaseClaims{}
	parseOpts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodEdDSA.Alg()}),
		jwt.WithIssuer(HostedEntitlementLeaseIssuer),
		jwt.WithAudience(HostedEntitlementLeaseAudience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	}
	if skipTimeValidation {
		parseOpts = append(parseOpts, jwt.WithoutClaimsValidation())
	}
	chained := false
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
			}
			// A lease carrying a provider license selects the provider's
			// bound signing key instead of the direct trust root. The
			// provider license itself must verify against the embedded
			// Pulse root before its bound key is trusted, so the chain
			// never widens beyond Pulse-issued MSP licenses.
			if leaseClaims, ok := t.Claims.(*EntitlementLeaseClaims); ok {
				if providerLicense := strings.TrimSpace(leaseClaims.ProviderLicense); providerLicense != "" {
					providerKey, err := providerLeaseSigningKeyFromLicense(providerLicense)
					if err != nil {
						return nil, err
					}
					chained = true
					return providerKey, nil
				}
			}
			return publicKey, nil
		},
		parseOpts...,
	)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("entitlement lease token is invalid")
	}
	if chained {
		capProviderChainedLeaseClaims(claims)
	}

	claims.OrgID = strings.TrimSpace(claims.OrgID)
	if claims.OrgID == "" {
		return nil, ErrEntitlementLeaseOrgIDMissing
	}
	claims.InstanceHost = normalizeHost(claims.InstanceHost)
	if claims.InstanceHost == "" {
		return nil, ErrEntitlementLeaseInstanceMissing
	}
	expected := normalizeHost(expectedInstanceHost)
	if expected != "" && !strings.EqualFold(claims.InstanceHost, expected) {
		return nil, ErrHostedEntitlementHostMismatch
	}
	normalizeEntitlementLeaseClaims(claims)
	return claims, nil
}

// providerLeaseSigningKeyFromLicense validates a Pulse-signed provider MSP
// license against the runtime license trust root and returns the entitlement
// lease signing key it binds. Every failure is terminal for the lease: an
// unverifiable provider license must never fall back to the direct root,
// or a forged chain would downgrade into a confusing signature error.
func providerLeaseSigningKeyFromLicense(licenseKey string) (ed25519.PublicKey, error) {
	license, err := ValidateLicense(licenseKey)
	if err != nil {
		return nil, fmt.Errorf("provider license in entitlement lease is invalid: %w", err)
	}
	if license == nil {
		return nil, errors.New("provider license in entitlement lease is invalid")
	}
	if license.Claims.Tier != TierMSP {
		return nil, fmt.Errorf("provider license tier %q cannot bind an entitlement lease signing key", license.Claims.Tier)
	}
	encoded := strings.TrimSpace(license.Claims.EntitlementSigningPublicKey)
	if encoded == "" {
		return nil, errors.New("provider license does not bind an entitlement lease signing key")
	}
	key, err := DecodePublicKey(encoded)
	if err != nil {
		return nil, fmt.Errorf("provider license entitlement signing key is malformed: %w", err)
	}
	return key, nil
}

// ProviderChainedLeaseCapabilities returns the capability ceiling for
// entitlement leases verified through a provider MSP license chain: the MSP
// tier set plus white_label (branded per-client reports are the hosted MSP
// packaging of that feature), minus capabilities backed by Pulse-operated
// services (relay, mobile app pairing, push routing). A provider-hosted
// deployment cannot grant those — client runtimes holding them would loop
// doomed registrations against Pulse's relay with credentials it can never
// verify.
func ProviderChainedLeaseCapabilities() []string {
	excluded := map[string]struct{}{
		FeatureRelay:             {},
		FeatureMobileApp:         {},
		FeaturePushNotifications: {},
	}
	capabilities := make([]string, 0)
	for _, capability := range DeriveCapabilitiesFromTier(TierMSP, []string{FeatureWhiteLabel}) {
		if _, drop := excluded[capability]; drop {
			continue
		}
		capabilities = append(capabilities, capability)
	}
	return capabilities
}

// capProviderChainedLeaseClaims bounds a chain-verified lease to
// ProviderChainedLeaseCapabilities. A provider key can therefore never mint
// enterprise-only, Pulse-service-backed, or future capabilities, regardless
// of what the lease claims.
func capProviderChainedLeaseClaims(claims *EntitlementLeaseClaims) {
	if claims == nil {
		return
	}
	allowed := make(map[string]struct{})
	for _, capability := range ProviderChainedLeaseCapabilities() {
		allowed[capability] = struct{}{}
	}
	kept := make([]string, 0, len(claims.Capabilities))
	for _, capability := range claims.Capabilities {
		if _, ok := allowed[capability]; ok {
			kept = append(kept, capability)
		}
	}
	claims.Capabilities = kept
}

// ResolveEntitlementLeaseBillingState applies a signed hosted entitlement lease
// to a locally cached billing state. Invalid or expired leases fail closed.
func ResolveEntitlementLeaseBillingState(state BillingState, expectedInstanceHost string, now time.Time) BillingState {
	token := strings.TrimSpace(state.EntitlementJWT)
	if token == "" {
		return normalizeTrialExpiry(state, now)
	}
	publicKey, err := HostedEntitlementPublicKey()
	if err != nil {
		return entitlementLeaseFallbackState(state, now)
	}
	if claims, err := VerifyEntitlementLeaseToken(token, publicKey, expectedInstanceHost, now); err == nil {
		return entitlementLeaseClaimsToBillingState(state, claims, now)
	}
	if claims, err := ParseEntitlementLeaseToken(token, publicKey, expectedInstanceHost); err == nil {
		resolved := entitlementLeaseClaimsToBillingState(state, claims, now)
		resolved.SubscriptionState = SubStateExpired
		resolved.Capabilities = []string{}
		resolved.Limits = map[string]int64{}
		resolved.MetersEnabled = []string{}
		resolved.PlanVersion = string(SubStateExpired)
		return normalizeTrialExpiry(resolved, now)
	}
	return entitlementLeaseFallbackState(state, now)
}

func entitlementLeaseClaimsToBillingState(state BillingState, claims *EntitlementLeaseClaims, now time.Time) BillingState {
	resolved := cloneBillingState(state)
	resolved.Capabilities = cloneStringSlice(claims.Capabilities)
	resolved.Limits = cloneInt64Map(claims.Limits)
	resolved.MetersEnabled = cloneStringSlice(claims.MetersEnabled)
	resolved.PlanVersion = claims.PlanVersion
	resolved.SubscriptionState = claims.SubscriptionState
	resolved.TrialStartedAt = cloneInt64Ptr(claims.TrialStartedAt)
	resolved.TrialEndsAt = cloneInt64Ptr(claims.TrialEndsAt)
	return normalizeTrialExpiry(resolved, now)
}

func entitlementLeaseFallbackState(state BillingState, now time.Time) BillingState {
	fallback := cloneBillingState(state)
	fallback.Capabilities = []string{}
	fallback.Limits = map[string]int64{}
	fallback.MetersEnabled = []string{}
	if fallback.TrialStartedAt != nil {
		fallback.SubscriptionState = SubStateExpired
		fallback.PlanVersion = string(SubStateExpired)
	} else {
		fallback.SubscriptionState = ""
		fallback.PlanVersion = ""
	}
	return normalizeTrialExpiry(fallback, now)
}
