package tools

import (
	"context"
	"encoding/json"
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

// Patrol passed guests' names as resource_id ("gitea", "nextcloud"). Backups
// record their subject by canonical ID, so each list came back empty and
// Patrol filed "No backup records exist" for three guests that had backups.
// A name or VMID now lists that guest's backups, and a reference that names
// nothing, or several resources, is an error rather than an empty list.
func TestPulseStorageBackupsAcceptAGuestsName(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(models.StateSnapshot{
		Containers: []models.Container{
			{ID: "pve1-node1-111", VMID: 111, Name: "nextcloud", Node: "node1", Instance: "pve1", Status: "running", Type: "lxc", LastSeen: now},
			{ID: "pve1-node1-112", VMID: 112, Name: "gitea", Node: "node1", Instance: "pve1", Status: "running", Type: "lxc", LastSeen: now},
		},
		Storage: []models.Storage{{ID: "pve1-node1-local", Name: "local", Node: "node1", Instance: "pve1", Type: "dir", Status: "available",
			Content: "backup", Enabled: true, Active: true, Total: 1000, Used: 500, LastSeen: now}},
		VMs: []models.VM{
			{ID: "pve1-node1-120", VMID: 120, Name: "web", Node: "node1", Instance: "pve1", Status: "running", Type: "qemu", LastSeen: now},
			{ID: "pve2-node9-130", VMID: 130, Name: "web", Node: "node9", Instance: "pve2", Status: "running", Type: "qemu", LastSeen: now},
		},
	})
	ids := map[string]string{}
	for _, r := range registry.List() {
		if r.Type == unifiedresources.ResourceTypeSystemContainer {
			ids[r.Name] = r.ID
		}
	}
	require.NotEmpty(t, ids["nextcloud"])
	require.NotEmpty(t, ids["gitea"])

	backup := func(id string, vmid int) models.StorageBackup {
		return models.StorageBackup{ID: id, Storage: "local", Node: "node1", StorageNode: "node1", Instance: "pve1",
			Type: "lxc", VMID: vmid, Time: now.Add(-time.Hour), Size: 100}
	}
	// Guest 199 is gone from Pulse; its backup keeps the old subject ID.
	orphan := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "pve1", 199)
	points := proxmoxmapper.FromPVEStorageBackups([]models.StorageBackup{backup("nc-1", 111), backup("nc-2", 111), backup("old-1", 199)},
		map[string]proxmoxmapper.GuestInfo{
			"pve1|node1|111": {ResourceID: ids["nextcloud"], ResourceType: unifiedresources.ResourceTypeSystemContainer, Name: "nextcloud"},
			"pve1|node1|199": {ResourceID: orphan, ResourceType: unifiedresources.ResourceTypeSystemContainer, Name: "old"},
		})

	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.UpsertPoints(ctx, points))
	executor := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider:  NewRecoveryPointsToolAdapter(manager, "default"),
		UnifiedResourceProvider: unifiedresources.NewUnifiedAIAdapter(registry),
	})

	type listed struct {
		Note   string                   `json:"note"`
		Points []recovery.RecoveryPoint `json:"points"`
	}
	list := func(ref string) (listed, CallToolResult) {
		t.Helper()
		result, err := executor.executeListBackups(ctx, map[string]interface{}{"resource_id": ref})
		require.NoError(t, err)
		var got listed
		if !result.IsError {
			require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &got))
		}
		return got, result
	}

	byID, _ := list(ids["nextcloud"])
	require.Len(t, byID.Points, 2)
	require.Empty(t, byID.Note)
	for _, ref := range []string{"nextcloud", "111"} {
		got, result := list(ref)
		require.False(t, result.IsError, "%s: %v", ref, result.Content)
		require.Len(t, got.Points, 2, ref)
		require.Contains(t, got.Note, ids["nextcloud"], ref)
	}

	// With a storage too, the name still resolves.
	var storageID string
	for _, r := range registry.List() {
		if r.Type == unifiedresources.ResourceTypeStorage {
			storageID = r.ID
		}
	}
	require.NotEmpty(t, storageID)
	result, err := executor.executeListBackups(ctx, map[string]interface{}{"resource_id": "nextcloud", "storage_id": storageID})
	require.NoError(t, err)
	require.False(t, result.IsError, result.Content)
	var onStorage listed
	require.NoError(t, json.Unmarshal([]byte(result.Content[0].Text), &onStorage))
	require.Len(t, onStorage.Points, 2)

	// A guest that has no backups is an empty list, not an error.
	none, result := list("gitea")
	require.False(t, result.IsError, result.Content)
	require.Empty(t, none.Points)

	// A subject Pulse no longer monitors is still listed by its ID.
	old, result := list(orphan)
	require.False(t, result.IsError, result.Content)
	require.Len(t, old.Points, 1)

	_, result = list("web")
	require.True(t, result.IsError, "two VMs are named web")
	require.Contains(t, result.Content[0].Text, "names 2 resources")

	_, result = list("missing")
	require.True(t, result.IsError, "a name nothing has must not read as no backups")
}
