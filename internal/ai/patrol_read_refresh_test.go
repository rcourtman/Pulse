package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// refreshingReadState stands in for the monitor's store: RefreshForRead
// publishes what the monitor has collected since the last rebuild.
type refreshingReadState struct {
	*unifiedresources.ResourceRegistry
	collected models.StateSnapshot
	refreshes int
}

func (r *refreshingReadState) RefreshForRead() {
	r.refreshes++
	r.IngestSnapshot(r.collected)
}

func containerDiskInReadState(rs unifiedresources.ReadState, name string) (float64, bool) {
	for _, container := range rs.Containers() {
		if container.Name() == name {
			return container.DiskPercent(), true
		}
	}
	return 0, false
}

// A Patrol run reads the store the monitor rebuilds at ingest boundaries, not
// after every Proxmox poll. A disk alert cleared at 3.3% started a run that
// read 92% from the previous generation and reported the disk still full
// (14 Nov 2026). Every run refreshes the store before it reads it.
func TestPatrolRunsRefreshTheReadStateBeforeReadingIt(t *testing.T) {
	container := func(usage float64) models.StateSnapshot {
		return models.StateSnapshot{Containers: []models.Container{{
			ID: "pmxlab:pmxlab:111", Instance: "pmxlab", Node: "pmxlab", VMID: 111, Name: "nextcloud",
			Type: "lxc", Status: "running", Disk: models.Disk{Total: 1000, Used: int64(usage * 10), Usage: usage},
		}}}
	}
	newService := func() (*PatrolService, *refreshingReadState) {
		stale := unifiedresources.NewRegistry(nil)
		stale.IngestSnapshot(container(92))
		readState := &refreshingReadState{ResourceRegistry: stale, collected: container(3.3)}
		ps := NewPatrolService(nil, &mockStateProvider{state: container(3.3)})
		ps.SetReadState(readState)
		ps.SetUnifiedResourceProvider(unifiedresources.NewUnifiedAIAdapter(stale))
		ps.SetConfig(PatrolConfig{Enabled: true, AnalyzeGuests: true, Interval: time.Hour})
		return ps, readState
	}

	t.Run("the check a cleared alert starts", func(t *testing.T) {
		ps, readState := newService()
		ps.runScopedPatrol(t.Context(), AlertClearedPatrolScope("pmxlab:pmxlab:111-disk::metric-threshold:disk", "pmxlab:pmxlab:111", "system-container"))
		if readState.refreshes == 0 {
			t.Fatal("a scoped run read the store without refreshing it")
		}
		if got, ok := containerDiskInReadState(readState, "nextcloud"); !ok || got > 3.4 {
			t.Fatalf("after the run the store holds %.1f%%, want the collected 3.3%%", got)
		}
	})

	t.Run("a full run", func(t *testing.T) {
		ps, readState := newService()
		ps.runPatrol(t.Context())
		if readState.refreshes == 0 {
			t.Fatal("a full run read the store without refreshing it")
		}
	})

	t.Run("a read state no monitor refreshes", func(t *testing.T) {
		ps := NewPatrolService(nil, &mockStateProvider{state: container(3.3)})
		ps.SetReadState(unifiedresources.NewRegistry(nil))
		ps.refreshReadStateForRun()
		var none *PatrolService
		none.refreshReadStateForRun()
	})
}
