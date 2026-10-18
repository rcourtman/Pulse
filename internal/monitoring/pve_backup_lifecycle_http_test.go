package monitoring

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

func TestPVEBackupLifecycleHTTP(t *testing.T) {
	for _, stage := range []string{"task", "task-log", "storage", "snapshot"} {
		t.Run(stage, func(t *testing.T) {
			entered, cancelled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var retired, currentMode atomic.Bool
			var oldReadsAfterWithdrawal atomic.Int32
			now := time.Now().Add(-time.Hour).Unix()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("unexpected provider mutation %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				if retired.Load() && !currentMode.Load() {
					oldReadsAfterWithdrawal.Add(1)
				}
				w.Header().Set("Content-Type", "application/json")
				label := "withdrawn"
				if currentMode.Load() {
					label = "current"
				}
				readStage := ""
				var data any = []any{}
				switch {
				case r.URL.Path == "/api2/json/nodes":
					data = []proxmox.Node{{Node: "node", Status: "online"}}
				case r.URL.Path == "/api2/json/nodes/node/tasks":
					readStage = "task"
					id := "100"
					if stage == "task-log" && !currentMode.Load() {
						id = ""
					}
					data = []proxmox.Task{{UPID: "UPID:node:00000001:00000001:6553F100:vzdump:" + id + ":root@pam:", Node: "node", ID: id, Type: "vzdump", Status: "OK", StartTime: now, EndTime: now + 60}}
				case strings.HasSuffix(r.URL.Path, "/log"):
					readStage = "task-log"
					data = []proxmox.TaskLogLine{{LineNumber: 1, Text: "INFO: Starting Backup of VM 100"}, {LineNumber: 2, Text: "INFO: Finished Backup of VM 100 (00:01:00)"}, {LineNumber: 3, Text: "TASK OK"}}
				case r.URL.Path == "/api2/json/nodes/node/storage":
					data = []proxmox.Storage{{Storage: "local", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}
				case r.URL.Path == "/api2/json/nodes/node/storage/local/content":
					readStage = "storage"
					data = []proxmox.StorageContent{{Volid: "local:backup/vzdump-qemu-100-" + label + ".vma.zst", Content: "backup", VMID: 100, CTime: now + 60, Size: 1024}}
				case strings.HasSuffix(r.URL.Path, "/qemu/100/snapshot"):
					readStage = "snapshot"
					data = []proxmox.Snapshot{{Name: label, SnapTime: now + 60}}
				default:
					t.Errorf("unexpected synthetic provider read %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if readStage == stage && !currentMode.Load() {
					// Consume headers before withdrawal, then retain the controlled
					// handler until the test releases it. Reader cancellation is
					// observed independently of handler completion.
					w.WriteHeader(http.StatusOK)
					w.(http.Flusher).Flush()
					close(entered)
					<-r.Context().Done()
					close(cancelled)
					<-release
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer server.Close()
			defer unblock()
			client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 10 * time.Second})
			require.NoError(t, err)
			m := backupLifecycleMonitor(client)
			startBackupLifecycleScan(m, client)
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("HTTP scan did not reach controlled read")
			}
			m.retirePVEInstanceRuntime("site")
			retired.Store(true)
			select {
			case <-cancelled:
			case <-time.After(5 * time.Second):
				t.Fatal("retirement did not cancel consumed HTTP read")
			}
			waitForBackupPollReturn(t, m, "site")
			unblock()
			empty := m.PVEBackupsSnapshot()
			require.Empty(t, empty.BackupTasks)
			require.Empty(t, empty.StorageBackups)
			require.Empty(t, empty.GuestSnapshots)
			require.Zero(t, oldReadsAfterWithdrawal.Load(), "withdrawn scan must not dispatch a later phase")
			currentMode.Store(true)
			fresh, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 10 * time.Second})
			require.NoError(t, err)
			m.replacePVEClient("site", fresh)
			m.state.UpdateVMsForInstance("site", []models.VM{{ID: "site:node:100", Instance: "site", Node: "node", VMID: 100, Name: "fixture-guest", Status: "running"}})
			startBackupLifecycleScan(m, fresh)
			waitForBackupPollReturn(t, m, "site")
			current := m.PVEBackupsSnapshot()
			require.Len(t, current.BackupTasks, 1)
			require.Len(t, current.StorageBackups, 1)
			require.Len(t, current.GuestSnapshots, 1)
			if stage == "snapshot" {
				payload, err := json.Marshal(map[string]models.PVEBackups{"retired": empty, "current": current})
				require.NoError(t, err)
				t.Logf("BACKUP_BROWSER_PRODUCER=%s", base64.StdEncoding.EncodeToString(payload))
			}
			t.Logf("BACKUP_HTTP_WITHDRAWAL_%s reader_cancelled=true next_phase_requests=0 retired=0/0/0 current=1/1/1", stage)
		})
	}
}

func TestPVEBackupLifecycleEndedRuntimeCannotStart(t *testing.T) {
	client := &lifecycleBackupClient{label: "current"}
	m := backupLifecycleMonitor(client)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.setRuntimeContext(ctx, nil)
	owner, _ := m.beginPVEBackupPoll("site", time.Now(), client)
	require.Nil(t, owner)
	require.Empty(t, m.lastPVEBackupPoll, "unstarted work cannot advance cadence")
}

func TestPVEBackupLifecyclePublicationOrdersRetirement(t *testing.T) {
	client := &lifecycleBackupClient{label: "current"}
	m := backupLifecycleMonitor(client)
	owner, _ := m.beginPVEBackupPoll("site", time.Now(), client)
	require.NotNil(t, owner)
	defer m.finishPVEBackupPoll("site", owner)
	ctx := context.WithValue(owner.ctx, pveBackupPollContextKey{}, owner)
	entered, release, published := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	go func() {
		m.applyPVEBackupEffect(ctx, "site", func() {
			close(entered)
			<-release
			m.state.UpdateGuestSnapshotsForInstance("site", []models.GuestSnapshot{{Instance: "site", Name: "accepted"}})
		})
		close(published)
	}()
	<-entered
	retired := make(chan struct{})
	go func() { m.retirePVEInstanceRuntime("site"); close(retired) }()
	select {
	case <-retired:
		t.Error("retirement interleaved an already admitted publication")
	case <-time.After(50 * time.Millisecond):
	}
	unblock()
	<-published
	<-retired
	require.Empty(t, m.PVEBackupsSnapshot().GuestSnapshots, "retirement must clear the accepted publication")
	m.applyPVEBackupEffect(ctx, "site", func() { t.Error("retired owner was admitted again") })
	t.Log("BACKUP_PUBLICATION_ORDER accepted_then_retired=true late_effects=0")
}
