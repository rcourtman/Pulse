package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

// These are ordinary inventory reads against loopback fixtures, not guest
// operations, installed backup jobs or a reproduction of a frozen workload.
type pbsBudgetFixture struct {
	*pbsPosturePollFixture
	phase, reads, lateReads atomic.Int32
	at                      atomic.Int64
	boundary                string
	empty                   bool
	datastores              []models.PBSDatastore
	blocked                 chan struct{}
}

func newPBSBudgetFixture(t *testing.T, kind, boundary string) *pbsBudgetFixture {
	t.Helper()
	count := 3
	if boundary == "snapshots" {
		count = 10
	} else if boundary == "tasks" {
		count = 1
	}
	f := &pbsBudgetFixture{pbsPosturePollFixture: newPBSPosturePollFixture(t, count, kind), boundary: boundary, blocked: make(chan struct{}, 32)}
	f.at.Store(f.backupAt)
	f.datastores = []models.PBSDatastore{{Name: "archive"}, {Name: "slow"}, {Name: "never"}}
	if boundary == "namespace" {
		f.datastores = []models.PBSDatastore{{Name: "archive", Namespaces: []models.PBSNamespace{{Path: ""}, {Path: "slow"}, {Path: "never"}}}}
	} else if boundary == "snapshots" || boundary == "tasks" {
		f.datastores = []models.PBSDatastore{{Name: "archive"}}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.reads.Add(1)
		stage := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		id := r.URL.Query().Get("backup-id")
		scope := ""
		if boundary == "namespace" {
			scope = r.URL.Query().Get("ns")
		} else if strings.Contains(r.URL.Path, "/slow/") {
			scope = "slow"
		} else if strings.Contains(r.URL.Path, "/never/") {
			scope = "never"
		}
		if f.phase.Load() == 1 {
			if scope == "never" {
				f.lateReads.Add(1)
			}
			blocked := scope == "slow" || boundary == "snapshots" && stage == "snapshots" && id != "100" || boundary == "tasks" && stage == "tasks"
			if blocked {
				f.blocked <- struct{}{}
				<-r.Context().Done()
				return
			}
		}
		var data any = []any{}
		switch stage {
		case "groups":
			ids := []int{100}
			if boundary == "snapshots" {
				// More groups than workers: unread queued groups must retain
				// their cache without making a request after the deadline.
				ids = []int{100, 101, 102, 103, 104, 105, 106, 107, 108, 109}
			} else if scope == "slow" {
				ids = []int{101}
			} else if scope == "never" {
				ids = []int{102}
			}
			groups := []map[string]any{}
			for _, vmid := range ids {
				if f.empty && f.phase.Load() == 1 && vmid == 100 {
					continue
				}
				groups = append(groups, map[string]any{"backup-type": kind, "backup-id": strconv.Itoa(vmid), "last-backup": f.at.Load(), "backup-count": 1})
			}
			data = groups
		case "snapshots":
			row := map[string]any{"backup-type": kind, "backup-id": id, "backup-time": f.at.Load(), "size": 1024,
				"files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}, "comment": "guest-" + id}
			if boundary == "tasks" && f.phase.Load() == 1 {
				row["size"], row["files"] = 0, []string{"guest.conf.blob"}
			}
			data = []map[string]any{row}
		case "tasks":
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(server.Close)
	var err error
	f.client, err = pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "fixture@pbs!reader", TokenValue: "fixture-only"})
	if err != nil {
		t.Fatal(err)
	}
	f.m.pbsClients = map[string]*pbs.Client{"pbs-main": f.client}
	f.m.setRuntimeContext(context.Background(), nil)
	return f
}

func (f *pbsBudgetFixture) collect(t *testing.T, ctx context.Context) {
	t.Helper()
	f.m.pollPBSBackups(ctx, "pbs-main", f.client, f.datastores, nil)
	f.wait(t)
}

func (f *pbsBudgetFixture) frame(t *testing.T) map[string]any {
	t.Helper()
	return map[string]any{
		"pbs":      map[string]any{"data": map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups}},
		"pve":      map[string]any{"data": f.m.state.GetSnapshot().PVEBackups},
		"postures": map[string]any{"data": f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()},
		"points":   f.points(t, "pbs-main"),
	}
}

