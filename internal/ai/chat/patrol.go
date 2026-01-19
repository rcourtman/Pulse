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
		result.Error = fmt.Errorf("AI service not available")
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
	return `You are performing a scheduled infrastructure patrol for Pulse, a Proxmox monitoring system.

## Your Task
Analyze the current infrastructure state and identify issues that require human attention. Use the available tools to gather context.

## Available Tools
Use these tools to gather information:
- pulse_get_topology: Get current state of all monitored infrastructure
- pulse_get_metrics: Get metrics for trend analysis
- pulse_get_active_alerts: Get currently active alerts
- pulse_get_findings: Get existing patrol findings

## Steps
1. First call pulse_get_topology to see what's being monitored
2. Check pulse_get_active_alerts for any ongoing issues
3. Review pulse_get_findings to see previously identified issues
4. For any resources that look concerning, use pulse_get_metrics for trend analysis

## Output Format
For each issue found, output in this EXACT format:

[FINDING]
KEY: <stable-issue-key>
SEVERITY: critical|warning|watch|info
CATEGORY: performance|reliability|security|capacity|configuration
RESOURCE: <resource-name>
RESOURCE_TYPE: node|vm|container|docker_container|storage|host
TITLE: <brief title>
DESCRIPTION: <detailed description>
RECOMMENDATION: <actionable recommendation>
EVIDENCE: <supporting data>
[/FINDING]

## Severity Guidelines (be conservative)
- CRITICAL: Service down, data loss imminent, disk >95%, node offline
- WARNING: Disk >85%, memory >90% sustained, backup failed >48h
- WATCH: Trends approaching thresholds within 7 days
- INFO: Minor config issues only

## DO NOT Report
- Stopped VMs/containers (unless they crashed with autostart enabled)
- Minor fluctuations from baseline
- Resources simply "busier than usual" but not near limits

If everything is healthy, output NO findings. An empty report is the best report.

Begin your patrol now.`
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

	validCategories := map[string]bool{"performance": true, "reliability": true, "security": true, "capacity": true, "configuration": true}
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

// generateFindingID creates a stable ID for a finding
func generateFindingID(resourceID, category, key string) string {
	input := fmt.Sprintf("%s:%s:%s", resourceID, category, key)
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash[:8])
}

// CreatePatrolSession creates a dedicated session for patrol
func (p *PatrolService) CreatePatrolSession(ctx context.Context) error {
	if p.service == nil || !p.service.IsRunning() {
		return fmt.Errorf("AI service not available")
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
