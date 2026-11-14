package cloudcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/lifecycle"
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
	RestoredMissing     bool
}

type TenantRuntimeContractReconcilePlanOptions struct {
	TenantIDs []string
	All       bool
}

type TenantRuntimeContractReconcilePlanItem struct {
	TenantID         string
	LiveContainerID  string
	ImageRef         string
	LiveRouteHost    string
	DesiredRouteHost string
	LivePublicURL    string
	DesiredPublicURL string
	Action           string
	Reason           string
}

type TenantRuntimeContractReconcilePlan struct {
	Tenants []*TenantRuntimeContractReconcilePlanItem
}

type TenantRuntimeImageRolloutPlanOptions struct {
	TenantIDs []string
	All       bool
	Image     string
}

type TenantRuntimeImageRolloutPlanItem struct {
	TenantID         string
	State            string
	LiveContainerID  string
	LiveImageRef     string
	TargetImageRef   string
	LiveRouteHost    string
	DesiredRouteHost string
	LivePublicURL    string
	DesiredPublicURL string
	Action           string
	Reason           string
}

type TenantRuntimeImageRolloutPlan struct {
	Tenants []*TenantRuntimeImageRolloutPlanItem
}

const (
	tenantRuntimeContractActionNoop    = "noop"
	tenantRuntimeContractActionRollout = "rollout"
	tenantRuntimeContractActionSkip    = "skip"
)

type tenantRuntimeRolloutRegistry interface {
	Get(tenantID string) (*registry.Tenant, error)
	List() ([]*registry.Tenant, error)
	UpdateRuntimeState(t *registry.Tenant) error
}

type tenantRuntimeRolloutDocker interface {
	CreateAndStart(ctx context.Context, tenantID, tenantDataDir string) (string, error)
	DesiredRuntimeRouting(tenantID string) cpDocker.TenantRuntimeRoutingContract
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
	registry       tenantRuntimeRolloutRegistry
	docker         tenantRuntimeRolloutDocker
	tenantsDir     string
	defaultImage   string
	synchronizer   tenantRuntimeRolloutSynchronizer
	admissionCheck func(context.Context) error
	now            func() time.Time
	sleep          func(time.Duration)
	healthTimeout  time.Duration
	healthPoll     time.Duration
}

var errTenantRuntimeMissing = errors.New("tenant runtime container missing")

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

	service, cleanup, err := newTenantRuntimeRolloutServiceFromConfig(cfg, image)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return service.Rollout(ctx, opts)
}

func PlanTenantRuntimeContractReconcile(
	ctx context.Context,
	cfg *CPConfig,
	opts TenantRuntimeContractReconcilePlanOptions,
) (*TenantRuntimeContractReconcilePlan, error) {
	service, cleanup, err := newTenantRuntimeRolloutServiceFromConfig(cfg, strings.TrimSpace(cfg.PulseImage))
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return service.PlanContractReconcile(ctx, opts)
}

func PlanTenantRuntimeImageRollout(
	ctx context.Context,
	cfg *CPConfig,
	opts TenantRuntimeImageRolloutPlanOptions,
) (*TenantRuntimeImageRolloutPlan, error) {
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

	service, cleanup, err := newTenantRuntimeRolloutServiceFromConfig(cfg, image)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return service.PlanImageRollout(ctx, opts)
}

