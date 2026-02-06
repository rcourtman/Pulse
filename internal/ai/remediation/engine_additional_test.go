package remediation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewEngine_Defaults(t *testing.T) {
	engine := NewEngine(EngineConfig{})
	if engine.config.MaxExecutions != 100 {
		t.Fatalf("expected MaxExecutions default, got %d", engine.config.MaxExecutions)
	}
	if engine.config.PlanExpiry != 24*time.Hour {
		t.Fatalf("expected PlanExpiry default, got %s", engine.config.PlanExpiry)
	}
	if engine.config.ExecutionTimeout != 5*time.Minute {
		t.Fatalf("expected ExecutionTimeout default, got %s", engine.config.ExecutionTimeout)
	}
}

func TestEngine_ValidatePlan_TitleRequired(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	err := engine.CreatePlan(&RemediationPlan{
		Steps: []RemediationStep{{Command: "echo ok"}},
	})
	if err == nil {
		t.Fatalf("expected error for missing title")
	}
}

func TestEngine_IsBlockedCommand_CaseInsensitive(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if !engine.isBlockedCommand("RM -RF /tmp") {
		t.Fatalf("expected blocked command detection")
	}
	if engine.isBlockedCommand("") {
		t.Fatalf("expected empty command to be allowed")
	}
}

func TestEngine_AssessRiskAndCategorize(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title: "Delete data",
		Steps: []RemediationStep{{Command: "delete something"}},
	}
	if engine.assessRiskLevel(plan) != RiskHigh {
		t.Fatalf("expected high risk")
	}
	plan.RiskLevel = RiskHigh
	if engine.categorize(plan) != CategoryGuided {
		t.Fatalf("expected guided for high risk")
	}

	plan = &RemediationPlan{
		Title: "Info only",
		Steps: []RemediationStep{{Description: "observe"}},
	}
	if engine.categorize(plan) != CategoryInformational {
		t.Fatalf("expected informational for no commands")
	}

	plan = &RemediationPlan{
		Title:     "Low risk",
		Steps:     []RemediationStep{{Command: "echo ok"}},
		RiskLevel: RiskLow,
	}
	if engine.categorize(plan) != CategoryOneClick {
		t.Fatalf("expected one-click for low risk small plan")
	}
}

func TestEngine_ListPlans_SkipsExpiredAndOrders(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	now := time.Now()
	expiredAt := now.Add(-1 * time.Hour)

	planOld := &RemediationPlan{
		Title:     "old",
		Steps:     []RemediationStep{{Command: "echo old"}},
		CreatedAt: now.Add(-2 * time.Hour),
	}
	planNew := &RemediationPlan{
		Title:     "new",
		Steps:     []RemediationStep{{Command: "echo new"}},
		CreatedAt: now.Add(-1 * time.Hour),
	}
	planExpired := &RemediationPlan{
		Title:     "expired",
		Steps:     []RemediationStep{{Command: "echo expired"}},
		CreatedAt: now,
		ExpiresAt: &expiredAt,
	}

	if err := engine.CreatePlan(planOld); err != nil {
		t.Fatalf("create plan old failed: %v", err)
	}
	if err := engine.CreatePlan(planNew); err != nil {
		t.Fatalf("create plan new failed: %v", err)
	}
	if err := engine.CreatePlan(planExpired); err != nil {
		t.Fatalf("create plan expired failed: %v", err)
	}

	plans := engine.ListPlans(10)
	if len(plans) != 2 {
		t.Fatalf("expected 2 non-expired plans, got %d", len(plans))
	}
	if plans[0].Title != "new" {
		t.Fatalf("expected newest plan first")
	}

	if len(engine.ListPlans(0)) == 0 {
		t.Fatalf("expected ListPlans to return results with default limit")
	}
}

