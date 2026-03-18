package monitoring

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

type cancellationProbePVEClient struct {
	fakeStorageClient

	allStorageCalled     chan struct{}
	allStorageCtxDone    chan struct{}
	getDisksCalled       chan struct{}
	getDisksCtxDone      chan struct{}
	allStorageCalledOnce sync.Once
	allStorageDoneOnce   sync.Once
	getDisksCalledOnce   sync.Once
	getDisksDoneOnce     sync.Once
}

func newCancellationProbePVEClient() *cancellationProbePVEClient {
	return &cancellationProbePVEClient{
		allStorageCalled:  make(chan struct{}),
		allStorageCtxDone: make(chan struct{}),
		getDisksCalled:    make(chan struct{}),
		getDisksCtxDone:   make(chan struct{}),
	}
}

func (c *cancellationProbePVEClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	c.allStorageCalledOnce.Do(func() { close(c.allStorageCalled) })
	<-ctx.Done()
	c.allStorageDoneOnce.Do(func() { close(c.allStorageCtxDone) })
	return nil, ctx.Err()
}

func (c *cancellationProbePVEClient) GetDisks(ctx context.Context, node string) ([]proxmox.Disk, error) {
	c.getDisksCalledOnce.Do(func() { close(c.getDisksCalled) })
	<-ctx.Done()
	c.getDisksDoneOnce.Do(func() { close(c.getDisksCtxDone) })
	return nil, ctx.Err()
}

func waitForSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

func TestPollStorageAsync_UsesRuntimeContextForShutdown(t *testing.T) {
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	client := newCancellationProbePVEClient()
	monitor := &Monitor{
		runtimeCtx: runCtx,
		state:      models.NewState(),
	}

	instanceCfg := &config.PVEInstance{
		MonitorStorage: true,
	}

	if err := monitor.pollStorageAsync(context.Background(), "inst", instanceCfg, client, nil); err != nil {
		t.Fatalf("pollStorageAsync() error = %v", err)
	}

	waitForSignal(t, client.allStorageCalled, time.Second, "expected async storage polling goroutine to call GetAllStorage")

	cancelRun()
	waitForSignal(t, client.allStorageCtxDone, time.Second, "expected async storage polling context to be canceled on runtime shutdown")
}

func TestMaybePollPhysicalDisksAsync_UsesRuntimeContextForShutdown(t *testing.T) {
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()

	client := newCancellationProbePVEClient()
	monitor := &Monitor{
		runtimeCtx:           runCtx,
		state:                models.NewState(),
		lastPhysicalDiskPoll: make(map[string]time.Time),
	}

	instanceCfg := &config.PVEInstance{}
	nodes := []proxmox.Node{{Node: "node1", Status: "online"}}
	nodeEffectiveStatus := map[string]string{"node1": "online"}
	modelNodes := []models.Node{{Name: "node1"}}

	monitor.maybePollPhysicalDisksAsync(context.Background(), "inst", instanceCfg, client, nodes, nodeEffectiveStatus, modelNodes)

	waitForSignal(t, client.getDisksCalled, time.Second, "expected async disk polling goroutine to call GetDisks")

	cancelRun()
	waitForSignal(t, client.getDisksCtxDone, time.Second, "expected async disk polling context to be canceled on runtime shutdown")
}
