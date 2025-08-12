package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// SystemSettingsHandler handles system settings including API token management
type SystemSettingsHandler struct {
	config *config.Config
	persistence *config.ConfigPersistence
}

// NewSystemSettingsHandler creates a new system settings handler
func NewSystemSettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence) *SystemSettingsHandler {
	return &SystemSettingsHandler{
		config: cfg,
		persistence: persistence,
	}
}

// APITokenResponse represents the API token response
type APITokenResponse struct {
	HasToken bool   `json:"hasToken"`
	Token    string `json:"token,omitempty"`
}

// HandleGetAPIToken returns the current API token status
func (h *SystemSettingsHandler) HandleGetAPIToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check if API token is set
	hasToken := h.config.APIToken != ""
	
	response := APITokenResponse{
		HasToken: hasToken,
	}
	
	// Only return the token if it's requested with proper auth
	if hasToken && r.URL.Query().Get("reveal") == "true" {
		// Verify the request is authenticated
		providedToken := r.Header.Get("X-API-Token")
		if providedToken == h.config.APIToken {
			response.Token = h.config.APIToken
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGenerateAPIToken generates a new API token
func (h *SystemSettingsHandler) HandleGenerateAPIToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// If a token already exists, require authentication
	if h.config.APIToken != "" {
		providedToken := r.Header.Get("X-API-Token")
		if providedToken != h.config.APIToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Generate a new secure token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate random token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	
	newToken := hex.EncodeToString(tokenBytes)
	
	// Save to system settings
	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load system settings")
		settings = &config.SystemSettings{}
	}
	
	settings.APIToken = newToken
	
	if err := h.persistence.SaveSystemSettings(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to save token", http.StatusInternalServerError)
		return
	}
	
	// Update the running config
	h.config.APIToken = newToken
	
	// Don't override if env var is set
	if os.Getenv("API_TOKEN") != "" {
		log.Warn().Msg("API_TOKEN environment variable is set and will override UI-configured token on restart")
	}
	
	log.Info().Msg("API token generated via UI")
	
	response := APITokenResponse{
		HasToken: true,
		Token:    newToken,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleDeleteAPIToken removes the API token
func (h *SystemSettingsHandler) HandleDeleteAPIToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication if token exists
	if h.config.APIToken != "" {
		providedToken := r.Header.Get("X-API-Token")
		if providedToken != h.config.APIToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Save to system settings
	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load system settings")
		settings = &config.SystemSettings{}
	}
	
	settings.APIToken = ""
	
	if err := h.persistence.SaveSystemSettings(*settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to remove token", http.StatusInternalServerError)
		return
	}
	
	// Update the running config
	h.config.APIToken = ""
	
	// Warn if env var is set
	if os.Getenv("API_TOKEN") != "" {
		log.Warn().Msg("API_TOKEN environment variable is set and will override this change on restart")
	}
	
	log.Info().Msg("API token removed via UI")
	
	response := APITokenResponse{
		HasToken: false,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleGetSystemSettings returns all system settings
func (h *SystemSettingsHandler) HandleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load system settings")
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}
	
	// Don't expose the actual token in this endpoint
	if settings.APIToken != "" {
		settings.APIToken = "***HIDDEN***"
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// HandleUpdateSystemSettings updates system settings
func (h *SystemSettingsHandler) HandleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication if token exists
	if h.config.APIToken != "" {
		providedToken := r.Header.Get("X-API-Token")
		if providedToken != h.config.APIToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	var settings config.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	// Don't allow updating API token through this endpoint
	existingSettings, _ := h.persistence.LoadSystemSettings()
	if existingSettings != nil {
		settings.APIToken = existingSettings.APIToken
	}
	
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}
	
	// Update relevant config fields
	if settings.PollingInterval > 0 {
		h.config.PollingInterval = time.Duration(settings.PollingInterval) * time.Second
	}
	if settings.UpdateChannel != "" {
		h.config.UpdateChannel = settings.UpdateChannel
	}
	h.config.AutoUpdateEnabled = settings.AutoUpdateEnabled
	if settings.AutoUpdateCheckInterval > 0 {
		h.config.AutoUpdateCheckInterval = time.Duration(settings.AutoUpdateCheckInterval) * time.Hour
	}
	if settings.AutoUpdateTime != "" {
		h.config.AutoUpdateTime = settings.AutoUpdateTime
	}
	
	log.Info().Msg("System settings updated via UI")
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}