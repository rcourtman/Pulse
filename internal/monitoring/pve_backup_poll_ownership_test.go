package monitoring

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// This adapter only blocks a read of synthetic task inventory. No guest, QGA,
// backup, installed service, storage fault or database is exercised.
type blockedBackupPollClient struct {
	PVEClientInterface
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (c *blockedBackupPollClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	c.calls.Add(1)
	c.entered <- struct{}{}
	select {
	case <-c.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestPVEBackupAsyncKeepsOneScanInFlight(t *testing.T) {
	for _, mode := range []string{"cycle-due", "cadence-reset"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &blockedBackupPollClient{entered: make(chan struct{}, 16), release: make(chan struct{})}
			var once sync.Once
			defer once.Do(func() { close(client.release) })
			m := &Monitor{state: models.NewState(), runtimeCtx: ctx, lastPVEBackupPoll: make(map[string]time.Time), config: &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}}
			if mode == "cadence-reset" {
				m.config.BackupPollingInterval = time.Hour
			}
			start := func() { m.pollPVEBackupsAsync("site", &config.PVEInstance{MonitorBackups: true}, client, nil, nil) }
			start()
			select {
			case <-client.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("first synthetic inventory read did not start")
			}
			if mode == "cadence-reset" {
				m.SetBackupPollingInterval(time.Minute)
			}
			for i := 0; i < 8; i++ {
				start()
			}
			select {
			case <-client.entered:
				t.Fatalf("due scheduling admitted overlapping backup inventory: calls=%d", client.calls.Load())
			case <-time.After(150 * time.Millisecond):
			}
			t.Logf("BACKUP_POLL_OWNERSHIP_%s requests=%d while first read remains blocked", mode, client.calls.Load())
		})
	}
}
