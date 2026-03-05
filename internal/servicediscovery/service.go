// Package servicediscovery provides infrastructure discovery capabilities.
// It discovers services, versions, configurations, and CLI access methods
// for VMs, LXCs, Docker containers, Kubernetes pods, and hosts.
package servicediscovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
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

// StateSnapshot holds an infrastructure snapshot used internally by the
// discovery service. Previously a shadow StateProvider interface provided
// this; now it is built exclusively from ReadState typed views.
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
	ID            string
	Name          string
	LinkedAgentID string
}

// Host represents a host system (via pulse-agent host telemetry).
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

// WSBroadcaster provides WebSocket broadcasting capabilities.
type WSBroadcaster interface {
	BroadcastDiscoveryProgress(progress *DiscoveryProgress)
}

// Service manages infrastructure discovery.
type Service struct {
	store      *Store
	scanner    *DeepScanner
	readState  unifiedresources.ReadState // Typed state access (sole source since SRC-03l)
	aiAnalyzer AIAnalyzer
	wsHub      WSBroadcaster // WebSocket hub for broadcasting progress

	mu                sync.RWMutex
	running           bool
	stopping          bool
	stopCh            chan struct{}
	loopDone          chan struct{}
	runCancel         context.CancelFunc
	intervalCh        chan time.Duration // Channel for live interval updates
	interval          time.Duration
	initialDelay      time.Duration
	lastRun           time.Time
	deepScanTimeout   time.Duration // Timeout for individual deep scans
	aiAnalysisTimeout time.Duration // Timeout for individual AI analysis calls
	maxDiscoveryAge   time.Duration // Max age before rediscovery (default 30 days)

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
	DataDir           string
	Interval          time.Duration // How often to run fingerprint collection (default 5 min)
	CacheExpiry       time.Duration // How long to cache AI analysis results
	DeepScanTimeout   time.Duration // Timeout for individual deep scans (default 60s)
	AIAnalysisTimeout time.Duration // Timeout for individual AI analysis calls (default 45s)

	// Fingerprint-based discovery settings
	MaxDiscoveryAge     time.Duration // Rediscover after this duration (default 30 days)
	FingerprintInterval time.Duration // How often to collect fingerprints (default 5 min)
}

const (
	defaultDiscoveryInterval     = 5 * time.Minute
	defaultDiscoveryCacheExpiry  = 1 * time.Hour
	defaultDiscoveryScanTimeout  = 60 * time.Second
	defaultDiscoveryMaxAge       = 30 * 24 * time.Hour
	minDiscoveryMaxAge           = 24 * time.Hour
	defaultDiscoveryInitialDelay = 30 * time.Second
)

// DefaultConfig returns the default discovery configuration.
func DefaultConfig() Config {
	return Config{
		Interval:            defaultDiscoveryInterval, // Fingerprint collection interval
		CacheExpiry:         defaultDiscoveryCacheExpiry,
		DeepScanTimeout:     defaultDiscoveryScanTimeout,
		MaxDiscoveryAge:     defaultDiscoveryMaxAge,
		FingerprintInterval: defaultDiscoveryInterval,
	}
}

func normalizeServiceConfig(cfg Config) Config {
	if cfg.Interval <= 0 {
		log.Warn().Dur("interval", cfg.Interval).Dur("default", defaultDiscoveryInterval).Msg("Invalid discovery interval; using default")
		cfg.Interval = defaultDiscoveryInterval
	}
	if cfg.CacheExpiry <= 0 {
		log.Warn().Dur("cache_expiry", cfg.CacheExpiry).Dur("default", defaultDiscoveryCacheExpiry).Msg("Invalid discovery cache expiry; using default")
		cfg.CacheExpiry = defaultDiscoveryCacheExpiry
	}
	if cfg.DeepScanTimeout <= 0 {
		log.Warn().Dur("deep_scan_timeout", cfg.DeepScanTimeout).Dur("default", defaultDiscoveryScanTimeout).Msg("Invalid deep scan timeout; using default")
		cfg.DeepScanTimeout = defaultDiscoveryScanTimeout
	}
	switch {
	case cfg.MaxDiscoveryAge <= 0:
		log.Warn().Dur("max_discovery_age", cfg.MaxDiscoveryAge).Dur("default", defaultDiscoveryMaxAge).Msg("Invalid max discovery age; using default")
		cfg.MaxDiscoveryAge = defaultDiscoveryMaxAge
	case cfg.MaxDiscoveryAge < minDiscoveryMaxAge:
		log.Warn().Dur("max_discovery_age", cfg.MaxDiscoveryAge).Dur("minimum", minDiscoveryMaxAge).Msg("Max discovery age below minimum; clamping")
		cfg.MaxDiscoveryAge = minDiscoveryMaxAge
	}
	return cfg
}

func normalizeDiscoveryInterval(interval time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	log.Warn().Dur("interval", interval).Dur("default", defaultDiscoveryInterval).Msg("Invalid discovery interval; using default")
	return defaultDiscoveryInterval
}

func normalizeDeepScanTimeout(timeout time.Duration) time.Duration {
	if timeout > 0 {
		return timeout
	}
	log.Warn().Dur("deep_scan_timeout", timeout).Dur("default", defaultDiscoveryScanTimeout).Msg("Invalid deep scan timeout; using default")
	return defaultDiscoveryScanTimeout
}

// NewService creates a new discovery service.
func NewService(store *Store, scanner *DeepScanner, cfg Config) *Service {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.CacheExpiry == 0 {
		cfg.CacheExpiry = 1 * time.Hour
	}
	if cfg.DeepScanTimeout == 0 {
		cfg.DeepScanTimeout = 60 * time.Second
	}
	if cfg.AIAnalysisTimeout <= 0 {
		cfg.AIAnalysisTimeout = 45 * time.Second
	}
	if cfg.MaxDiscoveryAge == 0 {
		cfg.MaxDiscoveryAge = 30 * 24 * time.Hour // 30 days
	}

	return &Service{
		store:             store,
		scanner:           scanner,
		interval:          cfg.Interval,
		initialDelay:      30 * time.Second,
		cacheExpiry:       cfg.CacheExpiry,
		deepScanTimeout:   cfg.DeepScanTimeout,
		aiAnalysisTimeout: cfg.AIAnalysisTimeout,
		maxDiscoveryAge:   cfg.MaxDiscoveryAge,
		stopCh:            make(chan struct{}),
		intervalCh:        make(chan time.Duration, 1), // Buffered to prevent blocking
		analysisCache:     make(map[string]*analysisCacheEntry),
		inProgress:        make(map[string]*discoveryInProgress),
	}
}

// SetAIAnalyzer sets the AI analyzer for discovery.
func (s *Service) SetAIAnalyzer(analyzer AIAnalyzer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiAnalyzer = analyzer
}

// SetReadState sets the typed ReadState provider for the discovery service.
// When set, getSnapshot() uses ReadState to build infrastructure snapshots.
func (s *Service) SetReadState(rs unifiedresources.ReadState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.readState = rs
}

