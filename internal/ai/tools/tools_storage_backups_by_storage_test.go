package tools

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A full storage's check asks pulse_storage for the backups kept there. The
// backups list filtered only by subject, so a storage's ID matched nothing and
// storage_id was ignored: one model saw no backups, another saw every backup
// on every storage. Both now get the backups kept on that storage.
func TestPulseStorageListsTheBackupsKeptOnAStorage(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	at := now.Add(-time.Hour)

	storage := func(id, name, node string, shared bool) models.Storage {
		return models.Storage{ID: id, Name: name, Node: node, Instance: "pve1", Type: "dir", Status: "available",
			Content: "backup,iso", Shared: shared, Enabled: true, Active: true, Total: 1000, Used: 900, LastSeen: now}
	}
	// A clustered connection's storage carries the cluster's name; its
	// backups carry the connection's name.
	clustered := storage("homelab-c1-local", "local", "c1", false)
	clustered.Instance = "homelab"
	pbsBacked := storage("pve1-cluster-pbs-store", "pbs-store", "cluster", true)
	pbsBacked.Type = "pbs"
	snapshot := models.StateSnapshot{
		Nodes: []models.Node{{ID: "homelab-c1", Name: "c1", Instance: "homelab-conn", ClusterName: "homelab",
			IsClusterMember: true, Status: "online", Type: "node", LastSeen: now}},
		Storage: []models.Storage{
			clustered,
			pbsBacked,
			storage("pve1-node1-local", "local", "node1", false),
			storage("pve1-node2-local", "local", "node2", false),
			storage("pve1-cluster-nfs", "nfs", "cluster", true),
		},
		PBSInstances: []models.PBSInstance{
			{ID: "pbs-pbs1", Name: "pbs1", Host: "https://pbs1.lan:8007", Status: "online", LastSeen: now,
				Datastores: []models.PBSDatastore{{Name: "store1", Total: 1000, Used: 500, Status: "available"}}},
			{ID: "pbs-pbs2", Name: "pbs2", Host: "https://pbs2.lan:8007", Status: "online", LastSeen: now,
				Datastores: []models.PBSDatastore{{Name: "store1", Total: 1000, Used: 500, Status: "available"}}},
		},
	}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	ids := map[string]string{}
	for _, r := range registry.List() {
		if r.Type != unifiedresources.ResourceTypeStorage {
			continue
		}
		key := r.Name
		if r.Proxmox != nil {
			key += "@" + r.Proxmox.NodeName
		} else {
			key += "@" + r.ParentName
		}
		ids[key] = r.ID
	}
	for _, key := range []string{"local@node1", "local@node2", "local@c1", "nfs@cluster", "store1@pbs1", "store1@pbs2"} {
		require.NotEmpty(t, ids[key], "no storage resource %s in %v", key, ids)
	}

	guest := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "pve1", 101)
	archive := func(id, storage, node, storageNode string, vmid int) models.StorageBackup {
		return models.StorageBackup{ID: id, Storage: storage, Node: node, StorageNode: storageNode, Instance: "pve1",
			Type: "lxc", VMID: vmid, Time: at, Size: 100}
	}
	points := proxmoxmapper.FromPVEStorageBackups([]models.StorageBackup{
		archive("n1-101", "local", "node1", "node1", 101),
		// Guest 102 migrated to node2; its old archive is still on node1's disk.
		archive("n1-102-old", "local", "node2", "node1", 102),
		archive("n2-102", "local", "node2", "node2", 102),
		archive("nfs-101", "nfs", "node1", "", 101),
		{ID: "c1-200", Storage: "local", Node: "c1", StorageNode: "c1", Instance: "homelab-conn", Type: "lxc", VMID: 200, Time: at, Size: 100},
	}, map[string]proxmoxmapper.GuestInfo{
		"pve1|node1|101": {ResourceID: guest, ResourceType: unifiedresources.ResourceTypeSystemContainer},
	})
	points = append(points, proxmoxmapper.FromPBSBackups([]models.PBSBackup{
		{ID: "pbs1-101", VMID: "101", BackupType: "ct", Instance: "pbs1", Datastore: "store1", BackupTime: at, Size: 400},
		{ID: "pbs2-101", VMID: "101", BackupType: "ct", Instance: "pbs2", Datastore: "store1", BackupTime: at, Size: 400},
	}, nil)...)

	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.UpsertPoints(ctx, points))
	executor := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider:  NewRecoveryPointsToolAdapter(manager, "default"),
		UnifiedResourceProvider: unifiedresources.NewUnifiedAIAdapter(registry),
	})

	list := func(args map[string]interface{}) []string {
		t.Helper()
		result, err := executor.executeListBackups(ctx, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Content)
		var response struct {
			Points []recovery.RecoveryPoint `json:"points"`
		}
		require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &response))
		var got []string
		for _, p := range response.Points {
			got = append(got, p.ID)
		}
		sort.Strings(got)
		return got
	}

	node1Local := []string{"pve-backup:n1-101", "pve-backup:n1-102-old"}
	require.Equal(t, node1Local, list(map[string]interface{}{"storage_id": ids["local@node1"]}))
	require.Equal(t, node1Local, list(map[string]interface{}{"resource_id": ids["local@node1"]}),
		"a storage's own ID passed as resource_id lists the backups kept on it")
	require.Equal(t, []string{"pve-backup:n2-102"}, list(map[string]interface{}{"storage_id": ids["local@node2"]}))
	require.Equal(t, []string{"pve-backup:c1-200", "pve-backup:n1-101", "pve-backup:n1-102-old", "pve-backup:n2-102"},
		list(map[string]interface{}{"storage_id": "local"}), "a name lists that storage on every node")
	require.Equal(t, []string{"pve-backup:nfs-101"}, list(map[string]interface{}{"storage_id": ids["nfs@cluster"]}))
	require.Equal(t, []string{"pve-backup:c1-200"}, list(map[string]interface{}{"storage_id": ids["local@c1"]}),
		"a clustered storage lists the backups its connection recorded")
	require.Equal(t, []string{"pbs-backup:pbs1-101"}, list(map[string]interface{}{"storage_id": ids["store1@pbs1"]}))
	require.Equal(t, []string{"pve-backup:n1-101"},
		list(map[string]interface{}{"storage_id": ids["local@node1"], "resource_id": guest}),
		"storage and guest filters combine")
	require.Equal(t, []string{"pve-backup:n1-101", "pve-backup:nfs-101"}, list(map[string]interface{}{"resource_id": guest}),
		"a guest's ID still lists that guest's backups")

	pbsResult, err := executor.executeListBackups(ctx, map[string]interface{}{"storage_id": "pbs-store"})
	require.NoError(t, err)
	require.False(t, pbsResult.IsError, pbsResult.Content)
	require.Contains(t, pbsResult.Content[0].Text, "listed on the PBS datastore",
		"a PBS-backed Proxmox storage says where its backups are listed")

	// Two clusters with one name: storage keeps each connection's name, so
	// neither connection borrows the other's backups.
	twins := unifiedresources.NewRegistry(nil)
	twinStorage := storage("home-n1-local", "local", "n1", false)
	twinStorage.Instance = "home"
	twins.IngestSnapshot(models.StateSnapshot{
		Nodes: []models.Node{
			{ID: "home-n1", Name: "n1", Instance: "home", ClusterName: "home", IsClusterMember: true, Status: "online", Type: "node", LastSeen: now},
			{ID: "remote-n1", Name: "n1", Instance: "remote", ClusterName: "home", IsClusterMember: true, Status: "online", Type: "node", LastSeen: now},
		},
		Storage: []models.Storage{twinStorage},
	})
	twinExecutor := NewPulseToolExecutor(ExecutorConfig{UnifiedResourceProvider: unifiedresources.NewUnifiedAIAdapter(twins)})
	var twinStorageResources []unifiedresources.Resource
	for _, r := range twins.List() {
		if r.Type == unifiedresources.ResourceTypeStorage {
			twinStorageResources = append(twinStorageResources, r)
		}
	}
	require.Equal(t, []recovery.RepositoryFilter{{Type: "proxmox-storage", Namespace: "home", Name: "local", Class: "n1"}},
		twinExecutor.backupRepositoriesForStorage(twinStorageResources))

	result, err := executor.executeListBackups(ctx, map[string]interface{}{"storage_id": "missing"})
	require.NoError(t, err)
	require.True(t, result.IsError, "an unknown storage_id must not list every backup")
}
