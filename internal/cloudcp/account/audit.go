package account

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auditlog"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func auditEvent(r *http.Request, eventName, outcome string) *zerolog.Event {
	e := log.Info()
	if outcome != "success" {
		e = log.Warn()
	}

	actorID := auditlog.ActorID(r)
	if actorID == "" {
		actorID = "admin_key"
	}

	return e.
		Str("audit_event", eventName).
		Str("outcome", outcome).
		Str("actor_id", actorID).
		Str("client_ip", auditlog.ClientIP(r)).
		Str("method", r.Method).
		Str("path", auditlog.RequestPath(r))
}
