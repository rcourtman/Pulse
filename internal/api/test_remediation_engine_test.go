package api

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// testRemediationEngine is an in-memory implementation of aicontracts.RemediationEngine
// used in tests that need a functioning engine without importing internal/ai/remediation.
type testRemediationEngine struct {
	mu            sync.RWMutex
	plans         map[string]*aicontracts.RemediationPlan
	executions    map[string]*aicontracts.RemediationExecution
	approvalRules map[string]*aicontracts.ApprovalRule
	executor      aicontracts.CommandExecutor
	planCounter   int
	execCounter   int
}

func newTestRemediationEngine() *testRemediationEngine {
	return &testRemediationEngine{
		plans:         make(map[string]*aicontracts.RemediationPlan),
		executions:    make(map[string]*aicontracts.RemediationExecution),
		approvalRules: make(map[string]*aicontracts.ApprovalRule),
	}
}

func (e *testRemediationEngine) CreatePlan(plan *aicontracts.RemediationPlan) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if plan.Title == "" {
		return fmt.Errorf("invalid plan: plan title is required")
	}
	if len(plan.Steps) == 0 {
		return fmt.Errorf("invalid plan: plan must have at least one step")
	}

	if plan.ID == "" {
		e.planCounter++
		plan.ID = fmt.Sprintf("plan-%d", e.planCounter)
	}
	if plan.CreatedAt.IsZero() {
		plan.CreatedAt = time.Now()
	}
	if plan.ExpiresAt == nil {
		expiry := plan.CreatedAt.Add(24 * time.Hour)
		plan.ExpiresAt = &expiry
	}
	if plan.RiskLevel == "" {
		plan.RiskLevel = aicontracts.RiskLow
	}
	if plan.Category == "" {
		plan.Category = aicontracts.CategoryGuided
	}

	e.plans[plan.ID] = plan
	return nil
}

func (e *testRemediationEngine) GetPlan(planID string) *aicontracts.RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if p, ok := e.plans[planID]; ok {
		cp := *p
		return &cp
	}
	return nil
}

func (e *testRemediationEngine) GetPlanForFinding(findingID string) *aicontracts.RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var best *aicontracts.RemediationPlan
	now := time.Now()
	for _, p := range e.plans {
		if p == nil || p.FindingID != findingID {
			continue
		}
		if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
			continue
		}
		if best == nil || p.CreatedAt.After(best.CreatedAt) {
			best = p
		}
	}
	if best != nil {
		cp := *best
		return &cp
	}
	return nil
}

func (e *testRemediationEngine) ListPlans(limit int) []*aicontracts.RemediationPlan {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	plans := make([]*aicontracts.RemediationPlan, 0, len(e.plans))
	now := time.Now()
	for _, p := range e.plans {
		if p == nil {
			continue
		}
		if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
			continue
		}
		cp := *p
		plans = append(plans, &cp)
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].CreatedAt.After(plans[j].CreatedAt)
	})
	if len(plans) > limit {
		plans = plans[:limit]
	}
	return plans
}

func (e *testRemediationEngine) GetLatestExecutionForPlan(planID string) *aicontracts.RemediationExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var latest *aicontracts.RemediationExecution
	var latestAt time.Time
	for _, ex := range e.executions {
		if ex == nil || ex.PlanID != planID {
			continue
		}
		ts := time.Time{}
		if ex.CompletedAt != nil {
			ts = *ex.CompletedAt
		} else if ex.StartedAt != nil {
			ts = *ex.StartedAt
		} else if ex.ApprovedAt != nil {
			ts = *ex.ApprovedAt
		}
		if ts.After(latestAt) {
			latestAt = ts
			latest = ex
		}
	}
	if latest == nil {
		return nil
	}
	cp := *latest
	return &cp
}

func (e *testRemediationEngine) ApprovePlan(planID, approvedBy string) (*aicontracts.RemediationExecution, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	plan, ok := e.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}
	if plan.ExpiresAt != nil && time.Now().After(*plan.ExpiresAt) {
		return nil, fmt.Errorf("plan %s has expired", planID)
	}

	e.execCounter++
	now := time.Now()
	ex := &aicontracts.RemediationExecution{
		ID:         fmt.Sprintf("exec-%d", e.execCounter),
		PlanID:     planID,
		Status:     aicontracts.ExecStatusApproved,
		ApprovedBy: approvedBy,
		ApprovedAt: &now,
	}
	e.executions[ex.ID] = ex
	return ex, nil
}

