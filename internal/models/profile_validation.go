package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ConfigKeyDefinition defines a valid configuration key with its type and constraints.
type ConfigKeyDefinition struct {
	Key         string      // Config key name
	Type        ConfigType  // Expected value type
	Description string      // Human-readable description
	Default     interface{} // Default value (nil if required)
	Required    bool        // Whether the key is required
	Min         *float64    // Minimum value for numbers
	Max         *float64    // Maximum value for numbers
	Pattern     string      // Regex pattern for strings
	Enum        []string    // Allowed values for enums
}

// ConfigType represents the type of a configuration value.
type ConfigType string

const (
	ConfigTypeString   ConfigType = "string"
	ConfigTypeBool     ConfigType = "bool"
	ConfigTypeInt      ConfigType = "int"
	ConfigTypeFloat    ConfigType = "float"
	ConfigTypeDuration ConfigType = "duration"
	ConfigTypeEnum     ConfigType = "enum"
)

// ValidConfigKeys defines agent configuration keys that are actually applied by the agent.
// These match the keys handled in applyRemoteSettings() in cmd/pulse-agent/main.go.
var ValidConfigKeys = []ConfigKeyDefinition{
	{
		Key:         "interval",
		Type:        ConfigTypeDuration,
		Description: "Polling interval for metrics collection",
		Default:     "30s",
	},
	{
		Key:         "enable_host",
		Type:        ConfigTypeBool,
		Description: "Enable host monitoring (metrics + command execution)",
		Default:     true,
	},
	{
		Key:         "enable_docker",
		Type:        ConfigTypeBool,
		Description: "Enable Docker container monitoring",
		Default:     true,
	},
	{
		Key:         "enable_kubernetes",
		Type:        ConfigTypeBool,
		Description: "Enable Kubernetes workload monitoring",
		Default:     false,
	},
	{
		Key:         "enable_proxmox",
		Type:        ConfigTypeBool,
		Description: "Enable Proxmox mode for node registration",
		Default:     false,
	},
	{
		Key:         "proxmox_type",
		Type:        ConfigTypeEnum,
		Description: "Proxmox type override (pve or pbs; auto-detect if unset)",
		Default:     "auto",
		Enum:        []string{"pve", "pbs", "auto"},
	},
	{
		Key:         "docker_runtime",
		Type:        ConfigTypeEnum,
		Description: "Container runtime preference (auto, docker, podman)",
		Default:     "auto",
		Enum:        []string{"auto", "docker", "podman"},
	},
	{
		Key:         "disable_auto_update",
		Type:        ConfigTypeBool,
		Description: "Disable automatic agent updates",
		Default:     false,
	},
	{
		Key:         "disable_docker_update_checks",
		Type:        ConfigTypeBool,
		Description: "Disable Docker image update detection",
		Default:     false,
	},
	{
		Key:         "kube_include_all_pods",
		Type:        ConfigTypeBool,
		Description: "Include all non-succeeded pods in Kubernetes reports",
		Default:     false,
	},
	{
		Key:         "kube_include_all_deployments",
		Type:        ConfigTypeBool,
		Description: "Include all deployments in Kubernetes reports",
		Default:     false,
	},
	{
		Key:         "log_level",
		Type:        ConfigTypeEnum,
		Description: "Agent log verbosity level",
		Default:     "info",
		Enum:        []string{"debug", "info", "warn", "error"},
	},
	{
		Key:         "report_ip",
		Type:        ConfigTypeString,
		Description: "Override the reported IP address for the agent",
		Default:     "",
	},
	{
		Key:         "disable_ceph",
		Type:        ConfigTypeBool,
		Description: "Disable local Ceph status polling",
		Default:     false,
	},
}

// ValidationError represents a validation error for a config key.
type ValidationError struct {
	Key     string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Key, e.Message)
}

// ValidationResult holds the result of config validation.
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationError
}

// ProfileValidator validates agent profile configurations.
type ProfileValidator struct {
	keyDefs map[string]ConfigKeyDefinition
}

