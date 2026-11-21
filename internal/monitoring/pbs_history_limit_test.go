package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

type pbsHistoryLimitGroup struct {
	kind, id string
	times    []int64
}

type pbsHistoryLimitInventory map[string][]pbsHistoryLimitGroup

type pbsHistoryLimitFixture struct {
	*pbsPosturePollFixture
	inventory    atomic.Pointer[pbsHistoryLimitInventory]
	edgeReads    atomic.Int32
	allEdgeReads atomic.Int32
}

func newPBSHistoryLimitFixture(t *testing.T) *pbsHistoryLimitFixture {
	t.Helper()
	f := &pbsHistoryLimitFixture{pbsPosturePollFixture: newPBSPosturePollFixture(t, 2)}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inv := f.inventory.Load()
		store := path.Base(path.Dir(r.URL.Path))
		groups := (*inv)[store]
		data := []map[string]any{}
		switch path.Base(r.URL.Path) {
		case "groups":
			for _, group := range groups {
				var latest int64
				if len(group.times) > 0 {
					latest = group.times[0]
				}
				data = append(data, map[string]any{"backup-type": group.kind, "backup-id": group.id, "last-backup": latest, "backup-count": len(group.times)})
			}
		case "snapshots":
			id, kind := r.URL.Query().Get("backup-id"), r.URL.Query().Get("backup-type")
			if store == "edge" {
				f.allEdgeReads.Add(1)
				if id == "100" {
					f.edgeReads.Add(1)
				}
			}
			for _, group := range groups {
				if group.id != id || group.kind != kind {
					continue
				}
				for _, at := range group.times {
					data = append(data, map[string]any{"backup-type": kind, "backup-id": id, "backup-time": at, "size": 1024,
						"files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}})
				}
			}
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

func pbsHistoryLimitTimes(at int64, count int) []int64 {
	times := make([]int64, count)
	for i := range times {
		times[i] = at - 60*int64(i)
	}
	return times
}

func (f *pbsHistoryLimitFixture) collect(t *testing.T, inv pbsHistoryLimitInventory) {
	t.Helper()
	f.inventory.Store(&inv)
	f.m.pollPBSBackups(context.Background(), "pbs-main", f.client, []models.PBSDatastore{{Name: "archive"}, {Name: "edge"}}, nil)
	f.wait(t)
}

func (f *pbsHistoryLimitFixture) primary(filler int) []pbsHistoryLimitGroup {
	groups := []pbsHistoryLimitGroup{{kind: "vm", id: "101", times: []int64{f.backupAt}}}
	for i := 0; filler > 0; i++ {
		count := min(filler, pbsBackupSnapshotsPerGroupLimit)
		groups = append(groups, pbsHistoryLimitGroup{kind: "host", id: "fixture-host-" + strconv.Itoa(i), times: pbsHistoryLimitTimes(f.backupAt-3600, count)})
		filler -= count
	}
	return groups
}

func (f *pbsHistoryLimitFixture) guestPoints(t *testing.T) []recovery.RecoveryPoint {
	t.Helper()
	points, _, err := f.store.ListPoints(context.Background(), recovery.ListPointsOptions{
		Provider: recovery.ProviderProxmoxPBS, SubjectResourceID: f.ids[0], Limit: 200,
	})
	if err != nil {
		t.Fatal(err)
	}
	return points
}

func TestPBSBackupHistoryInstanceLimitRetainsEvidence(t *testing.T) {
	for _, cached := range []bool{false, true} {
		name := "fetched"
		if cached {
			name = "cached"
		}
		t.Run(name, func(t *testing.T) {
			f := newPBSHistoryLimitFixture(t)
			at, omittedAt := f.backupAt, f.backupAt-60
			seedAt := at - 10
			if cached {
				seedAt = at
			}
			edge := func(times ...int64) []pbsHistoryLimitGroup {
				return []pbsHistoryLimitGroup{{kind: "vm", id: "100", times: times}}
			}
			f.collect(t, pbsHistoryLimitInventory{"archive": f.primary(0), "edge": edge(seedAt, omittedAt)})
			var old recovery.RecoveryPoint
			for _, point := range f.guestPoints(t) {
				if point.CompletedAt != nil && point.CompletedAt.Unix() == omittedAt {
					old = point
				}
			}
			if old.ID == "" || old.Evidence == nil {
				t.Fatal("no accepted historical witness")
			}

			primary := f.primary(pbsBackupLiveStateLimit - 2)
			f.collect(t, pbsHistoryLimitInventory{"archive": primary, "edge": edge(at, omittedAt)})
			wantReads := int32(2)
			if cached {
				wantReads = 1
			}
			if got := f.edgeReads.Load(); got != wantReads {
				t.Fatalf("wrong clipped group path: reads=%d want=%d", got, wantReads)
			}
			assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryPartial)
			raw := f.m.state.GetPBSBackupsForInstance("pbs-main")
			if len(raw) != pbsBackupLiveStateLimit {
				t.Fatalf("raw snapshots=%d want=%d", len(raw), pbsBackupLiveStateLimit)
			}
			found := false
			for _, row := range raw {
				if row.VMID == "100" && row.BackupType == "vm" {
					if row.CarriedOver || row.BackupTime.Unix() != at {
						t.Fatalf("clipping hid the newest established snapshot or freshened a carried one: %+v", row)
					}
					found = true
				}
			}
			if !found {
				t.Fatal("readable group lost all its submitted rows")
			}
			found = false
			for _, point := range f.guestPoints(t) {
				found = found || reflect.DeepEqual(point, old)
			}
			if !found {
				t.Fatal("overflow removed or renewed a verified historical point outside live retention")
			}

			// The provider now lists exactly 5,000 rows. The edge group
			// shrank to the size of its clipped cache without changing
			// its newest snapshot; only another read can confirm absence.
			before := f.edgeReads.Load()
			exact := append(edge(at), pbsHistoryLimitGroup{kind: "host", id: "empty-group"})
			f.collect(t, pbsHistoryLimitInventory{"archive": primary, "edge": exact})
			assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryComplete)
			if f.edgeReads.Load() != before+1 {
				t.Fatal("clipped cache substituted for complete snapshot read")
			}
			points := f.guestPoints(t)
			if len(points) != 1 || points[0].CompletedAt == nil || points[0].CompletedAt.Unix() != at {
				t.Fatalf("exact fit failed to reconcile only confirmed absence: %d points", len(points))
			}

			// Exhaust the budget before this namespace's nonempty group.
			// Its prior point remains, but it cannot be freshly confirmed.
			before = f.edgeReads.Load()
			f.collect(t, pbsHistoryLimitInventory{"archive": f.primary(pbsBackupLiveStateLimit - 1), "edge": edge(at)})
			assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryPartial, recovery.ProtectionUnreadScope{
				Kind: recovery.ProtectionUnreadScopeNamespace, Storage: "edge",
			})
			if f.edgeReads.Load() != before || !reflect.DeepEqual(f.guestPoints(t), points) {
				t.Fatal("skipped group started extra reads, renewed evidence or reconciled absence")
			}
			t.Logf("%s: 5,001 listed / 5,000 retained stayed partial; exact fit refetched; skipped group kept historical evidence", name)
		})
	}
}

func TestPBSBackupHistoryLimitPreservesQueuedBudget(t *testing.T) {
	f := newPBSHistoryLimitFixture(t)
	warm := pbsHistoryLimitGroup{kind: "vm", id: "100", times: pbsHistoryLimitTimes(f.backupAt, 99)}
	f.collect(t, pbsHistoryLimitInventory{"archive": f.primary(0), "edge": {warm}})
	queued := pbsHistoryLimitGroup{kind: "host", id: "queued", times: []int64{f.backupAt + 60}}
	tail := pbsHistoryLimitGroup{kind: "host", id: "tail", times: []int64{f.backupAt - 60}}
	f.collect(t, pbsHistoryLimitInventory{
		"archive": f.primary(pbsBackupLiveStateLimit - 101),
		"edge":    {queued, warm, tail},
	})
	if f.allEdgeReads.Load() != 2 || f.edgeReads.Load() != 1 {
		t.Fatal("cache hit forgot a queued reservation and started the skipped read")
	}
	raw := f.m.state.GetPBSBackupsForInstance("pbs-main")
	foundQueued := false
	for _, row := range raw {
		foundQueued = foundQueued || row.VMID == "queued"
	}
	if len(raw) != pbsBackupLiveStateLimit || !foundQueued {
		t.Fatal("poll failed to retain the admitted queued group")
	}
	assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryPartial, recovery.ProtectionUnreadScope{
		Kind: recovery.ProtectionUnreadScopeNamespace, Storage: "edge",
	})
}

