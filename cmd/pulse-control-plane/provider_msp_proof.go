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
	AllowNonProofWorkspaceName bool
}

type providerMSPProofRuntime struct {
	cfg         *cloudcp.CPConfig
	registry    *registry.TenantRegistry
	docker      *cpDocker.Manager
	provisioner *cpstripe.Provisioner
}

type providerMSPProofWorkspace struct {
	TenantID                  string
	DisplayName               string
	State                     string
	PlanVersion               string
	ContainerID               string
	PublicURL                 string
	InstallType               string
	InstallToken              string
	InstallTokenID            string
	InstallCommandGenerated   bool
	AgentTokenAuthVerified    bool
	SetupFactsTokenUseVisible bool
	HandoffExchangeVerified   bool
	HandoffTargetPath         string
}

type providerMSPProofReport struct {
	AccountID                 string
	AccountName               string
	OwnerUserID               string
	OwnerEmail                string
	PlanVersion               string
	PlanSource                string
	WorkspaceLimit            int
	WorkspaceCount            int
	Workspaces                []providerMSPProofWorkspace
	DockerlessProvisioning    bool
	RuntimeContainerVerified  bool
	HandoffExchangeVerified   bool
	InstallTokenBoundaryOK    bool
	SetupFactsTokenUseVisible bool
	AgentReportIngestVerified bool
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
	if !cfg.IsProviderHostedMSP() {
		return nil, fmt.Errorf("provider MSP proof requires CP_CONTROL_PLANE_MODE=%s", cloudcp.ControlPlaneModeProviderHostedMSP)
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
	dockerMgr, err = cpDocker.NewManager(cpDocker.ManagerConfig{
		Image:                    cfg.PulseImage,
		Network:                  cfg.DockerNetwork,
		BaseDomain:               providerMSPProofBaseDomainFromURL(cfg.BaseURL),
		TrialActivationPublicKey: cfg.TrialActivationPublicKey,
		TrustedProxyCIDRs:        cfg.TrustedProxyCIDRs,
		MemoryLimit:              cfg.TenantMemoryLimit,
		CPUShares:                cfg.TenantCPUShares,
		TenantLogMaxSize:         cfg.TenantLogMaxSize,
		TenantLogMaxFile:         cfg.TenantLogMaxFile,
	})
	if err != nil {
		if !cfg.AllowDockerlessProvisioning {
			reg.Close()
			return nil, fmt.Errorf("create docker manager: %w", err)
		}
		dockerMgr = nil
	}

	hostedEntitlements := entitlements.NewService(reg, cfg.BaseURL, cfg.TrialActivationPrivateKey)
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
		WorkspaceLimit:            bootstrap.WorkspaceLimit,
		RuntimeContainerVerified:  true,
		HandoffExchangeVerified:   true,
		InstallTokenBoundaryOK:    true,
		SetupFactsTokenUseVisible: true,
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

	exchangedTargetPath, err := rt.verifyProviderMSPProofHandoff(ctx, tenant, ownerUserID, targetPath)
	if err != nil {
		return providerMSPProofWorkspace{}, err
	}

	return providerMSPProofWorkspace{
		TenantID:                  tenant.ID,
		DisplayName:               tenant.DisplayName,
		State:                     string(tenant.State),
		PlanVersion:               tenant.PlanVersion,
		ContainerID:               tenant.ContainerID,
		PublicURL:                 publicURL,
		InstallType:               install.InstallType,
		InstallToken:              install.Token,
		InstallTokenID:            install.TokenID,
		InstallCommandGenerated:   true,
		AgentTokenAuthVerified:    tokenAuthVerified,
		SetupFactsTokenUseVisible: setupFactsVisible,
		HandoffExchangeVerified:   exchangedTargetPath == targetPath,
		HandoffTargetPath:         exchangedTargetPath,
	}, nil
}

func configPersistenceForProviderMSPProof(tenantDataDir string) *runtimeconfig.ConfigPersistence {
	return runtimeconfig.NewConfigPersistence(tenantDataDir)
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
	baseDomain := providerMSPProofBaseDomainFromURL(rt.cfg.BaseURL)
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
			if strings.TrimSpace(token.ID) == workspace.InstallTokenID {
				found = true
				break
			}
		}
		if !found {
			return false, fmt.Errorf("tenant %s missing proof install token id %s", workspace.TenantID, workspace.InstallTokenID)
		}
	}
	for _, left := range workspaces {
		for _, right := range workspaces {
			if left.TenantID == right.TenantID {
				continue
			}
			if left.InstallTokenID == right.InstallTokenID {
				return false, fmt.Errorf("tenant %s and %s reused install token id %s", left.TenantID, right.TenantID, left.InstallTokenID)
			}
			rightTenantDataDir, err := safeProviderMSPProofTenantDataDir(rt.cfg.TenantsDir(), right.TenantID)
			if err != nil {
				return false, err
			}
			accepted, err := providerMSPProofTenantAcceptsToken(rightTenantDataDir, left.InstallToken)
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

func providerMSPProofBaseDomainFromURL(baseURL string) string {
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
	baseDomain := providerMSPProofBaseDomainFromURL(rt.cfg.BaseURL)
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
	fmt.Printf("workspace_limit=%d\n", report.WorkspaceLimit)
	fmt.Printf("workspace_count=%d\n", report.WorkspaceCount)
	fmt.Printf("dockerless_provisioning=%t\n", report.DockerlessProvisioning)
	fmt.Printf("runtime_container_verified=%t\n", report.RuntimeContainerVerified)
	fmt.Printf("handoff_exchange_verified=%t\n", report.HandoffExchangeVerified)
	fmt.Printf("install_token_boundary_verified=%t\n", report.InstallTokenBoundaryOK)
	fmt.Printf("setup_facts_token_use_visible=%t\n", report.SetupFactsTokenUseVisible)
	fmt.Printf("agent_report_ingest_verified=%t\n", report.AgentReportIngestVerified)
	fmt.Printf("cleanup=%t\n", report.Cleanup)
	for _, workspace := range report.Workspaces {
		fmt.Printf("workspace=%s display_name=%q state=%s plan_version=%s container_id=%s public_url=%s install_type=%s install_token_id=%s install_command_generated=%t agent_token_auth_verified=%t setup_facts_token_use_visible=%t handoff_exchange_verified=%t handoff_target_path=%s\n",
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
			workspace.HandoffExchangeVerified,
			workspace.HandoffTargetPath,
		)
	}
}
