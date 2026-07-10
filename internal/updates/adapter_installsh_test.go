package updates

import (
	"context"
	"testing"
)

func TestDockerUpdater(t *testing.T) {
	updater := NewDockerUpdater()

	t.Run("SupportsApply", func(t *testing.T) {
		if updater.SupportsApply() {
			t.Error("DockerUpdater should not support auto apply")
		}
	})

	t.Run("GetDeploymentType", func(t *testing.T) {
		if updater.GetDeploymentType() != "docker" {
			t.Errorf("GetDeploymentType() = %q, want %q", updater.GetDeploymentType(), "docker")
		}
	})

	t.Run("PrepareUpdate uses default image repo", func(t *testing.T) {
		plan, err := updater.PrepareUpdate(context.Background(), UpdateRequest{Version: "v4.25.0"})
		if err != nil {
			t.Fatalf("PrepareUpdate() error = %v", err)
		}
		if got := plan.Instructions[0]; got != "docker pull rcourtman/pulse:4.25.0" {
			t.Fatalf("pull instruction = %q", got)
		}
		if got := plan.Instructions[2]; got != "docker run -d --name pulse rcourtman/pulse:4.25.0" {
			t.Fatalf("run instruction = %q", got)
		}
	})

	t.Run("PrepareUpdate uses configured image repo", func(t *testing.T) {
		t.Setenv("PULSE_DOCKER_IMAGE_REPO", "example/pulse-enterprise")

		plan, err := updater.PrepareUpdate(context.Background(), UpdateRequest{Version: "v4.25.0"})
		if err != nil {
			t.Fatalf("PrepareUpdate() error = %v", err)
		}
		if got := plan.Instructions[0]; got != "docker pull example/pulse-enterprise:4.25.0" {
			t.Fatalf("pull instruction = %q", got)
		}
		if got := plan.Instructions[2]; got != "docker run -d --name pulse example/pulse-enterprise:4.25.0" {
			t.Fatalf("run instruction = %q", got)
		}
	})
}

func TestAURUpdater(t *testing.T) {
	updater := NewAURUpdater()

	t.Run("SupportsApply", func(t *testing.T) {
		if updater.SupportsApply() {
			t.Error("AURUpdater should not support auto apply")
		}
	})

	t.Run("GetDeploymentType", func(t *testing.T) {
		if updater.GetDeploymentType() != "aur" {
			t.Errorf("GetDeploymentType() = %q, want %q", updater.GetDeploymentType(), "aur")
		}
	})

	t.Run("PrepareUpdate returns manual instructions", func(t *testing.T) {
		plan, err := updater.PrepareUpdate(context.Background(), UpdateRequest{Version: "v4.25.0"})
		if err != nil {
			t.Fatalf("PrepareUpdate() error = %v", err)
		}
		if plan.CanAutoUpdate {
			t.Error("AUR plan should not allow auto update")
		}
		if len(plan.Instructions) == 0 {
			t.Error("AUR plan should carry package-manager instructions")
		}
	})
}

func TestInstallShAdapter_SupportsApply(t *testing.T) {
	adapter := &InstallShAdapter{}

	if !adapter.SupportsApply() {
		t.Error("InstallShAdapter should support auto apply")
	}
}

func TestInstallShAdapter_GetDeploymentType(t *testing.T) {
	adapter := &InstallShAdapter{}

	if adapter.GetDeploymentType() != "systemd" {
		t.Errorf("GetDeploymentType() = %q, want %q", adapter.GetDeploymentType(), "systemd")
	}
}

func TestNewInstallShAdapter(t *testing.T) {
	adapter := NewInstallShAdapter()

	if adapter == nil {
		t.Fatal("NewInstallShAdapter returned nil")
	}
}
