// Package servicediscovery provides infrastructure discovery capabilities.
// It discovers services, versions, configurations, and CLI access methods
// for VMs, LXCs, Docker containers, Kubernetes pods, and hosts.
package servicediscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// sensitiveKeyPatterns defines patterns that indicate a label/env key might contain secrets.
// These patterns are case-insensitive and match if any part of the key contains them.
var sensitiveKeyPatterns = []string{
	"password", "passwd", "pwd",
	"secret",
	"key", "apikey", "api_key",
	"token",
	"credential", "cred",
	"auth",
	"private",
	"cert",
}

// filterSensitiveLabels removes or redacts labels that may contain sensitive values.
// It returns a new map with sensitive values replaced with "[REDACTED]".
// Keys are checked case-insensitively for sensitive patterns.
func filterSensitiveLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}

	filtered := make(map[string]string, len(labels))
	redactedCount := 0

	for key, value := range labels {
		keyLower := strings.ToLower(key)
		isSensitive := false

		for _, pattern := range sensitiveKeyPatterns {
			if strings.Contains(keyLower, pattern) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			filtered[key] = "[REDACTED]"
			redactedCount++
		} else {
			filtered[key] = value
		}
	}

	if redactedCount > 0 {
		log.Debug().
			Int("redacted_count", redactedCount).
			Int("total_labels", len(labels)).
			Msg("Redacted sensitive labels before AI analysis")
	}

	return filtered
}

// StateProvider provides access to the current infrastructure state.
type StateProvider interface {
	GetState() StateSnapshot
}

// StateSnapshot represents the infrastructure state. This mirrors models.StateSnapshot
// to avoid circular dependencies.
type StateSnapshot struct {
	VMs                []VM
	Containers         []Container
	DockerHosts        []DockerHost
	KubernetesClusters []KubernetesCluster
	Hosts              []Host
	Nodes              []Node
}

// Node represents a Proxmox VE node.
type Node struct {
	ID                string
	Name              string
	LinkedHostAgentID string
}

// Host represents a host system (via host-agent).
type Host struct {
	ID            string
	Hostname      string
	DisplayName   string
	Platform      string // e.g., "linux", "darwin", "windows"
	OSName        string // e.g., "Unraid", "Ubuntu", "Debian"
	OSVersion     string
	KernelVersion string
	Architecture  string // e.g., "amd64", "arm64"
	CPUCount      int
	Status        string
	Tags          []string
}

// VM represents a virtual machine.
type VM struct {
	VMID     int
	Name     string
	Node     string
	Status   string
	Instance string
	// Additional metadata for fingerprinting
	CPUs        int      // Number of CPU cores
	MaxMemory   uint64   // Max memory in bytes
	MaxDisk     uint64   // Max disk in bytes
	Tags        []string // User-defined tags
	OSName      string   // Detected OS name
	OSVersion   string   // OS version string
	IPAddresses []string // IP addresses assigned to the VM
	Template    bool     // True if this is a template
}

// Container represents an LXC container.
type Container struct {
	VMID     int
	Name     string
	Node     string
	Status   string
	Instance string
	// Additional metadata for fingerprinting
	CPUs        int      // Number of CPU cores
	MaxMemory   uint64   // Max memory in bytes
	MaxDisk     uint64   // Max disk in bytes
	Tags        []string // User-defined tags
	OSTemplate  string   // Template or OCI image used
	OSName      string   // Detected OS name
	IsOCI       bool     // True if OCI container (Proxmox 9.1+)
	IPAddresses []string // IP addresses assigned to the container
	Template    bool     // True if this is a template
}

// DockerHost represents a Docker host.
type DockerHost struct {
	AgentID    string
	Hostname   string
	Containers []DockerContainer
}

// DockerContainer represents a Docker container.
type DockerContainer struct {
	ID     string
	Name   string
	Image  string
	Status string
	Ports  []DockerPort
	Labels map[string]string
	Mounts []DockerMount
}

// DockerPort represents a port mapping.
type DockerPort struct {
	PublicPort  int
	PrivatePort int
	Protocol    string
}

// DockerMount represents a mount point.
type DockerMount struct {
	Source      string
	Destination string
}

// KubernetesCluster represents a Kubernetes cluster.
type KubernetesCluster struct {
	ID      string
	Name    string
	AgentID string
	Status  string
	Pods    []KubernetesPod
}

// KubernetesPod represents a Kubernetes pod.
type KubernetesPod struct {
	UID        string
	Name       string
	Namespace  string
	NodeName   string
	Phase      string
	Labels     map[string]string
	OwnerKind  string // e.g., "Deployment", "StatefulSet", "DaemonSet"
	OwnerName  string
	Containers []KubernetesPodContainer
}

// KubernetesPodContainer represents a container within a Kubernetes pod.
type KubernetesPodContainer struct {
	Name         string
	Image        string
	Ready        bool
	RestartCount int32
	State        string // e.g., "running", "waiting", "terminated"
}

// AIAnalyzer provides AI analysis capabilities for discovery.
type AIAnalyzer interface {
	AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error)
}

// WSMessage represents a WebSocket message for broadcasting.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// WSBroadcaster provides WebSocket broadcasting capabilities.
type WSBroadcaster interface {
	BroadcastDiscoveryProgress(progress *DiscoveryProgress)
}

// Service manages infrastructure discovery.
type Service struct {
	store         *Store
	scanner       *DeepScanner
	stateProvider StateProvider
	aiAnalyzer    AIAnalyzer
	wsHub         WSBroadcaster // WebSocket hub for broadcasting progress

	mu              sync.RWMutex
	running         bool
	stopCh          chan struct{}
	intervalCh      chan time.Duration // Channel for live interval updates
	interval        time.Duration
	initialDelay    time.Duration
	lastRun         time.Time
	deepScanTimeout time.Duration // Timeout for individual deep scans
	maxDiscoveryAge time.Duration // Max age before rediscovery (default 30 days)

	// Cache for AI analysis results (by image name)
	analysisCache map[string]*analysisCacheEntry
	cacheMu       sync.RWMutex
	cacheExpiry   time.Duration

	// In-progress discovery tracking (prevents duplicate concurrent discoveries)
	inProgressMu sync.Mutex
	inProgress   map[string]*discoveryInProgress
}

// discoveryInProgress tracks an ongoing discovery operation.
// Multiple callers can wait on the done channel for completion.
type discoveryInProgress struct {
	done   chan struct{}      // Closed when discovery completes
	result *ResourceDiscovery // Result after completion
	err    error              // Error after completion
}

// analysisCacheEntry holds a cached AI analysis result with its timestamp.
type analysisCacheEntry struct {
	result   *AIAnalysisResponse
	cachedAt time.Time
}

// Config holds discovery service configuration.
type Config struct {
	DataDir         string
	Interval        time.Duration // How often to run fingerprint collection (default 5 min)
	CacheExpiry     time.Duration // How long to cache AI analysis results
	DeepScanTimeout time.Duration // Timeout for individual deep scans (default 60s)

	// Fingerprint-based discovery settings
	MaxDiscoveryAge     time.Duration // Rediscover after this duration (default 30 days)
	FingerprintInterval time.Duration // How often to collect fingerprints (default 5 min)
}

// DefaultConfig returns the default discovery configuration.
func DefaultConfig() Config {
	return Config{
		Interval:            5 * time.Minute, // Fingerprint collection interval
		CacheExpiry:         1 * time.Hour,
		DeepScanTimeout:     60 * time.Second,
		MaxDiscoveryAge:     30 * 24 * time.Hour, // 30 days
		FingerprintInterval: 5 * time.Minute,
	}
}

// NewService creates a new discovery service.
func NewService(store *Store, scanner *DeepScanner, stateProvider StateProvider, cfg Config) *Service {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.CacheExpiry == 0 {
		cfg.CacheExpiry = 1 * time.Hour
	}
	if cfg.DeepScanTimeout == 0 {
		cfg.DeepScanTimeout = 60 * time.Second
	}
	if cfg.MaxDiscoveryAge == 0 {
		cfg.MaxDiscoveryAge = 30 * 24 * time.Hour // 30 days
	}

	return &Service{
		store:           store,
		scanner:         scanner,
		stateProvider:   stateProvider,
		interval:        cfg.Interval,
		initialDelay:    30 * time.Second,
		cacheExpiry:     cfg.CacheExpiry,
		deepScanTimeout: cfg.DeepScanTimeout,
		maxDiscoveryAge: cfg.MaxDiscoveryAge,
		stopCh:          make(chan struct{}),
		intervalCh:      make(chan time.Duration, 1), // Buffered to prevent blocking
		analysisCache:   make(map[string]*analysisCacheEntry),
		inProgress:      make(map[string]*discoveryInProgress),
	}
}

