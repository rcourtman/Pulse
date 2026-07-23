package api

import (
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// This file raises branch/function coverage for a handful of pure (or
// pure-over-argument) targets across setup_script_render.go,
// router_routes_ai_relay.go, hosted_entitlement_refresh.go and
// report_schedules.go. Each subtest drives concrete inputs and asserts the
// concrete output value/error. No live stores, network or filesystem are
// touched: the adapter methods under test ignore their receivers entirely.

func TestBranchcov0723AmDeriveSetupScriptServerName(t *testing.T) {
	cases := []struct {
		name       string
		serverHost string
		want       string
	}{
		// Empty / whitespace inputs collapse to the documented fallback.
		{"empty_string", "", "your-server"},
		{"whitespace_only_trimmed_to_empty", "   \t\n  ", "your-server"},
		// No scheme: the final return splits the host on ":" and keeps the name.
		{"plain_host_no_port", "pve1.local", "pve1.local"},
		{"plain_host_with_port_strips_port", "pve1.local:8006", "pve1.local"},
		// Scheme present: the name is derived from the post-"://" segment.
		{"scheme_host_no_port", "https://pve1.local", "pve1.local"},
		{"scheme_host_with_port_strips_port", "https://pve1.local:8006", "pve1.local"},
		// Trimming is applied BEFORE scheme detection, so surrounding whitespace
		// does not defeat the "://" branch.
		{"whitespace_trimmed_before_scheme_parse", "  https://pve1.local:8006  ", "pve1.local"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveSetupScriptServerName(tc.serverHost)
			if got != tc.want {
				t.Fatalf("deriveSetupScriptServerName(%q) = %q, want %q", tc.serverHost, got, tc.want)
			}
		})
	}
}

func TestBranchcov0723AmPatrolConfigAdapterIsValidAutonomyLevel(t *testing.T) {
	// IsValidPatrolAutonomyLevel ignores the receiver entirely (it delegates to
	// a pure package function), so a zero-valued adapter is safe to construct.
	adapter := &patrolConfigAdapter{}

	validLevels := []string{
		config.PatrolAutonomyMonitor,
		config.PatrolAutonomyApproval,
		config.PatrolAutonomyAssisted,
		config.PatrolAutonomyFull,
	}
	for _, level := range validLevels {
		t.Run("valid/"+level, func(t *testing.T) {
			if !adapter.IsValidPatrolAutonomyLevel(level) {
				t.Fatalf("IsValidPatrolAutonomyLevel(%q) = false, want true", level)
			}
		})
	}

	invalidCases := []struct {
		name  string
		level string
	}{
		{"empty", ""},
		{"unknown_level", "supervised"},
		{"wrong_case_upper", "FULL"},
		{"wrong_case_capitalised", "Monitor"},
		{"whitespace_padded", " monitor "},
	}
	for _, tc := range invalidCases {
		t.Run("invalid/"+tc.name, func(t *testing.T) {
			if adapter.IsValidPatrolAutonomyLevel(tc.level) {
				t.Fatalf("IsValidPatrolAutonomyLevel(%q) = true, want false", tc.level)
			}
		})
	}
}

func TestBranchcov0723AmApprovalStoreAdapterAssessRiskLevel(t *testing.T) {
	// AssessRiskLevel delegates to a pure function and never dereferences the
	// receiver, so a zero-valued adapter is safe.
	adapter := &approvalStoreAdapter{}

	cases := []struct {
		name       string
		command    string
		targetType string
		want       string
	}{
		// High-risk command (rm -rf) matches a high-risk pattern regardless of
		// the target type.
		{"high_risk_command_non_node_target", "rm -rf /var/log", "container", "high"},
		// Medium-risk command on a non-node target returns medium.
		{"medium_risk_command_non_node_target", "systemctl restart nginx", "agent", "medium"},
		// No pattern matches: default arm returns low.
		{"low_risk_command_default_arm", "echo hello", "agent", "low"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := adapter.AssessRiskLevel(tc.command, tc.targetType)
			if got != tc.want {
				t.Fatalf("AssessRiskLevel(%q, %q) = %q, want %q", tc.command, tc.targetType, got, tc.want)
			}
		})
	}

	// Regression lock documenting SOURCE BUG: the `targetType == "node"`
	// escalation block in approval.AssessRiskLevel is unreachable dead code.
	// The preceding unconditional medium-pattern loop returns RiskMedium for
	// any medium command before the node branch runs, and a non-medium command
	// cannot match the node-branch loop either. As a result the target type has
	// no observable effect, and the intended "node + medium => high" rule never
	// fires. This locks in the *current* (buggy) behaviour so a future fix is a
	// deliberate, visible change.
	t.Run("node_target_does_not_escalate_medium_command_dead_branch", func(t *testing.T) {
		got := adapter.AssessRiskLevel("systemctl restart nginx", "node")
		const want = "medium" // would be "high" if the node escalation worked
		if got != want {
			t.Fatalf("AssessRiskLevel(%q, %q) = %q, want %q (node escalation is currently dead code)",
				"systemctl restart nginx", "node", got, want)
		}
	})
}

func TestBranchcov0723AmHostedEntitlementRefreshError(t *testing.T) {
	t.Run("populated_message_returned_verbatim", func(t *testing.T) {
		err := &hostedEntitlementRefreshError{statusCode: 410, message: "lease revoked", permanent: true}
		const want = "lease revoked"
		if got := err.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("whitespace_message_falls_back_to_status_format", func(t *testing.T) {
		err := &hostedEntitlementRefreshError{statusCode: 503, message: "   "}
		const want = "hosted entitlement refresh failed with status 503"
		if got := err.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("zero_value_falls_back_to_status_zero", func(t *testing.T) {
		err := &hostedEntitlementRefreshError{}
		const want = "hosted entitlement refresh failed with status 0"
		if got := err.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("nil_receiver_returns_empty", func(t *testing.T) {
		var err *hostedEntitlementRefreshError
		if got := err.Error(); got != "" {
			t.Fatalf("nil receiver Error() = %q, want %q", got, "")
		}
	})

	// The type defines no Unwrap, so it does not wrap an underlying cause.
	t.Run("does_not_wrap_cause", func(t *testing.T) {
		err := &hostedEntitlementRefreshError{statusCode: 410, message: "lease revoked"}
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			t.Fatalf("errors.Unwrap() = %v, want nil (no wrapped cause)", unwrapped)
		}
		var target *hostedEntitlementRefreshError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As to the same concrete pointer type should succeed")
		}
	})
}

func TestBranchcov0723AmReportScheduleValidationError(t *testing.T) {
	t.Run("populated_value_returns_message", func(t *testing.T) {
		err := reportScheduleValidationError{code: "invalid_name", message: "Schedule name is required"}
		const want = "Schedule name is required"
		if got := err.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("zero_value_returns_empty", func(t *testing.T) {
		var err reportScheduleValidationError
		if got := err.Error(); got != "" {
			t.Fatalf("zero value Error() = %q, want %q", got, "")
		}
	})

	// Value-receiver error type with no Unwrap: it never wraps a cause.
	t.Run("does_not_wrap_cause", func(t *testing.T) {
		err := reportScheduleValidationError{code: "invalid_name", message: "boom"}
		if unwrapped := errors.Unwrap(err); unwrapped != nil {
			t.Fatalf("errors.Unwrap() = %v, want nil (no wrapped cause)", unwrapped)
		}
		var target reportScheduleValidationError
		if !errors.As(err, &target) {
			t.Fatalf("errors.As to the same concrete value type should succeed")
		}
	})
}
