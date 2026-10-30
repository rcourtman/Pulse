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

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Ordinary storage reads only: no guest-agent, backup operation or installed
// target. Identically named local repositories remain independent node scopes.
type storageBudgetClient struct {
	PVEClientInterface
	at         time.Time
	kind       string
	phase      string
	nodeErr    error
	calls      []string
	emptyFirst bool
	afterLate  func()
}

func (c *storageBudgetClient) before(ctx context.Context, phase string) {
	c.calls = append(c.calls, phase)
	if phase == c.phase {
		<-ctx.Done()
		if c.afterLate != nil {
			c.afterLate()
		}
	}
}

func (c *storageBudgetClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	c.before(ctx, "storage/"+node)
	if node == "node-b" && c.nodeErr != nil {
		return nil, c.nodeErr
	}
	if c.phase == "retry" && node == "node-b" {
		return nil, fmt.Errorf("storage query timeout")
	}
	storages := []proxmox.Storage{{Storage: "local", Type: "dir", Content: "backup", Active: 1, Enabled: 1}}
	if c.phase == "assignment" && node == "node-b" {
		storages[0].Active, storages[0].Enabled = 0, 0
	}
	if node == "node-a" {
		storages = append(storages,
			proxmox.Storage{Storage: "tail", Type: "dir", Content: "backup", Active: 1, Enabled: 1},
			proxmox.Storage{Storage: "last", Type: "dir", Content: "backup", Active: 1, Enabled: 1})
	}
	return storages, nil
}

func (c *storageBudgetClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	c.before(ctx, "content/"+node+"/"+storage)
	if c.emptyFirst && node == "node-a" && storage == "local" {
		return nil, nil
	}
	id := map[string]int{"node-a": 101, "node-b": 102, "node-c": 103}[node]
	return []proxmox.StorageContent{{Volid: fmt.Sprintf("%s:backup/vzdump-%s-%d-fixture.vma.zst", storage, c.kind, id),
		VMID: id, CTime: c.at.Unix(), Size: 4096, Content: "backup", Format: "vma.zst", Notes: "accepted " + c.at.Format(time.RFC3339)}}, nil
}

func (c *storageBudgetClient) GetAllStorage(ctx context.Context) ([]proxmox.Storage, error) {
	c.before(ctx, "assignment")
	// This late nil-error configuration would incorrectly excuse node-b if
	// it were accepted after expiry. The unknown local disk must be retained.
	return []proxmox.Storage{{Storage: "local", Nodes: "node-a,node-c"}}, nil
}

func newStorageBudgetFixture(t *testing.T, kind string) (*pveReconciliationFixture, *storageBudgetClient) {
	t.Helper()
	f := newPVEReconciliationFixture(t)
	f.nodes = append(f.nodes, proxmox.Node{Node: "node-c"})
	f.online["node-c"] = "online"
	f.m.state.UpdateVMs(nil)
	f.ids = nil
	for i, node := range []string{"node-a", "node-b", "node-c"} {
		id := 101 + i
		resourceType := unifiedresources.ResourceTypeVM
		if kind == "lxc" {
			resourceType = unifiedresources.ResourceTypeSystemContainer
			f.m.state.UpdateContainersForInstance("cluster", append(f.m.state.GetSnapshot().Containers,
				models.Container{ID: makeGuestID("cluster", node, id), Instance: "cluster", Node: node, VMID: id, Name: fmt.Sprintf("guest-%d", id), Status: "running", Type: "lxc"}))
		} else {
			f.m.state.UpdateVMsForInstance("cluster", append(f.m.state.GetSnapshot().VMs,
				models.VM{ID: makeGuestID("cluster", node, id), Instance: "cluster", Node: node, VMID: id, Name: fmt.Sprintf("guest-%d", id), Status: "running"}))
		}
		f.ids = append(f.ids, unifiedresources.ProxmoxGuestCanonicalID(resourceType, "cluster", id))
	}
	c := &storageBudgetClient{at: f.client.at, kind: kind}
	f.m.pveClients = map[string]PVEClientInterface{"cluster": c}
	f.m.runtimeCtx = context.Background()
	f.m.state.UpdateStorageBackupsForInstance("other-cluster", []models.StorageBackup{{ID: "peer", Instance: "other-cluster", StorageNode: "node-a", Node: "node-a", Storage: "local", VMID: 101, Type: kind, Time: c.at, CTime: c.at.Unix()}})
	return f, c
}

