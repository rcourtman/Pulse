package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

// proxyPrincipal records how proxy authentication admitted a request. The
// admin role arrives in a request header, so it is bound to the username it
// was presented with and never re-read from the request later.
type proxyPrincipal struct {
	username string
	isAdmin  bool
}

type proxyPrincipalContextKey struct{}

func attachProxyPrincipal(r *http.Request, username string, isAdmin bool) *http.Request {
	ctx := context.WithValue(r.Context(), proxyPrincipalContextKey{}, proxyPrincipal{username: username, isAdmin: isAdmin})
	return r.WithContext(ctx)
}

func proxyPrincipalFromContext(ctx context.Context) (proxyPrincipal, bool) {
	if ctx == nil {
		return proxyPrincipal{}, false
	}
	principal, ok := ctx.Value(proxyPrincipalContextKey{}).(proxyPrincipal)
	return principal, ok && principal.username != ""
}

// instanceAdminResolver returns the instance-administrator rule the
// organization access policy applies to signed-in users. A proxy-authenticated
// user is an administrator only when proxy authentication admitted that same
// username with the admin role; session users follow
// sessionUserCarriesAdminPrivileges. API tokens never qualify here: their
// organization access is decided by their bindings.
func instanceAdminResolver(cfg *config.Config) func(ctx context.Context, userID string) bool {
	return func(ctx context.Context, userID string) bool {
		userID = strings.TrimSpace(userID)
		if userID == "" || strings.HasPrefix(userID, "token:") {
			return false
		}
		if ctx != nil && isAdminBypassRequest(ctx) {
			return true
		}
		if principal, ok := proxyPrincipalFromContext(ctx); ok {
			return principal.isAdmin && principal.username == userID
		}
		return sessionUserCarriesAdminPrivileges(cfg, userID)
	}
}

// newOrgAccessChecker builds the organization access policy shared by tenant
// selection, action authority, organization handlers and live streams.
func newOrgAccessChecker(cfg *config.Config, persistence *config.MultiTenantPersistence, hostedMode bool) *DefaultAuthorizationChecker {
	var loader OrganizationLoader
	if persistence != nil {
		loader = NewMultiTenantOrganizationLoader(persistence)
	}
	checker := NewAuthorizationChecker(loader)
	checker.SetHostedMode(hostedMode)
	checker.SetInstanceAdminResolver(instanceAdminResolver(cfg))
	return checker
}

type orgAccessCheckerContextKey struct{}

// withOrgAccessChecker lets long-lived readers admitted by TenantMiddleware
// re-check the same policy after admission.
func withOrgAccessChecker(ctx context.Context, checker contextAuthorizationChecker) context.Context {
	if checker == nil {
		return ctx
	}
	return context.WithValue(ctx, orgAccessCheckerContextKey{}, checker)
}

// orgAccessRecheckInterval bounds how long a stream keeps writing after its
// user loses organization access. Writers call the revalidator before every
// frame, so it caches a granted decision for this long.
var orgAccessRecheckInterval = 10 * time.Second

// orgAccessRevalidator re-applies the organization access policy to a
// signed-in user's long-lived read (WebSocket or SSE). Membership removal,
// default-organization revocation and losing instance administration end the
// stream within orgAccessRecheckInterval. Token readers keep their own
// credential revalidation, which already rechecks organization bindings.
// It returns nil when the request has no user to recheck.
func orgAccessRevalidator(req *http.Request) func() bool {
	if req == nil {
		return nil
	}
	ctx := req.Context()
	checker, ok := ctx.Value(orgAccessCheckerContextKey{}).(contextAuthorizationChecker)
	if !ok || checker == nil {
		return nil
	}
	if getAPITokenRecordFromRequest(req) != nil {
		return nil
	}
	userID := strings.TrimSpace(auth.GetUser(ctx))
	if userID == "" {
		return nil
	}
	orgID := strings.TrimSpace(GetOrgID(ctx))
	if orgID == "" {
		orgID = "default"
	}

	var (
		mu        sync.Mutex
		checkedAt time.Time
		denied    bool
	)
	return func() bool {
		mu.Lock()
		defer mu.Unlock()
		if denied {
			return false
		}
		if !checkedAt.IsZero() && time.Since(checkedAt) < orgAccessRecheckInterval {
			return true
		}
		if !checker.CheckAccessContext(ctx, nil, userID, orgID).Allowed {
			denied = true
			return false
		}
		checkedAt = time.Now()
		return true
	}
}

