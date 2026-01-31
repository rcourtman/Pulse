package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock PatrolFindingCreator ---

type mockPatrolFindingCreator struct {
	createFindingFunc  func(input PatrolFindingInput) (string, bool, error)
	resolveFindingFunc func(findingID, reason string) error
	getActiveFn        func(resourceID, minSeverity string) []PatrolFindingInfo

	// Track calls for assertions
	createCalls  []PatrolFindingInput
	resolveCalls []struct {
		FindingID string
		Reason    string
	}
	getCalls []struct {
		ResourceID  string
		MinSeverity string
	}

	checked bool
}

func (m *mockPatrolFindingCreator) CreateFinding(input PatrolFindingInput) (string, bool, error) {
	m.createCalls = append(m.createCalls, input)
	if m.createFindingFunc != nil {
		return m.createFindingFunc(input)
	}
	return "finding-123", true, nil
}

func (m *mockPatrolFindingCreator) ResolveFinding(findingID, reason string) error {
	m.resolveCalls = append(m.resolveCalls, struct {
		FindingID string
		Reason    string
	}{findingID, reason})
	if m.resolveFindingFunc != nil {
		return m.resolveFindingFunc(findingID, reason)
	}
	return nil
}

func (m *mockPatrolFindingCreator) GetActiveFindings(resourceID, minSeverity string) []PatrolFindingInfo {
	m.getCalls = append(m.getCalls, struct {
		ResourceID  string
		MinSeverity string
	}{resourceID, minSeverity})
	m.checked = true
	if m.getActiveFn != nil {
		return m.getActiveFn(resourceID, minSeverity)
	}
	return nil
}

func (m *mockPatrolFindingCreator) HasCheckedFindings() bool {
	return m.checked
}

// --- Helper to create executor with patrol creator ---

func newPatrolTestExecutor(creator PatrolFindingCreator) *PulseToolExecutor {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	exec.SetPatrolFindingCreator(creator)
	return exec
}

func validReportArgs() map[string]interface{} {
	return map[string]interface{}{
		"key":           "high-cpu",
		"severity":      "warning",
		"category":      "performance",
		"resource_id":   "node/pve1",
		"resource_name": "pve1",
		"resource_type": "node",
		"title":         "High CPU usage on pve1",
		"description":   "CPU usage is at 92% for the last 15 minutes",
	}
}

// --- patrol_report_finding tests ---

func TestHandlePatrolReportFinding_NilCreator(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})
	// Creator is nil (not in a patrol run)

	result, err := handlePatrolReportFinding(context.Background(), exec, validReportArgs())
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "only available during a patrol run")
}

func TestHandlePatrolReportFinding_RequiresGetFindings(t *testing.T) {
	creator := &mockPatrolFindingCreator{}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolReportFinding(context.Background(), exec, validReportArgs())
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "patrol_get_findings")
}

func TestHandlePatrolReportFinding_ValidInput(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	args := validReportArgs()
	args["recommendation"] = "Consider migrating VMs to reduce load"
	args["evidence"] = "CPU at 92% over 15 minutes"

	result, err := handlePatrolReportFinding(context.Background(), exec, args)
	require.NoError(t, err)

	var parsed map[string]interface{}
	text := extractText(result)
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))

	assert.Equal(t, true, parsed["ok"])
	assert.Equal(t, "finding-123", parsed["finding_id"])
	assert.Equal(t, true, parsed["is_new"])

	// Verify the creator was called with correct input
	require.Len(t, creator.createCalls, 1)
	input := creator.createCalls[0]
	assert.Equal(t, "high-cpu", input.Key)
	assert.Equal(t, "warning", input.Severity)
	assert.Equal(t, "performance", input.Category)
	assert.Equal(t, "node/pve1", input.ResourceID)
	assert.Equal(t, "pve1", input.ResourceName)
	assert.Equal(t, "node", input.ResourceType)
	assert.Equal(t, "High CPU usage on pve1", input.Title)
	assert.Equal(t, "CPU usage is at 92% for the last 15 minutes", input.Description)
	assert.Equal(t, "Consider migrating VMs to reduce load", input.Recommendation)
	assert.Equal(t, "CPU at 92% over 15 minutes", input.Evidence)
}

