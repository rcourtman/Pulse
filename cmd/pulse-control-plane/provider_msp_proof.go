package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/api"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/account"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	cphandoff "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/handoff"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/portal"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
	runtimeconfig "github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/spf13/cobra"
)

const defaultProviderMSPProofWorkspacePrefix = "Provider MSP Proof"

var handoffTokenInputPattern = regexp.MustCompile(`name="token"\s+value="([^"]+)"`)

type providerMSPProofOptions struct {
	AccountName                string
	OwnerEmail                 string
	WorkspacePrefix            string
	WorkspaceCount             int
	InstallType                string
	TargetPath                 string
	Cleanup                    bool
	AllowEnvPlan               bool
	AllowNonProofWorkspaceName bool
}

type providerMSPProofRuntime struct {
	cfg         *cloudcp.CPConfig
	registry    *registry.TenantRegistry
	docker      *cpDocker.Manager
	provisioner *cpstripe.Provisioner
}

type providerMSPProofWorkspace struct {
	TenantID                   string
	DisplayName                string
	State                      string
	PlanVersion                string
	ContainerID                string
	PublicURL                  string
	InstallType                string
	InstallToken               string
	InstallTokenID             string
	InstallCommandGenerated    bool
	AgentTokenAuthVerified     bool
	SetupFactsTokenUseVisible  bool
	AgentReportIngestVerified  bool
	AgentReportAgentID         string
	AgentReportHostname        string
	TokenRotationVerified      bool
	RotatedInstallToken        string
	RotatedInstallTokenID      string
	OldInstallTokenRejected    bool
	RotatedAgentReportVerified bool
	HandoffExchangeVerified    bool
	HandoffTargetPath          string
}

type providerMSPProofReport struct {
	AccountID                 string
	AccountName               string
	OwnerUserID               string
	OwnerEmail                string
	PlanVersion               string
	PlanSource                string
	LicenseID                 string
	LicenseEmail              string
	WorkspaceLimit            int
	WorkspaceCount            int
	Workspaces                []providerMSPProofWorkspace
	DockerlessProvisioning    bool
	RuntimeContainerVerified  bool
	HandoffExchangeVerified   bool
	InstallTokenBoundaryOK    bool
	SetupFactsTokenUseVisible bool
	AgentReportIngestVerified bool
	TokenRotationVerified     bool
	Cleanup                   bool
}

func newProviderMSPProofCmd() *cobra.Command {
	opts := providerMSPProofOptions{
		WorkspacePrefix: defaultProviderMSPProofWorkspacePrefix,
		WorkspaceCount:  2,
		InstallType:     "pve",
		TargetPath:      "/settings/infrastructure?add=linux-host",
	}

	cmd := &cobra.Command{
		Use:   "proof",
		Short: "Prove the provider-hosted MSP workspace and handoff path",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newProviderMSPProofRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.close()

			report, err := rt.runProviderMSPProof(cmd.Context(), opts)
			if err != nil {
				return err
			}
			printProviderMSPProofReport(report)
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.AccountName, "account-name", "", "Provider MSP account display name")
	cmd.Flags().StringVar(&opts.OwnerEmail, "owner-email", "", "Provider owner email address")
	cmd.Flags().StringVar(&opts.WorkspacePrefix, "workspace-prefix", opts.WorkspacePrefix, "Proof workspace display-name prefix")
	cmd.Flags().IntVar(&opts.WorkspaceCount, "workspace-count", opts.WorkspaceCount, "Number of proof client workspaces to create; minimum 2")
	cmd.Flags().StringVar(&opts.InstallType, "install-type", opts.InstallType, "Hosted tenant install command type: pve or pbs")
	cmd.Flags().StringVar(&opts.TargetPath, "target-path", opts.TargetPath, "Tenant-local target path to verify during handoff exchange")
	cmd.Flags().BoolVar(&opts.Cleanup, "cleanup", false, "Delete proof workspaces after the proof completes")
	cmd.Flags().BoolVar(&opts.AllowEnvPlan, "allow-env-plan", false, "Allow CP_PROVIDER_MSP_PLAN_VERSION fallback instead of a signed provider MSP license file for local development")
	cmd.Flags().BoolVar(&opts.AllowNonProofWorkspaceName, "allow-non-proof-workspace-name", false, "Allow proof workspace names without proof/canary/rehearsal markers")
	_ = cmd.MarkFlagRequired("account-name")
	_ = cmd.MarkFlagRequired("owner-email")
	return cmd
}

func newProviderMSPProofRuntime(ctx context.Context) (*providerMSPProofRuntime, error) {
	cfg, err := cloudcp.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load control plane config: %w", err)
	}
	return newProviderMSPProofRuntimeFromConfig(cfg)
}

