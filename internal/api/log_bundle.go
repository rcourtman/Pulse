package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// This is deliberately not config.Config, even a redacted copy. New settings,
// free-form URLs, paths, names and credentials must not enter a diagnostic
// archive just because a new configuration field becomes JSON-serializable.
// Connection order is retained so log references can be compared with settings.
type supportBundleConfig struct {
	FrontendPort                 int
	AgentIngestPort              int
	PVEPollingInterval           time.Duration
	PBSPollingInterval           time.Duration
	PMGPollingInterval           time.Duration
	ConnectionTimeout            time.Duration
	BackupPollingInterval        time.Duration
	BackupPollingCycles          int
	EnableBackupPolling          bool
	TemperatureMonitoringEnabled bool
	AdaptivePollingEnabled       bool
	AdaptivePollingBaseInterval  time.Duration
	AdaptivePollingMinInterval   time.Duration
	AdaptivePollingMaxInterval   time.Duration
	MetricsRetentionRawHours     int
	MetricsRetentionMinuteHours  int
	MetricsRetentionHourlyDays   int
	MetricsRetentionDailyDays    int
	LogLevel                     string
	LogFormat                    string
	LogMaxSize                   int
	LogMaxAge                    int
	LogCompress                  bool
	HTTPSEnabled                 bool
	HideLocalLogin               bool
	MultiTenantEnabled           bool
	DiscoveryEnabled             bool
	UpdateChannel                string
	AutoUpdateEnabled            bool
	PVEInstances                 []supportBundleConnection
	PBSInstances                 []supportBundleConnection
	PMGInstances                 []supportBundleConnection
}

type supportBundleConnection struct {
	PasswordConfigured    bool
	TokenConfigured       bool
	VerifySSL             bool
	FingerprintConfigured bool
	Disabled              bool
	MonitoredSurfaces     []string
}

