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

func TestRecoveryIngestKeepsOverlappingSnapshotReadScopesInOrder(t *testing.T) {
	m := &Monitor{recoveryIngestRunning: true}
	guestA := recoverystore.PVESnapshotGuest{Node: "node-a", Kind: "qemu", VMID: 101}
	guestB := recoverystore.PVESnapshotGuest{Node: "node-b", Kind: "qemu", VMID: 202}
	enqueue := func(guests ...recoverystore.PVESnapshotGuest) {
		targets := make(map[recoverystore.PVESnapshotGuest]struct{}, len(guests))
		for _, guest := range guests {
			targets[guest] = struct{}{}
		}
		m.enqueueRecoveryIngest(recoveryIngestBatch{reconcile: &recoveryReconcileScope{
			provider: string(recovery.ProviderProxmoxPVE), idPrefix: "pve-snapshot:", instance: "site",
			snapshotGuests: targets,
		}})
	}
	enqueue(guestA, guestB)
	enqueue(guestB)
	enqueue(guestA)
	want := []map[recoverystore.PVESnapshotGuest]struct{}{
		{guestA: {}, guestB: {}}, {guestB: {}}, {guestA: {}},
	}
	if len(m.recoveryIngestPending) != len(want) {
		t.Fatalf("expected three pending overlapping read scopes, got %d", len(m.recoveryIngestPending))
	}
	for i, batch := range m.recoveryIngestPending {
		if !reflect.DeepEqual(want[i], batch.reconcile.snapshotGuests) {
			t.Fatalf("read scope %d was reordered or replaced: %v", i, batch.reconcile.snapshotGuests)
		}
	}
}

func TestRecoveryIngestDisjointSnapshotBatchDoesNotBlockPBSCoalescing(t *testing.T) {
	m := &Monitor{recoveryIngestRunning: true}
	pbsScope := recoveryReconcileScope{
		provider: string(recovery.ProviderProxmoxPBS),
		idPrefix: "pbs-backup:",
		instance: "pbs-main",
	}
	snapshotScope := recoveryReconcileScope{
		provider: string(recovery.ProviderProxmoxPVE),
		idPrefix: "pve-snapshot:",
		instance: "pve-1",
		snapshotGuests: map[recoverystore.PVESnapshotGuest]struct{}{
			{Node: "pve1", Kind: "qemu", VMID: 101}: {},
		},
	}

	m.enqueueRecoveryIngest(recoveryIngestBatch{
		points:    []recovery.RecoveryPoint{{ID: "pbs-backup:old"}},
		reconcile: &pbsScope,
	})
	m.enqueueRecoveryIngest(recoveryIngestBatch{
		points:    []recovery.RecoveryPoint{{ID: "pve-snapshot:1"}},
		reconcile: &snapshotScope,
	})
	m.enqueueRecoveryIngest(recoveryIngestBatch{
		points:    []recovery.RecoveryPoint{{ID: "pbs-backup:newest"}},
		reconcile: &pbsScope,
	})

	if got := len(m.recoveryIngestPending); got != 2 {
		t.Fatalf("pending batches = %d, want 2 (coalesced PBS batch + disjoint PVE snapshot batch)", got)
	}
	if got := m.recoveryIngestPending[0].points[0].ID; got != "pbs-backup:newest" {
		t.Fatalf("pending PBS batch point = %q, want pbs-backup:newest", got)
	}
	if got := m.recoveryIngestPending[1].points[0].ID; got != "pve-snapshot:1" {
		t.Fatalf("pending snapshot batch point = %q, want pve-snapshot:1", got)
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

// The first PBS polls after a restart run before guests have loaded. Their empty
// answer must not clear a guest a backup could belong to (#2681).
func TestRecoveryIngestKeepsUnsettledGuestsUntilAPollCanEvaluateThem(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		"Home",
		recovery.OutcomeUnknown,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		now.Add(-time.Minute),
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation(): %v", err)
	}
	taskTime := now.Add(-time.Hour)
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		observations: []recovery.ProtectionProviderObservation{observation},
		points: []recovery.RecoveryPoint{{
			ID: "pve-task:web1", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeLocal,
			Outcome: recovery.OutcomeSuccess, CompletedAt: &taskTime, SubjectResourceID: "resource:web1", ProviderScope: "Home",
		}},
	})
	state := func() recovery.ProtectionState {
		t.Helper()
		postures, _, err := store.ListProtectionPostures(ctx, recovery.ProtectionPostureQuery{SubjectResourceIDs: []string{"resource:web1"}, Page: 1, Limit: 1})
		if err != nil || len(postures) != 1 {
			t.Fatalf("ListProtectionPostures() = %v, %v", postures, err)
		}
		return postures[0].State
	}
	pbs := func() *recoveryReconcileScope {
		return &recoveryReconcileScope{provider: string(recovery.ProviderProxmoxPBS), idPrefix: "pbs-backup:", instance: "shared-pbs"}
	}
	if got := state(); got != recovery.ProtectionStateUnprotected {
		t.Fatalf("no snapshot yet: state = %s, want unprotected", got)
	}

	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{reconcile: pbs(), evaluatedSubjects: []string{"resource:web1"}, unsettledSubjects: []string{"resource:web1"}})
	if got := state(); got != recovery.ProtectionStateUnknown {
		t.Fatalf("unsettled snapshot: state = %s, want unknown", got)
	}

	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{reconcile: pbs()})
	if got := state(); got != recovery.ProtectionStateUnknown {
		t.Fatalf("poll with no guests known: state = %s, want the last answer, unknown", got)
	}

	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{reconcile: pbs(), evaluatedSubjects: []string{"resource:other"}})
	if got := state(); got != recovery.ProtectionStateUnknown {
		t.Fatalf("poll before this guest loaded: state = %s, want the last answer, unknown", got)
	}

	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{reconcile: pbs(), evaluatedSubjects: []string{"resource:web1"}})
	if got := state(); got != recovery.ProtectionStateUnprotected {
		t.Fatalf("settled: state = %s, want unprotected", got)
	}
}

