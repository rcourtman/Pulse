package updatedetection

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestDefaultManagerConfig(t *testing.T) {
	cfg := DefaultManagerConfig()
	if !cfg.Enabled {
		t.Fatalf("expected enabled by default")
	}
	if cfg.CheckInterval != 6*time.Hour {
		t.Fatalf("expected check interval 6h, got %v", cfg.CheckInterval)
	}
	if cfg.AlertDelayHours != 24 {
		t.Fatalf("expected alert delay 24, got %d", cfg.AlertDelayHours)
	}
	if !cfg.EnableDockerUpdates {
		t.Fatalf("expected docker updates enabled by default")
	}
}

func TestManagerConfigAccessors(t *testing.T) {
	cfg := ManagerConfig{
		Enabled:             false,
		CheckInterval:       time.Minute,
		AlertDelayHours:     12,
		EnableDockerUpdates: false,
	}
	mgr := NewManager(cfg, zerolog.Nop())
	if mgr.Enabled() {
		t.Fatalf("expected disabled manager")
	}
	mgr.SetEnabled(true)
	if !mgr.Enabled() {
		t.Fatalf("expected enabled manager after SetEnabled")
	}
	if mgr.AlertDelayHours() != 12 {
		t.Fatalf("expected alert delay 12, got %d", mgr.AlertDelayHours())
	}
}

func TestManagerProcessDockerContainerUpdate(t *testing.T) {
	now := time.Now()
	status := &ContainerUpdateStatus{
		UpdateAvailable: true,
		CurrentDigest:   "sha256:old",
		LatestDigest:    "sha256:new",
		LastChecked:     now,
		Error:           "boom",
	}

	t.Run("Disabled", func(t *testing.T) {
		cfg := DefaultManagerConfig()
		cfg.Enabled = false
		mgr := NewManager(cfg, zerolog.Nop())
		mgr.ProcessDockerContainerUpdate("host-1", "container-1", "nginx", "nginx:latest", "sha256:old", status)
		if mgr.store.Count() != 0 {
			t.Fatalf("expected no updates when disabled")
		}
	})

	t.Run("DockerUpdatesDisabled", func(t *testing.T) {
		cfg := DefaultManagerConfig()
		cfg.EnableDockerUpdates = false
		mgr := NewManager(cfg, zerolog.Nop())
		mgr.ProcessDockerContainerUpdate("host-1", "container-1", "nginx", "nginx:latest", "sha256:old", status)
		if mgr.store.Count() != 0 {
			t.Fatalf("expected no updates when docker updates disabled")
		}
	})

	t.Run("NilStatus", func(t *testing.T) {
		mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
		mgr.ProcessDockerContainerUpdate("host-1", "container-1", "nginx", "nginx:latest", "sha256:old", nil)
		if mgr.store.Count() != 0 {
			t.Fatalf("expected no updates for nil status")
		}
	})

	t.Run("NoUpdateDeletes", func(t *testing.T) {
		mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
		mgr.store.UpsertUpdate(&UpdateInfo{
			ID:         "docker:host-1:container-1",
			ResourceID: "container-1",
			HostID:     "host-1",
		})
		mgr.ProcessDockerContainerUpdate("host-1", "container-1", "nginx", "nginx:latest", "sha256:old", &ContainerUpdateStatus{
			UpdateAvailable: false,
			LastChecked:     now,
		})
		if mgr.store.Count() != 0 {
			t.Fatalf("expected update to be deleted when no update available")
		}
	})

	t.Run("UpdateAvailable", func(t *testing.T) {
		mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
		mgr.ProcessDockerContainerUpdate("host-1", "container-1", "nginx", "nginx:latest", "sha256:old", status)
		update := mgr.store.GetUpdatesForResource("container-1")
		if update == nil {
			t.Fatalf("expected update to be stored")
		}
		if update.ID != "docker:host-1:container-1" {
			t.Fatalf("unexpected update ID %q", update.ID)
		}
		if update.Error != "boom" {
			t.Fatalf("expected error to be stored")
		}
		if update.CurrentVersion != "nginx:latest" {
			t.Fatalf("expected current version to be stored")
		}
	})
}

func TestManagerCheckImageUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("Disabled", func(t *testing.T) {
		cfg := DefaultManagerConfig()
		cfg.Enabled = false
		mgr := NewManager(cfg, zerolog.Nop())
		info, err := mgr.CheckImageUpdate(ctx, "nginx:latest", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info != nil {
			t.Fatalf("expected nil info when disabled")
		}
	})

	t.Run("Cached", func(t *testing.T) {
		mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
		cacheKey := "registry-1.docker.io/library/nginx:latest"
		mgr.registry.cacheDigest(cacheKey, "sha256:new")

		info, err := mgr.CheckImageUpdate(ctx, "nginx:latest", "sha256:old")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if info == nil || !info.UpdateAvailable {
			t.Fatalf("expected update available from cache")
		}
	})
}

