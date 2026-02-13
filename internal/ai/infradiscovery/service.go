// Package infradiscovery provides infrastructure discovery for detecting
// applications and services running on monitored hosts. It uses LLM analysis to
// identify services from Docker containers, enabling AI systems like Patrol to
// understand where services run and propose correct remediation commands.
package infradiscovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// StateProvider provides access to the current infrastructure state.
type StateProvider interface {
	GetState() models.StateSnapshot
}

// AIAnalyzer provides AI analysis capabilities for discovery.
// This interface allows the discovery service to use LLM analysis
// without creating circular dependencies with the AI package.
type AIAnalyzer interface {
	// AnalyzeForDiscovery sends a prompt to the AI and returns the response.
	// The model parameter specifies which model to use (e.g., "anthropic:claude-haiku-4-5")
	AnalyzeForDiscovery(ctx context.Context, prompt string) (string, error)
}

// DiscoveredApp represents a detected application or service.
type DiscoveredApp struct {
	ID            string    `json:"id"`             // Unique ID: "docker:hostname:container"
	Type          string    `json:"type"`           // Application type: pbs, postgres, nginx, custom, etc.
	Name          string    `json:"name"`           // Human-readable name: "Proxmox Backup Server"
	Category      string    `json:"category"`       // Category: backup, database, web, monitoring, unknown
	RunsIn        string    `json:"runs_in"`        // Runtime: docker, systemd, native
	HostID        string    `json:"host_id"`        // Host identifier (agent ID or hostname)
	Hostname      string    `json:"hostname"`       // Human-readable hostname
	ContainerID   string    `json:"container_id"`   // Docker container ID (if applicable)
	ContainerName string    `json:"container_name"` // Docker container name (if applicable)
	ServiceUnit   string    `json:"service_unit"`   // Systemd unit name (if applicable)
	Ports         []int     `json:"ports"`          // Exposed ports
	CLIAccess     string    `json:"cli_access"`     // How to access CLI: "docker exec pbs proxmox-backup-manager"
	Confidence    float64   `json:"confidence"`     // Detection confidence 0-1
	DetectedAt    time.Time `json:"detected_at"`    // When this app was detected
	AIReasoning   string    `json:"ai_reasoning"`   // AI's reasoning for the identification
}

// ContainerInfo holds information about a container for AI analysis.
type ContainerInfo struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       []PortInfo        `json:"ports,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	EnvVarNames []string          `json:"env_var_names,omitempty"` // Just names, not values (security)
	Mounts      []string          `json:"mounts,omitempty"`
	Networks    []string          `json:"networks,omitempty"`
	Status      string            `json:"status,omitempty"`
	Command     string            `json:"command,omitempty"`
}

// PortInfo holds port mapping information.
type PortInfo struct {
	HostPort      int    `json:"host_port,omitempty"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"`
}

// DiscoveryResult represents the AI's analysis of a container.
type DiscoveryResult struct {
	ServiceType string  `json:"service_type"` // e.g., "postgres", "pbs", "nginx", "unknown"
	ServiceName string  `json:"service_name"` // Human-readable name
	Category    string  `json:"category"`     // backup, database, web, monitoring, etc.
	CLICommand  string  `json:"cli_command"`  // How to run CLI commands in this container
	Confidence  float64 `json:"confidence"`   // 0-1 confidence score
	Reasoning   string  `json:"reasoning"`    // Why the AI made this determination
}

// Service manages infrastructure discovery.
type Service struct {
	stateProvider  StateProvider
	knowledgeStore *knowledge.Store
	aiAnalyzer     AIAnalyzer
	mu             sync.RWMutex
	lastRun        time.Time
	interval       time.Duration
	stopCh         chan struct{}
	lifecycleCtx   context.Context
	lifecycleStop  context.CancelFunc
	workerWG       sync.WaitGroup
	running        bool
	discoveryMu    sync.Mutex
	discoveryRun   bool
	discoveries    []DiscoveredApp

	// Cache to avoid re-analyzing the same containers
	// Key: image name, Value: analysis result
	analysisCache     map[string]*analysisCacheEntry
	cacheMu           sync.RWMutex
	cacheExpiry       time.Duration
	aiAnalysisTimeout time.Duration
}

