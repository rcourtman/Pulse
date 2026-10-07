package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type lifecycleReplicationClient struct {
	stubPVEClient
	read       func(context.Context) ([]proxmox.ReplicationJob, error)
	clusterErr error
}

func (c *lifecycleReplicationClient) GetReplicationStatus(ctx context.Context) ([]proxmox.ReplicationJob, error) {
	return c.read(ctx)
}
func (c *lifecycleReplicationClient) GetClusterResources(ctx context.Context, _ string) ([]proxmox.ClusterResource, error) {
	return nil, c.clusterErr
}

func newReplicationLifecycleMonitor(t *testing.T, client PVEClientInterface, monitorGuests bool) *Monitor {
	t.Helper()
	physicalDisks := false
	m := newUnreachableTestMonitor(t, &config.Config{PVEInstances: []config.PVEInstance{{Name: "site-a", Host: "https://example.invalid:9999", MonitorVMs: monitorGuests, MonitorPhysicalDisks: &physicalDisks}}})
	m.pveClients["site-a"] = client
	m.lastClusterCheck["site-a"] = time.Now() // No external discovery; test only scheduler/client lifecycle.
	m.SetExecutor(newRealExecutor(m))
	return m
}
func runReplicationCycle(m *Monitor) {
	tasks := m.buildScheduledTasks(time.Now())
	for _, task := range tasks {
		if task.InstanceType == InstanceTypePVE && task.InstanceName == "site-a" {
			m.executeScheduledTask(context.Background(), task)
			return
		}
	}
	panic("missing scheduled site-a task")
}
func waitReplication(t *testing.T, done <-chan struct{}, stage string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatalf("%s did not finish", stage)
	}
}
func replicationSeed(m *Monitor) []models.ReplicationJob {
	rows := []models.ReplicationJob{{ID: "site-a-100-0", Instance: "site-a", JobID: "100-0", GuestID: 100, LastSyncStatus: "error", LastPolled: time.Unix(100, 0)}}
	m.state.UpdateReplicationJobsForInstance("site-a", rows)
	m.state.UpdateReplicationJobsForInstance("site-b", []models.ReplicationJob{{ID: "site-b-100-0", Instance: "site-b", JobID: "100-0", LastSyncStatus: "ok", LastPolled: time.Unix(200, 0)}})
	return m.state.GetSnapshot().ReplicationJobs
}

// These controls use only the old public-to-package entry points as well as
// the real scheduler/provider/executor path, so identical tests run on parent.
func TestReplicationLifecycleScheduledFallbackAndDisabledGuests(t *testing.T) {
	for _, monitorGuests := range []bool{true, false} {
		t.Run(fmt.Sprint(monitorGuests), func(t *testing.T) {
			called := make(chan struct{}, 1)
			c := &lifecycleReplicationClient{clusterErr: fmt.Errorf("cluster/resources unavailable"), read: func(ctx context.Context) ([]proxmox.ReplicationJob, error) {
				called <- struct{}{}
				return []proxmox.ReplicationJob{{ID: "100-0", GuestID: 100, LastSyncStatus: "ok"}}, nil
			}}
			m := newReplicationLifecycleMonitor(t, c, monitorGuests)
			runReplicationCycle(m)
			waitReplication(t, called, "ordinary replication read")
			deadline := time.Now().Add(3 * time.Second)
			for len(m.state.GetSnapshot().ReplicationJobs) == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			rows := m.state.GetSnapshot().ReplicationJobs
			if len(rows) != 1 || rows[0].ID != "site-a-100-0" || rows[0].LastSyncStatus != "ok" || rows[0].LastPolled.IsZero() {
				t.Fatalf("ordinary read not published: %+v", rows)
			}
		})
	}
}
func TestReplicationLifecycleInterruptedAndUnavailablePreserveInventory(t *testing.T) {
	for _, mode := range []string{"canceled-empty", "canceled-jobs", "404", "501", "403", "transport"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c := &lifecycleReplicationClient{read: func(context.Context) ([]proxmox.ReplicationJob, error) {
				switch mode {
				case "canceled-empty":
					cancel()
					return nil, nil
				case "canceled-jobs":
					cancel()
					return []proxmox.ReplicationJob{{ID: "100-0", GuestID: 100, LastSyncStatus: "ok"}}, nil
				default:
					return nil, fmt.Errorf("%s unavailable", mode)
				}
			}}
			m := newReplicationLifecycleMonitor(t, c, true)
			before := replicationSeed(m)
			m.pollReplicationStatus(ctx, "site-a", c, nil)
			if got := m.state.GetSnapshot().ReplicationJobs; !reflect.DeepEqual(got, before) {
				t.Fatalf("interrupted/unavailable inventory renewed or cleared: before=%+v after=%+v", before, got)
			}
			// Only a later complete successful read can refresh/clear this instance.
			c.read = func(context.Context) ([]proxmox.ReplicationJob, error) {
				return []proxmox.ReplicationJob{{ID: "100-0", GuestID: 100, LastSyncStatus: "ok"}}, nil
			}
			m.pollReplicationStatus(context.Background(), "site-a", c, nil)
			rows := m.state.GetSnapshot().ReplicationJobs
			if len(rows) != 2 {
				t.Fatalf("recovery lost another instance: %+v", rows)
			}
			for _, row := range rows {
				if row.Instance == "site-a" && (row.LastSyncStatus != "ok" || !row.LastPolled.After(time.Unix(100, 0))) {
					t.Fatalf("recovery not fresh: %+v", row)
				}
			}
			c.read = func(context.Context) ([]proxmox.ReplicationJob, error) { return []proxmox.ReplicationJob{}, nil }
			m.pollReplicationStatus(context.Background(), "site-a", c, nil)
			rows = m.state.GetSnapshot().ReplicationJobs
			if len(rows) != 1 || rows[0].Instance != "site-b" {
				t.Fatalf("complete empty should remove only site-a: %+v", rows)
			}
		})
	}
}

