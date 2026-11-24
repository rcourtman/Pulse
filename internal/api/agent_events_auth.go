package api

import (
	"net/http"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func (r *Router) handleAgentEvents(w http.ResponseWriter, req *http.Request) {
	r.agentEventBroadcaster.handleAgentEvents(w, req, agentEventTokenRevalidator(r.config, req))
}

// agentEventTokenRevalidator keeps a long-lived token stream bound to its
// admitted credential and organization, not its connection-time snapshot.
// Session withdrawal is checked too. The callback runs on the stream
// goroutine; it never writes an HTTP auth error after streaming has begun.
func agentEventTokenRevalidator(cfg *config.Config, req *http.Request) func() bool {
	revalidate := monitoringTokenSnapshotRevalidator(cfg, req)
	if revalidate == nil {
		// A signed-in user's stream also follows the organization access
		// policy, so lost membership or default access ends it.
		return combineAuthorization(sessionReadRevalidator(req), orgAccessRevalidator(req))
	}
	admittedContext := req.Context()
	return func() bool {
		current, ok := revalidate()
		if !ok {
			return false
		}
		// Replace over the admitted context, never the previous refresh. Only
		// the SSE goroutine owns this request and its disclosure snapshot.
		refreshed := req.WithContext(admittedContext)
		attachAPITokenRecord(refreshed, current)
		*req = *refreshed
		return true
	}
}

// monitoringTokenSnapshotRevalidator is shared by long-lived monitoring reads.
// It returns detached current authority without mutating the admitted request;
// WebSocket writes outlive the HTTP handler and must not race its middleware.
// A nil callback preserves existing session/proxy authentication behaviour.
func monitoringTokenSnapshotRevalidator(cfg *config.Config, req *http.Request) func() (*config.APITokenRecord, bool) {
	return tokenStreamSnapshotRevalidator(cfg, req, config.ScopeMonitoringRead)
}

// tokenStreamSnapshotRevalidator preserves the stream's admitted credential,
// principal and organization while checking its current operation-specific
// scope. Callers must retain any additional route policy (e.g. instance admin).
func tokenStreamSnapshotRevalidator(cfg *config.Config, req *http.Request, scopes ...string) func() (*config.APITokenRecord, bool) {
	admitted := getAPITokenRecordFromRequest(req)
	if admitted == nil {
		return nil
	}
	raw, provided := explicitAPITokenFromRequest(req)
	orgID := GetOrgID(req.Context())
	source, tenantInventory := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
	tenantInventory = tenantInventory && source.orgID != "" && source.orgID != "default"
	// Recheck the policy that admitted the request, so the stream of a token
	// that acts for its owner also ends when the owner loses membership.
	// A request TenantMiddleware did not admit rechecks the binding only.
	var checker AuthorizationChecker = NewAuthorizationChecker(nil)
	if policy, ok := req.Context().Value(orgAccessCheckerContextKey{}).(AuthorizationChecker); ok && policy != nil {
		checker = policy
	}

	return func() (*config.APITokenRecord, bool) {
		if !provided || raw == "" {
			return nil, false
		}
		config.Mu.RLock()

		live := cfg
		if tenantInventory {
			live = tenantTokenConfigForRequest(req)
			if live == nil {
				config.Mu.RUnlock()
				return nil, false
			}
		}
		// Reuse authentication's exclusive metadata ownership and detached
		// snapshot. A tenant stream must not fall back to another inventory
		// when its current owner disappears or revokes the credential.
		current, ok := validateAPITokenLocked(live, raw)
		if !ok || (tenantInventory && tenantTokenConfigForRequest(req) != live) {
			config.Mu.RUnlock()
			return nil, false
		}
		// Release the global config lock before checking organization
		// membership, which may read organization metadata from disk.
		config.Mu.RUnlock()
		// Keep the route's existing scope alternatives (notably mobile chat)
		// rather than requiring a new grant or pinning one interchangeable scope.
		hasScope := false
		for _, scope := range scopes {
			hasScope = hasScope || current.HasScope(scope)
		}
		if current.ID != admitted.ID || apiTokenAuthenticatedUser(current) != apiTokenAuthenticatedUser(admitted) ||
			!hasScope || !checker.CheckAccess(current, "", orgID).Allowed {
			return nil, false
		}
		return current, true
	}
}
