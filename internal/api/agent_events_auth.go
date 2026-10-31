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
// Session/proxy authentication is unchanged. The callback runs on the stream
// goroutine; it never writes an HTTP auth error after streaming has begun.
func agentEventTokenRevalidator(cfg *config.Config, req *http.Request) func() bool {
	revalidate := monitoringTokenSnapshotRevalidator(cfg, req)
	if revalidate == nil {
		return nil
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
func tokenStreamSnapshotRevalidator(cfg *config.Config, req *http.Request, scope string) func() (*config.APITokenRecord, bool) {
	admitted := getAPITokenRecordFromRequest(req)
	if admitted == nil {
		return nil
	}
	raw, provided := explicitAPITokenFromRequest(req)
	orgID := GetOrgID(req.Context())
	source, tenantInventory := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
	tenantInventory = tenantInventory && source.orgID != "" && source.orgID != "default"
	checker := NewAuthorizationChecker(nil)

	return func() (*config.APITokenRecord, bool) {
		if !provided || raw == "" {
			return nil, false
		}
		config.Mu.RLock()
		defer config.Mu.RUnlock()

		live := cfg
		if tenantInventory {
			live = tenantTokenConfigForRequest(req)
			if live == nil {
				return nil, false
			}
		}
		// Reuse authentication's exclusive metadata ownership and detached
		// snapshot. A tenant stream must not fall back to another inventory
		// when its current owner disappears or revokes the credential.
		current, ok := validateAPITokenLocked(live, raw)
		if !ok || (tenantInventory && tenantTokenConfigForRequest(req) != live) {
			return nil, false
		}
		if current.ID != admitted.ID || apiTokenAuthenticatedUser(current) != apiTokenAuthenticatedUser(admitted) ||
			!current.HasScope(scope) || !checker.CheckAccess(current, "", orgID).Allowed {
			return nil, false
		}
		return current, true
	}
}
