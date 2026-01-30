package investigation

import (
	"strings"
	"testing"
)

func TestGuardrails_DestructiveAndCustomPatterns(t *testing.T) {
	g := NewGuardrails()
	if !g.IsDestructiveAction("rm -rf /tmp") {
		t.Fatalf("expected destructive command")
	}

	g.AddDestructivePattern("custom destroy")
	if !g.IsDestructiveAction("custom destroy now") {
		t.Fatalf("expected custom destructive match")
	}
	if g.IsDestructiveAction("echo safe") {
		t.Fatalf("did not expect safe command to be destructive")
	}
}

func TestGuardrails_RequiresApproval(t *testing.T) {
	g := NewGuardrails()

	// Approval mode: everything needs approval
	if !g.RequiresApproval("warning", "approval", "echo ok") {
		t.Fatalf("expected approval mode to require approval")
	}

	// Assisted mode: warnings auto-fix, critical needs approval
	if g.RequiresApproval("warning", "assisted", "echo ok") {
		t.Fatalf("expected assisted mode warning to skip approval")
	}
	if !g.RequiresApproval("critical", "assisted", "echo ok") {
		t.Fatalf("expected assisted mode critical to require approval")
	}

	// Full mode: everything auto-fixes (user accepts risk)
	if g.RequiresApproval("warning", "full", "echo ok") {
		t.Fatalf("expected full mode to skip approval for warnings")
	}
	if g.RequiresApproval("critical", "full", "echo ok") {
		t.Fatalf("expected full mode to skip approval for critical")
	}
	if g.RequiresApproval("critical", "full", "rm -rf /tmp/test") {
		t.Fatalf("expected full mode to skip approval even for destructive commands")
	}

	// Unknown/default mode: requires approval
	if !g.RequiresApproval("warning", "unknown", "echo ok") {
		t.Fatalf("expected unknown mode to require approval")
	}

	// Destructive commands require approval in all modes except full
	if !g.RequiresApproval("warning", "assisted", "rm -rf /tmp/test") {
		t.Fatalf("expected destructive command to require approval in assisted mode")
	}
}

func TestGuardrails_ClassifyRisk(t *testing.T) {
	g := NewGuardrails()

	if g.ClassifyRisk("rm -rf /tmp") != "critical" {
		t.Fatalf("expected destructive command to be critical risk")
	}
	if g.ClassifyRisk("systemctl restart nginx") != "high" {
		t.Fatalf("expected restart to be high risk")
	}
	if g.ClassifyRisk("echo > /etc/hosts") != "medium" {
		t.Fatalf("expected config change to be medium risk")
	}
	if g.ClassifyRisk("cat /etc/hosts") != "low" {
		t.Fatalf("expected read-only command to be low risk")
	}
	if g.ClassifyRisk("unknown-command") != "medium" {
		t.Fatalf("expected default to be medium risk")
	}
}

func TestGuardrails_ValidateAndSanitize(t *testing.T) {
	g := NewGuardrails()
	if valid, reason := g.ValidateCommand(""); valid || reason == "" {
		t.Fatalf("expected empty command to be invalid")
	}
	longCmd := strings.Repeat("a", 4097)
	if valid, reason := g.ValidateCommand(longCmd); valid || reason == "" {
		t.Fatalf("expected long command to be invalid")
	}
	if valid, reason := g.ValidateCommand("echo ok; rm -rf /"); valid || reason == "" {
		t.Fatalf("expected injection to be invalid")
	}
	if valid, _ := g.ValidateCommand("echo ok"); !valid {
		t.Fatalf("expected command to be valid")
	}

	sanitized, changed := g.SanitizeCommand("  echo ok  ")
	if sanitized != "echo ok" || !changed {
		t.Fatalf("expected trim sanitize")
	}
	sanitized, changed = g.SanitizeCommand(" echo $(rm -rf /) ")
	if !strings.Contains(sanitized, "$(") || !changed {
		t.Fatalf("expected dangerous command to be flagged as changed")
	}
}

func TestGuardrails_RequiresApproval_TableDriven(t *testing.T) {
	g := NewGuardrails()

	tests := []struct {
		name           string
		autonomyLevel  string
		severity       string
		command        string
		expectApproval bool
	}{
		// Full mode — never requires approval
		{"full/critical/destructive", "full", "critical", "rm -rf /tmp/test", false},
		{"full/warning/safe", "full", "warning", "echo ok", false},

		// Approval mode — always requires approval
		{"approval/warning/safe", "approval", "warning", "echo ok", true},
		{"approval/critical/destructive", "approval", "critical", "rm -rf /tmp/test", true},

		// Assisted mode — auto-fix warnings (non-destructive), critical needs approval
		{"assisted/warning/safe", "assisted", "warning", "echo ok", false},
		{"assisted/critical/safe", "assisted", "critical", "echo ok", true},
		{"assisted/warning/destructive", "assisted", "warning", "rm -rf /tmp/test", true},

		// Monitor mode — always requires approval
		{"monitor/warning/safe", "monitor", "warning", "echo ok", true},

		// Empty/unknown — always requires approval
		{"empty/critical/safe", "", "critical", "echo ok", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := g.RequiresApproval(tt.severity, tt.autonomyLevel, tt.command)
			if got != tt.expectApproval {
				t.Errorf("RequiresApproval(%q, %q, %q) = %v, want %v",
					tt.severity, tt.autonomyLevel, tt.command, got, tt.expectApproval)
			}
		})
	}
}

func TestGuardrails_GetDestructivePatterns(t *testing.T) {
	g := NewGuardrails()
	g.AddDestructivePattern("custom")
	patterns := g.GetDestructivePatterns()
	found := false
	for _, pattern := range patterns {
		if pattern == "custom" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected custom pattern to be returned")
	}
}
