package cloudcp

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	runtimeconfig "github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

const (
	providerMSPRecoveryActionRecover = "recover"
	providerMSPRecoveryActionSkip    = "skip"
)

// ProviderMSPRecoveryOptions controls failed/degraded workspace recovery for a
// provider-hosted MSP control plane.
type ProviderMSPRecoveryOptions struct {
	TenantIDs     []string
	AllDegraded   bool
	DryRun        bool
	AllowEnvPlan  bool
	Image         string
	RunID         string
	SnapshotRoot  string
	HealthTimeout time.Duration
	PrunePrevious bool
}

// ProviderMSPRecoveryItem is the operator-visible recovery decision for one
// client workspace.
type ProviderMSPRecoveryItem struct {
	TenantID            string
	DisplayName         string
	State               string
	Action              string
	Reason              string
	Recovered           bool
	StuckProvisioning   bool
	PreviousContainerID string
	ActiveContainerID   string
	ActiveImageRef      string
	ActiveImageID       string
	RestoredMissing     bool
	ReconciledOnly      bool
	Error               string
}

// ProviderMSPRecoveryReport describes the dry-run plan or executed recovery
// outcome.
type ProviderMSPRecoveryReport struct {
	OK             bool
	DryRun         bool
	PlanVersion    string
	PlanSource     string
	LicenseID      string
	LicenseEmail   string
	WorkspaceLimit int
	Items          []ProviderMSPRecoveryItem
	RecoverCount   int
	RecoveredCount int
	SkippedCount   int
	ErrorCount     int
}

type providerMSPRecoveryDependencies struct {
	OpenRegistry         func(*CPConfig) (*registry.TenantRegistry, error)
	RolloutTenantRuntime func(context.Context, *CPConfig, TenantRuntimeRolloutOptions) (*TenantRuntimeRolloutResult, error)
	Now                  func() time.Time
}

// RecoverProviderMSPWorkspaces plans or executes recovery for failed, stuck, or
// unhealthy provider MSP client workspaces.
func RecoverProviderMSPWorkspaces(ctx context.Context, cfg *CPConfig, opts ProviderMSPRecoveryOptions) (*ProviderMSPRecoveryReport, error) {
	return recoverProviderMSPWorkspacesWithDependencies(ctx, cfg, opts, providerMSPRecoveryDependencies{})
}

func recoverProviderMSPWorkspacesWithDependencies(ctx context.Context, cfg *CPConfig, opts ProviderMSPRecoveryOptions, deps providerMSPRecoveryDependencies) (*ProviderMSPRecoveryReport, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	opts = normalizeProviderMSPRecoveryOptions(opts)
	if err := validateProviderMSPRecoveryOptions(opts); err != nil {
		return nil, err
	}
	if err := validateProviderMSPRecoveryConfig(cfg, opts); err != nil {
		return nil, err
	}
	deps = normalizeProviderMSPRecoveryDependencies(deps)

	workspaceLimit, _ := pkglicensing.WorkspaceLimitForPlan(cfg.ProviderMSPPlanVersion)
	report := &ProviderMSPRecoveryReport{
		OK:             true,
		DryRun:         opts.DryRun,
		PlanVersion:    strings.TrimSpace(cfg.ProviderMSPPlanVersion),
		PlanSource:     providerMSPPlanSourceOrDefault(cfg.ProviderMSPPlanSource),
		LicenseID:      strings.TrimSpace(cfg.ProviderMSPLicenseID),
		LicenseEmail:   strings.ToLower(strings.TrimSpace(cfg.ProviderMSPLicenseEmail)),
		WorkspaceLimit: workspaceLimit,
	}

	reg, err := deps.OpenRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}
	defer reg.Close()

	tenants, err := selectProviderMSPRecoveryTenants(reg, opts)
	if err != nil {
		return nil, err
	}
	now := deps.Now()
	for _, tenant := range tenants {
		item := planProviderMSPRecoveryItem(tenant, now)
		if item.Action == providerMSPRecoveryActionRecover {
			report.RecoverCount++
		} else {
			report.SkippedCount++
		}
		report.Items = append(report.Items, item)
	}
	if opts.DryRun {
		return report, nil
	}

	for idx := range report.Items {
		item := &report.Items[idx]
		if item.Action != providerMSPRecoveryActionRecover {
			continue
		}
		if err := validateProviderMSPRecoveryTenantData(cfg, item.TenantID); err != nil {
			item.Error = err.Error()
			report.OK = false
			report.ErrorCount++
			continue
		}

		result, err := deps.RolloutTenantRuntime(ctx, cfg, TenantRuntimeRolloutOptions{
			TenantID:      item.TenantID,
			Image:         opts.Image,
			RunID:         opts.RunID,
			SnapshotRoot:  opts.SnapshotRoot,
			HealthTimeout: opts.HealthTimeout,
			PrunePrevious: opts.PrunePrevious,
		})
		if err != nil {
			item.Error = err.Error()
			report.OK = false
			report.ErrorCount++
			continue
		}
		if result == nil || strings.TrimSpace(result.ActiveContainerID) == "" {
			item.Error = "tenant runtime recovery did not return an active container"
			report.OK = false
			report.ErrorCount++
			continue
		}
		if err := markProviderMSPRecoveryTenantActive(reg, item.TenantID, deps.Now()); err != nil {
			item.Error = err.Error()
			report.OK = false
			report.ErrorCount++
			continue
		}

		item.Recovered = true
		item.PreviousContainerID = result.PreviousContainerID
		item.ActiveContainerID = result.ActiveContainerID
		item.ActiveImageRef = result.ActiveImageRef
		item.ActiveImageID = result.ActiveImageID
		item.RestoredMissing = result.RestoredMissing
		item.ReconciledOnly = result.ReconciledOnly
		report.RecoveredCount++
	}
	if report.ErrorCount > 0 {
		return report, fmt.Errorf("provider MSP recovery failed for %d workspace(s)", report.ErrorCount)
	}
	return report, nil
}

