package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/license/entitlements"
)

// Public key for license validation (Ed25519).
// This will be embedded at build time or set via SetPublicKey.
// For development, leave empty to skip validation.
var (
	publicKeyMu sync.RWMutex
	publicKey   ed25519.PublicKey
)

// SetPublicKey sets the public key for license validation.
// This should be called during initialization with the production key.
func SetPublicKey(key ed25519.PublicKey) {
	publicKeyMu.Lock()
	defer publicKeyMu.Unlock()

	if len(key) == 0 {
		publicKey = nil
		return
	}

	keyCopy := make(ed25519.PublicKey, len(key))
	copy(keyCopy, key)
	publicKey = keyCopy
}

func currentPublicKey() ed25519.PublicKey {
	publicKeyMu.RLock()
	defer publicKeyMu.RUnlock()

	if len(publicKey) == 0 {
		return nil
	}

	keyCopy := make(ed25519.PublicKey, len(publicKey))
	copy(keyCopy, publicKey)
	return keyCopy
}

// License errors
var (
	ErrInvalidLicense     = errors.New("invalid license key")
	ErrExpiredLicense     = errors.New("license has expired")
	ErrMalformedLicense   = errors.New("malformed license key")
	ErrSignatureInvalid   = errors.New("license signature invalid")
	ErrFeatureNotIncluded = errors.New("feature not included in license")
	ErrNoPublicKey        = errors.New("no public key configured for validation")
)

// SubscriptionState represents the subscription lifecycle state.
type SubscriptionState = entitlements.SubscriptionState

const (
	SubStateTrial     = entitlements.SubStateTrial
	SubStateActive    = entitlements.SubStateActive
	SubStateGrace     = entitlements.SubStateGrace
	SubStateExpired   = entitlements.SubStateExpired
	SubStateSuspended = entitlements.SubStateSuspended
	SubStateCanceled  = entitlements.SubStateCanceled
)

// LimitCheckResult represents the result of evaluating a limit.
type LimitCheckResult = entitlements.LimitCheckResult

const (
	LimitAllowed   = entitlements.LimitAllowed
	LimitSoftBlock = entitlements.LimitSoftBlock
	LimitHardBlock = entitlements.LimitHardBlock
)

// Claims represents the JWT claims in a Pulse Pro license.
type Claims struct {
	// License ID (unique identifier)
	LicenseID string `json:"lid"`

	// Email of the license holder
	Email string `json:"email"`

	// License tier (pro, pro_annual, lifetime, msp, enterprise)
	Tier Tier `json:"tier"`

	// Issued at (Unix timestamp)
	IssuedAt int64 `json:"iat"`

	// Expires at (Unix timestamp, 0 for lifetime)
	ExpiresAt int64 `json:"exp,omitempty"`

	// Features explicitly granted (optional, tier implies features)
	Features []string `json:"features,omitempty"`

	// Max nodes (0 = unlimited)
	MaxNodes int `json:"max_nodes,omitempty"`

	// Max guests (0 = unlimited)
	MaxGuests int `json:"max_guests,omitempty"`

	// Entitlement primitives (B1) - when present, these override tier-based derivation.
	// When absent (nil/empty), entitlements are derived from Tier + existing fields.
	Capabilities  []string          `json:"capabilities,omitempty"`
	Limits        map[string]int64  `json:"limits,omitempty"`
	MetersEnabled []string          `json:"meters_enabled,omitempty"`
	PlanVersion   string            `json:"plan_version,omitempty"`
	SubState      SubscriptionState `json:"subscription_state,omitempty"`
}

// EffectiveCapabilities returns explicit capabilities when present; otherwise tier-derived capabilities.
func (c Claims) EffectiveCapabilities() []string {
	if c.Capabilities != nil && len(c.Capabilities) > 0 {
		return c.Capabilities
	}
	return DeriveCapabilitiesFromTier(c.Tier, c.Features)
}

// EffectiveLimits returns explicit limits when present; otherwise limits derived from legacy fields.
func (c Claims) EffectiveLimits() map[string]int64 {
	if c.Limits != nil && len(c.Limits) > 0 {
		return c.Limits
	}

	limits := make(map[string]int64)
	if c.MaxNodes > 0 {
		limits["max_nodes"] = int64(c.MaxNodes)
	}
	if c.MaxGuests > 0 {
		limits["max_guests"] = int64(c.MaxGuests)
	}
	return limits
}

