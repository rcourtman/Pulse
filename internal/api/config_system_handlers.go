package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rs/zerolog/log"
)

func (h *ConfigHandlers) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	// Load settings from persistence to get all fields including theme
	persistedSettings := config.DefaultSystemSettings()
	if persistence := h.getPersistence(r.Context()); persistence != nil {
		loadedSettings, err := persistence.LoadSystemSettings()
		if err != nil {
			log.Warn().Err(err).Msg("Failed to load persisted system settings")
		} else if loadedSettings != nil {
			persistedSettings = loadedSettings
		}
	} else {
		log.Warn().Msg("Failed to load persisted system settings: persistence unavailable")
	}
	if persistedSettings == nil {
		persistedSettings = config.DefaultSystemSettings()
	}

	// Get current values from running config
	settings := *persistedSettings
	cfg := h.getConfig(r.Context())
	if cfg != nil {
		settings.PVEPollingInterval = int(cfg.PVEPollingInterval.Seconds())
		settings.PBSPollingInterval = int(cfg.PBSPollingInterval.Seconds())
		settings.PMGPollingInterval = int(cfg.PMGPollingInterval.Seconds())
		settings.BackupPollingInterval = int(cfg.BackupPollingInterval.Seconds())
		settings.FrontendPort = cfg.FrontendPort
		settings.AllowedOrigins = cfg.AllowedOrigins
		settings.ConnectionTimeout = int(cfg.ConnectionTimeout.Seconds())
		settings.UpdateChannel = cfg.UpdateChannel
		settings.AutoUpdateEnabled = cfg.AutoUpdateEnabled
		settings.AutoUpdateCheckInterval = int(cfg.AutoUpdateCheckInterval.Hours())
		settings.AutoUpdateTime = cfg.AutoUpdateTime
		settings.LogLevel = cfg.LogLevel
		settings.DiscoveryEnabled = cfg.DiscoveryEnabled
		settings.DiscoverySubnet = cfg.DiscoverySubnet
		settings.DiscoveryConfig = config.CloneDiscoveryConfig(cfg.Discovery)
		settings.TemperatureMonitoringEnabled = cfg.TemperatureMonitoringEnabled
		settings.HideLocalLogin = cfg.HideLocalLogin
		settings.PublicURL = cfg.PublicURL
		settings.DisableDockerUpdateActions = cfg.DisableDockerUpdateActions
		settings.DisableLegacyRouteRedirects = cfg.DisableLegacyRouteRedirects
		backupEnabled := cfg.EnableBackupPolling
		settings.BackupPollingEnabled = &backupEnabled
	}

	// Create response structure that includes environment overrides
	response := struct {
		config.SystemSettings
		EnvOverrides map[string]bool `json:"envOverrides,omitempty"`
	}{
		SystemSettings: settings,
		EnvOverrides:   make(map[string]bool),
	}

	if cfg != nil {
		for key, val := range cfg.EnvOverrides {
			response.EnvOverrides[key] = val
		}
	}

	// Legacy fallback: preserve historic key when env var is set directly.
	if os.Getenv("PULSE_AUTH_HIDE_LOCAL_LOGIN") != "" && !response.EnvOverrides["hideLocalLogin"] {
		response.EnvOverrides["hideLocalLogin"] = true
	}

	if len(response.EnvOverrides) == 0 {
		response.EnvOverrides = nil
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ConfigHandlers) handleVerifyTemperatureSSH(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body to 8KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)

	var req struct {
		Nodes string `json:"nodes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("⚠️  Unable to parse verification request"))
		return
	}

	// Parse node list
	nodeList := strings.Fields(req.Nodes)
	if len(nodeList) == 0 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✓ No nodes to verify"))
		return
	}

	// Test SSH connectivity using temperature collector with the correct SSH key
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/home/pulse"
	}
	sshKeyPath := filepath.Join(homeDir, ".ssh/id_ed25519_sensors")
	tempCollector := monitoring.NewTemperatureCollectorWithPort("root", sshKeyPath, h.getConfig(r.Context()).SSHPort)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	successNodes := []string{}
	failedNodes := []string{}

	for _, node := range nodeList {
		// Try to SSH and run sensors command
		temp, err := tempCollector.CollectTemperature(ctx, node, node)
		if err == nil && temp != nil && temp.Available {
			successNodes = append(successNodes, node)
		} else {
			failedNodes = append(failedNodes, node)
		}
	}

	// Build response message
	var response strings.Builder

	if len(successNodes) > 0 {
		response.WriteString("✓ SSH connectivity verified for:\n")
		for _, node := range successNodes {
			response.WriteString(fmt.Sprintf("  • %s\n", node))
		}
	}

	if len(failedNodes) > 0 {
		if len(successNodes) > 0 {
			response.WriteString("\n")
		}
		response.WriteString("ℹ️  Temperature monitoring will be available once SSH connectivity is configured.\n")
		response.WriteString("\n")
		response.WriteString("Nodes pending configuration:\n")
		for _, node := range failedNodes {
			response.WriteString(fmt.Sprintf("  • %s\n", node))
		}
		response.WriteString("\n")
		response.WriteString("See: https://github.com/rcourtman/Pulse/blob/main/SECURITY.md for detailed SSH configuration options.\n")
	}

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(response.String()))
}

func (h *ConfigHandlers) handleGetMockMode(w http.ResponseWriter, r *http.Request) {
	status := struct {
		Enabled bool            `json:"enabled"`
		Config  mock.MockConfig `json:"config"`
	}{
		Enabled: mock.IsMockEnabled(),
		Config:  mock.GetConfig(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode mock mode status")
	}
}

type mockModeRequest struct {
	Enabled *bool `json:"enabled"`
	Config  struct {
		NodeCount      *int     `json:"nodeCount"`
		VMsPerNode     *int     `json:"vmsPerNode"`
		LXCsPerNode    *int     `json:"lxcsPerNode"`
		RandomMetrics  *bool    `json:"randomMetrics"`
		HighLoadNodes  []string `json:"highLoadNodes"`
		StoppedPercent *float64 `json:"stoppedPercent"`
	} `json:"config"`
}

func (h *ConfigHandlers) handleUpdateMockMode(w http.ResponseWriter, r *http.Request) {
	// Limit request body to 16KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var req mockModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Error().Err(err).Msg("Failed to decode mock mode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update configuration first if provided.
	currentCfg := mock.GetConfig()
	if req.Config.NodeCount != nil {
		if *req.Config.NodeCount <= 0 {
			http.Error(w, "nodeCount must be greater than zero", http.StatusBadRequest)
			return
		}
		currentCfg.NodeCount = *req.Config.NodeCount
	}
	if req.Config.VMsPerNode != nil {
		if *req.Config.VMsPerNode < 0 {
			http.Error(w, "vmsPerNode cannot be negative", http.StatusBadRequest)
			return
		}
		currentCfg.VMsPerNode = *req.Config.VMsPerNode
	}
	if req.Config.LXCsPerNode != nil {
		if *req.Config.LXCsPerNode < 0 {
			http.Error(w, "lxcsPerNode cannot be negative", http.StatusBadRequest)
			return
		}
		currentCfg.LXCsPerNode = *req.Config.LXCsPerNode
	}
	if req.Config.RandomMetrics != nil {
		currentCfg.RandomMetrics = *req.Config.RandomMetrics
	}
	if req.Config.HighLoadNodes != nil {
		currentCfg.HighLoadNodes = req.Config.HighLoadNodes
	}
	if req.Config.StoppedPercent != nil {
		if *req.Config.StoppedPercent < 0 || *req.Config.StoppedPercent > 1 {
			http.Error(w, "stoppedPercent must be between 0 and 1", http.StatusBadRequest)
			return
		}
		currentCfg.StoppedPercent = *req.Config.StoppedPercent
	}

	mock.SetMockConfig(currentCfg)

	if req.Enabled != nil {
		if h.getMonitor(r.Context()) != nil {
			h.getMonitor(r.Context()).SetMockMode(*req.Enabled)
		} else {
			mock.SetEnabled(*req.Enabled)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	status := struct {
		Enabled bool            `json:"enabled"`
		Config  mock.MockConfig `json:"config"`
	}{
		Enabled: mock.IsMockEnabled(),
		Config:  mock.GetConfig(),
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		log.Error().Err(err).Msg("Failed to encode mock mode response")
	}
}
