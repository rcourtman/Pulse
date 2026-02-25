package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

type OrganizationContextKey string

const (
	OrgIDContextKey OrganizationContextKey = "org_id"
	OrgContextKey   OrganizationContextKey = "org_object"
	subStateActive  string                 = "active"
	subStateGrace   string                 = "grace"
	subStateTrial   string                 = "trial"
)

// TenantMiddleware extracts the organization ID from the request and
// sets up the context for multi-tenant isolation.
type TenantMiddleware struct {
	persistence *config.MultiTenantPersistence
	authChecker AuthorizationChecker
	hostedMode  bool
}

// TenantMiddlewareConfig holds configuration for the tenant middleware.
type TenantMiddlewareConfig struct {
	Persistence *config.MultiTenantPersistence
	AuthChecker AuthorizationChecker
	HostedMode  bool
}

func resolveTenantOrgID(r *http.Request) string {
	orgID := ""
	explicitOrg := false

	if r != nil {
		orgID = strings.TrimSpace(r.Header.Get("X-Pulse-Org-ID"))
		explicitOrg = orgID != ""
		if orgID == "" {
			if cookie, err := r.Cookie(CookieNameOrgID); err == nil {
				orgID = strings.TrimSpace(cookie.Value)
				explicitOrg = orgID != ""
			}
		}
	}

	if orgID == "" {
		orgID = "default"
	}

	// Cloud/agent UX: if no org was explicitly selected, prefer an org-bound token.
	// This enables tenant-scoped install commands that only need --token.
	if !explicitOrg && orgID == "default" && r != nil {
		if tokenVal := auth.GetAPIToken(r.Context()); tokenVal != nil {
			if t, ok := tokenVal.(*config.APITokenRecord); ok && t != nil {
				if strings.TrimSpace(t.OrgID) != "" {
					orgID = strings.TrimSpace(t.OrgID)
				} else if len(t.OrgIDs) == 1 && strings.TrimSpace(t.OrgIDs[0]) != "" {
					orgID = strings.TrimSpace(t.OrgIDs[0])
				}
			}
		}
	}

	return orgID
}

func NewTenantMiddleware(p *config.MultiTenantPersistence) *TenantMiddleware {
	return &TenantMiddleware{persistence: p}
}

// NewTenantMiddlewareWithConfig creates a new TenantMiddleware with full configuration.
func NewTenantMiddlewareWithConfig(cfg TenantMiddlewareConfig) *TenantMiddleware {
	return &TenantMiddleware{
		persistence: cfg.Persistence,
		authChecker: cfg.AuthChecker,
		hostedMode:  cfg.HostedMode,
	}
}

// SetAuthChecker sets the authorization checker for the middleware.
func (m *TenantMiddleware) SetAuthChecker(checker AuthorizationChecker) {
	m.authChecker = checker
}