// Start begins the background discovery service.
func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running || s.stopping {
		s.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.running = true
	stopCh := make(chan struct{})
	loopDone := make(chan struct{})
	s.stopCh = stopCh
	s.loopDone = loopDone
	s.runCancel = cancel
	s.mu.Unlock()

	log.Info().
		Dur("interval", s.interval).
		Msg("Starting infrastructure discovery service")

	go s.runDiscoveryLoop(runCtx, stopCh, loopDone)
}

// Stop stops the background discovery service.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	stopCh := s.stopCh
	loopDone := s.loopDone
	runCancel := s.runCancel
	s.stopCh = nil
	s.loopDone = nil
	s.runCancel = nil
	s.mu.Unlock()

	if runCancel != nil {
		runCancel()
	}
	if stopCh != nil {
		close(stopCh)
	}
	if loopDone != nil {
		<-loopDone
	}
}

// SetInterval updates the scan interval. Takes effect immediately if running.
func (s *Service) SetInterval(interval time.Duration) {
	normalizedInterval := normalizeDiscoveryInterval(interval)

	s.mu.Lock()
	s.interval = normalizedInterval
	running := s.running
	s.mu.Unlock()

	// If running, send the new interval to the loop (non-blocking)
	if running {
		select {
		case s.intervalCh <- normalizedInterval:
			log.Info().Dur("interval", normalizedInterval).Msg("Discovery interval updated (live)")
		default:
			// Channel full, interval will be picked up eventually
			log.Debug().Dur("interval", normalizedInterval).Msg("Discovery interval updated (pending)")
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

	log.Info().Msg("webSocket hub connected to discovery service")
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

// discoveryLoop runs periodic fingerprint collection and automatic refreshes.
// Fingerprints detect changes cheaply; changed/stale/new resources are then refreshed.
func (s *Service) discoveryLoop(ctx context.Context) {
	s.mu.RLock()
	stopCh := s.stopCh
	s.mu.RUnlock()
	s.runDiscoveryLoop(ctx, stopCh, nil)
}

func (s *Service) runDiscoveryLoop(ctx context.Context, stopCh <-chan struct{}, done chan<- struct{}) {
	if done != nil {
		defer close(done)
	}

	delay := s.initialDelay
	if delay <= 0 {
		delay = defaultDiscoveryInitialDelay
	}

	startupTimer := time.NewTimer(delay)
	defer startupTimer.Stop()

	// Run initial fingerprint collection after a short delay
	select {
	case <-startupTimer.C:
	case <-stopCh:
		return
	case <-ctx.Done():
		return
	}

	s.collectFingerprints(ctx)
	s.runAutomaticDiscoveryRefresh(ctx)

	s.mu.RLock()
	currentInterval := s.interval
	s.mu.RUnlock()
	currentInterval = normalizeDiscoveryInterval(currentInterval)

	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.collectFingerprints(ctx)
			s.runAutomaticDiscoveryRefresh(ctx)
		case newInterval := <-s.intervalCh:
			// Interval changed - reset the ticker
			newInterval = normalizeDiscoveryInterval(newInterval)
			ticker.Stop()
			ticker = time.NewTicker(newInterval)
			log.Info().Dur("interval", newInterval).Msg("Fingerprint collection interval reset")
		case <-stopCh:
			log.Info().Msg("Stopping discovery service")
			return
		case <-ctx.Done():
			log.Info().Msg("discovery context cancelled")
			return
		}
	}
}

func (s *Service) finishDiscoveryLoop(stopCh <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Only clear lifecycle state if this is still the active run.
	if s.stopCh != nil && s.stopCh != stopCh {
		return
	}
	s.running = false
	s.stopping = false
	s.stopCh = nil
	s.loopDone = nil
}

func (s *Service) runAutomaticDiscoveryRefresh(ctx context.Context) {
	if ctx == nil || ctx.Err() != nil || s.store == nil {
		return
	}

	s.mu.RLock()
	analyzerConfigured := s.aiAnalyzer != nil
	maxDiscoveryAge := s.maxDiscoveryAge
	s.mu.RUnlock()
	if !analyzerConfigured {
		log.Debug().Msg("skipping automatic discovery refresh - AI analyzer not configured")
		return
	}

	changedResources, err := s.store.GetChangedResources()
	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch changed resources for automatic discovery refresh")
		return
	}
	staleResources, err := s.store.GetStaleResources(maxDiscoveryAge)
	if err != nil {
		log.Warn().Err(err).Msg("failed to fetch stale resources for automatic discovery refresh")
		return
	}

	candidates := make(map[string]struct{}, len(changedResources)+len(staleResources))
	for _, id := range changedResources {
		if strings.TrimSpace(id) != "" {
			candidates[id] = struct{}{}
		}
	}
	for _, id := range staleResources {
		if strings.TrimSpace(id) != "" {
			candidates[id] = struct{}{}
		}
	}
	if len(candidates) == 0 {
		return
	}

	resourceIDs := make([]string, 0, len(candidates))
	for id := range candidates {
		resourceIDs = append(resourceIDs, id)
	}
	sort.Strings(resourceIDs)

	log.Info().
		Int("changed", len(changedResources)).
		Int("stale", len(staleResources)).
		Int("total", len(resourceIDs)).
		Msg("Running automatic discovery refresh for changed/stale resources")

	discoveredCount := 0
	failedCount := 0

	for _, id := range resourceIDs {
		if ctx.Err() != nil {
			break
		}

		resourceType, targetID, resourceID, err := ParseResourceID(id)
		if err != nil {
			failedCount++
			log.Warn().
				Err(err).
				Str("resource_id", id).
				Msg("Skipping invalid resource ID during automatic discovery refresh")
			continue
		}

		_, err = s.DiscoverResource(ctx, DiscoveryRequest{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			TargetID:     targetID,
			HostID:       targetID,
			Hostname:     targetID,
		})
		if err != nil {
			failedCount++
			log.Warn().
				Err(err).
				Str("resource_id", id).
				Str("resource_type", string(resourceType)).
				Msg("Automatic discovery refresh failed for resource")
			continue
		}
		discoveredCount++
	}

	log.Info().
		Int("discovered", discoveredCount).
		Int("failed", failedCount).
		Msg("Automatic discovery refresh completed")
}

// getSnapshot returns the current infrastructure state from ReadState.
// Returns an empty snapshot and false if ReadState is not yet configured.
func (s *Service) getSnapshot() (StateSnapshot, bool) {
	rs := s.getReadState()
	if rs != nil {
		return s.snapshotFromReadState(rs), true
	}
	return StateSnapshot{}, false
}

// getReadState returns the ReadState provider, safely reading it under the
// service lock to avoid races with concurrent SetReadState calls.
func (s *Service) getReadState() unifiedresources.ReadState {
	s.mu.RLock()
	rs := s.readState
	s.mu.RUnlock()
	return rs
}

// hasStateAccess returns true when ReadState is configured. Safe for
// concurrent use.
func (s *Service) hasStateAccess() bool {
	return s.getReadState() != nil
}

