package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func monitoringWebSocketAuthorization(cfg *config.Config, req *http.Request) func() bool {
	// The socket writer outlives this HTTP request. Give it an immutable
	// request copy, not one its returning middleware may still be observing.
	revalidate := monitoringTokenSnapshotRevalidator(cfg, req.Clone(req.Context()))
	// A signed-in user's socket also follows the organization access policy,
	// so membership removal or lost default-organization access ends it.
	orgAccess := orgAccessRevalidator(req)
	if revalidate == nil {
		return combineAuthorization(sessionReadRevalidator(req), orgAccess)
	}
	return combineAuthorization(func() bool {
		_, ok := revalidate()
		return ok
	}, orgAccess)
}
