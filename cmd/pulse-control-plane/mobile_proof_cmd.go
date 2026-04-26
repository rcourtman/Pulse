package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp"
	cpDocker "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/entitlements"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	cpstripe "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/stripe"
	"github.com/spf13/cobra"
)

const defaultMobileProofOwnerEmail = "pulse-mobile-ga-proof@pulserelay.pro"

type mobileProofRuntime struct {
	cfg         *cloudcp.CPConfig
	registry    *registry.TenantRegistry
	docker      *cpDocker.Manager
	provisioner *cpstripe.Provisioner
}

func newMobileProofCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mobile-proof",
		Short: "Operate disposable hosted workspaces for Pulse Mobile release proofs",
		Long: "Operate disposable hosted workspaces for Pulse Mobile release proofs.\n\n" +
			"These commands intentionally use the normal hosted workspace provisioner and refuse\n" +
			"customer-shaped accounts or tenants unless explicitly overridden.",
	}
	cmd.AddCommand(newMobileProofCreateAccountCmd())
	cmd.AddCommand(newMobileProofCreateWorkspaceCmd())
	cmd.AddCommand(newMobileProofDeleteWorkspaceCmd())
	cmd.AddCommand(newMobileProofPurgeAccountCmd())
	return cmd
}

func newMobileProofCreateAccountCmd() *cobra.Command {
	var accountID string
	var displayName string
	var allowNonProofAccount bool

	cmd := &cobra.Command{
		Use:   "create-account",
		Short: "Create a disposable MSP account for Pulse Mobile proof",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newMobileProofRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.close()

			accountID = strings.TrimSpace(accountID)
			if accountID == "" {
				generated, err := registry.GenerateAccountID()
				if err != nil {
					return fmt.Errorf("generate proof account id: %w", err)
				}
				accountID = generated
			}
			displayName = strings.TrimSpace(displayName)
			if displayName == "" {
				displayName = fmt.Sprintf("Pulse Mobile GA Proof Account %s", time.Now().UTC().Format("20060102T150405Z"))
			}

			account := &registry.Account{
				ID:          accountID,
				Kind:        registry.AccountKindMSP,
				DisplayName: displayName,
			}
			if !allowNonProofAccount && !looksLikeMobileProofAccount(account) {
				return fmt.Errorf("refusing to create non-proof account %s; display name or account id must contain proof/canary/mobile/rehearsal/seed", accountID)
			}
			if err := rt.registry.CreateAccount(account); err != nil {
				return fmt.Errorf("create mobile proof account: %w", err)
			}

			fmt.Printf("account_id=%s\n", account.ID)
			fmt.Printf("kind=%s\n", account.Kind)
			fmt.Printf("display_name=%s\n", account.DisplayName)
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "Optional account ID; generated when omitted")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Proof account display name; defaults to a Pulse Mobile GA Proof Account timestamp")
	cmd.Flags().BoolVar(&allowNonProofAccount, "allow-non-proof-account", false, "Allow creation of an account that does not look like a proof account")
	return cmd
}

func newMobileProofCreateWorkspaceCmd() *cobra.Command {
	var accountID string
	var displayName string
	var ownerEmail string
	var allowNonProofAccount bool

	cmd := &cobra.Command{
		Use:   "create-workspace",
		Short: "Create a disposable hosted workspace for Pulse Mobile proof",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newMobileProofRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.close()

			accountID = strings.TrimSpace(accountID)
			if accountID == "" {
				return fmt.Errorf("--account-id is required")
			}
			if strings.TrimSpace(displayName) == "" {
				displayName = fmt.Sprintf("Pulse Mobile GA Proof %s", time.Now().UTC().Format("20060102T150405Z"))
			}
			ownerEmail = strings.ToLower(strings.TrimSpace(ownerEmail))
			if ownerEmail == "" {
				ownerEmail = defaultMobileProofOwnerEmail
			}

			account, err := rt.registry.GetAccount(accountID)
			if err != nil {
				return fmt.Errorf("load proof account %s: %w", accountID, err)
			}
			if account == nil {
				return fmt.Errorf("account %s not found", accountID)
			}
			if !allowNonProofAccount && !looksLikeMobileProofAccount(account) {
				return fmt.Errorf("refusing to create mobile proof workspace under non-proof account %s; pass --allow-non-proof-account only after confirming this is not a customer account", accountID)
			}
			if !strings.Contains(strings.ToLower(displayName), "proof") && !strings.Contains(strings.ToLower(displayName), "canary") {
				return fmt.Errorf("--display-name must contain proof or canary")
			}

			tenant, err := rt.provisioner.ProvisionWorkspaceForOwner(cmd.Context(), accountID, displayName, ownerEmail)
			if err != nil {
				return fmt.Errorf("create mobile proof workspace: %w", err)
			}

			printMobileProofTenant(rt.cfg, tenant)
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "Existing proof account ID that owns the disposable workspace")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Proof workspace display name; defaults to a Pulse Mobile GA Proof timestamp")
	cmd.Flags().StringVar(&ownerEmail, "owner-email", defaultMobileProofOwnerEmail, "Owner email to seed into the hosted workspace metadata")
	cmd.Flags().BoolVar(&allowNonProofAccount, "allow-non-proof-account", false, "Allow creation under an account that does not look like a proof account")
	return cmd
}

