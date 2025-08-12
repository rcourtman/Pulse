package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Config holds all application configuration
// NOTE: The envconfig tags are legacy and not used - configuration is loaded from encrypted JSON files
type Config struct {
	// Server settings
	BackendHost   string `envconfig:"BACKEND_HOST" default:"0.0.0.0"`
	BackendPort   int    `envconfig:"BACKEND_PORT" default:"3000"`
	FrontendHost  string `envconfig:"FRONTEND_HOST" default:"0.0.0.0"`
	FrontendPort  int    `envconfig:"FRONTEND_PORT" default:"7655"`
	ConfigPath    string `envconfig:"CONFIG_PATH" default:"/etc/pulse"`
	DataPath      string `envconfig:"DATA_PATH" default:"/var/lib/pulse"`

	// Proxmox VE connections
	PVEInstances []PVEInstance

	// Proxmox Backup Server connections
	PBSInstances []PBSInstance

	// Monitoring settings
	PollingInterval      time.Duration `envconfig:"POLLING_INTERVAL"` // Loaded from system.json
	ConcurrentPolling    bool          `envconfig:"CONCURRENT_POLLING" default:"true"`
	ConnectionTimeout    time.Duration `envconfig:"CONNECTION_TIMEOUT" default:"10s"`
	MetricsRetentionDays int           `envconfig:"METRICS_RETENTION_DAYS" default:"7"`
	BackupPollingCycles  int           `envconfig:"BACKUP_POLLING_CYCLES" default:"10"`
	WebhookBatchDelay    time.Duration `envconfig:"WEBHOOK_BATCH_DELAY" default:"10s"`

	// Logging settings
	LogLevel    string `envconfig:"LOG_LEVEL" default:"info"`
	LogFile     string `envconfig:"LOG_FILE" default:""`
	LogMaxSize  int    `envconfig:"LOG_MAX_SIZE" default:"100"` // MB
	LogMaxAge   int    `envconfig:"LOG_MAX_AGE" default:"30"`   // days
	LogCompress bool   `envconfig:"LOG_COMPRESS" default:"true"`

	// Security settings
	APIToken             string `envconfig:"API_TOKEN"`
	AllowedOrigins       string `envconfig:"ALLOWED_ORIGINS" default:"*"`
	IframeEmbeddingAllow string `envconfig:"IFRAME_EMBEDDING_ALLOW" default:"SAMEORIGIN"`

	// Update settings
	UpdateChannel           string        `envconfig:"UPDATE_CHANNEL" default:"stable"`
	AutoUpdateEnabled       bool          `envconfig:"AUTO_UPDATE_ENABLED" default:"false"`
	AutoUpdateCheckInterval time.Duration `envconfig:"AUTO_UPDATE_CHECK_INTERVAL" default:"24h"`
	AutoUpdateTime          string        `envconfig:"AUTO_UPDATE_TIME" default:"03:00"`
	
	// Discovery settings
	DiscoverySubnet string `envconfig:"DISCOVERY_SUBNET" default:"auto"`
	
	// Deprecated - for backward compatibility
	Port  int  `envconfig:"PORT"` // Maps to BackendPort
	Debug bool `envconfig:"DEBUG" default:"false"`
}

// PVEInstance represents a Proxmox VE connection
type PVEInstance struct {
	Name              string
	Host              string   // Primary endpoint (user-provided)
	User              string
	Password          string
	TokenName         string
	TokenValue        string
	Fingerprint       string
	VerifySSL         bool
	MonitorVMs        bool
	MonitorContainers bool
	MonitorStorage    bool
	MonitorBackups    bool
	
	// Cluster support
	IsCluster       bool              // True if this is a cluster
	ClusterName     string            // Cluster name if applicable
	ClusterEndpoints []ClusterEndpoint // All discovered cluster nodes
}

// ClusterEndpoint represents a single node in a cluster
type ClusterEndpoint struct {
	NodeID   string    // Node ID in cluster
	NodeName string    // Node name
	Host     string    // Full URL (e.g., https://node1.lan:8006)
	IP       string    // IP address
	Online   bool      // Current online status
	LastSeen time.Time // Last successful connection
}

