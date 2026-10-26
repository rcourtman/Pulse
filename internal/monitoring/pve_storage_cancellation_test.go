package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type cancellableStorageClient struct {
	mockPVEClientExtra
	storage func(context.Context, string) ([]proxmox.Storage, error)
	content func(context.Context, string, string) ([]proxmox.StorageContent, error)
	calls   []string
}

func (c *cancellableStorageClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	c.calls = append(c.calls, "storage/"+node)
	return c.storage(ctx, node)
}

func (c *cancellableStorageClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	c.calls = append(c.calls, "content/"+node+"/"+storage)
	return c.content(ctx, node, storage)
}

func storageCancellationFixture() (*Monitor, []proxmox.Node, map[string]string) {
	m := &Monitor{state: models.NewState(), backupPermissionWarnings: map[string]string{"cluster": "previous access warning"}}
	m.state.UpdateStorageBackupsForInstance("cluster", []models.StorageBackup{{ID: "accepted-file", Instance: "cluster", Storage: "archive", VMID: 101, Time: time.Now().Add(-time.Hour)}})
	m.state.UpdatePBSGuestConfirmationsForInstance("cluster", []models.PBSGuestConfirmation{{Storage: "pbs", VMID: 101, Time: 1000}})
	nodes, online := make([]proxmox.Node, 64), make(map[string]string)
	for i := range nodes {
		nodes[i].Node = fmt.Sprintf("node-%02d", i)
		online[nodes[i].Node] = "online"
	}
	return m, nodes, online
}

func cancellationTestStorages() []proxmox.Storage {
	return []proxmox.Storage{
		{Storage: "archive", Type: "dir", Content: "backup", Active: 1, Enabled: 1},
		{Storage: "archive-two", Type: "dir", Content: "backup", Active: 1, Enabled: 1},
	}
}

// Virtual time advances the unchanged production two-second retry, so this
// catches a cluster-sized deadline overrun without real waits or shorter delays.
func TestPVEStoragePollCancellationStopsRequests(t *testing.T) {
	for _, phase := range []string{"before-poll", "storage-error", "storage-success", "retry-wait", "retry-deadline", "content-error", "content-success", "storage-deadline"} {
		t.Run(phase, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				m, nodes, online := storageCancellationFixture()
				before := m.state.GetSnapshot().PVEBackups
				confirmations := m.state.PBSGuestConfirmationsForInstance("cluster")
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				wantElapsed, wantCalls := time.Duration(0), 1
				c := &cancellableStorageClient{}
				c.storage = func(ctx context.Context, _ string) ([]proxmox.Storage, error) {
					switch phase {
					case "storage-error":
						cancel()
						return nil, ctx.Err()
					case "storage-success":
						cancel()
					case "retry-wait", "retry-deadline":
						return nil, fmt.Errorf("provider request timeout")
					case "storage-deadline":
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return cancellationTestStorages(), nil
				}
				c.content = func(context.Context, string, string) ([]proxmox.StorageContent, error) {
					cancel()
					if phase == "content-error" {
						return nil, fmt.Errorf("403 forbidden")
					}
					return nil, nil
				}
				switch phase {
				case "before-poll":
					cancel()
					wantCalls = 0
				case "retry-wait":
					go func() { time.Sleep(100 * time.Millisecond); cancel() }()
					wantElapsed = 100 * time.Millisecond
				case "retry-deadline", "storage-deadline":
					cancel()
					ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
					defer cancel()
					wantElapsed = 100 * time.Millisecond
				case "content-error", "content-success":
					wantCalls = 2
				}
				started := time.Now()
				m.pollStorageBackupsWithNodes(ctx, "cluster", c, nodes, online)
				if elapsed := time.Since(started); elapsed != wantElapsed || len(c.calls) != wantCalls {
					t.Errorf("cancelled poll kept waiting or issuing reads: elapsed=%s want=%s calls=%d want=%d", elapsed, wantElapsed, len(c.calls), wantCalls)
				}
				if !reflect.DeepEqual(before, m.state.GetSnapshot().PVEBackups) || !reflect.DeepEqual(confirmations, m.state.PBSGuestConfirmationsForInstance("cluster")) || m.backupPermissionWarnings["cluster"] != "previous access warning" {
					t.Error("cancelled poll changed accepted inventory, confirmation or access evidence")
				}
				t.Logf("cancellation phase=%s elapsed=%s requests=%d", phase, time.Since(started), len(c.calls))
			})
		})
	}
}

