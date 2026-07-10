package updates

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// The adapters in this file are plan providers only: they describe how an
// update would be performed for a deployment type (GET /api/updates/plan).
// The actual apply for every deployment runs through the in-Go pipeline in
// manager.go ApplyUpdate.

// InstallShAdapter describes install.sh-based updates for systemd/LXC deployments
type InstallShAdapter struct{}

// NewInstallShAdapter creates a new install.sh adapter
func NewInstallShAdapter() *InstallShAdapter {
	return &InstallShAdapter{}
}

// SupportsApply returns true for systemd and proxmoxve deployments
func (a *InstallShAdapter) SupportsApply() bool {
	return true
}

// GetDeploymentType returns the deployment type
func (a *InstallShAdapter) GetDeploymentType() string {
	return "systemd" // Can be "systemd" or "proxmoxve"
}

// PrepareUpdate returns update plan information
func (a *InstallShAdapter) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	plan := &UpdatePlan{
		CanAutoUpdate:   true,
		RequiresRoot:    true,
		RollbackSupport: true,
		EstimatedTime:   "2-5 minutes",
		Instructions: []string{
			fmt.Sprintf("Download and install Pulse %s", request.Version),
			"Create backup of current installation",
			"Extract and apply update",
			"Restart Pulse service",
		},
		Prerequisites: []string{
			"Root access (sudo)",
			"Internet connection",
			"About 1.2GB free disk space for update staging",
		},
	}

	return plan, nil
}

// DockerUpdater provides instructions for Docker deployments
type DockerUpdater struct{}

const defaultDockerImageRepo = "rcourtman/pulse"

// NewDockerUpdater creates an updater for Docker deployments.
func NewDockerUpdater() *DockerUpdater {
	return &DockerUpdater{}
}

func (u *DockerUpdater) SupportsApply() bool {
	return false
}

func (u *DockerUpdater) GetDeploymentType() string {
	return "docker"
}

func (u *DockerUpdater) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	imageRepo := strings.TrimSpace(os.Getenv("PULSE_DOCKER_IMAGE_REPO"))
	if imageRepo == "" {
		imageRepo = defaultDockerImageRepo
	}
	imageRef := fmt.Sprintf("%s:%s", imageRepo, strings.TrimPrefix(request.Version, "v"))
	return &UpdatePlan{
		CanAutoUpdate: false,
		Instructions: []string{
			fmt.Sprintf("docker pull %s", imageRef),
			"docker stop pulse",
			fmt.Sprintf("docker run -d --name pulse %s", imageRef),
		},
		RequiresRoot:    false,
		RollbackSupport: true,
		EstimatedTime:   "1-2 minutes",
	}, nil
}

// AURUpdater provides instructions for Arch Linux AUR deployments
type AURUpdater struct{}

// NewAURUpdater creates an updater for Arch Linux AUR deployments.
func NewAURUpdater() *AURUpdater {
	return &AURUpdater{}
}

func (u *AURUpdater) SupportsApply() bool {
	return false
}

func (u *AURUpdater) GetDeploymentType() string {
	return "aur"
}

func (u *AURUpdater) PrepareUpdate(ctx context.Context, request UpdateRequest) (*UpdatePlan, error) {
	return &UpdatePlan{
		CanAutoUpdate: false,
		Instructions: []string{
			"yay -Syu pulse-monitoring",
			"# or",
			"paru -Syu pulse-monitoring",
		},
		RequiresRoot:    false,
		RollbackSupport: false,
		EstimatedTime:   "1-2 minutes",
	}, nil
}

// Ensure adapters implement Updater interface
var (
	_ Updater = (*InstallShAdapter)(nil)
	_ Updater = (*DockerUpdater)(nil)
	_ Updater = (*AURUpdater)(nil)
)
