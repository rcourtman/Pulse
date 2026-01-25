package investigation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ChatService interface for interacting with the AI chat system
type ChatService interface {
	CreateSession(ctx context.Context) (*Session, error)
	ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)
	DeleteSession(ctx context.Context, sessionID string) error
	SetAutonomousMode(enabled bool)
}

// CommandExecutor interface for executing commands directly (bypasses LLM)
type CommandExecutor interface {
	ExecuteCommand(ctx context.Context, command, targetHost string) (output string, exitCode int, err error)
}

// CommandResult represents the result of a command execution
type CommandResult struct {
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Error    string `json:"error,omitempty"`
}

// Session represents a chat session
type Session struct {
	ID string `json:"id"`
}

// ExecuteRequest represents a chat execution request
type ExecuteRequest struct {
	Prompt    string `json:"prompt"`
	SessionID string `json:"session_id,omitempty"`
}

// StreamCallback is called for each streaming event
type StreamCallback func(event StreamEvent)

// StreamEvent represents a streaming event
type StreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Message represents a chat message
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// FindingsStore interface for updating findings
type FindingsStore interface {
	Get(id string) *Finding
	Update(f *Finding) bool
}

// Finding represents a patrol finding (simplified for this package)
type Finding struct {
	ID                     string     `json:"id"`
	Severity               string     `json:"severity"`
	Category               string     `json:"category"`
	ResourceID             string     `json:"resource_id"`
	ResourceName           string     `json:"resource_name"`
	ResourceType           string     `json:"resource_type"`
	Title                  string     `json:"title"`
	Description            string     `json:"description"`
	Recommendation         string     `json:"recommendation,omitempty"`
	Evidence               string     `json:"evidence,omitempty"`
	InvestigationSessionID string     `json:"investigation_session_id,omitempty"`
	InvestigationStatus    string     `json:"investigation_status,omitempty"`
	InvestigationOutcome   string     `json:"investigation_outcome,omitempty"`
	LastInvestigatedAt     *time.Time `json:"last_investigated_at,omitempty"`
	InvestigationAttempts  int        `json:"investigation_attempts"`
}

// ApprovalStore interface for queuing fixes for approval
type ApprovalStore interface {
	Create(approval *Approval) error
}

// Approval represents a queued approval request
type Approval struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // "investigation_fix"
	FindingID   string    `json:"finding_id"`
	SessionID   string    `json:"session_id"`
	Description string    `json:"description"`
	Command     string    `json:"command"`
	RiskLevel   string    `json:"risk_level"`
	CreatedAt   time.Time `json:"created_at"`
}

// InfrastructureContextProvider provides discovered infrastructure context for investigations
type InfrastructureContextProvider interface {
	GetInfrastructureContext() string
}

// Orchestrator manages the investigation lifecycle
type Orchestrator struct {
	mu sync.RWMutex

	chatService     ChatService
	commandExecutor CommandExecutor
	store           *Store
	findingsStore   FindingsStore
	approvalStore   ApprovalStore
	guardrails      *Guardrails
	config          InvestigationConfig

	// Infrastructure context provider for CLI access information
	infraContextProvider InfrastructureContextProvider

	// Track running investigations
	runningCount int
	runningMu    sync.Mutex
}

// NewOrchestrator creates a new investigation orchestrator
func NewOrchestrator(
	chatService ChatService,
	store *Store,
	findingsStore FindingsStore,
	approvalStore ApprovalStore,
	config InvestigationConfig,
) *Orchestrator {
	return &Orchestrator{
		chatService:   chatService,
		store:         store,
		findingsStore: findingsStore,
		approvalStore: approvalStore,
		guardrails:    NewGuardrails(),
		config:        config,
	}
}

// SetConfig updates the orchestrator configuration
func (o *Orchestrator) SetConfig(config InvestigationConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.config = config
}

