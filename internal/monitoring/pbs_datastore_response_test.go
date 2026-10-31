package monitoring

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestPBSDatastoreWholeResponseRetainsEvidence(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, cold := range []bool{false, true} {
			for _, scenario := range []struct {
				name, body string
				status     int
			}{
				{name: "error envelope", body: `{"data":[],"errors":{"private":"fixture-only"}}`},
				{name: "discarded datastore", body: `{"data":[{"store":"archive"}],"data":[]}`},
				{name: "overwritten data", body: `{"data":null,"data":[]}`},
				{name: "case data", body: `{"data":null,"Data":[]}`},
				{name: "overwritten errors", body: `{"data":[],"errors":{"private":"fixture-only"},"errors":null}`},
				{name: "invalid utf8", body: "{\"data\":[],\"metadata\":\"\xff\"}"},
				{name: "created", body: `{"data":[]}`, status: http.StatusCreated},
				{name: "partial", body: `{"data":[]}`, status: http.StatusPartialContent},
			} {
				t.Run(fmt.Sprintf("%s/cold=%t/%s", kind, cold, scenario.name), func(t *testing.T) {
					f := newPBSNamespacePollFixture(t, kind)
					f.poll(t, "pbs-main")
					before := f.points(t, "pbs-main")
					if len(before) != 2 {
						t.Fatal("healthy control did not ingest root and tenant")
					}
					if cold {
						snapshot := f.m.state.GetSnapshot()
						f.m.state = models.NewState()
						f.m.state.UpdateVMs(snapshot.VMs)
						f.m.state.UpdateContainers(snapshot.Containers)
					}
					raw := f.m.state.GetSnapshot().PBSBackups
					key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant", backupType: kind, backupID: "101"}
					cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
					reads := f.backupReads.Load()
					f.datastoreBody.Store(&scenario.body)
					f.datastoreStatus.Store(int32(scenario.status))
					for range 2 {
						f.poll(t, "pbs-main")
						if !reflect.DeepEqual(before, f.points(t, "pbs-main")) || !reflect.DeepEqual(raw, f.m.state.GetSnapshot().PBSBackups) {
							t.Error("unread datastore scope deleted or renewed accepted artifacts")
						}
						if f.backupReads.Load() != reads || !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
							t.Error("unread datastore scope guessed a child read or renewed cache")
						}
						for _, p := range f.postures(t) {
							if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 {
								t.Errorf("unread scope kept protection: %+v", p)
								continue
							}
							q := p.ProviderStates[0]
							if q.HistoryCompleteness != recovery.ProtectionHistoryUnavailable || q.Permissions != operationaltrust.EvidencePermissionsUnknown || q.JobState != recovery.OutcomeFailed {
								t.Errorf("unread scope lost uncertainty: %+v", q)
							}
						}
						inst := pbsInstanceByName(t, f.m.state.GetSnapshot(), "pbs-main")
						if inst.Status != "online" || inst.ConnectionHealth != "healthy" {
							t.Error("unread backup history was confused with connectivity")
						}
					}
					f.datastoreBody.Store(nil)
					f.datastoreStatus.Store(0)
					f.backupAt.Add(60)
					f.poll(t, "pbs-main")
					for _, p := range f.postures(t) {
						if p.State != recovery.ProtectionStateProtected {
							t.Errorf("ordinary recovery did not restore protection: %+v", p)
						}
					}
					empty := `{"data":[],"errors":null}`
					f.datastoreBody.Store(&empty)
					reads = f.backupReads.Load()
					f.poll(t, "pbs-main")
					if len(f.points(t, "pbs-main")) != 0 || len(f.m.state.GetSnapshot().PBSBackups) != 0 || f.backupReads.Load() != reads || !f.m.pbsBackupCacheTimeFor("pbs-main", key).IsZero() {
						t.Error("complete empty scope did not reconcile genuine absence")
					}
				})
			}
		}
	}
}

// Export the actual public poll -> worker -> live/store/posture results for
// consuming-surface evidence. This does not introduce a second production path.
func TestPBSDatastoreWholeResponseConsumerLifecycle(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		t.Run(kind, func(t *testing.T) {
			f := newPBSNamespacePollFixture(t, kind)
			frames := map[string]any{}
			var prior map[string]recovery.RecoveryPoint
			var raw []models.PBSBackup
			for _, phase := range []string{"observed", "unread-datastores", "recovered", "confirmed-empty"} {
				switch phase {
				case "unread-datastores":
					bad := `{"data":[{"store":"archive"}],"data":[],"errors":{"private":"fixture-only"}}`
					f.datastoreBody.Store(&bad)
				case "confirmed-empty":
					empty := `{"data":[]}`
					f.datastoreBody.Store(&empty)
				default:
					f.datastoreBody.Store(nil)
				}
				f.poll(t, "pbs-main")
				points, rows := f.points(t, "pbs-main"), f.m.state.GetSnapshot().PBSBackups
				if phase == "unread-datastores" && (!reflect.DeepEqual(prior, points) || !reflect.DeepEqual(raw, rows)) {
					t.Fatal("unread response removed or renewed backup evidence")
				}
				for _, p := range f.postures(t) {
					if (phase == "observed" || phase == "recovered") != (p.State == recovery.ProtectionStateProtected) {
						t.Fatalf("phase %s posture=%+v", phase, p)
					}
				}
				want := 2
				if phase == "confirmed-empty" {
					want = 0
				}
				if len(points) != want || len(rows) != want {
					t.Fatalf("phase %s lost or invented inventory", phase)
				}
				frames[phase] = map[string]any{"pbs": map[string]any{"data": map[string]any{"backups": rows}}, "postures": map[string]any{"data": f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points}
				prior, raw = points, rows
			}
			if dest := os.Getenv("PULSE_PBS_DATASTORE_FRAMES"); dest != "" {
				hashes := map[string]string{}
				for _, file := range []string{"monitor_pbs_pmg.go", "monitor_backups.go", "backup_guard.go", "../../pkg/pbs/client.go", "../../pkg/pbs/backup_inventory_response.go", "../recovery/posture.go"} {
					content, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					hashes[file] = fmt.Sprintf("%x", sha256.Sum256(content))
				}
				data, err := json.Marshal(map[string]any{"runtime_sha256": hashes, "resource_ids": f.ids, "frames": frames})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(dest+"/frames-"+kind+".json", data, 0600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
