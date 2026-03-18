package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
)

// testRemediationEngine is a minimal in-memory implementation of aicontracts.RemediationEngine
// used in tests that need a functioning engine without importing internal/ai/remediation.
type testRemediationEngine struct {
	mu          sync.RWMutex
	plans       map[string]*aicontracts.RemediationPlan
	executions  map[string]*aicontracts.RemediationExecution
	planCounter int
	execCounter int
}

func newTestRemediationEngine() *testRemediationEngine {
	return &testRemediationEngine{
		plans:      make(map[string]*aicontracts.RemediationPlan),
		executions: make(map[string]*aicontracts.RemediationExecution),
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
	return nil
}

func (e *testRemediationEngine) GetLatestExecutionForPlan(_ string) *aicontracts.RemediationExecution {
	return nil
}

func (e *testRemediationEngine) ApprovePlan(planID, approvedBy string) (*aicontracts.RemediationExecution, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.plans[planID]; !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
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

func (e *testRemediationEngine) Execute(_ context.Context, _ string) error {
	return nil
}

func (e *testRemediationEngine) Rollback(_ context.Context, _ string) error {
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

func (e *testRemediationEngine) SetExecutionVerification(_ string, _ bool, _ string) {}

func (e *testRemediationEngine) ListExecutions(_ int) []*aicontracts.RemediationExecution {
	return nil
}

func (e *testRemediationEngine) AddApprovalRule(_ *aicontracts.ApprovalRule) {}

func (e *testRemediationEngine) IsAutoApproved(_ *aicontracts.RemediationPlan) bool {
	return false
}

func (e *testRemediationEngine) FormatPlanForContext(plan *aicontracts.RemediationPlan) string {
	if plan == nil {
		return ""
	}
	return fmt.Sprintf("Plan: %s - %s", plan.ID, plan.Title)
}

func (e *testRemediationEngine) SetCommandExecutor(_ aicontracts.CommandExecutor) {}
