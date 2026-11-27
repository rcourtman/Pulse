package alerts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrphanedDockerAlertHostsKeepsActiveAlertCleanupOrder(t *testing.T) {
	started := time.Unix(1800000000, 0)
	m := &Manager{activeAlerts: map[string]*Alert{
		"pve": {
			ID: "node-a", ResourceID: "pve:node-a", Type: "cpu", Level: AlertLevelCritical,
		},
		"legacy-known": {
			ID: "docker-host-offline-present", ResourceID: "docker:wrong", Type: "docker-host-offline", Level: AlertLevelCritical,
		},
		"known-container": {
			ID: "container-a", ResourceID: "docker:present/c1", Type: "container-health", Level: AlertLevelCritical,
		},
		"legacy-orphan": {
			ID: "docker-host-offline-missing/nested", ResourceID: "docker:present", Type: "docker-host-offline",
			Level: AlertLevelCritical, StartTime: started, ResourceName: "Legacy host",
		},
		"higher-priority": {
			ID: "container-b", ResourceID: "docker:gone/c2", Type: "container-health",
			Level: AlertLevelCritical, StartTime: started, ResourceName: "Critical container", Node: "right-hostname",
		},
		"lower-priority": {
			ID: "container-c", ResourceID: "docker:gone/c3", Type: "container-health",
			Level: AlertLevelWarning, StartTime: started.Add(-time.Hour), ResourceName: "Earlier container", Node: "wrong-hostname",
		},
		"blank": {
			ID: "container-d", ResourceID: "docker: /c4", Type: "container-health", Level: AlertLevelCritical,
		},
	}}
	known := map[string]struct{}{"present": {}}
	before := m.GetActiveAlerts()

	got := m.OrphanedDockerAlertHosts(known)
	require.Len(t, got, 2)
	require.Equal(t, "missing/nested", got[0].ID)
	require.Equal(t, "Legacy host", got[0].DisplayName)
	require.Equal(t, "missing/nested", got[0].Hostname)
	require.Equal(t, "gone", got[1].ID)
	require.Equal(t, "Critical container", got[1].DisplayName)
	require.Equal(t, "right-hostname", got[1].Hostname)
	require.Equal(t, before, m.GetActiveAlerts(), "the cleanup read must not change active alerts")

	known["missing/nested"] = struct{}{}
	known["gone"] = struct{}{}
	require.Empty(t, m.OrphanedDockerAlertHosts(known))
}
