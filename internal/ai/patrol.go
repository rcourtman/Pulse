package ai

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// ThresholdProvider provides user-configured alert thresholds for patrol to use
type ThresholdProvider interface {
	// GetNodeCPUThreshold returns the CPU alert trigger threshold for nodes (0-100%)
	GetNodeCPUThreshold() float64
	// GetNodeMemoryThreshold returns the memory alert trigger threshold for nodes (0-100%)
	GetNodeMemoryThreshold() float64
	// GetGuestMemoryThreshold returns the memory alert trigger threshold for guests (0-100%)
	GetGuestMemoryThreshold() float64
	// GetGuestDiskThreshold returns the disk alert trigger threshold for guests (0-100%)
	GetGuestDiskThreshold() float64
	// GetStorageThreshold returns the usage alert trigger threshold for storage (0-100%)
	GetStorageThreshold() float64
}

// PatrolThresholds holds calculated thresholds for patrol (derived from alert thresholds)
type PatrolThresholds struct {
	// Node thresholds
	NodeCPUWatch    float64 // CPU % to flag as "watch" (typically alertThreshold - 15)
	NodeCPUWarning  float64 // CPU % to flag as "warning" (typically alertThreshold - 5)
	NodeMemWatch    float64
	NodeMemWarning  float64
	// Guest thresholds (VMs/containers)
	GuestMemWatch   float64
	GuestMemWarning float64
	GuestDiskWatch  float64
	GuestDiskWarn   float64
	GuestDiskCrit   float64
	// Storage thresholds
	StorageWatch    float64
	StorageWarning  float64
	StorageCritical float64
}

// DefaultPatrolThresholds returns fallback thresholds when no provider is set
func DefaultPatrolThresholds() PatrolThresholds {
	return PatrolThresholds{
		NodeCPUWatch:    75, NodeCPUWarning: 85,
		NodeMemWatch:    75, NodeMemWarning: 85,
		GuestMemWatch:   80, GuestMemWarning: 88,
		GuestDiskWatch:  75, GuestDiskWarn: 85, GuestDiskCrit: 92,
		StorageWatch:    70, StorageWarning: 80, StorageCritical: 90,
	}
}

// CalculatePatrolThresholds derives patrol thresholds from alert thresholds
// Patrol warns ~10% BEFORE the alert fires so users can take action early
func CalculatePatrolThresholds(provider ThresholdProvider) PatrolThresholds {
	if provider == nil {
		return DefaultPatrolThresholds()
	}

	// Get user's alert thresholds
	nodeCPU := provider.GetNodeCPUThreshold()
	nodeMem := provider.GetNodeMemoryThreshold()
	guestMem := provider.GetGuestMemoryThreshold()
	guestDisk := provider.GetGuestDiskThreshold()
	storage := provider.GetStorageThreshold()

	// Calculate patrol thresholds (watch = alert-15%, warning = alert-5%)
	return PatrolThresholds{
		NodeCPUWatch:    clampThreshold(nodeCPU - 15),
		NodeCPUWarning:  clampThreshold(nodeCPU - 5),
		NodeMemWatch:    clampThreshold(nodeMem - 15),
		NodeMemWarning:  clampThreshold(nodeMem - 5),
		GuestMemWatch:   clampThreshold(guestMem - 12),
		GuestMemWarning: clampThreshold(guestMem - 5),
		GuestDiskWatch:  clampThreshold(guestDisk - 15),
		GuestDiskWarn:   clampThreshold(guestDisk - 8),
		GuestDiskCrit:   clampThreshold(guestDisk - 3),
		StorageWatch:    clampThreshold(storage - 15),
		StorageWarning:  clampThreshold(storage - 8),
		StorageCritical: clampThreshold(storage - 3),
	}
}

// clampThreshold ensures a threshold is within valid range
func clampThreshold(v float64) float64 {
	if v < 10 {
		return 10 // Never go below 10%
	}
	if v > 99 {
		return 99
	}
	return v
}

// PatrolConfig holds configuration for the AI patrol service
type PatrolConfig struct {
	// Enabled controls whether background patrol runs
	Enabled bool `json:"enabled"`
	// QuickCheckInterval is how often to do quick health checks
	QuickCheckInterval time.Duration `json:"quick_check_interval"`
	// DeepAnalysisInterval is how often to do thorough analysis
	DeepAnalysisInterval time.Duration `json:"deep_analysis_interval"`
	// AnalyzeNodes controls whether to analyze Proxmox nodes
	AnalyzeNodes bool `json:"analyze_nodes"`
	// AnalyzeGuests controls whether to analyze VMs/containers
	AnalyzeGuests bool `json:"analyze_guests"`
	// AnalyzeDocker controls whether to analyze Docker hosts
	AnalyzeDocker bool `json:"analyze_docker"`
	// AnalyzeStorage controls whether to analyze storage
	AnalyzeStorage bool `json:"analyze_storage"`
	// AnalyzePBS controls whether to analyze PBS backup servers
	AnalyzePBS bool `json:"analyze_pbs"`
	// AnalyzeHosts controls whether to analyze agent hosts (RAID, sensors)
	AnalyzeHosts bool `json:"analyze_hosts"`
}