// SetAIAnalyzer sets the AI analyzer for discovery.
func (s *Service) SetAIAnalyzer(analyzer AIAnalyzer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiAnalyzer = analyzer
}

// Start begins the background discovery service.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	log.Info().
		Dur("interval", s.interval).
		Msg("Starting infrastructure discovery service")

	go s.discoveryLoop(ctx)
}

// Stop stops the background discovery service.
func (s *Service) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		close(s.stopCh)
		s.running = false
	}
}

// SetInterval updates the scan interval. Takes effect immediately if running.
func (s *Service) SetInterval(interval time.Duration) {
	s.mu.Lock()
	s.interval = interval
	running := s.running
	s.mu.Unlock()

	// If running, send the new interval to the loop (non-blocking)
	if running {
		select {
		case s.intervalCh <- interval:
			log.Info().Dur("interval", interval).Msg("Discovery interval updated (live)")
		default:
			// Channel full, interval will be picked up eventually
			log.Debug().Dur("interval", interval).Msg("Discovery interval updated (pending)")
		}
	}
}

// needsDeepScan determines if a discovery result needs a deep scan based on quality.
// Returns true if the discovery is incomplete or low-confidence.
func (s *Service) needsDeepScan(discovery *ResourceDiscovery) bool {
	if discovery == nil {
		return true // No discovery at all
	}

	// Already has deep scan data (raw command outputs)
	if len(discovery.RawCommandOutput) > 0 {
		return false
	}

	// Low confidence - needs more investigation
	if discovery.Confidence < 0.7 {
		return true
	}

	// Unknown service type
	if discovery.ServiceType == "" || discovery.ServiceType == "unknown" {
		return true
	}

	// Missing key paths that deep scan could discover
	if len(discovery.Facts) == 0 && len(discovery.ConfigPaths) == 0 && len(discovery.LogPaths) == 0 {
		return true
	}

	return false
}

// SetWSHub sets the WebSocket hub for broadcasting progress updates.
func (s *Service) SetWSHub(hub WSBroadcaster) {
	s.mu.Lock()
	s.wsHub = hub
	s.mu.Unlock()

	// Wire up the scanner's progress callback to broadcast via WebSocket
	if s.scanner != nil {
		s.scanner.SetProgressCallback(s.broadcastProgress)
	}

	log.Info().Msg("WebSocket hub connected to discovery service")
}

// broadcastProgress broadcasts discovery progress to all WebSocket clients.
func (s *Service) broadcastProgress(progress *DiscoveryProgress) {
	s.mu.RLock()
	hub := s.wsHub
	s.mu.RUnlock()

	if hub == nil || progress == nil {
		return
	}

	hub.BroadcastDiscoveryProgress(progress)
}

// IsRunning returns whether the background discovery loop is active.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// discoveryLoop runs periodic fingerprint collection (NOT actual discovery).
// This is the new fingerprint-based approach: background loop only collects fingerprints
// to detect changes. Discovery only runs on-demand when data is actually needed.
func (s *Service) discoveryLoop(ctx context.Context) {
	delay := s.initialDelay
	if delay <= 0 {
		delay = 30 * time.Second
	}

	// Run initial fingerprint collection after a short delay
	select {
	case <-time.After(delay):
	case <-s.stopCh:
		return
	case <-ctx.Done():
		return
	}

	s.collectFingerprints(ctx)

	s.mu.RLock()
	currentInterval := s.interval
	s.mu.RUnlock()

	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collectFingerprints(ctx)
		case newInterval := <-s.intervalCh:
			// Interval changed - reset the ticker
			ticker.Stop()
			ticker = time.NewTicker(newInterval)
			log.Info().Dur("interval", newInterval).Msg("Fingerprint collection interval reset")
		case <-s.stopCh:
			log.Info().Msg("Stopping discovery service")
			return
		case <-ctx.Done():
			log.Info().Msg("Discovery context cancelled")
			return
		}
	}
}

// collectFingerprints collects fingerprints from all resources (Docker, LXC, VM).
// This is FREE (no AI calls) - it just hashes metadata to detect changes.
func (s *Service) collectFingerprints(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Stack().Msg("Recovered from panic in fingerprint collection")
		}
	}()

	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()

	if s.stateProvider == nil {
		return
	}

	state := s.stateProvider.GetState()
	changedCount := 0
	newCount := 0

	// Process Docker containers
	for _, host := range state.DockerHosts {
		for _, container := range host.Containers {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Generate new fingerprint (prefixed with docker: to avoid collisions)
			newFP := GenerateDockerFingerprint(host.AgentID, &container)
			fpKey := "docker:" + host.AgentID + ":" + newFP.ResourceID

			// Get previous fingerprint
			oldFP, _ := s.store.GetFingerprint(fpKey)

			// Update the fingerprint's ResourceID to include prefix for storage
			newFP.ResourceID = fpKey

			// Save new fingerprint
			if err := s.store.SaveFingerprint(newFP); err != nil {
				log.Warn().Err(err).Str("container", container.Name).Msg("Failed to save Docker fingerprint")
				continue
			}

			// Check if this is new or changed
			if oldFP == nil {
				newCount++
				log.Debug().
					Str("type", "docker").
					Str("container", container.Name).
					Str("hash", newFP.Hash).
					Msg("New fingerprint captured")
			} else if newFP.HasSchemaChanged(oldFP) {
				// Schema changed - don't count as "changed" to avoid mass rediscovery
				log.Debug().
					Str("type", "docker").
					Str("container", container.Name).
					Int("old_schema", oldFP.SchemaVersion).
					Int("new_schema", newFP.SchemaVersion).
					Msg("Fingerprint schema updated")
			} else if oldFP.Hash != newFP.Hash {
				changedCount++
				log.Info().
					Str("type", "docker").
					Str("container", container.Name).
					Str("old_hash", oldFP.Hash).
					Str("new_hash", newFP.Hash).
					Msg("Fingerprint changed - discovery will run on next request")
			}
		}
	}

	// Process LXC containers
	for _, lxc := range state.Containers {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Generate new fingerprint
		newFP := GenerateLXCFingerprint(lxc.Node, &lxc)
		fpKey := "lxc:" + lxc.Node + ":" + newFP.ResourceID

		// Get previous fingerprint
		oldFP, _ := s.store.GetFingerprint(fpKey)

		// Update the fingerprint's ResourceID to include prefix for storage
		newFP.ResourceID = fpKey

		// Save new fingerprint
		if err := s.store.SaveFingerprint(newFP); err != nil {
			log.Warn().Err(err).Str("lxc", lxc.Name).Msg("Failed to save LXC fingerprint")
			continue
		}

		// Check if this is new or changed
		if oldFP == nil {
			newCount++
			log.Debug().
				Str("type", "lxc").
				Str("name", lxc.Name).
				Int("vmid", lxc.VMID).
				Str("hash", newFP.Hash).
				Msg("New fingerprint captured")
		} else if newFP.HasSchemaChanged(oldFP) {
			log.Debug().
				Str("type", "lxc").
				Str("name", lxc.Name).
				Int("vmid", lxc.VMID).
				Int("old_schema", oldFP.SchemaVersion).
				Int("new_schema", newFP.SchemaVersion).
				Msg("Fingerprint schema updated")
		} else if oldFP.Hash != newFP.Hash {
			changedCount++
			log.Info().
				Str("type", "lxc").
				Str("name", lxc.Name).
				Int("vmid", lxc.VMID).
				Str("old_hash", oldFP.Hash).
				Str("new_hash", newFP.Hash).
				Msg("Fingerprint changed - discovery will run on next request")
		}
	}

	// Process VMs
	for _, vm := range state.VMs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Generate new fingerprint
		newFP := GenerateVMFingerprint(vm.Node, &vm)
		fpKey := "vm:" + vm.Node + ":" + newFP.ResourceID

		// Get previous fingerprint
		oldFP, _ := s.store.GetFingerprint(fpKey)

		// Update the fingerprint's ResourceID to include prefix for storage
		newFP.ResourceID = fpKey

		// Save new fingerprint
		if err := s.store.SaveFingerprint(newFP); err != nil {
			log.Warn().Err(err).Str("vm", vm.Name).Msg("Failed to save VM fingerprint")
			continue
		}

		// Check if this is new or changed
		if oldFP == nil {
			newCount++
			log.Debug().
				Str("type", "vm").
				Str("name", vm.Name).
				Int("vmid", vm.VMID).
				Str("hash", newFP.Hash).
				Msg("New fingerprint captured")
		} else if newFP.HasSchemaChanged(oldFP) {
			log.Debug().
				Str("type", "vm").
				Str("name", vm.Name).
				Int("vmid", vm.VMID).
				Int("old_schema", oldFP.SchemaVersion).
				Int("new_schema", newFP.SchemaVersion).
				Msg("Fingerprint schema updated")
		} else if oldFP.Hash != newFP.Hash {
			changedCount++
			log.Info().
				Str("type", "vm").
				Str("name", vm.Name).
				Int("vmid", vm.VMID).
				Str("old_hash", oldFP.Hash).
				Str("new_hash", newFP.Hash).
				Msg("Fingerprint changed - discovery will run on next request")
		}
	}

	// Process Kubernetes pods
	for _, cluster := range state.KubernetesClusters {
		for _, pod := range cluster.Pods {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Generate new fingerprint
			newFP := GenerateK8sPodFingerprint(cluster.ID, &pod)
			fpKey := "k8s:" + cluster.ID + ":" + pod.Namespace + "/" + pod.Name

			// Get previous fingerprint
			oldFP, _ := s.store.GetFingerprint(fpKey)

			// Update the fingerprint's ResourceID to include prefix for storage
			newFP.ResourceID = fpKey

			// Save new fingerprint
			if err := s.store.SaveFingerprint(newFP); err != nil {
				log.Warn().Err(err).Str("pod", pod.Name).Str("namespace", pod.Namespace).Msg("Failed to save K8s pod fingerprint")
				continue
			}

			// Check if this is new or changed
			if oldFP == nil {
				newCount++
				log.Debug().
					Str("type", "k8s").
					Str("name", pod.Name).
					Str("namespace", pod.Namespace).
					Str("cluster", cluster.Name).
					Str("hash", newFP.Hash).
					Msg("New fingerprint captured")
			} else if newFP.HasSchemaChanged(oldFP) {
				log.Debug().
					Str("type", "k8s").
					Str("name", pod.Name).
					Str("namespace", pod.Namespace).
					Str("cluster", cluster.Name).
					Int("old_schema", oldFP.SchemaVersion).
					Int("new_schema", newFP.SchemaVersion).
					Msg("Fingerprint schema updated")
			} else if oldFP.Hash != newFP.Hash {
				changedCount++
				log.Info().
					Str("type", "k8s").
					Str("name", pod.Name).
					Str("namespace", pod.Namespace).
					Str("cluster", cluster.Name).
					Str("old_hash", oldFP.Hash).
					Str("new_hash", newFP.Hash).
					Msg("Fingerprint changed - discovery will run on next request")
			}
		}
	}

	// Update last scan time
	s.store.SetLastFingerprintScan(time.Now())

	if newCount > 0 || changedCount > 0 {
		log.Info().
			Int("new", newCount).
			Int("changed", changedCount).
			Int("total", s.store.GetFingerprintCount()).
			Msg("Fingerprint collection complete")
	} else {
		log.Debug().
			Int("total", s.store.GetFingerprintCount()).
			Msg("Fingerprint collection complete - no changes")
	}

	// Cleanup orphaned data (fingerprints/discoveries for removed resources)
	s.cleanupOrphanedData(state)
}

