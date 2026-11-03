package api

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// AuthorizationChecker provides methods to check if a user or token can access an organization.
type AuthorizationChecker interface {
	// TokenCanAccessOrg checks if an API token is authorized to access the specified organization.
	TokenCanAccessOrg(token *config.APITokenRecord, orgID string) bool

	// UserCanAccessOrg checks if a user is a member of the specified organization.
	UserCanAccessOrg(userID, orgID string) bool

	// CheckAccess performs a comprehensive authorization check for a request.
	CheckAccess(token *config.APITokenRecord, userID, orgID string) AuthorizationResult
}

// contextAuthorizationChecker is implemented by checkers whose decision depends
// on request-bound authority (a proxy administrator, the dev bypass) and that
// can pick an organization for a request which did not name one.
type contextAuthorizationChecker interface {
	CheckAccessContext(ctx context.Context, token *config.APITokenRecord, userID, orgID string) AuthorizationResult
	LandingOrgID(ctx context.Context, userID string) string
}

// DefaultAuthorizationChecker implements AuthorizationChecker with the default logic.
type DefaultAuthorizationChecker struct {
	// orgLoader is used to load organization data for membership checks.
	orgLoader OrganizationLoader

	// hostedMode marks a runtime that serves a single hosted tenant. Its
	// default organization is not shared with other tenants.
	hostedMode bool

	// instanceAdmin reports whether the request's user administers the
	// instance. Nil means no user does.
	instanceAdmin func(ctx context.Context, userID string) bool
}

// OrganizationLoader provides methods to load organization data.
type OrganizationLoader interface {
	// GetOrganization returns the organization with the specified ID.
	GetOrganization(orgID string) (*models.Organization, error)
}

// NewAuthorizationChecker creates a new DefaultAuthorizationChecker.
func NewAuthorizationChecker(loader OrganizationLoader) *DefaultAuthorizationChecker {
	return &DefaultAuthorizationChecker{
		orgLoader: loader,
	}
}

// SetHostedMode marks the checker as serving a hosted single-tenant runtime.
func (c *DefaultAuthorizationChecker) SetHostedMode(hosted bool) {
	c.hostedMode = hosted
}

// SetInstanceAdminResolver sets the rule deciding who administers the instance.
func (c *DefaultAuthorizationChecker) SetInstanceAdminResolver(resolve func(ctx context.Context, userID string) bool) {
	c.instanceAdmin = resolve
}

// strictOrganizationLoader loads organization metadata without synthesizing
// an empty organization when org.json is missing. Access decisions use it so
// a missing or unreadable file never stands in for real membership data.
type strictOrganizationLoader interface {
	GetOrganizationStrict(orgID string) (*models.Organization, error)
}

// organizationIDLister lists organization IDs without reading their metadata.
type organizationIDLister interface {
	ListOrganizationIDs() ([]string, error)
}

// MultiTenantOrganizationLoader implements OrganizationLoader using MultiTenantPersistence.
type MultiTenantOrganizationLoader struct {
	persistence *config.MultiTenantPersistence
}

// NewMultiTenantOrganizationLoader creates a new organization loader.
func NewMultiTenantOrganizationLoader(persistence *config.MultiTenantPersistence) *MultiTenantOrganizationLoader {
	return &MultiTenantOrganizationLoader{
		persistence: persistence,
	}
}

// GetOrganization loads the organization with the specified ID.
func (l *MultiTenantOrganizationLoader) GetOrganization(orgID string) (*models.Organization, error) {
	if l.persistence == nil {
		return nil, fmt.Errorf("no persistence configured")
	}
	return l.persistence.LoadOrganization(orgID)
}

// GetOrganizationStrict loads org.json and fails when it does not exist.
func (l *MultiTenantOrganizationLoader) GetOrganizationStrict(orgID string) (*models.Organization, error) {
	if l.persistence == nil {
		return nil, fmt.Errorf("no persistence configured")
	}
	return l.persistence.LoadOrganizationStrict(orgID)
}

// ListOrganizationIDs lists the persisted organization IDs, default included.
func (l *MultiTenantOrganizationLoader) ListOrganizationIDs() ([]string, error) {
	if l.persistence == nil {
		return nil, fmt.Errorf("no persistence configured")
	}
	return l.persistence.ListOrganizationIDs()
}

