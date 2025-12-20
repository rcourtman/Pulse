package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/crypto"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog/log"
)

// ConfigPersistence handles saving and loading configuration
type ConfigPersistence struct {
	mu                 sync.RWMutex
	tx                 *importTransaction
	configDir          string
	alertFile          string
	emailFile          string
	webhookFile        string
	appriseFile        string
	nodesFile          string
	systemFile         string
	oidcFile           string
	apiTokensFile      string
	aiFile             string
	aiFindingsFile     string
	aiPatrolRunsFile   string
	aiUsageHistoryFile string
	crypto             *crypto.CryptoManager
}

// NewConfigPersistence creates a new config persistence manager.
// The process terminates if encryption cannot be initialized to avoid
// writing secrets to disk in plaintext.
func NewConfigPersistence(configDir string) *ConfigPersistence {
	cp, err := newConfigPersistence(configDir)
	if err != nil {
		log.Fatal().
			Str("configDir", configDir).
			Err(err).
			Msg("Failed to initialize config persistence")
	}
	return cp
}

func newConfigPersistence(configDir string) (*ConfigPersistence, error) {
	if configDir == "" {
		if envDir := os.Getenv("PULSE_DATA_DIR"); envDir != "" {
			configDir = envDir
		} else {
			configDir = "/etc/pulse"
		}
	}

	// Initialize crypto manager
	cryptoMgr, err := crypto.NewCryptoManagerAt(configDir)
	if err != nil {
		return nil, err
	}

	cp := &ConfigPersistence{
		configDir:          configDir,
		alertFile:          filepath.Join(configDir, "alerts.json"),
		emailFile:          filepath.Join(configDir, "email.enc"),
		webhookFile:        filepath.Join(configDir, "webhooks.enc"),
		appriseFile:        filepath.Join(configDir, "apprise.enc"),
		nodesFile:          filepath.Join(configDir, "nodes.enc"),
		systemFile:         filepath.Join(configDir, "system.json"),
		oidcFile:           filepath.Join(configDir, "oidc.enc"),
		apiTokensFile:      filepath.Join(configDir, "api_tokens.json"),
		aiFile:             filepath.Join(configDir, "ai.enc"),
		aiFindingsFile:     filepath.Join(configDir, "ai_findings.json"),
		aiPatrolRunsFile:   filepath.Join(configDir, "ai_patrol_runs.json"),
		aiUsageHistoryFile: filepath.Join(configDir, "ai_usage_history.json"),
		crypto:             cryptoMgr,
	}

	log.Debug().
		Str("configDir", configDir).
		Str("systemFile", cp.systemFile).
		Str("nodesFile", cp.nodesFile).
		Bool("encryptionEnabled", cryptoMgr != nil).
		Msg("Config persistence initialized")

	return cp, nil
}

// DataDir returns the configuration directory path
func (c *ConfigPersistence) DataDir() string {
	return c.configDir
}

// EnsureConfigDir ensures the configuration directory exists
func (c *ConfigPersistence) EnsureConfigDir() error {
	return os.MkdirAll(c.configDir, 0700)
}

func (c *ConfigPersistence) beginTransaction(tx *importTransaction) {
	c.mu.Lock()
	c.tx = tx
	c.mu.Unlock()
}

func (c *ConfigPersistence) endTransaction(tx *importTransaction) {
	c.mu.Lock()
	if c.tx == tx {
		c.tx = nil
	}
	c.mu.Unlock()
}

// writeConfigFileLocked writes a config file, staging it in a transaction if one is active.
// NOTE: Caller MUST hold c.mu lock (despite reading c.tx, which has separate synchronization)
func (c *ConfigPersistence) writeConfigFileLocked(path string, data []byte, perm os.FileMode) error {
	// Read transaction pointer - safe because caller holds c.mu
	// Transaction field is only modified while holding c.mu in begin/endTransaction
	tx := c.tx

	if tx != nil {
		return tx.StageFile(path, data, perm)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// LoadAPITokens loads API token metadata from disk.
func (c *ConfigPersistence) LoadAPITokens() ([]APITokenRecord, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.apiTokensFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []APITokenRecord{}, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return []APITokenRecord{}, nil
	}

	var tokens []APITokenRecord
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}

	for i := range tokens {
		tokens[i].ensureScopes()
	}

	return tokens, nil
}

// SaveAPITokens persists API token metadata to disk.
func (c *ConfigPersistence) SaveAPITokens(tokens []APITokenRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	// Backup previous state (best effort).
	if existing, err := os.ReadFile(c.apiTokensFile); err == nil && len(existing) > 0 {
		if err := os.WriteFile(c.apiTokensFile+".backup", existing, 0600); err != nil {
			log.Warn().Err(err).Msg("Failed to create API token backup file")
		}
	}

	sanitized := make([]APITokenRecord, len(tokens))
	for i := range tokens {
		record := tokens[i]
		record.ensureScopes()
		sanitized[i] = record
	}

	data, err := json.Marshal(sanitized)
	if err != nil {
		return err
	}

	return c.writeConfigFileLocked(c.apiTokensFile, data, 0600)
}

