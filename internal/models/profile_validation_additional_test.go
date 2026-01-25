package models

import "testing"

func TestValidationErrorError(t *testing.T) {
	err := ValidationError{Key: "cpu_threshold", Message: "invalid"}
	if got := err.Error(); got != "cpu_threshold: invalid" {
		t.Fatalf("Error() = %q, want cpu_threshold: invalid", got)
	}
}
