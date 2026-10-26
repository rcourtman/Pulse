package monitoring

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationaltrust"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	recoverystore "github.com/rcourtman/pulse-go-rewrite/internal/recovery/store"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

func recoveryIngestTestMonitor(t *testing.T) (*Monitor, *recoverymanager.Manager) {
	t.Helper()
	mtp := config.NewMultiTenantPersistence(t.TempDir())
	manager := recoverymanager.New(mtp)
	return &Monitor{recoveryManager: manager}, manager
}

func recoveryIngestTestPoint(id string, instance string, at time.Time) recovery.RecoveryPoint {
	return recovery.RecoveryPoint{
		ID:          id,
		Provider:    recovery.ProviderProxmoxPVE,
		Kind:        recovery.KindBackup,
		Mode:        recovery.ModeLocal,
		Outcome:     recovery.OutcomeSuccess,
		StartedAt:   &at,
		CompletedAt: &at,
		Details:     map[string]any{"instance": instance},
	}
}

func listRecoveryPointIDs(t *testing.T, manager *recoverymanager.Manager) map[string]bool {
	t.Helper()
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	points, _, err := store.ListPoints(context.Background(), recovery.ListPointsOptions{})
	if err != nil {
		t.Fatalf("ListPoints(): %v", err)
	}
	ids := make(map[string]bool, len(points))
	for _, p := range points {
		ids[p.ID] = true
	}
	return ids
}

// Regression test for the monitoring-layer wiring of #1580: a poll cycle's
// complete enumeration must delete in-scope points the source stopped
// reporting, so a backup deleted upstream cannot keep re-raising its alert.
func TestIngestRecoveryPointsBestEffortReconcilesScope(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	seed := []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-101", "pve1", now),
		// Same class, different instance: out of scope, must survive.
		recoveryIngestTestPoint("pve-backup:pve2-vzdump-100", "pve2", now),
	}
	if err := store.UpsertPoints(context.Background(), seed); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	// The next poll of pve1 enumerates only vzdump-100.
	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points: []recovery.RecoveryPoint{
			recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		},
		reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE),
			idPrefix: "pve-backup:",
			instance: "pve1",
		},
	})

	ids := listRecoveryPointIDs(t, manager)
	if ids["pve-backup:pve1-vzdump-101"] {
		t.Fatal("stale point pve-backup:pve1-vzdump-101 should have been reconciled away")
	}
	if !ids["pve-backup:pve1-vzdump-100"] {
		t.Fatal("still-enumerated point pve-backup:pve1-vzdump-100 must survive")
	}
	if !ids["pve-backup:pve2-vzdump-100"] {
		t.Fatal("other-instance point pve-backup:pve2-vzdump-100 must survive")
	}
}

// An empty enumeration is meaningful: the source reports no backups at all,
// so the whole scope clears (the delete-all-backups case from #1580).
func TestIngestRecoveryPointsBestEffortEmptyEnumerationClearsScope(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		recoveryIngestTestPoint("pve-backup:pve2-vzdump-100", "pve2", now),
	}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points: nil,
		reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE),
			idPrefix: "pve-backup:",
			instance: "pve1",
		},
	})

	ids := listRecoveryPointIDs(t, manager)
	if ids["pve-backup:pve1-vzdump-100"] {
		t.Fatal("pve1 scope should be empty after an empty enumeration")
	}
	if !ids["pve-backup:pve2-vzdump-100"] {
		t.Fatal("other-instance point must survive an empty pve1 enumeration")
	}
}

// A failed upsert must not reconcile: deleting on top of a failed batch could
// drop points the source still reports.
func TestIngestRecoveryPointsBestEffortSkipsReconcileWhenUpsertFails(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}

	now := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		recoveryIngestTestPoint("pve-backup:pve1-vzdump-101", "pve1", now),
	}); err != nil {
		t.Fatalf("UpsertPoints(): %v", err)
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	m.ingestRecoveryPointsBestEffort(cancelled, recoveryIngestBatch{
		points: []recovery.RecoveryPoint{
			recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now),
		},
		reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE),
			idPrefix: "pve-backup:",
			instance: "pve1",
		},
	})

	ids := listRecoveryPointIDs(t, manager)
	if !ids["pve-backup:pve1-vzdump-101"] {
		t.Fatal("reconcile must be skipped when the upsert fails; vzdump-101 was deleted")
	}
}

