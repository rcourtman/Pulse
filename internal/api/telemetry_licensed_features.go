package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rcourtman/pulse-go-rewrite/pkg/audit"
	"github.com/rs/zerolog/log"
)

// ApplyLicensedFeatureTelemetrySnapshot adds router-owned, content-free
// adoption counts for licensed features whose state lives behind the router
// rather than in persisted config files.
//
// Everything here is a count or a coarse boolean. Role names, permission sets,
// usernames, audit event types, actors, and audit event detail all stay on the
// install; only "how many" and "is a persistent store active" leave it.
func (r *Router) ApplyLicensedFeatureTelemetrySnapshot(s *telemetry.Snapshot, now time.Time) {
	if r == nil || s == nil {
		return
	}

	since := now.Add(-telemetry.PulseIntelligenceTelemetryWindow)
	for _, orgID := range r.pulseIntelligenceTelemetryOrgIDs() {
		applyRBACAdoption(s, r, orgID)
		applyAuditAdoption(s, orgID, since)
	}
}

// applyRBACAdoption counts operator-authored roles and role assignments.
// Built-in roles ship with every install and are excluded so the count means
// "this operator configured RBAC", not "this install booted".
func applyRBACAdoption(s *telemetry.Snapshot, r *Router, orgID string) {
	manager, ok := r.rbacProvider.PeekManager(orgID)
	if !ok || manager == nil {
		return
	}
	if err := rbacMigrationFailure(manager); err != nil {
		// A degraded store reports a partial picture; counting it would
		// understate adoption without any way to tell from the aggregate.
		log.Debug().Err(err).Str("org_id", orgID).Msg("Skipping RBAC telemetry for degraded store")
		return
	}

	for _, role := range manager.GetRoles() {
		if !role.IsBuiltIn {
			s.RBACCustomRoles++
		}
	}
	s.RBACUserAssignments += len(manager.GetUserAssignments())
}

// applyAuditAdoption records whether a persistent audit store is active and how
// many events it retained inside the telemetry window.
func applyAuditAdoption(s *telemetry.Snapshot, orgID string, since time.Time) {
	logger := getLoggerForOrg(orgID)
	if logger == nil || !isPersistentLogger(logger) {
		return
	}
	s.AuditLoggingPersistent = true

	count, err := logger.Count(audit.QueryFilter{StartTime: &since})
	if err != nil {
		log.Debug().Err(err).Str("org_id", orgID).Msg("Unable to count audit events for telemetry summary")
		return
	}
	s.AuditEvents30d += count
}
