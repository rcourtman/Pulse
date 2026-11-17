package api

import (
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// tokenMutationFailure is returned while config.Mu is held, but is written
// only after releasing it. Refusing a still-valid credential's lost privilege
// must not tell clients that the credential itself has become invalid.
type tokenMutationFailure struct {
	status       int
	message      string
	missingScope bool
}

func (f *tokenMutationFailure) Error() string { return f.message }

func (f *tokenMutationFailure) write(w http.ResponseWriter) {
	if f.missingScope {
		respondMissingScope(w, config.ScopeSettingsWrite)
		return
	}
	http.Error(w, f.message, f.status)
}

// currentTokenMutationCallerLocked rechecks the route's admitted token under
// the same config writer lock as the inventory transaction. Body reads and
// waiting for this lock can outlive revocation, rotation or a scope edit.
// The detached request record identifies the credential, not a lasting grant.
// Non-token authentication remains the route's responsibility.
func (r *Router) currentTokenMutationCallerLocked(req *http.Request) (*config.APITokenRecord, *tokenMutationFailure) {
	admitted := getAPITokenRecordFromRequest(req)
	if admitted == nil {
		return nil, nil
	}
	invalid := &tokenMutationFailure{status: http.StatusUnauthorized, message: "Invalid API token"}
	live := r.config
	if source, ok := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource); ok && source.orgID != "" && source.orgID != "default" {
		// The shared issuer also serves organization-scoped relay credentials.
		// Never fall back to another inventory when that owner is withdrawn.
		live = tenantTokenConfigForRequest(req)
	}
	if live == nil {
		return nil, invalid
	}
	for i := range live.APITokens {
		current := &live.APITokens[i]
		if !constantTimeStringEqual(current.Hash, admitted.Hash) {
			continue
		}
		// Authentication chooses the first matching credential hash. A newly
		// shadowing record must not let an older context choose a later grant.
		if current.ID != admitted.ID {
			return nil, invalid
		}
		role := strings.TrimSpace(current.Metadata[internalauth.RuntimeRoleMetadataKey])
		if current.IsExpired() || apiTokenAuthenticatedUser(current) != apiTokenAuthenticatedUser(admitted) ||
			internalauth.ValidateRoleScopes(role, current.Scopes) != nil {
			return nil, invalid
		}
		if !current.HasScope(config.ScopeSettingsWrite) {
			return nil, &tokenMutationFailure{status: http.StatusForbidden, message: "Missing settings:write scope", missingScope: true}
		}
		// Keep the route's original instance boundary, without imposing one on
		// the existing org-scoped issuer. A binding edit cannot turn an admitted
		// instance request into permission to mutate the global inventory.
		if (tokenActsForInstance(admitted) && !tokenActsForInstance(current)) ||
			!NewAuthorizationChecker(nil).CheckAccess(current, "", GetOrgID(req.Context())).Allowed {
			return nil, &tokenMutationFailure{status: http.StatusForbidden, message: "Admin privileges required"}
		}
		clone := current.Clone()
		attachAPITokenRecord(req, &clone)
		return &clone, nil
	}
	return nil, invalid
}
