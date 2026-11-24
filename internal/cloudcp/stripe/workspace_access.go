package stripe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// The control plane shares org.json with the running workspace, and a save over
// a snapshot that is no longer current is refused. Each attempt reloads the
// organization and derives the revocation from what it holds then, so nothing
// from an earlier snapshot is written back.
const workspaceAccessSaveAttempts = 5

// RevokeWorkspaceAccess removes an account member from every workspace of the
// account.
//
// A workspace authorizes each request against its own org.json. That file is
// seeded from the account's members when the workspace is provisioned and is
// never read back from the registry, so a removal that only reaches the
// registry stops new handoffs but leaves the member's row, and with it every
// workspace session the member already holds, in place.
func (p *Provisioner) RevokeWorkspaceAccess(ctx context.Context, accountID, userID string) error {
	return p.limitWorkspaceAccess(ctx, accountID, userID, "")
}

// LowerWorkspaceAccess caps an account member's role in every workspace of the
// account at the organization role of their new, lower account role. The
// handoff admits a member with the role stored in the workspace, so without
// this a demoted member keeps the earlier role in open and in new sessions.
func (p *Provisioner) LowerWorkspaceAccess(ctx context.Context, accountID, userID string, role registry.MemberRole) error {
	return p.limitWorkspaceAccess(ctx, accountID, userID, models.OrganizationRoleFromAccountRole(string(role)))
}

// SyncWorkspaceAccess ensures an account member is present in every workspace
// of the account with at least the organization role mapped from their account
// role. This keeps undoing a demotion or re-inviting a removed member working
// across existing workspaces while preserving any higher workspace-local grant.
func (p *Provisioner) SyncWorkspaceAccess(ctx context.Context, accountID, userID string, role registry.MemberRole) error {
	accountID = strings.TrimSpace(accountID)
	userID = strings.TrimSpace(userID)
	if p == nil || p.registry == nil {
		return fmt.Errorf("workspace access is unavailable")
	}
	if accountID == "" || userID == "" {
		return fmt.Errorf("account id and user id are required")
	}
	floor := models.OrganizationRoleFromAccountRole(string(role))
	if !models.IsValidOrganizationRole(floor) {
		return fmt.Errorf("invalid account role %q", role)
	}

	email := ""
	user, err := p.registry.GetUser(userID)
	if err != nil {
		return fmt.Errorf("get account user %s: %w", userID, err)
	}
	if user != nil {
		email = strings.ToLower(strings.TrimSpace(user.Email))
	}
	tenants, err := p.registry.ListByAccountID(accountID)
	if err != nil {
		return fmt.Errorf("list workspaces for %s: %w", accountID, err)
	}

	var problems []error
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			problems = append(problems, err)
			break
		}
		if err := p.syncWorkspaceMemberAccess(accountID, tenant.ID, userID, email, floor); err != nil {
			problems = append(problems, fmt.Errorf("workspace %s: %w", tenant.ID, err))
		}
	}
	return errors.Join(problems...)
}

func (p *Provisioner) syncWorkspaceMemberAccess(accountID, tenantID, userID, email string, floor models.OrganizationRole) error {
	tenantDataDir := p.tenantDataDir(tenantID)
	orgFile := filepath.Join(tenantDataDir, "orgs", tenantID, "org.json")
	if _, err := os.Stat(orgFile); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	mtp := config.NewMultiTenantPersistence(tenantDataDir)
	for attempt := 1; ; attempt++ {
		org, err := mtp.LoadOrganizationStrict(tenantID)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("load organization metadata: %w", err)
		}
		changed := syncOrganizationMemberAccess(org, userID, email, floor)
		if !changed {
			return nil
		}
		err = mtp.SaveOrganization(org)
		if errors.Is(err, config.ErrOrganizationChanged) && attempt < workspaceAccessSaveAttempts {
			continue
		}
		if err != nil {
			return fmt.Errorf("save organization metadata: %w", err)
		}
		if err := p.ensureHostedRuntimeFileOwnership(orgFile); err != nil {
			return fmt.Errorf("set organization metadata ownership: %w", err)
		}
		log.Info().
			Str("account_id", accountID).
			Str("tenant_id", tenantID).
			Str("user_id", userID).
			Str("role_floor", string(floor)).
			Msg("Synced account member role into workspace organization")
		return nil
	}
}

