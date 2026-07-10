package cloudcp

import (
	"context"
	"fmt"
	"strings"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
)

// ProviderMSPPortalLinkOptions describes a portal sign-in link request for an
// account member or pending invitee on an MSP control plane.
type ProviderMSPPortalLinkOptions struct {
	Email string
}

// ProviderMSPPortalLinkResult is the operator-facing result of minting a
// portal sign-in link.
type ProviderMSPPortalLinkResult struct {
	Email        string
	AccessState  string
	Role         string
	MagicLinkURL string
}

// ProviderMSPPortalLink mints a one-time portal sign-in link for an existing
// account member or a pending invitee. It exists so operators without a
// configured email provider still have a way to deliver sign-in links to
// teammates; the owner path is covered by BootstrapProviderMSP.
func ProviderMSPPortalLink(ctx context.Context, cfg *CPConfig, opts ProviderMSPPortalLinkOptions) (*ProviderMSPPortalLinkResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, fmt.Errorf("control plane config is required")
	}
	if !cfg.IsMSPControlPlane() {
		return nil, fmt.Errorf("provider MSP portal-link requires CP_CONTROL_PLANE_MODE=%s or %s", ControlPlaneModeProviderHostedMSP, ControlPlaneModePulseHostedMSP)
	}

	email, err := normalizeProviderMSPOwnerEmail(opts.Email)
	if err != nil {
		return nil, fmt.Errorf("email is invalid: %w", err)
	}

	reg, err := registry.NewTenantRegistry(cfg.ControlPlaneDir())
	if err != nil {
		return nil, fmt.Errorf("open tenant registry: %w", err)
	}
	defer reg.Close()

	accessState, role, err := providerMSPPortalAccessForEmail(reg, email)
	if err != nil {
		return nil, err
	}

	magicLinks, err := cpauth.NewService(cfg.ControlPlaneDir())
	if err != nil {
		return nil, fmt.Errorf("init magic link service: %w", err)
	}
	defer magicLinks.Close()

	token, err := magicLinks.GeneratePortalToken(email, "")
	if err != nil {
		return nil, fmt.Errorf("generate portal magic link: %w", err)
	}
	magicLinkURL := cpauth.BuildVerifyURL(cfg.BaseURL, token)
	if magicLinkURL == "" {
		return nil, fmt.Errorf("build portal magic link URL")
	}

	return &ProviderMSPPortalLinkResult{
		Email:        email,
		AccessState:  accessState,
		Role:         role,
		MagicLinkURL: magicLinkURL,
	}, nil
}

// providerMSPPortalAccessForEmail confirms the email already has portal
// access (an account membership) or a pending invitation, so the CLI cannot
// be used to mint sessions for arbitrary addresses.
func providerMSPPortalAccessForEmail(reg *registry.TenantRegistry, email string) (accessState string, role string, err error) {
	user, err := reg.GetUserByEmail(email)
	if err != nil {
		return "", "", fmt.Errorf("lookup user: %w", err)
	}
	if user != nil {
		accountIDs, err := reg.ListAccountsByUser(user.ID)
		if err != nil {
			return "", "", fmt.Errorf("list accounts for user: %w", err)
		}
		for _, accountID := range accountIDs {
			membership, err := reg.GetMembership(accountID, user.ID)
			if err != nil {
				return "", "", fmt.Errorf("lookup membership: %w", err)
			}
			if membership != nil {
				return "member", string(membership.Role), nil
			}
		}
	}

	invitations, err := reg.ListInvitationsByEmail(email)
	if err != nil {
		return "", "", fmt.Errorf("list invitations: %w", err)
	}
	for _, invitation := range invitations {
		if invitation == nil {
			continue
		}
		return "invited", string(invitation.Role), nil
	}

	return "", "", fmt.Errorf("%s is not an account member and has no pending invitation; invite them from the portal Access tab first", strings.ToLower(email))
}
