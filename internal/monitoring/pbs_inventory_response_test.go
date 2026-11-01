package monitoring

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

func TestPBSWholeInventoryResponseRetainsEvidence(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, stage := range []string{"namespace", "groups", "snapshots"} {
			for name, body := range map[string]string{
				"trailing value":   `{"data":[]} {"data":[]}`,
				"error envelope":   `{"data":[],"errors":{"private":"fixture-only"}}`,
				"overwritten data": `{"data":null,"data":[]}`,
				"invalid utf8":     "{\"data\":[],\"metadata\":\"\xff\"}",
			} {
				for _, warm := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/warm=%t", kind, stage, name, warm), func(t *testing.T) {
						f := newPBSNamespaceContinuityFixture(t, kind)
						if warm {
							f.collect(t, true, nil)
						}
						before, points := f.live(), f.points(t, "pbs-main")
						key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant", backupType: kind, backupID: "101"}
						cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
						f.rootAt.Add(15 * 60)
						f.tenantAt.Add(15 * 60)
						for range 2 {
							f.collectInventoryResponse(t, stage, body)
							rows := f.live()
							want := 1
							if warm {
								want = 2
								if !reflect.DeepEqual(rows["101"], before["101"]) {
									t.Error("unread response removed or changed accepted live file")
								}
							}
							if len(rows) != want || rows["100"].BackupTime.Unix() != f.rootAt.Load() {
								t.Error("unread response invented/lost files or held back readable sibling")
							}
							if !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
								t.Error("unread response renewed snapshot cache time")
							}
							for _, prior := range points {
								if prior.Details["namespace"] != "tenant" {
									continue
								}
								found := false
								for _, now := range f.points(t, "pbs-main") {
									found = found || (prior.ID == now.ID && reflect.DeepEqual(prior, now))
								}
								if !found {
									t.Error("unread response removed or renewed durable evidence")
								}
							}
							for _, p := range f.postures(t) {
								if !warm && p.SubjectResourceID == f.ids[1] {
									if p.State != recovery.ProtectionStateUnknown || len(p.ProviderStates) != 0 {
										t.Error("cold unread scope acquired evidence")
									}
									continue
								}
								if len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial || p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsUnknown || p.Coverage != recovery.ProtectionCoveragePartial {
									t.Errorf("unread response claimed complete history or denied access: %+v", p)
								}
								// Only the root snapshot was re-read this poll.
								if wantProtected := p.SubjectResourceID == f.ids[0]; (p.State == recovery.ProtectionStateProtected) != wantProtected {
									t.Errorf("unread response protection does not follow what was read (protected=%t): %+v", wantProtected, p)
								}
							}
						}
						if f.failedReads.Load() != 2 {
							t.Error("invalid response caused extra reads")
						}
						f.inventoryResponse.Store(nil)
						f.collect(t, true, nil)
						if f.live()["101"].BackupTime.Unix() != f.tenantAt.Load() {
							t.Error("complete ordinary read failed to recover")
						}
						for _, p := range f.postures(t) {
							if p.State != recovery.ProtectionStateProtected {
								t.Error("ordinary recovery failed to establish complete protection")
							}
						}
						f.rootEmpty.Store(true)
						f.tenantEmpty.Store(true)
						f.collect(t, true, nil)
						if len(f.live()) != 0 || len(f.points(t, "pbs-main")) != 0 {
							t.Error("complete empty read failed to reconcile confirmed absence")
						}
					})
				}
			}
		}
	}
}

// Exercise ordinary polling, raw publication, durable ingestion and posture.
// Optional export is evidence for the real Backups consumer, not a second path.
func TestPBSWholeInventoryConsumerLifecycle(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		t.Run(kind, func(t *testing.T) {
			f := newPBSNamespaceContinuityFixture(t, kind)
			frames := map[string]any{}
			for _, phase := range []string{"observed", "unread-namespaces", "unread-groups", "unread-snapshots", "recovered", "confirmed-empty"} {
				switch phase {
				case "unread-namespaces":
					f.rootAt.Add(15 * 60)
					f.collectInventoryResponse(t, "namespace", `{"data":[]} {"data":[]}`)
				case "unread-groups":
					f.collectInventoryResponse(t, "groups", `{"data":[],"errors":{"private":"fixture-only"}}`)
				case "unread-snapshots":
					f.tenantAt.Add(15 * 60)
					f.collectInventoryResponse(t, "snapshots", `{"data":null,"data":[]}`)
				default:
					f.inventoryResponse.Store(nil)
					if phase == "confirmed-empty" {
						f.rootEmpty.Store(true)
						f.tenantEmpty.Store(true)
					}
					f.collect(t, true, nil)
				}
				for _, p := range f.postures(t) {
					if p.SubjectResourceID == f.ids[1] || p.SubjectResourceID == f.ids[0] {
						// Unread phases still re-read the root snapshot.
						unread := phase == "unread-namespaces" || phase == "unread-groups" || phase == "unread-snapshots"
						healthy := phase == "observed" || phase == "recovered" || (unread && p.SubjectResourceID == f.ids[0])
						if healthy != (p.State == recovery.ProtectionStateProtected) {
							t.Fatalf("phase %s posture=%+v", phase, p)
						}
					}
				}
				want := 2
				wantPoints := 2
				if phase == "confirmed-empty" {
					want = 0
					wantPoints = 0
				} else if phase == "unread-namespaces" || phase == "unread-groups" || phase == "unread-snapshots" {
					wantPoints = 3
				}
				if len(f.live()) != want || len(f.points(t, "pbs-main")) != wantPoints {
					t.Fatalf("phase %s lost or invented inventory", phase)
				}
				frames[phase] = map[string]any{
					"pbs":      map[string]any{"data": map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups}},
					"postures": map[string]any{"data": f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()},
					"points":   f.points(t, "pbs-main"),
				}
			}
			if dest := os.Getenv("PULSE_PBS_RESPONSE_FRAMES"); dest != "" {
				hashes := map[string]string{}
				for _, file := range []string{"monitor_backups.go", "backup_guard.go", "../../pkg/pbs/client.go", "../../pkg/pbs/backup_inventory_response.go"} {
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