// DefaultPatrolConfig returns sensible defaults
func DefaultPatrolConfig() PatrolConfig {
	return PatrolConfig{
		Enabled:              true,
		QuickCheckInterval:   15 * time.Minute,
		DeepAnalysisInterval: 6 * time.Hour,
		AnalyzeNodes:         true,
		AnalyzeGuests:        true,
		AnalyzeDocker:        true,
		AnalyzeStorage:       true,
		AnalyzePBS:           true,
		AnalyzeHosts:         true,
	}
}

// PatrolStatus represents the current state of the patrol service
type PatrolStatus struct {
	Running          bool          `json:"running"`
	LastPatrolAt     *time.Time    `json:"last_patrol_at,omitempty"`
	LastDeepAnalysis *time.Time    `json:"last_deep_analysis_at,omitempty"`
	NextPatrolAt     *time.Time    `json:"next_patrol_at,omitempty"`
	LastDuration     time.Duration `json:"last_duration_ms"`
	ResourcesChecked int           `json:"resources_checked"`
	FindingsCount    int           `json:"findings_count"`
	ErrorCount       int           `json:"error_count"`
	Healthy          bool          `json:"healthy"`
}

// PatrolService runs background AI analysis of infrastructure
type PatrolService struct {
	mu sync.RWMutex

	aiService         *Service
	stateProvider     StateProvider
	thresholdProvider ThresholdProvider
	config            PatrolConfig
	findings          *FindingsStore

	// Cached thresholds (recalculated when thresholdProvider changes)
	thresholds PatrolThresholds

	// Runtime state
	running          bool
	stopCh           chan struct{}
	lastPatrol       time.Time
	lastDeepAnalysis time.Time
	lastDuration     time.Duration
	resourcesChecked int
	errorCount       int
}

// NewPatrolService creates a new patrol service
func NewPatrolService(aiService *Service, stateProvider StateProvider) *PatrolService {
	return &PatrolService{
		aiService:     aiService,
		stateProvider: stateProvider,
		config:        DefaultPatrolConfig(),
		findings:      NewFindingsStore(),
		thresholds:    DefaultPatrolThresholds(),
		stopCh:        make(chan struct{}),
	}
}

// SetConfig updates the patrol configuration
func (p *PatrolService) SetConfig(cfg PatrolConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = cfg
}

// SetThresholdProvider sets the provider for user-configured alert thresholds
// This allows patrol to warn BEFORE alerts fire
func (p *PatrolService) SetThresholdProvider(provider ThresholdProvider) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.thresholdProvider = provider
	p.thresholds = CalculatePatrolThresholds(provider)
	log.Debug().
		Float64("storageWatch", p.thresholds.StorageWatch).
		Float64("storageWarning", p.thresholds.StorageWarning).
		Float64("storageCritical", p.thresholds.StorageCritical).
		Msg("Patrol thresholds updated from alert config")
}

// SetFindingsPersistence enables findings persistence (load from and save to disk)
// This should be called before Start() to load any existing findings
func (p *PatrolService) SetFindingsPersistence(persistence FindingsPersistence) error {
	p.mu.Lock()
	findings := p.findings
	p.mu.Unlock()

	if findings != nil && persistence != nil {
		if err := findings.SetPersistence(persistence); err != nil {
			return err
		}
		log.Info().Msg("AI Patrol findings persistence enabled")
	}
	return nil
}

// GetConfig returns the current patrol configuration
func (p *PatrolService) GetConfig() PatrolConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// GetFindings returns the findings store
func (p *PatrolService) GetFindings() *FindingsStore {
	return p.findings
}

// GetStatus returns the current patrol status
func (p *PatrolService) GetStatus() PatrolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	status := PatrolStatus{
		Running:          p.running,
		LastDuration:     p.lastDuration,
		ResourcesChecked: p.resourcesChecked,
		FindingsCount:    len(p.findings.GetActive(FindingSeverityInfo)),
		ErrorCount:       p.errorCount,
	}

	if !p.lastPatrol.IsZero() {
		status.LastPatrolAt = &p.lastPatrol
	}
	if !p.lastDeepAnalysis.IsZero() {
		status.LastDeepAnalysis = &p.lastDeepAnalysis
	}

	if p.running && p.config.QuickCheckInterval > 0 {
		next := p.lastPatrol.Add(p.config.QuickCheckInterval)
		status.NextPatrolAt = &next
	}

	summary := p.findings.GetSummary()
	status.Healthy = summary.IsHealthy()

	return status
}

// Start begins the background patrol loop
func (p *PatrolService) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.mu.Unlock()

	log.Info().
		Dur("quick_interval", p.config.QuickCheckInterval).
		Dur("deep_interval", p.config.DeepAnalysisInterval).
		Msg("Starting AI Patrol Service")

	go p.patrolLoop(ctx)
}

