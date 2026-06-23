package server

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
)

func applyPulseIntelligenceTelemetrySnapshot(
	snap *telemetry.Snapshot,
	persistence *config.ConfigPersistence,
	currentCfg *config.Config,
	actionSnapshot telemetry.PulseIntelligenceActionSnapshot,
	now time.Time,
) {
	if snap == nil {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)

	applyPulseIntelligenceAIUsageSnapshot(snap, persistence, since)
	applyPulseIntelligencePatrolRunSnapshot(snap, persistence, since)
	applyPulseIntelligenceFindingSnapshot(snap, persistence, since)
	applyPulseIntelligenceExternalAgentSnapshot(snap, persistence, currentCfg, since, now)
	applyPulseIntelligenceWorkflowPromptSnapshot(snap, persistence, since)

	snap.PulseIntelligenceActionPlans30d = actionSnapshot.ActionPlans30d
	snap.PulseIntelligenceApprovalRequests30d = actionSnapshot.ApprovalRequests30d
	snap.PulseIntelligenceRejectedActionDecisions30d = actionSnapshot.RejectedActionDecisions30d
	snap.PulseIntelligenceApprovedActionDecisions30d = actionSnapshot.ApprovedActionDecisions30d
	snap.PulseIntelligenceApprovedActionAttempts30d = actionSnapshot.ApprovedActionAttempts30d
	snap.PulseIntelligenceApprovedActionSuccesses30d = actionSnapshot.ApprovedActionSuccesses30d

	applyPulseIntelligenceAdoptionSnapshot(snap)
}