func syncOrganizationMemberAccess(org *models.Organization, userID, email string, floor models.OrganizationRole) bool {
	matches := func(storedUserID string) bool {
		storedUserID = strings.TrimSpace(storedUserID)
		if storedUserID == "" {
			return false
		}
		return storedUserID == userID ||
			(email != "" && strings.Contains(storedUserID, "@") && strings.ToLower(storedUserID) == email)
	}

	matched := false
	changed := false
	for i := range org.Members {
		if !matches(org.Members[i].UserID) {
			continue
		}
		matched = true
		if !models.OrganizationRoleAtLeast(org.Members[i].Role, floor) {
			org.Members[i].Role = floor
			changed = true
		}
		if org.Members[i].Email == "" && email != "" {
			org.Members[i].Email = email
			changed = true
		}
	}
	if !matched {
		org.Members = append(org.Members, models.OrganizationMember{
			UserID:  userID,
			Email:   email,
			Role:    floor,
			AddedAt: time.Now().UTC(),
			AddedBy: userID,
		})
		changed = true
	}
	return changed
}

// limitWorkspaceAccess takes no runtime lifecycle lease: a rollout, a health
// check or an unfinished recovery on one workspace must not keep access in
// place in the others, or keep the member in the account. Every workspace is
// attempted; the caller keeps the account membership as it was when any of
// them fails, so the change can be retried.
func (p *Provisioner) limitWorkspaceAccess(ctx context.Context, accountID, userID string, ceiling models.OrganizationRole) error {
	accountID = strings.TrimSpace(accountID)
	userID = strings.TrimSpace(userID)
	if p == nil || p.registry == nil {
		return fmt.Errorf("workspace access is unavailable")
	}
	if accountID == "" || userID == "" {
		return fmt.Errorf("account id and user id are required")
	}

	email := ""
	user, err := p.registry.GetUser(userID)
	if err != nil {
		return fmt.Errorf("get account user %s: %w", userID, err)
	}
	if user != nil {
		email = strings.ToLower(strings.TrimSpace(user.Email))
	}
	tenants, err := p.registry.ListByAccountID(accountID)
	if err != nil {
		return fmt.Errorf("list workspaces for %s: %w", accountID, err)
	}

	var problems []error
	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			problems = append(problems, err)
			break
		}
		if err := p.limitWorkspaceMemberAccess(accountID, tenant.ID, userID, email, ceiling); err != nil {
			problems = append(problems, fmt.Errorf("workspace %s: %w", tenant.ID, err))
		}
	}
	return errors.Join(problems...)
}

func (p *Provisioner) limitWorkspaceMemberAccess(accountID, tenantID, userID, email string, ceiling models.OrganizationRole) error {
	tenantDataDir := p.tenantDataDir(tenantID)
	orgFile := filepath.Join(tenantDataDir, "orgs", tenantID, "org.json")
	if _, err := os.Stat(orgFile); errors.Is(err, os.ErrNotExist) {
		// No organization metadata: the workspace admits nobody yet.
		return nil
	}

	mtp := config.NewMultiTenantPersistence(tenantDataDir)
	for attempt := 1; ; attempt++ {
		org, err := mtp.LoadOrganizationStrict(tenantID)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("load organization metadata: %w", err)
		}
		changed, err := p.limitOrganizationMemberAccess(org, accountID, userID, email, ceiling)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		err = mtp.SaveOrganization(org)
		if errors.Is(err, config.ErrOrganizationChanged) && attempt < workspaceAccessSaveAttempts {
			continue
		}
		if err != nil {
			return fmt.Errorf("save organization metadata: %w", err)
		}
		if err := p.ensureHostedRuntimeFileOwnership(orgFile); err != nil {
			return fmt.Errorf("set organization metadata ownership: %w", err)
		}
		event := "Removed account member from workspace organization"
		if ceiling != "" {
			event = "Lowered account member role in workspace organization"
		}
		log.Info().
			Str("account_id", accountID).
			Str("tenant_id", tenantID).
			Str("user_id", userID).
			Str("role_ceiling", string(ceiling)).
			Str("owner_user_id", org.OwnerUserID).
			Msg(event)
		return nil
	}
}

