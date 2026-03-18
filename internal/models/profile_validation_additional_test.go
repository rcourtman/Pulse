package models

import (
	"strings"
	"testing"
)

func TestValidationErrorError(t *testing.T) {
	err := ValidationError{Key: "cpu_threshold", Message: "invalid"}
	if got := err.Error(); got != "cpu_threshold: invalid" {
		t.Fatalf("Error() = %q, want cpu_threshold: invalid", got)
	}
}

func TestProfileValidatorValidateValue_StringPatternBranches(t *testing.T) {
	validator := NewProfileValidator()

	matchDef := ConfigKeyDefinition{
		Key:     "report_ip",
		Type:    ConfigTypeString,
		Pattern: "^[a-z]+$",
	}
	if err := validator.validateValue(matchDef, "abc"); err != nil {
		t.Fatalf("validateValue() unexpected error for matching pattern: %v", err)
	}

	if err := validator.validateValue(matchDef, "ABC"); err == nil || !strings.Contains(err.Error(), "does not match pattern") {
		t.Fatalf("validateValue() expected pattern mismatch error, got: %v", err)
	}

	invalidPatternDef := ConfigKeyDefinition{
		Key:     "report_ip",
		Type:    ConfigTypeString,
		Pattern: "[",
	}
	if err := validator.validateValue(invalidPatternDef, "abc"); err == nil || !strings.Contains(err.Error(), "invalid pattern in definition") {
		t.Fatalf("validateValue() expected invalid pattern definition error, got: %v", err)
	}
}

func TestProfileValidatorValidateValue_IntTypeBranches(t *testing.T) {
	validator := NewProfileValidator()
	min := 1.0
	max := 10.0
	def := ConfigKeyDefinition{
		Key:  "workers",
		Type: ConfigTypeInt,
		Min:  &min,
		Max:  &max,
	}

	tests := []struct {
		name       string
		value      interface{}
		wantErrSub string
	}{
		{name: "int in range", value: 5},
		{name: "int64 in range", value: int64(6)},
		{name: "integral float64 in range", value: 7.0},
		{name: "float64 not integral", value: 7.5, wantErrSub: "expected integer, got float"},
		{name: "below min", value: 0, wantErrSub: "below minimum"},
		{name: "above max", value: 11, wantErrSub: "exceeds maximum"},
		{name: "wrong type", value: true, wantErrSub: "expected integer, got bool"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.validateValue(def, tc.value)
			if tc.wantErrSub == "" && err != nil {
				t.Fatalf("validateValue() unexpected error: %v", err)
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("validateValue() expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("validateValue() error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
			}
		})
	}
}

func TestProfileValidatorValidateValue_FloatTypeBranches(t *testing.T) {
	validator := NewProfileValidator()
	min := 0.5
	max := 2.5
	def := ConfigKeyDefinition{
		Key:  "threshold",
		Type: ConfigTypeFloat,
		Min:  &min,
		Max:  &max,
	}

	tests := []struct {
		name       string
		value      interface{}
		wantErrSub string
	}{
		{name: "int in range", value: 1},
		{name: "int64 in range", value: int64(2)},
		{name: "float64 in range", value: 1.5},
		{name: "below min", value: 0.25, wantErrSub: "below minimum"},
		{name: "above max", value: 3.0, wantErrSub: "exceeds maximum"},
		{name: "wrong type", value: "1.2", wantErrSub: "expected number, got string"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validator.validateValue(def, tc.value)
			if tc.wantErrSub == "" && err != nil {
				t.Fatalf("validateValue() unexpected error: %v", err)
			}
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("validateValue() expected error containing %q, got nil", tc.wantErrSub)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Fatalf("validateValue() error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
			}
		})
	}
}

func TestProfileValidatorValidateValue_EnumCaseInsensitive(t *testing.T) {
	validator := NewProfileValidator()
	def := ConfigKeyDefinition{
		Key:  "docker_runtime",
		Type: ConfigTypeEnum,
		Enum: []string{"auto", "docker"},
	}

	if err := validator.validateValue(def, "DOCKER"); err != nil {
		t.Fatalf("validateValue() expected case-insensitive enum match, got: %v", err)
	}

	if err := validator.validateValue(def, "podman"); err == nil || !strings.Contains(err.Error(), "value must be one of") {
		t.Fatalf("validateValue() expected enum error for unsupported value, got: %v", err)
	}
}

func TestProfileValidatorValidate_RequiredKeyBranches(t *testing.T) {
	validator := NewProfileValidator()
	validator.keyDefs = map[string]ConfigKeyDefinition{
		"required_key": {
			Key:      "required_key",
			Type:     ConfigTypeString,
			Required: true,
		},
		"optional_key": {
			Key:      "optional_key",
			Type:     ConfigTypeString,
			Required: false,
		},
	}

	missing := validator.Validate(AgentConfigMap{})
	if missing.Valid {
		t.Fatalf("Validate() expected missing required key to fail")
	}
	if len(missing.Errors) != 1 || missing.Errors[0].Key != "required_key" || !strings.Contains(missing.Errors[0].Message, "Required configuration key is missing") {
		t.Fatalf("Validate() expected required-key-missing error, got: %+v", missing.Errors)
	}

	nilRequired := validator.Validate(AgentConfigMap{
		"required_key": nil,
		"optional_key": nil,
	})
	if nilRequired.Valid {
		t.Fatalf("Validate() expected nil required key value to fail")
	}
	if len(nilRequired.Errors) != 1 || nilRequired.Errors[0].Key != "required_key" || !strings.Contains(nilRequired.Errors[0].Message, "value cannot be null") {
		t.Fatalf("Validate() expected nil-required-value error, got: %+v", nilRequired.Errors)
	}

	valid := validator.Validate(AgentConfigMap{
		"required_key": "set",
		"optional_key": nil,
	})
	if !valid.Valid {
		t.Fatalf("Validate() expected valid required key config to pass, got errors: %+v", valid.Errors)
	}
}
