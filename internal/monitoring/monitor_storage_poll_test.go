package monitoring

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type blockingStoragePollPVEClient struct {
	stubPVEClient
	started chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (c *blockingStoragePollPVEClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	if c.calls.Add(1) == 1 {
		close(c.started)
		select {
		case <-c.release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, nil
}

func TestPollStorageAsync_SkipsOverlappingPollForSameInstance(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := &blockingStoragePollPVEClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	monitor := &Monitor{
		runtimeCtx: runCtx,
		state:      models.NewState(),
	}
	cfg := &config.PVEInstance{MonitorStorage: true}

	monitor.pollStorageAsync("pve1", cfg, client, nil)
	waitForSignal(t, client.started, time.Second, "expected first storage poll to start")

	// A second poll while the first is still running for the same instance must be skipped.
	monitor.pollStorageAsync("pve1", cfg, client, nil)
	monitor.pollStorageAsync("pve1", cfg, client, nil)
	if got := client.calls.Load(); got != 1 {
		t.Fatalf("overlapping GetAllStorage calls = %d, want 1", got)
	}

	close(client.release)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		monitor.storagePollMu.Lock()
		busy := monitor.storagePollInFlight["pve1"]
		monitor.storagePollMu.Unlock()
		if !busy {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Once finished, the next storage poll for the same instance proceeds normally.
	monitor.pollStorageAsync("pve1", cfg, client, nil)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if client.calls.Load() >= 2 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("expected second storage poll after first completed, got calls=%d", client.calls.Load())
}
