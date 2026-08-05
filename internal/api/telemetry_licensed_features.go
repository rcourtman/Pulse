package api

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/telemetry"
	"github.com/rs/zerolog/log"
)

// ApplyLicensedFeatureTelemetrySnapshot adds router-owned, content-free
// adoption counts for licensed features whose state lives behind the router
// rather than in persisted config files.
//
// Counts only. Role names, permission sets, and usernames stay on the install;
// only "how many" leaves it. The `now` parameter is retained for symmetry with
// the other router telemetry boundaries.
func (r *Router) ApplyLicensedFeatureTelemetrySnapshot(s *telemetry.Snapshot, now time.Time) {
	if r == nil || s == nil {
		return
	}

	for _, orgID := range r.pulseIntelligenceTelemetryOrgIDs() {
		applyRBACAdoption(s, r, orgID)
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
