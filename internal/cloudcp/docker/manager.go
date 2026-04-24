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

	"github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/rs/zerolog/log"
)

// ManagerConfig holds Docker manager settings.
type ManagerConfig struct {
	Image                    string
	Network                  string
	BaseDomain               string
	TrialActivationPublicKey string
	TrustedProxyCIDRs        []string
	MemoryLimit              int64 // bytes
	CPUShares                int64
	TenantLogMaxSize         string
	TenantLogMaxFile         int
	ContainerPort            int // port inside the container (default 7655)
}

// Manager orchestrates Docker containers for tenant lifecycle.
type Manager struct {
	cli *client.Client
	cfg ManagerConfig
}

// RuntimeContainerInfo is the canonical live-container snapshot used by
// operator rollout and reconciliation flows.
type RuntimeContainerInfo struct {
	ID        string
	Name      string
	ImageRef  string
	ImageID   string
	Running   bool
	RouteHost string
	PublicURL string
}

type DiskUsageClass struct {
	ActiveCount int64
	TotalCount  int64
	Reclaimable int64
	TotalSize   int64
}

type DiskUsageSnapshot struct {
	Images     DiskUsageClass
	Containers DiskUsageClass
	BuildCache DiskUsageClass
	Volumes    DiskUsageClass
}

type RuntimeContainerSummary struct {
	ID           string
	Name         string
	Image        string
	ImageID      string
	State        string
	Status       string
	HealthStatus string
	Created      int64
}

const immutableOwnershipPathsEnv = "PULSE_IMMUTABLE_OWNERSHIP_PATHS"

const (
	tenantRuntimeUID        = 1000
	tenantRuntimeGID        = 1000
	defaultTenantLogMaxSize = "10m"
	defaultTenantLogMaxFile = 3
)

// NewManager creates a Docker manager connected to the local daemon.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	cli, err := client.New(client.FromEnv)
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

// DesiredRuntimeRouting returns the canonical hosted route/public URL contract
// for a tenant on this manager's configured base domain.
func (m *Manager) DesiredRuntimeRouting(tenantID string) TenantRuntimeRoutingContract {
	if m == nil {
		return TenantRuntimeRoutingContract{}
	}
	return CanonicalTenantRuntimeRouting(tenantID, m.cfg.BaseDomain)
}

func (m *Manager) DiskUsage(ctx context.Context) (*DiskUsageSnapshot, error) {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return nil, err
	}
	usage, err := m.cli.DiskUsage(ctx, client.DiskUsageOptions{
		Containers: true,
		Images:     true,
		BuildCache: true,
		Volumes:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("read docker disk usage: %w", err)
	}
	return &DiskUsageSnapshot{
		Images: DiskUsageClass{
			ActiveCount: usage.Images.ActiveCount,
			TotalCount:  usage.Images.TotalCount,
			Reclaimable: usage.Images.Reclaimable,
			TotalSize:   usage.Images.TotalSize,
		},
		Containers: DiskUsageClass{
			ActiveCount: usage.Containers.ActiveCount,
			TotalCount:  usage.Containers.TotalCount,
			Reclaimable: usage.Containers.Reclaimable,
			TotalSize:   usage.Containers.TotalSize,
		},
		BuildCache: DiskUsageClass{
			ActiveCount: usage.BuildCache.ActiveCount,
			TotalCount:  usage.BuildCache.TotalCount,
			Reclaimable: usage.BuildCache.Reclaimable,
			TotalSize:   usage.BuildCache.TotalSize,
		},
		Volumes: DiskUsageClass{
			ActiveCount: usage.Volumes.ActiveCount,
			TotalCount:  usage.Volumes.TotalCount,
			Reclaimable: usage.Volumes.Reclaimable,
			TotalSize:   usage.Volumes.TotalSize,
		},
	}, nil
}

