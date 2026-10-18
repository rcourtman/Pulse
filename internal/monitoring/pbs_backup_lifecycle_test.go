package monitoring

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/stretchr/testify/require"
)

func waitForPBSBackupReturn(t *testing.T, m *Monitor) {
	t.Helper()
	require.Eventually(t, func() bool {
		m.mu.RLock()
		defer m.mu.RUnlock()
		return len(m.pbsBackupPollers) == 0
	}, 5*time.Second, time.Millisecond, "backup reader must actually return")
}

// Real read-only HTTP and the ordinary PBS foreground -> background path.
// A new runtime need not cancel the previous parent's context, and removing
// failed-node state must not let its pending inventory resurrect that state.
func TestPBSBackupLifecycleHTTP(t *testing.T) {
	for _, stage := range []string{"groups", "snapshots", "tasks"} {
		for _, withdrawal := range []string{"runtime", "replace", "auth-retire"} {
			t.Run(stage+"/"+withdrawal, func(t *testing.T) {
				f := newPBSPosturePollFixture(t, 1)
				f.poll(t, "pbs-main", 0, []models.PBSDatastore{{Name: "main"}})
				original := f.m.PBSBackupsSnapshot()
				oldRead := time.Now().Add(-2 * pbsBackupCacheTTL)
				key := pbsBackupGroupKey{datastore: "main", backupType: "vm", backupID: "100"}
				f.m.setPBSBackupCacheTime("pbs-main", key, oldRead)
				entered, release := make(chan context.Context, 1), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				var resumed, inventoryStarted atomic.Bool
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet {
						t.Errorf("unexpected provider mutation %s %s", r.Method, r.URL.Path)
						http.Error(w, "read only", http.StatusMethodNotAllowed)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					readStage := ""
					body := `{"data":[]}`
					switch {
					case strings.HasSuffix(r.URL.Path, "/version"):
						body = `{"data":{"version":"3.4.2"}}`
					case r.URL.Path == "/api2/json/nodes/localhost/status":
						body = `{"data":{"cpu":0.1,"memory":{"used":512,"total":1024},"uptime":120}}`
					case r.URL.Path == "/api2/json/admin/datastore":
						body = `{"data":[{"store":"main","total":1000,"used":250,"avail":750}]}`
					case strings.HasSuffix(r.URL.Path, "/status"):
						body = `{"data":{"status":"available","total":1000,"used":250,"avail":750}}`
					case strings.HasSuffix(r.URL.Path, "/groups"):
						inventoryStarted.Store(true)
						readStage = "groups"
						body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-count":1,"last-backup":%d}]}`, f.backupAt)
					case strings.HasSuffix(r.URL.Path, "/snapshots"):
						readStage = "snapshots"
						size, files, label := 2048, `["index.json.blob"]`, "withdrawn"
						if stage == "tasks" && !resumed.Load() {
							size, files = 0, `[]`
						}
						if resumed.Load() {
							size, label = 4096, "current-owned"
						}
						body = fmt.Sprintf(`{"data":[{"backup-type":"vm","backup-id":"100","backup-time":%d,"size":%d,"files":%s,"verification":{"state":"ok"},"comment":%q}]}`, f.backupAt, size, files, label)
					case strings.HasSuffix(r.URL.Path, "/tasks") && inventoryStarted.Load():
						readStage = "tasks"
					}
					if readStage == stage && !resumed.Load() {
						w.WriteHeader(http.StatusOK)
						w.(http.Flusher).Flush()
						entered <- r.Context()
						<-release
					}
					_, _ = fmt.Fprint(w, body)
				}))
				defer server.Close()
				defer unblock()
				f.m.config = &config.Config{EnableBackupPolling: true, BackupPollingCycles: 1, PBSInstances: []config.PBSInstance{{
					Name: "pbs-main", Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only", MonitorDatastores: true, MonitorBackups: true,
				}}}
				f.m.lastPBSBackupPoll = make(map[string]time.Time)
				f.m.pollStatusMap = make(map[string]*pollStatus)
				f.m.pbsClients = make(map[string]*pbs.Client)
				f.m.initPBSClients(f.m.config)
				client, ok := f.m.getPBSClient("pbs-main")
				require.True(t, ok)
				f.m.setRuntimeContext(context.Background(), nil)
				f.m.pollPBSInstance(context.Background(), "pbs-main", client)
				var readCtx context.Context
				select {
				case readCtx = <-entered:
				case <-time.After(5 * time.Second):
					t.Fatal("ordinary backup scan did not reach controlled read")
				}
				switch withdrawal {
				case "runtime":
					f.m.setRuntimeContext(context.Background(), nil)
				case "replace":
					f.m.initPBSClients(f.m.config)
				case "auth-retire":
					f.m.removeFailedPBSNode("pbs-main")
					original = []models.PBSBackup{}
				}
				select {
				case <-readCtx.Done():
				case <-time.After(150 * time.Millisecond):
					t.Error("withdrawal did not cancel consumed PBS HTTP read")
				}
				unblock()
				waitForPBSBackupReturn(t, f.m)
				f.wait(t)
				require.Equal(t, original, f.m.PBSBackupsSnapshot(), "withdrawn inventory must not publish or resurrect raw backups")
				require.False(t, f.m.pbsBackupCacheTimeFor("pbs-main", key).After(oldRead), "withdrawn read must not renew history freshness")
				for _, point := range f.points(t, "pbs-main") {
					require.NotContains(t, fmt.Sprint(point), "withdrawn", "withdrawn inventory must not enter recovery projection")
				}
				held := f.m.PBSBackupsSnapshot()
				resumed.Store(true)
				f.m.initPBSClients(f.m.config)
				fresh, ok := f.m.getPBSClient("pbs-main")
				require.True(t, ok)
				f.m.pollPBSInstance(context.Background(), "pbs-main", fresh)
				waitForPBSBackupReturn(t, f.m)
				f.wait(t)
				current := f.m.PBSBackupsSnapshot()
				require.Len(t, current, 1)
				require.Equal(t, int64(4096), current[0].Size)
				require.Equal(t, "current-owned", current[0].Comment)
				f.assertProtected(t, "pbs-main")
				if stage == "snapshots" && withdrawal == "runtime" {
					payload, err := json.Marshal(map[string]any{"held": held, "current": current})
					require.NoError(t, err)
					t.Logf("PBS_LIFECYCLE_BROWSER_PRODUCER=%s", base64.StdEncoding.EncodeToString(payload))
				}
				t.Logf("PBS_LIFECYCLE_%s_%s cancelled=true late_publication=0 resumed=1", stage, withdrawal)
			})
		}
	}
}