// defaultOrgRestricted reports whether the default organization is one tenant
// among several in this process. Self-hosted multi-organization mode shares one
// process between organizations, so the default organization's estate belongs
// to the instance, not to every user who can sign in. Single-tenant and hosted
// runtimes have no other tenant to separate it from.
func (c *DefaultAuthorizationChecker) defaultOrgRestricted() bool {
	return IsMultiTenantEnabled() && !c.hostedMode
}

func (c *DefaultAuthorizationChecker) loadOrganizationForAccess(orgID string) (*models.Organization, error) {
	if c.orgLoader == nil {
		return nil, fmt.Errorf("no organization loader configured")
	}
	if strict, ok := c.orgLoader.(strictOrganizationLoader); ok {
		return strict.GetOrganizationStrict(orgID)
	}
	return c.orgLoader.GetOrganization(orgID)
}

// userDefaultOrgAccess decides default-organization access for a signed-in
// user. In multi-organization mode only instance administrators and explicit
// members of the default organization qualify. Having no other membership is
// not an entitlement: a user's last membership can be removed, and a pending
// invitation is not a membership.
func (c *DefaultAuthorizationChecker) userDefaultOrgAccess(ctx context.Context, userID string) AuthorizationResult {
	if !c.defaultOrgRestricted() {
		return AuthorizationResult{
			Allowed: true,
			Reason:  "Default organization is accessible to all authenticated users",
		}
	}
	if c.instanceAdmin != nil && c.instanceAdmin(ctx, userID) {
		return AuthorizationResult{
			Allowed: true,
			Reason:  "Instance administrators can access the default organization",
		}
	}
	org, err := c.loadOrganizationForAccess("default")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Error().
				Err(err).
				Str("user_id", userID).
				Msg("Failed to load default organization for access check")
		}
		return AuthorizationResult{
			Allowed: false,
			Reason:  "User is not a member of the default organization",
		}
	}
	// Membership is the entitlement. An owner field on the default
	// organization grants nothing by itself, so removing a member always
	// revokes their access.
	if org != nil && org.HasMemberUserID(userID) {
		return AuthorizationResult{
			Allowed: true,
			Reason:  "User is a member of the default organization",
		}
	}
	return AuthorizationResult{
		Allowed: false,
		Reason:  "User is not a member of the default organization",
	}
}

// LandingOrgID returns the organization a request that names none should use
// when the user cannot use the default organization: the first organization,
// by ID, the user belongs to. It returns "" when the default organization is
// usable or the user belongs to no organization.
func (c *DefaultAuthorizationChecker) LandingOrgID(ctx context.Context, userID string) string {
	userID = strings.TrimSpace(userID)
	if userID == "" || !c.defaultOrgRestricted() {
		return ""
	}
	if c.userDefaultOrgAccess(ctx, userID).Allowed {
		return ""
	}
	lister, ok := c.orgLoader.(organizationIDLister)
	if !ok {
		return ""
	}
	orgIDs, err := lister.ListOrganizationIDs()
	if err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Failed to list organizations for landing organization")
		return ""
	}
	for _, orgID := range orgIDs {
		if orgID == "default" {
			continue
		}
		if c.UserCanAccessOrg(userID, orgID) {
			return orgID
		}
	}
	return ""
}

// TokenCanAccessOrg checks if an API token is authorized to access the specified organization.
func (c *DefaultAuthorizationChecker) TokenCanAccessOrg(token *config.APITokenRecord, orgID string) bool {
	if token == nil {
		// No token means session-based auth - defer to user membership check
		return true
	}

	// Check if token can access the org
	canAccess := token.CanAccessOrg(orgID)

	if !canAccess {
		log.Debug().
			Str("token_id", token.ID).
			Str("token_name", token.Name).
			Str("org_id", orgID).
			Strs("bound_orgs", token.GetBoundOrgs()).
			Msg("Token denied access to organization")
	}

	return canAccess
}

