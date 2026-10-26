package monitoring

import (
	"bytes"
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
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// The default local directory has the same storage ID on every PVE node,
// but a successful read on one node says nothing about the other's disk.
type pveSameNameStorageClient struct{ *pveReconciliationClient }

func (c *pveSameNameStorageClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	rows, err := c.pveReconciliationClient.GetStorage(ctx, node)
	for i := range rows {
		rows[i].Storage = "local"
	}
	return rows, err
}

func TestPVELocalStorageFailureRetainsOtherNodeHistory(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			f := newPVEReconciliationFixture(t)
			if reverse {
				f.nodes = []proxmox.Node{f.nodes[1], f.nodes[0]}
			}
			c := &pveSameNameStorageClient{f.client}
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			before := f.m.state.GetSnapshot().PVEBackups.StorageBackups
			if len(before) != 2 {
				t.Fatalf("healthy local controls=%d want2", len(before))
			}
			f.client.contentErr = fmt.Errorf("503 unavailable")
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
			var beforeB, afterB models.StorageBackup
			for _, r := range before {
				if r.VMID == 102 {
					beforeB = r
				}
			}
			for _, r := range rows {
				if r.VMID == 102 {
					afterB = r
				}
			}
			if !reflect.DeepEqual(beforeB, afterB) {
				t.Errorf("failed node's local history disappeared after another node read the same storage name: rows=%+v", rows)
			}
			f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
		})
	}
}

func TestPVELocalStorageReadScopeSurvivesPartialPoll(t *testing.T) {
	for _, failure := range []string{"content", "catalog", "offline"} {
		for _, reverse := range []bool{false, true} {
			for _, emptyReadable := range []bool{false, true} {
				for _, moved := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/reverse=%t/empty=%t/moved=%t", failure, reverse, emptyReadable, moved), func(t *testing.T) {
						f := newPVEReconciliationFixture(t)
						if reverse {
							f.nodes = []proxmox.Node{f.nodes[1], f.nodes[0]}
						}
						c := &pveSameNameStorageClient{f.client}
						poll := func() {
							f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
							f.wait(t)
						}
						if moved {
							// The guest has migrated, but its archive is still read from node-b.
							vms := f.m.state.GetSnapshot().VMs
							for i := range vms {
								if vms[i].VMID == 102 {
									vms[i].Node = "node-a"
								}
							}
							f.m.state.UpdateVMs(vms)
						}
						poll()
						before := f.points(t)
						var rawB models.StorageBackup
						for _, row := range f.m.state.GetSnapshot().PVEBackups.StorageBackups {
							if row.VMID == 102 {
								rawB = row
							}
						}
						if moved && rawB.Node != "node-a" {
							t.Fatalf("fixture failed to separate guest location: %+v", rawB)
						}
						if rawB.StorageNode != "node-b" {
							t.Fatalf("local storage origin=%q want node-b", rawB.StorageNode)
						}
						f.m.state.UpdateStorage([]models.Storage{
							{ID: "local-a", Name: "local", Node: "node-a", Instance: "cluster", Content: "backup", Type: "dir"},
							{ID: "local-b", Name: "local", Node: "node-b", Instance: "cluster", Content: "backup", Type: "dir"},
						})
						f.client.at = f.client.at.Add(15 * time.Minute)
						f.client.emptyAll = emptyReadable
						switch failure {
						case "content":
							f.client.contentErr = fmt.Errorf("503 unavailable")
						case "catalog":
							f.client.nodeErr = fmt.Errorf("503 unavailable")
						case "offline":
							f.online["node-b"] = "offline"
						}
						for range 2 {
							poll()
							var gotB models.StorageBackup
							rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
							for _, row := range rows {
								if row.VMID == 102 {
									gotB = row
								}
							}
							if !reflect.DeepEqual(rawB, gotB) {
								t.Errorf("unread local backup changed or disappeared: rows=%+v", rows)
							}
							want := 2
							if emptyReadable {
								want = 1
							}
							if len(rows) != want {
								t.Errorf("live rows=%d want%d; a successful empty sibling must not retain its own absent files", len(rows), want)
							}
							for id, p := range before {
								if p.SubjectResourceID == f.ids[1] && !reflect.DeepEqual(p, f.points(t)[id]) {
									t.Error("unread point evidence changed")
								}
							}
							f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
						}
						f.client.contentErr = nil
						f.client.nodeErr = nil
						f.online["node-b"] = "online"
						f.client.emptyAll = true
						poll()
						if len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 || len(f.points(t)) != 0 {
							t.Fatal("complete empty recovery failed to clear confirmed-absent files")
						}
						f.client.emptyAll = false
						poll()
						if len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 2 || len(f.points(t)) != 2 {
							t.Fatal("ordinary complete read did not restore both backups")
						}
						f.assertHistory(t, recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient)
					})
				}
			}
		}
	}
}