const (
	maxPromptFieldLength     = 256
	maxPromptLabelCount      = 64
	maxPromptMountCount      = 64
	maxPromptNetworkCount    = 32
	maxPromptPortCount       = 64
	maxAnalysisCacheEntries  = 1024
	maxAIResponseLength      = 64 * 1024
	maxServiceTypeLength     = 64
	maxServiceNameLength     = 128
	maxCategoryLength        = 64
	maxCLICommandLength      = 256
	maxReasoningLength       = 1024
	maxAIParseLogFieldLength = 512
	maxAppFieldLength        = 256
)

var (
	identifierPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
	safeShellNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
)

// Config holds discovery service configuration.
type Config struct {
	Interval          time.Duration // How often to run discovery (default: 5 minutes)
	CacheExpiry       time.Duration // How long to cache analysis results (default: 1 hour)
	AIAnalysisTimeout time.Duration // Timeout for each AI analysis request (default: 45 seconds)
}

type analysisCacheEntry struct {
	result   *DiscoveryResult
	cachedAt time.Time
}

// ServiceStatus is a typed snapshot of the discovery service runtime state.
type ServiceStatus struct {
	Running        bool
	LastRun        time.Time
	Interval       time.Duration
	DiscoveredApps int
	CacheSize      int
	AIAnalyzerSet  bool
}

// ToMap provides backward compatibility for existing callers expecting map status data.
func (s ServiceStatus) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"running":         s.Running,
		"last_run":        s.LastRun,
		"interval":        s.Interval.String(),
		"discovered_apps": s.DiscoveredApps,
		"cache_size":      s.CacheSize,
		"ai_analyzer_set": s.AIAnalyzerSet,
	}
}

const (
	defaultDiscoveryInterval = 5 * time.Minute
	defaultCacheExpiry       = 1 * time.Hour
)

type emptyStateProvider struct{}

func (emptyStateProvider) GetState() models.StateSnapshot {
	return models.StateSnapshot{}
}

// DefaultConfig returns the default discovery configuration.
func DefaultConfig() Config {
	return Config{
		Interval:    defaultDiscoveryInterval,
		CacheExpiry: defaultCacheExpiry,
	}
}

func normalizeConfig(cfg Config) Config {
	if cfg.Interval <= 0 {
		log.Warn().
			Dur("configured_interval", cfg.Interval).
			Dur("default_interval", defaultDiscoveryInterval).
			Msg("Invalid infrastructure discovery interval; using default")
		cfg.Interval = defaultDiscoveryInterval
	}
	if cfg.CacheExpiry <= 0 {
		log.Warn().
			Dur("configured_cache_expiry", cfg.CacheExpiry).
			Dur("default_cache_expiry", defaultCacheExpiry).
			Msg("Invalid infrastructure discovery cache expiry; using default")
		cfg.CacheExpiry = defaultCacheExpiry
	}
	return cfg
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// NewService creates a new infrastructure discovery service.
func NewService(stateProvider StateProvider, knowledgeStore *knowledge.Store, cfg Config) *Service {
	cfg = normalizeConfig(cfg)

	if stateProvider == nil {
		log.Warn().Msg("Infrastructure discovery state provider not configured; using empty state provider")
		stateProvider = emptyStateProvider{}
	}
	if cfg.AIAnalysisTimeout <= 0 {
		cfg.AIAnalysisTimeout = 45 * time.Second
	}

	return &Service{
		stateProvider:     stateProvider,
		knowledgeStore:    knowledgeStore,
		interval:          cfg.Interval,
		cacheExpiry:       cfg.CacheExpiry,
		aiAnalysisTimeout: cfg.AIAnalysisTimeout,
		stopCh:            make(chan struct{}),
		discoveries:       make([]DiscoveredApp, 0),
		analysisCache:     make(map[string]*analysisCacheEntry),
	}
}

// SetAIAnalyzer sets the AI analyzer for discovery.
// This must be called before Start() for discovery to work.
func (s *Service) SetAIAnalyzer(analyzer AIAnalyzer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiAnalyzer = analyzer
}

// goRecover launches fn in a goroutine with panic recovery logging.
func goRecover(label string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Stack().
					Msgf("Recovered from panic in %s", label)
			}
		}()
		fn()
	}()
}

