// Package remediation provides AI-guided fix plans with safe execution capabilities.
package remediation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/safety"
	"github.com/rs/zerolog/log"
)

// PlanCategory categorizes remediation plans by safety level
type PlanCategory string

const (
	// CategoryInformational provides advice but no executable action
	CategoryInformational PlanCategory = "informational"
	// CategoryGuided provides commands for user to copy and run
	CategoryGuided PlanCategory = "guided"
	// CategoryOneClick can be executed with single approval
	CategoryOneClick PlanCategory = "one_click"
	// CategoryAutonomous can be auto-executed based on pre-approved rules
	CategoryAutonomous PlanCategory = "autonomous"
)

// RiskLevel indicates the risk of a remediation action
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // Safe, easily reversible
	RiskMedium   RiskLevel = "medium"   // Some risk, should be reviewed
	RiskHigh     RiskLevel = "high"     // Significant risk, careful review needed
	RiskCritical RiskLevel = "critical" // Could cause data loss or outage
)

// ExecutionStatus tracks the status of a remediation execution
type ExecutionStatus string

const (
	StatusPending    ExecutionStatus = "pending"
	StatusApproved   ExecutionStatus = "approved"
	StatusRunning    ExecutionStatus = "running"
	StatusCompleted  ExecutionStatus = "completed"
	StatusFailed     ExecutionStatus = "failed"
	StatusRolledBack ExecutionStatus = "rolled_back"
)

// RemediationStep represents a single step in a remediation plan
type RemediationStep struct {
	Order       int                    `json:"order"`
	Description string                 `json:"description"`
	Command     string                 `json:"command,omitempty"`
	Target      string                 `json:"target,omitempty"` // Resource or host to run on
	Rollback    string                 `json:"rollback,omitempty"`
	WaitAfter   time.Duration          `json:"wait_after,omitempty"`
	Condition   string                 `json:"condition,omitempty"` // Condition to check before running
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// RemediationPlan represents a complete plan to fix an issue
type RemediationPlan struct {
	ID            string            `json:"id"`
	FindingID     string            `json:"finding_id"`
	ResourceID    string            `json:"resource_id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Category      PlanCategory      `json:"category"`
	RiskLevel     RiskLevel         `json:"risk_level"`
	Steps         []RemediationStep `json:"steps"`
	Rationale     string            `json:"rationale,omitempty"`
	Prerequisites []string          `json:"prerequisites,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
}

// RemediationExecution tracks the execution of a remediation plan
type RemediationExecution struct {
	ID               string          `json:"id"`
	PlanID           string          `json:"plan_id"`
	Status           ExecutionStatus `json:"status"`
	ApprovedBy       string          `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time      `json:"approved_at,omitempty"`
	StartedAt        *time.Time      `json:"started_at,omitempty"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	CurrentStep      int             `json:"current_step"`
	StepResults      []StepResult    `json:"step_results,omitempty"`
	Error            string          `json:"error,omitempty"`
	RollbackError    string          `json:"rollback_error,omitempty"`
	Verified         *bool           `json:"verified,omitempty"`
	VerificationNote string          `json:"verification_note,omitempty"`
}

// StepResult records the result of executing a step
type StepResult struct {
	Step     int           `json:"step"`
	Success  bool          `json:"success"`
	Output   string        `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration_ms"`
	RunAt    time.Time     `json:"run_at"`
}

// ApprovalRule defines pre-approved actions for autonomous execution
type ApprovalRule struct {
	ID           string       `json:"id"`
	Description  string       `json:"description"`
	Category     PlanCategory `json:"category"`
	ResourceType string       `json:"resource_type,omitempty"` // Empty = any
	ActionType   string       `json:"action_type"`             // e.g., "restart_service", "clear_cache"
	MaxRiskLevel RiskLevel    `json:"max_risk_level"`
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	CreatedBy    string       `json:"created_by,omitempty"`
}

// CommandExecutor executes commands on target systems
type CommandExecutor interface {
	Execute(ctx context.Context, target, command string) (output string, err error)
}

// EngineConfig configures the remediation engine
type EngineConfig struct {
	DataDir          string
	MaxExecutions    int           // Max executions to keep in history
	PlanExpiry       time.Duration // How long plans are valid
	ExecutionTimeout time.Duration // Max time for a single step
}

// DefaultEngineConfig returns sensible defaults
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		MaxExecutions:    100,
		PlanExpiry:       24 * time.Hour,
		ExecutionTimeout: 5 * time.Minute,
	}
}

