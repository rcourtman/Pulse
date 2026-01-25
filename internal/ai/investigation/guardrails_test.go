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
	if !g.RequiresApproval("warning", "approval", "echo ok", true) {
		t.Fatalf("expected approval mode to require approval")
	}
	if !g.RequiresApproval("critical", "full", "echo ok", true) {
		t.Fatalf("expected critical to require approval")
	}
	if g.RequiresApproval("warning", "full", "echo ok", true) {
		t.Fatalf("expected full autonomy warning to skip approval")
	}
	if !g.RequiresApproval("warning", "controlled", "echo ok", true) {
		t.Fatalf("expected default to require approval")
	}

	// Autonomous mode bypasses ALL approvals
	if g.RequiresApproval("warning", "autonomous", "echo ok", true) {
		t.Fatalf("expected autonomous mode to skip approval for safe commands")
	}
	if g.RequiresApproval("critical", "autonomous", "echo ok", true) {
		t.Fatalf("expected autonomous mode to skip approval for critical findings")
	}
	if g.RequiresApproval("critical", "autonomous", "rm -rf /tmp/test", true) {
		t.Fatalf("expected autonomous mode to skip approval even for destructive commands")
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
