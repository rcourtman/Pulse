package monitoring

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func TestPVEPollReadKeepsUnpolledStorageAndOtherInstance(t *testing.T) {
	for _, disposition := range []string{"offline", "failed", "empty"} {
		t.Run(disposition, func(t *testing.T) {
			at := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
			state := models.NewState()
			state.Storage = []models.Storage{
				{ID: "other-node-local", Instance: "other", Node: "node", Name: "local", Total: 9000, Used: 8000, Free: 1000, LastSeen: at},
				{ID: "pve-node-local", Instance: "pve", Node: "node", Name: "local", Total: 1000, Used: 350, Free: 650, Usage: 35, LastSeen: at, Nodes: []string{"node"}, NodeIDs: []string{"pve-node"}, AliasIDs: []string{"legacy"}, ZFSPool: &models.ZFSPool{Name: "tank", Devices: []models.ZFSDevice{{Name: "disk", ReadErrors: 3}}, Datasets: []models.ZFSDataset{{Name: "tank/data", UsedBytes: 350}}, ScanDetails: &models.ZFSScan{Function: "scrub", Percentage: 42, StartedAt: &at}}},
			}
			before := state.GetSnapshot().Storage
			m := &Monitor{state: state, config: &config.Config{PVEInstances: []config.PVEInstance{{Name: "pve", DisableCeph: true}}}}
			client := &fakeStorageClient{}
			node := proxmox.Node{Node: "node", Status: "online"}
			if disposition == "offline" {
				node.Status = "offline"
			}
			if disposition == "empty" {
				client.storageByNode = map[string][]proxmox.Storage{"node": {}}
			}
			m.pollStorageWithNodes(context.Background(), "pve", client, []proxmox.Node{node})
			if got := state.GetSnapshot().Storage; !reflect.DeepEqual(got, before) {
				t.Fatalf("%s changed retained storage or another instance: got %+v, want %+v", disposition, got, before)
			}
		})
	}
}

func TestPVENodeCleanupReadIncludesOtherInstances(t *testing.T) {
	manager := alerts.NewManagerWithDataDir(t.TempDir())
	t.Cleanup(manager.Stop)
	cfg := manager.GetConfig()
	cfg.Enabled = true
	cfg.ActivationState = alerts.ActivationPending
	cfg.TimeThresholds = map[string]int{"node": 0}
	cfg.SuppressionWindow = 0
	manager.UpdateConfig(cfg)
	now := time.Now()
	peer := models.Node{ID: "other-peer", Instance: "other", Name: "peer", Status: "online", CPU: 0.99, LastSeen: now}
	gone := models.Node{ID: "old-gone", Instance: "pve", Name: "gone", Status: "online", CPU: 0.99, LastSeen: now}
	manager.CheckNode(peer)
	manager.CheckNode(gone)
	if got := manager.GetActiveAlerts(); len(got) != 2 {
		t.Fatalf("expected two live controls, got %+v", got)
	}
	state := models.NewState()
	state.Nodes = []models.Node{peer}
	m := &Monitor{state: state, alertManager: manager, metricsHistory: NewMetricsHistory(8, time.Hour)}
	local := models.Node{ID: "pve-local", Instance: "pve", Name: "local", Status: "online", CPU: 0.1, LastSeen: now}
	m.applyStorageFallbackAndRecordNodeMetrics("pve", nil, []models.Node{local}, nil, nil)
	got := manager.GetActiveAlerts()
	if len(got) != 1 || got[0].ResourceID != peer.ID {
		t.Fatalf("cleanup lost another instance's alert or kept a removed node: %+v", got)
	}
}