// Engine manages remediation plans and executions
type Engine struct {
	mu sync.RWMutex

	config   EngineConfig
	executor CommandExecutor

	// Plans (keyed by ID)
	plans map[string]*RemediationPlan

	// Executions (keyed by ID)
	executions map[string]*RemediationExecution

	// Approval rules
	approvalRules map[string]*ApprovalRule

	// Persistence
	dataDir string
}

// NewEngine creates a new remediation engine
func NewEngine(cfg EngineConfig) *Engine {
	if cfg.MaxExecutions <= 0 {
		cfg.MaxExecutions = 100
	}
	if cfg.PlanExpiry <= 0 {
		cfg.PlanExpiry = 24 * time.Hour
	}
	if cfg.ExecutionTimeout <= 0 {
		cfg.ExecutionTimeout = 5 * time.Minute
	}

	engine := &Engine{
		config:        cfg,
		plans:         make(map[string]*RemediationPlan),
		executions:    make(map[string]*RemediationExecution),
		approvalRules: make(map[string]*ApprovalRule),
		dataDir:       cfg.DataDir,
	}

	// Load from disk
	if cfg.DataDir != "" {
		if err := engine.loadFromDisk(); err != nil {
			log.Warn().Err(err).Msg("Failed to load remediation data from disk")
		}
	}

	return engine
}

// SetCommandExecutor sets the command executor
func (e *Engine) SetCommandExecutor(executor CommandExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executor = executor
}

// CreatePlan creates and stores a new remediation plan
func (e *Engine) CreatePlan(plan *RemediationPlan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Validate plan
	if err := e.validatePlan(plan); err != nil {
		return fmt.Errorf("invalid plan: %w", err)
	}

	// Generate ID if not set
	if plan.ID == "" {
		plan.ID = generatePlanID()
	}

	// Set timestamps
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}
	if plan.ExpiresAt == nil {
		expiry := plan.CreatedAt.Add(e.config.PlanExpiry)
		plan.ExpiresAt = &expiry
	}

	// Assess risk level if not set
	if plan.RiskLevel == "" {
		plan.RiskLevel = e.assessRiskLevel(plan)
	}

	// Determine category if not set
	if plan.Category == "" {
		plan.Category = e.categorize(plan)
	}

	e.plans[plan.ID] = plan

	log.Info().
		Str("plan_id", plan.ID).
		Str("finding_id", plan.FindingID).
		Str("category", string(plan.Category)).
		Str("risk", string(plan.RiskLevel)).
		Msg("Created remediation plan")

	go e.saveIfDirty()

	return nil
}

// validatePlan checks if a plan is valid
func (e *Engine) validatePlan(plan *RemediationPlan) error {
	if plan.Title == "" {
		return fmt.Errorf("plan title is required")
	}
	if len(plan.Steps) == 0 {
		return fmt.Errorf("plan must have at least one step")
	}

	// Check for blocked commands
	for _, step := range plan.Steps {
		if e.isBlockedCommand(step.Command) {
			return fmt.Errorf("plan contains blocked command: %s", step.Command)
		}
	}

	return nil
}

// isBlockedCommand checks if a command matches any blocked pattern.
// Delegates to the shared safety package for the canonical blocked command list.
func (e *Engine) isBlockedCommand(command string) bool {
	return safety.IsBlockedCommand(command)
}

// assessRiskLevel determines the risk level of a plan
func (e *Engine) assessRiskLevel(plan *RemediationPlan) RiskLevel {
	highRiskKeywords := []string{"delete", "remove", "destroy", "format", "wipe", "reset"}
	mediumRiskKeywords := []string{"restart", "stop", "kill", "force", "override"}

	for _, step := range plan.Steps {
		cmd := step.Command

		// Check for high risk
		for _, keyword := range highRiskKeywords {
			if containsIgnoreCase(cmd, keyword) {
				return RiskHigh
			}
		}

		// Check for medium risk
		for _, keyword := range mediumRiskKeywords {
			if containsIgnoreCase(cmd, keyword) {
				if plan.RiskLevel != RiskHigh {
					plan.RiskLevel = RiskMedium
				}
			}
		}
	}

	if plan.RiskLevel == "" {
		return RiskLow
	}
	return plan.RiskLevel
}

