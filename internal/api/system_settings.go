package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rs/zerolog/log"
)

// SystemSettingsHandler handles system settings
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

// HandleGetSystemSettings returns the current system settings
func (h *SystemSettingsHandler) HandleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load system settings")
		settings = &config.SystemSettings{
			PollingInterval: 5,
		}
	}
	
	// Log loaded settings for debugging
	if settings != nil {
		log.Debug().
			Int("pollingInterval", settings.PollingInterval).
			Str("theme", settings.Theme).
			Msg("Loaded system settings for API response")
	}

	// Include env override information
	response := struct {
		*config.SystemSettings
		EnvOverrides map[string]bool `json:"envOverrides,omitempty"`
	}{
		SystemSettings: settings,
		EnvOverrides:   h.config.EnvOverrides,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleUpdateSystemSettings updates the system settings
func (h *SystemSettingsHandler) HandleUpdateSystemSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		return
	}

	var settings config.SystemSettings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Load existing settings to preserve fields not in the request
	// (removed - not needed without API token preservation)

	// Update the config
	if settings.PollingInterval > 0 {
		h.config.PollingInterval = time.Duration(settings.PollingInterval) * time.Second
	}
	if settings.AllowedOrigins != "" {
		h.config.AllowedOrigins = settings.AllowedOrigins
	}
	if settings.ConnectionTimeout > 0 {
		h.config.ConnectionTimeout = time.Duration(settings.ConnectionTimeout) * time.Second
	}
	if settings.UpdateChannel != "" {
		h.config.UpdateChannel = settings.UpdateChannel
	}
	
	// Update auto-update settings
	h.config.AutoUpdateEnabled = settings.AutoUpdateEnabled
	if settings.AutoUpdateCheckInterval > 0 {
		h.config.AutoUpdateCheckInterval = time.Duration(settings.AutoUpdateCheckInterval) * time.Hour
	}
	if settings.AutoUpdateTime != "" {
		h.config.AutoUpdateTime = settings.AutoUpdateTime
	}
	
	// Validate theme if provided
	if settings.Theme != "" && settings.Theme != "light" && settings.Theme != "dark" {
		http.Error(w, "Invalid theme value. Must be 'light', 'dark', or empty", http.StatusBadRequest)
		return
	}

	// Save to persistence
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("System settings updated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}