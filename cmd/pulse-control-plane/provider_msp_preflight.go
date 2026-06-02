package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/spf13/cobra"
)

type providerMSPPreflightOptions struct {
	AllowEnvPlan  bool
	SkipImagePull bool
}

type providerMSPPreflightReport struct {
	OK             bool
	Environment    string
	ControlMode    string
	BaseURL        string
	PlanVersion    string
	PlanSource     string
	LicenseID      string
	LicenseEmail   string
	WorkspaceLimit int
	RegistryReady  bool
	Docker         *cpDocker.RuntimePrerequisiteReport
	Storage        *cloudcp.StorageGuardrailReport
	Failures       []string
}

type providerMSPPreflightRegistry interface {
	Ping() error
	Close() error
}

type providerMSPPreflightDocker interface {
	CheckRuntimePrerequisites(context.Context, cpDocker.RuntimePrerequisiteOptions) (*cpDocker.RuntimePrerequisiteReport, error)
	DiskUsage(context.Context) (*cpDocker.DiskUsageSnapshot, error)
	Close() error
}

type providerMSPPreflightDependencies struct {
	OpenRegistry func(*cloudcp.CPConfig) (providerMSPPreflightRegistry, error)
	NewDocker    func(*cloudcp.CPConfig) (providerMSPPreflightDocker, error)
	CheckStorage func(context.Context, *cloudcp.CPConfig, cloudcp.StorageDockerUsageProvider) (*cloudcp.StorageGuardrailReport, error)
}

func newProviderMSPPreflightCmd() *cobra.Command {
	opts := providerMSPPreflightOptions{}
	cmd := &cobra.Command{
		Use:   "preflight",
		Short: "Check provider-hosted MSP install readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cloudcp.LoadConfig()
			if err != nil {
				return fmt.Errorf("load control plane config: %w", err)
			}
			report, err := runProviderMSPPreflight(cmd.Context(), cfg, opts)
			printProviderMSPPreflightReport(report)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.AllowEnvPlan, "allow-env-plan", false, "Allow CP_PROVIDER_MSP_PLAN_VERSION fallback instead of a signed provider MSP license file")
	cmd.Flags().BoolVar(&opts.SkipImagePull, "skip-image-pull", false, "Only inspect the tenant runtime image instead of pulling it when missing")
	return cmd
}

func runProviderMSPPreflight(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPPreflightOptions) (*providerMSPPreflightReport, error) {
	return runProviderMSPPreflightWithDependencies(ctx, cfg, opts, providerMSPPreflightDependencies{})
}

func runProviderMSPPreflightWithDependencies(ctx context.Context, cfg *cloudcp.CPConfig, opts providerMSPPreflightOptions, deps providerMSPPreflightDependencies) (*providerMSPPreflightReport, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	deps = normalizeProviderMSPPreflightDependencies(deps)
	report := &providerMSPPreflightReport{
		OK:           true,
		Environment:  strings.TrimSpace(cfg.Environment),
		ControlMode:  string(cfg.ControlPlaneMode),
		BaseURL:      strings.TrimSpace(cfg.BaseURL),
		PlanVersion:  strings.TrimSpace(cfg.ProviderMSPPlanVersion),
		PlanSource:   strings.TrimSpace(cfg.ProviderMSPPlanSource),
		LicenseID:    strings.TrimSpace(cfg.ProviderMSPLicenseID),
		LicenseEmail: strings.TrimSpace(cfg.ProviderMSPLicenseEmail),
	}
	addFailure := func(format string, args ...any) {
		report.OK = false
		report.Failures = append(report.Failures, fmt.Sprintf(format, args...))
	}

	if !cfg.IsProviderHostedMSP() {
		addFailure("CP_CONTROL_PLANE_MODE must be %s", cloudcp.ControlPlaneModeProviderHostedMSP)
	}
	if cfg.UsesStripeBilling() {
		addFailure("provider-hosted MSP preflight must be Stripe-free")
	}
	workspaceLimit, known := pkglicensing.WorkspaceLimitForPlan(cfg.ProviderMSPPlanVersion)
	if !known {
		addFailure("provider MSP plan %q has no known workspace limit", cfg.ProviderMSPPlanVersion)
	} else {
		report.WorkspaceLimit = workspaceLimit
	}
	if !opts.AllowEnvPlan && strings.TrimSpace(cfg.ProviderMSPPlanSource) != cloudcp.ProviderMSPPlanSourceLicenseFile {
		addFailure("provider MSP preflight requires %s plan source; rerun with --allow-env-plan only for local development", cloudcp.ProviderMSPPlanSourceLicenseFile)
	}

	reg, err := deps.OpenRegistry(cfg)
	if err != nil {
		addFailure("open tenant registry: %v", err)
	} else {
		defer reg.Close()
		if err := reg.Ping(); err != nil {
			addFailure("tenant registry ping: %v", err)
		} else {
			report.RegistryReady = true
		}
	}

	dockerMgr, err := deps.NewDocker(cfg)
	if err != nil {
		addFailure("create Docker manager: %v", err)
	} else {
		defer dockerMgr.Close()
		dockerReport, checkErr := dockerMgr.CheckRuntimePrerequisites(ctx, cpDocker.RuntimePrerequisiteOptions{
			PullImage: !opts.SkipImagePull,
		})
		report.Docker = dockerReport
		if checkErr != nil {
			addFailure("check Docker runtime prerequisites: %v", checkErr)
		}
		if dockerReport == nil {
			addFailure("Docker runtime prerequisite report unavailable")
		} else if !dockerReport.OK {
			for _, failure := range dockerReport.Failures {
				addFailure("Docker prerequisite: %s", failure)
			}
		}

		storageReport, storageErr := deps.CheckStorage(ctx, cfg, dockerMgr)
		report.Storage = storageReport
		if storageErr != nil {
			addFailure("check storage guardrails: %v", storageErr)
		}
		if storageReport == nil {
			addFailure("storage guardrail report unavailable")
		} else if !storageReport.OK {
			for _, failure := range storageReport.Failures {
				addFailure("storage guardrail: %s", failure)
			}
		}
	}

	if !report.OK {
		return report, fmt.Errorf("provider MSP preflight failed: %s", strings.Join(report.Failures, "; "))
	}
	return report, nil
}