func TestHandlePatrolReportFinding_MissingRequiredFields(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	tests := []struct {
		name    string
		remove  string
		missing string
	}{
		{"missing key", "key", "key"},
		{"missing severity", "severity", "severity"},
		{"missing category", "category", "category"},
		{"missing resource_id", "resource_id", "resource_id"},
		{"missing resource_name", "resource_name", "resource_name"},
		{"missing resource_type", "resource_type", "resource_type"},
		{"missing title", "title", "title"},
		{"missing description", "description", "description"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := validReportArgs()
			delete(args, tt.remove)

			result, err := handlePatrolReportFinding(context.Background(), exec, args)
			require.NoError(t, err)

			text := extractText(result)
			assert.Contains(t, text, "missing required fields")
			assert.Contains(t, text, tt.missing)
		})
	}
}

func TestHandlePatrolReportFinding_AllRequiredFieldsMissing(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolReportFinding(context.Background(), exec, map[string]interface{}{})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "missing required fields")
	// Should mention all 8 required fields
	for _, field := range []string{"key", "severity", "category", "resource_id", "resource_name", "resource_type", "title", "description"} {
		assert.Contains(t, text, field)
	}
}

func TestHandlePatrolReportFinding_InvalidSeverity(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	args := validReportArgs()
	args["severity"] = "extreme"

	result, err := handlePatrolReportFinding(context.Background(), exec, args)
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "invalid severity")
	assert.Contains(t, text, "extreme")
}

func TestHandlePatrolReportFinding_InvalidCategory(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	args := validReportArgs()
	args["category"] = "networking"

	result, err := handlePatrolReportFinding(context.Background(), exec, args)
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "invalid category")
	assert.Contains(t, text, "networking")
}

func TestHandlePatrolReportFinding_ValidSeverities(t *testing.T) {
	for _, sev := range []string{"critical", "warning"} {
		t.Run(sev, func(t *testing.T) {
			creator := &mockPatrolFindingCreator{checked: true}
			exec := newPatrolTestExecutor(creator)

			args := validReportArgs()
			args["severity"] = sev

			result, err := handlePatrolReportFinding(context.Background(), exec, args)
			require.NoError(t, err)

			text := extractText(result)
			assert.NotContains(t, text, "invalid severity")

			var parsed map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &parsed))
			assert.Equal(t, true, parsed["ok"])
		})
	}
}

func TestHandlePatrolReportFinding_RejectedSeverities(t *testing.T) {
	for _, sev := range []string{"watch", "info"} {
		t.Run(sev, func(t *testing.T) {
			creator := &mockPatrolFindingCreator{checked: true}
			exec := newPatrolTestExecutor(creator)

			args := validReportArgs()
			args["severity"] = sev

			result, err := handlePatrolReportFinding(context.Background(), exec, args)
			require.NoError(t, err)

			text := extractText(result)
			assert.Contains(t, text, "invalid severity")
		})
	}
}

func TestHandlePatrolReportFinding_ValidCategories(t *testing.T) {
	for _, cat := range []string{"performance", "capacity", "reliability", "backup", "security", "general"} {
		t.Run(cat, func(t *testing.T) {
			creator := &mockPatrolFindingCreator{checked: true}
			exec := newPatrolTestExecutor(creator)

			args := validReportArgs()
			args["category"] = cat

			result, err := handlePatrolReportFinding(context.Background(), exec, args)
			require.NoError(t, err)

			text := extractText(result)
			assert.NotContains(t, text, "invalid category")

			var parsed map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &parsed))
			assert.Equal(t, true, parsed["ok"])
		})
	}
}

func TestHandlePatrolReportFinding_DuplicateFinding(t *testing.T) {
	creator := &mockPatrolFindingCreator{
		createFindingFunc: func(input PatrolFindingInput) (string, bool, error) {
			return "finding-existing", false, nil // isNew = false
		},
		checked: true,
	}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolReportFinding(context.Background(), exec, validReportArgs())
	require.NoError(t, err)

	var parsed map[string]interface{}
	text := extractText(result)
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))

	assert.Equal(t, true, parsed["ok"])
	assert.Equal(t, "finding-existing", parsed["finding_id"])
	assert.Equal(t, false, parsed["is_new"])
}

func TestHandlePatrolReportFinding_CreatorError(t *testing.T) {
	creator := &mockPatrolFindingCreator{
		createFindingFunc: func(input PatrolFindingInput) (string, bool, error) {
			return "", false, fmt.Errorf("validation failed: CPU is below threshold")
		},
		checked: true,
	}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolReportFinding(context.Background(), exec, validReportArgs())
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "failed to create finding")
	assert.Contains(t, text, "validation failed")
}

