package api

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"testing"
	"time"
)

func TestPatrolInvestigationMeasurementsDoNotRequireSurvivingFindings(t *testing.T) {
	p := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	if err := p.EnsurePatrolInvestigationTelemetry(now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	store := newTestInvestigationStore()
	session := store.Create("deleted-finding", "chat-session")
	session.Status = aicontracts.InvestigationStatusCompleted
	session.CompletedAt = &now
	session.Outcome = aicontracts.OutcomeFixQueued
	store.Update(session)
	h := &AISettingsHandler{defaultPersistence: p, investigationStores: map[string]aicontracts.InvestigationStore{"default": store}}
	closed := false
	h.closePatrolInvestigationRun("default", session, func(s *aicontracts.InvestigationSession) { closed = true })
	if !closed || p.PatrolInvestigationTelemetry(time.Now().UTC()).Runs != 1 {
		t.Fatal("close lost the measurement without a finding service")
	}
	// Later transitions must update the same closed run even though no finding
	// or Patrol service remains available to receive its UI projection.
	audit := unifiedresources.ActionAuditRecord{ID: "action", State: unifiedresources.ActionStateExpired,
		Origin: &unifiedresources.ActionOrigin{Surface: patrolActionOriginSurface, FindingID: session.FindingID, InvestigationID: session.ID, ProposalID: "proposal"}}
	h.applyPatrolActionAudit("default", audit)
	got := p.PatrolInvestigationTelemetry(time.Now().UTC())
	if !got.Available || got.Runs != 1 || got.Outcomes["fix_queued"] != 0 || got.Outcomes["needs_attention"] != 1 || got.Handoffs[aicontracts.HandoffActionExpired] != 1 {
		t.Fatalf("expired action projection: %+v", got)
	}
	// Replay must not add another outcome or rely on finding hydration.
	h.applyPatrolActionAudit("default", audit)
	if p.PatrolInvestigationTelemetry(time.Now().UTC()).Runs != 1 {
		t.Fatal("replayed transition double-counted")
	}
}

func TestPatrolInvestigationMeasurementClosesAfterFindingWasRemoved(t *testing.T) {
	p := config.NewConfigPersistence(t.TempDir())
	now := time.Now().UTC()
	if err := p.EnsurePatrolInvestigationTelemetry(now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	sessions := newTestInvestigationStore()
	s := sessions.Create("removed-finding", "session")
	s.Status = aicontracts.InvestigationStatusCompleted
	s.CompletedAt = &now
	s.Outcome = aicontracts.OutcomeNeedsAttention
	s.HandoffReason = aicontracts.HandoffHumanDecision
	sessions.Update(s)
	h := &AISettingsHandler{defaultPersistence: p}
	// No finding store at all: the closed-run measurement must still land.
	adapter := orchestratorFindingsAdapter{sessions: sessions, measured: func(s *aicontracts.InvestigationSession) { h.recordInvestigationMeasurement("tenant", s) }}
	if adapter.Update(&aicontracts.Finding{ID: s.FindingID, InvestigationSessionID: s.SessionID, InvestigationStatus: string(s.Status)}) {
		t.Fatal("missing finding unexpectedly updated")
	}
	got := p.PatrolInvestigationTelemetry(time.Now().UTC())
	if !got.Available || got.Runs != 1 || got.Handoffs[aicontracts.HandoffHumanDecision] != 1 {
		t.Fatalf("cleanup erased closing measurement: %+v", got)
	}
}