func TestPVEStoragePollRetryKeepsBoundedPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message string
		recover bool
		retry   bool
	}{
		{"timeout-recovers", "request timeout", true, true},
		{"deadline-recovers", "provider deadline exceeded", true, true},
		{"timeout-exhausted", "request timeout", false, true},
		{"permission-no-retry", "403 forbidden", false, false},
		{"transient-no-retry", "503 unavailable", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				m, nodes, online := storageCancellationFixture()
				c := &cancellableStorageClient{}
				c.storage = func(context.Context, string) ([]proxmox.Storage, error) {
					if tc.recover && len(c.calls) == 2 {
						return nil, nil // An accepted empty catalogue is still meaningful.
					}
					return nil, fmt.Errorf("%s", tc.message)
				}
				started := time.Now()
				m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, nodes[:1], online)
				wantCalls, wantElapsed := 1, time.Duration(0)
				if tc.retry {
					wantCalls, wantElapsed = 2, 2*time.Second
				}
				if len(c.calls) != wantCalls || time.Since(started) != wantElapsed {
					t.Fatalf("live retry policy changed: elapsed=%s calls=%v", time.Since(started), c.calls)
				}
				if got := len(m.state.GetSnapshot().PVEBackups.StorageBackups); (tc.recover && got != 0) || (!tc.recover && got != 1) {
					t.Fatalf("accepted-empty/failure preservation changed: live backups=%d", got)
				}
			})
		})
	}
}

type storageBudgetSnapshotClient struct {
	*cancellableStorageClient
	snapshotAt time.Time
	snapshots  []proxmox.Snapshot
}

func (c *storageBudgetSnapshotClient) GetBackupTasks(context.Context) ([]proxmox.Task, error) {
	return nil, nil
}

func (c *storageBudgetSnapshotClient) GetVMSnapshots(ctx context.Context, _ string, _ int) ([]proxmox.Snapshot, error) {
	c.snapshotAt = time.Now()
	return c.snapshots, ctx.Err()
}

func (c *storageBudgetSnapshotClient) GetContainerSnapshots(ctx context.Context, node string, vmid int) ([]proxmox.Snapshot, error) {
	return c.GetVMSnapshots(ctx, node, vmid)
}

// Exercise the real task -> storage -> snapshot handoff with 64 nodes, including
// a finite parent budget. The old per-node sleeps exhausted that parent before
// snapshots could start. No guest command, backup operation or database is used.
func TestPVEStorageBudgetHandsOffToSnapshots(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, finiteParent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/finite-parent=%t", kind, finiteParent), func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					m, nodes, online := storageCancellationFixture()
					if kind == "qemu" {
						m.state.UpdateVMsForInstance("cluster", []models.VM{{ID: "cluster-node-00-101", VMID: 101, Node: "node-00", Instance: "cluster", Name: "guest-101"}})
					} else {
						m.state.UpdateContainersForInstance("cluster", []models.Container{{ID: "cluster-node-00-101", VMID: 101, Node: "node-00", Instance: "cluster", Name: "guest-101"}})
					}
					started := time.Now()
					c := &storageBudgetSnapshotClient{cancellableStorageClient: &cancellableStorageClient{}, snapshots: []proxmox.Snapshot{{Name: "after-storage-timeout", SnapTime: started.Add(-time.Minute).Unix(), Description: "Snapshot listing recovered within its own budget"}}}
					c.storage = func(ctx context.Context, _ string) ([]proxmox.Storage, error) {
						if !c.snapshotAt.IsZero() {
							return nil, nil // Independent snapshot-size lookup, not an expired backup retry.
						}
						<-ctx.Done()
						return nil, ctx.Err()
					}
					parent, cancel := context.WithCancel(context.Background())
					if finiteParent {
						cancel()
						parent, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					}
					defer cancel()
					before := m.state.GetSnapshot().PVEBackups
					m.pollPVEBackupsAndSnapshots(parent, "cluster", c, nodes, online, time.Second)
					synctest.Wait()
					after := m.state.GetSnapshot().PVEBackups
					if elapsed := c.snapshotAt.Sub(started); elapsed != time.Second {
						t.Errorf("snapshot discovery started after %s, want the unchanged 1s backup budget", elapsed)
					}
					if time.Since(started) != time.Second || len(c.calls) != 2 || len(after.GuestSnapshots) != 1 {
						t.Errorf("snapshot handoff lost its bounded independent read: elapsed=%s storage reads=%d snapshots=%d", time.Since(started), len(c.calls), len(after.GuestSnapshots))
					}
					if !reflect.DeepEqual(before.StorageBackups, after.StorageBackups) {
						t.Error("exhausted storage scan replaced its last accepted files")
					}
					t.Logf("snapshot handoff kind=%s finite-parent=%t elapsed=%s storage-reads=%d snapshots=%d", kind, finiteParent, time.Since(started), len(c.calls), len(after.GuestSnapshots))
					if os.Getenv("PULSE_STORAGE_BUDGET_FRAMES") == "1" && finiteParent {
						source, err := os.ReadFile("monitor_backups.go")
						if err != nil {
							t.Fatal(err)
						}
						data, err := json.Marshal(map[string]any{"kind": kind, "runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "observed_at": time.Now().UTC(), "before": before, "after": after})
						if err != nil {
							t.Fatal(err)
						}
						t.Log("STORAGE_BUDGET_FRAME=" + string(data))
					}
				})
			})
		}
	}
}