// Stop stops the patrol service
func (p *PatrolService) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	close(p.stopCh)
	p.mu.Unlock()

	log.Info().Msg("Stopping AI Patrol Service")
}

// patrolLoop is the main background loop
func (p *PatrolService) patrolLoop(ctx context.Context) {
	// Run initial quick patrol shortly after startup
	initialDelay := 30 * time.Second
	select {
	case <-time.After(initialDelay):
		p.runPatrol(ctx, false)
	case <-p.stopCh:
		return
	case <-ctx.Done():
		return
	}

	quickTicker := time.NewTicker(p.config.QuickCheckInterval)
	defer quickTicker.Stop()

	// Deep analysis ticker (if configured)
	var deepTicker *time.Ticker
	if p.config.DeepAnalysisInterval > 0 {
		deepTicker = time.NewTicker(p.config.DeepAnalysisInterval)
		defer deepTicker.Stop()
	}

	for {
		select {
		case <-quickTicker.C:
			p.runPatrol(ctx, false)

		case <-func() <-chan time.Time {
			if deepTicker != nil {
				return deepTicker.C
			}
			return make(chan time.Time) // never fires
		}():
			p.runPatrol(ctx, true)

		case <-p.stopCh:
			return

		case <-ctx.Done():
			return
		}
	}
}

