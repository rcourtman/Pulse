package api

import (
	"context"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

type ssoManagementAdmissionKey struct{}

// This private, request-local record is created before authentication. Recovery
// cannot erase the request's unknown-state origin. Only checkAuth records how
// it actually admitted the request; headers and ambient cookies are not proof.
type ssoManagementAdmission struct {
	unknown       bool
	authenticated bool
}

func (r *Router) requireSSOManagementPermission(next http.HandlerFunc) http.HandlerFunc {
	permissioned := RequirePermission(r.config, r.authorizer, auth.ActionAdmin, auth.ResourceUsers, next)
	return func(w http.ResponseWriter, req *http.Request) {
		r.ensureSSOConfig()
		admission := &ssoManagementAdmission{unknown: r.ssoAuthLoadFailed.Load()}
		req = req.WithContext(context.WithValue(req.Context(), ssoManagementAdmissionKey{}, admission))
		permissioned(w, req)
	}
}
