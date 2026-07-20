package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/updates"
	"github.com/rs/zerolog/log"
)

func (r *Router) handleAgentFleetDiagnostics(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET is allowed", nil)
		return
	}

	monitor, err := r.getMonitor(req)
	if err != nil || monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "monitor_unavailable", "Monitor not available", nil)
		return
	}

	serverVersion := "dev"
	if versionInfo, err := updates.GetCurrentVersion(); err == nil && versionInfo != nil {
		serverVersion = versionInfo.Version
	}
	agentUpdateTargetVersion := currentAgentTargetVersion()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(monitor.GetAgentFleetDiagnosticsForTarget(serverVersion, agentUpdateTargetVersion, time.Now().UTC())); err != nil {
		log.Error().Err(err).Msg("Failed to serialize agent fleet diagnostics")
	}
}