func newProviderMSPProofRuntimeFromConfig(cfg *cloudcp.CPConfig) (*providerMSPProofRuntime, error) {
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	if !cfg.IsMSPControlPlane() {
		return nil, fmt.Errorf("provider MSP proof requires CP_CONTROL_PLANE_MODE=%s or %s", cloudcp.ControlPlaneModeProviderHostedMSP, cloudcp.ControlPlaneModePulseHostedMSP)
	}
	if err := os.MkdirAll(cfg.TenantsDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create tenants dir: %w", err)
	}
	if err := os.MkdirAll(cfg.ControlPlaneDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create control-plane dir: %w", err)
	}

	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}

	var dockerMgr *cpDocker.Manager
	dockerMgr, err = cpDocker.NewManager(providerMSPDockerManagerConfig(cfg))
	if err != nil {
		if !cfg.AllowDockerlessProvisioning {
			reg.Close()
			return nil, fmt.Errorf("create docker manager: %w", err)
		}
		dockerMgr = nil
	}

	hostedEntitlements := entitlements.NewService(reg, cfg.BaseURL, cfg.TrialActivationPrivateKey)
	if cfg.IsProviderHostedMSP() {
		hostedEntitlements.SetProviderLicense(cfg.ProviderMSPLicenseKey)
	}
	provisioner := cpstripe.NewProvisioner(
		reg,
		cfg.TenantsDir(),
		dockerMgr,
		nil,
		cfg.BaseURL,
		nil,
		cfg.EmailFrom,
		cfg.AllowDockerlessProvisioning,
		cpstripe.WithHostedEntitlementService(hostedEntitlements),
		cpstripe.WithTrialActivationPrivateKey(cfg.TrialActivationPrivateKey),
		cpstripe.WithAdmissionCheck(func(checkCtx context.Context) error {
			return cloudcp.EnforceStorageAdmission(checkCtx, cfg, dockerMgr)
		}),
		cpstripe.WithDefaultMSPPlanVersion(providerMSPProofPlanVersion(cfg)),
	)

	return &providerMSPProofRuntime{
		cfg:         cfg,
		registry:    reg,
		docker:      dockerMgr,
		provisioner: provisioner,
	}, nil
}

func (rt *providerMSPProofRuntime) close() {
	if rt == nil {
		return
	}
	if rt.docker != nil {
		rt.docker.Close()
	}
	if rt.registry != nil {
		rt.registry.Close()
	}
}

func (rt *providerMSPProofRuntime) runProviderMSPProof(ctx context.Context, opts providerMSPProofOptions) (*providerMSPProofReport, error) {
	if rt == nil || rt.cfg == nil || rt.registry == nil || rt.provisioner == nil {
		return nil, fmt.Errorf("provider MSP proof runtime is not initialized")
	}
	opts, err := normalizeProviderMSPProofOptions(opts)
	if err != nil {
		return nil, err
	}
	if !opts.AllowEnvPlan && strings.TrimSpace(rt.cfg.ProviderMSPPlanSource) != cloudcp.ProviderMSPPlanSourceLicenseFile {
		return nil, fmt.Errorf("provider MSP proof requires %s plan source; rerun with --allow-env-plan only for local development", cloudcp.ProviderMSPPlanSourceLicenseFile)
	}

	bootstrap, err := cloudcp.BootstrapProviderMSP(ctx, rt.cfg, cloudcp.ProviderMSPBootstrapOptions{
		AccountName:       opts.AccountName,
		OwnerEmail:        opts.OwnerEmail,
		GenerateMagicLink: false,
	})
	if err != nil {
		return nil, fmt.Errorf("bootstrap provider MSP account: %w", err)
	}

	createdTenants := make([]*registry.Tenant, 0, opts.WorkspaceCount)
	if opts.Cleanup {
		defer func() {
			_ = rt.cleanupProviderMSPProofTenants(context.Background(), createdTenants)
		}()
	}

	report := &providerMSPProofReport{
		AccountID:                 bootstrap.AccountID,
		AccountName:               bootstrap.AccountName,
		OwnerUserID:               bootstrap.OwnerUserID,
		OwnerEmail:                bootstrap.OwnerEmail,
		PlanVersion:               bootstrap.PlanVersion,
		PlanSource:                bootstrap.PlanSource,
		LicenseID:                 bootstrap.LicenseID,
		LicenseEmail:              bootstrap.LicenseEmail,
		WorkspaceLimit:            bootstrap.WorkspaceLimit,
		RuntimeContainerVerified:  true,
		HandoffExchangeVerified:   true,
		InstallTokenBoundaryOK:    true,
		SetupFactsTokenUseVisible: true,
		AgentReportIngestVerified: true,
		TokenRotationVerified:     true,
		Cleanup:                   opts.Cleanup,
	}

	for idx := 0; idx < opts.WorkspaceCount; idx++ {
		displayName := fmt.Sprintf("%s %02d", opts.WorkspacePrefix, idx+1)
		tenant, err := rt.createProviderMSPProofWorkspace(ctx, bootstrap.AccountID, displayName, bootstrap.OwnerEmail)
		if err != nil {
			return nil, fmt.Errorf("create proof workspace %d: %w", idx+1, err)
		}
		createdTenants = append(createdTenants, tenant)

		workspace, err := rt.proveProviderMSPWorkspace(ctx, tenant, bootstrap.OwnerUserID, opts.InstallType, opts.TargetPath)
		if err != nil {
			return nil, fmt.Errorf("prove workspace %s: %w", tenant.ID, err)
		}
		if workspace.ContainerID == "" {
			report.RuntimeContainerVerified = false
			report.DockerlessProvisioning = true
		}
		if !workspace.HandoffExchangeVerified {
			report.HandoffExchangeVerified = false
		}
		if !workspace.SetupFactsTokenUseVisible {
			report.SetupFactsTokenUseVisible = false
		}
		if !workspace.AgentReportIngestVerified {
			report.AgentReportIngestVerified = false
		}
		if !workspace.TokenRotationVerified {
			report.TokenRotationVerified = false
		}
		report.Workspaces = append(report.Workspaces, workspace)
	}

	boundaryOK, err := rt.verifyProviderMSPInstallTokenIsolation(report.Workspaces)
	if err != nil {
		return nil, err
	}
	report.InstallTokenBoundaryOK = boundaryOK
	report.WorkspaceCount = len(report.Workspaces)
	return report, nil
}

