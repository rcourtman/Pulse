package server

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

func TestApplyPulseIntelligenceTelemetrySnapshot_AggregatesContentFreeLoopCounts(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	recent := now.Add(-2 * time.Hour)
	old := now.Add(-(telemetry.PulseIntelligenceTelemetryWindow + time.Hour))

	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveAIUsageHistory([]config.AIUsageEventRecord{
		{Timestamp: recent, UseCase: "chat"},
		{Timestamp: recent.Add(-time.Minute), UseCase: "CHAT", ContextScope: "resource_context", ToolCallCount: 3},
		{Timestamp: recent.Add(-2 * time.Minute), UseCase: "chat", FindingID: "finding-123", ToolCallCount: 2},
		{Timestamp: recent, UseCase: "patrol"},
		{Timestamp: recent, UseCase: "unknown", ToolCallCount: 99},
		{Timestamp: old, UseCase: "chat", ToolCallCount: 99},
	}); err != nil {
		t.Fatalf("SaveAIUsageHistory: %v", err)
	}
	if err := persistence.SavePatrolRunHistory([]config.PatrolRunRecord{
		{StartedAt: recent.Add(-5 * time.Minute), CompletedAt: recent, NewFindings: 2, AutoFixCount: 1},
		{StartedAt: recent.Add(-time.Hour), CompletedAt: recent.Add(-30 * time.Minute), NewFindings: 3},
		{StartedAt: old, CompletedAt: old, NewFindings: 99, AutoFixCount: 99},
	}); err != nil {
		t.Fatalf("SavePatrolRunHistory: %v", err)
	}
	if err := persistence.SaveAIFindings(map[string]*config.AIFindingRecord{
		"recent-last-investigated": {
			ID:                 "recent-last-investigated",
			LastInvestigatedAt: &recent,
		},
		"recent-record-started": {
			ID: "recent-record-started",
			InvestigationRecord: &aicontracts.InvestigationRecord{
				ID:        "investigation-recent",
				FindingID: "recent-record-started",
				StartedAt: recent.Add(-15 * time.Minute),
			},
		},
		"recent-resolved": {
			ID:         "recent-resolved",
			ResolvedAt: &recent,
		},
		"recent-fix-verified": {
			ID:                   "recent-fix-verified",
			InvestigationOutcome: "fix_verified",
			InvestigationRecord: &aicontracts.InvestigationRecord{
				ID:          "investigation-verified",
				FindingID:   "recent-fix-verified",
				CompletedAt: &recent,
			},
		},
		"old-last-investigated": {
			ID:                 "old-last-investigated",
			LastInvestigatedAt: &old,
		},
		"old-resolved": {
			ID:         "old-resolved",
			ResolvedAt: &old,
		},
	}); err != nil {
		t.Fatalf("SaveAIFindings: %v", err)
	}
	if err := persistence.SaveExternalAgentActivityHistory([]config.ExternalAgentActivityRecord{
		{Timestamp: recent, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityFleetContext},
		{Timestamp: recent.Add(-time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityResourceContext},
		{Timestamp: recent.Add(-2 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityEventStream},
		{Timestamp: recent.Add(-3 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityProvisioning},
		{Timestamp: recent.Add(-4 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityOperatorState},
		{Timestamp: recent.Add(-5 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityFindingList},
		{Timestamp: recent.Add(-6 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityFindingDecision},
		{Timestamp: recent.Add(-7 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityActionPlan},
		{Timestamp: recent.Add(-8 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityActionDecision},
		{Timestamp: recent.Add(-9 * time.Minute), Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityActionExecute},
		{Timestamp: recent.Add(-10 * time.Minute), Surface: config.ExternalAgentActivitySurfacePulseMCP, Activity: config.ExternalAgentActivityActionExecute},
		{Timestamp: recent.Add(-10 * time.Minute), Surface: "support_api", Activity: config.ExternalAgentActivityResourceContext},
		{Timestamp: old, Surface: config.ExternalAgentActivitySurfaceAgentAPI, Activity: config.ExternalAgentActivityResourceContext},
	}); err != nil {
		t.Fatalf("SaveExternalAgentActivityHistory: %v", err)
	}
	if err := persistence.SaveWorkflowPromptActivityHistory([]config.WorkflowPromptActivityRecord{
		{Timestamp: recent, Surface: config.WorkflowPromptActivitySurfacePulseAssistant, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
		{Timestamp: recent.Add(-time.Minute), Surface: config.WorkflowPromptActivitySurfacePulseMCP, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
		{Timestamp: recent.Add(-2 * time.Minute), Surface: config.WorkflowPromptActivitySurfacePulsePatrol, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
		{Timestamp: recent.Add(-2*time.Minute - time.Second), Surface: config.WorkflowPromptActivitySurfacePatrolControl, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
		{Timestamp: recent.Add(-3 * time.Minute), Surface: config.WorkflowPromptActivitySurfacePatrolAutonomy, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
		{Timestamp: recent.Add(-4 * time.Minute), Surface: config.WorkflowPromptActivitySurfaceProActivation, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
		{Timestamp: recent.Add(-2 * time.Minute), Surface: config.WorkflowPromptActivitySurfacePulseAssistant, PromptName: agentcapabilities.PulseWorkflowPromptTriageFleet},
		{Timestamp: old, Surface: config.WorkflowPromptActivitySurfacePulseAssistant, PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop},
	}); err != nil {
		t.Fatalf("SaveWorkflowPromptActivityHistory: %v", err)
	}

	expiredAt := now.Add(-time.Minute)
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{
			{Scopes: []string{config.ScopeWildcard}, CreatedAt: recent},
			{Scopes: []string{config.ScopeWildcard}, ExpiresAt: &expiredAt, CreatedAt: recent},
		},
	}

	snap := telemetry.Snapshot{PaidLicense: true}
	actions := telemetry.PulseIntelligenceActionSnapshot{
		ActionPlans30d:             4,
		ApprovalRequests30d:        2,
		RejectedActionDecisions30d: 1,
		ApprovedActionDecisions30d: 1,
		ApprovedActionAttempts30d:  1,
		ApprovedActionSuccesses30d: 1,
	}
	applyPulseIntelligenceTelemetrySnapshot(&snap, persistence, cfg, actions, now)

	if snap.PulseIntelligenceAssistantAICalls30d != 3 {
		t.Fatalf("assistant AI calls = %d, want 3", snap.PulseIntelligenceAssistantAICalls30d)
	}
	if snap.PulseIntelligenceAssistantContextAICalls30d != 2 {
		t.Fatalf("assistant context AI calls = %d, want 2", snap.PulseIntelligenceAssistantContextAICalls30d)
	}
	if snap.PulseIntelligenceAssistantToolCalls30d != 5 {
		t.Fatalf("assistant tool calls = %d, want 5", snap.PulseIntelligenceAssistantToolCalls30d)
	}
	if snap.PulseIntelligencePatrolAICalls30d != 1 {
		t.Fatalf("patrol AI calls = %d, want 1", snap.PulseIntelligencePatrolAICalls30d)
	}
	if snap.PulseIntelligencePatrolRuns30d != 2 {
		t.Fatalf("patrol runs = %d, want 2", snap.PulseIntelligencePatrolRuns30d)
	}
	if snap.PulseIntelligencePatrolNewFindings30d != 5 {
		t.Fatalf("patrol new findings = %d, want 5", snap.PulseIntelligencePatrolNewFindings30d)
	}
	if snap.PulseIntelligencePatrolInvestigations30d != 3 {
		t.Fatalf("patrol investigations = %d, want 3", snap.PulseIntelligencePatrolInvestigations30d)
	}
	if snap.PulseIntelligencePatrolResolvedFindings30d != 2 {
		t.Fatalf("patrol resolved findings = %d, want 2", snap.PulseIntelligencePatrolResolvedFindings30d)
	}
	if snap.PulseIntelligencePatrolAutofixes30d != 1 {
		t.Fatalf("patrol autofixes = %d, want 1", snap.PulseIntelligencePatrolAutofixes30d)
	}
	if !snap.PulseIntelligenceExternalAgentEnabled || !snap.PulseIntelligenceExternalAgentUsed30d {
		t.Fatalf("external-agent booleans = enabled %v used %v, want true/true", snap.PulseIntelligenceExternalAgentEnabled, snap.PulseIntelligenceExternalAgentUsed30d)
	}
	if !snap.PulseIntelligenceExternalAgentOperationsLoopReady {
		t.Fatalf("wildcard token should make the Pulse MCP operations loop ready: %#v", snap)
	}
	if !snap.PulseIntelligenceMCPAdapterUsed30d {
		t.Fatalf("MCP adapter use should be true when a recent pulse_mcp activity marker exists: %#v", snap)
	}
	if snap.PulseIntelligenceExternalAgentContextRequests30d != 2 ||
		snap.PulseIntelligenceExternalAgentEventStreamRequests30d != 1 ||
		snap.PulseIntelligenceExternalAgentProvisioningRequests30d != 1 ||
		snap.PulseIntelligenceExternalAgentOperatorStateRequests30d != 1 ||
		snap.PulseIntelligenceExternalAgentFindingRequests30d != 2 ||
		snap.PulseIntelligenceExternalAgentActionRequests30d != 4 {
		t.Fatalf("external-agent class counters not applied: %#v", snap)
	}
	if snap.PulseIntelligenceActionPlans30d != 4 ||
		snap.PulseIntelligenceApprovalRequests30d != 2 ||
		snap.PulseIntelligenceRejectedActionDecisions30d != 1 ||
		snap.PulseIntelligenceApprovedActionDecisions30d != 1 ||
		snap.PulseIntelligenceApprovedActionAttempts30d != 1 ||
		snap.PulseIntelligenceApprovedActionSuccesses30d != 1 {
		t.Fatalf("action telemetry = %#v, want action plans 4, approval requests 2, rejected decisions 1, approved decisions 1, approved attempts 1, approved successes 1", snap)
	}
	if snap.PulseIntelligenceOperationsLoopStarterRequests30d != 6 ||
		snap.PulseIntelligenceAssistantOperationsLoopStarterRequests30d != 1 ||
		snap.PulseIntelligencePatrolOperationsLoopStarterRequests30d != 1 ||
		snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d != 4 ||
		snap.PulseIntelligenceProActivationOperationsLoopStarterRequests30d != 2 ||
		snap.PulseIntelligenceMCPOperationsLoopStarterRequests30d != 1 {
		t.Fatalf("workflow starter telemetry = %#v, want total 6, assistant 1, patrol 1, patrol-control 4, legacy pro activation 2, mcp 1", snap)
	}
	if !snap.PulseIntelligenceLoopConfigured {
		t.Fatalf("Pulse Intelligence loop should be configured when an external-agent token exists: %#v", snap)
	}
	if !snap.PulseIntelligenceLoopActive30d {
		t.Fatalf("Pulse Intelligence loop should be active when recent Assistant, Patrol, external-agent, or action activity exists: %#v", snap)
	}
	if !snap.PulseIntelligenceCompleteOperationsLoop30d {
		t.Fatalf("Pulse Intelligence complete operations loop should be active when Patrol issue evidence, contextual collaboration, and governed action decisions all exist: %#v", snap)
	}
	if !snap.PulseIntelligenceApprovedExecutionLoop30d {
		t.Fatalf("Pulse Intelligence approved-execution loop should be active when the complete loop includes an approved action attempt: %#v", snap)
	}
	if !snap.PulseIntelligenceResolvedOperationsLoop30d {
		t.Fatalf("Pulse Intelligence resolved operations loop should be active when contextual collaboration, approved success, and Patrol resolution all exist: %#v", snap)
	}
	if !snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d {
		t.Fatalf("Patrol-control resolved operations loop should be active when Patrol-control starter, issue evidence, contextual collaboration, approved decision, and verified outcome all exist: %#v", snap)
	}
	if !snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d {
		t.Fatalf("Patrol-control completed operations loop should be active when Patrol-control starter, issue evidence, contextual collaboration, and terminal decision proof all exist: %#v", snap)
	}
	if !snap.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d ||
		!snap.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d {
		t.Fatalf("paid Patrol-control loop cohorts should be active when paid posture and Patrol-control proof coexist: %#v", snap)
	}
	if !snap.PulseIntelligenceProActivationCompletedOperationsLoop30d ||
		!snap.PulseIntelligenceProActivationResolvedOperationsLoop30d ||
		!snap.PulseIntelligenceProActivationPaidCompletedOperationsLoop30d ||
		!snap.PulseIntelligenceProActivationPaidResolvedOperationsLoop30d {
		t.Fatalf("legacy Pro-activation cohorts should mirror Patrol-control proof for continuity: %#v", snap)
	}
	if !snap.PulseIntelligenceGovernedActionActive30d {
		t.Fatalf("Pulse Intelligence governed-action loop should be active when recent action/approval records exist: %#v", snap)
	}
	if !snap.PulseIntelligenceAssistantOperationsLoop30d ||
		!snap.PulseIntelligenceAssistantApprovedExecutionLoop30d ||
		!snap.PulseIntelligenceAssistantApprovedActionSuccessLoop30d ||
		!snap.PulseIntelligenceAssistantResolvedOperationsLoop30d {
		t.Fatalf("Assistant source-specific loop evidence should be active when Assistant collaboration completes the loop: %#v", snap)
	}
	if !snap.PulseIntelligenceExternalAgentOperationsLoop30d ||
		!snap.PulseIntelligenceExternalAgentApprovedExecutionLoop30d ||
		!snap.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d ||
		!snap.PulseIntelligenceExternalAgentResolvedOperationsLoop30d {
		t.Fatalf("external-agent source-specific loop evidence should be active when external-agent collaboration completes the loop: %#v", snap)
	}
	if !snap.PulseIntelligenceMCPAdapterOperationsLoop30d ||
		!snap.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d ||
		!snap.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d ||
		!snap.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d {
		t.Fatalf("MCP adapter source-specific loop evidence should be active when pulse-mcp collaboration completes the loop: %#v", snap)
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_OperationsLoopStarterIsActiveNotComplete(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveWorkflowPromptActivityHistory([]config.WorkflowPromptActivityRecord{{
		Timestamp:  now.Add(-time.Hour),
		Surface:    config.WorkflowPromptActivitySurfacePulseAssistant,
		PromptName: agentcapabilities.PulseWorkflowPromptOperationsLoop,
	}}); err != nil {
		t.Fatalf("SaveWorkflowPromptActivityHistory: %v", err)
	}

	snap := telemetry.Snapshot{AIEnabled: true}
	applyPulseIntelligenceTelemetrySnapshot(&snap, persistence, nil, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if snap.PulseIntelligenceOperationsLoopStarterRequests30d != 1 ||
		snap.PulseIntelligenceAssistantOperationsLoopStarterRequests30d != 1 ||
		snap.PulseIntelligencePatrolOperationsLoopStarterRequests30d != 0 ||
		snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d != 0 ||
		snap.PulseIntelligenceProActivationOperationsLoopStarterRequests30d != 0 ||
		snap.PulseIntelligenceMCPOperationsLoopStarterRequests30d != 0 {
		t.Fatalf("workflow starter telemetry = %#v, want one assistant operation-loop starter", snap)
	}
	if !snap.PulseIntelligenceLoopConfigured || !snap.PulseIntelligenceLoopActive30d {
		t.Fatalf("starter use should configure and activate the Pulse Intelligence loop when AI is enabled: %#v", snap)
	}
	if snap.PulseIntelligenceCompleteOperationsLoop30d ||
		snap.PulseIntelligenceApprovedExecutionLoop30d ||
		snap.PulseIntelligenceResolvedOperationsLoop30d ||
		snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d ||
		snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d ||
		snap.PulseIntelligenceProActivationCompletedOperationsLoop30d ||
		snap.PulseIntelligenceProActivationResolvedOperationsLoop30d ||
		snap.PulseIntelligenceGovernedActionActive30d ||
		snap.PulseIntelligenceAssistantOperationsLoop30d ||
		snap.PulseIntelligenceAssistantApprovedExecutionLoop30d ||
		snap.PulseIntelligenceAssistantApprovedActionSuccessLoop30d ||
		snap.PulseIntelligenceAssistantResolvedOperationsLoop30d ||
		snap.PulseIntelligenceExternalAgentOperationsLoop30d ||
		snap.PulseIntelligenceExternalAgentApprovedExecutionLoop30d ||
		snap.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d ||
		snap.PulseIntelligenceExternalAgentResolvedOperationsLoop30d ||
		snap.PulseIntelligenceMCPAdapterOperationsLoop30d ||
		snap.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d ||
		snap.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d ||
		snap.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d {
		t.Fatalf("starter-only activity must not satisfy completed, approved, resolved, or governed action loops: %#v", snap)
	}
}

func TestApplyPulseIntelligenceAdoptionSnapshot_PatrolControlResolvedRequiresStatusProof(t *testing.T) {
	base := telemetry.Snapshot{
		PulseIntelligencePatrolControlOperationsLoopStarterRequests30d: 1,
		PulseIntelligencePatrolNewFindings30d:                          1,
		PulseIntelligenceAssistantContextAICalls30d:                    1,
		PulseIntelligenceApprovedActionDecisions30d:                    1,
		PulseIntelligenceApprovedActionSuccesses30d:                    1,
		PulseIntelligenceExternalAgentEnabled:                          true,
		PulseIntelligenceExternalAgentOperationsLoopReady:              true,
	}

	tests := []struct {
		name string
		mut  func(*telemetry.Snapshot)
		want bool
	}{
		{
			name: "complete status proof",
			want: true,
		},
		{
			name: "missing Patrol-control starter",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d = 0
			},
		},
		{
			name: "missing patrol issue evidence",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligencePatrolNewFindings30d = 0
			},
		},
		{
			name: "missing contextual collaboration",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceAssistantContextAICalls30d = 0
			},
		},
		{
			name: "approved success without approved decision",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceApprovedActionDecisions30d = 0
			},
		},
		{
			name: "approved decision without verified outcome",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceApprovedActionSuccesses30d = 0
			},
		},
		{
			name: "MCP readiness is not required",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceExternalAgentOperationsLoopReady = false
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := base
			if tt.mut != nil {
				tt.mut(&snap)
			}

			applyPulseIntelligenceAdoptionSnapshot(&snap)

			if snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d != tt.want {
				t.Fatalf("PulseIntelligencePatrolControlResolvedOperationsLoop30d = %v, want %v: %#v",
					snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d,
					tt.want,
					snap,
				)
			}
			if snap.PulseIntelligenceProActivationResolvedOperationsLoop30d != snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d {
				t.Fatalf("legacy Pro-activation resolved loop should mirror Patrol-control proof: %#v", snap)
			}
		})
	}
}

func TestApplyPulseIntelligenceAdoptionSnapshot_PatrolControlCompletedIncludesRejectedDecision(t *testing.T) {
	base := telemetry.Snapshot{
		PulseIntelligencePatrolControlOperationsLoopStarterRequests30d: 1,
		PulseIntelligencePatrolNewFindings30d:                          1,
		PulseIntelligenceAssistantContextAICalls30d:                    1,
		PulseIntelligenceRejectedActionDecisions30d:                    1,
		PulseIntelligenceExternalAgentEnabled:                          true,
		PulseIntelligenceExternalAgentOperationsLoopReady:              true,
	}

	tests := []struct {
		name string
		mut  func(*telemetry.Snapshot)
		want bool
	}{
		{
			name: "rejected governed decision completes the Patrol-control loop",
			want: true,
		},
		{
			name: "approved decision needs verified outcome",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceRejectedActionDecisions30d = 0
				snap.PulseIntelligenceApprovedActionDecisions30d = 1
			},
		},
		{
			name: "approved decision with verified outcome completes the Patrol-control loop",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceRejectedActionDecisions30d = 0
				snap.PulseIntelligenceApprovedActionDecisions30d = 1
				snap.PulseIntelligenceApprovedActionSuccesses30d = 1
			},
			want: true,
		},
		{
			name: "starter is required",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d = 0
			},
		},
		{
			name: "patrol issue evidence is required",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligencePatrolNewFindings30d = 0
			},
		},
		{
			name: "contextual collaboration is required",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceAssistantContextAICalls30d = 0
			},
		},
		{
			name: "MCP readiness is not required",
			mut: func(snap *telemetry.Snapshot) {
				snap.PulseIntelligenceExternalAgentOperationsLoopReady = false
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := base
			if tt.mut != nil {
				tt.mut(&snap)
			}

			applyPulseIntelligenceAdoptionSnapshot(&snap)

			if snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d != tt.want {
				t.Fatalf("PulseIntelligencePatrolControlCompletedOperationsLoop30d = %v, want %v: %#v",
					snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d,
					tt.want,
					snap,
				)
			}
			if snap.PulseIntelligenceProActivationCompletedOperationsLoop30d != snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d {
				t.Fatalf("legacy Pro-activation completed loop should mirror Patrol-control proof: %#v", snap)
			}
			if snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d && snap.PulseIntelligenceRejectedActionDecisions30d > 0 {
				t.Fatalf("rejected terminal decision must not set resolved-loop proof: %#v", snap)
			}
		})
	}
}

func TestApplyPulseIntelligenceAdoptionSnapshot_PaidPatrolControlCohortsRequirePaidLicense(t *testing.T) {
	verified := telemetry.Snapshot{
		PulseIntelligencePatrolControlOperationsLoopStarterRequests30d: 1,
		PulseIntelligencePatrolNewFindings30d:                          1,
		PulseIntelligenceAssistantContextAICalls30d:                    1,
		PulseIntelligenceApprovedActionDecisions30d:                    1,
		PulseIntelligenceApprovedActionSuccesses30d:                    1,
		PulseIntelligenceExternalAgentEnabled:                          true,
		PulseIntelligenceExternalAgentOperationsLoopReady:              true,
	}
	rejected := telemetry.Snapshot{
		PulseIntelligencePatrolControlOperationsLoopStarterRequests30d: 1,
		PulseIntelligencePatrolNewFindings30d:                          1,
		PulseIntelligenceAssistantContextAICalls30d:                    1,
		PulseIntelligenceRejectedActionDecisions30d:                    1,
		PulseIntelligenceExternalAgentEnabled:                          true,
		PulseIntelligenceExternalAgentOperationsLoopReady:              true,
	}

	tests := []struct {
		name          string
		snapshot      telemetry.Snapshot
		wantCompleted bool
		wantResolved  bool
	}{
		{
			name:          "paid verified loop records paid completed and paid resolved cohorts",
			snapshot:      func() telemetry.Snapshot { s := verified; s.PaidLicense = true; return s }(),
			wantCompleted: true,
			wantResolved:  true,
		},
		{
			name:          "paid rejected terminal loop records paid completed only",
			snapshot:      func() telemetry.Snapshot { s := rejected; s.PaidLicense = true; return s }(),
			wantCompleted: true,
		},
		{
			name:     "free verified loop does not record paid cohorts",
			snapshot: verified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := tt.snapshot

			applyPulseIntelligenceAdoptionSnapshot(&snap)

			if snap.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d != tt.wantCompleted {
				t.Fatalf("PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d = %v, want %v: %#v",
					snap.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d,
					tt.wantCompleted,
					snap,
				)
			}
			if snap.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d != tt.wantResolved {
				t.Fatalf("PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d = %v, want %v: %#v",
					snap.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d,
					tt.wantResolved,
					snap,
				)
			}
			if snap.PulseIntelligenceProActivationPaidCompletedOperationsLoop30d != snap.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d ||
				snap.PulseIntelligenceProActivationPaidResolvedOperationsLoop30d != snap.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d {
				t.Fatalf("legacy Pro-activation paid cohorts should mirror Patrol-control paid cohorts: %#v", snap)
			}
		})
	}
}

func TestPulseIntelligenceExternalAgentActivityRecognizesContentFreeCollaboration(t *testing.T) {
	if pulseIntelligenceExternalAgentActivity(nil) {
		t.Fatal("nil snapshot should not report external-agent activity")
	}
	if !pulseIntelligenceExternalAgentActivity(&telemetry.Snapshot{
		PulseIntelligenceExternalAgentContextRequests30d: 1,
	}) {
		t.Fatal("context request count should report external-agent collaboration")
	}
	if !pulseIntelligenceExternalAgentActivity(&telemetry.Snapshot{
		PulseIntelligenceMCPAdapterUsed30d: true,
	}) {
		t.Fatal("MCP adapter marker should report external-agent collaboration")
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_DoesNotPromoteOrdinaryAIToken(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	lastUsed := now.Add(-time.Hour)
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{
			{Scopes: []string{config.ScopeAIChat}, LastUsedAt: &lastUsed, CreatedAt: now.Add(-2 * time.Hour)},
		},
	}

	snap := telemetry.Snapshot{}
	applyPulseIntelligenceTelemetrySnapshot(&snap, nil, cfg, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if snap.PulseIntelligenceExternalAgentEnabled || snap.PulseIntelligenceExternalAgentUsed30d {
		t.Fatalf("ordinary AI token should not be treated as external-agent capable: %#v", snap)
	}
	if snap.PulseIntelligenceLoopConfigured || snap.PulseIntelligenceLoopActive30d ||
		snap.PulseIntelligenceCompleteOperationsLoop30d ||
		snap.PulseIntelligenceApprovedExecutionLoop30d ||
		snap.PulseIntelligenceResolvedOperationsLoop30d ||
		snap.PulseIntelligenceGovernedActionActive30d {
		t.Fatalf("ordinary AI token should not set Pulse Intelligence adoption state: %#v", snap)
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_PromotesReadOnlyMCPTokenAsConfiguredOnly(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{
			{Scopes: []string{config.ScopeMonitoringRead}, CreatedAt: now.Add(-2 * time.Hour)},
			{Scopes: []string{config.ScopeSettingsWrite}, ExpiresAt: &expiredAt, CreatedAt: now.Add(-3 * time.Hour)},
		},
	}

	snap := telemetry.Snapshot{}
	applyPulseIntelligenceTelemetrySnapshot(&snap, config.NewConfigPersistence(t.TempDir()), cfg, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if !snap.PulseIntelligenceExternalAgentEnabled {
		t.Fatalf("read-only MCP-capable token should configure the external-agent surface: %#v", snap)
	}
	if snap.PulseIntelligenceExternalAgentOperationsLoopReady {
		t.Fatalf("read-only MCP-capable token must not satisfy full operations-loop readiness: %#v", snap)
	}
	if snap.PulseIntelligenceExternalAgentUsed30d {
		t.Fatalf("configured MCP-capable token should not count as external-agent activity without route activity: %#v", snap)
	}
	if !snap.PulseIntelligenceLoopConfigured ||
		snap.PulseIntelligenceLoopActive30d ||
		snap.PulseIntelligenceCompleteOperationsLoop30d ||
		snap.PulseIntelligenceApprovedExecutionLoop30d ||
		snap.PulseIntelligenceResolvedOperationsLoop30d {
		t.Fatalf("configured but unused MCP-capable token should configure, not activate, the Pulse Intelligence loop: %#v", snap)
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_PromotesFullMCPTokenAsOperationsLoopReady(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{
			{
				Scopes:    pulseIntelligenceExternalAgentSurfaceScopes(agentcapabilities.CanonicalManifest()),
				CreatedAt: now.Add(-2 * time.Hour),
			},
		},
	}

	snap := telemetry.Snapshot{}
	applyPulseIntelligenceTelemetrySnapshot(&snap, config.NewConfigPersistence(t.TempDir()), cfg, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if !snap.PulseIntelligenceExternalAgentEnabled ||
		!snap.PulseIntelligenceExternalAgentOperationsLoopReady {
		t.Fatalf("full Pulse MCP token should configure and ready the operations loop: %#v", snap)
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_DoesNotCombinePartialMCPTokensForOperationsLoopReadiness(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	surfaceScopes := pulseIntelligenceExternalAgentSurfaceScopes(agentcapabilities.CanonicalManifest())
	if len(surfaceScopes) < 2 {
		t.Fatalf("Pulse MCP operations loop should require multiple scopes, got %v", surfaceScopes)
	}
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{
			{Scopes: []string{surfaceScopes[0]}, CreatedAt: now.Add(-2 * time.Hour)},
			{Scopes: []string{surfaceScopes[1]}, CreatedAt: now.Add(-time.Hour)},
		},
	}

	snap := telemetry.Snapshot{}
	applyPulseIntelligenceTelemetrySnapshot(&snap, config.NewConfigPersistence(t.TempDir()), cfg, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if !snap.PulseIntelligenceExternalAgentEnabled {
		t.Fatalf("partial Pulse MCP tokens should still configure the external-agent surface: %#v", snap)
	}
	if snap.PulseIntelligenceExternalAgentOperationsLoopReady {
		t.Fatalf("split partial Pulse MCP tokens must not satisfy full operations-loop readiness: %#v", snap)
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_DoesNotInferExternalAgentUsageFromGenericTokenLastUse(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)
	lastUsed := now.Add(-time.Hour)
	cfg := &config.Config{
		APITokens: []config.APITokenRecord{
			{Scopes: []string{config.ScopeWildcard}, LastUsedAt: &lastUsed, CreatedAt: now.Add(-2 * time.Hour)},
		},
	}

	snap := telemetry.Snapshot{}
	applyPulseIntelligenceTelemetrySnapshot(&snap, config.NewConfigPersistence(t.TempDir()), cfg, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if !snap.PulseIntelligenceExternalAgentEnabled {
		t.Fatalf("external-agent capable token should configure the surface: %#v", snap)
	}
	if !snap.PulseIntelligenceExternalAgentOperationsLoopReady {
		t.Fatalf("wildcard token should satisfy full operations-loop readiness: %#v", snap)
	}
	if snap.PulseIntelligenceExternalAgentUsed30d {
		t.Fatalf("generic token last-use should not count as external-agent activity without route activity: %#v", snap)
	}
	if !snap.PulseIntelligenceLoopConfigured ||
		snap.PulseIntelligenceLoopActive30d ||
		snap.PulseIntelligenceCompleteOperationsLoop30d ||
		snap.PulseIntelligenceApprovedExecutionLoop30d ||
		snap.PulseIntelligenceResolvedOperationsLoop30d {
		t.Fatalf("configured but unused external-agent surface should not activate the 30-day loop: %#v", snap)
	}
}

func TestApplyPulseIntelligenceTelemetrySnapshot_DerivesConfiguredWithoutRecentActivity(t *testing.T) {
	now := time.Date(2026, 6, 20, 9, 30, 0, 0, time.UTC)

	snap := telemetry.Snapshot{
		AIEnabled:        true,
		PatrolEnabled:    true,
		AIActionsEnabled: true,
	}
	applyPulseIntelligenceTelemetrySnapshot(&snap, nil, nil, telemetry.PulseIntelligenceActionSnapshot{}, now)

	if !snap.PulseIntelligenceLoopConfigured {
		t.Fatalf("enabled Assistant/Patrol/action settings should configure the Pulse Intelligence loop: %#v", snap)
	}
	if snap.PulseIntelligenceLoopActive30d {
		t.Fatalf("configured settings without recent usage should not count as 30-day activity: %#v", snap)
	}
	if snap.PulseIntelligenceCompleteOperationsLoop30d {
		t.Fatalf("configured settings without recent usage should not count as complete operations-loop activity: %#v", snap)
	}
	if snap.PulseIntelligenceApprovedExecutionLoop30d {
		t.Fatalf("configured settings without recent usage should not count as approved-execution loop activity: %#v", snap)
	}
	if snap.PulseIntelligenceResolvedOperationsLoop30d {
		t.Fatalf("configured settings without recent usage should not count as resolved operations-loop activity: %#v", snap)
	}
	if snap.PulseIntelligenceGovernedActionActive30d {
		t.Fatalf("enabled action settings without action audit records should not count as governed-action activity: %#v", snap)
	}
}

func TestApplyPulseIntelligenceAdoptionSnapshot_SourceSpecificLoopEvidence(t *testing.T) {
	tests := []struct {
		name                  string
		snap                  telemetry.Snapshot
		wantAssistantLoop     bool
		wantAssistantResolved bool
		wantExternalLoop      bool
		wantExternalResolved  bool
		wantMCPLoop           bool
		wantMCPResolved       bool
	}{
		{
			name: "assistant collaboration stays first party",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:       1,
				PulseIntelligencePatrolResolvedFindings30d:  1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceApprovalRequests30d:        1,
				PulseIntelligenceApprovedActionDecisions30d: 1,
				PulseIntelligenceApprovedActionAttempts30d:  1,
				PulseIntelligenceApprovedActionSuccesses30d: 1,
			},
			wantAssistantLoop:     true,
			wantAssistantResolved: true,
		},
		{
			name: "external agent activity stays outside mcp subset",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:            1,
				PulseIntelligencePatrolResolvedFindings30d:       1,
				PulseIntelligenceExternalAgentContextRequests30d: 1,
				PulseIntelligenceApprovalRequests30d:             1,
				PulseIntelligenceApprovedActionDecisions30d:      1,
				PulseIntelligenceApprovedActionAttempts30d:       1,
				PulseIntelligenceApprovedActionSuccesses30d:      1,
			},
			wantExternalLoop:     true,
			wantExternalResolved: true,
		},
		{
			name: "pulse mcp is the mcp subset and external-agent umbrella",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:       1,
				PulseIntelligencePatrolResolvedFindings30d:  1,
				PulseIntelligenceMCPAdapterUsed30d:          true,
				PulseIntelligenceApprovalRequests30d:        1,
				PulseIntelligenceApprovedActionDecisions30d: 1,
				PulseIntelligenceApprovedActionAttempts30d:  1,
				PulseIntelligenceApprovedActionSuccesses30d: 1,
			},
			wantExternalLoop:     true,
			wantExternalResolved: true,
			wantMCPLoop:          true,
			wantMCPResolved:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := tt.snap
			applyPulseIntelligenceAdoptionSnapshot(&snap)
			if snap.PulseIntelligenceAssistantOperationsLoop30d != tt.wantAssistantLoop ||
				snap.PulseIntelligenceAssistantApprovedExecutionLoop30d != tt.wantAssistantLoop ||
				snap.PulseIntelligenceAssistantApprovedActionSuccessLoop30d != tt.wantAssistantResolved ||
				snap.PulseIntelligenceAssistantResolvedOperationsLoop30d != tt.wantAssistantResolved {
				t.Fatalf("Assistant loop evidence mismatch: %#v", snap)
			}
			if snap.PulseIntelligenceExternalAgentOperationsLoop30d != tt.wantExternalLoop ||
				snap.PulseIntelligenceExternalAgentApprovedExecutionLoop30d != tt.wantExternalLoop ||
				snap.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d != tt.wantExternalResolved ||
				snap.PulseIntelligenceExternalAgentResolvedOperationsLoop30d != tt.wantExternalResolved {
				t.Fatalf("external-agent loop evidence mismatch: %#v", snap)
			}
			if snap.PulseIntelligenceMCPAdapterOperationsLoop30d != tt.wantMCPLoop ||
				snap.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d != tt.wantMCPLoop ||
				snap.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d != tt.wantMCPResolved ||
				snap.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d != tt.wantMCPResolved {
				t.Fatalf("MCP adapter loop evidence mismatch: %#v", snap)
			}
		})
	}
}

func TestApplyPulseIntelligenceAdoptionSnapshot_CompleteLoopRequiresIssueEvidenceCollaborationAndDecision(t *testing.T) {
	tests := []struct {
		name         string
		snap         telemetry.Snapshot
		wantActive   bool
		wantComplete bool
		wantApproved bool
		wantResolved bool
	}{
		{
			name: "generic assistant and governed action is active but incomplete",
			snap: telemetry.Snapshot{
				PulseIntelligenceAssistantAICalls30d: 3,
				PulseIntelligenceActionPlans30d:      1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol and governed action without contextual collaboration is incomplete",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolRuns30d:  1,
				PulseIntelligenceActionPlans30d: 1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol and contextual collaboration without governed action is incomplete",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolRuns30d:              1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol run assistant tool collaboration and approval request is active but incomplete",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolRuns30d:         1,
				PulseIntelligenceAssistantToolCalls30d: 2,
				PulseIntelligenceApprovalRequests30d:   1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol issue assistant collaboration and rejected action decision completes governed action only",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:       1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceRejectedActionDecisions30d: 1,
			},
			wantActive:   true,
			wantComplete: true,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol issue assistant collaboration and approved action decision completes without execution",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:       1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceApprovedActionDecisions30d: 1,
			},
			wantActive:   true,
			wantComplete: true,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "generic patrol run with approved action attempt remains incomplete without issue evidence",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolRuns30d:             1,
				PulseIntelligenceExternalAgentUsed30d:      true,
				PulseIntelligenceApprovedActionAttempts30d: 1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol issue external-agent collaboration and governed action completes the loop",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:      1,
				PulseIntelligenceExternalAgentUsed30d:      true,
				PulseIntelligenceApprovedActionAttempts30d: 1,
			},
			wantActive:   true,
			wantComplete: true,
			wantApproved: true,
			wantResolved: false,
		},
		{
			name: "patrol issue external-agent capability activity and governed action completes the loop",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:            1,
				PulseIntelligenceExternalAgentContextRequests30d: 1,
				PulseIntelligenceApprovedActionAttempts30d:       1,
			},
			wantActive:   true,
			wantComplete: true,
			wantApproved: true,
			wantResolved: false,
		},
		{
			name: "patrol issue MCP adapter collaboration and governed action completes the loop",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolNewFindings30d:      1,
				PulseIntelligenceMCPAdapterUsed30d:         true,
				PulseIntelligenceApprovedActionAttempts30d: 1,
			},
			wantActive:   true,
			wantComplete: true,
			wantApproved: true,
			wantResolved: false,
		},
		{
			name: "external-agent capability activity without patrol is active but incomplete",
			snap: telemetry.Snapshot{
				PulseIntelligenceExternalAgentActionRequests30d: 1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol AI call assistant context and approval request is active but incomplete",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolAICalls30d:           1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceApprovalRequests30d:        1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "patrol investigation assistant context and approval request waits for a decision",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolInvestigations30d:    1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceApprovalRequests30d:        1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
		{
			name: "resolved operations loop requires patrol resolution collaboration and approved success",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolResolvedFindings30d:  1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceApprovedActionAttempts30d:  1,
				PulseIntelligenceApprovedActionSuccesses30d: 1,
			},
			wantActive:   true,
			wantComplete: true,
			wantApproved: true,
			wantResolved: true,
		},
		{
			name: "patrol resolution without governed action decision does not complete or resolve the operations loop",
			snap: telemetry.Snapshot{
				PulseIntelligencePatrolResolvedFindings30d:  1,
				PulseIntelligenceAssistantContextAICalls30d: 1,
				PulseIntelligenceApprovalRequests30d:        1,
			},
			wantActive:   true,
			wantComplete: false,
			wantApproved: false,
			wantResolved: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := tt.snap
			applyPulseIntelligenceAdoptionSnapshot(&snap)
			if snap.PulseIntelligenceLoopActive30d != tt.wantActive {
				t.Fatalf("PulseIntelligenceLoopActive30d = %v, want %v: %#v",
					snap.PulseIntelligenceLoopActive30d,
					tt.wantActive,
					snap,
				)
			}
			if snap.PulseIntelligenceCompleteOperationsLoop30d != tt.wantComplete {
				t.Fatalf("PulseIntelligenceCompleteOperationsLoop30d = %v, want %v: %#v",
					snap.PulseIntelligenceCompleteOperationsLoop30d,
					tt.wantComplete,
					snap,
				)
			}
			if snap.PulseIntelligenceApprovedExecutionLoop30d != tt.wantApproved {
				t.Fatalf("PulseIntelligenceApprovedExecutionLoop30d = %v, want %v: %#v",
					snap.PulseIntelligenceApprovedExecutionLoop30d,
					tt.wantApproved,
					snap,
				)
			}
			if snap.PulseIntelligenceResolvedOperationsLoop30d != tt.wantResolved {
				t.Fatalf("PulseIntelligenceResolvedOperationsLoop30d = %v, want %v: %#v",
					snap.PulseIntelligenceResolvedOperationsLoop30d,
					tt.wantResolved,
					snap,
				)
			}
		})
	}
}
