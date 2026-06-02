package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/spf13/cobra"
)

type providerMSPStatusOptions struct {
	AllowEnvPlan  bool
	PullImage     bool
	RequireBackup bool
}

type providerMSPStatusReport struct {
	OK                       bool
	Environment              string
	ControlMode              string
	BaseURL                  string
	PlanVersion              string
	PlanSource               string
	LicenseID                string
	LicenseEmail             string
	WorkspaceLimit           int
	RegistryReady            bool
	TotalTenants             int
	HealthyTenants           int
	UnhealthyTenants         int
	FailedTenants            int
	ProvisioningTenants      int
	StuckProvisioningTimeout time.Duration
	StuckProvisioningTenants []string
	CountsByState            map[registry.TenantState]int
	Preflight                *providerMSPPreflightReport
	Backup                   *providerMSPBackupStatus
	BackupReadyForUpgrade    bool
	Failures                 []string
	Warnings                 []string
}

type providerMSPBackupStatus struct {
	Directory           string
	LatestPath          string
	LatestCreatedAt     time.Time
	LatestModifiedAt    time.Time
	LatestAge           time.Duration
	LatestBytes         int64
	Verified            bool
	LicenseIncluded     bool
	RegistryTenantCount int
	RuntimeTenantCount  int
	Warning             string
}

type providerMSPStatusDependencies struct {
	OpenRegistry func(*cloudcp.CPConfig) (*registry.TenantRegistry, error)
	RunPreflight func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error)
	NewDocker    func(*cloudcp.CPConfig) (providerMSPStatusDocker, error)
	CheckBackup  func(context.Context, *cloudcp.CPConfig) (*providerMSPBackupStatus, error)
	Now          func() time.Time
}

type providerMSPStatusDocker interface {
	HealthCheck(context.Context, string) (bool, error)
	Close() error
}

func newProviderMSPStatusCmd() *cobra.Command {
	opts := providerMSPStatusOptions{}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report provider-hosted MSP operational readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			report, err := runProviderMSPStatus(cmd.Context(), cfg, opts)
			printProviderMSPStatusReport(report)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.AllowEnvPlan, "allow-env-plan", false, "Allow CP_PROVIDER_MSP_PLAN_VERSION fallback instead of a signed provider MSP license file for local development")
	cmd.Flags().BoolVar(&opts.PullImage, "pull-image", false, "Pull the tenant runtime image during status instead of inspecting only")
	cmd.Flags().BoolVar(&opts.RequireBackup, "require-backup", false, "Fail status unless a verified provider MSP backup archive is available")
	return cmd
}

func runProviderMSPStatus(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPStatusOptions) (*providerMSPStatusReport, error) {
	return runProviderMSPStatusWithDependencies(ctx, cfg, opts, providerMSPStatusDependencies{})
}