// Caller holds config.Mu.RLock. Only scalar values and newly allocated slices
// cross this boundary, so archive writing neither aliases nor mutates live state.
func newSupportBundleConfig(cfg *config.Config) supportBundleConfig {
	out := supportBundleConfig{
		FrontendPort: cfg.FrontendPort, AgentIngestPort: cfg.AgentIngestPort,
		PVEPollingInterval: cfg.PVEPollingInterval, PBSPollingInterval: cfg.PBSPollingInterval,
		PMGPollingInterval: cfg.PMGPollingInterval, ConnectionTimeout: cfg.ConnectionTimeout,
		BackupPollingInterval: cfg.BackupPollingInterval, BackupPollingCycles: cfg.BackupPollingCycles,
		EnableBackupPolling: cfg.EnableBackupPolling, TemperatureMonitoringEnabled: cfg.TemperatureMonitoringEnabled,
		AdaptivePollingEnabled: cfg.AdaptivePollingEnabled, AdaptivePollingBaseInterval: cfg.AdaptivePollingBaseInterval,
		AdaptivePollingMinInterval: cfg.AdaptivePollingMinInterval, AdaptivePollingMaxInterval: cfg.AdaptivePollingMaxInterval,
		MetricsRetentionRawHours: cfg.MetricsRetentionRawHours, MetricsRetentionMinuteHours: cfg.MetricsRetentionMinuteHours,
		MetricsRetentionHourlyDays: cfg.MetricsRetentionHourlyDays, MetricsRetentionDailyDays: cfg.MetricsRetentionDailyDays,
		LogLevel:   supportBundleEnum(cfg.LogLevel, "debug", "info", "warn", "error", "trace", "disabled"),
		LogFormat:  supportBundleEnum(cfg.LogFormat, "auto", "json", "console"),
		LogMaxSize: cfg.LogMaxSize, LogMaxAge: cfg.LogMaxAge, LogCompress: cfg.LogCompress,
		HTTPSEnabled: cfg.HTTPSEnabled, HideLocalLogin: cfg.HideLocalLogin, MultiTenantEnabled: cfg.MultiTenantEnabled,
		DiscoveryEnabled: cfg.DiscoveryEnabled,
		UpdateChannel:    supportBundleEnum(cfg.UpdateChannel, "stable", "rc", "beta"), AutoUpdateEnabled: cfg.AutoUpdateEnabled,
		PVEInstances: make([]supportBundleConnection, 0, len(cfg.PVEInstances)),
		PBSInstances: make([]supportBundleConnection, 0, len(cfg.PBSInstances)),
		PMGInstances: make([]supportBundleConnection, 0, len(cfg.PMGInstances)),
	}
	for _, c := range cfg.PVEInstances {
		out.PVEInstances = append(out.PVEInstances, supportBundleConnection{
			PasswordConfigured: c.Password != "", TokenConfigured: c.TokenValue != "", VerifySSL: c.VerifySSL,
			FingerprintConfigured: c.Fingerprint != "", Disabled: c.Disabled,
			MonitoredSurfaces: supportBundleSurfaces(
				[]string{"vms", "containers", "storage", "backups"},
				[]bool{c.MonitorVMs, c.MonitorContainers, c.MonitorStorage, c.MonitorBackups}),
		})
	}
	for _, c := range cfg.PBSInstances {
		out.PBSInstances = append(out.PBSInstances, supportBundleConnection{
			PasswordConfigured: c.Password != "", TokenConfigured: c.TokenValue != "", VerifySSL: c.VerifySSL,
			FingerprintConfigured: c.Fingerprint != "", Disabled: c.Disabled,
			MonitoredSurfaces: supportBundleSurfaces(
				[]string{"backups", "datastores", "sync", "verify", "prune", "garbage"},
				[]bool{c.MonitorBackups, c.MonitorDatastores, c.MonitorSyncJobs, c.MonitorVerifyJobs, c.MonitorPruneJobs, c.MonitorGarbageJobs}),
		})
	}
	for _, c := range cfg.PMGInstances {
		out.PMGInstances = append(out.PMGInstances, supportBundleConnection{
			PasswordConfigured: c.Password != "", TokenConfigured: c.TokenValue != "", VerifySSL: c.VerifySSL,
			FingerprintConfigured: c.Fingerprint != "", Disabled: c.Disabled,
			MonitoredSurfaces: supportBundleSurfaces(
				[]string{"mail", "queues", "quarantine", "domains"},
				[]bool{c.MailStatsEnabled(), c.MonitorQueues, c.MonitorQuarantine, c.MonitorDomainStats}),
		})
	}
	return out
}

func supportBundleSurfaces(names []string, enabled []bool) []string {
	out := make([]string, 0, len(names))
	for i, name := range names {
		if enabled[i] {
			out = append(out, name)
		}
	}
	return out
}

func supportBundleEnum(value string, allowed ...string) string {
	if value == "" {
		return ""
	}
	for _, candidate := range allowed {
		if value == candidate {
			return value
		}
	}
	return "[REDACTED]"
}

// Export only Go runtime tuning values, never arbitrary process environment
// names/values (proxy URLs, DSNs and provider options can contain credentials).
// Even an allowlisted name needs a bounded value grammar; invalid values reveal
// only that the setting was supplied, not its contents.
func supportBundleEnvironment(environ []string) []string {
	out := make([]string, 0, 3)
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		valid := false
		switch name {
		case "GOMAXPROCS":
			valid = supportBundleUnsigned(value, false)
		case "GOGC":
			valid = value == "off" || supportBundleUnsigned(value, true)
		case "GOMEMLIMIT":
			valid = value == "off"
			for _, suffix := range []string{"B", "KiB", "MiB", "GiB", "TiB"} {
				if strings.HasSuffix(value, suffix) && supportBundleUnsigned(strings.TrimSuffix(value, suffix), true) {
					valid = true
					break
				}
			}
		default:
			continue
		}
		if !valid {
			value = "[REDACTED]"
		}
		out = append(out, name+"="+value)
	}
	return out
}

func supportBundleUnsigned(value string, zero bool) bool {
	if value == "" || len(value) > 19 {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, err := strconv.ParseUint(value, 10, 63)
	return err == nil && (zero || n > 0)
}
