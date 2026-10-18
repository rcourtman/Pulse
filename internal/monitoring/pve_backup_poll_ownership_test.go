package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// This adapter only blocks a read of synthetic task inventory. No guest, QGA,
// backup, installed service, storage fault or database is exercised.
type blockedBackupPollClient struct {
	PVEClientInterface
	entered            chan struct{}
	release            chan struct{}
	calls              atomic.Int32
	ignoreCancellation bool
}

func (c *blockedBackupPollClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	c.calls.Add(1)
	c.entered <- struct{}{}
	if c.ignoreCancellation {
		<-c.release
		return nil, nil
	}
	select {
	case <-c.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func waitForBackupPollReturn(t *testing.T, m *Monitor, instance string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		m.mu.RLock()
		active := m.pveBackupPollInFlight[instance]
		m.mu.RUnlock()
		if !active {
			return
		}
		select {
		case <-tick.C:
		case <-deadline.C:
			t.Fatal("backup scan did not return and release ownership")
		}
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
			once.Do(func() { close(client.release) })
			waitForBackupPollReturn(t, m, "site")
			require.Equal(t, int32(1), client.calls.Load(), "coalesced triggers must not queue another scan after return")
		})
	}
}

func TestPVEBackupPollAdmissionIsAtomicAndConnectionScoped(t *testing.T) {
	m := &Monitor{config: &config.Config{EnableBackupPolling: true, BackupPollingInterval: time.Minute}}
	now := time.Now()
	var winners atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if run, _ := m.beginPVEBackupPoll("site", now); run {
				winners.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	require.Equal(t, int32(1), winners.Load())
	run, _ := m.beginPVEBackupPoll("site", now.Add(24*time.Hour))
	require.False(t, run, "elapsed cadence is not completion")
	run, _ = m.beginPVEBackupPoll("other-site", now)
	require.True(t, run, "one slow connection must not block its peers")
	m.finishPVEBackupPoll("site", now)
	m.finishPVEBackupPoll("other-site", now)
	require.Empty(t, m.pveBackupPollInFlight)
	completed := m.lastPVEBackupPoll["site"]
	require.False(t, completed.Before(now))
	run, _ = m.beginPVEBackupPoll("site", completed.Add(time.Second))
	require.False(t, run, "ordinary completion retains configured cadence")
	run, _ = m.beginPVEBackupPoll("site", completed.Add(time.Minute))
	require.True(t, run, "next due scan must resume")
	m.finishPVEBackupPoll("site", completed.Add(time.Minute))
}

func TestPVEBackupPollResetPreservesOwnerAndCatchUp(t *testing.T) {
	for _, reset := range []string{"shorter-interval", "re-enabled", "removed-timestamp"} {
		t.Run(reset, func(t *testing.T) {
			m := &Monitor{config: &config.Config{EnableBackupPolling: true, BackupPollingInterval: time.Hour}}
			now := time.Now()
			run, _ := m.beginPVEBackupPoll("site", now)
			require.True(t, run)
			switch reset {
			case "shorter-interval":
				m.SetBackupPollingInterval(time.Minute)
			case "re-enabled":
				m.SetBackupPollingEnabled(false)
				m.SetBackupPollingEnabled(true)
			case "removed-timestamp":
				m.mu.Lock()
				delete(m.lastPVEBackupPoll, "site")
				m.mu.Unlock()
			}
			run, _ = m.beginPVEBackupPoll("site", now)
			require.False(t, run, "timestamp reset cannot discard an in-flight owner")
			m.finishPVEBackupPoll("site", now)
			require.Empty(t, m.lastPVEBackupPoll, "old completion must not overwrite the catch-up request")
			run, _ = m.beginPVEBackupPoll("site", now)
			require.True(t, run, "catch-up is eligible only after actual return")
			m.finishPVEBackupPoll("site", now)
		})
	}
}

func TestPVEBackupAsyncCancellationIsNotCompletion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &blockedBackupPollClient{entered: make(chan struct{}, 16), release: make(chan struct{}), ignoreCancellation: true}
	var once sync.Once
	defer once.Do(func() { close(client.release) })
	m := &Monitor{state: models.NewState(), runtimeCtx: ctx, config: &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}}
	start := func() { m.pollPVEBackupsAsync("site", &config.PVEInstance{MonitorBackups: true}, client, nil, nil) }
	start()
	select {
	case <-client.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first synthetic inventory read did not start")
	}
	cancel()
	m.setRuntimeContext(context.Background(), nil)
	m.resetBackupPollTimestamps()
	run, _ := m.beginPVEBackupPoll("site", time.Now())
	require.False(t, run, "cancelled read still owns its connection until it returns")
	once.Do(func() { close(client.release) })
	waitForBackupPollReturn(t, m, "site")
	start()
	waitForBackupPollReturn(t, m, "site")
	require.Equal(t, int32(2), client.calls.Load(), "ordinary polling resumes after the old reader returns")
}

// Exercise the actual client and the whole asynchronous scan through task,
// storage and snapshot reads. The storage/snapshot stages must retain the same
// owner; task-reader return alone must not admit a second scan.
func TestPVEBackupAsyncOwnsCompleteHTTPScan(t *testing.T) {
	for _, blockedStage := range []string{"storage", "snapshot"} {
		t.Run(blockedStage, func(t *testing.T) {
			entered, release := make(chan struct{}, 16), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var taskReads, storageReads, snapshotReads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected mutation %s %s", r.Method, r.URL.Path)
				}
				stage := ""
				switch {
				case r.URL.Path == "/api2/json/nodes":
					fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
					return
				case r.URL.Path == "/api2/json/nodes/node/tasks":
					taskReads.Add(1)
				case r.URL.Path == "/api2/json/nodes/node/storage":
					storageReads.Add(1)
					stage = "storage"
				case strings.HasSuffix(r.URL.Path, "/qemu/100/snapshot"):
					snapshotReads.Add(1)
					stage = "snapshot"
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if stage == blockedStage {
					entered <- struct{}{}
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				fmt.Fprint(w, `{"data":[]}`)
			}))
			defer server.Close()
			defer unblock()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 10 * time.Second})
			require.NoError(t, err)
			m := &Monitor{state: models.NewState(), config: &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1}}
			m.state.UpdateVMsForInstance("site", []models.VM{{ID: "site-node-100", Instance: "site", Node: "node", VMID: 100, Status: "running"}})
			start := func() {
				m.pollPVEBackupsAsync("site", &config.PVEInstance{MonitorBackups: true}, client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"})
			}
			start()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatalf("HTTP scan did not reach %s", blockedStage)
			}
			for i := 0; i < 8; i++ {
				start()
			}
			unblock()
			waitForBackupPollReturn(t, m, "site")
			require.Equal(t, int32(1), taskReads.Load())
			require.Equal(t, int32(1), storageReads.Load())
			require.Equal(t, int32(1), snapshotReads.Load())
			start()
			waitForBackupPollReturn(t, m, "site")
			require.Equal(t, int32(2), taskReads.Load())
			require.Equal(t, int32(2), storageReads.Load())
			require.Equal(t, int32(2), snapshotReads.Load())
			t.Logf("BACKUP_HTTP_%s coalesced=8 first_scan=1/1/1 resumed=2/2/2", blockedStage)
		})
	}
}