func runProviderMSPStatusWithDependencies(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPStatusOptions, deps providerMSPStatusDependencies) (*providerMSPStatusReport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	deps = normalizeProviderMSPStatusDependencies(deps)

	workspaceLimit, _ := pkglicensing.WorkspaceLimitForPlan(cfg.ProviderMSPPlanVersion)
	report := &providerMSPStatusReport{
		OK:             true,
		Environment:    strings.TrimSpace(cfg.Environment),
		ControlMode:    string(cfg.ControlPlaneMode),
		BaseURL:        strings.TrimSpace(cfg.BaseURL),
		PlanVersion:    strings.TrimSpace(cfg.ProviderMSPPlanVersion),
		PlanSource:     strings.TrimSpace(cfg.ProviderMSPPlanSource),
		LicenseID:      strings.TrimSpace(cfg.ProviderMSPLicenseID),
		LicenseEmail:   strings.ToLower(strings.TrimSpace(cfg.ProviderMSPLicenseEmail)),
		WorkspaceLimit: workspaceLimit,
		CountsByState:  map[registry.TenantState]int{},
	}
	addFailure := func(format string, args ...any) {
		report.OK = false
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
	}
	addWarning := func(format string, args ...any) {
		report.Warnings = append(report.Warnings, fmt.Sprintf(format, args...))
	}
	addBackupProblem := func(format string, args ...any) {
		message := fmt.Sprintf(format, args...)
		if opts.RequireBackup {
			addFailure("backup: %s", message)
		} else {
			addWarning("backup: %s", message)
		}
	}

	preflight, preflightErr := deps.RunPreflight(ctx, cfg, providerMSPPreflightOptions{
		AllowEnvPlan:  opts.AllowEnvPlan,
		SkipImagePull: !opts.PullImage,
	})
	report.Preflight = preflight
	if preflightErr != nil {
		if preflight != nil && len(preflight.Failures) > 0 {
			for _, failure := range preflight.Failures {
				addFailure("preflight: %s", failure)
			}
		} else {
			addFailure("preflight: %v", preflightErr)
		}
	}
	if preflight != nil {
		report.RegistryReady = preflight.RegistryReady
		if preflight.WorkspaceLimit > 0 {
			report.WorkspaceLimit = preflight.WorkspaceLimit
		}
		if preflight.LicenseID != "" {
			report.LicenseID = preflight.LicenseID
		}
		if preflight.LicenseEmail != "" {
			report.LicenseEmail = strings.ToLower(preflight.LicenseEmail)
		}
		if !preflight.OK {
			for _, failure := range preflight.Failures {
				if !containsProviderMSPStatusFailure(report.Failures, "preflight: "+failure) {
					addFailure("preflight: %s", failure)
				}
			}
		}
	}

	reg, err := deps.OpenRegistry(cfg)
	if err != nil {
		addFailure("open tenant registry: %v", err)
		return report, providerMSPStatusError(report)
	}
	defer reg.Close()

	if err := reg.Ping(); err != nil {
		addFailure("tenant registry ping: %v", err)
	} else {
		report.RegistryReady = true
	}

	counts, err := reg.CountByState()
	if err != nil {
		addFailure("tenant state counts: %v", err)
	} else {
		report.CountsByState = counts
		for _, count := range counts {
			report.TotalTenants += count
		}
		report.FailedTenants = counts[registry.TenantStateFailed]
		report.ProvisioningTenants = counts[registry.TenantStateProvisioning]
		if report.FailedTenants > 0 {
			addFailure("failed workspaces: %d", report.FailedTenants)
		}
	}

	health, err := checkProviderMSPLiveRuntimeHealth(ctx, cfg, reg, deps)
	if err != nil {
		addFailure("tenant runtime health summary: %v", err)
	} else {
		report.HealthyTenants = health.Healthy
		report.UnhealthyTenants = health.Unhealthy
		for _, failure := range health.Failures {
			addFailure("runtime health: %s", failure)
		}
		if health.Unhealthy > 0 {
			addFailure("unhealthy active workspaces: %d", health.Unhealthy)
		}
	}

	stuck, err := cloudcp.InspectStuckProvisioning(reg, deps.Now())
	if err != nil {
		addFailure("stuck provisioning inspection: %v", err)
	} else if stuck != nil {
		report.StuckProvisioningTimeout = stuck.Timeout
		report.StuckProvisioningTenants = stuck.TenantIDs
		if stuck.Count > 0 {
			addFailure("stuck provisioning workspaces: %s", strings.Join(stuck.TenantIDs, ","))
		}
	}

	backup, backupErr := deps.CheckBackup(ctx, cfg)
	report.Backup = backup
	if backupErr != nil {
		addBackupProblem("%v", backupErr)
	} else if backup == nil {
		addBackupProblem("backup posture unavailable")
	} else {
		if !backup.LatestCreatedAt.IsZero() {
			backup.LatestAge = deps.Now().Sub(backup.LatestCreatedAt)
			if backup.LatestAge < 0 {
				backup.LatestAge = 0
			}
		}
		report.BackupReadyForUpgrade = backup.Verified
		if backup.Warning != "" {
			addBackupProblem("%s", backup.Warning)
		}
	}

	return report, providerMSPStatusError(report)
}

func normalizeProviderMSPStatusDependencies(deps providerMSPStatusDependencies) providerMSPStatusDependencies {
	if deps.OpenRegistry == nil {
		deps.OpenRegistry = func(cfg *cloudcp.CPConfig) (*registry.TenantRegistry, error) {
			return registry.NewTenantRegistry(cfg.ControlPlaneDir())
		}
	}
	if deps.RunPreflight == nil {
		deps.RunPreflight = runProviderMSPPreflight
	}
	if deps.NewDocker == nil {
		deps.NewDocker = func(cfg *cloudcp.CPConfig) (providerMSPStatusDocker, error) {
			return cpDocker.NewManager(providerMSPDockerManagerConfig(cfg))
		}
	}
	if deps.CheckBackup == nil {
		deps.CheckBackup = checkProviderMSPBackupStatus
	}
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	return deps
}

