package monitoring

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Identically named local disks are independent scopes even when one is
// disabled. No disabled content endpoint may be queried to prove that fact.
type disabledLocalBackupClient struct {
	*pveSameNameStorageClient
	disabled       map[string]bool
	kind           string
	shared         bool
	storageType    string
	config         []proxmox.Storage
	configErr      error
	configCalls    int
	contentQueries map[string]int
	afterConfig    func()
}

func (c *disabledLocalBackupClient) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	rows, err := c.pveSameNameStorageClient.GetStorage(ctx, node)
	for i := range rows {
		if c.disabled[node] {
			rows[i].Enabled = 0
		}
		if c.shared {
			rows[i].Shared = 1
		}
		if c.storageType != "" {
			rows[i].Type = c.storageType
		}
	}
	return rows, err
}

func (c *disabledLocalBackupClient) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	c.contentQueries[node]++
	if c.disabled[node] {
		return nil, fmt.Errorf("unexpected disabled content read")
	}
	rows, err := c.pveSameNameStorageClient.GetStorageContent(ctx, node, storage)
	if c.kind == "lxc" {
		for i := range rows {
			rows[i].Volid = strings.ReplaceAll(rows[i].Volid, "qemu", "lxc")
		}
	}
	return rows, err
}

func (c *disabledLocalBackupClient) GetAllStorage(context.Context) ([]proxmox.Storage, error) {
	c.configCalls++
	if c.afterConfig != nil {
		c.afterConfig()
	}
	return c.config, c.configErr
}

func newDisabledLocalBackupFixture(t *testing.T, kind string) (*pveReconciliationFixture, *disabledLocalBackupClient) {
	t.Helper()
	f := newPVEReconciliationFixture(t)
	c := &disabledLocalBackupClient{pveSameNameStorageClient: &pveSameNameStorageClient{f.client}, kind: kind,
		config: []proxmox.Storage{{Storage: "local", Type: "dir"}}, contentQueries: make(map[string]int)}
	if kind == "lxc" {
		f.m.state.UpdateVMs(nil)
		f.m.state.UpdateContainers([]models.Container{
			{ID: makeGuestID("cluster", "node-a", 101), VMID: 101, Instance: "cluster", Node: "node-a", Name: "guest-101", Status: "running"},
			{ID: makeGuestID("cluster", "node-b", 102), VMID: 102, Instance: "cluster", Node: "node-b", Name: "guest-102", Status: "running"},
		})
		for i, id := range []int{101, 102} {
			f.ids[i] = unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "cluster", id)
		}
	}
	return f, c
}

func TestPVEDisabledLocalStorageDoesNotLoseKnownHistory(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, reverse := range []bool{false, true} {
			for _, cold := range []bool{false, true} {
				for _, emptyPeer := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/reverse=%t/cold=%t/empty-peer=%t", kind, reverse, cold, emptyPeer), func(t *testing.T) {
						f, c := newDisabledLocalBackupFixture(t, kind)
						if reverse {
							f.nodes[0], f.nodes[1] = f.nodes[1], f.nodes[0]
						}
						poll := func() {
							f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
							f.wait(t)
						}
						poll()
						before, raw := f.points(t), f.m.state.GetSnapshot().PVEBackups.StorageBackups
						if len(before) != 2 || len(raw) != 2 {
							t.Fatal("healthy controls did not produce two independent backups")
						}
						if cold {
							f.m.state.UpdateStorageBackupsForInstance("cluster", nil)
						}
						c.disabled = map[string]bool{"node-b": true}
						c.contentQueries = make(map[string]int)
						f.client.at = f.client.at.Add(15 * time.Minute)
						f.client.emptyAll = emptyPeer
						for range 2 {
							poll()
							points := f.points(t)
							if len(points) != 2 {
								t.Errorf("disabled local disk was reconciled as absent: points=%d want2", len(points))
							}
							for id, p := range before {
								if p.SubjectResourceID == f.ids[1] && !reflect.DeepEqual(p, points[id]) {
									t.Error("unread local point disappeared or renewed its evidence")
								}
							}
							rows := f.m.state.GetSnapshot().PVEBackups.StorageBackups
							want := 2
							if cold {
								want--
							}
							if emptyPeer {
								want--
							}
							if len(rows) != want {
								t.Errorf("live local inventory=%d want%d; readable sibling cannot cover unread disk", len(rows), want)
							}
							if !cold {
								for _, old := range raw {
									if old.VMID != 102 {
										continue
									}
									var retained models.StorageBackup
									for _, r := range rows {
										if r.VMID == 102 {
											retained = r
										}
									}
									if !reflect.DeepEqual(old, retained) {
										t.Error("disabled raw row changed or disappeared")
									}
								}
							}
							f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
						}
						if c.contentQueries["node-b"] != 0 || c.contentQueries["node-a"] != 2 {
							t.Fatalf("no-query/independent-read boundary failed: %+v", c.contentQueries)
						}
						if c.configCalls != 2 {
							t.Errorf("ambiguous membership reads=%d want one per poll", c.configCalls)
						}
						c.disabled = nil
						f.client.emptyAll = false
						f.client.emptyB = true
						poll()
						if len(f.points(t)) != 1 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 1 {
							t.Error("successful empty read after re-enable did not remove only its absent point")
						}
						f.client.emptyB = false
						poll()
						if len(f.points(t)) != 2 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 2 {
							t.Error("ordinary accepted read did not restore both backups")
						}
						f.assertHistory(t, recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient)
					})
				}
			}
		}
	}
}