// authorizeVerifiedUserForRequestOrganization applies the organization access
// policy to a user whose credentials were verified after TenantMiddleware
// admitted the request. TenantMiddleware checks only the principals that the
// auth context middleware attached; HTTP Basic credentials are verified later,
// so TenantMiddleware admitted the request's organization without a principal.
// This applies the same policy instance, with the same default-organization
// bootstrap exception, to the now-verified user before that user acts in the
// admitted organization. A request that TenantMiddleware never admitted has no
// organization context and acts in the default organization.
func authorizeVerifiedUserForRequestOrganization(req *http.Request, userID string) AuthorizationResult {
	userID = strings.TrimSpace(userID)
	if req == nil || userID == "" {
		return AuthorizationResult{Allowed: false, Reason: "No authentication context provided"}
	}
	ctx := req.Context()
	orgID, admitted := ctx.Value(OrgIDContextKey).(string)
	orgID = strings.TrimSpace(orgID)
	if !admitted || orgID == "" {
		return AuthorizationResult{Allowed: true, Reason: "No organization was admitted for this request"}
	}
	checker, ok := ctx.Value(orgAccessCheckerContextKey{}).(contextAuthorizationChecker)
	if !ok || checker == nil {
		if orgID == "default" {
			return AuthorizationResult{Allowed: true, Reason: "Default organization without an organization access policy"}
		}
		return AuthorizationResult{Allowed: false, Reason: "Organization access policy unavailable"}
	}
	result := checker.CheckAccessContext(ctx, nil, userID, orgID)
	if !result.Allowed && orgID == "default" && orgNeutralBootstrapRequest(req) {
		return AuthorizationResult{Allowed: true, Reason: "Organization-neutral route"}
	}
	return result
}

// combineAuthorization requires every non-nil check to pass.
func combineAuthorization(checks ...func() bool) func() bool {
	active := make([]func() bool, 0, len(checks))
	for _, check := range checks {
		if check != nil {
			active = append(active, check)
		}
	}
	if len(active) == 0 {
		return nil
	}
	return func() bool {
		for _, check := range active {
			if !check() {
				return false
			}
		}
		return true
	}
}

// orgNeutralBootstrapRequest reports whether a request may run in the default
// organization's context for a user who cannot use it. These
// routes let the user load the app shell, see that they are signed in, find
// and accept invitations, and sign out; none of them reads the default
// organization's monitoring data.
func orgNeutralBootstrapRequest(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	path := r.URL.Path
	switch path {
	case "/api/security/status", "/api/logout", "/api/health":
		// /api/health is public and also issues the CSRF token that
		// accepting an invitation needs.
		return true
	case "/api/orgs":
		return r.Method == http.MethodGet
	case "/ws", "/simple-stats":
		return false
	}
	if path == "/api/org-invitations" || strings.HasPrefix(path, "/api/org-invitations/") {
		return true
	}
	// The single-page app shell and its static assets. Org-scoped non-API
	// routes are excluded above, and API routes are denied by default.
	return !strings.HasPrefix(path, "/api") && path != "/debug" && !strings.HasPrefix(path, "/debug/")
}

// defaultOrgMembershipMu serializes read-modify-write changes to the default
// organization's members. Each change loads and saves the whole org.json, so
// without it an add that overlaps a removal could save its older snapshot and
// restore a revoked entitlement.
var defaultOrgMembershipMu sync.Mutex

// defaultOrgAccessMigrationFile records, in the base data directory, that
// existing users' implicit default-organization access has been converted to
// explicit membership. Its presence stops the migration from running again,
// even if the default organization's metadata is later reset.
const defaultOrgAccessMigrationFile = "default-org-access-migration.json"

// defaultOrgAccessMigrationActor marks memberships the migration created, so
// administrators can tell them from grants they made.
const defaultOrgAccessMigrationActor = "migration:default-org-access"

type defaultOrgAccessMigrationRecord struct {
	MigratedAt time.Time `json:"migratedAt"`
	Seeded     []string  `json:"seeded"`
}

