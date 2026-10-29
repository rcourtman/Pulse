package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

type pbsInventoryResponse struct {
	stage string
	body  string
}

func (f *pbsNamespaceContinuityFixture) collectInventoryResponse(t *testing.T, stage, body string) {
	t.Helper()
	f.inventoryResponse.Store(&pbsInventoryResponse{stage: stage, body: body})
	var discoveryErr error
	if stage == "namespace" {
		_, discoveryErr = f.client.ListNamespaces(context.Background(), "archive", "", 0)
		if discoveryErr == nil {
			t.Error("namespace discovery accepted a missing inventory array")
		}
	}
	f.collect(t, stage != "namespace", discoveryErr)
}

func TestPBSMissingInventoryArrayRetainsUnreadScope(t *testing.T) {
	for _, kind := range []string{"vm", "ct"} {
		for _, stage := range []string{"namespace", "groups", "snapshots"} {
			for _, body := range []string{`null`, `{}`, `{"data":null}`} {
				for _, warm := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/%s/warm=%t", kind, stage, body, warm), func(t *testing.T) {
						f := newPBSNamespaceContinuityFixture(t, kind)
						if warm {
							f.collect(t, true, nil)
						}
						before, points := f.live(), f.points(t, "pbs-main")
						key := pbsBackupGroupKey{datastore: "archive", namespace: "tenant", backupType: kind, backupID: "101"}
						cacheAt := f.m.pbsBackupCacheTimeFor("pbs-main", key)
						f.rootAt.Add(15 * 60)
						f.tenantAt.Add(15 * 60) // Require a snapshot read at the existing TTL.
						for range 2 {
							f.collectInventoryResponse(t, stage, body)
							rows := f.live()
							wantCount := 1
							if warm {
								wantCount = 2
								if !reflect.DeepEqual(rows["101"], before["101"]) {
									t.Error("unread file was removed or changed")
								}
							}
							if len(rows) != wantCount || rows["100"].BackupTime.Unix() != f.rootAt.Load() {
								t.Errorf("unread scope invented/lost files or held back its readable sibling: %+v", rows)
							}
							if !f.m.pbsBackupCacheTimeFor("pbs-main", key).Equal(cacheAt) {
								t.Error("missing inventory renewed the snapshot cache")
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
									t.Error("unread historical point was removed or its evidence renewed")
								}
							}
							for _, p := range f.postures(t) {
								// With no accepted point, this guest has no established
								// provider association. Do not invent one from its VMID.
								if !warm && p.SubjectResourceID == f.ids[1] {
									if p.State != recovery.ProtectionStateUnknown || len(p.ProviderStates) != 0 {
										t.Errorf("cold unread guest gained evidence or protection: %+v", p)
									}
									continue
								}
								if p.State == recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 ||
									p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryPartial ||
									p.ProviderStates[0].Permissions != operationaltrust.EvidencePermissionsUnknown {
									t.Errorf("missing inventory invented complete protection or denied access: %+v", p)
								}
							}
						}
						if f.failedReads.Load() != 2 {
							t.Error("invalid inventory caused an extra request or hid a failed read")
						}
						f.inventoryResponse.Store(nil)
						f.collect(t, true, nil)
						if f.live()["101"].BackupTime.Unix() != f.tenantAt.Load() {
							t.Error("ordinary valid read did not restore inventory")
						}
						for _, p := range f.postures(t) {
							if p.State != recovery.ProtectionStateProtected {
								t.Error("complete ordinary recovery did not restore protection")
							}
						}
						f.rootEmpty.Store(true)
						f.tenantEmpty.Store(true)
						f.collect(t, true, nil)
						if len(f.live()) != 0 || len(f.points(t, "pbs-main")) != 0 {
							t.Error("explicit empty arrays did not reconcile confirmed absence")
						}
					})
				}
			}
		}
	}
}

func TestPBSMissingInventoryArrayKeepsOtherConnection(t *testing.T) {
	f := newPBSNamespaceContinuityFixture(t, "vm")
	f.collect(t, true, nil)
	// Populate a second connection through the same ordinary synthetic reader.
	f.m.pollPBSBackups(context.Background(), "pbs-other", f.client, []models.PBSDatastore{{Name: "archive"}}, nil)
	f.wait(t)
	before := f.m.state.GetSnapshot().PBSBackups
	points := f.points(t, "pbs-other")
	f.collectInventoryResponse(t, "groups", `{"data":null}`)
	var retained []models.PBSBackup
	for _, row := range f.m.state.GetSnapshot().PBSBackups {
		if row.Instance == "pbs-other" {
			retained = append(retained, row)
		}
	}
	var original []models.PBSBackup
	for _, row := range before {
		if row.Instance == "pbs-other" {
			original = append(original, row)
		}
	}
	if len(original) != 1 || !reflect.DeepEqual(original, retained) || !reflect.DeepEqual(points, f.points(t, "pbs-other")) {
		t.Error("invalid inventory altered another connection's files or point evidence")
	}
}

// This normal-suite lifecycle can export its actual poll/ingest/posture results
// for an evidence-only production Backups/API-hook desktop/narrow browser pass.
func TestPBSMissingInventoryArrayConsumerLifecycle(t *testing.T) {
	f := newPBSNamespaceContinuityFixture(t, "vm")
	frames := map[string]any{}
	for _, phase := range []string{"observed", "missing-namespaces", "null-groups", "null-snapshots", "recovered", "confirmed-empty"} {
		switch phase {
		case "missing-namespaces":
			f.rootAt.Add(15 * 60)
			f.collectInventoryResponse(t, "namespace", `{}`)
		case "null-groups":
			f.collectInventoryResponse(t, "groups", `{"data":null}`)
		case "null-snapshots":
			f.tenantAt.Add(15 * 60)
			f.collectInventoryResponse(t, "snapshots", `{"data":null}`)
		default:
			f.inventoryResponse.Store(nil)
			if phase == "confirmed-empty" {
				f.rootEmpty.Store(true)
				f.tenantEmpty.Store(true)
			}
			f.collect(t, true, nil)
		}
		postures := f.postures(t)
		for _, p := range postures {
			healthy := phase == "observed" || phase == "recovered"
			if healthy != (p.State == recovery.ProtectionStateProtected) {
				t.Fatalf("phase %s posture=%+v", phase, p)
			}
		}
		want := 2
		if phase == "confirmed-empty" {
			want = 0
		}
		wantPoints := want
		if phase == "missing-namespaces" || phase == "null-groups" || phase == "null-snapshots" {
			// The accepted root advances, but an incomplete scan cannot delete
			// its older historical point while preserving the unread tenant.
			wantPoints = 3
		}
		if len(f.live()) != want || len(f.points(t, "pbs-main")) != wantPoints {
			t.Fatalf("phase %s lost or invented backup facts", phase)
		}
		frames[phase] = map[string]any{"pbs": map[string]any{"data": map[string]any{"backups": f.m.state.GetSnapshot().PBSBackups}}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": f.points(t, "pbs-main")}
	}
	if os.Getenv("PULSE_PBS_INVENTORY_FRAMES") != "1" {
		return
	}
	hashes := map[string]string{}
	for _, file := range []string{"monitor_backups.go", "backup_guard.go", "../../pkg/pbs/client.go"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		hashes[file] = fmt.Sprintf("%x", sha256.Sum256(content))
	}
	data, err := json.Marshal(map[string]any{"runtime_sha256": hashes, "observed_at": time.Now().UTC(), "resource_ids": f.ids, "frames": frames})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("PBS_INVENTORY_FRAMES=" + string(data))
}