// SetCommandExecutor sets the command executor for auto-executing fixes
func (o *Orchestrator) SetCommandExecutor(executor CommandExecutor) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.commandExecutor = executor
}

// SetInfrastructureContextProvider sets the provider for discovered infrastructure context
// This enables investigations to know where services run (Docker, systemd, native)
// and propose correct CLI commands for remediation
func (o *Orchestrator) SetInfrastructureContextProvider(provider InfrastructureContextProvider) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.infraContextProvider = provider
}

// GetConfig returns the current configuration
func (o *Orchestrator) GetConfig() InvestigationConfig {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.config
}

// CanStartInvestigation checks if a new investigation can be started
func (o *Orchestrator) CanStartInvestigation() bool {
	o.runningMu.Lock()
	defer o.runningMu.Unlock()
	return o.runningCount < o.config.MaxConcurrent
}

// InvestigateFinding starts an investigation for a finding
func (o *Orchestrator) InvestigateFinding(ctx context.Context, finding *Finding, autonomyLevel string) error {
	// Check if we can start a new investigation
	if !o.CanStartInvestigation() {
		return fmt.Errorf("maximum concurrent investigations reached (%d)", o.config.MaxConcurrent)
	}

	// Increment running count
	o.runningMu.Lock()
	o.runningCount++
	o.runningMu.Unlock()

	// Decrement when done
	defer func() {
		o.runningMu.Lock()
		o.runningCount--
		o.runningMu.Unlock()
	}()

	// Enable autonomous mode for this investigation
	// This allows read-only commands to be auto-approved without user confirmation
	o.chatService.SetAutonomousMode(true)
	defer o.chatService.SetAutonomousMode(false)

	// Create chat session
	session, err := o.chatService.CreateSession(ctx)
	if err != nil {
		return fmt.Errorf("failed to create chat session: %w", err)
	}

	// Create investigation record
	investigation := o.store.Create(finding.ID, session.ID)

	// Update finding with investigation info
	finding.InvestigationSessionID = session.ID
	finding.InvestigationStatus = string(StatusRunning)
	finding.InvestigationAttempts++
	o.updateFinding(finding)

	// Update investigation status to running
	o.store.UpdateStatus(investigation.ID, StatusRunning)

	// Build the investigation prompt
	prompt := o.buildInvestigationPrompt(finding)

	log.Info().
		Str("finding_id", finding.ID).
		Str("session_id", session.ID).
		Str("investigation_id", investigation.ID).
		Str("severity", finding.Severity).
		Msg("Starting investigation")

	// Execute with timeout and turn limit
	err = o.executeWithLimits(ctx, investigation, prompt)
	if err != nil {
		// Mark as failed
		o.store.Fail(investigation.ID, err.Error())
		finding.InvestigationStatus = string(StatusFailed)
		now := time.Now()
		finding.LastInvestigatedAt = &now
		o.updateFinding(finding)
		return fmt.Errorf("investigation failed: %w", err)
	}

	// Process the result
	err = o.processResult(ctx, investigation, finding, autonomyLevel)
	if err != nil {
		return fmt.Errorf("failed to process result: %w", err)
	}

	return nil
}

