package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// SettingsResponse represents the current settings and capabilities
type SettingsResponse struct {
	Current      *config.Settings `json:"current"`
	Defaults     *config.Settings `json:"defaults"`
	Capabilities Capabilities     `json:"capabilities"`
}

// Capabilities represents what can be configured
type Capabilities struct {
	CanRestart       bool `json:"canRestart"`
	CanValidatePorts bool `json:"canValidatePorts"`
	RequiresRestart  bool `json:"requiresRestart"`
}

// SettingsUpdate represents a settings update request
type SettingsUpdate struct {
	Settings     *config.Settings `json:"settings"`
	RestartNow   bool             `json:"restartNow"`
	ValidateOnly bool             `json:"validateOnly"`
}

// getSettings returns current settings and configuration capabilities
func getSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create current settings from running config
	// Note: This endpoint seems to be for UI settings, not node configuration
	current := &config.Settings{
		Server: config.ServerSettings{
			Backend: config.PortSettings{
				Port: 3000, // These are fixed in our new system
				Host: "0.0.0.0",
			},
			Frontend: config.PortSettings{
				Port: 7655,
				Host: "0.0.0.0",
			},
		},
		Monitoring: config.MonitoringSettings{
			PollingInterval: 3, // seconds
		},
	}

	response := SettingsResponse{
		Current:  current,
		Defaults: config.DefaultSettings(),
		Capabilities: Capabilities{
			CanRestart:       true,
			CanValidatePorts: true,
			RequiresRestart:  true,
		},
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write settings response")
	}
}

// updateSettings updates the configuration
func updateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var update SettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate settings
	if err := update.Settings.Validate(); err != nil {
		log.Error().Err(err).Msg("Settings validation failed")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check port availability
	if !update.ValidateOnly {
		if !config.IsPortAvailable(update.Settings.Server.Backend.Host, update.Settings.Server.Backend.Port) {
			http.Error(w, "Backend port is not available", http.StatusConflict)
			return
		}
		if !config.IsPortAvailable(update.Settings.Server.Frontend.Host, update.Settings.Server.Frontend.Port) {
			http.Error(w, "Frontend port is not available", http.StatusConflict)
			return
		}
	}

	// If validate only, return success
	if update.ValidateOnly {
		if err := utils.WriteJSONResponse(w, map[string]interface{}{
			"valid":   true,
			"message": "Configuration is valid",
		}); err != nil {
			log.Error().Err(err).Msg("Failed to write validation response")
		}
		return
	}

	// Save configuration to file
	if err := saveSettings(update.Settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		http.Error(w, "Failed to save settings", http.StatusInternalServerError)
		return
	}

	// Prepare response
	response := map[string]interface{}{
		"success":         true,
		"message":         "Settings saved successfully",
		"requiresRestart": true,
	}

	// Handle restart if requested
	if update.RestartNow {
		// Schedule a graceful restart after response is sent
		go func() {
			log.Info().Msg("Scheduling graceful restart due to configuration change")
			// Use a more graceful shutdown mechanism
			// The systemd service will handle the restart
			time.Sleep(1 * time.Second) // Give time for response to be sent
			log.Info().Msg("Initiating graceful shutdown")
			os.Exit(0)
		}()
		response["restarting"] = true
	}

	if err := utils.WriteJSONResponse(w, response); err != nil {
		log.Error().Err(err).Msg("Failed to write settings update response")
	}
}

// saveSettings persists settings to the configuration file
func saveSettings(settings *config.Settings) error {
	// Determine config path - prefer /etc/pulse if writable
	configPath := "./pulse.yml"
	if _, err := os.Stat("/etc/pulse"); err == nil {
		// Check if we can write to /etc/pulse
		testFile := "/etc/pulse/.write-test"
		if f, err := os.Create(testFile); err == nil {
			f.Close()
			os.Remove(testFile)
			configPath = "/etc/pulse/pulse.yml"
		}
	}

	// Marshal to YAML
	data, err := yaml.Marshal(settings)
	if err != nil {
		return err
	}

	// Create directory if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	log.Info().Str("path", configPath).Msg("Configuration saved")
	return nil
}
