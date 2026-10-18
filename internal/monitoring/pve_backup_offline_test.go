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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Real read-only HTTP -> direct and cluster adapters -> ordinary polling,
// per-node state replacement and job-log cache. No native/guest/backup action.
func TestPVEOfflineBackupTasksRetainAgeAndHealthyPeerProgress(t *testing.T) {
	for _, clustered := range []bool{false, true} {
		name := "direct"
		if clustered {
			name = "cluster"
		}
		t.Run(name, func(t *testing.T) {
			var phase, bReads, logReads atomic.Int32
			const start = int64(1792281600)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				p := phase.Load()
				switch r.URL.Path {
				case "/api2/json/nodes":
					if p == 5 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					a, b := "online", "online"
					if p >= 1 && p <= 3 {
						b = "offline"
					}
					if p == 3 {
						a = "offline"
					}
					fmt.Fprintf(w, `{"data":[{"node":"pve-a","status":%q},{"node":"pve-b","status":%q}]}`, a, b)
				case "/api2/json/nodes/pve-a/tasks":
					require.NotEqual(t, int32(3), p, "offline node must not be queried")
					if p >= 2 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					id, status := "a-old", "ERROR: fixture storage unavailable"
					if p == 1 {
						id, status = "a-next", "OK"
					}
					// Missing row.node is bound to the actual request scope.
					fmt.Fprintf(w, `{"data":[{"upid":%q,"type":"vzdump","id":"100","status":%q,"starttime":%d,"endtime":%d}]}`, id, status, start, start+60)
				case "/api2/json/nodes/pve-b/tasks":
					bReads.Add(1)
					require.False(t, p >= 1 && p <= 3, "offline node must not be queried")
					if p == 4 {
						fmt.Fprint(w, `{"data":[]}`)
						return
					}
					fmt.Fprintf(w, `{"data":[{"upid":"b-job","node":"pve-b","type":"vzdump","status":"WARNINGS: 1","starttime":%d,"endtime":%d},{"upid":"b-running","node":"pve-b","type":"vzdump","id":"300","starttime":%d}]}`, start, start+60, start+120)
				default:
					if strings.HasSuffix(r.URL.Path, "/b-job/log") {
						logReads.Add(1)
						require.False(t, p >= 1 && p <= 3, "offline job must not be reread")
						fmt.Fprint(w, `{"data":[{"n":1,"t":"INFO: Starting Backup of VM 200 (qemu)"},{"n":2,"t":"WARN: synthetic-private-warning"},{"n":3,"t":"INFO: Finished Backup of VM 200 (00:01:00)"},{"n":4,"t":"TASK WARNINGS: 1"}]}`)
						return
					}
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			cfg := proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pve!pulse", TokenValue: "fixture", Timeout: time.Second}
			var client PVEClientInterface
			if clustered {
				cc := proxmox.NewClusterClient("fixture", cfg, []string{server.URL}, nil)
				client = cc
			} else {
				c, err := proxmox.NewClient(cfg)
				require.NoError(t, err)
				client = c
			}
			m := &Monitor{state: models.NewState()}
			unrelated := models.BackupTask{ID: "other-running", Instance: "other", Node: "pve-b", VMID: 200, Status: "running", StartTime: time.Unix(start, 0), ObservedAt: time.Unix(start, 0)}
			m.state.UpdateBackupTasksForInstance("other", []models.BackupTask{unrelated})
			project := func(label string) map[string]models.BackupTask {
				snapshot := m.state.GetSnapshot().PVEBackups
				wire, err := json.Marshal(map[string]any{"data": snapshot})
				require.NoError(t, err)
				require.NotContains(t, string(wire), "synthetic-private-warning")
				t.Logf("PVE_OFFLINE_PROJECTION_%s_%s=%s", name, label, wire)
				byID := map[string]models.BackupTask{}
				for _, task := range snapshot.BackupTasks {
					byID[task.ID] = task
				}
				return byID
			}
			m.pollBackupTasks(context.Background(), "site", client)
			original := project("initial")
			require.Len(t, original, 5)
			require.Equal(t, "pve-a", original["site-a-old"].Node, "task scope comes from its reader")
			require.Equal(t, "warning", original["site-b-job-vm200"].Status)
			require.Empty(t, original["site-b-running"].Status, "PVE omits status while the task is running; preserve the provider observation")
			intent, active := m.resolveBackupIntentContext("", "site", "pve-b", 300, original["site-b-running"].ObservedAt.Add(time.Second))
			require.True(t, active && intent.Active, "fresh running evidence remains usable")
			require.Len(t, m.vzdumpJobTaskCache, 1)
			for _, step := range []struct {
				phase int32
				label string
			}{{1, "offline"}, {2, "online-empty"}, {3, "all-offline"}, {5, "no-nodes"}} {
				phase.Store(step.phase)
				m.pollBackupTasks(context.Background(), "site", client)
				actual := project(step.label)
				for _, id := range []string{"site-b-job", "site-b-job-vm200", "site-b-running", "other-running"} {
					require.Equal(t, original[id], actual[id], "%s: unread node must retain the exact old observation, not renew or erase it", step.label)
				}
				if step.phase == 1 {
					require.Equal(t, "OK", actual["site-a-next"].Status, "healthy peer must progress")
					require.True(t, actual["site-a-next"].ObservedAt.After(original["site-a-old"].ObservedAt))
					require.NotContains(t, actual, "site-a-old")
				} else {
					require.NotContains(t, actual, "site-a-next", "a complete empty healthy node is authoritative for that node only")
					require.NotContains(t, actual, "site-a-old")
				}
				require.Equal(t, int32(1), bReads.Load())
				require.Equal(t, int32(1), logReads.Load())
				require.Len(t, m.vzdumpJobTaskCache, 1, "offline parent cache must not be pruned")
				_, active := m.resolveBackupIntentContext("", "site", "pve-b", 300, original["site-b-running"].ObservedAt.Add(backupIntentEvidenceMaxAge+time.Second))
				require.False(t, active, "retained offline evidence must expire rather than suppress alerts indefinitely")
			}
			phase.Store(4)
			m.pollBackupTasks(context.Background(), "site", client)
			empty := project("recovered-empty")
			require.Equal(t, map[string]models.BackupTask{"other-running": unrelated}, empty, "only an observed empty node may withdraw its retained tasks")
			require.Empty(t, m.vzdumpJobTaskCache)
			phase.Store(0)
			m.pollBackupTasks(context.Background(), "site", client)
			resumed := project("resumed")
			require.Len(t, resumed, 5)
			require.True(t, resumed["site-b-job-vm200"].ObservedAt.After(original["site-b-job-vm200"].ObservedAt))
			require.Equal(t, int32(2), logReads.Load(), "ordinary recovery can refill a previously emptied scope")
		})
	}
}

func TestBackupTaskCachePrunesOnlyReadableNodes(t *testing.T) {
	old := time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)
	entry := func(node string) vzdumpJobTaskCacheEntry {
		return vzdumpJobTaskCacheEntry{node: node, final: true, historyComplete: true, tasks: []models.BackupTask{{Node: node, ObservedAt: old}}}
	}
	m := &Monitor{vzdumpJobTaskCache: map[string]vzdumpJobTaskCacheEntry{
		"site|a-old": entry("a"), "site|a-present": entry("a"),
		"site|b": entry("b"), "site|unknown": entry(""), "other|a-old": entry("a"),
	}}
	originalB := m.vzdumpJobTaskCache["site|b"]
	m.pruneVzdumpJobTaskCache("site", map[string]struct{}{"site|a-present": {}}, map[string]bool{"b": true, "": true})
	require.NotContains(t, m.vzdumpJobTaskCache, "site|a-old")
	require.Contains(t, m.vzdumpJobTaskCache, "site|a-present")
	require.Equal(t, originalB, m.vzdumpJobTaskCache["site|b"], "retention must not renew a cached receipt")
	require.Contains(t, m.vzdumpJobTaskCache, "site|unknown")
	require.Contains(t, m.vzdumpJobTaskCache, "other|a-old")
	m.pruneVzdumpJobTaskCache("site", nil, nil)
	require.Len(t, m.vzdumpJobTaskCache, 1, "full collection can prune scopes after ordinary recovery")
	require.Contains(t, m.vzdumpJobTaskCache, "other|a-old")
}
