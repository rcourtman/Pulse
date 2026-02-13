package updates

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewUpdateHistory(t *testing.T) {
	t.Run("creates directory if it doesn't exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataDir := filepath.Join(tmpDir, "subdir", "nested")

		h, err := NewUpdateHistory(dataDir)
		if err != nil {
			t.Fatalf("NewUpdateHistory() error = %v", err)
		}
		if h == nil {
			t.Fatal("NewUpdateHistory() returned nil")
		}

		// Directory should exist
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			t.Error("Expected directory to be created")
		}
	})

	t.Run("initializes with empty cache", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, err := NewUpdateHistory(tmpDir)
		if err != nil {
			t.Fatalf("NewUpdateHistory() error = %v", err)
		}

		entries := h.ListEntries(HistoryFilter{})
		if len(entries) != 0 {
			t.Errorf("Expected empty cache, got %d entries", len(entries))
		}
	})

	t.Run("loads existing entries from file", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "update-history.jsonl")

		// Write some entries
		entry := UpdateHistoryEntry{
			EventID:     "test-event-1",
			Timestamp:   time.Now(),
			Action:      "update",
			VersionFrom: "1.0.0",
			VersionTo:   "1.1.0",
			Status:      StatusSuccess,
		}
		data, _ := json.Marshal(entry)
		if err := os.WriteFile(logPath, append(data, '\n'), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		h, err := NewUpdateHistory(tmpDir)
		if err != nil {
			t.Fatalf("NewUpdateHistory() error = %v", err)
		}

		entries := h.ListEntries(HistoryFilter{})
		if len(entries) != 1 {
			t.Errorf("Expected 1 entry, got %d", len(entries))
		}
		if entries[0].EventID != "test-event-1" {
			t.Errorf("Expected event ID 'test-event-1', got %q", entries[0].EventID)
		}
	})
}

func TestUpdateHistory_CreateEntry(t *testing.T) {
	ctx := context.Background()

	t.Run("creates entry with generated event ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		entry := UpdateHistoryEntry{
			Action:      "update",
			VersionFrom: "1.0.0",
			VersionTo:   "1.1.0",
			Status:      StatusInProgress,
		}

		eventID, err := h.CreateEntry(ctx, entry)
		if err != nil {
			t.Fatalf("CreateEntry() error = %v", err)
		}
		if eventID == "" {
			t.Error("Expected non-empty event ID")
		}

		// Verify entry is in cache
		retrieved, err := h.GetEntry(eventID)
		if err != nil {
			t.Fatalf("GetEntry() error = %v", err)
		}
		if retrieved.VersionFrom != "1.0.0" {
			t.Errorf("VersionFrom = %q, want '1.0.0'", retrieved.VersionFrom)
		}
	})

	t.Run("uses provided event ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		entry := UpdateHistoryEntry{
			EventID: "my-custom-id",
			Action:  "update",
			Status:  StatusInProgress,
		}

		eventID, err := h.CreateEntry(ctx, entry)
		if err != nil {
			t.Fatalf("CreateEntry() error = %v", err)
		}
		if eventID != "my-custom-id" {
			t.Errorf("EventID = %q, want 'my-custom-id'", eventID)
		}
	})

	t.Run("sets timestamp if not provided", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		before := time.Now()
		entry := UpdateHistoryEntry{
			Action: "update",
			Status: StatusInProgress,
		}

		eventID, err := h.CreateEntry(ctx, entry)
		if err != nil {
			t.Fatalf("CreateEntry() error = %v", err)
		}
		after := time.Now()

		retrieved, _ := h.GetEntry(eventID)
		if retrieved.Timestamp.Before(before) || retrieved.Timestamp.After(after) {
			t.Errorf("Timestamp not set correctly: %v", retrieved.Timestamp)
		}
	})

	t.Run("preserves provided timestamp", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		customTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		entry := UpdateHistoryEntry{
			Timestamp: customTime,
			Action:    "update",
			Status:    StatusInProgress,
		}

		eventID, _ := h.CreateEntry(ctx, entry)
		retrieved, _ := h.GetEntry(eventID)

		if !retrieved.Timestamp.Equal(customTime) {
			t.Errorf("Timestamp = %v, want %v", retrieved.Timestamp, customTime)
		}
	})

	t.Run("persists to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		entry := UpdateHistoryEntry{
			EventID: "persist-test",
			Action:  "update",
			Status:  StatusSuccess,
		}

		_, err := h.CreateEntry(ctx, entry)
		if err != nil {
			t.Fatalf("CreateEntry() error = %v", err)
		}

		// Read file directly
		logPath := filepath.Join(tmpDir, "update-history.jsonl")
		data, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("Failed to read history file: %v", err)
		}

		if len(data) == 0 {
			t.Error("History file is empty")
		}

		// Parse and verify
		var parsed UpdateHistoryEntry
		if err := json.Unmarshal(data[:len(data)-1], &parsed); err != nil {
			t.Fatalf("Failed to parse persisted entry: %v", err)
		}
		if parsed.EventID != "persist-test" {
			t.Errorf("Persisted EventID = %q, want 'persist-test'", parsed.EventID)
		}
	})
}