func normalizeProviderMSPRecoveryOptions(opts ProviderMSPRecoveryOptions) ProviderMSPRecoveryOptions {
	opts.TenantIDs = dedupeProviderMSPRecoveryTenantIDs(opts.TenantIDs)
	opts.Image = strings.TrimSpace(opts.Image)
	opts.RunID = strings.TrimSpace(opts.RunID)
	opts.SnapshotRoot = strings.TrimSpace(opts.SnapshotRoot)
	return opts
}

func validateProviderMSPRecoveryOptions(opts ProviderMSPRecoveryOptions) error {
	if opts.AllDegraded && len(opts.TenantIDs) > 0 {
		return fmt.Errorf("choose either --all-degraded or one or more --tenant-id values")
	}
	if !opts.AllDegraded && len(opts.TenantIDs) == 0 {
		return fmt.Errorf("choose --all-degraded or at least one --tenant-id")
	}
	return nil
}

func validateProviderMSPRecoveryConfig(cfg *CPConfig, opts ProviderMSPRecoveryOptions) error {
	if !cfg.IsProviderHostedMSP() {
		return fmt.Errorf("provider MSP recovery requires CP_CONTROL_PLANE_MODE=%s", ControlPlaneModeProviderHostedMSP)
	}
	if cfg.UsesStripeBilling() {
		return fmt.Errorf("provider-hosted MSP recovery must be Stripe-free")
	}
	if _, known := pkglicensing.WorkspaceLimitForPlan(cfg.ProviderMSPPlanVersion); !known {
		return fmt.Errorf("provider MSP plan %q has no known workspace limit", cfg.ProviderMSPPlanVersion)
	}
	if !opts.AllowEnvPlan && strings.TrimSpace(cfg.ProviderMSPPlanSource) != ProviderMSPPlanSourceLicenseFile {
		return fmt.Errorf("provider MSP recovery requires %s plan source; rerun with --allow-env-plan only for local development", ProviderMSPPlanSourceLicenseFile)
	}
	return nil
}

