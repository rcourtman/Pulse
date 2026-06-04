package cloudcp

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"strings"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// ProviderMSPBootstrapOptions describes the first-run owner/account bootstrap
// for provider-hosted MSP control planes.
type ProviderMSPBootstrapOptions struct {
	AccountID         string
	AccountName       string
	OwnerEmail        string
	GenerateMagicLink bool
}

// ProviderMSPBootstrapResult is the operator-facing result of a provider MSP
// bootstrap run.
type ProviderMSPBootstrapResult struct {
	AccountID      string
	AccountName    string
	OwnerUserID    string
	OwnerEmail     string
	PlanVersion    string
	PlanSource     string
	LicenseID      string
	LicenseEmail   string
	WorkspaceLimit int
	MagicLinkURL   string
}

// BootstrapProviderMSP creates or reuses the MSP account and owner identity for
// an MSP control plane. It is deliberately unavailable in normal Pulse-hosted
// mode so the MSP bootstrap path cannot leak into ordinary hosting.
func BootstrapProviderMSP(ctx context.Context, cfg *CPConfig, opts ProviderMSPBootstrapOptions) (*ProviderMSPBootstrapResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	if !cfg.IsMSPControlPlane() {
		return nil, fmt.Errorf("provider MSP bootstrap requires CP_CONTROL_PLANE_MODE=%s or %s", ControlPlaneModeProviderHostedMSP, ControlPlaneModePulseHostedMSP)
	}

	accountName := strings.TrimSpace(opts.AccountName)
	if accountName == "" {
		return nil, fmt.Errorf("account name is required")
	}
	ownerEmail, err := normalizeProviderMSPOwnerEmail(opts.OwnerEmail)
	if err != nil {
		return nil, err
	}
	workspaceLimit, known := pkglicensing.WorkspaceLimitForPlan(cfg.ProviderMSPPlanVersion)
	if !known {
		return nil, fmt.Errorf("provider MSP plan %q has no known workspace limit", cfg.ProviderMSPPlanVersion)
	}

	if err := os.MkdirAll(cfg.ControlPlaneDir(), 0o755); err != nil {
		return nil, fmt.Errorf("create control-plane dir: %w", err)
	}
	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}
	defer reg.Close()

	account, err := ensureProviderMSPAccount(reg, strings.TrimSpace(opts.AccountID), accountName)
	if err != nil {
		return nil, err
	}
	user, err := ensureProviderMSPOwnerUser(reg, ownerEmail)
	if err != nil {
		return nil, err
	}
	if err := ensureProviderMSPOwnerMembership(reg, account.ID, user.ID); err != nil {
		return nil, err
	}

	magicLinkURL := ""
	if opts.GenerateMagicLink {
		magicLinks, err := cpauth.NewService(cfg.ControlPlaneDir())
		if err != nil {
			return nil, fmt.Errorf("init magic link service: %w", err)
		}
		defer magicLinks.Close()

		token, err := magicLinks.GeneratePortalToken(ownerEmail, "")
		if err != nil {
			return nil, fmt.Errorf("generate portal magic link: %w", err)
		}
		magicLinkURL = cpauth.BuildVerifyURL(cfg.BaseURL, token)
		if magicLinkURL == "" {
			return nil, fmt.Errorf("build portal magic link URL")
		}
	}

	return &ProviderMSPBootstrapResult{
		AccountID:      account.ID,
		AccountName:    account.DisplayName,
		OwnerUserID:    user.ID,
		OwnerEmail:     ownerEmail,
		PlanVersion:    cfg.ProviderMSPPlanVersion,
		PlanSource:     providerMSPPlanSourceOrDefault(cfg.ProviderMSPPlanSource),
		LicenseID:      strings.TrimSpace(cfg.ProviderMSPLicenseID),
		LicenseEmail:   strings.ToLower(strings.TrimSpace(cfg.ProviderMSPLicenseEmail)),
		WorkspaceLimit: workspaceLimit,
		MagicLinkURL:   magicLinkURL,
	}, nil
}

func normalizeProviderMSPOwnerEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("owner email is required")
	}
	parsed, err := mail.ParseAddress(raw)
	if err != nil {
		return "", fmt.Errorf("owner email is invalid: %w", err)
	}
	email := strings.ToLower(strings.TrimSpace(parsed.Address))
	if email == "" || !strings.Contains(email, "@") {
		return "", fmt.Errorf("owner email is invalid")
	}
	return email, nil
}

