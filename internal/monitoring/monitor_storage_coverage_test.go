package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
	"github.com/stretchr/testify/require"
)

// Mock for PVE Client to simulate storage failures/successes
type mockPVEClientForStorage struct {
	mockPVEClientExtra // Embed existing mock

	ShouldFailStorageQuery bool
	ShouldTimeoutStorage   bool
	StorageToFail          map[string]bool // storage names that fail content retrieval
	Storages               []proxmox.Storage
	StorageQueries         []string
	ContentQueries         []string
}

func (m *mockPVEClientForStorage) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	m.StorageQueries = append(m.StorageQueries, node)
	if m.ShouldFailStorageQuery {
		return nil, fmt.Errorf("failed to get storage")
	}
	if m.ShouldTimeoutStorage {
		return nil, fmt.Errorf("timeout doing request")
	}
	return m.Storages, nil
}

func (m *mockPVEClientForStorage) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	m.ContentQueries = append(m.ContentQueries, node+"/"+storage)
	if m.StorageToFail != nil && m.StorageToFail[storage] {
		return nil, fmt.Errorf("failed to get content")
	}

	// The real endpoint labels backup archives explicitly; live disks from the
	// same enumeration must never count as successful backups.
	return []proxmox.StorageContent{
		{Volid: storage + ":backup/vzdump-qemu-100-2026_03_11-10_00_00.vma.zst", Content: "backup", VMID: 100, Size: 1024, CTime: time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC).Unix()},
		{Volid: storage + ":vm-100-disk-0", Content: "images", VMID: 100, Size: 4096},
	}, nil
}

func TestMonitor_PollStorageBackupsWithNodes_Coverage(t *testing.T) {
	// Setup
	m := &Monitor{
		state: models.NewState(),
	}

	// Setup State with VMs to test guest lookup logic.
	vms := []models.VM{
		{VMID: 100, Node: "node1", Instance: "pve1", Name: "vm100"},
	}
	m.state.UpdateVMsForInstance("pve1", vms)
	held := []models.StorageBackup{
		{ID: "offline", Instance: "pve1", Storage: "local", StorageNode: "node2", Node: "node2", VMID: 200, Volid: "local:backup/offline.vma.zst", Size: 2048},
		{ID: "unreadable", Instance: "pve1", Storage: "broken", StorageNode: "node1", Node: "node1", VMID: 300, Volid: "broken:backup/held.vma.zst", Size: 3072},
	}
	m.state.UpdateStorageBackupsForInstance("pve1", held)
	foreign := models.StorageBackup{ID: "foreign", Instance: "pve-other", Storage: "local", StorageNode: "node1", Node: "node1", VMID: 100, Size: 8192}
	m.state.UpdateStorageBackupsForInstance("pve-other", []models.StorageBackup{foreign})

	nodes := []proxmox.Node{
		{Node: "node1", Status: "online"},
		{Node: "node2", Status: "offline"}, // offline node logic
	}

	nodeEffectiveStatus := map[string]string{
		"node1": "online",
		"node2": "offline",
	}

	storages := []proxmox.Storage{
		{Storage: "local", Content: "backup", Type: "dir", Enabled: 1, Active: 1, Shared: 0},
		{Storage: "shared", Content: "backup", Type: "nfs", Enabled: 1, Active: 1, Shared: 1},
		{Storage: "broken", Content: "backup", Type: "dir", Enabled: 1, Active: 1, Shared: 0},
	}

	client := &mockPVEClientForStorage{
		Storages:      storages,
		StorageToFail: map[string]bool{"broken": true},
	}

	// EXECUTE
	ctx := context.Background()
	m.pollStorageBackupsWithNodes(ctx, "pve1", client, nodes, nodeEffectiveStatus)

	backups := m.state.GetPVEBackups().StorageBackups
	require.Len(t, backups, 5, "two readable archives plus offline, unreadable and foreign history must remain")
	for _, previous := range append(held, foreign) {
		require.Contains(t, backups, previous, "partial collection must preserve the exact owned artifact")
	}
	var freshStores []string
	for _, backup := range backups {
		if backup.ID == "offline" || backup.ID == "unreadable" || backup.ID == "foreign" {
			continue
		}
		freshStores = append(freshStores, backup.Storage)
		require.Equal(t, "pve1", backup.Instance)
		require.Equal(t, "node1", backup.Node)
		require.Equal(t, 100, backup.VMID)
		require.Equal(t, "qemu", backup.Type)
		require.Equal(t, int64(1024), backup.Size)
		if backup.Storage == "local" {
			require.Equal(t, "node1", backup.StorageNode)
		} else {
			require.Empty(t, backup.StorageNode, "shared storage identity must not acquire a local node")
		}
	}
	require.ElementsMatch(t, []string{"local", "shared"}, freshStores, "only typed backup archives may be admitted")
	require.Equal(t, []string{"node1"}, client.StorageQueries, "offline nodes must not be queried")
	require.Equal(t, []string{"node1/local", "node1/shared", "node1/broken"}, client.ContentQueries)

	// A timeout is not empty history: retry once, then retain all artifacts.
	client.ShouldTimeoutStorage = true
	m.pollStorageBackupsWithNodes(ctx, "pve1", client, nodes, nodeEffectiveStatus)
	require.ElementsMatch(t, backups, m.state.GetPVEBackups().StorageBackups)
	require.Equal(t, []string{"node1", "node1", "node1"}, client.StorageQueries, "timeout must retry once without probing the offline peer")
	require.Len(t, client.ContentQueries, 3, "failed storage discovery must not probe guessed content paths")
}
