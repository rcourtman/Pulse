package licensing

import (
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
)

// ActivationKeyPrefix is the prefix for activation keys issued by the license server.
// Used to auto-detect activation keys vs legacy JWTs in the Activate endpoint.
const ActivationKeyPrefix = "ppk_live_"

// DefaultLicenseServerURL is the production license server URL.
const DefaultLicenseServerURL = "https://license.pulserelay.pro"

// ActivationState holds the local state for an activation-key-based license.
// Persisted encrypted on disk via Persistence.SaveActivationState.
type ActivationState struct {
	InstallationID      string `json:"installation_id"`      // inst_...
	InstallationToken   string `json:"installation_token"`   // pit_live_... (secret)
	LicenseID           string `json:"license_id"`           // lic_...
	GrantJWT            string `json:"grant_jwt"`            // current relay grant JWT
	GrantJTI            string `json:"grant_jti"`            // grant ID for dedup
	GrantExpiresAt      int64  `json:"grant_expires_at"`     // Unix timestamp
	InstanceFingerprint string `json:"instance_fingerprint"` // stable UUID for this installation
	LicenseServerURL    string `json:"license_server_url"`   // base URL used for this activation
	ActivatedAt         int64  `json:"activated_at"`         // Unix timestamp of initial activation
	LastRefreshedAt     int64  `json:"last_refreshed_at"`    // Unix timestamp of last grant refresh
}

// GrantClaims are the claims parsed from a relay grant JWT payload.
// The grant is a short-lived JWT (72h TTL) issued by the license server.
type GrantClaims struct {
	Issuer         string   `json:"iss"`
	Audience       string   `json:"aud"`
	LicenseID      string   `json:"lid"`
	InstallationID string   `json:"iid"`
	LicenseVersion int64    `json:"lv"`
	State          string   `json:"st"`   // active|past_due|grace
	Tier           string   `json:"tier"` // matches Tier constants: "relay", "pro", "pro_plus", etc.
	PlanKey        string   `json:"plan"`
	Features       []string `json:"feat"`
	MaxAgents      int      `json:"max_agents"`
	MaxGuests      int      `json:"max_guests"`
	IssuedAt       int64    `json:"iat"`
	ExpiresAt      int64    `json:"exp"`
	GraceUntil     int64    `json:"grace_until"`
	JTI            string   `json:"jti"`   // unique grant ID
	Email          string   `json:"email"` // license owner email
}

// grantClaimsToClaims maps grant claims to the existing Claims struct
// so that all feature gating (HasFeature, RequireFeature, AgentLimit) works unchanged.
func grantClaimsToClaims(gc *GrantClaims) Claims {
	c := Claims{
		LicenseID:   gc.LicenseID,
		Email:       gc.Email,
		Tier:        Tier(gc.Tier),
		IssuedAt:    gc.IssuedAt,
		ExpiresAt:   gc.ExpiresAt,
		Features:    gc.Features,
		MaxAgents:   gc.MaxAgents,
		MaxGuests:   gc.MaxGuests,
		PlanVersion: gc.PlanKey,
	}

	// Map grant state to subscription state. Fail closed: unknown states
	// default to suspended so paid features are not granted by accident.
	switch gc.State {
	case "active":
		c.SubState = SubStateActive
	case "past_due":
		c.SubState = SubStateGrace
	case "grace":
		c.SubState = SubStateGrace
	default:
		c.SubState = SubStateSuspended
	}

	return c
}

// grantClaimsToLicense creates a License from parsed grant claims.
func grantClaimsToLicense(gc *GrantClaims, rawJWT string) *License {
	claims := grantClaimsToClaims(gc)

	lic := &License{
		Raw:         rawJWT,
		Claims:      claims,
		ValidatedAt: time.Now(),
	}

	// If the grant includes a grace_until timestamp, set the grace period end.
	if gc.GraceUntil > 0 {
		graceEnd := time.Unix(gc.GraceUntil, 0)
		lic.GracePeriodEnd = &graceEnd
	}

	return lic
}

// ActivateInstallationRequest is the payload sent to POST /v1/activate.
type ActivateInstallationRequest struct {
	ActivationKey       string `json:"activation_key"`
	InstanceName        string `json:"instance_name,omitempty"`
	InstanceFingerprint string `json:"instance_fingerprint"`
	ClientVersion       string `json:"client_version,omitempty"`
}

// ExchangeLegacyLicenseRequest is the payload sent to POST /v1/licenses/exchange.
// It converts a legacy v5 JWT-style license into the v6 activation/grant model.
type ExchangeLegacyLicenseRequest struct {
	LegacyLicenseKey    string `json:"-"`
	InstanceName        string `json:"instance_name,omitempty"`
	InstanceFingerprint string `json:"instance_fingerprint"`
	ClientVersion       string `json:"client_version,omitempty"`
}

// MarshalJSON emits the canonical v6 contract field `legacy_license_token`.
// UnmarshalJSON remains backward-compatible with earlier local stub/test
// paths that still decoded `legacy_license_key`.
func (r ExchangeLegacyLicenseRequest) MarshalJSON() ([]byte, error) {
	type exchangeLegacyLicenseRequestWire struct {
		LegacyLicenseToken  string `json:"legacy_license_token"`
		InstanceName        string `json:"instance_name,omitempty"`
		InstanceFingerprint string `json:"instance_fingerprint"`
		ClientVersion       string `json:"client_version,omitempty"`
	}
	return json.Marshal(exchangeLegacyLicenseRequestWire{
		LegacyLicenseToken:  r.LegacyLicenseKey,
		InstanceName:        r.InstanceName,
		InstanceFingerprint: r.InstanceFingerprint,
		ClientVersion:       r.ClientVersion,
	})
}

