package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// handleOIDCConfig exposes and updates the OIDC configuration.
func (r *Router) handleOIDCConfig(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		r.handleGetOIDCConfig(w, req)
	case http.MethodPut:
		r.handleUpdateOIDCConfig(w, req)
	default:
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET and PUT are supported", nil)
	}
}

func (r *Router) handleGetOIDCConfig(w http.ResponseWriter, req *http.Request) {
	cfg := r.ensureOIDCConfig()

	response := makeOIDCResponse(cfg, r.config.PublicURL)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode OIDC configuration response; returning HTTP 500 to caller")
	}
}

func (r *Router) handleUpdateOIDCConfig(w http.ResponseWriter, req *http.Request) {
	cfg := r.ensureOIDCConfig()

	if len(cfg.EnvOverrides) > 0 {
		writeErrorResponse(w, http.StatusConflict, "oidc_locked", "OIDC settings are managed via environment variables and cannot be changed at runtime", nil)
		return
	}

	var payload struct {
		Enabled           bool     `json:"enabled"`
		IssuerURL         string   `json:"issuerUrl"`
		ClientID          string   `json:"clientId"`
		ClientSecret      *string  `json:"clientSecret,omitempty"`
		RedirectURL       string   `json:"redirectUrl"`
		LogoutURL         string   `json:"logoutUrl"`
		Scopes            []string `json:"scopes"`
		UsernameClaim     string   `json:"usernameClaim"`
		EmailClaim        string   `json:"emailClaim"`
		GroupsClaim       string   `json:"groupsClaim"`
		AllowedGroups     []string `json:"allowedGroups"`
		AllowedDomains    []string `json:"allowedDomains"`
		AllowedEmails     []string `json:"allowedEmails"`
		ClearClientSecret bool     `json:"clearClientSecret"`
	}

	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request payload", nil)
		return
	}

	updated := &config.OIDCConfig{
		Enabled:        payload.Enabled,
		IssuerURL:      strings.TrimSpace(payload.IssuerURL),
		ClientID:       strings.TrimSpace(payload.ClientID),
		RedirectURL:    strings.TrimSpace(payload.RedirectURL),
		LogoutURL:      strings.TrimSpace(payload.LogoutURL),
		Scopes:         append([]string{}, payload.Scopes...),
		UsernameClaim:  strings.TrimSpace(payload.UsernameClaim),
		EmailClaim:     strings.TrimSpace(payload.EmailClaim),
		GroupsClaim:    strings.TrimSpace(payload.GroupsClaim),
		AllowedGroups:  append([]string{}, payload.AllowedGroups...),
		AllowedDomains: append([]string{}, payload.AllowedDomains...),
		AllowedEmails:  append([]string{}, payload.AllowedEmails...),
		EnvOverrides:   make(map[string]bool),
	}

	// Preserve existing secret unless explicitly changed.
	updated.ClientSecret = cfg.ClientSecret
	if payload.ClearClientSecret {
		updated.ClientSecret = ""
	}
	if payload.ClientSecret != nil {
		updated.ClientSecret = strings.TrimSpace(*payload.ClientSecret)
	}

	updated.ApplyDefaults(r.config.PublicURL)

	if err := updated.Validate(); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	if err := config.SaveOIDCConfig(updated); err != nil {
		log.Error().Err(err).Msg("Failed to persist OIDC configuration")
		writeErrorResponse(w, http.StatusInternalServerError, "save_failed", "Failed to save OIDC settings", nil)
		return
	}

	// Update in-memory configuration for immediate effect.
	r.config.OIDC = updated

	response := makeOIDCResponse(updated, r.config.PublicURL)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Error().Err(err).Msg("Failed to encode OIDC configuration response; returning HTTP 500 to caller")
	}
}

type oidcResponse struct {
	Enabled         bool            `json:"enabled"`
	IssuerURL       string          `json:"issuerUrl"`
	ClientID        string          `json:"clientId"`
	RedirectURL     string          `json:"redirectUrl"`
	LogoutURL       string          `json:"logoutUrl"`
	Scopes          []string        `json:"scopes"`
	UsernameClaim   string          `json:"usernameClaim"`
	EmailClaim      string          `json:"emailClaim"`
	GroupsClaim     string          `json:"groupsClaim"`
	AllowedGroups   []string        `json:"allowedGroups"`
	AllowedDomains  []string        `json:"allowedDomains"`
	AllowedEmails   []string        `json:"allowedEmails"`
	ClientSecretSet bool            `json:"clientSecretSet"`
	DefaultRedirect string          `json:"defaultRedirect"`
	EnvOverrides    map[string]bool `json:"envOverrides,omitempty"`
}

func makeOIDCResponse(cfg *config.OIDCConfig, publicURL string) oidcResponse {
	if cfg == nil {
		cfg = config.NewOIDCConfig()
		cfg.ApplyDefaults(publicURL)
	}

	resp := oidcResponse{
		Enabled:         cfg.Enabled,
		IssuerURL:       cfg.IssuerURL,
		ClientID:        cfg.ClientID,
		RedirectURL:     cfg.RedirectURL,
		LogoutURL:       cfg.LogoutURL,
		Scopes:          append([]string{}, cfg.Scopes...),
		UsernameClaim:   cfg.UsernameClaim,
		EmailClaim:      cfg.EmailClaim,
		GroupsClaim:     cfg.GroupsClaim,
		AllowedGroups:   append([]string{}, cfg.AllowedGroups...),
		AllowedDomains:  append([]string{}, cfg.AllowedDomains...),
		AllowedEmails:   append([]string{}, cfg.AllowedEmails...),
		ClientSecretSet: cfg.ClientSecret != "",
		DefaultRedirect: config.DefaultRedirectURL(publicURL),
	}

	if len(cfg.EnvOverrides) > 0 {
		resp.EnvOverrides = make(map[string]bool, len(cfg.EnvOverrides))
		for k, v := range cfg.EnvOverrides {
			resp.EnvOverrides[k] = v
		}
	}

	return resp
}
