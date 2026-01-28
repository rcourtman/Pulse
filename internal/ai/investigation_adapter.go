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