// PBSInstance represents a Proxmox Backup Server connection
type PBSInstance struct {
	Name               string
	Host               string
	User               string
	Password           string
	TokenName          string
	TokenValue         string
	Fingerprint        string
	VerifySSL          bool
	MonitorBackups     bool
	MonitorDatastores  bool
	MonitorSyncJobs    bool
	MonitorVerifyJobs  bool
	MonitorPruneJobs   bool
	MonitorGarbageJobs bool
}

// Global persistence instance for saving
var globalPersistence *ConfigPersistence

// Load reads configuration from encrypted persistence files
func Load() (*Config, error) {
	// Get data directory from environment
	dataDir := "/etc/pulse"
	if dir := os.Getenv("PULSE_DATA_DIR"); dir != "" {
		dataDir = dir
	}
	
	// Load .env file if it exists (for deployment overrides)
	envFile := filepath.Join(dataDir, ".env")
	if _, err := os.Stat(envFile); err == nil {
		if err := godotenv.Load(envFile); err != nil {
			log.Warn().Err(err).Str("file", envFile).Msg("Failed to load .env file")
		} else {
			log.Info().Str("file", envFile).Msg("Loaded .env file for deployment overrides")
		}
	}
	
	// Also try loading from current directory for development
	if err := godotenv.Load(); err == nil {
		log.Info().Msg("Loaded configuration from .env in current directory")
	}
	
	// Initialize config with defaults
	cfg := &Config{
		BackendHost:          "0.0.0.0",
		BackendPort:          3000,
		FrontendHost:         "0.0.0.0", 
		FrontendPort:         7655,
		ConfigPath:           dataDir,
		DataPath:             dataDir,
		ConcurrentPolling:    true,
		ConnectionTimeout:    10 * time.Second,
		MetricsRetentionDays: 7,
		BackupPollingCycles:  10,
		WebhookBatchDelay:    10 * time.Second,
		LogLevel:             "info",
		LogMaxSize:           100,
		LogMaxAge:            30,
		LogCompress:          true,
		AllowedOrigins:       "*",
		IframeEmbeddingAllow: "SAMEORIGIN",
		PollingInterval:      3 * time.Second,
		DiscoverySubnet:      "auto",
	}
	
	// Initialize persistence
	persistence := NewConfigPersistence(dataDir)
	hasSystemConfig := false
	if persistence != nil {
		// Store global persistence for saving
		globalPersistence = persistence
		// Load nodes configuration
		if nodesConfig, err := persistence.LoadNodesConfig(); err == nil && nodesConfig != nil {
			cfg.PVEInstances = nodesConfig.PVEInstances
			cfg.PBSInstances = nodesConfig.PBSInstances
			log.Info().
				Int("pve", len(cfg.PVEInstances)).
				Int("pbs", len(cfg.PBSInstances)).
				Msg("Loaded nodes configuration")
		} else if err != nil {
			log.Warn().Err(err).Msg("Failed to load nodes configuration")
		}
		
		// Load system configuration
		if systemSettings, err := persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
			hasSystemConfig = true
			if systemSettings.PollingInterval > 0 {
				cfg.PollingInterval = time.Duration(systemSettings.PollingInterval) * time.Second
			}
			if systemSettings.UpdateChannel != "" {
				cfg.UpdateChannel = systemSettings.UpdateChannel
			}
			cfg.AutoUpdateEnabled = systemSettings.AutoUpdateEnabled
			if systemSettings.AutoUpdateCheckInterval > 0 {
				cfg.AutoUpdateCheckInterval = time.Duration(systemSettings.AutoUpdateCheckInterval) * time.Hour
			}
			if systemSettings.AutoUpdateTime != "" {
				cfg.AutoUpdateTime = systemSettings.AutoUpdateTime
			}
			if systemSettings.AllowedOrigins != "" {
				cfg.AllowedOrigins = systemSettings.AllowedOrigins
			}
			if systemSettings.ConnectionTimeout > 0 {
				cfg.ConnectionTimeout = time.Duration(systemSettings.ConnectionTimeout) * time.Second
			}
			if systemSettings.APIToken != "" {
				cfg.APIToken = systemSettings.APIToken
			}
			log.Info().
				Dur("interval", cfg.PollingInterval).
				Str("updateChannel", cfg.UpdateChannel).
				Bool("hasAPIToken", cfg.APIToken != "").
				Msg("Loaded system configuration")
		}
	}
	
	// Limited environment variable support
	// NOTE: Node configuration is NOT done via env vars - use the web UI instead
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.FrontendPort = p  // Fixed: PORT should set FrontendPort (the actual listening port)
			log.Info().Int("port", p).Msg("Overriding frontend port from PORT env var")
		}
	}
	if apiToken := os.Getenv("API_TOKEN"); apiToken != "" {
		cfg.APIToken = apiToken
		log.Info().Msg("Overriding API token from env var")
	}
	if updateChannel := os.Getenv("UPDATE_CHANNEL"); updateChannel != "" {
		cfg.UpdateChannel = updateChannel
		log.Info().Str("channel", updateChannel).Msg("Overriding update channel from env var")
	} else if updateChannel := os.Getenv("PULSE_UPDATE_CHANNEL"); updateChannel != "" {
		cfg.UpdateChannel = updateChannel
		log.Info().Str("channel", updateChannel).Msg("Overriding update channel from PULSE_ env var")
	}
	
	// Auto-update settings from env vars
	if autoUpdateEnabled := os.Getenv("AUTO_UPDATE_ENABLED"); autoUpdateEnabled != "" {
		cfg.AutoUpdateEnabled = autoUpdateEnabled == "true" || autoUpdateEnabled == "1"
		log.Info().Bool("enabled", cfg.AutoUpdateEnabled).Msg("Overriding auto-update enabled from env var")
	}
	if interval := os.Getenv("AUTO_UPDATE_CHECK_INTERVAL"); interval != "" {
		if i, err := strconv.Atoi(interval); err == nil && i > 0 {
			cfg.AutoUpdateCheckInterval = time.Duration(i) * time.Hour
			log.Info().Int("hours", i).Msg("Overriding auto-update check interval from env var")
		}
	}
	if updateTime := os.Getenv("AUTO_UPDATE_TIME"); updateTime != "" {
		cfg.AutoUpdateTime = updateTime
		log.Info().Str("time", updateTime).Msg("Overriding auto-update time from env var")
	}
	
	// Other settings from env vars - only use if not already set from system.json
	if pollingInterval := os.Getenv("POLLING_INTERVAL"); pollingInterval != "" {
		// Only use env var if system.json doesn't exist (for backwards compatibility)
		if !hasSystemConfig {
			if i, err := strconv.Atoi(pollingInterval); err == nil && i > 0 {
				cfg.PollingInterval = time.Duration(i) * time.Second
				log.Info().Int("seconds", i).Msg("Using polling interval from env var (no system.json exists)")
			}
		} else {
			log.Debug().Str("env_value", pollingInterval).Msg("Ignoring POLLING_INTERVAL env var - using system.json value")
		}
	}
	if connectionTimeout := os.Getenv("CONNECTION_TIMEOUT"); connectionTimeout != "" {
		if i, err := strconv.Atoi(connectionTimeout); err == nil && i > 0 {
			cfg.ConnectionTimeout = time.Duration(i) * time.Second
			log.Info().Int("seconds", i).Msg("Overriding connection timeout from env var")
		}
	}
	if allowedOrigins := os.Getenv("ALLOWED_ORIGINS"); allowedOrigins != "" {
		cfg.AllowedOrigins = allowedOrigins
		log.Info().Str("origins", allowedOrigins).Msg("Overriding allowed origins from env var")
	}
	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		cfg.LogLevel = logLevel
		log.Info().Str("level", logLevel).Msg("Overriding log level from env var")
	}
	
	// Discovery settings from env vars
	if discoverySubnet := os.Getenv("DISCOVERY_SUBNET"); discoverySubnet != "" {
		cfg.DiscoverySubnet = discoverySubnet
		log.Info().Str("subnet", discoverySubnet).Msg("Overriding discovery subnet from env var")
	}
	
	// Set log level
	switch cfg.LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel) // Default to info level
	}
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return cfg, nil
}