// runPatrol executes a patrol run
func (p *PatrolService) runPatrol(ctx context.Context, deep bool) {
	p.mu.RLock()
	cfg := p.config
	p.mu.RUnlock()

	if !cfg.Enabled {
		return
	}

	// Check if AI service is enabled
	if p.aiService == nil || !p.aiService.IsEnabled() {
		log.Debug().Msg("AI Patrol: AI service not enabled, skipping patrol")
		return
	}

	start := time.Now()
	patrolType := "quick"
	if deep {
		patrolType = "deep"
	}

	log.Debug().Str("type", patrolType).Msg("AI Patrol: Starting patrol run")

	var resourceCount int
	var errors int

	// Get current state
	if p.stateProvider == nil {
		log.Warn().Msg("AI Patrol: No state provider available")
		return
	}

	state := p.stateProvider.GetState()

	// Analyze nodes
	if cfg.AnalyzeNodes {
		for _, node := range state.Nodes {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			findings := p.analyzeNode(node, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}
	}

	// Analyze VMs and containers
	if cfg.AnalyzeGuests {
		for _, vm := range state.VMs {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			// Calculate usage percentages from Memory/Disk structs
			var memUsage, diskUsage float64
			if vm.Memory.Total > 0 {
				memUsage = float64(vm.Memory.Used) / float64(vm.Memory.Total)
			}
			if vm.Disk.Total > 0 {
				diskUsage = float64(vm.Disk.Used) / float64(vm.Disk.Total)
			}
			// Handle LastBackup - pass nil if zero time
			var lastBackup *time.Time
			if !vm.LastBackup.IsZero() {
				t := vm.LastBackup
				lastBackup = &t
			}
			findings := p.analyzeGuest(vm.ID, vm.Name, "vm", vm.Node, vm.Status,
				vm.CPU, memUsage, diskUsage, lastBackup, vm.Template, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}

		for _, ct := range state.Containers {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			// Calculate usage percentages from Memory/Disk structs
			var memUsage, diskUsage float64
			if ct.Memory.Total > 0 {
				memUsage = float64(ct.Memory.Used) / float64(ct.Memory.Total)
			}
			if ct.Disk.Total > 0 {
				diskUsage = float64(ct.Disk.Used) / float64(ct.Disk.Total)
			}
			// Handle LastBackup - pass nil if zero time
			var lastBackup *time.Time
			if !ct.LastBackup.IsZero() {
				t := ct.LastBackup
				lastBackup = &t
			}
			findings := p.analyzeGuest(ct.ID, ct.Name, "container", ct.Node, ct.Status,
				ct.CPU, memUsage, diskUsage, lastBackup, ct.Template, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}
	}

	// Analyze Docker hosts
	if cfg.AnalyzeDocker {
		for _, dh := range state.DockerHosts {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			findings := p.analyzeDockerHost(dh, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}
	}

	// Analyze storage
	if cfg.AnalyzeStorage {
		for _, st := range state.Storage {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			findings := p.analyzeStorage(st, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}
	}

	// Analyze PBS instances (backup servers)
	if cfg.AnalyzePBS {
		for _, pbs := range state.PBSInstances {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			findings := p.analyzePBSInstance(pbs, state.PBSBackups, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}
	}

	// Analyze agent hosts (RAID, sensors)
	if cfg.AnalyzeHosts {
		for _, host := range state.Hosts {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resourceCount++
			findings := p.analyzeHost(host, deep)
			for _, f := range findings {
				if p.findings.Add(f) {
					log.Info().
						Str("finding_id", f.ID).
						Str("severity", string(f.Severity)).
						Str("resource", f.ResourceName).
						Str("title", f.Title).
						Msg("AI Patrol: New finding")
				}
			}
		}
	}

	// Auto-resolve findings that weren't seen in this patrol run
	p.autoResolveStaleFindings(start)

	// Cleanup old resolved findings
	cleaned := p.findings.Cleanup(24 * time.Hour)
	if cleaned > 0 {
		log.Debug().Int("cleaned", cleaned).Msg("AI Patrol: Cleaned up old findings")
	}

	duration := time.Since(start)

	p.mu.Lock()
	p.lastPatrol = time.Now()
	p.lastDuration = duration
	p.resourcesChecked = resourceCount
	p.errorCount = errors
	if deep {
		p.lastDeepAnalysis = time.Now()
	}
	p.mu.Unlock()

	summary := p.findings.GetSummary()
	log.Info().
		Str("type", patrolType).
		Dur("duration", duration).
		Int("resources", resourceCount).
		Int("critical", summary.Critical).
		Int("warning", summary.Warning).
		Int("watch", summary.Watch).
		Msg("AI Patrol: Completed patrol run")
}

// generateFindingID creates a stable ID for a finding based on resource and issue
func generateFindingID(resourceID, category, issue string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", resourceID, category, issue)))
	return fmt.Sprintf("%x", hash[:8])
}

// analyzeNode checks a Proxmox node for issues
func (p *PatrolService) analyzeNode(node models.Node, deep bool) []*Finding {
	var findings []*Finding

	// Calculate memory usage from Memory struct (as percentage 0-100)
	var memUsagePct float64
	if node.Memory.Total > 0 {
		memUsagePct = float64(node.Memory.Used) / float64(node.Memory.Total) * 100
	}

	// CPU as percentage (node.CPU is 0-1 ratio from Proxmox)
	cpuPct := node.CPU * 100

	// Check for offline nodes
	if node.Status == "offline" || node.Status == "unknown" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(node.ID, "reliability", "offline"),
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     node.ID,
			ResourceName:   node.Name,
			ResourceType:   "node",
			Title:          "Node offline",
			Description:    fmt.Sprintf("Node '%s' is not responding", node.Name),
			Recommendation: "Check network connectivity, SSH access, and Proxmox services on the node",
		})
	}

	// High CPU - use dynamic thresholds from user's alert config
	if cpuPct > p.thresholds.NodeCPUWatch {
		severity := FindingSeverityWatch
		if cpuPct > p.thresholds.NodeCPUWarning {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(node.ID, "performance", "high-cpu"),
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     node.ID,
			ResourceName:   node.Name,
			ResourceType:   "node",
			Title:          "High CPU usage",
			Description:    fmt.Sprintf("Node '%s' CPU at %.0f%%", node.Name, cpuPct),
			Recommendation: "Check which VMs/containers are consuming CPU. Consider load balancing.",
			Evidence:       fmt.Sprintf("CPU: %.1f%%", cpuPct),
		})
	}

	// High memory - use dynamic thresholds
	if memUsagePct > p.thresholds.NodeMemWatch {
		severity := FindingSeverityWatch
		if memUsagePct > p.thresholds.NodeMemWarning {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(node.ID, "performance", "high-memory"),
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     node.ID,
			ResourceName:   node.Name,
			ResourceType:   "node",
			Title:          "High memory usage",
			Description:    fmt.Sprintf("Node '%s' memory at %.0f%%", node.Name, memUsagePct),
			Recommendation: "Consider migrating some VMs to other nodes or increasing node RAM",
			Evidence:       fmt.Sprintf("Memory: %.1f%%", memUsagePct),
		})
	}

	return findings
}

// analyzeGuest checks a VM or container for issues
func (p *PatrolService) analyzeGuest(id, name, guestType, node, status string,
	cpu, memUsage, diskUsage float64, lastBackup *time.Time, template, deep bool) []*Finding {
	var findings []*Finding

	// Skip templates
	if template {
		return findings
	}

	// Convert ratios to percentages for comparison with thresholds
	memPct := memUsage * 100
	diskPct := diskUsage * 100

	// High memory (sustained) - use dynamic thresholds
	if memPct > p.thresholds.GuestMemWatch {
		severity := FindingSeverityWatch
		if memPct > p.thresholds.GuestMemWarning {
			severity = FindingSeverityWarning
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(id, "performance", "high-memory"),
			Severity:       severity,
			Category:       FindingCategoryPerformance,
			ResourceID:     id,
			ResourceName:   name,
			ResourceType:   guestType,
			Node:           node,
			Title:          "High memory usage",
			Description:    fmt.Sprintf("'%s' memory at %.0f%% - risk of OOM", name, memPct),
			Recommendation: "Consider increasing allocated RAM or investigating memory-hungry processes",
			Evidence:       fmt.Sprintf("Memory: %.1f%%", memPct),
		})
	}

	// High disk usage - use dynamic thresholds
	if diskPct > p.thresholds.GuestDiskWatch {
		severity := FindingSeverityWatch
		if diskPct > p.thresholds.GuestDiskWarn {
			severity = FindingSeverityWarning
		}
		if diskPct > p.thresholds.GuestDiskCrit {
			severity = FindingSeverityCritical
		}
		findings = append(findings, &Finding{
			ID:             generateFindingID(id, "capacity", "high-disk"),
			Severity:       severity,
			Category:       FindingCategoryCapacity,
			ResourceID:     id,
			ResourceName:   name,
			ResourceType:   guestType,
			Node:           node,
			Title:          "High disk usage",
			Description:    fmt.Sprintf("'%s' disk at %.0f%%", name, diskPct),
			Recommendation: "Clean up old files, logs, or docker images. Consider expanding disk.",
			Evidence:       fmt.Sprintf("Disk: %.1f%%", diskPct),
		})
	}

	// Backup check (only for running guests)
	if status == "running" && lastBackup != nil {
		daysSinceBackup := time.Since(*lastBackup).Hours() / 24
		if daysSinceBackup > 14 {
			severity := FindingSeverityWatch
			if daysSinceBackup > 30 {
				severity = FindingSeverityWarning
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(id, "backup", "stale"),
				Severity:       severity,
				Category:       FindingCategoryBackup,
				ResourceID:     id,
				ResourceName:   name,
				ResourceType:   guestType,
				Node:           node,
				Title:          "Backup overdue",
				Description:    fmt.Sprintf("'%s' hasn't been backed up in %.0f days", name, daysSinceBackup),
				Recommendation: "Check backup job configuration or run a manual backup",
				Evidence:       fmt.Sprintf("Last backup: %s", lastBackup.Format("2006-01-02")),
			})
		}
	} else if status == "running" && lastBackup == nil {
		findings = append(findings, &Finding{
			ID:             generateFindingID(id, "backup", "never"),
			Severity:       FindingSeverityWarning,
			Category:       FindingCategoryBackup,
			ResourceID:     id,
			ResourceName:   name,
			ResourceType:   guestType,
			Node:           node,
			Title:          "Never backed up",
			Description:    fmt.Sprintf("'%s' has no backup history", name),
			Recommendation: "Configure backup job for this guest",
		})
	}

	return findings
}

