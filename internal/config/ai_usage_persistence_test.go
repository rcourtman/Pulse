package config

import (
	"testing"
	"time"
)

func TestSaveLoadAIUsageHistory(t *testing.T) {
	dir := t.TempDir()
	cp := NewConfigPersistence(dir)

	now := time.Now()
	events := []AIUsageEventRecord{
		{
			Timestamp:     now,
			Provider:      "openai",
			RequestModel:  "openai:gpt-4o",
			InputTokens:   123,
			OutputTokens:  45,
			UseCase:       "chat",
			ToolCallCount: 2,
			TargetType:    "vm",
			TargetID:      "vm-101",
		},
	}

	if err := cp.SaveAIUsageHistory(events); err != nil {
		t.Fatalf("SaveAIUsageHistory: %v", err)
	}

	loaded, err := cp.LoadAIUsageHistory()
	if err != nil {
		t.Fatalf("LoadAIUsageHistory: %v", err)
	}

	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Provider != "openai" || loaded.Events[0].InputTokens != 123 {
		t.Fatalf("loaded event mismatch: %+v", loaded.Events[0])
	}
	if loaded.Events[0].ToolCallCount != 2 {
		t.Fatalf("loaded tool call count = %d, want 2", loaded.Events[0].ToolCallCount)
	}
}

func TestSaveLoadExternalAgentActivityHistory(t *testing.T) {
	dir := t.TempDir()
	cp := NewConfigPersistence(dir)

	now := time.Now().UTC()
	events := []ExternalAgentActivityRecord{
		{
			Timestamp: now,
			Surface:   ExternalAgentActivitySurfaceAgentAPI,
			Activity:  ExternalAgentActivityFleetContext,
		},
	}

	if err := cp.SaveExternalAgentActivityHistory(events); err != nil {
		t.Fatalf("SaveExternalAgentActivityHistory: %v", err)
	}

	loaded, err := cp.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}

	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Surface != ExternalAgentActivitySurfaceAgentAPI ||
		loaded.Events[0].Activity != ExternalAgentActivityFleetContext {
		t.Fatalf("loaded event mismatch: %+v", loaded.Events[0])
	}
}

func TestRecordExternalAgentActivityPrunesAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	cp := NewConfigPersistence(dir)

	now := time.Now().UTC()
	if err := cp.SaveExternalAgentActivityHistory([]ExternalAgentActivityRecord{
		{
			Timestamp: now.Add(-(externalAgentActivityHistoryRetention + time.Hour)),
			Surface:   ExternalAgentActivitySurfaceAgentAPI,
			Activity:  ExternalAgentActivityResourceContext,
		},
	}); err != nil {
		t.Fatalf("SaveExternalAgentActivityHistory: %v", err)
	}

	if err := cp.RecordExternalAgentActivity(ExternalAgentActivityRecord{
		Timestamp: now,
		Activity:  ExternalAgentActivityActionPlan,
	}); err != nil {
		t.Fatalf("RecordExternalAgentActivity: %v", err)
	}

	loaded, err := cp.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("expected only recent event after prune, got %d: %+v", len(loaded.Events), loaded.Events)
	}
	if loaded.Events[0].Surface != ExternalAgentActivitySurfaceAgentAPI ||
		loaded.Events[0].Activity != ExternalAgentActivityActionPlan {
		t.Fatalf("recorded event mismatch: %+v", loaded.Events[0])
	}
}

func TestSaveLoadWorkflowPromptActivityHistory(t *testing.T) {
	dir := t.TempDir()
	cp := NewConfigPersistence(dir)

	now := time.Now().UTC()
	events := []WorkflowPromptActivityRecord{
		{
			Timestamp:  now,
			Surface:    WorkflowPromptActivitySurfacePulseMCP,
			PromptName: "pulse_operations_loop",
		},
	}

	if err := cp.SaveWorkflowPromptActivityHistory(events); err != nil {
		t.Fatalf("SaveWorkflowPromptActivityHistory: %v", err)
	}

	loaded, err := cp.LoadWorkflowPromptActivityHistory()
	if err != nil {
		t.Fatalf("LoadWorkflowPromptActivityHistory: %v", err)
	}

	if len(loaded.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded.Events))
	}
	if loaded.Events[0].Surface != WorkflowPromptActivitySurfacePulseMCP ||
		loaded.Events[0].PromptName != "pulse_operations_loop" {
		t.Fatalf("loaded event mismatch: %+v", loaded.Events[0])
	}
}

func TestRecordWorkflowPromptActivityPrunesAndNormalizes(t *testing.T) {
	dir := t.TempDir()
	cp := NewConfigPersistence(dir)

	now := time.Now().UTC()
	if err := cp.SaveWorkflowPromptActivityHistory([]WorkflowPromptActivityRecord{
		{
			Timestamp:  now.Add(-(workflowPromptActivityHistoryRetention + time.Hour)),
			Surface:    WorkflowPromptActivitySurfacePulseMCP,
			PromptName: "pulse_operations_loop",
		},
	}); err != nil {
		t.Fatalf("SaveWorkflowPromptActivityHistory: %v", err)
	}

	if err := cp.RecordWorkflowPromptActivity(WorkflowPromptActivityRecord{
		Timestamp:  now,
		PromptName: " pulse_operations_loop ",
	}); err != nil {
		t.Fatalf("RecordWorkflowPromptActivity: %v", err)
	}

	loaded, err := cp.LoadWorkflowPromptActivityHistory()
	if err != nil {
		t.Fatalf("LoadWorkflowPromptActivityHistory: %v", err)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("expected only recent event after prune, got %d: %+v", len(loaded.Events), loaded.Events)
	}
	if loaded.Events[0].Surface != WorkflowPromptActivitySurfacePulseAssistant ||
		loaded.Events[0].PromptName != "pulse_operations_loop" {
		t.Fatalf("recorded event mismatch: %+v", loaded.Events[0])
	}
}
