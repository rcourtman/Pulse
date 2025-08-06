package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
	}
	
	// Initialize persistence
	persistence := NewConfigPersistence(dataDir)
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
		}
		
		// Load system configuration
		if systemSettings, err := persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
			if systemSettings.PollingInterval > 0 {
				cfg.PollingInterval = time.Duration(systemSettings.PollingInterval) * time.Second
			}
			log.Info().Dur("interval", cfg.PollingInterval).Msg("Loaded system configuration")
		}
	}
	
	// Limited environment variable support
	// NOTE: Node configuration is NOT done via env vars - use the web UI instead
	if port := os.Getenv("PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			cfg.BackendPort = p
			log.Info().Int("port", p).Msg("Overriding backend port from PORT env var")
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
	
	// Set log level
	switch cfg.LogLevel {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
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
		PollingInterval: int(cfg.PollingInterval.Seconds()),
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