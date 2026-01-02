package ai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// AlertTriggeredAnalyzer handles AI analysis triggered by firing alerts
// This provides token-efficient, real-time AI insights on specific resources
type AlertTriggeredAnalyzer struct {
	mu sync.RWMutex

	patrolService *PatrolService
	stateProvider StateProvider
	enabled       bool

	// Cooldown to prevent analyzing the same resource repeatedly
	lastAnalyzed map[string]time.Time
	cooldown     time.Duration

	// Track pending analyses to deduplicate concurrent alerts
	pending map[string]bool

	// Cleanup goroutine management
	cleanupTicker *time.Ticker
	stopCh        chan struct{}
}

// NewAlertTriggeredAnalyzer creates a new alert-triggered analyzer
func NewAlertTriggeredAnalyzer(patrolService *PatrolService, stateProvider StateProvider) *AlertTriggeredAnalyzer {
	return &AlertTriggeredAnalyzer{
		patrolService: patrolService,
		stateProvider: stateProvider,
		enabled:       false,
		lastAnalyzed:  make(map[string]time.Time),
		cooldown:      5 * time.Minute, // Don't re-analyze the same resource within 5 minutes
		pending:       make(map[string]bool),
		stopCh:        make(chan struct{}),
	}
}

// Start begins the background cleanup goroutine
func (a *AlertTriggeredAnalyzer) Start() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cleanupTicker != nil {
		return // Already started
	}

	ticker := time.NewTicker(30 * time.Minute)
	a.cleanupTicker = ticker
	stopCh := a.stopCh

	go func() {
		for {
			select {
			case <-ticker.C:
				a.CleanupOldCooldowns()
			case <-stopCh:
				return
			}
		}
	}()
	log.Debug().Msg("Alert-triggered analyzer cleanup goroutine started")
}

// Stop stops the background cleanup goroutine
func (a *AlertTriggeredAnalyzer) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cleanupTicker != nil {
		a.cleanupTicker.Stop()
		a.cleanupTicker = nil
	}
	select {
	case <-a.stopCh:
		// Already closed
	default:
		close(a.stopCh)
	}
	a.stopCh = make(chan struct{}) // Reset for potential restart
	log.Debug().Msg("Alert-triggered analyzer cleanup goroutine stopped")
}

// SetEnabled enables or disables alert-triggered analysis
func (a *AlertTriggeredAnalyzer) SetEnabled(enabled bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.enabled = enabled
	log.Info().Bool("enabled", enabled).Msg("Alert-triggered AI analysis setting updated")
}

// IsEnabled returns whether alert-triggered analysis is enabled
func (a *AlertTriggeredAnalyzer) IsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.enabled
}

// OnAlertFired is called when an alert fires - triggers AI analysis of the affected resource
func (a *AlertTriggeredAnalyzer) OnAlertFired(alert *alerts.Alert) {
	if alert == nil {
		return
	}

	a.mu.Lock()
	if !a.enabled {
		a.mu.Unlock()
		return
	}

	// Create a resource key for deduplication
	resourceKey := a.resourceKeyFromAlert(alert)
	if resourceKey == "" {
		a.mu.Unlock()
		log.Debug().
			Str("alertID", alert.ID).
			Str("type", alert.Type).
			Msg("Cannot determine resource key for alert, skipping AI analysis")
		return
	}

	// Check cooldown
	if lastTime, exists := a.lastAnalyzed[resourceKey]; exists {
		if time.Since(lastTime) < a.cooldown {
			a.mu.Unlock()
			log.Debug().
				Str("resourceKey", resourceKey).
				Str("alertID", alert.ID).
				Dur("cooldownRemaining", a.cooldown-time.Since(lastTime)).
				Msg("Resource recently analyzed, skipping due to cooldown")
			return
		}
	}

	// Check for pending analysis
	if a.pending[resourceKey] {
		a.mu.Unlock()
		log.Debug().
			Str("resourceKey", resourceKey).
			Str("alertID", alert.ID).
			Msg("Analysis already pending for resource, skipping duplicate")
		return
	}

	// Mark as pending
	a.pending[resourceKey] = true
	a.mu.Unlock()

	// Run analysis in background
	go a.analyzeResource(alert, resourceKey)
}

