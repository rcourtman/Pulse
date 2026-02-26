package licensing

import "time"

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
	JTI            string   `json:"jti"` // unique grant ID
}

// grantClaimsToClaims maps grant claims to the existing Claims struct
// so that all feature gating (HasFeature, RequireFeature, AgentLimit) works unchanged.
func grantClaimsToClaims(gc *GrantClaims) Claims {
	c := Claims{
		LicenseID: gc.LicenseID,
		Tier:      Tier(gc.Tier),
		IssuedAt:  gc.IssuedAt,
		ExpiresAt: gc.ExpiresAt,
		Features:  gc.Features,
		MaxAgents: gc.MaxAgents,
		MaxGuests: gc.MaxGuests,
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

// ActivateInstallationRequest is the payload sent to POST /v1/installations.
type ActivateInstallationRequest struct {
	ActivationKey       string `json:"activation_key"`
	InstanceFingerprint string `json:"instance_fingerprint"`
	Hostname            string `json:"hostname,omitempty"`
	Version             string `json:"version,omitempty"`
}

// ActivateInstallationResponse is the payload returned from the activation endpoint.
type ActivateInstallationResponse struct {
	InstallationID    string        `json:"installation_id"`
	InstallationToken string        `json:"installation_token"`
	LicenseID         string        `json:"license_id"`
	Grant             GrantEnvelope `json:"grant"`
}

// GrantEnvelope wraps a relay grant JWT with metadata.
type GrantEnvelope struct {
	JWT       string       `json:"jwt"`
	JTI       string       `json:"jti"`
	ExpiresAt int64        `json:"expires_at"`
	Refresh   RefreshHints `json:"refresh_policy"`
}

// RefreshHints contains the server's recommended grant refresh schedule.
type RefreshHints struct {
	IntervalSeconds int     `json:"interval_seconds"` // e.g. 21600 (6h)
	JitterPercent   float64 `json:"jitter_percent"`   // e.g. 0.2 (20%)
}

// RefreshGrantRequest is the payload sent to POST /v1/installations/{id}/grant/refresh.
type RefreshGrantRequest struct {
	InstanceFingerprint string `json:"instance_fingerprint"`
	CurrentJTI          string `json:"current_jti,omitempty"`
	Version             string `json:"version,omitempty"`
}

// RefreshGrantResponse is the payload returned from the grant refresh endpoint.
type RefreshGrantResponse struct {
	Grant GrantEnvelope `json:"grant"`
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
