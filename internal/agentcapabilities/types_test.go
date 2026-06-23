package agentcapabilities

import (
	"net/http"
	"strings"
	"testing"
)

func TestDefaultApprovalPolicyDescriptionUsesSharedPolicyVocabulary(t *testing.T) {
	cases := []struct {
		policy ApprovalPolicy
		want   string
	}{
		{ApprovalPolicyScopeOnly, "required scope is sufficient"},
		{ApprovalPolicyActionPlan, "governed action-plan approval is required"},
		{ApprovalPolicy("custom"), ""},
	}

	for _, tc := range cases {
		got := DefaultApprovalPolicyDescription(tc.policy)
		if tc.want == "" {
			if got != "" {
				t.Fatalf("DefaultApprovalPolicyDescription(%q) = %q, want empty", tc.policy, got)
			}
			continue
		}
		if got == "" || !strings.Contains(got, tc.want) {
			t.Fatalf("DefaultApprovalPolicyDescription(%q) = %q, want substring %q", tc.policy, got, tc.want)
		}
	}
}

func TestToolGovernanceDescriptorUsesSharedVocabulary(t *testing.T) {
	descriptor := ToolGovernanceDescriptor{
		Name:            "pulse_control",
		Description:     "Run governed control actions.",
		RequireControl:  true,
		ActionMode:      ActionModeWrite,
		ApprovalPolicy:  ApprovalPolicyActionPlan,
		ApprovalSummary: "approval required in controlled mode",
		Summary:         "Runs shared Pulse control actions.",
	}

	if descriptor.ActionMode != ActionModeWrite {
		t.Fatalf("descriptor action mode = %q, want %q", descriptor.ActionMode, ActionModeWrite)
	}
	if descriptor.ApprovalPolicy != ApprovalPolicyActionPlan {
		t.Fatalf("descriptor approval policy = %q, want %q", descriptor.ApprovalPolicy, ApprovalPolicyActionPlan)
	}
	if !descriptor.RequireControl {
		t.Fatal("descriptor must preserve Assistant control gating")
	}
}

func TestNormalizeToolGovernanceAppliesSharedDefaults(t *testing.T) {
	read := NormalizeToolGovernance(ToolGovernance{}, false, "Read state.")
	if read.ActionMode != ActionModeRead {
		t.Fatalf("read action mode = %q, want %q", read.ActionMode, ActionModeRead)
	}
	if read.ApprovalPolicy != ApprovalPolicyScopeOnly {
		t.Fatalf("read approval policy = %q, want %q", read.ApprovalPolicy, ApprovalPolicyScopeOnly)
	}
	if read.ApprovalSummary != "no approval required" {
		t.Fatalf("read approval summary = %q", read.ApprovalSummary)
	}
	if read.Summary != "Read state." {
		t.Fatalf("read summary = %q", read.Summary)
	}

	control := NormalizeToolGovernance(ToolGovernance{}, true, "Run actions.")
	if control.ActionMode != ActionModeWrite {
		t.Fatalf("control action mode = %q, want %q", control.ActionMode, ActionModeWrite)
	}
	if control.ApprovalPolicy != ApprovalPolicyActionPlan {
		t.Fatalf("control approval policy = %q, want %q", control.ApprovalPolicy, ApprovalPolicyActionPlan)
	}
	if control.ApprovalSummary != "hidden in read-only mode; approval required in controlled mode" {
		t.Fatalf("control approval summary = %q", control.ApprovalSummary)
	}

	mixed := NormalizeToolGovernance(ToolGovernance{ActionMode: ActionModeMixed}, false, "Mixed actions.")
	if mixed.ApprovalPolicy != ApprovalPolicyScopeOnly {
		t.Fatalf("mixed approval policy = %q, want %q", mixed.ApprovalPolicy, ApprovalPolicyScopeOnly)
	}
	if !strings.Contains(mixed.ApprovalSummary, "required scope is sufficient") {
		t.Fatalf("mixed approval summary = %q", mixed.ApprovalSummary)
	}
}

func TestNormalizeCapabilityGovernanceAppliesSharedDefaults(t *testing.T) {
	read := NormalizeCapabilityGovernance(Capability{
		Description: "Read capability.",
		Method:      http.MethodGet,
	})
	if read.ActionMode != ActionModeRead {
		t.Fatalf("read capability action mode = %q, want %q", read.ActionMode, ActionModeRead)
	}
	if read.ApprovalPolicy != ApprovalPolicyScopeOnly {
		t.Fatalf("read capability approval policy = %q, want %q", read.ApprovalPolicy, ApprovalPolicyScopeOnly)
	}
	if read.ApprovalSummary != "no approval required" {
		t.Fatalf("read capability approval summary = %q", read.ApprovalSummary)
	}
	if read.Summary != "Read capability." {
		t.Fatalf("read capability summary = %q", read.Summary)
	}

	write := NormalizeCapabilityGovernance(Capability{
		Description: "Write capability.",
		Method:      http.MethodPost,
	})
	if write.ActionMode != ActionModeWrite {
		t.Fatalf("write capability action mode = %q, want %q", write.ActionMode, ActionModeWrite)
	}
	if write.ApprovalPolicy != ApprovalPolicyScopeOnly {
		t.Fatalf("write capability approval policy = %q, want %q", write.ApprovalPolicy, ApprovalPolicyScopeOnly)
	}

	planned := NormalizeCapabilityGovernance(Capability{
		Description:    "Plan an action.",
		Method:         http.MethodPost,
		ActionMode:     ActionModeMixed,
		ApprovalPolicy: ApprovalPolicyActionPlan,
	})
	if planned.ActionMode != ActionModeMixed {
		t.Fatalf("explicit capability action mode = %q, want %q", planned.ActionMode, ActionModeMixed)
	}
	if planned.ApprovalPolicy != ApprovalPolicyActionPlan {
		t.Fatalf("explicit capability approval policy = %q, want %q", planned.ApprovalPolicy, ApprovalPolicyActionPlan)
	}
	if !strings.Contains(planned.ApprovalSummary, "governed action-plan approval is required") {
		t.Fatalf("explicit capability approval summary = %q", planned.ApprovalSummary)
	}
}

func TestNewToolGovernanceDescriptorAppliesSharedDefaults(t *testing.T) {
	descriptor := NewToolGovernanceDescriptor("pulse_control", "Run actions.", true, ToolGovernance{})
	if descriptor.Name != "pulse_control" || descriptor.Description != "Run actions." {
		t.Fatalf("descriptor identity = %+v", descriptor)
	}
	if !descriptor.RequireControl {
		t.Fatal("descriptor must preserve control gating")
	}
	if descriptor.ActionMode != ActionModeWrite || descriptor.ApprovalPolicy != ApprovalPolicyActionPlan {
		t.Fatalf("descriptor governance = %+v", descriptor)
	}
	if descriptor.ApprovalSummary != "hidden in read-only mode; approval required in controlled mode" {
		t.Fatalf("descriptor approval summary = %q", descriptor.ApprovalSummary)
	}
}
