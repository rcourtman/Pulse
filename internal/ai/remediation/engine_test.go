package remediation

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Mock command executor
type mockExecutor struct {
	commands []string
	results  map[string]struct {
		output string
		err    error
	}
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		results: make(map[string]struct {
			output string
			err    error
		}),
	}
}

func (m *mockExecutor) Execute(ctx context.Context, target, command string) (string, error) {
	m.commands = append(m.commands, command)
	if result, ok := m.results[command]; ok {
		return result.output, result.err
	}
	return "OK", nil
}

func TestEngine_CreatePlan(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		FindingID:   "finding-1",
		ResourceID:  "vm-101",
		Title:       "Restart service",
		Description: "Restart the web service to fix memory leak",
		Steps: []RemediationStep{
			{Order: 1, Description: "Restart service", Command: "systemctl restart nginx"},
			{Order: 2, Description: "Verify service", Command: "systemctl status nginx"},
		},
	}

	err := engine.CreatePlan(plan)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan.ID == "" {
		t.Error("Expected plan ID to be generated")
	}

	if plan.RiskLevel == "" {
		t.Error("Expected risk level to be assessed")
	}

	if plan.Category == "" {
		t.Error("Expected category to be set")
	}
}

func TestEngine_CreatePlan_Blocked(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title: "Dangerous plan",
		Steps: []RemediationStep{
			{Order: 1, Command: "rm -rf /"},
		},
	}

	err := engine.CreatePlan(plan)
	if err == nil {
		t.Error("Expected error for blocked command")
	}
}

func TestEngine_CreatePlan_NoSteps(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title: "Empty plan",
		Steps: []RemediationStep{},
	}

	err := engine.CreatePlan(plan)
	if err == nil {
		t.Error("Expected error for plan with no steps")
	}
}

func TestEngine_GetPlan(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title: "Test plan",
		Steps: []RemediationStep{{Command: "echo test"}},
	}
	_ = engine.CreatePlan(plan)

	retrieved := engine.GetPlan(plan.ID)
	if retrieved == nil {
		t.Fatal("Expected to retrieve plan")
	}

	if retrieved.Title != plan.Title {
		t.Errorf("Title mismatch")
	}
}

func TestEngine_GetPlan_NotFound(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	retrieved := engine.GetPlan("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for nonexistent plan")
	}
}

func TestEngine_GetPlanForFinding(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		FindingID: "finding-123",
		Title:     "Fix for finding",
		Steps:     []RemediationStep{{Command: "echo fix"}},
	}
	_ = engine.CreatePlan(plan)

	retrieved := engine.GetPlanForFinding("finding-123")
	if retrieved == nil {
		t.Fatal("Expected to retrieve plan by finding ID")
	}

	if retrieved.ID != plan.ID {
		t.Error("Plan ID mismatch")
	}
}

func TestEngine_ApprovePlan(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title: "Test plan",
		Steps: []RemediationStep{{Command: "echo test"}},
	}
	_ = engine.CreatePlan(plan)

	execution, err := engine.ApprovePlan(plan.ID, "admin")
	if err != nil {
		t.Fatalf("ApprovePlan failed: %v", err)
	}

	if execution.Status != StatusApproved {
		t.Errorf("Expected status Approved, got %s", execution.Status)
	}

	if execution.ApprovedBy != "admin" {
		t.Errorf("Expected approved by 'admin', got %s", execution.ApprovedBy)
	}
}

func TestEngine_ApprovePlan_NotFound(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	_, err := engine.ApprovePlan("nonexistent", "admin")
	if err == nil {
		t.Error("Expected error for nonexistent plan")
	}
}

func TestEngine_ApprovePlan_Expired(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	expired := time.Now().Add(-1 * time.Hour)
	plan := &RemediationPlan{
		Title:     "Expired plan",
		Steps:     []RemediationStep{{Command: "echo test"}},
		ExpiresAt: &expired,
	}
	_ = engine.CreatePlan(plan)

	_, err := engine.ApprovePlan(plan.ID, "admin")
	if err == nil {
		t.Error("Expected error for expired plan")
	}
}