func newMobileProofDeleteWorkspaceCmd() *cobra.Command {
	var tenantID string
	var allowNonProofTenant bool

	cmd := &cobra.Command{
		Use:   "delete-workspace",
		Short: "Soft-delete a disposable hosted workspace created for Pulse Mobile proof",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newMobileProofRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.close()

			tenantID = strings.TrimSpace(tenantID)
			if tenantID == "" {
				return fmt.Errorf("--tenant-id is required")
			}

			tenant, err := rt.registry.Get(tenantID)
			if err != nil {
				return fmt.Errorf("load tenant %s: %w", tenantID, err)
			}
			if tenant == nil {
				return fmt.Errorf("tenant %s not found", tenantID)
			}
			account, err := rt.registry.GetAccount(tenant.AccountID)
			if err != nil {
				return fmt.Errorf("load tenant account %s: %w", tenant.AccountID, err)
			}
			if !allowNonProofTenant && !looksLikeMobileProofTenant(tenant, account) {
				return fmt.Errorf("refusing to delete non-proof tenant %s; pass --allow-non-proof-tenant only after confirming this is not a customer workspace", tenantID)
			}

			previousState := tenant.State
			tenant.State = registry.TenantStateDeleting
			if err := rt.registry.Update(tenant); err != nil {
				return fmt.Errorf("mark tenant deleting: %w", err)
			}
			if err := rt.provisioner.DeprovisionWorkspaceContainer(cmd.Context(), tenant); err != nil {
				return fmt.Errorf("deprovision tenant container: %w", err)
			}
			tenant.ContainerID = ""
			tenant.State = registry.TenantStateDeleted
			if err := rt.registry.Update(tenant); err != nil {
				return fmt.Errorf("mark tenant deleted: %w", err)
			}

			fmt.Printf("tenant_id=%s\n", tenant.ID)
			fmt.Printf("account_id=%s\n", tenant.AccountID)
			fmt.Printf("previous_state=%s\n", previousState)
			fmt.Printf("state=%s\n", tenant.State)
			return nil
		},
	}

	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Disposable proof tenant ID to delete")
	cmd.Flags().BoolVar(&allowNonProofTenant, "allow-non-proof-tenant", false, "Allow deletion of a tenant that does not look like a proof tenant")
	return cmd
}

func newMobileProofPurgeAccountCmd() *cobra.Command {
	var accountID string
	var allowNonProofAccount bool
	var allowNonProofTenant bool

	cmd := &cobra.Command{
		Use:   "purge-account",
		Short: "Hard-delete a disposable Pulse Mobile proof account and its workspaces",
		RunE: func(cmd *cobra.Command, args []string) error {
			rt, err := newMobileProofRuntime(cmd.Context())
			if err != nil {
				return err
			}
			defer rt.close()

			accountID = strings.TrimSpace(accountID)
			if accountID == "" {
				return fmt.Errorf("--account-id is required")
			}

			account, err := rt.registry.GetAccount(accountID)
			if err != nil {
				return fmt.Errorf("load proof account %s: %w", accountID, err)
			}
			if account == nil {
				return fmt.Errorf("account %s not found", accountID)
			}
			if !allowNonProofAccount && !looksLikeMobileProofAccount(account) {
				return fmt.Errorf("refusing to purge non-proof account %s; pass --allow-non-proof-account only after confirming this is not a customer account", accountID)
			}

			tenants, err := rt.registry.ListByAccountID(accountID)
			if err != nil {
				return fmt.Errorf("list proof account workspaces: %w", err)
			}
			for _, tenant := range tenants {
				if tenant == nil {
					continue
				}
				if !allowNonProofTenant && !looksLikeMobileProofTenant(tenant, account) {
					return fmt.Errorf("refusing to purge non-proof tenant %s; pass --allow-non-proof-tenant only after confirming this is not a customer workspace", tenant.ID)
				}
			}

			purgedTenants := 0
			for _, tenant := range tenants {
				if tenant == nil {
					continue
				}
				previousState := tenant.State
				if err := rt.provisioner.DeprovisionWorkspaceContainer(cmd.Context(), tenant); err != nil {
					return fmt.Errorf("deprovision tenant %s container: %w", tenant.ID, err)
				}
				tenantDataDir, err := safeMobileProofTenantDataDir(rt.cfg.TenantsDir(), tenant.ID)
				if err != nil {
					return err
				}
				if err := os.RemoveAll(tenantDataDir); err != nil {
					return fmt.Errorf("remove tenant %s data dir: %w", tenant.ID, err)
				}
				if err := rt.registry.Delete(tenant.ID); err != nil {
					return fmt.Errorf("delete tenant %s registry row: %w", tenant.ID, err)
				}
				purgedTenants++
				fmt.Printf("tenant_purged=%s previous_state=%s account_id=%s\n", tenant.ID, previousState, tenant.AccountID)
			}

			if err := rt.registry.DeleteAccount(accountID); err != nil {
				return fmt.Errorf("delete proof account %s: %w", accountID, err)
			}

			fmt.Printf("account_purged=%s\n", account.ID)
			fmt.Printf("tenant_purged_count=%d\n", purgedTenants)
			return nil
		},
	}

	cmd.Flags().StringVar(&accountID, "account-id", "", "Disposable proof account ID to purge")
	cmd.Flags().BoolVar(&allowNonProofAccount, "allow-non-proof-account", false, "Allow purging an account that does not look like a proof account")
	cmd.Flags().BoolVar(&allowNonProofTenant, "allow-non-proof-tenant", false, "Allow purging a tenant that does not look like a proof tenant")
	return cmd
}