func pollStorageBudget(t *testing.T, f *pveReconciliationFixture, c *storageBudgetClient, ctx context.Context) {
	t.Helper()
	f.m.pollStorageBackupsWithNodes(ctx, "cluster", c, f.nodes, f.online)
	f.wait(t)
}

func storageBudgetFrame(t *testing.T, f *pveReconciliationFixture) map[string]any {
	t.Helper()
	return map[string]any{"pve": map[string]any{"data": f.m.state.GetSnapshot().PVEBackups},
		"postures": map[string]any{"data": f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()},
		"points":   f.points(t)}
}

func TestPVEStorageBudgetPublishesAcceptedListings(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, phase := range []string{"content/node-a/tail", "storage/node-b", "content/node-b/local", "retry", "assignment"} {
			for _, cold := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/cold=%t", kind, phase, cold), func(t *testing.T) {
					f, c := newStorageBudgetFixture(t, kind)
					pollStorageBudget(t, f, c, context.Background())
					before := f.points(t)
					if len(before) != 5 {
						t.Fatalf("healthy control points=%d, want 5", len(before))
					}
					frames := map[string]any{"observed": storageBudgetFrame(t, f)}
					priorRaw := make(map[string]models.StorageBackup)
					for _, raw := range f.m.state.GetSnapshot().PVEBackups.StorageBackups {
						priorRaw[raw.ID] = raw
					}
					if cold {
						f.m.state.UpdateStorageBackupsForInstance("cluster", nil)
					}
					c.at = c.at.Add(15 * time.Minute)
					c.phase, c.calls = phase, nil
					f.m.backupPermissionWarnings = map[string]string{"cluster": "previous access warning"}
					// The timeout starts only after fixture/storage preparation.
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					pollStorageBudget(t, f, c, ctx)
					if ctx.Err() != context.DeadlineExceeded {
						t.Fatal("fixture did not exhaust the actual child deadline")
					}
					if len(c.calls) == 0 || (phase != "retry" && c.calls[len(c.calls)-1] != phase) || (phase == "retry" && c.calls[len(c.calls)-1] != "storage/node-b") {
						t.Fatalf("queued provider reads started after expiry: %+v", c.calls)
					}
					points := f.points(t)
					if len(points) != len(before) {
						t.Fatalf("partial poll deleted durable history: got=%d want=%d", len(points), len(before))
					}
					accepted := func(raw models.StorageBackup) bool {
						return raw.StorageNode == "node-a" && (raw.Storage == "local" || phase != "content/node-a/tail") || raw.StorageNode == "node-c" && phase == "assignment"
					}
					for id, old := range before {
						var raw models.StorageBackup
						for _, candidate := range priorRaw {
							if "pve-backup:"+candidate.ID == id {
								raw = candidate
								break
							}
						}
						if raw.ID == "" {
							t.Fatalf("missing known point origin for %s", id)
						}
						if accepted(raw) {
							if points[id].CompletedAt == nil || !points[id].CompletedAt.Equal(c.at) || reflect.DeepEqual(old, points[id]) {
								t.Error("later timeout discarded an earlier accepted archive observation")
							}
						} else if !reflect.DeepEqual(old, points[id]) {
							t.Error("unread/late local disk renewed its prior durable evidence")
						}
					}
					rawRows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
					expectedRows := 0
					for _, old := range priorRaw {
						if old.Instance == "other-cluster" || !cold || accepted(old) {
							expectedRows++
						}
					}
					if len(rawRows) != expectedRows {
						t.Errorf("partial raw membership: got %d, want %d", len(rawRows), expectedRows)
					}
					for _, raw := range rawRows {
						old, known := priorRaw[raw.ID]
						if !known {
							t.Fatalf("invented raw record: %s", raw.ID)
						}
						if raw.Instance == "other-cluster" || !accepted(raw) {
							if (cold && raw.Instance == "cluster") || !reflect.DeepEqual(raw, old) {
								t.Error("unread or independent raw record was invented or freshened")
							}
						} else {
							expected := old
							// Match the provider-to-model Unix constructor as well as the instant;
							// reflect compares time.Location representation, not just clocks.
							expected.Time, expected.CTime = time.Unix(c.at.Unix(), 0), c.at.Unix()
							expected.Notes = "accepted " + c.at.Format(time.RFC3339)
							if !reflect.DeepEqual(raw, expected) {
								t.Errorf("accepted raw archive did not advance exactly its observed fields: got=%#v want=%#v", raw, expected)
							}
						}
					}
					f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
					if f.m.backupPermissionWarnings["cluster"] != "previous access warning" {
						t.Error("unread scope cleared its prior access warning")
					}
					frames["partial"] = storageBudgetFrame(t, f)
					c.phase, c.calls = "", nil
					pollStorageBudget(t, f, c, context.Background())
					for _, point := range f.points(t) {
						if point.CompletedAt == nil || !point.CompletedAt.Equal(c.at) {
							t.Error("ordinary later poll did not recover every local disk")
						}
					}
					f.assertHistory(t, recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient)
					if f.m.backupPermissionWarnings["cluster"] != "" {
						t.Error("complete accepted enumeration failed to clear the recovered access warning")
					}
					frames["recovered"] = storageBudgetFrame(t, f)
					if os.Getenv("PULSE_STORAGE_BUDGET_FRAMES") == "1" && kind == "qemu" && phase == "content/node-b/local" && !cold {
						source, err := os.ReadFile("monitor_backups.go")
						if err != nil {
							t.Fatal(err)
						}
						data, err := json.Marshal(map[string]any{"runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "frames": frames, "resource_ids": f.ids, "vms": f.m.state.GetSnapshot().VMs})
						if err != nil {
							t.Fatal(err)
						}
						t.Log("PVE_STORAGE_BUDGET_FRAME=" + string(data))
					}
				})
			}
		}
	}
}