func TestIngestRecoveryPointsBestEffortPersistsProviderObservationAndPosture(t *testing.T) {
	t.Parallel()

	m, manager := recoveryIngestTestMonitor(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	point := recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now)
	point.SubjectResourceID = "resource:vm-100"
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		"pve1",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}

	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points: []recovery.RecoveryPoint{point},
		observations: []recovery.ProtectionProviderObservation{
			observation,
		},
	})

	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	postures, _, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"resource:vm-100"},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures() error = %v", err)
	}
	if len(postures) != 1 ||
		postures[0].State != recovery.ProtectionStateProtected {
		t.Fatalf("postures = %#v, want one protected posture", postures)
	}
}

func TestIngestRecoveryPointsBestEffortPersistsProviderObservationBeforePointFailure(t *testing.T) {
	t.Parallel()

	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	now := time.Now().UTC().Truncate(time.Millisecond)
	existing := recoveryIngestTestPoint("pve-backup:pve1-vzdump-100", "pve1", now)
	existing.SubjectResourceID = "resource:vm-100"
	if err := store.UpsertPoints(context.Background(), []recovery.RecoveryPoint{existing}); err != nil {
		t.Fatalf("UpsertPoints(existing): %v", err)
	}
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		"pve1",
		recovery.OutcomeSuccess,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		now,
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation() error = %v", err)
	}

	invalidPoint := existing
	invalidPoint.ID = ""
	m.ingestRecoveryPointsBestEffort(context.Background(), recoveryIngestBatch{
		points:       []recovery.RecoveryPoint{invalidPoint},
		observations: []recovery.ProtectionProviderObservation{observation},
	})

	postures, _, err := store.ListProtectionPostures(
		context.Background(),
		recovery.ProtectionPostureQuery{
			SubjectResourceIDs: []string{"resource:vm-100"},
		},
	)
	if err != nil {
		t.Fatalf("ListProtectionPostures() error = %v", err)
	}
	if len(postures) != 1 ||
		postures[0].State != recovery.ProtectionStateProtected {
		t.Fatalf(
			"postures = %#v, want provider observation to survive point failure",
			postures,
		)
	}
}

func TestRecoveryIngestAcceptedProofCoalescesSameScope(t *testing.T) {
	monitor := &Monitor{recoveryIngestRunning: true}
	scope := recoveryReconcileScope{
		provider: string(recovery.ProviderProxmoxPBS),
		idPrefix: "pbs-backup:",
		instance: "pbs-large",
	}

	monitor.enqueueRecoveryIngest(recoveryIngestBatch{
		points:    []recovery.RecoveryPoint{{ID: "pbs-backup:superseded"}},
		reconcile: &scope,
	})
	for i := 1; i < 64; i++ {
		monitor.enqueueRecoveryIngest(recoveryIngestBatch{
			points:    []recovery.RecoveryPoint{{ID: "pbs-backup:latest"}},
			reconcile: &scope,
		})
	}
	if got := len(monitor.recoveryIngestPending); got != 1 {
		t.Fatalf("pending complete enumerations = %d, want one same-scope latest batch", got)
	}
	if got := monitor.recoveryIngestPending[0].points[0].ID; got != "pbs-backup:latest" {
		t.Fatalf("pending point = %q, want latest complete enumeration", got)
	}
}