func TestPBSBackupHistoryGroupLimitCountChangeRequiresRead(t *testing.T) {
	f := newPBSHistoryLimitFixture(t)
	times := pbsHistoryLimitTimes(f.backupAt, pbsBackupSnapshotsPerGroupLimit+1)
	collect := func(times []int64) {
		f.collect(t, pbsHistoryLimitInventory{
			"archive": f.primary(0),
			"edge":    {{kind: "vm", id: "100", times: times}},
		})
	}
	collect(times)
	assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryPartial)
	collect(times)
	assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryPartial)
	if f.edgeReads.Load() != 1 {
		t.Fatal("unchanged bounded group lost TTL reuse")
	}
	// Prune the second-newest snapshot. Count now matches the cache's size
	// and latest time is unchanged, but the former cap hid the oldest row.
	pruned := append([]int64{times[0]}, times[2:]...)
	collect(pruned)
	assertPBSHistoryLimitPosture(t, f, recovery.ProtectionHistoryComplete)
	if f.edgeReads.Load() != 2 {
		t.Fatal("shorter group reused a capped cache and skipped the required read")
	}
	remaining := make(map[int64]bool)
	for _, point := range f.guestPoints(t) {
		if point.CompletedAt != nil {
			remaining[point.CompletedAt.Unix()] = true
		}
	}
	if len(remaining) != len(pruned) || remaining[times[1]] || !remaining[times[len(times)-1]] {
		t.Fatal("exact-fit reread lost the previously clipped archive or kept the pruned one")
	}
}

