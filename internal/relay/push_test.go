package relay

import (
	"strings"
	"testing"
)

func TestNewPatrolFindingNotification(t *testing.T) {
	t.Run("warning finding", func(t *testing.T) {
		n := NewPatrolFindingNotification("finding-123", "warning", "performance", "High CPU usage detected")

		if n.Type != PushTypePatrolFinding {
			t.Errorf("Type: got %q, want %q", n.Type, PushTypePatrolFinding)
		}
		if n.Priority != PushPriorityNormal {
			t.Errorf("Priority: got %q, want %q", n.Priority, PushPriorityNormal)
		}
		if n.Title != "High CPU usage detected" {
			t.Errorf("Title: got %q, want %q", n.Title, "High CPU usage detected")
		}
		if n.ActionType != PushActionViewFinding {
			t.Errorf("ActionType: got %q, want %q", n.ActionType, PushActionViewFinding)
		}
		if n.ActionID != "finding-123" {
			t.Errorf("ActionID: got %q, want %q", n.ActionID, "finding-123")
		}
		if n.Category != "performance" {
			t.Errorf("Category: got %q, want %q", n.Category, "performance")
		}
		if n.Severity != "warning" {
			t.Errorf("Severity: got %q, want %q", n.Severity, "warning")
		}
	})

	t.Run("critical finding uses high priority", func(t *testing.T) {
		n := NewPatrolFindingNotification("finding-456", "critical", "reliability", "Resource offline")

		if n.Type != PushTypePatrolCritical {
			t.Errorf("Type: got %q, want %q", n.Type, PushTypePatrolCritical)
		}
		if n.Priority != PushPriorityHigh {
			t.Errorf("Priority: got %q, want %q", n.Priority, PushPriorityHigh)
		}
	})
}

func TestNewApprovalRequestNotification(t *testing.T) {
	t.Run("with risk level", func(t *testing.T) {
		n := NewApprovalRequestNotification("approval-789", "Fix disk space", "high")

		if n.Type != PushTypeApprovalRequest {
			t.Errorf("Type: got %q, want %q", n.Type, PushTypeApprovalRequest)
		}
		if n.Priority != PushPriorityHigh {
			t.Errorf("Priority: got %q, want %q", n.Priority, PushPriorityHigh)
		}
		if n.ActionType != PushActionApproveFix {
			t.Errorf("ActionType: got %q, want %q", n.ActionType, PushActionApproveFix)
		}
		if n.ActionID != "approval-789" {
			t.Errorf("ActionID: got %q, want %q", n.ActionID, "approval-789")
		}
		if !strings.Contains(n.Body, "high-risk") {
			t.Errorf("Body should contain risk level, got %q", n.Body)
		}
	})

	t.Run("without risk level", func(t *testing.T) {
		n := NewApprovalRequestNotification("approval-000", "Fix CPU", "")

		if strings.Contains(n.Body, "-risk") {
			t.Errorf("Body should not contain risk level when empty, got %q", n.Body)
		}
	})
}

func TestNewFixCompletedNotification(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		n := NewFixCompletedNotification("finding-100", "Cleared disk space", true)

		if n.Type != PushTypeFixCompleted {
			t.Errorf("Type: got %q, want %q", n.Type, PushTypeFixCompleted)
		}
		if n.Priority != PushPriorityNormal {
			t.Errorf("Priority: got %q, want %q", n.Priority, PushPriorityNormal)
		}
		if n.ActionType != PushActionViewFixResult {
			t.Errorf("ActionType: got %q, want %q", n.ActionType, PushActionViewFixResult)
		}
		if n.ActionID != "finding-100" {
			t.Errorf("ActionID: got %q, want %q", n.ActionID, "finding-100")
		}
		if !strings.Contains(n.Body, "successfully") {
			t.Errorf("Body should indicate success, got %q", n.Body)
		}
	})

	t.Run("failure", func(t *testing.T) {
		n := NewFixCompletedNotification("finding-101", "Restart service", false)

		if !strings.Contains(n.Body, "failed") {
			t.Errorf("Body should indicate failure, got %q", n.Body)
		}
	})
}

