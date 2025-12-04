// Package ai provides AI-powered infrastructure investigation and remediation.
package ai

import "github.com/rcourtman/pulse-go-rewrite/internal/config"

// Re-export config types for convenience
type Config = config.AIConfig

// Provider constants (re-exported for convenience)
const (
	ProviderAnthropic = config.AIProviderAnthropic
	ProviderOpenAI    = config.AIProviderOpenAI
	ProviderOllama    = config.AIProviderOllama
)

// NewDefaultConfig returns a new AI config with sensible defaults
func NewDefaultConfig() *Config {
	return config.NewDefaultAIConfig()
}