func normalizeProviderMSPProofOptions(opts providerMSPProofOptions) (providerMSPProofOptions, error) {
	opts.AccountName = strings.TrimSpace(opts.AccountName)
	opts.OwnerEmail = strings.ToLower(strings.TrimSpace(opts.OwnerEmail))
	opts.WorkspacePrefix = strings.TrimSpace(opts.WorkspacePrefix)
	opts.InstallType = strings.ToLower(strings.TrimSpace(opts.InstallType))
	opts.TargetPath = strings.TrimSpace(opts.TargetPath)
	if opts.AccountName == "" {
		return opts, fmt.Errorf("--account-name is required")
	}
	if opts.OwnerEmail == "" {
		return opts, fmt.Errorf("--owner-email is required")
	}
	if opts.WorkspacePrefix == "" {
		opts.WorkspacePrefix = defaultProviderMSPProofWorkspacePrefix
	}
	if opts.WorkspaceCount < 2 {
		return opts, fmt.Errorf("--workspace-count must be at least 2 so tenant isolation can be proven")
	}
	if _, err := apiInstallTypeForProviderMSPProof(opts.InstallType); err != nil {
		return opts, err
	}
	if !opts.AllowNonProofWorkspaceName && !containsProviderMSPProofMarker(opts.WorkspacePrefix) {
		return opts, fmt.Errorf("--workspace-prefix must contain proof, canary, rehearsal, or seed")
	}
	if opts.TargetPath == "" {
		opts.TargetPath = "/settings/infrastructure?add=linux-host"
	}
	if !strings.HasPrefix(opts.TargetPath, "/") || strings.HasPrefix(opts.TargetPath, "//") {
		return opts, fmt.Errorf("--target-path must be a local absolute path")
	}
	return opts, nil
}

func apiInstallTypeForProviderMSPProof(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pve", "pbs":
		return strings.ToLower(strings.TrimSpace(raw)), nil
	default:
		return "", fmt.Errorf("--install-type must be pve or pbs")
	}
}

func containsProviderMSPProofMarker(values ...string) bool {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		for _, marker := range []string{"proof", "canary", "rehearsal", "seed"} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	}
	return false
}

func (rt *providerMSPProofRuntime) createProviderMSPProofWorkspace(ctx context.Context, accountID, displayName, ownerEmail string) (*registry.Tenant, error) {
	handler := account.HandleCreateTenantWithWorkspaceLimitPolicy(
		rt.registry,
		rt.provisioner,
		account.WorkspaceLimitPolicy{
			ProviderHostedMSP:      true,
			ProviderMSPPlanVersion: providerMSPProofPlanVersion(rt.cfg),
		},
	)
	mux := http.NewServeMux()
	mux.Handle("/api/accounts/{account_id}/tenants", handler)

	payload, err := json.Marshal(map[string]string{"display_name": displayName})
	if err != nil {
		return nil, fmt.Errorf("marshal create workspace request: %w", err)
	}
	path := "/api/accounts/" + url.PathEscape(accountID) + "/tenants"
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-Email", ownerEmail)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		return nil, fmt.Errorf("workspace create status=%d body=%s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	var tenant registry.Tenant
	if err := json.Unmarshal(rec.Body.Bytes(), &tenant); err != nil {
		return nil, fmt.Errorf("decode workspace create response: %w", err)
	}
	if strings.TrimSpace(tenant.ID) == "" {
		return nil, fmt.Errorf("workspace create response missing tenant id")
	}
	return &tenant, nil
}

func (rt *providerMSPProofRuntime) proveProviderMSPWorkspace(ctx context.Context, tenant *registry.Tenant, ownerUserID, installType, targetPath string) (providerMSPProofWorkspace, error) {
	if tenant == nil {
		return providerMSPProofWorkspace{}, fmt.Errorf("tenant is required")
	}
	tenantDataDir, err := safeProviderMSPProofTenantDataDir(rt.cfg.TenantsDir(), tenant.ID)
	if err != nil {
		return providerMSPProofWorkspace{}, err
	}
	publicURL := rt.providerMSPProofTenantPublicURL(tenant.ID)
	install, err := api.GenerateHostedTenantAgentInstallCommand(api.HostedTenantAgentInstallCommandOptions{
		Config: &runtimeconfig.Config{
			DataPath:   tenantDataDir,
			ConfigPath: tenantDataDir,
			PublicURL:  publicURL,
		},
		Persistence: configPersistenceForProviderMSPProof(tenantDataDir),
		MultiTenant: runtimeconfig.NewMultiTenantPersistence(tenantDataDir),
		HostedMode:  true,
		OrgID:       tenant.ID,
		InstallType: installType,
		OwnerUserID: ownerUserID,
		BaseURL:     publicURL,
	})
	if err != nil {
		return providerMSPProofWorkspace{}, fmt.Errorf("generate hosted tenant install command: %w", err)
	}
	if install == nil || strings.TrimSpace(install.Command) == "" || strings.TrimSpace(install.Token) == "" {
		return providerMSPProofWorkspace{}, fmt.Errorf("hosted tenant install command result is incomplete")
	}

	tokenAuthVerified, err := markProviderMSPProofAgentTokenUsed(tenantDataDir, tenant.ID, install.Token)
	if err != nil {
		return providerMSPProofWorkspace{}, err
	}
	facts := portal.NewTenantDirWorkspaceSetupFactReader(rt.cfg.TenantsDir()).FactsForWorkspace(tenant.ID)
	setupFactsVisible := facts.AgentCount != nil && *facts.AgentCount > 0 &&
		facts.AgentTokenCount != nil && *facts.AgentTokenCount > 0 &&
		facts.LastAgentSeenAt != nil

	agentReport, err := rt.verifyProviderMSPProofAgentReportIngest(ctx, tenant, tenantDataDir, install.Token, install.TokenID)
	if err != nil {
		return providerMSPProofWorkspace{}, err
	}

	rotation, err := rt.verifyProviderMSPProofInstallTokenRotation(ctx, tenant, tenantDataDir, install.Token, install.TokenID)
	if err != nil {
		return providerMSPProofWorkspace{}, err
	}

	exchangedTargetPath, err := rt.verifyProviderMSPProofHandoff(ctx, tenant, ownerUserID, targetPath)
	if err != nil {
		return providerMSPProofWorkspace{}, err
	}

	return providerMSPProofWorkspace{
		TenantID:                   tenant.ID,
		DisplayName:                tenant.DisplayName,
		State:                      string(tenant.State),
		PlanVersion:                tenant.PlanVersion,
		ContainerID:                tenant.ContainerID,
		PublicURL:                  publicURL,
		InstallType:                install.InstallType,
		InstallToken:               install.Token,
		InstallTokenID:             install.TokenID,
		InstallCommandGenerated:    true,
		AgentTokenAuthVerified:     tokenAuthVerified,
		SetupFactsTokenUseVisible:  setupFactsVisible,
		AgentReportIngestVerified:  agentReport.Verified,
		AgentReportAgentID:         agentReport.AgentID,
		AgentReportHostname:        agentReport.Hostname,
		TokenRotationVerified:      rotation.Verified,
		RotatedInstallToken:        rotation.RotatedToken,
		RotatedInstallTokenID:      rotation.RotatedTokenID,
		OldInstallTokenRejected:    rotation.OldTokenRejected,
		RotatedAgentReportVerified: rotation.RotatedAgentReportVerified,
		HandoffExchangeVerified:    exchangedTargetPath == targetPath,
		HandoffTargetPath:          exchangedTargetPath,
	}, nil
}