type providerMSPLiveRuntimeHealth struct {
	Healthy   int
	Unhealthy int
	Failures  []string
}

func checkProviderMSPLiveRuntimeHealth(
	ctx context.Context,
	cfg *cloudcp.CPConfig,
	reg *registry.TenantRegistry,
	deps providerMSPStatusDependencies,
) (*providerMSPLiveRuntimeHealth, error) {
	if reg == nil {
		return nil, fmt.Errorf("tenant registry is required")
	}
	dockerMgr, err := deps.NewDocker(cfg)
	if err != nil {
		return nil, fmt.Errorf("create Docker manager: %w", err)
	}
	defer dockerMgr.Close()

	tenants, err := reg.ListByState(registry.TenantStateActive)
	if err != nil {
		return nil, fmt.Errorf("list active tenants: %w", err)
	}

	health := &providerMSPLiveRuntimeHealth{}
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		containerID := strings.TrimSpace(tenant.ContainerID)
		if containerID == "" {
			health.Unhealthy++
			health.Failures = append(health.Failures, fmt.Sprintf("workspace %s has no runtime container id", tenant.ID))
			continue
		}
		healthy, checkErr := dockerMgr.HealthCheck(ctx, containerID)
		if checkErr != nil {
			health.Unhealthy++
			health.Failures = append(health.Failures, fmt.Sprintf("workspace %s container %s: %v", tenant.ID, containerID, checkErr))
			continue
		}
		if healthy {
			health.Healthy++
		} else {
			health.Unhealthy++
		}
	}
	return health, nil
}

func checkProviderMSPBackupStatus(ctx context.Context, cfg *cloudcp.CPConfig) (*providerMSPBackupStatus, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	backupDir := filepath.Join(strings.TrimSpace(cfg.DataDir), "backups", "provider-msp")
	status := &providerMSPBackupStatus{Directory: backupDir}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			status.Warning = "no provider MSP backup directory found; run provider-msp backup create before upgrades or recovery drills"
			return status, nil
		}
		return status, fmt.Errorf("read provider MSP backup directory: %w", err)
	}

	var latestInfo os.FileInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return status, fmt.Errorf("stat provider MSP backup archive %s: %w", entry.Name(), infoErr)
		}
		if latestInfo == nil || info.ModTime().After(latestInfo.ModTime()) {
			latestInfo = info
			status.LatestPath = filepath.Join(backupDir, entry.Name())
			status.LatestModifiedAt = info.ModTime()
			status.LatestBytes = info.Size()
		}
	}
	if latestInfo == nil {
		status.Warning = "no provider MSP backup archive found; run provider-msp backup create before upgrades or recovery drills"
		return status, nil
	}

	verify, err := cloudcp.VerifyProviderMSPBackup(ctx, status.LatestPath)
	if err != nil {
		return status, fmt.Errorf("verify latest provider MSP backup %s: %w", status.LatestPath, err)
	}
	status.Verified = true
	status.LatestCreatedAt = verify.Manifest.CreatedAt
	status.LatestBytes = verify.VerifiedArchiveBytes
	status.LicenseIncluded = verify.Manifest.LicenseIncluded
	status.RegistryTenantCount = verify.Manifest.RegistryTenantCount
	status.RuntimeTenantCount = verify.Manifest.RuntimeTenantCount
	return status, nil
}

func providerMSPStatusError(report *providerMSPStatusReport) error {
	if report == nil || report.OK {
		return nil
	}
	return fmt.Errorf("provider MSP status failed: %s", strings.Join(report.Failures, "; "))
}

func containsProviderMSPStatusFailure(failures []string, want string) bool {
	for _, failure := range failures {
		if failure == want {
			return true
		}
	}
	return false
}