func TestRecoveryIngestPendingKeepsDistinctScopesAndEventBatches(t *testing.T) {
	monitor := &Monitor{recoveryIngestRunning: true}
	pbsScope := recoveryReconcileScope{
		provider: string(recovery.ProviderProxmoxPBS),
		idPrefix: "pbs-backup:",
		instance: "pbs-a",
	}
	pveScope := recoveryReconcileScope{
		provider: string(recovery.ProviderProxmoxPVE),
		idPrefix: "pve-snapshot:",
		instance: "pve-a",
	}

	monitor.enqueueRecoveryIngest(recoveryIngestBatch{
		points:    []recovery.RecoveryPoint{{ID: "pbs-backup:1"}},
		reconcile: &pbsScope,
	})
	monitor.enqueueRecoveryIngest(recoveryIngestBatch{
		points:    []recovery.RecoveryPoint{{ID: "pve-snapshot:1"}},
		reconcile: &pveScope,
	})
	monitor.enqueueRecoveryIngest(recoveryIngestBatch{
		points: []recovery.RecoveryPoint{{ID: "pve-task:1"}},
	})

	if got := len(monitor.recoveryIngestPending); got != 3 {
		t.Fatalf("pending batches = %d, want two source enumerations and one event batch", got)
	}
}

// Partial PBS batches must stay before the complete recovery that supersedes
// them: coalescing across a non-authoritative batch would resurrect deleted
// points after a later successful empty enumeration.
func TestRecoveryIngestCompletePollCannotOvertakePartialBatch(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m.recoveryIngestRunning = true
	scope := recoveryReconcileScope{provider: string(recovery.ProviderProxmoxPVE), idPrefix: "pve-backup:", instance: "pve1"}
	point := recoveryIngestTestPoint("pve-backup:old", "pve1", time.Now().UTC())
	m.enqueueRecoveryIngest(recoveryIngestBatch{points: []recovery.RecoveryPoint{point}, reconcile: &scope})
	m.enqueueRecoveryIngest(recoveryIngestBatch{points: []recovery.RecoveryPoint{point}})
	m.enqueueRecoveryIngest(recoveryIngestBatch{reconcile: &scope})
	// Repeated complete recovery may still coalesce on its own side of the barrier.
	m.enqueueRecoveryIngest(recoveryIngestBatch{reconcile: &scope})
	if got := len(m.recoveryIngestPending); got != 3 {
		t.Fatalf("pending=%d, want complete/partial/complete in source order", got)
	}
	for _, batch := range m.recoveryIngestPending {
		m.ingestRecoveryPointsBestEffort(context.Background(), batch)
	}
	if ids := listRecoveryPointIDs(t, manager); len(ids) != 0 {
		t.Fatalf("partial history resurrected after authoritative empty recovery: %v", ids)
	}
}

// Snapshot inventory in memory can be empty after an ordinary monitor
// replacement while its durable recovery points still exist. Exercise the
// actual poll, ingest queue and public store reader, not database internals.
type snapshotRecoveryClient struct {
	*snapshotCollectionClient
	afterStorage func()
}

func (c *snapshotRecoveryClient) GetStorage(context.Context, string) ([]proxmox.Storage, error) {
	if c.afterStorage != nil {
		c.afterStorage()
	}
	return nil, nil
}

type snapshotRecoveryFixture struct {
	m      *Monitor
	store  *recoverystore.Store
	client *snapshotRecoveryClient
	kind   string
}

func newSnapshotRecoveryFixture(t *testing.T, kind string) *snapshotRecoveryFixture {
	t.Helper()
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatal(err)
	}
	f := &snapshotRecoveryFixture{m: m, store: store, kind: kind,
		client: &snapshotRecoveryClient{snapshotCollectionClient: &snapshotCollectionClient{replies: map[string]snapshotCollectionReply{}}}}
	f.coldState(true)
	at := time.Now().UTC().Add(-16 * 24 * time.Hour).Truncate(time.Second)
	for _, guest := range []string{"node-a/101", "node-b/202"} {
		f.client.replies[kind+"/"+guest] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{{Name: "weekly", SnapTime: at.Unix(), Description: "accepted listing"}}}
	}
	t.Cleanup(func() { f.wait(t); _ = store.Close() })
	return f
}