func applyPulseIntelligenceAdoptionSnapshot(snap *telemetry.Snapshot) {
	if snap == nil {
		return
	}

	snap.PulseIntelligenceLoopConfigured = snap.AIEnabled ||
		snap.PatrolEnabled ||
		snap.AIActionsEnabled ||
		snap.PulseIntelligenceExternalAgentEnabled

	snap.PulseIntelligenceGovernedActionActive30d =
		snap.PulseIntelligenceActionPlans30d > 0 ||
			snap.PulseIntelligenceApprovalRequests30d > 0 ||
			snap.PulseIntelligenceRejectedActionDecisions30d > 0 ||
			snap.PulseIntelligenceApprovedActionDecisions30d > 0 ||
			snap.PulseIntelligenceApprovedActionAttempts30d > 0

	patrolActive := snap.PulseIntelligencePatrolAICalls30d > 0 ||
		snap.PulseIntelligencePatrolRuns30d > 0 ||
		snap.PulseIntelligencePatrolNewFindings30d > 0 ||
		snap.PulseIntelligencePatrolInvestigations30d > 0 ||
		snap.PulseIntelligencePatrolResolvedFindings30d > 0 ||
		snap.PulseIntelligencePatrolAutofixes30d > 0
	patrolIssueEvidenceCount := snap.PulseIntelligencePatrolNewFindings30d +
		snap.PulseIntelligencePatrolInvestigations30d +
		snap.PulseIntelligencePatrolResolvedFindings30d +
		snap.PulseIntelligencePatrolAutofixes30d
	patrolIssueEvidenceActive := patrolIssueEvidenceCount > 0
	assistantCollaborationCount := snap.PulseIntelligenceAssistantContextAICalls30d +
		snap.PulseIntelligenceAssistantToolCalls30d
	assistantCollaborationActive := assistantCollaborationCount > 0
	externalAgentCollaborationActive := pulseIntelligenceExternalAgentActivity(snap)
	externalAgentCollaborationCount := 0
	if externalAgentCollaborationActive {
		externalAgentCollaborationCount = 1
	}
	mcpAdapterCollaborationActive := snap.PulseIntelligenceMCPAdapterUsed30d
	contextualCollaborationCount := assistantCollaborationCount + externalAgentCollaborationCount
	contextualCollaborationActive := contextualCollaborationCount > 0
	governedActionDecisionActive := snap.PulseIntelligenceRejectedActionDecisions30d > 0 ||
		snap.PulseIntelligenceApprovedActionDecisions30d > 0 ||
		snap.PulseIntelligenceApprovedActionAttempts30d > 0
	approvedExecutionActive := snap.PulseIntelligenceApprovedActionAttempts30d > 0
	approvedSuccessActive := snap.PulseIntelligenceApprovedActionSuccesses30d > 0
	resolutionOutcomeActive := snap.PulseIntelligencePatrolResolvedFindings30d > 0
	patrolControlProof := telemetry.ClassifyPulseIntelligencePatrolControlProof(telemetry.PulseIntelligencePatrolControlProofInput{
		PatrolControlStarterCount:    snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d,
		PatrolIssueEvidenceCount:     patrolIssueEvidenceCount,
		ContextualCollaborationCount: contextualCollaborationCount,
		ApprovedDecisionCount:        snap.PulseIntelligenceApprovedActionDecisions30d,
		RejectedDecisionCount:        snap.PulseIntelligenceRejectedActionDecisions30d,
		VerifiedOutcomeCount:         snap.PulseIntelligenceApprovedActionSuccesses30d,
	})
	snap.PulseIntelligenceCompleteOperationsLoop30d =
		patrolIssueEvidenceActive &&
			contextualCollaborationActive &&
			governedActionDecisionActive
	snap.PulseIntelligenceApprovedExecutionLoop30d =
		patrolIssueEvidenceActive &&
			contextualCollaborationActive &&
			approvedExecutionActive
	snap.PulseIntelligenceResolvedOperationsLoop30d =
		resolutionOutcomeActive &&
			contextualCollaborationActive &&
			approvedSuccessActive
	snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d = patrolControlProof.Completed
	snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d = patrolControlProof.Resolved
	snap.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d =
		snap.PaidLicense && snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d
	snap.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d =
		snap.PaidLicense && snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d
	// Legacy Pro activation cohort fields mirror Patrol control for continuity.
	snap.PulseIntelligenceProActivationCompletedOperationsLoop30d = snap.PulseIntelligencePatrolControlCompletedOperationsLoop30d
	snap.PulseIntelligenceProActivationResolvedOperationsLoop30d = snap.PulseIntelligencePatrolControlResolvedOperationsLoop30d
	snap.PulseIntelligenceProActivationPaidCompletedOperationsLoop30d =
		snap.PulseIntelligencePatrolControlPaidCompletedOperationsLoop30d
	snap.PulseIntelligenceProActivationPaidResolvedOperationsLoop30d =
		snap.PulseIntelligencePatrolControlPaidResolvedOperationsLoop30d
	snap.PulseIntelligenceAssistantOperationsLoop30d =
		patrolIssueEvidenceActive &&
			assistantCollaborationActive &&
			governedActionDecisionActive
	snap.PulseIntelligenceAssistantApprovedExecutionLoop30d =
		patrolIssueEvidenceActive &&
			assistantCollaborationActive &&
			approvedExecutionActive
	snap.PulseIntelligenceAssistantApprovedActionSuccessLoop30d =
		patrolIssueEvidenceActive &&
			assistantCollaborationActive &&
			approvedSuccessActive
	snap.PulseIntelligenceAssistantResolvedOperationsLoop30d =
		resolutionOutcomeActive &&
			assistantCollaborationActive &&
			approvedSuccessActive
	snap.PulseIntelligenceExternalAgentOperationsLoop30d =
		patrolIssueEvidenceActive &&
			externalAgentCollaborationActive &&
			governedActionDecisionActive
	snap.PulseIntelligenceExternalAgentApprovedExecutionLoop30d =
		patrolIssueEvidenceActive &&
			externalAgentCollaborationActive &&
			approvedExecutionActive
	snap.PulseIntelligenceExternalAgentApprovedActionSuccessLoop30d =
		patrolIssueEvidenceActive &&
			externalAgentCollaborationActive &&
			approvedSuccessActive
	snap.PulseIntelligenceExternalAgentResolvedOperationsLoop30d =
		resolutionOutcomeActive &&
			externalAgentCollaborationActive &&
			approvedSuccessActive
	snap.PulseIntelligenceMCPAdapterOperationsLoop30d =
		patrolIssueEvidenceActive &&
			mcpAdapterCollaborationActive &&
			governedActionDecisionActive
	snap.PulseIntelligenceMCPAdapterApprovedExecutionLoop30d =
		patrolIssueEvidenceActive &&
			mcpAdapterCollaborationActive &&
			approvedExecutionActive
	snap.PulseIntelligenceMCPAdapterApprovedActionSuccessLoop30d =
		patrolIssueEvidenceActive &&
			mcpAdapterCollaborationActive &&
			approvedSuccessActive
	snap.PulseIntelligenceMCPAdapterResolvedOperationsLoop30d =
		resolutionOutcomeActive &&
			mcpAdapterCollaborationActive &&
			approvedSuccessActive

	snap.PulseIntelligenceLoopActive30d =
		snap.PulseIntelligenceAssistantAICalls30d > 0 ||
			snap.PulseIntelligenceOperationsLoopStarterRequests30d > 0 ||
			patrolActive ||
			pulseIntelligenceExternalAgentActivity(snap) ||
			snap.PulseIntelligenceGovernedActionActive30d
}

