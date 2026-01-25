package chat

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// PatrolFinding represents a finding from AI patrol analysis
type PatrolFinding struct {
	ID             string    `json:"id"`
	Key            string    `json:"key"`
	Severity       string    `json:"severity"`
	Category       string    `json:"category"`
	ResourceID     string    `json:"resource_id"`
	ResourceName   string    `json:"resource_name"`
	ResourceType   string    `json:"resource_type"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Recommendation string    `json:"recommendation"`
	Evidence       string    `json:"evidence"`
	Source         string    `json:"source"`
	DetectedAt     time.Time `json:"detected_at"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

// FindingsStore interface for storing patrol findings
type FindingsStore interface {
	Add(finding *PatrolFinding) bool // Returns true if finding is new
	GetActive() []*PatrolFinding
	GetDismissed() []*PatrolFinding
}

// PatrolStreamCallback is called for patrol streaming updates
type PatrolStreamCallback func(event PatrolStreamEvent)

// PatrolStreamEvent represents a streaming update from patrol
type PatrolStreamEvent struct {
	Type    string `json:"type"` // "start", "content", "thinking", "tool_use", "complete", "error"
	Content string `json:"content,omitempty"`
	Phase   string `json:"phase,omitempty"`
}

// PatrolResult contains the results of a patrol run
type PatrolResult struct {
	Findings     []*PatrolFinding
	NewFindings  int
	RawResponse  string
	InputTokens  int
	OutputTokens int
	Duration     time.Duration
	Error        error
}

// PatrolService runs AI patrol analysis
type PatrolService struct {
	mu sync.RWMutex

	service       *Service      // Chat service
	findingsStore FindingsStore // For storing findings
	sessionID     string        // Dedicated patrol session
	running       bool          // Whether a patrol is currently running
	lastRun       time.Time     // Last patrol run time
	lastResult    *PatrolResult // Last patrol result

	// Streaming support
	streamMu      sync.RWMutex
	subscribers   map[chan PatrolStreamEvent]struct{}
	currentOutput strings.Builder
}

// NewPatrolService creates a new patrol service
func NewPatrolService(service *Service) *PatrolService {
	return &PatrolService{
		service:     service,
		subscribers: make(map[chan PatrolStreamEvent]struct{}),
	}
}

// SetFindingsStore sets the findings store for persisting patrol results
func (p *PatrolService) SetFindingsStore(store FindingsStore) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.findingsStore = store
}

// IsRunning returns whether a patrol is currently executing
func (p *PatrolService) IsRunning() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// GetLastResult returns the most recent patrol result
func (p *PatrolService) GetLastResult() *PatrolResult {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastResult
}

// Subscribe returns a channel for receiving patrol stream events
func (p *PatrolService) Subscribe() chan PatrolStreamEvent {
	ch := make(chan PatrolStreamEvent, 100)
	p.streamMu.Lock()
	p.subscribers[ch] = struct{}{}
	p.streamMu.Unlock()
	return ch
}

// Unsubscribe removes a subscriber channel
func (p *PatrolService) Unsubscribe(ch chan PatrolStreamEvent) {
	p.streamMu.Lock()
	delete(p.subscribers, ch)
	p.streamMu.Unlock()
	close(ch)
}