func TestNotificationTruncation(t *testing.T) {
	longTitle := strings.Repeat("A", 200)
	n := NewPatrolFindingNotification("id", "warning", "capacity", longTitle)

	if len(n.Title) > 100 {
		t.Errorf("Title should be truncated to 100 chars, got %d", len(n.Title))
	}
	if !strings.HasSuffix(n.Title, "...") {
		t.Errorf("Truncated title should end with '...', got %q", n.Title[len(n.Title)-5:])
	}

	// Body truncation: build a notification with a very long body by using long category/severity
	longCategory := strings.Repeat("X", 200)
	n2 := NewPatrolFindingNotification("id", "warning", longCategory, "Title")

	if len(n2.Body) > 200 {
		t.Errorf("Body should be truncated to 200 chars, got %d", len(n2.Body))
	}
}

func TestSanitizeTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no sensitive data passes through",
			input: "High CPU usage detected",
			want:  "High CPU usage detected",
		},
		{
			name:  "node identifier replaced",
			input: "High CPU on node-1",
			want:  "High CPU on [resource]",
		},
		{
			name:  "pve cluster name replaced",
			input: "Disk full on pve-cluster02",
			want:  "Disk full on [resource]",
		},
		{
			name:  "vm identifier replaced",
			input: "Memory leak in vm-100",
			want:  "Memory leak in [resource]",
		},
		{
			name:  "ct identifier replaced",
			input: "Container ct-200 unresponsive",
			want:  "Container [resource] unresponsive",
		},
		{
			name:  "IPv4 address replaced",
			input: "Cannot reach 192.168.1.100",
			want:  "Cannot reach [resource]",
		},
		{
			name:  "FQDN replaced",
			input: "High load on web01.prod.example.com",
			want:  "High load on [resource]",
		},
		{
			name:  "multiple identifiers replaced",
			input: "node-1 to node-2 replication lag",
			want:  "[resource] to [resource] replication lag",
		},
		{
			name:  "adjacent placeholders collapsed",
			input: "node-1 node-2 offline",
			want:  "[resource] offline",
		},
		{
			name:  "IP in parentheses",
			input: "Host (192.168.1.10) unreachable",
			want:  "Host ([resource]) unreachable",
		},
		{
			name:  "IP in brackets (Go net format)",
			input: "Connection to [10.0.0.1]:8006 failed",
			want:  "Connection to [[resource]]:8006 failed",
		},
		{
			name:  "IP with port (no brackets)",
			input: "Connection to 10.0.0.1:8006 failed",
			want:  "Connection to [resource] failed",
		},
		{
			name:  "IP after equals sign",
			input: "Failed host=172.16.0.5 timeout",
			want:  "Failed host=[resource] timeout",
		},
		{
			name:  "IP in quotes",
			input: `Error from "10.20.30.40" connection reset`,
			want:  `Error from "[resource]" connection reset`,
		},
		{
			name:  "generic words preserved",
			input: "Storage pool nearly full",
			want:  "Storage pool nearly full",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTitle(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeTitleInBuilders(t *testing.T) {
	// Verify that all builder functions apply sanitization
	n1 := NewPatrolFindingNotification("id", "warning", "performance", "High CPU on node-prod-1")
	if strings.Contains(n1.Title, "node-prod-1") {
		t.Errorf("finding title should be sanitized, got %q", n1.Title)
	}

	n2 := NewApprovalRequestNotification("id", "Fix disk on vm-100", "")
	if strings.Contains(n2.Title, "vm-100") {
		t.Errorf("approval title should be sanitized, got %q", n2.Title)
	}

	n3 := NewFixCompletedNotification("id", "Restarted ct-50", true)
	if strings.Contains(n3.Title, "ct-50") {
		t.Errorf("fix title should be sanitized, got %q", n3.Title)
	}
}