// analyzeDockerHost checks a Docker host for issues
func (p *PatrolService) analyzeDockerHost(host models.DockerHost, deep bool) []*Finding {
	var findings []*Finding

	hostName := host.Hostname
	if host.DisplayName != "" {
		hostName = host.DisplayName
	}

	// Host offline
	if host.Status != "online" && host.Status != "connected" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "reliability", "offline"),
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "docker_host",
			Title:          "Docker host offline",
			Description:    fmt.Sprintf("Docker host '%s' is not responding", hostName),
			Recommendation: "Check network connectivity and docker-agent service",
		})
	}

	// Check individual containers
	for _, c := range host.Containers {
		// Restarting containers
		if c.State == "restarting" || c.RestartCount > 3 {
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "reliability", "restart-loop"),
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryReliability,
				ResourceID:     c.ID,
				ResourceName:   c.Name,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "Container restart loop",
				Description:    fmt.Sprintf("Container '%s' has restarted %d times", c.Name, c.RestartCount),
				Recommendation: "Check container logs: docker logs " + c.Name,
				Evidence:       fmt.Sprintf("State: %s, Restarts: %d", c.State, c.RestartCount),
			})
		}

		// High memory containers
		if c.MemoryPercent > 90 {
			findings = append(findings, &Finding{
				ID:             generateFindingID(c.ID, "performance", "high-memory"),
				Severity:       FindingSeverityWatch,
				Category:       FindingCategoryPerformance,
				ResourceID:     c.ID,
				ResourceName:   c.Name,
				ResourceType:   "docker_container",
				Node:           hostName,
				Title:          "High memory usage",
				Description:    fmt.Sprintf("Container '%s' using %.0f%% of allocated memory", c.Name, c.MemoryPercent),
				Recommendation: "Consider increasing container memory limit",
				Evidence:       fmt.Sprintf("Memory: %.1f%%", c.MemoryPercent),
			})
		}
	}

	return findings
}

