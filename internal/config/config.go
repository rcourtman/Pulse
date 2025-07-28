package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

// Config holds all application configuration
type Config struct {
	// Server settings
	Port       int    `envconfig:"PORT" default:"3000"`
	Debug      bool   `envconfig:"DEBUG" default:"false"`
	ConfigPath string `envconfig:"CONFIG_PATH" default:"/config"`
	DataPath   string `envconfig:"DATA_PATH" default:"/data"`

	// Proxmox VE connections
	PVEInstances []PVEInstance

	// Proxmox Backup Server connections
	PBSInstances []PBSInstance

	// Monitoring settings
	PollingInterval      time.Duration `default:"5s"` // Controlled by system.json, not env
	ConcurrentPolling    bool          `envconfig:"CONCURRENT_POLLING" default:"true"`
	ConnectionTimeout    time.Duration `envconfig:"CONNECTION_TIMEOUT" default:"10s"`
	MetricsRetentionDays int           `envconfig:"METRICS_RETENTION_DAYS" default:"7"`

	WebhookBatchDelay time.Duration `envconfig:"WEBHOOK_BATCH_DELAY" default:"10s"`

	// Security settings
	APIToken             string `envconfig:"API_TOKEN"`
	AllowedOrigins       string `envconfig:"ALLOWED_ORIGINS" default:"*"`
	IframeEmbeddingAllow string `envconfig:"IFRAME_EMBEDDING_ALLOW" default:"SAMEORIGIN"`

	// Update settings
	AutoUpdateTime    string `envconfig:"AUTO_UPDATE_TIME" default:"03:00"`
}

// PVEInstance represents a Proxmox VE connection
type PVEInstance struct {
	Name              string
	Host              string
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

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file
	envPath := os.Getenv("ENV_FILE")
	if envPath == "" {
		envPath = ".env"
	}

	// Check if we're running in a container
	if _, err := os.Stat("/config/.env"); err == nil {
		envPath = "/config/.env"
	}

	// Load .env file if it exists
	if _, err := os.Stat(envPath); err == nil {
		if err := godotenv.Load(envPath); err != nil {
			return nil, fmt.Errorf("failed to load .env file: %w", err)
		}
	}

	// Parse base configuration
	var cfg Config
	// Set default polling interval (will be overridden by system.json if present)
	cfg.PollingInterval = 5 * time.Second
	
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to process env config: %w", err)
	}

	// Try to load nodes from persisted config first
	persistence := NewConfigPersistence(cfg.ConfigPath)
	if nodesConfig, err := persistence.LoadNodesConfig(); err == nil && nodesConfig != nil {
		cfg.PVEInstances = nodesConfig.PVEInstances
		cfg.PBSInstances = nodesConfig.PBSInstances
	} else {
		// Fall back to loading from environment variables
		cfg.PVEInstances = loadPVEInstances()
		cfg.PBSInstances = loadPBSInstances()
	}

	// Load system settings
	if systemSettings, err := persistence.LoadSystemSettings(); err == nil && systemSettings != nil {
		// Apply system settings to config
		if systemSettings.PollingInterval > 0 {
			cfg.PollingInterval = time.Duration(systemSettings.PollingInterval) * time.Second
			log.Info().Dur("interval", cfg.PollingInterval).Msg("Using polling interval from system settings")
		}
	} else {
		log.Info().Dur("interval", cfg.PollingInterval).Msg("Using default polling interval")
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
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