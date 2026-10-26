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
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

type pbsNamespaceContinuityFixture struct {
	*pbsPosturePollFixture
	rootStatus, tenantStatus atomic.Int32
	rootEmpty, tenantEmpty   atomic.Bool
	rootAt, tenantAt         atomic.Int64
	rootReads, tenantReads   atomic.Int32
}

func newPBSNamespaceContinuityFixture(t *testing.T, kind string) *pbsNamespaceContinuityFixture {
	t.Helper()
	f := &pbsNamespaceContinuityFixture{pbsPosturePollFixture: newPBSPosturePollFixture(t, 2, kind)}
	f.rootAt.Store(f.backupAt)
	f.tenantAt.Store(f.backupAt)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, at, status, empty := "100", f.rootAt.Load(), f.rootStatus.Load(), f.rootEmpty.Load()
		if r.URL.Query().Get("ns") == "tenant" {
			id, at, status, empty = "101", f.tenantAt.Load(), f.tenantStatus.Load(), f.tenantEmpty.Load()
			f.tenantReads.Add(1)
		} else {
			f.rootReads.Add(1)
		}
		var data any = []any{}
		switch r.URL.Path {
		case "/api2/json/admin/datastore/archive/groups":
			if status == http.StatusOK {
				_, _ = w.Write([]byte(`{"data":`))
				return
			}
			if status != 0 {
				http.Error(w, "fixture unavailable", int(status))
				return
			}
			if !empty {
				data = []map[string]any{{"backup-type": kind, "backup-id": id, "last-backup": at, "backup-count": 1}}
			}
		case "/api2/json/admin/datastore/archive/snapshots":
			data = []map[string]any{{"backup-type": kind, "backup-id": id, "backup-time": at, "size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}, "comment": "guest-" + id}}
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
	return f
}

func (f *pbsNamespaceContinuityFixture) collect(t *testing.T, discovered bool, discoveryErr error) {
	t.Helper()
	ds := models.PBSDatastore{Name: "archive", Namespaces: []models.PBSNamespace{{Path: ""}}}
	if discovered {
		ds.Namespaces = append(ds.Namespaces, models.PBSNamespace{Path: "tenant"})
	}
	errs := map[string]error{}
	if discoveryErr != nil {
		errs["archive"] = discoveryErr
	}
	f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, []models.PBSDatastore{ds}, errs)
	f.wait(t)
}

func (f *pbsNamespaceContinuityFixture) live() map[string]models.PBSBackup {
	out := map[string]models.PBSBackup{}
	for _, b := range f.m.state.GetSnapshot().PBSBackups {
		if b.Instance == "pbs-main" {
			out[b.VMID] = b
		}
	}
	return out
}

func TestPBSNamespaceTransientGroupReadKeepsLiveInventory(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, ns := range []string{"", "tenant"} {
			for _, status := range []int{http.StatusServiceUnavailable, http.StatusOK} {
				t.Run(fmt.Sprintf("%s/ns=%s/status=%d", kind, ns, status), func(t *testing.T) {
					f := newPBSNamespaceContinuityFixture(t, kind)
					f.collect(t, true, nil)
					before, points := f.live(), f.points(t, "pbs-main")
					if len(before) != 2 || len(points) != 2 {
						t.Fatal("healthy control did not list both namespaces")
					}
					failed, accepted := "100", "101"
					if ns == "tenant" {
						failed, accepted = "101", "100"
						f.tenantStatus.Store(int32(status))
						f.rootAt.Add(15 * 60)
					} else {
						f.rootStatus.Store(int32(status))
						f.tenantAt.Add(15 * 60)
					}
					key := pbsBackupGroupKey{datastore: "archive", namespace: ns, backupType: kind, backupID: failed}
					cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
					for range 2 {
						f.collect(t, true, nil)
						got := f.live()
						if len(got) != 2 || !reflect.DeepEqual(got[failed], before[failed]) {
							t.Errorf("transient namespace failure evicted or changed accepted live history: got=%+v", got)
						}
						if got[accepted].BackupTime.Unix() != before[accepted].BackupTime.Add(15*time.Minute).Unix() {
							t.Error("accepted sibling did not advance")
						}
						if !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
							t.Error("unread namespace cache time changed")
						}
						for _, prior := range points {
							if prior.Details["namespace"] == ns {
								found := false
								for _, now := range f.points(t, "pbs-main") {
									if now.ID == prior.ID {
										found = reflect.DeepEqual(prior, now)
									}
								}
								if !found {
									t.Error("carried namespace renewed or lost recovery evidence")
								}
							}
						}
						for _, p := range f.postures(t) {
							if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial || p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsUnknown {
								t.Errorf("incomplete history claimed current protection: %+v", p)
							}
						}
					}
					f.rootStatus.Store(0)
					f.tenantStatus.Store(0)
					f.collect(t, true, nil)
					for _, p := range f.postures(t) {
						if p.State != recovery.ProtectionStateProtected {
							t.Errorf("ordinary read failed to recover: %+v", p)
						}
					}
					f.rootEmpty.Store(true)
					f.tenantEmpty.Store(true)
					f.collect(t, true, nil)
					if len(f.live()) != 0 || len(f.points(t, "pbs-main")) != 0 {
						t.Fatal("complete empty read failed to clear confirmed absence")
					}
				})
			}
		}
	}
}

