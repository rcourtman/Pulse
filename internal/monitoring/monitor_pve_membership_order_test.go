package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Cluster membership detection must run before the poll cycle's node-state
// commit. A newly added connection whose add-time detection failed (#437)
// otherwise publishes its nodes with an empty cluster name, and that
// unclassified window is what let a second site's same-named node bind into
// an established cluster's slot (MSP support case).
func TestPollPVEInstanceCommitsClusterIdentityOnFirstPoll(t *testing.T) {
	originalDetect := detectMonitorPVECluster
	t.Cleanup(func() { detectMonitorPVECluster = originalDetect })
	detectMonitorPVECluster = func(clientConfig proxmox.ClientConfig, existingEndpoints []config.ClusterEndpoint) (bool, string, []config.ClusterEndpoint) {
		return true, "rewo", []config.ClusterEndpoint{
			{NodeID: "node/pve01", NodeName: "pve01", Host: "https://192.168.1.11:8006", IP: "192.168.1.11", Online: true, LastSeen: time.Now()},
		}
	}

	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				// Standalone-shaped: add-time cluster detection failed.
				{Name: "rewo", Host: "https://192.168.1.11:8006"},
			},
		},
		state:                    models.NewState(),
		pveClients:               make(map[string]PVEClientInterface),
		nodeLastOnline:           make(map[string]time.Time),
		nodeSnapshots:            make(map[string]NodeMemorySnapshot),
		guestSnapshots:           make(map[string]GuestMemorySnapshot),
		nodeRRDMemCache:          make(map[string]rrdMemCacheEntry),
		metricsHistory:           NewMetricsHistory(32, time.Hour),
		guestMetadataCache:       make(map[string]guestMetadataCacheEntry),
		guestMetadataLimiter:     make(map[string]time.Time),
		lastClusterCheck:         make(map[string]time.Time),
		lastPhysicalDiskPoll:     make(map[string]time.Time),
		lastPVEBackupPoll:        make(map[string]time.Time),
		lastPBSBackupPoll:        make(map[string]time.Time),
		authFailures:             make(map[string]int),
		lastAuthAttempt:          make(map[string]time.Time),
		pollStatusMap:            make(map[string]*pollStatus),
		nodePendingUpdatesCache:  make(map[string]pendingUpdatesCache),
		instanceInfoCache:        make(map[string]*instanceInfo),
		lastOutcome:              make(map[string]taskOutcome),
		failureCounts:            make(map[string]int),
		removedDockerHosts:       make(map[string]time.Time),
		dockerTokenBindings:      make(map[string]string),
		dockerCommands:           make(map[string]*dockerHostCommand),
		dockerCommandIndex:       make(map[string]string),
		guestAgentFSInfoTimeout:  defaultGuestAgentFSInfoTimeout,
		guestAgentNetworkTimeout: defaultGuestAgentNetworkTimeout,
		guestAgentOSInfoTimeout:  defaultGuestAgentOSInfoTimeout,
		guestAgentVersionTimeout: defaultGuestAgentVersionTimeout,
		guestAgentRetries:        defaultGuestAgentRetries,
		alertManager:             alerts.NewManager(),
		notificationMgr:          notifications.NewNotificationManager(""),
	}
	defer m.alertManager.Stop()
	defer m.notificationMgr.Stop()

	mockClient := &mockPVEClientExtended{
		nodes: []proxmox.Node{
			{Node: "pve01", Status: "online"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	m.pollPVEInstance(ctx, "rewo", mockClient)

	if len(m.state.Nodes) != 1 {
		t.Fatalf("nodes = %#v, want 1", m.state.Nodes)
	}
	node := m.state.Nodes[0]
	if node.Name != "pve01" {
		t.Fatalf("node = %#v, want pve01", node)
	}
	if node.ClusterName != "rewo" {
		t.Fatalf("first poll committed ClusterName = %q, want rewo (membership detection must precede the node-state commit)", node.ClusterName)
	}
	if !node.IsClusterMember {
		t.Fatalf("first poll committed IsClusterMember = false, want true")
	}
}