func (m *Manager) ListManagedRuntimeContainers(ctx context.Context) ([]RuntimeContainerSummary, error) {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return nil, err
	}
	filters := client.Filters{}
	filters = filters.Add("label", "pulse.managed=true")
	result, err := m.cli.ContainerList(ctx, client.ContainerListOptions{
		All:     true,
		Filters: filters,
	})
	if err != nil {
		return nil, fmt.Errorf("list managed tenant containers: %w", err)
	}
	containers := make([]RuntimeContainerSummary, 0, len(result.Items))
	for _, item := range result.Items {
		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(strings.TrimSpace(item.Names[0]), "/")
		}
		healthStatus := string(container.NoHealthcheck)
		if item.Health != nil {
			healthStatus = string(item.Health.Status)
		}
		containers = append(containers, RuntimeContainerSummary{
			ID:           item.ID,
			Name:         name,
			Image:        item.Image,
			ImageID:      item.ImageID,
			State:        string(item.State),
			Status:       item.Status,
			HealthStatus: healthStatus,
			Created:      item.Created,
		})
	}
	return containers, nil
}

// IsNotFound reports whether Docker treated an identifier as missing.
func IsNotFound(err error) bool {
	return errdefs.IsNotFound(err)
}

func (m *Manager) ensureDaemonReachable(ctx context.Context) error {
	if m == nil || m.cli == nil {
		return fmt.Errorf("docker client unavailable")
	}
	if _, err := m.cli.Ping(ctx, client.PingOptions{}); err != nil {
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

	resp, err := m.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image:  m.cfg.Image,
			Labels: labels,
			Env:    tenantEnv(tenantID, m.cfg.BaseDomain, m.cfg.TrialActivationPublicKey, m.tenantTrustedProxyCIDRs(ctx)),
		},
		HostConfig: &container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			LogConfig:     tenantRuntimeLogConfig(m.cfg.TenantLogMaxSize, m.cfg.TenantLogMaxFile),
			Resources: container.Resources{
				Memory:    m.cfg.MemoryLimit,
				CPUShares: m.cfg.CPUShares,
			},
			Mounts: tenantMounts(tenantDataDir),
		},
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				m.cfg.Network: {},
			},
		},
		Name: containerName,
	})
	if err != nil {
		return "", fmt.Errorf("create container for %s: %w", tenantID, err)
	}

	if _, err := m.cli.ContainerStart(ctx, resp.ID, client.ContainerStartOptions{}); err != nil {
		return resp.ID, fmt.Errorf("start container for %s: %w", tenantID, err)
	}

	log.Info().
		Str("tenant_id", tenantID).
		Str("container_id", resp.ID[:12]).
		Str("container_name", containerName).
		Msg("Tenant container started")

	return resp.ID, nil
}

func tenantRuntimeLogConfig(maxSize string, maxFile int) container.LogConfig {
	maxSize = strings.TrimSpace(maxSize)
	if maxSize == "" {
		maxSize = defaultTenantLogMaxSize
	}
	if maxFile <= 0 {
		maxFile = defaultTenantLogMaxFile
	}
	return container.LogConfig{
		Type: "json-file",
		Config: map[string]string{
			"max-size": maxSize,
			"max-file": fmt.Sprintf("%d", maxFile),
		},
	}
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

func tenantEnv(tenantID, baseDomain, trialActivationPublicKey string, trustedProxyCIDRs []string) []string {
	routing := CanonicalTenantRuntimeRouting(tenantID, baseDomain)
	tenantID = strings.TrimSpace(tenantID)

	env := []string{
		"PULSE_DATA_DIR=/etc/pulse",
		"PULSE_HOSTED_MODE=true",
		"PULSE_TENANT_ID=" + tenantID,
		"PULSE_MULTI_TENANT_ENABLED=true",
		fmt.Sprintf("PUID=%d", tenantRuntimeUID),
		fmt.Sprintf("PGID=%d", tenantRuntimeGID),
		fmt.Sprintf("%s=%s", immutableOwnershipPathsEnv, strings.Join(tenantImmutableOwnershipPaths(), ":")),
	}
	if routing.PublicURL != "" {
		env = append(env, "PULSE_PUBLIC_URL="+routing.PublicURL)
	}
	if strings.TrimSpace(trialActivationPublicKey) != "" {
		env = append(env, "PULSE_TRIAL_ACTIVATION_PUBLIC_KEY="+strings.TrimSpace(trialActivationPublicKey))
	}
	if len(trustedProxyCIDRs) > 0 {
		env = append(env, "PULSE_TRUSTED_PROXY_CIDRS="+strings.Join(trustedProxyCIDRs, ","))
	}
	return env
}

func routeHostFromLabels(labels map[string]string) string {
	for key, value := range labels {
		if !strings.HasPrefix(key, "traefik.http.routers.") || !strings.HasSuffix(key, ".rule") {
			continue
		}
		if host := parseTraefikHostRule(value); host != "" {
			return host
		}
	}
	return ""
}

func parseTraefikHostRule(rule string) string {
	rule = strings.TrimSpace(rule)
	if !strings.HasPrefix(rule, "Host(`") || !strings.HasSuffix(rule, "`)") {
		return ""
	}
	return strings.TrimSuffix(strings.TrimPrefix(rule, "Host(`"), "`)")
}

func envValue(env []string, key string) string {
	prefix := key + "="
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			continue
		}
		return strings.TrimPrefix(item, prefix)
	}
	return ""
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
		inspect, err := m.cli.NetworkInspect(ctx, networkName, client.NetworkInspectOptions{})
		if err != nil {
			log.Warn().Err(err).Str("network", networkName).Msg("Failed to inspect tenant network for trusted proxy CIDRs")
		} else {
			for _, cfg := range inspect.Network.IPAM.Config {
				appendCIDR(cfg.Subnet.String())
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
	_, err := m.cli.ContainerStop(ctx, containerID, client.ContainerStopOptions{Timeout: &timeout})
	return err
}

// Start starts a stopped tenant container.
func (m *Manager) Start(ctx context.Context, containerID string) error {
	_, err := m.cli.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	return err
}

// Remove removes a stopped tenant container.
func (m *Manager) Remove(ctx context.Context, containerID string) error {
	_, err := m.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		Force: true,
	})
	return err
}