// EntitlementMetersEnabled returns metering keys for evaluator sources.
func (c *Claims) EntitlementMetersEnabled() []string {
	if c == nil {
		return nil
	}
	return c.MetersEnabled
}

// EntitlementPlanVersion returns plan metadata for evaluator sources.
func (c *Claims) EntitlementPlanVersion() string {
	if c == nil {
		return ""
	}
	return c.PlanVersion
}

// EntitlementSubscriptionState returns normalized subscription state for evaluator sources.
func (c *Claims) EntitlementSubscriptionState() SubscriptionState {
	if c == nil || c.SubState == "" {
		return SubStateActive
	}
	return c.SubState
}

// License represents a validated Pulse Pro license.
type License struct {
	// Raw JWT token
	Raw string `json:"-"`

	// Validated claims
	Claims Claims `json:"claims"`

	// Validation metadata
	ValidatedAt time.Time `json:"validated_at"`

	// Grace period end (if license was validated during grace period)
	GracePeriodEnd *time.Time `json:"grace_period_end,omitempty"`
}

// IsExpired checks if the license has expired.
func (l *License) IsExpired() bool {
	if l.Claims.ExpiresAt == 0 {
		return false // Lifetime license never expires
	}
	return time.Now().Unix() > l.Claims.ExpiresAt
}

// IsLifetime returns true if this is a lifetime license.
func (l *License) IsLifetime() bool {
	return l.Claims.ExpiresAt == 0 || l.Claims.Tier == TierLifetime
}

// DaysRemaining returns the number of days until expiration.
// Returns -1 for lifetime licenses.
func (l *License) DaysRemaining() int {
	if l.IsLifetime() {
		return -1
	}
	remaining := time.Until(time.Unix(l.Claims.ExpiresAt, 0))
	if remaining < 0 {
		return 0
	}
	return int(remaining.Hours() / 24)
}

// ExpiresAt returns the expiration time, or nil for lifetime.
func (l *License) ExpiresAt() *time.Time {
	if l.IsLifetime() {
		return nil
	}
	t := time.Unix(l.Claims.ExpiresAt, 0)
	return &t
}

// HasFeature checks if the license grants a specific feature.
func (l *License) HasFeature(feature string) bool {
	for _, capability := range l.Claims.EffectiveCapabilities() {
		if capability == feature {
			return true
		}
	}
	return false
}

// AllFeatures returns all features granted by this license.
func (l *License) AllFeatures() []string {
	features := append([]string(nil), l.Claims.EffectiveCapabilities()...)
	sort.Strings(features)
	return features
}

// Service manages license validation and feature gating.
type Service struct {
	mu      sync.RWMutex
	license *License

	// Grace period duration when license validation fails
	gracePeriod time.Duration

	// Callback when license changes
	onLicenseChange func(*License)

	// Optional canonical evaluator for B2 entitlement checks.
	// When set and no JWT license is active, HasFeature/Status/SubscriptionState
	// delegate to evaluator primitives (capabilities/limits/subscription state).
	// When nil, falls through to existing tier-based logic.
	evaluator *entitlements.Evaluator

	// Optional subscription state machine hook.
	// Only tracks whether a hook is configured; current derivation remains claim/license based.
	stateMachineConfigured bool
}

// DefaultGracePeriod is the duration after license expiration during which
// features remain available. All grace period logic MUST use this constant.
const DefaultGracePeriod = 7 * 24 * time.Hour

// NewService creates a new license service.
func NewService() *Service {
	return &Service{
		gracePeriod: DefaultGracePeriod,
	}
}

// ensureGracePeriodEnd sets the grace period end time on the license if not already set.
// Must be called while holding s.mu.
func (s *Service) ensureGracePeriodEnd() {
	if s.license != nil && s.license.GracePeriodEnd == nil {
		gracePeriodEnd := time.Unix(s.license.Claims.ExpiresAt, 0).Add(s.gracePeriod)
		s.license.GracePeriodEnd = &gracePeriodEnd
	}
}