// categorize determines the category of a plan
func (e *Engine) categorize(plan *RemediationPlan) PlanCategory {
	// If no commands, it's informational
	hasCommands := false
	for _, step := range plan.Steps {
		if step.Command != "" {
			hasCommands = true
			break
		}
	}
	if !hasCommands {
		return CategoryInformational
	}

	// High risk is never autonomous
	if plan.RiskLevel == RiskHigh || plan.RiskLevel == RiskCritical {
		return CategoryGuided
	}

	// Low risk simple commands can be one-click
	if plan.RiskLevel == RiskLow && len(plan.Steps) <= 3 {
		return CategoryOneClick
	}

	return CategoryGuided
}

// GetPlan returns a plan by ID
func (e *Engine) GetPlan(planID string) *RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if plan, ok := e.plans[planID]; ok {
		// Return a copy
		copy := *plan
		return &copy
	}
	return nil
}

// GetPlanForFinding returns the plan for a finding
func (e *Engine) GetPlanForFinding(findingID string) *RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, plan := range e.plans {
		if plan.FindingID == findingID {
			// Check if expired
			if plan.ExpiresAt != nil && time.Now().After(*plan.ExpiresAt) {
				continue
			}
			copy := *plan
			return &copy
		}
	}
	return nil
}

// ListPlans returns remediation plans, ordered by creation time (newest first).
func (e *Engine) ListPlans(limit int) []*RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	plans := make([]*RemediationPlan, 0, len(e.plans))
	now := time.Now()
	for _, plan := range e.plans {
		if plan == nil {
			continue
		}
		// Skip expired plans
		if plan.ExpiresAt != nil && now.After(*plan.ExpiresAt) {
			continue
		}
		copy := *plan
		plans = append(plans, &copy)
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})

	if len(plans) > limit {
		plans = plans[:limit]
	}

	return plans
}

// GetLatestExecutionForPlan returns the most recent execution for a plan.
func (e *Engine) GetLatestExecutionForPlan(planID string) *RemediationExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var latest *RemediationExecution
	var latestAt time.Time

	for _, execution := range e.executions {
		if execution == nil || execution.PlanID != planID {
			continue
		}
		ts := time.Time{}
		if execution.CompletedAt != nil {
			ts = *execution.CompletedAt
		} else if execution.StartedAt != nil {
			ts = *execution.StartedAt
		} else if execution.ApprovedAt != nil {
			ts = *execution.ApprovedAt
		}
		if ts.After(latestAt) {
			latestAt = ts
			latest = execution
		}
	}

	if latest == nil {
		return nil
	}
	copy := *latest
	return &copy
}

// ApprovePlan approves a plan for execution
func (e *Engine) ApprovePlan(planID, approvedBy string) (*RemediationExecution, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, ok := e.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	// Check if expired
	if plan.ExpiresAt != nil && time.Now().After(*plan.ExpiresAt) {
		return nil, fmt.Errorf("plan has expired")
	}

	// Create execution
	now := time.Now()
	execution := &RemediationExecution{
		ID:         generateExecutionID(),
		PlanID:     planID,
		Status:     StatusApproved,
		ApprovedBy: approvedBy,
		ApprovedAt: &now,
	}

	e.executions[execution.ID] = execution

	log.Info().
		Str("execution_id", execution.ID).
		Str("plan_id", planID).
		Str("approved_by", approvedBy).
		Msg("Remediation plan approved")

	go e.saveIfDirty()

	return execution, nil
}

