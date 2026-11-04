package unifiedresources

import (
	"fmt"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

// pbsCollisionState has two connections reusing node names, VMIDs and a guest
// name, the shape of issue 2681, with snapshots that each take a different path
// through the attribution rule.
func pbsCollisionState(now time.Time) *models.State {
	state := models.NewState()
	vm := func(instance string, vmid int, name string) models.VM {
		return models.VM{ID: fmt.Sprintf("%s:pve:%d", instance, vmid), VMID: vmid, Name: name, Node: "pve", Instance: instance, Type: "qemu", Status: "running", LastSeen: now}
	}
	state.UpdateVMsForInstance("alpha", []models.VM{vm("alpha", 100, "web"), vm("alpha", 110, "quiet-a"), vm("alpha", 120, "app"), vm("alpha", 130, "typed-vm"), vm("alpha", 200, "solo")})
	state.UpdateVMsForInstance("bravo", []models.VM{vm("bravo", 100, "db"), vm("bravo", 110, "quiet-b"), vm("bravo", 121, "app"), vm("bravo", 140, "viewed-b")})
	state.UpdateVMsForInstance("charlie", []models.VM{vm("charlie", 140, "viewed-c")})
	state.UpdateContainersForInstance("bravo", []models.Container{{ID: "bravo:pve:130", VMID: 130, Name: "typed-ct", Node: "pve", Instance: "bravo", Type: "lxc", Status: "running", LastSeen: now}})
	state.UpdatePBSInstances([]models.PBSInstance{{
		ID: "pbs-1", Name: "pbs-main", Host: "https://pbs:8007", Status: "online", LastSeen: now,
		Datastores: []models.PBSDatastore{{Name: "tank", Status: "online", Total: 100, Used: 10}},
	}})
	snapshot := func(backupType, vmid, comment string, age time.Duration) models.PBSBackup {
		when := now.Add(-age)
		return models.PBSBackup{ID: fmt.Sprintf("%s/%s/%d", backupType, vmid, when.Unix()), Instance: "pbs-main", Datastore: "tank", BackupType: backupType, VMID: vmid, Comment: comment, BackupTime: when}
	}
	state.UpdatePBSBackups("pbs-main", []models.PBSBackup{
		snapshot("vm", "100", "web", time.Hour),  // names alpha's guest
		snapshot("vm", "100", "db", 2*time.Hour), // names bravo's guest
		snapshot("vm", "110", "", time.Hour),     // nothing settles it
		snapshot("vm", "120", "", time.Hour),     // one guest has this VMID, another its name
		snapshot("vm", "121", "", time.Hour),     // the other guest with that name
		snapshot("vm", "130", "", time.Hour),     // one VM with this VMID
		snapshot("ct", "130", "", time.Hour),     // one container with this VMID
		snapshot("vm", "140", "", 3*time.Hour),   // settled by a storage view below
		snapshot("vm", "200", "", time.Hour),     // no collision
		snapshot("host", "200", "", time.Hour),   // a host backup, not VM 200
		snapshot("vm", "999", "", time.Hour),     // no such guest
	})
	state.UpdatePBSGuestConfirmationsForInstance("charlie", []models.PBSGuestConfirmation{{Storage: "pbs-c", BackupType: "vm", VMID: 140, Time: now.Add(-3 * time.Hour).Unix()}})
	state.SyncGuestBackupTimes()
	return state
}

func pbsFacets(t *testing.T, resources []Resource) (*PBSData, *StorageMeta) {
	t.Helper()
	var server *PBSData
	var datastore *StorageMeta
	for i := range resources {
		switch resources[i].Type {
		case ResourceTypePBS:
			server = resources[i].PBS
		case ResourceTypeStorage:
			datastore = resources[i].Storage
		}
	}
	if server == nil || datastore == nil {
		t.Fatalf("expected a PBS server and a datastore, got %d resources", len(resources))
	}
	return server, datastore
}

// The rollups name exactly the guests the guest backup badge credits.
func TestPBSRollupsNameTheGuestsOfTheAttributionRule(t *testing.T) {
	now := time.Now().UTC()
	state := pbsCollisionState(now)
	snapshot := state.GetSnapshot()

	var badged []string
	for _, vm := range snapshot.VMs {
		if !vm.LastBackup.IsZero() {
			badged = append(badged, vm.Name)
		}
	}
	for _, container := range snapshot.Containers {
		if !container.LastBackup.IsZero() {
			badged = append(badged, container.Name)
		}
	}
	sort.Strings(badged)
	if want := []string{"app", "app", "db", "solo", "typed-ct", "typed-vm", "viewed-c", "web"}; !slices.Equal(badged, want) {
		t.Fatalf("badged guests = %v, want %v", badged, want)
	}

	registry := NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	server, datastore := pbsFacets(t, registry.List())

	if server.ProtectedWorkloadCount != len(badged) {
		t.Fatalf("protected workloads = %d (%v), want %d", server.ProtectedWorkloadCount, server.ProtectedWorkloadNames, len(badged))
	}
	if datastore.ConsumerCount != len(badged) {
		t.Fatalf("datastore consumers = %d (%+v), want %d", datastore.ConsumerCount, datastore.TopConsumers, len(badged))
	}
	sawSolo := false
	for _, consumer := range datastore.TopConsumers {
		if !slices.Contains(badged, consumer.Name) {
			t.Errorf("datastore names %q, which no badge credits", consumer.Name)
		}
		if consumer.Name == "solo" {
			sawSolo = true
			if consumer.DiskCount != 1 {
				t.Errorf("solo counts %d snapshots; a host backup with its VMID is not one of them", consumer.DiskCount)
			}
		}
	}
	if !sawSolo {
		t.Errorf("expected solo among the top consumers, got %+v", datastore.TopConsumers)
	}
}

func TestPBSRollupsFollowAWithdrawnStorageView(t *testing.T) {
	now := time.Now().UTC()
	state := pbsCollisionState(now)
	state.UpdatePBSGuestConfirmationsForInstance("charlie", nil)

	registry := NewRegistry(nil)
	registry.IngestSnapshot(state.GetSnapshot())
	server, _ := pbsFacets(t, registry.List())
	if slices.Contains(server.ProtectedWorkloadNames, "viewed-c") || server.ProtectedWorkloadCount != 7 {
		t.Fatalf("expected the unsettled snapshot to protect nobody, got %d: %v", server.ProtectedWorkloadCount, server.ProtectedWorkloadNames)
	}
}

// A registry that read a snapshot answers for itself, so one with no backups left
// clears the rollups. A registry seeded from resources never saw backups, so the
// rollups it was given are the only answer it has.
func TestPBSRollupsKeepTheSeedOnlyWithoutABackupRead(t *testing.T) {
	now := time.Now().UTC()
	state := pbsCollisionState(now)
	adapter := NewMonitorAdapter(NewRegistry(nil))
	adapter.PopulateFromSnapshot(state.GetSnapshot())
	resources := adapter.GetAll()
	wantServer, wantDatastore := pbsFacets(t, resources)
	if wantServer.ProtectedWorkloadCount == 0 || wantDatastore.ConsumerCount == 0 {
		t.Fatalf("expected rollups from the snapshot, got %+v / %+v", wantServer, wantDatastore)
	}

	seeded := NewRegistry(nil)
	seeded.IngestResources(resources)
	server, datastore := pbsFacets(t, seeded.List())
	if server.ProtectedWorkloadCount != wantServer.ProtectedWorkloadCount || !slices.Equal(server.ProtectedWorkloadNames, wantServer.ProtectedWorkloadNames) || server.ProtectedWorkloadSummary != wantServer.ProtectedWorkloadSummary {
		t.Fatalf("seeded rollup = %+v, want %+v", server, wantServer)
	}
	if datastore.ConsumerCount != wantDatastore.ConsumerCount || len(datastore.TopConsumers) != len(wantDatastore.TopConsumers) {
		t.Fatalf("seeded consumers = %+v, want %+v", datastore, wantDatastore)
	}

	// The same registry, once it reads a snapshot, answers for itself.
	state.UpdatePBSBackups("pbs-main", nil)
	seeded.IngestSnapshot(state.GetSnapshot())
	server, datastore = pbsFacets(t, seeded.List())
	if server.ProtectedWorkloadCount != 0 || server.ProtectedWorkloadSummary != "" || datastore.ConsumerCount != 0 {
		t.Fatalf("a read with no backups left must clear the rollups, got %+v / %+v", server, datastore)
	}
}

// A guest an operator folded into another guest is that row now, so its snapshots
// count there rather than vanish with the folded row. The holder has no settled
// snapshot of its own, so both rollups can only have got this from the fold.
func TestPBSRollupsFollowAGuestFoldedIntoAnother(t *testing.T) {
	now := time.Now().UTC()
	state := pbsCollisionState(now)
	holderID := ProxmoxGuestCanonicalID(ResourceTypeVM, "alpha", 110)
	foldedID := ProxmoxGuestCanonicalID(ResourceTypeVM, "bravo", 100)
	store := NewMemoryStore()
	if err := store.AddLink(ResourceLink{ResourceA: holderID, ResourceB: foldedID, PrimaryID: holderID, CreatedAt: now}); err != nil {
		t.Fatalf("add link: %v", err)
	}
	registry := NewRegistry(store)
	registry.IngestSnapshot(state.GetSnapshot())

	if _, ok := registry.Get(foldedID); ok {
		t.Fatal("expected the link to fold bravo's guest into alpha's")
	}
	server, datastore := pbsFacets(t, registry.List())
	if server.ProtectedWorkloadCount != 8 || !slices.Contains(server.ProtectedWorkloadNames, "quiet-a") {
		t.Fatalf("protected workloads = %d %v, want 8 with quiet-a holding db's snapshot", server.ProtectedWorkloadCount, server.ProtectedWorkloadNames)
	}
	if datastore.ConsumerCount != 8 {
		t.Fatalf("datastore consumers = %d %+v, want 8", datastore.ConsumerCount, datastore.TopConsumers)
	}
}
