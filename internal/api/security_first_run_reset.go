package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
	"github.com/rs/zerolog/log"
)

type firstRunResetResponse struct {
	BootstrapToken     string `json:"bootstrapToken"`
	BootstrapTokenPath string `json:"bootstrapTokenPath,omitempty"`
}

func devModeEnabled() bool {
	return os.Getenv("PULSE_DEV") == "true" || strings.EqualFold(os.Getenv("NODE_ENV"), "development")
}

func (r *Router) handleResetFirstRunSecurity(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !devModeEnabled() {
		http.Error(w, "Development mode required", http.StatusForbidden)
		return
	}
	if !CheckAuth(r.config, w, req) {
		return
	}
	if !ensureSettingsWriteScope(r.config, w, req) {
		return
	}

	// Prepare the recovery credential before changing any active authentication
	// state. A failure here must leave the caller's current credentials usable so
	// they can repair the data path and retry the reset.
	token, bootstrapCreated, path, err := loadOrCreateBootstrapToken(r.config.DataPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to recreate bootstrap token during first-run reset")
		http.Error(w, "Failed to recreate bootstrap token", http.StatusInternalServerError)
		return
	}
	rollbackBootstrap := func() {
		if !bootstrapCreated {
			return
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Str("token_path", path).Msg("Failed to roll back bootstrap token after first-run reset failure")
		}
	}

	// Remove restart-time password configuration before committing token
	// revocation. Runtime authentication remains unchanged until both durable
	// operations succeed.
	authEnvSnapshots, err := snapshotAuthEnvFiles(r.config.ConfigPath, r.config.DataPath)
	if err != nil {
		rollbackBootstrap()
		log.Warn().Err(err).Msg("Failed to snapshot auth env files during first-run reset")
		http.Error(w, "Failed to read persisted auth configuration", http.StatusInternalServerError)
		return
	}
	if err := removeAuthEnvFiles(r.config.ConfigPath, r.config.DataPath); err != nil {
		rollbackBootstrap()
		if restoreErr := restoreAuthEnvFiles(authEnvSnapshots); restoreErr != nil {
			log.Error().Err(restoreErr).Msg("Failed to restore auth env files after first-run reset failure")
		}
		log.Warn().Err(err).Msg("Failed to remove auth env files during first-run reset")
		http.Error(w, "Failed to remove persisted auth configuration", http.StatusInternalServerError)
		return
	}

	config.Mu.Lock()
	previousAuthUser := strings.TrimSpace(r.config.AuthUser)
	if r.persistence != nil {
		if err := r.persistence.SaveAPITokens([]config.APITokenRecord{}); err != nil {
			config.Mu.Unlock()
			rollbackBootstrap()
			if restoreErr := restoreAuthEnvFiles(authEnvSnapshots); restoreErr != nil {
				log.Error().Err(restoreErr).Msg("Failed to restore auth env files after API token reset failure")
			}
			log.Warn().Err(err).Msg("Failed to clear persisted API tokens during first-run reset")
			http.Error(w, "Failed to clear persisted API tokens", http.StatusInternalServerError)
			return
		}
	}
	r.config.AuthUser = ""
	r.config.AuthPass = ""
	r.config.APIToken = ""
	r.config.APITokens = nil
	config.Mu.Unlock()

	for _, key := range []string{
		"PULSE_AUTH_USER",
		"PULSE_AUTH_PASS",
		"REQUIRE_AUTH",
	} {
		_ = os.Unsetenv(key)
	}

	if previousAuthUser != "" {
		InvalidateUserSessions(previousAuthUser)
	}
	r.clearSession(w, req)
	r.bootstrapTokenHash = internalauth.HashAPIToken(token)
	r.bootstrapTokenPath = path

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(firstRunResetResponse{
		BootstrapToken:     token,
		BootstrapTokenPath: path,
	})
}