func TestPVEStorageBudgetAcceptedEmptyDoesNotKeepRawRows(t *testing.T) {
	f, c := newStorageBudgetFixture(t, "qemu")
	pollStorageBudget(t, f, c, context.Background())
	c.phase, c.emptyFirst = "storage/node-b", true
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	pollStorageBudget(t, f, c, ctx)
	for _, raw := range f.m.state.GetSnapshot().PVEBackups.StorageBackups {
		if raw.Instance == "cluster" && raw.StorageNode == "node-a" && raw.Storage == "local" {
			t.Error("accepted empty scope retained its obsolete raw row")
		}
	}
	if len(f.points(t)) != 5 {
		t.Error("incomplete instance-wide enumeration reconciled durable absence")
	}
	c.phase = ""
	pollStorageBudget(t, f, c, context.Background())
	if len(f.points(t)) != 4 {
		t.Error("ordinary complete enumeration did not reconcile confirmed absence")
	}
}

func TestPVEStorageBudgetOwnerWithdrawalStillBlocksPublication(t *testing.T) {
	for _, transition := range []string{"client", "runtime", "retire"} {
		t.Run(transition, func(t *testing.T) {
			f, c := newStorageBudgetFixture(t, "qemu")
			pollStorageBudget(t, f, c, context.Background())
			before := f.points(t)
			owner := f.m.beginPVEBackupPoll("cluster", c, time.Time{}, time.Now())
			if owner == nil {
				t.Fatal("missing backup owner")
			}
			defer f.m.finishPVEBackupPoll("cluster", owner)
			ctx, cancel := context.WithTimeout(context.WithValue(owner.ctx, pveBackupPollContextKey{}, owner), time.Second)
			defer cancel()
			c.phase, c.at = "content/node-b/local", c.at.Add(15*time.Minute)
			var expected models.PVEBackups
			c.afterLate = func() {
				switch transition {
				case "client":
					f.m.replacePVEClient("cluster", &storageBudgetClient{})
				case "runtime":
					f.m.setRuntimeContext(context.Background(), nil)
				case "retire":
					f.m.retirePVEInstanceRuntime("cluster")
				}
				expected = f.m.state.GetSnapshot().PVEBackups
			}
			pollStorageBudget(t, f, c, ctx)
			if !reflect.DeepEqual(expected, f.m.state.GetSnapshot().PVEBackups) || !reflect.DeepEqual(before, f.points(t)) {
				t.Error("expired scan published after explicit owner withdrawal")
			}
		})
	}
}