// analyzeResource performs AI analysis on the resource associated with an alert
func (a *AlertTriggeredAnalyzer) analyzeResource(alert *alerts.Alert, resourceKey string) {
	defer func() {
		a.mu.Lock()
		delete(a.pending, resourceKey)
		a.lastAnalyzed[resourceKey] = time.Now()
		a.mu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Info().
		Str("alertID", alert.ID).
		Str("type", alert.Type).
		Str("resource", alert.ResourceName).
		Str("resourceKey", resourceKey).
		Float64("value", alert.Value).
		Float64("threshold", alert.Threshold).
		Msg("Starting AI analysis triggered by alert")

	startTime := time.Now()

	// Determine what type of resource this is and analyze it
	findings := a.analyzeResourceByAlert(ctx, alert)

	duration := time.Since(startTime)

	if len(findings) > 0 {
		log.Info().
			Str("alertID", alert.ID).
			Str("resourceKey", resourceKey).
			Int("findingsCount", len(findings)).
			Dur("duration", duration).
			Msg("Alert-triggered AI analysis completed with findings")

		// Add findings to the patrol service's findings store
		if a.patrolService != nil && a.patrolService.findings != nil {
			for _, finding := range findings {
				// Link finding to the triggering alert
				finding.AlertID = alert.ID
				a.patrolService.findings.Add(finding)
			}
		}
	} else {
		log.Debug().
			Str("alertID", alert.ID).
			Str("resourceKey", resourceKey).
			Dur("duration", duration).
			Msg("Alert-triggered AI analysis completed with no additional findings")
	}

	if a.patrolService != nil && a.patrolService.aiService != nil {
		summary := "Alert-triggered AI analysis completed"
		if len(findings) > 0 {
			summary = fmt.Sprintf("Alert-triggered AI analysis found %d findings", len(findings))
		}
		a.patrolService.aiService.RecordIncidentAnalysis(alert.ID, summary, map[string]interface{}{
			"findings": len(findings),
			"duration": duration.String(),
		})
	}
}

// analyzeResourceByAlert determines the resource type from the alert and analyzes it
func (a *AlertTriggeredAnalyzer) analyzeResourceByAlert(ctx context.Context, alert *alerts.Alert) []*Finding {
	if a.patrolService == nil {
		return nil
	}

	// Parse alert type to determine what kind of resource this is
	alertType := strings.ToLower(alert.Type)

	switch {
	// Node alerts
	case strings.HasPrefix(alertType, "node") ||
		alertType == "cpu" && strings.Contains(alert.ResourceID, "/node/") ||
		alertType == "memory" && strings.Contains(alert.ResourceID, "/node/"):
		return a.analyzeNodeFromAlert(ctx, alert)

	// Guest (VM/Container) alerts
	case strings.Contains(alertType, "container") ||
		strings.Contains(alertType, "vm") ||
		strings.HasPrefix(alertType, "qemu") ||
		strings.HasPrefix(alertType, "lxc") ||
		strings.Contains(alert.ResourceID, "/qemu/") ||
		strings.Contains(alert.ResourceID, "/lxc/"):
		return a.analyzeGuestFromAlert(ctx, alert)

	// Docker alerts
	case strings.Contains(alertType, "docker"):
		return a.analyzeDockerFromAlert(ctx, alert)

	// Storage alerts
	case strings.Contains(alertType, "storage") ||
		strings.HasSuffix(alertType, "-usage"):
		return a.analyzeStorageFromAlert(ctx, alert)

	// Generic CPU/Memory/Disk alerts - try to determine from resource ID
	case alertType == "cpu" || alertType == "memory" || alertType == "disk":
		return a.analyzeGenericResourceFromAlert(ctx, alert)

	default:
		log.Debug().
			Str("alertType", alertType).
			Str("resourceID", alert.ResourceID).
			Msg("Unknown alert type for targeted AI analysis, skipping")
		return nil
	}
}

// analyzeNodeFromAlert analyzes a Proxmox node triggered by an alert
func (a *AlertTriggeredAnalyzer) analyzeNodeFromAlert(_ context.Context, alert *alerts.Alert) []*Finding {
	if a.stateProvider == nil {
		return nil
	}

	state := a.stateProvider.GetState()

	// Find the node - first try ResourceID, then ResourceName
	var targetNode *models.Node
	for i := range state.Nodes {
		node := &state.Nodes[i]
		if node.ID == alert.ResourceID || node.Name == alert.ResourceName || node.Name == alert.Node {
			targetNode = node
			break
		}
	}

	if targetNode == nil {
		log.Warn().
			Str("alertID", alert.ID).
			Str("resourceID", alert.ResourceID).
			Str("resourceName", alert.ResourceName).
			Msg("Could not find node for alert-triggered analysis")
		return nil
	}

	// Use patrol service's node analysis
	return a.patrolService.analyzeNode(*targetNode)
}

// analyzeGuestFromAlert analyzes a VM/Container triggered by an alert
func (a *AlertTriggeredAnalyzer) analyzeGuestFromAlert(_ context.Context, alert *alerts.Alert) []*Finding {
	if a.stateProvider == nil {
		return nil
	}

	state := a.stateProvider.GetState()

	// Check VMs
	for _, vm := range state.VMs {
		if vm.ID == alert.ResourceID || vm.Name == alert.ResourceName {
			var lastBackup *time.Time
			if !vm.LastBackup.IsZero() {
				lastBackup = &vm.LastBackup
			}
			return a.patrolService.analyzeGuest(
				vm.ID, vm.Name, "vm", vm.Node, vm.Status,
				vm.CPU, vm.Memory.Usage, vm.Disk.Usage,
				lastBackup, vm.Template,
			)
		}
	}

	// Check containers
	for _, ct := range state.Containers {
		if ct.ID == alert.ResourceID || ct.Name == alert.ResourceName {
			var lastBackup *time.Time
			if !ct.LastBackup.IsZero() {
				lastBackup = &ct.LastBackup
			}
			return a.patrolService.analyzeGuest(
				ct.ID, ct.Name, "container", ct.Node, ct.Status,
				ct.CPU, ct.Memory.Usage, ct.Disk.Usage,
				lastBackup, ct.Template,
			)
		}
	}

	log.Warn().
		Str("alertID", alert.ID).
		Str("resourceID", alert.ResourceID).
		Msg("Could not find guest for alert-triggered analysis")
	return nil
}

// analyzeDockerFromAlert analyzes a Docker container/host triggered by an alert
func (a *AlertTriggeredAnalyzer) analyzeDockerFromAlert(_ context.Context, alert *alerts.Alert) []*Finding {
	if a.stateProvider == nil {
		return nil
	}

	state := a.stateProvider.GetState()

	// Try to find the Docker host or container
	// Containers are nested inside DockerHosts
	for _, dh := range state.DockerHosts {
		// Check if this is a host alert
		if dh.ID == alert.ResourceID || dh.Hostname == alert.ResourceName {
			return a.patrolService.analyzeDockerHost(dh)
		}

		// Check containers within this host
		for _, container := range dh.Containers {
			if container.ID == alert.ResourceID || container.Name == alert.ResourceName {
				return a.patrolService.analyzeDockerHost(dh)
			}
		}
	}

	log.Warn().
		Str("alertID", alert.ID).
		Str("resourceID", alert.ResourceID).
		Msg("Could not find Docker resource for alert-triggered analysis")
	return nil
}

// analyzeStorageFromAlert analyzes a storage resource triggered by an alert
func (a *AlertTriggeredAnalyzer) analyzeStorageFromAlert(_ context.Context, alert *alerts.Alert) []*Finding {
	if a.stateProvider == nil {
		return nil
	}

	state := a.stateProvider.GetState()

	// Storage is at the top level of StateSnapshot
	for _, storage := range state.Storage {
		if storage.ID == alert.ResourceID || storage.Name == alert.ResourceName {
			return a.patrolService.analyzeStorage(storage)
		}
	}

	log.Warn().
		Str("alertID", alert.ID).
		Str("resourceID", alert.ResourceID).
		Msg("Could not find storage for alert-triggered analysis")
	return nil
}

// analyzeGenericResourceFromAlert tries to determine resource type and analyze
func (a *AlertTriggeredAnalyzer) analyzeGenericResourceFromAlert(ctx context.Context, alert *alerts.Alert) []*Finding {
	// Try each resource type in order of likelihood
	resourceID := alert.ResourceID

	switch {
	case strings.Contains(resourceID, "/node/"):
		return a.analyzeNodeFromAlert(ctx, alert)
	case strings.Contains(resourceID, "/qemu/") || strings.Contains(resourceID, "/lxc/"):
		return a.analyzeGuestFromAlert(ctx, alert)
	case strings.Contains(resourceID, "docker"):
		return a.analyzeDockerFromAlert(ctx, alert)
	default:
		// Try guest first (most common), then node
		findings := a.analyzeGuestFromAlert(ctx, alert)
		if len(findings) > 0 {
			return findings
		}
		return a.analyzeNodeFromAlert(ctx, alert)
	}
}

// resourceKeyFromAlert creates a unique key for the resource in an alert
func (a *AlertTriggeredAnalyzer) resourceKeyFromAlert(alert *alerts.Alert) string {
	if alert.ResourceID != "" {
		return alert.ResourceID
	}
	if alert.ResourceName != "" && alert.Instance != "" {
		return fmt.Sprintf("%s/%s", alert.Instance, alert.ResourceName)
	}
	if alert.ResourceName != "" {
		return alert.ResourceName
	}
	return ""
}

// CleanupOldCooldowns removes expired cooldown entries to prevent memory growth
func (a *AlertTriggeredAnalyzer) CleanupOldCooldowns() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now()
	for key, lastTime := range a.lastAnalyzed {
		// Remove entries older than 1 hour
		if now.Sub(lastTime) > time.Hour {
			delete(a.lastAnalyzed, key)
		}
	}
}
