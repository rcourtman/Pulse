package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	internalauth "github.com/rcourtman/pulse-go-rewrite/internal/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

type apiTokenDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Suffix     string     `json:"suffix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
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
	Name   string    `json:"name"`
	Scopes *[]string `json:"scopes"`
}

// handleCreateAPIToken generates and stores a new API token.
func (r *Router) handleCreateAPIToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload createTokenRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil && err != io.EOF {
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
		log.Warn().Err(err).Msg("Invalid scopes provided for API token creation")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate API token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	record, err := config.NewAPITokenRecord(rawToken, name, scopes)
	if err != nil {
		log.Error().Err(err).Str("token_name", name).Msg("Failed to construct API token record")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	r.config.APITokens = append(r.config.APITokens, *record)
	r.config.SortAPITokens()
	r.config.APITokenEnabled = true

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			// Rollback the in-memory addition
			r.config.APITokens = r.config.APITokens[:len(r.config.APITokens)-1]
			log.Error().Err(err).Msg("Failed to persist API tokens after creation")
			http.Error(w, "Failed to save token to disk: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

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

	id := strings.TrimPrefix(req.URL.Path, "/api/security/tokens/")
	if id == "" {
		http.Error(w, "Token ID required", http.StatusBadRequest)
		return
	}

	config.Mu.Lock()
	defer config.Mu.Unlock()

	removed := r.config.RemoveAPIToken(id)
	if !removed {
		http.Error(w, "Token not found", http.StatusNotFound)
		return
	}

	r.config.SortAPITokens()
	r.config.APITokenEnabled = r.config.HasAPITokens()

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			log.Warn().Err(err).Msg("Failed to persist API tokens after deletion")
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
