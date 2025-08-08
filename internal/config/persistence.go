package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
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
	crypto      *crypto.CryptoManager
}

// NewConfigPersistence creates a new config persistence manager
func NewConfigPersistence(configDir string) *ConfigPersistence {
	if configDir == "" {
		configDir = "/etc/pulse"
	}
	
	// Initialize crypto manager
	cryptoMgr, err := crypto.NewCryptoManager()
	if err != nil {
		log.Error().Err(err).Msg("Failed to initialize crypto manager, using unencrypted storage")
		cryptoMgr = nil
	}
	
	cp := &ConfigPersistence{
		configDir:   configDir,
		alertFile:   filepath.Join(configDir, "alerts.json"),
		emailFile:   filepath.Join(configDir, "email.enc"),
		webhookFile: filepath.Join(configDir, "webhooks.json"),
		nodesFile:   filepath.Join(configDir, "nodes.enc"),
		systemFile:  filepath.Join(configDir, "system.json"),
		crypto:      cryptoMgr,
	}
	
	log.Debug().
		Str("configDir", configDir).
		Str("systemFile", cp.systemFile).
		Str("nodesFile", cp.nodesFile).
		Bool("encryptionEnabled", cryptoMgr != nil).
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

// SaveEmailConfig saves email configuration to file (encrypted)
func (c *ConfigPersistence) SaveEmailConfig(config notifications.EmailConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Marshal to JSON first
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	if err := c.EnsureConfigDir(); err != nil {
		return err
	}
	
	// Encrypt if crypto manager is available
	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
	}
	
	// Save with restricted permissions (owner read/write only)
	if err := os.WriteFile(c.emailFile, data, 0600); err != nil {
		return err
	}
	
	log.Info().
		Str("file", c.emailFile).
		Bool("encrypted", c.crypto != nil).
		Msg("Email configuration saved")
	return nil
}

// LoadEmailConfig loads email configuration from file (decrypts if encrypted)
func (c *ConfigPersistence) LoadEmailConfig() (*notifications.EmailConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.emailFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if encrypted file doesn't exist
			return &notifications.EmailConfig{
				Enabled:  false,
				SMTPPort: 587,
				TLS:      true,
				To:       []string{},
			}, nil
		}
		return nil, err
	}
	
	// Decrypt if crypto manager is available
	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			return nil, err
		}
		data = decrypted
	}
	
	var config notifications.EmailConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	log.Info().
		Str("file", c.emailFile).
		Bool("encrypted", c.crypto != nil).
		Msg("Email configuration loaded")
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
	PollingInterval         int    `json:"pollingInterval"`
	BackendPort             int    `json:"backendPort,omitempty"`
	FrontendPort            int    `json:"frontendPort,omitempty"`
	AllowedOrigins          string `json:"allowedOrigins,omitempty"`
	ConnectionTimeout       int    `json:"connectionTimeout,omitempty"`
	UpdateChannel           string `json:"updateChannel,omitempty"`
	AutoUpdateEnabled       bool   `json:"autoUpdateEnabled,omitempty"`
	AutoUpdateCheckInterval int    `json:"autoUpdateCheckInterval,omitempty"`
	AutoUpdateTime          string `json:"autoUpdateTime,omitempty"`
}

// SaveNodesConfig saves nodes configuration to file (encrypted)
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
	
	// Encrypt if crypto manager is available
	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
	}
	
	if err := os.WriteFile(c.nodesFile, data, 0600); err != nil {
		return err
	}
	
	log.Info().Str("file", c.nodesFile).
		Int("pve", len(pveInstances)).
		Int("pbs", len(pbsInstances)).
		Bool("encrypted", c.crypto != nil).
		Msg("Nodes configuration saved")
	return nil
}

// LoadNodesConfig loads nodes configuration from file (decrypts if encrypted)
func (c *ConfigPersistence) LoadNodesConfig() (*NodesConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	data, err := os.ReadFile(c.nodesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if encrypted file doesn't exist
			log.Info().Msg("No encrypted nodes configuration found, returning empty config")
			return &NodesConfig{
				PVEInstances: []PVEInstance{},
				PBSInstances: []PBSInstance{},
			}, nil
		}
		return nil, err
	}
	
	// Decrypt if crypto manager is available
	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			return nil, err
		}
		data = decrypted
	}
	
	var config NodesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	
	// Fix for bug where TokenName was incorrectly set when using password auth
	// If a PBS instance has both Password and TokenName, clear the TokenName
	for i := range config.PBSInstances {
		if config.PBSInstances[i].Password != "" && config.PBSInstances[i].TokenName != "" {
			log.Info().
				Str("instance", config.PBSInstances[i].Name).
				Msg("Fixing PBS config: clearing TokenName since Password is set")
			config.PBSInstances[i].TokenName = ""
			config.PBSInstances[i].TokenValue = ""
		}
		
		// Fix for missing port in PBS host
		host := config.PBSInstances[i].Host
		if host != "" && !strings.Contains(host, ":8007") {
			// Add default PBS port if missing
			if strings.HasPrefix(host, "https://") {
				config.PBSInstances[i].Host = host + ":8007"
			} else if strings.HasPrefix(host, "http://") {
				config.PBSInstances[i].Host = host + ":8007"
			} else if !strings.Contains(host, "://") {
				// No protocol specified, add https and port
				config.PBSInstances[i].Host = "https://" + host + ":8007"
			}
			log.Info().
				Str("instance", config.PBSInstances[i].Name).
				Str("oldHost", host).
				Str("newHost", config.PBSInstances[i].Host).
				Msg("Fixed PBS host by adding default port 8007")
		}
	}
	
	log.Info().Str("file", c.nodesFile).
		Int("pve", len(config.PVEInstances)).
		Int("pbs", len(config.PBSInstances)).
		Bool("encrypted", c.crypto != nil).
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