func newTenantRuntimeRolloutServiceFromConfig(
	cfg *CPConfig,
	image string,
) (*tenantRuntimeRolloutService, func(), error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("control plane config is required")
	}
	if err := os.MkdirAll(cfg.TenantsDir(), 0o755); err != nil {
		return nil, nil, fmt.Errorf("ensure tenants dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ControlPlaneDir(), 0o755); err != nil {
		return nil, nil, fmt.Errorf("ensure control-plane dir: %w", err)
	}

	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return nil, nil, fmt.Errorf("open tenant registry: %w", err)
	}

	dockerMgr, err := cpDocker.NewManager(cpDocker.ManagerConfig{
		Image:                    image,
		Network:                  cfg.DockerNetwork,
		IsolateTenantNetworks:    cfg.IsMSPControlPlane(),
		BaseDomain:               baseDomainFromURL(cfg.BaseURL),
		TrialActivationPublicKey: cfg.TrialActivationPublicKey,
		ControlPlaneBaseURL:      cfg.BaseURL,
		ProviderHosted:           cfg.IsProviderHostedMSP(),
		TrustedProxyCIDRs:        cfg.TrustedProxyCIDRs,
		TenantDisplayName:        tenantDisplayNameResolver(reg),
		TenantReportBrand: cpDocker.TenantReportBrandConfig{
			DisplayName: cfg.ReportBrandDisplayName,
			LogoPath:    cfg.ReportBrandLogoPath,
			LogoBase64:  cfg.ReportBrandLogoBase64,
			LogoFormat:  cfg.ReportBrandLogoFormat,
		},
		MemoryLimit:      cfg.TenantMemoryLimit,
		CPUShares:        cfg.TenantCPUShares,
		TenantLogMaxSize: cfg.TenantLogMaxSize,
		TenantLogMaxFile: cfg.TenantLogMaxFile,
	})
	if err != nil {
		_ = reg.Close()
		return nil, nil, fmt.Errorf("create docker manager: %w", err)
	}

	cleanup := func() {
		_ = dockerMgr.Close()
		_ = reg.Close()
	}
	service := &tenantRuntimeRolloutService{
		registry:     reg,
		docker:       dockerMgr,
		tenantsDir:   cfg.TenantsDir(),
		defaultImage: strings.TrimSpace(image),
		synchronizer: filesystemTenantRuntimeSynchronizer{},
		admissionCheck: func(checkCtx context.Context) error {
			return EnforceStorageAdmission(checkCtx, cfg, dockerMgr)
		},
		now:           func() time.Time { return time.Now().UTC() },
		sleep:         time.Sleep,
		healthTimeout: defaultTenantRuntimeRolloutHealthTimeout,
		healthPoll:    defaultTenantRuntimeRolloutHealthPoll,
	}
	return service, cleanup, nil
}

