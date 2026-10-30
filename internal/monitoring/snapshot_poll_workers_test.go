package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Synthetic API listings only: no guest agent, task operation or installed host.
type snapshotPoolClient struct {
	mockPVEClientExtra
	read func(context.Context, string, int) ([]proxmox.Snapshot, error)
}

func (c *snapshotPoolClient) GetVMSnapshots(ctx context.Context, _ string, id int) ([]proxmox.Snapshot, error) {
	return c.read(ctx, "qemu", id)
}
func (c *snapshotPoolClient) GetContainerSnapshots(ctx context.Context, _ string, id int) ([]proxmox.Snapshot, error) {
	return c.read(ctx, "lxc", id)
}
func (c *snapshotPoolClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	return nil, nil // Optional sizing is not the measurement.
}

func snapshotPoolMonitor(count int) (*Monitor, map[string]models.GuestSnapshot) {
	m := &Monitor{state: models.NewState()}
	var vms []models.VM
	var cts []models.Container
	previous := make(map[string]models.GuestSnapshot, count)
	var snapshots []models.GuestSnapshot
	for i := 0; i < count; i++ {
		id := 1000 + i
		kind := "qemu"
		if i%2 == 0 {
			vms = append(vms, models.VM{ID: fmt.Sprintf("site-node-%d", id), Instance: "site", Node: "node", VMID: id, Name: fmt.Sprintf("vm%d", id)})
		} else {
			kind = "lxc"
			cts = append(cts, models.Container{ID: fmt.Sprintf("site-node-%d", id), Instance: "site", Node: "node", VMID: id, Name: fmt.Sprintf("ct%d", id)})
		}
		snap := models.GuestSnapshot{ID: fmt.Sprintf("site-node-%d-prior", id), Instance: "site", Node: "node", VMID: id, Type: kind, Name: "prior", Time: time.Unix(1700000000, 0), Description: "last accepted listing", SizeBytes: 2048}
		previous[fmt.Sprintf("%s/%d", kind, id)] = snap
		snapshots = append(snapshots, snap)
	}
	// Templates and another connection must never enter this collector's queue.
	vms = append(vms, models.VM{ID: "site-node-1", Instance: "site", Node: "node", VMID: 1, Template: true})
	cts = append(cts, models.Container{ID: "site-node-2", Instance: "site", Node: "node", VMID: 2, Template: true})
	m.state.UpdateVMsForInstance("site", vms)
	m.state.UpdateContainersForInstance("site", cts)
	m.state.UpdateVMsForInstance("peer", []models.VM{{ID: "peer-node-1000", Instance: "peer", Node: "node", VMID: 1000}})
	m.state.UpdateGuestSnapshotsForInstance("site", snapshots)
	m.state.UpdateGuestSnapshotsForInstance("peer", []models.GuestSnapshot{{ID: "peer-prior", Instance: "peer", Node: "node", VMID: 1000, Type: "qemu", Name: "independent", Time: time.Unix(1690000000, 0)}})
	return m, previous
}

// Count only this collector's goroutines, not runtime/GC/testing workers. No
// stacks are logged or retained. Both the old semaphore and new pool are
// measured by the same frame prefix, without relying on compiler func numbers.
func snapshotCollectorGoroutines(t *testing.T) int {
	t.Helper()
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)
	require.Less(t, n, len(buf), "goroutine measurement was truncated")
	count := 0
	for _, stack := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(stack, ".(*Monitor).pollGuestSnapshots.func") {
			count++
		}
	}
	return count
}