func TestHandlePatrolReportFinding_OptionalFieldsOmitted(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	// Only required fields, no recommendation or evidence
	result, err := handlePatrolReportFinding(context.Background(), exec, validReportArgs())
	require.NoError(t, err)

	var parsed map[string]interface{}
	text := extractText(result)
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, true, parsed["ok"])

	// Verify optional fields are empty strings
	require.Len(t, creator.createCalls, 1)
	assert.Equal(t, "", creator.createCalls[0].Recommendation)
	assert.Equal(t, "", creator.createCalls[0].Evidence)
}

// --- patrol_resolve_finding tests ---

func TestHandlePatrolResolveFinding_NilCreator(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	result, err := handlePatrolResolveFinding(context.Background(), exec, map[string]interface{}{
		"finding_id": "f-123",
		"reason":     "CPU recovered",
	})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "only available during a patrol run")
}

func TestHandlePatrolResolveFinding_RequiresGetFindings(t *testing.T) {
	creator := &mockPatrolFindingCreator{}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolResolveFinding(context.Background(), exec, map[string]interface{}{
		"finding_id": "f-123",
		"reason":     "CPU recovered",
	})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "patrol_get_findings")
}

func TestHandlePatrolResolveFinding_ValidInput(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolResolveFinding(context.Background(), exec, map[string]interface{}{
		"finding_id": "f-123",
		"reason":     "CPU has returned to 35%",
	})
	require.NoError(t, err)

	var parsed map[string]interface{}
	text := extractText(result)
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))

	assert.Equal(t, true, parsed["ok"])
	assert.Equal(t, true, parsed["resolved"])

	require.Len(t, creator.resolveCalls, 1)
	assert.Equal(t, "f-123", creator.resolveCalls[0].FindingID)
	assert.Equal(t, "CPU has returned to 35%", creator.resolveCalls[0].Reason)
}

func TestHandlePatrolResolveFinding_MissingFindingID(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolResolveFinding(context.Background(), exec, map[string]interface{}{
		"reason": "CPU recovered",
	})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "missing required field: finding_id")
}

func TestHandlePatrolResolveFinding_MissingReason(t *testing.T) {
	creator := &mockPatrolFindingCreator{checked: true}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolResolveFinding(context.Background(), exec, map[string]interface{}{
		"finding_id": "f-123",
	})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "missing required field: reason")
}

func TestHandlePatrolResolveFinding_ResolveError(t *testing.T) {
	creator := &mockPatrolFindingCreator{
		resolveFindingFunc: func(findingID, reason string) error {
			return fmt.Errorf("finding not found: %s", findingID)
		},
		checked: true,
	}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolResolveFinding(context.Background(), exec, map[string]interface{}{
		"finding_id": "f-nonexistent",
		"reason":     "CPU recovered",
	})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "failed to resolve finding")
	assert.Contains(t, text, "finding not found")
}

// --- patrol_get_findings tests ---

func TestHandlePatrolGetFindings_NilCreator(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	result, err := handlePatrolGetFindings(context.Background(), exec, map[string]interface{}{})
	require.NoError(t, err)

	text := extractText(result)
	assert.Contains(t, text, "only available during a patrol run")
}

func TestHandlePatrolGetFindings_EmptyResult(t *testing.T) {
	creator := &mockPatrolFindingCreator{}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolGetFindings(context.Background(), exec, map[string]interface{}{})
	require.NoError(t, err)

	var parsed map[string]interface{}
	text := extractText(result)
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))

	assert.Equal(t, true, parsed["ok"])
	assert.Equal(t, float64(0), parsed["count"])
}