// Start begins the background discovery service.
func (s *Service) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	serviceCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		cancel()
		return
	}
	s.stopCh = make(chan struct{})
	s.lifecycleCtx = serviceCtx
	s.lifecycleStop = cancel
	s.stopped = false
	s.running = true
	s.workerWG.Add(2)
	s.mu.Unlock()

	log.Info().
		Dur("interval", s.interval).
		Msg("Starting infrastructure discovery service")

	// Run immediately on startup
	go func() {
		defer s.workerWG.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Stack().
					Msg("Recovered from panic in initial infrastructure discovery")
			}
		}()
		s.RunDiscovery(serviceCtx)
	}()

	// Start periodic discovery loop
	go func() {
		defer s.workerWG.Done()
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Stack().
					Msg("Recovered from panic in infrastructure discovery loop")
			}
		}()
		s.discoveryLoop(serviceCtx)
	}()
}

// Stop stops the background discovery service.
func (s *Service) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}

	stopCh := s.stopCh
	stop := s.lifecycleStop
	wasRunning := s.running

	s.running = false
	s.stopped = true
	s.lifecycleCtx = nil
	s.lifecycleStop = nil
	s.mu.Unlock()

	if stop != nil {
		stop()
	}
	if wasRunning {
		close(stopCh)
		s.workerWG.Wait()
	}
}

func (s *Service) isStopped() bool {
	s.mu.RLock()
	stopped := s.stopped
	stopCh := s.stopCh
	s.mu.RUnlock()
	if stopped {
		return true
	}

	select {
	case <-stopCh:
		return true
	default:
		return false
	}
}

// discoveryLoop runs periodic discovery.
func (s *Service) discoveryLoop(ctx context.Context) {
	defer func() {
		s.mu.Lock()
		s.running = false
		s.stopped = true
		s.lifecycleCtx = nil
		s.lifecycleStop = nil
		s.mu.Unlock()
	}()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.RunDiscovery(ctx)
		case <-stopCh:
			log.Info().Msg("Stopping infrastructure discovery service")
			return
		case <-ctx.Done():
			log.Info().Msg("infrastructure discovery context cancelled")
			return
		}
	}
}

// RunDiscovery performs a discovery scan using AI analysis.
func (s *Service) RunDiscovery(ctx context.Context) []DiscoveredApp {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.isStopped() {
		log.Debug().Msg("Infrastructure discovery service stopped, skipping discovery run")
		return nil
	}
	if ctx.Err() != nil {
		log.Debug().Msg("Infrastructure discovery context canceled before run, skipping discovery")
		return nil
	}

	start := time.Now()

	if s.stateProvider == nil {
		log.Error().Msg("State provider is not configured; skipping discovery")
		return nil
	}

	state := s.stateProvider.GetState()

	s.mu.RLock()
	analyzer := s.aiAnalyzer
	s.mu.RUnlock()

	if analyzer == nil {
		log.Debug().
			Int("docker_hosts", len(state.DockerHosts)).
			Msg("AI analyzer not set, skipping discovery")
		return nil
	}

	var apps []DiscoveredApp

	// Collect all containers from all Docker hosts
	var allContainers []struct {
		Container models.DockerContainer
		Host      models.DockerHost
	}

	for _, dockerHost := range state.DockerHosts {
		for _, container := range dockerHost.Containers {
			allContainers = append(allContainers, struct {
				Container models.DockerContainer
				Host      models.DockerHost
			}{container, dockerHost})
		}
	}

	if len(allContainers) == 0 {
		log.Debug().
			Int("docker_hosts", len(state.DockerHosts)).
			Msg("No Docker containers found for discovery")
		s.mu.Lock()
		s.lastRun = time.Now()
		s.mu.Unlock()
		return apps
	}

	// Analyze containers (check cache first, batch uncached ones)
	for _, item := range allContainers {
		if s.isStopped() {
			log.Debug().Msg("Infrastructure discovery stopped mid-run, aborting discovery")
			return nil
		}
		if ctx.Err() != nil {
			log.Debug().Msg("Infrastructure discovery canceled mid-run, aborting discovery")
			return nil
		}

		app := s.analyzeContainer(ctx, analyzer, item.Container, item.Host)
		if app != nil {
			apps = append(apps, *app)
		}
	}

	if s.isStopped() || ctx.Err() != nil {
		log.Debug().Msg("Infrastructure discovery interrupted before persistence, skipping cache updates")
		return nil
	}

	// Save discoveries to knowledge store
	s.saveDiscoveries(apps)

	// Update cache
	s.mu.Lock()
	s.discoveries = apps
	s.lastRun = time.Now()
	s.mu.Unlock()

	log.Info().
		Int("containers_scanned", len(allContainers)).
		Int("apps_discovered", len(apps)).
		Dur("duration", time.Since(start)).
		Msg("AI infrastructure discovery completed")

	return apps
}