// analyzeStorage checks storage for issues
func (p *PatrolService) analyzeStorage(storage models.Storage, deep bool) []*Finding {
	var findings []*Finding

	// Note: storage.Usage is already a percentage (0-100, e.g. 85.5 means 85.5%)
	// If Usage is 0 but we have bytes data, calculate it as percentage
	usage := storage.Usage
	if usage == 0 && storage.Total > 0 {
		usage = float64(storage.Used) / float64(storage.Total) * 100
	}

	// High storage usage - use dynamic thresholds from user's alert config
	if usage > p.thresholds.StorageWatch {
		severity := FindingSeverityWatch
		if usage > p.thresholds.StorageWarning {
			severity = FindingSeverityWarning
		}
		if usage > p.thresholds.StorageCritical {
			severity = FindingSeverityCritical
		}

		findings = append(findings, &Finding{
			ID:             generateFindingID(storage.ID, "capacity", "high-usage"),
			Severity:       severity,
			Category:       FindingCategoryCapacity,
			ResourceID:     storage.ID,
			ResourceName:   storage.Name,
			ResourceType:   "storage",
			Title:          "Storage filling up",
			Description:    fmt.Sprintf("Storage '%s' at %.0f%% capacity", storage.Name, usage),
			Recommendation: "Clean up old backups, snapshots, or unused disk images",
			Evidence:       fmt.Sprintf("Usage: %.1f%%", usage),
		})
	}

	return findings
}

// autoResolveHealthyResources marks findings as resolved when they weren't seen in the current patrol
// patrolStartTime is used to determine which findings are stale (LastSeenAt < patrolStartTime)
func (p *PatrolService) autoResolveStaleFindings(patrolStartTime time.Time) {
	// Get all active findings and check if they're stale
	activeFindings := p.findings.GetActive(FindingSeverityInfo)
	
	for _, f := range activeFindings {
		// If the finding wasn't updated during this patrol (LastSeenAt is before patrol started),
		// it means the condition that caused it has been resolved
		if f.LastSeenAt.Before(patrolStartTime) {
			if p.findings.Resolve(f.ID, true) {
				log.Info().
					Str("finding_id", f.ID).
					Str("resource", f.ResourceName).
					Str("title", f.Title).
					Msg("AI Patrol: Auto-resolved finding")
			}
		}
	}
}

// GetFindingsForResource returns active findings for a specific resource
func (p *PatrolService) GetFindingsForResource(resourceID string) []*Finding {
	return p.findings.GetByResource(resourceID)
}

// GetFindingsSummary returns a summary of all findings
func (p *PatrolService) GetFindingsSummary() FindingsSummary {
	return p.findings.GetSummary()
}

// GetAllFindings returns all active findings sorted by severity
func (p *PatrolService) GetAllFindings() []*Finding {
	findings := p.findings.GetActive(FindingSeverityInfo)
	
	// Sort by severity (critical first) then by time
	severityOrder := map[FindingSeverity]int{
		FindingSeverityCritical: 0,
		FindingSeverityWarning:  1,
		FindingSeverityWatch:    2,
		FindingSeverityInfo:     3,
	}
	
	sort.Slice(findings, func(i, j int) bool {
		if severityOrder[findings[i].Severity] != severityOrder[findings[j].Severity] {
			return severityOrder[findings[i].Severity] < severityOrder[findings[j].Severity]
		}
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})
	
	return findings
}

// GetFindingsHistory returns all findings including resolved ones for history display
// Optionally filter by startTime
func (p *PatrolService) GetFindingsHistory(startTime *time.Time) []*Finding {
	findings := p.findings.GetAll(startTime)
	
	// Sort by detected time (newest first)
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].DetectedAt.After(findings[j].DetectedAt)
	})
	
	return findings
}

// ForcePatrol triggers an immediate patrol run
func (p *PatrolService) ForcePatrol(ctx context.Context, deep bool) {
	go p.runPatrol(ctx, deep)
}

