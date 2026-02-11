package models

import (
	"testing"
	"time"
)

func TestUpsertDockerHost(t *testing.T) {
	state := NewState()

	// Test insert new host
	host1 := DockerHost{
		ID:       "host-1",
		Hostname: "docker-host-1",
		Status:   "online",
	}
	state.UpsertDockerHost(host1)

	hosts := state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host, got %d", len(hosts))
	}
	if hosts[0].ID != "host-1" {
		t.Errorf("Expected host ID 'host-1', got %q", hosts[0].ID)
	}

	// Test update existing host
	host1Updated := DockerHost{
		ID:       "host-1",
		Hostname: "docker-host-1",
		Status:   "offline",
	}
	state.UpsertDockerHost(host1Updated)

	hosts = state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host after update, got %d", len(hosts))
	}
	if hosts[0].Status != "offline" {
		t.Errorf("Expected status 'offline', got %q", hosts[0].Status)
	}

	// Test CustomDisplayName preservation
	state.SetDockerHostCustomDisplayName("host-1", "My Custom Name")
	host1WithoutCustomName := DockerHost{
		ID:       "host-1",
		Hostname: "docker-host-1",
		Status:   "online",
	}
	state.UpsertDockerHost(host1WithoutCustomName)

	hosts = state.GetDockerHosts()
	if hosts[0].CustomDisplayName != "My Custom Name" {
		t.Errorf("CustomDisplayName should be preserved, got %q", hosts[0].CustomDisplayName)
	}

	// Test Hidden flag preservation
	state.SetDockerHostHidden("host-1", true)
	host1Reset := DockerHost{
		ID:       "host-1",
		Hostname: "docker-host-1",
		Status:   "online",
		Hidden:   false, // explicitly false
	}
	state.UpsertDockerHost(host1Reset)

	hosts = state.GetDockerHosts()
	if !hosts[0].Hidden {
		t.Error("Hidden flag should be preserved on upsert")
	}

	// Test PendingUninstall flag preservation
	state.SetDockerHostPendingUninstall("host-1", true)
	host1Reset2 := DockerHost{
		ID:               "host-1",
		Hostname:         "docker-host-1",
		PendingUninstall: false,
	}
	state.UpsertDockerHost(host1Reset2)

	hosts = state.GetDockerHosts()
	if !hosts[0].PendingUninstall {
		t.Error("PendingUninstall flag should be preserved on upsert")
	}

	// Test Command preservation
	cmd := &DockerHostCommandStatus{Type: "test"}
	state.SetDockerHostCommand("host-1", cmd)
	host1Reset3 := DockerHost{
		ID:       "host-1",
		Hostname: "docker-host-1",
		Command:  nil,
	}
	state.UpsertDockerHost(host1Reset3)

	hosts = state.GetDockerHosts()
	if hosts[0].Command == nil || hosts[0].Command.Type != "test" {
		t.Error("Command should be preserved on upsert")
	}
}

func TestUpsertDockerHost_Sorting(t *testing.T) {
	state := NewState()

	// Insert hosts in non-alphabetical order
	state.UpsertDockerHost(DockerHost{ID: "3", Hostname: "charlie"})
	state.UpsertDockerHost(DockerHost{ID: "1", Hostname: "alpha"})
	state.UpsertDockerHost(DockerHost{ID: "2", Hostname: "bravo"})

	hosts := state.GetDockerHosts()
	if len(hosts) != 3 {
		t.Fatalf("Expected 3 hosts, got %d", len(hosts))
	}

	// Hosts should be sorted by hostname
	if hosts[0].Hostname != "alpha" || hosts[1].Hostname != "bravo" || hosts[2].Hostname != "charlie" {
		t.Errorf("Hosts should be sorted by hostname, got: %v, %v, %v",
			hosts[0].Hostname, hosts[1].Hostname, hosts[2].Hostname)
	}
}

