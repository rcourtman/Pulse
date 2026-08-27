package alerting

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeadManConfigurationHandlersKeepCredentialSecretAndUpdatesAtomic(t *testing.T) {
	const secretURL = "https://watchdog.example.com/ping/credential-token"

	t.Run("GET masks configured URL", func(t *testing.T) {
		monitor := new(MockAlertMonitor)
		monitor.On("DeadManConfig").Return(notifications.DeadManConfig{PingURL: secretURL}).Once()
		handler := NewAlertHandlers(nil, monitor, nil)
		response := httptest.NewRecorder()

		handler.GetDeadManConfig(response, httptest.NewRequest(http.MethodGet, "/api/alerts/deadman/config", nil))

		assert.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), secretURL)
		assert.NotContains(t, response.Body.String(), "credential-token")
		assert.Contains(t, response.Body.String(), deadManRedactedPingURL)
		assert.Contains(t, response.Body.String(), `"configured":true`)
		monitor.AssertExpectations(t)
	})

	t.Run("redacted sentinel preserves stored URL", func(t *testing.T) {
		monitor := new(MockAlertMonitor)
		monitor.On("DeadManConfig").Return(notifications.DeadManConfig{PingURL: secretURL}).Once()
		monitor.On("UpdateDeadManConfig", notifications.DeadManConfig{PingURL: secretURL}).Return(nil).Once()
		handler := NewAlertHandlers(nil, monitor, nil)
		response := httptest.NewRecorder()

		handler.UpdateDeadManConfig(response, httptest.NewRequest(
			http.MethodPut,
			"/api/alerts/deadman/config",
			strings.NewReader(`{"pingUrl":"***REDACTED***"}`),
		))

		assert.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), secretURL)
		monitor.AssertExpectations(t)
	})

	t.Run("empty URL explicitly removes destination", func(t *testing.T) {
		monitor := new(MockAlertMonitor)
		monitor.On("UpdateDeadManConfig", notifications.DeadManConfig{}).Return(nil).Once()
		handler := NewAlertHandlers(nil, monitor, nil)
		response := httptest.NewRecorder()

		handler.UpdateDeadManConfig(response, httptest.NewRequest(
			http.MethodPut,
			"/api/alerts/deadman/config",
			strings.NewReader(`{"pingUrl":""}`),
		))

		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `"configured":false`)
		monitor.AssertExpectations(t)
	})

	t.Run("invalid same-host URL never reaches persistence", func(t *testing.T) {
		monitor := new(MockAlertMonitor)
		handler := NewAlertHandlers(nil, monitor, nil)
		response := httptest.NewRecorder()

		handler.UpdateDeadManConfig(response, httptest.NewRequest(
			http.MethodPut,
			"/api/alerts/deadman/config",
			strings.NewReader(`{"pingUrl":"http://127.0.0.1/ping/token"}`),
		))

		assert.Equal(t, http.StatusBadRequest, response.Code)
		monitor.AssertNotCalled(t, "UpdateDeadManConfig", mock.Anything)
	})

	t.Run("persistence errors do not echo URL", func(t *testing.T) {
		monitor := new(MockAlertMonitor)
		monitor.On("UpdateDeadManConfig", notifications.DeadManConfig{PingURL: secretURL}).Return(errors.New("disk full near credential-token")).Once()
		handler := NewAlertHandlers(nil, monitor, nil)
		response := httptest.NewRecorder()

		handler.UpdateDeadManConfig(response, httptest.NewRequest(
			http.MethodPut,
			"/api/alerts/deadman/config",
			strings.NewReader(`{"pingUrl":"https://watchdog.example.com/ping/credential-token"}`),
		))

		assert.Equal(t, http.StatusInternalServerError, response.Code)
		assert.NotContains(t, response.Body.String(), "credential-token")
		monitor.AssertExpectations(t)
	})
}

func TestDeadManStatusHandlerReturnsOnlyOperationalReadModel(t *testing.T) {
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	monitor := new(MockAlertMonitor)
	monitor.On("DeadManStatus").Return(monitoring.DeadManStatus{
		Configured:             true,
		State:                  "healthy",
		HeartbeatIntervalSecs:  60,
		RecommendedGraceSecs:   180,
		LastMonitoringProgress: &now,
		LastSuccessAt:          &now,
	}).Once()
	handler := NewAlertHandlers(nil, monitor, nil)
	response := httptest.NewRecorder()

	handler.GetDeadManStatus(response, httptest.NewRequest(http.MethodGet, "/api/alerts/deadman/status", nil))

	assert.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"state":"healthy"`)
	assert.NotContains(t, response.Body.String(), "pingUrl")
	assert.NotContains(t, response.Body.String(), "fingerprint")
	monitor.AssertExpectations(t)
}
