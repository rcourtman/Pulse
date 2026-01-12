package models

import (
	"testing"
)

func TestProfileValidator_ValidateStringType(t *testing.T) {
	validator := NewProfileValidator()

	tests := []struct {
		name    string
		config  AgentConfigMap
		wantErr bool
		errKey  string
	}{
		{
			name:    "valid string",
			config:  AgentConfigMap{"disk_paths": "/,/home"},
			wantErr: false,
		},
		{
			name:    "invalid string type",
			config:  AgentConfigMap{"disk_paths": 123},
			wantErr: true,
			errKey:  "disk_paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)
			if tt.wantErr && result.Valid {
				t.Errorf("expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
			if tt.wantErr && len(result.Errors) > 0 {
				found := false
				for _, err := range result.Errors {
					if err.Key == tt.errKey {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error for key %s, but got: %v", tt.errKey, result.Errors)
				}
			}
		})
	}
}

func TestProfileValidator_ValidateBoolType(t *testing.T) {
	validator := NewProfileValidator()

	tests := []struct {
		name    string
		config  AgentConfigMap
		wantErr bool
		errKey  string
	}{
		{
			name:    "valid bool true",
			config:  AgentConfigMap{"enable_docker": true},
			wantErr: false,
		},
		{
			name:    "valid bool false",
			config:  AgentConfigMap{"enable_docker": false},
			wantErr: false,
		},
		{
			name:    "invalid bool type - string",
			config:  AgentConfigMap{"enable_docker": "true"},
			wantErr: true,
			errKey:  "enable_docker",
		},
		{
			name:    "invalid bool type - int",
			config:  AgentConfigMap{"enable_docker": 1},
			wantErr: true,
			errKey:  "enable_docker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)
			if tt.wantErr && result.Valid {
				t.Errorf("expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProfileValidator_ValidateIntType(t *testing.T) {
	validator := NewProfileValidator()

	tests := []struct {
		name    string
		config  AgentConfigMap
		wantErr bool
		errKey  string
	}{
		{
			name:    "valid int",
			config:  AgentConfigMap{"metric_buffer_size": 100},
			wantErr: false,
		},
		{
			name:    "valid int as float64 (JSON unmarshal)",
			config:  AgentConfigMap{"metric_buffer_size": float64(100)},
			wantErr: false,
		},
		{
			name:    "int below minimum",
			config:  AgentConfigMap{"metric_buffer_size": 5},
			wantErr: true,
			errKey:  "metric_buffer_size",
		},
		{
			name:    "int above maximum",
			config:  AgentConfigMap{"metric_buffer_size": 20000},
			wantErr: true,
			errKey:  "metric_buffer_size",
		},
		{
			name:    "invalid int type - string",
			config:  AgentConfigMap{"metric_buffer_size": "100"},
			wantErr: true,
			errKey:  "metric_buffer_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)
			if tt.wantErr && result.Valid {
				t.Errorf("expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProfileValidator_ValidateFloatType(t *testing.T) {
	validator := NewProfileValidator()

	tests := []struct {
		name    string
		config  AgentConfigMap
		wantErr bool
		errKey  string
	}{
		{
			name:    "valid float",
			config:  AgentConfigMap{"cpu_threshold_warning": 80.5},
			wantErr: false,
		},
		{
			name:    "valid float as int",
			config:  AgentConfigMap{"cpu_threshold_warning": 80},
			wantErr: false,
		},
		{
			name:    "float below minimum",
			config:  AgentConfigMap{"cpu_threshold_warning": -10.0},
			wantErr: true,
			errKey:  "cpu_threshold_warning",
		},
		{
			name:    "float above maximum",
			config:  AgentConfigMap{"cpu_threshold_warning": 150.0},
			wantErr: true,
			errKey:  "cpu_threshold_warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)
			if tt.wantErr && result.Valid {
				t.Errorf("expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProfileValidator_ValidateDurationType(t *testing.T) {
	validator := NewProfileValidator()

	tests := []struct {
		name    string
		config  AgentConfigMap
		wantErr bool
		errKey  string
	}{
		{
			name:    "valid duration seconds",
			config:  AgentConfigMap{"interval": "30s"},
			wantErr: false,
		},
		{
			name:    "valid duration minutes",
			config:  AgentConfigMap{"interval": "5m"},
			wantErr: false,
		},
		{
			name:    "valid duration hours",
			config:  AgentConfigMap{"interval": "1h"},
			wantErr: false,
		},
		{
			name:    "valid duration complex",
			config:  AgentConfigMap{"interval": "1h30m45s"},
			wantErr: false,
		},
		{
			name:    "invalid duration format",
			config:  AgentConfigMap{"interval": "30"},
			wantErr: true,
			errKey:  "interval",
		},
		{
			name:    "invalid duration - not a string",
			config:  AgentConfigMap{"interval": 30},
			wantErr: true,
			errKey:  "interval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)
			if tt.wantErr && result.Valid {
				t.Errorf("expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProfileValidator_ValidateEnumType(t *testing.T) {
	validator := NewProfileValidator()

	tests := []struct {
		name    string
		config  AgentConfigMap
		wantErr bool
		errKey  string
	}{
		{
			name:    "valid enum debug",
			config:  AgentConfigMap{"log_level": "debug"},
			wantErr: false,
		},
		{
			name:    "valid enum info",
			config:  AgentConfigMap{"log_level": "info"},
			wantErr: false,
		},
		{
			name:    "valid enum warn",
			config:  AgentConfigMap{"log_level": "warn"},
			wantErr: false,
		},
		{
			name:    "valid enum error",
			config:  AgentConfigMap{"log_level": "error"},
			wantErr: false,
		},
		{
			name:    "invalid enum value",
			config:  AgentConfigMap{"log_level": "trace"},
			wantErr: true,
			errKey:  "log_level",
		},
		{
			name:    "invalid enum type",
			config:  AgentConfigMap{"log_level": 1},
			wantErr: true,
			errKey:  "log_level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.config)
			if tt.wantErr && result.Valid {
				t.Errorf("expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("expected validation to pass, but it failed: %v", result.Errors)
			}
		})
	}
}

func TestProfileValidator_UnknownKeysWarning(t *testing.T) {
	validator := NewProfileValidator()

	config := AgentConfigMap{
		"interval":    "30s",
		"unknown_key": "some_value",
	}

	result := validator.Validate(config)

	// Should still be valid (warnings don't fail validation)
	if !result.Valid {
		t.Errorf("expected validation to pass with warnings, but it failed: %v", result.Errors)
	}

	// Should have a warning for unknown key
	if len(result.Warnings) == 0 {
		t.Errorf("expected a warning for unknown key, but got none")
	}

	found := false
	for _, w := range result.Warnings {
		if w.Key == "unknown_key" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected warning for 'unknown_key', got: %v", result.Warnings)
	}
}

func TestProfileValidator_ComplexConfig(t *testing.T) {
	validator := NewProfileValidator()

	config := AgentConfigMap{
		"interval":               "30s",
		"enable_docker":          true,
		"enable_system_metrics":  true,
		"enable_process_metrics": false,
		"log_level":              "info",
		"metric_buffer_size":     100,
		"cpu_threshold_warning":  80.0,
		"cpu_threshold_critical": 95.0,
		"disk_paths":             "/,/home",
	}

	result := validator.Validate(config)

	if !result.Valid {
		t.Errorf("expected complex config to be valid, but got errors: %v", result.Errors)
	}
}

func TestProfileValidator_NullValue(t *testing.T) {
	validator := NewProfileValidator()

	config := AgentConfigMap{
		"interval": nil,
	}

	result := validator.Validate(config)

	// Null values should be valid for non-required keys
	if !result.Valid {
		t.Errorf("expected null value to be valid for non-required key, but got errors: %v", result.Errors)
	}
}

func TestGetConfigKeyDefinitions(t *testing.T) {
	defs := GetConfigKeyDefinitions()

	if len(defs) == 0 {
		t.Error("expected config key definitions to be non-empty")
	}

	// Check some known keys exist
	expectedKeys := []string{"interval", "enable_docker", "log_level", "metric_buffer_size"}
	for _, key := range expectedKeys {
		found := false
		for _, def := range defs {
			if def.Key == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected key %s to be in definitions", key)
		}
	}
}

func TestGetConfigKeyDefinition(t *testing.T) {
	def, found := GetConfigKeyDefinition("interval")
	if !found {
		t.Error("expected to find 'interval' definition")
	}
	if def.Type != ConfigTypeDuration {
		t.Errorf("expected 'interval' type to be Duration, got %s", def.Type)
	}

	_, found = GetConfigKeyDefinition("nonexistent_key")
	if found {
		t.Error("expected not to find 'nonexistent_key' definition")
	}
}

func TestAgentProfile_MergedConfig(t *testing.T) {
	parentProfile := AgentProfile{
		ID:   "parent-1",
		Name: "Parent Profile",
		Config: AgentConfigMap{
			"interval":      "30s",
			"enable_docker": true,
			"log_level":     "info",
		},
	}

	childProfile := AgentProfile{
		ID:       "child-1",
		Name:     "Child Profile",
		ParentID: "parent-1",
		Config: AgentConfigMap{
			"interval":  "10s",   // Override parent
			"log_level": "debug", // Override parent
		},
	}

	profiles := []AgentProfile{parentProfile, childProfile}

	merged := childProfile.MergedConfig(profiles)

	// Child's interval should override parent's
	if merged["interval"] != "10s" {
		t.Errorf("expected interval to be '10s', got %v", merged["interval"])
	}

	// Child's log_level should override parent's
	if merged["log_level"] != "debug" {
		t.Errorf("expected log_level to be 'debug', got %v", merged["log_level"])
	}

	// Parent's enable_docker should be inherited
	if merged["enable_docker"] != true {
		t.Errorf("expected enable_docker to be true, got %v", merged["enable_docker"])
	}
}

func TestAgentProfile_MergedConfigNoParent(t *testing.T) {
	profile := AgentProfile{
		ID:   "profile-1",
		Name: "Standalone Profile",
		Config: AgentConfigMap{
			"interval": "30s",
		},
	}

	profiles := []AgentProfile{profile}

	merged := profile.MergedConfig(profiles)

	if merged["interval"] != "30s" {
		t.Errorf("expected interval to be '30s', got %v", merged["interval"])
	}
}

func TestAgentProfile_MergedConfigParentNotFound(t *testing.T) {
	profile := AgentProfile{
		ID:       "child-1",
		Name:     "Child Profile",
		ParentID: "nonexistent-parent",
		Config: AgentConfigMap{
			"interval": "10s",
		},
	}

	profiles := []AgentProfile{profile}

	// Should return just the child's config if parent not found
	merged := profile.MergedConfig(profiles)

	if merged["interval"] != "10s" {
		t.Errorf("expected interval to be '10s', got %v", merged["interval"])
	}
}