// cleanupOrphanedData removes fingerprints and discoveries for resources that no longer exist.
func (s *Service) cleanupOrphanedData(state StateSnapshot) {
	// Safety check: Don't cleanup if state appears empty
	// This prevents catastrophic deletion if state provider has an error
	totalResources := len(state.Containers) + len(state.VMs) + len(state.KubernetesClusters)
	for _, host := range state.DockerHosts {
		totalResources += len(host.Containers)
	}
	if totalResources == 0 {
		log.Debug().Msg("Skipping orphaned data cleanup - state is empty (may be an error)")
		return
	}

	// Build set of current resource IDs
	currentIDs := make(map[string]bool)

	// Docker containers
	for _, host := range state.DockerHosts {
		for _, container := range host.Containers {
			fpKey := "docker:" + host.AgentID + ":" + container.Name
			currentIDs[fpKey] = true
		}
	}

	// LXC containers
	for _, lxc := range state.Containers {
		fpKey := "lxc:" + lxc.Node + ":" + strconv.Itoa(lxc.VMID)
		currentIDs[fpKey] = true
	}

	// VMs
	for _, vm := range state.VMs {
		fpKey := "vm:" + vm.Node + ":" + strconv.Itoa(vm.VMID)
		currentIDs[fpKey] = true
	}

	// Kubernetes pods
	for _, cluster := range state.KubernetesClusters {
		for _, pod := range cluster.Pods {
			fpKey := "k8s:" + cluster.ID + ":" + pod.Namespace + "/" + pod.Name
			currentIDs[fpKey] = true
		}
	}

	// Run cleanup
	fpRemoved := s.store.CleanupOrphanedFingerprints(currentIDs)
	discRemoved := s.store.CleanupOrphanedDiscoveries(currentIDs)

	if fpRemoved > 0 || discRemoved > 0 {
		log.Info().
			Int("fingerprints_removed", fpRemoved).
			Int("discoveries_removed", discRemoved).
			Msg("Cleaned up orphaned data")
	}
}

// discoverDockerContainers runs discovery on Docker containers using metadata.
// Automatically runs deep scans when the shallow scan results are incomplete or low-confidence.
func (s *Service) discoverDockerContainers(ctx context.Context, hosts []DockerHost) {
	s.mu.RLock()
	analyzer := s.aiAnalyzer
	s.mu.RUnlock()

	if analyzer == nil {
		log.Debug().Msg("AI analyzer not set, skipping Docker discovery")
		return
	}

	for _, host := range hosts {
		for _, container := range host.Containers {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Build resource ID
			id := MakeResourceID(ResourceTypeDocker, host.AgentID, container.Name)

			// Check if we already have a recent discovery
			if !s.store.NeedsRefresh(id, s.cacheExpiry) {
				continue
			}

			// Check existing discovery to see if it needs a deep scan
			existing, _ := s.store.Get(id)

			// Analyze using metadata (shallow discovery)
			discovery := s.analyzeDockerContainer(ctx, analyzer, container, host)
			if discovery != nil {
				// Smart auto deep scan: enhance if discovery is incomplete or low-confidence
				// Also deep scan if there's no existing discovery (first time)
				if s.scanner != nil && (existing == nil || s.needsDeepScan(discovery)) {
					log.Info().
						Str("id", id).
						Float64("confidence", discovery.Confidence).
						Str("serviceType", discovery.ServiceType).
						Bool("firstDiscovery", existing == nil).
						Msg("Auto deep scan triggered due to incomplete discovery")
					discovery = s.enhanceWithDeepScan(ctx, discovery, host)
				}

				// Suggest web interface URL using Docker host hostname
				discovery.SuggestedURL = SuggestWebURL(discovery, host.Hostname)

				if err := s.store.Save(discovery); err != nil {
					log.Warn().Err(err).Str("id", id).Msg("Failed to save discovery")
				}
			}
		}
	}
}

