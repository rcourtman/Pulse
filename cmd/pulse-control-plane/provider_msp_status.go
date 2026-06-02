package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/spf13/cobra"
)

type providerMSPStatusOptions struct {
	AllowEnvPlan bool
	PullImage    bool
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
	Failures                 []string
}

type providerMSPStatusDependencies struct {
	OpenRegistry func(*cloudcp.CPConfig) (*registry.TenantRegistry, error)
	RunPreflight func(context.Context, *cloudcp.CPConfig, providerMSPPreflightOptions) (*providerMSPPreflightReport, error)
	Now          func() time.Time
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

	healthy, unhealthy, err := reg.HealthSummary()
	if err != nil {
		addFailure("tenant health summary: %v", err)
	} else {
		report.HealthyTenants = healthy
		report.UnhealthyTenants = unhealthy
		if unhealthy > 0 {
			addFailure("unhealthy active workspaces: %d", unhealthy)
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
	if deps.Now == nil {
		deps.Now = func() time.Time { return time.Now().UTC() }
	}
	return deps
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
