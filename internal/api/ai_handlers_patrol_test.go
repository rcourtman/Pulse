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
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
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

	}

	// Step 2: Verify persistence by fetching settings again
	{
		req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
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

func TestAISettingsHandler_DiscoveryIntervalPersistsExplicitSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		intervalHours int
	}{
		{name: "six hour automatic scan", intervalHours: 6},
		{name: "manual only", intervalHours: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			cfg := &config.Config{DataPath: tmp}
			persistence := config.NewConfigPersistence(tmp)
			handler := newTestAISettingsHandler(cfg, persistence, nil)

			body, _ := json.Marshal(AISettingsUpdateRequest{
				DiscoveryEnabled:       ptr(true),
				DiscoveryIntervalHours: ptr(tt.intervalHours),
			})
			req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
			rec := httptest.NewRecorder()
			handler.HandleUpdateAISettings(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("PUT status = %d, body=%s", rec.Code, rec.Body.String())
			}

			var updateResp AISettingsResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &updateResp); err != nil {
				t.Fatalf("decode update: %v", err)
			}
			if !updateResp.DiscoveryEnabled {
				t.Fatalf("expected DiscoveryEnabled=true in update response")
			}
			if updateResp.DiscoveryIntervalHours != tt.intervalHours {
				t.Fatalf("expected update DiscoveryIntervalHours=%d, got %d", tt.intervalHours, updateResp.DiscoveryIntervalHours)
			}

			saved, err := persistence.LoadAIConfig()
			if err != nil {
				t.Fatalf("load saved AI config: %v", err)
			}
			if saved.DiscoveryIntervalHours != tt.intervalHours {
				t.Fatalf("expected persisted DiscoveryIntervalHours=%d, got %d", tt.intervalHours, saved.DiscoveryIntervalHours)
			}

			getReq := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
			getRec := httptest.NewRecorder()
			handler.HandleGetAISettings(getRec, getReq)
			if getRec.Code != http.StatusOK {
				t.Fatalf("GET status = %d, body=%s", getRec.Code, getRec.Body.String())
			}

			var getResp AISettingsResponse
			if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
				t.Fatalf("decode get: %v", err)
			}
			if getResp.DiscoveryIntervalHours != tt.intervalHours {
				t.Fatalf("expected GET DiscoveryIntervalHours=%d, got %d", tt.intervalHours, getResp.DiscoveryIntervalHours)
			}
		})
	}
}

func TestAISettingsHandler_PatrolEnabled_ResponseReflectsToggleImmediately(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	// First disable patrol explicitly.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			PatrolEnabled: ptr(false),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("disable PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("disable decode: %v", err)
		}
		if resp.PatrolEnabled {
			t.Fatalf("expected PatrolEnabled=false after disable, got true")
		}
	}

	// Then enable patrol; response must immediately reflect enabled=true.
	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			PatrolEnabled: ptr(true),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("enable PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("enable decode: %v", err)
		}
		if !resp.PatrolEnabled {
			t.Fatalf("expected PatrolEnabled=true in update response, got false (would cause UI desync)")
		}
		if resp.PatrolIntervalMinutes <= 0 {
			t.Fatalf("expected patrol interval to be set when enabling, got %d", resp.PatrolIntervalMinutes)
		}
	}
}

func TestAISettingsHandler_PatrolTriggerSettings_SplitAndLegacyCompatibility(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			PatrolAlertTriggersEnabled:   ptr(false),
			PatrolAnomalyTriggersEnabled: ptr(true),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("split PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("split decode: %v", err)
		}
		if resp.PatrolAlertTriggersEnabled {
			t.Fatal("expected alert-triggered patrols to be disabled")
		}
		if !resp.PatrolAnomalyTriggersEnabled {
			t.Fatal("expected anomaly-triggered patrols to stay enabled")
		}
		if !resp.PatrolEventTriggersEnabled {
			t.Fatal("expected legacy aggregate toggle to remain true while one trigger source stays enabled")
		}
	}

	{
		body, _ := json.Marshal(AISettingsUpdateRequest{
			PatrolEventTriggersEnabled: ptr(false),
		})
		req := newLoopbackRequest(http.MethodPut, "/api/settings/ai", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		handler.HandleUpdateAISettings(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("legacy PUT status = %d, body=%s", rec.Code, rec.Body.String())
		}

		var resp AISettingsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("legacy decode: %v", err)
		}
		if resp.PatrolAlertTriggersEnabled || resp.PatrolAnomalyTriggersEnabled || resp.PatrolEventTriggersEnabled {
			t.Fatalf("expected legacy aggregate toggle to disable both scoped trigger sources, got %+v", resp)
		}
	}
}