// enhanceWithDeepScan runs a deep scan and merges the results into the discovery.
func (s *Service) enhanceWithDeepScan(ctx context.Context, discovery *ResourceDiscovery, host DockerHost) *ResourceDiscovery {
	s.mu.RLock()
	timeout := s.deepScanTimeout
	analyzer := s.aiAnalyzer
	s.mu.RUnlock()

	if s.scanner == nil || analyzer == nil {
		return discovery
	}

	// Create a timeout context for the deep scan
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := DiscoveryRequest{
		ResourceType: discovery.ResourceType,
		ResourceID:   discovery.ResourceID,
		HostID:       discovery.HostID,
		Hostname:     discovery.Hostname,
	}

	scanResult, err := s.scanner.Scan(scanCtx, req)
	if err != nil {
		log.Debug().Err(err).Str("id", discovery.ID).Msg("Deep scan failed during background discovery")
		return discovery
	}

	if len(scanResult.CommandOutputs) == 0 {
		return discovery
	}

	// Build analysis request with command outputs
	analysisReq := AIAnalysisRequest{
		ResourceType:   discovery.ResourceType,
		ResourceID:     discovery.ResourceID,
		HostID:         discovery.HostID,
		Hostname:       discovery.Hostname,
		CommandOutputs: scanResult.CommandOutputs,
	}

	// Add metadata if available
	if s.stateProvider != nil {
		analysisReq.Metadata = s.getResourceMetadata(req)
	}

	// Build prompt and analyze
	prompt := s.buildDeepAnalysisPrompt(analysisReq)
	response, err := analyzer.AnalyzeForDiscovery(scanCtx, prompt)
	if err != nil {
		log.Debug().Err(err).Str("id", discovery.ID).Msg("Deep analysis failed during background discovery")
		return discovery
	}

	result := s.parseAIResponse(response)
	if result == nil {
		return discovery
	}

	// Merge results - deep scan results take precedence for non-empty fields
	if result.ServiceType != "" && result.ServiceType != "unknown" {
		discovery.ServiceType = result.ServiceType
	}
	if result.ServiceName != "" {
		discovery.ServiceName = result.ServiceName
	}
	if result.ServiceVersion != "" {
		discovery.ServiceVersion = result.ServiceVersion
	}
	if result.Category != "" && result.Category != CategoryUnknown {
		discovery.Category = result.Category
	}
	if result.CLIAccess != "" {
		discovery.CLIAccess = s.formatCLIAccess(discovery.ResourceType, discovery.ResourceID, result.CLIAccess)
	}
	if len(result.Facts) > 0 {
		discovery.Facts = result.Facts
	}
	if len(result.ConfigPaths) > 0 {
		discovery.ConfigPaths = result.ConfigPaths
	}
	if len(result.DataPaths) > 0 {
		discovery.DataPaths = result.DataPaths
	}
	if len(result.LogPaths) > 0 {
		discovery.LogPaths = result.LogPaths
	}
	if len(result.Ports) > 0 {
		discovery.Ports = result.Ports
	}
	if result.Confidence > discovery.Confidence {
		discovery.Confidence = result.Confidence
	}
	if result.Reasoning != "" {
		discovery.AIReasoning = result.Reasoning
	}

	// Store raw command outputs
	discovery.RawCommandOutput = scanResult.CommandOutputs
	discovery.ScanDuration = scanResult.CompletedAt.Sub(scanResult.StartedAt).Milliseconds()
	discovery.UpdatedAt = time.Now()

	// Parse docker_mounts if present (for LXCs/VMs running Docker)
	if dockerMountsOutput, ok := scanResult.CommandOutputs["docker_mounts"]; ok {
		discovery.DockerMounts = parseDockerMounts(dockerMountsOutput)
		if len(discovery.DockerMounts) > 0 {
			log.Debug().
				Str("id", discovery.ID).
				Int("mountCount", len(discovery.DockerMounts)).
				Msg("Parsed Docker bind mounts from discovery")
		}
	}

	log.Info().
		Str("id", discovery.ID).
		Int("commandOutputs", len(scanResult.CommandOutputs)).
		Int("dockerMounts", len(discovery.DockerMounts)).
		Dur("scanDuration", scanResult.CompletedAt.Sub(scanResult.StartedAt)).
		Msg("Enhanced discovery with deep scan")

	return discovery
}

