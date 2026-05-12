package relay

import (
	"os"
	"strings"

	"github.com/rs/zerolog/log"
)

// Env vars for headless / container deployments that need to bootstrap relay
// without going through Settings → Relay. They override the persisted file
// config; if you set them you accept that UI changes will be re-overridden on
// next start.
const (
	EnvRelayEnabled   = "PULSE_RELAY_ENABLED"
	EnvRelayServerURL = "PULSE_RELAY_SERVER"
)

// ApplyEnvOverrides mutates cfg in place to reflect PULSE_RELAY_* environment
// overrides. Unset / empty / unparseable env vars do not override; the file
// (or default) value remains. Invalid server URLs are logged and ignored —
// the override silently falls back rather than leaving relay wedged on a
// malformed endpoint.
func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if rawEnabled := strings.TrimSpace(os.Getenv(EnvRelayEnabled)); rawEnabled != "" {
		if parsed, ok := parseEnvBool(rawEnabled); ok {
			cfg.Enabled = parsed
			log.Info().
				Str("env_var", EnvRelayEnabled).
				Bool("enabled", parsed).
				Msg("relay configuration overridden by environment variable")
		} else {
			log.Warn().
				Str("env_var", EnvRelayEnabled).
				Str("value", rawEnabled).
				Msg("relay env override is not a recognized boolean; ignoring")
		}
	}

	if rawURL := strings.TrimSpace(os.Getenv(EnvRelayServerURL)); rawURL != "" {
		if err := validateRelayServerURL(rawURL); err != nil {
			log.Warn().
				Str("env_var", EnvRelayServerURL).
				Str("value", rawURL).
				Err(err).
				Msg("relay env override is not a valid ws/wss URL; keeping persisted value")
		} else {
			cfg.ServerURL = rawURL
			log.Info().
				Str("env_var", EnvRelayServerURL).
				Str("server_url", rawURL).
				Msg("relay configuration overridden by environment variable")
		}
	}
}

// parseEnvBool returns (value, ok). ok=false means the input was not a
// recognizable boolean and the caller should leave the config field alone.
// Distinct from utils.ParseBool which silently coerces everything unknown to
// false; for env overrides we need to tell "unset" from "explicit false".
func parseEnvBool(rawValue string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	}
	return false, false
}
