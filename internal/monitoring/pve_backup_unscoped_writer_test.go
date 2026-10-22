package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Real read-only HTTP adapters -> ordinary scan -> archive/age/recovery
// consumers. A scheduled parent's current log is not a complete guest census:
// it can fail, wait behind the log budget, or end after the previous guest.
// No guest operation, database, installed service or external provider is used.
func TestPVEArchiveScanRetainsArchivesDuringUnscopedWriters(t *testing.T) {
	for _, cluster := range []bool{false, true} {
		t.Run(fmt.Sprintf("cluster=%t", cluster), func(t *testing.T) {
			at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
			fixture := &taskBarrierClient{at: at}
			var snapshotStarted atomic.Bool
			var logReads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				var data any
				switch {
				case r.URL.Path == "/api2/json/nodes":
					data = []proxmox.Node{{Node: "node", Status: "online"}}
				case r.URL.Path == "/api2/json/access/permissions":
					require.Equal(t, "/nodes/node", r.URL.Query().Get("path"))
					data = map[string]any{"/nodes/node": map[string]int{"Sys.Audit": 1}}
				case r.URL.Path == "/api2/json/nodes/node/tasks":
					fixture.record("tasks")
					require.Equal(t, "all", r.URL.Query().Get("source"))
					tasks := []proxmox.Task{}
					if fixture.phase != "initial" && fixture.phase != "empty" {
						id, status, end := "", "running", int64(0)
						if fixture.phase == "running-known" {
							id = "100"
						} else if fixture.phase == "finished" {
							status, end = "OK", at.Add(time.Minute).Unix()
						} else if fixture.phase == "missing-status" {
							status = ""
						}
						count := 1
						if fixture.phase == "deferred" {
							count = maxVzdumpJobLogFetchesPerPoll + 1
						}
						for i := 0; i < count; i++ {
							tasks = append(tasks, proxmox.Task{UPID: fmt.Sprintf("parent-%s-%d", fixture.phase, i), Node: "node", Type: "vzdump", ID: id, Status: status, StartTime: at.Add(-2 * time.Minute).Unix(), EndTime: end})
						}
					}
					data = tasks
				case strings.HasSuffix(r.URL.Path, "/log"):
					logReads.Add(1)
					if fixture.phase == "unreadable" || fixture.phase == "finished" {
						w.WriteHeader(http.StatusForbidden)
						return
					}
					data = []proxmox.TaskLogLine{}
					if fixture.phase == "previous-guest-finished" {
						// This is valid positive evidence for guest 101, not a
						// promise that the parent cannot start guest 100 next.
						data = testLogLines("INFO: Starting Backup of VM 101", "INFO: Finished Backup of VM 101 (00:01:00)")
					}
				case r.URL.Path == "/api2/json/nodes/node/storage":
					if snapshotStarted.Load() {
						data = []proxmox.Storage{}
					} else {
						data, _ = fixture.GetStorage(r.Context(), "node")
					}
				case r.URL.Path == "/api2/json/nodes/node/storage/archive/content":
					data, _ = fixture.GetStorageContent(r.Context(), "node", "archive")
					if fixture.phase == "empty" {
						data = []proxmox.StorageContent{}
					}
				case strings.HasSuffix(r.URL.Path, "/qemu/100/snapshot"):
					snapshotStarted.Store(true)
					data, _ = fixture.GetVMSnapshots(r.Context(), "node", 100)
				default:
					t.Errorf("unexpected read %s", r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: 2 * time.Second}
			var client PVEClientInterface
			if cluster {
				client = proxmox.NewClusterClient("unscoped", cfg, []string{server.URL}, nil)
			} else {
				var err error
				client, err = proxmox.NewClient(cfg)
				require.NoError(t, err)
			}
			m := backupLifecycleMonitor(client)
			t.Cleanup(func() { awaitBackupProjectionIngest(m) })
			peer := models.StorageBackup{ID: "independent", Instance: "other", Node: "node", Storage: "archive", VMID: 100, Type: "qemu", Time: at.Add(-time.Hour), CTime: at.Add(-time.Hour).Unix(), Size: 8192}
			m.state.UpdateStorageBackupsForInstance("other", []models.StorageBackup{peer})
			frames := map[string]models.PVEBackups{}
			var held []models.StorageBackup
			for _, phase := range []string{"initial", "unreadable", "missing-status", "previous-guest-finished", "deferred", "running-known", "finished", "empty"} {
				fixture.phase, fixture.calls = phase, nil
				snapshotStarted.Store(false)
				logReads.Store(0)
				m.pollPVEBackupsAndSnapshots(context.Background(), "site", client, []proxmox.Node{{Node: "node", Status: "online"}}, map[string]string{"node": "online"}, 2*time.Second)
				awaitBackupProjectionIngest(m)
				got := m.PVEBackupsSnapshot()
				var own []models.StorageBackup
				for _, backup := range got.StorageBackups {
					if backup.Instance == "other" {
						require.Equal(t, peer, backup, "same-VMID peer evidence is independent")
					} else {
						own = append(own, backup)
					}
				}
				require.Len(t, got.StorageBackups, len(own)+1)
				require.Len(t, got.GuestSnapshots, 1, "unscoped archives must not block independent snapshots")
				require.Equal(t, "snapshot-"+phase, got.GuestSnapshots[0].Name)
				switch phase {
				case "initial":
					require.Len(t, own, 1)
					held = own
				case "unreadable", "missing-status", "previous-guest-finished", "deferred":
					require.Equal(t, held, own, "a running unidentified writer cannot add, remove or renew archives")
					require.NotContains(t, fixture.calls, "content", "do not classify unknown output using incomplete guest-log attribution")
					require.NotEmpty(t, got.BackupTasks, "positive live task observations remain useful")
				case "running-known", "finished":
					require.Contains(t, fixture.calls, "content", "scoped active and terminal parents allow normal archive collection")
					require.Len(t, own, 2)
					for _, backup := range own {
						if backup.ID != held[0].ID {
							require.Equal(t, phase == "running-known", backup.InProgress)
						}
					}
				case "empty":
					require.Empty(t, own, "ordinary complete emptiness still removes vanished archives")
				}
				if phase == "deferred" {
					require.EqualValues(t, maxVzdumpJobLogFetchesPerPoll, logReads.Load(), "do not expand the log budget to qualify output")
					require.Len(t, got.BackupTasks, maxVzdumpJobLogFetchesPerPoll+1)
				} else if phase == "previous-guest-finished" {
					require.Len(t, got.BackupTasks, 2, "the observed sibling still has its per-guest result")
					require.Equal(t, "OK", got.BackupTasks[1].Status)
				}
				if phase != "empty" {
					want := held[0].Time
					if phase == "finished" {
						want = at
					}
					require.True(t, want.Equal(m.state.GetSnapshot().VMs[0].LastBackup), "an unidentified writer must not renew Last Backup")
				}
				points := proxmoxmapper.FromPVEStorageBackups(own, nil)
				successes := 0
				for _, point := range points {
					if point.Outcome == recovery.OutcomeSuccess {
						successes++
					}
				}
				want := 1
				if phase == "finished" {
					want = 2
				} else if phase == "empty" {
					want = 0
				}
				require.Equal(t, want, successes, "growing output is not a successful restore artifact")
				frames[phase] = got
			}
			if !cluster {
				wire, err := json.Marshal(frames)
				require.NoError(t, err)
				t.Logf("PVE_UNSCOPED_WRITER_FRAMES=%s", wire)
			}
		})
	}
}

func TestPVEArchiveAdmissionUsesLiveWriterScope(t *testing.T) {
	for _, id := range []string{"", "0", "-1", "invalid"} {
		t.Run("id="+id, func(t *testing.T) {
			for _, history := range []bool{false, true} {
				t.Run(fmt.Sprintf("history=%t", history), func(t *testing.T) {
					client := &unscopedWriterHistoryClient{lifecycleBackupClient: lifecycleBackupClient{label: "writer"}, id: id}
					m := backupLifecycleMonitor(client)
					if history {
						m.recoveryManager = recoverymanager.New(nil) // No database.
					}
					require.False(t, m.pollBackupTasks(context.Background(), "site", client), "missing/nonpositive/malformed live guest scope cannot qualify new archives, including after terminal history backfill")
					require.NotEmpty(t, m.PVEBackupsSnapshot().BackupTasks, "keep observed positives while holding archive admission")
					if history {
						require.Equal(t, "OK", m.PVEBackupsSnapshot().BackupTasks[0].Status, "history can still update positive outcomes, not live archive admission")
					}
				})
			}
		})
	}
}

type unscopedWriterHistoryClient struct {
	lifecycleBackupClient
	id string
}

func (c *unscopedWriterHistoryClient) GetBackupTasks(ctx context.Context) ([]proxmox.Task, error) {
	tasks, err := c.lifecycleBackupClient.GetBackupTasks(ctx)
	tasks[0].ID, tasks[0].Status, tasks[0].EndTime = c.id, "running", 0
	return tasks, err
}

func (c *unscopedWriterHistoryClient) GetBackupTaskHistory(ctx context.Context, _ time.Time) (proxmox.BackupTaskHistory, error) {
	tasks, err := c.lifecycleBackupClient.GetBackupTasks(ctx)
	// This control advances completion of the same scheduled parent, not a
	// different guest identity that merely reuses its UPID.
	tasks[0].ID = c.id
	return proxmox.BackupTaskHistory{Tasks: tasks, Nodes: []proxmox.BackupTaskHistoryNode{{Node: "node", Complete: true}}}, err
}
