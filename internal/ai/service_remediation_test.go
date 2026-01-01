package ai

import (
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// ============================================================================
// Remediation Tests
// ============================================================================

func TestService_Remediation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-ai-rem-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	patrol := NewPatrolService(svc, nil)
	svc.patrolService = patrol // Manual link for test

	remLog := NewRemediationLog(RemediationLogConfig{DataDir: tmpDir})
	svc.SetRemediationLog(remLog)

	// Test logRemediation - use an actionable command (not diagnostic)
	req := ExecuteRequest{
		TargetID:   "vm-101",
		TargetType: "vm",
		Prompt:     "High CPU",
	}
	svc.logRemediation(req, "systemctl restart myservice", "output", true)

	// Verify it was logged
	history := remLog.GetForResource("vm-101", 5)
	if len(history) != 1 {
		t.Fatalf("Expected 1 remediation record, got %d", len(history))
	}
	if history[0].Action != "systemctl restart myservice" {
		t.Errorf("Expected action 'systemctl restart myservice', got %s", history[0].Action)
	}

	// Test buildRemediationContext
	ctx := svc.buildRemediationContext("vm-101", "High CPU")
	if !containsString(ctx, "Remediation History for This Resource") {
		t.Error("Expected remediation history section in context")
	}
	if !containsString(ctx, "systemctl restart myservice") {
		t.Error("Expected logged action in context")
	}

	// Test with similar problem
	ctx2 := svc.buildRemediationContext("other-vm", "High CPU")
	if !containsString(ctx2, "Past Successful Fixes for Similar Issues") {
		t.Error("Expected successful fixes section in context for similar problem")
	}
}

func TestService_Remediation_NoPatrolService(t *testing.T) {
	svc := NewService(nil, nil)
	// patrolService is nil, logRemediation should handle gracefully

	req := ExecuteRequest{
		TargetID:   "vm-102",
		TargetType: "vm",
		Prompt:     "Test",
	}
	// Should not panic
	svc.logRemediation(req, "test", "output", true)
}

func TestService_Remediation_NoRemediationLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pulse-ai-rem-test-nolog-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	persistence := config.NewConfigPersistence(tmpDir)
	svc := NewService(persistence, nil)
	patrol := NewPatrolService(svc, nil)
	svc.patrolService = patrol
	// remediationLog is nil on patrol

	req := ExecuteRequest{
		TargetID:   "vm-103",
		TargetType: "vm",
		Prompt:     "Test",
	}
	// Should not panic
	svc.logRemediation(req, "test", "output", true)
}

func TestService_BuildRemediationContext_Empty(t *testing.T) {
	svc := NewService(nil, nil)

	// With no remediationLog set, should return empty
	ctx := svc.buildRemediationContext("unknown", "Unknown problem")
	if ctx != "" {
		t.Error("Expected empty context when no remediation log")
	}
}

// ============================================================================
// String Utility Tests
// ============================================================================

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"empty string", "", 5, ""},
		{"under limit", "123", 5, "123"},
		{"at limit", "12345", 5, "12345"},
		{"over limit", "1234567890", 5, "12..."},
		{"exactly at max minus ellipsis", "1234", 4, "1234"},
		{"just over", "12345", 4, "1..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.max)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, expected %q",
					tt.input, tt.max, result, tt.expected)
			}
		})
	}
}

func TestContainsString(t *testing.T) {
	tests := []struct {
		haystack string
		needle   string
		expected bool
	}{
		{"hello world", "world", true},
		{"hello world", "foo", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		if containsString(tt.haystack, tt.needle) != tt.expected {
			t.Errorf("containsString(%q, %q) expected %v",
				tt.haystack, tt.needle, tt.expected)
		}
	}
}
