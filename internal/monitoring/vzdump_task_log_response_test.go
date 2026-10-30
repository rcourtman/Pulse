package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Actual HTTP reader -> ordinary polling -> live/cache/durable guest results.
// No guest-agent endpoint, task operation or external provider is involved.
func TestVzdumpTaskLogMalformedReadRetainsAcceptedEvidence(t *testing.T) {
	for _, seeded := range []bool{false, true} {
		for _, malformed := range []string{"null", "trailing", "unfinished", "gap", "reordered", "null-text", "conflicting-error"} {
			t.Run(fmt.Sprintf("seeded=%t/%s", seeded, malformed), func(t *testing.T) {
				ctx := context.Background()
				job, prefix := vzdumpGuestResultFixture()
				var phase, reads atomic.Int32
				completed := append(append([]proxmox.TaskLogLine{}, prefix...),
					proxmox.TaskLogLine{LineNumber: 6, Text: "INFO: Finished Backup of VM 103 (00:02:00)"},
					proxmox.TaskLogLine{LineNumber: 7, Text: "TASK ERROR: job errors"})
				badLines := append([]proxmox.TaskLogLine{}, completed...)
				switch malformed {
				case "gap":
					badLines[5].LineNumber++
				case "reordered":
					badLines[4], badLines[5] = badLines[5], badLines[4]
				}
				bad, err := json.Marshal(map[string]any{"data": badLines})
				require.NoError(t, err)
				switch malformed {
				case "null":
					bad = []byte(`{"data":null}`)
				case "trailing":
					bad = append(bad, []byte(` {"data":[]}`)...)
				case "null-text":
					bad = []byte(`{"data":[{"n":1,"t":null},{"n":2,"t":"INFO: Finished Backup of VM 103 (00:02:00)"}]}`)
				case "conflicting-error":
					bad = append(bad[:len(bad)-1], []byte(`,"errors":{"message":"fixture-private-detail"}}`)...)
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					p := phase.Load()
					switch r.URL.Path {
					case "/api2/json/nodes":
						fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"},{"node":"pve2","status":"online"}]}`)
					case "/api2/json/access/permissions":
						json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{r.URL.Query().Get("path"): map[string]int{"Sys.Audit": 1}}})
					case "/api2/json/nodes/pve1/tasks":
						task := proxmox.Task{UPID: testJobUPID, Node: "pve1", Type: "vzdump", StartTime: job.StartTime.Unix()}
						if p >= 2 {
							task.EndTime, task.Status = job.EndTime.Unix(), "job errors"
						}
						json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{task}})
					case "/api2/json/nodes/pve2/tasks":
						status := "OK"
						if p > 0 {
							status = "ERROR: independent node failure"
						}
						json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{{UPID: "independent", Node: "pve2", Type: "vzdump", ID: "104", StartTime: job.StartTime.Unix(), EndTime: job.EndTime.Unix(), Status: status}}})
					default:
						require.Equal(t, "/api2/json/nodes/pve1/tasks/"+testJobUPID+"/log", r.URL.Path)
						require.Equal(t, http.MethodGet, r.Method)
						reads.Add(1)
						if p == 0 {
							json.NewEncoder(w).Encode(map[string]any{"data": prefix})
						} else if p == 3 {
							json.NewEncoder(w).Encode(map[string]any{"data": completed})
						} else {
							if malformed == "unfinished" {
								w.Header().Set("Content-Length", fmt.Sprint(len(bad)+100))
							}
							w.Write(bad)
						}
					}
				}))
				defer server.Close()
				client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
				require.NoError(t, err)
				manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
				m := &Monitor{state: models.NewState(), recoveryManager: manager}
				store, err := manager.StoreForOrg("default")
				require.NoError(t, err)
				defer store.Close()
				peer := models.BackupTask{ID: "peer-same-job-vm103", Instance: "other", Node: "pve1", VMID: 103, Type: "vzdump", Status: "error", Error: "independent connection failure", StartTime: job.StartTime, EndTime: job.EndTime, ObservedAt: job.StartTime}
				m.state.UpdateBackupTasksForInstance(peer.Instance, []models.BackupTask{peer})
				frames := map[string]any{}
				var accepted []models.BackupTask
				key := "site|" + testJobUPID
				observed := time.Now().UTC().Add(-2 * backupIntentEvidenceMaxAge)
				first := 1
				if seeded {
					first = 0
				}
				for p := first; p <= 4; p++ {
					phase.Store(int32(p))
					// Model separate polls beyond the persisted millisecond watermark.
					coverage, err := store.BackupHistoryCoverage(ctx, job.StartTime, time.Now().UTC())
					require.NoError(t, err)
					for _, source := range coverage.Sources {
						if wait := time.Until(source.LastObservedAt.Add(time.Millisecond)); wait > 0 {
							time.Sleep(wait)
						}
					}
					m.pollBackupTasks(ctx, "site", client)
					snapshot := m.state.GetSnapshot()
					var guests []models.BackupTask
					for _, task := range snapshot.PVEBackups.BackupTasks {
						if task.Instance == "other" {
							require.Equal(t, peer, task)
						} else if task.Node == "pve2" {
							require.Equal(t, 104, task.VMID)
							if p > 0 {
								require.Equal(t, "ERROR: independent node failure", task.Status)
							}
						} else if task.VMID > 0 {
							guests = append(guests, task)
						}
					}
					if p == 0 {
						assertVzdumpGuestResults(t, guests, "running")
						_, active := m.resolveBackupIntentContext("", "site", "pve1", 103, time.Now())
						require.True(t, active, "accepted fresh log establishes guest backup intent")
						entry := m.vzdumpJobTaskCache[key]
						for i := range entry.tasks {
							entry.tasks[i].ObservedAt = observed
						}
						m.vzdumpJobTaskCache[key] = entry
						accepted = append([]models.BackupTask{}, entry.tasks...)
					} else if p < 3 && !seeded {
						require.Empty(t, guests, "malformed cold reads cannot invent guest results")
					} else if p == 1 {
						// State publishes tasks in its own chronology, not parser order.
						// Compare every field and multiplicity without imposing a new order.
						require.ElementsMatch(t, accepted, guests, "unread log retains exact accepted evidence without renewal")
					} else if p == 2 {
						assertVzdumpGuestResults(t, guests, "unknown")
						for _, guest := range guests {
							require.Equal(t, observed, guest.ObservedAt)
							require.False(t, guest.EndTime.IsZero())
						}
					} else {
						assertVzdumpGuestResults(t, guests, "OK")
						require.True(t, m.vzdumpJobTaskCache[key].final, "ordinary complete read recovers and finalises")
					}
					if p > 0 && p < 3 {
						require.False(t, m.vzdumpJobTaskCache[key].final)
						_, active := m.resolveBackupIntentContext("", "site", "pve1", 103, time.Now())
						require.False(t, active, "malformed success cannot renew a guest's alert deferral")
					}
					points, _, err := store.ListPoints(ctx, recovery.ListPointsOptions{Kind: recovery.KindOther})
					require.NoError(t, err)
					byID := map[string]recovery.RecoveryPoint{}
					for _, point := range points {
						byID[point.ID] = point
					}
					parentID := "pve-task:site-" + testJobUPID
					require.Equal(t, p >= 3, byID[parentID].Details["guestTaskDetailsComplete"], "malformed reads cannot qualify all-guest report coverage")
					last, present := byID[parentID+"-vm103"]
					if !seeded && p < 3 {
						require.False(t, present)
					} else {
						require.True(t, present)
						want := recovery.OutcomeRunning
						if p == 2 {
							want = recovery.OutcomeUnknown
						} else if p >= 3 {
							want = recovery.OutcomeSuccess
						}
						require.Equal(t, want, last.Outcome)
					}
					frames[[]string{"accepted", "unread-running", "unread-ended", "recovered", "cached"}[p]] = map[string]any{"pve": map[string]any{"data": snapshot.PVEBackups}}
				}
				require.EqualValues(t, 4-first, reads.Load(), "complete cached log is not refetched; each malformed poll spends one request")
				if seeded && malformed == "trailing" && os.Getenv("PULSE_TASK_LOG_FRAMES") == "1" {
					b, err := json.Marshal(frames)
					require.NoError(t, err)
					t.Log("TASK_LOG_FRAMES=" + string(b))
				}
			})
		}
	}
}

