package investigation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/finding"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rcourtman/pulse-go-rewrite/internal/license"
	"github.com/rs/zerolog/log"
)

// ChatService interface for interacting with the AI chat system
type ChatService interface {
	CreateSession(ctx context.Context) (*Session, error)
	ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)
	DeleteSession(ctx context.Context, sessionID string) error
	ListAvailableTools(ctx context.Context, prompt string) []string
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
	Prompt         string `json:"prompt"`
	SessionID      string `json:"session_id,omitempty"`
	MaxTurns       int    `json:"max_turns,omitempty"`       // Override max agentic turns (0 = use default)
	AutonomousMode *bool  `json:"autonomous_mode,omitempty"` // Per-request autonomous override (nil = use service default)
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
	ID               string          `json:"id"`
	Role             string          `json:"role"`
	Content          string          `json:"content"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCallInfo  `json:"tool_calls,omitempty"`
	ToolResult       *ToolResultInfo `json:"tool_result,omitempty"`
	Timestamp        time.Time       `json:"timestamp"`
}

// ToolCallInfo represents a tool invocation in an investigation message
type ToolCallInfo struct {
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// ToolResultInfo represents the result of a tool invocation
type ToolResultInfo struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// FindingsStore interface for updating findings
type FindingsStore interface {
	Get(id string) *Finding
	Update(f *Finding) bool
}

// Finding is the shared finding type from the finding package.
// Type alias ensures full backwards compatibility — *investigation.Finding
// and *finding.Finding are the same type.
type Finding = finding.Finding

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

// AutonomyLevelProvider provides the current autonomy level (for re-checking before fix execution)
type AutonomyLevelProvider interface {
	GetCurrentAutonomyLevel() string
	IsFullModeUnlocked() bool
}

// FixVerifier verifies that a fix actually resolved the issue it was intended to fix.
type FixVerifier interface {
	VerifyFixResolved(ctx context.Context, finding *Finding) (bool, error)
}

// MetricsCallback allows the parent package to receive metrics events
// without creating circular imports.
type MetricsCallback interface {
	RecordInvestigationOutcome(outcome string)
	RecordFixVerification(result string)
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

	// Autonomy level provider for re-checking before fix execution
	autonomyProvider AutonomyLevelProvider

	// Fix verifier for post-fix verification
	fixVerifier FixVerifier

	// License checker for defense-in-depth autonomy clamping
	licenseChecker LicenseChecker

	// Optional metrics callback for recording investigation outcomes
	metricsCallback MetricsCallback

	// Track running investigations
	runningCount int
	runningMu    sync.Mutex

	// Shutdown coordination
	shutdownCtx context.Context    // Cancelled on Shutdown() to signal all investigations
	shutdownFn  context.CancelFunc // Triggers shutdownCtx cancellation
	wg          sync.WaitGroup     // Tracks in-flight InvestigateFinding calls
}

// NewOrchestrator creates a new investigation orchestrator
func NewOrchestrator(
	chatService ChatService,
	store *Store,
	findingsStore FindingsStore,
	approvalStore ApprovalStore,
	config InvestigationConfig,
) *Orchestrator {
	ctx, cancel := context.WithCancel(context.Background())
	return &Orchestrator{
		chatService:   chatService,
		store:         store,
		findingsStore: findingsStore,
		approvalStore: approvalStore,
		guardrails:    NewGuardrails(),
		config:        config,
		shutdownCtx:   ctx,
		shutdownFn:    cancel,
	}
}

// GetStore returns the investigation store for external cleanup/maintenance.
func (o *Orchestrator) GetStore() *Store {
	return o.store
}

// CleanupInvestigationStore runs periodic maintenance on the investigation store:
// removes old/stuck sessions and enforces a size limit.
func (o *Orchestrator) CleanupInvestigationStore(maxAge time.Duration, maxSessions int) {
	o.store.Cleanup(maxAge)
	o.store.EnforceSizeLimit(maxSessions)
}

// SetConfig updates the orchestrator configuration
func (o *Orchestrator) SetConfig(config InvestigationConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.config = config
}

// SetMetricsCallback sets the metrics callback for recording investigation outcomes
func (o *Orchestrator) SetMetricsCallback(cb MetricsCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.metricsCallback = cb
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

// SetAutonomyLevelProvider sets the provider for fetching current autonomy level
// This enables re-checking autonomy level before fix execution
func (o *Orchestrator) SetAutonomyLevelProvider(provider AutonomyLevelProvider) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.autonomyProvider = provider
}

// SetFixVerifier sets the verifier used to confirm fixes resolved the issue
func (o *Orchestrator) SetFixVerifier(fv FixVerifier) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.fixVerifier = fv
}

// LicenseChecker provides license feature checking for defense-in-depth validation
type LicenseChecker interface {
	HasFeature(feature string) bool
}

// SetLicenseChecker sets the license checker for defense-in-depth autonomy clamping
func (o *Orchestrator) SetLicenseChecker(checker LicenseChecker) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.licenseChecker = checker
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

// Shutdown signals all running investigations to stop, persists state,
// and waits for them to finish up to the context deadline.
func (o *Orchestrator) Shutdown(ctx context.Context) error {
	log.Info().Msg("investigation orchestrator: starting shutdown")

	// Signal all running investigations to cancel
	o.shutdownFn()

	// Force-save investigation state before waiting
	if o.store != nil {
		if err := o.store.ForceSave(); err != nil {
			log.Error().Err(err).Msg("investigation orchestrator: failed to force-save store during shutdown")
		}
	}

	// Wait for in-flight investigations to finish (or context to expire)
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Info().Msg("investigation orchestrator: all investigations completed")
		return nil
	case <-ctx.Done():
		o.runningMu.Lock()
		remaining := o.runningCount
		o.runningMu.Unlock()
		log.Warn().Int("remaining", remaining).Msg("investigation orchestrator: shutdown timed out with running investigations")
		return fmt.Errorf("shutdown timed out with %d investigations still running", remaining)
	}
}

// InvestigateFinding starts an investigation for a finding
func (o *Orchestrator) InvestigateFinding(ctx context.Context, finding *Finding, autonomyLevel string) error {
	// Check if shutdown is in progress
	select {
	case <-o.shutdownCtx.Done():
		return fmt.Errorf("orchestrator is shutting down")
	default:
	}

	// Check if we can start a new investigation
	if !o.CanStartInvestigation() {
		return fmt.Errorf("maximum concurrent investigations reached (%d)", o.config.MaxConcurrent)
	}

	// Track this investigation for graceful shutdown
	o.wg.Add(1)

	// Increment running count
	o.runningMu.Lock()
	o.runningCount++
	o.runningMu.Unlock()

	// Decrement when done
	defer func() {
		o.runningMu.Lock()
		o.runningCount--
		o.runningMu.Unlock()
		o.wg.Done()
	}()

	// Make ctx shutdown-aware: if the orchestrator's shutdownCtx is cancelled,
	// ctx will also be cancelled, propagating to all downstream operations
	// (session creation, LLM calls, tool execution).
	ctx, ctxCancel := context.WithCancel(ctx)
	defer ctxCancel()
	stop := context.AfterFunc(o.shutdownCtx, func() { ctxCancel() })
	defer stop()

	// Only enable autonomous mode if patrol autonomy is "full".
	// For all other levels, write commands (pulse_control, pulse_file_edit)
	// must go through the normal approval gate. The investigation AI should
	// propose fixes via PROPOSED_FIX markers, not execute them directly.
	//
	// NOTE: We pass autonomous mode via the ExecuteRequest rather than
	// mutating shared chatService state, which is unsafe under concurrent
	// investigations.
	useAutonomous := autonomyLevel == "full"

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

	toolsAvailable := o.chatService.ListAvailableTools(ctx, prompt)
	if len(toolsAvailable) > 0 {
		investigation.ToolsAvailable = toolsAvailable
		o.store.Update(investigation)
	}

	log.Info().
		Str("finding_id", finding.ID).
		Str("session_id", session.ID).
		Str("investigation_id", investigation.ID).
		Str("severity", finding.Severity).
		Msg("Starting investigation")

	// Execute with timeout and turn limit
	err = o.executeWithLimits(ctx, investigation, prompt, useAutonomous)
	if err != nil {
		// Mark as failed
		o.store.Fail(investigation.ID, err.Error())
		if isTimeoutError(err) {
			o.store.SetOutcome(investigation.ID, OutcomeTimedOut)
			finding.InvestigationOutcome = string(OutcomeTimedOut)
		}
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

	// Add targeted guidance to reduce ambiguous fixes
	var extraGuidance string
	if strings.EqualFold(finding.ResourceType, "storage") {
		extraGuidance = `