func TestUpdateHistory_UpdateEntry(t *testing.T) {
	ctx := context.Background()

	t.Run("updates existing entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		// Create initial entry
		entry := UpdateHistoryEntry{
			Action: "update",
			Status: StatusInProgress,
		}
		eventID, _ := h.CreateEntry(ctx, entry)

		// Update it
		err := h.UpdateEntry(ctx, eventID, func(e *UpdateHistoryEntry) error {
			e.Status = StatusSuccess
			e.DurationMs = 5000
			return nil
		})
		if err != nil {
			t.Fatalf("UpdateEntry() error = %v", err)
		}

		// Verify update
		retrieved, _ := h.GetEntry(eventID)
		if retrieved.Status != StatusSuccess {
			t.Errorf("Status = %v, want %v", retrieved.Status, StatusSuccess)
		}
		if retrieved.DurationMs != 5000 {
			t.Errorf("DurationMs = %d, want 5000", retrieved.DurationMs)
		}
	})

	t.Run("returns error for non-existent entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		err := h.UpdateEntry(ctx, "non-existent", func(e *UpdateHistoryEntry) error {
			return nil
		})
		if err == nil {
			t.Error("Expected error for non-existent entry")
		}
	})

	t.Run("persists update to file", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		// Create and update entry
		entry := UpdateHistoryEntry{
			EventID: "update-persist-test",
			Action:  "update",
			Status:  StatusInProgress,
		}
		eventID, _ := h.CreateEntry(ctx, entry)

		h.UpdateEntry(ctx, eventID, func(e *UpdateHistoryEntry) error {
			e.Status = StatusFailed
			e.Error = &UpdateError{Message: "test error"}
			return nil
		})

		// Create new history instance to read from file
		h2, _ := NewUpdateHistory(tmpDir)
		retrieved, err := h2.GetEntry(eventID)
		if err != nil {
			t.Fatalf("Failed to get entry from new instance: %v", err)
		}
		if retrieved.Status != StatusFailed {
			t.Errorf("Persisted Status = %v, want %v", retrieved.Status, StatusFailed)
		}
		if retrieved.Error == nil || retrieved.Error.Message != "test error" {
			t.Errorf("Persisted Error not correct")
		}
	})
}

func TestUpdateHistory_GetEntry(t *testing.T) {
	ctx := context.Background()

	t.Run("returns entry by ID", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		entry := UpdateHistoryEntry{
			EventID:     "get-test",
			Action:      "update",
			VersionFrom: "1.0.0",
			VersionTo:   "2.0.0",
			Status:      StatusSuccess,
		}
		h.CreateEntry(ctx, entry)

		retrieved, err := h.GetEntry("get-test")
		if err != nil {
			t.Fatalf("GetEntry() error = %v", err)
		}
		if retrieved.VersionFrom != "1.0.0" {
			t.Errorf("VersionFrom = %q, want '1.0.0'", retrieved.VersionFrom)
		}
		if retrieved.VersionTo != "2.0.0" {
			t.Errorf("VersionTo = %q, want '2.0.0'", retrieved.VersionTo)
		}
	})

	t.Run("returns error for non-existent entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		_, err := h.GetEntry("non-existent")
		if err == nil {
			t.Error("Expected error for non-existent entry")
		}
	})

	t.Run("returns defensive copy", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{
			EventID: "immutable",
			Status:  StatusSuccess,
			Error:   &UpdateError{Message: "original"},
		})

		entry, err := h.GetEntry("immutable")
		if err != nil {
			t.Fatalf("GetEntry() error = %v", err)
		}
		entry.Status = StatusFailed
		entry.Error.Message = "changed"

		again, err := h.GetEntry("immutable")
		if err != nil {
			t.Fatalf("GetEntry() error = %v", err)
		}
		if again.Status != StatusSuccess {
			t.Fatalf("stored status mutated via returned pointer: %s", again.Status)
		}
		if again.Error == nil || again.Error.Message != "original" {
			t.Fatalf("stored error mutated via returned pointer: %+v", again.Error)
		}
	})
}

