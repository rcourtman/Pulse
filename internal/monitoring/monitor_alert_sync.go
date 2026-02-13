package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/logging"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// syncAlertsToState copies the latest alert manager data into the shared state snapshot.
// This keeps WebSocket broadcasts aligned with in-memory acknowledgement updates.
func (m *Monitor) syncAlertsToState() {
	if m.pruneStaleDockerAlerts() {
		if logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Msg("pruned stale docker alerts during sync")
		}
	}

	activeAlerts := m.alertManager.GetActiveAlerts()
	modelAlerts := make([]models.Alert, 0, len(activeAlerts))
	for _, alert := range activeAlerts {
		modelAlerts = append(modelAlerts, models.Alert{
			ID:              alert.ID,
			Type:            alert.Type,
			Level:           string(alert.Level),
			ResourceID:      alert.ResourceID,
			ResourceName:    alert.ResourceName,
			Node:            alert.Node,
			NodeDisplayName: alert.NodeDisplayName,
			Instance:        alert.Instance,
			Message:         alert.Message,
			Value:           alert.Value,
			Threshold:       alert.Threshold,
			StartTime:       alert.StartTime,
			Acknowledged:    alert.Acknowledged,
			AckTime:         alert.AckTime,
			AckUser:         alert.AckUser,
		})
		if alert.Acknowledged && logging.IsLevelEnabled(zerolog.DebugLevel) {
			log.Debug().Str("alertID", alert.ID).Interface("ackTime", alert.AckTime).Msg("syncing acknowledged alert")
		}
	}
	m.state.UpdateActiveAlerts(modelAlerts)

	recentlyResolved := m.alertManager.GetRecentlyResolved()
	if len(recentlyResolved) > 0 {
		log.Info().Int("count", len(recentlyResolved)).Msg("syncing recently resolved alerts")
	}
	m.state.UpdateRecentlyResolved(recentlyResolved)
}

// SyncAlertState is the exported wrapper used by APIs that mutate alerts outside the poll loop.
func (m *Monitor) SyncAlertState() {
	m.syncAlertsToState()
}

// pruneStaleDockerAlerts removes docker alerts that reference hosts no longer present in state.
func (m *Monitor) pruneStaleDockerAlerts() bool {
	if m.alertManager == nil {
		return false
	}

	hosts := m.state.GetDockerHosts()
	knownHosts := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		hostID := strings.TrimSpace(host.ID)
		if hostID != "" {
			knownHosts[hostID] = struct{}{}
		}
	}

	if len(knownHosts) == 0 {
		// Still allow stale entries to be cleared if no hosts remain.
	}

	active := m.alertManager.GetActiveAlerts()
	processed := make(map[string]struct{})
	cleared := false

	for _, alert := range active {
		var hostID string

		switch {
		case alert.Type == "docker-host-offline":
			hostID = strings.TrimPrefix(alert.ID, "docker-host-offline-")
		case strings.HasPrefix(alert.ResourceID, "docker:"):
			resource := strings.TrimPrefix(alert.ResourceID, "docker:")
			if idx := strings.Index(resource, "/"); idx >= 0 {
				hostID = resource[:idx]
			} else {
				hostID = resource
			}
		default:
			continue
		}

		hostID = strings.TrimSpace(hostID)
		if hostID == "" {
			continue
		}

		if _, known := knownHosts[hostID]; known {
			continue
		}
		if _, alreadyCleared := processed[hostID]; alreadyCleared {
			continue
		}

		host := models.DockerHost{
			ID:          hostID,
			DisplayName: alert.ResourceName,
			Hostname:    alert.Node,
		}
		if host.DisplayName == "" {
			host.DisplayName = hostID
		}
		if host.Hostname == "" {
			host.Hostname = hostID
		}

		m.alertManager.HandleDockerHostRemoved(host)
		processed[hostID] = struct{}{}
		cleared = true
	}

	return cleared
}