// SaveAlertConfig saves alert configuration to file
func (c *ConfigPersistence) SaveAlertConfig(config alerts.AlertConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Ensure critical defaults are set before saving
	if config.StorageDefault.Trigger <= 0 {
		config.StorageDefault.Trigger = 85
		config.StorageDefault.Clear = 80
	}
	if config.MinimumDelta <= 0 {
		margin := config.HysteresisMargin
		if margin <= 0 {
			margin = 5.0
		}
		for id, override := range config.Overrides {
			if override.Usage != nil {
				if override.Usage.Clear <= 0 {
					override.Usage.Clear = override.Usage.Trigger - margin
					if override.Usage.Clear < 0 {
						override.Usage.Clear = 0
					}
				}
				config.Overrides[id] = override
			}
		}
		config.MinimumDelta = 2.0
	}
	if config.SuppressionWindow <= 0 {
		config.SuppressionWindow = 5
	}
	if config.HysteresisMargin <= 0 {
		config.HysteresisMargin = 5.0
	}

	// Host Defaults: Allow Trigger=0 to disable specific alerts
	if config.HostDefaults.CPU == nil || config.HostDefaults.CPU.Trigger < 0 {
		config.HostDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	} else if config.HostDefaults.CPU.Trigger == 0 {
		// Trigger=0 means disabled, set Clear=0 too
		config.HostDefaults.CPU.Clear = 0
	} else if config.HostDefaults.CPU.Clear <= 0 {
		config.HostDefaults.CPU.Clear = config.HostDefaults.CPU.Trigger - 5
		if config.HostDefaults.CPU.Clear <= 0 {
			config.HostDefaults.CPU.Clear = 75
		}
	}
	if config.HostDefaults.Memory == nil || config.HostDefaults.Memory.Trigger < 0 {
		config.HostDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}
	} else if config.HostDefaults.Memory.Trigger == 0 {
		// Trigger=0 means disabled, set Clear=0 too
		config.HostDefaults.Memory.Clear = 0
	} else if config.HostDefaults.Memory.Clear <= 0 {
		config.HostDefaults.Memory.Clear = config.HostDefaults.Memory.Trigger - 5
		if config.HostDefaults.Memory.Clear <= 0 {
			config.HostDefaults.Memory.Clear = 80
		}
	}
	if config.HostDefaults.Disk == nil || config.HostDefaults.Disk.Trigger < 0 {
		config.HostDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
	} else if config.HostDefaults.Disk.Trigger == 0 {
		// Trigger=0 means disabled, set Clear=0 too
		config.HostDefaults.Disk.Clear = 0
	} else if config.HostDefaults.Disk.Clear <= 0 {
		config.HostDefaults.Disk.Clear = config.HostDefaults.Disk.Trigger - 5
		if config.HostDefaults.Disk.Clear <= 0 {
			config.HostDefaults.Disk.Clear = 85
		}
	}

	config.MetricTimeThresholds = alerts.NormalizeMetricTimeThresholds(config.MetricTimeThresholds)
	if config.TimeThreshold <= 0 {
		config.TimeThreshold = 5
	}
	if config.TimeThresholds == nil {
		config.TimeThresholds = make(map[string]int)
	}
	ensureDelay := func(key string) {
		if delay, ok := config.TimeThresholds[key]; !ok || delay <= 0 {
			config.TimeThresholds[key] = config.TimeThreshold
		}
	}
	ensureDelay("guest")
	ensureDelay("node")
	ensureDelay("storage")
	ensureDelay("pbs")
	if delay, ok := config.TimeThresholds["all"]; ok && delay <= 0 {
		config.TimeThresholds["all"] = config.TimeThreshold
	}
	if config.SnapshotDefaults.WarningDays < 0 {
		config.SnapshotDefaults.WarningDays = 0
	}
	if config.SnapshotDefaults.CriticalDays < 0 {
		config.SnapshotDefaults.CriticalDays = 0
	}
	if config.SnapshotDefaults.CriticalDays > 0 && config.SnapshotDefaults.WarningDays > config.SnapshotDefaults.CriticalDays {
		config.SnapshotDefaults.WarningDays = config.SnapshotDefaults.CriticalDays
	}
	if config.SnapshotDefaults.CriticalDays == 0 && config.SnapshotDefaults.WarningDays > 0 {
		config.SnapshotDefaults.CriticalDays = config.SnapshotDefaults.WarningDays
	}
	if config.SnapshotDefaults.WarningSizeGiB < 0 {
		config.SnapshotDefaults.WarningSizeGiB = 0
	}
	if config.SnapshotDefaults.CriticalSizeGiB < 0 {
		config.SnapshotDefaults.CriticalSizeGiB = 0
	}
	if config.SnapshotDefaults.CriticalSizeGiB > 0 && config.SnapshotDefaults.WarningSizeGiB > config.SnapshotDefaults.CriticalSizeGiB {
		config.SnapshotDefaults.WarningSizeGiB = config.SnapshotDefaults.CriticalSizeGiB
	}
	if config.SnapshotDefaults.CriticalSizeGiB == 0 && config.SnapshotDefaults.WarningSizeGiB > 0 {
		config.SnapshotDefaults.CriticalSizeGiB = config.SnapshotDefaults.WarningSizeGiB
	}
	if config.BackupDefaults.WarningDays < 0 {
		config.BackupDefaults.WarningDays = 0
	}
	if config.BackupDefaults.CriticalDays < 0 {
		config.BackupDefaults.CriticalDays = 0
	}
	if config.BackupDefaults.CriticalDays > 0 && config.BackupDefaults.WarningDays > config.BackupDefaults.CriticalDays {
		config.BackupDefaults.WarningDays = config.BackupDefaults.CriticalDays
	}
	config.DockerIgnoredContainerPrefixes = alerts.NormalizeDockerIgnoredPrefixes(config.DockerIgnoredContainerPrefixes)

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	if err := c.writeConfigFileLocked(c.alertFile, data, 0600); err != nil {
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
					CPU:         &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
					Memory:      &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
					Disk:        &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
					Temperature: &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
				},
				HostDefaults: alerts.ThresholdConfig{
					CPU:    &alerts.HysteresisThreshold{Trigger: 80, Clear: 75},
					Memory: &alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
					Disk:   &alerts.HysteresisThreshold{Trigger: 90, Clear: 85},
				},
				StorageDefault: alerts.HysteresisThreshold{Trigger: 85, Clear: 80},
				TimeThreshold:  5,
				TimeThresholds: map[string]int{
					"guest":   5,
					"node":    5,
					"storage": 5,
					"pbs":     5,
				},
				MinimumDelta:      2.0,
				SuppressionWindow: 5,
				HysteresisMargin:  5.0,
				SnapshotDefaults: alerts.SnapshotAlertConfig{
					Enabled:         false,
					WarningDays:     30,
					CriticalDays:    45,
					WarningSizeGiB:  0,
					CriticalSizeGiB: 0,
				},
				BackupDefaults: alerts.BackupAlertConfig{
					Enabled:      false,
					WarningDays:  7,
					CriticalDays: 14,
					FreshHours:   24,
					StaleHours:   72,
				},
				Overrides: make(map[string]alerts.ThresholdConfig),
			}, nil
		}
		return nil, err
	}

	var config alerts.AlertConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// For empty config files ({}), enable alerts by default
	// This handles the case where the file exists but is empty
	if string(data) == "{}" {
		config.Enabled = true
	}
	if config.StorageDefault.Trigger <= 0 {
		config.StorageDefault.Trigger = 85
		config.StorageDefault.Clear = 80
	}
	if config.MinimumDelta <= 0 {
		config.MinimumDelta = 2.0
	}
	if config.SuppressionWindow <= 0 {
		config.SuppressionWindow = 5
	}
	if config.HysteresisMargin <= 0 {
		config.HysteresisMargin = 5.0
	}
	if config.NodeDefaults.Temperature == nil || config.NodeDefaults.Temperature.Trigger <= 0 {
		config.NodeDefaults.Temperature = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	}
	// Host Defaults: Allow Trigger=0 to disable specific alerts
	if config.HostDefaults.CPU == nil || config.HostDefaults.CPU.Trigger < 0 {
		config.HostDefaults.CPU = &alerts.HysteresisThreshold{Trigger: 80, Clear: 75}
	} else if config.HostDefaults.CPU.Trigger == 0 {
		// Trigger=0 means disabled, set Clear=0 too
		config.HostDefaults.CPU.Clear = 0
	} else if config.HostDefaults.CPU.Clear <= 0 {
		config.HostDefaults.CPU.Clear = config.HostDefaults.CPU.Trigger - 5
		if config.HostDefaults.CPU.Clear <= 0 {
			config.HostDefaults.CPU.Clear = 75
		}
	}
	if config.HostDefaults.Memory == nil || config.HostDefaults.Memory.Trigger < 0 {
		config.HostDefaults.Memory = &alerts.HysteresisThreshold{Trigger: 85, Clear: 80}
	} else if config.HostDefaults.Memory.Trigger == 0 {
		// Trigger=0 means disabled, set Clear=0 too
		config.HostDefaults.Memory.Clear = 0
	} else if config.HostDefaults.Memory.Clear <= 0 {
		config.HostDefaults.Memory.Clear = config.HostDefaults.Memory.Trigger - 5
		if config.HostDefaults.Memory.Clear <= 0 {
			config.HostDefaults.Memory.Clear = 80
		}
	}
	if config.HostDefaults.Disk == nil || config.HostDefaults.Disk.Trigger < 0 {
		config.HostDefaults.Disk = &alerts.HysteresisThreshold{Trigger: 90, Clear: 85}
	} else if config.HostDefaults.Disk.Trigger == 0 {
		// Trigger=0 means disabled, set Clear=0 too
		config.HostDefaults.Disk.Clear = 0
	} else if config.HostDefaults.Disk.Clear <= 0 {
		config.HostDefaults.Disk.Clear = config.HostDefaults.Disk.Trigger - 5
		if config.HostDefaults.Disk.Clear <= 0 {
			config.HostDefaults.Disk.Clear = 85
		}
	}
	if config.TimeThreshold <= 0 {
		config.TimeThreshold = 5
	}
	if config.TimeThresholds == nil {
		config.TimeThresholds = make(map[string]int)
	}
	ensureDelay := func(key string) {
		if delay, ok := config.TimeThresholds[key]; !ok || delay <= 0 {
			config.TimeThresholds[key] = config.TimeThreshold
		}
	}
	ensureDelay("guest")
	ensureDelay("node")
	ensureDelay("storage")
	ensureDelay("pbs")
	if delay, ok := config.TimeThresholds["all"]; ok && delay <= 0 {
		config.TimeThresholds["all"] = config.TimeThreshold
	}
	if config.SnapshotDefaults.WarningDays < 0 {
		config.SnapshotDefaults.WarningDays = 0
	}
	if config.SnapshotDefaults.CriticalDays < 0 {
		config.SnapshotDefaults.CriticalDays = 0
	}
	if config.SnapshotDefaults.CriticalDays > 0 && config.SnapshotDefaults.WarningDays > config.SnapshotDefaults.CriticalDays {
		config.SnapshotDefaults.WarningDays = config.SnapshotDefaults.CriticalDays
	}
	if config.SnapshotDefaults.WarningSizeGiB < 0 {
		config.SnapshotDefaults.WarningSizeGiB = 0
	}
	if config.SnapshotDefaults.CriticalSizeGiB < 0 {
		config.SnapshotDefaults.CriticalSizeGiB = 0
	}
	if config.SnapshotDefaults.CriticalSizeGiB > 0 && config.SnapshotDefaults.WarningSizeGiB > config.SnapshotDefaults.CriticalSizeGiB {
		config.SnapshotDefaults.WarningSizeGiB = config.SnapshotDefaults.CriticalSizeGiB
	}
	if config.SnapshotDefaults.CriticalSizeGiB == 0 && config.SnapshotDefaults.WarningSizeGiB > 0 {
		config.SnapshotDefaults.CriticalSizeGiB = config.SnapshotDefaults.WarningSizeGiB
	}
	if config.BackupDefaults.WarningDays < 0 {
		config.BackupDefaults.WarningDays = 0
	}
	if config.BackupDefaults.CriticalDays < 0 {
		config.BackupDefaults.CriticalDays = 0
	}
	if config.BackupDefaults.CriticalDays > 0 && config.BackupDefaults.WarningDays > config.BackupDefaults.CriticalDays {
		config.BackupDefaults.WarningDays = config.BackupDefaults.CriticalDays
	}
	// Default indicator thresholds for dashboard (separate from alert thresholds)
	if config.BackupDefaults.FreshHours <= 0 {
		config.BackupDefaults.FreshHours = 24
	}
	if config.BackupDefaults.StaleHours <= 0 {
		config.BackupDefaults.StaleHours = 72
	}
	// Ensure stale threshold is at least as large as fresh threshold
	if config.BackupDefaults.StaleHours < config.BackupDefaults.FreshHours {
		config.BackupDefaults.StaleHours = config.BackupDefaults.FreshHours
	}
	config.MetricTimeThresholds = alerts.NormalizeMetricTimeThresholds(config.MetricTimeThresholds)
	config.DockerIgnoredContainerPrefixes = alerts.NormalizeDockerIgnoredPrefixes(config.DockerIgnoredContainerPrefixes)

	// Migration: Set I/O metrics to Off (0) if they have the old default values
	// This helps existing users avoid noisy I/O alerts
	if config.GuestDefaults.DiskRead != nil && config.GuestDefaults.DiskRead.Trigger == 150 {
		config.GuestDefaults.DiskRead = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	}
	if config.GuestDefaults.DiskWrite != nil && config.GuestDefaults.DiskWrite.Trigger == 150 {
		config.GuestDefaults.DiskWrite = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	}
	if config.GuestDefaults.NetworkIn != nil && config.GuestDefaults.NetworkIn.Trigger == 200 {
		config.GuestDefaults.NetworkIn = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	}
	if config.GuestDefaults.NetworkOut != nil && config.GuestDefaults.NetworkOut.Trigger == 200 {
		config.GuestDefaults.NetworkOut = &alerts.HysteresisThreshold{Trigger: 0, Clear: 0}
	}

	log.Info().
		Str("file", c.alertFile).
		Bool("enabled", config.Enabled).
		Msg("Alert configuration loaded")
	return &config, nil
}