func TestVzdumpTaskLogExplicitEmptyDoesNotResurrectGuests(t *testing.T) {
	job, lines := vzdumpGuestResultFixture()
	var phase atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"pve1","status":"online"}]}`)
		case "/api2/json/access/permissions":
			fmt.Fprint(w, `{"data":{"/nodes/pve1":{"Sys.Audit":1}}}`)
		case "/api2/json/nodes/pve1/tasks":
			json.NewEncoder(w).Encode(map[string]any{"data": []proxmox.Task{{UPID: testJobUPID, Node: "pve1", Type: "vzdump", StartTime: job.StartTime.Unix()}}})
		default:
			require.True(t, strings.HasSuffix(r.URL.Path, "/log"))
			switch phase.Load() {
			case 0, 3:
				json.NewEncoder(w).Encode(map[string]any{"data": lines})
			case 1:
				fmt.Fprint(w, `{"data":[]}`)
			case 2:
				fmt.Fprint(w, `{"data":null}`)
			}
		}
	}))
	defer server.Close()
	client, err := proxmox.NewClient(proxmox.ClientConfig{Host: server.URL, TokenName: "fixture@pam!readonly", TokenValue: "fixture"})
	require.NoError(t, err)
	m := &Monitor{state: models.NewState()}
	for p := range 4 {
		phase.Store(int32(p))
		m.pollBackupTasks(context.Background(), "site", client)
		count := len(m.state.GetBackupTasks())
		if p == 0 || p == 3 {
			require.Equal(t, 4, count)
		} else {
			require.Equal(t, 1, count, "valid empty clears guest prefix; later unread cannot resurrect older cache data")
		}
		require.False(t, m.vzdumpJobTaskCache["site|"+testJobUPID].final)
	}
}