// SetLicenseChangeCallback sets a callback for license change events.
func (s *Service) SetLicenseChangeCallback(cb func(*License)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onLicenseChange = cb
}

// SetEvaluator overrides the entitlement evaluator.
// In normal operation, Activate and Clear manage the evaluator automatically.
// This method exists for testing; production code should not call it directly.
func (s *Service) SetEvaluator(eval *entitlements.Evaluator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evaluator = eval
}

// SetStateMachine sets the optional subscription state machine hook.
// This is nil-safe and does not alter feature entitlement behavior.
func (s *Service) SetStateMachine(sm any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stateMachineConfigured = sm != nil
}

// Evaluator returns the current evaluator, or nil if not set.
func (s *Service) Evaluator() *entitlements.Evaluator {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.evaluator
}

// Activate validates and activates a license key.
func (s *Service) Activate(licenseKey string) (*License, error) {
	license, err := ValidateLicense(licenseKey)
	if err != nil {
		return nil, fmt.Errorf("validate license: %w", err)
	}

	s.mu.Lock()
	s.license = cloneLicense(license)
	source := entitlements.NewTokenSource(&s.license.Claims)
	s.evaluator = entitlements.NewEvaluator(source)
	cb := s.onLicenseChange
	snapshot := cloneLicense(s.license)
	s.mu.Unlock()

	if cb != nil {
		cb(snapshot)
	}

	// Keep legacy mutability in explicit dev-mode to avoid breaking existing
	// test fixtures that patch claims after activation. In production mode,
	// callers receive an immutable snapshot to prevent state tampering.
	if os.Getenv("PULSE_LICENSE_DEV_MODE") == "true" {
		return s.license, nil
	}
	return snapshot, nil
}

// Clear removes the current license.
func (s *Service) Clear() {
	s.mu.Lock()
	s.license = nil
	s.evaluator = nil
	cb := s.onLicenseChange
	s.mu.Unlock()

	if cb != nil {
		cb(nil)
	}
}

// Current returns the current license, or nil if none.
func (s *Service) Current() *License {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneLicense(s.license)
}

// IsValid returns true if a valid, non-expired license is active.
func (s *Service) IsValid() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.license == nil {
		return false
	}

	state := s.currentJWTSubscriptionStateLocked(time.Now())
	return subscriptionStateHasPaidFeatures(state)
}

// HasFeature checks if the current license grants a feature.
func (s *Service) HasFeature(feature string) bool {
	// In demo mode or dev mode, grant all Pro features
	if isDemoMode() || isDevMode() {
		return true
	}

	s.mu.Lock() // Need write lock since we may update grace period
	defer s.mu.Unlock()

	if s.license == nil {
		// Hosted path: evaluator drives entitlements when no JWT is present.
		if s.evaluator != nil {
			// Always include free-tier features, regardless of evaluator state.
			return TierHasFeature(TierFree, feature) || s.evaluator.HasCapability(feature)
		}
		// No license activated — still grant free tier features
		return TierHasFeature(TierFree, feature)
	}

	// JWT takes precedence whenever a license is present (including hybrid mode).
	if !subscriptionStateHasPaidFeatures(s.currentJWTSubscriptionStateLocked(time.Now())) {
		return TierHasFeature(TierFree, feature)
	}

	return s.license.HasFeature(feature)
}

// isDemoMode returns true if the demo/mock mode is enabled
func isDemoMode() bool {
	return strings.EqualFold(os.Getenv("PULSE_MOCK_MODE"), "true")
}

// isDevMode returns true if running in development mode
func isDevMode() bool {
	return strings.EqualFold(os.Getenv("PULSE_DEV"), "true")
}

func isLicenseValidationDevMode() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("PULSE_LICENSE_DEV_MODE")), "true")
}

// LicenseState represents the current state of the license
type LicenseState string

const (
	LicenseStateNone        LicenseState = "none"
	LicenseStateActive      LicenseState = "active"
	LicenseStateExpired     LicenseState = "expired"
	LicenseStateGracePeriod LicenseState = "grace_period"
)

const (
	maxLicenseKeyLength     = 16 << 10 // 16 KiB
	maxLicenseSegmentLength = 8 << 10  // 8 KiB
	maxLicensePayloadSize   = 8 << 10  // 8 KiB decoded JSON
)