func pulseIntelligenceExternalAgentActivity(snap *telemetry.Snapshot) bool {
	if snap == nil {
		return false
	}
	evidence := telemetry.PulseIntelligenceExternalAgentEvidence{
		Used:                  snap.PulseIntelligenceExternalAgentUsed30d,
		MCPAdapterUsed:        snap.PulseIntelligenceMCPAdapterUsed30d,
		ContextRequests:       snap.PulseIntelligenceExternalAgentContextRequests30d,
		EventStreamRequests:   snap.PulseIntelligenceExternalAgentEventStreamRequests30d,
		ProvisioningRequests:  snap.PulseIntelligenceExternalAgentProvisioningRequests30d,
		OperatorStateRequests: snap.PulseIntelligenceExternalAgentOperatorStateRequests30d,
		FindingRequests:       snap.PulseIntelligenceExternalAgentFindingRequests30d,
		ActionRequests:        snap.PulseIntelligenceExternalAgentActionRequests30d,
	}
	return evidence.CollaborationActive()
}

func applyPulseIntelligenceAIUsageSnapshot(snap *telemetry.Snapshot, persistence *config.ConfigPersistence, since time.Time) {
	if snap == nil || persistence == nil {
		return
	}
	history, err := persistence.LoadAIUsageHistory()
	if err != nil || history == nil {
		return
	}
	evidence := telemetry.PulseIntelligenceAIUsageEvidenceFromHistory(history, since)
	snap.PulseIntelligenceAssistantAICalls30d = evidence.AssistantAICalls
	snap.PulseIntelligenceAssistantContextAICalls30d = evidence.AssistantContextAICalls
	snap.PulseIntelligenceAssistantToolCalls30d = evidence.AssistantToolCalls
	snap.PulseIntelligencePatrolAICalls30d = evidence.PatrolAICalls
}

func applyPulseIntelligencePatrolRunSnapshot(snap *telemetry.Snapshot, persistence *config.ConfigPersistence, since time.Time) {
	if snap == nil || persistence == nil {
		return
	}
	history, err := persistence.LoadPatrolRunHistory()
	if err != nil || history == nil {
		return
	}
	for _, run := range history.Runs {
		observedAt := run.CompletedAt
		if observedAt.IsZero() {
			observedAt = run.StartedAt
		}
		if observedAt.IsZero() || observedAt.Before(since) {
			continue
		}
		snap.PulseIntelligencePatrolRuns30d++
		if run.NewFindings > 0 {
			snap.PulseIntelligencePatrolNewFindings30d += run.NewFindings
		}
		if run.AutoFixCount > 0 {
			snap.PulseIntelligencePatrolAutofixes30d += run.AutoFixCount
		}
	}
}

func applyPulseIntelligenceFindingSnapshot(snap *telemetry.Snapshot, persistence *config.ConfigPersistence, since time.Time) {
	if snap == nil || persistence == nil {
		return
	}
	data, err := persistence.LoadAIFindings()
	if err != nil || data == nil {
		return
	}
	for _, finding := range data.Findings {
		if pulseIntelligenceFindingInvestigatedSince(finding, since) {
			snap.PulseIntelligencePatrolInvestigations30d++
		}
		if pulseIntelligenceFindingResolvedSince(finding, since) {
			snap.PulseIntelligencePatrolResolvedFindings30d++
		}
	}
}

func pulseIntelligenceFindingInvestigatedSince(finding *config.AIFindingRecord, since time.Time) bool {
	if finding == nil {
		return false
	}
	if finding.LastInvestigatedAt != nil && !finding.LastInvestigatedAt.IsZero() && !finding.LastInvestigatedAt.Before(since) {
		return true
	}
	record := finding.InvestigationRecord
	if record == nil {
		return false
	}
	if !record.StartedAt.IsZero() && !record.StartedAt.Before(since) {
		return true
	}
	return record.CompletedAt != nil && !record.CompletedAt.IsZero() && !record.CompletedAt.Before(since)
}

func pulseIntelligenceFindingResolvedSince(finding *config.AIFindingRecord, since time.Time) bool {
	if finding == nil {
		return false
	}
	if finding.ResolvedAt != nil && !finding.ResolvedAt.IsZero() && !finding.ResolvedAt.Before(since) {
		return true
	}
	switch strings.TrimSpace(finding.InvestigationOutcome) {
	case "resolved", "fix_verified":
	default:
		return false
	}
	if finding.LastInvestigatedAt != nil && !finding.LastInvestigatedAt.IsZero() && !finding.LastInvestigatedAt.Before(since) {
		return true
	}
	record := finding.InvestigationRecord
	return record != nil && record.CompletedAt != nil && !record.CompletedAt.IsZero() && !record.CompletedAt.Before(since)
}

