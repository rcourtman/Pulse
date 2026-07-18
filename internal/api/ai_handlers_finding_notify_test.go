package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAISettingsHandler_UpdateSettingsPersistsFindingNotificationPolicy(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		PatrolFindingNotificationsEnabled: ptr(false),
		PatrolFindingNotifyMinSeverity:    ptr("Critical"),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.False(t, resp.PatrolFindingNotificationsEnabled)
	require.Equal(t, "critical", resp.PatrolFindingNotifyMinSeverity)

	saved, err := persistence.LoadAIConfig()
	require.NoError(t, err)
	require.False(t, saved.PatrolFindingNotificationsEnabled)
	require.Equal(t, "critical", saved.PatrolFindingNotifyMinSeverity)
}

func TestAISettingsHandler_UpdateSettingsRejectsInvalidFindingNotifySeverity(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	body, err := json.Marshal(AISettingsUpdateRequest{
		PatrolFindingNotifyMinSeverity: ptr("emergency"),
	})
	require.NoError(t, err)

	req := newLoopbackRequest(http.MethodPut, "/api/settings/ai/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.HandleUpdateAISettings(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "patrol_finding_notify_min_severity")
}

func TestAISettingsHandler_GetSettingsReportsFindingNotificationDefaults(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	cfg := &config.Config{DataPath: tmp}
	persistence := config.NewConfigPersistence(tmp)
	handler := newTestAISettingsHandler(cfg, persistence, nil)

	req := newLoopbackRequest(http.MethodGet, "/api/settings/ai", nil)
	rec := httptest.NewRecorder()
	handler.HandleGetAISettings(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp AISettingsResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.True(t, resp.PatrolFindingNotificationsEnabled)
	require.Equal(t, "warning", resp.PatrolFindingNotifyMinSeverity)
}
