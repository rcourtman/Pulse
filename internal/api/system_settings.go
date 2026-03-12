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
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/discovery"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
	"github.com/rcourtman/pulse-go-rewrite/internal/websocket"
	"github.com/rs/zerolog/log"
)

// SystemSettingsMonitor defines the monitor interface needed by system settings
type SystemSettingsMonitor interface {
	GetDiscoveryService() *discovery.Service
	StartDiscoveryService(ctx context.Context, wsHub *websocket.Hub, subnet string)
	StopDiscoveryService()
	EnableTemperatureMonitoring()
	DisableTemperatureMonitoring()
	GetNotificationManager() *notifications.NotificationManager
}

// SystemSettingsHandler handles system settings
type SystemSettingsHandler struct {
	stateMu                  sync.RWMutex
	config                   *config.Config
	persistence              *config.ConfigPersistence
	wsHub                    *websocket.Hub
	reloadSystemSettingsFunc func() // Function to reload cached system settings
	reloadMonitorFunc        func() error
	telemetryToggleFunc      func(enabled bool) // Called when telemetry is toggled at runtime
	mtMonitor                interface {
		GetMonitor(string) (*monitoring.Monitor, error)
	}
	defaultMonitor SystemSettingsMonitor
}

// NewSystemSettingsHandler creates a new system settings handler
func NewSystemSettingsHandler(cfg *config.Config, persistence *config.ConfigPersistence, wsHub *websocket.Hub, mtm *monitoring.MultiTenantMonitor, monitor SystemSettingsMonitor, reloadSystemSettingsFunc func(), reloadMonitorFunc func() error) *SystemSettingsHandler {
	// If mtm is provided, try to populate defaultMonitor from "default" org if not provided.
	if monitor == nil && mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			monitor = m
		}
	}
	return &SystemSettingsHandler{
		config:                   cfg,
		persistence:              persistence,
		wsHub:                    wsHub,
		mtMonitor:                mtm,
		defaultMonitor:           monitor,
		reloadSystemSettingsFunc: reloadSystemSettingsFunc,
		reloadMonitorFunc:        reloadMonitorFunc,
	}
}

// SetTelemetryToggleFunc sets the callback invoked when telemetry is toggled
// at runtime (true = start, false = stop).
func (h *SystemSettingsHandler) SetTelemetryToggleFunc(fn func(enabled bool)) {
	h.telemetryToggleFunc = fn
}

// SetMonitor updates the monitor reference used by the handler at runtime.
func (h *SystemSettingsHandler) SetMonitor(m SystemSettingsMonitor) {
	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.defaultMonitor = m
}

// SetMultiTenantMonitor updates the multi-tenant monitor reference
func (h *SystemSettingsHandler) SetMultiTenantMonitor(mtm *monitoring.MultiTenantMonitor) {
	var defaultMonitor SystemSettingsMonitor
	if mtm != nil {
		if m, err := mtm.GetMonitor("default"); err == nil {
			defaultMonitor = m
		}
	}

	h.stateMu.Lock()
	defer h.stateMu.Unlock()
	h.mtMonitor = mtm
	if defaultMonitor != nil {
		h.defaultMonitor = defaultMonitor
	}
}

func (h *SystemSettingsHandler) getMonitor(ctx context.Context) SystemSettingsMonitor {
	h.stateMu.RLock()
	mtMonitor := h.mtMonitor
	defaultMonitor := h.defaultMonitor
	h.stateMu.RUnlock()

	if mtMonitor != nil {
		orgID := GetOrgID(ctx)
		if m, err := mtMonitor.GetMonitor(orgID); err == nil && m != nil {
			return m
		}
	}
	return defaultMonitor
}

// SetConfig updates the configuration reference used by the handler.
func (h *SystemSettingsHandler) SetConfig(cfg *config.Config) {
	if cfg == nil {
		return
	}
	h.config = cfg
}

func discoveryConfigMap(raw map[string]interface{}) (map[string]interface{}, bool) {
	if raw == nil {
		return nil, false
	}
	if val, ok := raw["discoveryConfig"]; ok {
		if cfgMap, ok := val.(map[string]interface{}); ok {
			return cfgMap, true
		}
		return nil, true
	}
	return nil, false
}

