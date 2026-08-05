package api

import (
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// withAuditReadActivity records that an entitled operator reached a
// license-gated audit read or export surface, then runs the handler.
//
// This wrapper belongs INSIDE RequireLicenseFeature so it only fires for
// requests that cleared the licence gate. Recording the read, rather than the
// presence of an audit store, is the point: the SQLite audit logger is
// installed unconditionally for defense in depth, so store presence is true on
// every install and distinguishes nothing.
//
// The record carries a timestamp and a coarse activity class. Query filters,
// the actor, the requested range, and every audit row read stay on the install.
func (r *Router) withAuditReadActivity(activity string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		r.recordAuditReadActivity(req, activity)
		handler(w, req)
	}
}

func (r *Router) recordAuditReadActivity(req *http.Request, activity string) {
	if r == nil || req == nil {
		return
	}
	persistence := r.persistenceForOrg(req.Context())
	if persistence == nil {
		return
	}
	if err := persistence.RecordAuditReadActivity(config.AuditReadActivityRecord{
		Timestamp: time.Now().UTC(),
		Activity:  activity,
	}); err != nil {
		log.Debug().Err(err).Str("activity", activity).Msg("Failed to record audit read activity")
	}
}
