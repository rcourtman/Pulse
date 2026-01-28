// Package unified provides a unified alert/finding system.
package unified

import (
	"path/filepath"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rs/zerolog/log"
)

// SetupConfig contains configuration for setting up the unified system
type SetupConfig struct {
	// DataDir is the directory for persistent storage
	DataDir string
	// AlertManager is the existing alerts.Manager to bridge
	AlertManager *alerts.Manager
	// PatrolTriggerFunc is called when patrol should be triggered
	PatrolTriggerFunc PatrolTriggerFunc
	// AutoEnhance enables automatic AI enhancement of threshold findings
	AutoEnhance bool
	// TriggerPatrolOnAlert triggers patrol when alerts fire/clear
	TriggerPatrolOnAlert bool
}

// SetupResult contains the initialized unified system components
type SetupResult struct {
	// Integration is the main entry point for the unified system
	Integration *Integration
	// Store is the unified findings store
	Store *UnifiedStore
	// Bridge is the alert-to-finding bridge
	Bridge *AlertBridge
	// Adapter is the alerts.Manager adapter
	Adapter *AlertManagerAdapter
}

// Setup initializes the unified alert/finding system and wires it to an existing alerts.Manager.
// This is the main entry point for integrating the unified system into an application.
//
// Example usage:
//
//	result, err := unified.Setup(unified.SetupConfig{
//	    DataDir:              dataPath,
//	    AlertManager:         alertManager,
//	    PatrolTriggerFunc:    func(resourceID, resourceType, reason, alertType string) { patrol.Trigger(resourceID, reason) },
//	    AutoEnhance:          true,
//	    TriggerPatrolOnAlert: true,
//	})
//	if err != nil {
//	    return err
//	}
//	defer result.Integration.Stop()
//
//	// Start the unified system
//	result.Integration.Start()
func Setup(cfg SetupConfig) (*SetupResult, error) {
	if cfg.DataDir == "" {
		cfg.DataDir = "."
	}

	// Create data directory for unified findings
	unifiedDataDir := filepath.Join(cfg.DataDir, "unified")

	// Create the integration with default config
	integrationCfg := DefaultIntegrationConfig(unifiedDataDir)
	integrationCfg.BridgeConfig.AutoEnhance = cfg.AutoEnhance
	integrationCfg.BridgeConfig.TriggerPatrolOnNew = cfg.TriggerPatrolOnAlert
	integrationCfg.BridgeConfig.TriggerPatrolOnClear = cfg.TriggerPatrolOnAlert

	integration := NewIntegration(integrationCfg)

	// Create adapter for the alert manager
	var adapter *AlertManagerAdapter
	if cfg.AlertManager != nil {
		adapter = NewAlertManagerAdapter(cfg.AlertManager)
		integration.SetAlertProvider(adapter)
	}

	// Set patrol trigger if provided
	if cfg.PatrolTriggerFunc != nil {
		integration.SetPatrolTrigger(cfg.PatrolTriggerFunc)
	}

	result := &SetupResult{
		Integration: integration,
		Store:       integration.GetStore(),
		Bridge:      integration.GetBridge(),
		Adapter:     adapter,
	}

	log.Info().
		Str("data_dir", unifiedDataDir).
		Bool("auto_enhance", cfg.AutoEnhance).
		Bool("trigger_patrol", cfg.TriggerPatrolOnAlert).
		Msg("Unified alert/finding system configured")

	return result, nil
}

// QuickSetup is a convenience function that sets up the unified system with minimal configuration.
// It uses sensible defaults and only requires the alert manager.
func QuickSetup(alertManager *alerts.Manager, dataDir string) (*SetupResult, error) {
	return Setup(SetupConfig{
		DataDir:              dataDir,
		AlertManager:         alertManager,
		AutoEnhance:          true,
		TriggerPatrolOnAlert: true,
	})
}

// SetupWithPatrol sets up the unified system with patrol integration.
func SetupWithPatrol(alertManager *alerts.Manager, dataDir string, patrolTrigger PatrolTriggerFunc) (*SetupResult, error) {
	return Setup(SetupConfig{
		DataDir:              dataDir,
		AlertManager:         alertManager,
		PatrolTriggerFunc:    patrolTrigger,
		AutoEnhance:          true,
		TriggerPatrolOnAlert: true,
	})
}