func (f *snapshotRecoveryFixture) coldState(withInventory bool) {
	f.m.state = models.NewState()
	if !withInventory {
		return
	}
	for _, site := range []string{"site", "other-site"} {
		var vms []models.VM
		var containers []models.Container
		for _, guest := range []struct {
			node string
			id   int
		}{{"node-a", 101}, {"node-b", 202}} {
			if site == "other-site" && guest.id != 101 {
				continue
			}
			id := makeGuestID(site, guest.node, guest.id)
			if f.kind == "qemu" {
				vms = append(vms, models.VM{ID: id, Instance: site, Node: guest.node, VMID: guest.id, Name: "database", Type: f.kind})
			} else {
				containers = append(containers, models.Container{ID: id, Instance: site, Node: guest.node, VMID: guest.id, Name: "database", Type: f.kind})
			}
		}
		f.m.state.UpdateGuestsForInstance(site, vms, containers)
	}
}

func (f *snapshotRecoveryFixture) wait(t *testing.T) {
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
			t.Fatal("snapshot recovery ingest did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *snapshotRecoveryFixture) poll(t *testing.T, site string) {
	t.Helper()
	f.m.pollGuestSnapshots(context.Background(), site, f.client)
	f.wait(t)
}

func (f *snapshotRecoveryFixture) points(t *testing.T) map[string]recovery.RecoveryPoint {
	t.Helper()
	points, _, err := f.store.ListPoints(context.Background(), recovery.ListPointsOptions{Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]recovery.RecoveryPoint, len(points))
	for _, point := range points {
		out[point.ID] = point
	}
	return out
}

func TestPVESnapshotIncompletePollRetainsRecoveryHistory(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, cold := range []bool{false, true} {
			for _, failure := range []string{"503 unavailable", "403 forbidden", "596 node unavailable", "invalid snapshot response"} {
				t.Run(fmt.Sprintf("%s/cold=%t/%s", kind, cold, failure), func(t *testing.T) {
					f := newSnapshotRecoveryFixture(t, kind)
					f.poll(t, "site")
					f.poll(t, "other-site")
					before := f.points(t)
					if len(before) != 3 {
						t.Fatalf("control has %d points, want three", len(before))
					}
					if cold {
						f.coldState(true)
					}
					// A rejected response may contain data, but neither its rows
					// nor its absence can replace the last accepted observation.
					f.client.replies[kind+"/node-a/101"] = snapshotCollectionReply{
						snapshots: []proxmox.Snapshot{{Name: "unaccepted", SnapTime: time.Now().Unix()}}, err: fmt.Errorf("%s", failure)}
					f.client.replies[kind+"/node-b/202"] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{{Name: "fresh-peer", SnapTime: time.Now().Add(-time.Hour).Unix()}}}
					for range 3 {
						f.poll(t, "site")
					}
					after := f.points(t)
					failedID, otherID := "pve-snapshot:site-node-a-101-weekly", "pve-snapshot:other-site-node-a-101-weekly"
					if !reflect.DeepEqual(before[failedID], after[failedID]) {
						t.Fatalf("failed guest lost or renewed its durable point: before=%+v after=%+v", before[failedID], after[failedID])
					}
					if !reflect.DeepEqual(before[otherID], after[otherID]) || len(after) != 4 || after["pve-snapshot:site-node-b-202-fresh-peer"].ID == "" {
						t.Fatalf("partial poll changed other scope or failed to ingest accepted peer: %+v", after)
					}
					// Deletion waits for a complete pass, not for another guest
					// to happen to succeed. Accepted empty lists still remove
					// this instance's snapshots once the full scope is readable.
					f.client.replies[kind+"/node-a/101"] = snapshotCollectionReply{}
					f.client.replies[kind+"/node-b/202"] = snapshotCollectionReply{}
					f.poll(t, "site")
					after = f.points(t)
					if len(after) != 1 || !reflect.DeepEqual(before[otherID], after[otherID]) {
						t.Fatalf("complete empty listing failed to clear only its own scope: %+v", after)
					}
				})
			}
		}
	}
}