// analyzePBSInstance checks a PBS backup server for issues
func (p *PatrolService) analyzePBSInstance(pbs models.PBSInstance, allBackups []models.PBSBackup, deep bool) []*Finding {
	var findings []*Finding

	pbsName := pbs.Name
	if pbsName == "" {
		pbsName = pbs.Host
	}

	// Check PBS connectivity
	if pbs.Status != "online" && pbs.Status != "connected" && pbs.Status != "" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(pbs.ID, "reliability", "offline"),
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     pbs.ID,
			ResourceName:   pbsName,
			ResourceType:   "pbs",
			Title:          "PBS server offline",
			Description:    fmt.Sprintf("Proxmox Backup Server '%s' is not responding", pbsName),
			Recommendation: "Check network connectivity and PBS service status",
		})
	}

	// Check each datastore capacity
	for _, ds := range pbs.Datastores {
		usage := ds.Usage
		if usage == 0 && ds.Total > 0 {
			usage = float64(ds.Used) / float64(ds.Total) * 100
		}

		// PBS datastores should trigger earlier than regular storage
		// since running out of backup space is critical
		if usage > p.thresholds.StorageWatch {
			severity := FindingSeverityWatch
			if usage > p.thresholds.StorageWarning {
				severity = FindingSeverityWarning
			}
			if usage > p.thresholds.StorageCritical {
				severity = FindingSeverityCritical
			}

			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":"+ds.Name, "capacity", "high-usage"),
				Severity:       severity,
				Category:       FindingCategoryCapacity,
				ResourceID:     pbs.ID + ":" + ds.Name,
				ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
				ResourceType:   "pbs_datastore",
				Title:          "PBS datastore filling up",
				Description:    fmt.Sprintf("Datastore '%s' on PBS '%s' at %.0f%% capacity", ds.Name, pbsName, usage),
				Recommendation: "Run garbage collection, prune old backups, or expand storage",
				Evidence:       fmt.Sprintf("Usage: %.1f%%", usage),
			})
		}

		// Check for datastore errors
		if ds.Error != "" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":"+ds.Name, "reliability", "error"),
				Severity:       FindingSeverityCritical,
				Category:       FindingCategoryReliability,
				ResourceID:     pbs.ID + ":" + ds.Name,
				ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
				ResourceType:   "pbs_datastore",
				Title:          "PBS datastore error",
				Description:    fmt.Sprintf("Datastore '%s' has an error: %s", ds.Name, ds.Error),
				Recommendation: "Check PBS server logs and datastore configuration",
				Evidence:       ds.Error,
			})
		}
	}

	// Check for backup staleness per datastore
	// Build a map of latest backup time per datastore
	datastoreLastBackup := make(map[string]time.Time)
	for _, backup := range allBackups {
		if backup.Instance != pbs.ID && backup.Instance != pbs.Name {
			continue
		}
		dsKey := backup.Datastore
		if backup.BackupTime.After(datastoreLastBackup[dsKey]) {
			datastoreLastBackup[dsKey] = backup.BackupTime
		}
	}

	for _, ds := range pbs.Datastores {
		lastBackup, hasBackups := datastoreLastBackup[ds.Name]
		
		if !hasBackups {
			// No backups found for this datastore - might be intentional (empty datastore)
			// Only warn if datastore has actual content
			if ds.Used > 0 {
				findings = append(findings, &Finding{
					ID:             generateFindingID(pbs.ID+":"+ds.Name, "backup", "no-recent"),
					Severity:       FindingSeverityWatch,
					Category:       FindingCategoryBackup,
					ResourceID:     pbs.ID + ":" + ds.Name,
					ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
					ResourceType:   "pbs_datastore",
					Title:          "No recent backup metadata",
					Description:    fmt.Sprintf("Datastore '%s' has content but no recent backup entries visible", ds.Name),
					Recommendation: "Verify backup jobs are configured and running",
				})
			}
			continue
		}

		hoursSinceBackup := time.Since(lastBackup).Hours()
		if hoursSinceBackup > 48 {
			severity := FindingSeverityWatch
			if hoursSinceBackup > 72 {
				severity = FindingSeverityWarning
			}
			if hoursSinceBackup > 168 { // 7 days
				severity = FindingSeverityCritical
			}

			findings = append(findings, &Finding{
				ID:             generateFindingID(pbs.ID+":"+ds.Name, "backup", "stale"),
				Severity:       severity,
				Category:       FindingCategoryBackup,
				ResourceID:     pbs.ID + ":" + ds.Name,
				ResourceName:   fmt.Sprintf("%s/%s", pbsName, ds.Name),
				ResourceType:   "pbs_datastore",
				Title:          "Stale backups",
				Description:    fmt.Sprintf("No backups to '%s/%s' in %.0f hours", pbsName, ds.Name, hoursSinceBackup),
				Recommendation: "Check backup job schedule and logs for failures",
				Evidence:       fmt.Sprintf("Last backup: %s", lastBackup.Format("2006-01-02 15:04")),
			})
		}
	}

	// Check backup jobs for failures (only during deep analysis)
	if deep {
		for _, job := range pbs.BackupJobs {
			if job.Status == "error" || job.Error != "" {
				findings = append(findings, &Finding{
					ID:             generateFindingID(pbs.ID+":job:"+job.ID, "backup", "job-failed"),
					Severity:       FindingSeverityWarning,
					Category:       FindingCategoryBackup,
					ResourceID:     pbs.ID + ":job:" + job.ID,
					ResourceName:   fmt.Sprintf("%s/job/%s", pbsName, job.ID),
					ResourceType:   "pbs_job",
					Title:          "Backup job failed",
					Description:    fmt.Sprintf("Backup job '%s' on PBS '%s' is failing", job.ID, pbsName),
					Recommendation: "Check PBS task logs for error details",
					Evidence:       job.Error,
				})
			}
		}

		for _, job := range pbs.VerifyJobs {
			if job.Status == "error" || job.Error != "" {
				findings = append(findings, &Finding{
					ID:             generateFindingID(pbs.ID+":verify:"+job.ID, "backup", "verify-failed"),
					Severity:       FindingSeverityWarning,
					Category:       FindingCategoryBackup,
					ResourceID:     pbs.ID + ":verify:" + job.ID,
					ResourceName:   fmt.Sprintf("%s/verify/%s", pbsName, job.ID),
					ResourceType:   "pbs_job",
					Title:          "Verify job failed",
					Description:    fmt.Sprintf("Verify job '%s' on PBS '%s' is failing", job.ID, pbsName),
					Recommendation: "Check PBS task logs - verify failures may indicate backup corruption",
					Evidence:       job.Error,
				})
			}
		}
	}

	return findings
}