// broadcast sends an event to all subscribers
func (p *PatrolService) broadcast(event PatrolStreamEvent) {
	p.streamMu.RLock()
	defer p.streamMu.RUnlock()

	for ch := range p.subscribers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

// RunPatrol executes a patrol analysis
func (p *PatrolService) RunPatrol(ctx context.Context) error {
	_, err := p.RunPatrolWithResult(ctx)
	return err
}

// RunPatrolWithResult executes a patrol analysis and returns detailed results
func (p *PatrolService) RunPatrolWithResult(ctx context.Context) (*PatrolResult, error) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return nil, fmt.Errorf("patrol already running")
	}
	p.running = true
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	start := time.Now()
	result := &PatrolResult{}

	// Check service is available
	if p.service == nil || !p.service.IsRunning() {
		result.Error = fmt.Errorf("Pulse Assistant service not available")
		return result, result.Error
	}

	// Build patrol prompt
	prompt := p.buildPatrolPrompt()

	log.Info().Msg("AI Patrol: Starting infrastructure analysis")
	p.broadcast(PatrolStreamEvent{Type: "start", Phase: "Starting patrol analysis"})

	// Clear output buffer
	p.streamMu.Lock()
	p.currentOutput.Reset()
	p.streamMu.Unlock()

	// Execute via chat service streaming
	var responseBuffer strings.Builder

	err := p.service.ExecuteStream(ctx, ExecuteRequest{
		Prompt:    prompt,
		SessionID: p.sessionID,
	}, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var contentEvent ContentData
			if json.Unmarshal(event.Data, &contentEvent) == nil && contentEvent.Text != "" {
				responseBuffer.WriteString(contentEvent.Text)
				p.streamMu.Lock()
				p.currentOutput.WriteString(contentEvent.Text)
				p.streamMu.Unlock()
				p.broadcast(PatrolStreamEvent{Type: "content", Content: contentEvent.Text})
			}
		case "thinking":
			var thinkingEvent ThinkingData
			if json.Unmarshal(event.Data, &thinkingEvent) == nil && thinkingEvent.Text != "" {
				p.broadcast(PatrolStreamEvent{Type: "thinking", Content: thinkingEvent.Text})
			}
		case "tool_start":
			p.broadcast(PatrolStreamEvent{Type: "tool_use", Phase: "Using tools to gather context"})
		}
	})

	if err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		p.broadcast(PatrolStreamEvent{Type: "error", Content: err.Error()})
		log.Error().Err(err).Msg("AI Patrol: Analysis failed")
		return result, err
	}

	result.RawResponse = responseBuffer.String()
	result.Duration = time.Since(start)

	// Parse findings from response
	findings := p.parseFindings(result.RawResponse)
	result.Findings = findings

	// Store findings
	p.mu.RLock()
	store := p.findingsStore
	p.mu.RUnlock()

	if store != nil {
		for _, f := range findings {
			if store.Add(f) {
				result.NewFindings++
			}
		}
	} else {
		result.NewFindings = len(findings)
	}

	// Save result
	p.mu.Lock()
	p.lastRun = time.Now()
	p.lastResult = result
	p.mu.Unlock()

	p.broadcast(PatrolStreamEvent{Type: "complete", Phase: "Patrol complete"})

	log.Info().
		Int("findings", len(findings)).
		Int("new_findings", result.NewFindings).
		Dur("duration", result.Duration).
		Msg("AI Patrol: Analysis complete")

	return result, nil
}

// buildPatrolPrompt creates the prompt for patrol analysis
func (p *PatrolService) buildPatrolPrompt() string {
	return `You are the AI Patrol agent for Pulse, an infrastructure monitoring system. You have comprehensive access to monitor Proxmox, Docker, Kubernetes, PBS, and host systems.

## Your Mission
Perform a thorough infrastructure health check. Find issues that need human attention - not just threshold breaches, but patterns, trends, and potential problems before they become critical.

## Your Toolkit
You have access to powerful monitoring tools:

**Infrastructure Overview:**
- pulse_get_topology - Complete snapshot of all nodes, VMs, containers, storage
- pulse_list_infrastructure - List all monitored resources
- pulse_search_resources - Find specific resources
- pulse_get_resource - Deep dive into a specific resource

**Intelligent Analysis:**
- pulse_get_metrics - Time-series data to analyze trends
- pulse_get_baselines - Learned NORMAL behavior for each resource (compare against this!)
- pulse_get_patterns - Detected anomaly patterns

**Storage & Data Safety:**
- pulse_list_storage - Storage pools and usage
- pulse_get_disk_health - SMART data, errors, failure predictions
- pulse_list_physical_disks - Physical disk information
- pulse_list_backups - Backup status and history
- pulse_list_pbs_jobs - PBS backup job status
- pulse_list_snapshots - VM/container snapshots

**Hardware Health:**
- pulse_get_temperatures - CPU and disk temperatures
- pulse_get_host_raid_status - RAID array health
- pulse_get_host_ceph_details - Ceph details on hosts
- pulse_get_ceph_status - Ceph cluster status

**Cluster & Network:**
- pulse_get_cluster_status - PVE cluster health
- pulse_get_connection_health - API connectivity
- pulse_get_network_stats - Network throughput
- pulse_get_diskio_stats - Disk I/O statistics
- pulse_get_replication - Replication job status

**Docker/Swarm:**
- pulse_get_swarm_status - Docker Swarm cluster
- pulse_list_docker_services - Swarm services
- pulse_list_docker_updates - Containers with available updates

**Kubernetes:**
- pulse_get_kubernetes_clusters - K8s cluster status
- pulse_get_kubernetes_nodes - Node health
- pulse_get_kubernetes_pods - Pod status

**Current Issues:**
- pulse_list_alerts - Active threshold alerts
- pulse_list_findings - Previously identified patrol findings
- pulse_list_resolved_alerts - Recently resolved alerts

## Patrol Strategy
1. **Start broad**: Get topology to understand what you're monitoring
2. **Check known issues**: Review active alerts and existing findings
3. **Analyze trends**: Use metrics + baselines to spot resources drifting toward problems
4. **Check hardware**: Disk health, temperatures, RAID status - catch failures early
5. **Verify backups**: Are critical systems being backed up successfully?
6. **Look for patterns**: Correlate data - high CPU + high disk I/O might indicate a specific problem

## What Makes a Good Finding
- **Actionable**: User can do something about it
- **Evidenced**: Back it up with data from your investigation
- **Contextual**: Compare against baselines, not just arbitrary thresholds
- **Predictive**: Trends heading toward problems, not just current breaches

## Output Format
For each issue, output:

[FINDING]
KEY: <stable-issue-key>
SEVERITY: critical|warning|watch|info
CATEGORY: performance|reliability|security|capacity|backup
RESOURCE: <resource-name>
RESOURCE_TYPE: node|vm|container|docker_container|storage|host|pbs
TITLE: <brief title>
DESCRIPTION: <detailed description with context>
RECOMMENDATION: <specific actionable steps>
EVIDENCE: <data from your investigation>
[/FINDING]

## Severity Guidelines
- CRITICAL: Immediate action needed - service down, imminent data loss, disk >95%, failing hardware
- WARNING: Action needed soon - disk >85% and growing, backup failures >24h, degraded RAID
- WATCH: Monitor closely - trends approaching thresholds, intermittent issues
- INFO: Awareness only - minor optimizations, non-urgent improvements

## DO NOT Report
- Stopped VMs/containers (unless autostart is enabled and they crashed)
- Resources within their normal baseline range
- Transient spikes that have already resolved
- Issues already covered by existing findings

If your investigation finds everything healthy, output NO findings. A clean patrol is the best outcome.

Begin your patrol investigation now.`
}

