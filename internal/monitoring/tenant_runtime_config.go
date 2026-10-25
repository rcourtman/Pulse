package monitoring

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// loadTenantRuntimeConfig deliberately does not call config.Load: that loader
// owns the process environment, logger and default-org persistence. A tenant
// starts (including after import) from its own persisted resources and settings,
// retaining only the effective process settings and environment locks in base.
func loadTenantRuntimeConfig(base *config.Config, p *config.ConfigPersistence) (*config.Config, error) {
	if base == nil || p == nil {
		return nil, fmt.Errorf("tenant configuration is unavailable")
	}
	cfg := base.DeepCopy()
	cfg.DataPath, cfg.ConfigPath = p.GetConfigDir(), p.GetConfigDir()
	// A process-level override names one database file, not a tenant template.
	cfg.MetricsDBPath = ""
	cfg.PVEInstances, cfg.PBSInstances, cfg.PMGInstances = nil, nil, nil
	cfg.APITokens, cfg.APIToken = nil, ""
	nodes, err := p.LoadNodesConfig()
	if err != nil {
		return nil, fmt.Errorf("load tenant nodes: %w", err)
	}
	if nodes != nil {
		cfg.PVEInstances, cfg.PBSInstances, cfg.PMGInstances = nodes.PVEInstances, nodes.PBSInstances, nodes.PMGInstances
	}
	tokens, err := p.LoadAPITokens()
	if err != nil {
		return nil, fmt.Errorf("load tenant API tokens: %w", err)
	}
	cfg.APITokens = tokens
	cfg.SortAPITokens()
	s, err := p.LoadSystemSettings()
	if err != nil {
		return nil, fmt.Errorf("load tenant system settings: %w", err)
	}
	if s == nil {
		return cfg, nil
	}
	locked := func(keys ...string) bool {
		for _, key := range keys {
			if cfg.EnvOverrides[key] {
				return true
			}
		}
		return false
	}
	if s.PVEPollingInterval > 0 && !locked("PVE_POLLING_INTERVAL") {
		cfg.PVEPollingInterval = min(time.Duration(s.PVEPollingInterval)*time.Second, time.Hour)
	}
	if s.PBSPollingInterval > 0 && !locked("PBS_POLLING_INTERVAL") {
		cfg.PBSPollingInterval = time.Duration(s.PBSPollingInterval) * time.Second
	}
	if s.PMGPollingInterval > 0 && !locked("PMG_POLLING_INTERVAL") {
		cfg.PMGPollingInterval = time.Duration(s.PMGPollingInterval) * time.Second
	}
	if s.BackupPollingInterval >= 0 && !locked("BACKUP_POLLING_INTERVAL") {
		cfg.BackupPollingInterval = time.Duration(s.BackupPollingInterval) * time.Second
	}
	if s.BackupPollingEnabled != nil && !locked("ENABLE_BACKUP_POLLING") {
		cfg.EnableBackupPolling = *s.BackupPollingEnabled
	}
	if s.AdaptivePollingEnabled != nil && !locked("ADAPTIVE_POLLING_ENABLED") {
		cfg.AdaptivePollingEnabled = *s.AdaptivePollingEnabled
	}
	if s.AdaptivePollingBaseInterval > 0 && !locked("ADAPTIVE_POLLING_BASE_INTERVAL") {
		cfg.AdaptivePollingBaseInterval = time.Duration(s.AdaptivePollingBaseInterval) * time.Second
	}
	if s.AdaptivePollingMinInterval > 0 && !locked("ADAPTIVE_POLLING_MIN_INTERVAL") {
		cfg.AdaptivePollingMinInterval = time.Duration(s.AdaptivePollingMinInterval) * time.Second
	}
	if s.AdaptivePollingMaxInterval > 0 && !locked("ADAPTIVE_POLLING_MAX_INTERVAL") {
		cfg.AdaptivePollingMaxInterval = time.Duration(s.AdaptivePollingMaxInterval) * time.Second
	}
	cfg.UpdateChannel = config.EffectiveUpdateChannel(s.UpdateChannel, cfg.UpdateChannel)
	cfg.AutoUpdateEnabled = config.EffectiveAutoUpdateEnabled(cfg.UpdateChannel, s.AutoUpdateEnabled)
	if !locked("ALLOWED_ORIGINS", "allowedOrigins") {
		cfg.AllowedOrigins = s.AllowedOrigins
	}
	if s.ConnectionTimeout > 0 && !locked("connectionTimeout") {
		cfg.ConnectionTimeout = time.Duration(s.ConnectionTimeout) * time.Second
	}
	if s.LogLevel != "" && !locked("logLevel") {
		cfg.LogLevel = s.LogLevel
	}
	if !locked("discoveryEnabled") {
		cfg.DiscoveryEnabled = s.DiscoveryEnabled
	}
	if !locked("discoverySubnet") {
		cfg.DiscoverySubnet = s.DiscoverySubnet
	}
	discovery := config.NormalizeDiscoveryConfig(config.CloneDiscoveryConfig(s.DiscoveryConfig))
	// Merge only persisted discovery fields not fixed by the process environment.
	if locked("discoveryEnvironmentOverride") {
		discovery.EnvironmentOverride = cfg.Discovery.EnvironmentOverride
	}
	if locked("discoverySubnetAllowlist") {
		discovery.SubnetAllowlist = cfg.Discovery.SubnetAllowlist
	}
	if locked("discoverySubnetBlocklist") {
		discovery.SubnetBlocklist = cfg.Discovery.SubnetBlocklist
	}
	if locked("discoveryMaxHostsPerScan") {
		discovery.MaxHostsPerScan = cfg.Discovery.MaxHostsPerScan
	}
	if locked("discoveryMaxConcurrent") {
		discovery.MaxConcurrent = cfg.Discovery.MaxConcurrent
	}
	if locked("discoveryEnableReverseDns") {
		discovery.EnableReverseDNS = cfg.Discovery.EnableReverseDNS
	}
	if locked("discoveryScanGateways") {
		discovery.ScanGateways = cfg.Discovery.ScanGateways
	}
	if locked("discoveryDialTimeoutMs") {
		discovery.DialTimeout = cfg.Discovery.DialTimeout
	}
	if locked("discoveryHttpTimeoutMs") {
		discovery.HTTPTimeout = cfg.Discovery.HTTPTimeout
	}
	cfg.Discovery = discovery
	if !locked("temperatureMonitoringEnabled") {
		cfg.TemperatureMonitoringEnabled = s.TemperatureMonitoringEnabled
	}
	if s.DNSCacheTimeout > 0 && !locked("DNS_CACHE_TIMEOUT") {
		cfg.DNSCacheTimeout = time.Duration(s.DNSCacheTimeout) * time.Second
	}
	if !locked("SSH_PORT") {
		cfg.SSHPort = s.SSHPort
		if cfg.SSHPort <= 0 {
			cfg.SSHPort = 22
		}
	}
	if !locked("PULSE_AUTH_HIDE_LOCAL_LOGIN", "hideLocalLogin") {
		cfg.HideLocalLogin = s.HideLocalLogin
	}
	if !locked("PULSE_DISABLE_DOCKER_UPDATE_ACTIONS", "disableDockerUpdateActions") {
		cfg.DisableDockerUpdateActions = s.DisableDockerUpdateActions
	}
	if !locked("PULSE_ENABLE_PROXMOX_GUEST_DOCKER_INVENTORY", "enableProxmoxGuestDockerInventory") {
		cfg.EnableProxmoxGuestDockerInventory = s.EnableProxmoxGuestDockerInventory
	}
	if !locked("PULSE_TELEMETRY", "telemetryEnabled") {
		cfg.TelemetryEnabled = s.TelemetryEnabled == nil || *s.TelemetryEnabled
	}
	if !locked("publicURL") {
		cfg.PublicURL = s.PublicURL
	}
	if s.MetricsRetentionRawHours > 0 {
		cfg.MetricsRetentionRawHours = s.MetricsRetentionRawHours
	}
	if s.MetricsRetentionMinuteHours > 0 {
		cfg.MetricsRetentionMinuteHours = s.MetricsRetentionMinuteHours
	}
	if s.MetricsRetentionHourlyDays > 0 {
		cfg.MetricsRetentionHourlyDays = s.MetricsRetentionHourlyDays
	}
	if s.MetricsRetentionDailyDays > 0 {
		cfg.MetricsRetentionDailyDays = s.MetricsRetentionDailyDays
	}
	return cfg, nil
}

// ReloadTenant rebuilds only the selected organization's runtime from its own
// persisted configuration. Never use the instance-wide reload for this path.
// removeTenant retains its availability guard when shutdown is incomplete.
func (mtm *MultiTenantMonitor) ReloadTenant(orgID string) (*Monitor, error) {
	orgID = strings.TrimSpace(orgID)
	if mtm == nil || orgID == "" || orgID == "default" {
		return nil, fmt.Errorf("tenant reload requires a non-default organization")
	}
	if err := mtm.removeTenant(orgID, false); err != nil {
		return nil, err
	}
	return mtm.GetMonitor(orgID)
}