func TestPVELocalStorageCarryForwardKeepsUnknownOriginsConservative(t *testing.T) {
	failures := pveBackupReadFailures{local: map[pveLocalBackupStorage]struct{}{{node: "failed", storage: "local"}: {}}}
	previous := []models.StorageBackup{
		{ID: "legacy", Instance: "cluster", Storage: "local", Node: "guest-moved"},
		{ID: "independent", Instance: "cluster", Storage: "local", Node: "guest-moved", StorageNode: "healthy"},
		{ID: "affected", Instance: "cluster", Storage: "local", Node: "guest-moved", StorageNode: "failed"},
		{ID: "other", Instance: "other-cluster", Storage: "local", StorageNode: "failed"},
	}
	got, _ := preserveFailedStorageBackups("cluster", models.StateSnapshot{PVEBackups: models.PVEBackups{StorageBackups: previous}}, failures, nil)
	if len(got) != 2 || got[0].ID != "legacy" || got[1].ID != "affected" {
		t.Fatalf("scope/legacy fallback lost isolation: %+v", got)
	}
	// With no node catalog, unknown origins cannot be treated as absent.
	failures = pveBackupReadFailures{nodes: map[string]struct{}{"failed": {}}}
	got, _ = preserveFailedStorageBackups("cluster", models.StateSnapshot{PVEBackups: models.PVEBackups{StorageBackups: previous}}, failures, nil)
	if len(got) != 2 || got[0].ID != "legacy" || got[1].ID != "affected" {
		t.Fatalf("unread-node fallback lost isolation: %+v", got)
	}
	encoded, err := json.Marshal(previous)
	if err != nil || bytes.Contains(encoded, []byte("storageNode")) || bytes.Contains(encoded, []byte("StorageNode")) {
		t.Fatalf("runtime origin leaked into existing JSON contract: %s/%v", encoded, err)
	}
}

// The normal Go suite checks the full accepted/partial/empty lifecycle.
// Exporting these same frames for browser evidence is optional.
func TestPVELocalStorageAcceptedReadLifecycle(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	c := &pveSameNameStorageClient{f.client}
	frames := map[string]any{}
	for _, phase := range []string{"observed", "partial", "recovered", "confirmed-empty"} {
		switch phase {
		case "partial":
			f.client.contentErr = fmt.Errorf("503 unavailable")
			f.client.at = f.client.at.Add(15 * time.Minute)
		case "recovered":
			f.client.contentErr = nil
		case "confirmed-empty":
			f.client.emptyAll = true
		}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
		f.wait(t)
		rows, points, postures := f.m.state.GetSnapshot().PVEBackups, f.points(t), f.postures(t)
		want := 2
		if phase == "confirmed-empty" {
			want = 0
		}
		if len(rows.StorageBackups) != want || len(points) != want {
			t.Fatalf("phase %s live/recorded inventory = %d/%d, want %d", phase, len(rows.StorageBackups), len(points), want)
		}
		for _, p := range postures {
			if phase == "observed" || phase == "recovered" {
				if p.State != recovery.ProtectionStateProtected {
					t.Fatalf("phase %s accepted completed backup is not protected: %+v", phase, p)
				}
			} else if p.State == recovery.ProtectionStateProtected {
				t.Fatalf("phase %s claimed protection without complete current backup evidence", phase)
			}
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": rows}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points}
	}
	if os.Getenv("PULSE_LOCAL_STORAGE_FRAMES") != "1" {
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
	t.Log("LOCAL_STORAGE_FRAMES=" + string(data))
}

// Shared storage still has one authoritative listing, unlike two local disks.
type pveSharedStorageClient struct {
	*pveSameNameStorageClient
	byFlag bool
}

func (c *pveSharedStorageClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	rows, err := c.pveSameNameStorageClient.GetStorage(ctx, node)
	for i := range rows {
		if c.byFlag {
			rows[i].Shared = 1
		} else {
			rows[i].Type = "nfs"
		}
	}
	return rows, err
}

func TestPVELocalStorageScopeKeepsAcceptedSharedAbsence(t *testing.T) {
	for _, byFlag := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("flag=%t/reverse=%t", byFlag, reverse), func(t *testing.T) {
				f := newPVEReconciliationFixture(t)
				if reverse {
					f.nodes = []proxmox.Node{f.nodes[1], f.nodes[0]}
				}
				c := &pveSharedStorageClient{&pveSameNameStorageClient{f.client}, byFlag}
				poll := func() {
					f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
					f.wait(t)
				}
				poll()
				for _, row := range f.m.state.GetSnapshot().PVEBackups.StorageBackups {
					if row.StorageNode != "" {
						t.Fatal("shared listing acquired a local-disk origin")
					}
				}
				f.client.emptyAll = true
				f.client.nodeErr = fmt.Errorf("503 unavailable")
				poll()
				if len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 {
					t.Fatal("unread peer overrode an accepted shared-storage empty listing")
				}
				// Durable reconciliation remains conservative for a partial
				// provider scan, regardless of the live shared listing.
				if len(f.points(t)) != 2 {
					t.Fatal("partial shared read deleted durable history")
				}
				f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
			})
		}
	}
}

func TestPVELocalStorageUnreadNodeDoesNotRetainRemovedSiblingStorage(t *testing.T) {
	for _, offline := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("offline=%t/reverse=%t", offline, reverse), func(t *testing.T) {
				f := newPVEReconciliationFixture(t)
				if reverse {
					f.nodes = []proxmox.Node{f.nodes[1], f.nodes[0]}
				}
				c := &pveSameNameStorageClient{f.client}
				poll := func() {
					f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
					f.wait(t)
				}
				poll()
				f.m.state.UpdateStorage([]models.Storage{
					{ID: "local-a", Name: "local", Node: "node-a", Instance: "cluster", Content: "backup", Type: "dir"},
					{ID: "local-b", Name: "local", Node: "node-b", Instance: "cluster", Content: "backup", Type: "dir"},
				})
				// One catalogue confirms its local target is gone. The other
				// node is unread and cannot establish absence on its own disk.
				f.client.noStorages = true
				if offline {
					f.online["node-b"] = "offline"
				} else {
					f.client.nodeErr = fmt.Errorf("503 unavailable")
				}
				poll()
				rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
				if len(rows) != 1 || rows[0].VMID != 102 {
					t.Fatalf("unread node retained an independently removed local target: %+v", rows)
				}
				f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
				f.client.nodeErr = nil
				f.online["node-b"] = "online"
				poll()
				if len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 || len(f.points(t)) != 0 {
					t.Fatal("complete catalogue absence did not clear the remaining history")
				}
			})
		}
	}
}
