package monitoring

import (
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// Exercise the production reset boundary without Start or its background
// goroutines: shutting down a test differently cannot repair this race.
func TestGetStateConcurrentReset(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)

	startTime := time.Unix(1700000000, 0)
	m := &Monitor{startTime: startTime}
	m.mu.Lock()
	m.resetStateLocked()
	m.mu.Unlock()

	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 1000; i++ {
			m.mu.Lock()
			m.resetStateLocked()
			m.mu.Unlock()
		}
	}()
	go func() {
		defer workers.Done()
		<-start
		for i := 0; i < 1000; i++ {
			snapshot := m.GetState()
			if snapshot.Stats.StartTime != startTime || snapshot.Stats.Version != "2.0.0-go" {
				t.Errorf("reader observed partially initialized reset stats: %+v", snapshot.Stats)
				return
			}
		}
	}()
	close(start)
	workers.Wait()
}

func TestGetStateResetSnapshotIsolation(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	defer mustSetMockEnabled(t, previous)

	m := &Monitor{state: models.NewState(), startTime: time.Unix(1700000000, 0)}
	m.state.UpdateNodes([]models.Node{{ID: "old-node", Name: "old-node"}})
	m.state.UpdateActiveAlerts([]models.Alert{{ID: "old-alert"}})
	m.state.UpdateRecentlyResolved([]models.ResolvedAlert{{Alert: models.Alert{ID: "old-resolved"}}})
	before := m.GetState()
	m.mu.Lock()
	m.resetStateLocked()
	m.mu.Unlock()
	after := m.GetState()
	if len(before.Nodes) != 1 || before.Nodes[0].ID != "old-node" {
		t.Fatal("reset changed the previously returned snapshot")
	}
	if len(before.ActiveAlerts) != 1 || before.ActiveAlerts[0].ID != "old-alert" ||
		len(before.RecentlyResolved) != 1 || before.RecentlyResolved[0].ID != "old-resolved" {
		t.Fatal("snapshot lost state-backed alerts without an alert manager")
	}
	if len(after.ActiveAlerts) != 0 || len(after.RecentlyResolved) != 0 {
		t.Fatal("reset retained stale state-backed alerts")
	}
	if len(after.Nodes) != 0 || after.Stats.StartTime != m.startTime || after.Stats.Version != "2.0.0-go" {
		t.Fatalf("reset did not expose an initialized empty state: %+v", after)
	}
	if snapshot := (&Monitor{}).GetState(); len(snapshot.Nodes) != 0 {
		t.Fatal("nil state must return an empty snapshot")
	}
}