func TestHandlePatrolGetFindings_WithFindings(t *testing.T) {
	creator := &mockPatrolFindingCreator{
		getActiveFn: func(resourceID, minSeverity string) []PatrolFindingInfo {
			return []PatrolFindingInfo{
				{
					ID:           "f-1",
					Key:          "high-cpu",
					Severity:     "warning",
					Category:     "performance",
					ResourceID:   "node/pve1",
					ResourceName: "pve1",
					ResourceType: "node",
					Title:        "High CPU on pve1",
					DetectedAt:   "2024-01-15T10:00:00Z",
				},
				{
					ID:           "f-2",
					Key:          "high-disk",
					Severity:     "critical",
					Category:     "capacity",
					ResourceID:   "node/pve2",
					ResourceName: "pve2",
					ResourceType: "node",
					Title:        "Disk almost full on pve2",
					DetectedAt:   "2024-01-15T09:00:00Z",
				},
			}
		},
	}
	exec := newPatrolTestExecutor(creator)

	result, err := handlePatrolGetFindings(context.Background(), exec, map[string]interface{}{})
	require.NoError(t, err)

	var parsed map[string]interface{}
	text := extractText(result)
	require.NoError(t, json.Unmarshal([]byte(text), &parsed))

	assert.Equal(t, true, parsed["ok"])
	assert.Equal(t, float64(2), parsed["count"])

	findings := parsed["findings"].([]interface{})
	require.Len(t, findings, 2)

	f1 := findings[0].(map[string]interface{})
	assert.Equal(t, "f-1", f1["id"])
	assert.Equal(t, "high-cpu", f1["key"])
	assert.Equal(t, "warning", f1["severity"])

	f2 := findings[1].(map[string]interface{})
	assert.Equal(t, "f-2", f2["id"])
	assert.Equal(t, "critical", f2["severity"])
}

func TestHandlePatrolGetFindings_WithFilters(t *testing.T) {
	creator := &mockPatrolFindingCreator{
		getActiveFn: func(resourceID, minSeverity string) []PatrolFindingInfo {
			// Verify filters are passed through
			return nil
		},
	}
	exec := newPatrolTestExecutor(creator)

	_, err := handlePatrolGetFindings(context.Background(), exec, map[string]interface{}{
		"resource_id": "node/pve1",
		"severity":    "warning",
	})
	require.NoError(t, err)

	require.Len(t, creator.getCalls, 1)
	assert.Equal(t, "node/pve1", creator.getCalls[0].ResourceID)
	assert.Equal(t, "warning", creator.getCalls[0].MinSeverity)
}

func TestHandlePatrolGetFindings_NoFilters(t *testing.T) {
	creator := &mockPatrolFindingCreator{}
	exec := newPatrolTestExecutor(creator)

	_, err := handlePatrolGetFindings(context.Background(), exec, map[string]interface{}{})
	require.NoError(t, err)

	require.Len(t, creator.getCalls, 1)
	assert.Equal(t, "", creator.getCalls[0].ResourceID)
	assert.Equal(t, "", creator.getCalls[0].MinSeverity)
}

// --- SetPatrolFindingCreator tests ---

func TestSetPatrolFindingCreator(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	// Initially nil
	assert.Nil(t, exec.GetPatrolFindingCreator())

	// Set a creator
	creator := &mockPatrolFindingCreator{}
	exec.SetPatrolFindingCreator(creator)
	assert.NotNil(t, exec.GetPatrolFindingCreator())

	// Clear it
	exec.SetPatrolFindingCreator(nil)
	assert.Nil(t, exec.GetPatrolFindingCreator())
}

// --- Tool registration tests ---

func TestPatrolToolsRegistered(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	// Patrol tools should be registered but availability depends on patrolFindingCreator
	tools := exec.registry.ListTools(ControlLevelReadOnly)
	found := map[string]bool{}
	for _, tool := range tools {
		if tool.Name == "patrol_report_finding" || tool.Name == "patrol_resolve_finding" || tool.Name == "patrol_get_findings" {
			found[tool.Name] = true
		}
	}

	assert.True(t, found["patrol_report_finding"], "patrol_report_finding should be registered")
	assert.True(t, found["patrol_resolve_finding"], "patrol_resolve_finding should be registered")
	assert.True(t, found["patrol_get_findings"], "patrol_get_findings should be registered")
}

func TestPatrolToolsAvailability(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{})

	// Without creator, patrol tools should not be available
	assert.False(t, exec.isToolAvailable("patrol_report_finding"))
	assert.False(t, exec.isToolAvailable("patrol_resolve_finding"))
	assert.False(t, exec.isToolAvailable("patrol_get_findings"))

	// Set creator
	exec.SetPatrolFindingCreator(&mockPatrolFindingCreator{})

	// Now they should be available
	assert.True(t, exec.isToolAvailable("patrol_report_finding"))
	assert.True(t, exec.isToolAvailable("patrol_resolve_finding"))
	assert.True(t, exec.isToolAvailable("patrol_get_findings"))
}

// --- Helper ---

func extractText(result CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}
