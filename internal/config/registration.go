package config

import (
	"os"
	"strconv"
	"time"
)

// RegistrationConfig holds registration token configuration
type RegistrationConfig struct {
	RequireToken     bool
	DefaultValidity  time.Duration
	DefaultMaxUses   int
	AllowUnprotected bool
}

// GetRegistrationConfig returns the registration configuration from environment
func GetRegistrationConfig() RegistrationConfig {
	config := RegistrationConfig{
		RequireToken:     false,
		DefaultValidity:  15 * time.Minute,
		DefaultMaxUses:   1,
		AllowUnprotected: true,
	}

	// Check if registration tokens are required
	if os.Getenv("REQUIRE_REGISTRATION_TOKEN") == "true" {
		config.RequireToken = true
		config.AllowUnprotected = false
	}

	// Allow explicitly disabling protection for homelab use
	if os.Getenv("ALLOW_UNPROTECTED_AUTO_REGISTER") == "true" {
		config.AllowUnprotected = true
	}

	// Set default validity from environment
	if validityStr := os.Getenv("REGISTRATION_TOKEN_DEFAULT_VALIDITY"); validityStr != "" {
		if validitySec, err := strconv.Atoi(validityStr); err == nil {
			config.DefaultValidity = time.Duration(validitySec) * time.Second
		}
	}

	// Set default max uses from environment
	if maxUsesStr := os.Getenv("REGISTRATION_TOKEN_DEFAULT_MAX_USES"); maxUsesStr != "" {
		if maxUses, err := strconv.Atoi(maxUsesStr); err == nil {
			config.DefaultMaxUses = maxUses
		}
	}

	return config
}