func TestPVESnapshotUnobservedInventoryPreservesRecoveryHistory(t *testing.T) {
	for _, kind := range []string{"qemu", "lxc"} {
		for _, noInventory := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/no-inventory=%t", kind, noInventory), func(t *testing.T) {
				f := newSnapshotRecoveryFixture(t, kind)
				f.poll(t, "site")
				before := f.points(t)
				f.coldState(!noInventory)
				for key := range f.client.replies {
					f.client.replies[key] = snapshotCollectionReply{err: fmt.Errorf("503 unavailable")}
				}
				f.poll(t, "site")
				if !reflect.DeepEqual(before, f.points(t)) {
					t.Fatal("unobserved scope deleted or renewed durable snapshots")
				}
			})
		}
	}
}

func TestPVESnapshotCancelledAfterListingsCannotReconcile(t *testing.T) {
	f := newSnapshotRecoveryFixture(t, "qemu")
	f.poll(t, "site")
	before := f.points(t)
	f.coldState(true)
	for key := range f.client.replies {
		f.client.replies[key] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{{Name: "new", SnapTime: time.Now().Unix()}}}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Sizing runs after every successful listing, before durable ingestion.
	f.client.afterStorage = cancel
	f.m.pollGuestSnapshots(ctx, "site", f.client)
	f.wait(t)
	if ctx.Err() == nil {
		t.Fatal("control did not cancel the poll after the listings")
	}
	after := f.points(t)
	for id, point := range before {
		if !reflect.DeepEqual(point, after[id]) {
			t.Fatalf("cancelled poll reconciled away accepted history %s", id)
		}
	}
	if len(after) != 4 {
		t.Fatalf("accepted pre-cancellation listings should remain useful, points=%d", len(after))
	}
}

func TestPVESnapshotCompleteRecoveryKeepsOtherClassesAndUnpolledRows(t *testing.T) {
	f := newSnapshotRecoveryFixture(t, "lxc")
	f.poll(t, "site")
	at := time.Now().UTC().Add(-time.Hour)
	unrelated := []recovery.RecoveryPoint{
		recoveryIngestTestPoint("pve-backup:site-archive", "site", at),
		recoveryIngestTestPoint("pve-task:site-job", "site", at),
	}
	if err := f.store.UpsertPoints(context.Background(), unrelated); err != nil {
		t.Fatal(err)
	}
	before := f.points(t)
	// Even if every current target succeeds, a carried-forward row belongs to
	// a guest that was not read and cannot authorise instance-wide deletion.
	f.m.state.UpdateGuestsForInstance("site", nil, []models.Container{{ID: makeGuestID("site", "node-a", 101), Instance: "site", Node: "node-a", VMID: 101, Type: "lxc"}})
	f.client.replies["lxc/node-a/101"] = snapshotCollectionReply{}
	f.poll(t, "site")
	if !reflect.DeepEqual(before, f.points(t)) {
		t.Fatal("unpolled carried-forward guest permitted durable reconciliation")
	}
	// Restoring the same inventory and accepting new listings replaces old
	// snapshots without deleting independent archive or execution records.
	f.coldState(true)
	for key := range f.client.replies {
		f.client.replies[key] = snapshotCollectionReply{snapshots: []proxmox.Snapshot{{Name: "recovered", SnapTime: at.Unix()}}}
	}
	f.poll(t, "site")
	after := f.points(t)
	for _, point := range unrelated {
		if !reflect.DeepEqual(before[point.ID], after[point.ID]) {
			t.Fatalf("snapshot recovery changed another class: %s", point.ID)
		}
	}
	if len(after) != 4 || after["pve-snapshot:site-node-a-101-recovered"].ID == "" || after["pve-snapshot:site-node-b-202-recovered"].ID == "" {
		t.Fatalf("ordinary complete recovery failed to replace snapshot identities: %+v", after)
	}
}
