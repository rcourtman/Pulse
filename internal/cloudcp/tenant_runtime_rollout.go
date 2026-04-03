package cloudcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rs/zerolog/log"
)

const (
	defaultTenantRuntimeRolloutHealthTimeout = 90 * time.Second
	defaultTenantRuntimeRolloutHealthPoll    = 2 * time.Second
)

// TenantRuntimeRolloutOptions controls the hosted tenant runtime rollout path.
type TenantRuntimeRolloutOptions struct {
	TenantID           string
	Image              string
	RunID              string
	SnapshotRoot       string
	HealthTimeout      time.Duration
	HealthPollInterval time.Duration
	PrunePrevious      bool
}

// TenantRuntimeRolloutResult summarizes the canonical runtime state after a
// rollout or drift-reconciliation pass.
type TenantRuntimeRolloutResult struct {
	TenantID            string
	PreviousContainerID string
	ActiveContainerID   string
	ActiveImageRef      string
	ActiveImageID       string
	BackupContainerName string
	ReconciledOnly      bool
}

type tenantRuntimeRolloutRegistry interface {
	Get(tenantID string) (*registry.Tenant, error)
	Update(t *registry.Tenant) error
}

type tenantRuntimeRolloutDocker interface {
	CreateAndStart(ctx context.Context, tenantID, tenantDataDir string) (string, error)
	HealthCheck(ctx context.Context, containerID string) (bool, error)
	Inspect(ctx context.Context, containerIDOrName string) (*cpDocker.RuntimeContainerInfo, error)
	Remove(ctx context.Context, containerID string) error
	Rename(ctx context.Context, containerIDOrName, newName string) error
	Start(ctx context.Context, containerID string) error
	Stop(ctx context.Context, containerID string) error
}

type tenantRuntimeRolloutSynchronizer interface {
	Snapshot(ctx context.Context, srcDir, snapshotDir string) error
	Restore(ctx context.Context, snapshotDir, dstDir string) error
}

type tenantRuntimeRolloutService struct {
	registry      tenantRuntimeRolloutRegistry
	docker        tenantRuntimeRolloutDocker
	tenantsDir    string
	synchronizer  tenantRuntimeRolloutSynchronizer
	now           func() time.Time
	sleep         func(time.Duration)
	healthTimeout time.Duration
	healthPoll    time.Duration
}

// RolloutTenantRuntime executes the canonical hosted tenant runtime rollout
// path using the control plane's registry and Docker manager.
func RolloutTenantRuntime(ctx context.Context, cfg *CPConfig, opts TenantRuntimeRolloutOptions) (*TenantRuntimeRolloutResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}

	image := strings.TrimSpace(opts.Image)
	if image == "" {
		image = strings.TrimSpace(cfg.PulseImage)
	}
	if image == "" {
		return nil, fmt.Errorf("missing tenant runtime image")
	}
	opts.Image = image

	if err := os.MkdirAll(cfg.TenantsDir(), 0o755); err != nil {
		return nil, fmt.Errorf("ensure tenants dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ControlPlaneDir(), 0o755); err != nil {
		return nil, fmt.Errorf("ensure control-plane dir: %w", err)
	}

	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}
	defer reg.Close()

	dockerMgr, err := cpDocker.NewManager(cpDocker.ManagerConfig{
		Image:                    image,
		Network:                  cfg.DockerNetwork,
		BaseDomain:               baseDomainFromURL(cfg.BaseURL),
		TrialActivationPublicKey: cfg.TrialActivationPublicKey,
		TrustedProxyCIDRs:        cfg.TrustedProxyCIDRs,
		MemoryLimit:              cfg.TenantMemoryLimit,
		CPUShares:                cfg.TenantCPUShares,
	})
	if err != nil {
		return nil, fmt.Errorf("create docker manager: %w", err)
	}
	defer dockerMgr.Close()

	service := &tenantRuntimeRolloutService{
		registry:      reg,
		docker:        dockerMgr,
		tenantsDir:    cfg.TenantsDir(),
		synchronizer:  rsyncTenantRuntimeSynchronizer{},
		now:           func() time.Time { return time.Now().UTC() },
		sleep:         time.Sleep,
		healthTimeout: defaultTenantRuntimeRolloutHealthTimeout,
		healthPoll:    defaultTenantRuntimeRolloutHealthPoll,
	}
	return service.Rollout(ctx, opts)
}

