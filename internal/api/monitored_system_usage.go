package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func monitoredSystemCandidateStateFromEnabled(
	enabled bool,
) unifiedresources.MonitoredSystemCandidateState {
	if enabled {
		return unifiedresources.MonitoredSystemCandidateStateActive
	}
	return unifiedresources.MonitoredSystemCandidateStateInactive
}

type monitoredSystemUsageSnapshot struct {
	count             int
	readState         unifiedresources.ReadState
	available         bool
	unavailableReason string
}

func monitoredSystemUsage(monitor *monitoring.Monitor) monitoredSystemUsageSnapshot {
	usage := monitor.MonitoredSystemUsage()
	return monitoredSystemUsageSnapshot{
		count:             usage.Count,
		readState:         usage.ReadState,
		available:         usage.Available,
		unavailableReason: usage.UnavailableReason,
	}
}

func writeMonitoredSystemUsageUnavailable(w http.ResponseWriter, reason string) {
	details := map[string]string{}
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		details["reason"] = trimmed
	}
	writeErrorResponse(
		w,
		http.StatusServiceUnavailable,
		"monitored_system_usage_unavailable",
		"Unable to verify monitored-system inventory right now",
		details,
	)
}

func legacyConnectionCounts(monitor *monitoring.Monitor) legacyConnectionCountsModel {
	return legacyConnectionCountsModel{}
}

func legacyConnectionCountsFromReadState(rs unifiedresources.ReadState) legacyConnectionCountsModel {
	return legacyConnectionCountsModel{}
}
