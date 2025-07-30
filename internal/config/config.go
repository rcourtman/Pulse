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
type Config struct {
	// Server settings
	BackendHost   string `envconfig:"BACKEND_HOST" default:"0.0.0.0"`
	BackendPort   int    `envconfig:"BACKEND_PORT" default:"3000"`
	FrontendHost  string `envconfig:"FRONTEND_HOST" default:"0.0.0.0"`
	FrontendPort  int    `envconfig:"FRONTEND_PORT" default:"7655"`
	ConfigPath    string `envconfig:"CONFIG_PATH" default:"/etc/pulse"`
	DataPath      string `envconfig:"DATA_PATH" default:"/data"`

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

// Global config manager instance for saving
var globalConfigManager *ConfigManager

// Load reads configuration from the unified config file
func Load() (*Config, error) {
	// Determine config file path
	configPath := os.Getenv("PULSE_CONFIG")
	if configPath == "" {
		configPath = "/etc/pulse/pulse.yml"
	}
	
	// Create config manager
	manager, err := NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Store global manager for saving
	globalConfigManager = manager
	
	// Get unified config
	unified := manager.Get()
	
	// Convert to legacy format for compatibility
	cfg, err := unified.ToLegacyConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to convert config: %w", err)
	}
	
	// Override with environment variables if present
	// This allows env vars to override config file for deployments
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

// SaveConfig saves the configuration back to the YAML file
func SaveConfig(cfg *Config) error {
	if globalConfigManager == nil {
		return fmt.Errorf("config manager not initialized")
	}
	
	// Update the unified config with the new data
	return globalConfigManager.Update(func(uc *UnifiedConfig) {
		// Convert PVE instances
		uc.Nodes.PVE = make([]PVENode, 0, len(cfg.PVEInstances))
		for _, pve := range cfg.PVEInstances {
			node := PVENode{
				Name:              pve.Name,
				Host:              pve.Host,
				User:              pve.User,
				Password:          pve.Password,
				TokenName:         pve.TokenName,
				TokenValue:        pve.TokenValue,
				Fingerprint:       pve.Fingerprint,
				VerifySSL:         pve.VerifySSL,
				MonitorVMs:        pve.MonitorVMs,
				MonitorContainers: pve.MonitorContainers,
				MonitorStorage:    pve.MonitorStorage,
				MonitorBackups:    pve.MonitorBackups,
				IsCluster:         pve.IsCluster,
				ClusterName:       pve.ClusterName,
			}
			
			// Convert cluster endpoints
			if len(pve.ClusterEndpoints) > 0 {
				node.ClusterEndpoints = make([]ClusterEndpointConfig, 0, len(pve.ClusterEndpoints))
				for _, ep := range pve.ClusterEndpoints {
					node.ClusterEndpoints = append(node.ClusterEndpoints, ClusterEndpointConfig{
						NodeID:   ep.NodeID,
						NodeName: ep.NodeName,
						Host:     ep.Host,
						IP:       ep.IP,
					})
				}
			}
			
			uc.Nodes.PVE = append(uc.Nodes.PVE, node)
		}
		
		// Convert PBS instances
		uc.Nodes.PBS = make([]PBSNode, 0, len(cfg.PBSInstances))
		for _, pbs := range cfg.PBSInstances {
			node := PBSNode{
				Name:               pbs.Name,
				Host:               pbs.Host,
				User:               pbs.User,
				Password:           pbs.Password,
				TokenName:          pbs.TokenName,
				TokenValue:         pbs.TokenValue,
				Fingerprint:        pbs.Fingerprint,
				VerifySSL:          pbs.VerifySSL,
				MonitorBackups:     pbs.MonitorBackups,
				MonitorDatastores:  pbs.MonitorDatastores,
				MonitorSyncJobs:    pbs.MonitorSyncJobs,
				MonitorVerifyJobs:  pbs.MonitorVerifyJobs,
				MonitorPruneJobs:   pbs.MonitorPruneJobs,
				MonitorGarbageJobs: pbs.MonitorGarbageJobs,
			}
			uc.Nodes.PBS = append(uc.Nodes.PBS, node)
		}
	})
}

// loadPVEInstances loads all PVE instances from environment variables
func loadPVEInstances() []PVEInstance {
	var instances []PVEInstance

	// Check for single instance (backward compatibility with JavaScript version)
	if host := os.Getenv("PROXMOX_HOST"); host != "" {
		// Parse token ID format: user@realm!tokenname
		tokenID := os.Getenv("PROXMOX_TOKEN_ID")
		tokenSecret := os.Getenv("PROXMOX_TOKEN_SECRET")
		
		var tokenName, tokenUser string
		if tokenID != "" {
			// Split user@realm!tokenname
			parts := strings.Split(tokenID, "!")
			if len(parts) == 2 {
				tokenUser = parts[0]
				tokenName = parts[1]
			}
		}
		
		instance := PVEInstance{
			Name:              os.Getenv("PROXMOX_NODE_NAME"),
			Host:              host,
			User:              tokenUser,
			Password:          "",
			TokenName:         tokenName,
			TokenValue:        tokenSecret,
			Fingerprint:       os.Getenv("PROXMOX_FINGERPRINT"),
			VerifySSL:         os.Getenv("PROXMOX_ALLOW_SELF_SIGNED_CERTS") != "true",
			MonitorVMs:        os.Getenv("PROXMOX_MONITOR_VMS") != "false",
			MonitorContainers: os.Getenv("PROXMOX_MONITOR_CONTAINERS") != "false",
			MonitorStorage:    os.Getenv("PROXMOX_MONITOR_STORAGE") != "false",
			MonitorBackups:    os.Getenv("PROXMOX_MONITOR_BACKUPS") != "false",
		}
		if instance.Name == "" {
			instance.Name = "Main"
		}
		instances = append(instances, instance)
	}

	// Check for multiple instances (PROXMOX_HOST_2, PROXMOX_HOST_3, etc.)
	for i := 2; i <= 10; i++ {
		suffix := fmt.Sprintf("_%d", i)
		if host := os.Getenv("PROXMOX_HOST" + suffix); host != "" {
			// Parse token ID format: user@realm!tokenname
			tokenID := os.Getenv("PROXMOX_TOKEN_ID" + suffix)
			tokenSecret := os.Getenv("PROXMOX_TOKEN_SECRET" + suffix)
			
			var tokenName, tokenUser string
			if tokenID != "" {
				// Split user@realm!tokenname
				parts := strings.Split(tokenID, "!")
				if len(parts) == 2 {
					tokenUser = parts[0]
					tokenName = parts[1]
				}
			}
			
			instance := PVEInstance{
				Name:              os.Getenv("PROXMOX_NODE_NAME" + suffix),
				Host:              host,
				User:              tokenUser,
				Password:          "",
				TokenName:         tokenName,
				TokenValue:        tokenSecret,
				Fingerprint:       os.Getenv("PROXMOX_FINGERPRINT" + suffix),
				VerifySSL:         os.Getenv("PROXMOX_ALLOW_SELF_SIGNED_CERTS"+suffix) != "true",
				MonitorVMs:        os.Getenv("PROXMOX_MONITOR_VMS"+suffix) != "false",
				MonitorContainers: os.Getenv("PROXMOX_MONITOR_CONTAINERS"+suffix) != "false",
				MonitorStorage:    os.Getenv("PROXMOX_MONITOR_STORAGE"+suffix) != "false",
				MonitorBackups:    os.Getenv("PROXMOX_MONITOR_BACKUPS"+suffix) != "false",
			}
			if instance.Name == "" {
				instance.Name = fmt.Sprintf("PVE-%d", i)
			}
			instances = append(instances, instance)
		}
	}

	return instances
}

// loadPBSInstances loads all PBS instances from environment variables
func loadPBSInstances() []PBSInstance {
	var instances []PBSInstance

	// Check for single instance (backward compatibility)
	if host := os.Getenv("PBS_HOST"); host != "" {
		// Parse token ID format: user@realm!tokenname
		tokenID := os.Getenv("PBS_TOKEN_ID")
		tokenSecret := os.Getenv("PBS_TOKEN_SECRET")
		
		var tokenName, tokenUser string
		if tokenID != "" {
			// Split user@realm!tokenname
			parts := strings.Split(tokenID, "!")
			if len(parts) == 2 {
				tokenUser = parts[0]  // e.g., "admin@pbs"
				tokenName = parts[1]  // e.g., "pulse-readonly"
			}
		}
		
		instance := PBSInstance{
			Name:               "Main",
			Host:               host,
			User:               tokenUser,  // User@realm part
			Password:           "",
			TokenName:          tokenName,  // Just the token name
			TokenValue:         tokenSecret,
			Fingerprint:        os.Getenv("PBS_FINGERPRINT"),
			VerifySSL:          os.Getenv("PBS_ALLOW_SELF_SIGNED_CERTS") != "true",
			MonitorBackups:     os.Getenv("PBS_MONITOR_BACKUPS") != "false",
			MonitorDatastores:  os.Getenv("PBS_MONITOR_DATASTORES") != "false",
			MonitorSyncJobs:    os.Getenv("PBS_MONITOR_SYNC_JOBS") != "false",
			MonitorVerifyJobs:  os.Getenv("PBS_MONITOR_VERIFY_JOBS") != "false",
			MonitorPruneJobs:   os.Getenv("PBS_MONITOR_PRUNE_JOBS") != "false",
			MonitorGarbageJobs: os.Getenv("PBS_MONITOR_GARBAGE_JOBS") != "false",
		}
		instances = append(instances, instance)
	}

	// Check for multiple instances
	for i := 1; i <= 10; i++ {
		suffix := fmt.Sprintf("_%d", i)
		if host := os.Getenv("PBS_HOST" + suffix); host != "" {
			instance := PBSInstance{
				Name:               os.Getenv("PBS_NAME" + suffix),
				Host:               host,
				User:               os.Getenv("PBS_USER" + suffix),
				Password:           os.Getenv("PBS_PASSWORD" + suffix),
				TokenName:          os.Getenv("PBS_TOKEN_NAME" + suffix),
				TokenValue:         os.Getenv("PBS_TOKEN_VALUE" + suffix),
				Fingerprint:        os.Getenv("PBS_FINGERPRINT" + suffix),
				VerifySSL:          os.Getenv("PBS_VERIFY_SSL"+suffix) != "false",
				MonitorBackups:     os.Getenv("PBS_MONITOR_BACKUPS"+suffix) != "false",
				MonitorDatastores:  os.Getenv("PBS_MONITOR_DATASTORES"+suffix) != "false",
				MonitorSyncJobs:    os.Getenv("PBS_MONITOR_SYNC_JOBS"+suffix) != "false",
				MonitorVerifyJobs:  os.Getenv("PBS_MONITOR_VERIFY_JOBS"+suffix) != "false",
				MonitorPruneJobs:   os.Getenv("PBS_MONITOR_PRUNE_JOBS"+suffix) != "false",
				MonitorGarbageJobs: os.Getenv("PBS_MONITOR_GARBAGE_JOBS"+suffix) != "false",
			}
			if instance.Name == "" {
				instance.Name = fmt.Sprintf("PBS-%d", i)
			}
			instances = append(instances, instance)
		}
	}

	return instances
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

	// Validate PBS instances
	for i, pbs := range c.PBSInstances {
		if pbs.Host == "" {
			return fmt.Errorf("PBS instance %d: host is required", i+1)
		}
		if !strings.HasPrefix(pbs.Host, "http://") && !strings.HasPrefix(pbs.Host, "https://") {
			return fmt.Errorf("PBS instance %d: host must start with http:// or https://", i+1)
		}
		// Must have either password or token
		if pbs.Password == "" && (pbs.TokenName == "" || pbs.TokenValue == "") {
			return fmt.Errorf("PBS instance %d: either password or token authentication is required", i+1)
		}
	}

	return nil
}