// Rollout performs the tenant runtime rollout or a drift reconciliation if the
// canonical live container is already on the requested image.
func (s *tenantRuntimeRolloutService) Rollout(ctx context.Context, opts TenantRuntimeRolloutOptions) (*TenantRuntimeRolloutResult, error) {
	if s == nil {
		return nil, fmt.Errorf("rollout service is nil")
	}
	tenantID := strings.TrimSpace(opts.TenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("tenant id is required")
	}
	image := strings.TrimSpace(opts.Image)
	if image == "" {
		return nil, fmt.Errorf("image is required")
	}

	tenant, err := s.registry.Get(tenantID)
	if err != nil {
		return nil, fmt.Errorf("load tenant %s: %w", tenantID, err)
	}
	if tenant == nil {
		return nil, fmt.Errorf("tenant %s not found", tenantID)
	}

	healthTimeout := opts.HealthTimeout
	if healthTimeout <= 0 {
		healthTimeout = s.healthTimeout
	}
	healthPoll := opts.HealthPollInterval
	if healthPoll <= 0 {
		healthPoll = s.healthPoll
	}
	runID := sanitizeTenantRuntimeRolloutRunID(opts.RunID, s.now())

	live, err := s.resolveLiveContainer(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(tenant.ContainerID) != "" && tenant.ContainerID != live.ID {
		log.Warn().
			Str("tenant_id", tenantID).
			Str("registry_container_id", tenant.ContainerID).
			Str("live_container_id", live.ID).
			Msg("Tenant registry container_id drifted from the live runtime container")
	}

	canonicalName := tenantRuntimeContainerName(tenantID)
	if live.Name == canonicalName && live.ImageRef == image {
		healthy, err := s.waitForHealth(ctx, live.ID, healthTimeout, healthPoll)
		if err == nil {
			if updateErr := s.persistTenantRuntimeState(tenant, live, healthy); updateErr != nil {
				return nil, updateErr
			}
			return &TenantRuntimeRolloutResult{
				TenantID:            tenantID,
				PreviousContainerID: live.ID,
				ActiveContainerID:   live.ID,
				ActiveImageRef:      live.ImageRef,
				ActiveImageID:       live.ImageID,
				ReconciledOnly:      true,
			}, nil
		}
		log.Warn().
			Err(err).
			Str("tenant_id", tenantID).
			Str("container_id", live.ID).
			Msg("Live tenant container already matches requested image but failed health sync; recreating canonically")
	}

	tenantDataDir := filepath.Join(s.tenantsDir, tenantID)
	snapshotRoot := strings.TrimSpace(opts.SnapshotRoot)
	if snapshotRoot == "" {
		snapshotRoot = filepath.Join(filepath.Dir(s.tenantsDir), "backups", "rollout")
	}
	snapshotDir := filepath.Join(snapshotRoot, runID, tenantID)
	if err := s.synchronizer.Snapshot(ctx, tenantDataDir, snapshotDir); err != nil {
		return nil, fmt.Errorf("snapshot tenant data for %s: %w", tenantID, err)
	}

	backupName := canonicalName + ".pre-" + runID
	if existing, inspectErr := s.docker.Inspect(ctx, backupName); inspectErr == nil && existing != nil {
		return nil, fmt.Errorf("backup container name %s already exists", backupName)
	} else if inspectErr != nil && !cpDocker.IsNotFound(inspectErr) {
		return nil, fmt.Errorf("inspect backup container %s: %w", backupName, inspectErr)
	}

	if err := s.docker.Stop(ctx, live.ID); err != nil {
		return nil, fmt.Errorf("stop live tenant container %s: %w", live.ID, err)
	}
	if err := s.docker.Rename(ctx, live.ID, backupName); err != nil {
		return nil, fmt.Errorf("rename live tenant container %s -> %s: %w", live.ID, backupName, err)
	}

	restorePrevious := func(newContainerID string, rolloutErr error) error {
		var restoreProblems []string
		if strings.TrimSpace(newContainerID) != "" {
			if removeErr := s.docker.Remove(ctx, newContainerID); removeErr != nil {
				restoreProblems = append(restoreProblems, fmt.Sprintf("remove failed container: %v", removeErr))
			}
		}
		if syncErr := s.synchronizer.Restore(ctx, snapshotDir, tenantDataDir); syncErr != nil {
			restoreProblems = append(restoreProblems, fmt.Sprintf("restore tenant data: %v", syncErr))
		}
		if renameErr := s.docker.Rename(ctx, backupName, canonicalName); renameErr != nil {
			restoreProblems = append(restoreProblems, fmt.Sprintf("rename backup container back: %v", renameErr))
		}
		if startErr := s.docker.Start(ctx, canonicalName); startErr != nil {
			restoreProblems = append(restoreProblems, fmt.Sprintf("restart previous container: %v", startErr))
		}
		restoredInfo, inspectErr := s.resolveLiveContainer(ctx, tenant)
		if inspectErr != nil {
			restoreProblems = append(restoreProblems, fmt.Sprintf("inspect restored container: %v", inspectErr))
		} else {
			healthy, healthErr := s.waitForHealth(ctx, restoredInfo.ID, healthTimeout, healthPoll)
			if healthErr != nil {
				restoreProblems = append(restoreProblems, fmt.Sprintf("verify restored container health: %v", healthErr))
			}
			if updateErr := s.persistTenantRuntimeState(tenant, restoredInfo, healthy); updateErr != nil {
				restoreProblems = append(restoreProblems, fmt.Sprintf("persist restored tenant state: %v", updateErr))
			}
		}
		if len(restoreProblems) == 0 {
			return rolloutErr
		}
		return fmt.Errorf("%w; rollback problems: %s", rolloutErr, strings.Join(restoreProblems, "; "))
	}

	newContainerID, err := s.docker.CreateAndStart(ctx, tenantID, tenantDataDir)
	if err != nil {
		return nil, restorePrevious("", fmt.Errorf("start new tenant runtime: %w", err))
	}

	healthy, err := s.waitForHealth(ctx, newContainerID, healthTimeout, healthPoll)
	if err != nil || !healthy {
		if err == nil {
			err = fmt.Errorf("tenant runtime %s failed health checks", newContainerID)
		}
		return nil, restorePrevious(newContainerID, err)
	}

	newInfo, err := s.resolveLiveContainer(ctx, tenant)
	if err != nil {
		return nil, restorePrevious(newContainerID, fmt.Errorf("inspect new tenant runtime: %w", err))
	}
	if newInfo.Name != canonicalName {
		return nil, restorePrevious(newContainerID, fmt.Errorf("new tenant runtime is not using canonical container name %s", canonicalName))
	}
	if err := s.persistTenantRuntimeState(tenant, newInfo, true); err != nil {
		return nil, restorePrevious(newContainerID, err)
	}
	if opts.PrunePrevious {
		if err := s.docker.Remove(ctx, backupName); err != nil {
			return nil, fmt.Errorf("remove previous tenant runtime %s: %w", backupName, err)
		}
		backupName = ""
	}

	return &TenantRuntimeRolloutResult{
		TenantID:            tenantID,
		PreviousContainerID: live.ID,
		ActiveContainerID:   newInfo.ID,
		ActiveImageRef:      newInfo.ImageRef,
		ActiveImageID:       newInfo.ImageID,
		BackupContainerName: backupName,
		ReconciledOnly:      false,
	}, nil
}

func (s *tenantRuntimeRolloutService) resolveLiveContainer(ctx context.Context, tenant *registry.Tenant) (*cpDocker.RuntimeContainerInfo, error) {
	if tenant == nil {
		return nil, fmt.Errorf("tenant is nil")
	}
	canonicalName := tenantRuntimeContainerName(tenant.ID)
	info, err := s.docker.Inspect(ctx, canonicalName)
	if err == nil {
		return info, nil
	}
	if !cpDocker.IsNotFound(err) {
		return nil, fmt.Errorf("inspect canonical tenant container %s: %w", canonicalName, err)
	}

	containerID := strings.TrimSpace(tenant.ContainerID)
	if containerID == "" {
		return nil, fmt.Errorf("tenant %s has no canonical runtime container and no registry container_id", tenant.ID)
	}
	info, err = s.docker.Inspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspect tenant container %s: %w", containerID, err)
	}
	return info, nil
}