func assertPBSHistoryLimitPosture(t *testing.T, f *pbsHistoryLimitFixture, history recovery.ProtectionHistoryCompleteness, scopes ...recovery.ProtectionUnreadScope) {
	t.Helper()
	for _, posture := range f.postures(t) {
		if len(posture.ProviderStates) != 1 {
			t.Fatalf("wanted one provider state, got %+v", posture)
		}
		provider := posture.ProviderStates[0]
		if provider.HistoryCompleteness != history || !slices.Equal(provider.UnreadScopes, scopes) {
			t.Fatalf("history=%s scopes=%+v, want %s with scopes=%+v", provider.HistoryCompleteness, provider.UnreadScopes, history, scopes)
		}
		if history == recovery.ProtectionHistoryPartial &&
			(provider.Permissions != operationaltrust.EvidencePermissionsUnknown || posture.Coverage != recovery.ProtectionCoveragePartial) {
			t.Fatalf("local limit invented complete or denied evidence: %+v", posture)
		}
		if history == recovery.ProtectionHistoryComplete &&
			(provider.Permissions != operationaltrust.EvidencePermissionsSufficient || posture.Coverage != recovery.ProtectionCoverageComplete || posture.State != recovery.ProtectionStateProtected) {
			t.Fatalf("exact fit lost authorized, complete evidence: %+v", posture)
		}
		if posture.SubjectResourceID == f.ids[1] && posture.State != recovery.ProtectionStateProtected {
			t.Fatalf("independently read sibling lost its current protection: %+v", posture)
		}
	}
}
