package api

import (
	"fmt"

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

// DefaultAuthorizationChecker implements AuthorizationChecker with the default logic.
type DefaultAuthorizationChecker struct {
	// orgLoader is used to load organization data for membership checks.
	orgLoader OrganizationLoader
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

// TokenCanAccessOrg checks if an API token is authorized to access the specified organization.
// It uses the token's CanAccessOrg method and logs warnings for legacy tokens.
func (c *DefaultAuthorizationChecker) TokenCanAccessOrg(token *config.APITokenRecord, orgID string) bool {
	if token == nil {
		// No token means session-based auth - defer to user membership check
		return true
	}

	// Check if token can access the org
	canAccess := token.CanAccessOrg(orgID)

	// Log warning for legacy tokens with wildcard access
	if token.IsLegacyToken() && orgID != "default" {
		log.Warn().
			Str("token_id", token.ID).
			Str("token_name", token.Name).
			Str("org_id", orgID).
			Msg("Legacy token with wildcard access used for non-default org - consider binding to specific org")
	}

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

	// If no org loader is configured, preserve legacy default-org behavior
	// for deployments that do not yet persist org membership.
	if c.orgLoader == nil {
		if orgID == "default" {
			log.Warn().
				Str("user_id", userID).
				Str("org_id", orgID).
				Msg("No organization loader configured, allowing default-org access for legacy compatibility")
			return true
		}
		log.Warn().
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("No organization loader configured, denying access to non-default org")
		return false
	}

	org, err := c.orgLoader.GetOrganization(orgID)
	if err != nil {
		if orgID == "default" {
			log.Warn().
				Err(err).
				Str("user_id", userID).
				Str("org_id", orgID).
				Msg("Failed to load default organization; allowing legacy fallback access")
			return true
		}
		log.Error().
			Err(err).
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("Failed to load organization for access check")
		return false
	}

	if org == nil {
		if orgID == "default" {
			log.Warn().
				Str("user_id", userID).
				Str("org_id", orgID).
				Msg("Default organization metadata missing; allowing legacy fallback access")
			return true
		}
		log.Debug().
			Str("user_id", userID).
			Str("org_id", orgID).
			Msg("Organization not found for access check")
		return false
	}

	// Legacy default orgs may not have member metadata; preserve access until
	// membership is explicitly configured.
	if orgID == "default" && len(org.Members) == 0 {
		return true
	}

	canAccess := org.CanUserAccess(userID)
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

	// IsLegacyToken indicates if the access was granted via a legacy wildcard token.
	IsLegacyToken bool
}

// CheckAccess performs a comprehensive authorization check for a request.
func (c *DefaultAuthorizationChecker) CheckAccess(token *config.APITokenRecord, userID, orgID string) AuthorizationResult {
	// Check token-based access first
	if token != nil {
		if !token.CanAccessOrg(orgID) {
			return AuthorizationResult{
				Allowed: false,
				Reason:  "Token is not authorized for this organization",
			}
		}
		return AuthorizationResult{
			Allowed:       true,
			Reason:        "Token authorized for organization",
			IsLegacyToken: token.IsLegacyToken(),
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