func parseDiscoveryStringArray(value interface{}, field string) ([]string, error) {
	items, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%s must be an array of CIDR strings", field)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		entry, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s entries must be strings", field)
		}
		out = append(out, entry)
	}
	return out, nil
}

func parseDiscoveryWholeNumber(value interface{}, field string) (int, error) {
	raw, ok := value.(float64)
	if !ok {
		return 0, fmt.Errorf("%s must be a number", field)
	}
	if raw != float64(int(raw)) {
		return 0, fmt.Errorf("%s must be a whole number", field)
	}
	return int(raw), nil
}

func applyDiscoveryConfigOverrides(current config.DiscoveryConfig, cfgMap map[string]interface{}) (config.DiscoveryConfig, error) {
	if envVal, ok := cfgMap["environmentOverride"]; ok {
		envStr, ok := envVal.(string)
		if !ok {
			return current, fmt.Errorf("discoveryConfig.environmentOverride must be a string")
		}
		canonicalEnv, valid := config.CanonicalDiscoveryEnvironment(envStr)
		if !valid {
			return current, fmt.Errorf("invalid discovery environment override: %s", envStr)
		}
		current.EnvironmentOverride = canonicalEnv
	}

	if val, ok := cfgMap["subnetAllowlist"]; ok {
		allowlist, err := parseDiscoveryStringArray(val, "discoveryConfig.subnetAllowlist")
		if err != nil {
			return current, err
		}
		current.SubnetAllowlist = allowlist
	}

	if val, ok := cfgMap["subnetBlocklist"]; ok {
		blocklist, err := parseDiscoveryStringArray(val, "discoveryConfig.subnetBlocklist")
		if err != nil {
			return current, err
		}
		current.SubnetBlocklist = blocklist
	}

	if val, ok := cfgMap["maxHostsPerScan"]; ok {
		maxHosts, err := parseDiscoveryWholeNumber(val, "discoveryConfig.maxHostsPerScan")
		if err != nil {
			return current, err
		}
		current.MaxHostsPerScan = maxHosts
	}

	if val, ok := cfgMap["maxConcurrent"]; ok {
		maxConcurrent, err := parseDiscoveryWholeNumber(val, "discoveryConfig.maxConcurrent")
		if err != nil {
			return current, err
		}
		current.MaxConcurrent = maxConcurrent
	}

	if val, ok := cfgMap["enableReverseDns"]; ok {
		enabled, ok := val.(bool)
		if !ok {
			return current, fmt.Errorf("discoveryConfig.enableReverseDns must be a boolean")
		}
		current.EnableReverseDNS = enabled
	}

	if val, ok := cfgMap["scanGateways"]; ok {
		enabled, ok := val.(bool)
		if !ok {
			return current, fmt.Errorf("discoveryConfig.scanGateways must be a boolean")
		}
		current.ScanGateways = enabled
	}

	if val, ok := cfgMap["dialTimeoutMs"]; ok {
		dialTimeout, err := parseDiscoveryWholeNumber(val, "discoveryConfig.dialTimeoutMs")
		if err != nil {
			return current, err
		}
		current.DialTimeout = dialTimeout
	}

	if val, ok := cfgMap["httpTimeoutMs"]; ok {
		httpTimeout, err := parseDiscoveryWholeNumber(val, "discoveryConfig.httpTimeoutMs")
		if err != nil {
			return current, err
		}
		current.HTTPTimeout = httpTimeout
	}

	return config.NormalizeDiscoveryConfig(current), nil
}