// SaveEmailConfig saves email configuration to file (encrypted)
func (c *ConfigPersistence) SaveEmailConfig(config notifications.EmailConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Marshal to JSON first
	data, err := json.Marshal(config)
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
	if err := c.writeConfigFileLocked(c.emailFile, data, 0600); err != nil {
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

// SaveAppriseConfig saves Apprise configuration to file (encrypted if available)
func (c *ConfigPersistence) SaveAppriseConfig(config notifications.AppriseConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	config = notifications.NormalizeAppriseConfig(config)

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
	}

	if err := c.writeConfigFileLocked(c.appriseFile, data, 0600); err != nil {
		return err
	}

	log.Info().
		Str("file", c.appriseFile).
		Bool("encrypted", c.crypto != nil).
		Msg("Apprise configuration saved")
	return nil
}

// LoadAppriseConfig loads Apprise configuration from file (decrypts if encrypted)
func (c *ConfigPersistence) LoadAppriseConfig() (*notifications.AppriseConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.appriseFile)
	if err != nil {
		if os.IsNotExist(err) {
			defaultCfg := notifications.AppriseConfig{
				Enabled:        false,
				Mode:           notifications.AppriseModeCLI,
				Targets:        []string{},
				CLIPath:        "apprise",
				TimeoutSeconds: 15,
				APIKeyHeader:   "X-API-KEY",
			}
			return &defaultCfg, nil
		}
		return nil, err
	}

	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			return nil, err
		}
		data = decrypted
	}

	var config notifications.AppriseConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	normalized := notifications.NormalizeAppriseConfig(config)

	log.Info().
		Str("file", c.appriseFile).
		Bool("encrypted", c.crypto != nil).
		Msg("Apprise configuration loaded")
	return &normalized, nil
}

// SaveWebhooks saves webhook configurations to file
func (c *ConfigPersistence) SaveWebhooks(webhooks []notifications.WebhookConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(webhooks)
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

	if err := c.writeConfigFileLocked(c.webhookFile, data, 0600); err != nil {
		return err
	}

	log.Info().Str("file", c.webhookFile).
		Int("count", len(webhooks)).
		Bool("encrypted", c.crypto != nil).
		Msg("Webhooks saved")
	return nil
}

// LoadWebhooks loads webhook configurations from file (decrypts if encrypted)
func (c *ConfigPersistence) LoadWebhooks() ([]notifications.WebhookConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// First try to load from encrypted file
	data, err := os.ReadFile(c.webhookFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Check for legacy unencrypted file
			legacyFile := filepath.Join(c.configDir, "webhooks.json")
			legacyData, legacyErr := os.ReadFile(legacyFile)
			if legacyErr == nil {
				// Legacy file exists, parse it
				var webhooks []notifications.WebhookConfig
				if err := json.Unmarshal(legacyData, &webhooks); err == nil {
					log.Info().
						Str("file", legacyFile).
						Int("count", len(webhooks)).
						Msg("Found unencrypted webhooks - migration needed")

					// Return the loaded webhooks - migration will be handled by caller
					return webhooks, nil
				}
			}
			// No webhooks file exists
			return []notifications.WebhookConfig{}, nil
		}
		return nil, err
	}

	// Decrypt if crypto manager is available
	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			// Try parsing as plain JSON (migration case)
			var webhooks []notifications.WebhookConfig
			if jsonErr := json.Unmarshal(data, &webhooks); jsonErr == nil {
				log.Info().
					Str("file", c.webhookFile).
					Int("count", len(webhooks)).
					Msg("Loaded unencrypted webhooks (will encrypt on next save)")
				return webhooks, nil
			}
			return nil, fmt.Errorf("failed to decrypt webhooks: %w", err)
		}
		data = decrypted
	}

	var webhooks []notifications.WebhookConfig
	if err := json.Unmarshal(data, &webhooks); err != nil {
		return nil, err
	}

	log.Info().
		Str("file", c.webhookFile).
		Int("count", len(webhooks)).
		Bool("encrypted", c.crypto != nil).
		Msg("Webhooks loaded")
	return webhooks, nil
}