func normalizeProviderMSPPreflightDependencies(deps providerMSPPreflightDependencies) providerMSPPreflightDependencies {
	if deps.OpenRegistry == nil {
		deps.OpenRegistry = func(cfg *cloudcp.CPConfig) (providerMSPPreflightRegistry, error) {
			return registry.NewTenantRegistry(cfg.ControlPlaneDir())
		}
	}
	if deps.NewDocker == nil {
		deps.NewDocker = func(cfg *cloudcp.CPConfig) (providerMSPPreflightDocker, error) {
			return cpDocker.NewManager(providerMSPDockerManagerConfig(cfg))
		}
	}
	if deps.CheckStorage == nil {
		deps.CheckStorage = cloudcp.CheckStorageGuardrails
	}
	return deps
}

func providerMSPDockerManagerConfig(cfg *cloudcp.CPConfig) cpDocker.ManagerConfig {
	if cfg == nil {
		return cpDocker.ManagerConfig{}
	}
	return cpDocker.ManagerConfig{
		Image:                    cfg.PulseImage,
		Network:                  cfg.DockerNetwork,
		IsolateTenantNetworks:    true,
		BaseDomain:               providerMSPBaseDomainFromURL(cfg.BaseURL),
		TrialActivationPublicKey: cfg.TrialActivationPublicKey,
		TrustedProxyCIDRs:        cfg.TrustedProxyCIDRs,
		MemoryLimit:              cfg.TenantMemoryLimit,
		CPUShares:                cfg.TenantCPUShares,
		TenantLogMaxSize:         cfg.TenantLogMaxSize,
		TenantLogMaxFile:         cfg.TenantLogMaxFile,
	}
}

func printProviderMSPPreflightReport(report *providerMSPPreflightReport) {
	if report == nil {
		fmt.Println("provider_msp_preflight_ok=false")
		return
	}
	fmt.Printf("provider_msp_preflight_ok=%t\n", report.OK)
	fmt.Printf("environment=%s\n", report.Environment)
	fmt.Printf("control_plane_mode=%s\n", report.ControlMode)
	fmt.Printf("base_url=%s\n", report.BaseURL)
	fmt.Printf("plan_version=%s\n", report.PlanVersion)
	fmt.Printf("plan_source=%s\n", report.PlanSource)
	fmt.Printf("license_id=%s\n", report.LicenseID)
	fmt.Printf("license_email=%s\n", report.LicenseEmail)
	fmt.Printf("workspace_limit=%d\n", report.WorkspaceLimit)
	fmt.Printf("registry_ready=%t\n", report.RegistryReady)
	if report.Docker != nil {
		fmt.Printf("docker_reachable=%t\n", report.Docker.DockerReachable)
		fmt.Printf("docker_network=%s\n", report.Docker.NetworkName)
		fmt.Printf("docker_network_ok=%t\n", report.Docker.NetworkOK)
		fmt.Printf("docker_network_id=%s\n", report.Docker.NetworkID)
		fmt.Printf("tenant_runtime_image=%s\n", report.Docker.ImageRef)
		fmt.Printf("tenant_runtime_image_available=%t\n", report.Docker.ImageAvailable)
		fmt.Printf("tenant_runtime_image_pulled=%t\n", report.Docker.ImagePulled)
		fmt.Printf("tenant_runtime_image_pull_required=%t\n", report.Docker.ImagePullRequired)
		fmt.Printf("tenant_runtime_image_id=%s\n", report.Docker.ImageID)
	}
	if report.Storage != nil {
		fmt.Printf("storage_guardrails_enabled=%t\n", report.Storage.Enabled)
		fmt.Printf("storage_guardrails_ok=%t\n", report.Storage.OK)
		for _, fs := range report.Storage.Filesystems {
			status := "ok"
			if !fs.OK {
				status = "fail"
			}
			fmt.Printf("storage_filesystem=%s path=%s status=%s available_bytes=%d min_available_bytes=%d\n",
				fs.Name,
				fs.Path,
				status,
				fs.AvailableBytes,
				fs.MinAvailableBytes,
			)
			if fs.Error != "" {
				fmt.Printf("storage_filesystem_error=%s path=%s error=%s\n", fs.Name, fs.Path, fs.Error)
			}
		}
		buildCacheStatus := "ok"
		if !report.Storage.BuildCache.OK {
			buildCacheStatus = "fail"
		}
		fmt.Printf("docker_build_cache_status=%s\n", buildCacheStatus)
		fmt.Printf("docker_build_cache_total_bytes=%d\n", report.Storage.BuildCache.TotalBytes)
		fmt.Printf("docker_build_cache_max_bytes=%d\n", report.Storage.BuildCache.MaxBytes)
		if report.Storage.BuildCache.Error != "" {
			fmt.Printf("docker_build_cache_error=%s\n", report.Storage.BuildCache.Error)
		}
	}
	for _, failure := range report.Failures {
		fmt.Printf("failure=%s\n", failure)
	}
}
