package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestHandleResetAICostHistory_ServiceUnavailable(t *testing.T) {
	t.Parallel()

	handler := newTestAISettingsHandler(&config.Config{DataPath: t.TempDir()}, nil, nil)
	// Force legacyAIService to nil — the constructor creates a minimal
	// service even with nil persistence, but we want the nil-service path.
	handler.legacyAIService = nil

	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, rec.Code)
	}
}

func TestHandleResetAICostHistory_MethodPUT(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	handler := newTestAISettingsHandler(&config.Config{DataPath: tmp}, config.NewConfigPersistence(tmp), nil)

	req := httptest.NewRequest(http.MethodPut, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleResetAICostHistory_MethodDELETE(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	handler := newTestAISettingsHandler(&config.Config{DataPath: tmp}, config.NewConfigPersistence(tmp), nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d", http.StatusMethodNotAllowed, rec.Code)
	}
}

func TestHandleResetAICostHistory_BackupCreated(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Seed a usage history file so the backup path is exercised.
	events := []config.AIUsageEventRecord{
		{Provider: "openai", RequestModel: "gpt-4o-mini", UseCase: "chat", InputTokens: 10, OutputTokens: 5},
	}
	if err := persistence.SaveAIUsageHistory(events); err != nil {
		t.Fatalf("SaveAIUsageHistory: %v", err)
	}

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}

	backupFile, _ := resp["backup_file"].(string)
	if backupFile == "" {
		t.Fatalf("expected backup_file in response, got %v", resp)
	}
	if !strings.HasPrefix(backupFile, "ai_usage_history.json.bak-") {
		t.Fatalf("backup_file has unexpected prefix: %s", backupFile)
	}

	// Verify the backup file actually exists on disk.
	backupPath := filepath.Join(tmp, backupFile)
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file should exist at %s: %v", backupPath, err)
	}

	// The original file may be re-created empty by ClearCostHistory — that is fine.
	// What matters is the backup exists and the response is correct.
}

func TestHandleResetAICostHistory_NoUsageFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// No ai_usage_history.json exists — should succeed without backup.
	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("expected ok=true")
	}
	if _, exists := resp["backup_file"]; exists {
		t.Fatalf("expected no backup_file when no usage file exists, got %v", resp["backup_file"])
	}
}

func TestHandleResetAICostHistory_BackupRenameFail(t *testing.T) {
	t.Parallel()

	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	// Create the usage file.
	usagePath := filepath.Join(tmp, "ai_usage_history.json")
	if err := os.WriteFile(usagePath, []byte(`[{"provider":"test"}]`), 0644); err != nil {
		t.Fatalf("write usage file: %v", err)
	}

	// Make the directory read-only so os.Rename fails.
	if err := os.Chmod(tmp, 0555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() {
		os.Chmod(tmp, 0755) // restore so TempDir cleanup works
	})

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", nil)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d; body: %s", http.StatusInternalServerError, rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Failed to backup") {
		t.Fatalf("expected backup failure message, got: %s", rec.Body.String())
	}
}

func TestHandleResetAICostHistory_LargeBody(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// This endpoint takes no body — a large body should not cause issues
	// (handler ignores it). Verify the handler still succeeds.
	largeBody := strings.NewReader(strings.Repeat("x", 64*1024))
	req := httptest.NewRequest(http.MethodPost, "/api/ai/cost/reset", largeBody)
	rec := httptest.NewRecorder()
	handler.HandleResetAICostHistory(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}
