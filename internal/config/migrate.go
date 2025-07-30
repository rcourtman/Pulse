package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog/log"
)

// MigrateToUnified migrates from the old multi-file config to unified config
func MigrateToUnified(configDir string, outputPath string) error {
	if configDir == "" {
		configDir = "/etc/pulse"
	}
	if outputPath == "" {
		outputPath = "/etc/pulse/pulse.yml"
	}
	
	log.Info().
		Str("configDir", configDir).
		Str("output", outputPath).
		Msg("Migrating to unified configuration")
	
	// Start with default config
	unified := &UnifiedConfig{
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
		AutoUpdate: AutoUpdateConfig{
			Enabled: false,
			Channel: "stable",
			Time:    "03:00",
		},
	}
	
	// Load system.json if exists
	systemFile := filepath.Join(configDir, "system.json")
	if data, err := os.ReadFile(systemFile); err == nil {
		var system SystemSettings
		if err := json.Unmarshal(data, &system); err == nil {
			if system.PollingInterval > 0 {
				unified.Monitoring.PollingInterval = fmt.Sprintf("%ds", system.PollingInterval)
				log.Info().Int("seconds", system.PollingInterval).Msg("Migrated polling interval")
			}
		}
	}
	
	// Load nodes.json if exists
	nodesFile := filepath.Join(configDir, "nodes.json")
	if data, err := os.ReadFile(nodesFile); err == nil {
		var nodes NodesConfig
		if err := json.Unmarshal(data, &nodes); err == nil {
			// Convert PVE nodes
			for _, pve := range nodes.PVEInstances {
				unified.Nodes.PVE = append(unified.Nodes.PVE, PVENode{
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
				})
			}
			
			// Convert PBS nodes
			for _, pbs := range nodes.PBSInstances {
				unified.Nodes.PBS = append(unified.Nodes.PBS, PBSNode{
					Name:              pbs.Name,
					Host:              pbs.Host,
					User:              pbs.User,
					Password:          pbs.Password,
					TokenName:         pbs.TokenName,
					TokenValue:        pbs.TokenValue,
					Fingerprint:       pbs.Fingerprint,
					VerifySSL:         pbs.VerifySSL,
					MonitorBackups:    pbs.MonitorBackups,
					MonitorDatastores: pbs.MonitorDatastores,
					MonitorSyncJobs:   pbs.MonitorSyncJobs,
					MonitorVerifyJobs: pbs.MonitorVerifyJobs,
					MonitorPruneJobs:  pbs.MonitorPruneJobs,
					MonitorGarbageJobs: pbs.MonitorGarbageJobs,
				})
			}
			
			log.Info().
				Int("pve", len(unified.Nodes.PVE)).
				Int("pbs", len(unified.Nodes.PBS)).
				Msg("Migrated nodes")
		}
	}
	
	// Load alerts.json if exists
	alertsFile := filepath.Join(configDir, "alerts.json")
	if data, err := os.ReadFile(alertsFile); err == nil {
		var alertConfig alerts.AlertConfig
		if err := json.Unmarshal(data, &alertConfig); err == nil {
			unified.Alerts = alertConfig
			log.Info().Msg("Migrated alert configuration")
		}
	}
	
	// Load email.json if exists
	emailFile := filepath.Join(configDir, "email.json")
	if data, err := os.ReadFile(emailFile); err == nil {
		var emailConfig notifications.EmailConfig
		if err := json.Unmarshal(data, &emailConfig); err == nil {
			unified.Notifications.Email = emailConfig
			log.Info().Msg("Migrated email configuration")
		}
	}
	
	// Load webhooks.json if exists
	webhooksFile := filepath.Join(configDir, "webhooks.json")
	if data, err := os.ReadFile(webhooksFile); err == nil {
		var webhooks []notifications.WebhookConfig
		if err := json.Unmarshal(data, &webhooks); err == nil {
			unified.Notifications.Webhooks = webhooks
			log.Info().Int("count", len(webhooks)).Msg("Migrated webhooks")
		}
	}
	
	// Save unified config
	manager := &ConfigManager{
		config:   unified,
		filePath: outputPath,
	}
	
	if err := manager.save(); err != nil {
		return fmt.Errorf("failed to save unified config: %w", err)
	}
	
	log.Info().Str("path", outputPath).Msg("Migration completed successfully")
	
	// Create backup of old configs
	backupDir := filepath.Join(configDir, "backup-"+time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(backupDir, 0755); err == nil {
		files := []string{"system.json", "nodes.json", "alerts.json", "email.json", "webhooks.json"}
		for _, file := range files {
			src := filepath.Join(configDir, file)
			dst := filepath.Join(backupDir, file)
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(dst, data, 0644)
			}
		}
		log.Info().Str("dir", backupDir).Msg("Created backup of old configuration files")
	}
	
	return nil
}