// Rollout performs the tenant runtime rollout or drift reconciliation. The
// lease spans registry reads, stopped-writer capture, compensation and commit;
// an interrupted transaction leaves a durable hold that monitors also honor.
func (s *tenantRuntimeRolloutService) Rollout(ctx context.Context, opts TenantRuntimeRolloutOptions) (*TenantRuntimeRolloutResult, error) {
	if s == nil {
		return nil, fmt.Errorf("rollout service is nil")
	}
	tenantID := strings.TrimSpace(opts.TenantID)
	if !lifecycle.ValidTenantID(tenantID) {
		return nil, fmt.Errorf("valid tenant id is required")
	}
	image := strings.TrimSpace(opts.Image)
	if image == "" {
		image = strings.TrimSpace(s.defaultImage)
	}
	if image == "" {
		return nil, fmt.Errorf("image is required")
	}
	ctx, lease, err := lifecycle.Acquire(ctx, filepath.Join(filepath.Dir(s.tenantsDir), "control-plane"), tenantID)
	if err != nil {
		return nil, err
	}
	defer lease.Close()

	tenant, err := s.registry.Get(tenantID)
	if err != nil {
		return nil, fmt.Errorf("load tenant %s: %w", tenantID, err)
	}
	if tenant == nil {
		return nil, fmt.Errorf("tenant %s not found", tenantID)
	}
	switch tenant.State {
	case registry.TenantStateActive, registry.TenantStateProvisioning, registry.TenantStateFailed:
	default:
		return nil, fmt.Errorf("tenant %s state %s does not allow runtime recovery", tenantID, tenant.State)
	}
	healthTimeout := opts.HealthTimeout
	if healthTimeout <= 0 {
		healthTimeout = s.healthTimeout
	}
	healthPoll := opts.HealthPollInterval
	if healthPoll <= 0 {
		healthPoll = s.healthPoll
	}
	live, err := s.resolveLiveContainer(ctx, tenant)
	if err != nil && !errors.Is(err, errTenantRuntimeMissing) {
		return nil, err
	}
	if errors.Is(err, errTenantRuntimeMissing) {
		live = nil
	}
	canonicalName := tenantRuntimeContainerName(tenantID)
	if live != nil && tenantRuntimeMatchesContract(live, canonicalName, image, s.docker.DesiredRuntimeRouting(tenantID)) {
		healthy, healthErr := s.waitForHealth(ctx, live.ID, healthTimeout, healthPoll)
		if healthErr == nil {
			if err := s.persistTenantRuntimeState(tenant, live, healthy); err != nil {
				return nil, err
			}
			return &TenantRuntimeRolloutResult{
				TenantID: tenantID, PreviousContainerID: live.ID, ActiveContainerID: live.ID,
				ActiveImageRef: live.ImageRef, ActiveImageID: live.ImageID, ReconciledOnly: true,
			}, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	if err := s.enforceAdmission(ctx, "tenant runtime rollout"); err != nil {
		return nil, err
	}
	runID := sanitizeTenantRuntimeRolloutRunID(opts.RunID, s.now())
	snapshotRoot := strings.TrimSpace(opts.SnapshotRoot)
	if snapshotRoot == "" {
		snapshotRoot = filepath.Join(filepath.Dir(s.tenantsDir), "backups", "rollout")
	}
	tx := &tenantRuntimeTransaction{
		service: s, lease: lease, tenant: tenant, previous: live,
		dataDir: filepath.Join(s.tenantsDir, tenantID), canonicalName: canonicalName,
		healthTimeout: healthTimeout, healthPoll: healthPoll,
		record: lifecycle.Recovery{TenantID: tenantID, SnapshotDir: filepath.Join(snapshotRoot, runID, tenantID), Phase: "stopping"},
	}
	if live != nil {
		tx.record.PreviousContainerID = live.ID
		tx.record.BackupContainerName = canonicalName + ".pre-" + runID
		if existing, err := s.docker.Inspect(ctx, tx.record.BackupContainerName); err == nil && existing != nil {
			return nil, fmt.Errorf("backup container name %s already exists", tx.record.BackupContainerName)
		} else if err != nil && !cpDocker.IsNotFound(err) {
			return nil, fmt.Errorf("inspect backup container: %w", err)
		}
	}
	if err := lease.MarkRecovery(tx.record); err != nil {
		return nil, fmt.Errorf("record rollout recovery hold: %w", err)
	}
	if live != nil {
		if err := s.docker.Stop(ctx, live.ID); err != nil {
			return nil, tx.abort(ctx, fmt.Errorf("stop live tenant container: %w", err), false)
		}
		if err := tx.requireStopped(ctx, live.ID); err != nil {
			return nil, tx.held(err)
		}
	}
	// This is an observed negative health result: the exact runtime was
	// inspected as stopped (or both live identities were not found), just as
	// the monitor's health check treats a non-running container as unhealthy.
	// Do not leave an unfinished restore green or misdate the last observation.
	observedAt := s.now()
	unavailable := *tenant
	unavailable.HealthCheckOK = false
	unavailable.LastHealthCheck = &observedAt
	if err := s.registry.UpdateRuntimeState(&unavailable); err != nil {
		return nil, tx.abort(ctx, fmt.Errorf("record stopped runtime health: %w", err), false)
	}
	// Missing-runtime recovery also captures the existing data before a new
	// image is allowed to touch it. A failed first boot can mutate that data.
	if err := s.synchronizer.Snapshot(ctx, tx.dataDir, tx.record.SnapshotDir); err != nil {
		return nil, tx.abort(ctx, fmt.Errorf("snapshot tenant data: %w", err), false)
	}
	if live != nil {
		if err := s.docker.Rename(ctx, live.ID, tx.record.BackupContainerName); err != nil {
			return nil, tx.abort(ctx, fmt.Errorf("rename live tenant container: %w", err), false)
		}
		if err := tx.requireStopped(ctx, live.ID); err != nil {
			return nil, tx.held(err)
		}
	}
	if info, err := s.docker.Inspect(ctx, canonicalName); !cpDocker.IsNotFound(err) {
		return nil, tx.abort(ctx, fmt.Errorf("canonical name is not confirmed vacant before create (present=%t): %v", info != nil, err), false)
	}
	tx.record.Phase = "creating"
	if err := lease.MarkRecovery(tx.record); err != nil {
		return nil, tx.abort(ctx, err, false)
	}
	newID, createErr := s.docker.CreateAndStart(ctx, tenantID, tx.dataDir)
	tx.record.NewContainerID = newID // An error may still return a created ID.
	if err := lease.MarkRecovery(tx.record); err != nil {
		return nil, tx.abort(ctx, errors.Join(createErr, err), true)
	}
	if createErr != nil {
		return nil, tx.abort(ctx, fmt.Errorf("start new tenant runtime: %w", createErr), true)
	}
	if strings.TrimSpace(newID) == "" {
		return nil, tx.abort(ctx, fmt.Errorf("new tenant runtime returned no identity"), true)
	}
	if _, err := s.waitForHealth(ctx, newID, healthTimeout, healthPoll); err != nil {
		return nil, tx.abort(ctx, err, true)
	}
	newInfo, err := s.docker.Inspect(ctx, newID)
	if err != nil {
		return nil, tx.abort(ctx, fmt.Errorf("inspect new tenant runtime: %w", err), true)
	}
	if newInfo == nil || newInfo.ID != newID || !newInfo.Running || newInfo.Paused || newInfo.Restarting || !tenantRuntimeMatchesContract(newInfo, canonicalName, image, s.docker.DesiredRuntimeRouting(tenantID)) {
		return nil, tx.abort(ctx, fmt.Errorf("new tenant runtime does not match its canonical identity and runtime contract"), true)
	}
	if err := s.persistTenantRuntimeState(tenant, newInfo, true); err != nil {
		return nil, tx.abort(ctx, err, true)
	}
	if err := lease.Complete(); err != nil {
		return nil, tx.held(fmt.Errorf("runtime committed but recovery hold completion failed: %w", err))
	}
	previousID := tenant.ContainerID
	if live != nil {
		previousID = live.ID
	}
	result := &TenantRuntimeRolloutResult{
		TenantID: tenantID, PreviousContainerID: previousID, ActiveContainerID: newID,
		ActiveImageRef: newInfo.ImageRef, ActiveImageID: newInfo.ImageID,
		BackupContainerName: tx.record.BackupContainerName, RestoredMissing: live == nil,
	}
	if opts.PrunePrevious && live != nil {
		if err := s.docker.Remove(ctx, live.ID); err != nil {
			return result, fmt.Errorf("rollout committed to %s; previous container %s retained: %w", newID, live.ID, err)
		}
		result.BackupContainerName = ""
	}
	return result, nil
}

func (s *tenantRuntimeRolloutService) enforceAdmission(ctx context.Context, operation string) error {
	if s == nil || s.admissionCheck == nil {
		return nil
	}
	if err := s.admissionCheck(ctx); err != nil {
		return fmt.Errorf("%s admission denied: %w", operation, err)
	}
	return nil
}

func tenantRuntimeMatchesContract(
	live *cpDocker.RuntimeContainerInfo,
	canonicalName string,
	image string,
	desiredRouting cpDocker.TenantRuntimeRoutingContract,
) bool {
	if live == nil {
		return false
	}
	if live.Name != canonicalName || live.ImageRef != image {
		return false
	}
	// A runtime created by an older control plane can run the right image without the
	// provider marker, so the marker has to match too or it would never be recreated.
	return live.RouteHost == desiredRouting.Host && live.PublicURL == desiredRouting.PublicURL && live.ProviderHosted == desiredRouting.ProviderHosted
}

func (s *tenantRuntimeRolloutService) PlanContractReconcile(
	ctx context.Context,
	opts TenantRuntimeContractReconcilePlanOptions,
) (*TenantRuntimeContractReconcilePlan, error) {
	if s == nil {
		return nil, fmt.Errorf("rollout service is nil")
	}
	tenants, err := s.selectContractReconcileTenants(opts)
	if err != nil {
		return nil, err
	}
	plan := &TenantRuntimeContractReconcilePlan{
		Tenants: make([]*TenantRuntimeContractReconcilePlanItem, 0, len(tenants)),
	}
	for _, tenant := range tenants {
		item := &TenantRuntimeContractReconcilePlanItem{
			TenantID: strings.TrimSpace(tenant.ID),
		}
		live, err := s.resolveLiveContainer(ctx, tenant)
		if err != nil {
			if errors.Is(err, errTenantRuntimeMissing) {
				desiredRouting := s.docker.DesiredRuntimeRouting(tenant.ID)
				item.ImageRef = strings.TrimSpace(s.defaultImage)
				item.DesiredRouteHost = desiredRouting.Host
				item.DesiredPublicURL = desiredRouting.PublicURL
				item.Action = tenantRuntimeContractActionRollout
				item.Reason = "tenant runtime container is missing; recreate from existing tenant data"
			} else {
				item.Action = tenantRuntimeContractActionSkip
				item.Reason = err.Error()
			}
			plan.Tenants = append(plan.Tenants, item)
			continue
		}
		item.LiveContainerID = strings.TrimSpace(live.ID)
		item.ImageRef = strings.TrimSpace(live.ImageRef)
		item.LiveRouteHost = strings.TrimSpace(live.RouteHost)
		item.LivePublicURL = strings.TrimSpace(live.PublicURL)

		desiredRouting := s.docker.DesiredRuntimeRouting(tenant.ID)
		item.DesiredRouteHost = desiredRouting.Host
		item.DesiredPublicURL = desiredRouting.PublicURL

		if item.ImageRef == "" {
			item.Action = tenantRuntimeContractActionSkip
			item.Reason = "live runtime image reference is empty"
			plan.Tenants = append(plan.Tenants, item)
			continue
		}

		if tenantRuntimeMatchesContract(live, tenantRuntimeContainerName(tenant.ID), item.ImageRef, desiredRouting) {
			item.Action = tenantRuntimeContractActionNoop
			item.Reason = "runtime already matches canonical hosted contract"
		} else {
			item.Action = tenantRuntimeContractActionRollout
			item.Reason = "runtime contract drift detected"
		}
		plan.Tenants = append(plan.Tenants, item)
	}
	return plan, nil
}

func (s *tenantRuntimeRolloutService) PlanImageRollout(
	ctx context.Context,
	opts TenantRuntimeImageRolloutPlanOptions,
) (*TenantRuntimeImageRolloutPlan, error) {
	if s == nil {
		return nil, fmt.Errorf("rollout service is nil")
	}
	targetImage := strings.TrimSpace(opts.Image)
	if targetImage == "" {
		targetImage = strings.TrimSpace(s.defaultImage)
	}
	if targetImage == "" {
		return nil, fmt.Errorf("image is required")
	}
	tenants, err := s.selectImageRolloutTenants(opts)
	if err != nil {
		return nil, err
	}
	plan := &TenantRuntimeImageRolloutPlan{
		Tenants: make([]*TenantRuntimeImageRolloutPlanItem, 0, len(tenants)),
	}
	for _, tenant := range tenants {
		item := &TenantRuntimeImageRolloutPlanItem{
			TenantID:       strings.TrimSpace(tenant.ID),
			State:          string(tenant.State),
			TargetImageRef: targetImage,
		}
		if tenant.State != registry.TenantStateActive {
			item.Action = tenantRuntimeContractActionSkip
			item.Reason = fmt.Sprintf("tenant state is %s, not active", tenant.State)
			plan.Tenants = append(plan.Tenants, item)
			continue
		}

		desiredRouting := s.docker.DesiredRuntimeRouting(tenant.ID)
		item.DesiredRouteHost = desiredRouting.Host
		item.DesiredPublicURL = desiredRouting.PublicURL

		live, err := s.resolveLiveContainer(ctx, tenant)
		if err != nil {
			if errors.Is(err, errTenantRuntimeMissing) {
				item.Action = tenantRuntimeContractActionRollout
				item.Reason = "tenant runtime container is missing; recreate from existing tenant data"
			} else {
				item.Action = tenantRuntimeContractActionSkip
				item.Reason = err.Error()
			}
			plan.Tenants = append(plan.Tenants, item)
			continue
		}

		item.LiveContainerID = strings.TrimSpace(live.ID)
		item.LiveImageRef = strings.TrimSpace(live.ImageRef)
		item.LiveRouteHost = strings.TrimSpace(live.RouteHost)
		item.LivePublicURL = strings.TrimSpace(live.PublicURL)

		if tenantRuntimeMatchesContract(live, tenantRuntimeContainerName(tenant.ID), targetImage, desiredRouting) {
			item.Action = tenantRuntimeContractActionNoop
			item.Reason = "runtime already matches target image and canonical hosted contract"
		} else if item.LiveImageRef != targetImage {
			item.Action = tenantRuntimeContractActionRollout
			item.Reason = "runtime image differs from target"
		} else {
			item.Action = tenantRuntimeContractActionRollout
			item.Reason = "runtime contract drift detected"
		}
		plan.Tenants = append(plan.Tenants, item)
	}
	return plan, nil
}

func (s *tenantRuntimeRolloutService) selectContractReconcileTenants(
	opts TenantRuntimeContractReconcilePlanOptions,
) ([]*registry.Tenant, error) {
	if s == nil {
		return nil, fmt.Errorf("rollout service is nil")
	}
	if opts.All && len(dedupeNonEmptyStrings(opts.TenantIDs)) > 0 {
		return nil, fmt.Errorf("choose either --all or one or more tenant ids")
	}
	if opts.All {
		tenants, err := s.registry.List()
		if err != nil {
			return nil, fmt.Errorf("list tenants: %w", err)
		}
		return tenants, nil
	}

	tenantIDs := dedupeNonEmptyStrings(opts.TenantIDs)
	if len(tenantIDs) == 0 {
		return nil, fmt.Errorf("at least one tenant id or --all is required")
	}

	tenants := make([]*registry.Tenant, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenant, err := s.registry.Get(tenantID)
		if err != nil {
			return nil, fmt.Errorf("load tenant %s: %w", tenantID, err)
		}
		if tenant == nil {
			tenants = append(tenants, &registry.Tenant{ID: tenantID})
			continue
		}
		tenants = append(tenants, tenant)
	}
	return tenants, nil
}

func (s *tenantRuntimeRolloutService) selectImageRolloutTenants(
	opts TenantRuntimeImageRolloutPlanOptions,
) ([]*registry.Tenant, error) {
	if s == nil {
		return nil, fmt.Errorf("rollout service is nil")
	}
	if opts.All && len(dedupeNonEmptyStrings(opts.TenantIDs)) > 0 {
		return nil, fmt.Errorf("choose either --all or one or more tenant ids")
	}
	if opts.All {
		tenants, err := s.registry.List()
		if err != nil {
			return nil, fmt.Errorf("list tenants: %w", err)
		}
		active := make([]*registry.Tenant, 0, len(tenants))
		for _, tenant := range tenants {
			if tenant != nil && tenant.State == registry.TenantStateActive {
				active = append(active, tenant)
			}
		}
		return active, nil
	}

	tenantIDs := dedupeNonEmptyStrings(opts.TenantIDs)
	if len(tenantIDs) == 0 {
		return nil, fmt.Errorf("at least one tenant id or --all is required")
	}

	tenants := make([]*registry.Tenant, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		tenant, err := s.registry.Get(tenantID)
		if err != nil {
			return nil, fmt.Errorf("load tenant %s: %w", tenantID, err)
		}
		if tenant == nil {
			return nil, fmt.Errorf("tenant %s not found", tenantID)
		}
		tenants = append(tenants, tenant)
	}
	return tenants, nil
}

func dedupeNonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
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
		return nil, fmt.Errorf("%w: tenant %s has no canonical runtime container and no registry container_id", errTenantRuntimeMissing, tenant.ID)
	}
	info, err = s.docker.Inspect(ctx, containerID)
	if err != nil {
		if cpDocker.IsNotFound(err) {
			return nil, fmt.Errorf("%w: inspect tenant container %s: %w", errTenantRuntimeMissing, containerID, err)
		}
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
		if err := ctx.Err(); err != nil {
			return false, err
		}
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
	updated := *tenant
	updated.ContainerID = info.ID
	updated.CurrentImageDigest = info.ImageID
	updated.DesiredImageDigest = info.ImageID
	updated.LastHealthCheck = &now
	updated.HealthCheckOK = healthy
	if err := s.registry.UpdateRuntimeState(&updated); err != nil {
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
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		return now.UTC().Format("20060102T150405Z")
	}
	return sanitized
}

type filesystemTenantRuntimeSynchronizer struct {
	renameTree func(string, string) error
}

func (s filesystemTenantRuntimeSynchronizer) rename(src, dst string) error {
	if s.renameTree != nil {
		return s.renameTree(src, dst)
	}
	return os.Rename(src, dst)
}

func (filesystemTenantRuntimeSynchronizer) Snapshot(ctx context.Context, srcDir, snapshotDir string) error {
	return syncTenantRuntimeTree(ctx, srcDir, snapshotDir)
}

// Restore prepares every byte before moving the destination. The previous
// post-attempt tree is retained beside it, not cleared or silently discarded.
// The runtime lease must remain held and every writer stopped throughout.
func (s filesystemTenantRuntimeSynchronizer) Restore(ctx context.Context, snapshotDir, dstDir string) error {
	src, dst, err := validateTenantRuntimeTreePaths(snapshotDir, dstDir)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	parent := filepath.Dir(dst)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, "."+filepath.Base(dst)+".restore-*")
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(stage)
		}
	}()
	if err := copyTenantRuntimeTree(ctx, src, stage); err != nil {
		return fmt.Errorf("stage restore; destination unchanged: %w", err)
	}
	complete = true
	if err := ctx.Err(); err != nil {
		return err
	}
	retained := ""
	if _, err := os.Lstat(dst); err == nil {
		reservation, err := os.MkdirTemp(parent, "."+filepath.Base(dst)+".pre-restore-*")
		if err != nil {
			return err
		}
		// Rename cannot replace an existing directory. Reserve its private
		// parent, then move into an unused child on the same filesystem.
		retained = filepath.Join(reservation, "data")
		if err := s.rename(dst, retained); err != nil {
			return fmt.Errorf("retain destination before restore: %w; complete replacement retained at %s", err, stage)
		}
		if err := syncTenantRuntimeDirectory(reservation); err != nil {
			return fmt.Errorf("sync retained data at %s (replacement at %s): %w", retained, stage, err)
		}
		if err := syncTenantRuntimeDirectory(parent); err != nil {
			return fmt.Errorf("sync retained directory at %s (replacement at %s): %w", retained, stage, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := s.rename(stage, dst); err != nil {
		if retained != "" {
			if undoErr := s.rename(retained, dst); undoErr != nil {
				return fmt.Errorf("install staged restore: %w; original destination retained at %s, replacement at %s; undo failed: %v", err, retained, stage, undoErr)
			}
		}
		return fmt.Errorf("install staged restore: %w; complete replacement retained at %s", err, stage)
	}
	if err := syncTenantRuntimeDirectory(parent); err != nil {
		return fmt.Errorf("sync restored directory; retained tree at %s: %w", retained, err)
	}
	log.Info().Str("tenant_data_dir", dst).Str("retained_tree", retained).Msg("Tenant data restored from complete staged copy")
	return nil
}

func canonicalTenantRuntimePath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return "", err
	}
	resolved, err = canonicalTenantRuntimePath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, filepath.Base(path)), nil
}