func TestRemoveDockerHost(t *testing.T) {
	state := NewState()

	// Insert hosts
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1"})
	state.UpsertDockerHost(DockerHost{ID: "host-2", Hostname: "host2"})

	// Remove existing host
	removed, ok := state.RemoveDockerHost("host-1")
	if !ok {
		t.Error("Expected RemoveDockerHost to return true for existing host")
	}
	if removed.ID != "host-1" {
		t.Errorf("Expected removed host ID 'host-1', got %q", removed.ID)
	}

	hosts := state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 host after removal, got %d", len(hosts))
	}
	if hosts[0].ID != "host-2" {
		t.Errorf("Remaining host should be 'host-2', got %q", hosts[0].ID)
	}

	// Remove non-existing host
	removed, ok = state.RemoveDockerHost("non-existent")
	if ok {
		t.Error("Expected RemoveDockerHost to return false for non-existent host")
	}
	if removed.ID != "" {
		t.Errorf("Expected empty DockerHost for non-existent removal, got ID %q", removed.ID)
	}
}

func TestSetDockerHostStatus(t *testing.T) {
	state := NewState()

	// Test with no hosts
	changed := state.SetDockerHostStatus("host-1", "online")
	if changed {
		t.Error("SetDockerHostStatus should return false when host doesn't exist")
	}

	// Add host and set status
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1", Status: "offline"})

	changed = state.SetDockerHostStatus("host-1", "online")
	if !changed {
		t.Error("SetDockerHostStatus should return true when host exists")
	}

	hosts := state.GetDockerHosts()
	if hosts[0].Status != "online" {
		t.Errorf("Expected status 'online', got %q", hosts[0].Status)
	}

	// Set same status (no change)
	changed = state.SetDockerHostStatus("host-1", "online")
	if !changed {
		t.Error("SetDockerHostStatus should return true even when status unchanged")
	}
}

func TestSetDockerHostHidden(t *testing.T) {
	state := NewState()

	// Test with non-existent host
	_, ok := state.SetDockerHostHidden("host-1", true)
	if ok {
		t.Error("SetDockerHostHidden should return false for non-existent host")
	}

	// Add host and set hidden
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1", Hidden: false})

	host, ok := state.SetDockerHostHidden("host-1", true)
	if !ok {
		t.Error("SetDockerHostHidden should return true for existing host")
	}
	if !host.Hidden {
		t.Error("Expected Hidden to be true")
	}

	hosts := state.GetDockerHosts()
	if !hosts[0].Hidden {
		t.Error("Host in state should have Hidden=true")
	}

	// Set back to false
	host, ok = state.SetDockerHostHidden("host-1", false)
	if !ok || host.Hidden {
		t.Error("SetDockerHostHidden should set Hidden back to false")
	}
}

func TestSetDockerHostPendingUninstall(t *testing.T) {
	state := NewState()

	// Test with non-existent host
	_, ok := state.SetDockerHostPendingUninstall("host-1", true)
	if ok {
		t.Error("SetDockerHostPendingUninstall should return false for non-existent host")
	}

	// Add host and set pending uninstall
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1"})

	host, ok := state.SetDockerHostPendingUninstall("host-1", true)
	if !ok {
		t.Error("SetDockerHostPendingUninstall should return true for existing host")
	}
	if !host.PendingUninstall {
		t.Error("Expected PendingUninstall to be true")
	}

	hosts := state.GetDockerHosts()
	if !hosts[0].PendingUninstall {
		t.Error("Host in state should have PendingUninstall=true")
	}
}

func TestSetDockerHostCommand(t *testing.T) {
	state := NewState()

	cmd := &DockerHostCommandStatus{Type: "upgrade", Status: "running"}

	// Test with non-existent host
	_, ok := state.SetDockerHostCommand("host-1", cmd)
	if ok {
		t.Error("SetDockerHostCommand should return false for non-existent host")
	}

	// Add host and set command
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1"})

	host, ok := state.SetDockerHostCommand("host-1", cmd)
	if !ok {
		t.Error("SetDockerHostCommand should return true for existing host")
	}
	if host.Command == nil || host.Command.Type != "upgrade" {
		t.Error("Command should be set correctly")
	}

	// Clear command
	host, ok = state.SetDockerHostCommand("host-1", nil)
	if !ok || host.Command != nil {
		t.Error("SetDockerHostCommand should allow clearing command")
	}
}