func exercisePBSBudgetAcceptedListings(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, boundary := range []string{"datastore", "namespace", "snapshots", "tasks"} {
			for _, cold := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/cold=%t", kind, boundary, cold), func(t *testing.T) {
					f := newPBSBudgetFixture(t, kind, boundary)
					f.collect(t, context.Background())
					oldRaw := f.m.state.GetSnapshot().PBSBackups
					oldPoints := f.points(t, "pbs-main")
					frames := map[string]any{"observed": f.frame(t)}
					f.m.mu.RLock()
					cache := make(map[pbsBackupGroupKey]time.Time)
					for key, at := range f.m.pbsBackupCacheTime["pbs-main"] {
						cache[key] = at
					}
					f.m.mu.RUnlock()
					if cold {
						f.m.state.UpdatePBSBackups("pbs-main", nil)
					}
					f.at.Add(15 * 60)
					f.phase.Store(1)
					startReads := f.reads.Load()
					owner := f.m.beginPBSBackupPoll("pbs-main", f.client, time.Time{}, time.Now())
					if owner == nil {
						t.Fatal("collection owner was not admitted")
					}
					defer f.m.finishPBSBackupPoll("pbs-main", owner)
					parent := context.WithValue(owner.ctx, pbsBackupPollContextKey{}, owner)
					ctx, cancel := context.WithTimeout(parent, time.Second)
					defer cancel()
					f.collect(t, ctx)
					if ctx.Err() != context.DeadlineExceeded || f.lateReads.Load() != 0 {
						t.Fatalf("budget did not expire cleanly: err=%v queued late reads=%d", ctx.Err(), f.lateReads.Load())
					}
					if boundary == "snapshots" && f.reads.Load()-startReads > 2+pbsBackupSnapshotFetchWorkers {
						t.Error("queued snapshot work started after expiry")
					}
					raw := f.m.state.GetSnapshot().PBSBackups
					advanced := false
					for _, row := range raw {
						if row.VMID == "100" {
							advanced = row.BackupTime.Unix() == f.at.Load()
							if boundary == "tasks" && (!row.InProgress || row.WriteActivityObserved || row.WriteActive) {
								t.Error("unread live tasks invented terminal or successful write evidence")
							}
						} else {
							found := false
							for _, old := range oldRaw {
								found = found || reflect.DeepEqual(row, old)
							}
							if cold || !found {
								t.Error("unread raw history was invented or freshened")
							}
						}
					}
					if !advanced || !cold && len(raw) != len(oldRaw) || cold && len(raw) != 1 {
						t.Errorf("accepted snapshot discarded or unread membership changed: advanced=%t rows=%d prior=%d cold=%t", advanced, len(raw), len(oldRaw), cold)
					}
					for key, old := range cache {
						at := f.m.pbsBackupCacheTimeFor("pbs-main", key)
						if key.backupID == "100" {
							if !at.After(old) || !at.Before(time.Now()) {
								t.Error("accepted read time was not published with its snapshot")
							}
						} else if !at.Equal(old) {
							t.Error("unread cached snapshot's read time changed")
						}
					}
					points := f.points(t, "pbs-main")
					for _, old := range oldPoints {
						found := false
						for _, now := range points {
							found = found || reflect.DeepEqual(old, now)
						}
						if !found {
							t.Error("partial enumeration changed/deleted previously accepted durable evidence")
						}
					}
					for _, posture := range f.postures(t) {
						if posture.State == recovery.ProtectionStateProtected || len(posture.ProviderStates) != 1 || posture.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial || posture.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsUnknown {
							t.Errorf("partial collection invented protection: %+v", posture)
						}
					}
					frames["partial"] = f.frame(t)
					f.phase.Store(2)
					f.collect(t, context.Background())
					for _, posture := range f.postures(t) {
						if posture.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryComplete || posture.State != recovery.ProtectionStateProtected {
							t.Errorf("ordinary complete poll failed to recover: %+v", posture)
						}
					}
					frames["recovered"] = f.frame(t)
					if !cold && (boundary == "datastore" || boundary == "tasks") {
						if path := os.Getenv("PULSE_PBS_BUDGET_FRAMES"); path != "" {
							data, err := json.Marshal(frames)
							if err != nil {
								t.Fatal(err)
							}
							if err := os.WriteFile(path+"-"+kind+"-"+boundary+".json", data, 0600); err != nil {
								t.Fatal(err)
							}
							t.Logf("PULSE_PBS_FRAME %s-%s %x %s", kind, boundary, sha256.Sum256(data), data)
						}
					}
					t.Logf("kind=%s boundary=%s cold=%t: accepted snapshot advanced; unread raw/cache/durable evidence retained; partial posture and ordinary recovery proved", kind, boundary, cold)
				})
			}
		}
	}
}

func TestPBSBudgetAcceptedEmptyDoesNotProveUnreadAbsence(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		t.Run(kind, func(t *testing.T) {
			f := newPBSBudgetFixture(t, kind, "datastore")
			f.collect(t, context.Background())
			points := f.points(t, "pbs-main")
			f.empty = true
			f.phase.Store(1)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			f.collect(t, ctx)
			if len(f.m.state.GetSnapshot().PBSBackups) != 2 || !reflect.DeepEqual(points, f.points(t, "pbs-main")) {
				t.Error("accepted empty scope retained raw rows or partial scope deleted durable evidence")
			}
			for _, row := range f.m.state.GetSnapshot().PBSBackups {
				if row.VMID == "100" {
					t.Error("accepted empty listing failed to remove its raw row")
				}
			}
		})
	}
}