// MigrateWebhooksIfNeeded checks for legacy webhooks.json and migrates to encrypted format
func (c *ConfigPersistence) MigrateWebhooksIfNeeded() error {
	// Check if encrypted file already exists
	if _, err := os.Stat(c.webhookFile); err == nil {
		// Encrypted file exists, no migration needed
		return nil
	}

	// Check for legacy unencrypted file
	legacyFile := filepath.Join(c.configDir, "webhooks.json")
	legacyData, err := os.ReadFile(legacyFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No legacy file, nothing to migrate
			return nil
		}
		return fmt.Errorf("failed to read legacy webhooks file: %w", err)
	}

	// Parse legacy webhooks
	var webhooks []notifications.WebhookConfig
	if err := json.Unmarshal(legacyData, &webhooks); err != nil {
		return fmt.Errorf("failed to parse legacy webhooks: %w", err)
	}

	log.Info().
		Str("from", legacyFile).
		Str("to", c.webhookFile).
		Int("count", len(webhooks)).
		Msg("Migrating webhooks to encrypted format")

	// Save to encrypted file
	if err := c.SaveWebhooks(webhooks); err != nil {
		return fmt.Errorf("failed to save encrypted webhooks: %w", err)
	}

	// Create backup of original file
	backupFile := legacyFile + ".backup"
	if err := os.Rename(legacyFile, backupFile); err != nil {
		log.Warn().Err(err).Msg("Failed to rename legacy webhooks file to backup")
	} else {
		log.Info().Str("backup", backupFile).Msg("Legacy webhooks file backed up")
	}

	return nil
}

// NodesConfig represents the saved nodes configuration
type NodesConfig struct {
	PVEInstances []PVEInstance `json:"pveInstances"`
	PBSInstances []PBSInstance `json:"pbsInstances"`
	PMGInstances []PMGInstance `json:"pmgInstances"`
}

// SystemSettings represents system configuration settings
type SystemSettings struct {
	PVEPollingInterval           int             `json:"pvePollingInterval"` // PVE polling interval in seconds
	PBSPollingInterval           int             `json:"pbsPollingInterval"` // PBS polling interval in seconds
	PMGPollingInterval           int             `json:"pmgPollingInterval"` // PMG polling interval in seconds
	BackupPollingInterval        int             `json:"backupPollingInterval,omitempty"`
	BackupPollingEnabled         *bool           `json:"backupPollingEnabled,omitempty"`
	AdaptivePollingEnabled       *bool           `json:"adaptivePollingEnabled,omitempty"`
	AdaptivePollingBaseInterval  int             `json:"adaptivePollingBaseInterval,omitempty"`
	AdaptivePollingMinInterval   int             `json:"adaptivePollingMinInterval,omitempty"`
	AdaptivePollingMaxInterval   int             `json:"adaptivePollingMaxInterval,omitempty"`
	BackendPort                  int             `json:"backendPort,omitempty"`
	FrontendPort                 int             `json:"frontendPort,omitempty"`
	AllowedOrigins               string          `json:"allowedOrigins,omitempty"`
	ConnectionTimeout            int             `json:"connectionTimeout,omitempty"`
	UpdateChannel                string          `json:"updateChannel,omitempty"`
	AutoUpdateEnabled            bool            `json:"autoUpdateEnabled"` // Removed omitempty so false is saved
	AutoUpdateCheckInterval      int             `json:"autoUpdateCheckInterval,omitempty"`
	AutoUpdateTime               string          `json:"autoUpdateTime,omitempty"`
	LogLevel                     string          `json:"logLevel,omitempty"`
	DiscoveryEnabled             bool            `json:"discoveryEnabled"`
	DiscoverySubnet              string          `json:"discoverySubnet,omitempty"`
	DiscoveryConfig              DiscoveryConfig `json:"discoveryConfig"`
	Theme                        string          `json:"theme,omitempty"`               // User theme preference: "light", "dark", or empty for system default
	AllowEmbedding               bool            `json:"allowEmbedding"`                // Allow iframe embedding
	AllowedEmbedOrigins          string          `json:"allowedEmbedOrigins,omitempty"` // Comma-separated list of allowed origins for embedding
	TemperatureMonitoringEnabled bool            `json:"temperatureMonitoringEnabled"`
	DNSCacheTimeout              int             `json:"dnsCacheTimeout,omitempty"`            // DNS cache timeout in seconds (0 = default 5 minutes)
	SSHPort                      int             `json:"sshPort,omitempty"`                    // Default SSH port for temperature monitoring (0 = use 22)
	WebhookAllowedPrivateCIDRs   string          `json:"webhookAllowedPrivateCIDRs,omitempty"` // Comma-separated list of private CIDR ranges allowed for webhooks (e.g., "192.168.1.0/24,10.0.0.0/8")
	HideLocalLogin               bool            `json:"hideLocalLogin"`                       // Hide local login form (username/password)

	// Metrics retention configuration (in hours)
	// These control how long historical metrics are stored at each aggregation tier.
	// Longer retention enables trend analysis but increases storage usage.
	MetricsRetentionRawHours    int `json:"metricsRetentionRawHours,omitempty"`    // Raw data (~5s intervals), default: 2 hours
	MetricsRetentionMinuteHours int `json:"metricsRetentionMinuteHours,omitempty"` // Minute averages, default: 24 hours
	MetricsRetentionHourlyDays  int `json:"metricsRetentionHourlyDays,omitempty"`  // Hourly averages, default: 7 days
	MetricsRetentionDailyDays   int `json:"metricsRetentionDailyDays,omitempty"`   // Daily averages, default: 90 days

	// APIToken removed - now handled via .env file only
}

// DefaultSystemSettings returns a SystemSettings struct populated with sane defaults.
func DefaultSystemSettings() *SystemSettings {
	defaultDiscovery := DefaultDiscoveryConfig()
	return &SystemSettings{
		PVEPollingInterval:           10,
		PBSPollingInterval:           60,
		PMGPollingInterval:           60,
		AutoUpdateEnabled:            false,
		DiscoveryEnabled:             false,
		DiscoverySubnet:              "auto",
		DiscoveryConfig:              defaultDiscovery,
		AllowEmbedding:               false,
		TemperatureMonitoringEnabled: true,
		DNSCacheTimeout:              300, // Default: 5 minutes (300 seconds)
	}
}

// SaveNodesConfig saves nodes configuration to file (encrypted)
func (c *ConfigPersistence) SaveNodesConfig(pveInstances []PVEInstance, pbsInstances []PBSInstance, pmgInstances []PMGInstance) error {
	return c.saveNodesConfig(pveInstances, pbsInstances, pmgInstances, false)
}

