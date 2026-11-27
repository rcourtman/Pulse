package alerts

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// OrphanedDockerAlertHosts returns cleanup identities in active alert order.
// Keep that order because the first orphaned alert for each host supplies its
// name and hostname for legacy tracking cleanup. Only clone and sort alerts
// that can trigger a cleanup; the full active-alert snapshot is for display.
func (m *Manager) OrphanedDockerAlertHosts(knownHosts map[string]struct{}) []models.DockerHost {
	candidates := sortActiveAlerts(m.collectActiveAlertReadCopies(func(alert *Alert) bool {
		// Match cloneAlertForOutput's legacy/canonical resource backfill.
		// Inferred identity can replace a non-empty stored resource ID.
		resourceID := alert.ResourceID
		if inferred := inferCanonicalResourceIDFromLegacyAlert(alert); inferred != "" {
			resourceID = inferred
		}
		id := dockerAlertCleanupHostID(alert.Type, resourceID)
		_, known := knownHosts[id]
		return id != "" && !known
	}))
	if len(candidates) == 0 {
		return nil
	}
	var hosts []models.DockerHost
	seen := make(map[string]struct{})
	for _, alert := range candidates {
		id := dockerAlertCleanupHostID(alert.Type, alert.ResourceID)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		name := alert.ResourceName
		if name == "" {
			name = id
		}
		hostname := alert.Node
		if hostname == "" {
			hostname = id
		}
		hosts = append(hosts, models.DockerHost{ID: id, DisplayName: name, Hostname: hostname})
	}
	return hosts
}

func dockerAlertCleanupHostID(alertType, resourceID string) string {
	if alertType == "docker-host-offline" {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(resourceID), "docker:"))
	}
	if resource, ok := strings.CutPrefix(resourceID, "docker:"); ok {
		host, _, _ := strings.Cut(resource, "/")
		return strings.TrimSpace(host)
	}
	return ""
}
