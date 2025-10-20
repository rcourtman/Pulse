package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// SystemSettingsHandler handles system settings
type SystemSettingsHandler struct {
	config                   *config.Config
	persistence              *config.ConfigPersistence
	wsHub                    *websocket.Hub
	reloadSystemSettingsFunc func() // Function to reload cached system settings
	monitor                  interface {
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
}, reloadSystemSettingsFunc func()) *SystemSettingsHandler {
	return &SystemSettingsHandler{
		config:                   cfg,
		persistence:              persistence,
		wsHub:                    wsHub,
		monitor:                  monitor,
		reloadSystemSettingsFunc: reloadSystemSettingsFunc,
	}
}

// SetMonitor updates the monitor reference used by the handler at runtime.
func (h *SystemSettingsHandler) SetMonitor(m interface {
	GetDiscoveryService() *discovery.Service
	StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string)
	StopDiscoveryService()
}) {
	h.monitor = m
}

// validateSystemSettings validates settings before applying them
func validateSystemSettings(settings *config.SystemSettings, rawRequest map[string]interface{}) error {
	// Note: PVE polling is hardcoded to 10s since Proxmox cluster/resources endpoint only updates every 10s
	// Legacy polling interval fields are ignored if provided

	if val, ok := rawRequest["pbsPollingInterval"]; ok {
		if interval, ok := val.(float64); ok {
			if interval <= 0 {
				return fmt.Errorf("PBS polling interval must be positive (minimum 10 seconds)")
			}
			if interval < 10 {
				return fmt.Errorf("PBS polling interval must be at least 10 seconds")
			}
			if interval > 3600 {
				return fmt.Errorf("PBS polling interval cannot exceed 3600 seconds (1 hour)")
			}
		} else {
			return fmt.Errorf("PBS polling interval must be a number")
		}
	}

	if val, ok := rawRequest["pmgPollingInterval"]; ok {
		if interval, ok := val.(float64); ok {
			if interval <= 0 {
				return fmt.Errorf("PMG polling interval must be positive (minimum 10 seconds)")
			}
			if interval < 10 {
				return fmt.Errorf("PMG polling interval must be at least 10 seconds")
			}
			if interval > 3600 {
				return fmt.Errorf("PMG polling interval cannot exceed 3600 seconds (1 hour)")
			}
		} else {
			return fmt.Errorf("PMG polling interval must be a number")
		}
	}

	if val, ok := rawRequest["backupPollingInterval"]; ok {
		if interval, ok := val.(float64); ok {
			if interval < 0 {
				return fmt.Errorf("Backup polling interval cannot be negative")
			}
			if interval > 0 && interval < 10 {
				return fmt.Errorf("Backup polling interval must be at least 10 seconds")
			}
			if interval > 604800 {
				return fmt.Errorf("Backup polling interval cannot exceed 604800 seconds (7 days)")
			}
		} else {
			return fmt.Errorf("Backup polling interval must be a number")
		}
	}

	// Validate boolean fields have correct type
	if val, ok := rawRequest["autoUpdateEnabled"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("autoUpdateEnabled must be a boolean")
		}
	}

	if val, ok := rawRequest["discoveryEnabled"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("discoveryEnabled must be a boolean")
		}
	}

	if val, ok := rawRequest["allowEmbedding"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("allowEmbedding must be a boolean")
		}
	}

	if val, ok := rawRequest["backupPollingEnabled"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("backupPollingEnabled must be a boolean")
		}
	}

	// Validate auto-update check interval (min 1 hour, max 7 days)
	if val, ok := rawRequest["autoUpdateCheckInterval"]; ok {
		if interval, ok := val.(float64); ok {
			if interval < 0 {
				return fmt.Errorf("auto-update check interval cannot be negative")
			}
			if interval > 0 && interval < 1 {
				return fmt.Errorf("auto-update check interval must be at least 1 hour")
			}
			if interval > 168 {
				return fmt.Errorf("auto-update check interval cannot exceed 168 hours (7 days)")
			}
		} else {
			return fmt.Errorf("auto-update check interval must be a number")
		}
	}

	if val, ok := rawRequest["discoveryConfig"]; ok {
		cfgMap, ok := val.(map[string]interface{})
		if !ok {
			return fmt.Errorf("discoveryConfig must be an object")
		}

		if envVal, exists := cfgMap["environmentOverride"]; exists {
			envStr, ok := envVal.(string)
			if !ok {
				return fmt.Errorf("discoveryConfig.environmentOverride must be a string")
			}
			if !config.IsValidDiscoveryEnvironment(envStr) {
				return fmt.Errorf("invalid discovery environment override: %s", envStr)
			}
		}

		if allowVal, exists := cfgMap["subnetAllowlist"]; exists {
			items, ok := allowVal.([]interface{})
			if !ok {
				return fmt.Errorf("discoveryConfig.subnetAllowlist must be an array of CIDR strings")
			}
			for _, item := range items {
				cidr, ok := item.(string)
				if !ok {
					return fmt.Errorf("discoveryConfig.subnetAllowlist entries must be strings")
				}
				if _, _, err := net.ParseCIDR(cidr); err != nil {
					return fmt.Errorf("invalid CIDR in discoveryConfig.subnetAllowlist: %s", cidr)
				}
			}
		}

		if blockVal, exists := cfgMap["subnetBlocklist"]; exists {
			items, ok := blockVal.([]interface{})
			if !ok {
				return fmt.Errorf("discoveryConfig.subnetBlocklist must be an array of CIDR strings")
			}
			for _, item := range items {
				cidr, ok := item.(string)
				if !ok {
					return fmt.Errorf("discoveryConfig.subnetBlocklist entries must be strings")
				}
				if _, _, err := net.ParseCIDR(cidr); err != nil {
					return fmt.Errorf("invalid CIDR in discoveryConfig.subnetBlocklist: %s", cidr)
				}
			}
		}

		if hostsVal, exists := cfgMap["maxHostsPerScan"]; exists {
			value, ok := hostsVal.(float64)
			if !ok {
				return fmt.Errorf("discoveryConfig.maxHostsPerScan must be a number")
			}
			if value <= 0 {
				return fmt.Errorf("discoveryConfig.maxHostsPerScan must be greater than zero")
			}
		}

		if concurrentVal, exists := cfgMap["maxConcurrent"]; exists {
			value, ok := concurrentVal.(float64)
			if !ok {
				return fmt.Errorf("discoveryConfig.maxConcurrent must be a number")
			}
			if value <= 0 || value > 1000 {
				return fmt.Errorf("discoveryConfig.maxConcurrent must be between 1 and 1000")
			}
		}

		if val, exists := cfgMap["enableReverseDns"]; exists {
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("discoveryConfig.enableReverseDns must be a boolean")
			}
		}

		if val, exists := cfgMap["scanGateways"]; exists {
			if _, ok := val.(bool); !ok {
				return fmt.Errorf("discoveryConfig.scanGateways must be a boolean")
			}
		}

		if val, exists := cfgMap["dialTimeoutMs"]; exists {
			timeout, ok := val.(float64)
			if !ok {
				return fmt.Errorf("discoveryConfig.dialTimeoutMs must be a number")
			}
			if timeout <= 0 {
				return fmt.Errorf("discoveryConfig.dialTimeoutMs must be greater than zero")
			}
		}

		if val, exists := cfgMap["httpTimeoutMs"]; exists {
			timeout, ok := val.(float64)
			if !ok {
				return fmt.Errorf("discoveryConfig.httpTimeoutMs must be a number")
			}
			if timeout <= 0 {
				return fmt.Errorf("discoveryConfig.httpTimeoutMs must be greater than zero")
			}
		}
	}

	// Validate connection timeout (min 1 second, max 5 minutes)
	if val, ok := rawRequest["connectionTimeout"]; ok {
		if timeout, ok := val.(float64); ok {
			if timeout < 0 {
				return fmt.Errorf("connection timeout cannot be negative")
			}
			if timeout > 0 && timeout < 1 {
				return fmt.Errorf("connection timeout must be at least 1 second")
			}
			if timeout > 300 {
				return fmt.Errorf("connection timeout cannot exceed 300 seconds (5 minutes)")
			}
		} else {
			return fmt.Errorf("connection timeout must be a number")
		}
	}

	// Validate theme
	if val, ok := rawRequest["theme"]; ok {
		if theme, ok := val.(string); ok {
			if theme != "" && theme != "light" && theme != "dark" {
				return fmt.Errorf("theme must be 'light', 'dark', or empty")
			}
		} else {
			return fmt.Errorf("theme must be a string")
		}
	}

	// Validate update channel
	if val, ok := rawRequest["updateChannel"]; ok {
		if channel, ok := val.(string); ok {
			if channel != "" && channel != "stable" && channel != "rc" {
				return fmt.Errorf("update channel must be 'stable' or 'rc'")
			}
		} else {
			return fmt.Errorf("update channel must be a string")
		}
	}

	return nil
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
		settings = config.DefaultSystemSettings()
	}
	if settings == nil {
		settings = config.DefaultSystemSettings()
	}

	// Log loaded settings for debugging
	if settings != nil {
		log.Debug().
			Str("theme", settings.Theme).
			Msg("Loaded system settings for API response")

		// Always expose effective backup polling configuration
		settings.BackupPollingInterval = int(h.config.BackupPollingInterval.Seconds())
		enabled := h.config.EnableBackupPolling
		settings.BackupPollingEnabled = &enabled
		settings.DiscoveryConfig = config.CloneDiscoveryConfig(h.config.Discovery)
	}

	// Include env override information
	response := struct {
		*config.SystemSettings
		EnvOverrides map[string]bool `json:"envOverrides,omitempty"`
	}{
		SystemSettings: settings,
		EnvOverrides:   h.config.EnvOverrides,
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write system settings response")
	}
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

	// Check if using proxy auth and if so, verify admin status
	if h.config.ProxyAuthSecret != "" {
		if valid, username, isAdmin := CheckProxyAuth(h.config, r); valid {
			if !isAdmin {
				// User is authenticated but not an admin
				log.Warn().
					Str("ip", r.RemoteAddr).
					Str("path", r.URL.Path).
					Str("method", r.Method).
					Str("username", username).
					Msg("Non-admin user attempted to update system settings")

					// Return forbidden error
				utils.WriteJSONError(w, "Admin privileges required", http.StatusForbidden)
				return
			}
		}
	}

	// Load existing settings first to preserve fields not in the request
	existingSettings, err := h.persistence.LoadSystemSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to load existing settings")
		existingSettings = config.DefaultSystemSettings()
	}
	if existingSettings == nil {
		existingSettings = config.DefaultSystemSettings()
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

	// Validate the settings
	if err := validateSystemSettings(&updates, rawRequest); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Start with existing settings
	settings := *existingSettings
	discoveryConfigUpdated := false

	// Only update fields that were provided in the request
	// Note: PVE polling is hardcoded to 10s, legacy polling fields are ignored
	if _, ok := rawRequest["pbsPollingInterval"]; ok {
		settings.PBSPollingInterval = updates.PBSPollingInterval
	}
	if _, ok := rawRequest["pmgPollingInterval"]; ok {
		settings.PMGPollingInterval = updates.PMGPollingInterval
	}
	if _, ok := rawRequest["backupPollingInterval"]; ok {
		settings.BackupPollingInterval = updates.BackupPollingInterval
	}
	if updates.AllowedOrigins != "" {
		settings.AllowedOrigins = updates.AllowedOrigins
	}
	if _, ok := rawRequest["connectionTimeout"]; ok {
		settings.ConnectionTimeout = updates.ConnectionTimeout
	}
	if updates.UpdateChannel != "" {
		settings.UpdateChannel = updates.UpdateChannel
	}
	if _, ok := rawRequest["autoUpdateCheckInterval"]; ok {
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
	if _, ok := rawRequest["discoveryConfig"]; ok {
		settings.DiscoveryConfig = config.CloneDiscoveryConfig(updates.DiscoveryConfig)
		discoveryConfigUpdated = true
	}
	// Allow clearing of AllowedEmbedOrigins by setting to empty string
	if _, ok := rawRequest["allowedEmbedOrigins"]; ok {
		settings.AllowedEmbedOrigins = updates.AllowedEmbedOrigins
	}

	// Boolean fields need special handling since false is a valid value
	if _, ok := rawRequest["autoUpdateEnabled"]; ok {
		settings.AutoUpdateEnabled = updates.AutoUpdateEnabled
	}
	if _, ok := rawRequest["discoveryEnabled"]; ok {
		settings.DiscoveryEnabled = updates.DiscoveryEnabled
	}
	if _, ok := rawRequest["allowEmbedding"]; ok {
		settings.AllowEmbedding = updates.AllowEmbedding
	}
	if _, ok := rawRequest["backupPollingEnabled"]; ok {
		settings.BackupPollingEnabled = updates.BackupPollingEnabled
	}

	// Update the config
	// Note: PVE polling is hardcoded to 10s
	if settings.AllowedOrigins != "" {
		h.config.AllowedOrigins = settings.AllowedOrigins
	}
	if settings.ConnectionTimeout > 0 {
		h.config.ConnectionTimeout = time.Duration(settings.ConnectionTimeout) * time.Second
	}
	if settings.PMGPollingInterval > 0 {
		h.config.PMGPollingInterval = time.Duration(settings.PMGPollingInterval) * time.Second
	}
	if _, ok := rawRequest["backupPollingInterval"]; ok {
		if settings.BackupPollingInterval <= 0 {
			h.config.BackupPollingInterval = 0
		} else {
			h.config.BackupPollingInterval = time.Duration(settings.BackupPollingInterval) * time.Second
		}
	}
	if settings.BackupPollingEnabled != nil {
		h.config.EnableBackupPolling = *settings.BackupPollingEnabled
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
	h.config.Discovery = config.CloneDiscoveryConfig(settings.DiscoveryConfig)

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
		if discoveryConfigUpdated && settings.DiscoveryEnabled {
			if svc := h.monitor.GetDiscoveryService(); svc != nil {
				log.Info().Msg("Discovery configuration changed; triggering refresh")
				svc.ForceRefresh()
			}
		}
	}

	// Save to persistence
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Reload cached system settings after successful save
	if h.reloadSystemSettingsFunc != nil {
		h.reloadSystemSettingsFunc()
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

	if err := utils.WriteJSONResponse(w, map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Msg("Failed to write system settings update response")
	}
}

// HandleSSHConfig writes SSH configuration for Pulse user
func (h *SystemSettingsHandler) HandleSSHConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 32KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	defer r.Body.Close()

	// Read SSH config content from request body
	sshConfig, err := io.ReadAll(r.Body)
	if err != nil {
		// Check if body was too large
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			log.Warn().Msg("SSH config request body too large")
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		log.Error().Err(err).Msg("Failed to read SSH config from request")
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Basic validation: ensure it looks like SSH config
	configStr := string(sshConfig)
	if len(configStr) == 0 {
		log.Error().Msg("Empty SSH config received")
		http.Error(w, "Empty SSH config", http.StatusBadRequest)
		return
	}

	// Security: Use allowlist-based validation (safer than blocklist)
	// Only permit the specific directives Pulse needs for ProxyJump
	allowedDirectives := map[string]bool{
		"host":                    true,
		"hostname":                true,
		"proxyjump":               true,
		"user":                    true,
		"identityfile":            true,
		"stricthostkeychecking":   true,
	}

	// Parse and validate each line
	scanner := bufio.NewScanner(strings.NewReader(configStr))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Strip comments
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}

		// Skip empty lines and whitespace
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract directive (first word)
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		directive := strings.ToLower(fields[0])
		if !allowedDirectives[directive] {
			log.Warn().
				Str("directive", fields[0]).
				Int("line", lineNum).
				Msg("Rejected SSH config with forbidden directive")
			http.Error(w, fmt.Sprintf("SSH config contains forbidden directive: %s", fields[0]), http.StatusBadRequest)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("Failed to parse SSH config")
		http.Error(w, "Invalid SSH config format", http.StatusBadRequest)
		return
	}

	// Get the Pulse user's home directory
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse" // fallback
	}

	// Create .ssh directory if it doesn't exist
	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		log.Error().Err(err).Str("dir", sshDir).Msg("Failed to create .ssh directory")
		http.Error(w, "Failed to create SSH directory", http.StatusInternalServerError)
		return
	}

	// Write SSH config file
	configPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(configPath, sshConfig, 0600); err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("Failed to write SSH config")
		http.Error(w, "Failed to write SSH config", http.StatusInternalServerError)
		return
	}

	log.Info().Str("path", configPath).Int("size", len(sshConfig)).Msg("SSH config written successfully")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Msg("Failed to encode success response")
	}
}