func TestPVEDisabledLocalStorageUnknownAssignmentIsNotAbsence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config []proxmox.Storage
		err    error
	}{
		{name: "missing-local-config"},
		{name: "transient-config", err: fmt.Errorf("503 unavailable")},
		{name: "denied-config", err: fmt.Errorf("403 forbidden")},
		{name: "allowed-node-not-peer", config: []proxmox.Storage{{Storage: "local", Nodes: "node-a,node-b"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, c := newDisabledLocalBackupFixture(t, "qemu")
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			before := f.points(t)
			c.disabled = map[string]bool{"node-b": true}
			c.config = tc.config
			c.configErr = tc.err
			f.client.emptyAll = true
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			if len(f.points(t)) != 2 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 1 {
				t.Error("unknown assignment deleted local history")
			}
			for id, p := range before {
				if p.SubjectResourceID == f.ids[1] && !reflect.DeepEqual(p, f.points(t)[id]) {
					t.Error("unknown assignment renewed unread evidence")
				}
			}
			f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
		})
	}
}

func TestPVEDisabledLocalStorageRespectsConfirmedRestriction(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprintf("reverse=%t", reverse), func(t *testing.T) {
			f, c := newDisabledLocalBackupFixture(t, "qemu")
			if reverse {
				f.nodes[0], f.nodes[1] = f.nodes[1], f.nodes[0]
			}
			c.disabled = map[string]bool{"node-b": true}
			c.config = []proxmox.Storage{{Storage: "local", Nodes: " node-a "}}
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			if len(f.points(t)) != 1 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 1 || c.contentQueries["node-b"] != 0 {
				t.Error("confirmed node restriction lost its no-query/accepted-scope boundary")
			}
			for _, p := range f.postures(t) {
				if p.SubjectResourceID == f.ids[0] && (p.State != recovery.ProtectionStateProtected || len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != recovery.ProtectionHistoryComplete) {
					t.Errorf("confirmed assigned scope not complete: %+v", p)
				}
			}
		})
	}
}

func TestPVEDisabledLocalStorageSharedReadStillCoversPeer(t *testing.T) {
	for _, storageType := range []string{"dir", "nfs"} {
		t.Run(storageType, func(t *testing.T) {
			f, c := newDisabledLocalBackupFixture(t, "qemu")
			c.storageType = storageType
			c.shared = storageType == "dir"
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			c.disabled = map[string]bool{"node-b": true}
			f.client.emptyAll = true
			f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
			f.wait(t)
			if len(f.points(t)) != 0 || len(f.m.state.GetSnapshot().PVEBackups.StorageBackups) != 0 {
				t.Error("skipped shared peer overrode accepted shared absence")
			}
			if c.configCalls != 0 {
				t.Error("unambiguous shared read acquired unnecessary config")
			}
		})
	}
}