// analyzeDockerContainer analyzes a Docker container using AI.
func (s *Service) analyzeDockerContainer(ctx context.Context, analyzer AIAnalyzer, c DockerContainer, host DockerHost) *ResourceDiscovery {
	// Check cache first (per-image timestamp)
	s.cacheMu.RLock()
	entry, found := s.analysisCache[c.Image]
	cacheValid := found && time.Since(entry.cachedAt) < s.cacheExpiry
	s.cacheMu.RUnlock()

	var result *AIAnalysisResponse

	if cacheValid {
		result = entry.result
	} else {
		// Build prompt for AI analysis
		prompt := s.buildMetadataAnalysisPrompt(c, host)

		response, err := analyzer.AnalyzeForDiscovery(ctx, prompt)
		if err != nil {
			log.Warn().Err(err).Str("container", c.Name).Msg("AI analysis failed")
			return nil
		}

		result = s.parseAIResponse(response)
		if result == nil {
			log.Warn().Str("container", c.Name).Msg("Failed to parse AI response")
			return nil
		}

		// Cache the result with its own timestamp
		s.cacheMu.Lock()
		s.analysisCache[c.Image] = &analysisCacheEntry{
			result:   result,
			cachedAt: time.Now(),
		}
		s.cacheMu.Unlock()
	}

	// Skip unknown/low-confidence results
	if result.ServiceType == "unknown" || result.Confidence < 0.5 {
		return nil
	}

	// Build CLI access string
	cliAccess := result.CLIAccess
	if cliAccess != "" {
		cliAccess = strings.ReplaceAll(cliAccess, "{container}", c.Name)
	}

	// Extract ports
	var ports []PortInfo
	for _, p := range c.Ports {
		ports = append(ports, PortInfo{
			Port:     p.PrivatePort,
			Protocol: p.Protocol,
			Address:  fmt.Sprintf(":%d", p.PublicPort),
		})
	}

	return &ResourceDiscovery{
		ID:             MakeResourceID(ResourceTypeDocker, host.AgentID, c.Name),
		ResourceType:   ResourceTypeDocker,
		ResourceID:     c.Name,
		HostID:         host.AgentID,
		Hostname:       host.Hostname,
		ServiceType:    result.ServiceType,
		ServiceName:    result.ServiceName,
		ServiceVersion: result.ServiceVersion,
		Category:       result.Category,
		CLIAccess:      cliAccess,
		Facts:          result.Facts,
		ConfigPaths:    result.ConfigPaths,
		DataPaths:      result.DataPaths,
		LogPaths:       result.LogPaths,
		Ports:          ports,
		Confidence:     result.Confidence,
		AIReasoning:    result.Reasoning,
		DiscoveredAt:   time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// DiscoverResource performs deep discovery on a specific resource.
// Uses fingerprint-based detection to avoid unnecessary AI calls:
// - Returns cached discovery if fingerprint hasn't changed
// - Runs discovery only when fingerprint changed or discovery is too old
// - Prevents duplicate concurrent discoveries for the same resource
func (s *Service) DiscoverResource(ctx context.Context, req DiscoveryRequest) (*ResourceDiscovery, error) {
	// Redirect PVE node requests to linked host agent if available
	// This ensures we always scan and store data under the canonical Host Agent ID
	if req.ResourceType == ResourceTypeHost && s.stateProvider != nil {
		state := s.stateProvider.GetState()
		for _, node := range state.Nodes {
			// Check if the requested ID matches the Node Name or ID
			if node.Name == req.HostID || node.Name == req.ResourceID || node.ID == req.ResourceID {
				if node.LinkedHostAgentID != "" {
					log.Info().
						Str("from_host", req.HostID).
						Str("to_agent", node.LinkedHostAgentID).
						Msg("Redirecting discovery scan to linked host agent")
					req.HostID = node.LinkedHostAgentID
					req.ResourceID = node.LinkedHostAgentID
				}
				break
			}
		}
	}

	resourceID := MakeResourceID(req.ResourceType, req.HostID, req.ResourceID)

	// Get current fingerprint (if available)
	// Fingerprint key matches the resource ID format: type:host:id
	currentFP, _ := s.store.GetFingerprint(resourceID)

	// Get existing discovery
	existing, _ := s.store.Get(resourceID)

	// Determine if we need to run discovery
	needsDiscovery := false
	reason := ""

	if req.Force {
		needsDiscovery = true
		reason = "forced"
	} else if existing == nil {
		needsDiscovery = true
		reason = "no existing discovery"
	} else if currentFP != nil && existing.Fingerprint != currentFP.Hash {
		// Fingerprint hash differs - check if it's just a schema version change
		if existing.FingerprintSchemaVersion != 0 && existing.FingerprintSchemaVersion != currentFP.SchemaVersion {
			// Schema changed but container didn't - don't trigger rediscovery
			// This prevents mass rediscovery when we upgrade the fingerprint algorithm
			log.Debug().
				Str("id", resourceID).
				Int("old_schema", existing.FingerprintSchemaVersion).
				Int("new_schema", currentFP.SchemaVersion).
				Msg("Fingerprint schema changed, but not triggering rediscovery")
		} else {
			// Same schema version, different hash = real container change
			needsDiscovery = true
			reason = "fingerprint changed"
		}
	} else if time.Since(existing.DiscoveredAt) > s.maxDiscoveryAge {
		needsDiscovery = true
		reason = "discovery too old"
	}

	// Return cached discovery if still valid
	if !needsDiscovery && existing != nil {
		log.Debug().Str("id", resourceID).Msg("Discovery still valid, returning cached")
		return existing, nil
	}

	// Check for duplicate concurrent discovery requests
	s.inProgressMu.Lock()
	if inProg, ok := s.inProgress[resourceID]; ok {
		// Discovery already in progress - wait for it
		s.inProgressMu.Unlock()
		log.Debug().Str("id", resourceID).Msg("Discovery already in progress, waiting for result")

		select {
		case <-inProg.done:
			return inProg.result, inProg.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Claim this discovery slot
	inProg := &discoveryInProgress{
		done: make(chan struct{}),
	}
	s.inProgress[resourceID] = inProg
	s.inProgressMu.Unlock()

	// Ensure we clean up and notify waiters when done
	defer func() {
		close(inProg.done)
		s.inProgressMu.Lock()
		delete(s.inProgress, resourceID)
		s.inProgressMu.Unlock()
	}()

	log.Info().Str("id", resourceID).Str("reason", reason).Msg("Running discovery")

	s.mu.RLock()
	analyzer := s.aiAnalyzer
	s.mu.RUnlock()

	if analyzer == nil {
		inProg.err = fmt.Errorf("AI analyzer not configured")
		return nil, inProg.err
	}

	// Run deep scan if scanner is available
	var scanResult *ScanResult
	var scanError error
	if s.scanner != nil {
		scanResult, scanError = s.scanner.Scan(ctx, req)
		if scanError != nil {
			log.Warn().
				Err(scanError).
				Str("id", resourceID).
				Str("resource_type", string(req.ResourceType)).
				Msg("Deep scan failed, falling back to metadata-only analysis. For full discovery, ensure the host agent is connected with commands enabled.")
		}
	}

	// Build analysis request
	analysisReq := AIAnalysisRequest{
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		HostID:       req.HostID,
		Hostname:     req.Hostname,
	}

	if scanResult != nil {
		analysisReq.CommandOutputs = scanResult.CommandOutputs
	}

	// Add metadata if available
	if s.stateProvider != nil {
		analysisReq.Metadata = s.getResourceMetadata(req)
	}

	// Build prompt and analyze
	prompt := s.buildDeepAnalysisPrompt(analysisReq)

	// Broadcast progress: AI analysis starting
	s.broadcastProgress(&DiscoveryProgress{
		ResourceID:  resourceID,
		Status:      DiscoveryStatusRunning,
		CurrentStep: "Analyzing with Pulse Assistant...",
	})

	response, err := analyzer.AnalyzeForDiscovery(ctx, prompt)
	if err != nil {
		inProg.err = fmt.Errorf("AI analysis failed: %w", err)
		return nil, inProg.err
	}

	result := s.parseAIResponse(response)
	if result == nil {
		// Truncate response for error message
		truncated := response
		if len(truncated) > 500 {
			truncated = truncated[:500] + "..."
		}
		inProg.err = fmt.Errorf("failed to parse AI response: %s", truncated)
		return nil, inProg.err
	}

	// Resolve hostname from metadata if not provided in request
	hostname := req.Hostname
	if hostname == "" && analysisReq.Metadata != nil {
		if name, ok := analysisReq.Metadata["name"].(string); ok && name != "" {
			hostname = name
		}
	}

	// Build discovery result
	discovery := &ResourceDiscovery{
		ID:               resourceID,
		ResourceType:     req.ResourceType,
		ResourceID:       req.ResourceID,
		HostID:           req.HostID,
		Hostname:         hostname,
		ServiceType:      result.ServiceType,
		ServiceName:      result.ServiceName,
		ServiceVersion:   result.ServiceVersion,
		Category:         result.Category,
		CLIAccess:        s.formatCLIAccess(req.ResourceType, req.ResourceID, result.CLIAccess),
		CLIAccessVersion: CLIAccessVersion,
		Facts:            result.Facts,
		ConfigPaths:      result.ConfigPaths,
		DataPaths:        result.DataPaths,
		LogPaths:         result.LogPaths,
		Ports:            result.Ports,
		Confidence:       result.Confidence,
		AIReasoning:      result.Reasoning,
		DiscoveredAt:     time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Store fingerprint with discovery
	if currentFP != nil {
		discovery.Fingerprint = currentFP.Hash
		discovery.FingerprintedAt = currentFP.GeneratedAt
		discovery.FingerprintSchemaVersion = currentFP.SchemaVersion
	}

	if scanResult != nil {
		discovery.RawCommandOutput = scanResult.CommandOutputs
		discovery.ScanDuration = scanResult.CompletedAt.Sub(scanResult.StartedAt).Milliseconds()

		// Parse docker_mounts if present (for LXCs/VMs running Docker)
		if dockerMountsOutput, ok := scanResult.CommandOutputs["docker_mounts"]; ok {
			discovery.DockerMounts = parseDockerMounts(dockerMountsOutput)
			if len(discovery.DockerMounts) > 0 {
				log.Debug().
					Str("id", discovery.ID).
					Int("mountCount", len(discovery.DockerMounts)).
					Msg("Parsed Docker bind mounts from on-demand discovery")
			}
		}
	} else if scanError != nil {
		// Add note to reasoning when we couldn't run commands
		metadataNote := "[Note: Discovery was limited to metadata-only analysis because command execution was unavailable. "
		if strings.Contains(scanError.Error(), "no connected agent") {
			metadataNote += "To enable full discovery with command execution, ensure the host agent has 'Pulse Commands' enabled in Settings → Unified Agents.]"
		} else {
			metadataNote += "Error: " + scanError.Error() + "]"
		}
		if discovery.AIReasoning != "" {
			discovery.AIReasoning = metadataNote + " " + discovery.AIReasoning
		} else {
			discovery.AIReasoning = metadataNote
		}
	}

	// Preserve user notes from existing discovery
	if existing != nil {
		discovery.UserNotes = existing.UserNotes
		discovery.UserSecrets = existing.UserSecrets
		if discovery.DiscoveredAt.IsZero() || existing.DiscoveredAt.Before(discovery.DiscoveredAt) {
			discovery.DiscoveredAt = existing.DiscoveredAt
		}
	}

	// Suggest web interface URL based on service type and external IP
	if s.stateProvider != nil {
		if externalIP := s.getResourceExternalIP(req); externalIP != "" {
			discovery.SuggestedURL = SuggestWebURL(discovery, externalIP)
		}
	}

	// Broadcast progress: Discovery complete
	s.broadcastProgress(&DiscoveryProgress{
		ResourceID:      resourceID,
		Status:          DiscoveryStatusCompleted,
		CurrentStep:     "Discovery complete",
		PercentComplete: 100,
	})

	// Save discovery
	if err := s.store.Save(discovery); err != nil {
		inProg.err = fmt.Errorf("failed to save discovery: %w", err)
		return nil, inProg.err
	}

	// Store result for any waiting goroutines
	inProg.result = discovery
	return discovery, nil
}

// getResourceMetadata retrieves metadata for a resource from the state.
func (s *Service) getResourceMetadata(req DiscoveryRequest) map[string]any {
	if s.stateProvider == nil {
		return nil
	}

	state := s.stateProvider.GetState()
	metadata := make(map[string]any)

	switch req.ResourceType {
	case ResourceTypeLXC:
		for _, c := range state.Containers {
			if fmt.Sprintf("%d", c.VMID) == req.ResourceID && c.Node == req.HostID {
				metadata["name"] = c.Name
				metadata["status"] = c.Status
				metadata["vmid"] = c.VMID
				break
			}
		}
	case ResourceTypeVM:
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == req.ResourceID && vm.Node == req.HostID {
				metadata["name"] = vm.Name
				metadata["status"] = vm.Status
				metadata["vmid"] = vm.VMID
				break
			}
		}
	case ResourceTypeDocker:
		for _, host := range state.DockerHosts {
			if host.AgentID == req.HostID || host.Hostname == req.HostID {
				for _, c := range host.Containers {
					if c.Name == req.ResourceID {
						metadata["image"] = c.Image
						metadata["status"] = c.Status
						// Filter sensitive labels before sending to AI
						metadata["labels"] = filterSensitiveLabels(c.Labels)
						break
					}
				}
				break
			}
		}
	case ResourceTypeHost:
		for _, host := range state.Hosts {
			if host.ID == req.ResourceID || host.Hostname == req.ResourceID || host.ID == req.HostID {
				metadata["hostname"] = host.Hostname
				metadata["display_name"] = host.DisplayName
				metadata["platform"] = host.Platform
				metadata["os_name"] = host.OSName
				metadata["os_version"] = host.OSVersion
				metadata["kernel_version"] = host.KernelVersion
				metadata["architecture"] = host.Architecture
				metadata["cpu_count"] = host.CPUCount
				metadata["status"] = host.Status
				if len(host.Tags) > 0 {
					metadata["tags"] = host.Tags
				}
				break
			}
		}
	}

	return metadata
}

// getResourceExternalIP retrieves the external IP address for a resource from the state.
// For LXC/VM, this is the first IP from the Proxmox guest agent.
// For Docker containers, this is the Docker host's IP/hostname.
func (s *Service) getResourceExternalIP(req DiscoveryRequest) string {
	if s.stateProvider == nil {
		return ""
	}

	state := s.stateProvider.GetState()

	switch req.ResourceType {
	case ResourceTypeLXC:
		for _, c := range state.Containers {
			if fmt.Sprintf("%d", c.VMID) == req.ResourceID && c.Node == req.HostID {
				if len(c.IPAddresses) > 0 {
					return c.IPAddresses[0]
				}
				return ""
			}
		}
	case ResourceTypeVM:
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == req.ResourceID && vm.Node == req.HostID {
				if len(vm.IPAddresses) > 0 {
					return vm.IPAddresses[0]
				}
				return ""
			}
		}
	case ResourceTypeDocker:
		// For Docker containers, use the Docker host's hostname/IP
		for _, host := range state.DockerHosts {
			if host.AgentID == req.HostID || host.Hostname == req.HostID {
				// Use hostname if it looks like an IP, otherwise it's a hostname
				return host.Hostname
			}
		}
	case ResourceTypeDockerVM, ResourceTypeDockerLXC:
		// For Docker containers inside VMs/LXCs, find the VM/LXC's IP
		// The hostID contains the parent resource info
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == req.HostID || vm.Name == req.HostID {
				if len(vm.IPAddresses) > 0 {
					return vm.IPAddresses[0]
				}
			}
		}
		for _, c := range state.Containers {
			if fmt.Sprintf("%d", c.VMID) == req.HostID || c.Name == req.HostID {
				if len(c.IPAddresses) > 0 {
					return c.IPAddresses[0]
				}
			}
		}
	}

	return ""
}

// formatCLIAccess formats the CLI access string with actual values.
func (s *Service) formatCLIAccess(resourceType ResourceType, resourceID, cliTemplate string) string {
	if cliTemplate == "" {
		// Use default template
		cliTemplate = GetCLIAccessTemplate(resourceType)
	}

	result := cliTemplate
	result = strings.ReplaceAll(result, "{vmid}", resourceID)
	result = strings.ReplaceAll(result, "{container}", resourceID)
	result = strings.ReplaceAll(result, "{command}", "...")

	return result
}

// buildMetadataAnalysisPrompt builds a prompt for shallow metadata-based analysis.
func (s *Service) buildMetadataAnalysisPrompt(c DockerContainer, host DockerHost) string {
	info := map[string]any{
		"name":   c.Name,
		"image":  c.Image,
		"status": c.Status,
		"host":   host.Hostname,
	}

	if len(c.Ports) > 0 {
		var ports []map[string]any
		for _, p := range c.Ports {
			ports = append(ports, map[string]any{
				"public":   p.PublicPort,
				"private":  p.PrivatePort,
				"protocol": p.Protocol,
			})
		}
		info["ports"] = ports
	}

	if len(c.Labels) > 0 {
		// Filter sensitive labels before sending to AI
		info["labels"] = filterSensitiveLabels(c.Labels)
	}

	if len(c.Mounts) > 0 {
		var mounts []string
		for _, m := range c.Mounts {
			mounts = append(mounts, m.Destination)
		}
		info["mounts"] = mounts
	}

	infoJSON, _ := json.MarshalIndent(info, "", "  ")

	return fmt.Sprintf(`Analyze this Docker container and identify what service it's running.

Container Information:
%s

Based on the image name, ports, labels, and mounts, determine:
1. What service/application is this?
2. What category does it belong to?
3. How should CLI commands be executed?

Respond in this exact JSON format:
{
  "service_type": "lowercase_type",
  "service_name": "Human Readable Name",
  "service_version": "version if detectable from image tag",
  "category": "database|web_server|cache|monitoring|backup|nvr|storage|container|network|security|media|home_automation|unknown",
  "cli_access": "docker exec {container} <cli-tool>",
  "facts": [],
  "config_paths": [],
  "data_paths": [],
  "log_paths": [],
  "ports": [],
  "confidence": 0.0-1.0,
  "reasoning": "Brief explanation"
}

Respond with ONLY valid JSON.`, string(infoJSON))
}

// buildDeepAnalysisPrompt builds a prompt for deep analysis with command outputs.
func (s *Service) buildDeepAnalysisPrompt(req AIAnalysisRequest) string {
	var sections []string

	sections = append(sections, fmt.Sprintf(`Resource Type: %s
Resource ID: %s
Host: %s (%s)`, req.ResourceType, req.ResourceID, req.Hostname, req.HostID))

	if len(req.Metadata) > 0 {
		metaJSON, _ := json.MarshalIndent(req.Metadata, "", "  ")
		sections = append(sections, fmt.Sprintf("Metadata:\n%s", string(metaJSON)))
	}

	if len(req.CommandOutputs) > 0 {
		sections = append(sections, "Command Outputs:")
		for name, output := range req.CommandOutputs {
			// Truncate long outputs
			if len(output) > 2000 {
				output = output[:2000] + "\n... (truncated)"
			}
			sections = append(sections, fmt.Sprintf("--- %s ---\n%s", name, output))
		}
	}

	// Use different prompts for HOST vs other resource types
	if req.ResourceType == ResourceTypeHost {
		return fmt.Sprintf(`Analyze this HOST system and provide detailed discovery information.

%s

IMPORTANT: This is a HOST discovery. Focus on identifying the HOST OPERATING SYSTEM and its primary role/purpose, NOT individual services or containers running on it.

Based on all available information, determine:
1. What is the host operating system? (e.g., Unraid, Proxmox, Ubuntu Server, Debian, TrueNAS)
2. What is the OS version?
3. What is the primary role/purpose of this host? (e.g., NAS, hypervisor, media server, backup server)
4. What are the key system paths?
5. What storage is available?
6. What services are running? (list as facts, not as the primary identification)

Respond in this exact JSON format:
{
  "service_type": "lowercase_os_type (e.g., unraid, proxmox, ubuntu, debian, truenas)",
  "service_name": "Human Readable OS Name and Role (e.g., Unraid NAS Server, Proxmox VE Hypervisor)",
  "service_version": "OS version number",
  "category": "storage|virtualizer|container|network|unknown",
  "cli_access": "ssh user@hostname",
  "facts": [
    {"category": "version|config|service|port|hardware|network|storage|dependency|security", "key": "fact_name", "value": "fact_value", "source": "command_name", "confidence": 0.9}
  ],
  "config_paths": ["/etc/", "/boot/config/"],
  "data_paths": ["/mnt/data", "/storage"],
  "log_paths": ["/var/log/"],
  "ports": [{"port": 22, "protocol": "tcp", "process": "sshd", "address": "0.0.0.0"}],
  "confidence": 0.0-1.0,
  "reasoning": "Explanation of host identification"
}

Important:
- The service_type and service_name MUST reflect the HOST OS, not services running on it
- List Docker containers, VMs, or other services as facts with category "service"
- Include storage information (disks, pools, arrays) as facts with category "storage"
- Include hardware info (CPU, RAM) as facts with category "hardware"

Respond with ONLY valid JSON.`, strings.Join(sections, "\n\n"))
	}

	return fmt.Sprintf(`Analyze this infrastructure resource and provide detailed discovery information.

%s

Based on all available information, determine:
1. What service/application is running?
2. What version is it?
3. What are the important configuration paths?
4. What data paths should be backed up?
5. What log paths are useful for troubleshooting?
6. What ports are in use?
7. Any special hardware (GPU, TPU, etc.)?
8. Any dependencies (databases, message queues, etc.)?

Respond in this exact JSON format:
{
  "service_type": "lowercase_type (e.g., frigate, postgres, pbs)",
  "service_name": "Human Readable Name",
  "service_version": "version number if found",
  "category": "database|web_server|cache|monitoring|backup|nvr|storage|container|virtualizer|network|security|media|home_automation|unknown",
  "cli_access": "command to access this service's CLI",
  "facts": [
    {"category": "version|config|service|port|hardware|network|storage|dependency|security", "key": "fact_name", "value": "fact_value", "source": "command_name", "confidence": 0.9}
  ],
  "config_paths": ["/path/to/config.yml"],
  "data_paths": ["/path/to/data"],
  "log_paths": ["/var/log/service/", "/path/to/app.log"],
  "ports": [{"port": 8080, "protocol": "tcp", "process": "nginx", "address": "0.0.0.0"}],
  "confidence": 0.0-1.0,
  "reasoning": "Explanation of identification"
}

Important:
- Extract version numbers from package lists, process output, or config files
- Identify config and data paths from mount points and file listings
- Identify log paths (e.g., /var/log/, application-specific logs) for troubleshooting
- Note any special hardware like Coral TPU, NVIDIA GPU
- For LXC/VM, the CLI access should use pct exec or qm guest exec
- For Docker, use docker exec

Respond with ONLY valid JSON.`, strings.Join(sections, "\n\n"))
}

// parseAIResponse parses the AI's JSON response.
func (s *Service) parseAIResponse(response string) *AIAnalysisResponse {
	log.Debug().Str("raw_response", response).Msg("Discovery raw response")
	response = strings.TrimSpace(response)

	// Handle markdown code blocks
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		var jsonLines []string
		inBlock := false
		for _, line := range lines {
			if strings.HasPrefix(line, "```") {
				inBlock = !inBlock
				continue
			}
			if inBlock {
				jsonLines = append(jsonLines, line)
			}
		}
		response = strings.Join(jsonLines, "\n")
	}

	// Find JSON object
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	var result AIAnalysisResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Debug().Err(err).Str("response", response).Msg("Failed to parse AI response")
		return nil
	}

	// Set discovered_at for facts
	now := time.Now()
	for i := range result.Facts {
		result.Facts[i].DiscoveredAt = now
	}

	return &result
}

