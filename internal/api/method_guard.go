package api

import (
	"net/http"
	"strings"
)

// requireRequestMethod admits only the listed methods to a handler that
// changes state, answering anything else with 405 and an Allow header. Bare
// mux patterns and path-dispatched routes do not filter methods, and GET/HEAD
// pass both the demo-mode read-only guard and the CSRF check while
// SameSite=Lax session cookies ride cross-site top-level GET navigations, so a
// mutating handler must refuse safe methods itself.
func requireRequestMethod(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, method := range allowed {
		if r.Method == method {
			return true
		}
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}
