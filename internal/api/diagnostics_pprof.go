package api

import (
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
)

const pprofRoutePrefix = "/api/diagnostics/pprof"

func pprofEnabled() bool {
	return strings.EqualFold(os.Getenv("PULSE_ENABLE_PPROF"), "true")
}

func (r *Router) handlePprofRedirect(w http.ResponseWriter, req *http.Request) {
	if !pprofEnabled() {
		http.NotFound(w, req)
		return
	}

	target := pprofRoutePrefix + "/"
	if req.URL.RawQuery != "" {
		target += "?" + req.URL.RawQuery
	}
	http.Redirect(w, req, target, http.StatusMovedPermanently)
}

func (r *Router) handlePprof(w http.ResponseWriter, req *http.Request) {
	if !pprofEnabled() {
		http.NotFound(w, req)
		return
	}

	relative := strings.TrimPrefix(req.URL.Path, pprofRoutePrefix)
	switch relative {
	case "", "/":
		pprof.Index(w, req)
		return
	case "/cmdline":
		pprof.Cmdline(w, req)
		return
	case "/profile":
		pprof.Profile(w, req)
		return
	case "/symbol":
		pprof.Symbol(w, req)
		return
	case "/trace":
		pprof.Trace(w, req)
		return
	default:
		profile := strings.TrimPrefix(relative, "/")
		pprof.Handler(profile).ServeHTTP(w, req)
	}
}