func (r *ExchangeLegacyLicenseRequest) UnmarshalJSON(data []byte) error {
	type exchangeLegacyLicenseRequestWire struct {
		LegacyLicenseToken  string `json:"legacy_license_token"`
		LegacyLicenseKey    string `json:"legacy_license_key"`
		InstanceName        string `json:"instance_name,omitempty"`
		InstanceFingerprint string `json:"instance_fingerprint"`
		ClientVersion       string `json:"client_version,omitempty"`
	}
	var wire exchangeLegacyLicenseRequestWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	r.LegacyLicenseKey = wire.LegacyLicenseToken
	if r.LegacyLicenseKey == "" {
		r.LegacyLicenseKey = wire.LegacyLicenseKey
	}
	r.InstanceName = wire.InstanceName
	r.InstanceFingerprint = wire.InstanceFingerprint
	r.ClientVersion = wire.ClientVersion
	return nil
}

// ActivateInstallationResponse is the payload returned from the activation endpoint.
// The response is nested: license, installation, grant, refresh_policy are top-level keys.
type ActivateInstallationResponse struct {
	License       ActivateResponseLicense      `json:"license"`
	Installation  ActivateResponseInstallation `json:"installation"`
	Grant         GrantEnvelope                `json:"grant"`
	RefreshPolicy RefreshHints                 `json:"refresh_policy"`
}

// ActivateResponseLicense is the license portion of the activation response.
type ActivateResponseLicense struct {
	LicenseID      string   `json:"license_id"`
	State          string   `json:"state"`
	Tier           string   `json:"tier"`
	MaxAgents      int      `json:"max_agents"`
	MaxGuests      int      `json:"max_guests"`
	Features       []string `json:"features"`
	LicenseVersion int64    `json:"license_version"`
}

// ActivateResponseInstallation is the installation portion of the activation response.
type ActivateResponseInstallation struct {
	InstallationID    string `json:"installation_id"`
	InstallationToken string `json:"installation_token"`
	Status            string `json:"status"`
}

// GrantEnvelope wraps a relay grant JWT with metadata.
type GrantEnvelope struct {
	JWT       string `json:"jwt"`
	JTI       string `json:"jti"`
	ExpiresAt string `json:"expires_at"` // RFC3339 timestamp from the server
}

// RefreshHints contains the server's recommended grant refresh schedule.
type RefreshHints struct {
	IntervalSeconds int     `json:"recommended_refresh_after_sec"` // e.g. 21600 (6h)
	JitterPercent   float64 `json:"jitter_percent"`                // e.g. 0.2 (20%)
}

// RefreshGrantRequest is the payload sent to POST /v1/grants/refresh.
type RefreshGrantRequest struct {
	InstallationID      string `json:"installation_id"`
	InstanceFingerprint string `json:"instance_fingerprint"`
	CurrentGrantJTI     string `json:"current_grant_jti,omitempty"`
	ClientVersion       string `json:"client_version,omitempty"`
}

// RefreshGrantResponse is the payload returned from the grant refresh endpoint.
type RefreshGrantResponse struct {
	Grant         GrantEnvelope `json:"grant"`
	RefreshPolicy RefreshHints  `json:"refresh_policy"`
}

// RevocationEvent is a single event from the revocation feed.
type RevocationEvent struct {
	Seq               int64  `json:"seq"`
	Action            string `json:"action"` // revoke_license|revoke_installation|bump_license_version
	LicenseID         string `json:"license_id"`
	InstallationID    string `json:"installation_id"`
	MinLicenseVersion int64  `json:"min_license_version"`
	ReasonCode        string `json:"reason_code"`
	Reason            string `json:"reason"`
	EffectiveAt       string `json:"effective_at"` // ISO8601
}

// RevocationFeedResponse is the payload returned from GET /v1/revocations.
type RevocationFeedResponse struct {
	FromSeq   int64             `json:"from_seq"`
	NextSeq   int64             `json:"next_seq"`
	LatestSeq int64             `json:"latest_seq"`
	HasMore   bool              `json:"has_more"`
	Events    []RevocationEvent `json:"events"`
}

// ParseExpiresAt parses the RFC3339 expires_at string from a GrantEnvelope
// into a Unix timestamp. Returns 0 if the string is empty or unparseable.
// The grant JWT's own exp claim is the authoritative expiry; this envelope
// field is advisory for logging/display only.
func (g GrantEnvelope) ParseExpiresAt() int64 {
	if g.ExpiresAt == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, g.ExpiresAt)
	if err != nil {
		log.Warn().Str("expires_at", g.ExpiresAt).Msg("Failed to parse grant envelope expires_at as RFC3339")
		return 0
	}
	return t.Unix()
}

// LicenseServerError is a structured error from the license server.
type LicenseServerError struct {
	StatusCode int    `json:"-"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
}

func (e *LicenseServerError) Error() string {
	return e.Code + ": " + e.Message
}
