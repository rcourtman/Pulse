package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

// ManagerConfig holds Docker manager settings.
type ManagerConfig struct {
	Image         string
	Network       string
	BaseDomain    string
	MemoryLimit   int64 // bytes
	CPUShares     int64
	ContainerPort int // port inside the container (default 7655)
}

// Manager orchestrates Docker containers for tenant lifecycle.
type Manager struct {
	cli *client.Client
	cfg ManagerConfig
}

// NewManager creates a Docker manager connected to the local daemon.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	if cfg.ContainerPort == 0 {
		cfg.ContainerPort = 7655
	}
	return &Manager{cli: cli, cfg: cfg}, nil
}

// Close closes the Docker client.
func (m *Manager) Close() error {
	if m.cli != nil {
		return m.cli.Close()
	}
	return nil
}

// CreateAndStart creates and starts a tenant container.
// tenantDataDir is the host path that gets bind-mounted to /etc/pulse in the container.
func (m *Manager) CreateAndStart(ctx context.Context, tenantID, tenantDataDir string) (containerID string, err error) {
	labels := TraefikLabels(tenantID, m.cfg.BaseDomain, m.cfg.ContainerPort)
	labels["pulse.managed"] = "true"

	containerName := "pulse-" + tenantID

	resp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image:  m.cfg.Image,
			Labels: labels,
			Env: []string{
				"PULSE_DATA_DIR=/etc/pulse",
				"PULSE_HOSTED_MODE=true",
			},
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			Resources: container.Resources{
				Memory:    m.cfg.MemoryLimit,
				CPUShares: m.cfg.CPUShares,
			},
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: tenantDataDir,
					Target: "/etc/pulse",
				},
				{
					Type:     mount.TypeBind,
					Source:   filepath.Join(tenantDataDir, "billing.json"),
					Target:   "/etc/pulse/billing.json",
					ReadOnly: true,
				},
				{
					Type:     mount.TypeBind,
					Source:   filepath.Join(tenantDataDir, "secrets", "handoff.key"),
					Target:   "/etc/pulse/secrets/handoff.key",
					ReadOnly: true,
				},
				{
					Type:     mount.TypeBind,
					Source:   filepath.Join(tenantDataDir, ".cloud_handoff_key"),
					Target:   "/etc/pulse/.cloud_handoff_key",
					ReadOnly: true,
				},
			},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				m.cfg.Network: {},
			},
		},
		nil, // platform
		containerName,
	)
	if err != nil {
		return "", fmt.Errorf("create container for %s: %w", tenantID, err)
	}

	if err := m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return resp.ID, fmt.Errorf("start container for %s: %w", tenantID, err)
	}

	log.Info().
		Str("tenant_id", tenantID).
		Str("container_id", resp.ID[:12]).
		Str("container_name", containerName).
		Msg("Tenant container started")

	return resp.ID, nil
}

// Stop stops a tenant container gracefully.
func (m *Manager) Stop(ctx context.Context, containerID string) error {
	timeout := 30
	return m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// Remove removes a stopped tenant container.
func (m *Manager) Remove(ctx context.Context, containerID string) error {
	return m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
}

// StopAndRemove stops then removes a tenant container.
func (m *Manager) StopAndRemove(ctx context.Context, containerID string) error {
	if err := m.Stop(ctx, containerID); err != nil {
		log.Warn().Err(err).Str("container_id", containerID).Msg("Failed to stop container, forcing remove")
	}
	return m.Remove(ctx, containerID)
}

// HealthCheck performs an HTTP health check against a running container.
// It connects to the container's published port via the Docker network.
func (m *Manager) HealthCheck(ctx context.Context, containerID string) (bool, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("inspect container: %w", err)
	}

	if !inspect.State.Running {
		return false, nil
	}

	// Find the container's IP on our network
	netSettings, ok := inspect.NetworkSettings.Networks[m.cfg.Network]
	if !ok || netSettings.IPAddress == "" {
		return false, fmt.Errorf("container not connected to network %s", m.cfg.Network)
	}

	healthURL := fmt.Sprintf("http://%s:%d/api/health", netSettings.IPAddress, m.cfg.ContainerPort)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(healthURL)
	if err != nil {
		return false, nil // unreachable, not an error condition
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK, nil
}