## Storage Verification
- Confirm the storage is actually required by the affected workload before proposing a fix.
- Verify storage scope (node membership) and guest mounts or data paths using available tools.
- If dependency cannot be confirmed, respond with NEEDS_ATTENTION and state what evidence is missing.
`
	}

	return fmt.Sprintf(`You are investigating a finding from Pulse Patrol. Your goal is to:
1. Understand the issue using available tools
2. Determine if it can be automatically fixed
3. If fixable, propose a specific remediation command
%s
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
		extraGuidance,
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
func (o *Orchestrator) executeWithLimits(ctx context.Context, investigation *InvestigationSession, prompt string, autonomousMode bool) error {
	// Create context with timeout. The passed-in ctx is already shutdown-aware
	// (derived via context.AfterFunc in InvestigateFinding), so the timeout
	// context inherits both the caller's deadline and the shutdown signal.
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
	var toolsUsed []string
	var evidenceIDs []string
	toolsUsedSet := make(map[string]bool)
	evidenceSet := make(map[string]bool)

	// Execute the prompt with investigation-specific turn limit
	req := ExecuteRequest{
		Prompt:         prompt,
		SessionID:      investigation.SessionID,
		MaxTurns:       maxTurns,
		AutonomousMode: &autonomousMode,
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
		case "tool_start":
			var data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(event.Data, &data); err == nil {
				if data.Name != "" && !toolsUsedSet[data.Name] {
					toolsUsedSet[data.Name] = true
					toolsUsed = append(toolsUsed, data.Name)
				}
				if data.ID != "" && !evidenceSet[data.ID] {
					evidenceSet[data.ID] = true
					evidenceIDs = append(evidenceIDs, data.ID)
				}
			}
		case "tool_end":
			var data struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(event.Data, &data); err == nil {
				if data.Name != "" && !toolsUsedSet[data.Name] {
					toolsUsedSet[data.Name] = true
					toolsUsed = append(toolsUsed, data.Name)
				}
				if data.ID != "" && !evidenceSet[data.ID] {
					evidenceSet[data.ID] = true
					evidenceIDs = append(evidenceIDs, data.ID)
				}
			}
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
		investigation.ToolsUsed = toolsUsed
		investigation.EvidenceIDs = evidenceIDs
		o.store.Update(investigation)
		return err
	}
	if streamErr != nil {
		investigation.ToolsUsed = toolsUsed
		investigation.EvidenceIDs = evidenceIDs
		o.store.Update(investigation)
		return streamErr
	}

	// Store the summary
	investigation.Summary = lastContent
	investigation.ToolsUsed = toolsUsed
	investigation.EvidenceIDs = evidenceIDs
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
		// Guard: investigation produced a fix with no commands — nothing to execute
		if len(fix.Commands) == 0 {
			log.Warn().Str("finding_id", finding.ID).Msg("investigation fix has no commands, marking needs_attention")
			finding.InvestigationOutcome = string(OutcomeNeedsAttention)
			finding.LastInvestigatedAt = &now
			return nil
		}

		// Blocked commands must never be executed (even with approval). If the
		// investigation suggests a blocked command, keep the record but require
		// manual attention.
		for _, cmd := range fix.Commands {
			if safety.IsBlockedCommand(cmd) {
				log.Warn().
					Str("finding_id", finding.ID).
					Str("command", cmd).
					Msg("Investigation proposed blocked command; forcing needs_attention")
				outcome = OutcomeNeedsAttention
				fix.Rationale = fmt.Sprintf("%s\n\nBlocked by safety policy: %s", fix.Rationale, cmd)
				o.store.Complete(investigation.ID, outcome, summary, fix)
				finding.InvestigationStatus = string(StatusCompleted)
				finding.InvestigationOutcome = string(outcome)
				o.updateFinding(finding)
				return nil
			}
		}

		// Re-check current autonomy level before deciding on fix execution
		// This handles cases where user changed autonomy during investigation
		currentLevel := autonomyLevel
		if o.autonomyProvider != nil {
			currentLevel = o.autonomyProvider.GetCurrentAutonomyLevel()
			// Also check if full mode is still unlocked
			if currentLevel == "full" && !o.autonomyProvider.IsFullModeUnlocked() {
				currentLevel = "assisted" // Downgrade if full mode was locked
			}
		}

		// Defense-in-depth: clamp autonomy if no auto-fix license
		if o.licenseChecker != nil && !o.licenseChecker.HasFeature(license.FeatureAIAutoFix) {
			if currentLevel == "assisted" || currentLevel == "full" {
				currentLevel = "approval"
				log.Warn().Str("finding_id", finding.ID).Msg("auto-fix requires Pro license - clamping to approval mode")
			}
		}

		// Check if fix requires approval based on current autonomy level.
		// Any risky command in a sequence requires approval.
		requiresApproval := false
		riskRank := func(level string) int {
			switch strings.ToLower(strings.TrimSpace(level)) {
			case "low":
				return 0
			case "medium":
				return 1
			case "high":
				return 2
			case "critical":
				return 3
			default:
				return 1
			}
		}
		bestRisk := "medium"
		destructive := false
		for _, cmd := range fix.Commands {
			if o.guardrails.RequiresApproval(finding.Severity, currentLevel, cmd) {
				requiresApproval = true
			}
			if o.guardrails.IsDestructiveAction(cmd) {
				destructive = true
			}
			level := o.guardrails.ClassifyRisk(cmd)
			if riskRank(level) > riskRank(bestRisk) {
				bestRisk = level
			}
		}
		fix.RiskLevel = bestRisk
		fix.Destructive = destructive

		if requiresApproval {
			// Queue for approval
			approval := &Approval{
				ID:          uuid.New().String(),
				Type:        "investigation_fix",
				FindingID:   finding.ID,
				SessionID:   investigation.SessionID,
				Description: fix.Description,
				Command:     strings.Join(fix.Commands, "\n"),
				RiskLevel:   fix.RiskLevel,
				CreatedAt:   time.Now(),
			}

			if o.approvalStore != nil {
				if err := o.approvalStore.Create(approval); err != nil {
					log.Error().Err(err).Msg("failed to queue fix for approval")
					outcome = OutcomeNeedsAttention
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
					Int("command_count", len(fix.Commands)).
					Str("target_host", fix.TargetHost).
					Str("risk_level", fix.RiskLevel).
					Msg("Auto-executing fix (full autonomy mode)")

				var combined strings.Builder
				allOK := true
				for i, cmd := range fix.Commands {
					output, exitCode, execErr := o.commandExecutor.ExecuteCommand(ctx, cmd, fix.TargetHost)
					if combined.Len() > 0 {
						combined.WriteString("\n\n")
					}
					combined.WriteString(fmt.Sprintf("Command %d/%d: %s\n", i+1, len(fix.Commands), cmd))
					if output != "" {
						combined.WriteString(output)
					}
					if execErr != nil {
						log.Error().
							Err(execErr).
							Str("finding_id", finding.ID).
							Str("command", cmd).
							Msg("Auto-executed fix command failed")
						outcome = OutcomeFixFailed
						fix.Rationale = fmt.Sprintf("%s\n\nAuto-execution failed: %v\n\n%s", fix.Rationale, execErr, combined.String())
						allOK = false
						break
					}
					if exitCode != 0 {
						log.Warn().
							Str("finding_id", finding.ID).
							Str("command", cmd).
							Int("exit_code", exitCode).
							Str("output", output).
							Msg("Auto-executed fix command returned non-zero exit code")
						outcome = OutcomeFixFailed
						fix.Rationale = fmt.Sprintf("%s\n\nAuto-execution returned exit code %d\n\n%s", fix.Rationale, exitCode, combined.String())
						allOK = false
						break
					}
				}

				if allOK {
					log.Info().
						Str("finding_id", finding.ID).
						Int("command_count", len(fix.Commands)).
						Msg("Auto-executed fix successfully")
					outcome = OutcomeFixExecuted
					fix.Rationale = fmt.Sprintf("%s\n\nAuto-executed successfully:\n%s", fix.Rationale, combined.String())

					// Verify the fix actually resolved the issue
					o.mu.RLock()
					verifier := o.fixVerifier
					verificationDelay := o.config.VerificationDelay
					o.mu.RUnlock()

					if verifier != nil {
						if verificationDelay > 0 {
							time.Sleep(verificationDelay)
						}
						verified, verifyErr := verifier.VerifyFixResolved(ctx, finding)
						o.mu.RLock()
						vmc := o.metricsCallback
						o.mu.RUnlock()
						if verifyErr != nil {
							if errors.Is(verifyErr, ErrVerificationUnknown) {
								log.Warn().Err(verifyErr).Str("finding_id", finding.ID).Msg("fix verification inconclusive")
								outcome = OutcomeFixVerificationUnknown
								fix.Rationale += fmt.Sprintf("\n\nVerification inconclusive: %v", verifyErr)
								if vmc != nil {
									vmc.RecordFixVerification("unknown")
								}
							} else {
								log.Error().Err(verifyErr).Str("finding_id", finding.ID).Msg("fix verification failed with error")
								outcome = OutcomeFixVerificationFailed
								fix.Rationale += fmt.Sprintf("\n\nVerification error: %v", verifyErr)
								if vmc != nil {
									vmc.RecordFixVerification("error")
								}
							}
						} else if !verified {
							log.Warn().Str("finding_id", finding.ID).Msg("fix executed but issue persists")
							outcome = OutcomeFixVerificationFailed
							fix.Rationale += "\n\nVerification: Issue persists after fix execution."
							if vmc != nil {
								vmc.RecordFixVerification("failed")
							}
						} else {
							log.Info().Str("finding_id", finding.ID).Msg("fix verified - issue resolved")
							outcome = OutcomeFixVerified
							fix.Rationale += "\n\nVerification: Issue confirmed resolved."
							if vmc != nil {
								vmc.RecordFixVerification("verified")
							}
						}
					}
				}
			} else {
				// No command executor available, fall back to queueing for approval
				log.Warn().
					Str("finding_id", finding.ID).
					Int("command_count", len(fix.Commands)).
					Msg("Full autonomy enabled but no command executor available, queueing for approval")
				outcome = OutcomeFixQueued
			}
		}

		o.store.Complete(investigation.ID, outcome, summary, fix)
	} else {
		o.store.Complete(investigation.ID, outcome, summary, nil)
	}

	// Record metrics
	o.mu.RLock()
	mc := o.metricsCallback
	o.mu.RUnlock()
	if mc != nil {
		mc.RecordInvestigationOutcome(string(outcome))
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

// maxCommandLength is the maximum allowed length for a proposed fix command.
// Commands longer than this are rejected as likely malformed.
const maxCommandLength = 2000

// maxCommandsPerFix is the maximum number of commands the investigation may
// propose as a single fix. This prevents accidentally interpreting long
// explanations as commands.
const maxCommandsPerFix = 10

// parseInvestigationSummary extracts fix proposals from the investigation output.
// Marker detection is case-insensitive to handle varying LLM output styles.
func (o *Orchestrator) parseInvestigationSummary(summary string) (*Fix, Outcome) {
	upper := strings.ToUpper(summary)

	// Look for PROPOSED_FIX marker (case-insensitive)
	if idx := strings.Index(upper, "PROPOSED_FIX:"); idx >= 0 {
		// Extract the command from the original string (preserving case)
		remaining := summary[idx+len("PROPOSED_FIX:"):]

		// If the remaining text starts with a code fence, extract until the closing fence
		// so multiline fenced commands are captured correctly.
		var commandText string
		trimmedRemaining := strings.TrimSpace(remaining)
		if strings.HasPrefix(trimmedRemaining, "```") {
			// Find the closing fence after the opening one
			afterOpen := trimmedRemaining[3:]
			closeIdx := strings.Index(afterOpen, "```")
			if closeIdx >= 0 {
				fenced := trimmedRemaining[:3+closeIdx+3]
				commandText = stripMarkdownCodeFences(fenced)
			} else {
				// No closing fence — take the first line only
				endIdx := strings.Index(remaining, "\n")
				if endIdx < 0 {
					endIdx = len(remaining)
				}
				commandText = trim(remaining[:endIdx])
			}
		} else {
			// Single-line command — take to end of line
			endIdx := strings.Index(remaining, "\n")
			if endIdx < 0 {
				endIdx = len(remaining)
			}
			commandText = trim(remaining[:endIdx])
			commandText = stripMarkdownCodeFences(commandText)
		}

		commandText = strings.TrimSpace(commandText)

		// Split a fenced multi-line "script" into individual commands. This allows
		// investigations to propose a short sequence like:
		// 1) check something
		// 2) apply fix
		// 3) restart service
		//
		// Each command is still safety-checked and guardrailed independently later.
		commandText = strings.ReplaceAll(commandText, "\r\n", "\n")
		lines := strings.Split(commandText, "\n")
		commands := make([]string, 0, len(lines))
		for _, line := range lines {
			l := strings.TrimSpace(line)
			if l == "" {
				continue
			}
			// Accept common "shell prompt" prefixes in model output.
			if strings.HasPrefix(l, "$") {
				l = strings.TrimSpace(strings.TrimPrefix(l, "$"))
			}
			if l == "" {
				continue
			}
			commands = append(commands, l)
			if len(commands) >= maxCommandsPerFix {
				break
			}
		}

		log.Debug().
			Int("marker_pos", idx).
			Str("raw_command", commandText).
			Int("command_count", len(commands)).
			Msg("Found PROPOSED_FIX marker")

		// Look for TARGET_HOST (case-insensitive)
		targetHost := "local"
		if hostIdx := strings.Index(upper, "TARGET_HOST:"); hostIdx >= 0 {
			hostRemaining := summary[hostIdx+len("TARGET_HOST:"):]
			hostEndIdx := strings.Index(hostRemaining, "\n")
			if hostEndIdx < 0 {
				hostEndIdx = len(hostRemaining)
			}
			targetHost = trim(hostRemaining[:hostEndIdx])
		}

		// Validate commands
		if len(commands) == 0 {
			log.Warn().Msg("PROPOSED_FIX marker found but command is empty, falling back to needs_attention")
			return nil, OutcomeNeedsAttention
		}
		for _, cmd := range commands {
			if len(cmd) > maxCommandLength {
				log.Warn().
					Int("command_length", len(cmd)).
					Int("max_length", maxCommandLength).
					Msg("PROPOSED_FIX command exceeds max length, falling back to needs_attention")
				return nil, OutcomeNeedsAttention
			}
		}

		return &Fix{
			ID:          uuid.New().String(),
			Description: "Proposed fix from investigation",
			Commands:    commands,
			TargetHost:  targetHost,
			Rationale:   summary,
		}, OutcomeFixQueued
	}

	// Look for CANNOT_FIX marker (case-insensitive)
	if strings.Index(upper, "CANNOT_FIX:") >= 0 {
		log.Debug().Msg("found CANNOT_FIX marker")
		return nil, OutcomeCannotFix
	}

	// Look for NEEDS_ATTENTION marker (case-insensitive)
	if strings.Index(upper, "NEEDS_ATTENTION:") >= 0 {
		log.Debug().Msg("found NEEDS_ATTENTION marker")
		return nil, OutcomeNeedsAttention
	}

	// Default to needs attention if we couldn't parse the output
	return nil, OutcomeNeedsAttention
}

// stripMarkdownCodeFences removes surrounding markdown code fences from a string.
// Handles formats like: ```bash\ncommand\n```, ```command```, `command`
func stripMarkdownCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// Handle triple backtick fences (possibly with language tag)
	if strings.HasPrefix(s, "```") {
		s = s[3:]
		// Strip optional language tag on the same line
		if idx := strings.Index(s, "\n"); idx >= 0 {
			// Check if everything before newline looks like a language tag (no spaces typically)
			tag := strings.TrimSpace(s[:idx])
			if tag == "" || (!strings.Contains(tag, " ") && len(tag) < 20) {
				s = s[idx+1:]
			}
		}
		// Strip closing fence
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		return strings.TrimSpace(s)
	}

	// Handle single backtick wrapping
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return strings.TrimSpace(s[1 : len(s)-1])
	}

	return s
}

// isTimeoutError returns true if the error is caused by a context deadline or timeout.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) ||
		strings.Contains(err.Error(), "context deadline exceeded")
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

	// Prevent duplicate investigations
	if finding.InvestigationStatus == "running" {
		return fmt.Errorf("investigation already running for finding %s", findingID)
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
