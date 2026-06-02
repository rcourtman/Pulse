package monitoring

import (
	"context"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/mock"
)

// TestCephPoolAlertsDeduplicatedAcrossSources is a regression guard for
// rcourtman/Pulse#1341 ("dual-source Ceph" pool alerts).
//
// The v5 bug: when the SAME physical Ceph cluster was reported by both the
// Proxmox API poller AND a Pulse host-agent, the agent-sourced clusters were
// stored under an "agent:"-prefixed instance namespace while the API-sourced
// clusters used the bare instance name. Alert evaluation ran on the raw,
// un-deduplicated cluster list, so one pool produced two pool-storage IDs
// (agent:pve5-ceph-pool-foo vs pve5-ceph-pool-foo) -> duplicate/flapping pool
// alerts and a per-pool override that appeared to "revert" between identities.
//
// v6 architecture closes the gap differently from the v5 patch (there is no
// "agent:" prefix here): both ingest paths reconcile by FSID *before* alert
// evaluation runs.
//
//   - host-agent path (internal/monitoring/monitor_agents.go ~L2073-2090):
//     convertAgentCephToGlobalCluster(Instance=hostname, Source=host-agent)
//     -> State.UpsertCephCluster -> checkCephPoolStorage(STORED cluster).
//   - Proxmox path (internal/monitoring/ceph.go pollCephCluster):
//     buildCephClusterModel(Instance=instance, Source=proxmox-api)
//     -> State.UpdateCephClustersForInstance -> checkCephPoolStorage(STORED cluster).
//
// Both State methods funnel through upsertCephClusterInSlice
// (internal/models/ceph_cluster_identity.go), which collapses same-FSID
// clusters into a single reconciled entry (the Proxmox source wins identity by
// rank). checkCephPoolStorage is therefore only ever invoked with the single
// reconciled cluster, and the per-pool override resolves across source aliases
// via storageThresholdLookupIDs/cephPoolStorageSourceAliasID
// (internal/alerts/storage_override_identity.go) while clearStorageAliasAlerts
// removes any alias-keyed alert.
//
// This test reconstructs the exact dual-source scenario and asserts the
// observable outcome a user sees: exactly ONE active Ceph pool usage alert for
// the pool, the per-pool override honored, and no second/flapping alert across
// either ingest order or repeated interleaved cycles.
func TestCephPoolAlertsDeduplicatedAcrossSources(t *testing.T) {
	const (
		instance = "pve5"
		hostID   = "host-pve5"
		fsid     = "1341-dual-source-fsid"
		pool     = "data_replication"
	)
	poolStorageID := models.CephPoolStorageID(instance, pool)

	newMonitor := func(t *testing.T) (*Monitor, *alerts.Manager) {
		manager := alerts.NewManagerWithDataDir(t.TempDir())
		manager.UpdateConfig(alerts.AlertConfig{
			Enabled:         true,
			ActivationState: alerts.ActivationActive,
			StorageDefault:  alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			TimeThresholds:  map[string]int{"storage": 0},
			// A 50% per-pool override on the canonical pool-storage ID.
			Overrides: map[string]alerts.ThresholdConfig{
				poolStorageID: {Usage: &alerts.HysteresisThreshold{Trigger: 50, Clear: 45}},
			},
		})
		return &Monitor{state: models.NewState(), alertManager: manager}, manager
	}

	// ingestAgentCeph mirrors the host-agent glue in monitor_agents.go: convert
	// the agent report to a global cluster, upsert into state, then run the pool
	// alert check on the STORED (reconciled) cluster.
	ingestAgentCeph := func(m *Monitor, ts time.Time) {
		report := &agentshost.CephCluster{
			FSID:   fsid,
			Health: agentshost.CephHealth{Status: "HEALTH_OK"},
			PGMap: agentshost.CephPGMap{
				NumPGs:         128,
				BytesTotal:     100,
				BytesUsed:      61,
				BytesAvailable: 39,
				UsagePercent:   61,
			},
			Pools: []agentshost.CephPool{
				{ID: 1, Name: pool, BytesUsed: 61, BytesAvailable: 39, Objects: 100, PercentUsed: 61},
			},
		}
		cluster := convertAgentCephToGlobalCluster(report, instance, hostID, ts)
		stored := m.state.UpsertCephCluster(cluster)
		m.checkCephPoolStorage(stored)
	}

	// ingestProxmoxCeph drives the real Proxmox poll entry point with a mock
	// client reporting the SAME FSID and pool.
	ingestProxmoxCeph := func(m *Monitor) {
		client := &mockCephPVEClient{}
		client.On("GetCephStatus", mock.Anything).Return(&proxmox.CephStatus{
			FSID:   fsid,
			Health: proxmox.CephHealth{Status: "HEALTH_OK"},
		}, nil)
		client.On("GetCephDF", mock.Anything).Return(&proxmox.CephDF{
			Data: proxmox.CephDFData{
				Stats: proxmox.CephDFStats{TotalBytes: 100, TotalUsedBytes: 61, TotalAvailBytes: 39},
				Pools: []proxmox.CephDFPool{
					{ID: 1, Name: pool, Stats: proxmox.CephDFPoolStat{BytesUsed: 61, MaxAvail: 39, Objects: 100, PercentUsed: 61}},
				},
			},
		}, nil)
		m.pollCephCluster(context.Background(), instance, client, true)
	}

	assertSingleAlert := func(t *testing.T, manager *alerts.Manager) alerts.Alert {
		t.Helper()
		active := manager.GetActiveAlerts()
		if len(active) != 1 {
			t.Fatalf("active alerts = %d, want exactly one Ceph pool usage alert: %+v", len(active), active)
		}
		a := active[0]
		if a.ResourceID != poolStorageID {
			t.Fatalf("alert ResourceID = %q, want canonical pool-storage ID %q", a.ResourceID, poolStorageID)
		}
		if a.Threshold != 50 {
			t.Fatalf("alert Threshold = %v, want honored per-pool override 50", a.Threshold)
		}
		return a
	}

	assertSingleCluster := func(t *testing.T, m *Monitor) {
		t.Helper()
		clusters := m.state.GetSnapshot().CephClusters
		if len(clusters) != 1 {
			t.Fatalf("CephClusters = %d, want one reconciled (FSID-deduplicated) cluster: %+v", len(clusters), clusters)
		}
	}

	t.Run("agent reported first, then proxmox", func(t *testing.T) {
		m, manager := newMonitor(t)
		defer manager.Stop()

		ingestAgentCeph(m, time.Now())
		first := assertSingleAlert(t, manager)
		assertSingleCluster(t, m)

		ingestProxmoxCeph(m)
		second := assertSingleAlert(t, manager)
		assertSingleCluster(t, m)

		if !second.StartTime.Equal(first.StartTime) {
			t.Fatalf("alert StartTime changed (%v -> %v): alert was cleared and re-raised (flap) when the second source arrived", first.StartTime, second.StartTime)
		}
	})

	t.Run("proxmox reported first, then agent", func(t *testing.T) {
		m, manager := newMonitor(t)
		defer manager.Stop()

		ingestProxmoxCeph(m)
		first := assertSingleAlert(t, manager)
		assertSingleCluster(t, m)

		ingestAgentCeph(m, time.Now())
		second := assertSingleAlert(t, manager)
		assertSingleCluster(t, m)

		if !second.StartTime.Equal(first.StartTime) {
			t.Fatalf("alert StartTime changed (%v -> %v): alert was cleared and re-raised (flap) when the second source arrived", first.StartTime, second.StartTime)
		}
	})

	t.Run("repeated interleaved cycles do not flap or duplicate", func(t *testing.T) {
		m, manager := newMonitor(t)
		defer manager.Stop()

		ingestAgentCeph(m, time.Now())
		baseline := assertSingleAlert(t, manager)

		for i := 0; i < 5; i++ {
			ingestProxmoxCeph(m)
			assertSingleAlert(t, manager)
			ingestAgentCeph(m, time.Now())
			assertSingleAlert(t, manager)
		}

		final := assertSingleAlert(t, manager)
		assertSingleCluster(t, m)
		if final.ID != baseline.ID || !final.StartTime.Equal(baseline.StartTime) {
			t.Fatalf("alert identity changed across interleaved cycles (flap): %s@%v -> %s@%v", baseline.ID, baseline.StartTime, final.ID, final.StartTime)
		}
	})
}
