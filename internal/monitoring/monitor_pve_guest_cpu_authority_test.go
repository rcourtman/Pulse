package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestRecordGuestMetricsKeepsLiveAndHistoricalProxmoxCPUUnitsAligned(t *testing.T) {
	previousMockMode := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() {
		mustSetMockEnabled(t, previousMockMode)
	})

	cfg := metrics.DefaultConfig(t.TempDir())
	cfg.WriteBufferSize = 32
	cfg.FlushInterval = time.Hour
	store, err := metrics.NewStore(cfg)
	if err != nil {
		t.Fatalf("new metrics store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	if err := store.WaitForMaintenance(5 * time.Second); err != nil {
		t.Fatalf("wait for metrics maintenance: %v", err)
	}

	history := NewMetricsHistory(32, time.Hour)
	monitor := &Monitor{
		metricsHistory: history,
		metricsStore:   store,
	}
	monitor.recordGuestMetrics(
		[]models.VM{{
			ID:       "cluster-a-node-1-301",
			Status:   "running",
			CPU:      0.0058,
			CPUs:     8,
			LastSeen: time.Now().UTC(),
		}},
		[]models.Container{{
			ID:       "cluster-a-node-1-302",
			Status:   "running",
			Type:     "lxc",
			CPU:      0.0058,
			CPUs:     1,
			LastSeen: time.Now().UTC(),
		}},
	)
	store.Flush()

	for _, tc := range []struct {
		name         string
		resourceType string
		resourceID   string
	}{
		{name: "qemu", resourceType: "vm", resourceID: "cluster-a-node-1-301"},
		{name: "lxc", resourceType: "container", resourceID: "cluster-a-node-1-302"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			historyPoints := history.GetGuestMetrics(tc.resourceID, "cpu", time.Hour)
			if len(historyPoints) != 1 || historyPoints[0].Value != 0.58 {
				t.Fatalf("in-memory CPU history = %+v, want one 0.58 point", historyPoints)
			}

			now := time.Now().UTC()
			storePoints, err := store.Query(tc.resourceType, tc.resourceID, "cpu", now.Add(-time.Minute), now.Add(time.Minute), 0)
			if err != nil {
				t.Fatalf("query persistent CPU history: %v", err)
			}
			if len(storePoints) != 1 || storePoints[0].Value != 0.58 {
				t.Fatalf("persistent CPU history = %+v, want one 0.58 point", storePoints)
			}
		})
	}
}
