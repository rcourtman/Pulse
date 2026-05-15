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
	ctx := svc.buildRemediationContext("vm-101")
	if !containsString(ctx, "Prior Actions Recorded For This Resource") {
		t.Error("Expected remediation history section in context")
	}
	if !containsString(ctx, "systemctl restart myservice") {
		t.Error("Expected logged action in context")
	}

	// Similar problems on other resources must not inject recommended fixes.
	ctx2 := svc.buildRemediationContext("other-vm")
	if ctx2 != "" {
		t.Error("Expected no keyword-matched remediation context for another resource")
	}
}

func TestService_BuildRemediationContext_Empty(t *testing.T) {
	svc := NewService(nil, nil)

	// With no remediationLog set, should return empty
	ctx := svc.buildRemediationContext("unknown")
	if ctx != "" {
		t.Error("Expected empty context when no remediation log")
	}
}

func TestService_BuildSystemPrompt_FindingContextDoesNotForceLifecycleTool(t *testing.T) {
	svc := NewService(nil, nil)

	prompt := svc.buildSystemPrompt(ExecuteRequest{FindingID: "finding-123"}, "")
	if containsString(prompt, "use ONE of these tools") {
		t.Fatalf("finding context must not force a lifecycle tool choice:\n%s", prompt)
	}
	if containsString(prompt, "Past Successful Fixes for Similar Issues") {
		t.Fatalf("finding context must not include keyword-matched remediation suggestions:\n%s", prompt)
	}
	if !containsString(prompt, "Lifecycle tools are available when current evidence supports them") {
		t.Fatalf("expected neutral lifecycle-tool context, got:\n%s", prompt)
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
