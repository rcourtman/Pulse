package apihttp

import (
	"encoding/json"
	"net/http"
	"strings"

	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

// EnsureScope admits sessions and API tokens carrying the required scope.
func EnsureScope(w http.ResponseWriter, r *http.Request, scope string) bool {
	return EnsureAnyScope(w, r, scope)
}

// EnsureAnyScope admits sessions and API tokens carrying any required scope.
func EnsureAnyScope(w http.ResponseWriter, r *http.Request, scopes ...string) bool {
	normalized := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			return true
		}
		normalized = append(normalized, scope)
	}
	if len(normalized) == 0 {
		return true
	}

	record := internalauth.GetAPIToken(r.Context())
	if record == nil {
		return true
	}
	for _, scope := range normalized {
		if record.HasScope(scope) {
			return true
		}
	}

	if len(normalized) == 1 {
		RespondMissingScope(w, normalized[0])
		return false
	}
	if w != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":          "missing_scope",
			"requiredScopes": normalized,
		})
	}
	return false
}

// RespondMissingScope writes the canonical single-scope denial envelope.
func RespondMissingScope(w http.ResponseWriter, scope string) {
	if w == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":         "missing_scope",
		"requiredScope": scope,
	})
}