func TestSetDockerHostCustomDisplayName(t *testing.T) {
	state := NewState()

	// Test with non-existent host
	_, ok := state.SetDockerHostCustomDisplayName("host-1", "Custom Name")
	if ok {
		t.Error("SetDockerHostCustomDisplayName should return false for non-existent host")
	}

	// Add host and set custom name
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1"})

	host, ok := state.SetDockerHostCustomDisplayName("host-1", "My Docker Server")
	if !ok {
		t.Error("SetDockerHostCustomDisplayName should return true for existing host")
	}
	if host.CustomDisplayName != "My Docker Server" {
		t.Errorf("Expected CustomDisplayName 'My Docker Server', got %q", host.CustomDisplayName)
	}

	// Clear custom name
	host, ok = state.SetDockerHostCustomDisplayName("host-1", "")
	if !ok || host.CustomDisplayName != "" {
		t.Error("SetDockerHostCustomDisplayName should allow clearing name")
	}
}

func TestTouchDockerHost(t *testing.T) {
	state := NewState()

	now := time.Now()

	// Test with non-existent host
	ok := state.TouchDockerHost("host-1", now)
	if ok {
		t.Error("TouchDockerHost should return false for non-existent host")
	}

	// Add host and touch it
	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1"})

	later := now.Add(time.Hour)
	ok = state.TouchDockerHost("host-1", later)
	if !ok {
		t.Error("TouchDockerHost should return true for existing host")
	}

	hosts := state.GetDockerHosts()
	if !hosts[0].LastSeen.Equal(later) {
		t.Errorf("LastSeen should be updated to %v, got %v", later, hosts[0].LastSeen)
	}
}

func TestRemoveStaleDockerHosts(t *testing.T) {
	state := NewState()

	now := time.Now()
	old := now.Add(-2 * time.Hour)
	recent := now.Add(-30 * time.Minute)

	// Add hosts with different last seen times
	state.UpsertDockerHost(DockerHost{ID: "old-1", Hostname: "old1", LastSeen: old})
	state.UpsertDockerHost(DockerHost{ID: "old-2", Hostname: "old2", LastSeen: old})
	state.UpsertDockerHost(DockerHost{ID: "recent", Hostname: "recent", LastSeen: recent})

	// Remove hosts older than 1 hour
	cutoff := now.Add(-1 * time.Hour)
	removed := state.RemoveStaleDockerHosts(cutoff)

	if len(removed) != 2 {
		t.Fatalf("Expected 2 removed hosts, got %d", len(removed))
	}

	hosts := state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("Expected 1 remaining host, got %d", len(hosts))
	}
	if hosts[0].ID != "recent" {
		t.Errorf("Expected remaining host ID 'recent', got %q", hosts[0].ID)
	}

	// Remove with no stale hosts
	removed = state.RemoveStaleDockerHosts(cutoff)
	if len(removed) != 0 {
		t.Errorf("Expected 0 removed hosts, got %d", len(removed))
	}
}

func TestGetDockerHosts_Copy(t *testing.T) {
	state := NewState()

	state.UpsertDockerHost(DockerHost{ID: "host-1", Hostname: "host1"})

	hosts1 := state.GetDockerHosts()
	hosts2 := state.GetDockerHosts()

	// Modify one slice
	if len(hosts1) > 0 {
		hosts1[0].Hostname = "modified"
	}

	// Other slice should be unchanged
	if len(hosts2) > 0 && hosts2[0].Hostname == "modified" {
		t.Error("GetDockerHosts should return a copy, not the same slice")
	}

	// Original state should be unchanged
	hosts3 := state.GetDockerHosts()
	if hosts3[0].Hostname == "modified" {
		t.Error("State should be unchanged by modifications to returned slice")
	}
}

func TestGetDockerHosts_DeepCopyNestedFields(t *testing.T) {
	state := NewState()
	command := &DockerHostCommandStatus{ID: "cmd-1", Status: "running"}

	host := DockerHost{
		ID:       "host-1",
		Hostname: "host1",
		Containers: []DockerContainer{
			{
				ID:     "ct-1",
				Labels: map[string]string{"app": "original"},
			},
		},
		NetworkInterfaces: []HostNetworkInterface{{Name: "eth0", Addresses: []string{"192.168.1.10"}}},
		Command:           command,
	}

	state.UpsertDockerHost(host)

	// Mutate caller-owned host and verify stored state was detached on write.
	host.Containers[0].Labels["app"] = "changed-before-read"
	host.NetworkInterfaces[0].Addresses[0] = "192.168.1.99"
	command.Status = "mutated-before-read"

	first := state.GetDockerHosts()
	first[0].Containers[0].Labels["app"] = "changed-after-read"
	first[0].NetworkInterfaces[0].Addresses[0] = "10.0.0.1"
	first[0].Command.Status = "mutated-after-read"

	second := state.GetDockerHosts()
	if got := second[0].Containers[0].Labels["app"]; got != "original" {
		t.Fatalf("expected container label to remain original, got %q", got)
	}
	if got := second[0].NetworkInterfaces[0].Addresses[0]; got != "192.168.1.10" {
		t.Fatalf("expected network address to remain 192.168.1.10, got %q", got)
	}
	if got := second[0].Command.Status; got != "running" {
		t.Fatalf("expected command status to remain running, got %q", got)
	}
}