func validateTenantRuntimeTreePaths(srcDir, dstDir string) (string, string, error) {
	if strings.TrimSpace(srcDir) == "" || strings.TrimSpace(dstDir) == "" {
		return "", "", fmt.Errorf("source and destination directories are required")
	}
	for _, path := range []string{srcDir, dstDir} {
		info, err := os.Lstat(path)
		if err != nil && !os.IsNotExist(err) {
			return "", "", err
		}
		if err == nil && !info.IsDir() {
			return "", "", fmt.Errorf("tree root %s must be a directory, not a symlink or file", path)
		}
	}
	src, err := canonicalTenantRuntimePath(srcDir)
	if err != nil {
		return "", "", err
	}
	dst, err := canonicalTenantRuntimePath(dstDir)
	if err != nil {
		return "", "", err
	}
	inside := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
	}
	if inside(src, dst) || inside(dst, src) {
		return "", "", fmt.Errorf("source and destination trees must not overlap")
	}
	info, err := os.Lstat(src)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("source must be a directory")
	}
	return src, dst, nil
}

// syncTenantRuntimeTree only creates new destinations. A repeated run ID must
// never overwrite an earlier complete (or interrupted) snapshot.
func syncTenantRuntimeTree(ctx context.Context, srcDir, dstDir string) error {
	src, dst, err := validateTenantRuntimeTreePaths(srcDir, dstDir)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(dst, 0o700); err != nil {
		return fmt.Errorf("create exclusive snapshot %s: %w", dst, err)
	}
	if err := copyTenantRuntimeTree(ctx, src, dst); err != nil {
		return err
	}
	return syncTenantRuntimeDirectory(filepath.Dir(dst))
}

