package admin

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

type statusResponse struct {
	Version      string                       `json:"version"`
	TotalTenants int                          `json:"total_tenants"`
	Healthy      int                          `json:"healthy"`
	Unhealthy    int                          `json:"unhealthy"`
	ByState      map[registry.TenantState]int `json:"by_state"`
}

// HandleHealthz returns 200 "ok" unconditionally (liveness probe).
func HandleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("ok")); err != nil {
		log.Error().Err(err).Msg("cloudcp.admin: write /healthz response")
	}
}

// HandleReadyz returns a handler that checks database connectivity (readiness probe).
func HandleReadyz(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := reg.Ping(); err != nil {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			if _, writeErr := w.Write([]byte("not ready")); writeErr != nil {
				log.Error().Err(writeErr).Msg("cloudcp.admin: write /readyz unavailable response")
			}
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write([]byte("ready")); writeErr != nil {
			log.Error().Err(writeErr).Msg("cloudcp.admin: write /readyz response")
		}
	}
}

// HandleStatus returns a handler that reports aggregate tenant status.
func HandleStatus(reg *registry.TenantRegistry, version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		counts, err := reg.CountByState()
		if err != nil {
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
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		resp := statusResponse{
			Version:      version,
			TotalTenants: total,
			Healthy:      healthy,
			Unhealthy:    unhealthy,
			ByState:      counts,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		encodeJSON(w, resp)
	}
}
