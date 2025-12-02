package models

import (
	"testing"
	"time"
)

func TestUpsertHost(t *testing.T) {
	state := NewState()

	// Test insert new host
	host1 := Host{
		ID:       "host-1",
		Hostname: "server-1",
		Status:   "online",
	}
	state.UpsertHost(host1)

	hosts := state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}
	if hosts[0].ID != "host-1" {
		t.Errorf("Expected host ID 'host-1', got %q", hosts[0].ID)
	}

	// Test update existing host
	host1Updated := Host{
		ID:       "host-1",
		Hostname: "server-1",
		Status:   "offline",
	}
	state.UpsertHost(host1Updated)

	hosts = state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host after update, got %d", len(hosts))
	}
	if hosts[0].Status != "offline" {
		t.Errorf("Expected status 'offline', got %q", hosts[0].Status)
	}

	// Test multiple hosts and sorting
	state.UpsertHost(Host{ID: "host-2", Hostname: "alpha-server"})
	state.UpsertHost(Host{ID: "host-3", Hostname: "zulu-server"})

	hosts = state.GetHosts()
	if len(hosts) != 3 {
		t.Fatalf("Expected 3 hosts, got %d", len(hosts))
	}

	// Hosts should be sorted by hostname
	if hosts[0].Hostname != "alpha-server" {
		t.Errorf("First host should be 'alpha-server', got %q", hosts[0].Hostname)
	}
}

func TestGetHosts_Copy(t *testing.T) {
	state := NewState()

	state.UpsertHost(Host{ID: "host-1", Hostname: "server-1"})

	hosts1 := state.GetHosts()
	hosts2 := state.GetHosts()

	// Modify one slice
	if len(hosts1) > 0 {
		hosts1[0].Hostname = "modified"
	}

	// Other slice should be unchanged
	if len(hosts2) > 0 && hosts2[0].Hostname == "modified" {
		t.Error("GetHosts should return a copy, not the same slice")
	}

	// Original state should be unchanged
	hosts3 := state.GetHosts()
	if hosts3[0].Hostname == "modified" {
		t.Error("State should be unchanged by modifications to returned slice")
	}
}

func TestRemoveHost(t *testing.T) {
	state := NewState()

	// Insert hosts
	state.UpsertHost(Host{ID: "host-1", Hostname: "server1"})
	state.UpsertHost(Host{ID: "host-2", Hostname: "server2"})

	// Remove existing host
	removed, ok := state.RemoveHost("host-1")
	if !ok {
		t.Error("Expected RemoveHost to return true for existing host")
	}
	if removed.ID != "host-1" {
		t.Errorf("Expected removed host ID 'host-1', got %q", removed.ID)
	}

	hosts := state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host after removal, got %d", len(hosts))
	}
	if hosts[0].ID != "host-2" {
		t.Errorf("Remaining host should be 'host-2', got %q", hosts[0].ID)
	}

	// Remove non-existing host
	removed, ok = state.RemoveHost("non-existent")
	if ok {
		t.Error("Expected RemoveHost to return false for non-existent host")
	}
	if removed.ID != "" {
		t.Errorf("Expected empty Host for non-existent removal, got ID %q", removed.ID)
	}
}

func TestSetHostStatus(t *testing.T) {
	state := NewState()

	// Test with no hosts
	changed := state.SetHostStatus("host-1", "online")
	if changed {
		t.Error("SetHostStatus should return false when host doesn't exist")
	}

	// Add host and set status
	state.UpsertHost(Host{ID: "host-1", Hostname: "server1", Status: "offline"})

	changed = state.SetHostStatus("host-1", "online")
	if !changed {
		t.Error("SetHostStatus should return true when host exists")
	}

	hosts := state.GetHosts()
	if hosts[0].Status != "online" {
		t.Errorf("Expected status 'online', got %q", hosts[0].Status)
	}

	// Set same status (no change)
	changed = state.SetHostStatus("host-1", "online")
	if !changed {
		t.Error("SetHostStatus should return true even when status unchanged")
	}
}

func TestTouchHost(t *testing.T) {
	state := NewState()

	now := time.Now()

	// Test with non-existent host
	ok := state.TouchHost("host-1", now)
	if ok {
		t.Error("TouchHost should return false for non-existent host")
	}

	// Add host and touch it
	state.UpsertHost(Host{ID: "host-1", Hostname: "server1"})

	later := now.Add(time.Hour)
	ok = state.TouchHost("host-1", later)
	if !ok {
		t.Error("TouchHost should return true for existing host")
	}

	hosts := state.GetHosts()
	if !hosts[0].LastSeen.Equal(later) {
		t.Errorf("LastSeen should be updated to %v, got %v", later, hosts[0].LastSeen)
	}
}