// migrateDefaultOrgAccess runs once when shared-process multi-organization mode
// starts limiting the default organization. Before, every signed-in user could
// read it. Users with a role assignment in the instance's RBAC store, who
// belong to no other organization and have no pending invitation, were
// evidently using the default organization, so they become its viewer
// members. Everyone else must be granted access by an instance administrator;
// proxy-authenticated users have no stored record to migrate from.
//
// It fails without writing the marker when any organization's metadata or the
// RBAC assignments cannot be read, so a later start retries with complete
// data. A tenant directory without org.json counts as unreadable: its members
// are unknown, and treating them as absent could grant a tenant-only user the
// default organization.
func migrateDefaultOrgAccess(persistence *config.MultiTenantPersistence, manager auth.Manager, cfg *config.Config, hostedMode bool) error {
	if persistence == nil || hostedMode || !IsMultiTenantEnabled() {
		return nil
	}
	defaultOrgMembershipMu.Lock()
	defer defaultOrgMembershipMu.Unlock()
	markerPath := filepath.Join(persistence.BaseDataDir(), defaultOrgAccessMigrationFile)
	if _, err := os.Stat(markerPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check default organization access migration: %w", err)
	}

	orgIDs, err := persistence.ListOrganizationIDs()
	if err != nil {
		return fmt.Errorf("list organizations: %w", err)
	}
	elsewhere := make(map[string]struct{})
	for _, orgID := range orgIDs {
		if orgID == "default" {
			continue
		}
		org, loadErr := persistence.LoadOrganizationStrict(orgID)
		if loadErr != nil {
			return fmt.Errorf("load organization %s: %w", orgID, loadErr)
		}
		if owner := strings.TrimSpace(org.OwnerUserID); owner != "" {
			elsewhere[owner] = struct{}{}
		}
		for _, member := range org.Members {
			elsewhere[strings.TrimSpace(member.UserID)] = struct{}{}
		}
		for _, invitation := range org.PendingInvitations {
			elsewhere[strings.TrimSpace(invitation.UserID)] = struct{}{}
		}
	}

	defaultOrg, err := persistence.LoadOrganizationStrict("default")
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("load default organization: %w", err)
		}
		defaultOrg = &models.Organization{ID: "default", DisplayName: "default"}
	}

	var assignments []auth.UserRoleAssignment
	if manager != nil {
		if errorAware, ok := manager.(auth.ErrorAwareManager); ok {
			assignments, err = errorAware.GetUserAssignmentsWithError()
			if err != nil {
				return fmt.Errorf("read RBAC assignments: %w", err)
			}
		} else {
			assignments = manager.GetUserAssignments()
		}
	}

	seeded := make([]string, 0)
	if len(assignments) > 0 {
		now := time.Now().UTC()
		for _, assignment := range assignments {
			username := strings.TrimSpace(assignment.Username)
			if username == "" || strings.HasPrefix(username, "token:") || len(assignment.RoleIDs) == 0 {
				continue
			}
			if _, ok := elsewhere[username]; ok {
				continue
			}
			// Administrators reach the default organization by authority.
			if sessionUserCarriesAdminPrivileges(cfg, username) || defaultOrg.CanUserIDAccess(username) {
				continue
			}
			defaultOrg.Members = append(defaultOrg.Members, models.OrganizationMember{
				UserID:  username,
				Role:    models.OrgRoleViewer,
				AddedAt: now,
				AddedBy: defaultOrgAccessMigrationActor,
			})
			seeded = append(seeded, username)
		}
	}
	sort.Strings(seeded)
	if len(seeded) > 0 {
		if err := persistence.SaveOrganization(defaultOrg); err != nil {
			return fmt.Errorf("save default organization: %w", err)
		}
	}

	record, err := json.MarshalIndent(defaultOrgAccessMigrationRecord{MigratedAt: time.Now().UTC(), Seeded: seeded}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode default organization access migration: %w", err)
	}
	if err := os.WriteFile(markerPath, record, 0o600); err != nil {
		return fmt.Errorf("record default organization access migration: %w", err)
	}
	log.Warn().
		Int("seeded_members", len(seeded)).
		Strs("seeded_users", seeded).
		Msg("Default organization access now requires instance administration or default organization membership; existing users without another organization were added as members. Review them under Settings > Organization access.")
	return nil
}