// snapshotFromReadState converts ReadState typed views into the local
// StateSnapshot type used by servicediscovery functions.
func (s *Service) snapshotFromReadState(rs unifiedresources.ReadState) StateSnapshot {
	// VMs
	vmViews := rs.VMs()
	vms := make([]VM, 0, len(vmViews))
	for _, v := range vmViews {
		vms = append(vms, VM{
			VMID:        v.VMID(),
			Name:        v.Name(),
			Node:        v.Node(),
			Status:      string(v.Status()),
			Instance:    v.Instance(),
			IPAddresses: v.IPAddresses(),
		})
	}

	// LXC containers
	ctViews := rs.Containers()
	containers := make([]Container, 0, len(ctViews))
	for _, v := range ctViews {
		containers = append(containers, Container{
			VMID:        v.VMID(),
			Name:        v.Name(),
			Node:        v.Node(),
			Status:      string(v.Status()),
			Instance:    v.Instance(),
			IPAddresses: v.IPAddresses(),
		})
	}

	// Docker hosts — build host → children map from flat DockerContainers list
	dcViews := rs.DockerContainers()
	childrenByParent := make(map[string][]DockerContainer, len(dcViews))
	for _, dc := range dcViews {
		parentID := dc.ParentID()
		ports := dc.Ports()
		sdPorts := make([]DockerPort, 0, len(ports))
		for _, p := range ports {
			sdPorts = append(sdPorts, DockerPort{
				PublicPort:  p.PublicPort,
				PrivatePort: p.PrivatePort,
				Protocol:    p.Protocol,
			})
		}
		mounts := dc.Mounts()
		sdMounts := make([]DockerMount, 0, len(mounts))
		for _, m := range mounts {
			sdMounts = append(sdMounts, DockerMount{
				Source:      m.Source,
				Destination: m.Destination,
			})
		}
		childrenByParent[parentID] = append(childrenByParent[parentID], DockerContainer{
			ID:     dc.ContainerID(),
			Name:   dc.Name(),
			Image:  dc.Image(),
			Status: string(dc.Status()),
			Ports:  sdPorts,
			Labels: dc.Labels(),
			Mounts: sdMounts,
		})
	}

	dhViews := rs.DockerHosts()
	dockerHosts := make([]DockerHost, 0, len(dhViews))
	for _, dh := range dhViews {
		dockerHosts = append(dockerHosts, DockerHost{
			AgentID:    dh.AgentID(),
			Hostname:   dh.Hostname(),
			Containers: childrenByParent[dh.ID()],
		})
	}

	// Hosts
	// Note: CPUCount is not available via HostView (not mapped in unifiedresources
	// AgentData). This is a known data gap tracked as SRC-01c; it only affects the
	// "cpu_count" metadata field in AI analysis prompts.
	hViews := rs.Hosts()
	hosts := make([]Host, 0, len(hViews))
	for _, h := range hViews {
		// Use AgentID (original agent ID) rather than the registry hash
		// ID, because discovery lookup code matches against request IDs which
		// use the source-level host agent ID.
		hostID := h.AgentID()
		if hostID == "" {
			hostID = h.ID()
		}
		hosts = append(hosts, Host{
			ID:            hostID,
			Hostname:      h.Hostname(),
			DisplayName:   h.Name(),
			Platform:      h.Platform(),
			OSName:        h.OSName(),
			OSVersion:     h.OSVersion(),
			KernelVersion: h.KernelVersion(),
			Architecture:  h.Architecture(),
			Status:        string(h.Status()),
			Tags:          h.Tags(),
		})
	}

	// Nodes
	nViews := rs.Nodes()
	nodes := make([]Node, 0, len(nViews))
	for _, n := range nViews {
		// Use SourceID (original Proxmox node ID) rather than the registry
		// hash ID, because discovery lookup code matches against request IDs
		// which use the source-level node ID.
		nodeID := n.SourceID()
		if nodeID == "" {
			nodeID = n.ID()
		}
		nodes = append(nodes, Node{
			ID:            nodeID,
			Name:          n.Name(),
			LinkedAgentID: n.LinkedAgentID(),
		})
	}

	// Kubernetes clusters (ReadState provides K8sClusters + Pods separately).
	// Note: PodView does not expose NodeName or sub-container details; these
	// fields are zero-valued.
	clusterViews := rs.K8sClusters()
	podViews := rs.Pods()
	podsByCluster := make(map[string][]KubernetesPod, len(podViews))
	for _, pv := range podViews {
		parentID := pv.ParentID()
		podsByCluster[parentID] = append(podsByCluster[parentID], KubernetesPod{
			UID:       pv.PodUID(),
			Name:      pv.Name(),
			Namespace: pv.Namespace(),
			Phase:     pv.PodPhase(),
			Labels:    pv.Labels(),
			OwnerKind: pv.OwnerKind(),
			OwnerName: pv.OwnerName(),
		})
	}
	clusters := make([]KubernetesCluster, 0, len(clusterViews))
	for _, cv := range clusterViews {
		clusters = append(clusters, KubernetesCluster{
			ID:      cv.ID(),
			Name:    cv.Name(),
			AgentID: cv.AgentID(),
			Status:  string(cv.Status()),
			Pods:    podsByCluster[cv.ID()],
		})
	}

	return StateSnapshot{
		VMs:                vms,
		Containers:         containers,
		DockerHosts:        dockerHosts,
		Hosts:              hosts,
		Nodes:              nodes,
		KubernetesClusters: clusters,
	}
}

