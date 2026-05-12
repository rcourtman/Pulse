package ai

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/forecast"
)

func TestBuildCapacityActionProposal_AttachesProposalForCapacityFinding(t *testing.T) {
	finding := &Finding{
		ID:           "f-cap-1",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "node-a/storage/tank",
		ResourceName: "tank",
		ResourceType: "storage",
		Title:        "Storage pool tank at 87.3% usage",
	}

	proposal := buildCapacityActionProposal(finding)
	if proposal == nil {
		t.Fatal("expected ProposedActionPlan for storage capacity finding, got nil")
	}
	if !proposal.RequiresApproval {
		t.Error("RequiresApproval = false; capacity proposals must always be approval-gated")
	}
	if proposal.Allowed {
		t.Error("Allowed = true; current templates ship preflight-only (Allowed=false)")
	}
	if proposal.Source != forecast.CapacityActionPlanSource {
		t.Errorf("Source = %q; want %q so the frontend renders the capacity-forecast card", proposal.Source, forecast.CapacityActionPlanSource)
	}
	if proposal.ProjectedMetric == nil {
		t.Fatal("ProjectedMetric must be populated so the operator sees current/projected/threshold")
	}
	if proposal.ProjectedMetric.Metric != "usage_percent" {
		t.Errorf("ProjectedMetric.Metric = %q; want usage_percent for storage", proposal.ProjectedMetric.Metric)
	}
	if proposal.ProjectedMetric.CurrentValue == 0 {
		t.Error("ProjectedMetric.CurrentValue = 0; expected 87.3 parsed from finding title")
	}
	if proposal.Preflight == nil {
		t.Fatal("Preflight projection must be present so the card surfaces SafetyChecks")
	}
	if len(proposal.Preflight.SafetyChecks) == 0 {
		t.Error("Preflight.SafetyChecks must round-trip from the template")
	}
	if proposal.ActionID == "" || !strings.HasPrefix(proposal.ActionID, "capacity-forecast-") {
		t.Errorf("ActionID = %q; want capacity-forecast- prefix", proposal.ActionID)
	}
}

func TestBuildCapacityActionProposal_ReturnsNilForNonCapacityFinding(t *testing.T) {
	cases := []struct {
		name     string
		category FindingCategory
	}{
		{"performance", FindingCategoryPerformance},
		{"reliability", FindingCategoryReliability},
		{"backup", FindingCategoryBackup},
		{"security", FindingCategorySecurity},
		{"general", FindingCategoryGeneral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			finding := &Finding{
				ID:           "f-noncap",
				Severity:     FindingSeverityWarning,
				Category:     tc.category,
				ResourceID:   "node-a/storage/tank",
				ResourceName: "tank",
				ResourceType: "storage",
				Title:        "Storage pool tank at 87.3% usage",
			}
			if proposal := buildCapacityActionProposal(finding); proposal != nil {
				t.Fatalf("expected nil proposal for category %s; got %+v", tc.category, proposal)
			}
		})
	}
}

func TestBuildCapacityActionProposal_ReturnsNilForUnregisteredResourceType(t *testing.T) {
	finding := &Finding{
		ID:           "f-cap-unknown",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "agent/raid/0",
		ResourceName: "controller-0",
		ResourceType: "agent_raid",
		Title:        "RAID controller has no capacity template",
	}
	if proposal := buildCapacityActionProposal(finding); proposal != nil {
		t.Fatalf("expected nil proposal for unregistered resource type; got %+v", proposal)
	}
}

func TestBuildCapacityActionProposal_VMDiskFindingMatchesGuestTemplate(t *testing.T) {
	finding := &Finding{
		ID:           "f-vm-cap",
		Severity:     FindingSeverityCritical,
		Category:     FindingCategoryCapacity,
		ResourceID:   "node-a/qemu/101",
		ResourceName: "appserver",
		ResourceType: "qemu",
		Title:        "appserver disk at 95.2% used",
	}
	proposal := buildCapacityActionProposal(finding)
	if proposal == nil {
		t.Fatal("expected proposal for qemu disk finding, got nil")
	}
	if proposal.ProjectedMetric == nil || proposal.ProjectedMetric.Metric != "disk_usage_percent" {
		t.Errorf("ProjectedMetric.Metric = %v; want disk_usage_percent for guest disk", proposal.ProjectedMetric)
	}
	if proposal.ProjectedMetric.CurrentValue == 0 {
		t.Error("CurrentValue = 0; expected parsed value from title")
	}
}

func TestGenerateRemediationPlan_AttachesProposalForCapacityFinding(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	engine := newTestRemediationEngine()
	ps.SetRemediationEngine(engine)

	finding := &Finding{
		ID:           "f-gen-cap",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryCapacity,
		ResourceID:   "node-a/storage/tank",
		ResourceName: "tank",
		ResourceType: "storage",
		Title:        "Storage pool tank at 88.2% usage",
		Description:  "Tank pool exceeded warning threshold.",
		DetectedAt:   time.Now(),
	}

	ps.generateRemediationPlan(finding)

	plan := engine.GetPlanForFinding(finding.ID)
	if plan == nil {
		t.Fatal("expected remediation plan to be created for capacity finding")
	}
	if plan.ProposedActionPlan == nil {
		t.Fatal("RemediationPlan.ProposedActionPlan = nil; capacity finding should attach a proposal via the forecast template registry")
	}
	if !plan.ProposedActionPlan.RequiresApproval {
		t.Error("attached proposal must have RequiresApproval=true")
	}
	if plan.ProposedActionPlan.Source != forecast.CapacityActionPlanSource {
		t.Errorf("attached proposal Source = %q; want %q", plan.ProposedActionPlan.Source, forecast.CapacityActionPlanSource)
	}
}

func TestGenerateRemediationPlan_DoesNotAttachProposalForNonCapacityFinding(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	engine := newTestRemediationEngine()
	ps.SetRemediationEngine(engine)

	finding := &Finding{
		ID:           "f-gen-perf",
		Severity:     FindingSeverityWarning,
		Category:     FindingCategoryPerformance,
		ResourceID:   "node-a/qemu/101",
		ResourceName: "appserver",
		ResourceType: "qemu",
		Title:        "High CPU on appserver: 92.0%",
		Description:  "Sustained CPU usage above warning threshold.",
		DetectedAt:   time.Now(),
	}

	ps.generateRemediationPlan(finding)

	plan := engine.GetPlanForFinding(finding.ID)
	if plan == nil {
		t.Fatal("expected remediation plan to be created for performance finding")
	}
	if plan.ProposedActionPlan != nil {
		t.Fatalf("non-capacity finding must not attach a forecast proposal; got %+v", plan.ProposedActionPlan)
	}
}

func TestExtractCurrentValue(t *testing.T) {
	cases := []struct {
		name  string
		title string
		want  float64
	}{
		{"trailing percent", "Storage pool tank at 87.3% usage", 87.3},
		{"colon then percent", "High disk usage on foo: 95%", 95.0},
		{"integer percent", "Disk used 100%", 100.0},
		{"no percent", "Storage pool tank running low", 0},
		{"percent in middle", "appserver disk at 92.5% used", 92.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractCurrentValue(&Finding{Title: tc.title})
			if got != tc.want {
				t.Errorf("extractCurrentValue(%q) = %v; want %v", tc.title, got, tc.want)
			}
		})
	}
}
