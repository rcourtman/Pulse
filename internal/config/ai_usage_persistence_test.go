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

// History-read day markers are one per UTC day and sit outside the record
// cap, so a busy agent's per-request records cannot erase the days it read
// history on.
func TestExternalAgentHistoryReadMarkersAreDailyAndSurviveTheCap(t *testing.T) {
	cp := NewConfigPersistence(t.TempDir())
	// Midday, so the same-day pair below cannot straddle midnight.
	now := time.Now().UTC().Truncate(24 * time.Hour).Add(12 * time.Hour)
	marker := func(at time.Time) ExternalAgentActivityRecord {
		return ExternalAgentActivityRecord{Timestamp: at, Surface: ExternalAgentActivitySurfaceAgentAPI, Activity: ExternalAgentActivityHistoryRead}
	}
	for _, at := range []time.Time{now.Add(-48 * time.Hour), now.Add(-24 * time.Hour), now, now.Add(time.Second)} {
		if err := cp.RecordExternalAgentActivity(marker(at)); err != nil {
			t.Fatalf("RecordExternalAgentActivity: %v", err)
		}
	}
	for i := 0; i < maxExternalAgentActivityHistoryRecords+50; i++ {
		if err := cp.RecordExternalAgentActivity(ExternalAgentActivityRecord{Timestamp: now.Add(time.Duration(i) * time.Millisecond), Surface: ExternalAgentActivitySurfaceAgentAPI, Activity: ExternalAgentActivityFleetContext}); err != nil {
			t.Fatalf("RecordExternalAgentActivity: %v", err)
		}
	}
	loaded, err := cp.LoadExternalAgentActivityHistory()
	if err != nil {
		t.Fatalf("LoadExternalAgentActivityHistory: %v", err)
	}
	markers, others := 0, 0
	for _, event := range loaded.Events {
		if event.Activity == ExternalAgentActivityHistoryRead {
			markers++
		} else {
			others++
		}
	}
	if markers != 3 || others != maxExternalAgentActivityHistoryRecords {
		t.Fatalf("markers=%d others=%d; want 3 daily markers kept beside %d capped records", markers, others, maxExternalAgentActivityHistoryRecords)
	}
}
