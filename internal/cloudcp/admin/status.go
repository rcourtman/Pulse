package admin

import (
	"encoding/json"
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
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
	_, _ = w.Write([]byte("ok"))
}

// HandleReadyz returns a handler that checks database connectivity (readiness probe).
func HandleReadyz(reg *registry.TenantRegistry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := reg.Ping(); err != nil {
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
		_ = json.NewEncoder(w).Encode(resp)
	}
}