// Execute runs an approved execution
func (e *Engine) Execute(ctx context.Context, executionID string) error {
	e.mu.Lock()
	execution, ok := e.executions[executionID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("execution not found: %s", executionID)
	}

	if execution.Status != StatusApproved {
		e.mu.Unlock()
		return fmt.Errorf("execution is not approved: %s", execution.Status)
	}

	plan, ok := e.plans[execution.PlanID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("plan not found: %s", execution.PlanID)
	}

	executor := e.executor
	e.mu.Unlock()

	if executor == nil {
		return fmt.Errorf("no command executor configured")
	}

	// Start execution
	e.mu.Lock()
	now := time.Now()
	execution.Status = StatusRunning
	execution.StartedAt = &now
	e.mu.Unlock()

	// Execute steps
	var lastError error
	for i, step := range plan.Steps {
		if step.Command == "" {
			continue // Skip informational steps
		}

		e.mu.Lock()
		execution.CurrentStep = i
		e.mu.Unlock()

		// Create timeout context for this step
		stepCtx, cancel := context.WithTimeout(ctx, e.config.ExecutionTimeout)

		start := time.Now()
		output, err := executor.Execute(stepCtx, step.Target, step.Command)
		duration := time.Since(start)

		cancel()

		result := StepResult{
			Step:     i,
			Success:  err == nil,
			Output:   truncateString(output, 10000),
			Duration: duration,
			RunAt:    start,
		}
		if err != nil {
			result.Error = err.Error()
			lastError = err
		}

		e.mu.Lock()
		execution.StepResults = append(execution.StepResults, result)
		e.mu.Unlock()

		// Stop on error
		if err != nil {
			log.Warn().
				Str("execution_id", executionID).
				Int("step", i).
				Err(err).
				Msg("Remediation step failed")
			break
		}

		// Wait if specified
		if step.WaitAfter > 0 {
			select {
			case <-ctx.Done():
				lastError = ctx.Err()
			case <-time.After(step.WaitAfter):
			}
		}

		log.Debug().
			Str("execution_id", executionID).
			Int("step", i).
			Dur("duration", duration).
			Msg("Completed remediation step")
	}

	// Mark as completed or failed
	e.mu.Lock()
	completedAt := time.Now()
	execution.CompletedAt = &completedAt
	if lastError != nil {
		execution.Status = StatusFailed
		execution.Error = lastError.Error()
	} else {
		execution.Status = StatusCompleted
	}
	e.mu.Unlock()

	log.Info().
		Str("execution_id", executionID).
		Str("status", string(execution.Status)).
		Msg("Remediation execution finished")

	go e.saveIfDirty()

	return lastError
}

// Rollback attempts to rollback a failed execution
func (e *Engine) Rollback(ctx context.Context, executionID string) error {
	e.mu.Lock()
	execution, ok := e.executions[executionID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("execution not found: %s", executionID)
	}

	plan, ok := e.plans[execution.PlanID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("plan not found: %s", execution.PlanID)
	}

	executor := e.executor
	e.mu.Unlock()

	if executor == nil {
		return fmt.Errorf("no command executor configured")
	}

	// Execute rollback steps in reverse order
	var rollbackErrors []string
	for i := len(execution.StepResults) - 1; i >= 0; i-- {
		result := execution.StepResults[i]
		if !result.Success {
			continue // Skip steps that didn't run
		}

		step := plan.Steps[result.Step]
		if step.Rollback == "" {
			continue // No rollback command
		}

		_, err := executor.Execute(ctx, step.Target, step.Rollback)
		if err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Sprintf("step %d: %s", result.Step, err.Error()))
			log.Warn().
				Str("execution_id", executionID).
				Int("step", result.Step).
				Err(err).
				Msg("Rollback step failed")
		}
	}

	e.mu.Lock()
	if len(rollbackErrors) > 0 {
		execution.RollbackError = fmt.Sprintf("rollback errors: %v", rollbackErrors)
	} else {
		execution.Status = StatusRolledBack
	}
	e.mu.Unlock()

	go e.saveIfDirty()

	if len(rollbackErrors) > 0 {
		return fmt.Errorf("rollback had errors")
	}
	return nil
}

// GetExecution returns an execution by ID
func (e *Engine) GetExecution(executionID string) *RemediationExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if exec, ok := e.executions[executionID]; ok {
		copy := *exec
		return &copy
	}
	return nil
}

// SetExecutionVerification updates the verification status of an execution
func (e *Engine) SetExecutionVerification(executionID string, verified bool, note string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if exec, ok := e.executions[executionID]; ok {
		exec.Verified = &verified
		exec.VerificationNote = note
	}
}

// ListExecutions returns recent executions
func (e *Engine) ListExecutions(limit int) []*RemediationExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*RemediationExecution
	for _, exec := range e.executions {
		copy := *exec
		result = append(result, &copy)
	}

	// Sort by time (most recent first) - simple bubble sort for small lists
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			iTime := result[i].ApprovedAt
			jTime := result[j].ApprovedAt
			if iTime != nil && jTime != nil && jTime.After(*iTime) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// AddApprovalRule adds a pre-approval rule