// GetLicenseState returns the current license state and the license itself
func (s *Service) GetLicenseState() (LicenseState, *License) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.license == nil {
		if s.evaluator != nil {
			switch s.evaluator.SubscriptionState() {
			case SubStateActive, SubStateTrial:
				return LicenseStateActive, nil
			case SubStateGrace:
				return LicenseStateGracePeriod, nil
			default:
				// Suspended/canceled/expired (or unknown) are treated as non-entitled.
				return LicenseStateExpired, nil
			}
		}
		return LicenseStateNone, nil
	}

	state := s.currentJWTSubscriptionStateLocked(time.Now())
	switch state {
	case SubStateActive, SubStateTrial:
		return LicenseStateActive, cloneLicense(s.license)
	case SubStateGrace:
		return LicenseStateGracePeriod, cloneLicense(s.license)
	default:
		return LicenseStateExpired, cloneLicense(s.license)
	}
}

// GetLicenseStateString returns the current license state as string and whether features are available
// This implements the LicenseChecker interface for the AI service
func (s *Service) GetLicenseStateString() (string, bool) {
	state, _ := s.GetLicenseState()
	hasFeatures := state == LicenseStateActive || state == LicenseStateGracePeriod
	return string(state), hasFeatures
}

// RequireFeature returns an error if the feature is not available.
// This is the primary method for feature gating.
func (s *Service) RequireFeature(feature string) error {
	if !s.HasFeature(feature) {
		return fmt.Errorf("%w: %s requires Pulse Pro", ErrFeatureNotIncluded, GetFeatureDisplayName(feature))
	}
	return nil
}

// SubscriptionState returns the current normalized subscription state.
func (s *Service) SubscriptionState() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stateMachineConfigured && s.license != nil && s.license.Claims.SubState != "" {
		return string(s.license.Claims.SubState)
	}

	if s.license == nil && s.evaluator != nil {
		return string(s.evaluator.SubscriptionState())
	}
	if s.license == nil {
		return string(SubStateExpired)
	}

	return string(s.currentJWTSubscriptionStateLocked(time.Now()))
}

// Status returns a summary of the current license status.
func (s *Service) Status() *LicenseStatus {
	s.mu.Lock() // Need write lock since we may update grace period
	defer s.mu.Unlock()

	status := &LicenseStatus{
		Valid:    false,
		Tier:     TierFree,
		Features: append([]string(nil), TierFeatures[TierFree]...),
	}

	if s.license == nil {
		// Hosted path: evaluator drives status when no JWT is present.
		if s.evaluator != nil {
			status.Tier = TierPro // hosted billing-backed tenants are effectively "pro" for display
			status.Features = unionFeatures(TierFeatures[TierFree], evaluatorFeatures(s.evaluator))

			if maxNodes, ok := s.evaluator.GetLimit("max_nodes"); ok {
				status.MaxNodes = safeIntFromInt64(maxNodes)
			}
			if maxGuests, ok := s.evaluator.GetLimit("max_guests"); ok {
				status.MaxGuests = safeIntFromInt64(maxGuests)
			}

			subState := s.evaluator.SubscriptionState()
			switch subState {
			case SubStateActive, SubStateTrial, SubStateGrace:
				status.Valid = true
				if subState == SubStateGrace {
					status.InGracePeriod = true
				}
			default:
				status.Valid = false
			}
		}
		return status
	}

	status.Email = s.license.Claims.Email
	status.Tier = s.license.Claims.Tier
	status.IsLifetime = s.license.IsLifetime()
	status.DaysRemaining = s.license.DaysRemaining()
	status.Features = s.license.AllFeatures()

	if maxNodes, ok := s.license.Claims.EffectiveLimits()["max_nodes"]; ok {
		status.MaxNodes = safeIntFromInt64(maxNodes)
	}
	if maxGuests, ok := s.license.Claims.EffectiveLimits()["max_guests"]; ok {
		status.MaxGuests = safeIntFromInt64(maxGuests)
	}

	if s.license.ExpiresAt() != nil {
		exp := s.license.ExpiresAt().Format(time.RFC3339)
		status.ExpiresAt = &exp
	}

	switch subState := s.currentJWTSubscriptionStateLocked(time.Now()); subState {
	case SubStateActive, SubStateTrial:
		status.Valid = true
	case SubStateGrace:
		status.Valid = true
		status.InGracePeriod = true
		if s.license.GracePeriodEnd != nil {
			graceEnd := s.license.GracePeriodEnd.Format(time.RFC3339)
			status.GracePeriodEnd = &graceEnd
		}
	default:
		status.Valid = false
		status.Features = append([]string(nil), TierFeatures[TierFree]...)
	}

	return status
}

