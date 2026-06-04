package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
)

type stateSummaryResponse struct {
	ActiveAlerts int                      `json:"activeAlerts"`
	Nodes        int                      `json:"nodes"`
	VMs          int                      `json:"vms"`
	Containers   int                      `json:"containers"`
	DockerHosts  []stateSummaryDockerHost `json:"dockerHosts"`
	LastUpdate   time.Time                `json:"lastUpdate"`
}

type stateSummaryDockerHost struct {
	Name            string  `json:"name"`
	Containers      int     `json:"containers"`
	UptimeSeconds   int64   `json:"uptimeSeconds"`
	CPUUsagePercent float64 `json:"cpuUsagePercent"`
}

func buildStateSummary(readState unifiedresources.ReadState, activeAlerts int, lastUpdate time.Time) stateSummaryResponse {
	if readState == nil {
		return stateSummaryResponse{
			ActiveAlerts: activeAlerts,
			LastUpdate:   lastUpdate,
		}
	}

	dockerHosts := make([]stateSummaryDockerHost, 0, len(readState.DockerHosts()))
	for _, host := range readState.DockerHosts() {
		if host == nil {
			continue
		}

		name := strings.TrimSpace(host.CustomDisplayName())
		if name == "" {
			name = strings.TrimSpace(host.DisplayName())
		}
		if name == "" {
			name = strings.TrimSpace(host.Hostname())
		}
		if name == "" {
			name = host.ID()
		}

		dockerHosts = append(dockerHosts, stateSummaryDockerHost{
			Name:            name,
			Containers:      len(host.Containers()),
			UptimeSeconds:   host.UptimeSeconds(),
			CPUUsagePercent: host.CPUPercent(),
		})
	}

	return stateSummaryResponse{
		ActiveAlerts: activeAlerts,
		Nodes:        len(readState.Nodes()),
		VMs:          len(readState.VMs()),
		Containers:   len(readState.Containers()),
		DockerHosts:  dockerHosts,
		LastUpdate:   lastUpdate,
	}
}

func (r *Router) handleStateSummary(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only GET method is allowed", nil)
		return
	}

	authWriter := &responseCapture{ResponseWriter: w}
	if !checkAuth(r.config, authWriter, req, false) {
		if !authWriter.wrote {
			writeErrorResponse(w, http.StatusUnauthorized, "unauthorized",
				"Authentication required", nil)
		}
		return
	}

	if record := getAPITokenRecordFromRequest(req); record != nil && !record.HasScope(config.ScopeMonitoringRead) {
		respondMissingScope(w, config.ScopeMonitoringRead)
		return
	}

	monitor := r.getTenantMonitor(req.Context())
	if monitor == nil {
		writeErrorResponse(w, http.StatusInternalServerError, "no_monitor",
			"Monitor not available", nil)
		return
	}

	readState := monitor.GetUnifiedReadStateOrSnapshot()
	snapshot := monitor.ReadSnapshot()
	if err := utils.WriteJSONResponse(w, buildStateSummary(readState, len(snapshot.ActiveAlerts), snapshot.LastUpdate)); err != nil {
		log.Error().Err(err).Msg("Failed to encode state summary response")
		writeErrorResponse(w, http.StatusInternalServerError, "encoding_error",
			"Failed to encode state summary", nil)
	}
}