// SaveNodesConfigAllowEmpty saves nodes configuration even when all nodes are removed.
// Use sparingly for explicit administrative actions (e.g. deleting the final node).
func (c *ConfigPersistence) SaveNodesConfigAllowEmpty(pveInstances []PVEInstance, pbsInstances []PBSInstance, pmgInstances []PMGInstance) error {
	return c.saveNodesConfig(pveInstances, pbsInstances, pmgInstances, true)
}

func (c *ConfigPersistence) saveNodesConfig(pveInstances []PVEInstance, pbsInstances []PBSInstance, pmgInstances []PMGInstance, allowEmpty bool) error {
	// CRITICAL: Prevent saving empty nodes when in mock mode
	// Mock mode should NEVER modify real node configuration
	if mock.IsMockEnabled() {
		log.Warn().Msg("Skipping nodes save - mock mode is enabled")
		return nil // Silently succeed to prevent errors but don't save
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// CRITICAL: Never save empty nodes configuration
	// This prevents data loss from accidental wipes
	if !allowEmpty && len(pveInstances) == 0 && len(pbsInstances) == 0 && len(pmgInstances) == 0 {
		// If we're replacing an existing non-empty config, block the wipe.
		// We must not call LoadNodesConfig here because it acquires c.mu again.
		if _, err := os.Stat(c.nodesFile); err == nil {
			data, err := os.ReadFile(c.nodesFile)
			if err != nil {
				return fmt.Errorf("refusing to save empty nodes config: failed to read existing nodes config: %w", err)
			}
			if c.crypto != nil {
				decrypted, err := c.crypto.Decrypt(data)
				if err != nil {
					return fmt.Errorf("refusing to save empty nodes config: existing nodes config is not decryptable: %w", err)
				}
				data = decrypted
			}

			var existing NodesConfig
			if err := json.Unmarshal(data, &existing); err != nil {
				return fmt.Errorf("refusing to save empty nodes config: existing nodes config is not parseable: %w", err)
			}

			if len(existing.PVEInstances) > 0 || len(existing.PBSInstances) > 0 || len(existing.PMGInstances) > 0 {
				log.Error().
					Int("existing_pve", len(existing.PVEInstances)).
					Int("existing_pbs", len(existing.PBSInstances)).
					Int("existing_pmg", len(existing.PMGInstances)).
					Msg("BLOCKED attempt to save empty nodes config - would delete existing nodes!")
				return fmt.Errorf("refusing to save empty nodes config when %d nodes exist",
					len(existing.PVEInstances)+len(existing.PBSInstances)+len(existing.PMGInstances))
			}
		}
	}

	config := NodesConfig{
		PVEInstances: pveInstances,
		PBSInstances: pbsInstances,
		PMGInstances: pmgInstances,
	}

	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	// Create TIMESTAMPED backup of existing file before overwriting (if it exists and has content)
	// This ensures we keep multiple backups and can recover from disasters
	if info, err := os.Stat(c.nodesFile); err == nil && info.Size() > 0 {
		// Create timestamped backup
		timestampedBackup := fmt.Sprintf("%s.backup-%s", c.nodesFile, time.Now().Format("20060102-150405"))
		if backupData, err := os.ReadFile(c.nodesFile); err == nil {
			if err := os.WriteFile(timestampedBackup, backupData, 0600); err != nil {
				log.Warn().Err(err).Msg("Failed to create timestamped backup of nodes config")
			} else {
				log.Info().Str("backup", timestampedBackup).Msg("Created timestamped backup of nodes config")
			}
		}

		// Also maintain a "latest" backup for quick recovery
		latestBackup := c.nodesFile + ".backup"
		if backupData, err := os.ReadFile(c.nodesFile); err == nil {
			if err := os.WriteFile(latestBackup, backupData, 0600); err != nil {
				log.Warn().Err(err).Msg("Failed to create latest backup of nodes config")
			}
		}

		// Clean up old timestamped backups (keep last 10)
		c.cleanupOldBackups(c.nodesFile + ".backup-*")
	}

	// Encrypt if crypto manager is available
	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
	}

	if err := c.writeConfigFileLocked(c.nodesFile, data, 0600); err != nil {
		return err
	}

	log.Info().Str("file", c.nodesFile).
		Int("pve", len(pveInstances)).
		Int("pbs", len(pbsInstances)).
		Int("pmg", len(pmgInstances)).
		Bool("encrypted", c.crypto != nil).
		Msg("Nodes configuration saved")
	return nil
}