// collectFingerprints collects fingerprints from all resources (Docker, LXC, VM).
// This is metadata-only and does not invoke the AI analyzer.
func (s *Service) collectFingerprints(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Stack().Msg("recovered from panic in fingerprint collection")
		}
	}()

	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()

	snap, ok := s.getSnapshot()
	if !ok {
		return
	}
	changedCount := 0
	newCount := 0

	// Process Docker containers
	for _, host := range snap.DockerHosts {
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
			oldFP, err := s.store.GetFingerprint(fpKey)
			if err != nil {
				log.Warn().
					Err(err).
					Str("resource_id", fpKey).
					Str("container", container.Name).
					Msg("Failed to load previous Docker fingerprint")
			}

			// Update the fingerprint's ResourceID to include prefix for storage
			newFP.ResourceID = fpKey

			// Save new fingerprint
			if err := s.store.SaveFingerprint(newFP); err != nil {
				log.Warn().Err(err).Str("container", container.Name).Msg("failed to save Docker fingerprint")
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

	// Process system containers (LXC)
	lxcNew, lxcChanged := s.processFingerprint(ctx, GenerateLXCFingerprint, "system-container", "system-container:", snap.Containers)
	newCount += lxcNew
	changedCount += lxcChanged

	// Process VMs
	vmNew, vmChanged := s.processFingerprint(ctx, GenerateVMFingerprint, "vm", "vm:", snap.VMs)
	newCount += vmNew
	changedCount += vmChanged

	// Process Kubernetes pods
	for _, cluster := range snap.KubernetesClusters {
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
			oldFP, err := s.store.GetFingerprint(fpKey)
			if err != nil {
				log.Warn().
					Err(err).
					Str("resource_id", fpKey).
					Str("pod", pod.Name).
					Str("namespace", pod.Namespace).
					Msg("Failed to load previous K8s pod fingerprint")
			}

			// Update the fingerprint's ResourceID to include prefix for storage
			newFP.ResourceID = fpKey

			// Save new fingerprint
			if err := s.store.SaveFingerprint(newFP); err != nil {
				log.Warn().Err(err).Str("pod", pod.Name).Str("namespace", pod.Namespace).Msg("failed to save K8s pod fingerprint")
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
	s.cleanupOrphanedData(snap)
}

// processOrphanedDataFingerprint processes fingerprints for LXC containers and VMs.
func (s *Service) processFingerprint(
	ctx context.Context,
	generateFP interface{},
	resourceType string,
	prefix string,
	items interface{},
) (int, int) {
	var changedCount, newCount int

	fpFuncVal := reflect.ValueOf(generateFP)
	if fpFuncVal.Kind() != reflect.Func {
		return 0, 0
	}

	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		return 0, 0
	}

	for i := 0; i < v.Len(); i++ {
		select {
		case <-ctx.Done():
			return newCount, changedCount
		default:
		}

		item := v.Index(i).Interface()
		itemVal := reflect.ValueOf(item)

		node := itemVal.FieldByName("Node").String()
		name := itemVal.FieldByName("Name").String()
		vmid := itemVal.FieldByName("VMID").Int()

		args := []reflect.Value{reflect.ValueOf(node), reflect.ValueOf(item)}
		newFP := fpFuncVal.Call(args)[0].Interface().(*ContainerFingerprint)
		fpKey := prefix + node + ":" + newFP.ResourceID

		oldFP, err := s.store.GetFingerprint(fpKey)
		if err != nil {
			log.Warn().
				Err(err).
				Str("resource_id", fpKey).
				Str(resourceType, name).
				Int("vmid", int(vmid)).
				Msg("Failed to load previous " + resourceType + " fingerprint")
		}

		newFP.ResourceID = fpKey

		if err := s.store.SaveFingerprint(newFP); err != nil {
			log.Warn().Err(err).Str(resourceType, name).Msg("failed to save " + resourceType + " fingerprint")
			continue
		}

		if oldFP == nil {
			newCount++
			log.Debug().
				Str("type", resourceType).
				Str("name", name).
				Int("vmid", int(vmid)).
				Str("hash", newFP.Hash).
				Msg("New fingerprint captured")
		} else if newFP.HasSchemaChanged(oldFP) {
			log.Debug().
				Str("type", resourceType).
				Str("name", name).
				Int("vmid", int(vmid)).
				Int("old_schema", oldFP.SchemaVersion).
				Int("new_schema", newFP.SchemaVersion).
				Msg("Fingerprint schema updated")
		} else if oldFP.Hash != newFP.Hash {
			changedCount++
			log.Info().
				Str("type", resourceType).
				Str("name", name).
				Int("vmid", int(vmid)).
				Str("old_hash", oldFP.Hash).
				Str("new_hash", newFP.Hash).
				Msg("Fingerprint changed - discovery will run on next request")
		}
	}

	return newCount, changedCount
}

// cleanupOrphanedData removes fingerprints and discoveries for resources that no longer exist.
func (s *Service) cleanupOrphanedData(snap StateSnapshot) {
	// Safety check: Don't cleanup if state appears empty
	// This prevents catastrophic deletion if state provider has an error
	totalResources := len(snap.Containers) + len(snap.VMs) + len(snap.KubernetesClusters)
	for _, host := range snap.DockerHosts {
		totalResources += len(host.Containers)
	}
	if totalResources == 0 {
		log.Debug().Msg("skipping orphaned data cleanup - state is empty (may be an error)")
		return
	}

	// Build set of current resource IDs
	currentIDs := make(map[string]bool)

	// Docker containers
	for _, host := range snap.DockerHosts {
		for _, container := range host.Containers {
			fpKey := "docker:" + host.AgentID + ":" + container.Name
			currentIDs[fpKey] = true
		}
	}

	// System containers
	for _, ct := range snap.Containers {
		fpKey := "system-container:" + ct.Node + ":" + strconv.Itoa(ct.VMID)
		currentIDs[fpKey] = true
	}

	// VMs
	for _, vm := range snap.VMs {
		fpKey := "vm:" + vm.Node + ":" + strconv.Itoa(vm.VMID)
		currentIDs[fpKey] = true
	}

	// Kubernetes pods
	for _, cluster := range snap.KubernetesClusters {
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
		log.Debug().Msg("aI analyzer not set, skipping Docker discovery")
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
			existing, err := s.store.Get(id)
			if err != nil {
				log.Warn().Err(err).Str("id", id).Msg("Failed to load existing discovery before shallow analysis")
			}

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
					log.Warn().Err(err).Str("id", id).Msg("failed to save discovery")
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
	timeout = normalizeDeepScanTimeout(timeout)

	if s.scanner == nil || analyzer == nil {
		return discovery
	}

	// Create a timeout context for the deep scan
	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req := DiscoveryRequest{
		ResourceType: discovery.ResourceType,
		ResourceID:   discovery.ResourceID,
		TargetID:     discovery.TargetID,
		Hostname:     discovery.Hostname,
	}

	scanResult, err := s.scanner.Scan(scanCtx, req)
	if err != nil {
		log.Debug().Err(err).Str("id", discovery.ID).Msg("deep scan failed during background discovery")
		return discovery
	}

	if len(scanResult.CommandOutputs) == 0 {
		return discovery
	}

	// Build analysis request with command outputs
	targetID := canonicalDiscoveryTargetID(discovery)
	analysisReq := AIAnalysisRequest{
		ResourceType:   discovery.ResourceType,
		ResourceID:     discovery.ResourceID,
		TargetID:       targetID,
		Hostname:       discovery.Hostname,
		CommandOutputs: scanResult.CommandOutputs,
	}

	// Add metadata if available
	if s.hasStateAccess() {
		analysisReq.Metadata = s.getResourceMetadata(req)
	}

	// Build prompt and analyze
	prompt := s.buildDeepAnalysisPrompt(analysisReq)
	response, err := analyzer.AnalyzeForDiscovery(scanCtx, prompt)
	if err != nil {
		log.Debug().Err(err).Str("id", discovery.ID).Msg("deep analysis failed during background discovery")
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
	if ctx == nil {
		ctx = context.Background()
	}

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

		analyzeCtx, cancel := context.WithTimeout(ctx, s.aiAnalysisTimeout)
		defer cancel()

		response, err := analyzer.AnalyzeForDiscovery(analyzeCtx, prompt)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Warn().
					Err(err).
					Str("container", c.Name).
					Dur("timeout", s.aiAnalysisTimeout).
					Msg("AI metadata analysis timed out")
				return nil
			}
			log.Warn().Err(err).Str("container", c.Name).Msg("aI analysis failed")
			return nil
		}

		result = s.parseAIResponse(response)
		if result == nil {
			log.Warn().Str("container", c.Name).Msg("failed to parse AI response")
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
	if ctx == nil {
		ctx = context.Background()
	}

	legacyHostID := strings.TrimSpace(req.HostID)
	req = normalizeDiscoveryRequestAliases(req)

	originalReq := req
	aliasIDs := make([]string, 0, 2)
	if req.TargetID != "" && req.ResourceID != "" {
		aliasIDs = append(aliasIDs, MakeResourceID(req.ResourceType, req.TargetID, req.ResourceID))
	}
	if legacyHostID != "" && req.ResourceID != "" && legacyHostID != req.TargetID {
		aliasIDs = append(aliasIDs, MakeResourceID(req.ResourceType, legacyHostID, req.ResourceID))
	}
	req = s.normalizeDiscoveryRequest(req, &aliasIDs)

	resourceID := MakeResourceID(req.ResourceType, req.TargetID, req.ResourceID)

	// Get current fingerprint (if available)
	// Fingerprint key matches the resource ID format: type:scope:id
	currentFP, err := s.store.GetFingerprint(resourceID)
	if err != nil {
		log.Warn().Err(err).Str("id", resourceID).Msg("Failed to load current fingerprint; continuing without fingerprint check")
	}

	// Get existing discovery
	existing, err := s.store.Get(resourceID)
	if err != nil {
		log.Warn().Err(err).Str("id", resourceID).Msg("Failed to load existing discovery; running fresh discovery")
	}

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
		s.upgradeCLIAccessIfNeeded(existing)
		log.Debug().Str("id", resourceID).Msg("discovery still valid, returning cached")
		return existing, nil
	}

	// Check for duplicate concurrent discovery requests
	s.inProgressMu.Lock()
	if inProg, ok := s.inProgress[resourceID]; ok {
		// Discovery already in progress - wait for it
		s.inProgressMu.Unlock()
		log.Debug().Str("id", resourceID).Msg("discovery already in progress, waiting for result")

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

	log.Info().Str("id", resourceID).Str("reason", reason).Msg("running discovery")

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
		TargetID:     req.TargetID,
		Hostname:     req.Hostname,
	}

	if scanResult != nil {
		analysisReq.CommandOutputs = scanResult.CommandOutputs
	}

	// Add metadata if available
	if s.hasStateAccess() {
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

	analyzeCtx, cancel := context.WithTimeout(ctx, s.aiAnalysisTimeout)
	defer cancel()

	response, err := analyzer.AnalyzeForDiscovery(analyzeCtx, prompt)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			inProg.err = fmt.Errorf("AI analysis timed out after %s", s.aiAnalysisTimeout)
			return nil, inProg.err
		}
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
		TargetID:         req.TargetID,
		HostID:           req.TargetID,
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

	// Suggest web interface URL based on service type and external IP.
	// If no URL can be inferred, capture diagnostics for logs and UI.
	urlSuggestionDiagnostic := ""
	urlSuggestionSourceCode := ""
	urlSuggestionSourceDetail := ""
	if !s.hasStateAccess() {
		urlSuggestionDiagnostic = "ReadState unavailable"
	} else {
		externalIP := s.getResourceExternalIP(req)
		if externalIP == "" {
			urlSuggestionDiagnostic = "no host or IP candidate available"
		} else {
			primaryURL, primaryCode, primaryDetail := suggestWebURLWithReason(discovery, externalIP)
			discovery.SuggestedURL = primaryURL
			if discovery.SuggestedURL != "" {
				urlSuggestionSourceCode = primaryCode
				urlSuggestionSourceDetail = primaryDetail
			} else {
				fallbackURL, fallbackCode, fallbackDetail := s.suggestHostManagementURLWithReason(req, externalIP)
				discovery.SuggestedURL = fallbackURL
				if discovery.SuggestedURL != "" {
					urlSuggestionSourceCode = fallbackCode
					urlSuggestionSourceDetail = fallbackDetail
				} else {
					urlSuggestionDiagnostic = formatURLSuggestionDiagnostic(primaryCode, primaryDetail, fallbackCode, fallbackDetail)
					log.Debug().
						Str("id", discovery.ID).
						Str("resource_type", string(req.ResourceType)).
						Str("host", externalIP).
						Str("primary_reason_code", primaryCode).
						Str("fallback_reason_code", fallbackCode).
						Str("diagnostic", urlSuggestionDiagnostic).
						Msg("Unable to infer suggested URL")
				}
			}
		}
	}
	discovery.SuggestedURLSourceCode = urlSuggestionSourceCode
	discovery.SuggestedURLSourceDetail = urlSuggestionSourceDetail
	discovery.SuggestedURLDiagnostic = urlSuggestionDiagnostic

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
	s.cleanupAliasedDiscoveries(resourceID, aliasIDs)

	// Store result for any waiting goroutines
	inProg.result = discovery
	if originalReq != req {
		originalTargetID := canonicalRequestTargetID(originalReq)
		log.Debug().
			Str("original_id", MakeResourceID(originalReq.ResourceType, originalTargetID, originalReq.ResourceID)).
			Str("canonical_id", resourceID).
			Msg("Discovery request canonicalized")
	}
	return discovery, nil
}

// normalizeDiscoveryRequest resolves discovery aliases to a canonical target ID.
// This prevents duplicate discoveries for the same physical host under different IDs.
func (s *Service) normalizeDiscoveryRequest(req DiscoveryRequest, aliasIDs *[]string) DiscoveryRequest {
	req = normalizeDiscoveryRequestAliases(req)
	requestTargetID := req.TargetID

	if req.ResourceType != ResourceTypeAgent {
		return req
	}
	snap, ok := s.getSnapshot()
	if !ok {
		return req
	}
	addAlias := func(hostID, resourceID string) {
		if hostID == "" || resourceID == "" {
			return
		}
		id := MakeResourceID(ResourceTypeAgent, hostID, resourceID)
		for _, existing := range *aliasIDs {
			if existing == id {
				return
			}
		}
		*aliasIDs = append(*aliasIDs, id)
	}

	for _, host := range snap.Hosts {
		if host.ID == requestTargetID || host.ID == req.ResourceID || host.Hostname == requestTargetID || host.Hostname == req.ResourceID || (req.Hostname != "" && host.Hostname == req.Hostname) {
			addAlias(host.ID, host.ID)
			addAlias(host.Hostname, host.Hostname)
			if req.Hostname == "" {
				req.Hostname = host.Hostname
			}
			req.TargetID = host.ID
			req.HostID = req.TargetID
			req.ResourceID = host.ID
			return req
		}
	}

	for _, node := range snap.Nodes {
		if node.Name == requestTargetID || node.Name == req.ResourceID || node.ID == requestTargetID || node.ID == req.ResourceID || (req.Hostname != "" && node.Name == req.Hostname) {
			addAlias(node.Name, node.Name)
			addAlias(node.ID, node.ID)
			if req.Hostname == "" {
				req.Hostname = node.Name
			}
			if node.LinkedAgentID != "" {
				log.Info().
					Str("from_target", requestTargetID).
					Str("to_agent", node.LinkedAgentID).
					Msg("Redirecting discovery scan to linked host agent")
				addAlias(node.LinkedAgentID, node.LinkedAgentID)
				req.TargetID = node.LinkedAgentID
				req.HostID = req.TargetID
				req.ResourceID = node.LinkedAgentID
				return req
			}
			req.TargetID = node.Name
			req.HostID = req.TargetID
			req.ResourceID = node.Name
			return req
		}
	}

	return req
}

func (s *Service) cleanupAliasedDiscoveries(canonicalID string, aliasIDs []string) {
	seen := make(map[string]struct{}, len(aliasIDs))
	for _, aliasID := range aliasIDs {
		if aliasID == "" || aliasID == canonicalID {
			continue
		}
		if _, ok := seen[aliasID]; ok {
			continue
		}
		seen[aliasID] = struct{}{}
		if err := s.store.Delete(aliasID); err != nil {
			log.Debug().Err(err).Str("id", aliasID).Msg("failed to clean up aliased discovery")
		}
	}
}

// getResourceMetadata retrieves metadata for a resource from the state.
func (s *Service) getResourceMetadata(req DiscoveryRequest) map[string]any {
	snap, ok := s.getSnapshot()
	if !ok {
		return nil
	}
	metadata := make(map[string]any)
	requestTargetID := canonicalRequestTargetID(req)

	switch req.ResourceType {
	case ResourceTypeSystemContainer:
		for _, c := range snap.Containers {
			if fmt.Sprintf("%d", c.VMID) == req.ResourceID && c.Node == requestTargetID {
				metadata["name"] = c.Name
				metadata["status"] = c.Status
				metadata["vmid"] = c.VMID
				break
			}
		}
	case ResourceTypeVM:
		for _, vm := range snap.VMs {
			if fmt.Sprintf("%d", vm.VMID) == req.ResourceID && vm.Node == requestTargetID {
				metadata["name"] = vm.Name
				metadata["status"] = vm.Status
				metadata["vmid"] = vm.VMID
				break
			}
		}
	case ResourceTypeDocker:
		for _, host := range snap.DockerHosts {
			if host.AgentID == requestTargetID || host.Hostname == requestTargetID {
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
	case ResourceTypeAgent:
		for _, host := range snap.Hosts {
			if host.ID == req.ResourceID || host.Hostname == req.ResourceID || host.ID == requestTargetID {
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
// For system containers/VMs, this is the first IP from the Proxmox guest agent.
// For Docker containers, this is the Docker host's IP/hostname.
func (s *Service) getResourceExternalIP(req DiscoveryRequest) string {
	snap, ok := s.getSnapshot()
	if !ok {
		return ""
	}
	requestTargetID := canonicalRequestTargetID(req)

	switch req.ResourceType {
	case ResourceTypeSystemContainer:
		for _, c := range snap.Containers {
			if fmt.Sprintf("%d", c.VMID) == req.ResourceID && c.Node == requestTargetID {
				if len(c.IPAddresses) > 0 {
					return c.IPAddresses[0]
				}
				return ""
			}
		}
	case ResourceTypeVM:
		for _, vm := range snap.VMs {
			if fmt.Sprintf("%d", vm.VMID) == req.ResourceID && vm.Node == requestTargetID {
				if len(vm.IPAddresses) > 0 {
					return vm.IPAddresses[0]
				}
				return ""
			}
		}
	case ResourceTypeDocker:
		// For Docker containers, use the Docker host's hostname/IP
		for _, host := range snap.DockerHosts {
			if host.AgentID == requestTargetID || host.Hostname == requestTargetID {
				// Use hostname if it looks like an IP, otherwise it's a hostname
				return host.Hostname
			}
		}
	case ResourceTypeDockerVM, ResourceTypeDockerSystemContainer:
		// For Docker containers inside VMs/system containers, find the parent's IP
		// The target ID contains the parent resource info.
		for _, vm := range snap.VMs {
			if fmt.Sprintf("%d", vm.VMID) == requestTargetID || vm.Name == requestTargetID {
				if len(vm.IPAddresses) > 0 {
					return vm.IPAddresses[0]
				}
			}
		}
		for _, c := range snap.Containers {
			if fmt.Sprintf("%d", c.VMID) == requestTargetID || c.Name == requestTargetID {
				if len(c.IPAddresses) > 0 {
					return c.IPAddresses[0]
				}
			}
		}
	case ResourceTypeAgent:
		// Host-agent resources: prefer the reported hostname from state
		for _, host := range snap.Hosts {
			if host.ID == req.ResourceID || host.Hostname == req.ResourceID || host.ID == requestTargetID || host.Hostname == requestTargetID {
				if isURLHostCandidate(host.Hostname) {
					return host.Hostname
				}
			}
		}

		// Proxmox node resources routed through host discovery: fall back to node name
		for _, node := range snap.Nodes {
			if node.ID == req.ResourceID || node.Name == req.ResourceID || node.ID == requestTargetID || node.Name == requestTargetID {
				if isURLHostCandidate(node.Name) {
					return node.Name
				}
			}
		}

		// Last-resort fallback from request values (when state snapshot doesn't have a direct match yet)
		if isURLHostCandidate(req.Hostname) {
			return req.Hostname
		}
	}

	return ""
}

func isURLHostCandidate(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	// Reject obvious non-host labels (display names, paths, etc.)
	if strings.ContainsAny(trimmed, " /\\") {
		return false
	}
	return true
}

func formatURLSuggestionDiagnostic(primaryCode, primaryDetail, fallbackCode, fallbackDetail string) string {
	parts := make([]string, 0, 2)
	if primaryCode != "" {
		if primaryDetail != "" {
			parts = append(parts, fmt.Sprintf("primary=%s (%s)", primaryCode, primaryDetail))
		} else {
			parts = append(parts, "primary="+primaryCode)
		}
	}
	if fallbackCode != "" {
		if fallbackDetail != "" {
			parts = append(parts, fmt.Sprintf("fallback=%s (%s)", fallbackCode, fallbackDetail))
		} else {
			parts = append(parts, "fallback="+fallbackCode)
		}
	}
	if len(parts) == 0 {
		return "no suggestion diagnostics available"
	}
	return strings.Join(parts, "; ")
}

const (
	legacyURLSuggestionUnavailablePrefix = "[URL suggestion unavailable:"
	legacyURLSuggestionSourcePrefix      = "[URL suggestion source:"
)

func parseLegacyURLSuggestionReasoning(reasoning string) (cleanedReasoning, sourceCode, sourceDetail, diagnostic string) {
	cleaned := strings.TrimSpace(reasoning)
	for {
		changed := false
		if note, next, ok := consumeLegacyURLSuggestionNote(cleaned, legacyURLSuggestionUnavailablePrefix); ok {
			if diagnostic == "" {
				diagnostic = note
			}
			cleaned = next
			changed = true
		}
		if note, next, ok := consumeLegacyURLSuggestionNote(cleaned, legacyURLSuggestionSourcePrefix); ok {
			if sourceCode == "" && sourceDetail == "" {
				sourceCode, sourceDetail = parseLegacyURLSuggestionSource(note)
			}
			cleaned = next
			changed = true
		}
		if !changed {
			break
		}
	}
	return cleaned, sourceCode, sourceDetail, diagnostic
}

func consumeLegacyURLSuggestionNote(text, prefix string) (note, remaining string, ok bool) {
	trimmed := strings.TrimSpace(text)
	if !strings.HasPrefix(trimmed, prefix) {
		return "", text, false
	}

	closingBracket := strings.Index(trimmed, "]")
	if closingBracket <= len(prefix) {
		return "", text, false
	}

	note = strings.TrimSpace(trimmed[len(prefix):closingBracket])
	remaining = strings.TrimSpace(trimmed[closingBracket+1:])
	return note, remaining, true
}

func parseLegacyURLSuggestionSource(note string) (sourceCode, sourceDetail string) {
	trimmed := strings.TrimSpace(note)
	if trimmed == "" {
		return "", ""
	}
	if strings.HasSuffix(trimmed, ")") {
		if idx := strings.Index(trimmed, " ("); idx > 0 {
			sourceCode = strings.TrimSpace(trimmed[:idx])
			sourceDetail = strings.TrimSpace(trimmed[idx+2 : len(trimmed)-1])
			if sourceCode != "" {
				return sourceCode, sourceDetail
			}
		}
	}
	return trimmed, ""
}

// suggestHostManagementURL provides host-level fallback URL suggestions when
// AI discovery does not identify a known web service.
func (s *Service) suggestHostManagementURL(req DiscoveryRequest, host string) string {
	url, _, _ := s.suggestHostManagementURLWithReason(req, host)
	return url
}

func (s *Service) suggestHostManagementURLWithReason(req DiscoveryRequest, host string) (string, string, string) {
	if req.ResourceType != ResourceTypeAgent {
		return "", "host_fallback_not_applicable", "not a host resource"
	}
	if host == "" {
		return "", "no_host", "no host or IP candidate available"
	}
	snap, ok := s.getSnapshot()
	if !ok {
		return "", "state_provider_unavailable", "state unavailable"
	}
	requestTargetID := canonicalRequestTargetID(req)

	nodeMatchesReq := func(node Node) bool {
		return node.ID == requestTargetID ||
			node.Name == requestTargetID ||
			node.ID == req.ResourceID ||
			node.Name == req.ResourceID ||
			(req.Hostname != "" && node.Name == req.Hostname)
	}

	var matchedHost *Host
	for i := range snap.Hosts {
		h := &snap.Hosts[i]
		if h.ID == requestTargetID || h.Hostname == requestTargetID || h.ID == req.ResourceID || h.Hostname == req.ResourceID {
			matchedHost = h
			break
		}
	}

	// Proxmox nodes (or host agents linked to nodes) should suggest the node UI.
	for _, node := range snap.Nodes {
		if nodeMatchesReq(node) {
			return buildURL("https", host, 8006, ""), "host_management_profile_proxmox_node", "Proxmox node profile"
		}
		if matchedHost != nil && node.LinkedAgentID != "" && node.LinkedAgentID == matchedHost.ID {
			return buildURL("https", host, 8006, ""), "host_management_profile_linked_proxmox_node", "Linked Proxmox node profile"
		}
	}

	if matchedHost == nil {
		return "", "host_not_found_in_state", "host not found in state"
	}

	descriptor := strings.ToLower(strings.TrimSpace(strings.Join([]string{
		matchedHost.OSName,
		matchedHost.DisplayName,
		matchedHost.Platform,
	}, " ")))

	switch {
	case strings.Contains(descriptor, "proxmox backup"):
		return buildURL("https", host, 8007, ""), "host_management_profile_pbs", "Proxmox Backup profile"
	case strings.Contains(descriptor, "proxmox mail gateway"), strings.Contains(descriptor, "pmg"):
		return buildURL("https", host, 8006, ""), "host_management_profile_pmg", "Proxmox Mail Gateway profile"
	case strings.Contains(descriptor, "proxmox ve"), strings.Contains(descriptor, "proxmox"):
		return buildURL("https", host, 8006, ""), "host_management_profile_pve", "Proxmox VE profile"
	case strings.Contains(descriptor, "truenas"),
		strings.Contains(descriptor, "unraid"),
		strings.Contains(descriptor, "openmediavault"):
		return buildURL("http", host, 80, ""), "host_management_profile_nas", "NAS management profile"
	default:
		return "", "host_platform_not_recognized", "unknown host management profile"
	}
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

	infoJSON, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		log.Warn().Err(err).Str("container", c.Name).Msg("Failed to marshal Docker metadata for discovery prompt")
		infoJSON = []byte("{}")
	}

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
Target: %s (%s)`, req.ResourceType, req.ResourceID, req.Hostname, req.TargetID))

	if len(req.Metadata) > 0 {
		metaJSON, err := json.MarshalIndent(req.Metadata, "", "  ")
		if err != nil {
			log.Warn().Err(err).Msg("Failed to marshal discovery metadata for analysis prompt")
			sections = append(sections, fmt.Sprintf("Metadata:\n%v", req.Metadata))
		} else {
			sections = append(sections, fmt.Sprintf("Metadata:\n%s", string(metaJSON)))
		}
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
	if req.ResourceType == ResourceTypeAgent {
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
	log.Debug().Str("raw_response", response).Msg("discovery raw response")
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
		log.Debug().Err(err).Str("response", response).Msg("failed to parse AI response")
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
	if err != nil {
		return nil, fmt.Errorf("get discovery %q: %w", id, err)
	}
	if d == nil {
		return nil, nil
	}
	s.upgradeCLIAccessIfNeeded(d)
	return d, nil
}

func (s *Service) GetDiscoveryByResource(resourceType ResourceType, targetID, resourceID string) (*ResourceDiscovery, error) {
	req := DiscoveryRequest{
		ResourceType: resourceType,
		TargetID:     targetID,
		HostID:       targetID,
		ResourceID:   resourceID,
	}
	aliasIDs := []string{MakeResourceID(resourceType, targetID, resourceID)}
	req = s.normalizeDiscoveryRequest(req, &aliasIDs)

	d, err := s.store.GetByResource(resourceType, req.TargetID, req.ResourceID)
	if err != nil || d == nil {
		for _, aliasID := range aliasIDs {
			if aliasID == "" {
				continue
			}
			dAlias, errAlias := s.store.Get(aliasID)
			if errAlias == nil && dAlias != nil {
				s.upgradeCLIAccessIfNeeded(dAlias)
				return dAlias, nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("get discovery for %s/%s/%s: %w", resourceType, req.TargetID, req.ResourceID, err)
		}
		return nil, nil
	}

	s.upgradeCLIAccessIfNeeded(d)
	return d, nil
}

// ListDiscoveries returns all discoveries.
func (s *Service) ListDiscoveries() ([]*ResourceDiscovery, error) {
	discoveries, err := s.store.List()
	if err != nil {
		return nil, fmt.Errorf("list discoveries: %w", err)
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
		return nil, fmt.Errorf("list discoveries by type %q: %w", resourceType, err)
	}
	discoveries = s.deduplicateDiscoveries(discoveries)
	for _, d := range discoveries {
		s.upgradeCLIAccessIfNeeded(d)
	}
	return discoveries, nil
}

// ListDiscoveriesByTarget returns discoveries for a specific target ID.
func (s *Service) ListDiscoveriesByTarget(targetID string) ([]*ResourceDiscovery, error) {
	discoveries, err := s.store.ListByTarget(targetID)
	if err != nil {
		return nil, fmt.Errorf("list discoveries by target %q: %w", targetID, err)
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
	snap, ok := s.getSnapshot()
	if !ok {
		return discoveries
	}
	if len(snap.Nodes) == 0 {
		return discoveries
	}

	// Map linked agent IDs to their PVE node source(s)
	// AgentID -> NodeName
	linkedAgents := make(map[string]string)
	for _, node := range snap.Nodes {
		if node.LinkedAgentID != "" {
			linkedAgents[node.LinkedAgentID] = node.Name
		}
	}

	if len(linkedAgents) == 0 {
		return discoveries
	}

	// Check which agents actually have discovery data
	hasAgentDiscovery := make(map[string]bool)
	for _, d := range discoveries {
		if d.ResourceType == ResourceTypeAgent {
			discoveryTargetID := canonicalDiscoveryTargetID(d)
			// discoveryTargetID is usually the agent ID for host resources
			if _, ok := linkedAgents[discoveryTargetID]; ok {
				hasAgentDiscovery[discoveryTargetID] = true
			}
		}
	}

	// Filter out PVE node discoveries if the corresponding agent discovery exists
	filtered := make([]*ResourceDiscovery, 0, len(discoveries))
	for _, d := range discoveries {
		if d.ResourceType == ResourceTypeAgent {
			// If this discovery is for a PVE node (by name/ID)
			// check if it maps to an agent that ALREADY has a discovery in this list

			// Is this discovery's ID satisfying a Node check?
			isPVENode := false
			var linkedAgentID string
			discoveryTargetID := canonicalDiscoveryTargetID(d)

			for _, node := range snap.Nodes {
				if discoveryTargetID == node.Name || discoveryTargetID == node.ID || d.ResourceID == node.Name {
					isPVENode = true
					linkedAgentID = node.LinkedAgentID
					break
				}
			}

			if isPVENode && linkedAgentID != "" && hasAgentDiscovery[linkedAgentID] && discoveryTargetID != linkedAgentID {
				// We have the agent discovery, so skip this redundant PVE node discovery
				continue
			}
		}
		filtered = append(filtered, d)
	}

	return filtered
}

// upgradeCLIAccessIfNeeded upgrades cached discovery fields to current versions.
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
	if d.Hostname == "" {
		if snap, ok := s.getSnapshot(); ok {
			hostname := s.lookupHostnameFromState(d.ResourceType, canonicalDiscoveryTargetID(d), d.ResourceID, snap)
			if hostname != "" {
				d.Hostname = hostname
				upgraded = true
				log.Debug().
					Str("id", d.ID).
					Str("hostname", hostname).
					Msg("Populated missing hostname from state")
			}
		}
	}

	// Migrate legacy URL suggestion notes from AI reasoning into structured fields.
	cleanedReasoning, sourceCode, sourceDetail, diagnostic := parseLegacyURLSuggestionReasoning(d.AIReasoning)
	if d.SuggestedURLSourceCode == "" && sourceCode != "" {
		d.SuggestedURLSourceCode = sourceCode
		upgraded = true
	}
	if d.SuggestedURLSourceDetail == "" && sourceDetail != "" {
		d.SuggestedURLSourceDetail = sourceDetail
		upgraded = true
	}
	if d.SuggestedURLDiagnostic == "" && diagnostic != "" {
		d.SuggestedURLDiagnostic = diagnostic
		upgraded = true
	}
	if cleanedReasoning != d.AIReasoning {
		d.AIReasoning = cleanedReasoning
		upgraded = true
	}

	_ = upgraded // Suppress unused variable warning if logging is disabled
}

// lookupHostnameFromState finds the hostname/name for a resource from state
func (s *Service) lookupHostnameFromState(resourceType ResourceType, hostID, resourceID string, snap StateSnapshot) string {
	switch resourceType {
	case ResourceTypeSystemContainer:
		for _, c := range snap.Containers {
			if fmt.Sprintf("%d", c.VMID) == resourceID && c.Node == hostID {
				return c.Name
			}
		}
	case ResourceTypeVM:
		for _, vm := range snap.VMs {
			if fmt.Sprintf("%d", vm.VMID) == resourceID && vm.Node == hostID {
				return vm.Name
			}
		}
	case ResourceTypeDocker:
		for _, host := range snap.DockerHosts {
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
	return s.GetStatusSnapshot().ToMap()
}

// GetStatusSnapshot returns the typed status snapshot including fingerprint statistics.
func (s *Service) GetStatusSnapshot() ServiceStatus {
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

	return ServiceStatus{
		Running:             s.running,
		LastRun:             s.lastRun,
		Interval:            s.interval.String(),
		CacheSize:           cacheSize,
		AIAnalyzerSet:       s.aiAnalyzer != nil,
		ScannerSet:          s.scanner != nil,
		StoreSet:            s.store != nil,
		DeepScanTimeout:     s.deepScanTimeout.String(),
		AIAnalysisTimeout:   s.aiAnalysisTimeout.String(),
		MaxDiscoveryAge:     s.maxDiscoveryAge.String(),
		FingerprintCount:    fingerprintCount,
		LastFingerprintScan: lastFingerprintScan,
	}
}

func canonicalDiscoveryTargetID(discovery *ResourceDiscovery) string {
	if discovery == nil {
		return ""
	}
	return strings.TrimSpace(discovery.TargetID)
}

func canonicalRequestTargetID(req DiscoveryRequest) string {
	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(req.HostID)
	}
	return targetID
}

func normalizeDiscoveryRequestAliases(req DiscoveryRequest) DiscoveryRequest {
	req.TargetID = canonicalRequestTargetID(req)
	req.HostID = req.TargetID // Keep legacy alias in sync for compatibility paths.
	return req
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
	if age < minDiscoveryMaxAge {
		age = minDiscoveryMaxAge
	}

	s.maxDiscoveryAge = age
	log.Info().Dur("max_discovery_age", age).Msg("max discovery age updated")
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
func (s *Service) GetDiscoveryForAIChat(ctx context.Context, resourceType ResourceType, targetID, resourceID string) (*ResourceDiscovery, error) {
	// This is the same as DiscoverResource but without Force
	return s.DiscoverResource(ctx, DiscoveryRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		TargetID:     targetID,
		HostID:       targetID,
		Force:        false, // Let fingerprint logic decide
	})
}

// GetDiscoveriesForAIContext returns discoveries for multiple resources.
// Used when AI chat needs context about the infrastructure.
// Only runs discovery for resources that actually need it (fingerprint changed).
func (s *Service) GetDiscoveriesForAIContext(ctx context.Context, resourceIDs []string) ([]*ResourceDiscovery, error) {
	var results []*ResourceDiscovery
	for _, id := range resourceIDs {
		resourceType, targetID, resourceID, err := ParseResourceID(id)
		if err != nil {
			log.Debug().Err(err).Str("id", id).Msg("failed to parse resource ID for AI context")
			continue
		}
		discovery, err := s.GetDiscoveryForAIChat(ctx, resourceType, targetID, resourceID)
		if err != nil {
			log.Debug().Err(err).Str("id", id).Msg("failed to get discovery for AI context")
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
		return 0, fmt.Errorf("get changed discovery resources: %w", err)
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
		return 0, fmt.Errorf("get stale discovery resources: %w", err)
	}
	return len(stale), nil
}