// analyzeContainer uses AI to analyze a single container.
func (s *Service) analyzeContainer(ctx context.Context, analyzer AIAnalyzer, c models.DockerContainer, host models.DockerHost) *DiscoveredApp {
	if s.isStopped() || ctx.Err() != nil {
		return nil
	}

	// Check cache first
	s.cacheMu.RLock()
	entry, found := s.analysisCache[container.Image]
	cacheValid := found && time.Since(entry.cachedAt) < s.cacheExpiry
	s.cacheMu.RUnlock()

	var result *DiscoveryResult

	if found && cacheValid {
		result = entry.result
		log.Debug().
			Str("host", host.Hostname).
			Str("host_id", host.AgentID).
			Str("container", c.Name).
			Str("container_id", c.ID).
			Str("image", c.Image).
			Msg("Using cached analysis result")
	} else {
		// Build container info for AI analysis
		info := s.buildContainerInfo(container)

		// Create analysis prompt
		prompt, err := s.buildAnalysisPrompt(info)
		if err != nil {
			if s.isStopped() || ctx.Err() != nil {
				log.Debug().
					Err(err).
					Str("container", c.Name).
					Str("image", c.Image).
					Msg("Infrastructure discovery interrupted during AI analysis")
				return nil
			}
			log.Warn().
				Err(err).
				Str("host", host.Hostname).
				Str("host_id", host.AgentID).
				Str("container", c.Name).
				Str("container_id", c.ID).
				Str("image", c.Image).
				Msg("Failed to build AI analysis prompt for container")
			return nil
		}

		// Call AI with a per-request timeout so a stalled model call can't hang discovery.
		analyzeCtx, cancel := context.WithTimeout(ctx, s.aiAnalysisTimeout)
		defer cancel()

		response, err := analyzer.AnalyzeForDiscovery(analyzeCtx, prompt)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				log.Warn().
					Err(err).
					Str("container", container.Name).
					Str("image", container.Image).
					Dur("timeout", s.aiAnalysisTimeout).
					Msg("AI analysis timed out for container")
				return nil
			}
			log.Warn().
				Err(err).
				Str("container", container.Name).
				Str("image", container.Image).
				Msg("AI analysis failed for container")
			return nil
		}

		// Parse response
		result = s.parseAIResponse(response)
		if result == nil {
			log.Warn().
				Str("host", host.Hostname).
				Str("host_id", host.AgentID).
				Str("container", c.Name).
				Str("container_id", c.ID).
				Str("image", c.Image).
				Str("response", response).
				Msg("Failed to parse AI response")
			return nil
		}

		// Cache the result
		s.cacheMu.Lock()
		if _, exists := s.analysisCache[c.Image]; !exists && len(s.analysisCache) >= maxAnalysisCacheEntries {
			log.Warn().
				Int("cache_entries", len(s.analysisCache)).
				Int("cache_limit", maxAnalysisCacheEntries).
				Msg("Discovery analysis cache reached size limit; clearing cache")
			s.analysisCache = make(map[string]*DiscoveryResult)
		}
		s.analysisCache[c.Image] = result
		s.lastCacheUpdate = time.Now()
		s.cacheMu.Unlock()

		log.Debug().
			Str("host", host.Hostname).
			Str("host_id", host.AgentID).
			Str("container", c.Name).
			Str("container_id", c.ID).
			Str("image", c.Image).
			Str("service_type", result.ServiceType).
			Float64("confidence", result.Confidence).
			Msg("AI analyzed container")
	}

	// Skip unknown/low-confidence results
	if result.ServiceType == "unknown" || result.Confidence < 0.5 {
		log.Debug().
			Str("host", host.Hostname).
			Str("host_id", host.AgentID).
			Str("container", c.Name).
			Str("container_id", c.ID).
			Str("image", c.Image).
			Str("service_type", result.ServiceType).
			Float64("confidence", result.Confidence).
			Msg("Skipping low-confidence or unknown AI discovery result")
		return nil
	}

	// Build CLI access string
	cliAccess := result.CLICommand
	if cliAccess != "" {
		safeContainerName := shellQuoteIfNeeded(sanitizeText(c.Name, maxAppFieldLength))
		// Replace placeholder with actual container name
		cliAccess = strings.ReplaceAll(cliAccess, "{container}", safeContainerName)
		cliAccess = strings.ReplaceAll(cliAccess, "${container}", safeContainerName)
	}

	hostID := sanitizeText(host.AgentID, maxAppFieldLength)
	hostname := sanitizeText(host.Hostname, maxAppFieldLength)
	containerID := sanitizeText(c.ID, maxAppFieldLength)
	containerName := sanitizeText(c.Name, maxAppFieldLength)
	if hostname == "" {
		hostname = "unknown-host"
	}
	if containerName == "" {
		containerName = "unknown-container"
	}

	// Extract ports
	var ports []int
	for _, p := range c.Ports {
		if p.PublicPort > 0 && p.PublicPort <= 65535 {
			ports = append(ports, int(p.PublicPort))
		} else if p.PrivatePort > 0 && p.PrivatePort <= 65535 {
			ports = append(ports, int(p.PrivatePort))
		}
	}

	return &DiscoveredApp{
		ID:            fmt.Sprintf("docker:%s:%s", hostname, containerName),
		Type:          result.ServiceType,
		Name:          result.ServiceName,
		Category:      result.Category,
		RunsIn:        "docker",
		HostID:        hostID,
		Hostname:      hostname,
		ContainerID:   containerID,
		ContainerName: containerName,
		Ports:         ports,
		CLIAccess:     cliAccess,
		Confidence:    result.Confidence,
		DetectedAt:    time.Now(),
		AIReasoning:   result.Reasoning,
	}
}