func TestPBSNamespaceDiscoveryContinuityAndAccessWithdrawal(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, denied := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/denied=%t", kind, denied), func(t *testing.T) {
				f := newPBSNamespaceContinuityFixture(t, kind)
				f.collect(t, true, nil)
				prior := f.live()["101"]
				key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant", backupType: kind, backupID: "101"}
				cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
				childReads := f.tenantReads.Load()
				f.rootAt.Add(15 * 60)
				discoveryErr := fmt.Errorf("fixture namespace discovery unavailable")
				if denied {
					discoveryErr = fmt.Errorf("api error 403: fixture denied")
				}
				for range 2 {
					f.collect(t, false, discoveryErr)
					got := f.live()
					if denied {
						if len(got) != 1 {
							t.Errorf("discovery denial restored unread raw inventory: %+v", got)
						}
					} else if len(got) != 2 || !reflect.DeepEqual(got["101"], prior) {
						t.Errorf("transient discovery erased or changed unread namespace: %+v", got)
					}
					if !denied && !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
						t.Error("failed discovery renewed snapshot reuse")
					}
					if f.tenantReads.Load() != childReads {
						t.Error("discovery failure guessed a hidden namespace read")
					}
					for _, p := range f.postures(t) {
						if p.State == recovery.ProtectionStateProtected {
							t.Errorf("failed discovery claimed protection: %+v", p)
						}
					}
				}
				f.collect(t, true, nil)
				if len(f.live()) != 2 {
					t.Fatal("ordinary discovery did not recover inventory")
				}
				// A complete discovery that removes the child is authoritative absence.
				f.collect(t, false, nil)
				if len(f.live()) != 1 || len(f.points(t, "pbs-main")) != 1 {
					t.Fatal("complete namespace removal retained absent history")
				}
				if !f.m.pbsBackupCacheTimeFor("pbs-main", key).IsZero() {
					t.Error("removed namespace kept snapshot cache time")
				}
			})
		}
	}
}

func TestPBSNamespaceMixedFailuresDoNotResurrectDeniedScope(t *testing.T) {
	for _, rootDenied := range []bool{false, true} {
		for _, discoveryFailed := range []bool{false, true} {
			t.Run(fmt.Sprintf("root-denied=%t/discovery-failed=%t", rootDenied, discoveryFailed), func(t *testing.T) {
				f := newPBSNamespaceContinuityFixture(t, "vm")
				f.collect(t, true, nil)
				before := f.live()
				other := before["100"]
				other.ID = "other-instance-backup"
				other.Instance = "pbs-other"
				f.m.state.UpdatePBSBackups("pbs-other", []models.PBSBackup{other})
				denied, retained := "101", "100"
				f.rootStatus.Store(http.StatusServiceUnavailable)
				f.tenantStatus.Store(http.StatusForbidden)
				if rootDenied {
					denied, retained = "100", "101"
					f.rootStatus.Store(http.StatusUnauthorized)
					f.tenantStatus.Store(http.StatusServiceUnavailable)
				}
				var discoveryErr error
				if discoveryFailed {
					discoveryErr = fmt.Errorf("fixture transient discovery failure")
				}
				for range 2 {
					f.collect(t, true, discoveryErr)
					got := f.live()
					if len(got) != 1 || !reflect.DeepEqual(got[retained], before[retained]) {
						t.Errorf("mixed failure lost transient scope or retained denied scope %s: %+v", denied, got)
					}
					if len(f.points(t, "pbs-main")) != 2 {
						t.Error("mixed access failure deleted historical recovery facts")
					}
					for _, row := range f.m.state.GetSnapshot().PBSBackups {
						if row.Instance == "pbs-other" && !reflect.DeepEqual(row, other) {
							t.Error("failure crossed instance scope")
						}
					}
					if len(f.m.state.GetSnapshot().PBSBackups) != 2 {
						t.Error("other instance inventory disappeared or denied row was resurrected")
					}
					for _, p := range f.postures(t) {
						if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 ||
							p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsUnknown ||
							p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryUnavailable {
							t.Errorf("mixed rejected/unread scopes invented protection or universal denial: %+v", p)
						}
					}
				}
				f.rootStatus.Store(0)
				f.tenantStatus.Store(0)
				f.collect(t, true, nil)
				if len(f.live()) != 2 {
					t.Fatal("ordinary mixed-failure recovery did not restore both namespaces")
				}
			})
		}
	}
}