// parseDockerMounts parses the docker_mounts command output into a slice of DockerBindMount.
// The output format is:
// CONTAINER:container_name
// source|destination|type
// source|destination|type
// CONTAINER:another_container
// source|destination|type
func parseDockerMounts(output string) []DockerBindMount {
	if output == "" || output == "no_docker_mounts" {
		return nil
	}

	var mounts []DockerBindMount
	var currentContainer string

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a container header
		if strings.HasPrefix(line, "CONTAINER:") {
			currentContainer = strings.TrimPrefix(line, "CONTAINER:")
			continue
		}

		// Skip if we don't have a current container
		if currentContainer == "" {
			continue
		}

		// Parse mount line: source|destination|type
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		mount := DockerBindMount{
			ContainerName: currentContainer,
			Source:        parts[0],
			Destination:   parts[1],
		}
		if len(parts) >= 3 {
			mount.Type = parts[2]
		}

		// Only include bind mounts and volumes (skip tmpfs, etc.)
		if mount.Type == "" || mount.Type == "bind" || mount.Type == "volume" {
			mounts = append(mounts, mount)
		}
	}

	return mounts
}

// GetDiscovery retrieves a discovery by ID.
func (s *Service) GetDiscovery(id string) (*ResourceDiscovery, error) {
	d, err := s.store.Get(id)
	if err != nil || d == nil {
		return d, err
	}
	s.upgradeCLIAccessIfNeeded(d)
	return d, nil
}