func TestPVEStorageBudgetUnreadNodeKeepsUnscopedPBSConfirmation(t *testing.T) {
	for _, mode := range []string{"deadline", "denied"} {
		t.Run(mode, func(t *testing.T) {
			f, c := newStorageBudgetFixture(t, "qemu")
			pollStorageBudget(t, f, c, context.Background())
			f.m.config = &config.Config{PBSInstances: []config.PBSInstance{{Name: "pbs"}}}
			prior := []models.PBSGuestConfirmation{{Storage: "not-in-live-storage-cache", BackupType: "vm", VMID: 102, Time: c.at.Unix()}}
			f.m.state.UpdatePBSGuestConfirmationsForInstance("cluster", prior)
			if mode == "deadline" {
				c.phase = "storage/node-b"
			} else {
				c.nodeErr = fmt.Errorf("403 forbidden")
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			pollStorageBudget(t, f, c, ctx)
			if !reflect.DeepEqual(prior, f.m.state.PBSGuestConfirmationsForInstance("cluster")) {
				t.Error("unread node erased prior PBS attribution without an accepted listing")
			}
			c.phase, c.nodeErr = "", nil
			pollStorageBudget(t, f, c, context.Background())
			if len(f.m.state.PBSGuestConfirmationsForInstance("cluster")) != 0 {
				t.Error("complete read failed to remove absent PBS attribution")
			}
		})
	}
}

// The collection budget cannot stand in for evidence that the recovery reader
// failed. Preserve wanted alerts (and a real prior evaluation warning) until
// an ordinary evaluation runs with a live context.
func TestPVEStorageBudgetDoesNotInventAlertReadFailure(t *testing.T) {
	for _, priorWarning := range []bool{false, true} {
		t.Run(fmt.Sprintf("prior-warning=%t", priorWarning), func(t *testing.T) {
			f, c := newStorageBudgetFixture(t, "qemu")
			pollStorageBudget(t, f, c, context.Background())
			evaluator, _ := backupEvaluationMonitor(t)
			f.m.alertManager = evaluator.alertManager
			f.m.alertManager.RaiseSystemAlert(alerts.SystemAlertInput{Type: "fixture-wanted", Level: alerts.AlertLevelCritical, Fingerprint: "wanted-condition", Message: "Existing wanted condition"})
			if priorWarning {
				f.m.alertManager.RaiseSystemAlert(alerts.SystemAlertInput{Type: alerts.BackupEvaluationAlertType, Level: alerts.AlertLevelWarning, Fingerprint: "prior-observed-read-failure", Message: "Prior observed reader failure"})
			}
			before := f.m.alertManager.GetActiveAlerts()
			want := 1
			if priorWarning {
				want++
			}
			if len(before) != want {
				t.Fatalf("missing wanted/prior alert control: %+v", before)
			}
			c.phase, c.at = "content/node-b/local", c.at.Add(15*time.Minute)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			pollStorageBudget(t, f, c, ctx)
			after := f.m.alertManager.GetActiveAlerts()
			if len(after) != len(before) {
				t.Fatalf("collection expiry invented or cleared an alert: before=%+v after=%+v", before, after)
			}
			byID := make(map[string]alerts.Alert, len(after))
			for _, alert := range after {
				byID[alert.ID] = alert
			}
			for _, old := range before {
				got, ok := byID[old.ID]
				if !ok || got.Type != old.Type || got.Level != old.Level || got.Message != old.Message || !got.StartTime.Equal(old.StartTime) || !reflect.DeepEqual(got.Metadata, old.Metadata) {
					t.Errorf("collection budget changed an existing alert condition: before=%+v after=%+v", old, got)
				}
			}
		})
	}
}