func TestEngine_Execute(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	executor := newMockExecutor()
	engine.SetCommandExecutor(executor)

	plan := &RemediationPlan{
		Title: "Test execution",
		Steps: []RemediationStep{
			{Order: 1, Command: "echo step1"},
			{Order: 2, Command: "echo step2"},
		},
	}
	_ = engine.CreatePlan(plan)

	execution, _ := engine.ApprovePlan(plan.ID, "admin")

	ctx := context.Background()
	err := engine.Execute(ctx, execution.ID)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Check commands were executed
	if len(executor.commands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(executor.commands))
	}

	// Check execution status
	updated := engine.GetExecution(execution.ID)
	if updated.Status != StatusCompleted {
		t.Errorf("Expected status Completed, got %s", updated.Status)
	}
}

func TestEngine_Execute_Failure(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	executor := newMockExecutor()
	executor.results["echo fail"] = struct {
		output string
		err    error
	}{output: "", err: fmt.Errorf("command failed")}
	engine.SetCommandExecutor(executor)

	plan := &RemediationPlan{
		Title: "Failing plan",
		Steps: []RemediationStep{
			{Order: 1, Command: "echo fail"},
		},
	}
	_ = engine.CreatePlan(plan)

	execution, _ := engine.ApprovePlan(plan.ID, "admin")

	ctx := context.Background()
	err := engine.Execute(ctx, execution.ID)
	if err == nil {
		t.Error("Expected error from failing command")
	}

	updated := engine.GetExecution(execution.ID)
	if updated.Status != StatusFailed {
		t.Errorf("Expected status Failed, got %s", updated.Status)
	}
}

func TestEngine_Execute_NotApproved(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	engine.SetCommandExecutor(newMockExecutor())

	ctx := context.Background()
	err := engine.Execute(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent execution")
	}
}

func TestEngine_Execute_NoExecutor(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title: "Test",
		Steps: []RemediationStep{{Command: "echo test"}},
	}
	_ = engine.CreatePlan(plan)
	execution, _ := engine.ApprovePlan(plan.ID, "admin")

	ctx := context.Background()
	err := engine.Execute(ctx, execution.ID)
	if err == nil {
		t.Error("Expected error when no executor configured")
	}
}

func TestEngine_Rollback(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())
	executor := newMockExecutor()
	engine.SetCommandExecutor(executor)

	plan := &RemediationPlan{
		Title: "Rollback test",
		Steps: []RemediationStep{
			{Order: 1, Command: "echo do", Rollback: "echo undo"},
		},
	}
	_ = engine.CreatePlan(plan)

	execution, _ := engine.ApprovePlan(plan.ID, "admin")

	ctx := context.Background()
	_ = engine.Execute(ctx, execution.ID)

	// Rollback
	err := engine.Rollback(ctx, execution.ID)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Check rollback command was executed
	foundRollback := false
	for _, cmd := range executor.commands {
		if cmd == "echo undo" {
			foundRollback = true
		}
	}
	if !foundRollback {
		t.Error("Expected rollback command to be executed")
	}
}

func TestEngine_ListExecutions(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	for i := 0; i < 3; i++ {
		plan := &RemediationPlan{
			Title: fmt.Sprintf("Plan %d", i),
			Steps: []RemediationStep{{Command: "echo test"}},
		}
		_ = engine.CreatePlan(plan)
		_, _ = engine.ApprovePlan(plan.ID, "admin")
	}

	executions := engine.ListExecutions(10)
	if len(executions) != 3 {
		t.Errorf("Expected 3 executions, got %d", len(executions))
	}
}

func TestEngine_AddApprovalRule(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	rule := &ApprovalRule{
		Description:  "Auto-approve low-risk restarts",
		Category:     CategoryOneClick,
		ActionType:   "restart",
		MaxRiskLevel: RiskLow,
		Enabled:      true,
	}

	engine.AddApprovalRule(rule)

	if rule.ID == "" {
		t.Error("Expected rule ID to be generated")
	}
}

func TestEngine_IsAutoApproved(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	engine.AddApprovalRule(&ApprovalRule{
		Category:     CategoryOneClick,
		MaxRiskLevel: RiskLow,
		Enabled:      true,
	})

	plan := &RemediationPlan{
		Title:     "Low risk plan",
		Category:  CategoryOneClick,
		RiskLevel: RiskLow,
		Steps:     []RemediationStep{{Command: "echo safe"}},
	}

	if !engine.IsAutoApproved(plan) {
		t.Error("Expected plan to be auto-approved")
	}

	plan.RiskLevel = RiskHigh
	if engine.IsAutoApproved(plan) {
		t.Error("Expected high-risk plan to NOT be auto-approved")
	}
}