// buildInvestigationPrompt creates the investigation prompt for a finding
func (o *Orchestrator) buildInvestigationPrompt(finding *Finding) string {
	// Get infrastructure context if available
	var infraContext string
	o.mu.RLock()
	if o.infraContextProvider != nil {
		infraContext = o.infraContextProvider.GetInfrastructureContext()
	}
	o.mu.RUnlock()

	// Build infrastructure context section
	var infraSection string
	if infraContext != "" {
		infraSection = fmt.Sprintf(`
%s
**IMPORTANT**: When proposing commands, use the CLI access method shown above.
- If a service runs in Docker, use 'docker exec <container> <command>' instead of direct commands
- Example: For PBS in Docker, use 'docker exec pbs proxmox-backup-manager gc pbs-delly' not 'proxmox-backup-manager gc pbs-delly'
- This ensures commands execute in the correct environment where the service actually runs.

`, infraContext)
	}

	return fmt.Sprintf(`You are investigating a finding from Pulse Patrol. Your goal is to:
1. Understand the issue using available tools
2. Determine if it can be automatically fixed
3. If fixable, propose a specific remediation command
%s
## Finding Details
- **Title**: %s
- **Severity**: %s
- **Category**: %s
- **Resource**: %s (%s, type: %s)
- **Description**: %s
%s
%s

## Investigation Steps
1. Use monitoring tools to gather current state of the resource
2. Analyze the root cause of the issue
3. Determine if automatic remediation is possible
4. If a fix is possible, provide a SPECIFIC command that can be executed

## Response Format
After your investigation, provide a summary in this exact format:

### Investigation Summary
[Brief summary of what you found]

### Root Cause
[Explanation of the root cause]

### Recommendation
[What should be done - be specific]

### Proposed Fix
If you have a specific command that can fix this issue, provide it as:
PROPOSED_FIX: <command>
TARGET_HOST: <hostname or "local">

If the issue cannot be automatically fixed, explain why:
CANNOT_FIX: <reason>

If the issue needs human attention, state:
NEEDS_ATTENTION: <reason>

Remember:
- Be thorough but efficient (you have limited turns)
- Only propose commands you're confident will help
- Never propose destructive commands (they'll be blocked anyway)
- Focus on the specific resource mentioned in the finding`,
		infraSection,
		finding.Title,
		finding.Severity,
		finding.Category,
		finding.ResourceName,
		finding.ResourceID,
		finding.ResourceType,
		finding.Description,
		formatOptional("Evidence", finding.Evidence),
		formatOptional("Recommendation", finding.Recommendation),
	)
}

func formatOptional(label, value string) string {
	if value == "" {
		return ""
	}
	return fmt.Sprintf("- **%s**: %s", label, value)
}

// executeWithLimits runs the investigation with turn and timeout limits
func (o *Orchestrator) executeWithLimits(ctx context.Context, investigation *InvestigationSession, prompt string) error {
	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	log.Debug().
		Str("investigation_id", investigation.ID).
		Str("session_id", investigation.SessionID).
		Dur("timeout", o.config.Timeout).
		Int("max_turns", o.config.MaxTurns).
		Msg("Starting investigation execution with limits")

	// Track turn count
	turnCount := 0
	maxTurns := o.config.MaxTurns

	var lastContent string
	var streamErr error

	// Execute the prompt
	req := ExecuteRequest{
		Prompt:    prompt,
		SessionID: investigation.SessionID,
	}

	log.Debug().
		Str("session_id", investigation.SessionID).
		Int("prompt_len", len(prompt)).
		Msg("Calling chatService.ExecuteStream")

	err := o.chatService.ExecuteStream(timeoutCtx, req, func(event StreamEvent) {
		switch event.Type {
		case "content":
			var data struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(event.Data, &data); err == nil {
				lastContent += data.Text
			}
		case "tool_end":
			// Each tool call counts as a turn
			turnCount++
			o.store.IncrementTurnCount(investigation.ID)

			if turnCount >= maxTurns {
				// We've hit the limit, but let the current response complete
				log.Warn().
					Str("investigation_id", investigation.ID).
					Int("turn_count", turnCount).
					Int("max_turns", maxTurns).
					Msg("Investigation hit turn limit")
			}
		case "error":
			var data struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &data); err == nil {
				streamErr = fmt.Errorf("stream error: %s", data.Message)
			}
		}
	})

	if err != nil {
		return err
	}
	if streamErr != nil {
		return streamErr
	}

	// Store the summary
	investigation.Summary = lastContent
	o.store.Update(investigation)

	return nil
}