// LoadNodesConfig loads nodes configuration from file (decrypts if encrypted)
func (c *ConfigPersistence) LoadNodesConfig() (*NodesConfig, error) {
	c.mu.RLock()
	unlocked := false
	defer func() {
		if !unlocked {
			c.mu.RUnlock()
		}
	}()

	data, err := os.ReadFile(c.nodesFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config if encrypted file doesn't exist
			log.Info().Msg("No encrypted nodes configuration found, returning empty config")
			return &NodesConfig{
				PVEInstances: []PVEInstance{},
				PBSInstances: []PBSInstance{},
				PMGInstances: []PMGInstance{},
			}, nil
		}
		return nil, err
	}

	// Decrypt if crypto manager is available
	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			// Decryption failed - file may be corrupted
			log.Error().Err(err).Str("file", c.nodesFile).Msg("Failed to decrypt nodes config - file may be corrupted")

			// Try to restore from backup
			backupFile := c.nodesFile + ".backup"
			if backupData, backupErr := os.ReadFile(backupFile); backupErr == nil {
				log.Info().Str("backup", backupFile).Msg("Attempting to restore nodes config from backup")
				if decryptedBackup, decryptErr := c.crypto.Decrypt(backupData); decryptErr == nil {
					log.Info().Msg("Successfully decrypted backup file")
					data = decryptedBackup

					// Move corrupted file out of the way with timestamp
					corruptedFile := fmt.Sprintf("%s.corrupted-%s", c.nodesFile, time.Now().Format("20060102-150405"))
					if renameErr := os.Rename(c.nodesFile, corruptedFile); renameErr != nil {
						log.Warn().Err(renameErr).Msg("Failed to rename corrupted file")
					} else {
						log.Warn().Str("corruptedFile", corruptedFile).Msg("Moved corrupted nodes config")
					}

					// Restore backup as current file
					if writeErr := os.WriteFile(c.nodesFile, backupData, 0600); writeErr != nil {
						log.Error().Err(writeErr).Msg("Failed to restore backup as current file")
					} else {
						log.Info().Msg("Successfully restored nodes config from backup")
					}
				} else {
					log.Error().Err(decryptErr).Msg("Backup file is also corrupted or encrypted with different key")

					// CRITICAL: Don't delete the corrupted file - leave it for manual recovery
					// Create an empty config so startup can continue, but log prominently
					log.Error().
						Str("corruptedFile", c.nodesFile).
						Str("backupFile", backupFile).
						Msg("CRITICAL: Both nodes.enc and backup are corrupted/unreadable. Encryption key may have been regenerated. Manual recovery required. Starting with empty config.")

					// Move corrupted file with timestamp for forensics
					corruptedFile := fmt.Sprintf("%s.corrupted-%s", c.nodesFile, time.Now().Format("20060102-150405"))
					os.Rename(c.nodesFile, corruptedFile)

					// Create empty but valid config so system can start
					emptyConfig := NodesConfig{PVEInstances: []PVEInstance{}, PBSInstances: []PBSInstance{}, PMGInstances: []PMGInstance{}}
					emptyData, _ := json.Marshal(emptyConfig)
					if c.crypto != nil {
						emptyData, _ = c.crypto.Encrypt(emptyData)
					}
					os.WriteFile(c.nodesFile, emptyData, 0600)

					return &emptyConfig, nil
				}
			} else {
				log.Error().Err(backupErr).Msg("No backup file available for recovery")

				// CRITICAL: Don't delete the corrupted file - leave it for manual recovery
				log.Error().
					Str("corruptedFile", c.nodesFile).
					Msg("CRITICAL: nodes.enc is corrupted and no backup exists. Encryption key may have been regenerated. Manual recovery required. Starting with empty config.")

				// Move corrupted file with timestamp for forensics
				corruptedFile := fmt.Sprintf("%s.corrupted-%s", c.nodesFile, time.Now().Format("20060102-150405"))
				os.Rename(c.nodesFile, corruptedFile)

				// Create empty but valid config so system can start
				emptyConfig := NodesConfig{PVEInstances: []PVEInstance{}, PBSInstances: []PBSInstance{}, PMGInstances: []PMGInstance{}}
				emptyData, _ := json.Marshal(emptyConfig)
				if c.crypto != nil {
					emptyData, _ = c.crypto.Encrypt(emptyData)
				}
				os.WriteFile(c.nodesFile, emptyData, 0600)

				return &emptyConfig, nil
			}
		} else {
			data = decrypted
		}
	}

	var config NodesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.PVEInstances == nil {
		config.PVEInstances = []PVEInstance{}
	}
	if config.PBSInstances == nil {
		config.PBSInstances = []PBSInstance{}
	}
	if config.PMGInstances == nil {
		config.PMGInstances = []PMGInstance{}
	}

	// Track if any migrations were applied
	migrationApplied := false

	normalizeHostWithDefault := func(host, defaultPort string) (string, bool) {
		normalized := normalizeHostPort(host, defaultPort)
		if normalized == "" || normalized == host {
			return host, false
		}
		return normalized, true
	}

	// Normalize PVE hosts (and cluster endpoints) to restore default ports for configs
	// saved during the 4.32.0 regression that skipped default port injection.
	for i := range config.PVEInstances {
		if normalized, changed := normalizeHostWithDefault(config.PVEInstances[i].Host, defaultPVEPort); changed {
			log.Info().
				Str("instance", config.PVEInstances[i].Name).
				Str("oldHost", config.PVEInstances[i].Host).
				Str("newHost", normalized).
				Msg("Normalized PVE host to include default port")
			config.PVEInstances[i].Host = normalized
			migrationApplied = true
		}

		for j := range config.PVEInstances[i].ClusterEndpoints {
			if normalized, changed := normalizeHostWithDefault(config.PVEInstances[i].ClusterEndpoints[j].Host, defaultPVEPort); changed {
				log.Info().
					Str("instance", config.PVEInstances[i].Name).
					Str("node", config.PVEInstances[i].ClusterEndpoints[j].NodeName).
					Str("oldHost", config.PVEInstances[i].ClusterEndpoints[j].Host).
					Str("newHost", normalized).
					Msg("Normalized PVE cluster endpoint host to include default port")
				config.PVEInstances[i].ClusterEndpoints[j].Host = normalized
				migrationApplied = true
			}
		}
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
			migrationApplied = true
		}

		if normalized, changed := normalizeHostWithDefault(config.PBSInstances[i].Host, defaultPBSPort); changed {
			log.Info().
				Str("instance", config.PBSInstances[i].Name).
				Str("oldHost", config.PBSInstances[i].Host).
				Str("newHost", normalized).
				Msg("Normalized PBS host to include default port")
			config.PBSInstances[i].Host = normalized
			migrationApplied = true
		}

		// Migration: Ensure MonitorBackups is enabled for PBS instances
		// This fixes issue #411 where PBS backups weren't showing
		if !config.PBSInstances[i].MonitorBackups {
			log.Info().
				Str("instance", config.PBSInstances[i].Name).
				Msg("Enabling MonitorBackups for PBS instance (was disabled)")
			config.PBSInstances[i].MonitorBackups = true
			migrationApplied = true
		}
	}

	for i := range config.PMGInstances {
		if config.PMGInstances[i].Password != "" && config.PMGInstances[i].TokenName != "" {
			log.Info().
				Str("instance", config.PMGInstances[i].Name).
				Msg("Fixing PMG config: clearing TokenName since Password is set")
			config.PMGInstances[i].TokenName = ""
			config.PMGInstances[i].TokenValue = ""
			migrationApplied = true
		}

		if normalized, changed := normalizeHostWithDefault(config.PMGInstances[i].Host, defaultPVEPort); changed {
			log.Info().
				Str("instance", config.PMGInstances[i].Name).
				Str("oldHost", config.PMGInstances[i].Host).
				Str("newHost", normalized).
				Msg("Normalized PMG host to include default port")
			config.PMGInstances[i].Host = normalized
			migrationApplied = true
		}
	}

	log.Info().Str("file", c.nodesFile).
		Int("pve", len(config.PVEInstances)).
		Int("pbs", len(config.PBSInstances)).
		Int("pmg", len(config.PMGInstances)).
		Bool("encrypted", c.crypto != nil).
		Msg("Nodes configuration loaded")

	// If any migrations were applied, save the updated configuration after releasing the read lock
	// This prevents unlock/relock race condition where another goroutine could modify config
	// between unlock and relock, causing migrated data to be lost
	if migrationApplied {
		// Make copies while still holding read lock to ensure consistency
		pveCopy := make([]PVEInstance, len(config.PVEInstances))
		copy(pveCopy, config.PVEInstances)
		pbsCopy := make([]PBSInstance, len(config.PBSInstances))
		copy(pbsCopy, config.PBSInstances)
		pmgCopy := make([]PMGInstance, len(config.PMGInstances))
		copy(pmgCopy, config.PMGInstances)

		// Release read lock before saving (SaveNodesConfig acquires write lock)
		unlocked = true
		c.mu.RUnlock()

		log.Info().Msg("Migrations applied, saving updated configuration")
		if err := c.SaveNodesConfig(pveCopy, pbsCopy, pmgCopy); err != nil {
			log.Error().Err(err).Msg("Failed to save configuration after migration")
		}

		// Don't reacquire lock - we're returning
		return &config, nil
	}

	return &config, nil
}

// SaveSystemSettings saves system settings to file
func (c *ConfigPersistence) SaveSystemSettings(settings SystemSettings) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	if err := c.writeConfigFileLocked(c.systemFile, data, 0600); err != nil {
		return err
	}

	// Also update the .env file if it exists
	envFile := filepath.Join(c.configDir, ".env")
	if err := c.updateEnvFile(envFile, settings); err != nil {
		log.Warn().Err(err).Msg("Failed to update .env file")
		// Don't fail the operation if .env update fails
	}

	log.Info().Str("file", c.systemFile).Msg("System settings saved")
	return nil
}

// SaveOIDCConfig stores OIDC settings, encrypting them when a crypto manager is available.
func (c *ConfigPersistence) SaveOIDCConfig(settings OIDCConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	// Do not persist runtime-only flags.
	settings.EnvOverrides = nil

	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
	}

	if err := c.writeConfigFileLocked(c.oidcFile, data, 0600); err != nil {
		return err
	}

	log.Info().Str("file", c.oidcFile).Msg("OIDC configuration saved")
	return nil
}

// LoadOIDCConfig retrieves the persisted OIDC settings. It returns nil when no configuration exists yet.
func (c *ConfigPersistence) LoadOIDCConfig() (*OIDCConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.oidcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			return nil, err
		}
		data = decrypted
	}

	var settings OIDCConfig
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	log.Info().Str("file", c.oidcFile).Msg("OIDC configuration loaded")
	return &settings, nil
}

