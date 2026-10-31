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
	admitted := getAPITokenRecordFromRequest(req)
	if admitted == nil {
		return nil
	}
	raw, provided := explicitAPITokenFromRequest(req)
	orgID := GetOrgID(req.Context())
	source, tenantInventory := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
	tenantInventory = tenantInventory && source.orgID != "" && source.orgID != "default"
	checker := NewAuthorizationChecker(nil)

	return func() bool {
		if !provided || raw == "" {
			return false
		}
		config.Mu.RLock()
		defer config.Mu.RUnlock()

		live := cfg
		if tenantInventory {
			live = tenantTokenConfigForRequest(req)
			if live == nil {
				return false
			}
		}
		// Reuse authentication's exclusive metadata ownership and detached
		// snapshot. A tenant stream must not fall back to another inventory
		// when its current owner disappears or revokes the credential.
		current, ok := validateAPITokenLocked(live, raw)
		if !ok || (tenantInventory && tenantTokenConfigForRequest(req) != live) {
			return false
		}
		if current.ID != admitted.ID || apiTokenAuthenticatedUser(current) != apiTokenAuthenticatedUser(admitted) ||
			!current.HasScope(config.ScopeMonitoringRead) || !checker.CheckAccess(current, "", orgID).Allowed {
			return false
		}
		// In particular, withdrawing ai:execute must redact the very next
		// action/approval frame, even though monitoring:read still permits it.
		attachAPITokenRecord(req, current)
		return true
	}
}