func TestUpdateHistory_ListEntries(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all entries in reverse order", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		// Create entries
		for i := 1; i <= 3; i++ {
			entry := UpdateHistoryEntry{
				EventID: "event-" + string(rune('0'+i)),
				Action:  "update",
				Status:  StatusSuccess,
			}
			h.CreateEntry(ctx, entry)
		}

		entries := h.ListEntries(HistoryFilter{})
		if len(entries) != 3 {
			t.Fatalf("Expected 3 entries, got %d", len(entries))
		}

		// Should be in reverse order (newest first)
		if entries[0].EventID != "event-3" {
			t.Errorf("First entry should be event-3, got %s", entries[0].EventID)
		}
		if entries[2].EventID != "event-1" {
			t.Errorf("Last entry should be event-1, got %s", entries[2].EventID)
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		// Create entries with different statuses
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", Status: StatusSuccess})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", Status: StatusFailed})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e3", Status: StatusSuccess})

		entries := h.ListEntries(HistoryFilter{Status: StatusSuccess})
		if len(entries) != 2 {
			t.Errorf("Expected 2 success entries, got %d", len(entries))
		}
		for _, e := range entries {
			if e.Status != StatusSuccess {
				t.Errorf("Entry %s has status %v, want %v", e.EventID, e.Status, StatusSuccess)
			}
		}
	})

	t.Run("filters by action", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", Action: "update"})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", Action: "rollback"})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e3", Action: "update"})

		entries := h.ListEntries(HistoryFilter{Action: "rollback"})
		if len(entries) != 1 {
			t.Errorf("Expected 1 rollback entry, got %d", len(entries))
		}
		if entries[0].EventID != "e2" {
			t.Errorf("Expected e2, got %s", entries[0].EventID)
		}
	})

	t.Run("filters by deployment type", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", DeploymentType: "docker"})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", DeploymentType: "bare-metal"})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e3", DeploymentType: "docker"})

		entries := h.ListEntries(HistoryFilter{DeploymentType: "docker"})
		if len(entries) != 2 {
			t.Errorf("Expected 2 docker entries, got %d", len(entries))
		}
	})

	t.Run("respects limit", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		for i := 0; i < 10; i++ {
			h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "event"})
		}

		entries := h.ListEntries(HistoryFilter{Limit: 5})
		if len(entries) != 5 {
			t.Errorf("Expected 5 entries with limit, got %d", len(entries))
		}
	})

	t.Run("combines filters", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", Action: "update", Status: StatusSuccess})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", Action: "update", Status: StatusFailed})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e3", Action: "rollback", Status: StatusSuccess})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e4", Action: "update", Status: StatusSuccess})

		entries := h.ListEntries(HistoryFilter{Action: "update", Status: StatusSuccess})
		if len(entries) != 2 {
			t.Errorf("Expected 2 entries, got %d", len(entries))
		}
	})
}

func TestUpdateHistory_GetLatestSuccessful(t *testing.T) {
	ctx := context.Background()

	t.Run("returns latest successful entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", Status: StatusSuccess})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", Status: StatusFailed})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e3", Status: StatusSuccess})

		entry, err := h.GetLatestSuccessful()
		if err != nil {
			t.Fatalf("GetLatestSuccessful() error = %v", err)
		}
		if entry.EventID != "e3" {
			t.Errorf("Expected e3, got %s", entry.EventID)
		}
	})

	t.Run("returns error when no successful entries", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", Status: StatusFailed})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", Status: StatusInProgress})

		_, err := h.GetLatestSuccessful()
		if err == nil {
			t.Error("Expected error when no successful entries")
		}
	})

	t.Run("returns error when cache is empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		_, err := h.GetLatestSuccessful()
		if err == nil {
			t.Error("Expected error when cache is empty")
		}
	})

	t.Run("returns defensive copy", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e1", Status: StatusSuccess})
		h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "e2", Status: StatusSuccess})

		entry, err := h.GetLatestSuccessful()
		if err != nil {
			t.Fatalf("GetLatestSuccessful() error = %v", err)
		}
		entry.EventID = "changed"

		again, err := h.GetLatestSuccessful()
		if err != nil {
			t.Fatalf("GetLatestSuccessful() error = %v", err)
		}
		if again.EventID != "e2" {
			t.Fatalf("stored latest-success entry mutated via returned pointer: %s", again.EventID)
		}
	})
}