// UserCanAccessOrg checks if a user is a member of the specified organization.
func (c *DefaultAuthorizationChecker) UserCanAccessOrg(userID, orgID string) bool {
	if userID == "" {
		return false
	}

	// Single-tenant deployments keep the default organization open to every
	// authenticated user; multi-organization mode limits it.
	if orgID == "default" {
		return c.userDefaultOrgAccess(context.Background(), userID).Allowed
	}

	// Fail closed when membership data cannot be resolved.
	if c.orgLoader == nil {
		log.Warn().
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("No organization loader configured, denying access")
		return false
	}

	org, err := c.loadOrganizationForAccess(orgID)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("Failed to load organization for access check")
		return false
	}

	if org == nil {
		log.Debug().
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("Organization not found for access check")
		return false
	}

	canAccess := org.CanUserIDAccess(userID)
	if !canAccess {
		log.Debug().
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("User is not a member of the organization")
	}

	return canAccess
}

// AuthorizationResult contains the result of an authorization check.
type AuthorizationResult struct {
	// Allowed indicates if access is allowed.
	Allowed bool

	// Reason provides a human-readable reason for the decision.
	Reason string
}

// CheckAccess performs a comprehensive authorization check for a request.
// Prefer CheckAccessContext where a request context exists: request-bound
// authority such as a proxy administrator is only visible there.
func (c *DefaultAuthorizationChecker) CheckAccess(token *config.APITokenRecord, userID, orgID string) AuthorizationResult {
	return c.CheckAccessContext(context.Background(), token, userID, orgID)
}

// CheckAccessContext is the organization access policy every caller shares:
// HTTP tenant selection, action authority, organization handlers and live
// streams.
func (c *DefaultAuthorizationChecker) CheckAccessContext(ctx context.Context, token *config.APITokenRecord, userID, orgID string) AuthorizationResult {
	if ctx == nil {
		ctx = context.Background()
	}
	// The default organization stays accessible to legacy unbound tokens, and
	// to authenticated users unless multi-organization mode limits it to
	// instance administrators and its members. A token explicitly bound to organizations is
	// scoped to those organizations only — binding expresses intent to limit,
	// so it must not implicitly reach the default org's data (bind "default"
	// explicitly to grant that). Without this, a client-bound MSP token that
	// leaks from a client site could read the provider's own default-org
	// estate.
	if orgID == "default" {
		if token != nil {
			if token.IsLegacyToken() {
				return AuthorizationResult{
					Allowed: true,
					Reason:  "Default organization is accessible to unbound tokens",
				}
			}
			// Org-bound tokens fall through to the explicit binding check.
		} else if userID != "" {
			return c.userDefaultOrgAccess(ctx, userID)
		}
	}

	// Check token-based access first
	if token != nil {
		if !token.CanAccessOrg(orgID) {
			return AuthorizationResult{
				Allowed: false,
				Reason:  "Token is not authorized for this organization",
			}
		}
		return AuthorizationResult{
			Allowed: true,
			Reason:  "Token authorized for organization",
		}
	}

	// Fall back to user-based access
	if userID != "" {
		if c.UserCanAccessOrg(userID, orgID) {
			return AuthorizationResult{
				Allowed: true,
				Reason:  "User is a member of the organization",
			}
		}
		return AuthorizationResult{
			Allowed: false,
			Reason:  "User is not a member of the organization",
		}
	}

	// No token and no user - deny access
	return AuthorizationResult{
		Allowed: false,
		Reason:  "No authentication context provided",
	}
}

// CanAccessOrg implements websocket.OrgAuthChecker for use with the WebSocket hub.
func (c *DefaultAuthorizationChecker) CanAccessOrg(userID string, tokenInterface interface{}, orgID string) bool {
	// Convert token interface to APITokenRecord
	var token *config.APITokenRecord
	if tokenInterface != nil {
		if t, ok := tokenInterface.(*config.APITokenRecord); ok {
			token = t
		}
	}

	result := c.CheckAccess(token, userID, orgID)
	return result.Allowed
}

// CanAccessOrgContext implements websocket.OrgAuthContextChecker.
func (c *DefaultAuthorizationChecker) CanAccessOrgContext(ctx context.Context, userID string, tokenInterface interface{}, orgID string) bool {
	var token *config.APITokenRecord
	if tokenInterface != nil {
		if t, ok := tokenInterface.(*config.APITokenRecord); ok {
			token = t
		}
	}
	return c.CheckAccessContext(ctx, token, userID, orgID).Allowed
}
