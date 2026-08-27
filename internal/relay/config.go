package relay

import "strings"

const (
	AlertMinimumSeverityAll      = "all"
	AlertMinimumSeverityCritical = "critical"
)

// Config holds relay client configuration.
type Config struct {
	Enabled              bool   `json:"enabled"`
	ServerURL            string `json:"server_url"`
	InstanceSecret       string `json:"instance_secret"`
	AlertMinimumSeverity string `json:"alert_minimum_severity,omitempty"`

	// Instance identity keypair for MITM prevention.
	IdentityPrivateKey  string `json:"identity_private_key,omitempty"`
	IdentityPublicKey   string `json:"identity_public_key,omitempty"`
	IdentityFingerprint string `json:"identity_fingerprint,omitempty"`
}

// DefaultServerURL is the production relay server endpoint.
const DefaultServerURL = "wss://relay.pulserelay.pro/ws/instance"

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:              false,
		ServerURL:            DefaultServerURL,
		AlertMinimumSeverity: AlertMinimumSeverityAll,
	}
}

// NormalizeAlertMinimumSeverity preserves global alert push delivery unless
// the operator explicitly chooses critical-only mobile attention.
func NormalizeAlertMinimumSeverity(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), AlertMinimumSeverityCritical) {
		return AlertMinimumSeverityCritical
	}
	return AlertMinimumSeverityAll
}

// AlertMeetsMinimumSeverity reports whether one alert level is eligible for
// the configured mobile push floor. Unknown levels fail closed only when the
// operator selected critical-only delivery.
func AlertMeetsMinimumSeverity(level, minimumSeverity string) bool {
	if NormalizeAlertMinimumSeverity(minimumSeverity) == AlertMinimumSeverityAll {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(level), AlertMinimumSeverityCritical)
}