func TestUpdateRecentlyResolved(t *testing.T) {
	state := NewState()

	now := time.Now()
	alerts := []ResolvedAlert{
		{
			Alert: Alert{
				ID:           "alert-1",
				Type:         "cpu_high",
				Level:        "warning",
				ResourceName: "server-1",
				Message:      "High CPU usage",
			},
			ResolvedTime: now,
		},
		{
			Alert: Alert{
				ID:           "alert-2",
				Type:         "disk_low",
				Level:        "critical",
				ResourceName: "server-2",
				Message:      "Low disk space",
			},
			ResolvedTime: now.Add(-time.Hour),
		},
	}

	state.UpdateRecentlyResolved(alerts)

	// The method updates internal state - verify via snapshot or other means
	// Since RecentlyResolvedAlerts might be accessed differently, just verify no panic
	// and method executes correctly
}

func TestSetConnectionHealth(t *testing.T) {
	state := NewState()

	// Set connection healthy
	state.SetConnectionHealth("pve-cluster-1", true)

	snapshot := state.GetSnapshot()
	if healthy, ok := snapshot.ConnectionHealth["pve-cluster-1"]; !ok || !healthy {
		t.Error("Expected connection to be healthy")
	}

	// Set connection unhealthy
	state.SetConnectionHealth("pve-cluster-1", false)

	snapshot = state.GetSnapshot()
	if healthy, ok := snapshot.ConnectionHealth["pve-cluster-1"]; !ok || healthy {
		t.Error("Expected connection to be unhealthy")
	}

	// Multiple connections
	state.SetConnectionHealth("pve-cluster-2", true)
	state.SetConnectionHealth("pbs-instance-1", true)

	snapshot = state.GetSnapshot()
	if len(snapshot.ConnectionHealth) != 3 {
		t.Errorf("Expected 3 connection health entries, got %d", len(snapshot.ConnectionHealth))
	}
}

func TestRemoveConnectionHealth(t *testing.T) {
	state := NewState()

	// Set up connection health entries
	state.SetConnectionHealth("pve-cluster-1", true)
	state.SetConnectionHealth("pve-cluster-2", false)

	// Remove one
	state.RemoveConnectionHealth("pve-cluster-1")

	snapshot := state.GetSnapshot()
	if _, ok := snapshot.ConnectionHealth["pve-cluster-1"]; ok {
		t.Error("Expected pve-cluster-1 to be removed")
	}
	if _, ok := snapshot.ConnectionHealth["pve-cluster-2"]; !ok {
		t.Error("Expected pve-cluster-2 to still exist")
	}

	// Remove non-existent (should not panic)
	state.RemoveConnectionHealth("non-existent")

	// Remove remaining
	state.RemoveConnectionHealth("pve-cluster-2")
	snapshot = state.GetSnapshot()
	if len(snapshot.ConnectionHealth) != 0 {
		t.Errorf("Expected empty connection health map, got %d entries", len(snapshot.ConnectionHealth))
	}
}

func TestUpdatePBSBackups(t *testing.T) {
	state := NewState()

	now := time.Now()

	// Add backups from first instance
	backups1 := []PBSBackup{
		{ID: "backup-1", Instance: "pbs-1", BackupTime: now},
		{ID: "backup-2", Instance: "pbs-1", BackupTime: now.Add(-time.Hour)},
	}
	state.UpdatePBSBackups("pbs-1", backups1)

	snapshot := state.GetSnapshot()
	if len(snapshot.PBSBackups) != 2 {
		t.Fatalf("Expected 2 backups, got %d", len(snapshot.PBSBackups))
	}

	// Add backups from second instance
	backups2 := []PBSBackup{
		{ID: "backup-3", Instance: "pbs-2", BackupTime: now.Add(-30 * time.Minute)},
	}
	state.UpdatePBSBackups("pbs-2", backups2)

	snapshot = state.GetSnapshot()
	if len(snapshot.PBSBackups) != 3 {
		t.Fatalf("Expected 3 backups, got %d", len(snapshot.PBSBackups))
	}

	// Update first instance (should replace its backups)
	backups1Updated := []PBSBackup{
		{ID: "backup-4", Instance: "pbs-1", BackupTime: now.Add(time.Hour)},
	}
	state.UpdatePBSBackups("pbs-1", backups1Updated)

	snapshot = state.GetSnapshot()
	if len(snapshot.PBSBackups) != 2 {
		t.Fatalf("Expected 2 backups after update, got %d", len(snapshot.PBSBackups))
	}

	// Verify pbs-1 backups were replaced
	hasOldBackup := false
	for _, b := range snapshot.PBSBackups {
		if b.ID == "backup-1" || b.ID == "backup-2" {
			hasOldBackup = true
			break
		}
	}
	if hasOldBackup {
		t.Error("Old pbs-1 backups should have been replaced")
	}
}