func printProviderMSPStatusReport(report *providerMSPStatusReport) {
	if report == nil {
		fmt.Println("provider_msp_status_ok=false")
		return
	}
	fmt.Printf("provider_msp_status_ok=%t\n", report.OK)
	fmt.Printf("environment=%s\n", report.Environment)
	fmt.Printf("control_plane_mode=%s\n", report.ControlMode)
	fmt.Printf("base_url=%s\n", report.BaseURL)
	fmt.Printf("plan_version=%s\n", report.PlanVersion)
	fmt.Printf("plan_source=%s\n", report.PlanSource)
	fmt.Printf("license_id=%s\n", report.LicenseID)
	fmt.Printf("license_email=%s\n", report.LicenseEmail)
	fmt.Printf("workspace_limit=%d\n", report.WorkspaceLimit)
	fmt.Printf("registry_ready=%t\n", report.RegistryReady)
	fmt.Printf("total_tenants=%d\n", report.TotalTenants)
	fmt.Printf("healthy_tenants=%d\n", report.HealthyTenants)
	fmt.Printf("unhealthy_tenants=%d\n", report.UnhealthyTenants)
	fmt.Printf("failed_tenants=%d\n", report.FailedTenants)
	fmt.Printf("provisioning_tenants=%d\n", report.ProvisioningTenants)
	fmt.Printf("stuck_provisioning_timeout=%s\n", report.StuckProvisioningTimeout)
	fmt.Printf("stuck_provisioning_count=%d\n", len(report.StuckProvisioningTenants))
	for _, tenantID := range report.StuckProvisioningTenants {
		fmt.Printf("stuck_provisioning_tenant=%s\n", tenantID)
	}
	for _, state := range sortedProviderMSPStatusStates(report.CountsByState) {
		fmt.Printf("tenant_state=%s count=%d\n", state, report.CountsByState[state])
	}
	if report.Preflight != nil && report.Preflight.Docker != nil {
		fmt.Printf("docker_reachable=%t\n", report.Preflight.Docker.DockerReachable)
		fmt.Printf("docker_network_ok=%t\n", report.Preflight.Docker.NetworkOK)
		fmt.Printf("tenant_runtime_image=%s\n", report.Preflight.Docker.ImageRef)
		fmt.Printf("tenant_runtime_image_available=%t\n", report.Preflight.Docker.ImageAvailable)
		fmt.Printf("tenant_runtime_image_pulled=%t\n", report.Preflight.Docker.ImagePulled)
	}
	if report.Preflight != nil && report.Preflight.Storage != nil {
		fmt.Printf("storage_guardrails_enabled=%t\n", report.Preflight.Storage.Enabled)
		fmt.Printf("storage_guardrails_ok=%t\n", report.Preflight.Storage.OK)
	}
	fmt.Printf("backup_ready_for_upgrade=%t\n", report.BackupReadyForUpgrade)
	if report.Backup != nil {
		fmt.Printf("backup_directory=%s\n", report.Backup.Directory)
		fmt.Printf("latest_backup_path=%s\n", report.Backup.LatestPath)
		if !report.Backup.LatestCreatedAt.IsZero() {
			fmt.Printf("latest_backup_created_at=%s\n", report.Backup.LatestCreatedAt.UTC().Format(time.RFC3339))
		}
		if !report.Backup.LatestModifiedAt.IsZero() {
			fmt.Printf("latest_backup_modified_at=%s\n", report.Backup.LatestModifiedAt.UTC().Format(time.RFC3339))
		}
		fmt.Printf("latest_backup_age=%s\n", report.Backup.LatestAge)
		fmt.Printf("latest_backup_bytes=%d\n", report.Backup.LatestBytes)
		fmt.Printf("latest_backup_verified=%t\n", report.Backup.Verified)
		fmt.Printf("latest_backup_license_included=%t\n", report.Backup.LicenseIncluded)
		fmt.Printf("latest_backup_registry_tenants=%d\n", report.Backup.RegistryTenantCount)
		fmt.Printf("latest_backup_runtime_tenants=%d\n", report.Backup.RuntimeTenantCount)
	}
	for _, warning := range report.Warnings {
		fmt.Printf("warning=%s\n", warning)
	}
	for _, failure := range report.Failures {
		fmt.Printf("failure=%s\n", failure)
	}
}

func sortedProviderMSPStatusStates(counts map[registry.TenantState]int) []registry.TenantState {
	states := make([]registry.TenantState, 0, len(counts))
	for state := range counts {
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		return string(states[i]) < string(states[j])
	})
	return states
}