func (s *Service) GetDiscoveryByResource(resourceType ResourceType, hostID, resourceID string) (*ResourceDiscovery, error) {
	originalHostID := hostID
	originalResourceID := resourceID
	redirected := false

	// Redirect PVE node lookups to linked host agent if available
	// This ensures UI components looking up a PVE node by name (e.g. NodeDrawer) get the data associated with the Host Agent
	if resourceType == ResourceTypeHost && s.stateProvider != nil {
		state := s.stateProvider.GetState()
		for _, node := range state.Nodes {
			if node.Name == hostID || node.Name == resourceID || node.ID == resourceID {
				if node.LinkedHostAgentID != "" {
					log.Debug().
						Str("from_host", hostID).
						Str("to_agent", node.LinkedHostAgentID).
						Msg("Redirecting discovery lookup to linked host agent")
					hostID = node.LinkedHostAgentID
					resourceID = node.LinkedHostAgentID
					redirected = true
				}
				break
			}
		}
	}

	d, err := s.store.GetByResource(resourceType, hostID, resourceID)
	// If redirected and not found, try the original ID (fallback for unmigrated data)
	if (err != nil || d == nil) && redirected {
		log.Debug().
			Str("redirected_host", hostID).
			Str("original_host", originalHostID).
			Msg("Redirected lookup failed, trying fallback to original ID")
		dOriginal, errOriginal := s.store.GetByResource(resourceType, originalHostID, originalResourceID)
		if errOriginal == nil && dOriginal != nil {
			log.Debug().
				Str("original_host", originalHostID).
				Msg("Fallback lookup succeeded - returning legacy discovery")
			s.upgradeCLIAccessIfNeeded(dOriginal)
			return dOriginal, nil
		}
	}

	if err != nil || d == nil {
		return d, err
	}
	s.upgradeCLIAccessIfNeeded(d)
	return d, nil
}

// ListDiscoveries returns all discoveries.
func (s *Service) ListDiscoveries() ([]*ResourceDiscovery, error) {
	discoveries, err := s.store.List()
	if err != nil {
		return nil, err
	}
	discoveries = s.deduplicateDiscoveries(discoveries)
	for _, d := range discoveries {
		s.upgradeCLIAccessIfNeeded(d)
	}
	return discoveries, nil
}

// ListDiscoveriesByType returns discoveries for a specific resource type.
func (s *Service) ListDiscoveriesByType(resourceType ResourceType) ([]*ResourceDiscovery, error) {
	discoveries, err := s.store.ListByType(resourceType)
	if err != nil {
		return nil, err
	}
	discoveries = s.deduplicateDiscoveries(discoveries)
	for _, d := range discoveries {
		s.upgradeCLIAccessIfNeeded(d)
	}
	return discoveries, nil
}

// ListDiscoveriesByHost returns discoveries for a specific host.
func (s *Service) ListDiscoveriesByHost(hostID string) ([]*ResourceDiscovery, error) {
	discoveries, err := s.store.ListByHost(hostID)
	if err != nil {
		return nil, err
	}
	discoveries = s.deduplicateDiscoveries(discoveries)
	for _, d := range discoveries {
		s.upgradeCLIAccessIfNeeded(d)
	}
	return discoveries, nil
}

