// Package aidiscovery provides AI-powered infrastructure discovery capabilities.
// It discovers services, versions, configurations, and CLI access methods
// for VMs, LXCs, Docker containers, Kubernetes pods, and hosts.
package aidiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// StateProvider provides access to the current infrastructure state.
type StateProvider interface {
	GetState() StateSnapshot
}

// StateSnapshot represents the infrastructure state. This mirrors models.StateSnapshot
// to avoid circular dependencies.
type StateSnapshot struct {
	VMs         []VM
	Containers  []Container
	DockerHosts []DockerHost
}

// VM represents a virtual machine.
type VM struct {
	VMID     int
	Name     string
	Node     string
	Status   string
	Instance string
}

// Container represents an LXC container.
type Container struct {
	VMID     int
	Name     string
	Node     string
	Status   string
	Instance string
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

// AIAnalyzer provides AI analysis capabilities for discovery.
type AIAnalyzer interface {
	AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error)
}

// Service manages AI-powered infrastructure discovery.
type Service struct {
	store         *Store
	scanner       *DeepScanner
	stateProvider StateProvider
	aiAnalyzer    AIAnalyzer

	mu           sync.RWMutex
	running      bool
	stopCh       chan struct{}
	interval     time.Duration
	initialDelay time.Duration
	lastRun      time.Time

	// Cache for AI analysis results (by image name)
	analysisCache   map[string]*AIAnalysisResponse
	cacheMu         sync.RWMutex
	cacheExpiry     time.Duration
	lastCacheUpdate time.Time
}

// Config holds discovery service configuration.
type Config struct {
	DataDir     string
	Interval    time.Duration // How often to run background discovery
	CacheExpiry time.Duration // How long to cache AI analysis results
}

// DefaultConfig returns the default discovery configuration.
func DefaultConfig() Config {
	return Config{
		Interval:    10 * time.Minute,
		CacheExpiry: 1 * time.Hour,
	}
}

// NewService creates a new discovery service.
func NewService(store *Store, scanner *DeepScanner, stateProvider StateProvider, cfg Config) *Service {
	if cfg.Interval == 0 {
		cfg.Interval = 10 * time.Minute
	}
	if cfg.CacheExpiry == 0 {
		cfg.CacheExpiry = 1 * time.Hour
	}

	return &Service{
		store:         store,
		scanner:       scanner,
		stateProvider: stateProvider,
		interval:      cfg.Interval,
		initialDelay:  30 * time.Second,
		cacheExpiry:   cfg.CacheExpiry,
		stopCh:        make(chan struct{}),
		analysisCache: make(map[string]*AIAnalysisResponse),
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
		Msg("Starting AI-powered infrastructure discovery service")

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

// SetInterval updates the scan interval. Takes effect on next Start().
func (s *Service) SetInterval(interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interval = interval
}

// IsRunning returns whether the background discovery loop is active.
func (s *Service) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// discoveryLoop runs periodic discovery.
func (s *Service) discoveryLoop(ctx context.Context) {
	delay := s.initialDelay
	if delay <= 0 {
		delay = 30 * time.Second
	}

	// Run initial discovery after a short delay
	select {
	case <-time.After(delay):
	case <-s.stopCh:
		return
	case <-ctx.Done():
		return
	}

	s.runBackgroundDiscovery(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.runBackgroundDiscovery(ctx)
		case <-s.stopCh:
			log.Info().Msg("Stopping AI discovery service")
			return
		case <-ctx.Done():
			log.Info().Msg("AI discovery context cancelled")
			return
		}
	}
}

// runBackgroundDiscovery runs discovery on all resources in the background.
func (s *Service) runBackgroundDiscovery(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Stack().Msg("Recovered from panic in background AI discovery")
		}
	}()

	s.mu.Lock()
	s.lastRun = time.Now()
	s.mu.Unlock()

	// For background discovery, we only do shallow analysis based on metadata
	// Deep scanning is triggered on-demand via DiscoverResource
	if s.stateProvider == nil {
		return
	}

	state := s.stateProvider.GetState()
	s.discoverDockerContainers(ctx, state.DockerHosts)
}