// SaveAIConfig stores AI settings, encrypting them when a crypto manager is available.
func (c *ConfigPersistence) SaveAIConfig(settings AIConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	if c.crypto != nil {
		encrypted, err := c.crypto.Encrypt(data)
		if err != nil {
			return err
		}
		data = encrypted
	}

	if err := c.writeConfigFileLocked(c.aiFile, data, 0600); err != nil {
		return err
	}

	log.Info().Str("file", c.aiFile).Bool("enabled", settings.Enabled).Msg("AI configuration saved")
	return nil
}

// LoadAIConfig retrieves the persisted AI settings. It returns default config when no configuration exists yet.
func (c *ConfigPersistence) LoadAIConfig() (*AIConfig, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.aiFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if file doesn't exist
			return NewDefaultAIConfig(), nil
		}
		return nil, err
	}

	if c.crypto != nil {
		decrypted, err := c.crypto.Decrypt(data)
		if err != nil {
			return nil, err
		}
		data = decrypted
	}

	// Start with defaults so new fields get proper values
	settings := NewDefaultAIConfig()
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, err
	}

	// Migration: Ensure patrol settings have sensible defaults for existing configs
	// PatrolIntervalMinutes=0 means it was never set - use default
	if settings.PatrolIntervalMinutes <= 0 {
		settings.PatrolIntervalMinutes = 15
	}

	log.Info().Str("file", c.aiFile).Bool("enabled", settings.Enabled).Bool("patrol_enabled", settings.PatrolEnabled).Msg("AI configuration loaded")
	return settings, nil
}

// AIFindingsData represents persisted AI findings with metadata
type AIFindingsData struct {
	// Version for future migrations
	Version int `json:"version"`
	// LastSaved for debugging/diagnostics
	LastSaved time.Time `json:"last_saved"`
	// Findings is a map of finding ID to finding data
	Findings map[string]*AIFindingRecord `json:"findings"`
}

// AIFindingRecord is a persisted finding with full history
type AIFindingRecord struct {
	ID             string     `json:"id"`
	Key            string     `json:"key,omitempty"`
	Severity       string     `json:"severity"`
	Category       string     `json:"category"`
	ResourceID     string     `json:"resource_id"`
	ResourceName   string     `json:"resource_name"`
	ResourceType   string     `json:"resource_type"`
	Node           string     `json:"node,omitempty"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Recommendation string     `json:"recommendation,omitempty"`
	Evidence       string     `json:"evidence,omitempty"`
	DetectedAt     time.Time  `json:"detected_at"`
	LastSeenAt     time.Time  `json:"last_seen_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	AutoResolved   bool       `json:"auto_resolved"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	SnoozedUntil   *time.Time `json:"snoozed_until,omitempty"`
	AlertID        string     `json:"alert_id,omitempty"`
}

// SaveAIFindings persists AI findings to disk
func (c *ConfigPersistence) SaveAIFindings(findings map[string]*AIFindingRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	data := AIFindingsData{
		Version:   1,
		LastSaved: time.Now(),
		Findings:  findings,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := c.writeConfigFileLocked(c.aiFindingsFile, jsonData, 0600); err != nil {
		return err
	}

	log.Debug().
		Str("file", c.aiFindingsFile).
		Int("count", len(findings)).
		Msg("AI findings saved")
	return nil
}

// LoadAIFindings loads AI findings from disk
func (c *ConfigPersistence) LoadAIFindings() (*AIFindingsData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.aiFindingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty data if file doesn't exist
			return &AIFindingsData{
				Version:  1,
				Findings: make(map[string]*AIFindingRecord),
			}, nil
		}
		return nil, err
	}

	var findingsData AIFindingsData
	if err := json.Unmarshal(data, &findingsData); err != nil {
		log.Error().Err(err).Str("file", c.aiFindingsFile).Msg("Failed to parse AI findings file")
		// Return empty data on parse error rather than failing
		return &AIFindingsData{
			Version:  1,
			Findings: make(map[string]*AIFindingRecord),
		}, nil
	}

	if findingsData.Findings == nil {
		findingsData.Findings = make(map[string]*AIFindingRecord)
	}

	log.Info().
		Str("file", c.aiFindingsFile).
		Int("count", len(findingsData.Findings)).
		Time("last_saved", findingsData.LastSaved).
		Msg("AI findings loaded")
	return &findingsData, nil
}

// PatrolRunHistoryData represents persisted patrol run history with metadata
type PatrolRunHistoryData struct {
	Version   int               `json:"version"`
	LastSaved time.Time         `json:"last_saved"`
	Runs      []PatrolRunRecord `json:"runs"`
}

// PatrolRunRecord represents a single patrol check run
type PatrolRunRecord struct {
	ID               string    `json:"id"`
	StartedAt        time.Time `json:"started_at"`
	CompletedAt      time.Time `json:"completed_at"`
	DurationMs       int64     `json:"duration_ms"`
	Type             string    `json:"type"` // "quick" or "deep"
	ResourcesChecked int       `json:"resources_checked"`
	// Breakdown by resource type
	NodesChecked   int `json:"nodes_checked"`
	GuestsChecked  int `json:"guests_checked"`
	DockerChecked  int `json:"docker_checked"`
	StorageChecked int `json:"storage_checked"`
	HostsChecked   int `json:"hosts_checked"`
	PBSChecked     int `json:"pbs_checked"`
	// Findings from this run
	NewFindings      int      `json:"new_findings"`
	ExistingFindings int      `json:"existing_findings"`
	ResolvedFindings int      `json:"resolved_findings"`
	AutoFixCount     int      `json:"auto_fix_count,omitempty"`
	FindingsSummary  string   `json:"findings_summary"`
	FindingIDs       []string `json:"finding_ids,omitempty"`
	ErrorCount       int      `json:"error_count"`
	Status           string   `json:"status"` // "healthy", "issues_found", "critical", "error"
	// AI Analysis details
	AIAnalysis   string `json:"ai_analysis,omitempty"`   // The AI's raw response/analysis
	InputTokens  int    `json:"input_tokens,omitempty"`  // Tokens sent to AI
	OutputTokens int    `json:"output_tokens,omitempty"` // Tokens received from AI
}

// AIUsageHistoryData represents persisted AI usage history with metadata
type AIUsageHistoryData struct {
	Version   int                  `json:"version"`
	LastSaved time.Time            `json:"last_saved"`
	Events    []AIUsageEventRecord `json:"events"`
}

// AIUsageEventRecord is a persisted usage event for an AI provider call.
// This intentionally excludes prompt/response content for privacy.
type AIUsageEventRecord struct {
	Timestamp     time.Time `json:"timestamp"`
	Provider      string    `json:"provider"`
	RequestModel  string    `json:"request_model"`
	ResponseModel string    `json:"response_model,omitempty"`
	UseCase       string    `json:"use_case,omitempty"` // "chat" or "patrol"
	InputTokens   int       `json:"input_tokens,omitempty"`
	OutputTokens  int       `json:"output_tokens,omitempty"`
	TargetType    string    `json:"target_type,omitempty"`
	TargetID      string    `json:"target_id,omitempty"`
	FindingID     string    `json:"finding_id,omitempty"`
}