func (s *tenantRuntimeRolloutService) waitForHealth(ctx context.Context, containerID string, timeout, poll time.Duration) (bool, error) {
	if timeout <= 0 {
		timeout = defaultTenantRuntimeRolloutHealthTimeout
	}
	if poll <= 0 {
		poll = defaultTenantRuntimeRolloutHealthPoll
	}
	deadline := s.now().Add(timeout)
	for {
		healthy, err := s.docker.HealthCheck(ctx, containerID)
		if err == nil && healthy {
			return true, nil
		}
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		if !s.now().Before(deadline) {
			if err != nil {
				return false, fmt.Errorf("container %s did not become healthy before timeout: %w", containerID, err)
			}
			return false, fmt.Errorf("container %s did not become healthy before timeout", containerID)
		}
		s.sleep(poll)
	}
}

func (s *tenantRuntimeRolloutService) persistTenantRuntimeState(tenant *registry.Tenant, info *cpDocker.RuntimeContainerInfo, healthy bool) error {
	if tenant == nil {
		return fmt.Errorf("tenant is nil")
	}
	if info == nil {
		return fmt.Errorf("runtime container info is nil")
	}
	now := s.now()
	tenant.ContainerID = info.ID
	tenant.CurrentImageDigest = info.ImageID
	tenant.DesiredImageDigest = info.ImageID
	tenant.LastHealthCheck = &now
	tenant.HealthCheckOK = healthy
	if err := s.registry.Update(tenant); err != nil {
		return fmt.Errorf("update tenant %s runtime registry state: %w", tenant.ID, err)
	}
	return nil
}