// discoverDockerContainers runs discovery on Docker containers using metadata.
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

			// Analyze using metadata (shallow discovery)
			discovery := s.analyzeDockerContainer(ctx, analyzer, container, host)
			if discovery != nil {
				if err := s.store.Save(discovery); err != nil {
					log.Warn().Err(err).Str("id", id).Msg("Failed to save discovery")
				}
			}
		}
	}
}

// analyzeDockerContainer analyzes a Docker container using AI.
func (s *Service) analyzeDockerContainer(ctx context.Context, analyzer AIAnalyzer, c DockerContainer, host DockerHost) *ResourceDiscovery {
	// Check cache first
	s.cacheMu.RLock()
	cached, found := s.analysisCache[c.Image]
	cacheValid := time.Since(s.lastCacheUpdate) < s.cacheExpiry
	s.cacheMu.RUnlock()

	var result *AIAnalysisResponse

	if found && cacheValid {
		result = cached
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

		// Cache the result
		s.cacheMu.Lock()
		s.analysisCache[c.Image] = result
		s.lastCacheUpdate = time.Now()
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
		Ports:          ports,
		Confidence:     result.Confidence,
		AIReasoning:    result.Reasoning,
		DiscoveredAt:   time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// DiscoverResource performs deep discovery on a specific resource.
func (s *Service) DiscoverResource(ctx context.Context, req DiscoveryRequest) (*ResourceDiscovery, error) {
	resourceID := MakeResourceID(req.ResourceType, req.HostID, req.ResourceID)

	// Check if we have a recent discovery and force isn't set
	if !req.Force {
		existing, err := s.store.Get(resourceID)
		if err == nil && existing != nil {
			age := time.Since(existing.UpdatedAt)
			if age < 5*time.Minute {
				log.Debug().Str("id", resourceID).Dur("age", age).Msg("Using recent discovery")
				return existing, nil
			}
		}
	}

	s.mu.RLock()
	analyzer := s.aiAnalyzer
	s.mu.RUnlock()

	if analyzer == nil {
		return nil, fmt.Errorf("AI analyzer not configured")
	}

	// Run deep scan if scanner is available
	var scanResult *ScanResult
	if s.scanner != nil {
		var err error
		scanResult, err = s.scanner.Scan(ctx, req)
		if err != nil {
			log.Warn().Err(err).Str("id", resourceID).Msg("Deep scan failed, using metadata only")
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
	response, err := analyzer.AnalyzeForDiscovery(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI analysis failed: %w", err)
	}

	result := s.parseAIResponse(response)
	if result == nil {
		// Truncate response for error message
		truncated := response
		if len(truncated) > 500 {
			truncated = truncated[:500] + "..."
		}
		return nil, fmt.Errorf("failed to parse AI response: %s", truncated)
	}

	// Build discovery result
	discovery := &ResourceDiscovery{
		ID:             resourceID,
		ResourceType:   req.ResourceType,
		ResourceID:     req.ResourceID,
		HostID:         req.HostID,
		Hostname:       req.Hostname,
		ServiceType:    result.ServiceType,
		ServiceName:    result.ServiceName,
		ServiceVersion: result.ServiceVersion,
		Category:       result.Category,
		CLIAccess:      s.formatCLIAccess(req.ResourceType, req.ResourceID, result.CLIAccess),
		Facts:          result.Facts,
		ConfigPaths:    result.ConfigPaths,
		DataPaths:      result.DataPaths,
		Ports:          result.Ports,
		Confidence:     result.Confidence,
		AIReasoning:    result.Reasoning,
		DiscoveredAt:   time.Now(),
		UpdatedAt:      time.Now(),
	}

	if scanResult != nil {
		discovery.RawCommandOutput = scanResult.CommandOutputs
		discovery.ScanDuration = scanResult.CompletedAt.Sub(scanResult.StartedAt).Milliseconds()
	}

	// Preserve user notes from existing discovery
	existing, _ := s.store.Get(resourceID)
	if existing != nil {
		discovery.UserNotes = existing.UserNotes
		discovery.UserSecrets = existing.UserSecrets
		if discovery.DiscoveredAt.IsZero() || existing.DiscoveredAt.Before(discovery.DiscoveredAt) {
			discovery.DiscoveredAt = existing.DiscoveredAt
		}
	}

	// Save discovery
	if err := s.store.Save(discovery); err != nil {
		return nil, fmt.Errorf("failed to save discovery: %w", err)
	}

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
						metadata["labels"] = c.Labels
						break
					}
				}
				break
			}
		}
	}

	return metadata
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
		info["labels"] = c.Labels
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

	return fmt.Sprintf(`Analyze this infrastructure resource and provide detailed discovery information.

%s

Based on all available information, determine:
1. What service/application is running?
2. What version is it?
3. What are the important configuration paths?
4. What data paths should be backed up?
5. What ports are in use?
6. Any special hardware (GPU, TPU, etc.)?
7. Any dependencies (databases, message queues, etc.)?

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
  "ports": [{"port": 8080, "protocol": "tcp", "process": "nginx", "address": "0.0.0.0"}],
  "confidence": 0.0-1.0,
  "reasoning": "Explanation of identification"
}

Important:
- Extract version numbers from package lists, process output, or config files
- Identify config and data paths from mount points and file listings
- Note any special hardware like Coral TPU, NVIDIA GPU
- For LXC/VM, the CLI access should use pct exec or qm guest exec
- For Docker, use docker exec

Respond with ONLY valid JSON.`, strings.Join(sections, "\n\n"))
}

