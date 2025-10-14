package api

import (
	"encoding/json"
	"io"
	"net/http"
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
}

func toAPITokenDTO(record config.APITokenRecord) apiTokenDTO {
	return apiTokenDTO{
		ID:         record.ID,
		Name:       record.Name,
		Prefix:     record.Prefix,
		Suffix:     record.Suffix,
		CreatedAt:  record.CreatedAt,
		LastUsedAt: record.LastUsedAt,
	}
}

// handleListAPITokens returns all configured API tokens (metadata only).
func (r *Router) handleListAPITokens(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

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
	Name string `json:"name"`
}

// handleCreateAPIToken generates and stores a new API token.
func (r *Router) handleCreateAPIToken(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload createTokenRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil && err != io.EOF {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		name = "API token"
	}

	rawToken, err := internalauth.GenerateAPIToken()
	if err != nil {
		log.Error().Err(err).Msg("Failed to generate API token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	record, err := config.NewAPITokenRecord(rawToken, name)
	if err != nil {
		log.Error().Err(err).Msg("Failed to construct API token record")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	r.config.APITokens = append(r.config.APITokens, *record)
	r.config.SortAPITokens()
	r.config.APITokenEnabled = true

	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens(r.config.APITokens); err != nil {
			log.Warn().Err(err).Msg("Failed to persist API tokens after creation")
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
