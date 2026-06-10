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
	IsolateTenantNetworks    bool
	TenantNetworkPrefix      string
	SupportContainerLabels   []string
	BaseDomain               string
	TrialActivationPublicKey string
	TrustedProxyCIDRs        []string
	// TenantDisplayName resolves a tenant's workspace display name at
	// container-create time; the value is injected as PULSE_TENANT_NAME so
	// alert webhook payloads carry a human-readable workspace label instead
	// of falling back to the tenant ID. Display-name changes after creation
	// apply on the next runtime rollout, which recreates the container with
	// freshly resolved env. May be nil.
	TenantDisplayName func(tenantID string) string
	TenantReportBrand TenantReportBrandConfig
	TenantRuntimeUID  int
	TenantRuntimeGID  int
	MemoryLimit       int64 // bytes
	CPUShares         int64
	TenantLogMaxSize  string
	TenantLogMaxFile  int
	ContainerPort     int // port inside the container (default 7655)
}

type TenantReportBrandConfig struct {
	DisplayName string
	LogoPath    string
	LogoBase64  string
	LogoFormat  string
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

type RuntimePrerequisiteOptions struct {
	PullImage bool
}

type RuntimePrerequisiteReport struct {
	OK                bool
	DockerReachable   bool
	NetworkName       string
	NetworkID         string
	NetworkOK         bool
	ImageRef          string
	ImageID           string
	ImageAvailable    bool
	ImagePulled       bool
	ImagePullRequired bool
	Failures          []string
}

const immutableOwnershipPathsEnv = "PULSE_IMMUTABLE_OWNERSHIP_PATHS"

const (
	tenantRuntimeUID        = 1000
	tenantRuntimeGID        = 1000
	defaultTenantLogMaxSize = "10m"
	defaultTenantLogMaxFile = 3
)

const (
	providerSupportTraefikLabel      = "pulse.provider-msp.role=traefik"
	providerSupportControlPlaneLabel = "pulse.provider-msp.role=control-plane"
	tenantRuntimeNetworkLabel        = "pulse.provider-msp.network"
	tenantRuntimeNetworkLabelValue   = "tenant"
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
	cfg.Network = strings.TrimSpace(cfg.Network)
	cfg.TenantNetworkPrefix = strings.TrimSpace(cfg.TenantNetworkPrefix)
	if cfg.IsolateTenantNetworks {
		if cfg.TenantNetworkPrefix == "" {
			if cfg.Network != "" {
				cfg.TenantNetworkPrefix = cfg.Network + "-tenant"
			} else {
				cfg.TenantNetworkPrefix = "pulse-tenant"
			}
		}
		if len(cfg.SupportContainerLabels) == 0 {
			cfg.SupportContainerLabels = []string{
				providerSupportTraefikLabel,
				providerSupportControlPlaneLabel,
			}
		}
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

func (m *Manager) CheckRuntimePrerequisites(ctx context.Context, opts RuntimePrerequisiteOptions) (*RuntimePrerequisiteReport, error) {
	if m == nil {
		return &RuntimePrerequisiteReport{
			OK:       false,
			Failures: []string{"docker manager is not initialized"},
		}, nil
	}
	report := &RuntimePrerequisiteReport{
		OK:          true,
		NetworkName: strings.TrimSpace(m.cfg.Network),
		ImageRef:    strings.TrimSpace(m.cfg.Image),
	}
	addFailure := func(format string, args ...any) {
		report.OK = false
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
	}

	if err := m.ensureDaemonReachable(ctx); err != nil {
		addFailure("%v", err)
		return report, nil
	}
	report.DockerReachable = true

	if report.NetworkName == "" {
		addFailure("tenant Docker network is required")
	} else {
		inspect, err := m.cli.NetworkInspect(ctx, report.NetworkName, client.NetworkInspectOptions{})
		if err != nil {
			addFailure("inspect tenant Docker network %q: %v", report.NetworkName, err)
		} else {
			report.NetworkOK = true
			report.NetworkID = strings.TrimSpace(inspect.Network.ID)
		}
	}

	image, pulled, err := m.ensureRuntimeImageAvailable(ctx, opts.PullImage)
	report.ImagePulled = pulled
	report.ImageID = image
	if err != nil {
		if !opts.PullImage && errdefs.IsNotFound(err) {
			report.ImagePullRequired = true
			addFailure("tenant runtime image %q is not present locally; rerun preflight with image pulling enabled or pull it before creating a workspace", report.ImageRef)
		} else {
			addFailure("%v", err)
		}
	} else {
		report.ImageAvailable = true
	}

	return report, nil
}

func (m *Manager) tenantNetworkName(tenantID string) string {
	if m == nil {
		return ""
	}
	prefix := strings.TrimSpace(m.cfg.TenantNetworkPrefix)
	if prefix == "" {
		prefix = strings.TrimSpace(m.cfg.Network)
	}
	if prefix == "" {
		prefix = "pulse-tenant"
	}
	return sanitizeDockerNameFragment(prefix + "-" + strings.TrimSpace(tenantID))
}

func sanitizeDockerNameFragment(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "pulse-tenant"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		allowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' || r == '-'
		if !allowed {
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		b.WriteRune(r)
		lastDash = r == '-'
	}
	out := strings.Trim(b.String(), "-_.")
	if out == "" {
		return "pulse-tenant"
	}
	return out
}

func (m *Manager) ensureTenantNetwork(ctx context.Context, tenantID string) (string, error) {
	if m == nil {
		return "", fmt.Errorf("docker manager unavailable")
	}
	if !m.cfg.IsolateTenantNetworks {
		networkName := strings.TrimSpace(m.cfg.Network)
		if networkName == "" {
			return "", fmt.Errorf("tenant Docker network is required")
		}
		return networkName, nil
	}

	networkName := m.tenantNetworkName(tenantID)
	inspect, err := m.cli.NetworkInspect(ctx, networkName, client.NetworkInspectOptions{})
	if err == nil {
		if got := strings.TrimSpace(inspect.Network.Labels["pulse.tenant.id"]); got != "" && got != tenantID {
			return "", fmt.Errorf("tenant network %q belongs to tenant %q, not %q", networkName, got, tenantID)
		}
		return networkName, nil
	}
	if !errdefs.IsNotFound(err) {
		return "", fmt.Errorf("inspect tenant network %q: %w", networkName, err)
	}

	_, err = m.cli.NetworkCreate(ctx, networkName, client.NetworkCreateOptions{
		Driver: "bridge",
		Labels: map[string]string{
			"pulse.managed":           "true",
			"pulse.tenant.id":         tenantID,
			tenantRuntimeNetworkLabel: tenantRuntimeNetworkLabelValue,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create tenant network %q: %w", networkName, err)
	}
	return networkName, nil
}

func (m *Manager) connectSupportContainersToTenantNetwork(ctx context.Context, networkName string) error {
	if m == nil || !m.cfg.IsolateTenantNetworks {
		return nil
	}
	for _, label := range m.cfg.SupportContainerLabels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		if err := m.connectSupportContainersByLabel(ctx, networkName, label); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) connectSupportContainersByLabel(ctx context.Context, networkName, label string) error {
	filters := client.Filters{}
	filters = filters.Add("label", label)
	result, err := m.cli.ContainerList(ctx, client.ContainerListOptions{
		Filters: filters,
	})
	if err != nil {
		return fmt.Errorf("list provider support containers for label %q: %w", label, err)
	}
	if len(result.Items) == 0 {
		return fmt.Errorf("provider support container with label %q is required for isolated tenant network %q", label, networkName)
	}

	for _, item := range result.Items {
		if item.ID == "" {
			continue
		}
		if err := m.ensureContainerConnectedToNetwork(ctx, networkName, item.ID); err != nil {
			return fmt.Errorf("connect provider support container %s to tenant network %q: %w", item.ID[:min(len(item.ID), 12)], networkName, err)
		}
	}
	return nil
}

func (m *Manager) ensureContainerConnectedToNetwork(ctx context.Context, networkName, containerID string) error {
	inspect, err := m.cli.NetworkInspect(ctx, networkName, client.NetworkInspectOptions{})
	if err != nil {
		return fmt.Errorf("inspect tenant network %q: %w", networkName, err)
	}
	for id := range inspect.Network.Containers {
		if id == containerID || strings.HasPrefix(id, containerID) || strings.HasPrefix(containerID, id) {
			return nil
		}
	}
	_, err = m.cli.NetworkConnect(ctx, networkName, client.NetworkConnectOptions{
		Container:      containerID,
		EndpointConfig: &network.EndpointSettings{},
	})
	return err
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

func (m *Manager) ensureRuntimeImageAvailable(ctx context.Context, pullIfMissing bool) (imageID string, pulled bool, err error) {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return "", false, err
	}
	imageRef := strings.TrimSpace(m.cfg.Image)
	if imageRef == "" {
		return "", false, fmt.Errorf("tenant runtime image is required")
	}
	inspect, err := m.cli.ImageInspect(ctx, imageRef)
	if err == nil {
		return strings.TrimSpace(inspect.ID), false, nil
	}
	if !errdefs.IsNotFound(err) {
		return "", false, fmt.Errorf("inspect tenant runtime image %q: %w", imageRef, err)
	}
	if !pullIfMissing {
		return "", false, err
	}
	rc, err := m.cli.ImagePull(ctx, imageRef, client.ImagePullOptions{})
	if err != nil {
		return "", false, fmt.Errorf("pull tenant runtime image %q: %w", imageRef, err)
	}
	defer func() {
		if closeErr := rc.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close tenant runtime image pull stream %q: %w", imageRef, closeErr)
		}
	}()
	if _, err := io.Copy(io.Discard, rc); err != nil {
		return "", true, fmt.Errorf("read tenant runtime image pull output %q: %w", imageRef, err)
	}
	inspect, err = m.cli.ImageInspect(ctx, imageRef)
	if err != nil {
		return "", true, fmt.Errorf("inspect pulled tenant runtime image %q: %w", imageRef, err)
	}
	return strings.TrimSpace(inspect.ID), true, nil
}

// CreateAndStart creates and starts a tenant container.
// tenantDataDir is the host path that gets bind-mounted to /etc/pulse in the container.
func (m *Manager) CreateAndStart(ctx context.Context, tenantID, tenantDataDir string) (containerID string, err error) {
	if err := m.ensureDaemonReachable(ctx); err != nil {
		return "", err
	}
	if _, _, err := m.ensureRuntimeImageAvailable(ctx, true); err != nil {
		return "", fmt.Errorf("prepare tenant runtime image: %w", err)
	}
	if err := prepareTenantRuntimeMountSources(tenantDataDir, tenantRuntimeUIDFor(m.cfg), tenantRuntimeGIDFor(m.cfg)); err != nil {
		return "", fmt.Errorf("prepare tenant runtime mounts for %s: %w", tenantID, err)
	}

	tenantNetworkName, err := m.ensureTenantNetwork(ctx, tenantID)
	if err != nil {
		return "", err
	}

	labels := TraefikLabels(tenantID, m.cfg.BaseDomain, m.cfg.ContainerPort, tenantNetworkName)
	labels["pulse.managed"] = "true"

	containerName := "pulse-" + tenantID

	resp, err := m.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:     tenantRuntimeContainerConfig(tenantID, m.cfg, labels, m.tenantTrustedProxyCIDRs(ctx, tenantNetworkName)),
		HostConfig: tenantRuntimeHostConfig(tenantDataDir, m.cfg),
		NetworkingConfig: &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				tenantNetworkName: {},
			},
		},
		Name: containerName,
	})
	if err != nil {
		return "", fmt.Errorf("create container for %s: %w", tenantID, err)
	}

	if err := m.connectSupportContainersToTenantNetwork(ctx, tenantNetworkName); err != nil {
		_, _ = m.cli.ContainerRemove(ctx, resp.ID, client.ContainerRemoveOptions{Force: true})
		return resp.ID, err
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

func tenantRuntimeContainerConfig(tenantID string, cfg ManagerConfig, labels map[string]string, trustedProxyCIDRs []string) *container.Config {
	return &container.Config{
		Image:  cfg.Image,
		User:   tenantRuntimeUserFor(cfg),
		Labels: labels,
		Env:    tenantEnvForRuntime(tenantID, tenantDisplayNameFor(cfg, tenantID), cfg.BaseDomain, cfg.TrialActivationPublicKey, trustedProxyCIDRs, tenantRuntimeUIDFor(cfg), tenantRuntimeGIDFor(cfg), cfg.TenantReportBrand),
	}
}

func tenantDisplayNameFor(cfg ManagerConfig, tenantID string) string {
	if cfg.TenantDisplayName == nil {
		return ""
	}
	return strings.TrimSpace(cfg.TenantDisplayName(tenantID))
}

func tenantRuntimeUserFor(cfg ManagerConfig) string {
	return fmt.Sprintf("%d:%d", tenantRuntimeUIDFor(cfg), tenantRuntimeGIDFor(cfg))
}

func tenantRuntimeUIDFor(cfg ManagerConfig) int {
	if cfg.TenantRuntimeUID > 0 {
		return cfg.TenantRuntimeUID
	}
	return tenantRuntimeUID
}

func tenantRuntimeGIDFor(cfg ManagerConfig) int {
	if cfg.TenantRuntimeGID > 0 {
		return cfg.TenantRuntimeGID
	}
	return tenantRuntimeGID
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

func tenantRuntimeHostConfig(tenantDataDir string, cfg ManagerConfig) *container.HostConfig {
	return &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
		LogConfig:     tenantRuntimeLogConfig(cfg.TenantLogMaxSize, cfg.TenantLogMaxFile),
		Resources: container.Resources{
			Memory:    cfg.MemoryLimit,
			CPUShares: cfg.CPUShares,
		},
		Mounts:         tenantMounts(tenantDataDir),
		SecurityOpt:    tenantRuntimeSecurityOptions(),
		CapDrop:        []string{"ALL"},
		ReadonlyRootfs: true,
		Tmpfs: map[string]string{
			"/run": "rw,noexec,nosuid,nodev,size=16m",
			"/tmp": "rw,noexec,nosuid,nodev,size=64m",
		},
	}
}

func tenantRuntimeSecurityOptions() []string {
	// Do not set seccomp=unconfined. Docker's default seccomp profile remains
	// active unless the daemon has been explicitly weakened outside Pulse.
	return []string{"no-new-privileges:true"}
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

func tenantEnv(tenantID, tenantName, baseDomain, trialActivationPublicKey string, trustedProxyCIDRs []string, reportBrand TenantReportBrandConfig) []string {
	return tenantEnvForRuntime(tenantID, tenantName, baseDomain, trialActivationPublicKey, trustedProxyCIDRs, tenantRuntimeUID, tenantRuntimeGID, reportBrand)
}

func tenantEnvForRuntime(tenantID, tenantName, baseDomain, trialActivationPublicKey string, trustedProxyCIDRs []string, runtimeUID, runtimeGID int, reportBrand TenantReportBrandConfig) []string {
	routing := CanonicalTenantRuntimeRouting(tenantID, baseDomain)
	tenantID = strings.TrimSpace(tenantID)

	env := []string{
		"PULSE_DATA_DIR=/etc/pulse",
		"PULSE_HOSTED_MODE=true",
		"PULSE_TENANT_ID=" + tenantID,
		"PULSE_MULTI_TENANT_ENABLED=true",
		fmt.Sprintf("PUID=%d", runtimeUID),
		fmt.Sprintf("PGID=%d", runtimeGID),
		fmt.Sprintf("%s=%s", immutableOwnershipPathsEnv, strings.Join(tenantImmutableOwnershipPaths(), ":")),
	}
	if name := strings.TrimSpace(tenantName); name != "" {
		env = append(env, "PULSE_TENANT_NAME="+name)
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
	if value := strings.TrimSpace(reportBrand.DisplayName); value != "" {
		env = append(env, "PULSE_REPORT_PROVIDER_BRAND_DISPLAY_NAME="+value)
	}
	if value := strings.TrimSpace(reportBrand.LogoPath); value != "" {
		env = append(env, "PULSE_REPORT_PROVIDER_BRAND_LOGO_PATH="+value)
	}
	if value := strings.TrimSpace(reportBrand.LogoBase64); value != "" {
		env = append(env, "PULSE_REPORT_PROVIDER_BRAND_LOGO_BASE64="+value)
	}
	if value := strings.TrimSpace(reportBrand.LogoFormat); value != "" {
		env = append(env, "PULSE_REPORT_PROVIDER_BRAND_LOGO_FORMAT="+value)
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

func (m *Manager) tenantTrustedProxyCIDRs(ctx context.Context, networkNames ...string) []string {
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

	if len(networkNames) == 0 && m != nil {
		networkNames = []string{m.cfg.Network}
	}
	for _, networkName := range networkNames {
		networkName = strings.TrimSpace(networkName)
		if m == nil || m.cli == nil || networkName == "" {
			continue
		}
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
	if err := filepath.WalkDir(tenantDataDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := os.Lchown(path, uid, gid); err != nil {
			return fmt.Errorf("chown %s to %d:%d: %w", path, uid, gid, err)
		}
		return nil
	}); err != nil {
		return err
	}

	for _, relPath := range tenantRuntimeOwnershipPaths() {
		path := filepath.Join(tenantDataDir, relPath)
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("tenant runtime mount source %s must not be a symlink", path)
		}
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
	networkNames := m.tenantNetworkNamesForContainer(ctx, containerID)
	_, err := m.cli.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{
		Force: true,
	})
	if err == nil {
		for _, networkName := range networkNames {
			if removeErr := m.removeTenantNetwork(ctx, networkName); removeErr != nil {
				log.Warn().Err(removeErr).Str("network", networkName).Msg("Failed to remove tenant runtime network")
			}
		}
	}
	return err
}

func (m *Manager) tenantNetworkNamesForContainer(ctx context.Context, containerID string) []string {
	if m == nil || !m.cfg.IsolateTenantNetworks || strings.TrimSpace(containerID) == "" {
		return nil
	}
	inspectResult, err := m.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return nil
	}
	inspect := inspectResult.Container
	if inspect.NetworkSettings == nil {
		return nil
	}
	var out []string
	for networkName := range inspect.NetworkSettings.Networks {
		if m.isTenantNetwork(ctx, networkName) {
			out = append(out, networkName)
		}
	}
	sort.Strings(out)
	return out
}

func (m *Manager) isTenantNetwork(ctx context.Context, networkName string) bool {
	networkName = strings.TrimSpace(networkName)
	if m == nil || networkName == "" {
		return false
	}
	inspect, err := m.cli.NetworkInspect(ctx, networkName, client.NetworkInspectOptions{})
	if err != nil {
		return false
	}
	return strings.TrimSpace(inspect.Network.Labels[tenantRuntimeNetworkLabel]) == tenantRuntimeNetworkLabelValue
}

func (m *Manager) removeTenantNetwork(ctx context.Context, networkName string) error {
	if m == nil || strings.TrimSpace(networkName) == "" {
		return nil
	}
	if err := m.disconnectSupportContainersFromTenantNetwork(ctx, networkName); err != nil {
		return err
	}
	if _, err := m.cli.NetworkRemove(ctx, networkName, client.NetworkRemoveOptions{}); err != nil {
		return fmt.Errorf("remove tenant network %q: %w", networkName, err)
	}
	return nil
}

func (m *Manager) disconnectSupportContainersFromTenantNetwork(ctx context.Context, networkName string) error {
	if m == nil || !m.cfg.IsolateTenantNetworks {
		return nil
	}
	for _, label := range m.cfg.SupportContainerLabels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		filters := client.Filters{}
		filters = filters.Add("label", label)
		result, err := m.cli.ContainerList(ctx, client.ContainerListOptions{
			All:     true,
			Filters: filters,
		})
		if err != nil {
			return fmt.Errorf("list provider support containers for disconnect label %q: %w", label, err)
		}
		for _, item := range result.Items {
			if item.ID == "" {
				continue
			}
			_, err := m.cli.NetworkDisconnect(ctx, networkName, client.NetworkDisconnectOptions{
				Container: item.ID,
				Force:     true,
			})
			if err != nil && !errdefs.IsNotFound(err) {
				return fmt.Errorf("disconnect provider support container %s from tenant network %q: %w", item.ID[:min(len(item.ID), 12)], networkName, err)
			}
		}
	}
	return nil
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

	networkNames := m.healthCheckNetworkCandidates(inspect)
	var netSettings *network.EndpointSettings
	for _, networkName := range networkNames {
		candidate, ok := inspect.NetworkSettings.Networks[networkName]
		if ok && candidate != nil && candidate.IPAddress.IsValid() {
			netSettings = candidate
			break
		}
	}
	if netSettings == nil {
		return false, fmt.Errorf("container not connected to tenant health network candidates %s", strings.Join(networkNames, ", "))
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

func (m *Manager) healthCheckNetworkCandidates(inspect container.InspectResponse) []string {
	var names []string
	seen := make(map[string]struct{})
	appendName := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	if m != nil && m.cfg.IsolateTenantNetworks && inspect.Config != nil {
		if tenantID := strings.TrimSpace(inspect.Config.Labels["pulse.tenant.id"]); tenantID != "" {
			appendName(m.tenantNetworkName(tenantID))
		}
	}
	if m != nil {
		appendName(m.cfg.Network)
	}
	if inspect.NetworkSettings != nil {
		for name := range inspect.NetworkSettings.Networks {
			appendName(name)
		}
	}
	return names
}