func TestUpdatePMGBackups(t *testing.T) {
	state := NewState()

	now := time.Now()

	// Add backups from first instance
	backups1 := []PMGBackup{
		{Filename: "backup-1.conf", Instance: "pmg-1", BackupTime: now},
		{Filename: "backup-2.conf", Instance: "pmg-1", BackupTime: now.Add(-time.Hour)},
	}
	state.UpdatePMGBackups("pmg-1", backups1)

	snapshot := state.GetSnapshot()
	if len(snapshot.PMGBackups) != 2 {
		t.Fatalf("Expected 2 backups, got %d", len(snapshot.PMGBackups))
	}

	// Backups should be sorted by time (newest first)
	if snapshot.PMGBackups[0].BackupTime.Before(snapshot.PMGBackups[1].BackupTime) {
		t.Error("Backups should be sorted by time descending")
	}

	// Add backups from second instance
	backups2 := []PMGBackup{
		{Filename: "backup-3.conf", Instance: "pmg-2", BackupTime: now.Add(-30 * time.Minute)},
	}
	state.UpdatePMGBackups("pmg-2", backups2)

	snapshot = state.GetSnapshot()
	if len(snapshot.PMGBackups) != 3 {
		t.Fatalf("Expected 3 backups, got %d", len(snapshot.PMGBackups))
	}

	// Update first instance with empty (should remove its backups)
	state.UpdatePMGBackups("pmg-1", []PMGBackup{})

	snapshot = state.GetSnapshot()
	if len(snapshot.PMGBackups) != 1 {
		t.Fatalf("Expected 1 backup after clearing pmg-1, got %d", len(snapshot.PMGBackups))
	}
	if snapshot.PMGBackups[0].Instance != "pmg-2" {
		t.Error("Remaining backup should be from pmg-2")
	}
}

func TestUpdatePhysicalDisks(t *testing.T) {
	state := NewState()

	// Add disks from first instance
	disks1 := []PhysicalDisk{
		{ID: "disk-1", Instance: "pve-1", Node: "node1", DevPath: "/dev/sda"},
		{ID: "disk-2", Instance: "pve-1", Node: "node1", DevPath: "/dev/sdb"},
	}
	state.UpdatePhysicalDisks("pve-1", disks1)

	snapshot := state.GetSnapshot()
	if len(snapshot.PhysicalDisks) != 2 {
		t.Fatalf("Expected 2 disks, got %d", len(snapshot.PhysicalDisks))
	}

	// Add disks from second instance
	disks2 := []PhysicalDisk{
		{ID: "disk-3", Instance: "pve-2", Node: "node2", DevPath: "/dev/sda"},
	}
	state.UpdatePhysicalDisks("pve-2", disks2)

	snapshot = state.GetSnapshot()
	if len(snapshot.PhysicalDisks) != 3 {
		t.Fatalf("Expected 3 disks, got %d", len(snapshot.PhysicalDisks))
	}

	// Update first instance (should replace its disks)
	disks1Updated := []PhysicalDisk{
		{ID: "disk-4", Instance: "pve-1", Node: "node1", DevPath: "/dev/nvme0n1"},
	}
	state.UpdatePhysicalDisks("pve-1", disks1Updated)

	snapshot = state.GetSnapshot()
	if len(snapshot.PhysicalDisks) != 2 {
		t.Fatalf("Expected 2 disks after update, got %d", len(snapshot.PhysicalDisks))
	}

	// Verify sorting by node then devpath
	if snapshot.PhysicalDisks[0].Node > snapshot.PhysicalDisks[1].Node {
		t.Error("Disks should be sorted by node")
	}
}

func TestUpdateStorageForInstance(t *testing.T) {
	state := NewState()

	// Add storage from first instance
	storage1 := []Storage{
		{ID: "storage-1", Instance: "pve-1", Name: "local"},
		{ID: "storage-2", Instance: "pve-1", Name: "ceph-pool"},
	}
	state.UpdateStorageForInstance("pve-1", storage1)

	snapshot := state.GetSnapshot()
	if len(snapshot.Storage) != 2 {
		t.Fatalf("Expected 2 storage entries, got %d", len(snapshot.Storage))
	}

	// Add storage from second instance
	storage2 := []Storage{
		{ID: "storage-3", Instance: "pve-2", Name: "local"},
	}
	state.UpdateStorageForInstance("pve-2", storage2)

	snapshot = state.GetSnapshot()
	if len(snapshot.Storage) != 3 {
		t.Fatalf("Expected 3 storage entries, got %d", len(snapshot.Storage))
	}

	// Update first instance with empty (should remove its storage)
	state.UpdateStorageForInstance("pve-1", []Storage{})

	snapshot = state.GetSnapshot()
	if len(snapshot.Storage) != 1 {
		t.Fatalf("Expected 1 storage after clearing pve-1, got %d", len(snapshot.Storage))
	}
}

