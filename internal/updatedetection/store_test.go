package updatedetection

import (
	"testing"
	"time"
)

func TestStore_UpsertAndGet(t *testing.T) {
	store := NewStore()

	update := &UpdateInfo{
		ID:           "update-1",
		ResourceID:   "container-abc",
		ResourceType: "docker",
		ResourceName: "nginx",
		HostID:       "host-1",
		Type:         UpdateTypeDockerImage,
		LastChecked:  time.Now(),
	}

	store.UpsertUpdate(update)

	// Test GetAllUpdates
	all := store.GetAllUpdates()
	if len(all) != 1 {
		t.Errorf("expected 1 update, got %d", len(all))
	}

	// Test GetUpdatesForHost
	hostUpdates := store.GetUpdatesForHost("host-1")
	if len(hostUpdates) != 1 {
		t.Errorf("expected 1 update for host, got %d", len(hostUpdates))
	}

	// Test GetUpdatesForResource
	resourceUpdate := store.GetUpdatesForResource("container-abc")
	if resourceUpdate == nil {
		t.Fatal("expected update for resource, got nil")
	}
	if resourceUpdate.ID != "update-1" {
		t.Errorf("expected update ID 'update-1', got %q", resourceUpdate.ID)
	}

	// Test Count
	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}
}

func TestStore_UpsertPreservesFirstDetected(t *testing.T) {
	store := NewStore()

	firstTime := time.Now().Add(-24 * time.Hour)
	update := &UpdateInfo{
		ID:            "update-1",
		ResourceID:    "container-abc",
		HostID:        "host-1",
		FirstDetected: firstTime,
		LastChecked:   time.Now(),
	}

	store.UpsertUpdate(update)

	// Upsert again with different LastChecked
	update2 := &UpdateInfo{
		ID:            "update-1",
		ResourceID:    "container-abc",
		HostID:        "host-1",
		FirstDetected: time.Now(), // Should be ignored
		LastChecked:   time.Now(),
	}
	store.UpsertUpdate(update2)

	// FirstDetected should be preserved
	result := store.GetUpdatesForResource("container-abc")
	if result == nil {
		t.Fatal("expected update, got nil")
	}
	if !result.FirstDetected.Equal(firstTime) {
		t.Errorf("FirstDetected changed: got %v, want %v", result.FirstDetected, firstTime)
	}
}

func TestStore_DeleteUpdate(t *testing.T) {
	store := NewStore()

	update := &UpdateInfo{
		ID:         "update-1",
		ResourceID: "container-abc",
		HostID:     "host-1",
	}
	store.UpsertUpdate(update)

	if store.Count() != 1 {
		t.Fatal("expected 1 update before delete")
	}

	store.DeleteUpdate("update-1")

	if store.Count() != 0 {
		t.Errorf("expected 0 updates after delete, got %d", store.Count())
	}

	if store.GetUpdatesForResource("container-abc") != nil {
		t.Error("expected nil for deleted resource")
	}

	if len(store.GetUpdatesForHost("host-1")) != 0 {
		t.Error("expected empty updates for host after delete")
	}
}

func TestStore_DeleteUpdatesForResource(t *testing.T) {
	store := NewStore()

	store.UpsertUpdate(&UpdateInfo{ID: "update-1", ResourceID: "container-abc", HostID: "host-1"})
	store.UpsertUpdate(&UpdateInfo{ID: "update-2", ResourceID: "container-def", HostID: "host-1"})

	store.DeleteUpdatesForResource("container-abc")

	if store.Count() != 1 {
		t.Errorf("expected 1 update after delete, got %d", store.Count())
	}

	if store.GetUpdatesForResource("container-abc") != nil {
		t.Error("expected nil for deleted resource")
	}

	if store.GetUpdatesForResource("container-def") == nil {
		t.Error("expected update for non-deleted resource")
	}
}

func TestStore_DeleteUpdatesForHost(t *testing.T) {
	store := NewStore()

	store.UpsertUpdate(&UpdateInfo{ID: "update-1", ResourceID: "container-abc", HostID: "host-1"})
	store.UpsertUpdate(&UpdateInfo{ID: "update-2", ResourceID: "container-def", HostID: "host-1"})
	store.UpsertUpdate(&UpdateInfo{ID: "update-3", ResourceID: "container-ghi", HostID: "host-2"})

	store.DeleteUpdatesForHost("host-1")

	if store.Count() != 1 {
		t.Errorf("expected 1 update after host delete, got %d", store.Count())
	}

	if len(store.GetUpdatesForHost("host-1")) != 0 {
		t.Error("expected no updates for deleted host")
	}

	if len(store.GetUpdatesForHost("host-2")) != 1 {
		t.Error("expected 1 update for other host")
	}
}

func TestStore_GetSummary(t *testing.T) {
	store := NewStore()

	now := time.Now()
	store.UpsertUpdate(&UpdateInfo{
		ID:          "update-1",
		ResourceID:  "container-1",
		HostID:      "host-1",
		Type:        UpdateTypeDockerImage,
		Severity:    SeveritySecurity,
		LastChecked: now,
	})
	store.UpsertUpdate(&UpdateInfo{
		ID:          "update-2",
		ResourceID:  "container-2",
		HostID:      "host-1",
		Type:        UpdateTypeDockerImage,
		LastChecked: now.Add(-time.Hour),
	})
	store.UpsertUpdate(&UpdateInfo{
		ID:          "update-3",
		ResourceID:  "host-1-packages",
		HostID:      "host-1",
		Type:        UpdateTypePackage,
		LastChecked: now.Add(-2 * time.Hour),
	})

	summaries := store.GetSummary()

	summary, ok := summaries["host-1"]
	if !ok {
		t.Fatal("expected summary for host-1")
	}

	if summary.TotalUpdates != 3 {
		t.Errorf("expected 3 total updates, got %d", summary.TotalUpdates)
	}
	if summary.SecurityUpdates != 1 {
		t.Errorf("expected 1 security update, got %d", summary.SecurityUpdates)
	}
	if summary.ContainerUpdates != 2 {
		t.Errorf("expected 2 container updates, got %d", summary.ContainerUpdates)
	}
	if summary.PackageUpdates != 1 {
		t.Errorf("expected 1 package update, got %d", summary.PackageUpdates)
	}
	if !summary.LastChecked.Equal(now) {
		t.Errorf("expected LastChecked to be %v, got %v", now, summary.LastChecked)
	}
}