// SaveConfig saves the configuration back to encrypted files
func SaveConfig(cfg *Config) error {
	if globalPersistence == nil {
		return fmt.Errorf("config persistence not initialized")
	}
	
	// Save nodes configuration
	if err := globalPersistence.SaveNodesConfig(cfg.PVEInstances, cfg.PBSInstances); err != nil {
		return fmt.Errorf("failed to save nodes config: %w", err)
	}
	
	// Save system configuration
	systemSettings := SystemSettings{
		PollingInterval:         int(cfg.PollingInterval.Seconds()),
		UpdateChannel:           cfg.UpdateChannel,
		AutoUpdateEnabled:       cfg.AutoUpdateEnabled,
		AutoUpdateCheckInterval: int(cfg.AutoUpdateCheckInterval.Hours()),
		AutoUpdateTime:          cfg.AutoUpdateTime,
		AllowedOrigins:          cfg.AllowedOrigins,
		ConnectionTimeout:       int(cfg.ConnectionTimeout.Seconds()),
		APIToken:                cfg.APIToken,
	}
	if err := globalPersistence.SaveSystemSettings(systemSettings); err != nil {
		return fmt.Errorf("failed to save system config: %w", err)
	}
	
	return nil
}

// UpdatePollingInterval updates just the polling interval
func UpdatePollingInterval(interval int) error {
	if globalPersistence == nil {
		return fmt.Errorf("config persistence not initialized")
	}
	
	systemSettings := SystemSettings{
		PollingInterval: interval,
	}
	return globalPersistence.SaveSystemSettings(systemSettings)
}


// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server settings
	if c.BackendPort <= 0 || c.BackendPort > 65535 {
		return fmt.Errorf("invalid backend port: %d", c.BackendPort)
	}
	if c.FrontendPort <= 0 || c.FrontendPort > 65535 {
		return fmt.Errorf("invalid frontend port: %d", c.FrontendPort)
	}
	
	// Validate monitoring settings
	if c.PollingInterval < time.Second {
		return fmt.Errorf("polling interval must be at least 1 second")
	}
	if c.ConnectionTimeout < time.Second {
		return fmt.Errorf("connection timeout must be at least 1 second")
	}
	
	// Validate PVE instances
	for i, pve := range c.PVEInstances {
		if pve.Host == "" {
			return fmt.Errorf("PVE instance %d: host is required", i+1)
		}
		if !strings.HasPrefix(pve.Host, "http://") && !strings.HasPrefix(pve.Host, "https://") {
			return fmt.Errorf("PVE instance %d: host must start with http:// or https://", i+1)
		}
		// Must have either password or token
		if pve.Password == "" && (pve.TokenName == "" || pve.TokenValue == "") {
			return fmt.Errorf("PVE instance %d: either password or token authentication is required", i+1)
		}
	}

	// Validate and auto-fix PBS instances
	validPBS := []PBSInstance{}
	for i, pbs := range c.PBSInstances {
		if pbs.Host == "" {
			log.Warn().Int("instance", i+1).Msg("PBS instance missing host, skipping")
			continue
		}
		// Auto-fix missing protocol
		if !strings.HasPrefix(pbs.Host, "http://") && !strings.HasPrefix(pbs.Host, "https://") {
			pbs.Host = "https://" + pbs.Host
			log.Info().Str("host", pbs.Host).Msg("PBS host auto-corrected to include https://")
		}
		// Check authentication
		if pbs.Password == "" && (pbs.TokenName == "" || pbs.TokenValue == "") {
			log.Warn().Int("instance", i+1).Str("host", pbs.Host).Msg("PBS instance missing authentication, skipping")
			continue
		}
		validPBS = append(validPBS, pbs)
	}
	c.PBSInstances = validPBS

	return nil
}