// analyzeHost checks an agent host for issues (RAID, sensors, connectivity)
func (p *PatrolService) analyzeHost(host models.Host, deep bool) []*Finding {
	var findings []*Finding

	hostName := host.DisplayName
	if hostName == "" {
		hostName = host.Hostname
	}

	// Check host connectivity
	if host.Status != "online" && host.Status != "connected" && host.Status != "" {
		findings = append(findings, &Finding{
			ID:             generateFindingID(host.ID, "reliability", "offline"),
			Severity:       FindingSeverityCritical,
			Category:       FindingCategoryReliability,
			ResourceID:     host.ID,
			ResourceName:   hostName,
			ResourceType:   "host",
			Title:          "Host agent offline",
			Description:    fmt.Sprintf("Host '%s' agent is not reporting", hostName),
			Recommendation: "Check network connectivity and pulse-agent service status",
		})
	}

	// Check RAID arrays
	for _, raid := range host.RAID {
		raidName := raid.Device
		if raid.Name != "" {
			raidName = raid.Name
		}

		// Check for degraded/failed state
		switch raid.State {
		case "degraded", "DEGRADED":
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-degraded"),
				Severity:       FindingSeverityCritical,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID array degraded",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' is degraded (%d/%d devices active)", raidName, hostName, raid.ActiveDevices, raid.TotalDevices),
				Recommendation: "Replace failed drive and initiate rebuild. Check dmesg for drive errors.",
				Evidence:       fmt.Sprintf("State: %s, Active: %d/%d, Failed: %d", raid.State, raid.ActiveDevices, raid.TotalDevices, raid.FailedDevices),
			})

		case "recovering", "rebuilding", "resyncing", "RECOVERING":
			severity := FindingSeverityWarning
			if raid.RebuildPercent < 50 {
				severity = FindingSeverityWatch // Early in rebuild, less urgent
			}
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-rebuilding"),
				Severity:       severity,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID array rebuilding",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' is rebuilding (%.1f%% complete)", raidName, hostName, raid.RebuildPercent),
				Recommendation: "Monitor rebuild progress. Avoid heavy I/O if possible. Array is vulnerable until rebuild completes.",
				Evidence:       fmt.Sprintf("State: %s, Progress: %.1f%%, Speed: %s", raid.State, raid.RebuildPercent, raid.RebuildSpeed),
			})

		case "inactive", "INACTIVE":
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-inactive"),
				Severity:       FindingSeverityCritical,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID array inactive",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' is inactive", raidName, hostName),
				Recommendation: "RAID array is not running. Check mdadm status and system logs.",
				Evidence:       fmt.Sprintf("State: %s", raid.State),
			})
		}

		// Check for failed devices even if array state is "clean"
		if raid.FailedDevices > 0 && raid.State != "degraded" {
			findings = append(findings, &Finding{
				ID:             generateFindingID(host.ID+":"+raid.Device, "reliability", "raid-failed-devices"),
				Severity:       FindingSeverityWarning,
				Category:       FindingCategoryReliability,
				ResourceID:     host.ID + ":" + raid.Device,
				ResourceName:   fmt.Sprintf("%s/%s", hostName, raidName),
				ResourceType:   "host_raid",
				Title:          "RAID has failed devices",
				Description:    fmt.Sprintf("RAID array '%s' on '%s' has %d failed device(s)", raidName, hostName, raid.FailedDevices),
				Recommendation: "Replace failed drives. Array may still be operational due to spares.",
				Evidence:       fmt.Sprintf("Failed: %d, Spare: %d", raid.FailedDevices, raid.SpareDevices),
			})
		}
	}

	// Check high temperature (during deep analysis)
	if deep && len(host.Sensors.TemperatureCelsius) > 0 {
		for sensorName, temp := range host.Sensors.TemperatureCelsius {
			if temp > 85 {
				severity := FindingSeverityWarning
				if temp > 95 {
					severity = FindingSeverityCritical
				}
				findings = append(findings, &Finding{
					ID:             generateFindingID(host.ID+":temp:"+sensorName, "reliability", "high-temp"),
					Severity:       severity,
					Category:       FindingCategoryReliability,
					ResourceID:     host.ID + ":temp:" + sensorName,
					ResourceName:   fmt.Sprintf("%s/%s", hostName, sensorName),
					ResourceType:   "host_sensor",
					Title:          "High temperature",
					Description:    fmt.Sprintf("Sensor '%s' on '%s' reading %.0f°C", sensorName, hostName, temp),
					Recommendation: "Check cooling, fans, and airflow. High temps can cause hardware damage.",
					Evidence:       fmt.Sprintf("Temperature: %.1f°C", temp),
				})
			}
		}
	}

	return findings
}
