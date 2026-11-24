package api

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func apiTokenBoundUser(record *config.APITokenRecord) string {
	if record == nil {
		return ""
	}
	ownerUserID := strings.TrimSpace(apiTokenOwnerUserID(*record))
	if ownerUserID == "" || strings.HasPrefix(ownerUserID, "token:") {
		return ""
	}
	return ownerUserID
}

func apiTokenAuthenticatedUser(record *config.APITokenRecord) string {
	if ownerUserID := apiTokenBoundUser(record); ownerUserID != "" {
		return ownerUserID
	}
	if record == nil || strings.TrimSpace(record.ID) == "" {
		return ""
	}
	return fmt.Sprintf("token:%s", strings.TrimSpace(record.ID))
}

// apiTokenActsForOwner reports whether a token stands in for the person who
// owns it rather than for a machine: it names a human owner and holds a scope
// that lets a phone, an Assistant client or an action reviewer act in that
// owner's name. Agent, runner and plain monitoring or settings credentials
// record who issued them but belong to the machine or integration they were
// issued for.
func apiTokenActsForOwner(record *config.APITokenRecord) bool {
	if apiTokenBoundUser(record) == "" {
		return false
	}
	for _, scope := range []string{
		config.ScopeRelayMobileAccess,
		config.ScopeAIChat,
		config.ScopeAIExecute,
		config.ScopeActionsPlan,
		config.ScopeActionsApprove,
		config.ScopeActionsExecute,
	} {
		if record.HasScope(scope) {
			return true
		}
	}
	return false
}
