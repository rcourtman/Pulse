package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// UnifiedConfig represents the complete configuration in a single structure
type UnifiedConfig struct {
	// Server configuration
	Server ServerConfig `yaml:"server" json:"server"`
	
	// Monitoring configuration
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`
	
	// Logging configuration
	Logging LoggingConfig `yaml:"logging" json:"logging"`
	
	// Security configuration
	Security SecurityConfig `yaml:"security" json:"security"`
	
	// Nodes configuration
	Nodes UnifiedNodesConfig `yaml:"nodes" json:"nodes"`
	
	// Alerts configuration
	Alerts alerts.AlertConfig `yaml:"alerts" json:"alerts"`
	
	// Notifications configuration
	Notifications NotificationsConfig `yaml:"notifications" json:"notifications"`
	
	// Auto-update configuration
	AutoUpdate AutoUpdateConfig `yaml:"auto_update" json:"auto_update"`
}

// ServerConfig holds server settings
type ServerConfig struct {
	Backend  ServerEndpoint `yaml:"backend" json:"backend"`
	Frontend ServerEndpoint `yaml:"frontend" json:"frontend"`
}

// ServerEndpoint represents a server endpoint configuration
type ServerEndpoint struct {
	Host string `yaml:"host" json:"host"`
	Port int    `yaml:"port" json:"port"`
}

// MonitoringConfig holds monitoring settings
type MonitoringConfig struct {
	PollingInterval      string `yaml:"polling_interval" json:"polling_interval"` // e.g., "3s", "1m"
	ConcurrentPolling    bool   `yaml:"concurrent_polling" json:"concurrent_polling"`
	ConnectionTimeout    string `yaml:"connection_timeout" json:"connection_timeout"` // e.g., "10s"
	BackupPollingCycles  int    `yaml:"backup_polling_cycles" json:"backup_polling_cycles"`
	MetricsRetentionDays int    `yaml:"metrics_retention_days" json:"metrics_retention_days"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level      string `yaml:"level" json:"level"`           // debug, info, warn, error
	File       string `yaml:"file" json:"file"`             // log file path
	MaxSize    int    `yaml:"max_size" json:"max_size"`     // MB
	MaxAge     int    `yaml:"max_age" json:"max_age"`       // days
	MaxBackups int    `yaml:"max_backups" json:"max_backups"`
	Compress   bool   `yaml:"compress" json:"compress"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	APIToken             string   `yaml:"api_token" json:"api_token"`
	AllowedOrigins       []string `yaml:"allowed_origins" json:"allowed_origins"`
	IframeEmbeddingAllow string   `yaml:"iframe_embedding_allow" json:"iframe_embedding_allow"`
}

// UnifiedNodesConfig holds all node configurations
type UnifiedNodesConfig struct {
	PVE []PVENode `yaml:"pve" json:"pve"`
	PBS []PBSNode `yaml:"pbs" json:"pbs"`
}

// PVENode represents a Proxmox VE node configuration
type PVENode struct {
	Name              string `yaml:"name" json:"name"`
	Host              string `yaml:"host" json:"host"`
	User              string `yaml:"user,omitempty" json:"user,omitempty"`
	Password          string `yaml:"password,omitempty" json:"password,omitempty"`
	TokenName         string `yaml:"token_name,omitempty" json:"token_name,omitempty"`
	TokenValue        string `yaml:"token_value,omitempty" json:"token_value,omitempty"`
	Fingerprint       string `yaml:"fingerprint,omitempty" json:"fingerprint,omitempty"`
	VerifySSL         bool   `yaml:"verify_ssl" json:"verify_ssl"`
	MonitorVMs        bool   `yaml:"monitor_vms" json:"monitor_vms"`
	MonitorContainers bool   `yaml:"monitor_containers" json:"monitor_containers"`
	MonitorStorage    bool   `yaml:"monitor_storage" json:"monitor_storage"`
	MonitorBackups    bool   `yaml:"monitor_backups" json:"monitor_backups"`
	
	// Cluster configuration
	IsCluster        bool                    `yaml:"is_cluster,omitempty" json:"is_cluster,omitempty"`
	ClusterName      string                  `yaml:"cluster_name,omitempty" json:"cluster_name,omitempty"`
	ClusterEndpoints []ClusterEndpointConfig `yaml:"cluster_endpoints,omitempty" json:"cluster_endpoints,omitempty"`
}

// ClusterEndpointConfig represents a cluster endpoint in the config file
type ClusterEndpointConfig struct {
	NodeID   string `yaml:"node_id" json:"node_id"`
	NodeName string `yaml:"node_name" json:"node_name"`
	Host     string `yaml:"host" json:"host"`
	IP       string `yaml:"ip,omitempty" json:"ip,omitempty"`
}

// PBSNode represents a Proxmox Backup Server node configuration
type PBSNode struct {
	Name              string `yaml:"name" json:"name"`
	Host              string `yaml:"host" json:"host"`
	User              string `yaml:"user,omitempty" json:"user,omitempty"`
	Password          string `yaml:"password,omitempty" json:"password,omitempty"`
	TokenName         string `yaml:"token_name,omitempty" json:"token_name,omitempty"`
	TokenValue        string `yaml:"token_value,omitempty" json:"token_value,omitempty"`
	Fingerprint       string `yaml:"fingerprint,omitempty" json:"fingerprint,omitempty"`
	VerifySSL         bool   `yaml:"verify_ssl" json:"verify_ssl"`
	MonitorBackups    bool   `yaml:"monitor_backups" json:"monitor_backups"`
	MonitorDatastores bool   `yaml:"monitor_datastores" json:"monitor_datastores"`
	MonitorSyncJobs   bool   `yaml:"monitor_sync_jobs" json:"monitor_sync_jobs"`
	MonitorVerifyJobs bool   `yaml:"monitor_verify_jobs" json:"monitor_verify_jobs"`
	MonitorPruneJobs  bool   `yaml:"monitor_prune_jobs" json:"monitor_prune_jobs"`
	MonitorGarbageJobs bool  `yaml:"monitor_garbage_jobs" json:"monitor_garbage_jobs"`
}

// NotificationsConfig holds notification settings
type NotificationsConfig struct {
	Email    notifications.EmailConfig      `yaml:"email" json:"email"`
	Webhooks []notifications.WebhookConfig  `yaml:"webhooks" json:"webhooks"`
}

// AutoUpdateConfig holds auto-update settings
type AutoUpdateConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Channel string `yaml:"channel" json:"channel"`
	Time    string `yaml:"time" json:"time"` // e.g., "03:00"
}

// ConfigManager manages the unified configuration
type ConfigManager struct {
	mu       sync.RWMutex
	config   *UnifiedConfig
	filePath string
	watcher  *fsnotify.Watcher
	onChange func(*UnifiedConfig)
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configPath string) (*ConfigManager, error) {
	if configPath == "" {
		configPath = "/etc/pulse/pulse.yml"
	}
	
	cm := &ConfigManager{
		filePath: configPath,
	}
	
	// Load initial configuration
	if err := cm.Load(); err != nil {
		return nil, err
	}
	
	return cm, nil
}

// Load reads the configuration from file
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	log.Info().Str("path", cm.filePath).Msg("Loading unified configuration")
	
	// Check if config file exists
	if _, err := os.Stat(cm.filePath); os.IsNotExist(err) {
		// Create default configuration
		log.Info().Msg("Config file not found, creating default configuration")
		cm.config = cm.defaultConfig()
		return cm.save()
	}
	
	// Read config file
	data, err := os.ReadFile(cm.filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	
	// Parse based on file extension
	var config UnifiedConfig
	ext := filepath.Ext(cm.filePath)
	switch ext {
	case ".yml", ".yaml":
		if err := yaml.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}
	
	// Apply defaults to any missing values
	cm.applyDefaults(&config)
	
	// Resolve credentials from environment variables or files
	credResolver := NewCredentialResolver()
	
	// Resolve PVE node credentials
	for i := range config.Nodes.PVE {
		if err := credResolver.ResolveNodeCredentials(&config.Nodes.PVE[i], fmt.Sprintf("pve.%s", config.Nodes.PVE[i].Name)); err != nil {
			return fmt.Errorf("failed to resolve credentials for PVE node %s: %w", config.Nodes.PVE[i].Name, err)
		}
	}
	
	// Resolve PBS node credentials
	for i := range config.Nodes.PBS {
		if err := credResolver.ResolveNodeCredentials(&config.Nodes.PBS[i], fmt.Sprintf("pbs.%s", config.Nodes.PBS[i].Name)); err != nil {
			return fmt.Errorf("failed to resolve credentials for PBS node %s: %w", config.Nodes.PBS[i].Name, err)
		}
	}
	
	// Check config file security (now just logs at debug level)
	credResolver.CheckConfigSecurity(cm.filePath)
	
	cm.config = &config
	
	// Automatically secure the config file permissions
	cm.mu.Unlock() // Temporarily unlock to call EnsureSecurePermissions
	cm.EnsureSecurePermissions()
	cm.mu.Lock()
	
	log.Info().
		Str("backend", fmt.Sprintf("%s:%d", config.Server.Backend.Host, config.Server.Backend.Port)).
		Str("polling", config.Monitoring.PollingInterval).
		Int("pve_nodes", len(config.Nodes.PVE)).
		Int("pbs_nodes", len(config.Nodes.PBS)).
		Msg("Configuration loaded successfully")
	
	return nil
}

// Save writes the current configuration to file
func (cm *ConfigManager) Save() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.save()
}

// EnsureSecurePermissions sets secure file permissions on the config file
func (cm *ConfigManager) EnsureSecurePermissions() error {
	// Set 0600 permissions (read/write for owner only)
	if err := os.Chmod(cm.filePath, 0600); err != nil {
		// Don't fail if we can't change permissions, just warn
		log.Warn().
			Err(err).
			Str("file", cm.filePath).
			Msg("Could not set secure permissions on config file")
		return nil
	}
	
	log.Info().
		Str("file", cm.filePath).
		Msg("Config file permissions secured (mode 0600)")
	return nil
}

// save is the internal save method (must be called with lock held)
func (cm *ConfigManager) save() error {
	// Ensure directory exists
	dir := filepath.Dir(cm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	// Marshal based on file extension
	var data []byte
	var err error
	ext := filepath.Ext(cm.filePath)
	switch ext {
	case ".yml", ".yaml":
		data, err = yaml.Marshal(cm.config)
	case ".json":
		data, err = json.MarshalIndent(cm.config, "", "  ")
	default:
		return fmt.Errorf("unsupported config file format: %s", ext)
	}
	
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	
	// Write to file with secure permissions
	if err := os.WriteFile(cm.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	
	log.Info().Str("path", cm.filePath).Msg("Configuration saved")
	return nil
}

// Get returns a copy of the current configuration
func (cm *ConfigManager) Get() UnifiedConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return *cm.config
}

// Update updates the configuration
func (cm *ConfigManager) Update(updateFunc func(*UnifiedConfig)) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// Apply the update
	updateFunc(cm.config)
	
	// Save to file
	if err := cm.save(); err != nil {
		return err
	}
	
	// Notify watchers
	if cm.onChange != nil {
		go cm.onChange(cm.config)
	}
	
	return nil
}

// Watch starts watching the config file for changes
func (cm *ConfigManager) Watch(onChange func(*UnifiedConfig)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	
	cm.watcher = watcher
	cm.onChange = onChange
	
	// Watch the config file
	if err := watcher.Add(cm.filePath); err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}
	
	// Start watching in background
	go cm.watchLoop()
	
	return nil
}

// watchLoop handles file system events
func (cm *ConfigManager) watchLoop() {
	for {
		select {
		case event, ok := <-cm.watcher.Events:
			if !ok {
				return
			}
			
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Info().Str("file", event.Name).Msg("Config file changed, reloading")
				if err := cm.Load(); err != nil {
					log.Error().Err(err).Msg("Failed to reload config")
				} else if cm.onChange != nil {
					cm.onChange(cm.config)
				}
			}
			
		case err, ok := <-cm.watcher.Errors:
			if !ok {
				return
			}
			log.Error().Err(err).Msg("Config watcher error")
		}
	}
}

// Stop stops watching the config file
func (cm *ConfigManager) Stop() {
	if cm.watcher != nil {
		cm.watcher.Close()
	}
}

// defaultConfig returns the default configuration
func (cm *ConfigManager) defaultConfig() *UnifiedConfig {
	return &UnifiedConfig{
		Server: ServerConfig{
			Backend: ServerEndpoint{
				Host: "0.0.0.0",
				Port: 3000,
			},
			Frontend: ServerEndpoint{
				Host: "0.0.0.0",
				Port: 7655,
			},
		},
		Monitoring: MonitoringConfig{
			PollingInterval:      "5s",
			ConcurrentPolling:    true,
			ConnectionTimeout:    "10s",
			BackupPollingCycles:  10,
			MetricsRetentionDays: 7,
		},
		Logging: LoggingConfig{
			Level:      "info",
			File:       "",
			MaxSize:    100,
			MaxAge:     30,
			MaxBackups: 3,
			Compress:   true,
		},
		Security: SecurityConfig{
			APIToken:             "",
			AllowedOrigins:       []string{"*"},
			IframeEmbeddingAllow: "SAMEORIGIN",
		},
		Nodes: UnifiedNodesConfig{
			PVE: []PVENode{},
			PBS: []PBSNode{},
		},
		Alerts: alerts.AlertConfig{
			Enabled: true,
			GuestDefaults: alerts.ThresholdConfig{
				CPU:        &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:     &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:       &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			},
			NodeDefaults: alerts.ThresholdConfig{
				CPU:        &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
				Memory:     &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
				Disk:       &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			},
			StorageDefault: alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
			Schedule: alerts.ScheduleConfig{
				Cooldown: 5 * 60, // 5 minutes
				Grouping: alerts.GroupingConfig{
					Enabled: true,
					Window:  30,
					ByNode:  true,
					ByGuest: true,
				},
				Escalation: alerts.EscalationConfig{
					Enabled: false,
				},
			},
			MinimumDelta:      1.0,
			SuppressionWindow: 5,
			HysteresisMargin:  5.0,
		},
		Notifications: NotificationsConfig{
			Email: notifications.EmailConfig{
				Enabled: false,
			},
			Webhooks: []notifications.WebhookConfig{},
		},
		AutoUpdate: AutoUpdateConfig{
			Enabled: false,
			Channel: "stable",
			Time:    "03:00",
		},
	}
}

// applyDefaults fills in any missing values with defaults
func (cm *ConfigManager) applyDefaults(config *UnifiedConfig) {
	defaults := cm.defaultConfig()
	
	// Server defaults
	if config.Server.Backend.Host == "" {
		config.Server.Backend.Host = defaults.Server.Backend.Host
	}
	if config.Server.Backend.Port == 0 {
		config.Server.Backend.Port = defaults.Server.Backend.Port
	}
	if config.Server.Frontend.Host == "" {
		config.Server.Frontend.Host = defaults.Server.Frontend.Host
	}
	if config.Server.Frontend.Port == 0 {
		config.Server.Frontend.Port = defaults.Server.Frontend.Port
	}
	
	// Monitoring defaults
	if config.Monitoring.PollingInterval == "" {
		config.Monitoring.PollingInterval = defaults.Monitoring.PollingInterval
	}
	if config.Monitoring.ConnectionTimeout == "" {
		config.Monitoring.ConnectionTimeout = defaults.Monitoring.ConnectionTimeout
	}
	if config.Monitoring.BackupPollingCycles == 0 {
		config.Monitoring.BackupPollingCycles = defaults.Monitoring.BackupPollingCycles
	}
	if config.Monitoring.MetricsRetentionDays == 0 {
		config.Monitoring.MetricsRetentionDays = defaults.Monitoring.MetricsRetentionDays
	}
	
	// Logging defaults
	if config.Logging.Level == "" {
		config.Logging.Level = defaults.Logging.Level
	}
	if config.Logging.MaxSize == 0 {
		config.Logging.MaxSize = defaults.Logging.MaxSize
	}
	if config.Logging.MaxAge == 0 {
		config.Logging.MaxAge = defaults.Logging.MaxAge
	}
	if config.Logging.MaxBackups == 0 {
		config.Logging.MaxBackups = defaults.Logging.MaxBackups
	}
	
	// Security defaults
	if len(config.Security.AllowedOrigins) == 0 {
		config.Security.AllowedOrigins = defaults.Security.AllowedOrigins
	}
	if config.Security.IframeEmbeddingAllow == "" {
		config.Security.IframeEmbeddingAllow = defaults.Security.IframeEmbeddingAllow
	}
	
	// Alert defaults
	if !config.Alerts.Enabled && config.Alerts.GuestDefaults.CPU == nil {
		config.Alerts = defaults.Alerts
	}
	
	// Auto-update defaults
	if config.AutoUpdate.Channel == "" {
		config.AutoUpdate.Channel = defaults.AutoUpdate.Channel
	}
	if config.AutoUpdate.Time == "" {
		config.AutoUpdate.Time = defaults.AutoUpdate.Time
	}
}

// ToLegacyConfig converts UnifiedConfig to the legacy Config format for compatibility
func (uc *UnifiedConfig) ToLegacyConfig() (*Config, error) {
	// Parse durations
	pollingInterval, err := time.ParseDuration(uc.Monitoring.PollingInterval)
	if err != nil {
		return nil, fmt.Errorf("invalid polling interval: %w", err)
	}
	
	connectionTimeout, err := time.ParseDuration(uc.Monitoring.ConnectionTimeout)
	if err != nil {
		return nil, fmt.Errorf("invalid connection timeout: %w", err)
	}
	
	// Convert nodes
	var pveInstances []PVEInstance
	for _, node := range uc.Nodes.PVE {
		instance := PVEInstance{
			Name:              node.Name,
			Host:              node.Host,
			User:              node.User,
			Password:          node.Password,
			TokenName:         node.TokenName,
			TokenValue:        node.TokenValue,
			Fingerprint:       node.Fingerprint,
			VerifySSL:         node.VerifySSL,
			MonitorVMs:        node.MonitorVMs,
			MonitorContainers: node.MonitorContainers,
			MonitorStorage:    node.MonitorStorage,
			MonitorBackups:    node.MonitorBackups,
			IsCluster:         node.IsCluster,
			ClusterName:       node.ClusterName,
		}
		
		// Convert cluster endpoints
		if len(node.ClusterEndpoints) > 0 {
			instance.ClusterEndpoints = make([]ClusterEndpoint, len(node.ClusterEndpoints))
			for i, ep := range node.ClusterEndpoints {
				instance.ClusterEndpoints[i] = ClusterEndpoint{
					NodeID:   ep.NodeID,
					NodeName: ep.NodeName,
					Host:     ep.Host,
					IP:       ep.IP,
					Online:   true, // Default to true, will be updated by monitoring
					LastSeen: time.Now(),
				}
			}
		}
		
		pveInstances = append(pveInstances, instance)
	}
	
	var pbsInstances []PBSInstance
	for _, node := range uc.Nodes.PBS {
		pbsInstances = append(pbsInstances, PBSInstance{
			Name:              node.Name,
			Host:              node.Host,
			User:              node.User,
			Password:          node.Password,
			TokenName:         node.TokenName,
			TokenValue:        node.TokenValue,
			Fingerprint:       node.Fingerprint,
			VerifySSL:         node.VerifySSL,
			MonitorBackups:    node.MonitorBackups,
			MonitorDatastores: node.MonitorDatastores,
			MonitorSyncJobs:   node.MonitorSyncJobs,
			MonitorVerifyJobs: node.MonitorVerifyJobs,
			MonitorPruneJobs:  node.MonitorPruneJobs,
			MonitorGarbageJobs: node.MonitorGarbageJobs,
		})
	}
	
	// Convert allowed origins to comma-separated string
	allowedOrigins := "*"
	if len(uc.Security.AllowedOrigins) > 0 {
		allowedOrigins = uc.Security.AllowedOrigins[0]
		for i := 1; i < len(uc.Security.AllowedOrigins); i++ {
			allowedOrigins += "," + uc.Security.AllowedOrigins[i]
		}
	}
	
	return &Config{
		BackendHost:          uc.Server.Backend.Host,
		BackendPort:          uc.Server.Backend.Port,
		FrontendHost:         uc.Server.Frontend.Host,
		FrontendPort:         uc.Server.Frontend.Port,
		ConfigPath:           "/etc/pulse",
		DataPath:             "/data",
		PVEInstances:         pveInstances,
		PBSInstances:         pbsInstances,
		PollingInterval:      pollingInterval,
		ConcurrentPolling:    uc.Monitoring.ConcurrentPolling,
		ConnectionTimeout:    connectionTimeout,
		MetricsRetentionDays: uc.Monitoring.MetricsRetentionDays,
		BackupPollingCycles:  uc.Monitoring.BackupPollingCycles,
		LogLevel:             uc.Logging.Level,
		LogFile:              uc.Logging.File,
		LogMaxSize:           uc.Logging.MaxSize,
		LogMaxAge:            uc.Logging.MaxAge,
		LogCompress:          uc.Logging.Compress,
		APIToken:             uc.Security.APIToken,
		AllowedOrigins:       allowedOrigins,
		IframeEmbeddingAllow: uc.Security.IframeEmbeddingAllow,
		AutoUpdateTime:       uc.AutoUpdate.Time,
	}, nil
}