// limitOrganizationMemberAccess removes the principal from the organization,
// or, given a ceiling, lowers a stored role above it. It matches what the
// handoff admits: the stable user ID, or a legacy record keyed by the email.
// An organization the principal owns passes to another account owner.
func (p *Provisioner) limitOrganizationMemberAccess(org *models.Organization, accountID, userID, email string, ceiling models.OrganizationRole) (bool, error) {
	matches := func(storedUserID string) bool {
		storedUserID = strings.TrimSpace(storedUserID)
		if storedUserID == "" {
			return false
		}
		return storedUserID == userID ||
			(email != "" && strings.Contains(storedUserID, "@") && strings.ToLower(storedUserID) == email)
	}
	exceedsCeiling := func(role models.OrganizationRole) bool {
		return models.IsValidOrganizationRole(role) && !models.OrganizationRoleAtLeast(ceiling, role)
	}

	changed := false
	newOwnerUserID := ""
	if matches(org.OwnerUserID) && ceiling != models.OrgRoleOwner {
		successor, err := p.workspaceOwnerSuccessor(accountID, userID)
		if err != nil {
			return false, err
		}
		// Without another account owner the workspace is left without one
		// rather than with the owner whose access is being taken away.
		org.OwnerUserID, org.OwnerEmail = "", ""
		if successor != nil {
			newOwnerUserID = strings.TrimSpace(successor.ID)
			org.OwnerUserID = newOwnerUserID
			org.OwnerEmail = strings.ToLower(strings.TrimSpace(successor.Email))
		}
		changed = true
	}

	members := make([]models.OrganizationMember, 0, len(org.Members)+1)
	newOwnerIsMember := false
	for _, member := range org.Members {
		switch {
		case matches(member.UserID):
			if ceiling == "" {
				changed = true
				continue
			}
			if exceedsCeiling(member.Role) {
				member.Role = ceiling
				changed = true
			}
		case newOwnerUserID != "" && strings.TrimSpace(member.UserID) == newOwnerUserID:
			member.Role = models.OrgRoleOwner
			newOwnerIsMember = true
		}
		members = append(members, member)
	}
	if newOwnerUserID != "" && !newOwnerIsMember {
		members = append(members, models.OrganizationMember{
			UserID:  newOwnerUserID,
			Email:   org.OwnerEmail,
			Role:    models.OrgRoleOwner,
			AddedAt: time.Now().UTC(),
			AddedBy: newOwnerUserID,
		})
	}

	// An invitation still pending in the workspace could otherwise be accepted
	// from a session the principal already holds.
	invitations := make([]models.OrganizationInvitation, 0, len(org.PendingInvitations))
	for _, invitation := range org.PendingInvitations {
		if matches(invitation.UserID) {
			if ceiling == "" {
				changed = true
				continue
			}
			if exceedsCeiling(invitation.Role) {
				invitation.Role = ceiling
				changed = true
			}
		}
		invitations = append(invitations, invitation)
	}

	if changed {
		org.Members = members
		org.PendingInvitations = invitations
	}
	return changed, nil
}

// workspaceOwnerSuccessor returns the account's longest-standing owner other
// than userID, the owner buildSeededTenantOrganization gives a workspace that
// is provisioned without a named one. It returns nil when there is none.
func (p *Provisioner) workspaceOwnerSuccessor(accountID, userID string) (*registry.User, error) {
	memberships, err := p.registry.ListMembersByAccount(accountID)
	if err != nil {
		return nil, fmt.Errorf("list account members for %s: %w", accountID, err)
	}
	var successor *registry.AccountMembership
	for _, membership := range memberships {
		if membership == nil || membership.Role != registry.MemberRoleOwner {
			continue
		}
		candidateID := strings.TrimSpace(membership.UserID)
		if candidateID == "" || candidateID == userID {
			continue
		}
		if successor == nil ||
			membership.CreatedAt.Before(successor.CreatedAt) ||
			(membership.CreatedAt.Equal(successor.CreatedAt) && candidateID < strings.TrimSpace(successor.UserID)) {
			successor = membership
		}
	}
	if successor == nil {
		return nil, nil
	}
	user, err := p.registry.GetUser(successor.UserID)
	if err != nil {
		return nil, fmt.Errorf("get account user %s: %w", successor.UserID, err)
	}
	if user == nil || strings.TrimSpace(user.ID) == "" {
		return nil, fmt.Errorf("account membership references missing user %s", successor.UserID)
	}
	return user, nil
}