func (s *Service) currentJWTSubscriptionStateLocked(now time.Time) SubscriptionState {
	if s.license == nil {
		return SubStateExpired
	}

	// Explicit revocation states in JWT claims should always revoke paid access.
	switch s.license.Claims.SubState {
	case SubStateSuspended, SubStateCanceled, SubStateExpired:
		return s.license.Claims.SubState
	}

	// Expiration/grace handling applies regardless of claim substate.
	if s.license.IsExpired() {
		s.ensureGracePeriodEnd()
		if s.license.GracePeriodEnd != nil && now.Before(*s.license.GracePeriodEnd) {
			return SubStateGrace
		}
		return SubStateExpired
	}

	if s.license.Claims.SubState != "" {
		return s.license.Claims.SubState
	}

	return SubStateActive
}

func subscriptionStateHasPaidFeatures(state SubscriptionState) bool {
	switch state {
	case SubStateActive, SubStateTrial, SubStateGrace:
		return true
	default:
		return false
	}
}

func evaluatorFeatures(eval *entitlements.Evaluator) []string {
	if eval == nil {
		return []string{}
	}

	// Derive a stable, known capability list from the evaluator by enumerating
	// all feature keys currently used by tier-based gating.
	known := make(map[string]struct{}, 32)
	for _, features := range TierFeatures {
		for _, feature := range features {
			known[feature] = struct{}{}
		}
	}

	caps := make([]string, 0, len(known))
	for feature := range known {
		if eval.HasCapability(feature) {
			caps = append(caps, feature)
		}
	}
	sort.Strings(caps)
	return caps
}

