package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// SystemSettingsHandler handles system settings
type SystemSettingsHandler struct {
	config *config.Config
	persistence *config.ConfigPersistence
	wsHub *websocket.Hub
	monitor interface {
		GetDiscoveryService() *discovery.Service
		StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string)
		StopDiscoveryService()
	}
}

// NewSystemSettingsHandler creates a new system settings handler
func NewSystemSettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, wsHub *websocket.Hub, monitor interface {
	GetDiscoveryService() *discovery.Service
	StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string)
	StopDiscoveryService()
}) *SystemSettingsHandler {
	return &SystemSettingsHandler{
		config: cfg,
		persistence: persistence,
		wsHub: wsHub,
		monitor: monitor,
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

	// Load existing settings first to preserve fields not in the request
	existingSettings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load existing settings")
		existingSettings = &config.SystemSettings{}
	}
	if existingSettings == nil {
		existingSettings = &config.SystemSettings{}
	}

	// Read the request body into a map to check which fields were provided
	var rawRequest map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawRequest); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Convert the map back to JSON for decoding into struct
	jsonBytes, err := json.Marshal(rawRequest)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Decode into updates struct
	var updates config.SystemSettings
	if err := json.Unmarshal(jsonBytes, &updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Start with existing settings
	settings := *existingSettings
	
	// Only update fields that were provided in the request
	if updates.PollingInterval > 0 {
		settings.PollingInterval = updates.PollingInterval
	}
	if updates.PVEPollingInterval > 0 {
		settings.PVEPollingInterval = updates.PVEPollingInterval
	}
	if updates.PBSPollingInterval > 0 {
		settings.PBSPollingInterval = updates.PBSPollingInterval
	}
	if updates.AllowedOrigins != "" {
		settings.AllowedOrigins = updates.AllowedOrigins
	}
	if updates.ConnectionTimeout > 0 {
		settings.ConnectionTimeout = updates.ConnectionTimeout
	}
	if updates.UpdateChannel != "" {
		settings.UpdateChannel = updates.UpdateChannel
	}
	if updates.AutoUpdateCheckInterval > 0 {
		settings.AutoUpdateCheckInterval = updates.AutoUpdateCheckInterval
	}
	if updates.AutoUpdateTime != "" {
		settings.AutoUpdateTime = updates.AutoUpdateTime
	}
	if updates.Theme != "" {
		settings.Theme = updates.Theme
	}
	if updates.DiscoverySubnet != "" {
		settings.DiscoverySubnet = updates.DiscoverySubnet
	}
	
	// Boolean fields need special handling since false is a valid value
	if _, ok := rawRequest["autoUpdateEnabled"]; ok {
		settings.AutoUpdateEnabled = updates.AutoUpdateEnabled
	}
	if _, ok := rawRequest["discoveryEnabled"]; ok {
		settings.DiscoveryEnabled = updates.DiscoveryEnabled
	}

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
	
	// Update discovery settings and manage the service
	prevDiscoveryEnabled := h.config.DiscoveryEnabled
	h.config.DiscoveryEnabled = settings.DiscoveryEnabled
	if settings.DiscoverySubnet != "" {
		h.config.DiscoverySubnet = settings.DiscoverySubnet
	}
	
	// Start or stop discovery service based on setting change
	if h.monitor != nil {
		if settings.DiscoveryEnabled && !prevDiscoveryEnabled {
			// Discovery was just enabled, start the service
			subnet := h.config.DiscoverySubnet
			if subnet == "" {
				subnet = "auto"
			}
			h.monitor.StartDiscoveryService(context.Background(), h.wsHub, subnet)
			log.Info().Msg("Discovery service started via settings update")
		} else if !settings.DiscoveryEnabled && prevDiscoveryEnabled {
			// Discovery was just disabled, stop the service
			h.monitor.StopDiscoveryService()
			log.Info().Msg("Discovery service stopped via settings update")
		} else if settings.DiscoveryEnabled && settings.DiscoverySubnet != "" {
			// Subnet changed while discovery is enabled, update it
			if svc := h.monitor.GetDiscoveryService(); svc != nil {
				svc.SetSubnet(settings.DiscoverySubnet)
			}
		}
	}

	// Save to persistence
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	log.Info().Msg("System settings updated")

	// Broadcast theme change to all connected clients if theme was updated
	if settings.Theme != "" && h.wsHub != nil {
		h.wsHub.BroadcastMessage(websocket.Message{
			Type: "settingsUpdate",
			Data: map[string]interface{}{
				"theme": settings.Theme,
			},
		})
		log.Debug().Str("theme", settings.Theme).Msg("Broadcasting theme change to WebSocket clients")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}