func (e *Engine) AddApprovalRule(rule *ApprovalRule) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}

	e.approvalRules[rule.ID] = rule
	go e.saveIfDirty()
}

// IsAutoApproved checks if a plan can be auto-executed
func (e *Engine) IsAutoApproved(plan *RemediationPlan) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, rule := range e.approvalRules {
		if !rule.Enabled {
			continue
		}

		// Check category
		if rule.Category != "" && rule.Category != plan.Category {
			continue
		}

		// Check risk level
		if riskValue(plan.RiskLevel) > riskValue(rule.MaxRiskLevel) {
			continue
		}

		// If we get here, the rule matches
		return true
	}

	return false
}

// FormatPlanForContext formats a plan for AI context
func (e *Engine) FormatPlanForContext(plan *RemediationPlan) string {
	if plan == nil {
		return ""
	}

	result := fmt.Sprintf("\n## Remediation Plan: %s\n", plan.Title)
	result += fmt.Sprintf("Category: %s | Risk: %s\n", plan.Category, plan.RiskLevel)
	result += fmt.Sprintf("Description: %s\n\n", plan.Description)

	if len(plan.Prerequisites) > 0 {
		result += "Prerequisites:\n"
		for _, prereq := range plan.Prerequisites {
			result += "- " + prereq + "\n"
		}
		result += "\n"
	}

	result += "Steps:\n"
	for i, step := range plan.Steps {
		result += fmt.Sprintf("%d. %s\n", i+1, step.Description)
		if step.Command != "" {
			result += fmt.Sprintf("   Command: %s\n", step.Command)
		}
		if step.Rollback != "" {
			result += fmt.Sprintf("   Rollback: %s\n", step.Rollback)
		}
	}

	if len(plan.Warnings) > 0 {
		result += "\nWarnings:\n"
		for _, warning := range plan.Warnings {
			result += "- " + warning + "\n"
		}
	}

	return result
}

// saveIfDirty saves to disk if there are changes
func (e *Engine) saveIfDirty() {
	if err := e.saveToDisk(); err != nil {
		log.Warn().Err(err).Msg("Failed to save remediation data")
	}
}

// saveToDisk persists data
func (e *Engine) saveToDisk() error {
	if e.dataDir == "" {
		return nil
	}

	e.mu.RLock()
	data := struct {
		Plans         map[string]*RemediationPlan      `json:"plans"`
		Executions    map[string]*RemediationExecution `json:"executions"`
		ApprovalRules map[string]*ApprovalRule         `json:"approval_rules"`
	}{
		Plans:         e.plans,
		Executions:    e.executions,
		ApprovalRules: e.approvalRules,
	}
	e.mu.RUnlock()

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(e.dataDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(e.dataDir, "remediation.json")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0600); err != nil {
		return err
	}

	return os.Rename(tmpPath, path)
}

// loadFromDisk loads data
func (e *Engine) loadFromDisk() error {
	if e.dataDir == "" {
		return nil
	}

	path := filepath.Join(e.dataDir, "remediation.json")
	jsonData, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var data struct {
		Plans         map[string]*RemediationPlan      `json:"plans"`
		Executions    map[string]*RemediationExecution `json:"executions"`
		ApprovalRules map[string]*ApprovalRule         `json:"approval_rules"`
	}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return err
	}

	if data.Plans != nil {
		e.plans = data.Plans
	}
	if data.Executions != nil {
		e.executions = data.Executions
	}
	if data.ApprovalRules != nil {
		e.approvalRules = data.ApprovalRules
	}

	return nil
}

// Helper functions

var planCounter, executionCounter, ruleCounter int64

func generatePlanID() string {
	planCounter++
	return fmt.Sprintf("plan-%s-%d", time.Now().Format("20060102150405"), planCounter%1000)
}

func generateExecutionID() string {
	executionCounter++
	return fmt.Sprintf("exec-%s-%d", time.Now().Format("20060102150405"), executionCounter%1000)
}

func generateRuleID() string {
	ruleCounter++
	return fmt.Sprintf("rule-%s-%d", time.Now().Format("20060102150405"), ruleCounter%1000)
}

func containsIgnoreCase(s, substr string) bool {
	sLower := toLower(s)
	subLower := toLower(substr)
	return contains(sLower, subLower)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c = c + 32
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func riskValue(r RiskLevel) int {
	switch r {
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	case RiskCritical:
		return 4
	default:
		return 0
	}
}
