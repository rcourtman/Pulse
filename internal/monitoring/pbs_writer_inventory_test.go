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

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/stretchr/testify/require"
)

// An unread writer inventory cannot turn a manifestless snapshot into a
// terminal failure. Exercise actual HTTP -> poll -> raw/durable/public state
// through ordinary recovery, including withdrawal of a prior no-writer fact.
// These fixtures perform no guest or backup operation.
func TestPBSWriterInventoryDoesNotInventTerminalFailure(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, family := range []string{"backup", "syncjob"} {
			for _, invalid := range []string{`{}`, `{"data":null}`, `{"data":[{}]}`, `{"data":[null]}`, `{"data":[],"errors":{"private":"fixture-only"}}`} {
				t.Run(kind+"/"+family+"/"+invalid, func(t *testing.T) {
					f := newPBSPosturePollFixture(t, 2, kind)
					var phase atomic.Int32
					var writes atomic.Int32
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.Method != http.MethodGet {
							writes.Add(1)
						}
						switch {
						case strings.HasSuffix(r.URL.Path, "/groups"):
							_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{
								{"backup-type": kind, "backup-id": "100", "last-backup": f.backupAt, "backup-count": 1},
								{"backup-type": kind, "backup-id": "101", "last-backup": f.backupAt, "backup-count": 1},
							}})
						case strings.HasSuffix(r.URL.Path, "/snapshots"):
							id := r.URL.Query().Get("backup-id")
							row := map[string]any{"backup-type": kind, "backup-id": id, "backup-time": f.backupAt,
								"size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}}
							if id == "100" && phase.Load() != 5 {
								row["size"], row["files"] = 0, []string{"guest.conf.blob"}
							}
							_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{row}})
						case strings.HasSuffix(r.URL.Path, "/tasks"):
							if (phase.Load() == 1 || phase.Load() == 3) && r.URL.Query().Get("typefilter") == family {
								_, _ = w.Write([]byte(invalid))
							} else if (phase.Load() == 0 || phase.Load() == 4) && r.URL.Query().Get("typefilter") == "backup" {
								_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"worker_type": "backup", "worker_id": "archive:" + kind + "/100"}}})
							} else {
								_, _ = w.Write([]byte(`{"data":[]}`))
							}
						default:
							http.NotFound(w, r)
						}
					}))
					t.Cleanup(server.Close)
					client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
					require.NoError(t, err)
					frames := map[string]any{}
					var identity string
					for i, label := range []string{"active", "unread", "empty", "unread-again", "active-again", "complete"} {
						phase.Store(int32(i))
						f.m.pollPBSBackups(context.Background(), "pbs-main", client, []models.PBSDatastore{{Name: "archive"}}, nil)
						f.wait(t)
						raw := f.m.state.GetSnapshot().PBSBackups
						require.Len(t, raw, 2)
						for _, b := range raw {
							if b.VMID == "101" {
								require.False(t, b.InProgress, "accepted sibling must not acquire writer uncertainty")
								continue
							}
							if identity == "" {
								identity = b.ID
							}
							wantedObservation := i == 0 || i == 2 || i == 4
							require.Equal(t, identity, b.ID, "ordinary polls must update the same snapshot")
							require.Equal(t, i != 5, b.InProgress)
							require.Equal(t, wantedObservation, b.WriteActivityObserved, label)
							require.Equal(t, i == 0 || i == 4, b.WriteActive, label)
						}
						points := f.points(t, "pbs-main")
						require.Len(t, points, 2)
						for _, p := range points {
							if fmt.Sprint(p.Details["vmid"]) == "101" {
								require.Equal(t, recovery.OutcomeSuccess, p.Outcome)
								continue
							}
							want := recovery.OutcomeRunning
							if i == 2 {
								want = recovery.OutcomeFailed
							} else if i == 5 {
								want = recovery.OutcomeSuccess
							}
							require.Equal(t, want, p.Outcome, label)
							if i != 5 {
								require.Nil(t, p.CompletedAt, "neither writer absence nor uncertainty supplies a completion time")
								require.Nil(t, p.Verified)
							}
						}
						postures := f.postures(t)
						for _, p := range postures {
							if p.SubjectResourceID == f.ids[0] && i != 5 {
								require.NotEqual(t, recovery.ProtectionStateProtected, p.State, "incomplete artifact cannot claim protection")
							}
						}
						frames[label] = map[string]any{
							"pbs":      map[string]any{"data": map[string]any{"backups": raw}},
							"pve":      map[string]any{"data": f.m.state.GetSnapshot().PVEBackups},
							"postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()},
							"points":   points,
						}
					}
					require.Zero(t, writes.Load())
					if dir := os.Getenv("PULSE_PBS_WRITER_FRAMES"); dir != "" && invalid == `{}` && family == "syncjob" {
						data, err := json.Marshal(frames)
						require.NoError(t, err)
						require.NoError(t, os.WriteFile(dir+"/frames-"+kind+".json", data, 0600))
					}
				})
			}
		}
	}
}