// SaveAIUsageHistory persists AI usage events to disk.
func (c *ConfigPersistence) SaveAIUsageHistory(events []AIUsageEventRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	data := AIUsageHistoryData{
		Version:   1,
		LastSaved: time.Now(),
		Events:    events,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := c.writeConfigFileLocked(c.aiUsageHistoryFile, jsonData, 0600); err != nil {
		return err
	}

	log.Debug().
		Str("file", c.aiUsageHistoryFile).
		Int("count", len(events)).
		Msg("AI usage history saved")
	return nil
}

// LoadAIUsageHistory loads AI usage events from disk.
func (c *ConfigPersistence) LoadAIUsageHistory() (*AIUsageHistoryData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.aiUsageHistoryFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &AIUsageHistoryData{
				Version: 1,
				Events:  make([]AIUsageEventRecord, 0),
			}, nil
		}
		return nil, err
	}

	var usageData AIUsageHistoryData
	if err := json.Unmarshal(data, &usageData); err != nil {
		log.Error().Err(err).Str("file", c.aiUsageHistoryFile).Msg("Failed to parse AI usage history file")
		return &AIUsageHistoryData{
			Version: 1,
			Events:  make([]AIUsageEventRecord, 0),
		}, nil
	}

	if usageData.Events == nil {
		usageData.Events = make([]AIUsageEventRecord, 0)
	}

	log.Info().
		Str("file", c.aiUsageHistoryFile).
		Int("count", len(usageData.Events)).
		Time("last_saved", usageData.LastSaved).
		Msg("AI usage history loaded")
	return &usageData, nil
}

// SavePatrolRunHistory persists patrol run history to disk
func (c *ConfigPersistence) SavePatrolRunHistory(runs []PatrolRunRecord) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.EnsureConfigDir(); err != nil {
		return err
	}

	data := PatrolRunHistoryData{
		Version:   1,
		LastSaved: time.Now(),
		Runs:      runs,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := c.writeConfigFileLocked(c.aiPatrolRunsFile, jsonData, 0600); err != nil {
		return err
	}

	log.Debug().
		Str("file", c.aiPatrolRunsFile).
		Int("count", len(runs)).
		Msg("Patrol run history saved")
	return nil
}

// LoadPatrolRunHistory loads patrol run history from disk
func (c *ConfigPersistence) LoadPatrolRunHistory() (*PatrolRunHistoryData, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.aiPatrolRunsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty data if file doesn't exist
			return &PatrolRunHistoryData{
				Version: 1,
				Runs:    make([]PatrolRunRecord, 0),
			}, nil
		}
		return nil, err
	}

	var historyData PatrolRunHistoryData
	if err := json.Unmarshal(data, &historyData); err != nil {
		log.Error().Err(err).Str("file", c.aiPatrolRunsFile).Msg("Failed to parse patrol run history file")
		// Return empty data on parse error rather than failing
		return &PatrolRunHistoryData{
			Version: 1,
			Runs:    make([]PatrolRunRecord, 0),
		}, nil
	}

	if historyData.Runs == nil {
		historyData.Runs = make([]PatrolRunRecord, 0)
	}

	log.Info().
		Str("file", c.aiPatrolRunsFile).
		Int("count", len(historyData.Runs)).
		Time("last_saved", historyData.LastSaved).
		Msg("Patrol run history loaded")
	return &historyData, nil
}

// LoadSystemSettings loads system settings from file
func (c *ConfigPersistence) LoadSystemSettings() (*SystemSettings, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	data, err := os.ReadFile(c.systemFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return nil if file doesn't exist - let env vars take precedence
			return nil, nil
		}
		return nil, err
	}

	settings := DefaultSystemSettings()
	if settings == nil {
		settings = &SystemSettings{}
	}
	if err := json.Unmarshal(data, settings); err != nil {
		return nil, err
	}

	log.Info().Str("file", c.systemFile).Msg("System settings loaded")
	return settings, nil
}

// updateEnvFile updates the .env file with new system settings
func (c *ConfigPersistence) updateEnvFile(envFile string, settings SystemSettings) error {
	// Check if .env file exists
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// File doesn't exist, nothing to update
		return nil
	}

	// Read the existing .env file content
	existingContent, err := os.ReadFile(envFile)
	if err != nil {
		return err
	}

	// Read the existing .env file
	file, err := os.Open(envFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip legacy POLLING_INTERVAL lines - replaced by system.json PVE_POLLING_INTERVAL
		if strings.HasPrefix(line, "POLLING_INTERVAL=") {
			continue
		} else if strings.HasPrefix(line, "UPDATE_CHANNEL=") && settings.UpdateChannel != "" {
			lines = append(lines, fmt.Sprintf("UPDATE_CHANNEL=%s", settings.UpdateChannel))
		} else if strings.HasPrefix(line, "AUTO_UPDATE_ENABLED=") {
			// Always update AUTO_UPDATE_ENABLED when the line exists
			lines = append(lines, fmt.Sprintf("AUTO_UPDATE_ENABLED=%t", settings.AutoUpdateEnabled))
		} else if strings.HasPrefix(line, "AUTO_UPDATE_CHECK_INTERVAL=") && settings.AutoUpdateCheckInterval > 0 {
			lines = append(lines, fmt.Sprintf("AUTO_UPDATE_CHECK_INTERVAL=%d", settings.AutoUpdateCheckInterval))
		} else {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Note: legacy POLLING_INTERVAL is deprecated and no longer written

	// Build the new content
	content := strings.Join(lines, "\n")
	if len(lines) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	// Only write if content actually changed
	// This prevents unnecessary .env rewrites that trigger the config watcher,
	// which can cause API tokens to be overwritten (related to #685)
	if string(existingContent) == content {
		log.Debug().Str("file", envFile).Msg("Skipping .env update - content unchanged")
		return nil
	}

	// Write to temp file first
	tempFile := envFile + ".tmp"
	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		return err
	}

	// Atomic rename
	return os.Rename(tempFile, envFile)
}

// IsEncryptionEnabled returns whether the config persistence has encryption enabled
func (c *ConfigPersistence) IsEncryptionEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.crypto != nil
}

// cleanupOldBackups removes old backup files, keeping only the most recent N backups
func (c *ConfigPersistence) cleanupOldBackups(pattern string) {
	// Use filepath.Glob to find all backup files matching the pattern
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Warn().Err(err).Str("pattern", pattern).Msg("Failed to find backup files for cleanup")
		return
	}

	// Keep only the last 10 backups
	const maxBackups = 10
	if len(matches) <= maxBackups {
		return
	}

	// Sort by modification time (oldest first)
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var files []fileInfo
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: match, modTime: info.ModTime()})
	}

	// Sort oldest first using efficient sort
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// Delete oldest backups (keep last 10)
	toDelete := len(files) - maxBackups
	for i := 0; i < toDelete; i++ {
		if err := os.Remove(files[i].path); err != nil {
			log.Warn().Err(err).Str("file", files[i].path).Msg("Failed to delete old backup")
		} else {
			log.Debug().Str("file", files[i].path).Msg("Deleted old backup")
		}
	}
}

// LoadGuestMetadata loads all guest metadata from disk (for AI context)
func (c *ConfigPersistence) LoadGuestMetadata() (map[string]*GuestMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := filepath.Join(c.configDir, "guest_metadata.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*GuestMetadata), nil
		}
		return nil, err
	}

	var metadata map[string]*GuestMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}

// LoadDockerMetadata loads all docker metadata from disk (for AI context)
func (c *ConfigPersistence) LoadDockerMetadata() (map[string]*DockerMetadata, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filePath := filepath.Join(c.configDir, "docker_metadata.json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*DockerMetadata), nil
		}
		return nil, err
	}

	// Try versioned format first
	var fileData struct {
		Containers map[string]*DockerMetadata `json:"containers,omitempty"`
	}
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, err
	}

	if fileData.Containers != nil {
		return fileData.Containers, nil
	}

	// Fall back to legacy format (direct map)
	var metadata map[string]*DockerMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, err
	}

	return metadata, nil
}