func applyPulseIntelligenceExternalAgentSnapshot(
	snap *telemetry.Snapshot,
	persistence *config.ConfigPersistence,
	currentCfg *config.Config,
	since time.Time,
	now time.Time,
) {
	if snap == nil {
		return
	}
	if currentCfg != nil {
		surfaceScopes := pulseIntelligenceExternalAgentSurfaceScopes(agentcapabilities.CanonicalManifest())
		if len(surfaceScopes) > 0 {
			for _, token := range currentCfg.APITokens {
				if pulseIntelligenceExternalAgentToken(token, surfaceScopes, now) {
					snap.PulseIntelligenceExternalAgentEnabled = true
				}
				if pulseIntelligenceExternalAgentOperationsLoopToken(token, surfaceScopes, now) {
					snap.PulseIntelligenceExternalAgentOperationsLoopReady = true
				}
				if snap.PulseIntelligenceExternalAgentEnabled && snap.PulseIntelligenceExternalAgentOperationsLoopReady {
					break
				}
			}
		}
	}
	if persistence == nil {
		return
	}
	history, err := persistence.LoadExternalAgentActivityHistory()
	if err != nil || history == nil {
		return
	}
	evidence := telemetry.PulseIntelligenceExternalAgentEvidenceFromHistory(history, since)
	snap.PulseIntelligenceExternalAgentUsed30d = evidence.Used
	snap.PulseIntelligenceMCPAdapterUsed30d = evidence.MCPAdapterUsed
	snap.PulseIntelligenceExternalAgentContextRequests30d = evidence.ContextRequests
	snap.PulseIntelligenceExternalAgentEventStreamRequests30d = evidence.EventStreamRequests
	snap.PulseIntelligenceExternalAgentProvisioningRequests30d = evidence.ProvisioningRequests
	snap.PulseIntelligenceExternalAgentOperatorStateRequests30d = evidence.OperatorStateRequests
	snap.PulseIntelligenceExternalAgentFindingRequests30d = evidence.FindingRequests
	snap.PulseIntelligenceExternalAgentActionRequests30d = evidence.ActionRequests
}

func applyPulseIntelligenceWorkflowPromptSnapshot(snap *telemetry.Snapshot, persistence *config.ConfigPersistence, since time.Time) {
	if snap == nil || persistence == nil {
		return
	}
	history, err := persistence.LoadWorkflowPromptActivityHistory()
	if err != nil || history == nil {
		return
	}
	for _, event := range history.Events {
		if event.Timestamp.IsZero() || event.Timestamp.Before(since) {
			continue
		}
		if strings.TrimSpace(event.PromptName) != agentcapabilities.PulseWorkflowPromptOperationsLoop {
			continue
		}
		snap.PulseIntelligenceOperationsLoopStarterRequests30d++
		switch strings.TrimSpace(event.Surface) {
		case config.WorkflowPromptActivitySurfacePulseAssistant:
			snap.PulseIntelligenceAssistantOperationsLoopStarterRequests30d++
		case config.WorkflowPromptActivitySurfacePulsePatrol:
			snap.PulseIntelligencePatrolOperationsLoopStarterRequests30d++
			snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d++
		case config.WorkflowPromptActivitySurfacePatrolControl:
			snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d++
		case config.WorkflowPromptActivitySurfacePatrolAutonomy:
			snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d++
			// This telemetry field is the compatibility aggregate for paid Patrol autonomy handoffs.
			snap.PulseIntelligenceProActivationOperationsLoopStarterRequests30d++
		case config.WorkflowPromptActivitySurfaceProActivation:
			snap.PulseIntelligencePatrolControlOperationsLoopStarterRequests30d++
			snap.PulseIntelligenceProActivationOperationsLoopStarterRequests30d++
		case config.WorkflowPromptActivitySurfacePulseMCP:
			snap.PulseIntelligenceMCPOperationsLoopStarterRequests30d++
		}
	}
}

func pulseIntelligenceExternalAgentSurfaceScopes(manifest agentcapabilities.Manifest) []string {
	return agentcapabilities.RequiredCapabilityScopes(
		agentcapabilities.ManifestSurfaceToolCapabilities(manifest, agentcapabilities.SurfaceIDPulseMCP),
	)
}

func pulseIntelligenceExternalAgentToken(token config.APITokenRecord, surfaceScopes []string, now time.Time) bool {
	if token.ExpiresAt != nil && now.After(token.ExpiresAt.UTC()) {
		return false
	}
	token = token.Clone()
	for _, scope := range surfaceScopes {
		if strings.TrimSpace(scope) == "" {
			continue
		}
		if token.HasScope(scope) {
			return true
		}
	}
	return false
}

func pulseIntelligenceExternalAgentOperationsLoopToken(token config.APITokenRecord, surfaceScopes []string, now time.Time) bool {
	if token.ExpiresAt != nil && now.After(token.ExpiresAt.UTC()) {
		return false
	}
	token = token.Clone()
	coversLoop := false
	for _, scope := range surfaceScopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		coversLoop = true
		if !token.HasScope(scope) {
			return false
		}
	}
	return coversLoop
}