// NewProfileValidator creates a new profile validator.
func NewProfileValidator() *ProfileValidator {
	keyDefs := make(map[string]ConfigKeyDefinition)
	for _, def := range ValidConfigKeys {
		keyDefs[def.Key] = def
	}
	return &ProfileValidator{keyDefs: keyDefs}
}

// Validate validates an agent profile configuration.
func (v *ProfileValidator) Validate(config AgentConfigMap) ValidationResult {
	result := ValidationResult{Valid: true}

	// Check for unknown keys
	for key := range config {
		if _, ok := v.keyDefs[key]; !ok {
			result.Warnings = append(result.Warnings, ValidationError{
				Key:     key,
				Message: "Unknown configuration key (will be ignored by agent)",
			})
		}
	}

	// Validate known keys
	for key, value := range config {
		def, ok := v.keyDefs[key]
		if !ok {
			continue // Skip unknown keys (already warned)
		}

		if err := v.validateValue(def, value); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Key:     key,
				Message: err.Error(),
			})
		}
	}

	// Check for required keys
	for _, def := range v.keyDefs {
		if def.Required {
			if _, ok := config[def.Key]; !ok {
				result.Valid = false
				result.Errors = append(result.Errors, ValidationError{
					Key:     def.Key,
					Message: "Required configuration key is missing",
				})
			}
		}
	}

	return result
}

// validateValue validates a single configuration value against its definition.
func (v *ProfileValidator) validateValue(def ConfigKeyDefinition, value interface{}) error {
	if value == nil {
		if def.Required {
			return fmt.Errorf("value cannot be null")
		}
		return nil
	}

	switch def.Type {
	case ConfigTypeString:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		if def.Pattern != "" {
			re, err := regexp.Compile(def.Pattern)
			if err != nil {
				return fmt.Errorf("invalid pattern in definition: %w", err)
			}
			if !re.MatchString(s) {
				return fmt.Errorf("value does not match pattern %s", def.Pattern)
			}
		}

	case ConfigTypeBool:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", value)
		}

	case ConfigTypeInt:
		var num float64
		switch n := value.(type) {
		case int:
			num = float64(n)
		case int64:
			num = float64(n)
		case float64:
			if n != float64(int64(n)) {
				return fmt.Errorf("expected integer, got float")
			}
			num = n
		default:
			return fmt.Errorf("expected integer, got %T", value)
		}
		if def.Min != nil && num < *def.Min {
			return fmt.Errorf("value %v is below minimum %v", num, *def.Min)
		}
		if def.Max != nil && num > *def.Max {
			return fmt.Errorf("value %v exceeds maximum %v", num, *def.Max)
		}

	case ConfigTypeFloat:
		var num float64
		switch n := value.(type) {
		case int:
			num = float64(n)
		case int64:
			num = float64(n)
		case float64:
			num = n
		default:
			return fmt.Errorf("expected number, got %T", value)
		}
		if def.Min != nil && num < *def.Min {
			return fmt.Errorf("value %v is below minimum %v", num, *def.Min)
		}
		if def.Max != nil && num > *def.Max {
			return fmt.Errorf("value %v exceeds maximum %v", num, *def.Max)
		}

	case ConfigTypeDuration:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected duration string, got %T", value)
		}
		if _, err := time.ParseDuration(s); err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}

	case ConfigTypeEnum:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		found := false
		for _, allowed := range def.Enum {
			if strings.EqualFold(s, allowed) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value must be one of: %s", strings.Join(def.Enum, ", "))
		}
	}

	return nil
}

// GetConfigKeyDefinitions returns all valid configuration key definitions.
func GetConfigKeyDefinitions() []ConfigKeyDefinition {
	return ValidConfigKeys
}

// GetConfigKeyDefinition returns the definition for a specific key.
func GetConfigKeyDefinition(key string) (ConfigKeyDefinition, bool) {
	for _, def := range ValidConfigKeys {
		if def.Key == key {
			return def, true
		}
	}
	return ConfigKeyDefinition{}, false
}