func TestEngine_CreatePlan_ExpiresPriorPlansForFinding(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan1 := &RemediationPlan{
		FindingID: "finding-1",
		Title:     "p1",
		Steps:     []RemediationStep{{Command: "echo one"}},
	}
	if err := engine.CreatePlan(plan1); err != nil {
		t.Fatalf("create plan1 failed: %v", err)
	}

	plan2 := &RemediationPlan{
		FindingID: "finding-1",
		Title:     "p2",
		Steps:     []RemediationStep{{Command: "echo two"}},
	}
	if err := engine.CreatePlan(plan2); err != nil {
		t.Fatalf("create plan2 failed: %v", err)
	}

	plans := engine.ListPlans(10)
	if len(plans) != 1 {
		t.Fatalf("expected only the latest plan to remain active, got %d", len(plans))
	}
	if plans[0].Title != "p2" {
		t.Fatalf("expected latest plan to be p2, got %q", plans[0].Title)
	}
}

func TestEngine_GetPlanForFinding_SkipsExpired(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	expiredAt := time.Now().Add(-1 * time.Hour)

	plan := &RemediationPlan{
		FindingID: "finding-1",
		Title:     "expired plan",
		Steps:     []RemediationStep{{Command: "echo test"}},
		ExpiresAt: &expiredAt,
	}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	if engine.GetPlanForFinding("finding-1") != nil {
		t.Fatalf("expected expired plan to be skipped")
	}
}

func TestEngine_GetLatestExecutionForPlan(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	planID := "plan-1"

	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	engine.executions["e1"] = &RemediationExecution{
		ID:          "e1",
		PlanID:      planID,
		CompletedAt: &older,
	}
	engine.executions["e2"] = &RemediationExecution{
		ID:         "e2",
		PlanID:     planID,
		ApprovedAt: &newer,
	}

	latest := engine.GetLatestExecutionForPlan(planID)
	if latest == nil || latest.ID != "e2" {
		t.Fatalf("expected latest execution to be e2")
	}
}

func TestEngine_GetLatestExecutionForPlan_None(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if engine.GetLatestExecutionForPlan("missing") != nil {
		t.Fatalf("expected nil when no executions exist")
	}
}

func TestEngine_Execute_Errors(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	if err := engine.Execute(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error for missing execution")
	}

	engine.executions["e1"] = &RemediationExecution{ID: "e1", Status: StatusRunning}
	if err := engine.Execute(context.Background(), "e1"); err == nil {
		t.Fatalf("expected error for non-approved execution")
	}

	engine.executions["e2"] = &RemediationExecution{ID: "e2", Status: StatusApproved, PlanID: "missing-plan"}
	if err := engine.Execute(context.Background(), "e2"); err == nil {
		t.Fatalf("expected error for missing plan")
	}

	plan := &RemediationPlan{Title: "p", Steps: []RemediationStep{{Command: "echo"}}}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	exec := &RemediationExecution{ID: "e3", Status: StatusApproved, PlanID: plan.ID}
	engine.executions["e3"] = exec
	if err := engine.Execute(context.Background(), "e3"); err == nil {
		t.Fatalf("expected error when executor is missing")
	}
}

func TestEngine_Rollback_SuccessAndError(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	executor := newMockExecutor()
	executor.results["rollback-1"] = struct {
		output string
		err    error
	}{output: "ok", err: nil}
	executor.results["rollback-2"] = struct {
		output string
		err    error
	}{output: "ok", err: nil}
	engine.SetCommandExecutor(executor)

	plan := &RemediationPlan{
		Title: "with rollback",
		Steps: []RemediationStep{
			{Order: 1, Command: "cmd-1", Rollback: "rollback-1"},
			{Order: 2, Command: "cmd-2", Rollback: "rollback-2"},
		},
	}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	exec, _ := engine.ApprovePlan(plan.ID, "admin")
	if err := engine.Execute(context.Background(), exec.ID); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if err := engine.Rollback(context.Background(), exec.ID); err != nil {
		t.Fatalf("rollback failed: %v", err)
	}
	updated := engine.GetExecution(exec.ID)
	if updated.Status != StatusRolledBack {
		t.Fatalf("expected rolled back status")
	}
}