func TestManagerGetUpdatesAndAccessors(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
	now := time.Now()

	update1 := &UpdateInfo{
		ID:           "update-1",
		ResourceID:   "res-1",
		ResourceType: "docker",
		ResourceName: "nginx",
		HostID:       "host-1",
		Type:         UpdateTypeDockerImage,
		Severity:     SeveritySecurity,
		LastChecked:  now,
	}
	update2 := &UpdateInfo{
		ID:           "update-2",
		ResourceID:   "res-2",
		ResourceType: "vm",
		ResourceName: "vm-1",
		HostID:       "host-2",
		Type:         UpdateTypePackage,
		LastChecked:  now.Add(-time.Minute),
	}
	mgr.store.UpsertUpdate(update1)
	mgr.store.UpsertUpdate(update2)

	if mgr.GetTotalCount() != 2 {
		t.Fatalf("expected total count 2, got %d", mgr.GetTotalCount())
	}

	all := mgr.GetUpdates(UpdateFilters{})
	if len(all) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(all))
	}

	filtered := mgr.GetUpdates(UpdateFilters{HostID: "host-1"})
	if len(filtered) != 1 || filtered[0].ID != "update-1" {
		t.Fatalf("expected one update for host-1")
	}

	if len(mgr.GetUpdatesForHost("host-1")) != 1 {
		t.Fatalf("expected 1 update for host-1")
	}
	if mgr.GetUpdatesForResource("res-2") == nil {
		t.Fatalf("expected update for resource res-2")
	}

	summary := mgr.GetSummary()
	if summary["host-1"].TotalUpdates != 1 {
		t.Fatalf("expected summary for host-1")
	}

	mgr.AddRegistryConfig(RegistryConfig{Host: "example.com", Username: "user"})
	if _, ok := mgr.registry.configs["example.com"]; !ok {
		t.Fatalf("expected registry config to be stored")
	}

	mgr.DeleteUpdatesForHost("host-1")
	if len(mgr.GetUpdatesForHost("host-1")) != 0 {
		t.Fatalf("expected updates removed for host-1")
	}
}

func TestManagerCleanupStale(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
	now := time.Now()

	mgr.store.UpsertUpdate(&UpdateInfo{
		ID:          "stale",
		ResourceID:  "res-1",
		HostID:      "host-1",
		LastChecked: now.Add(-2 * time.Hour),
	})
	mgr.store.UpsertUpdate(&UpdateInfo{
		ID:          "fresh",
		ResourceID:  "res-2",
		HostID:      "host-1",
		LastChecked: now,
	})

	removed := mgr.CleanupStale(time.Hour)
	if removed != 1 {
		t.Fatalf("expected 1 stale update removed, got %d", removed)
	}
	if mgr.GetTotalCount() != 1 {
		t.Fatalf("expected one update remaining")
	}
}

func TestManagerGetUpdatesReadyForAlert(t *testing.T) {
	mgr := NewManager(DefaultManagerConfig(), zerolog.Nop())
	mgr.alertDelayHours = 1
	now := time.Now()

	mgr.store.UpsertUpdate(&UpdateInfo{
		ID:            "ready",
		ResourceID:    "res-1",
		HostID:        "host-1",
		FirstDetected: now.Add(-2 * time.Hour),
		LastChecked:   now,
	})
	mgr.store.UpsertUpdate(&UpdateInfo{
		ID:            "recent",
		ResourceID:    "res-2",
		HostID:        "host-1",
		FirstDetected: now.Add(-30 * time.Minute),
		LastChecked:   now,
	})
	mgr.store.UpsertUpdate(&UpdateInfo{
		ID:            "error",
		ResourceID:    "res-3",
		HostID:        "host-1",
		FirstDetected: now.Add(-2 * time.Hour),
		LastChecked:   now,
		Error:         "rate limited",
	})

	ready := mgr.GetUpdatesReadyForAlert()
	if len(ready) != 1 || ready[0].ID != "ready" {
		t.Fatalf("expected only ready update, got %d", len(ready))
	}
}

func TestUpdateFilters(t *testing.T) {
	update := &UpdateInfo{
		HostID:       "host-1",
		ResourceType: "docker",
		Type:         UpdateTypeDockerImage,
		Severity:     SeveritySecurity,
		Error:        "",
	}

	if !(&UpdateFilters{}).IsEmpty() {
		t.Fatalf("expected empty filters to be empty")
	}
	if (&UpdateFilters{HostID: "host-1"}).IsEmpty() {
		t.Fatalf("expected non-empty filters")
	}

	if (&UpdateFilters{HostID: "other"}).Matches(update) {
		t.Fatalf("expected host mismatch to fail")
	}
	if (&UpdateFilters{ResourceType: "vm"}).Matches(update) {
		t.Fatalf("expected resource type mismatch to fail")
	}
	if (&UpdateFilters{UpdateType: UpdateTypePackage}).Matches(update) {
		t.Fatalf("expected update type mismatch to fail")
	}
	if (&UpdateFilters{Severity: SeverityBugfix}).Matches(update) {
		t.Fatalf("expected severity mismatch to fail")
	}

	hasError := true
	if (&UpdateFilters{HasError: &hasError}).Matches(update) {
		t.Fatalf("expected error filter to fail")
	}

	update.Error = "boom"
	hasError = false
	if (&UpdateFilters{HasError: &hasError}).Matches(update) {
		t.Fatalf("expected error=false filter to fail")
	}

	update.Error = ""
	hasError = false
	filters := UpdateFilters{
		HostID:       "host-1",
		ResourceType: "docker",
		UpdateType:   UpdateTypeDockerImage,
		Severity:     SeveritySecurity,
		HasError:     &hasError,
	}
	if !filters.Matches(update) {
		t.Fatalf("expected filters to match update")
	}
}