func unionFeatures(a, b []string) []string {
	set := make(map[string]struct{}, len(a)+len(b))
	for _, v := range a {
		set[v] = struct{}{}
	}
	for _, v := range b {
		set[v] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func safeIntFromInt64(v int64) int {
	// Avoid overflow on 32-bit platforms; clamp to max int.
	maxInt := int64(^uint(0) >> 1)
	if v > maxInt {
		return int(maxInt)
	}
	if v < 0 {
		return 0
	}
	return int(v)
}

func cloneLicense(in *License) *License {
	if in == nil {
		return nil
	}

	out := *in
	out.Claims = cloneClaims(in.Claims)
	if in.GracePeriodEnd != nil {
		graceEnd := *in.GracePeriodEnd
		out.GracePeriodEnd = &graceEnd
	}
	return &out
}

func cloneClaims(in Claims) Claims {
	out := in
	if in.Features != nil {
		out.Features = append([]string(nil), in.Features...)
	}
	if in.Capabilities != nil {
		out.Capabilities = append([]string(nil), in.Capabilities...)
	}
	if in.Limits != nil {
		out.Limits = make(map[string]int64, len(in.Limits))
		for key, value := range in.Limits {
			out.Limits[key] = value
		}
	}
	if in.MetersEnabled != nil {
		out.MetersEnabled = append([]string(nil), in.MetersEnabled...)
	}
	return out
}

// LicenseStatus is the JSON response for license status API.
type LicenseStatus struct {
	Valid          bool     `json:"valid"`
	Tier           Tier     `json:"tier"`
	Email          string   `json:"email,omitempty"`
	ExpiresAt      *string  `json:"expires_at,omitempty"`
	IsLifetime     bool     `json:"is_lifetime"`
	DaysRemaining  int      `json:"days_remaining"`
	Features       []string `json:"features"`
	MaxNodes       int      `json:"max_nodes,omitempty"`
	MaxGuests      int      `json:"max_guests,omitempty"`
	InGracePeriod  bool     `json:"in_grace_period,omitempty"`
	GracePeriodEnd *string  `json:"grace_period_end,omitempty"`
}

// ValidateLicense validates a license key and returns the license if valid.
func ValidateLicense(licenseKey string) (*License, error) {
	// Trim whitespace
	licenseKey = strings.TrimSpace(licenseKey)
	if licenseKey == "" {
		return nil, ErrInvalidLicense
	}
	if len(licenseKey) > maxLicenseKeyLength {
		return nil, fmt.Errorf("%w: license key exceeds size limit", ErrMalformedLicense)
	}

	// Parse JWT (base64url.base64url.base64url)
	parts := strings.Split(licenseKey, ".")
	if len(parts) != 3 {
		return nil, ErrMalformedLicense
	}
	for _, part := range parts {
		if part == "" {
			return nil, fmt.Errorf("%w: empty jwt segment", ErrMalformedLicense)
		}
		if len(part) > maxLicenseSegmentLength {
			return nil, fmt.Errorf("%w: jwt segment exceeds size limit", ErrMalformedLicense)
		}
	}

	// Decode header (not used currently, but validate it exists)
	_, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid header encoding", ErrMalformedLicense)
	}

	// Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid payload encoding", ErrMalformedLicense)
	}
	if len(payloadBytes) > maxLicensePayloadSize {
		return nil, fmt.Errorf("%w: payload exceeds size limit", ErrMalformedLicense)
	}

	// Decode signature
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature encoding", ErrMalformedLicense)
	}

	// Verify signature
	// In production, public key MUST be set. In dev mode, we skip signature validation.
	devMode := isLicenseValidationDevMode()
	signedData := []byte(parts[0] + "." + parts[1])
	key := currentPublicKey()

	if len(key) > 0 {
		if !ed25519.Verify(key, signedData, signature) {
			return nil, ErrSignatureInvalid
		}
	} else if !devMode {
		// No public key and not in dev mode - fail validation
		return nil, fmt.Errorf("%w: signature verification required", ErrNoPublicKey)
	}
	// If devMode and no public key, we skip signature verification (for testing only)

	// Parse claims
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: invalid claims JSON", ErrMalformedLicense)
	}

	// Validate required fields
	if claims.LicenseID == "" {
		return nil, fmt.Errorf("%w: missing license ID", ErrMalformedLicense)
	}
	if claims.Email == "" {
		return nil, fmt.Errorf("%w: missing email", ErrMalformedLicense)
	}
	if claims.Tier == "" {
		return nil, fmt.Errorf("%w: missing tier", ErrMalformedLicense)
	}

	license := &License{
		Raw:         licenseKey,
		Claims:      claims,
		ValidatedAt: time.Now(),
	}

	// Check expiration with grace period support
	if license.IsExpired() {
		// Calculate how long ago it expired
		expirationTime := time.Unix(claims.ExpiresAt, 0)
		gracePeriodEnd := expirationTime.Add(DefaultGracePeriod)

		if time.Now().Before(gracePeriodEnd) {
			// Within grace period - allow activation but mark as in grace period
			license.GracePeriodEnd = &gracePeriodEnd
			// License is still valid during grace period
		} else {
			// Past grace period - reject
			return nil, fmt.Errorf("%w: expired on %s (grace period ended %s)",
				ErrExpiredLicense,
				expirationTime.Format("2006-01-02"),
				gracePeriodEnd.Format("2006-01-02"))
		}
	}

	return license, nil
}

// GenerateLicenseForTesting creates a test license (DO NOT USE IN PRODUCTION).
// This is only for development/testing without a real license server.
func GenerateLicenseForTesting(email string, tier Tier, expiresIn time.Duration) (string, error) {
	claims := Claims{
		LicenseID: fmt.Sprintf("test_%d", time.Now().UnixNano()),
		Email:     email,
		Tier:      tier,
		IssuedAt:  time.Now().Unix(),
	}
	if expiresIn > 0 {
		claims.ExpiresAt = time.Now().Add(expiresIn).Unix()
	}

	// Create unsigned JWT (for testing only)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal test license claims: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	// Fake signature (testing only - real licenses need proper signing)
	signature := base64.RawURLEncoding.EncodeToString([]byte("test-signature-not-valid"))

	return header + "." + payload + "." + signature, nil
}
