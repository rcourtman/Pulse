package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog/log"
)

// ConfigPersistence handles saving and loading configuration
type ConfigPersistence struct {
	mu          sync.RWMutex
	configDir   string
	alertFile   string
	emailFile   string
	webhookFile string
	nodesFile   string
	systemFile  string
}

// NewConfigPersistence creates a new config persistence manager
func NewConfigPersistence(configDir string) *ConfigPersistence {
	if configDir == "" {
		configDir = "/etc/pulse"
	}
	
	cp := &ConfigPersistence{
		configDir:   configDir,
		alertFile:   filepath.Join(configDir, "alerts.json"),
		emailFile:   filepath.Join(configDir, "email.json"),
		webhookFile: filepath.Join(configDir, "webhooks.json"),
		nodesFile:   filepath.Join(configDir, "nodes.json"),
		systemFile:  filepath.Join(configDir, "system.json"),
	}
	
	log.Debug().
		Str("configDir", configDir).
		Str("systemFile", cp.systemFile).
		Str("nodesFile", cp.nodesFile).
		Msg("Config persistence initialized")
	
	return cp
}

// EnsureConfigDir ensures the configuration directory exists
func (c *ConfigPersistence) EnsureConfigDir() error {
	return os.MkdirAll(c.configDir, 0755)
}

// SaveAlertConfig saves alert configuration to file
func (c *ConfigPersistence) SaveAlertConfig(config alerts.AlertConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}
	
	if err := os.WriteFile(c.alertFile, data, 0644); err != nil {
		return err
	}
	
	log.Info().Str("file", c.alertFile).Msg("Alert configuration saved")
	return nil
}

// LoadAlertConfig loads alert configuration from file
func (c *ConfigPersistence) LoadAlertConfig() (*alerts.AlertConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.alertFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return &alerts.AlertConfig{
				Enabled: true,
				GuestDefaults: alerts.ThresholdConfig{
					CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
					Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
					Disk:   &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
				},
				NodeDefaults: alerts.ThresholdConfig{
					CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
					Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
					Disk:   &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
				},
				StorageDefault:    alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
				MinimumDelta:      2.0,
				SuppressionWindow: 5,
				HysteresisMargin:  5.0,
				Overrides:         make(map[string]alerts.ThresholdConfig),
			}, nil
		}
		return nil, err
	}
	
	var config alerts.AlertConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	log.Info().Str("file", c.alertFile).Msg("Alert configuration loaded")
	return &config, nil
}

// SaveEmailConfig saves email configuration to file
func (c *ConfigPersistence) SaveEmailConfig(config notifications.EmailConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Don't save password in plain text - in production, use proper secret management
	configToSave := config
	if configToSave.Password != "" {
		configToSave.Password = "***ENCRYPTED***"
	}
	
	data, err := json.MarshalIndent(configToSave, "", "  ")
	if err != nil {
		return err
	}
	
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}
	
	if err := os.WriteFile(c.emailFile, data, 0644); err != nil {
		return err
	}
	
	log.Info().Str("file", c.emailFile).Msg("Email configuration saved")
	return nil
}

// LoadEmailConfig loads email configuration from file
func (c *ConfigPersistence) LoadEmailConfig() (*notifications.EmailConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.emailFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if file doesn't exist
			return &notifications.EmailConfig{
				Enabled:  false,
				SMTPPort: 587,
				TLS:      true,
				To:       []string{},
			}, nil
		}
		return nil, err
	}
	
	var config notifications.EmailConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	// Password needs to be re-entered by user for security
	if config.Password == "***ENCRYPTED***" {
		config.Password = ""
	}
	
	log.Info().Str("file", c.emailFile).Msg("Email configuration loaded")
	return &config, nil
}

// SaveWebhooks saves webhook configurations to file
func (c *ConfigPersistence) SaveWebhooks(webhooks []notifications.WebhookConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	data, err := json.MarshalIndent(webhooks, "", "  ")
	if err != nil {
		return err
	}
	
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}
	
	if err := os.WriteFile(c.webhookFile, data, 0644); err != nil {
		return err
	}
	
	log.Info().Str("file", c.webhookFile).Int("count", len(webhooks)).Msg("Webhooks saved")
	return nil
}

// LoadWebhooks loads webhook configurations from file
func (c *ConfigPersistence) LoadWebhooks() ([]notifications.WebhookConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.webhookFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty list if file doesn't exist
			return []notifications.WebhookConfig{}, nil
		}
		return nil, err
	}
	
	var webhooks []notifications.WebhookConfig
	if err := json.Unmarshal(data, &webhooks); err != nil {
		return nil, err
	}
	
	log.Info().Str("file", c.webhookFile).Int("count", len(webhooks)).Msg("Webhooks loaded")
	return webhooks, nil
}

// NodesConfig represents the saved nodes configuration
type NodesConfig struct {
	PVEInstances []PVEInstance `json:"pveInstances"`
	PBSInstances []PBSInstance `json:"pbsInstances"`
}

// SystemSettings represents system configuration settings
type SystemSettings struct {
	PollingInterval   int    `json:"pollingInterval"`
	BackendPort       int    `json:"backendPort,omitempty"`
	FrontendPort      int    `json:"frontendPort,omitempty"`
	AllowedOrigins    string `json:"allowedOrigins,omitempty"`
	ConnectionTimeout int    `json:"connectionTimeout,omitempty"`
}

// SaveNodesConfig saves nodes configuration to file
func (c *ConfigPersistence) SaveNodesConfig(pveInstances []PVEInstance, pbsInstances []PBSInstance) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	config := NodesConfig{
		PVEInstances: pveInstances,
		PBSInstances: pbsInstances,
	}
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}
	
	if err := os.WriteFile(c.nodesFile, data, 0644); err != nil {
		return err
	}
	
	log.Info().Str("file", c.nodesFile).
		Int("pve", len(pveInstances)).
		Int("pbs", len(pbsInstances)).
		Msg("Nodes configuration saved")
	return nil
}

// LoadNodesConfig loads nodes configuration from file
func (c *ConfigPersistence) LoadNodesConfig() (*NodesConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.nodesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return nil if file doesn't exist - will use env vars
			return nil, nil
		}
		return nil, err
	}
	
	var config NodesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	log.Info().Str("file", c.nodesFile).
		Int("pve", len(config.PVEInstances)).
		Int("pbs", len(config.PBSInstances)).
		Msg("Nodes configuration loaded")
	return &config, nil
}

// SaveSystemSettings saves system settings to file
func (c *ConfigPersistence) SaveSystemSettings(settings SystemSettings) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}
	
	if err := os.WriteFile(c.systemFile, data, 0644); err != nil {
		return err
	}
	
	log.Info().Str("file", c.systemFile).Msg("System settings saved")
	return nil
}

// LoadSystemSettings loads system settings from file
func (c *ConfigPersistence) LoadSystemSettings() (*SystemSettings, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.systemFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default settings if file doesn't exist
			return &SystemSettings{
				PollingInterval: 5,
			}, nil
		}
		return nil, err
	}
	
	var settings SystemSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}
	
	log.Info().Str("file", c.systemFile).Msg("System settings loaded")
	return &settings, nil
}

// Helper function
func ptrFloat64(v float64) *float64 {
	return &v
}