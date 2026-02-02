package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

func TestAISettingsHandler_PatrolInterval_SpecificCheck(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)

	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// Step 1: Update settings to set patrol interval to 15 minutes explicitly
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			PatrolIntervalMinutes: ptr(15),
		})
		req := httptest.NewRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		// The API response should return 15 minutes, NOT 360 (6 hours)
		if resp.PatrolIntervalMinutes != 15 {
			t.Fatalf("expected PatrolIntervalMinutes=15, got %d. Did the migration logic override the user setting?", resp.PatrolIntervalMinutes)
		}

		// Check that the preset is cleared as expected when setting explicit minutes
		if resp.PatrolSchedulePreset != "" {
			t.Fatalf("expected PatrolSchedulePreset to be empty, got %q", resp.PatrolSchedulePreset)
		}
	}

	// Step 2: Verify persistence by fetching settings again
	{
		req := httptest.NewRequest(http.MethodGet, "/api/settings/ai", nil)
		rec := httptest.NewRecorder()
		handler.HandleGetAISettings(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}

		if resp.PatrolIntervalMinutes != 15 {
			t.Fatalf("expected persisted PatrolIntervalMinutes=15, got %d", resp.PatrolIntervalMinutes)
		}
	}
}