// The exact-parent control uses only pre-existing entry points. It observes the
// obsolete write for a bounded window; the companion owned-boundary test also
// waits for final goroutine termination, not just absence within this window.
func TestReplicationLifecycleObsoleteScheduledCompletion(t *testing.T) {
	for _, mode := range []string{"runtime", "client", "retire"} {
		t.Run(mode, func(t *testing.T) {
			started := make(chan struct{})
			release := make(chan struct{})
			returned := make(chan struct{})
			var released bool
			old := &lifecycleReplicationClient{read: func(context.Context) ([]proxmox.ReplicationJob, error) {
				close(started)
				<-release
				defer close(returned)
				return []proxmox.ReplicationJob{{ID: "obsolete-0", LastSyncStatus: "ok"}}, nil
			}}
			m := newReplicationLifecycleMonitor(t, old, true)
			m.setRuntimeContext(context.Background(), nil)
			defer func() {
				if !released {
					close(release)
				}
			}()
			runReplicationCycle(m)
			waitReplication(t, started, "old scheduled read")
			if mode == "runtime" {
				m.setRuntimeContext(context.Background(), nil)
			}
			if mode == "retire" {
				m.retirePVEInstanceRuntime("site-a")
			}
			newer := &lifecycleReplicationClient{read: func(context.Context) ([]proxmox.ReplicationJob, error) {
				return []proxmox.ReplicationJob{{ID: "current-0", LastSyncStatus: "error"}}, nil
			}}
			m.mu.Lock()
			m.pveClients["site-a"] = newer
			m.lastClusterCheck["site-a"] = time.Now()
			m.mu.Unlock()
			runReplicationCycle(m)
			end := time.Now().Add(3 * time.Second)
			for {
				rows := m.state.GetSnapshot().ReplicationJobs
				if len(rows) == 1 && rows[0].JobID == "current-0" {
					break
				}
				if time.Now().After(end) {
					t.Fatalf("current scheduled read did not recover: %+v", rows)
				}
				time.Sleep(time.Millisecond)
			}
			close(release)
			released = true
			waitReplication(t, returned, "obsolete client return")
			end = time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(end) {
				rows := m.state.GetSnapshot().ReplicationJobs
				if len(rows) != 1 || rows[0].JobID != "current-0" {
					t.Fatalf("obsolete scheduled result replaced current inventory: %+v", rows)
				}
				time.Sleep(time.Millisecond)
			}
		})
	}
}
