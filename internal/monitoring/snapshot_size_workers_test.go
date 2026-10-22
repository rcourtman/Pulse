package monitoring

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type snapshotSizeWorkerClient struct {
	fakeSnapshotClient
	storage func(context.Context, string) ([]proxmox.Storage, error)
	content func(context.Context, string, string) ([]proxmox.StorageContent, error)
}

func (c snapshotSizeWorkerClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	return c.storage(ctx, node)
}

func (c snapshotSizeWorkerClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	return c.content(ctx, node, storage)
}

func snapshotSizeWorkerInputs(count int) []models.GuestSnapshot {
	inputs := make([]models.GuestSnapshot, count)
	for i := range inputs {
		node := fmt.Sprintf("node-%03d", i)
		inputs[i] = models.GuestSnapshot{ID: "site-" + node + "-100-weekly", Instance: "site", Node: node, VMID: 100, Name: "weekly"}
	}
	return inputs
}

func snapshotSizeWorkerCount() int {
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
				if strings.Contains(frame.Function, ".(*Monitor).collectSnapshotSizes.func") {
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

func TestSnapshotSizePollingBoundsWorkersAndRequests(t *testing.T) {
	for _, count := range []int{0, 1, 7, 8, 128} {
		t.Run(fmt.Sprintf("nodes=%d", count), func(t *testing.T) {
			entered := make(chan struct{}, count)
			release, done := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			t.Cleanup(unblock)
			var active, peak, lists, contents atomic.Int32
			client := snapshotSizeWorkerClient{
				storage: func(ctx context.Context, node string) ([]proxmox.Storage, error) {
					lists.Add(1)
					n := active.Add(1)
					defer active.Add(-1)
					for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
					}
					entered <- struct{}{}
					<-release
					return []proxmox.Storage{{Storage: "fast", Content: "images", Active: 1, Enabled: 1}}, nil
				},
				content: func(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
					contents.Add(1)
					return []proxmox.StorageContent{{Volid: "fast:vm-100-disk-0@weekly", VMID: 100, Content: "images", Size: 1024}}, nil
				},
			}
			var totals map[string]int64
			go func() {
				totals = (&Monitor{}).collectSnapshotSizes(context.Background(), "site", client, snapshotSizeWorkerInputs(count))
				close(done)
			}()
			awaitSnapshotWorkers(t, entered, min(count, 8))
			if got := snapshotSizeWorkerCount(); got != min(count, 8) {
				t.Errorf("actual sizing workers = %d, want %d, not one per node", got, min(count, 8))
			}
			unblock()
			awaitSnapshotPoll(t, done)
			if peak.Load() != int32(min(count, 8)) || lists.Load() != int32(count) || contents.Load() != int32(count) || len(totals) != count {
				t.Fatalf("worker/request/result census: peak=%d lists=%d contents=%d totals=%d nodes=%d", peak.Load(), lists.Load(), contents.Load(), len(totals), count)
			}
			for _, input := range snapshotSizeWorkerInputs(count) {
				if totals[input.ID] != 1024 {
					t.Fatalf("same-VMID nodes lost isolation: %s = %d", input.ID, totals[input.ID])
				}
			}
		})
	}
}

func TestSnapshotSizeCancellationJoinsReadersAndKeepsCompletedPeer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, cancelled := make(chan struct{}, 8), make(chan struct{}, 8)
	release, done := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	var lists, contents atomic.Int32
	client := snapshotSizeWorkerClient{
		storage: func(ctx context.Context, node string) ([]proxmox.Storage, error) {
			lists.Add(1)
			return []proxmox.Storage{{Storage: "fast", Content: "images", Active: 1, Enabled: 1}}, nil
		},
		content: func(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
			contents.Add(1)
			if node == "node-000" {
				return []proxmox.StorageContent{{Volid: "fast:vm-100-disk-0@weekly", VMID: 100, Content: "images", Size: 2048}}, nil
			}
			entered <- struct{}{}
			<-ctx.Done()
			cancelled <- struct{}{}
			// Model a reader that observes cancellation before it returns.
			// Cancellation itself is not proof of released occupancy.
			<-release
			return nil, ctx.Err()
		},
	}
	var totals map[string]int64
	go func() {
		totals = (&Monitor{}).collectSnapshotSizes(ctx, "site", client, snapshotSizeWorkerInputs(64))
		close(done)
	}()
	// Eight blocked nodes means the first worker has already published the
	// complete node-000 total and moved to another job. No timing sleep.
	awaitSnapshotWorkers(t, entered, 8)
	cancel()
	awaitSnapshotWorkers(t, cancelled, 8)
	select {
	case <-done:
		t.Fatal("sizing released ownership while cancelled readers were still occupied")
	default:
	}
	unblock()
	awaitSnapshotPoll(t, done)
	if lists.Load() != 9 || contents.Load() != 9 || len(totals) != 1 || totals["site-node-000-100-weekly"] != 2048 {
		t.Fatalf("cancelled nodes became partial sizes or discarded completed peer: lists=%d contents=%d totals=%v", lists.Load(), contents.Load(), totals)
	}
}

func TestSnapshotSizePreCancelledScanDoesNotRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := snapshotSizeWorkerClient{storage: func(context.Context, string) ([]proxmox.Storage, error) {
		t.Error("cancelled scan dispatched a storage read")
		return nil, nil
	}}
	if got := (&Monitor{}).collectSnapshotSizes(ctx, "site", client, snapshotSizeWorkerInputs(64)); len(got) != 0 {
		t.Fatal(got)
	}
}