type providerMSPProofAgentReportIngest struct {
	Verified bool
	AgentID  string
	Hostname string
}

type providerMSPProofInstallTokenRotation struct {
	Verified                   bool
	RotatedToken               string
	RotatedTokenID             string
	OldTokenRejected           bool
	RotatedAgentReportVerified bool
}

func (rt *providerMSPProofRuntime) verifyProviderMSPProofAgentReportIngest(ctx context.Context, tenant *registry.Tenant, tenantDataDir, rawToken, tokenID string) (providerMSPProofAgentReportIngest, error) {
	if tenant == nil {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant is required")
	}
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant id is required")
	}

	tokens, err := configPersistenceForProviderMSPProof(tenantDataDir).LoadAPITokens()
	if err != nil {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("load tenant api tokens before report ingest: %w", err)
	}
	tenantCfg := &runtimeconfig.Config{
		DataPath:   tenantDataDir,
		ConfigPath: tenantDataDir,
		PublicURL:  rt.providerMSPProofTenantPublicURL(tenantID),
		APITokens:  tokens,
	}
	tenantPersistence := runtimeconfig.NewMultiTenantPersistence(tenantDataDir)
	if !tenantPersistence.OrgExists(tenantID) {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s organization metadata is missing from runtime data dir", tenantID)
	}

	tenantMonitor := monitoring.NewMultiTenantMonitor(tenantCfg, tenantPersistence, nil)
	defer tenantMonitor.Stop()

	handler := api.NewUnifiedAgentHandlers(tenantMonitor, nil, nil)
	report := providerMSPProofAgentReport()
	body, err := json.Marshal(report)
	if err != nil {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("marshal provider MSP proof agent report: %w", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(ctx, api.OrgIDContextKey, tenantID))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	api.RequireAuth(tenantCfg, api.RequireScope(runtimeconfig.ScopeAgentReport, handler.HandleReport))(rec, req)
	if rec.Code != http.StatusOK {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s agent report status=%d body=%s", tenantID, rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	var payload struct {
		AgentID string `json:"agentId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("decode provider MSP proof agent report response: %w", err)
	}
	if strings.TrimSpace(payload.AgentID) == "" {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s agent report response omitted agent id", tenantID)
	}

	monitor, ok := tenantMonitor.PeekMonitor(tenantID)
	if !ok || monitor == nil {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s monitor was not initialized by agent report", tenantID)
	}
	hosts := monitor.GetLiveHostsSnapshot()
	if len(hosts) != 1 {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s host count after agent report = %d, want 1", tenantID, len(hosts))
	}
	host := hosts[0]
	if host.ID != payload.AgentID {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s ingested host id=%q, response agent id=%q", tenantID, host.ID, payload.AgentID)
	}
	if host.Hostname != "pve1" {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s ingested hostname=%q, want pve1", tenantID, host.Hostname)
	}
	if strings.TrimSpace(tokenID) != "" && strings.TrimSpace(host.TokenID) != strings.TrimSpace(tokenID) {
		return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s ingested token id=%q, want %q", tenantID, host.TokenID, tokenID)
	}

	if defaultMonitor, ok := tenantMonitor.PeekMonitor("default"); ok && defaultMonitor != nil {
		if defaultHosts := defaultMonitor.GetLiveHostsSnapshot(); len(defaultHosts) != 0 {
			return providerMSPProofAgentReportIngest{}, fmt.Errorf("tenant %s report leaked into default runtime: %#v", tenantID, defaultHosts)
		}
	}

	return providerMSPProofAgentReportIngest{
		Verified: true,
		AgentID:  payload.AgentID,
		Hostname: host.Hostname,
	}, nil
}

func providerMSPProofAgentReport() agentshost.Report {
	return agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "provider-msp-proof-agent",
			Version:         "6.0.0-provider-msp-proof",
			Type:            "unified",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:            "provider-msp-proof-machine",
			MachineID:     "provider-msp-proof-machine",
			Hostname:      "pve1",
			DisplayName:   "pve1",
			Platform:      "linux",
			OSName:        "Debian",
			OSVersion:     "12",
			Architecture:  "amd64",
			CPUCount:      4,
			UptimeSeconds: 3600,
			KernelVersion: "6.1.0-provider-proof",
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 12.5,
			Memory: agentshost.MemoryMetric{
				TotalBytes: 8 * 1024 * 1024 * 1024,
				UsedBytes:  2 * 1024 * 1024 * 1024,
				FreeBytes:  6 * 1024 * 1024 * 1024,
				Usage:      25,
			},
		},
		Timestamp:  time.Now().UTC(),
		SequenceID: "provider-msp-proof-report",
	}
}

func configPersistenceForProviderMSPProof(tenantDataDir string) *runtimeconfig.ConfigPersistence {
	return runtimeconfig.NewConfigPersistence(tenantDataDir)
}

func (rt *providerMSPProofRuntime) verifyProviderMSPProofInstallTokenRotation(ctx context.Context, tenant *registry.Tenant, tenantDataDir, oldRawToken, oldTokenID string) (providerMSPProofInstallTokenRotation, error) {
	if tenant == nil {
		return providerMSPProofInstallTokenRotation{}, fmt.Errorf("tenant is required")
	}
	tenantID := strings.TrimSpace(tenant.ID)
	if tenantID == "" {
		return providerMSPProofInstallTokenRotation{}, fmt.Errorf("tenant id is required")
	}

	newRawToken, newTokenID, err := rotateProviderMSPProofInstallToken(tenantDataDir, tenantID, oldRawToken, oldTokenID)
	if err != nil {
		return providerMSPProofInstallTokenRotation{}, err
	}
	oldRejected, err := rt.verifyProviderMSPProofAgentReportRejected(ctx, tenant, tenantDataDir, oldRawToken)
	if err != nil {
		return providerMSPProofInstallTokenRotation{}, err
	}
	if !oldRejected {
		return providerMSPProofInstallTokenRotation{}, fmt.Errorf("tenant %s accepted rotated-out install token %s", tenantID, oldTokenID)
	}

	rotatedReport, err := rt.verifyProviderMSPProofAgentReportIngest(ctx, tenant, tenantDataDir, newRawToken, newTokenID)
	if err != nil {
		return providerMSPProofInstallTokenRotation{}, fmt.Errorf("verify rotated install token report ingest: %w", err)
	}

	return providerMSPProofInstallTokenRotation{
		Verified:                   oldRejected && rotatedReport.Verified,
		RotatedToken:               newRawToken,
		RotatedTokenID:             newTokenID,
		OldTokenRejected:           oldRejected,
		RotatedAgentReportVerified: rotatedReport.Verified,
	}, nil
}

func rotateProviderMSPProofInstallToken(tenantDataDir, tenantID, oldRawToken, oldTokenID string) (string, string, error) {
	persistence := configPersistenceForProviderMSPProof(tenantDataDir)
	tokens, err := persistence.LoadAPITokens()
	if err != nil {
		return "", "", fmt.Errorf("load tenant api tokens before rotation: %w", err)
	}
	cfg := &runtimeconfig.Config{APITokens: tokens}
	oldRecord, ok := cfg.ValidateAPIToken(oldRawToken)
	if !ok || oldRecord == nil {
		return "", "", fmt.Errorf("tenant %s old install token did not validate before rotation", tenantID)
	}
	if strings.TrimSpace(oldRecord.ID) != strings.TrimSpace(oldTokenID) {
		return "", "", fmt.Errorf("tenant %s old install token id=%q, want %q", tenantID, oldRecord.ID, oldTokenID)
	}
	if strings.TrimSpace(oldRecord.OrgID) != tenantID {
		return "", "", fmt.Errorf("tenant %s old install token org id=%q", tenantID, oldRecord.OrgID)
	}

	newRawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		return "", "", fmt.Errorf("generate rotated install token: %w", err)
	}
	newRecord, err := runtimeconfig.NewAPITokenRecord(newRawToken, oldRecord.Name, oldRecord.Scopes)
	if err != nil {
		return "", "", fmt.Errorf("create rotated install token record: %w", err)
	}
	newRecord.OrgID = oldRecord.OrgID
	newRecord.OrgIDs = append([]string{}, oldRecord.OrgIDs...)
	newRecord.Metadata = cloneProviderMSPProofTokenMetadata(oldRecord.Metadata)
	if oldRecord.ExpiresAt != nil {
		expiresAt := *oldRecord.ExpiresAt
		newRecord.ExpiresAt = &expiresAt
	}

	nextTokens := make([]runtimeconfig.APITokenRecord, 0, len(cfg.APITokens))
	removedOld := false
	for _, token := range cfg.APITokens {
		if strings.TrimSpace(token.ID) == strings.TrimSpace(oldTokenID) {
			removedOld = true
			continue
		}
		nextTokens = append(nextTokens, token)
	}
	if !removedOld {
		return "", "", fmt.Errorf("tenant %s old install token id %s disappeared before rotation", tenantID, oldTokenID)
	}
	nextTokens = append(nextTokens, *newRecord)
	rotatedCfg := &runtimeconfig.Config{APITokens: nextTokens}
	rotatedCfg.SortAPITokens()
	if err := persistence.SaveAPITokens(rotatedCfg.APITokens); err != nil {
		return "", "", fmt.Errorf("persist rotated tenant install token: %w", err)
	}

	loaded, err := persistence.LoadAPITokens()
	if err != nil {
		return "", "", fmt.Errorf("reload rotated tenant api tokens: %w", err)
	}
	loadedCfg := &runtimeconfig.Config{APITokens: loaded}
	if _, ok := loadedCfg.ValidateAPIToken(oldRawToken); ok {
		return "", "", fmt.Errorf("tenant %s old install token still validates after rotation", tenantID)
	}
	replacement, ok := loadedCfg.ValidateAPIToken(newRawToken)
	if !ok || replacement == nil {
		return "", "", fmt.Errorf("tenant %s rotated install token does not validate", tenantID)
	}
	if strings.TrimSpace(replacement.OrgID) != tenantID {
		return "", "", fmt.Errorf("tenant %s rotated install token org id=%q", tenantID, replacement.OrgID)
	}
	if replacement.Metadata["issued_via"] != "hosted_agent_install_command" {
		return "", "", fmt.Errorf("tenant %s rotated install token lost install metadata", tenantID)
	}
	if err := persistence.SaveAPITokens(loadedCfg.APITokens); err != nil {
		return "", "", fmt.Errorf("persist rotated tenant install token use: %w", err)
	}
	return newRawToken, replacement.ID, nil
}

func cloneProviderMSPProofTokenMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	clone := make(map[string]string, len(metadata))
	for key, value := range metadata {
		clone[key] = value
	}
	return clone
}

func (rt *providerMSPProofRuntime) verifyProviderMSPProofAgentReportRejected(ctx context.Context, tenant *registry.Tenant, tenantDataDir, rawToken string) (bool, error) {
	if tenant == nil {
		return false, fmt.Errorf("tenant is required")
	}
	tenantID := strings.TrimSpace(tenant.ID)
	tokens, err := configPersistenceForProviderMSPProof(tenantDataDir).LoadAPITokens()
	if err != nil {
		return false, fmt.Errorf("load tenant api tokens before old-token rejection proof: %w", err)
	}
	tenantCfg := &runtimeconfig.Config{
		DataPath:   tenantDataDir,
		ConfigPath: tenantDataDir,
		PublicURL:  rt.providerMSPProofTenantPublicURL(tenantID),
		APITokens:  tokens,
	}
	tenantPersistence := runtimeconfig.NewMultiTenantPersistence(tenantDataDir)
	if !tenantPersistence.OrgExists(tenantID) {
		return false, fmt.Errorf("tenant %s organization metadata is missing from runtime data dir", tenantID)
	}
	tenantMonitor := monitoring.NewMultiTenantMonitor(tenantCfg, tenantPersistence, nil)
	defer tenantMonitor.Stop()

	handler := api.NewUnifiedAgentHandlers(tenantMonitor, nil, nil)
	body, err := json.Marshal(providerMSPProofAgentReport())
	if err != nil {
		return false, fmt.Errorf("marshal provider MSP proof rejected agent report: %w", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/agents/agent/report", bytes.NewReader(body))
	req = req.WithContext(context.WithValue(ctx, api.OrgIDContextKey, tenantID))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	api.RequireAuth(tenantCfg, api.RequireScope(runtimeconfig.ScopeAgentReport, handler.HandleReport))(rec, req)
	if rec.Code == http.StatusOK {
		return false, nil
	}
	if rec.Code != http.StatusUnauthorized {
		return false, fmt.Errorf("tenant %s rotated-out token report status=%d body=%s, want %d", tenantID, rec.Code, strings.TrimSpace(rec.Body.String()), http.StatusUnauthorized)
	}
	return true, nil
}

func markProviderMSPProofAgentTokenUsed(tenantDataDir, tenantID, rawToken string) (bool, error) {
	persistence := configPersistenceForProviderMSPProof(tenantDataDir)
	tokens, err := persistence.LoadAPITokens()
	if err != nil {
		return false, fmt.Errorf("load tenant api tokens: %w", err)
	}
	cfg := &runtimeconfig.Config{APITokens: tokens}
	record, ok := cfg.ValidateAPIToken(rawToken)
	if !ok || record == nil {
		return false, fmt.Errorf("generated install token did not validate in tenant %s", tenantID)
	}
	if strings.TrimSpace(record.OrgID) != tenantID {
		return false, fmt.Errorf("install token org id = %q, want %q", record.OrgID, tenantID)
	}
	if err := persistence.SaveAPITokens(cfg.APITokens); err != nil {
		return false, fmt.Errorf("persist used tenant api token: %w", err)
	}
	return true, nil
}

func (rt *providerMSPProofRuntime) verifyProviderMSPProofHandoff(ctx context.Context, tenant *registry.Tenant, ownerUserID, targetPath string) (string, error) {
	baseDomain := providerMSPBaseDomainFromURL(rt.cfg.BaseURL)
	if baseDomain == "" {
		return "", fmt.Errorf("could not derive provider base domain from %q", rt.cfg.BaseURL)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/accounts/{account_id}/tenants/{tenant_id}/handoff", cphandoff.HandleHandoff(rt.registry, rt.cfg.TenantsDir()))
	handoffURL := "https://" + baseDomain + "/api/accounts/" + url.PathEscape(tenant.AccountID) + "/tenants/" + url.PathEscape(tenant.ID) + "/handoff?target_path=" + url.QueryEscape(targetPath)
	req := httptest.NewRequest(http.MethodPost, handoffURL, nil)
	req = req.WithContext(ctx)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = baseDomain
	req.Header.Set("X-User-ID", ownerUserID)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		return "", fmt.Errorf("handoff status=%d body=%s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	matches := handoffTokenInputPattern.FindStringSubmatch(rec.Body.String())
	if len(matches) != 2 || strings.TrimSpace(matches[1]) == "" {
		return "", fmt.Errorf("handoff response did not include a token input")
	}

	tenantDataDir, err := safeProviderMSPProofTenantDataDir(rt.cfg.TenantsDir(), tenant.ID)
	if err != nil {
		return "", err
	}
	form := url.Values{"token": []string{matches[1]}}
	exchangeURL := rt.providerMSPProofTenantPublicURL(tenant.ID) + "/api/cloud/handoff/exchange?format=json"
	exchangeReq := httptest.NewRequest(http.MethodPost, exchangeURL, strings.NewReader(form.Encode()))
	exchangeReq = exchangeReq.WithContext(ctx)
	exchangeReq.RemoteAddr = "127.0.0.1:12345"
	exchangeReq.Header.Set("Accept", "application/json")
	exchangeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	exchangeRec := httptest.NewRecorder()
	api.HandleHandoffExchange(tenantDataDir).ServeHTTP(exchangeRec, exchangeReq)
	if exchangeRec.Code != http.StatusOK {
		return "", fmt.Errorf("handoff exchange status=%d body=%s", exchangeRec.Code, strings.TrimSpace(exchangeRec.Body.String()))
	}

	var payload struct {
		TenantID   string `json:"tenant_id"`
		UserID     string `json:"user_id"`
		TargetPath string `json:"target_path"`
	}
	if err := json.Unmarshal(exchangeRec.Body.Bytes(), &payload); err != nil {
		return "", fmt.Errorf("decode handoff exchange response: %w", err)
	}
	if payload.TenantID != tenant.ID {
		return "", fmt.Errorf("handoff exchange tenant_id=%q, want %q", payload.TenantID, tenant.ID)
	}
	if payload.UserID != ownerUserID {
		return "", fmt.Errorf("handoff exchange user_id=%q, want %q", payload.UserID, ownerUserID)
	}
	return payload.TargetPath, nil
}

func (rt *providerMSPProofRuntime) verifyProviderMSPInstallTokenIsolation(workspaces []providerMSPProofWorkspace) (bool, error) {
	for _, workspace := range workspaces {
		activeTokenID := strings.TrimSpace(workspace.RotatedInstallTokenID)
		if activeTokenID == "" {
			activeTokenID = strings.TrimSpace(workspace.InstallTokenID)
		}
		tenantDataDir, err := safeProviderMSPProofTenantDataDir(rt.cfg.TenantsDir(), workspace.TenantID)
		if err != nil {
			return false, err
		}
		tokens, err := configPersistenceForProviderMSPProof(tenantDataDir).LoadAPITokens()
		if err != nil {
			return false, fmt.Errorf("load tenant %s api tokens: %w", workspace.TenantID, err)
		}
		found := false
		for _, token := range tokens {
			if strings.TrimSpace(token.ID) == activeTokenID {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Errorf("tenant %s missing active proof install token id %s", workspace.TenantID, activeTokenID)
		}
	}
	for _, left := range workspaces {
		leftTokenID := strings.TrimSpace(left.RotatedInstallTokenID)
		if leftTokenID == "" {
			leftTokenID = strings.TrimSpace(left.InstallTokenID)
		}
		leftRawToken := strings.TrimSpace(left.RotatedInstallToken)
		if leftRawToken == "" {
			leftRawToken = strings.TrimSpace(left.InstallToken)
		}
		for _, right := range workspaces {
			if left.TenantID == right.TenantID {
				continue
			}
			rightTokenID := strings.TrimSpace(right.RotatedInstallTokenID)
			if rightTokenID == "" {
				rightTokenID = strings.TrimSpace(right.InstallTokenID)
			}
			if leftTokenID == rightTokenID {
				return false, fmt.Errorf("tenant %s and %s reused install token id %s", left.TenantID, right.TenantID, leftTokenID)
			}
			rightTenantDataDir, err := safeProviderMSPProofTenantDataDir(rt.cfg.TenantsDir(), right.TenantID)
			if err != nil {
				return false, err
			}
			accepted, err := providerMSPProofTenantAcceptsToken(rightTenantDataDir, leftRawToken)
			if err != nil {
				return false, err
			}
			if accepted {
				return false, fmt.Errorf("tenant %s accepted install token minted for tenant %s", right.TenantID, left.TenantID)
			}
		}
	}
	return true, nil
}

func providerMSPProofTenantAcceptsToken(tenantDataDir, rawToken string) (bool, error) {
	tokens, err := configPersistenceForProviderMSPProof(tenantDataDir).LoadAPITokens()
	if err != nil {
		return false, fmt.Errorf("load tenant api tokens: %w", err)
	}
	cfg := &runtimeconfig.Config{APITokens: tokens}
	_, ok := cfg.ValidateAPIToken(rawToken)
	return ok, nil
}

func (rt *providerMSPProofRuntime) cleanupProviderMSPProofTenants(ctx context.Context, tenants []*registry.Tenant) error {
	var failures []string
	for i := len(tenants) - 1; i >= 0; i-- {
		tenant := tenants[i]
		if tenant == nil {
			continue
		}
		if err := rt.provisioner.DeprovisionWorkspaceContainer(ctx, tenant); err != nil {
			failures = append(failures, fmt.Sprintf("%s deprovision: %v", tenant.ID, err))
		}
		tenantDataDir, err := safeProviderMSPProofTenantDataDir(rt.cfg.TenantsDir(), tenant.ID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s dir: %v", tenant.ID, err))
			continue
		}
		if err := os.RemoveAll(tenantDataDir); err != nil {
			failures = append(failures, fmt.Sprintf("%s remove data: %v", tenant.ID, err))
		}
		if err := rt.registry.Delete(tenant.ID); err != nil {
			failures = append(failures, fmt.Sprintf("%s delete registry: %v", tenant.ID, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("cleanup failures: %s", strings.Join(failures, "; "))
	}
	return nil
}

func safeProviderMSPProofTenantDataDir(tenantsDir, tenantID string) (string, error) {
	tenantsDir = strings.TrimSpace(tenantsDir)
	tenantID = strings.TrimSpace(tenantID)
	if tenantsDir == "" {
		return "", fmt.Errorf("tenants dir is required")
	}
	if tenantID == "" || strings.ContainsAny(tenantID, `/\`) {
		return "", fmt.Errorf("unsafe tenant id %q", tenantID)
	}
	cleanTenantsDir, err := filepath.Abs(tenantsDir)
	if err != nil {
		return "", fmt.Errorf("resolve tenants dir: %w", err)
	}
	candidate := filepath.Join(cleanTenantsDir, tenantID)
	if !strings.HasPrefix(candidate, cleanTenantsDir+string(os.PathSeparator)) {
		return "", fmt.Errorf("tenant data dir escaped tenants dir")
	}
	return candidate, nil
}

func providerMSPProofPlanVersion(cfg *cloudcp.CPConfig) string {
	if cfg == nil || strings.TrimSpace(cfg.ProviderMSPPlanVersion) == "" {
		return "msp_starter"
	}
	return strings.TrimSpace(cfg.ProviderMSPPlanVersion)
}

func providerMSPBaseDomainFromURL(baseURL string) string {
	domain := strings.TrimSpace(baseURL)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(domain, prefix) {
			domain = strings.TrimPrefix(domain, prefix)
			break
		}
	}
	if idx := strings.IndexAny(domain, ":/"); idx >= 0 {
		domain = domain[:idx]
	}
	return domain
}

func (rt *providerMSPProofRuntime) providerMSPProofTenantPublicURL(tenantID string) string {
	baseDomain := providerMSPBaseDomainFromURL(rt.cfg.BaseURL)
	if baseDomain == "" {
		return ""
	}
	return "https://" + strings.TrimSpace(tenantID) + "." + baseDomain
}

func printProviderMSPProofReport(report *providerMSPProofReport) {
	if report == nil {
		fmt.Println("provider_msp_control_plane_proof_ok=false")
		return
	}
	fmt.Println("provider_msp_control_plane_proof_ok=true")
	fmt.Printf("account_id=%s\n", report.AccountID)
	fmt.Printf("account_name=%s\n", report.AccountName)
	fmt.Printf("owner_user_id=%s\n", report.OwnerUserID)
	fmt.Printf("owner_email=%s\n", report.OwnerEmail)
	fmt.Printf("plan_version=%s\n", report.PlanVersion)
	fmt.Printf("plan_source=%s\n", report.PlanSource)
	fmt.Printf("license_id=%s\n", report.LicenseID)
	fmt.Printf("license_email=%s\n", report.LicenseEmail)
	fmt.Printf("workspace_limit=%d\n", report.WorkspaceLimit)
	fmt.Printf("workspace_count=%d\n", report.WorkspaceCount)
	fmt.Printf("dockerless_provisioning=%t\n", report.DockerlessProvisioning)
	fmt.Printf("runtime_container_verified=%t\n", report.RuntimeContainerVerified)
	fmt.Printf("handoff_exchange_verified=%t\n", report.HandoffExchangeVerified)
	fmt.Printf("install_token_boundary_verified=%t\n", report.InstallTokenBoundaryOK)
	fmt.Printf("setup_facts_token_use_visible=%t\n", report.SetupFactsTokenUseVisible)
	fmt.Printf("agent_report_ingest_verified=%t\n", report.AgentReportIngestVerified)
	fmt.Printf("token_rotation_verified=%t\n", report.TokenRotationVerified)
	fmt.Printf("cleanup=%t\n", report.Cleanup)
	for _, workspace := range report.Workspaces {
		fmt.Printf("workspace=%s display_name=%q state=%s plan_version=%s container_id=%s public_url=%s install_type=%s install_token_id=%s install_command_generated=%t agent_token_auth_verified=%t setup_facts_token_use_visible=%t agent_report_ingest_verified=%t agent_report_agent_id=%s agent_report_hostname=%s token_rotation_verified=%t rotated_install_token_id=%s old_install_token_rejected=%t rotated_agent_report_verified=%t handoff_exchange_verified=%t handoff_target_path=%s\n",
			workspace.TenantID,
			workspace.DisplayName,
			workspace.State,
			workspace.PlanVersion,
			workspace.ContainerID,
			workspace.PublicURL,
			workspace.InstallType,
			workspace.InstallTokenID,
			workspace.InstallCommandGenerated,
			workspace.AgentTokenAuthVerified,
			workspace.SetupFactsTokenUseVisible,
			workspace.AgentReportIngestVerified,
			workspace.AgentReportAgentID,
			workspace.AgentReportHostname,
			workspace.TokenRotationVerified,
			workspace.RotatedInstallTokenID,
			workspace.OldInstallTokenRejected,
			workspace.RotatedAgentReportVerified,
			workspace.HandoffExchangeVerified,
			workspace.HandoffTargetPath,
		)
	}
}