// processResult analyzes the investigation output and takes appropriate action
func (o *Orchestrator) processResult(ctx context.Context, investigation *InvestigationSession, finding *Finding, autonomyLevel string) error {
	summary := investigation.Summary

	// Parse the summary for fix proposals
	fix, outcome := o.parseInvestigationSummary(summary)

	now := time.Now()
	finding.LastInvestigatedAt = &now

	if fix != nil {
		// Check if fix requires approval
		requiresApproval := o.guardrails.RequiresApproval(
			finding.Severity,
			autonomyLevel,
			fix.Commands[0],
			o.config.CriticalRequireApproval,
		)

		fix.RiskLevel = o.guardrails.ClassifyRisk(fix.Commands[0])
		fix.Destructive = o.guardrails.IsDestructiveAction(fix.Commands[0])

		if requiresApproval {
			// Queue for approval
			approval := &Approval{
				ID:          uuid.New().String(),
				Type:        "investigation_fix",
				FindingID:   finding.ID,
				SessionID:   investigation.SessionID,
				Description: fix.Description,
				Command:     fix.Commands[0],
				RiskLevel:   fix.RiskLevel,
				CreatedAt:   time.Now(),
			}

			if o.approvalStore != nil {
				if err := o.approvalStore.Create(approval); err != nil {
					log.Error().Err(err).Msg("Failed to queue fix for approval")
				} else {
					o.store.SetApprovalID(investigation.ID, approval.ID)
					outcome = OutcomeFixQueued
				}
			}
		} else {
			// In full autonomy mode for non-critical, non-destructive fixes
			// Execute the fix automatically
			if o.commandExecutor != nil {
				log.Info().
					Str("finding_id", finding.ID).
					Str("command", fix.Commands[0]).
					Str("target_host", fix.TargetHost).
					Str("risk_level", fix.RiskLevel).
					Msg("Auto-executing fix (full autonomy mode)")

				output, exitCode, execErr := o.commandExecutor.ExecuteCommand(ctx, fix.Commands[0], fix.TargetHost)
				if execErr != nil {
					log.Error().
						Err(execErr).
						Str("finding_id", finding.ID).
						Str("command", fix.Commands[0]).
						Msg("Auto-executed fix failed")
					outcome = OutcomeFixFailed
					fix.Rationale = fmt.Sprintf("%s\n\nAuto-execution failed: %v", fix.Rationale, execErr)
				} else if exitCode != 0 {
					log.Warn().
						Str("finding_id", finding.ID).
						Str("command", fix.Commands[0]).
						Int("exit_code", exitCode).
						Str("output", output).
						Msg("Auto-executed fix returned non-zero exit code")
					outcome = OutcomeFixFailed
					fix.Rationale = fmt.Sprintf("%s\n\nAuto-execution returned exit code %d:\n%s", fix.Rationale, exitCode, output)
				} else {
					log.Info().
						Str("finding_id", finding.ID).
						Str("command", fix.Commands[0]).
						Str("output", output).
						Msg("Auto-executed fix successfully")
					outcome = OutcomeFixExecuted
					fix.Rationale = fmt.Sprintf("%s\n\nAuto-executed successfully:\n%s", fix.Rationale, output)
				}
			} else {
				// No command executor available, fall back to queueing for approval
				log.Warn().
					Str("finding_id", finding.ID).
					Str("command", fix.Commands[0]).
					Msg("Full autonomy enabled but no command executor available, queueing for approval")
				outcome = OutcomeFixQueued
			}
		}

		o.store.Complete(investigation.ID, outcome, summary, fix)
	} else {
		o.store.Complete(investigation.ID, outcome, summary, nil)
	}

	// Update finding
	finding.InvestigationStatus = string(StatusCompleted)
	finding.InvestigationOutcome = string(outcome)
	o.updateFinding(finding)

	log.Info().
		Str("finding_id", finding.ID).
		Str("investigation_id", investigation.ID).
		Str("outcome", string(outcome)).
		Msg("Investigation completed")

	return nil
}

