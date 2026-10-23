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
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Two independent PVE nodes: an ordinary read failure is not an authoritative
// deletion. This fixture drives the real poller, asynchronous ingest and posture
// reader; it neither operates a guest nor changes database/schema internals.
type pveReconciliationClient struct {
	PVEClientInterface
	at           time.Time
	nodeErr      error
	contentErr   error
	emptyB       bool
	emptyAll     bool
	noStorages   bool
	afterContent func()
}

func (c *pveReconciliationClient) GetStorage(_ context.Context, node string) ([]proxmox.Storage, error) {
	if node == "node-b" && c.nodeErr != nil {
		return nil, c.nodeErr
	}
	if c.noStorages {
		return nil, nil
	}
	return []proxmox.Storage{{Storage: "archive-" + node, Type: "dir", Content: "backup", Active: 1, Enabled: 1}}, nil
}

func (c *pveReconciliationClient) GetStorageContent(_ context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	if node == "node-b" && c.contentErr != nil {
		return nil, c.contentErr
	}
	if node == "node-b" && c.afterContent != nil {
		c.afterContent()
	}
	if c.emptyAll || (node == "node-b" && c.emptyB) {
		return nil, nil
	}
	vmid := 101
	if node == "node-b" {
		vmid = 102
	}
	return []proxmox.StorageContent{{Volid: fmt.Sprintf("%s:backup/vzdump-qemu-%d.vma.zst", storage, vmid),
		VMID: vmid, CTime: c.at.Unix(), Size: 2048, Content: "backup", Format: "vma.zst"}}, nil
}

type pveReconciliationFixture struct {
	m      *Monitor
	store  *recoverystore.Store
	client *pveReconciliationClient
	nodes  []proxmox.Node
	online map[string]string
	ids    []string
}

func newPVEReconciliationFixture(t *testing.T) *pveReconciliationFixture {
	t.Helper()
	m, manager := recoveryIngestTestMonitor(t)
	f := &pveReconciliationFixture{m: m, client: &pveReconciliationClient{at: time.Now().UTC().Add(-time.Hour).Truncate(time.Second)},
		nodes: []proxmox.Node{{Node: "node-a"}, {Node: "node-b"}}, online: map[string]string{"node-a": "online", "node-b": "online"}}
	var err error
	f.store, err = manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int{101, 102} {
		f.ids = append(f.ids, unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeVM, "cluster", id))
	}
	f.coldState()
	t.Cleanup(func() { f.wait(t); _ = f.store.Close() })
	return f
}

func (f *pveReconciliationFixture) coldState() {
	f.m.state = models.NewState()
	f.m.state.UpdateVMs([]models.VM{
		{ID: makeGuestID("cluster", "node-a", 101), VMID: 101, Instance: "cluster", Node: "node-a", Name: "guest-101", Status: "running"},
		{ID: makeGuestID("cluster", "node-b", 102), VMID: 102, Instance: "cluster", Node: "node-b", Name: "guest-102", Status: "running"},
	})
}

func (f *pveReconciliationFixture) wait(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		f.m.recoveryIngestMu.Lock()
		running := f.m.recoveryIngestRunning
		f.m.recoveryIngestMu.Unlock()
		if !running {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("PVE backup ingest did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *pveReconciliationFixture) poll(t *testing.T) {
	t.Helper()
	f.m.pollStorageBackupsWithNodes(context.Background(), "cluster", f.client, f.nodes, f.online)
	f.wait(t)
}

func (f *pveReconciliationFixture) points(t *testing.T) map[string]recovery.RecoveryPoint {
	t.Helper()
	points, _, err := f.store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]recovery.RecoveryPoint)
	for _, point := range points {
		out[point.ID] = point
	}
	return out
}

func (f *pveReconciliationFixture) postures(t *testing.T) []recovery.ProtectionPosture {
	t.Helper()
	postures, total, err := f.store.ListProtectionPostures(context.Background(), recovery.ProtectionPostureQuery{SubjectResourceIDs: f.ids})
	if err != nil || total != len(f.ids) {
		t.Fatalf("postures total=%d err=%v", total, err)
	}
	return postures
}

func (f *pveReconciliationFixture) assertHistory(t *testing.T, history recovery.ProtectionHistoryCompleteness, permissions operationaltrust.EvidencePermissions) {
	t.Helper()
	for _, p := range f.postures(t) {
		if len(p.ProviderStates) != 1 || p.ProviderStates[0].HistoryCompleteness != history || p.ProviderStates[0].Permissions != permissions {
			t.Errorf("posture lost source-bound history/access evidence: %+v", p)
		}
		if history != recovery.ProtectionHistoryComplete && p.State == recovery.ProtectionStateProtected {
			t.Errorf("incomplete collection claimed protection: %+v", p)
		}
	}
}

