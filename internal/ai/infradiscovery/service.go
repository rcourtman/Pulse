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
	"strings"
	"sync"
	"time"

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
	running        bool
	discoveries    []DiscoveredApp

	// Cache to avoid re-analyzing the same containers
	// Key: image name, Value: analysis result
	analysisCache     map[string]*analysisCacheEntry
	cacheMu           sync.RWMutex
	cacheExpiry       time.Duration
	aiAnalysisTimeout time.Duration
}

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

// DefaultConfig returns the default discovery configuration.
func DefaultConfig() Config {
	return Config{
		Interval:          5 * time.Minute,
		CacheExpiry:       1 * time.Hour,
		AIAnalysisTimeout: 45 * time.Second,
	}
}

// NewService creates a new infrastructure discovery service.
func NewService(stateProvider StateProvider, knowledgeStore *knowledge.Store, cfg Config) *Service {
	if cfg.Interval == 0 {
		cfg.Interval = 5 * time.Minute
	}
	if cfg.CacheExpiry == 0 {
		cfg.CacheExpiry = 1 * time.Hour
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

	// Run immediately on startup
	goRecover("initial infrastructure discovery", func() { s.RunDiscovery(ctx) })

	// Start periodic discovery loop
	goRecover("infrastructure discovery loop", func() { s.discoveryLoop(ctx) })
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

// discoveryLoop runs periodic discovery.
func (s *Service) discoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.RunDiscovery(ctx)
		case <-s.stopCh:
			log.Info().Msg("Stopping infrastructure discovery service")
			return
		case <-ctx.Done():
			log.Info().Msg("Infrastructure discovery context cancelled")
			return
		}
	}
}

// RunDiscovery performs a discovery scan using AI analysis.
func (s *Service) RunDiscovery(ctx context.Context) []DiscoveredApp {
	start := time.Now()
	state := s.stateProvider.GetState()

	s.mu.RLock()
	analyzer := s.aiAnalyzer
	s.mu.RUnlock()

	if analyzer == nil {
		log.Debug().Msg("AI analyzer not set, skipping discovery")
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
		log.Debug().Msg("No Docker containers found for discovery")
		s.mu.Lock()
		s.lastRun = time.Now()
		s.mu.Unlock()
		return apps
	}

	// Analyze containers (check cache first, batch uncached ones)
	for _, item := range allContainers {
		select {
		case <-ctx.Done():
			log.Info().Err(ctx.Err()).Msg("Infrastructure discovery cancelled")
			return apps
		default:
		}
		app := s.analyzeContainer(ctx, analyzer, item.Container, item.Host)
		if app != nil {
			apps = append(apps, *app)
		}
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
func (s *Service) analyzeContainer(ctx context.Context, analyzer AIAnalyzer, container models.DockerContainer, host models.DockerHost) *DiscoveredApp {
	if ctx == nil {
		ctx = context.Background()
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
			Str("container", container.Name).
			Str("image", container.Image).
			Msg("Using cached analysis result")
	} else {
		// Build container info for AI analysis
		info := s.buildContainerInfo(container)

		// Create analysis prompt
		prompt, err := s.buildAnalysisPrompt(info)
		if err != nil {
			log.Warn().
				Err(err).
				Str("container", c.Name).
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
				Str("container", container.Name).
				Str("response", response).
				Msg("Failed to parse AI response")
			return nil
		}

		// Cache the result
		s.cacheMu.Lock()
		s.analysisCache[container.Image] = &analysisCacheEntry{
			result:   result,
			cachedAt: time.Now(),
		}
		s.cacheMu.Unlock()

		log.Debug().
			Str("container", container.Name).
			Str("image", container.Image).
			Str("service_type", result.ServiceType).
			Float64("confidence", result.Confidence).
			Msg("AI analyzed container")
	}

	// Skip unknown/low-confidence results
	if result.ServiceType == "unknown" || result.Confidence < 0.5 {
		return nil
	}

	// Build CLI access string
	cliAccess := result.CLICommand
	if cliAccess != "" {
		// Replace placeholder with actual container name
		cliAccess = strings.ReplaceAll(cliAccess, "{container}", container.Name)
		cliAccess = strings.ReplaceAll(cliAccess, "${container}", container.Name)
	}

	// Extract ports
	var ports []int
	for _, port := range container.Ports {
		if port.PublicPort > 0 {
			ports = append(ports, int(port.PublicPort))
		} else if port.PrivatePort > 0 {
			ports = append(ports, int(port.PrivatePort))
		}
	}

	return &DiscoveredApp{
		ID:            fmt.Sprintf("docker:%s:%s", host.Hostname, container.Name),
		Type:          result.ServiceType,
		Name:          result.ServiceName,
		Category:      result.Category,
		RunsIn:        "docker",
		HostID:        host.AgentID,
		Hostname:      host.Hostname,
		ContainerID:   container.ID,
		ContainerName: container.Name,
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
		Name:   container.Name,
		Image:  container.Image,
		Status: container.Status,
	}

	// Extract ports
	for _, port := range container.Ports {
		info.Ports = append(info.Ports, PortInfo{
			HostPort:      int(port.PublicPort),
			ContainerPort: int(port.PrivatePort),
			Protocol:      port.Protocol,
		})
	}

	// Extract labels
	if len(container.Labels) > 0 {
		info.Labels = container.Labels
	}

	// Extract mount destinations
	for _, mount := range container.Mounts {
		info.Mounts = append(info.Mounts, mount.Destination)
	}

	// Extract network names
	for _, network := range container.Networks {
		info.Networks = append(info.Networks, network.Name)
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
			Str("response", response).
			Msg("Failed to parse AI response as JSON")
		return nil
	}

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
	goRecover("ForceRefresh infrastructure discovery", func() { s.RunDiscovery(ctx) })
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

// GetStatus returns the current service status.
func (s *Service) GetStatus() map[string]interface{} {
	return s.GetStatusSnapshot().ToMap()
}