func (e *testRemediationEngine) Execute(ctx context.Context, executionID string) error {
	e.mu.Lock()
	ex, ok := e.executions[executionID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("execution not found: %s", executionID)
	}
	if ex.Status != aicontracts.ExecStatusApproved {
		e.mu.Unlock()
		return fmt.Errorf("execution is not approved: %s", ex.Status)
	}
	plan, ok := e.plans[ex.PlanID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("plan not found: %s", ex.PlanID)
	}
	executor := e.executor
	if executor == nil {
		e.mu.Unlock()
		return fmt.Errorf("no command executor configured")
	}
	// Atomically transition to running under the same lock to prevent double-execute.
	now := time.Now()
	ex.Status = aicontracts.ExecStatusRunning
	ex.StartedAt = &now
	e.mu.Unlock()

	var lastError error
	for i, step := range plan.Steps {
		if step.Command == "" {
			continue
		}
		e.mu.Lock()
		ex.CurrentStep = i
		e.mu.Unlock()

		start := time.Now()
		output, err := executor.Execute(ctx, step.Target, step.Command)
		duration := time.Since(start)

		result := aicontracts.StepResult{
			Step:     i,
			Success:  err == nil,
			Output:   output,
			Duration: duration,
			RunAt:    start,
		}
		if err != nil {
			result.Error = err.Error()
			lastError = err
		}

		e.mu.Lock()
		ex.StepResults = append(ex.StepResults, result)
		e.mu.Unlock()

		if err != nil {
			break
		}
	}

	e.mu.Lock()
	completedAt := time.Now()
	ex.CompletedAt = &completedAt
	if lastError != nil {
		ex.Status = aicontracts.ExecStatusFailed
		ex.Error = lastError.Error()
	} else {
		ex.Status = aicontracts.ExecStatusCompleted
	}
	e.mu.Unlock()

	return nil
}

func (e *testRemediationEngine) Rollback(ctx context.Context, executionID string) error {
	e.mu.Lock()
	ex, ok := e.executions[executionID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("execution not found: %s", executionID)
	}
	if ex.Status != aicontracts.ExecStatusCompleted && ex.Status != aicontracts.ExecStatusFailed {
		e.mu.Unlock()
		return fmt.Errorf("execution cannot be rolled back in status: %s", ex.Status)
	}
	plan, ok := e.plans[ex.PlanID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("plan not found: %s", ex.PlanID)
	}
	executor := e.executor
	e.mu.Unlock()

	if executor == nil {
		return fmt.Errorf("no command executor configured")
	}

	for i := len(plan.Steps) - 1; i >= 0; i-- {
		step := plan.Steps[i]
		if step.Rollback == "" {
			continue
		}
		_, err := executor.Execute(ctx, step.Target, step.Rollback)
		if err != nil {
			e.mu.Lock()
			ex.Status = aicontracts.ExecStatusFailed
			ex.RollbackError = err.Error()
			e.mu.Unlock()
			return fmt.Errorf("rollback step %d failed: %w", i, err)
		}
	}

	e.mu.Lock()
	ex.Status = aicontracts.ExecStatusRolledBack
	e.mu.Unlock()
	return nil
}

func (e *testRemediationEngine) GetExecution(executionID string) *aicontracts.RemediationExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if ex, ok := e.executions[executionID]; ok {
		cp := *ex
		return &cp
	}
	return nil
}

func (e *testRemediationEngine) SetExecutionVerification(executionID string, verified bool, note string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ex, ok := e.executions[executionID]; ok {
		ex.Verified = &verified
		ex.VerificationNote = note
	}
}

func (e *testRemediationEngine) ListExecutions(limit int) []*aicontracts.RemediationExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	list := make([]*aicontracts.RemediationExecution, 0, len(e.executions))
	for _, ex := range e.executions {
		if ex == nil {
			continue
		}
		cp := *ex
		list = append(list, &cp)
	}
	if len(list) > limit {
		list = list[:limit]
	}
	return list
}

func (e *testRemediationEngine) AddApprovalRule(rule *aicontracts.ApprovalRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if rule != nil && rule.ID != "" {
		e.approvalRules[rule.ID] = rule
	}
}

func (e *testRemediationEngine) IsAutoApproved(_ *aicontracts.RemediationPlan) bool {
	return false
}

func (e *testRemediationEngine) FormatPlanForContext(plan *aicontracts.RemediationPlan) string {
	if plan == nil {
		return ""
	}
	return fmt.Sprintf("Plan: %s - %s", plan.ID, plan.Title)
}

func (e *testRemediationEngine) SetCommandExecutor(executor aicontracts.CommandExecutor) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executor = executor
}
