package ai

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
)

// InvestigationOrchestratorAdapter adapts investigation.Orchestrator to InvestigationOrchestrator interface
type InvestigationOrchestratorAdapter struct {
	orchestrator *investigation.Orchestrator
}

// NewInvestigationOrchestratorAdapter creates a new adapter
func NewInvestigationOrchestratorAdapter(o *investigation.Orchestrator) *InvestigationOrchestratorAdapter {
	return &InvestigationOrchestratorAdapter{orchestrator: o}
}

// InvestigateFinding starts an investigation for a finding
func (a *InvestigationOrchestratorAdapter) InvestigateFinding(ctx context.Context, finding *InvestigationFinding, autonomyLevel string) error {
	// Convert to investigation package's Finding type
	invFinding := &investigation.Finding{
		ID:                     finding.ID,
		Key:                    finding.Key,
		Severity:               finding.Severity,
		Category:               finding.Category,
		ResourceID:             finding.ResourceID,
		ResourceName:           finding.ResourceName,
		ResourceType:           finding.ResourceType,
		Title:                  finding.Title,
		Description:            finding.Description,
		Recommendation:         finding.Recommendation,
		Evidence:               finding.Evidence,
		InvestigationSessionID: finding.InvestigationSessionID,
		InvestigationStatus:    finding.InvestigationStatus,
		InvestigationOutcome:   finding.InvestigationOutcome,
		LastInvestigatedAt:     finding.LastInvestigatedAt,
		InvestigationAttempts:  finding.InvestigationAttempts,
	}

	return a.orchestrator.InvestigateFinding(ctx, invFinding, autonomyLevel)
}

// GetInvestigationByFinding returns the latest investigation for a finding
func (a *InvestigationOrchestratorAdapter) GetInvestigationByFinding(findingID string) *InvestigationSession {
	inv := a.orchestrator.GetInvestigationByFinding(findingID)
	if inv == nil {
		return nil
	}

	// Convert to ai package's InvestigationSession type
	session := &InvestigationSession{
		ID:             inv.ID,
		FindingID:      inv.FindingID,
		SessionID:      inv.SessionID,
		Status:         string(inv.Status),
		StartedAt:      inv.StartedAt,
		CompletedAt:    inv.CompletedAt,
		TurnCount:      inv.TurnCount,
		Outcome:        string(inv.Outcome),
		ToolsAvailable: inv.ToolsAvailable,
		ToolsUsed:      inv.ToolsUsed,
		EvidenceIDs:    inv.EvidenceIDs,
		Summary:        inv.Summary,
		Error:          inv.Error,
		ApprovalID:     inv.ApprovalID,
	}

	if inv.ProposedFix != nil {
		session.ProposedFix = &InvestigationFix{
			ID:          inv.ProposedFix.ID,
			Description: inv.ProposedFix.Description,
			Commands:    inv.ProposedFix.Commands,
			RiskLevel:   inv.ProposedFix.RiskLevel,
			Destructive: inv.ProposedFix.Destructive,
			TargetHost:  inv.ProposedFix.TargetHost,
			Rationale:   inv.ProposedFix.Rationale,
		}
	}

	return session
}

// GetRunningCount returns the number of running investigations
func (a *InvestigationOrchestratorAdapter) GetRunningCount() int {
	return a.orchestrator.GetRunningCount()
}

// GetFixedCount returns the number of issues auto-fixed by Patrol
func (a *InvestigationOrchestratorAdapter) GetFixedCount() int {
	return a.orchestrator.GetFixedCount()
}

// CanStartInvestigation returns true if a new investigation can be started
func (a *InvestigationOrchestratorAdapter) CanStartInvestigation() bool {
	return a.orchestrator.CanStartInvestigation()
}

// ReinvestigateFinding triggers a re-investigation of a finding
func (a *InvestigationOrchestratorAdapter) ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error {
	return a.orchestrator.ReinvestigateFinding(ctx, findingID, autonomyLevel)
}

// Shutdown delegates to the underlying orchestrator's Shutdown method.
func (a *InvestigationOrchestratorAdapter) Shutdown(ctx context.Context) error {
	return a.orchestrator.Shutdown(ctx)
}

// SetFixVerifier registers a PatrolService as the fix verifier on the underlying orchestrator.
func (a *InvestigationOrchestratorAdapter) SetFixVerifier(patrol *PatrolService) {
	a.orchestrator.SetFixVerifier(&patrolFixVerifier{patrol: patrol})
}

// SetMetricsCallback wires patrol Prometheus metrics into the orchestrator.
func (a *InvestigationOrchestratorAdapter) SetMetricsCallback() {
	a.orchestrator.SetMetricsCallback(&patrolMetricsCallback{})
}

// SetLicenseChecker wires license checking into the orchestrator for defense-in-depth.
func (a *InvestigationOrchestratorAdapter) SetLicenseChecker(checker investigation.LicenseChecker) {
	a.orchestrator.SetLicenseChecker(checker)
}

// patrolMetricsCallback adapts PatrolMetrics to the investigation.MetricsCallback interface.
type patrolMetricsCallback struct{}

func (c *patrolMetricsCallback) RecordInvestigationOutcome(outcome string) {
	GetPatrolMetrics().RecordInvestigationOutcome(outcome)
}

func (c *patrolMetricsCallback) RecordFixVerification(result string) {
	GetPatrolMetrics().RecordFixVerification(result)
}

// patrolFixVerifier adapts PatrolService to the investigation.FixVerifier interface.
type patrolFixVerifier struct {
	patrol *PatrolService
}

// VerifyFixResolved delegates to PatrolService.VerifyFixResolved, converting types.
func (v *patrolFixVerifier) VerifyFixResolved(ctx context.Context, finding *investigation.Finding) (bool, error) {
	return v.patrol.VerifyFixResolved(ctx, finding.ResourceID, finding.ResourceType, finding.Key, finding.ID)
}
