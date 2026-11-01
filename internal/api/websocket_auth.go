package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func monitoringWebSocketAuthorization(cfg *config.Config, req *http.Request) func() bool {
	// The socket writer outlives this HTTP request. Give it an immutable
	// request copy, not one its returning middleware may still be observing.
	revalidate := monitoringTokenSnapshotRevalidator(cfg, req.Clone(req.Context()))
	if revalidate == nil {
		return sessionReadRevalidator(req)
	}
	return func() bool {
		_, ok := revalidate()
		return ok
	}
}
