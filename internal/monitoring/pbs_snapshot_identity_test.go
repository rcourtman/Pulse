package monitoring

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func conflictingPBSSnapshotBody(kind, failure string, at int64) string {
	row := map[string]any{"backup-type": kind, "backup-id": "101", "backup-time": at, "size": 1024, "files": []string{"index.json.blob"}, "verification": map[string]any{"state": "ok"}, "comment": "untrusted-new-file"}
	rows := []map[string]any{row}
	switch failure {
	case "wrong-kind":
		if kind == "vm" {
			row["backup-type"] = "ct"
		} else {
			row["backup-type"] = "vm"
		}
	case "wrong-id":
		row["backup-id"] = "100"
	case "missing-kind":
		delete(row, "backup-type")
	case "missing-id":
		delete(row, "backup-id")
	case "identical-repeat":
		rows = append(rows, row)
	case "conflicting-repeat":
		rows = append(rows, map[string]any{"backup-type": kind, "backup-id": "101", "backup-time": at, "size": 2048, "verification": map[string]any{"state": "failed"}})
	}
	b, _ := json.Marshal(map[string]any{"data": rows})
	return string(b)
}

func TestPBSConflictingSnapshotIdentityRetainsUnreadScope(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, failure := range []string{"wrong-kind", "wrong-id", "missing-kind", "missing-id", "identical-repeat", "conflicting-repeat"} {
			for _, warm := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/warm-%t", kind, failure, warm), func(t *testing.T) {
					f := newPBSNamespaceContinuityFixture(t, kind)
					if warm {
						f.collect(t, true, nil)
					}
					before, points := f.live(), f.points(t, "pbs-main")
					key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant", backupType: kind, backupID: "101"}
					cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
					f.rootAt.Add(15 * 60)
					f.tenantAt.Add(15 * 60)
					f.inventoryResponse.Store(&pbsInventoryResponse{stage: "snapshots", body: conflictingPBSSnapshotBody(kind, failure, f.tenantAt.Load())})
					for range 2 {
						f.collect(t, true, nil)
						raw := f.m.state.GetSnapshot().PBSBackups
						want := 1
						if warm {
							want = 2
						}
						if len(raw) != want || f.live()["100"].BackupTime.Unix() != f.rootAt.Load() {
							t.Errorf("invalid read displaced independent scope or invented files: %+v", raw)
						}
						for _, row := range raw {
							if row.Namespace != "tenant" {
								continue
							}
							if !warm || !reflect.DeepEqual(row, before["101"]) {
								t.Errorf("malformed group published or rewrote raw history: %+v", row)
							}
						}
						if !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
							t.Error("invalid response renewed cache")
						}
						for _, prior := range points {
							if prior.Details["namespace"] != "tenant" {
								continue
							}
							found := false
							for _, now := range f.points(t, "pbs-main") {
								found = found || reflect.DeepEqual(prior, now)
							}
							if !found {
								t.Error("unread historical point was deleted or renewed")
							}
						}
						for _, p := range f.postures(t) {
							if p.State == recovery.ProtectionStateProtected {
								t.Errorf("invalid group advertised protected: %+v", p)
							}
							for _, provider := range p.ProviderStates {
								if provider.HistoryCompleteness != recovery.ProtectionHistoryPartial || provider.Permissions != operationaltrust.EvidencePermissionsUnknown {
									t.Errorf("malformed read treated as complete or access refusal: %+v", provider)
								}
							}
						}
					}
					if f.failedReads.Load() != 2 {
						t.Errorf("malformed response caused extra reads or poisoned TTL reuse: %d", f.failedReads.Load())
					}
					f.inventoryResponse.Store(nil)
					f.collect(t, true, nil)
					if len(f.live()) != 2 || f.live()["101"].BackupTime.Unix() != f.tenantAt.Load() || !f.m.pbsBackupCacheTimeFor("pbs-main", key).After(cacheAt) {
						t.Error("ordinary valid read did not recover without cache reset")
					}
					for _, p := range f.postures(t) {
						if p.State != recovery.ProtectionStateProtected {
							t.Errorf("valid recovery not protected: %+v", p)
						}
					}
					// A valid empty snapshot array is still authoritative for this
					// group; the independent root namespace must survive it.
					f.tenantAt.Add(int64(time.Hour / time.Second))
					f.inventoryResponse.Store(&pbsInventoryResponse{stage: "snapshots", body: `{"data":[]}`})
					f.collect(t, true, nil)
					if len(f.live()) != 1 || f.live()["100"].VMID != "100" {
						t.Error("confirmed empty snapshots did not remove only their own group")
					}
					points = f.points(t, "pbs-main")
					if len(points) != 1 || points[0].Details["namespace"] != "" {
						t.Error("complete empty snapshot read did not reconcile only its own group")
					}
				})
			}
		}
	}
}
