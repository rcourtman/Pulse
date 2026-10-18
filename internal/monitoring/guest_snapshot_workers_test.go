package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type snapshotWorkerClient struct {
	fakeSnapshotClient
	read func(context.Context, string, int) ([]proxmox.Snapshot, error)
}

func (c *snapshotWorkerClient) GetVMSnapshots(ctx context.Context, _ string, vmid int) ([]proxmox.Snapshot, error) {
	return c.read(ctx, "qemu", vmid)
}

func (c *snapshotWorkerClient) GetContainerSnapshots(ctx context.Context, _ string, vmid int) ([]proxmox.Snapshot, error) {
	return c.read(ctx, "lxc", vmid)
}

// Mix both existing snapshot readers, and keep templates/other connections
// outside the work set. These are synthetic API reads, not guest commands.
func snapshotWorkerMonitor(count int) *Monitor {
	m := &Monitor{state: models.NewState()}
	vms := make([]models.VM, 0, count)
	containers := make([]models.Container, 0, count/4)
	for i := 0; i < count; i++ {
		vmid := 1000 + i
		if i%4 == 0 {
			containers = append(containers, models.Container{ID: fmt.Sprintf("lxc/%d", vmid), VMID: vmid, Node: "member", Instance: "site"})
		} else {
			vms = append(vms, models.VM{ID: fmt.Sprintf("qemu/%d", vmid), VMID: vmid, Node: "member", Instance: "site"})
		}
	}
	vms = append(vms, models.VM{ID: "qemu/10", VMID: 10, Node: "member", Instance: "site", Template: true})
	containers = append(containers, models.Container{ID: "lxc/11", VMID: 11, Node: "member", Instance: "site", Template: true})
	m.state.UpdateGuestsForInstance("site", vms, containers)
	m.state.UpdateVMsForInstance("other", []models.VM{{ID: "qemu/12", VMID: 12, Node: "member", Instance: "other"}})
	return m
}

// Count this collector's actual goroutines, not process-global totals or just
// active HTTP requests. The old semaphore limits HTTP but leaves N waiters.
func snapshotPollWorkerGoroutines() int {
	records := make([]runtime.StackRecord, 64)
	for {
		n, ok := runtime.GoroutineProfile(records)
		if !ok {
			records = make([]runtime.StackRecord, n+16)
			continue
		}
		count := 0
		for _, record := range records[:n] {
			frames := runtime.CallersFrames(record.Stack())
			for {
				frame, more := frames.Next()
				if strings.Contains(frame.Function, ".(*Monitor).pollGuestSnapshots.func") {
					count++
					break
				}
				if !more {
					break
				}
			}
		}
		return count
	}
}

func awaitSnapshotWorkers(t *testing.T, events <-chan struct{}, count int) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for i := 0; i < count; i++ {
		select {
		case <-events:
		case <-deadline.C:
			t.Fatalf("only %d/%d snapshot reads started", i, count)
		}
	}
}

func awaitSnapshotPoll(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot poll did not return after releasing its readers")
	}
}

