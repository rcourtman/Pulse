package ai

import (
	"context"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/investigation"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// InvestigationOrchestratorAdapter adapts investigation.Orchestrator to the
// aicontracts.InvestigationOrchestrator interface.
type InvestigationOrchestratorAdapter struct {
	orchestrator *investigation.Orchestrator
}

// NewInvestigationOrchestratorAdapter creates a new adapter.
func NewInvestigationOrchestratorAdapter(o *investigation.Orchestrator) *InvestigationOrchestratorAdapter {
	return &InvestigationOrchestratorAdapter{orchestrator: o}
}

// InvestigateFinding starts an investigation for a finding.
func (a *InvestigationOrchestratorAdapter) InvestigateFinding(ctx context.Context, f *aicontracts.Finding, autonomyLevel string) error {
	return a.orchestrator.InvestigateFinding(ctx, f, autonomyLevel)
}

// GetInvestigationByFinding returns the latest investigation for a finding.
// Since investigation.InvestigationSession is now a type alias for
// aicontracts.InvestigationSession, no conversion is needed.
func (a *InvestigationOrchestratorAdapter) GetInvestigationByFinding(findingID string) *aicontracts.InvestigationSession {
	return a.orchestrator.GetInvestigationByFinding(findingID)
}

// GetRunningCount returns the number of running investigations.
func (a *InvestigationOrchestratorAdapter) GetRunningCount() int {
	return a.orchestrator.GetRunningCount()
}

// GetFixedCount returns the number of issues auto-fixed by Patrol.
func (a *InvestigationOrchestratorAdapter) GetFixedCount() int {
	return a.orchestrator.GetFixedCount()
}

// CanStartInvestigation returns true if a new investigation can be started.
func (a *InvestigationOrchestratorAdapter) CanStartInvestigation() bool {
	return a.orchestrator.CanStartInvestigation()
}

// ReinvestigateFinding triggers a re-investigation of a finding.
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
func (v *patrolFixVerifier) VerifyFixResolved(ctx context.Context, finding *aicontracts.Finding) (bool, error) {
	return v.patrol.VerifyFixResolved(ctx, finding.ResourceID, finding.ResourceType, finding.Key, finding.ID)
}