// A removed connection never polls again, so any later ingest clears its answer.
func TestRecoveryIngestForgetsUnsettledGuestsOfARemovedConnection(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	m.config = &config.Config{PBSInstances: []config.PBSInstance{{Name: "shared-pbs"}}}
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	observation, err := recovery.NewProtectionProviderObservation(
		recovery.ProviderProxmoxPVE,
		"pve-backup-enumeration",
		"Home",
		recovery.OutcomeUnknown,
		recovery.ProtectionHistoryComplete,
		operationaltrust.EvidencePermissionsSufficient,
		false,
		now.Add(-time.Minute),
		now,
		nil,
	)
	if err != nil {
		t.Fatalf("NewProtectionProviderObservation(): %v", err)
	}
	taskTime := now.Add(-time.Hour)
	points := []recovery.RecoveryPoint{{
		ID: "pve-task:web1", Provider: recovery.ProviderProxmoxPVE, Kind: recovery.KindOther, Mode: recovery.ModeLocal,
		Outcome: recovery.OutcomeSuccess, CompletedAt: &taskTime, SubjectResourceID: "resource:web1", ProviderScope: "Home",
	}}
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{observations: []recovery.ProtectionProviderObservation{observation}, points: points})
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		reconcile:         &recoveryReconcileScope{provider: string(recovery.ProviderProxmoxPBS), idPrefix: "pbs-backup:", instance: "shared-pbs"},
		evaluatedSubjects: []string{"resource:web1"},
		unsettledSubjects: []string{"resource:web1"},
	})
	state := func() recovery.ProtectionState {
		t.Helper()
		postures, _, err := store.ListProtectionPostures(ctx, recovery.ProtectionPostureQuery{SubjectResourceIDs: []string{"resource:web1"}, Page: 1, Limit: 1})
		if err != nil || len(postures) != 1 {
			t.Fatalf("ListProtectionPostures() = %v, %v", postures, err)
		}
		return postures[0].State
	}
	if got := state(); got != recovery.ProtectionStateUnknown {
		t.Fatalf("connection configured: state = %s, want unknown", got)
	}

	// The next ingest is only a PVE poll's points.
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{points: points})
	if got := state(); got != recovery.ProtectionStateUnknown {
		t.Fatalf("connection still configured after another ingest: state = %s, want unknown", got)
	}

	m.config = &config.Config{}
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{points: points})
	if got := state(); got != recovery.ProtectionStateUnprotected {
		t.Fatalf("connection removed: state = %s, want unprotected", got)
	}
}