func TestPBSNamespaceSuccessfulEmptySiblingStaysEmpty(t *testing.T) {
	f := newPBSNamespaceContinuityFixture(t, "vm")
	f.collect(t, true, nil)
	before := f.live()["101"]
	f.rootEmpty.Store(true)
	f.tenantStatus.Store(http.StatusServiceUnavailable)
	for range 2 {
		// A failed discovery cannot override the root's actual successful empty read.
		f.collect(t, true, fmt.Errorf("fixture discovery unavailable"))
		got := f.live()
		if len(got) != 1 || !reflect.DeepEqual(got["101"], before) {
			t.Errorf("carry-forward resurrected a confirmed-absent sibling or dropped unread history: %+v", got)
		}
	}
	f.tenantStatus.Store(0)
	f.tenantEmpty.Store(true)
	f.collect(t, true, nil)
	if len(f.live()) != 0 || len(f.points(t, "pbs-main")) != 0 {
		t.Fatal("complete empty recovery retained absent files")
	}
}

// The registered Go suite checks each phase. Optional frame export is evidence
// for the production Coverage/API-hook browser proof, not a separate test page.
func TestPBSNamespaceContinuityLifecycle(t *testing.T) {
	f := newPBSNamespaceContinuityFixture(t, "vm")
	frames := map[string]any{}
	original := map[string]models.PBSBackup{}
	for _, phase := range []string{"observed", "partial", "recovered", "denied", "restored", "confirmed-empty"} {
		switch phase {
		case "partial":
			f.tenantStatus.Store(http.StatusServiceUnavailable)
			f.rootAt.Add(15 * 60)
		case "recovered", "restored":
			f.tenantStatus.Store(0)
		case "denied":
			f.tenantStatus.Store(http.StatusForbidden)
		case "confirmed-empty":
			f.rootEmpty.Store(true)
			f.tenantEmpty.Store(true)
		}
		f.collect(t, true, nil)
		rows := f.live()
		want := 2
		if phase == "denied" {
			want = 1
		}
		if phase == "confirmed-empty" {
			want = 0
		}
		if len(rows) != want {
			t.Fatalf("phase %s live rows=%d want=%d", phase, len(rows), want)
		}
		if phase == "observed" {
			original = rows
		}
		if phase == "partial" && !reflect.DeepEqual(rows["101"], original["101"]) {
			t.Fatal("unread row changed")
		}
		postures := f.postures(t)
		for _, p := range postures {
			healthy := phase == "observed" || phase == "recovered" || phase == "restored"
			if healthy != (p.State == recovery.ProtectionStateProtected) {
				t.Fatalf("phase %s posture=%+v", phase, p)
			}
		}
		frames[phase] = map[string]any{"pbs": map[string]any{"data": map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups}}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": f.points(t, "pbs-main")}
	}
	if os.Getenv("PULSE_PBS_NAMESPACE_FRAMES") != "1" {
		return
	}
	source, err := os.ReadFile("monitor_backups.go")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "observed_at": time.Now().UTC(), "resource_ids": f.ids, "frames": frames})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("PBS_NAMESPACE_FRAMES=" + string(data))
}

func TestPBSNamespaceCarryForwardCannotDisplaceAcceptedRowsAtLimit(t *testing.T) {
	f := newPBSNamespaceContinuityFixture(t, "vm")
	f.collect(t, true, nil)
	root, child := f.live()["100"], f.live()["101"]
	previous := []models.PBSBackup{root}
	for i := range pbsBackupLiveStateLimit {
		row := child
		row.ID = fmt.Sprintf("carried-%d", i)
		row.VMID = fmt.Sprintf("%d", 200+i)
		row.BackupTime = child.BackupTime.Add(time.Duration(i) * time.Second)
		previous = append(previous, row)
	}
	f.m.state.UpdatePBSBackups("pbs-main", previous)
	f.tenantStatus.Store(http.StatusServiceUnavailable)
	f.collect(t, true, nil)
	rows := f.m.state.GetSnapshot().PBSBackups
	if len(rows) != pbsBackupLiveStateLimit {
		t.Fatalf("live bound=%d want=%d", len(rows), pbsBackupLiveStateLimit)
	}
	foundAccepted, foundNewest := false, false
	for _, row := range rows {
		foundAccepted = foundAccepted || row.ID == root.ID
		foundNewest = foundNewest || row.ID == fmt.Sprintf("carried-%d", pbsBackupLiveStateLimit-1)
		if row.ID == "carried-0" {
			t.Error("oldest carry-forward displaced a newer retained file")
		}
	}
	if !foundAccepted || !foundNewest {
		t.Error("carry-forward displaced accepted or newest retained inventory")
	}
}
