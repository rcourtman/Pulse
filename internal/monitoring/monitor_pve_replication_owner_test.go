package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func replicationOwner(t *testing.T, m *Monitor) *pveReplicationPoll {
	t.Helper()
	m.mu.RLock()
	owner := m.pveReplicationPolls["site-a"]
	m.mu.RUnlock()
	if owner == nil {
		t.Fatal("replication claim missing")
	}
	return owner
}
func TestReplicationLifecycleSingleFlightAndCycleCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int32
	c := &lifecycleReplicationClient{read: func(ctx context.Context) ([]proxmox.ReplicationJob, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return []proxmox.ReplicationJob{{ID: "100-0", LastSyncStatus: "ok"}}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	m := newReplicationLifecycleMonitor(t, c, true)
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.setRuntimeContext(runCtx, nil)
	t.Cleanup(func() { cancel(); close(release) })
	runReplicationCycle(m)
	waitReplication(t, started, "first read")
	owner := replicationOwner(t, m)
	// executeScheduledTask canceled its short-lived task context on return.
	if owner.ctx.Err() != nil {
		t.Fatalf("cycle canceled background read: %v", owner.ctx.Err())
	}
	deadline, ok := owner.ctx.Deadline()
	if !ok || time.Until(deadline) > pveReplicationPollTimeout {
		t.Fatal("background read has no bounded deadline")
	}
	for i := 0; i < 5; i++ {
		runReplicationCycle(m)
	}
	if calls.Load() != 1 {
		t.Fatalf("overlapping cycles stacked %d reads", calls.Load())
	}
	cancel()
	waitReplication(t, owner.done, "runtime cancellation")
	if len(m.state.GetSnapshot().ReplicationJobs) != 0 {
		t.Fatal("cancellation published an inventory")
	}
	// A new ordinary runtime/cycle recovers without a manual inventory invocation.
	m.setRuntimeContext(context.Background(), nil)
	c.read = func(context.Context) ([]proxmox.ReplicationJob, error) {
		return []proxmox.ReplicationJob{{ID: "100-0", LastSyncStatus: "ok"}}, nil
	}
	runReplicationCycle(m)
	end := time.Now().Add(3 * time.Second)
	for len(m.state.GetSnapshot().ReplicationJobs) == 0 && time.Now().Before(end) {
		time.Sleep(time.Millisecond)
	}
	if len(m.state.GetSnapshot().ReplicationJobs) != 1 {
		t.Fatal("ordinary recovery did not publish")
	}
}
func TestReplicationLifecycleReplacementRejectsLateCompletion(t *testing.T) {
	for _, mode := range []string{"runtime", "client", "retire", "client-empty", "client-error"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			var released bool
			old := &lifecycleReplicationClient{read: func(ctx context.Context) ([]proxmox.ReplicationJob, error) {
				close(started)
				<-release
				return []proxmox.ReplicationJob{{ID: "stale-0", LastSyncStatus: "ok"}}, nil
			}}
			m := newReplicationLifecycleMonitor(t, old, true)
			m.setRuntimeContext(context.Background(), nil)
			before := replicationSeed(m)
			defer func() {
				if !released {
					close(release)
				}
			}()
			runReplicationCycle(m)
			waitReplication(t, started, "old read")
			owner := replicationOwner(t, m)
			switch mode {
			case "runtime":
				m.setRuntimeContext(context.Background(), nil) // Old parent deliberately still live.
			case "retire":
				m.retirePVEInstanceRuntime("site-a")
			}
			newer := &lifecycleReplicationClient{read: func(context.Context) ([]proxmox.ReplicationJob, error) {
				if mode == "client-empty" {
					return []proxmox.ReplicationJob{}, nil
				}
				if mode == "client-error" {
					return nil, fmt.Errorf("403 permission denied")
				}
				return []proxmox.ReplicationJob{{ID: "fresh-0", LastSyncStatus: "error"}}, nil
			}}
			m.mu.Lock()
			m.pveClients["site-a"] = newer
			m.lastClusterCheck["site-a"] = time.Now()
			m.mu.Unlock()
			// Dispatch through current provider binding, then wait for the owned terminal
			// boundary even when a successful empty result cannot be observed as a row.
			runReplicationCycle(m)
			// It may already have completed, so capture the terminal through state/claim.
			end := time.Now().Add(3 * time.Second)
			for {
				m.mu.RLock()
				current := m.pveReplicationPolls["site-a"]
				m.mu.RUnlock()
				if current == nil {
					break
				}
				if time.Now().After(end) {
					t.Fatal("replacement read did not terminate")
				}
				time.Sleep(time.Millisecond)
			}
			expected := m.state.GetSnapshot().ReplicationJobs
			if mode == "client-error" && !reflect.DeepEqual(expected, before) {
				t.Fatalf("failed replacement renewed inventory: %+v", expected)
			}
			if mode != "client-error" {
				expectedCount := 2
				if mode == "client-empty" {
					expectedCount = 1
				}
				if len(expected) != expectedCount {
					t.Fatalf("replacement result %+v", expected)
				}
				for _, row := range expected {
					if row.Instance == "site-a" && row.JobID != "fresh-0" {
						t.Fatalf("wrong replacement identity: %+v", row)
					}
				}
			}
			close(release)
			released = true
			waitReplication(t, owner.done, "old late completion")
			if got := m.state.GetSnapshot().ReplicationJobs; !reflect.DeepEqual(got, expected) {
				t.Fatalf("old read overwrote replacement: before=%+v after=%+v", expected, got)
			}
			if owner.ctx.Err() == nil {
				t.Fatal("superseded read not canceled")
			}
		})
	}
}