func (m *TenantMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Extract Org ID
		// Priority:
		// 1. Header: X-Pulse-Org-ID (for API clients/agents)
		// 2. Cookie: pulse_org_id (for browser session)
		// 3. Fallback: "default" (for backward compatibility)

		orgID := resolveTenantOrgID(r)

		var loadedOrg *models.Organization

		// 2. Validate Organization Exists (only for non-default orgs)
		// This must check existence WITHOUT creating directories to prevent DoS.
		// It also must run BEFORE feature checks to ensure invalid org IDs return 400 (Bad Request)
		// rather than 501/402 (feature disabled/unlicensed).
		if orgID != "default" && m.persistence != nil {
			if !m.persistence.OrgExists(orgID) {
				writeJSONError(w, http.StatusBadRequest, "invalid_org", "Invalid Organization ID")
				return
			}
		}

		// 2b. Load org metadata and enforce lifecycle status for non-default orgs.
		// If loading fails, keep backward-compatible behavior by falling back later.
		if orgID != "default" && m.persistence != nil {
			org, loadErr := m.persistence.LoadOrganization(orgID)
			if loadErr == nil && org != nil {
				loadedOrg = org
				status := models.NormalizeOrgStatus(org.Status)
				if status == models.OrgStatusSuspended || status == models.OrgStatusPendingDeletion {
					writeJSONError(w, http.StatusForbidden, "org_suspended", "Organization is suspended")
					return
				}
			}
		}

		// 3. Tenant access gating for non-default orgs.
		// Hosted mode: tenant routing is infrastructure — check subscription lifecycle instead of FeatureMultiTenant.
		// Self-hosted mode: non-default orgs require PULSE_MULTI_TENANT_ENABLED=true AND FeatureMultiTenant (Enterprise/MSP).
		if orgID != "default" {
			if m.hostedMode {
				// Hosted mode: verify the org has a valid subscription or bounded trial.
				checkCtx := context.WithValue(r.Context(), OrgIDContextKey, orgID)
				if !isHostedSubscriptionValid(checkCtx) {
					writeHostedSubscriptionRequiredError(w)
					return
				}
			} else {
				// Self-hosted mode: multi-tenant requires feature flag + Enterprise license.
				if !IsMultiTenantEnabled() {
					writeMultiTenantDisabledError(w)
					return
				}
				checkCtx := context.WithValue(r.Context(), OrgIDContextKey, orgID)
				if !hasMultiTenantFeatureForContext(checkCtx) {
					writeMultiTenantRequiredError(w)
					return
				}
			}
		}

		// 4. Authorization Check
		// Check if the authenticated user/token is allowed to access this organization.
		// Note: This runs AFTER AuthContextMiddleware, so auth context is available.
		if m.authChecker != nil {
			// Dev-only emergency bypass should skip tenant membership checks too.
			// Use request context marker to avoid leaking global bypass state across tests.
			if isAdminBypassRequest(r.Context()) {
				ctx := context.WithValue(r.Context(), OrgIDContextKey, orgID)
				org := loadedOrg
				if org == nil {
					org = &models.Organization{ID: orgID, DisplayName: orgID}
				}
				ctx = context.WithValue(ctx, OrgContextKey, org)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Get API token from context (set by AuthContextMiddleware)
			var token *config.APITokenRecord
			if tokenVal := auth.GetAPIToken(r.Context()); tokenVal != nil {
				if t, ok := tokenVal.(*config.APITokenRecord); ok {
					token = t
				}
			}

			// Get user ID from context (set by AuthContextMiddleware)
			userID := auth.GetUser(r.Context())

			// Only perform authorization check if we have auth context
			// If no auth context, the route's RequireAuth will handle authentication errors
			if token != nil || userID != "" {
				// Perform authorization check using the interface method
				result := m.authChecker.CheckAccess(token, userID, orgID)
				if !result.Allowed {
					log.Warn().
						Str("org_id", orgID).
						Str("user_id", userID).
						Str("reason", result.Reason).
						Msg("Unauthorized access attempt to organization")
					writeJSONError(w, http.StatusForbidden, "access_denied", result.Reason)
					return
				}

				// Log warning for legacy tokens accessing non-default orgs
				if result.IsLegacyToken {
					log.Warn().
						Str("org_id", orgID).
						Msg("Legacy token with wildcard access used - consider binding to specific org")
				}
			}
		}

		// 5. Inject into Context
		ctx := context.WithValue(r.Context(), OrgIDContextKey, orgID)

		org := loadedOrg
		if org == nil {
			org = &models.Organization{ID: orgID, DisplayName: orgID}
		}
		ctx = context.WithValue(ctx, OrgContextKey, org)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// Helper to get OrgID from context
func GetOrgID(ctx context.Context) string {
	if id, ok := ctx.Value(OrgIDContextKey).(string); ok {
		return id
	}
	return "default"
}

// Helper to get Organization from context
func GetOrganization(ctx context.Context) *models.Organization {
	if org, ok := ctx.Value(OrgContextKey).(*models.Organization); ok {
		return org
	}
	return &models.Organization{ID: "default", DisplayName: "Default Organization"}
}

// isHostedSubscriptionValid checks whether a hosted Cloud tenant has a valid subscription
// or a bounded trial. This replaces the FeatureMultiTenant check for hosted mode, where
// tenant routing is infrastructure rather than a paid feature.
func isHostedSubscriptionValid(ctx context.Context) bool {
	svc := getLicenseServiceForContext(ctx)
	subState := strings.ToLower(strings.TrimSpace(svc.SubscriptionState()))
	eval := svc.Evaluator()
	hasTrialEnd := eval != nil && eval.TrialEndsAt() != nil
	switch subState {
	case subStateActive, subStateGrace:
		return true
	case subStateTrial:
		return hasTrialEnd
	default:
		return false
	}
}

// writeHostedSubscriptionRequiredError writes a 402 response for Cloud tenants
// whose subscription is not active (expired, canceled, or missing).
func writeHostedSubscriptionRequiredError(w http.ResponseWriter) {
	writePaymentRequired(w, map[string]interface{}{
		"error":   "subscription_required",
		"message": "Your Cloud subscription is not active. Please check your billing status.",
	})
}