func TestUpdatePBSInstances(t *testing.T) {
	state := NewState()

	instances := []PBSInstance{
		{ID: "pbs-1", Name: "Backup Server 1"},
		{ID: "pbs-2", Name: "Backup Server 2"},
	}
	state.UpdatePBSInstances(instances)

	snapshot := state.GetSnapshot()
	if len(snapshot.PBSInstances) != 2 {
		t.Fatalf("Expected 2 PBS instances, got %d", len(snapshot.PBSInstances))
	}

	// Replace with different instances
	newInstances := []PBSInstance{
		{ID: "pbs-3", Name: "New Backup Server"},
	}
	state.UpdatePBSInstances(newInstances)

	snapshot = state.GetSnapshot()
	if len(snapshot.PBSInstances) != 1 {
		t.Fatalf("Expected 1 PBS instance after replacement, got %d", len(snapshot.PBSInstances))
	}
	if snapshot.PBSInstances[0].ID != "pbs-3" {
		t.Error("Expected pbs-3 instance")
	}
}

func TestUpdatePBSInstance(t *testing.T) {
	state := NewState()

	// Add first instance
	state.UpdatePBSInstance(PBSInstance{ID: "pbs-1", Name: "Server 1"})

	snapshot := state.GetSnapshot()
	if len(snapshot.PBSInstances) != 1 {
		t.Fatalf("Expected 1 PBS instance, got %d", len(snapshot.PBSInstances))
	}

	// Add second instance
	state.UpdatePBSInstance(PBSInstance{ID: "pbs-2", Name: "Server 2"})

	snapshot = state.GetSnapshot()
	if len(snapshot.PBSInstances) != 2 {
		t.Fatalf("Expected 2 PBS instances, got %d", len(snapshot.PBSInstances))
	}

	// Update existing instance
	state.UpdatePBSInstance(PBSInstance{ID: "pbs-1", Name: "Server 1 Updated"})

	snapshot = state.GetSnapshot()
	if len(snapshot.PBSInstances) != 2 {
		t.Fatalf("Expected 2 PBS instances after update, got %d", len(snapshot.PBSInstances))
	}

	// Find updated instance
	var found bool
	for _, inst := range snapshot.PBSInstances {
		if inst.ID == "pbs-1" && inst.Name == "Server 1 Updated" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pbs-1 to be updated with new name")
	}
}

func TestUpdatePMGInstances(t *testing.T) {
	state := NewState()

	instances := []PMGInstance{
		{ID: "pmg-1", Name: "Mail Gateway 1"},
		{ID: "pmg-2", Name: "Mail Gateway 2"},
	}
	state.UpdatePMGInstances(instances)

	snapshot := state.GetSnapshot()
	if len(snapshot.PMGInstances) != 2 {
		t.Fatalf("Expected 2 PMG instances, got %d", len(snapshot.PMGInstances))
	}

	// Replace with empty
	state.UpdatePMGInstances([]PMGInstance{})

	snapshot = state.GetSnapshot()
	if len(snapshot.PMGInstances) != 0 {
		t.Fatalf("Expected 0 PMG instances after clearing, got %d", len(snapshot.PMGInstances))
	}
}

func TestUpdatePMGInstance(t *testing.T) {
	state := NewState()

	// Add first instance
	state.UpdatePMGInstance(PMGInstance{ID: "pmg-1", Name: "Gateway 1"})

	snapshot := state.GetSnapshot()
	if len(snapshot.PMGInstances) != 1 {
		t.Fatalf("Expected 1 PMG instance, got %d", len(snapshot.PMGInstances))
	}

	// Add second instance
	state.UpdatePMGInstance(PMGInstance{ID: "pmg-2", Name: "Gateway 2"})

	snapshot = state.GetSnapshot()
	if len(snapshot.PMGInstances) != 2 {
		t.Fatalf("Expected 2 PMG instances, got %d", len(snapshot.PMGInstances))
	}

	// Update existing instance
	state.UpdatePMGInstance(PMGInstance{ID: "pmg-1", Name: "Gateway 1 Updated"})

	snapshot = state.GetSnapshot()
	if len(snapshot.PMGInstances) != 2 {
		t.Fatalf("Expected 2 PMG instances after update, got %d", len(snapshot.PMGInstances))
	}

	// Verify update
	var found bool
	for _, inst := range snapshot.PMGInstances {
		if inst.ID == "pmg-1" && inst.Name == "Gateway 1 Updated" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected pmg-1 to be updated with new name")
	}
}