func testSnapshotPollWorkersBoundWaitingAndActiveWork(t *testing.T) {
	for _, count := range []int{0, 1, 8, 9, 512} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				m, previous := snapshotPoolMonitor(count)
				gate := make(chan struct{})
				var mu sync.Mutex
				calls := make(map[string]int, count)
				active, maxActive := 0, 0
				c := &snapshotPoolClient{read: func(_ context.Context, kind string, id int) ([]proxmox.Snapshot, error) {
					mu.Lock()
					calls[fmt.Sprintf("%s/%d", kind, id)]++
					active++
					maxActive = max(maxActive, active)
					mu.Unlock()
					<-gate
					mu.Lock()
					active--
					mu.Unlock()
					return []proxmox.Snapshot{{Name: "fresh", SnapTime: 1700000100, Description: "accepted"}}, nil
				}}
				before := snapshotCollectorGoroutines(t)
				done := make(chan struct{})
				go func() {
					m.pollGuestSnapshots(context.Background(), "site", c)
					close(done)
				}()
				synctest.Wait()
				waiting := snapshotCollectorGoroutines(t) - before
				// Release even when an assertion fails, so the old-source control
				// can finish and report its real resource-bound failure cleanly.
				close(gate)
				synctest.Wait()
				<-done
				t.Logf("guests=%d collector_goroutines=%d max_active_reads=%d", count, waiting, maxActive)
				require.LessOrEqual(t, waiting, 10, "eight workers plus dispatcher/closer, not one waiting goroutine per guest")
				require.Equal(t, min(count, 8), maxActive, "preserve eight-way provider concurrency")
				require.Len(t, calls, count)
				for key := range previous {
					require.Equal(t, 1, calls[key], "each eligible guest needs exactly one read: %s", key)
				}
				rows := m.state.GetSnapshot().PVEBackups.GuestSnapshots
				require.Len(t, rows, count+1, "fresh target listings plus untouched peer")
				for _, row := range rows {
					if row.Instance == "peer" {
						require.Equal(t, models.GuestSnapshot{ID: "peer-prior", Instance: "peer", Node: "node", VMID: 1000, Type: "qemu", Name: "independent", Time: time.Unix(1690000000, 0)}, row)
						continue
					}
					kind := previous[fmt.Sprintf("%s/%d", row.Type, row.VMID)].Type
					require.NotEmpty(t, kind, "snapshot belongs to an eligible typed guest")
					require.Equal(t, models.GuestSnapshot{ID: fmt.Sprintf("site-node-%d-fresh", row.VMID), Instance: "site", Node: "node", VMID: row.VMID, Type: kind, Name: "fresh", Time: time.Unix(1700000100, 0), Description: "accepted", VMState: kind == "qemu"}, row)
				}
			})
		})
	}
}

func TestSnapshotPollWorkersCancelQueuedReadsWithoutLosingAcceptedListings(t *testing.T) {
	for _, stop := range []string{"cancel", "deadline"} {
		t.Run(stop, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				m, previous := snapshotPoolMonitor(512)
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				var mu sync.Mutex
				calls := make(map[string]int)
				accepted := ""
				c := &snapshotPoolClient{read: func(ctx context.Context, kind string, id int) ([]proxmox.Snapshot, error) {
					key := fmt.Sprintf("%s/%d", kind, id)
					mu.Lock()
					calls[key]++
					first := accepted == ""
					if first {
						accepted = key
					}
					mu.Unlock()
					if first {
						return []proxmox.Snapshot{{Name: "accepted-before-stop", SnapTime: 1700000100}}, nil
					}
					<-ctx.Done()
					// A nil-error late body must not masquerade as an accepted
					// result or authorize removal of the last known listing.
					return []proxmox.Snapshot{{Name: "late-unaccepted", SnapTime: 1700000200}}, nil
				}}
				done := make(chan struct{})
				go func() {
					m.pollGuestSnapshots(ctx, "site", c)
					close(done)
				}()
				synctest.Wait()
				beforeStop := len(calls)
				require.Equal(t, 9, beforeStop, "one accepted listing and eight blocked reads")
				if stop == "deadline" {
					time.Sleep(time.Minute)
				} else {
					cancel()
				}
				synctest.Wait()
				<-done
				require.Len(t, calls, beforeStop, "queued guests must not start reads after cancellation")
				rows := m.state.GetSnapshot().PVEBackups.GuestSnapshots
				require.Len(t, rows, len(previous)+1)
				for _, row := range rows {
					if row.Instance == "peer" {
						require.Equal(t, "independent", row.Name)
						continue
					}
					key := fmt.Sprintf("%s/%d", row.Type, row.VMID)
					if key == accepted {
						require.Equal(t, "accepted-before-stop", row.Name)
						require.Equal(t, time.Unix(1700000100, 0), row.Time)
					} else {
						require.Equal(t, previous[key], row, "unread/late listing must preserve exact continuity: %s", key)
					}
				}
				// A later ordinary observation drains every eligible guest. No
				// restart, wider credentials or extra read path is involved.
				c.read = func(_ context.Context, kind string, id int) ([]proxmox.Snapshot, error) {
					mu.Lock()
					calls[fmt.Sprintf("%s/%d", kind, id)]++
					mu.Unlock()
					return []proxmox.Snapshot{{Name: "recovered", SnapTime: 1700000300}}, nil
				}
				m.pollGuestSnapshots(context.Background(), "site", c)
				require.Len(t, calls, len(previous))
				rows = m.state.GetSnapshot().PVEBackups.GuestSnapshots
				require.Len(t, rows, len(previous)+1)
				for _, row := range rows {
					if row.Instance == "site" {
						require.Equal(t, "recovered", row.Name)
						require.Equal(t, time.Unix(1700000300, 0), row.Time)
					}
				}
			})
		})
	}
}
