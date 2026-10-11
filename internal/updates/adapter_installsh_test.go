package updates

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
		if got := plan.Instructions[0]; got != "docker pull 'rcourtman/pulse:4.25.0'" {
			t.Fatalf("pull instruction = %q", got)
		}
	})

	t.Run("PrepareUpdate uses configured image repo", func(t *testing.T) {
		t.Setenv("PULSE_DOCKER_IMAGE_REPO", "example/pulse-enterprise")

		plan, err := updater.PrepareUpdate(context.Background(), UpdateRequest{Version: "v4.25.0"})
		if err != nil {
			t.Fatalf("PrepareUpdate() error = %v", err)
		}
		if got := plan.Instructions[0]; got != "docker pull 'example/pulse-enterprise:4.25.0'" {
			t.Fatalf("pull instruction = %q", got)
		}
	})
}

func TestDockerUpdatePlanPreservesSavedDeployment(t *testing.T) {
	t.Setenv("PULSE_DOCKER_IMAGE_REPO", "")
	plan, err := NewDockerUpdater().PrepareUpdate(context.Background(), UpdateRequest{Version: "v6.5.0"})
	if err != nil {
		t.Fatal(err)
	}
	if plan.CanAutoUpdate || plan.RequiresRoot || !plan.RollbackSupport {
		t.Fatalf("manual Docker plan capabilities changed: %+v", plan)
	}
	// Only the image pull is a shell recipe. We do not know the operator's
	// service name, Compose project, mounts or recreation/recovery procedure.
	for _, instruction := range plan.Instructions[1:] {
		if strings.HasPrefix(instruction, "docker ") || strings.Contains(instruction, "--name pulse") {
			t.Fatalf("plan guesses a deployment mutation: %q", instruction)
		}
	}
	text := strings.Join(plan.Instructions, "\n")
	for _, required := range []string{
		"consistent private backup", "do not delete volumes", "original Compose file",
		"saved docker run command", "container manager", "same data mounts, ports, environment and restart policy",
		"actual service or container name", "pull failed, leave the running container unchanged",
		"Do not start a second Pulse", "running image, Pulse version and health",
		"does not undo data migrations",
	} {
		if !strings.Contains(text, required) {
			t.Errorf("Docker plan omits %q", required)
		}
	}
	if len(plan.Prerequisites) != 2 || !strings.Contains(plan.Prerequisites[0], "original saved deployment") {
		t.Fatalf("saved deployment prerequisite absent: %v", plan.Prerequisites)
	}
}

func TestDockerUpdatePullInstructionIsLiteral(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the copied Docker plan is a POSIX shell recipe")
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, repository, version string }{
		{"stable", "", "v6.5.0"},
		{"preview", "registry.example:5000/team/pulse", "6.6.0-rc.1"},
		{"blank-override", "  ", "v6.5.0"},
		{"version-shell-syntax", "", "v6.5.0; printf unexpected > \"$DOCKER_PLAN_SENTINEL\" #"},
		{"repository-shell-syntax", "example/pulse'; printf unexpected > \"$DOCKER_PLAN_SENTINEL\"; #", "v6.5.0"},
		{"substitution", "$(printf unexpected > \"$DOCKER_PLAN_SENTINEL\")/pulse", "v6.5.0"},
		{"multiline", "example/pulse", "v6.5.0\nprintf unexpected > \"$DOCKER_PLAN_SENTINEL\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PULSE_DOCKER_IMAGE_REPO", tc.repository)
			plan, err := NewDockerUpdater().PrepareUpdate(context.Background(), UpdateRequest{Version: tc.version})
			if err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			capture, sentinel := filepath.Join(directory, "arguments"), filepath.Join(directory, "unexpected")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			// This function replaces Docker entirely. No daemon, socket or
			// external image is used; the actual shell parses the copied text.
			cmd := exec.CommandContext(ctx, bash, "-c", `
docker() { printf '%s\0' "$@" > "$DOCKER_PLAN_CAPTURE"; }
eval "$DOCKER_PLAN_COMMAND"
`)
			cmd.Env = append(os.Environ(), "DOCKER_PLAN_COMMAND="+plan.Instructions[0],
				"DOCKER_PLAN_CAPTURE="+capture, "DOCKER_PLAN_SENTINEL="+sentinel)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("copied pull failed: %v: %s", err, output)
			}
			repository := strings.TrimSpace(tc.repository)
			if repository == "" {
				repository = defaultDockerImageRepo
			}
			want := "pull\x00" + repository + ":" + strings.TrimPrefix(tc.version, "v") + "\x00"
			got, err := os.ReadFile(capture)
			if err != nil || string(got) != want {
				t.Errorf("copied pull arguments = %q, %v; want %q", got, err, want)
			}
			if _, err := os.Lstat(sentinel); !os.IsNotExist(err) {
				t.Errorf("copied pull executed embedded shell syntax: %v", err)
			}
		})
	}
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