func TestEngine_RiskAssessment(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	tests := []struct {
		command  string
		expected RiskLevel
	}{
		{"echo hello", RiskLow},
		{"systemctl restart nginx", RiskMedium},
		{"delete /tmp/cache", RiskHigh}, // Contains "delete"
	}

	for _, tt := range tests {
		plan := &RemediationPlan{
			Title: "Test",
			Steps: []RemediationStep{{Command: tt.command}},
		}

		risk := engine.assessRiskLevel(plan)
		if risk != tt.expected {
			t.Errorf("assessRiskLevel(%s) = %s, want %s", tt.command, risk, tt.expected)
		}
	}
}

func TestEngine_Categorization(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	// No commands = informational
	plan := &RemediationPlan{
		Title:     "Info only",
		RiskLevel: RiskLow,
		Steps:     []RemediationStep{{Description: "Manual step"}},
	}
	cat := engine.categorize(plan)
	if cat != CategoryInformational {
		t.Errorf("Expected Informational for no commands, got %s", cat)
	}

	// High risk = guided
	plan = &RemediationPlan{
		Title:     "High risk",
		RiskLevel: RiskHigh,
		Steps:     []RemediationStep{{Command: "dangerous"}},
	}
	cat = engine.categorize(plan)
	if cat != CategoryGuided {
		t.Errorf("Expected Guided for high risk, got %s", cat)
	}

	// Low risk simple = one click
	plan = &RemediationPlan{
		Title:     "Simple",
		RiskLevel: RiskLow,
		Steps:     []RemediationStep{{Command: "echo test"}},
	}
	cat = engine.categorize(plan)
	if cat != CategoryOneClick {
		t.Errorf("Expected OneClick for simple low risk, got %s", cat)
	}
}

func TestEngine_FormatPlanForContext(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	plan := &RemediationPlan{
		Title:       "Fix CPU issue",
		Description: "Restart the service to clear memory",
		Category:    CategoryOneClick,
		RiskLevel:   RiskLow,
		Steps: []RemediationStep{
			{Description: "Stop service", Command: "systemctl stop nginx", Rollback: "systemctl start nginx"},
			{Description: "Start service", Command: "systemctl start nginx"},
		},
		Prerequisites: []string{"SSH access to host"},
		Warnings:      []string{"Service will be temporarily unavailable"},
	}

	context := engine.FormatPlanForContext(plan)

	if context == "" {
		t.Error("Expected non-empty context")
	}

	if !containsStr(context, "Fix CPU issue") {
		t.Error("Expected title in context")
	}

	if !containsStr(context, "Steps:") {
		t.Error("Expected steps in context")
	}

	if !containsStr(context, "Warnings:") {
		t.Error("Expected warnings in context")
	}
}

func TestEngine_FormatPlanForContext_Nil(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	context := engine.FormatPlanForContext(nil)
	if context != "" {
		t.Error("Expected empty context for nil plan")
	}
}

func TestBlockedCommands(t *testing.T) {
	engine := NewEngine(DefaultEngineConfig())

	blockedCommands := []string{
		"rm -rf /",
		"shutdown -h now",
		"zpool destroy tank",
		"DROP DATABASE users",
		"dd if=/dev/zero of=/dev/sda",
	}

	for _, cmd := range blockedCommands {
		if !engine.isBlockedCommand(cmd) {
			t.Errorf("Expected command to be blocked: %s", cmd)
		}
	}

	allowedCommands := []string{
		"systemctl restart nginx",
		"docker restart container",
		"echo hello",
	}

	for _, cmd := range allowedCommands {
		if engine.isBlockedCommand(cmd) {
			t.Errorf("Expected command to be allowed: %s", cmd)
		}
	}
}

func TestRiskValue(t *testing.T) {
	tests := []struct {
		risk     RiskLevel
		expected int
	}{
		{RiskLow, 1},
		{RiskMedium, 2},
		{RiskHigh, 3},
		{RiskCritical, 4},
	}

	for _, tt := range tests {
		result := riskValue(tt.risk)
		if result != tt.expected {
			t.Errorf("riskValue(%s) = %d, want %d", tt.risk, result, tt.expected)
		}
	}
}

// Helper
func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
