package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/rs/zerolog/log"
)

// Investigation measurements share the installation data root, like the
// heartbeat, so all organizations use the same observation window. Only the
// local digest namespaces runs; no organization or run identity is exported.
func (h *AISettingsHandler) initializeInvestigationMeasurements(orgID string, store aicontracts.InvestigationStore) {
	_, _, _, persistence, _, _ := h.stateRefs()
	if persistence == nil {
		return
	}
	if err := persistence.EnsurePatrolInvestigationTelemetry(time.Now().UTC()); err != nil {
		log.Warn().Err(err).Msg("Investigation measurement history unavailable")
		return
	}
	// Recover a crash between the durable session close and its measurement.
	// Pre-observation sessions are ignored, never presented as complete history.
	if err := persistence.RecordPatrolInvestigationMeasurements(orgID, store.GetAll(), time.Now().UTC()); err != nil {
		log.Warn().Err(err).Msg("Investigation measurement replay incomplete; telemetry coverage unavailable")
	}
}

func (h *AISettingsHandler) recordInvestigationMeasurement(orgID string, session *aicontracts.InvestigationSession) {
	_, _, _, persistence, _, _ := h.stateRefs()
	if persistence == nil {
		return
	}
	if err := persistence.RecordPatrolInvestigationMeasurement(orgID, session, time.Now().UTC()); err != nil {
		log.Warn().Err(err).Msg("Investigation measurement not persisted; telemetry coverage unavailable")
	}
}
