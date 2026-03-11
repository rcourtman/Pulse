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
	// TrialEntitlementLeaseIssuer is the JWT issuer for hosted entitlement cache leases.
	TrialEntitlementLeaseIssuer = "pulse-pro-entitlement-lease"

	// TrialEntitlementLeaseAudience is the JWT audience for hosted entitlement cache leases.
	TrialEntitlementLeaseAudience = "pulse-pro-entitlement-cache"
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
	jwt.RegisteredClaims
}

func normalizeEntitlementLeaseClaims(claims *EntitlementLeaseClaims) {
	if claims == nil {
		return
	}

	claims.PlanVersion = CanonicalizePlanVersion(strings.TrimSpace(claims.PlanVersion))
	claims.Capabilities = cloneStringSlice(claims.Capabilities)
	claims.Limits = cloneInt64Map(claims.Limits)
	claims.MetersEnabled = cloneStringSlice(claims.MetersEnabled)
	claims.TrialStartedAt = cloneInt64Ptr(claims.TrialStartedAt)
	claims.TrialEndsAt = cloneInt64Ptr(claims.TrialEndsAt)
	if limit, known := CloudPlanAgentLimits[claims.PlanVersion]; known {
		if claims.Limits == nil {
			claims.Limits = map[string]int64{}
		}
		claims.Limits["max_agents"] = int64(limit)
	}
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
		claims.Issuer = TrialEntitlementLeaseIssuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = jwt.ClaimStrings{TrialEntitlementLeaseAudience}
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
		jwt.WithIssuer(TrialEntitlementLeaseIssuer),
		jwt.WithAudience(TrialEntitlementLeaseAudience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	}
	if skipTimeValidation {
		parseOpts = append(parseOpts, jwt.WithoutClaimsValidation())
	}
	parsed, err := jwt.ParseWithClaims(
		token,
		claims,
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodEdDSA.Alg() {
				return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
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
		return nil, ErrTrialActivationHostMismatch
	}
	normalizeEntitlementLeaseClaims(claims)
	return claims, nil
}

// ResolveEntitlementLeaseBillingState applies a signed hosted entitlement lease
// to a locally cached billing state. Invalid or expired leases fail closed.
func ResolveEntitlementLeaseBillingState(state BillingState, expectedInstanceHost string, now time.Time) BillingState {
	token := strings.TrimSpace(state.EntitlementJWT)
	if token == "" {
		return normalizeTrialExpiry(state, now)
	}
	publicKey, err := TrialActivationPublicKey()
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