// buildContainerInfo extracts relevant information from a container for AI analysis.
func (s *Service) buildContainerInfo(container models.DockerContainer) ContainerInfo {
	info := ContainerInfo{
		Name:   sanitizeText(c.Name, maxPromptFieldLength),
		Image:  sanitizeText(c.Image, maxPromptFieldLength),
		Status: sanitizeText(c.Status, maxPromptFieldLength),
	}

	// Extract ports
	for _, p := range c.Ports {
		if len(info.Ports) >= maxPromptPortCount {
			break
		}
		if p.PrivatePort <= 0 || p.PrivatePort > 65535 {
			continue
		}
		hostPort := 0
		if p.PublicPort > 0 && p.PublicPort <= 65535 {
			hostPort = int(p.PublicPort)
		}
		info.Ports = append(info.Ports, PortInfo{
			HostPort:      hostPort,
			ContainerPort: int(p.PrivatePort),
			Protocol:      sanitizeProtocol(p.Protocol),
		})
	}

	// Extract labels
	if len(c.Labels) > 0 {
		info.Labels = make(map[string]string)
		for key, value := range c.Labels {
			if len(info.Labels) >= maxPromptLabelCount {
				break
			}
			safeKey := sanitizeText(key, maxPromptFieldLength)
			if safeKey == "" {
				continue
			}
			info.Labels[safeKey] = sanitizeText(value, maxPromptFieldLength)
		}
		if len(info.Labels) == 0 {
			info.Labels = nil
		}
	}

	// Extract mount destinations
	for _, m := range c.Mounts {
		if len(info.Mounts) >= maxPromptMountCount {
			break
		}
		if destination := sanitizeText(m.Destination, maxPromptFieldLength); destination != "" {
			info.Mounts = append(info.Mounts, destination)
		}
	}

	// Extract network names
	for _, n := range c.Networks {
		if len(info.Networks) >= maxPromptNetworkCount {
			break
		}
		if networkName := sanitizeText(n.Name, maxPromptFieldLength); networkName != "" {
			info.Networks = append(info.Networks, networkName)
		}
	}

	return info
}