func TestPVEBackupIncompletePollRetainsRecoveryHistory(t *testing.T) {
	for _, tc := range []struct {
		name        string
		change      func(*pveReconciliationFixture)
		history     recovery.ProtectionHistoryCompleteness
		permissions operationaltrust.EvidencePermissions
	}{
		{"node-unavailable", func(f *pveReconciliationFixture) { f.client.nodeErr = fmt.Errorf("503 unavailable") }, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown},
		{"node-denied", func(f *pveReconciliationFixture) { f.client.nodeErr = fmt.Errorf("403 forbidden") }, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsPartial},
		{"node-offline", func(f *pveReconciliationFixture) { f.online["node-b"] = "offline" }, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown},
		{"content-unavailable", func(f *pveReconciliationFixture) { f.client.contentErr = fmt.Errorf("503 unavailable") }, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsUnknown},
		{"content-denied", func(f *pveReconciliationFixture) { f.client.contentErr = fmt.Errorf("403 forbidden") }, recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsPartial},
		{"no-nodes-observed", func(f *pveReconciliationFixture) { f.nodes = nil }, recovery.ProtectionHistoryUnavailable, operationaltrust.EvidencePermissionsUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPVEReconciliationFixture(t)
			f.poll(t)
			original := f.points(t)
			if len(original) != 2 {
				t.Fatalf("healthy control points=%d, want 2", len(original))
			}
			// Durable facts can outlive live inventory, e.g. after an ordinary
			// monitor replacement. No raw-row cache may be assumed here.
			f.coldState()
			f.client.at = f.client.at.Add(15 * time.Minute)
			tc.change(f)
			for range 2 {
				f.poll(t)
				points := f.points(t)
				if len(points) != len(original) {
					t.Errorf("incomplete poll deleted recovery history: points=%d want=%d", len(points), len(original))
				}
				for id, point := range original {
					if point.SubjectResourceID == f.ids[1] && !reflect.DeepEqual(points[id], point) {
						t.Error("unreadable guest history was deleted or freshened without an observation")
					}
				}
				f.assertHistory(t, tc.history, tc.permissions)
				if len(f.nodes) > 0 {
					for _, point := range points {
						if point.SubjectResourceID == f.ids[0] && (point.CompletedAt == nil || !point.CompletedAt.Equal(f.client.at)) {
							t.Error("independent successful node did not publish its fresh result")
						}
					}
				}
			}
		})
	}
}

func TestPVEBackupCompletePollStillReconcilesOnlyItsScope(t *testing.T) {
	for _, noStorages := range []bool{false, true} {
		t.Run(fmt.Sprintf("no-storages=%t", noStorages), func(t *testing.T) {
			f := newPVEReconciliationFixture(t)
			f.poll(t)
			seed := []recovery.RecoveryPoint{
				recoveryIngestTestPoint("pve-backup:other", "other-cluster", f.client.at),
				recoveryIngestTestPoint("pve-task:task", "cluster", f.client.at),
			}
			if err := f.store.UpsertPoints(context.Background(), seed); err != nil {
				t.Fatal(err)
			}
			f.client.emptyB = true
			f.poll(t)
			if len(f.points(t)) != 3 {
				t.Fatal("complete poll did not remove exactly the confirmed-absent backup")
			}
			f.client.emptyAll, f.client.noStorages = true, noStorages
			f.poll(t)
			points := f.points(t)
			if len(points) != 2 || points[seed[0].ID].ID == "" || points[seed[1].ID].ID == "" {
				t.Fatalf("empty complete poll crossed instance or evidence-class scope: %+v", points)
			}
			f.client.emptyB, f.client.emptyAll, f.client.noStorages = false, false, false
			f.poll(t)
			if len(f.points(t)) != 4 {
				t.Fatal("ordinary complete poll did not recover both backups")
			}
			f.assertHistory(t, recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient)
		})
	}
}

func TestPVEBackupCancelledPollCannotPublishAnEmptySuccess(t *testing.T) {
	f := newPVEReconciliationFixture(t)
	f.poll(t)
	beforePoints, beforePostures := f.points(t), f.postures(t)
	beforeRaw := f.m.state.GetSnapshot().PVEBackups.StorageBackups
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.client.emptyAll, f.client.afterContent = true, cancel
	f.m.pollStorageBackupsWithNodes(ctx, "cluster", f.client, f.nodes, f.online)
	f.wait(t)
	if !reflect.DeepEqual(beforePoints, f.points(t)) || !reflect.DeepEqual(beforeRaw, f.m.state.GetSnapshot().PVEBackups.StorageBackups) {
		t.Error("cancelled poll published empty backup inventory or deleted durable history")
	}
	// evaluatedAt is a reader timestamp; compare provider observations instead.
	for i, p := range f.postures(t) {
		if !reflect.DeepEqual(p.ProviderStates, beforePostures[i].ProviderStates) {
			t.Error("cancelled poll renewed provider evidence without a committed payload")
		}
	}
}

func TestPVEBackupReconciliationProducer(t *testing.T) {
	if os.Getenv("PULSE_PVE_RECONCILIATION_EXPORT") != "1" {
		t.Skip("producer export is opt-in")
	}
	f := newPVEReconciliationFixture(t)
	frames := map[string]any{}
	capture := func(name string) {
		frames[name] = map[string]any{"pve": map[string]any{"data": f.m.state.GetSnapshot().PVEBackups},
			"postures": map[string]any{"data": f.postures(t), "policy": recovery.DefaultProtectionPosturePolicy.Payload()}, "points": f.points(t)}
	}
	f.poll(t)
	capture("observed")
	f.coldState()
	f.client.nodeErr = fmt.Errorf("503 unavailable")
	f.poll(t)
	capture("partial")
	f.client.nodeErr = fmt.Errorf("403 forbidden")
	f.poll(t)
	capture("denied")
	f.client.nodeErr = nil
	f.poll(t)
	capture("recovered")
	f.client.emptyAll = true
	f.poll(t)
	capture("confirmed-empty")
	source, err := os.ReadFile("monitor_backups.go")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"runtime_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "observed_at": time.Now().UTC(), "resource_ids": f.ids, "frames": frames})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("PVE_RECONCILIATION_PRODUCER=" + string(data))
}