// parseFindings extracts findings from the AI response
func (p *PatrolService) parseFindings(response string) []*PatrolFinding {
	var findings []*PatrolFinding

	findingRe := regexp.MustCompile(`(?s)\[FINDING\](.*?)\[/FINDING\]`)
	matches := findingRe.FindAllStringSubmatch(response, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		block := match[1]

		finding := p.parseFindingBlock(block)
		if finding != nil {
			findings = append(findings, finding)
		}
	}

	return findings
}

// parseFindingBlock parses a single finding block
func (p *PatrolService) parseFindingBlock(block string) *PatrolFinding {
	extractField := func(name string) string {
		re := regexp.MustCompile(`(?i)` + name + `:\s*(.+?)(?:\n|$)`)
		match := re.FindStringSubmatch(block)
		if len(match) >= 2 {
			return strings.TrimSpace(match[1])
		}
		return ""
	}

	key := extractField("KEY")
	severity := strings.ToLower(extractField("SEVERITY"))
	category := strings.ToLower(extractField("CATEGORY"))
	resource := extractField("RESOURCE")
	resourceType := strings.ToLower(extractField("RESOURCE_TYPE"))
	title := extractField("TITLE")
	description := extractField("DESCRIPTION")
	recommendation := extractField("RECOMMENDATION")
	evidence := extractField("EVIDENCE")

	if key == "" || title == "" {
		return nil
	}

	validSeverities := map[string]bool{"critical": true, "warning": true, "watch": true, "info": true}
	if !validSeverities[severity] {
		severity = "info"
	}

	validCategories := map[string]bool{"performance": true, "reliability": true, "security": true, "capacity": true, "backup": true, "configuration": true}
	if !validCategories[category] {
		category = "configuration"
	}

	id := generateFindingID(resource, category, key)

	now := time.Now()
	return &PatrolFinding{
		ID:             id,
		Key:            key,
		Severity:       severity,
		Category:       category,
		ResourceID:     resource,
		ResourceName:   resource,
		ResourceType:   resourceType,
		Title:          title,
		Description:    description,
		Recommendation: recommendation,
		Evidence:       evidence,
		Source:         "ai-patrol",
		DetectedAt:     now,
		LastSeenAt:     now,
	}
}

// generateFindingID creates a stable ID for a finding based on resource, category, and key.
// All three components are included to ensure distinct issues on the same resource remain separate.
func generateFindingID(resourceID, category, key string) string {
	input := fmt.Sprintf("%s:%s:%s", resourceID, category, key)
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:8])
}

// CreatePatrolSession creates a dedicated session for patrol
func (p *PatrolService) CreatePatrolSession(ctx context.Context) error {
	if p.service == nil || !p.service.IsRunning() {
		return fmt.Errorf("Pulse Assistant service not available")
	}

	session, err := p.service.CreateSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to create patrol session: %w", err)
	}

	p.mu.Lock()
	p.sessionID = session.ID
	p.mu.Unlock()

	log.Info().Str("session_id", session.ID).Msg("AI Patrol: Created dedicated session")
	return nil
}

// GetSessionID returns the current patrol session ID
func (p *PatrolService) GetSessionID() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sessionID
}
