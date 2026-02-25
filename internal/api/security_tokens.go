package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

type apiTokenDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Suffix     string     `json:"suffix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	Scopes     []string   `json:"scopes"`
}

func toAPITokenDTO(record config.APITokenRecord) apiTokenDTO {
	return apiTokenDTO{
		ID:         record.ID,
		Name:       record.Name,
		Prefix:     record.Prefix,
		Suffix:     record.Suffix,
		CreatedAt:  record.CreatedAt,
		LastUsedAt: record.LastUsedAt,
		ExpiresAt:  record.ExpiresAt,
		Scopes:     append([]string{}, record.Scopes...),
	}
}

func normalizeRequestedScopes(raw *[]string) ([]string, error) {
	if raw == nil {
		return []string{config.ScopeWildcard}, nil
	}

	requested := *raw
	if len(requested) == 0 {
		return nil, fmt.Errorf("select at least one scope or omit the field for full access")
	}

	seen := make(map[string]struct{}, len(requested))
	normalized := make([]string, 0, len(requested))
	hasWildcard := false

	for _, scope := range requested {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			return nil, fmt.Errorf("scope identifiers cannot be blank")
		}
		if scope == config.ScopeWildcard {
			hasWildcard = true
			continue
		}
		if !config.IsKnownScope(scope) {
			return nil, fmt.Errorf("unknown scope %q", scope)
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		normalized = append(normalized, scope)
	}

	if hasWildcard {
		if len(normalized) > 0 {
			return nil, fmt.Errorf("wildcard '*' cannot be combined with other scopes")
		}
		return []string{config.ScopeWildcard}, nil
	}

	sort.Strings(normalized)
	return normalized, nil
}

// handleListAPITokens returns all configured API tokens (metadata only).
func (r *Router) handleListAPITokens(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config.Mu.RLock()
	defer config.Mu.RUnlock()

	tokens := make([]apiTokenDTO, 0, len(r.config.APITokens))
	for _, record := range r.config.APITokens {
		tokens = append(tokens, toAPITokenDTO(record))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"tokens": tokens,
	})
}

type createTokenRequest struct {
	Name      string    `json:"name"`
	Scopes    *[]string `json:"scopes"`
	ExpiresIn *string   `json:"expiresIn,omitempty"` // e.g. "24h", "720h", "8760h"
}

func (r *Router) auditTokenEvent(req *http.Request, event string, success bool, details string) {
	user := internalauth.GetUser(req.Context())
	if user == "" && r != nil && r.config != nil {
		user = getAuthUsername(r.config, req)
	}
	LogAuditEventForTenant(GetOrgID(req.Context()), event, user, GetClientIP(req), req.URL.Path, success, details)
}

// handleCreateAPIToken generates and stores a new API token.
func (r *Router) handleCreateAPIToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload createTokenRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil && err != io.EOF {
		r.auditTokenEvent(req, "token_created", false, "Failed to decode create token request body")
		log.Warn().Err(err).Msg("Failed to decode API token create request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = "API token"
	}

	scopes, err := normalizeRequestedScopes(payload.Scopes)
	if err != nil {
		r.auditTokenEvent(req, "token_created", false, "Invalid scope request for token creation")
		log.Warn().Err(err).Msg("Invalid scopes provided for API token creation")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Scope escalation prevention: if the caller is an API token, the new token's
	// scopes must be a subset of the caller's scopes. Session-based (browser) requests
	// bypass this check since sessions have implicit full access.
	if callerToken := getAPITokenRecordFromRequest(req); callerToken != nil {
		for _, requested := range scopes {
			if !callerToken.HasScope(requested) {
				r.auditTokenEvent(req, "token_created", false,
					fmt.Sprintf("Scope escalation denied: caller missing scope %q", requested))
				http.Error(w, fmt.Sprintf("Cannot grant scope %q: your token does not have this scope", requested), http.StatusForbidden)
				return
			}
		}
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		r.auditTokenEvent(req, "token_created", false, "Failed to generate API token")
		log.Error().Err(err).Msg("Failed to generate API token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	record, err := config.NewAPITokenRecord(rawToken, name, scopes)
	if err != nil {
		r.auditTokenEvent(req, "token_created", false, "Failed to construct API token record")
		log.Error().Err(err).Str("token_name", name).Msg("Failed to construct API token record")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	record.OrgID = strings.TrimSpace(GetOrgID(req.Context()))
	if record.OrgID == "" {
		record.OrgID = "default"
	}

	if payload.ExpiresIn != nil && *payload.ExpiresIn != "" {
		dur, err := time.ParseDuration(*payload.ExpiresIn)
		if err != nil {
			r.auditTokenEvent(req, "token_created", false, "Invalid token expiration duration")
			http.Error(w, "Invalid expiresIn duration: "+err.Error(), http.StatusBadRequest)
			return
		}
		if dur < time.Minute {
			r.auditTokenEvent(req, "token_created", false, "Token expiration duration below minimum")
			http.Error(w, "Token expiration must be at least 1 minute", http.StatusBadRequest)
			return
		}
		exp := time.Now().UTC().Add(dur)
		record.ExpiresAt = &exp
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	r.config.APITokens = append(r.config.APITokens, *record)
	r.config.SortAPITokens()

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			// Rollback the in-memory addition
			r.config.APITokens = r.config.APITokens[:len(r.config.APITokens)-1]
			r.auditTokenEvent(req, "token_created", false, "Failed to persist API token")
			log.Error().Err(err).Msg("Failed to persist API tokens after creation")
			http.Error(w, "Failed to save token", http.StatusInternalServerError)
			return
		}
	}

	r.auditTokenEvent(req, "token_created", true, fmt.Sprintf("Created API token id=%s scopes=%s", record.ID, strings.Join(record.Scopes, ",")))

	log.Info().
		Str("audit_event", "token_created").
		Str("token_id", record.ID).
		Str("token_name", record.Name).
		Str("client_ip", req.RemoteAddr).
		Msg("API token created")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":  rawToken,
		"record": toAPITokenDTO(*record),
	})
}

// handleDeleteAPIToken removes an API token by ID.
func (r *Router) handleDeleteAPIToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenID := strings.TrimPrefix(req.URL.Path, "/api/security/tokens/")
	if tokenID == "" {
		r.auditTokenEvent(req, "token_deleted", false, "Missing API token ID in delete request")
		http.Error(w, "Token ID required", http.StatusBadRequest)
		return
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	removed := r.config.RemoveAPIToken(tokenID)
	if removed == nil {
		r.auditTokenEvent(req, "token_deleted", false, fmt.Sprintf("API token id=%s not found for deletion", tokenID))
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	// If this was a migrated env token, suppress re-migration on restart
	if strings.HasPrefix(removed.Name, "Migrated from .env") {
		r.config.SuppressEnvMigration(removed.Hash)
		if r.persistence != nil {
			if err := r.persistence.SaveEnvTokenSuppressions(r.config.SuppressedEnvMigrations); err != nil {
				log.Warn().Err(err).Msg("Failed to persist env token suppression list")
			}
		}
	}

	r.config.SortAPITokens()

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			log.Warn().Err(err).Msg("Failed to persist API tokens after deletion")
		}
	}

	r.auditTokenEvent(req, "token_deleted", true, fmt.Sprintf("Deleted API token id=%s", removed.ID))

	log.Info().
		Str("audit_event", "token_deleted").
		Str("token_id", removed.ID).
		Str("token_name", removed.Name).
		Str("client_ip", req.RemoteAddr).
		Msg("API token deleted")

	w.WriteHeader(http.StatusNoContent)
}

// handleRotateAPIToken atomically rotates an API token: creates a new token and removes the old one.
func (r *Router) handleRotateAPIToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract token ID: /api/security/tokens/{id}/rotate
	path := strings.TrimPrefix(req.URL.Path, "/api/security/tokens/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || parts[1] != "rotate" {
		r.auditTokenEvent(req, "token_rotated", false, "Missing or invalid API token ID for rotate request")
		http.Error(w, "Token ID required", http.StatusBadRequest)
		return
	}
	tokenID := strings.TrimSpace(parts[0])

	config.Mu.Lock()
	defer config.Mu.Unlock()

	// Find existing token
	var oldRecord config.APITokenRecord
	found := false
	for idx := range r.config.APITokens {
		if r.config.APITokens[idx].ID == tokenID {
			oldRecord = r.config.APITokens[idx] // copy for safety (slice may reallocate)
			found = true
			break
		}
	}
	if !found {
		r.auditTokenEvent(req, "token_rotated", false, fmt.Sprintf("API token id=%s not found for rotation", tokenID))
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	// Scope escalation prevention: if the caller is an API token, they can only
	// rotate tokens whose scopes are a subset of their own. This prevents a
	// limited-scope token from rotating a wildcard token to obtain its raw value.
	if callerToken := getAPITokenRecordFromRequest(req); callerToken != nil {
		for _, targetScope := range oldRecord.Scopes {
			if !callerToken.HasScope(targetScope) {
				r.auditTokenEvent(req, "token_rotated", false,
					fmt.Sprintf("Scope escalation denied: caller missing scope %q on target token id=%s", targetScope, tokenID))
				http.Error(w, fmt.Sprintf("Cannot rotate token with scope %q: your token does not have this scope", targetScope), http.StatusForbidden)
				return
			}
		}
	}

	// Generate new token
	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		r.auditTokenEvent(req, "token_rotated", false, fmt.Sprintf("Failed to generate replacement token for id=%s", tokenID))
		log.Error().Err(err).Msg("Failed to generate token during rotation")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	newRecord, err := config.NewAPITokenRecord(rawToken, oldRecord.Name, oldRecord.Scopes)
	if err != nil {
		r.auditTokenEvent(req, "token_rotated", false, fmt.Sprintf("Failed to construct replacement token for id=%s", tokenID))
		log.Error().Err(err).Msg("Failed to construct token record during rotation")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Preserve org bindings from old token
	newRecord.OrgID = oldRecord.OrgID
	newRecord.OrgIDs = append([]string{}, oldRecord.OrgIDs...)
	if oldRecord.Metadata != nil {
		newRecord.Metadata = make(map[string]string, len(oldRecord.Metadata))
		for k, v := range oldRecord.Metadata {
			newRecord.Metadata[k] = v
		}
	}
	// Preserve expiration policy if set
	if oldRecord.ExpiresAt != nil {
		t := *oldRecord.ExpiresAt
		newRecord.ExpiresAt = &t
	}

	// If this was a migrated env token, suppress re-migration on restart.
	if strings.HasPrefix(oldRecord.Name, "Migrated from .env") {
		r.config.SuppressEnvMigration(oldRecord.Hash)
		if r.persistence != nil {
			if err := r.persistence.SaveEnvTokenSuppressions(r.config.SuppressedEnvMigrations); err != nil {
				log.Warn().Err(err).Msg("Failed to persist env token suppression list")
			}
		}
	}

	// Remove old, add new (rollback if persistence fails)
	prevTokens := append([]config.APITokenRecord{}, r.config.APITokens...)
	r.config.RemoveAPIToken(tokenID)
	r.config.APITokens = append(r.config.APITokens, *newRecord)
	r.config.SortAPITokens()

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			// Rollback the in-memory rotation so the in-memory config stays consistent with disk.
			r.config.APITokens = prevTokens
			r.config.SortAPITokens()
			r.auditTokenEvent(req, "token_rotated", false, fmt.Sprintf("Failed to persist rotated token for id=%s", tokenID))
			log.Error().Err(err).Msg("Failed to persist API tokens after rotation")
			http.Error(w, "Failed to save rotated token", http.StatusInternalServerError)
			return
		}
	}

	r.auditTokenEvent(req, "token_rotated", true, fmt.Sprintf("Rotated API token old_id=%s new_id=%s", tokenID, newRecord.ID))

	log.Info().
		Str("audit_event", "token_rotated").
		Str("old_token_id", tokenID).
		Str("new_token_id", newRecord.ID).
		Str("token_name", newRecord.Name).
		Str("client_ip", req.RemoteAddr).
		Msg("API token rotated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"token":  rawToken,
		"record": toAPITokenDTO(*newRecord),
	})
}