// buildAnalysisPrompt creates the prompt for AI container analysis.
func (s *Service) buildAnalysisPrompt(info ContainerInfo) (string, error) {
	// Convert info to JSON for the prompt
	infoJSON, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal container info for analysis prompt: %w", err)
	}

	return fmt.Sprintf(`Analyze this Docker container and identify what service or application it's running.

Container Information:
%s

Based on the image name, ports, labels, environment variables, mounts, and other signals, determine:
1. What service/application is this? (e.g., postgres, redis, nginx, proxmox-backup-server, grafana, etc.)
2. What category does it belong to? (database, cache, web, backup, monitoring, message_queue, storage, etc.)
3. How should CLI commands be executed for this service?

Respond in this exact JSON format:
{
  "service_type": "the_service_type",
  "service_name": "Human Readable Name",
  "category": "category",
  "cli_command": "docker exec {container} <cli-tool>",
  "confidence": 0.95,
  "reasoning": "Brief explanation of why you identified it this way"
}

Important guidelines:
- service_type should be lowercase, no spaces (e.g., "postgres", "redis", "pbs", "nginx")
- For CLI command, use {container} as a placeholder for the container name
- If the service has a CLI tool, include it (e.g., "docker exec {container} psql -U postgres" for PostgreSQL)
- If no CLI is applicable, use empty string for cli_command
- Set confidence between 0 and 1 (1 = certain, 0.5 = guess)
- If you cannot identify the service, use service_type "unknown" with low confidence

Common services to look for:
- Databases: PostgreSQL, MySQL, MariaDB, MongoDB, Redis, Elasticsearch
- Backup: Proxmox Backup Server (PBS), Restic, Borg
- Web: Nginx, Apache, Traefik, Caddy, HAProxy
- Monitoring: Prometheus, Grafana, Loki, Alertmanager
- Message queues: RabbitMQ, Kafka
- Storage: MinIO, Nextcloud
- Home automation: Home Assistant
- Media: Plex, Jellyfin
- CI/CD: Jenkins, Drone, GitLab Runner

Respond with ONLY the JSON, no other text.`, string(infoJSON)), nil
}

// parseAIResponse parses the AI's JSON response.
func (s *Service) parseAIResponse(response string) *DiscoveryResult {
	// Try to extract JSON from the response
	response = strings.TrimSpace(response)
	if response == "" {
		return nil
	}
	if len(response) > maxAIResponseLength {
		log.Debug().
			Int("response_length", len(response)).
			Msg("AI response exceeded maximum allowed length")
		return nil
	}

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

	// Find JSON object in response
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	var result DiscoveryResult
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		log.Debug().
			Err(err).
			Str("response", sanitizeText(response, maxAIParseLogFieldLength)).
			Msg("Failed to parse AI response as JSON")
		return nil
	}

	result = normalizeDiscoveryResult(result)

	return &result
}

// saveDiscoveries persists discovered applications to the knowledge store.
func (s *Service) saveDiscoveries(apps []DiscoveredApp) {
	if s.knowledgeStore == nil {
		return
	}

	for _, app := range apps {
		// Create a descriptive note for each discovered application
		title := fmt.Sprintf("%s (%s)", app.Name, app.RunsIn)

		var content string
		if app.CLIAccess != "" {
			content = fmt.Sprintf(
				"Detected %s running in %s on %s. CLI access: %s",
				app.Name,
				app.RunsIn,
				app.Hostname,
				app.CLIAccess,
			)
		} else {
			content = fmt.Sprintf(
				"Detected %s running in %s on %s. No CLI access available.",
				app.Name,
				app.RunsIn,
				app.Hostname,
			)
		}

		// Save to knowledge store under the host's ID
		err := s.knowledgeStore.SaveNote(
			app.HostID,
			app.Hostname,
			"host",
			knowledge.CategoryInfra,
			title,
			content,
		)
		if err != nil {
			log.Warn().
				Err(err).
				Str("app_id", app.ID).
				Str("host", app.Hostname).
				Msg("Failed to save infrastructure discovery to knowledge store")
		}
	}
}

func (s *Service) tryStartDiscovery() bool {
	s.discoveryMu.Lock()
	defer s.discoveryMu.Unlock()
	if s.discoveryRun {
		return false
	}
	s.discoveryRun = true
	return true
}

func (s *Service) finishDiscovery() {
	s.discoveryMu.Lock()
	s.discoveryRun = false
	s.discoveryMu.Unlock()
}

// GetDiscoveries returns the cached list of discovered applications.
func (s *Service) GetDiscoveries() []DiscoveredApp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]DiscoveredApp, len(s.discoveries))
	copy(result, s.discoveries)
	return result
}

// GetLastRun returns the time of the last discovery run.
func (s *Service) GetLastRun() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastRun
}