// validateSystemSettings validates settings before applying them
func validateSystemSettings(_ *config.SystemSettings, rawRequest map[string]interface{}) error {
	if val, ok := rawRequest["pvePollingInterval"]; ok {
		if interval, ok := val.(float64); ok {
			if interval <= 0 {
				return fmt.Errorf("PVE polling interval must be positive (minimum 10 seconds)")
			}
			if interval < 10 {
				return fmt.Errorf("PVE polling interval must be at least 10 seconds")
			}
			if interval > 3600 {
				return fmt.Errorf("PVE polling interval cannot exceed 3600 seconds (1 hour)")
			}
		} else {
			return fmt.Errorf("PVE polling interval must be a number")
		}
	}

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
				return fmt.Errorf("backup polling interval cannot be negative")
			}
			if interval > 0 && interval < 10 {
				return fmt.Errorf("backup polling interval must be at least 10 seconds")
			}
			if interval > 604800 {
				return fmt.Errorf("backup polling interval cannot exceed 604800 seconds (7 days)")
			}
		} else {
			return fmt.Errorf("backup polling interval must be a number")
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

	if val, ok := rawRequest["temperatureMonitoringEnabled"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("temperatureMonitoringEnabled must be a boolean")
		}
	}

	if val, ok := rawRequest["disableLocalUpgradeMetrics"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("disableLocalUpgradeMetrics must be a boolean")
		}
	}

	if val, ok := rawRequest["reduceProUpsellNoise"]; ok {
		if _, ok := val.(bool); !ok {
			return fmt.Errorf("reduceProUpsellNoise must be a boolean")
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

	if cfgMap, cfgProvided := discoveryConfigMap(rawRequest); cfgProvided {
		if cfgMap == nil {
			return fmt.Errorf("discoveryConfig must be an object")
		}

		allowedDiscoveryConfigFields := map[string]struct{}{
			"environmentOverride": {},
			"subnetAllowlist":     {},
			"subnetBlocklist":     {},
			"maxHostsPerScan":     {},
			"maxConcurrent":       {},
			"enableReverseDns":    {},
			"scanGateways":        {},
			"dialTimeoutMs":       {},
			"httpTimeoutMs":       {},
		}
		for key := range cfgMap {
			if _, ok := allowedDiscoveryConfigFields[key]; !ok {
				return fmt.Errorf("discoveryConfig.%s is not supported", key)
			}
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
			if value <= 0 || value != float64(int(value)) {
				return fmt.Errorf("discoveryConfig.maxHostsPerScan must be a whole number greater than zero")
			}
		}

		if concurrentVal, exists := cfgMap["maxConcurrent"]; exists {
			value, ok := concurrentVal.(float64)
			if !ok {
				return fmt.Errorf("discoveryConfig.maxConcurrent must be a number")
			}
			if value <= 0 || value > 1000 || value != float64(int(value)) {
				return fmt.Errorf("discoveryConfig.maxConcurrent must be a whole number between 1 and 1000")
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
			if timeout <= 0 || timeout != float64(int(timeout)) {
				return fmt.Errorf("discoveryConfig.dialTimeoutMs must be a whole number greater than zero")
			}
		}

		if val, exists := cfgMap["httpTimeoutMs"]; exists {
			timeout, ok := val.(float64)
			if !ok {
				return fmt.Errorf("discoveryConfig.httpTimeoutMs must be a number")
			}
			if timeout <= 0 || timeout != float64(int(timeout)) {
				return fmt.Errorf("discoveryConfig.httpTimeoutMs must be a whole number greater than zero")
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
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	// SECURITY: Session users must match configured admin identity for settings reads.
	if !ensureSettingsReadScope(h.config, w, r) {
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

		settings.UpdateChannel = config.EffectiveUpdateChannel(settings.UpdateChannel, h.config.UpdateChannel)
		// Always expose effective backup polling configuration
		settings.PVEPollingInterval = int(h.config.PVEPollingInterval.Seconds())
		settings.BackupPollingInterval = int(h.config.BackupPollingInterval.Seconds())
		enabled := h.config.EnableBackupPolling
		settings.BackupPollingEnabled = &enabled
		settings.DiscoveryConfig = config.CloneDiscoveryConfig(h.config.Discovery)
		settings.TemperatureMonitoringEnabled = h.config.TemperatureMonitoringEnabled
		// Expose Docker update actions setting (respects env override)
		settings.DisableDockerUpdateActions = h.config.DisableDockerUpdateActions
		// Expose effective telemetry value (respects env override)
		effectiveTelemetry := h.config.TelemetryEnabled
		settings.TelemetryEnabled = &effectiveTelemetry
		settings.AutoUpdateEnabled = config.EffectiveAutoUpdateEnabled(settings.UpdateChannel, settings.AutoUpdateEnabled)
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
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
		return
	}

	// Require authentication
	if !CheckAuth(h.config, w, r) {
		// CheckAuth handles explicit failures (rate limits, invalid tokens)
		// but returns false silently for missing credentials.
		// We ensure a 401 is returned nicely even if we risk a double-write warning in rare cases.
		writeErrorResponse(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", nil)
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
				writeErrorResponse(w, http.StatusForbidden, "forbidden", "Admin privileges required", nil)
				return
			}
		}
	}

	// SECURITY: Session users must match configured admin identity for settings writes.
	if !ensureSettingsWriteScope(h.config, w, r) {
		return
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

	// Limit request body to 64KB to prevent memory exhaustion
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)

	// Read the request body into a map to check which fields were provided
	var rawRequest map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawRequest); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	// Convert the map back to JSON for decoding into struct
	jsonBytes, err := json.Marshal(rawRequest)
	if err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	// Decode into updates struct
	var updates config.SystemSettings
	if err := json.Unmarshal(jsonBytes, &updates); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid request body", nil)
		return
	}

	// Validate the settings
	if err := validateSystemSettings(&updates, rawRequest); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
		return
	}

	// Reject early if telemetryEnabled is locked by env var
	if _, ok := rawRequest["telemetryEnabled"]; ok && h.config.EnvOverrides["PULSE_TELEMETRY"] {
		writeErrorResponse(w, http.StatusConflict, "env_locked", "telemetryEnabled is locked by the PULSE_TELEMETRY environment variable", nil)
		return
	}

	// Start with existing settings
	settings := *existingSettings
	discoveryConfigUpdated := false
	prevTempEnabled := h.config.TemperatureMonitoringEnabled
	tempToggleRequested := false
	pveIntervalChanged := false

	// Only update fields that were provided in the request
	if _, ok := rawRequest["pvePollingInterval"]; ok {
		settings.PVEPollingInterval = updates.PVEPollingInterval
	}
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
	if cfgMap, ok := discoveryConfigMap(rawRequest); ok && cfgMap != nil {
		current := config.CloneDiscoveryConfig(settings.DiscoveryConfig)
		normalizedDiscoveryConfig, err := applyDiscoveryConfigOverrides(current, cfgMap)
		if err != nil {
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}
		settings.DiscoveryConfig = normalizedDiscoveryConfig
		discoveryConfigUpdated = true
	}
	// Allow clearing of AllowedEmbedOrigins by setting to empty string
	if _, ok := rawRequest["allowedEmbedOrigins"]; ok {
		settings.AllowedEmbedOrigins = updates.AllowedEmbedOrigins
	}
	// Allow configuring webhook private CIDR allowlist
	if _, ok := rawRequest["webhookAllowedPrivateCIDRs"]; ok {
		settings.WebhookAllowedPrivateCIDRs = updates.WebhookAllowedPrivateCIDRs
	}
	// Allow configuring public URL for notifications (used in email alerts)
	if _, ok := rawRequest["publicURL"]; ok {
		settings.PublicURL = updates.PublicURL
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
	if _, ok := rawRequest["hideLocalLogin"]; ok {
		settings.HideLocalLogin = updates.HideLocalLogin
	}
	if _, ok := rawRequest["backupPollingEnabled"]; ok {
		settings.BackupPollingEnabled = updates.BackupPollingEnabled
	}
	if _, ok := rawRequest["temperatureMonitoringEnabled"]; ok {
		settings.TemperatureMonitoringEnabled = updates.TemperatureMonitoringEnabled
		tempToggleRequested = true
	}
	if _, ok := rawRequest["disableDockerUpdateActions"]; ok {
		settings.DisableDockerUpdateActions = updates.DisableDockerUpdateActions
	}
	if _, ok := rawRequest["disableLocalUpgradeMetrics"]; ok {
		settings.DisableLocalUpgradeMetrics = updates.DisableLocalUpgradeMetrics
	}
	if _, ok := rawRequest["reduceProUpsellNoise"]; ok {
		settings.ReduceProUpsellNoise = updates.ReduceProUpsellNoise
	}
	if _, ok := rawRequest["telemetryEnabled"]; ok && updates.TelemetryEnabled != nil {
		settings.TelemetryEnabled = updates.TelemetryEnabled
		// Note: h.config.TelemetryEnabled is updated after successful persistence below
	}
	if _, ok := rawRequest["fullWidthMode"]; ok {
		settings.FullWidthMode = updates.FullWidthMode
	}
	settings.UpdateChannel = config.EffectiveUpdateChannel(settings.UpdateChannel, h.config.UpdateChannel)
	settings.AutoUpdateEnabled = config.EffectiveAutoUpdateEnabled(settings.UpdateChannel, settings.AutoUpdateEnabled)

	// Pre-save validation (may return errors before anything is persisted)
	if settings.Theme != "" && settings.Theme != "light" && settings.Theme != "dark" {
		writeErrorResponse(w, http.StatusBadRequest, "validation_error", "Invalid theme value. Must be 'light', 'dark', or empty", nil)
		return
	}
	// Validate CIDRs before persistence (parse only — do NOT mutate runtime yet).
	var parsedCIDRNets []*net.IPNet
	cidrUpdateRequested := false
	if _, ok := rawRequest["webhookAllowedPrivateCIDRs"]; ok {
		cidrUpdateRequested = true
		var err error
		parsedCIDRNets, err = notifications.ParseAllowedPrivateCIDRs(settings.WebhookAllowedPrivateCIDRs)
		if err != nil {
			log.Error().Err(err).Msg("Failed to validate webhook allowed private CIDRs")
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("Invalid webhook allowed private CIDRs: %v", err), nil)
			return
		}
	}

	// Detect PVE interval change (compare before mutation, apply after save)
	if _, ok := rawRequest["pvePollingInterval"]; ok && settings.PVEPollingInterval > 0 {
		newInterval := time.Duration(settings.PVEPollingInterval) * time.Second
		if newInterval < 10*time.Second {
			newInterval = 10 * time.Second
		}
		if h.config.PVEPollingInterval != newInterval {
			pveIntervalChanged = true
		}
	}

	prevDiscoveryEnabled := h.config.DiscoveryEnabled

	// ---- Persist FIRST, then apply to in-memory config ----
	// This ensures runtime state never diverges from disk on save failure.
	if err := h.persistence.SaveSystemSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save system settings")
		writeErrorResponse(w, http.StatusInternalServerError, "save_failed", "Failed to save settings", nil)
		return
	}

	// ---- Apply all in-memory config mutations (safe: disk is committed) ----
	if _, ok := rawRequest["pvePollingInterval"]; ok && settings.PVEPollingInterval > 0 {
		newInterval := time.Duration(settings.PVEPollingInterval) * time.Second
		if newInterval < 10*time.Second {
			newInterval = 10 * time.Second
		}
		h.config.PVEPollingInterval = newInterval
	}
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
	h.config.UpdateChannel = settings.UpdateChannel
	h.config.AutoUpdateEnabled = config.EffectiveAutoUpdateEnabled(settings.UpdateChannel, settings.AutoUpdateEnabled)
	if settings.AutoUpdateCheckInterval > 0 {
		h.config.AutoUpdateCheckInterval = time.Duration(settings.AutoUpdateCheckInterval) * time.Hour
	}
	if settings.AutoUpdateTime != "" {
		h.config.AutoUpdateTime = settings.AutoUpdateTime
	}
	h.config.DiscoveryEnabled = settings.DiscoveryEnabled
	if settings.DiscoverySubnet != "" {
		h.config.DiscoverySubnet = settings.DiscoverySubnet
	}
	h.config.Discovery = config.CloneDiscoveryConfig(settings.DiscoveryConfig)
	if tempToggleRequested {
		h.config.TemperatureMonitoringEnabled = settings.TemperatureMonitoringEnabled
	}
	h.config.DisableDockerUpdateActions = settings.DisableDockerUpdateActions
	h.config.DisableLocalUpgradeMetrics = settings.DisableLocalUpgradeMetrics
	if _, ok := rawRequest["telemetryEnabled"]; ok && settings.TelemetryEnabled != nil {
		h.config.TelemetryEnabled = *settings.TelemetryEnabled
		if h.telemetryToggleFunc != nil {
			h.telemetryToggleFunc(*settings.TelemetryEnabled)
		}
	}
	if _, ok := rawRequest["publicURL"]; ok {
		h.config.PublicURL = settings.PublicURL
	}

	// ---- Side effects (discovery, temperature, notifications) ----
	if h.getMonitor(r.Context()) != nil {
		if settings.DiscoveryEnabled && !prevDiscoveryEnabled {
			subnet := h.config.DiscoverySubnet
			if subnet == "" {
				subnet = "auto"
			}
			h.getMonitor(r.Context()).StartDiscoveryService(context.Background(), h.wsHub, subnet)
			log.Info().Msg("Discovery service started via settings update")
		} else if !settings.DiscoveryEnabled && prevDiscoveryEnabled {
			h.getMonitor(r.Context()).StopDiscoveryService()
			log.Info().Msg("Discovery service stopped via settings update")
		} else if settings.DiscoveryEnabled && settings.DiscoverySubnet != "" {
			if svc := h.getMonitor(r.Context()).GetDiscoveryService(); svc != nil {
				svc.SetSubnet(settings.DiscoverySubnet)
			}
		}
		if discoveryConfigUpdated && settings.DiscoveryEnabled {
			if svc := h.getMonitor(r.Context()).GetDiscoveryService(); svc != nil {
				log.Info().Msg("Discovery configuration changed; triggering refresh")
				svc.ForceRefresh()
			}
		}
	}
	if tempToggleRequested && h.getMonitor(r.Context()) != nil {
		if settings.TemperatureMonitoringEnabled && !prevTempEnabled {
			h.getMonitor(r.Context()).EnableTemperatureMonitoring()
		} else if !settings.TemperatureMonitoringEnabled && prevTempEnabled {
			h.getMonitor(r.Context()).DisableTemperatureMonitoring()
		}
	}
	if cidrUpdateRequested && h.getMonitor(r.Context()) != nil {
		if nm := h.getMonitor(r.Context()).GetNotificationManager(); nm != nil {
			nm.ApplyAllowedPrivateCIDRs(settings.WebhookAllowedPrivateCIDRs, parsedCIDRNets)
		}
	}
	if _, ok := rawRequest["publicURL"]; ok {
		if h.getMonitor(r.Context()) != nil {
			if nm := h.getMonitor(r.Context()).GetNotificationManager(); nm != nil {
				nm.SetPublicURL(settings.PublicURL)
				log.Info().Str("publicURL", settings.PublicURL).Msg("Updated notification public URL from settings")
			}
		}
	}

	// Reload cached system settings after successful save
	if h.reloadSystemSettingsFunc != nil {
		h.reloadSystemSettingsFunc()
	}

	if pveIntervalChanged && h.reloadMonitorFunc != nil {
		if err := h.reloadMonitorFunc(); err != nil {
			log.Error().Err(err).Msg("Failed to reload monitor after PVE polling interval change")
			writeErrorResponse(w, http.StatusInternalServerError, "reload_failed", "Configuration saved but failed to reload monitor", nil)
			return
		}
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
		writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", nil)
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
			writeErrorResponse(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body too large", nil)
			return
		}
		log.Error().Err(err).Msg("Failed to read SSH config from request")
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Failed to read request body", nil)
		return
	}

	// Basic validation: ensure it looks like SSH config
	configStr := string(sshConfig)
	if len(configStr) == 0 {
		log.Error().Msg("Empty SSH config received")
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Empty SSH config", nil)
		return
	}

	// Security: Use allowlist-based validation (safer than blocklist)
	// Only permit the specific directives Pulse needs for ProxyJump
	allowedDirectives := map[string]bool{
		"host":                  true,
		"hostname":              true,
		"proxyjump":             true,
		"user":                  true,
		"identityfile":          true,
		"stricthostkeychecking": true,
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
			writeErrorResponse(w, http.StatusBadRequest, "validation_error", fmt.Sprintf("SSH config contains forbidden directive: %s", fields[0]), nil)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("Failed to parse SSH config")
		writeErrorResponse(w, http.StatusBadRequest, "invalid_request", "Invalid SSH config format", nil)
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
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to create SSH directory", nil)
		return
	}
	// Harden permissions even when the directory already existed.
	if err := os.Chmod(sshDir, 0700); err != nil {
		log.Error().Err(err).Str("dir", sshDir).Msg("Failed to set .ssh directory permissions")
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to secure SSH directory", nil)
		return
	}

	// Write SSH config file
	configPath := filepath.Join(sshDir, "config")
	if err := os.WriteFile(configPath, sshConfig, 0600); err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("Failed to write SSH config")
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to write SSH config", nil)
		return
	}
	// os.WriteFile does not change mode on existing files; enforce least-privilege.
	if err := os.Chmod(configPath, 0600); err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("Failed to set SSH config file permissions")
		writeErrorResponse(w, http.StatusInternalServerError, "internal_error", "Failed to secure SSH config", nil)
		return
	}

	log.Info().Str("path", configPath).Int("size", len(sshConfig)).Msg("SSH config written successfully")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		log.Error().Err(err).Msg("Failed to encode success response")
	}
}
