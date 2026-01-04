package monitoring

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// Mock for PVE Client to simulate storage failures/successes
type mockPVEClientForStorage struct {
	mockPVEClientExtra // Embed existing mock

	ShouldFailStorageQuery bool
	ShouldTimeoutStorage   bool
	StorageToFail          map[string]bool // storage names that fail content retrieval
	Storages               []proxmox.Storage
}

func (m *mockPVEClientForStorage) GetStorage(ctx context.Context, node string) ([]proxmox.Storage, error) {
	if m.ShouldFailStorageQuery {
		return nil, fmt.Errorf("failed to get storage")
	}
	if m.ShouldTimeoutStorage {
		return nil, fmt.Errorf("timeout doing request")
	}
	return m.Storages, nil
}

func (m *mockPVEClientForStorage) GetStorageContent(ctx context.Context, node, storage string) ([]proxmox.StorageContent, error) {
	if m.StorageToFail != nil && m.StorageToFail[storage] {
		return nil, fmt.Errorf("failed to get content")
	}

	// Return some dummy content
	return []proxmox.StorageContent{
		{Volid: fmt.Sprintf("backup/vzdump-qemu-100-%s.vma.zst", time.Now().Format("2006_01_02-15_04_05")), Size: 1024, CTime: time.Now().Unix()},
	}, nil
}

func TestMonitor_PollStorageBackupsWithNodes_Coverage(t *testing.T) {
	// Setup
	m := &Monitor{
		state: models.NewState(),
	}

	// Setup State with VMs to test guest lookup logic
	vms := []models.VM{
		{VMID: 100, Node: "node1", Instance: "pve1", Name: "vm100"},
	}
	m.state.UpdateVMsForInstance("pve1", vms)

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

	// Verify State
	snapshot := m.state.GetSnapshot()
	if len(snapshot.PVEBackups.StorageBackups) == 0 {
		t.Error("Expected backups to be found")
	}

	// Check offline node preservation logic
	// If a storage was previously known for 'node2' (offline), it should be preserved if not shared.
	// But we didn't seed initial state with old backups for node2.

	// Test Timeout Logic
	client.ShouldTimeoutStorage = true
	m.pollStorageBackupsWithNodes(ctx, "pve1", client, nodes, nodeEffectiveStatus)
	// Should log warning and retry (mock returns timeout again, so fails)
}
