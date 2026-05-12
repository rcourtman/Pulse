package relay

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

// Env vars for headless / container deployments that bootstrap relay without
// going through Settings → Relay. Saving from the UI after an override is
// active persists the env-effective state to disk, so clearing the env alone
// does not revert.
const (
	EnvRelayEnabled   = "PULSE_RELAY_ENABLED"
	EnvRelayServerURL = "PULSE_RELAY_SERVER"
)

// ApplyEnvOverrides mutates cfg in place to reflect PULSE_RELAY_* environment
// overrides. Unset / empty / unparseable env vars leave the file value
// untouched; invalid server URLs are logged and ignored.
func ApplyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	if rawEnabled := utils.GetenvTrim(EnvRelayEnabled); rawEnabled != "" {
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

	if rawURL := utils.GetenvTrim(EnvRelayServerURL); rawURL != "" {
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

// parseEnvBool distinguishes "unset" from "explicit false"; utils.ParseBool
// can't (it coerces everything unrecognized to false).
func parseEnvBool(rawValue string) (value bool, ok bool) {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	}
	return false, false
}