func tenantRuntimeContainerName(tenantID string) string {
	return "pulse-" + strings.TrimSpace(tenantID)
}

func sanitizeTenantRuntimeRolloutRunID(raw string, now time.Time) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = now.UTC().Format("20060102T150405Z")
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		return now.UTC().Format("20060102T150405Z")
	}
	return sanitized
}

type rsyncTenantRuntimeSynchronizer struct{}

func (rsyncTenantRuntimeSynchronizer) Snapshot(ctx context.Context, srcDir, snapshotDir string) error {
	return runTenantRuntimeRsync(ctx, srcDir, snapshotDir)
}

func (rsyncTenantRuntimeSynchronizer) Restore(ctx context.Context, snapshotDir, dstDir string) error {
	return runTenantRuntimeRsync(ctx, snapshotDir, dstDir)
}

func runTenantRuntimeRsync(ctx context.Context, srcDir, dstDir string) error {
	srcDir = strings.TrimSpace(srcDir)
	dstDir = strings.TrimSpace(dstDir)
	if srcDir == "" || dstDir == "" {
		return fmt.Errorf("source and destination directories are required")
	}
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("create destination dir %s: %w", dstDir, err)
	}
	cmd := exec.CommandContext(
		ctx,
		"rsync",
		"-a",
		"--delete",
		"--numeric-ids",
		withTrailingSeparator(srcDir),
		withTrailingSeparator(dstDir),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return fmt.Errorf("rsync %s -> %s: %w", srcDir, dstDir, err)
		}
		return fmt.Errorf("rsync %s -> %s: %w: %s", srcDir, dstDir, err, trimmed)
	}
	return nil
}

func withTrailingSeparator(path string) string {
	cleaned := filepath.Clean(path)
	if strings.HasSuffix(cleaned, string(os.PathSeparator)) {
		return cleaned
	}
	return cleaned + string(os.PathSeparator)
}
