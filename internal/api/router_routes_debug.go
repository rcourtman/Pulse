package api

import (
	"net/http"
	"net/http/pprof"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// registerDebugRoutes registers /debug/pprof endpoints gated behind admin auth
// and settings:read scope. These endpoints expose Go runtime profiling data and
// must only be accessible to authenticated admin users.
func (r *Router) registerDebugRoutes() {
	// Register exact /debug/pprof (no trailing slash) to prevent ServeMux's
	// automatic 301 redirect which would leak endpoint existence without auth.
	r.mux.HandleFunc("/debug/pprof", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		// Redirect to trailing-slash version inside the auth boundary.
		http.Redirect(w, req, "/debug/pprof/", http.StatusMovedPermanently)
	})))
	r.mux.HandleFunc("/debug/pprof/", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		log.Debug().Str("path", req.URL.Path).Str("ip", req.RemoteAddr).Msg("pprof index request")
		pprof.Index(w, req)
	})))
	r.mux.HandleFunc("/debug/pprof/cmdline", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		log.Debug().Str("ip", req.RemoteAddr).Msg("pprof cmdline request")
		pprof.Cmdline(w, req)
	})))
	r.mux.HandleFunc("/debug/pprof/profile", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		log.Debug().Str("ip", req.RemoteAddr).Msg("pprof profile request")
		pprof.Profile(w, req)
	})))
	r.mux.HandleFunc("/debug/pprof/symbol", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		log.Debug().Str("ip", req.RemoteAddr).Msg("pprof symbol request")
		pprof.Symbol(w, req)
	})))
	r.mux.HandleFunc("/debug/pprof/trace", RequireAdmin(r.config, RequireScope(config.ScopeSettingsRead, func(w http.ResponseWriter, req *http.Request) {
		log.Debug().Str("ip", req.RemoteAddr).Msg("pprof trace request")
		pprof.Trace(w, req)
	})))
}

// pprofEnabled reports whether pprof endpoints should be registered.
// Returns true unless PULSE_PPROF_DISABLED is set to a truthy value.
func pprofEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("PULSE_PPROF_DISABLED")))
	return v != "true" && v != "1"
}