func TestAddRemovedDockerHost(t *testing.T) {
	state := NewState()

	now := time.Now()
	earlier := now.Add(-1 * time.Hour)

	// Add removed host
	entry1 := RemovedDockerHost{
		ID:        "host-1",
		Hostname:  "host1",
		RemovedAt: earlier,
	}
	state.AddRemovedDockerHost(entry1)

	removed := state.GetRemovedDockerHosts()
	if len(removed) != 1 {
		t.Fatalf("Expected 1 removed host, got %d", len(removed))
	}
	if removed[0].ID != "host-1" {
		t.Errorf("Expected ID 'host-1', got %q", removed[0].ID)
	}

	// Add another removed host (should be sorted by RemovedAt desc)
	entry2 := RemovedDockerHost{
		ID:        "host-2",
		Hostname:  "host2",
		RemovedAt: now,
	}
	state.AddRemovedDockerHost(entry2)

	removed = state.GetRemovedDockerHosts()
	if len(removed) != 2 {
		t.Fatalf("Expected 2 removed hosts, got %d", len(removed))
	}
	// Most recent should be first
	if removed[0].ID != "host-2" {
		t.Errorf("Most recently removed host should be first, got %q", removed[0].ID)
	}

	// Update existing entry (replace)
	entry1Updated := RemovedDockerHost{
		ID:        "host-1",
		Hostname:  "host1-updated",
		RemovedAt: now.Add(time.Minute), // even more recent
	}
	state.AddRemovedDockerHost(entry1Updated)

	removed = state.GetRemovedDockerHosts()
	if len(removed) != 2 {
		t.Fatalf("Expected 2 removed hosts after update, got %d", len(removed))
	}
	// host-1 should now be first (most recent)
	if removed[0].ID != "host-1" {
		t.Errorf("Updated host should be first, got %q", removed[0].ID)
	}
	if removed[0].Hostname != "host1-updated" {
		t.Errorf("Expected updated hostname, got %q", removed[0].Hostname)
	}
}

func TestRemoveRemovedDockerHost(t *testing.T) {
	state := NewState()

	now := time.Now()

	// Add some removed hosts
	state.AddRemovedDockerHost(RemovedDockerHost{ID: "host-1", RemovedAt: now})
	state.AddRemovedDockerHost(RemovedDockerHost{ID: "host-2", RemovedAt: now})

	// Remove one
	state.RemoveRemovedDockerHost("host-1")

	removed := state.GetRemovedDockerHosts()
	if len(removed) != 1 {
		t.Fatalf("Expected 1 removed host after deletion, got %d", len(removed))
	}
	if removed[0].ID != "host-2" {
		t.Errorf("Expected remaining host ID 'host-2', got %q", removed[0].ID)
	}

	// Remove non-existent (should not panic or error)
	state.RemoveRemovedDockerHost("non-existent")
	removed = state.GetRemovedDockerHosts()
	if len(removed) != 1 {
		t.Errorf("Removing non-existent should not change count, got %d", len(removed))
	}
}

func TestGetRemovedDockerHosts_Copy(t *testing.T) {
	state := NewState()

	state.AddRemovedDockerHost(RemovedDockerHost{ID: "host-1", RemovedAt: time.Now()})

	hosts1 := state.GetRemovedDockerHosts()
	hosts2 := state.GetRemovedDockerHosts()

	// Modify one slice
	if len(hosts1) > 0 {
		hosts1[0].Hostname = "modified"
	}

	// Other slice should be unchanged
	if len(hosts2) > 0 && hosts2[0].Hostname == "modified" {
		t.Error("GetRemovedDockerHosts should return a copy, not the same slice")
	}
}