func TestGuestSnapshotPollingBoundsWorkersNotOnlyRequests(t *testing.T) {
	for _, count := range []int{0, 1, 7, 8, 512} {
		t.Run(fmt.Sprintf("guests=%d", count), func(t *testing.T) {
			m := snapshotWorkerMonitor(count)
			entered := make(chan struct{}, count)
			release, done := make(chan struct{}), make(chan struct{})
			var unblock sync.Once
			var inflight, peak atomic.Int32
			var mu sync.Mutex
			calls := make(map[int]int)
			client := &snapshotWorkerClient{read: func(ctx context.Context, kind string, vmid int) ([]proxmox.Snapshot, error) {
				mu.Lock()
				calls[vmid]++
				mu.Unlock()
				if (vmid%4 == 0) != (kind == "lxc") {
					t.Errorf("guest %d reached wrong reader %s", vmid, kind)
				}
				active := inflight.Add(1)
				defer inflight.Add(-1)
				for previous := peak.Load(); active > previous && !peak.CompareAndSwap(previous, active); previous = peak.Load() {
				}
				entered <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return []proxmox.Snapshot{{Name: "weekly", SnapTime: 1792028400, Description: "fixture"}}, nil
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			go func() { m.pollGuestSnapshots(ctx, "site", client); close(done) }()
			defer func() { unblock.Do(func() { close(release) }); awaitSnapshotPoll(t, done) }()
			want := min(8, count)
			awaitSnapshotWorkers(t, entered, want)
			// Let the gated fan-out reach a stable profile before recording it.
			workers, stable := -1, 0
			deadline := time.Now().Add(time.Second)
			for stable < 3 && time.Now().Before(deadline) {
				observed := snapshotPollWorkerGoroutines()
				if observed == workers {
					stable++
				} else {
					workers, stable = observed, 0
				}
				time.Sleep(10 * time.Millisecond)
			}
			t.Logf("SNAPSHOT_WORKERS guests=%d goroutines=%d inflight=%d", count, workers, inflight.Load())
			if workers != want || inflight.Load() != int32(want) {
				t.Errorf("snapshot workers/requests = %d/%d; want %d/%d", workers, inflight.Load(), want, want)
			}
			unblock.Do(func() { close(release) })
			awaitSnapshotPoll(t, done)
			if peak.Load() != int32(want) || inflight.Load() != 0 {
				t.Errorf("request bound or actual completion lost: peak=%d active=%d", peak.Load(), inflight.Load())
			}
			if workers := snapshotPollWorkerGoroutines(); workers != 0 {
				t.Errorf("%d workers remained after poll completion", workers)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(calls) != count {
				t.Fatalf("read %d guests, want %d (templates/other connection excluded)", len(calls), count)
			}
			for vmid, reads := range calls {
				if vmid < 1000 || vmid >= 1000+count || reads != 1 {
					t.Errorf("guest %d read %d times", vmid, reads)
				}
			}
			if got := m.state.GetSnapshot().PVEBackups.GuestSnapshots; len(got) != count {
				t.Fatalf("completed snapshot inventory = %d, want %d", len(got), count)
			} else {
				for _, snapshot := range got {
					wantType := "qemu"
					if snapshot.VMID%4 == 0 {
						wantType = "lxc"
					}
					if snapshot.Name != "weekly" || snapshot.Description != "fixture" || snapshot.Instance != "site" || snapshot.Node != "member" || snapshot.Type != wantType || snapshot.Time.Unix() != 1792028400 {
						t.Errorf("snapshot provenance changed: %+v", snapshot)
					}
				}
			}
		})
	}
}

func TestGuestSnapshotWorkersCancellationRetainsUnreadAndWaitsForReturn(t *testing.T) {
	const count = 128
	m := snapshotWorkerMonitor(count)
	previous := make([]models.GuestSnapshot, count)
	for i := range previous {
		kind := "qemu"
		if i%4 == 0 {
			kind = "lxc"
		}
		previous[i] = models.GuestSnapshot{ID: fmt.Sprintf("site-member-%d-retained", 1000+i), VMID: 1000 + i, Name: "retained", Node: "member", Instance: "site", Type: kind, Time: time.Unix(1792028400, 0), Description: "original evidence"}
	}
	m.state.UpdateGuestSnapshotsForInstance("site", previous)
	other := models.GuestSnapshot{ID: "other-member-12-independent", VMID: 12, Name: "independent", Node: "member", Instance: "other", Type: "qemu", Time: time.Unix(1792028300, 0), SizeBytes: 4096}
	m.state.UpdateGuestSnapshotsForInstance("other", []models.GuestSnapshot{other})
	entered := make(chan struct{}, count)
	release, done := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	var reads atomic.Int32
	client := &snapshotWorkerClient{read: func(ctx context.Context, _ string, _ int) ([]proxmox.Snapshot, error) {
		reads.Add(1)
		entered <- struct{}{}
		// A cancelled request is not evidence that its reader has returned.
		<-release
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { m.pollGuestSnapshots(ctx, "site", client); close(done) }()
	defer func() { unblock.Do(func() { close(release) }); awaitSnapshotPoll(t, done) }()
	awaitSnapshotWorkers(t, entered, 8)
	cancel()
	select {
	case <-done:
		t.Fatal("poll returned before its active readers did")
	case <-time.After(20 * time.Millisecond):
	}
	unblock.Do(func() { close(release) })
	awaitSnapshotPoll(t, done)
	if reads.Load() != 8 {
		t.Errorf("cancelled queue started %d reads, want only the original eight", reads.Load())
	}
	want := append(previous, other)
	got := m.state.GetSnapshot().PVEBackups.GuestSnapshots
	sort.Slice(want, func(i, j int) bool { return want[i].ID < want[j].ID })
	sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })
	if !reflect.DeepEqual(got, want) {
		t.Error("cancellation replaced unread evidence or changed independent connection")
	}
	if workers := snapshotPollWorkerGoroutines(); workers != 0 {
		t.Errorf("cancelled poll leaked %d workers", workers)
	}
	client.read = func(context.Context, string, int) ([]proxmox.Snapshot, error) { return nil, nil }
	m.pollGuestSnapshots(context.Background(), "site", client)
	if got := m.state.GetSnapshot().PVEBackups.GuestSnapshots; len(got) != 1 || got[0] != other {
		t.Errorf("ordinary authoritative empty read did not resume scoped replacement: %+v", got)
	}
	t.Logf("SNAPSHOT_CANCEL reads=%d unread_retained=%d independent=1 actual_return_waited=true ordinary_recovery=true", reads.Load(), count)
}

func TestGuestSnapshotWorkersKeepCompletedProgressWhenQueueIsCancelled(t *testing.T) {
	const count = 32
	m := snapshotWorkerMonitor(count)
	previous := make([]models.GuestSnapshot, count)
	for i := range previous {
		kind := "qemu"
		if i%4 == 0 {
			kind = "lxc"
		}
		previous[i] = models.GuestSnapshot{ID: fmt.Sprintf("site-member-%d-retained", 1000+i), VMID: 1000 + i, Name: "retained", Node: "member", Instance: "site", Type: kind, Time: time.Unix(1792028400, 0)}
	}
	m.state.UpdateGuestSnapshotsForInstance("site", previous)
	entered := make(chan struct{}, count)
	release, done := make(chan struct{}), make(chan struct{})
	var unblock sync.Once
	var reads atomic.Int32
	client := &snapshotWorkerClient{read: func(ctx context.Context, _ string, vmid int) ([]proxmox.Snapshot, error) {
		reads.Add(1)
		switch vmid {
		case 1001:
			return []proxmox.Snapshot{{Name: "fresh", SnapTime: 1792028460}}, nil
		case 1002:
			return nil, nil // An observed empty list may replace this guest only.
		case 1003:
			return nil, fmt.Errorf("fixture inventory unavailable")
		}
		entered <- struct{}{}
		<-release
		return nil, ctx.Err()
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { m.pollGuestSnapshots(ctx, "site", client); close(done) }()
	defer func() { unblock.Do(func() { close(release) }); awaitSnapshotPoll(t, done) }()
	awaitSnapshotWorkers(t, entered, 8)
	cancel()
	unblock.Do(func() { close(release) })
	awaitSnapshotPoll(t, done)
	if reads.Load() != 11 {
		t.Errorf("partial poll started %d reads; want three completed and eight active", reads.Load())
	}
	want := make([]models.GuestSnapshot, 0, count-1)
	for _, snapshot := range previous {
		if snapshot.VMID != 1001 && snapshot.VMID != 1002 {
			want = append(want, snapshot)
		}
	}
	want = append(want, models.GuestSnapshot{ID: "site-member-1001-fresh", VMID: 1001, Name: "fresh", Node: "member", Instance: "site", Type: "qemu", Time: time.Unix(1792028460, 0), VMState: true})
	got := m.state.GetSnapshot().PVEBackups.GuestSnapshots
	sort.Slice(want, func(i, j int) bool { return want[i].ID < want[j].ID })
	sort.Slice(got, func(i, j int) bool { return got[i].ID < got[j].ID })
	if !reflect.DeepEqual(got, want) {
		t.Errorf("partial cancellation lost completed or unread evidence: got=%+v want=%+v", got, want)
	}
	t.Logf("SNAPSHOT_PARTIAL started=%d fresh=1 observed_empty=1 unavailable=1 retained=%d", reads.Load(), count-2)
}

func BenchmarkGuestSnapshotFleetPoll(b *testing.B) {
	for _, count := range []int{128, 1024} {
		b.Run(fmt.Sprintf("guests=%d", count), func(b *testing.B) {
			m := snapshotWorkerMonitor(count)
			client := &snapshotWorkerClient{read: func(context.Context, string, int) ([]proxmox.Snapshot, error) {
				return []proxmox.Snapshot{{Name: "weekly", SnapTime: 1792028400}}, nil
			}}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.pollGuestSnapshots(context.Background(), "site", client)
			}
			b.StopTimer()
			if got := len(m.state.GetSnapshot().PVEBackups.GuestSnapshots); got != count {
				b.Fatalf("completed snapshot inventory = %d, want %d", got, count)
			}
		})
	}
}
