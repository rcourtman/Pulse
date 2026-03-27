package docker

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/rs/zerolog/log"
)

// ManagerConfig holds Docker manager settings.
type ManagerConfig struct {
	Image             string
	Network           string
	BaseDomain        string
	TrustedProxyCIDRs []string
	MemoryLimit       int64 // bytes
	CPUShares         int64
	ContainerPort     int // port inside the container (default 7655)
}

// Manager orchestrates Docker containers for tenant lifecycle.
type Manager struct {
	cli *client.Client
	cfg ManagerConfig
}

const immutableOwnershipPathsEnv = "PULSE_IMMUTABLE_OWNERSHIP_PATHS"

const (
	tenantRuntimeUID = 1000
	tenantRuntimeGID = 1000
)

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

func (m *Manager) ensureDaemonReachable(ctx context.Context) error {
	if m == nil || m.cli == nil {
		return fmt.Errorf("docker client unavailable")
	}
	if _, err := m.cli.Ping(ctx); err != nil {
		return fmt.Errorf("ping docker daemon: %w", err)
	}
	return nil
}

// CreateAndStart creates and starts a tenant container.
// tenantDataDir is the host path that gets bind-mounted to /etc/pulse in the container.
func (m *Manager) CreateAndStart(ctx context.Context, tenantID, tenantDataDir string) (containerID string, err error) {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return "", err
	}
	if err := prepareTenantRuntimeMountSources(tenantDataDir, tenantRuntimeUID, tenantRuntimeGID); err != nil {
		return "", fmt.Errorf("prepare tenant runtime mounts for %s: %w", tenantID, err)
	}

	labels := TraefikLabels(tenantID, m.cfg.BaseDomain, m.cfg.ContainerPort)
	labels["pulse.managed"] = "true"

	containerName := "pulse-" + tenantID

	resp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image:  m.cfg.Image,
			Labels: labels,
			Env:    tenantEnv(tenantID, m.cfg.BaseDomain, m.tenantTrustedProxyCIDRs(ctx)),
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			Resources: container.Resources{
				Memory:    m.cfg.MemoryLimit,
				CPUShares: m.cfg.CPUShares,
			},
			Mounts: tenantMounts(tenantDataDir),
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

func tenantImmutableOwnershipPaths() []string {
	return []string{
		"/etc/pulse/secrets/handoff.key",
		"/etc/pulse/.cloud_handoff_key",
	}
}

func tenantRuntimeOwnershipPaths() []string {
	return []string{
		"billing.json",
		filepath.Join("secrets", "handoff.key"),
		".cloud_handoff_key",
	}
}

func tenantEnv(tenantID, baseDomain string, trustedProxyCIDRs []string) []string {
	publicURL := ""
	tenantID = strings.TrimSpace(tenantID)
	baseDomain = strings.TrimSpace(baseDomain)
	if tenantID != "" && baseDomain != "" {
		publicURL = fmt.Sprintf("https://%s.%s", tenantID, baseDomain)
	}

	env := []string{
		"PULSE_DATA_DIR=/etc/pulse",
		"PULSE_HOSTED_MODE=true",
		"PULSE_TENANT_ID=" + tenantID,
		"PULSE_MULTI_TENANT_ENABLED=true",
		fmt.Sprintf("PUID=%d", tenantRuntimeUID),
		fmt.Sprintf("PGID=%d", tenantRuntimeGID),
		fmt.Sprintf("%s=%s", immutableOwnershipPathsEnv, strings.Join(tenantImmutableOwnershipPaths(), ":")),
	}
	if publicURL != "" {
		env = append(env, "PULSE_PUBLIC_URL="+publicURL)
	}
	if len(trustedProxyCIDRs) > 0 {
		env = append(env, "PULSE_TRUSTED_PROXY_CIDRS="+strings.Join(trustedProxyCIDRs, ","))
	}
	return env
}

func (m *Manager) tenantTrustedProxyCIDRs(ctx context.Context) []string {
	values := make([]string, 0, len(m.cfg.TrustedProxyCIDRs)+1)
	seen := make(map[string]struct{})
	appendCIDR := func(raw string) {
		canonical := canonicalTrustedProxyCIDR(raw)
		if canonical == "" {
			return
		}
		if _, ok := seen[canonical]; ok {
			return
		}
		seen[canonical] = struct{}{}
		values = append(values, canonical)
	}

	for _, cidr := range m.cfg.TrustedProxyCIDRs {
		appendCIDR(cidr)
	}

	if m != nil && m.cli != nil && strings.TrimSpace(m.cfg.Network) != "" {
		networkName := strings.TrimSpace(m.cfg.Network)
		inspect, err := m.cli.NetworkInspect(ctx, networkName, network.InspectOptions{})
		if err != nil {
			log.Warn().Err(err).Str("network", networkName).Msg("Failed to inspect tenant network for trusted proxy CIDRs")
		} else {
			for _, cfg := range inspect.IPAM.Config {
				appendCIDR(cfg.Subnet)
			}
		}
	}

	sort.Strings(values)
	return values
}

func canonicalTrustedProxyCIDR(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if prefix, err := netip.ParsePrefix(raw); err == nil {
		return prefix.Masked().String()
	}
	if addr, err := netip.ParseAddr(raw); err == nil {
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		return netip.PrefixFrom(addr, bits).Masked().String()
	}
	return ""
}

func tenantMounts(tenantDataDir string) []mount.Mount {
	return []mount.Mount{
		{
			Type:   mount.TypeBind,
			Source: tenantDataDir,
			Target: "/etc/pulse",
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
	}
}

func prepareTenantRuntimeMountSources(tenantDataDir string, uid, gid int) error {
	for _, relPath := range tenantRuntimeOwnershipPaths() {
		path := filepath.Join(tenantDataDir, relPath)
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
		if err := os.Chown(path, uid, gid); err != nil {
			return fmt.Errorf("chown %s to %d:%d: %w", path, uid, gid, err)
		}
	}
	return nil
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