func TestEngine_Rollback_Error(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	executor := newMockExecutor()
	executor.results["rollback-bad"] = struct {
		output string
		err    error
	}{output: "", err: errors.New("rollback failed")}
	engine.SetCommandExecutor(executor)

	plan := &RemediationPlan{
		Title: "rollback error",
		Steps: []RemediationStep{
			{Order: 1, Command: "cmd-1", Rollback: "rollback-bad"},
		},
	}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	exec, _ := engine.ApprovePlan(plan.ID, "admin")
	if err := engine.Execute(context.Background(), exec.ID); err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if err := engine.Rollback(context.Background(), exec.ID); err == nil {
		t.Fatalf("expected rollback error")
	}
	updated := engine.GetExecution(exec.ID)
	if updated.RollbackError == "" {
		t.Fatalf("expected rollback error to be recorded")
	}
}

func TestEngine_ListExecutions_Additional(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	old := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	engine.executions["e1"] = &RemediationExecution{ID: "e1", ApprovedAt: &old}
	engine.executions["e2"] = &RemediationExecution{ID: "e2", ApprovedAt: &newer}

	execs := engine.ListExecutions(1)
	if len(execs) != 1 {
		t.Fatalf("expected limit to apply")
	}
	if execs[0].ID != "e2" {
		t.Fatalf("expected most recent execution first")
	}
}

func TestEngine_AddApprovalRuleAndAutoApprove(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	engine.AddApprovalRule(&ApprovalRule{
		Category:     CategoryOneClick,
		MaxRiskLevel: RiskMedium,
		Enabled:      true,
	})

	plan := &RemediationPlan{
		Title:     "plan",
		Steps:     []RemediationStep{{Command: "echo"}},
		Category:  CategoryOneClick,
		RiskLevel: RiskLow,
	}

	if !engine.IsAutoApproved(plan) {
		t.Fatalf("expected plan to be auto-approved")
	}

	engine.AddApprovalRule(&ApprovalRule{
		Category:     CategoryGuided,
		MaxRiskLevel: RiskLow,
		Enabled:      false,
	})
	if !engine.IsAutoApproved(plan) {
		t.Fatalf("disabled rule should not block auto-approval")
	}

	plan.RiskLevel = RiskHigh
	if engine.IsAutoApproved(plan) {
		t.Fatalf("expected high risk plan to be blocked by max risk")
	}
}

func TestEngine_FormatPlanForContext_Additional(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if engine.FormatPlanForContext(nil) != "" {
		t.Fatalf("expected empty format for nil plan")
	}

	plan := &RemediationPlan{
		Title:       "Fix issue",
		Description: "do the thing",
		Category:    CategoryGuided,
		RiskLevel:   RiskMedium,
		Prerequisites: []string{
			"backup data",
		},
		Warnings: []string{"service restart"},
		Steps: []RemediationStep{
			{Order: 1, Description: "step one", Command: "cmd-1", Rollback: "rb-1"},
		},
	}

	formatted := engine.FormatPlanForContext(plan)
	if formatted == "" {
		t.Fatalf("expected formatted plan")
	}
	if !contains(formatted, "Prerequisites") || !contains(formatted, "Warnings") {
		t.Fatalf("expected prerequisites and warnings sections")
	}
	if !contains(formatted, "Rollback") {
		t.Fatalf("expected rollback details")
	}
}