func normalizeProviderMSPRecoveryDependencies(deps providerMSPRecoveryDependencies) providerMSPRecoveryDependencies {
	if deps.OpenRegistry == nil {
		deps.OpenRegistry = func(cfg *CPConfig) (*registry.TenantRegistry, error) {
			return registry.NewTenantRegistry(cfg.ControlPlaneDir())
		}
	}
	if deps.RolloutTenantRuntime == nil {
		deps.RolloutTenantRuntime = RolloutTenantRuntime
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	return deps
}

func selectProviderMSPRecoveryTenants(reg *registry.TenantRegistry, opts ProviderMSPRecoveryOptions) ([]*registry.Tenant, error) {
	if opts.AllDegraded {
		tenants, err := reg.List()
		if err != nil {
			return nil, fmt.Errorf("list tenants: %w", err)
		}
		return tenants, nil
	}

	tenants := make([]*registry.Tenant, 0, len(opts.TenantIDs))
	for _, tenantID := range opts.TenantIDs {
		tenant, err := reg.Get(tenantID)
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

func planProviderMSPRecoveryItem(tenant *registry.Tenant, now time.Time) ProviderMSPRecoveryItem {
	if tenant == nil {
		return ProviderMSPRecoveryItem{Action: providerMSPRecoveryActionSkip, Reason: "workspace is missing"}
	}
	item := ProviderMSPRecoveryItem{
		TenantID:    strings.TrimSpace(tenant.ID),
		DisplayName: strings.TrimSpace(tenant.DisplayName),
		State:       string(tenant.State),
		Action:      providerMSPRecoveryActionSkip,
	}
	if item.TenantID == "" {
		item.Reason = "workspace id is missing"
		return item
	}
	switch tenant.State {
	case registry.TenantStateFailed:
		item.Action = providerMSPRecoveryActionRecover
		item.Reason = "workspace is failed"
	case registry.TenantStateProvisioning:
		if now.IsZero() {
			now = time.Now().UTC()
		}
		if !tenant.CreatedAt.IsZero() && tenant.CreatedAt.UTC().After(now.UTC().Add(-provisioningTimeout)) {
			item.Reason = "workspace is still within the provisioning timeout"
			return item
		}
		item.Action = providerMSPRecoveryActionRecover
		item.Reason = "workspace is stuck in provisioning"
		item.StuckProvisioning = true
	case registry.TenantStateActive:
		if tenant.HealthCheckOK {
			item.Reason = "workspace is active and healthy"
			return item
		}
		item.Action = providerMSPRecoveryActionRecover
		item.Reason = "workspace health check is failing"
	default:
		item.Reason = fmt.Sprintf("workspace state %q is not recoverable by provider MSP recovery", tenant.State)
	}
	return item
}

func validateProviderMSPRecoveryTenantData(cfg *CPConfig, tenantID string) error {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" || strings.ContainsAny(tenantID, `/\`) {
		return fmt.Errorf("unsafe tenant id %q", tenantID)
	}
	tenantDataDir := filepath.Join(cfg.TenantsDir(), tenantID)
	if err := requireDirectory(tenantDataDir, "provider MSP tenant data dir"); err != nil {
		return err
	}
	if err := requireRegularFile(filepath.Join(tenantDataDir, "secrets", "handoff.key"), "provider MSP tenant handoff key"); err != nil {
		return err
	}
	if err := requireRegularFile(filepath.Join(tenantDataDir, cloudauth.HandoffKeyFile), "provider MSP tenant cloud handoff key"); err != nil {
		return err
	}
	if !runtimeconfig.NewMultiTenantPersistence(tenantDataDir).OrgExists(tenantID) {
		return fmt.Errorf("provider MSP tenant %s organization metadata is missing", tenantID)
	}
	return nil
}

func markProviderMSPRecoveryTenantActive(reg *registry.TenantRegistry, tenantID string, now time.Time) error {
	tenant, err := reg.Get(tenantID)
	if err != nil {
		return fmt.Errorf("reload recovered workspace %s: %w", tenantID, err)
	}
	if tenant == nil {
		return fmt.Errorf("recovered workspace %s disappeared from registry", tenantID)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tenant.State = registry.TenantStateActive
	tenant.HealthCheckOK = true
	tenant.LastHealthCheck = &now
	if err := reg.Update(tenant); err != nil {
		return fmt.Errorf("mark recovered workspace %s active: %w", tenantID, err)
	}
	return nil
}

func dedupeProviderMSPRecoveryTenantIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
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
