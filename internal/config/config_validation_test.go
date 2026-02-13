package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func getValidConfig() *Config {
	return &Config{
		FrontendPort:                7655,
		PVEPollingInterval:          30 * time.Second,
		ConnectionTimeout:           10 * time.Second,
		AdaptivePollingMinInterval:  10 * time.Second,
		AdaptivePollingBaseInterval: 30 * time.Second,
		AdaptivePollingMaxInterval:  5 * time.Minute,
		OIDC:                        &OIDCConfig{Enabled: false},
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		isValid bool
		errMsg  string
	}{
		{
			name:    "Valid Config",
			mutate:  func(c *Config) {},
			isValid: true,
		},
		{
			name:    "Invalid Frontend Port Low",
			mutate:  func(c *Config) { c.FrontendPort = 0 },
			isValid: false,
			errMsg:  "invalid frontend port",
		},
		{
			name:    "Invalid SSH Port Low",
			mutate:  func(c *Config) { c.SSHPort = -1 },
			isValid: false,
			errMsg:  "invalid SSH port",
		},
		{
			name:    "Invalid SSH Port High",
			mutate:  func(c *Config) { c.SSHPort = 70000 },
			isValid: false,
			errMsg:  "invalid SSH port",
		},
		{
			name:    "Invalid PVE Polling Interval Low",
			mutate:  func(c *Config) { c.PVEPollingInterval = 1 * time.Second },
			isValid: false,
			errMsg:  "PVE polling interval must be at least 10 seconds",
		},
		{
			name:    "Invalid PVE Polling Interval High",
			mutate:  func(c *Config) { c.PVEPollingInterval = 2 * time.Hour },
			isValid: false,
			errMsg:  "PVE polling interval cannot exceed 1 hour",
		},
		{
			name:    "Invalid Connection Timeout",
			mutate:  func(c *Config) { c.ConnectionTimeout = 100 * time.Millisecond },
			isValid: false,
			errMsg:  "connection timeout must be at least 1 second",
		},
		{
			name:    "Invalid Adaptive Min <= 0",
			mutate:  func(c *Config) { c.AdaptivePollingMinInterval = 0 },
			isValid: false,
			errMsg:  "adaptive polling min interval must be greater than 0",
		},
		{
			name:    "Invalid Adaptive Base <= 0",
			mutate:  func(c *Config) { c.AdaptivePollingBaseInterval = 0 },
			isValid: false,
			errMsg:  "adaptive polling base interval must be greater than 0",
		},
		{
			name:    "Invalid Adaptive Max <= 0",
			mutate:  func(c *Config) { c.AdaptivePollingMaxInterval = 0 },
			isValid: false,
			errMsg:  "adaptive polling max interval must be greater than 0",
		},
		{
			name: "Invalid Adaptive Min > Max",
			mutate: func(c *Config) {
				c.AdaptivePollingMinInterval = 10 * time.Minute
				c.AdaptivePollingMaxInterval = 5 * time.Minute
			},
			isValid: false,
			errMsg:  "adaptive polling min interval cannot exceed max interval",
		},
		{
			name: "Invalid Adaptive Base Out of Range",
			mutate: func(c *Config) {
				c.AdaptivePollingBaseInterval = 1 * time.Second
				c.AdaptivePollingMinInterval = 10 * time.Second
			},
			isValid: false,
			errMsg:  "adaptive polling base interval must be between min and max intervals",
		},
		{
			name: "Invalid PVE Instance Host Empty",
			mutate: func(c *Config) {
				c.PVEInstances = []PVEInstance{{Host: ""}}
			},
			isValid: false,
			errMsg:  "host is required",
		},
		{
			name: "Invalid PVE Instance Schema",
			mutate: func(c *Config) {
				c.PVEInstances = []PVEInstance{{Host: "ftp://host"}}
			},
			isValid: false,
			errMsg:  "host must start with http:// or https://",
		},
		{
			name: "Invalid PVE Instance No Auth",
			mutate: func(c *Config) {
				c.PVEInstances = []PVEInstance{{Host: "https://host"}}
			},
			isValid: false,
			errMsg:  "either password or token authentication is required",
		},
		{
			name: "Valid PVE Instance",
			mutate: func(c *Config) {
				c.PVEInstances = []PVEInstance{{Host: "https://host", Password: "pass"}}
			},
			isValid: true,
		},
		{
			name: "Invalid OIDC",
			mutate: func(c *Config) {
				c.OIDC = &OIDCConfig{Enabled: true, IssuerURL: ""}
			},
			isValid: false,
			errMsg:  "issuer url is required", // OIDC.Validate error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := getValidConfig()
			tt.mutate(cfg)
			err := cfg.Validate()
			if tt.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestConfig_Validate_PBSAutoFix(t *testing.T) {
	cfg := getValidConfig()
	// PBS with missing schema
	cfg.PBSInstances = []PBSInstance{
		{Host: "pbs.local", Password: "pass"},
	}

	err := cfg.Validate()
	assert.NoError(t, err)

	// Verified it was autofixed
	assert.Equal(t, "https://pbs.local", cfg.PBSInstances[0].Host)
}

func TestConfig_Validate_PBS_SkipInvalid(t *testing.T) {
	cfg := getValidConfig()
	cfg.PBSInstances = []PBSInstance{
		{Host: ""}, // Should be skipped
		{Host: "valid", Password: "pass"},
		{Host: "noauth"}, // Should be skipped
	}

	err := cfg.Validate()
	assert.NoError(t, err)

	// Should only have the valid one left
	assert.Len(t, cfg.PBSInstances, 1)
	assert.Equal(t, "https://valid", cfg.PBSInstances[0].Host)
}