func TestEngine_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	engine := NewEngine(EngineConfig{})
	engine.plans["plan-1"] = &RemediationPlan{
		ID:    "plan-1",
		Title: "save plan",
		Steps: []RemediationStep{{Command: "echo ok"}},
	}
	engine.executions["exec-1"] = &RemediationExecution{
		ID:     "exec-1",
		PlanID: "plan-1",
		Status: StatusApproved,
	}
	engine.approvalRules["rule-1"] = &ApprovalRule{
		ID:           "rule-1",
		Category:     CategoryGuided,
		MaxRiskLevel: RiskMedium,
		Enabled:      true,
	}
	engine.dataDir = dir

	if err := engine.saveToDisk(); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "remediation.json")); err != nil {
		t.Fatalf("expected remediation.json to exist: %v", err)
	}

	loaded := NewEngine(EngineConfig{DataDir: dir})
	if len(loaded.plans) == 0 || len(loaded.executions) == 0 || len(loaded.approvalRules) == 0 {
		t.Fatalf("expected data to load from disk")
	}
}

func TestTruncateString(t *testing.T) {
	short := "abc"
	if got := truncateString(short, 10); got != short {
		t.Fatalf("expected short string to remain unchanged")
	}

	long := "this is a long string"
	got := truncateString(long, 4)
	if got != "this..." {
		t.Fatalf("expected truncation with ellipsis, got %q", got)
	}
}

func TestEngine_Rollback_Errors(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if err := engine.Rollback(context.Background(), "missing"); err == nil {
		t.Fatalf("expected error for missing execution")
	}

	engine.executions["e1"] = &RemediationExecution{ID: "e1", PlanID: "missing-plan"}
	if err := engine.Rollback(context.Background(), "e1"); err == nil {
		t.Fatalf("expected error for missing plan")
	}

	plan := &RemediationPlan{Title: "p", Steps: []RemediationStep{{Command: "echo"}}}
	if err := engine.CreatePlan(plan); err != nil {
		t.Fatalf("create plan failed: %v", err)
	}
	engine.executions["e2"] = &RemediationExecution{ID: "e2", PlanID: plan.ID}
	if err := engine.Rollback(context.Background(), "e2"); err == nil {
		t.Fatalf("expected error for missing executor")
	}
}

func TestEngine_SaveToDisk_NoDir(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if err := engine.saveToDisk(); err != nil {
		t.Fatalf("expected saveToDisk to no-op without DataDir, got %v", err)
	}
}

func TestEngine_SaveIfDirty(t *testing.T) {
	dir := t.TempDir()
	engine := NewEngine(EngineConfig{DataDir: dir})
	engine.plans["p1"] = &RemediationPlan{ID: "p1", Title: "t", Steps: []RemediationStep{{Command: "echo"}}}
	engine.saveIfDirty()

	if _, err := os.Stat(filepath.Join(dir, "remediation.json")); err != nil {
		t.Fatalf("expected saveIfDirty to persist data: %v", err)
	}
}

func TestEngine_SaveIfDirty_Error(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	engine := NewEngine(EngineConfig{DataDir: filePath})
	engine.plans["p1"] = &RemediationPlan{ID: "p1", Title: "t", Steps: []RemediationStep{{Command: "echo"}}}
	engine.saveIfDirty()
}

func TestEngine_GetExecution_NotFound(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	if engine.GetExecution("missing") != nil {
		t.Fatalf("expected nil for missing execution")
	}
}

func TestRiskValue_Additional(t *testing.T) {
	cases := map[RiskLevel]int{
		RiskLow:              1,
		RiskMedium:           2,
		RiskHigh:             3,
		RiskCritical:         4,
		RiskLevel("unknown"): 0,
	}
	for level, expected := range cases {
		if got := riskValue(level); got != expected {
			t.Fatalf("expected %d for %s, got %d", expected, level, got)
		}
	}
}

func TestEngine_SaveToDisk_Error(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(filePath, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	engine := NewEngine(EngineConfig{DataDir: filePath})
	engine.plans["p1"] = &RemediationPlan{ID: "p1", Title: "t", Steps: []RemediationStep{{Command: "echo"}}}
	if err := engine.saveToDisk(); err == nil {
		t.Fatalf("expected saveToDisk to fail with invalid data dir")
	}
}
