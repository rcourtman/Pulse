package admin

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

// HandleHealthz returns 200 "ok" unconditionally (liveness probe).
func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// HandleReadyz returns a handler that checks database connectivity (readiness probe).
func HandleReadyz(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reg == nil {
			log.Error().Msg("Control plane readiness check failed: registry dependency unavailable")
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}

		if err := reg.Ping(); err != nil {
			log.Warn().Err(err).Msg("Control plane readiness check failed: registry ping error")
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}
}

// HandleStatus returns a handler that reports aggregate tenant status.
func HandleStatus(reg *registry.TenantRegistry, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if reg == nil {
			log.Error().Msg("Control plane status check failed: registry dependency unavailable")
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		counts, err := reg.CountByState()
		if err != nil {
			log.Error().Err(err).Msg("Control plane status check failed: count by state")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		// Opportunistically sync gauges on status calls (in addition to the background updater).
		for state, c := range counts {
			cpmetrics.TenantsByState.WithLabelValues(string(state)).Set(float64(c))
		}

		total := 0
		for _, c := range counts {
			total += c
		}

		healthy, unhealthy, err := reg.HealthSummary()
		if err != nil {
			log.Error().Err(err).Msg("Control plane status check failed: health summary")
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := map[string]any{
			"version":       version,
			"total_tenants": total,
			"healthy":       healthy,
			"unhealthy":     unhealthy,
			"by_state":      counts,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