// ForceRefresh triggers an immediate discovery scan.
func (s *Service) ForceRefresh(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.isStopped() {
		log.Debug().Msg("Infrastructure discovery service stopped, skipping force refresh")
		return
	}

	s.mu.RLock()
	serviceCtx := s.lifecycleCtx
	running := s.running
	s.mu.RUnlock()
	if running && serviceCtx != nil {
		ctx = serviceCtx
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Stack().
					Msg("Recovered from panic in ForceRefresh infrastructure discovery")
			}
		}()
		if s.isStopped() || ctx.Err() != nil {
			log.Debug().Msg("Infrastructure discovery force refresh canceled before execution")
			return
		}

		s.RunDiscovery(ctx)
	}()
}

// ClearCache clears the analysis cache, forcing re-analysis of all containers.
func (s *Service) ClearCache() {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	s.analysisCache = make(map[string]*analysisCacheEntry)
}

// GetStatusSnapshot returns a typed status snapshot for the discovery service.
func (s *Service) GetStatusSnapshot() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	s.cacheMu.RLock()
	cacheSize := len(s.analysisCache)
	s.cacheMu.RUnlock()

	return ServiceStatus{
		Running:        s.running,
		LastRun:        s.lastRun,
		Interval:       s.interval,
		DiscoveredApps: len(s.discoveries),
		CacheSize:      cacheSize,
		AIAnalyzerSet:  s.aiAnalyzer != nil,
	}
}

func normalizeDiscoveryResult(result DiscoveryResult) DiscoveryResult {
	result.ServiceType = normalizeIdentifier(result.ServiceType, maxServiceTypeLength)
	if result.ServiceType == "" {
		result.ServiceType = "unknown"
	}

	result.ServiceName = sanitizeText(result.ServiceName, maxServiceNameLength)
	if result.ServiceName == "" {
		result.ServiceName = "Unknown"
	}

	result.Category = normalizeIdentifier(result.Category, maxCategoryLength)
	if result.Category == "" {
		result.Category = "unknown"
	}

	result.CLICommand = sanitizeCLICommand(result.CLICommand)
	result.Reasoning = sanitizeText(result.Reasoning, maxReasoningLength)
	result.Confidence = clampConfidence(result.Confidence)

	return result
}

func normalizeIdentifier(input string, maxLen int) string {
	input = strings.ToLower(sanitizeText(input, maxLen))
	if input == "" {
		return ""
	}
	if !identifierPattern.MatchString(input) {
		return ""
	}
	return input
}

func sanitizeCLICommand(input string) string {
	command := sanitizeText(input, maxCLICommandLength)
	if command == "" {
		return ""
	}
	commandLower := strings.ToLower(command)
	if !strings.HasPrefix(commandLower, "docker exec ") && !strings.HasPrefix(commandLower, "docker container exec ") {
		return ""
	}
	if strings.Contains(command, ";") ||
		strings.Contains(command, "&&") ||
		strings.Contains(command, "||") ||
		strings.Contains(command, "`") ||
		strings.Contains(command, "$(") {
		return ""
	}
	if !strings.Contains(command, "{container}") && !strings.Contains(command, "${container}") {
		return ""
	}
	return command
}

func sanitizeProtocol(input string) string {
	protocol := strings.ToLower(sanitizeText(input, 16))
	switch protocol {
	case "", "tcp", "udp", "sctp":
		return protocol
	default:
		return ""
	}
}

func sanitizeText(input string, maxLen int) string {
	if input == "" || maxLen <= 0 {
		return ""
	}

	var b strings.Builder
	b.Grow(len(input))

	pendingSpace := false
	for _, r := range input {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			pendingSpace = true
			continue
		}
		if pendingSpace && b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
		pendingSpace = false
	}

	return truncateText(strings.TrimSpace(b.String()), maxLen)
}

func truncateText(input string, maxLen int) string {
	if maxLen <= 0 || input == "" {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= maxLen {
		return input
	}
	return string(runes[:maxLen])
}

func clampConfidence(confidence float64) float64 {
	if math.IsNaN(confidence) || math.IsInf(confidence, 0) {
		return 0
	}
	if confidence < 0 {
		return 0
	}
	if confidence > 1 {
		return 1
	}
	return confidence
}

func shellQuoteIfNeeded(input string) string {
	if input == "" {
		return ""
	}
	if safeShellNamePattern.MatchString(input) {
		return input
	}
	escaped := strings.ReplaceAll(input, `'`, `'"'"'`)
	return "'" + escaped + "'"
}