func TestUpdateHistory_CacheManagement(t *testing.T) {
	ctx := context.Background()

	t.Run("trims cache when exceeding max size", func(t *testing.T) {
		tmpDir := t.TempDir()
		h, _ := NewUpdateHistory(tmpDir)

		// h.maxCache is 100, add more than that
		for i := 0; i < 110; i++ {
			h.CreateEntry(ctx, UpdateHistoryEntry{EventID: "event"})
		}

		entries := h.ListEntries(HistoryFilter{})
		if len(entries) > 100 {
			t.Errorf("Cache should be trimmed to 100, got %d", len(entries))
		}
	})
}

func TestUpdateHistory_LoadCache(t *testing.T) {
	t.Run("handles malformed JSON lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "update-history.jsonl")

		// Write valid entry followed by invalid line
		validEntry := UpdateHistoryEntry{EventID: "valid", Status: StatusSuccess}
		validData, _ := json.Marshal(validEntry)
		content := string(validData) + "\n{invalid json}\n"
		os.WriteFile(logPath, []byte(content), 0644)

		// Should load without error, skipping invalid line
		h, err := NewUpdateHistory(tmpDir)
		if err != nil {
			t.Fatalf("NewUpdateHistory() error = %v", err)
		}

		entries := h.ListEntries(HistoryFilter{})
		if len(entries) != 1 {
			t.Errorf("Expected 1 valid entry, got %d", len(entries))
		}
	})

	t.Run("handles empty lines", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "update-history.jsonl")

		entry := UpdateHistoryEntry{EventID: "test", Status: StatusSuccess}
		data, _ := json.Marshal(entry)
		content := "\n" + string(data) + "\n\n"
		os.WriteFile(logPath, []byte(content), 0644)

		h, _ := NewUpdateHistory(tmpDir)
		entries := h.ListEntries(HistoryFilter{})
		if len(entries) != 1 {
			t.Errorf("Expected 1 entry, got %d", len(entries))
		}
	})

	t.Run("trims entries exceeding max cache on load", func(t *testing.T) {
		tmpDir := t.TempDir()
		logPath := filepath.Join(tmpDir, "update-history.jsonl")

		// Write 150 entries (more than maxCache of 100)
		var content string
		for i := 0; i < 150; i++ {
			entry := UpdateHistoryEntry{EventID: "event", Status: StatusSuccess}
			data, _ := json.Marshal(entry)
			content += string(data) + "\n"
		}
		os.WriteFile(logPath, []byte(content), 0644)

		h, _ := NewUpdateHistory(tmpDir)
		entries := h.ListEntries(HistoryFilter{})
		if len(entries) != 100 {
			t.Errorf("Expected 100 entries after load, got %d", len(entries))
		}
	})
}

func TestUpdateHistoryEntry_Fields(t *testing.T) {
	t.Run("all fields serialize correctly", func(t *testing.T) {
		entry := UpdateHistoryEntry{
			EventID:        "test-123",
			Timestamp:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			Action:         "update",
			Channel:        "stable",
			VersionFrom:    "1.0.0",
			VersionTo:      "2.0.0",
			DeploymentType: "docker",
			InitiatedBy:    InitiatedByUser,
			InitiatedVia:   InitiatedViaUI,
			Status:         StatusSuccess,
			DurationMs:     5000,
			BackupPath:     "/backups/backup.tar.gz",
			LogPath:        "/logs/update.log",
			Error:          nil,
			DownloadBytes:  1048576,
			RelatedEventID: "related-456",
			Notes:          "Test update",
		}

		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		var parsed UpdateHistoryEntry
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		if parsed.EventID != entry.EventID {
			t.Errorf("EventID mismatch")
		}
		if parsed.Action != entry.Action {
			t.Errorf("Action mismatch")
		}
		if parsed.Status != entry.Status {
			t.Errorf("Status mismatch")
		}
		if parsed.DurationMs != entry.DurationMs {
			t.Errorf("DurationMs mismatch")
		}
		if parsed.DownloadBytes != entry.DownloadBytes {
			t.Errorf("DownloadBytes mismatch")
		}
	})

	t.Run("error field serializes correctly", func(t *testing.T) {
		entry := UpdateHistoryEntry{
			EventID: "err-test",
			Status:  StatusFailed,
			Error: &UpdateError{
				Message: "download failed",
				Code:    "DOWNLOAD_ERROR",
				Details: "connection timeout",
			},
		}

		data, _ := json.Marshal(entry)
		var parsed UpdateHistoryEntry
		json.Unmarshal(data, &parsed)

		if parsed.Error == nil {
			t.Fatal("Error should not be nil")
		}
		if parsed.Error.Message != "download failed" {
			t.Errorf("Error.Message = %q, want 'download failed'", parsed.Error.Message)
		}
		if parsed.Error.Code != "DOWNLOAD_ERROR" {
			t.Errorf("Error.Code = %q, want 'DOWNLOAD_ERROR'", parsed.Error.Code)
		}
	})

	t.Run("optional fields omit when empty", func(t *testing.T) {
		entry := UpdateHistoryEntry{
			EventID: "minimal",
			Status:  StatusSuccess,
		}

		data, _ := json.Marshal(entry)
		str := string(data)

		// These optional fields should not appear
		if contains(str, "backup_path") {
			t.Error("backup_path should be omitted when empty")
		}
		if contains(str, "log_path") {
			t.Error("log_path should be omitted when empty")
		}
		if contains(str, "error") {
			t.Error("error should be omitted when nil")
		}
	})
}