// parseAIResponse parses the AI's JSON response.
func (s *Service) parseAIResponse(response string) *AIAnalysisResponse {
	log.Debug().Str("raw_response", response).Msg("AI discovery raw response")
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

// GetDiscovery retrieves a discovery by ID.
func (s *Service) GetDiscovery(id string) (*ResourceDiscovery, error) {
	return s.store.Get(id)
}

// GetDiscoveryByResource retrieves a discovery by resource type and ID.
func (s *Service) GetDiscoveryByResource(resourceType ResourceType, hostID, resourceID string) (*ResourceDiscovery, error) {
	return s.store.GetByResource(resourceType, hostID, resourceID)
}

// ListDiscoveries returns all discoveries.
func (s *Service) ListDiscoveries() ([]*ResourceDiscovery, error) {
	return s.store.List()
}

// ListDiscoveriesByType returns discoveries for a specific resource type.
func (s *Service) ListDiscoveriesByType(resourceType ResourceType) ([]*ResourceDiscovery, error) {
	return s.store.ListByType(resourceType)
}

// ListDiscoveriesByHost returns discoveries for a specific host.
func (s *Service) ListDiscoveriesByHost(hostID string) ([]*ResourceDiscovery, error) {
	return s.store.ListByHost(hostID)
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

// GetStatus returns the service status.
func (s *Service) GetStatus() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.cacheMu.RLock()
	cacheSize := len(s.analysisCache)
	s.cacheMu.RUnlock()

	return map[string]any{
		"running":         s.running,
		"last_run":        s.lastRun,
		"interval":        s.interval.String(),
		"cache_size":      cacheSize,
		"ai_analyzer_set": s.aiAnalyzer != nil,
		"scanner_set":     s.scanner != nil,
		"store_set":       s.store != nil,
	}
}

// ClearCache clears the AI analysis cache.
func (s *Service) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.analysisCache = make(map[string]*AIAnalysisResponse)
	s.lastCacheUpdate = time.Time{}
}