func TestRecoveryIngestSkipsUnchangedCompleteInventoryAndAppliesDeltas(t *testing.T) {
	m, manager := recoveryIngestTestMonitor(t)
	store, err := manager.StoreForOrg("default")
	if err != nil {
		t.Fatalf("StoreForOrg(default): %v", err)
	}
	ctx := context.Background()

	scope := &recoveryReconcileScope{
		provider: string(recovery.ProviderProxmoxPBS),
		idPrefix: "pbs-backup:",
		instance: "pbs-main",
	}
	baseBackup := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	t1 := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Millisecond)
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)
	t4 := t3.Add(time.Minute)
	t5 := t4.Add(time.Minute)

	boolPtr := func(v bool) *bool { return &v }
	makePoint := func(id string, verified bool, outcome recovery.Outcome, ingestedAt time.Time) recovery.RecoveryPoint {
		t.Helper()
		p := recovery.RecoveryPoint{
			ID:                   id,
			Provider:             recovery.ProviderProxmoxPBS,
			ProviderScope:        "pbs-main",
			Kind:                 recovery.KindBackup,
			Mode:                 recovery.ModeRemote,
			Outcome:              outcome,
			StartedAt:            &baseBackup,
			CompletedAt:          &baseBackup,
			Verified:             boolPtr(verified),
			SubjectResourceID:    "proxmox:vm:pve:100",
			RepositoryResourceID: "pbs:datastore:pbs-main:store",
			RepositoryRef:        &recovery.ExternalRef{Type: "pbs-datastore", ID: "pbs-main:store", Namespace: "pbs-main"},
			Details:              map[string]any{"vmid": "100"},
		}
		ev, err := recovery.NewRecoveryPointEvidence(p, "pbs-backup-inventory", ingestedAt)
		if err != nil {
			t.Fatalf("NewRecoveryPointEvidence: %v", err)
		}
		p.Evidence = ev
		return p
	}
	makeObs := func(completeness recovery.ProtectionHistoryCompleteness, perms operationaltrust.EvidencePermissions, at time.Time) recovery.ProtectionProviderObservation {
		t.Helper()
		obs, err := recovery.NewProtectionProviderObservation(
			recovery.ProviderProxmoxPBS, "pbs-backup-inventory", "pbs-main",
			recovery.OutcomeSuccess, completeness, perms, true, at, at, nil,
		)
		if err != nil {
			t.Fatalf("NewProtectionProviderObservation: %v", err)
		}
		return obs
	}
	pointIngestedAt := func(id string) time.Time {
		t.Helper()
		pts, _, err := store.ListPoints(ctx, recovery.ListPointsOptions{Provider: recovery.ProviderProxmoxPBS, Limit: 20})
		if err != nil {
			t.Fatalf("ListPoints: %v", err)
		}
		for _, p := range pts {
			if p.ID == id && p.Evidence != nil {
				return p.Evidence.IngestedAt
			}
		}
		return time.Time{}
	}

	// Poll 1: initial complete healthy inventory persists both points.
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		observations: []recovery.ProtectionProviderObservation{
			makeObs(recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient, t1),
		},
		points: []recovery.RecoveryPoint{
			makePoint("pbs-backup:pbs-main:store:vm:100:1", true, recovery.OutcomeSuccess, t1),
			makePoint("pbs-backup:pbs-main:store:vm:100:2", false, recovery.OutcomeSuccess, t1),
		},
		reconcile: scope,
	})
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:1"); !got.Equal(t1) {
		t.Fatalf("poll 1 point 1 ingestedAt = %v, want %v", got, t1)
	}

	// Poll 2: semantically identical complete poll at t2 must NOT rewrite unchanged rows.
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		observations: []recovery.ProtectionProviderObservation{
			makeObs(recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient, t2),
		},
		points: []recovery.RecoveryPoint{
			makePoint("pbs-backup:pbs-main:store:vm:100:1", true, recovery.OutcomeSuccess, t2),
			makePoint("pbs-backup:pbs-main:store:vm:100:2", false, recovery.OutcomeSuccess, t2),
		},
		reconcile: scope,
	})
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:1"); !got.Equal(t1) {
		t.Fatalf("unchanged poll 2 rewrote point 1: ingestedAt = %v, want preserved %v", got, t1)
	}
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:2"); !got.Equal(t1) {
		t.Fatalf("unchanged poll 2 rewrote point 2: ingestedAt = %v, want preserved %v", got, t1)
	}

	// Poll 3: only point 2 changes verification false -> true; point 1 stays untouched while point 2 updates.
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		observations: []recovery.ProtectionProviderObservation{
			makeObs(recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient, t3),
		},
		points: []recovery.RecoveryPoint{
			makePoint("pbs-backup:pbs-main:store:vm:100:1", true, recovery.OutcomeSuccess, t3),
			makePoint("pbs-backup:pbs-main:store:vm:100:2", true, recovery.OutcomeSuccess, t3),
		},
		reconcile: scope,
	})
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:1"); !got.Equal(t1) {
		t.Fatalf("delta poll 3 needlessly rewrote unchanged point 1: ingestedAt = %v, want %v", got, t1)
	}
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:2"); !got.Equal(t3) {
		t.Fatalf("delta poll 3 did not update changed point 2: ingestedAt = %v, want %v", got, t3)
	}

	// Poll 4: point 2 is pruned on PBS -> reconcile deletes it while keeping point 1.
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		observations: []recovery.ProtectionProviderObservation{
			makeObs(recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient, t4),
		},
		points: []recovery.RecoveryPoint{
			makePoint("pbs-backup:pbs-main:store:vm:100:1", true, recovery.OutcomeSuccess, t4),
		},
		reconcile: scope,
	})
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:2"); !got.IsZero() {
		t.Fatalf("removed point 2 still present after poll 4 (ingestedAt=%v)", got)
	}

	// Poll 5: partial observation bypasses steady-state skip so re-read point 1 renews its IngestedAt for pointConfirmedByObservation.
	m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
		observations: []recovery.ProtectionProviderObservation{
			makeObs(recovery.ProtectionHistoryPartial, operationaltrust.EvidencePermissionsPartial, t5),
		},
		points: []recovery.RecoveryPoint{
			makePoint("pbs-backup:pbs-main:store:vm:100:1", true, recovery.OutcomeSuccess, t5),
		},
	})
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:1"); !got.Equal(t5) {
		t.Fatalf("partial poll 5 did not renew re-read point 1 evidence: ingestedAt = %v, want %v", got, t5)
	}

	// Poll 6 & 7: OutcomeRunning points always renew IngestedAt even across identical complete polls.
	t6 := t5.Add(time.Minute)
	t7 := t6.Add(time.Minute)
	for _, at := range []time.Time{t6, t7} {
		m.ingestRecoveryPointsBestEffort(ctx, recoveryIngestBatch{
			observations: []recovery.ProtectionProviderObservation{
				makeObs(recovery.ProtectionHistoryComplete, operationaltrust.EvidencePermissionsSufficient, at),
			},
			points: []recovery.RecoveryPoint{
				makePoint("pbs-backup:pbs-main:store:vm:100:1", true, recovery.OutcomeSuccess, at),
				makePoint("pbs-backup:pbs-main:store:vm:100:running", false, recovery.OutcomeRunning, at),
			},
			reconcile: scope,
		})
	}
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:1"); !got.Equal(t6) {
		t.Fatalf("completed point 1 should not be rewritten on poll 7: ingestedAt = %v, want %v", got, t6)
	}
	if got := pointIngestedAt("pbs-backup:pbs-main:store:vm:100:running"); !got.Equal(t7) {
		t.Fatalf("running point must renew IngestedAt on every poll: ingestedAt = %v, want %v", got, t7)
	}
}