// parseInvestigationSummary extracts fix proposals from the investigation output
func (o *Orchestrator) parseInvestigationSummary(summary string) (*Fix, Outcome) {
	// Look for PROPOSED_FIX marker
	if idx := indexString(summary, "PROPOSED_FIX:"); idx >= 0 {
		// Extract the command
		remaining := summary[idx+len("PROPOSED_FIX:"):]
		// Find the end of the line
		endIdx := indexString(remaining, "\n")
		if endIdx < 0 {
			endIdx = len(remaining)
		}
		command := trim(remaining[:endIdx])

		// Look for TARGET_HOST
		targetHost := "local"
		if hostIdx := indexString(summary, "TARGET_HOST:"); hostIdx >= 0 {
			hostRemaining := summary[hostIdx+len("TARGET_HOST:"):]
			hostEndIdx := indexString(hostRemaining, "\n")
			if hostEndIdx < 0 {
				hostEndIdx = len(hostRemaining)
			}
			targetHost = trim(hostRemaining[:hostEndIdx])
		}

		if command != "" {
			return &Fix{
				ID:          uuid.New().String(),
				Description: "Proposed fix from investigation",
				Commands:    []string{command},
				TargetHost:  targetHost,
				Rationale:   summary,
			}, OutcomeFixQueued
		}
	}

	// Look for CANNOT_FIX marker
	if idx := indexString(summary, "CANNOT_FIX:"); idx >= 0 {
		return nil, OutcomeCannotFix
	}

	// Look for NEEDS_ATTENTION marker
	if idx := indexString(summary, "NEEDS_ATTENTION:"); idx >= 0 {
		return nil, OutcomeNeedsAttention
	}

	// Default to needs attention if we couldn't parse the output
	return nil, OutcomeNeedsAttention
}

func (o *Orchestrator) updateFinding(finding *Finding) {
	if o.findingsStore != nil {
		o.findingsStore.Update(finding)
	}
}

// trim removes leading and trailing whitespace
func trim(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

// ReinvestigateFinding triggers a re-investigation of a finding
func (o *Orchestrator) ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error {
	if o.findingsStore == nil {
		return fmt.Errorf("findings store not configured")
	}

	finding := o.findingsStore.Get(findingID)
	if finding == nil {
		return fmt.Errorf("finding not found: %s", findingID)
	}

	// Convert to our local Finding type
	localFinding := &Finding{
		ID:                     finding.ID,
		Severity:               finding.Severity,
		Category:               finding.Category,
		ResourceID:             finding.ResourceID,
		ResourceName:           finding.ResourceName,
		ResourceType:           finding.ResourceType,
		Title:                  finding.Title,
		Description:            finding.Description,
		Recommendation:         finding.Recommendation,
		Evidence:               finding.Evidence,
		InvestigationSessionID: finding.InvestigationSessionID,
		InvestigationStatus:    finding.InvestigationStatus,
		InvestigationOutcome:   finding.InvestigationOutcome,
		LastInvestigatedAt:     finding.LastInvestigatedAt,
		InvestigationAttempts:  finding.InvestigationAttempts,
	}

	return o.InvestigateFinding(ctx, localFinding, autonomyLevel)
}

// GetInvestigation returns an investigation by ID
func (o *Orchestrator) GetInvestigation(id string) *InvestigationSession {
	return o.store.Get(id)
}

// GetInvestigationByFinding returns the latest investigation for a finding
func (o *Orchestrator) GetInvestigationByFinding(findingID string) *InvestigationSession {
	return o.store.GetLatestByFinding(findingID)
}

// GetRunningInvestigations returns all running investigations
func (o *Orchestrator) GetRunningInvestigations() []*InvestigationSession {
	return o.store.GetRunning()
}

// GetRunningCount returns the number of running investigations
func (o *Orchestrator) GetRunningCount() int {
	return o.store.CountRunning()
}

// GetFixedCount returns the number of investigations that resulted in a fix being executed
func (o *Orchestrator) GetFixedCount() int {
	return o.store.CountFixed()
}