// deduplicateDiscoveries filters out redundant discoveries where a PVE node
// is represented by both its Node Name and its Linked Host Agent ID.
// The Host Agent ID is preferred.
func (s *Service) deduplicateDiscoveries(discoveries []*ResourceDiscovery) []*ResourceDiscovery {
	if s.stateProvider == nil {
		return discoveries
	}

	state := s.stateProvider.GetState()
	if len(state.Nodes) == 0 {
		return discoveries
	}

	// Map linked agent IDs to their PVE node source(s)
	// AgentID -> NodeName
	linkedAgents := make(map[string]string)
	for _, node := range state.Nodes {
		if node.LinkedHostAgentID != "" {
			linkedAgents[node.LinkedHostAgentID] = node.Name
		}
	}

	if len(linkedAgents) == 0 {
		return discoveries
	}

	// Check which agents actually have discovery data
	hasAgentDiscovery := make(map[string]bool)
	for _, d := range discoveries {
		if d.ResourceType == ResourceTypeHost {
			// d.HostID is usually the agent ID for host resources
			if _, ok := linkedAgents[d.HostID]; ok {
				hasAgentDiscovery[d.HostID] = true
			}
		}
	}

	// Filter out PVE node discoveries if the corresponding agent discovery exists
	filtered := make([]*ResourceDiscovery, 0, len(discoveries))
	for _, d := range discoveries {
		if d.ResourceType == ResourceTypeHost {
			// If this discovery is for a PVE node (by name/ID)
			// check if it maps to an agent that ALREADY has a discovery in this list

			// Is this discovery's ID satisfying a Node check?
			isPVENode := false
			var linkedAgentID string

			for _, node := range state.Nodes {
				if d.HostID == node.Name || d.HostID == node.ID || d.ResourceID == node.Name {
					isPVENode = true
					linkedAgentID = node.LinkedHostAgentID
					break
				}
			}

			if isPVENode && linkedAgentID != "" && hasAgentDiscovery[linkedAgentID] && d.HostID != linkedAgentID {
				// We have the agent discovery, so skip this redundant PVE node discovery
				continue
			}
		}
		filtered = append(filtered, d)
	}

	return filtered
}

// upgradeDiscoveryIfNeeded upgrades cached discovery fields to current versions.
// This ensures cached discoveries get the new instructional CLI access format
// and have hostname populated without requiring a full re-discovery.
func (s *Service) upgradeCLIAccessIfNeeded(d *ResourceDiscovery) {
	if d == nil {
		return
	}

	upgraded := false

	// Upgrade CLI access if version is outdated
	if d.CLIAccessVersion < CLIAccessVersion {
		oldCLI := d.CLIAccess
		d.CLIAccess = GetCLIAccessTemplate(d.ResourceType)
		d.CLIAccessVersion = CLIAccessVersion
		upgraded = true

		log.Debug().
			Str("id", d.ID).
			Str("old_cli", oldCLI).
			Str("new_cli", d.CLIAccess).
			Int("new_version", CLIAccessVersion).
			Msg("Upgraded CLI access pattern to new version")
	}

	// Fix empty hostname by looking up the resource name from state
	if d.Hostname == "" && s.stateProvider != nil {
		state := s.stateProvider.GetState()
		hostname := s.lookupHostnameFromState(d.ResourceType, d.HostID, d.ResourceID, state)
		if hostname != "" {
			d.Hostname = hostname
			upgraded = true
			log.Debug().
				Str("id", d.ID).
				Str("hostname", hostname).
				Msg("Populated missing hostname from state")
		}
	}

	_ = upgraded // Suppress unused variable warning if logging is disabled
}

// lookupHostnameFromState finds the hostname/name for a resource from state
func (s *Service) lookupHostnameFromState(resourceType ResourceType, hostID, resourceID string, state StateSnapshot) string {
	switch resourceType {
	case ResourceTypeLXC:
		for _, c := range state.Containers {
			if fmt.Sprintf("%d", c.VMID) == resourceID && c.Node == hostID {
				return c.Name
			}
		}
	case ResourceTypeVM:
		for _, vm := range state.VMs {
			if fmt.Sprintf("%d", vm.VMID) == resourceID && vm.Node == hostID {
				return vm.Name
			}
		}
	case ResourceTypeDocker:
		for _, host := range state.DockerHosts {
			if host.AgentID == hostID || host.Hostname == hostID {
				for _, c := range host.Containers {
					if c.Name == resourceID {
						return host.Hostname
					}
				}
			}
		}
	}
	return ""
}

// UpdateNotes updates user notes for a discovery.
func (s *Service) UpdateNotes(id string, notes string, secrets map[string]string) error {
	return s.store.UpdateNotes(id, notes, secrets)
}

// DeleteDiscovery deletes a discovery.
func (s *Service) DeleteDiscovery(id string) error {
	return s.store.Delete(id)
}

// GetProgress returns the progress of an ongoing discovery.
func (s *Service) GetProgress(resourceID string) *DiscoveryProgress {
	if s.scanner == nil {
		return nil
	}
	return s.scanner.GetProgress(resourceID)
}

// GetStatus returns the service status including fingerprint statistics.
func (s *Service) GetStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.cacheMu.RLock()
	cacheSize := len(s.analysisCache)
	s.cacheMu.RUnlock()

	// Get fingerprint stats
	fingerprintCount := 0
	var lastFingerprintScan time.Time
	if s.store != nil {
		fingerprintCount = s.store.GetFingerprintCount()
		lastFingerprintScan = s.store.GetLastFingerprintScan()
	}

	return map[string]any{
		"running":               s.running,
		"last_run":              s.lastRun,
		"interval":              s.interval.String(),
		"cache_size":            cacheSize,
		"ai_analyzer_set":       s.aiAnalyzer != nil,
		"scanner_set":           s.scanner != nil,
		"store_set":             s.store != nil,
		"deep_scan_timeout":     s.deepScanTimeout.String(),
		"max_discovery_age":     s.maxDiscoveryAge.String(),
		"fingerprint_count":     fingerprintCount,
		"last_fingerprint_scan": lastFingerprintScan,
	}
}

// GetMaxDiscoveryAge returns the current max discovery age (staleness threshold).
func (s *Service) GetMaxDiscoveryAge() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxDiscoveryAge
}

// SetMaxDiscoveryAge updates the max discovery age (staleness threshold).
// Discoveries older than this duration will be re-run when requested.
func (s *Service) SetMaxDiscoveryAge(age time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Enforce minimum of 1 day
	if age < 24*time.Hour {
		age = 24 * time.Hour
	}

	s.maxDiscoveryAge = age
	log.Info().Dur("max_discovery_age", age).Msg("Max discovery age updated")
}

// ClearCache clears the AI analysis cache.
func (s *Service) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.analysisCache = make(map[string]*analysisCacheEntry)
}

// --- AI Chat Integration Methods ---

// GetDiscoveryForAIChat returns discovery data for AI chat context.
// It will run discovery if needed (fingerprint changed or no data exists).
// This is the just-in-time discovery approach: only call AI when data is actually needed.
func (s *Service) GetDiscoveryForAIChat(ctx context.Context, resourceType ResourceType, hostID, resourceID string) (*ResourceDiscovery, error) {
	// This is the same as DiscoverResource but without Force
	return s.DiscoverResource(ctx, DiscoveryRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		HostID:       hostID,
		Force:        false, // Let fingerprint logic decide
	})
}

// GetDiscoveriesForAIContext returns discoveries for multiple resources.
// Used when AI chat needs context about the infrastructure.
// Only runs discovery for resources that actually need it (fingerprint changed).
func (s *Service) GetDiscoveriesForAIContext(ctx context.Context, resourceIDs []string) ([]*ResourceDiscovery, error) {
	var results []*ResourceDiscovery
	for _, id := range resourceIDs {
		resourceType, hostID, resourceID, err := ParseResourceID(id)
		if err != nil {
			log.Debug().Err(err).Str("id", id).Msg("Failed to parse resource ID for AI context")
			continue
		}
		discovery, err := s.GetDiscoveryForAIChat(ctx, resourceType, hostID, resourceID)
		if err != nil {
			log.Debug().Err(err).Str("id", id).Msg("Failed to get discovery for AI context")
			continue
		}
		if discovery != nil {
			results = append(results, discovery)
		}
	}
	return results, nil
}

// GetChangedResourceCount returns the count of resources whose fingerprint has changed
// since their last discovery.
func (s *Service) GetChangedResourceCount() (int, error) {
	if s.store == nil {
		return 0, nil
	}
	changed, err := s.store.GetChangedResources()
	if err != nil {
		return 0, err
	}
	return len(changed), nil
}

// GetStaleResourceCount returns the count of resources whose discovery is older
// than maxDiscoveryAge.
func (s *Service) GetStaleResourceCount() (int, error) {
	if s.store == nil {
		return 0, nil
	}
	stale, err := s.store.GetStaleResources(s.maxDiscoveryAge)
	if err != nil {
		return 0, err
	}
	return len(stale), nil
}