func TestPVEDisabledLocalStorageConfigCancellationDoesNotPublish(t *testing.T) {
	f, c := newDisabledLocalBackupFixture(t, "qemu")
	f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
	f.wait(t)
	before, raw := f.points(t), f.m.state.GetSnapshot().PVEBackups.StorageBackups
	postures := f.postures(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.disabled = map[string]bool{"node-b": true}
	c.afterConfig = cancel
	f.client.emptyAll = true
	f.m.pollStorageBackupsWithNodes(ctx, "cluster", c, f.nodes, f.online)
	f.wait(t)
	if !reflect.DeepEqual(raw, f.m.state.GetSnapshot().PVEBackups.StorageBackups) || !reflect.DeepEqual(before, f.points(t)) {
		t.Error("cancelled assignment read published inventory or reconciled points")
	}
	after := f.postures(t)
	for i := range after {
		if !reflect.DeepEqual(postures[i].ProviderStates, after[i].ProviderStates) {
			t.Error("cancelled assignment read renewed provider evidence")
		}
	}
}

// The standard Go suite checks this producer's lifecycle. Its optional export
// feeds the real Backups consumer as evidence, not a separate application.
func TestPVEDisabledLocalStorageAcceptedReadLifecycle(t *testing.T) {
	f, c := newDisabledLocalBackupFixture(t, "lxc")
	frames := map[string]any{}
	for _, phase := range []string{"observed", "disabled", "assignment-unavailable", "recovered", "confirmed-empty"} {
		switch phase {
		case "disabled":
			c.disabled = map[string]bool{"node-b": true}
			f.client.at = f.client.at.Add(15 * time.Minute)
		case "assignment-unavailable":
			c.configErr = fmt.Errorf("503 unavailable")
		case "recovered":
			c.disabled = nil
			c.configErr = nil
		case "confirmed-empty":
			f.client.emptyB = true
		}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
		f.wait(t)
		rows, points, postures := f.m.state.GetSnapshot().PVEBackups, f.points(t), f.postures(t)
		want := 2
		if phase == "confirmed-empty" {
			want = 1
		}
		if len(rows.StorageBackups) != want || len(points) != want {
			t.Fatalf("phase %s inventory/points=%d/%d want%d", phase, len(rows.StorageBackups), len(points), want)
		}
		if phase == "disabled" || phase == "assignment-unavailable" {
			f.assertHistory(t, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown)
		}
		frames[phase] = map[string]any{"pve": map[string]any{"data": rows}, "postures": map[string]any{"data": postures, "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": points}
	}
	if os.Getenv("PULSE_DISABLED_LOCAL_FRAMES") != "1" {
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
	t.Log("DISABLED_LOCAL_FRAMES=" + string(data))
}

func TestPVEDisabledLocalStorageAllSkippedRemainsUnavailable(t *testing.T) {
	f, c := newDisabledLocalBackupFixture(t, "lxc")
	f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
	f.wait(t)
	before, raw := f.points(t), f.m.state.GetSnapshot().PVEBackups.StorageBackups
	c.disabled = map[string]bool{"node-a": true, "node-b": true}
	c.contentQueries = make(map[string]int)
	for range 2 {
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
		f.wait(t)
		if !reflect.DeepEqual(before, f.points(t)) || !reflect.DeepEqual(raw, f.m.state.GetSnapshot().PVEBackups.StorageBackups) {
			t.Error("all-skipped collection changed accepted facts")
		}
		f.assertHistory(t, recovery.ProtectionHistoryUnavailable, operationaltrust.EvidencePermissionsUnknown)
	}
	if c.configCalls != 0 || len(c.contentQueries) != 0 {
		t.Error("unambiguously unread collection acquired unnecessary requests")
	}
}

func TestPVEDisabledLocalStorageDoesNotChangeAnotherConnection(t *testing.T) {
	f, c := newDisabledLocalBackupFixture(t, "qemu")
	otherRaw := []models.StorageBackup{{ID: "other-cluster-local-file", Instance: "other-cluster", Node: "node-b", StorageNode: "node-b", Storage: "local", VMID: 102, Time: f.client.at, CTime: f.client.at.Unix()}}
	otherPoint := recoveryIngestTestPoint("pve-backup:other-cluster-file", "other-cluster", f.client.at)
	if err := f.store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{otherPoint}); err != nil {
		t.Fatal(err)
	}
	otherPoint = f.points(t)[otherPoint.ID]
	f.m.state.UpdateStorageBackupsForInstance("other-cluster", otherRaw)
	for _, disabled := range []bool{false, true, false} {
		c.disabled = map[string]bool{"node-b": disabled}
		f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", c, f.nodes, f.online)
		f.wait(t)
		var gotOther []models.StorageBackup
		for _, row := range f.m.state.GetSnapshot().PVEBackups.StorageBackups {
			if row.Instance == "other-cluster" {
				gotOther = append(gotOther, row)
			}
		}
		if !reflect.DeepEqual(otherRaw, gotOther) || !reflect.DeepEqual(otherPoint, f.points(t)[otherPoint.ID]) {
			t.Error("local skip crossed connection scope")
		}
	}
}