func providerMSPPlanSourceOrDefault(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ProviderMSPPlanSourceEnvFallback
	}
	return source
}

func ensureProviderMSPAccount(reg *registry.TenantRegistry, requestedID, accountName string) (*registry.Account, error) {
	if reg == nil {
		return nil, fmt.Errorf("registry unavailable")
	}
	if requestedID != "" {
		account, err := reg.GetAccount(requestedID)
		if err != nil {
			return nil, fmt.Errorf("lookup account: %w", err)
		}
		if account != nil {
			return updateProviderMSPAccountName(reg, account, accountName)
		}
		return createProviderMSPAccount(reg, requestedID, accountName)
	}

	accounts, err := reg.ListAccounts()
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	var mspAccounts []*registry.Account
	for _, account := range accounts {
		if account != nil && account.Kind == registry.AccountKindMSP {
			mspAccounts = append(mspAccounts, account)
		}
	}
	switch len(mspAccounts) {
	case 0:
		accountID, err := registry.GenerateAccountID()
		if err != nil {
			return nil, fmt.Errorf("generate account id: %w", err)
		}
		return createProviderMSPAccount(reg, accountID, accountName)
	case 1:
		return updateProviderMSPAccountName(reg, mspAccounts[0], accountName)
	default:
		return nil, fmt.Errorf("multiple MSP accounts exist; rerun with --account-id")
	}
}

func createProviderMSPAccount(reg *registry.TenantRegistry, accountID, accountName string) (*registry.Account, error) {
	account := &registry.Account{
		ID:          strings.TrimSpace(accountID),
		Kind:        registry.AccountKindMSP,
		DisplayName: accountName,
	}
	if account.ID == "" {
		return nil, fmt.Errorf("account id is required")
	}
	if err := reg.CreateAccount(account); err != nil {
		return nil, fmt.Errorf("create MSP account: %w", err)
	}
	return account, nil
}

func updateProviderMSPAccountName(reg *registry.TenantRegistry, account *registry.Account, accountName string) (*registry.Account, error) {
	if account.Kind != registry.AccountKindMSP {
		return nil, fmt.Errorf("account %q is %q, want %q", account.ID, account.Kind, registry.AccountKindMSP)
	}
	if account.DisplayName == accountName {
		return account, nil
	}
	account.DisplayName = accountName
	if err := reg.UpdateAccount(account); err != nil {
		return nil, fmt.Errorf("update MSP account: %w", err)
	}
	return account, nil
}

func ensureProviderMSPOwnerUser(reg *registry.TenantRegistry, ownerEmail string) (*registry.User, error) {
	user, err := reg.GetUserByEmail(ownerEmail)
	if err != nil {
		return nil, fmt.Errorf("lookup owner user: %w", err)
	}
	if user != nil {
		return user, nil
	}
	userID, err := registry.GenerateUserID()
	if err != nil {
		return nil, fmt.Errorf("generate owner user id: %w", err)
	}
	user = &registry.User{
		ID:    userID,
		Email: ownerEmail,
	}
	if err := reg.CreateUser(user); err != nil {
		reloaded, reloadErr := reg.GetUserByEmail(ownerEmail)
		if reloadErr != nil || reloaded == nil {
			return nil, fmt.Errorf("create owner user: %w", err)
		}
		user = reloaded
	}
	return user, nil
}

func ensureProviderMSPOwnerMembership(reg *registry.TenantRegistry, accountID, userID string) error {
	membership, err := reg.GetMembership(accountID, userID)
	if err != nil {
		return fmt.Errorf("lookup owner membership: %w", err)
	}
	if membership == nil {
		if err := reg.CreateMembership(&registry.AccountMembership{
			AccountID: accountID,
			UserID:    userID,
			Role:      registry.MemberRoleOwner,
		}); err != nil {
			return fmt.Errorf("create owner membership: %w", err)
		}
		return nil
	}
	if membership.Role == registry.MemberRoleOwner {
		return nil
	}
	if err := reg.UpdateMembershipRole(accountID, userID, registry.MemberRoleOwner); err != nil {
		return fmt.Errorf("promote owner membership: %w", err)
	}
	return nil
}