func copyTenantRuntimeTree(ctx context.Context, srcDir, dstDir string) error {
	type pendingDirectory struct {
		path string
		info fs.FileInfo
	}
	pendingDirs := make([]pendingDirectory, 0, 8)

	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("rel path for %s: %w", path, err)
		}
		targetPath := dstDir
		if relPath != "." {
			targetPath = filepath.Join(dstDir, relPath)
		}

		switch mode := info.Mode(); {
		case info.IsDir():
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", targetPath, err)
			}
			pendingDirs = append(pendingDirs, pendingDirectory{path: targetPath, info: info})
			return nil
		case mode&os.ModeSymlink != 0:
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read symlink %s: %w", path, err)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return fmt.Errorf("create symlink parent %s: %w", filepath.Dir(targetPath), err)
			}
			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return fmt.Errorf("create symlink %s: %w", targetPath, err)
			}
			if err := preserveOwnership(targetPath, info, true); err != nil {
				return fmt.Errorf("preserve symlink ownership %s: %w", targetPath, err)
			}
			return nil
		case mode.IsRegular():
			if err := copyTenantRuntimeFile(ctx, path, targetPath, info); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unsupported file type in tenant data: %s", path)
		}
	})
	if err != nil {
		return fmt.Errorf("sync %s -> %s: %w", srcDir, dstDir, err)
	}

	for i := len(pendingDirs) - 1; i >= 0; i-- {
		dir := pendingDirs[i]
		if err := preserveDirectoryMetadata(dir.path, dir.info); err != nil {
			return fmt.Errorf("preserve directory metadata %s: %w", dir.path, err)
		}
	}
	return nil
}

func syncTenantRuntimeDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

type tenantRuntimeContextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r tenantRuntimeContextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func copyTenantRuntimeFile(ctx context.Context, srcPath, dstPath string, info fs.FileInfo) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create file parent %s: %w", filepath.Dir(dstPath), err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("open destination file %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, tenantRuntimeContextReader{ctx: ctx, reader: srcFile}); err != nil {
		return fmt.Errorf("copy file %s -> %s: %w", srcPath, dstPath, err)
	}
	if err := dstFile.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("chmod destination file %s: %w", dstPath, err)
	}
	if err := preserveOwnership(dstPath, info, false); err != nil {
		return fmt.Errorf("preserve file ownership %s: %w", dstPath, err)
	}
	if err := os.Chtimes(dstPath, info.ModTime(), info.ModTime()); err != nil {
		return fmt.Errorf("preserve file times %s: %w", dstPath, err)
	}
	if err := dstFile.Sync(); err != nil {
		return err
	}
	return dstFile.Close()
}

func preserveDirectoryMetadata(path string, info fs.FileInfo) error {
	if err := preserveOwnership(path, info, false); err != nil {
		return err
	}
	if err := os.Chmod(path, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	return syncTenantRuntimeDirectory(path)
}

func preserveOwnership(path string, info fs.FileInfo, symlink bool) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return nil
	}

	var err error
	if symlink {
		err = os.Lchown(path, int(stat.Uid), int(stat.Gid))
	} else {
		err = os.Chown(path, int(stat.Uid), int(stat.Gid))
	}
	if err == nil {
		return nil
	}
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.ENOSYS) {
		return nil
	}
	return err
}