func TestConstants(t *testing.T) {
	t.Run("UpdateAction constants", func(t *testing.T) {
		if "update" != "update" {
			t.Errorf("\"update\" = %q, want 'update'", "update")
		}
		if "rollback" != "rollback" {
			t.Errorf("\"rollback\" = %q, want 'rollback'", "rollback")
		}
	})

	t.Run("UpdateStatusType constants", func(t *testing.T) {
		if StatusInProgress != "in_progress" {
			t.Errorf("StatusInProgress = %q, want 'in_progress'", StatusInProgress)
		}
		if StatusSuccess != "success" {
			t.Errorf("StatusSuccess = %q, want 'success'", StatusSuccess)
		}
		if StatusFailed != "failed" {
			t.Errorf("StatusFailed = %q, want 'failed'", StatusFailed)
		}
		if StatusRolledBack != "rolled_back" {
			t.Errorf("StatusRolledBack = %q, want 'rolled_back'", StatusRolledBack)
		}
		if StatusCancelled != "cancelled" {
			t.Errorf("StatusCancelled = %q, want 'cancelled'", StatusCancelled)
		}
	})

	t.Run("InitiatedBy constants", func(t *testing.T) {
		if InitiatedByUser != "user" {
			t.Errorf("InitiatedByUser = %q, want 'user'", InitiatedByUser)
		}
		if InitiatedByAuto != "auto" {
			t.Errorf("InitiatedByAuto = %q, want 'auto'", InitiatedByAuto)
		}
		if InitiatedByAPI != "api" {
			t.Errorf("InitiatedByAPI = %q, want 'api'", InitiatedByAPI)
		}
	})

	t.Run("InitiatedVia constants", func(t *testing.T) {
		if InitiatedViaUI != "ui" {
			t.Errorf("InitiatedViaUI = %q, want 'ui'", InitiatedViaUI)
		}
		if InitiatedViaAPI != "api" {
			t.Errorf("InitiatedViaAPI = %q, want 'api'", InitiatedViaAPI)
		}
		if InitiatedViaCLI != "cli" {
			t.Errorf("InitiatedViaCLI = %q, want 'cli'", InitiatedViaCLI)
		}
		if InitiatedViaScript != "script" {
			t.Errorf("InitiatedViaScript = %q, want 'script'", InitiatedViaScript)
		}
		if InitiatedViaWebhook != "webhook" {
			t.Errorf("InitiatedViaWebhook = %q, want 'webhook'", InitiatedViaWebhook)
		}
	})
}

func TestHistoryFilter(t *testing.T) {
	t.Run("zero value filter matches all", func(t *testing.T) {
		filter := HistoryFilter{}
		if filter.Status != "" {
			t.Error("Status should be empty by default")
		}
		if filter.Action != "" {
			t.Error("Action should be empty by default")
		}
		if filter.DeploymentType != "" {
			t.Error("DeploymentType should be empty by default")
		}
		if filter.Limit != 0 {
			t.Error("Limit should be 0 by default")
		}
	})
}

func TestUpdateError(t *testing.T) {
	t.Run("all fields serialize", func(t *testing.T) {
		err := UpdateError{
			Message: "Something went wrong",
			Code:    "ERR_001",
			Details: "Additional details",
		}

		data, _ := json.Marshal(err)
		var parsed UpdateError
		json.Unmarshal(data, &parsed)

		if parsed.Message != err.Message {
			t.Errorf("Message mismatch")
		}
		if parsed.Code != err.Code {
			t.Errorf("Code mismatch")
		}
		if parsed.Details != err.Details {
			t.Errorf("Details mismatch")
		}
	})

	t.Run("optional fields omit when empty", func(t *testing.T) {
		err := UpdateError{
			Message: "Error message",
		}

		data, _ := json.Marshal(err)
		str := string(data)

		if contains(str, "code") {
			t.Error("code should be omitted when empty")
		}
		if contains(str, "details") {
			t.Error("details should be omitted when empty")
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
