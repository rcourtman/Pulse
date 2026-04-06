package api

import "net/http"

// hideCommercialReadSurfaceInDemo returns a generic 404 for demo-only public
// runtimes so billing and license detail endpoints are not surfaced to users.
func hideCommercialReadSurfaceInDemo(w http.ResponseWriter, r *http.Request, demoMode bool) bool {
	if !demoMode {
		return false
	}

	http.NotFound(w, r)
	return true
}