// Rename renames a tenant container to the supplied Docker name.
func (m *Manager) Rename(ctx context.Context, containerIDOrName, newName string) error {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return err
	}
	if _, err := m.cli.ContainerRename(ctx, containerIDOrName, client.ContainerRenameOptions{NewName: newName}); err != nil {
		return fmt.Errorf("rename container %s -> %s: %w", containerIDOrName, newName, err)
	}
	return nil
}

// Inspect returns the canonical live metadata for the supplied container
// identifier.
func (m *Manager) Inspect(ctx context.Context, containerIDOrName string) (*RuntimeContainerInfo, error) {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return nil, err
	}
	inspectResult, err := m.cli.ContainerInspect(ctx, containerIDOrName, client.ContainerInspectOptions{})
	if err != nil {
		return nil, fmt.Errorf("inspect container %s: %w", containerIDOrName, err)
	}
	inspect := inspectResult.Container
	imageRef := ""
	if inspect.Config != nil {
		imageRef = strings.TrimSpace(inspect.Config.Image)
	}
	running := inspect.State != nil && inspect.State.Running
	routeHost := ""
	publicURL := ""
	if inspect.Config != nil {
		routeHost = routeHostFromLabels(inspect.Config.Labels)
		publicURL = envValue(inspect.Config.Env, "PULSE_PUBLIC_URL")
	}
	return &RuntimeContainerInfo{
		ID:        inspect.ID,
		Name:      strings.TrimPrefix(strings.TrimSpace(inspect.Name), "/"),
		ImageRef:  imageRef,
		ImageID:   strings.TrimSpace(inspect.Image),
		Running:   running,
		RouteHost: routeHost,
		PublicURL: publicURL,
	}, nil
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
	inspectResult, err := m.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return false, fmt.Errorf("inspect container: %w", err)
	}
	inspect := inspectResult.Container

	if !inspect.State.Running {
		return false, nil
	}

	// Find the container's IP on our network
	netSettings, ok := inspect.NetworkSettings.Networks[m.cfg.Network]
	if !ok || !netSettings.IPAddress.IsValid() {
		return false, fmt.Errorf("container not connected to network %s", m.cfg.Network)
	}

	healthURL := fmt.Sprintf("http://%s:%d/api/health", netSettings.IPAddress.String(), m.cfg.ContainerPort)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Get(healthURL)
	if err != nil {
		return false, nil // unreachable, not an error condition
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK, nil
}