func newMobileProofRuntime(ctx context.Context) (*mobileProofRuntime, error) {
	cfg, err := cloudcp.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load control plane config: %w", err)
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

	dockerMgr, err := cpDocker.NewManager(cpDocker.ManagerConfig{
		Image:                    cfg.PulseImage,
		Network:                  cfg.DockerNetwork,
		BaseDomain:               mobileProofBaseDomainFromURL(cfg.BaseURL),
		TrialActivationPublicKey: cfg.TrialActivationPublicKey,
		TrustedProxyCIDRs:        cfg.TrustedProxyCIDRs,
		MemoryLimit:              cfg.TenantMemoryLimit,
		CPUShares:                cfg.TenantCPUShares,
		TenantLogMaxSize:         cfg.TenantLogMaxSize,
		TenantLogMaxFile:         cfg.TenantLogMaxFile,
	})
	if err != nil {
		reg.Close()
		return nil, fmt.Errorf("create docker manager: %w", err)
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
	)

	return &mobileProofRuntime{
		cfg:         cfg,
		registry:    reg,
		docker:      dockerMgr,
		provisioner: provisioner,
	}, nil
}

func safeMobileProofTenantDataDir(tenantsDir, tenantID string) (string, error) {
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

func (rt *mobileProofRuntime) close() {
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

func looksLikeMobileProofAccount(account *registry.Account) bool {
	if account == nil {
		return false
	}
	return containsMobileProofMarker(account.ID, account.DisplayName) ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(account.ID)), "a_msp_") ||
		strings.Contains(strings.ToLower(strings.TrimSpace(account.ID)), "ownerseed")
}

func looksLikeMobileProofTenant(tenant *registry.Tenant, account *registry.Account) bool {
	if tenant == nil {
		return false
	}
	return containsMobileProofMarker(
		tenant.ID,
		tenant.AccountID,
		tenant.Email,
		tenant.DisplayName,
		tenant.StripeCustomerID,
		tenant.StripeSubscriptionID,
		tenant.StripePriceID,
		tenant.PlanVersion,
	) || looksLikeMobileProofAccount(account)
}

func containsMobileProofMarker(values ...string) bool {
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" {
			continue
		}
		for _, marker := range []string{"mobile", "proof", "canary", "rehearsal", "seed"} {
			if strings.Contains(normalized, marker) {
				return true
			}
		}
	}
	return false
}

func mobileProofBaseDomainFromURL(baseURL string) string {
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

func printMobileProofTenant(cfg *cloudcp.CPConfig, tenant *registry.Tenant) {
	if tenant == nil {
		return
	}
	baseDomain := ""
	if cfg != nil {
		baseDomain = mobileProofBaseDomainFromURL(cfg.BaseURL)
	}
	publicURL := ""
	if baseDomain != "" {
		publicURL = fmt.Sprintf("https://%s.%s", strings.ToLower(tenant.ID), baseDomain)
	}

	fmt.Printf("tenant_id=%s\n", tenant.ID)
	fmt.Printf("account_id=%s\n", tenant.AccountID)
	fmt.Printf("display_name=%s\n", tenant.DisplayName)
	fmt.Printf("state=%s\n", tenant.State)
	fmt.Printf("plan_version=%s\n", tenant.PlanVersion)
	fmt.Printf("container_id=%s\n", tenant.ContainerID)
	if publicURL != "" {
		fmt.Printf("public_url=%s\n", publicURL)
		fmt.Printf("onboarding_url=%s/api/onboarding/qr\n", publicURL)
	}
}
