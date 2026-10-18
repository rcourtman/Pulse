package alerting

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Configuration responses intentionally let an authorised editor round-trip
// targets; routine logs must not duplicate their credentials or config keys.
func TestAppriseConfigurationLogsWithholdSecrets(t *testing.T) {
	const secret = "apprise-api-synthetic-secret"
	var captured bytes.Buffer
	original := log.Logger
	log.Logger = zerolog.New(zerolog.SyncWriter(&captured))
	defer func() { log.Logger = original }()
	cfg := notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP, ServerURL: "https://fixture/" + secret,
		ConfigKey: secret, APIKey: secret, CLIPath: secret, APIKeyHeader: secret, Targets: []string{"future://" + secret}}
	manager := new(MockNotificationManager)
	persistence := new(MockNotificationConfigPersistence)
	monitor := new(MockNotificationMonitor)
	manager.On("GetAppriseConfig").Return(cfg).Twice()
	manager.On("SetAppriseConfig", mock.MatchedBy(func(got notifications.AppriseConfig) bool {
		return got.ServerURL == cfg.ServerURL && got.ConfigKey == secret && got.APIKey == secret && got.Targets[0] == cfg.Targets[0]
	})).Return().Once()
	persistence.On("SaveAppriseConfig", mock.MatchedBy(func(got notifications.AppriseConfig) bool {
		return got.ServerURL == cfg.ServerURL && got.ConfigKey == secret && got.APIKey == secret && got.Targets[0] == cfg.Targets[0]
	})).Return(nil).Once()
	monitor.On("GetNotificationManager").Return(manager)
	monitor.On("GetConfigPersistence").Return(persistence)
	body, _ := json.Marshal(cfg)
	w := httptest.NewRecorder()
	NewNotificationHandlers(nil, monitor).UpdateAppriseConfig(w, httptest.NewRequest(http.MethodPost, "/api/notifications/apprise", bytes.NewReader(body)))
	if w.Code != http.StatusOK || strings.Contains(captured.String(), secret) {
		t.Fatal("configuration update failed or a synthetic credential reached its log")
	}
	if !strings.Contains(captured.String(), "hasConfigKey") || !strings.Contains(captured.String(), "targetCount") {
		t.Fatal("structured configuration diagnostics were lost")
	}
	var response appriseConfigResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.APIKey != "" || !response.HasAPIKey {
		t.Fatal("saved API-key response contract changed")
	}
	if response.ServerURL != cfg.ServerURL || response.ConfigKey != cfg.ConfigKey || len(response.Targets) != 1 || response.Targets[0] != cfg.Targets[0] {
		t.Fatal("authorised configuration round-trip changed")
	}
	manager.AssertExpectations(t)
	persistence.AssertExpectations(t)
}

// Exercise the real sender through the handler, not an assumed safe mock error.
func TestAppriseTestResponseWithholdsProviderSecrets(t *testing.T) {
	const secret = "apprise-api-synthetic-secret"
	var captured bytes.Buffer
	original := log.Logger
	log.Logger = zerolog.New(zerolog.SyncWriter(&captured))
	defer func() { log.Logger = original }()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, "provider-private-content ", secret, " ", r.URL.String())
	}))
	defer server.Close()
	manager := notifications.NewNotificationManagerWithDataDir("", t.TempDir())
	defer manager.Stop()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	monitor := new(MockNotificationMonitor)
	monitor.On("GetNotificationManager").Return(manager)
	body, _ := json.Marshal(map[string]any{"method": "apprise", "config": notifications.AppriseConfig{Enabled: true, Mode: notifications.AppriseModeHTTP,
		ServerURL: server.URL + "/" + secret, ConfigKey: secret, APIKey: secret}})
	w := httptest.NewRecorder()
	NewNotificationHandlers(nil, monitor).TestNotification(w, httptest.NewRequest(http.MethodPost, "/api/notifications/test", bytes.NewReader(body)))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "HTTP 401") {
		t.Fatal("structured HTTP failure was lost from the test result")
	}
	for _, forbidden := range []string{secret, "provider-private-content"} {
		if strings.Contains(w.Body.String(), forbidden) || strings.Contains(captured.String(), forbidden) {
			t.Error("test response or log exposed synthetic provider credentials/content")
		}
	}
}

func TestRedactSecretsFromURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// No secrets - should pass through unchanged
		{
			name:     "no secrets in URL",
			input:    "https://example.com/webhook",
			expected: "https://example.com/webhook",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "URL with unrelated query params",
			input:    "https://example.com/api?foo=bar&baz=qux",
			expected: "https://example.com/api?foo=bar&baz=qux",
		},

		// Telegram bot token patterns
		{
			name:     "telegram bot token with sendMessage",
			input:    "https://api.telegram.org/bot123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11/sendMessage",
			expected: "https://api.telegram.org/botREDACTED/sendMessage",
		},
		{
			name:     "telegram bot token no trailing path",
			input:    "https://api.telegram.org/bot123456:ABC-token",
			expected: "https://api.telegram.org/botREDACTED",
		},
		{
			name:     "telegram bot token with query string",
			input:    "https://api.telegram.org/bot123456:ABC-token?chat_id=123",
			expected: "https://api.telegram.org/botREDACTED?chat_id=123",
		},
		{
			name:     "telegram bot token with path and query",
			input:    "https://api.telegram.org/bot123456:token/sendMessage?chat_id=123",
			expected: "https://api.telegram.org/botREDACTED/sendMessage?chat_id=123",
		},

		// Query parameter secrets
		{
			name:     "token query param",
			input:    "https://example.com/webhook?token=secret123",
			expected: "https://example.com/webhook?token=REDACTED",
		},
		{
			name:     "apikey query param",
			input:    "https://example.com/api?apikey=xyz123",
			expected: "https://example.com/api?apikey=REDACTED",
		},
		{
			name:     "api_key query param with underscore",
			input:    "https://example.com/api?api_key=xyz123",
			expected: "https://example.com/api?api_key=REDACTED",
		},
		{
			name:     "key query param",
			input:    "https://example.com/api?key=mykey123",
			expected: "https://example.com/api?key=REDACTED",
		},
		{
			name:     "secret query param",
			input:    "https://example.com/api?secret=mysecret",
			expected: "https://example.com/api?secret=REDACTED",
		},
		{
			name:     "password query param",
			input:    "https://example.com/api?password=pass123",
			expected: "https://example.com/api?password=REDACTED",
		},

		// Multiple parameters
		{
			name:     "secret param with other params before",
			input:    "https://example.com/api?foo=bar&token=secret",
			expected: "https://example.com/api?foo=bar&token=REDACTED",
		},
		{
			name:     "secret param with other params after",
			input:    "https://example.com/api?token=secret&foo=bar",
			expected: "https://example.com/api?token=REDACTED&foo=bar",
		},
		{
			name:     "multiple different secret params",
			input:    "https://example.com/api?token=tok&apikey=key",
			expected: "https://example.com/api?token=REDACTED&apikey=REDACTED",
		},

		// Edge cases
		{
			name:     "secret param with fragment",
			input:    "https://example.com/api?token=secret#section",
			expected: "https://example.com/api?token=REDACTED#section",
		},
		{
			name:     "bot in path but not telegram pattern",
			input:    "https://example.com/robots.txt",
			expected: "https://example.com/robots.txt",
		},
		{
			name:     "combined telegram and query param secrets",
			input:    "https://api.telegram.org/bot123:token/send?token=abc",
			expected: "https://api.telegram.org/botREDACTED/send?token=REDACTED",
		},
		// Boundary checking - prefixed params should NOT be redacted
		{
			name:     "prefixed param name should not match",
			input:    "https://example.com/api?extra_token=abc&myapikey=xyz",
			expected: "https://example.com/api?extra_token=abc&myapikey=xyz",
		},
		{
			name:     "prefixed param with real sensitive param",
			input:    "https://example.com/api?extra_token=abc&token=secret",
			expected: "https://example.com/api?extra_token=abc&token=REDACTED",
		},
		{
			name:     "multiple prefixed params unchanged",
			input:    "https://example.com/api?mytoken=a&yourkey=b&thesecret=c",
			expected: "https://example.com/api?mytoken=a&yourkey=b&thesecret=c",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSecretsFromURL(tt.input)
			if result != tt.expected {
				t.Errorf("redactSecretsFromURL(%q)\ngot:  %q\nwant: %q", tt.input, result, tt.expected)
			}
		})
	}
}

type MockNotificationMonitor struct {
	mock.Mock
}

func (m *MockNotificationMonitor) GetNotificationManager() NotificationManager {
	args := m.Called()
	return args.Get(0).(NotificationManager)
}

func (m *MockNotificationMonitor) GetConfigPersistence() NotificationConfigPersistence {
	args := m.Called()
	return args.Get(0).(NotificationConfigPersistence)
}

type MockNotificationManager struct {
	mock.Mock
}

func (m *MockNotificationManager) GetEmailConfig() notifications.EmailConfig {
	args := m.Called()
	return args.Get(0).(notifications.EmailConfig)
}

func (m *MockNotificationManager) SetEmailConfig(cfg notifications.EmailConfig) {
	m.Called(cfg)
}

func (m *MockNotificationManager) GetAppriseConfig() notifications.AppriseConfig {
	args := m.Called()
	return args.Get(0).(notifications.AppriseConfig)
}

func (m *MockNotificationManager) SetAppriseConfig(cfg notifications.AppriseConfig) {
	m.Called(cfg)
}

func (m *MockNotificationManager) GetWebhooks() []notifications.WebhookConfig {
	args := m.Called()
	return args.Get(0).([]notifications.WebhookConfig)
}

func (m *MockNotificationManager) ValidateWebhookURL(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *MockNotificationManager) AddWebhook(w notifications.WebhookConfig) {
	m.Called(w)
}

func (m *MockNotificationManager) UpdateWebhook(id string, w notifications.WebhookConfig) error {
	args := m.Called(id, w)
	return args.Error(0)
}

func (m *MockNotificationManager) DeleteWebhook(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockNotificationManager) SendTestWebhook(w notifications.WebhookConfig) error {
	args := m.Called(w)
	return args.Error(0)
}

func (m *MockNotificationManager) SendTestNotificationWithConfig(method string, cfg *notifications.EmailConfig, nodeInfo *notifications.TestNodeInfo) error {
	args := m.Called(method, cfg, nodeInfo)
	return args.Error(0)
}

func (m *MockNotificationManager) SendTestAppriseWithConfig(cfg notifications.AppriseConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

func (m *MockNotificationManager) SendTestNotification(method string) error {
	args := m.Called(method)
	return args.Error(0)
}

func (m *MockNotificationManager) GetWebhookHistory() []notifications.WebhookDelivery {
	args := m.Called()
	return args.Get(0).([]notifications.WebhookDelivery)
}

func (m *MockNotificationManager) TestEnhancedWebhook(w notifications.EnhancedWebhookConfig) (int, string, error) {
	args := m.Called(w)
	return args.Int(0), args.String(1), args.Error(2)
}

func (m *MockNotificationManager) GetQueueStats() (map[string]int, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int), args.Error(1)
}

func (m *MockNotificationManager) GetTelemetryStats(since time.Time) (notifications.TelemetryStats, error) {
	args := m.Called(since)
	return args.Get(0).(notifications.TelemetryStats), args.Error(1)
}

func (m *MockNotificationManager) GetDeliveryLog(since time.Time, limit int) ([]notifications.DeliveryLogEntry, error) {
	args := m.Called(since, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]notifications.DeliveryLogEntry), args.Error(1)
}

func (m *MockNotificationManager) IsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

type MockNotificationConfigPersistence struct {
	mock.Mock
}

func (m *MockNotificationConfigPersistence) SaveEmailConfig(cfg notifications.EmailConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

func (m *MockNotificationConfigPersistence) SaveAppriseConfig(cfg notifications.AppriseConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

func (m *MockNotificationConfigPersistence) SaveWebhooks(w []notifications.WebhookConfig) error {
	args := m.Called(w)
	return args.Error(0)
}

func (m *MockNotificationConfigPersistence) IsEncryptionEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func TestNotificationHandlers(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)

	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)

	h := NewNotificationHandlers(nil, mockMonitor)

	t.Run("SetMonitor", func(t *testing.T) {
		h.SetMonitor(mockMonitor)
		// Should not panic and should replace the monitor
	})

	t.Run("GetEmailConfig", func(t *testing.T) {
		cfg := notifications.EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.com",
			Password: "password123",
		}
		mockManager.On("GetEmailConfig").Return(cfg).Once()

		req := httptest.NewRequest("GET", "/api/notifications/email", nil)
		w := httptest.NewRecorder()
		h.GetEmailConfig(w, req)

		assert.Equal(t, 200, w.Code)
		var resp notifications.EmailConfig
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "smtp.example.com", resp.SMTPHost)
		assert.Empty(t, resp.Password) // Should be redacted
	})

	t.Run("UpdateEmailConfig", func(t *testing.T) {
		cfg := notifications.EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.com",
			Password: "newpassword",
		}
		mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
		mockManager.On("SetEmailConfig", mock.Anything).Return().Once()
		mockPersistence.On("SaveEmailConfig", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(cfg)
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, 200, w.Code)
		mockManager.AssertExpectations(t)
		mockPersistence.AssertExpectations(t)
	})

	t.Run("UpdateEmailConfig_PreservesOmittedTagRouting", func(t *testing.T) {
		existing := notifications.EmailConfig{
			TagFilter:       []string{"customer:alpha", "critical"},
			TagMode:         "any",
			MinimumSeverity: "critical",
		}
		mockManager.On("GetEmailConfig").Return(existing).Once()
		mockManager.On("SetEmailConfig", mock.MatchedBy(func(cfg notifications.EmailConfig) bool {
			return assert.ObjectsAreEqual(
				[]string{"customer:alpha", "critical"},
				cfg.TagFilter,
			) && cfg.TagMode == "any" && cfg.MinimumSeverity == "critical"
		})).Return().Once()
		mockPersistence.On("SaveEmailConfig", mock.MatchedBy(func(cfg notifications.EmailConfig) bool {
			return assert.ObjectsAreEqual(
				[]string{"customer:alpha", "critical"},
				cfg.TagFilter,
			) && cfg.TagMode == "any" && cfg.MinimumSeverity == "critical"
		})).Return(nil).Once()

		req := httptest.NewRequest(
			"PUT",
			"/api/notifications/email",
			bytes.NewBufferString(`{"enabled":true}`),
		)
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("UpdateEmailConfig_ClearsTagRoutingExplicitly", func(t *testing.T) {
		existing := notifications.EmailConfig{
			TagFilter: []string{"customer:alpha"},
			TagMode:   "any",
		}
		mockManager.On("GetEmailConfig").Return(existing).Once()
		mockManager.On("SetEmailConfig", mock.MatchedBy(func(cfg notifications.EmailConfig) bool {
			return len(cfg.TagFilter) == 0 && cfg.TagMode == "all"
		})).Return().Once()
		mockPersistence.On("SaveEmailConfig", mock.MatchedBy(func(cfg notifications.EmailConfig) bool {
			return len(cfg.TagFilter) == 0 && cfg.TagMode == "all"
		})).Return(nil).Once()

		req := httptest.NewRequest(
			"PUT",
			"/api/notifications/email",
			bytes.NewBufferString(`{"enabled":true,"tagFilter":[],"tagFilterMode":"all"}`),
		)
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("UpdateEmailConfig_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader([]byte("{invalid}")))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("GetWebhooks", func(t *testing.T) {
		webhooks := []notifications.WebhookConfig{
			{
				ID:              "wh1",
				Name:            "Test Webhook",
				URL:             "https://example.com",
				Headers:         map[string]string{"Authorization": "Bearer token"},
				Mention:         "@everyone",
				TagFilter:       []string{"customer:alpha", "critical"},
				TagMode:         "any",
				MinimumSeverity: "critical",
			},
		}
		mockManager.On("GetWebhooks").Return(webhooks).Once()

		req := httptest.NewRequest("GET", "/api/notifications/webhooks", nil)
		w := httptest.NewRecorder()
		h.GetWebhooks(w, req)

		assert.Equal(t, 200, w.Code)
		var resp []map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, 1, len(resp))
		assert.Equal(t, "wh1", resp[0]["id"])
		assert.Equal(t, "@everyone", resp[0]["mention"])
		assert.Equal(t, []interface{}{"customer:alpha", "critical"}, resp[0]["tagFilter"])
		assert.Equal(t, "any", resp[0]["tagFilterMode"])
		assert.Equal(t, "critical", resp[0]["minimumSeverity"])
		headers := resp[0]["headers"].(map[string]interface{})
		assert.Equal(t, "***REDACTED***", headers["Authorization"])
	})

	t.Run("CreateWebhook", func(t *testing.T) {
		webhook := notifications.WebhookConfig{
			Name: "New Webhook",
			URL:  "https://example.com/new",
		}
		mockManager.On("ValidateWebhookURL", "https://example.com/new").Return(nil).Once()
		mockManager.On("AddWebhook", mock.Anything).Return().Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(webhook)
		req := httptest.NewRequest("POST", "/api/notifications/webhooks", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.CreateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("CreateWebhook_ValidationError", func(t *testing.T) {
		webhook := notifications.WebhookConfig{URL: "invalid"}
		mockManager.On("ValidateWebhookURL", "invalid").Return(fmt.Errorf("invalid url")).Once()
		body, _ := json.Marshal(webhook)
		req := httptest.NewRequest("POST", "/api/notifications/webhooks", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.CreateWebhook(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("GetNotificationHealth", func(t *testing.T) {
		stats := map[string]int{
			"pending": 1,
			"sending": 2,
			"sent":    10,
			"failed":  0,
			"dlq":     0,
		}
		mockManager.On("GetQueueStats").Return(stats, nil).Once()
		mockManager.On("GetTelemetryStats", mock.Anything).Return(
			notifications.TelemetryStats{},
			nil,
		).Once()
		mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		mockPersistence.On("IsEncryptionEnabled").Return(true).Once()

		req := httptest.NewRequest("GET", "/api/notifications/health", nil)
		w := httptest.NewRecorder()
		h.GetNotificationHealth(w, req)

		assert.Equal(t, 200, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		queue := resp["queue"].(map[string]interface{})
		assert.Equal(t, float64(1), queue["pending"])
		assert.Equal(t, true, queue["healthy"])
	})

	t.Run("GetAppriseConfig", func(t *testing.T) {
		cfg := notifications.AppriseConfig{Enabled: true, APIKey: "secret-key"}
		mockManager.On("GetAppriseConfig").Return(cfg).Once()

		req := httptest.NewRequest("GET", "/api/notifications/apprise", nil)
		w := httptest.NewRecorder()
		h.GetAppriseConfig(w, req)

		assert.Equal(t, 200, w.Code)
		var resp map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.NotContains(t, w.Body.String(), "secret-key") // Should be redacted
		assert.Equal(t, true, resp["hasApiKey"])
	})

	t.Run("UpdateAppriseConfig", func(t *testing.T) {
		cfg := notifications.AppriseConfig{Enabled: true}
		mockManager.On("GetAppriseConfig").Return(cfg).Twice() // preserve lookup + response echo
		mockManager.On("SetAppriseConfig", mock.Anything).Return().Once()
		mockPersistence.On("SaveAppriseConfig", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(cfg)
		req := httptest.NewRequest("PUT", "/api/notifications/apprise", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateAppriseConfig(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("UpdateAppriseConfig_PreservesSavedAPIKeyWhenBlank", func(t *testing.T) {
		existing := notifications.AppriseConfig{Enabled: true, APIKey: "saved-key", MinimumSeverity: "critical"}
		mockManager.On("GetAppriseConfig").Return(existing).Twice() // preserve lookup + response echo
		mockManager.On("SetAppriseConfig", mock.MatchedBy(func(cfg notifications.AppriseConfig) bool {
			return cfg.APIKey == "saved-key" && cfg.MinimumSeverity == "critical"
		})).Return().Once()
		mockPersistence.On("SaveAppriseConfig", mock.MatchedBy(func(cfg notifications.AppriseConfig) bool {
			return cfg.APIKey == "saved-key" && cfg.MinimumSeverity == "critical"
		})).Return(nil).Once()

		req := httptest.NewRequest(
			"PUT",
			"/api/notifications/apprise",
			bytes.NewBufferString(`{"enabled":true,"apiKey":""}`),
		)
		w := httptest.NewRecorder()
		h.UpdateAppriseConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "saved-key") // Response should stay redacted
	})

	t.Run("UpdateAppriseConfig_ReplacesAPIKey", func(t *testing.T) {
		mockManager.On("SetAppriseConfig", mock.MatchedBy(func(cfg notifications.AppriseConfig) bool {
			return cfg.APIKey == "new-key"
		})).Return().Once()
		mockPersistence.On("SaveAppriseConfig", mock.MatchedBy(func(cfg notifications.AppriseConfig) bool {
			return cfg.APIKey == "new-key"
		})).Return(nil).Once()
		mockManager.On("GetAppriseConfig").Return(notifications.AppriseConfig{Enabled: true, APIKey: "new-key"}).Twice()

		req := httptest.NewRequest(
			"PUT",
			"/api/notifications/apprise",
			bytes.NewBufferString(`{"enabled":true,"apiKey":"new-key"}`),
		)
		w := httptest.NewRecorder()
		h.UpdateAppriseConfig(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "new-key") // Response should stay redacted
	})

	t.Run("UpdateWebhook", func(t *testing.T) {
		webhook := notifications.WebhookConfig{ID: "wh1", Name: "Updated"}
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
		mockManager.On("ValidateWebhookURL", mock.Anything).Return(nil).Once()
		mockManager.On("UpdateWebhook", "wh1", mock.Anything).Return(nil).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(webhook)
		req := httptest.NewRequest("PUT", "/api/notifications/webhooks/wh1", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("UpdateWebhook_PreservesOmittedTagRouting", func(t *testing.T) {
		existing := notifications.WebhookConfig{
			ID:              "wh1",
			URL:             "https://example.com/hook",
			TagFilter:       []string{"customer:alpha", "critical"},
			TagMode:         "any",
			MinimumSeverity: "critical",
		}
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{existing}).Once()
		mockManager.On("ValidateWebhookURL", existing.URL).Return(nil).Once()
		mockManager.On(
			"UpdateWebhook",
			"wh1",
			mock.MatchedBy(func(cfg notifications.WebhookConfig) bool {
				return assert.ObjectsAreEqual(
					[]string{"customer:alpha", "critical"},
					cfg.TagFilter,
				) && cfg.TagMode == "any" && cfg.MinimumSeverity == "critical"
			}),
		).Return(nil).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		req := httptest.NewRequest(
			"PUT",
			"/api/notifications/webhooks/wh1",
			bytes.NewBufferString(`{"name":"Updated","url":"https://example.com/hook"}`),
		)
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var response map[string]interface{}
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
		assert.Equal(t, []interface{}{"customer:alpha", "critical"}, response["tagFilter"])
		assert.Equal(t, "any", response["tagFilterMode"])
		assert.Equal(t, "critical", response["minimumSeverity"])
	})

	t.Run("DeleteWebhook", func(t *testing.T) {
		mockManager.On("DeleteWebhook", "wh1").Return(nil).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		req := httptest.NewRequest("DELETE", "/api/notifications/webhooks/wh1", nil)
		w := httptest.NewRecorder()
		h.DeleteWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("GetWebhookTemplates", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/notifications/webhooks/templates", nil)
		w := httptest.NewRecorder()
		h.GetWebhookTemplates(w, req)
		assert.Equal(t, 200, w.Code)
		var templates []notifications.WebhookTemplate
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &templates))

		findTemplate := func(service string) notifications.WebhookTemplate {
			for _, tmpl := range templates {
				if tmpl.Service == service {
					return tmpl
				}
			}
			return notifications.WebhookTemplate{}
		}

		discord := findTemplate("discord")
		generic := findTemplate("generic")
		assert.Equal(t, "Discord", discord.Label)
		assert.Equal(t, "Generic", generic.Label)
		assert.Equal(t, "@everyone or <@USER_ID> or <@&ROLE_ID>", discord.MentionPlaceholder)
		assert.Equal(t, "Discord: Use @everyone, @here, <@USER_ID>, or <@&ROLE_ID>", discord.MentionHelp)
	})

	t.Run("GetWebhookHistory", func(t *testing.T) {
		mockManager.On("GetWebhookHistory").Return([]notifications.WebhookDelivery{}).Once()
		req := httptest.NewRequest("GET", "/api/notifications/webhooks/history", nil)
		w := httptest.NewRecorder()
		h.GetWebhookHistory(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("GetEmailProviders", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/notifications/email/providers", nil)
		w := httptest.NewRecorder()
		h.GetEmailProviders(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("HandleNotifications_Router", func(t *testing.T) {
		routes := []struct {
			method string
			path   string
			setup  func()
		}{
			{"GET", "/api/notifications/email", func() { mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once() }},
			{"PUT", "/api/notifications/email", func() {
				mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
				mockManager.On("SetEmailConfig", mock.Anything).Return().Once()
				mockPersistence.On("SaveEmailConfig", mock.Anything).Return(nil).Once()
			}},
			{"GET", "/api/notifications/apprise", func() { mockManager.On("GetAppriseConfig").Return(notifications.AppriseConfig{}).Once() }},
			{"PUT", "/api/notifications/apprise", func() {
				mockManager.On("SetAppriseConfig", mock.Anything).Return().Once()
				mockPersistence.On("SaveAppriseConfig", mock.Anything).Return(nil).Once()
				mockManager.On("GetAppriseConfig").Return(notifications.AppriseConfig{}).Twice() // preserve lookup + response echo
			}},
			{"GET", "/api/notifications/webhooks", func() { mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once() }},
			{"POST", "/api/notifications/webhooks", func() {
				mockManager.On("ValidateWebhookURL", mock.Anything).Return(nil).Once()
				mockManager.On("AddWebhook", mock.Anything).Return().Once()
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
				mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()
			}},
			{"POST", "/api/notifications/webhooks/test", func() {
				mockManager.On("TestEnhancedWebhook", mock.Anything).Return(200, "OK", nil).Once()
				mockManager.On("IsEnabled").Return(true).Once()
			}},
			{"PUT", "/api/notifications/webhooks/wh1", func() {
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
				mockManager.On("ValidateWebhookURL", mock.Anything).Return(nil).Once()
				mockManager.On("UpdateWebhook", "wh1", mock.Anything).Return(nil).Once()
				mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()
			}},
			{"DELETE", "/api/notifications/webhooks/wh1", func() {
				mockManager.On("DeleteWebhook", "wh1").Return(nil).Once()
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
				mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()
			}},
			{"GET", "/api/notifications/webhook-templates", func() {}},
			{"GET", "/api/notifications/webhook-history", func() { mockManager.On("GetWebhookHistory").Return([]notifications.WebhookDelivery{}).Once() }},
			{"GET", "/api/notifications/email-providers", func() {}},
			{"GET", "/api/notifications/health", func() {
				mockManager.On("GetQueueStats").Return(map[string]int{}, nil).Once()
				mockManager.On("GetTelemetryStats", mock.Anything).Return(
					notifications.TelemetryStats{},
					nil,
				).Once()
				mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
				mockPersistence.On("IsEncryptionEnabled").Return(true).Once()
			}},
		}

		for _, route := range routes {
			t.Run(route.method+"_"+route.path, func(t *testing.T) {
				route.setup()
				var body []byte
				if route.method == "POST" || route.method == "PUT" {
					body = []byte("{}")
				}
				req := httptest.NewRequest(route.method, route.path, bytes.NewReader(body))
				w := httptest.NewRecorder()
				h.HandleNotifications(w, req)
				assert.Equal(t, 200, w.Code)
			})
		}

		// Test 404
		req := httptest.NewRequest("GET", "/api/notifications/unknown", nil)
		w := httptest.NewRecorder()
		h.HandleNotifications(w, req)
		assert.Equal(t, 404, w.Code)
	})

	t.Run("TestNotification", func(t *testing.T) {
		mockManager.On("SendTestNotification", "email").Return(nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]string{"method": "email"})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_Webhook", func(t *testing.T) {
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
		mockManager.On("SendTestWebhook", mock.Anything).Return(nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]string{"method": "webhook", "webhookId": "wh1"})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestWebhook", func(t *testing.T) {
		mockManager.On("TestEnhancedWebhook", mock.Anything).Return(200, "OK", nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]string{"url": "https://example.com/test", "service": "ntfy"})
		req := httptest.NewRequest("POST", "/api/notifications/webhooks/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestWebhook(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("CreateWebhook_CanonicalizesPushoverLegacyAliasesAtAPIIngress", func(t *testing.T) {
		mockManager.ExpectedCalls = nil
		mockManager.Calls = nil
		mockPersistence.ExpectedCalls = nil
		mockPersistence.Calls = nil

		mockManager.On("ValidateWebhookURL", "https://api.pushover.net/1/messages.json").Return(nil).Once()
		mockManager.On("AddWebhook", mock.MatchedBy(func(w notifications.WebhookConfig) bool {
			return w.Service == "pushover" &&
				w.CustomFields["token"] == "legacy-app" &&
				w.CustomFields["user"] == "legacy-user" &&
				w.CustomFields["app_token"] == "" &&
				w.CustomFields["user_token"] == ""
		})).Return().Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name":    "Pushover",
			"url":     "https://api.pushover.net/1/messages.json",
			"service": "pushover",
			"customFields": map[string]string{
				"app_token":  "legacy-app",
				"user_token": "legacy-user",
			},
		})
		req := httptest.NewRequest("POST", "/api/notifications/webhooks", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.CreateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
		assert.NotContains(t, w.Body.String(), "app_token")
		assert.NotContains(t, w.Body.String(), "user_token")
		// Responses name the canonical fields but, like the list, mask their values.
		assert.Contains(t, w.Body.String(), "\"token\":\"***REDACTED***\"")
		assert.Contains(t, w.Body.String(), "\"user\":\"***REDACTED***\"")
		assert.NotContains(t, w.Body.String(), "legacy-app")
		assert.NotContains(t, w.Body.String(), "legacy-user")
	})

	t.Run("TestWebhook_UsesNotificationsOwnedTemplateSynthesis", func(t *testing.T) {
		mockManager.On("TestEnhancedWebhook", mock.MatchedBy(func(webhook notifications.EnhancedWebhookConfig) bool {
			return webhook.Service == "discord" &&
				strings.Contains(webhook.PayloadTemplate, `"username": "Pulse Monitoring"`) &&
				webhook.Headers["Content-Type"] == "application/json"
		})).Return(200, "OK", nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]string{"url": "https://example.com/test", "service": "discord"})
		req := httptest.NewRequest("POST", "/api/notifications/webhooks/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestWebhook(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestWebhook_CanonicalizesPushoverLegacyAliasesAtAPIIngress", func(t *testing.T) {
		mockManager.ExpectedCalls = nil
		mockManager.Calls = nil
		mockPersistence.ExpectedCalls = nil
		mockPersistence.Calls = nil

		mockManager.On("TestEnhancedWebhook", mock.MatchedBy(func(webhook notifications.EnhancedWebhookConfig) bool {
			_, hasLegacyApp := webhook.CustomFields["app_token"]
			_, hasLegacyUser := webhook.CustomFields["user_token"]
			return webhook.Service == "pushover" &&
				webhook.CustomFields["token"] == "legacy-app" &&
				webhook.CustomFields["user"] == "legacy-user" &&
				!hasLegacyApp &&
				!hasLegacyUser
		})).Return(200, "OK", nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"url":     "https://api.pushover.net/1/messages.json",
			"service": "pushover",
			"customFields": map[string]string{
				"app_token":  "legacy-app",
				"user_token": "legacy-user",
			},
		})
		req := httptest.NewRequest("POST", "/api/notifications/webhooks/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_EmailWithConfig", func(t *testing.T) {
		mockManager.On("SendTestNotificationWithConfig", "email", mock.Anything, mock.Anything).Return(nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]interface{}{
			"method": "email",
			"config": notifications.EmailConfig{Enabled: true, SMTPHost: "smtp.example.com", Password: "test"},
		})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_AppriseWithConfig", func(t *testing.T) {
		mockManager.On("SendTestAppriseWithConfig", mock.Anything).Return(nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]interface{}{
			"method": "apprise",
			"config": notifications.AppriseConfig{Enabled: true, APIKey: "test"},
		})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_AppriseWithConfig_UsesSavedAPIKeyWhenBlank", func(t *testing.T) {
		saved := notifications.AppriseConfig{Enabled: true, APIKey: "saved-key"}
		mockManager.On("GetAppriseConfig").Return(saved).Once()
		mockManager.On("SendTestAppriseWithConfig", mock.MatchedBy(func(cfg notifications.AppriseConfig) bool {
			return cfg.APIKey == "saved-key"
		})).Return(nil).Once()
		mockManager.On("IsEnabled").Return(true).Once()
		body, _ := json.Marshal(map[string]interface{}{
			"method": "apprise",
			"config": notifications.AppriseConfig{Enabled: true},
		})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("UpdateWebhook_PreserveRedacted", func(t *testing.T) {
		existing := notifications.WebhookConfig{
			ID:           "wh1",
			Headers:      map[string]string{"Auth": "secret"},
			CustomFields: map[string]string{"Key": "value"},
		}
		updated := notifications.WebhookConfig{
			ID:           "wh1",
			URL:          "https://example.com/new",
			Headers:      map[string]string{"Auth": "***REDACTED***"},
			CustomFields: map[string]string{"Key": "***REDACTED***"},
		}

		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{existing}).Once()
		mockManager.On("ValidateWebhookURL", "https://example.com/new").Return(nil).Once()
		mockManager.On("UpdateWebhook", "wh1", mock.MatchedBy(func(w notifications.WebhookConfig) bool {
			return w.Headers["Auth"] == "secret" && w.CustomFields["Key"] == "value"
		})).Return(nil).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(updated)
		req := httptest.NewRequest("PUT", "/api/notifications/webhooks/wh1", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("UpdateWebhook_CanonicalizesPushoverLegacyAliasesAtAPIIngress", func(t *testing.T) {
		mockManager.ExpectedCalls = nil
		mockManager.Calls = nil
		mockPersistence.ExpectedCalls = nil
		mockPersistence.Calls = nil

		existing := notifications.WebhookConfig{
			ID:      "wh-push",
			URL:     "https://api.pushover.net/1/messages.json",
			Service: "pushover",
		}

		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{existing}).Once()
		mockManager.On("ValidateWebhookURL", "https://api.pushover.net/1/messages.json").Return(nil).Once()
		mockManager.On("UpdateWebhook", "wh-push", mock.MatchedBy(func(w notifications.WebhookConfig) bool {
			return w.Service == "pushover" &&
				w.CustomFields["token"] == "legacy-app" &&
				w.CustomFields["user"] == "legacy-user" &&
				w.CustomFields["app_token"] == "" &&
				w.CustomFields["user_token"] == ""
		})).Return(nil).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"url":     "https://api.pushover.net/1/messages.json",
			"service": "pushover",
			"customFields": map[string]string{
				"app_token":  "legacy-app",
				"user_token": "legacy-user",
			},
		})
		req := httptest.NewRequest("PUT", "/api/notifications/webhooks/wh-push", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
		assert.NotContains(t, w.Body.String(), "app_token")
		assert.NotContains(t, w.Body.String(), "user_token")
		// Responses name the canonical fields but, like the list, mask their values.
		assert.Contains(t, w.Body.String(), "\"token\":\"***REDACTED***\"")
		assert.Contains(t, w.Body.String(), "\"user\":\"***REDACTED***\"")
		assert.NotContains(t, w.Body.String(), "legacy-app")
		assert.NotContains(t, w.Body.String(), "legacy-user")
	})

	t.Run("TestNotification_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader([]byte("{invalid}")))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("TestNotification_RequestBodyTooLarge", func(t *testing.T) {
		oversized := `{"method":"email","config":{"smtpHost":"` + strings.Repeat("a", notificationTestRequestBodyLimit) + `"}}`
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader([]byte(oversized)))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})

	t.Run("TestNotification_WebhookNotFound", func(t *testing.T) {
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		body, _ := json.Marshal(map[string]string{"method": "webhook", "webhookId": "nonexistent"})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 404, w.Code)
	})

	t.Run("TestWebhook_RequestBodyTooLarge", func(t *testing.T) {
		oversized := `{"url":"https://example.com/` + strings.Repeat("a", webhookTestRequestBodyLimit) + `","service":"generic"}`
		req := httptest.NewRequest("POST", "/api/notifications/webhooks/test", bytes.NewReader([]byte(oversized)))
		w := httptest.NewRecorder()
		h.TestWebhook(w, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	})
}

func TestNotificationConfigPersistenceFailureDoesNotPublishRuntimeState(t *testing.T) {
	newHandlers := func() (*NotificationHandlers, *MockNotificationManager, *MockNotificationConfigPersistence) {
		monitor := new(MockNotificationMonitor)
		manager := new(MockNotificationManager)
		persistence := new(MockNotificationConfigPersistence)
		monitor.On("GetNotificationManager").Return(manager)
		monitor.On("GetConfigPersistence").Return(persistence)
		return NewNotificationHandlers(nil, monitor), manager, persistence
	}

	t.Run("email", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		manager.On("GetEmailConfig").Return(notifications.EmailConfig{Enabled: true}).Once()
		persistence.On("SaveEmailConfig", mock.MatchedBy(func(cfg notifications.EmailConfig) bool {
			return !cfg.Enabled
		})).Return(errors.New("storage unavailable")).Once()

		req := httptest.NewRequest(http.MethodPut, "/api/notifications/email", strings.NewReader(`{"enabled":false}`))
		rec := httptest.NewRecorder()
		h.UpdateEmailConfig(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		manager.AssertNotCalled(t, "SetEmailConfig", mock.Anything)
		persistence.AssertExpectations(t)
	})

	t.Run("apprise", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		manager.On("GetAppriseConfig").Return(notifications.AppriseConfig{Enabled: true}).Once()
		persistence.On("SaveAppriseConfig", mock.MatchedBy(func(cfg notifications.AppriseConfig) bool {
			return !cfg.Enabled
		})).Return(errors.New("storage unavailable")).Once()

		req := httptest.NewRequest(http.MethodPut, "/api/notifications/apprise", strings.NewReader(`{"enabled":false}`))
		rec := httptest.NewRecorder()
		h.UpdateAppriseConfig(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		manager.AssertNotCalled(t, "SetAppriseConfig", mock.Anything)
		persistence.AssertExpectations(t)
	})

	t.Run("webhook create", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		manager.On("ValidateWebhookURL", "https://example.com/new").Return(nil).Once()
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "existing"}}).Once()
		persistence.On("SaveWebhooks", mock.MatchedBy(func(webhooks []notifications.WebhookConfig) bool {
			return len(webhooks) == 2 && webhooks[1].Name == "new"
		})).Return(errors.New("storage unavailable")).Once()

		req := httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks", strings.NewReader(`{"name":"new","url":"https://example.com/new"}`))
		rec := httptest.NewRecorder()
		h.CreateWebhook(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		manager.AssertNotCalled(t, "AddWebhook", mock.Anything)
		persistence.AssertExpectations(t)
	})

	t.Run("webhook update", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		existing := notifications.WebhookConfig{ID: "wh1", Name: "old", URL: "https://example.com/hook"}
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{existing}).Once()
		manager.On("ValidateWebhookURL", existing.URL).Return(nil).Once()
		persistence.On("SaveWebhooks", mock.MatchedBy(func(webhooks []notifications.WebhookConfig) bool {
			return len(webhooks) == 1 && webhooks[0].Name == "new"
		})).Return(errors.New("storage unavailable")).Once()

		req := httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/wh1", strings.NewReader(`{"name":"new","url":"https://example.com/hook"}`))
		rec := httptest.NewRecorder()
		h.UpdateWebhook(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		manager.AssertNotCalled(t, "UpdateWebhook", mock.Anything, mock.Anything)
		persistence.AssertExpectations(t)
	})

	t.Run("webhook delete", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
		persistence.On("SaveWebhooks", mock.MatchedBy(func(webhooks []notifications.WebhookConfig) bool {
			return len(webhooks) == 0
		})).Return(errors.New("storage unavailable")).Once()

		req := httptest.NewRequest(http.MethodDelete, "/api/notifications/webhooks/wh1", nil)
		rec := httptest.NewRecorder()
		h.DeleteWebhook(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		manager.AssertNotCalled(t, "DeleteWebhook", mock.Anything)
		persistence.AssertExpectations(t)
	})
}

func TestClassifyNotificationError(t *testing.T) {
	tests := []struct {
		name           string
		err            string
		wantSummary    string
		wantHasDetail  bool
		summaryMustNot []string // substrings that must NOT appear in summary
	}{
		{
			name:           "connection refused",
			err:            "dial tcp smtp.gmail.com:587: connect: connection refused",
			wantSummary:    "Could not connect to the server — check host, port, and firewall settings",
			wantHasDetail:  true,
			summaryMustNot: []string{"dial tcp"},
		},
		{
			name:           "no such host",
			err:            "dial tcp: lookup smtp.example.com: no such host",
			wantSummary:    "Server hostname not found — check the server address",
			wantHasDetail:  true,
			summaryMustNot: []string{"dial tcp"},
		},
		{
			name:           "i/o timeout",
			err:            "dial tcp 10.0.0.1:587: i/o timeout",
			wantSummary:    "Connection timed out — the server may be unreachable or the port may be blocked",
			wantHasDetail:  true,
			summaryMustNot: []string{"dial tcp"},
		},
		{
			name:           "context deadline exceeded",
			err:            "context deadline exceeded",
			wantSummary:    "Connection timed out — the server may be unreachable or the port may be blocked",
			wantHasDetail:  true,
			summaryMustNot: []string{"context"},
		},
		{
			name:           "x509 certificate error",
			err:            "x509: certificate signed by unknown authority",
			wantSummary:    "TLS certificate error — check certificate settings or try enabling 'Skip TLS Verify'",
			wantHasDetail:  true,
			summaryMustNot: []string{"x509:"},
		},
		{
			name:           "smtp auth failure",
			err:            "535 5.7.8 Username and Password not accepted",
			wantSummary:    "Authentication failed — check username and password",
			wantHasDetail:  true,
			summaryMustNot: []string{"535"},
		},
		{
			name:           "executable not found",
			err:            `exec: "apprise": executable file not found in $PATH`,
			wantSummary:    "Required program not found — ensure it is installed on the server",
			wantHasDetail:  true,
			summaryMustNot: []string{"exec:"},
		},
		{
			name:           "permission denied",
			err:            "open /etc/ssl/certs: permission denied",
			wantSummary:    "Permission denied — the server process lacks access to the required resource",
			wantHasDetail:  true,
			summaryMustNot: []string{"open /etc"},
		},
		{
			name:           "eof",
			err:            "read tcp 192.168.1.1:587: EOF",
			wantSummary:    "The server closed the connection unexpectedly — check if TLS/StartTLS settings are correct",
			wantHasDetail:  true,
			summaryMustNot: []string{"read tcp"},
		},
		{
			name:          "unknown error passes through",
			err:           "some unknown error",
			wantSummary:   "some unknown error",
			wantHasDetail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, detail := classifyNotificationError(fmt.Errorf("%s", tt.err))
			assert.Equal(t, tt.wantSummary, summary)
			if tt.wantHasDetail {
				assert.Equal(t, tt.err, detail, "detail should contain original error")
			} else {
				assert.Empty(t, detail, "detail should be empty for unclassified errors")
			}
			for _, banned := range tt.summaryMustNot {
				assert.NotContains(t, summary, banned, "summary must not contain Go internal prefix %q", banned)
			}
		})
	}
}

func TestWriteTestNotificationError(t *testing.T) {
	t.Run("classified error returns JSON with detail", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := fmt.Errorf("dial tcp smtp.gmail.com:587: connect: connection refused")
		writeTestNotificationError(w, err, http.StatusBadRequest)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

		var resp map[string]string
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Could not connect to the server — check host, port, and firewall settings", resp["error"])
		assert.Equal(t, "dial tcp smtp.gmail.com:587: connect: connection refused", resp["detail"])
	})

	t.Run("unclassified error returns JSON without detail", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := fmt.Errorf("some unknown error")
		writeTestNotificationError(w, err, http.StatusBadRequest)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp map[string]string
		assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "some unknown error", resp["error"])
		assert.Empty(t, resp["detail"])
	})
}

func TestNotificationHandlers_SetMonitorConcurrentAccess(t *testing.T) {
	mockMonitor1 := new(MockNotificationMonitor)
	mockMonitor2 := new(MockNotificationMonitor)
	h := NewNotificationHandlers(nil, mockMonitor1)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				if (worker+j)%2 == 0 {
					h.SetMonitor(mockMonitor1)
				} else {
					h.SetMonitor(mockMonitor2)
				}
				_ = h.getMonitor(context.Background())
			}
		}(i)
	}

	wg.Wait()
	assert.NotNil(t, h.getMonitor(context.Background()))
}

// Webhook signing secrets must be masked on list reads and preserved when an
// update echoes the masked placeholder back.
func TestNotificationHandlersWebhookSigningSecretLifecycle(t *testing.T) {
	mockMonitor := new(MockNotificationMonitor)
	mockManager := new(MockNotificationManager)
	mockPersistence := new(MockNotificationConfigPersistence)
	mockMonitor.On("GetNotificationManager").Return(mockManager)
	mockMonitor.On("GetConfigPersistence").Return(mockPersistence)
	h := NewNotificationHandlers(nil, mockMonitor)

	stored := notifications.WebhookConfig{
		ID:            "wh-signed",
		Name:          "Signed Webhook",
		URL:           "https://psa.example.com/inbound/pulse",
		Enabled:       true,
		Service:       "generic",
		SigningSecret: "stored-secret",
	}

	t.Run("GetWebhooksMasksSigningSecret", func(t *testing.T) {
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
		req := httptest.NewRequest("GET", "/api/notifications/webhooks", nil)
		w := httptest.NewRecorder()
		h.GetWebhooks(w, req)

		assert.Equal(t, 200, w.Code)
		var resp []map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, 1, len(resp))
		assert.Equal(t, "***REDACTED***", resp[0]["signingSecret"])
	})

	t.Run("UpdateWebhookPreservesRedactedSigningSecret", func(t *testing.T) {
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
		mockManager.On("ValidateWebhookURL", stored.URL).Return(nil).Once()
		mockManager.On("UpdateWebhook", "wh-signed", mock.MatchedBy(func(w notifications.WebhookConfig) bool {
			return w.SigningSecret == "stored-secret"
		})).Return(nil).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		update := stored
		update.SigningSecret = "***REDACTED***"
		body, _ := json.Marshal(update)
		req := httptest.NewRequest("PUT", "/api/notifications/webhooks/wh-signed", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)
		assert.Equal(t, 200, w.Code)
		mockManager.AssertExpectations(t)
	})
}

// A masked value means keep that value, not ignore the rest of the edit.
// All values here are synthetic; neither the response nor errors may reveal
// a preserved credential the editor did not submit.
func TestWebhookEditPreservesOnlyMaskedValues(t *testing.T) {
	const masked = "***REDACTED***"
	stored := notifications.WebhookConfig{ID: "edit", URL: "https://example.test/hook", Enabled: true,
		Headers:      map[string]string{"Authorization": "synthetic-auth", "Content-Type": "text/plain", "X-Remove": "old"},
		CustomFields: map[string]string{"token": "synthetic-token", "channel": "old", "remove": "old"}}
	cases := []struct {
		name        string
		headers     map[string]string
		fields      map[string]string
		wantHeaders map[string]string
		wantFields  map[string]string
		invalid     bool
	}{
		{"edit and remove beside masks", map[string]string{"Authorization": masked, "Content-Type": "application/json", "X-New": "new"},
			map[string]string{"token": masked, "channel": "new", "added": "new"},
			map[string]string{"Authorization": "synthetic-auth", "Content-Type": "application/json", "X-New": "new"},
			map[string]string{"token": "synthetic-token", "channel": "new", "added": "new"}, false},
		{"clear beside masks", map[string]string{"Authorization": masked, "Content-Type": ""},
			map[string]string{"token": masked, "channel": ""},
			map[string]string{"Authorization": "synthetic-auth"},
			map[string]string{"token": "synthetic-token", "channel": ""}, false},
		{"header case is insensitive", map[string]string{"authorization": masked}, map[string]string{"token": masked},
			map[string]string{"authorization": "synthetic-auth"}, map[string]string{"token": "synthetic-token"}, false},
		{"explicit empty maps", map[string]string{}, map[string]string{}, map[string]string{}, map[string]string{}, false},
		{"omitted maps keep replacement semantics", nil, nil, nil, nil, false},
		{"unknown masked header", map[string]string{"X-Unknown": masked}, nil, nil, nil, true},
		{"unknown masked field", nil, map[string]string{"unknown": masked}, nil, nil, true},
		{"custom field case is significant", nil, map[string]string{"Token": masked}, nil, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := new(MockNotificationManager)
			persistence := new(MockNotificationConfigPersistence)
			monitor := new(MockNotificationMonitor)
			monitor.On("GetNotificationManager").Return(manager)
			monitor.On("GetConfigPersistence").Return(persistence)
			manager.On("GetWebhooks").Return([]notifications.WebhookConfig{stored}).Once()
			if !tc.invalid {
				manager.On("ValidateWebhookURL", stored.URL).Return(nil).Once()
				matches := func(got notifications.WebhookConfig) bool {
					return reflect.DeepEqual(got.Headers, tc.wantHeaders) && reflect.DeepEqual(got.CustomFields, tc.wantFields)
				}
				persistence.On("SaveWebhooks", mock.MatchedBy(func(got []notifications.WebhookConfig) bool {
					return len(got) == 1 && matches(got[0])
				})).Return(nil).Once()
				manager.On("UpdateWebhook", stored.ID, mock.MatchedBy(matches)).Return(nil).Once()
			}
			incoming := stored
			incoming.Headers, incoming.CustomFields = tc.headers, tc.fields
			// Marshal maps explicitly: WebhookConfig's omitempty would turn an
			// explicit empty CustomFields map into an omitted field instead.
			form := map[string]any{"id": incoming.ID, "url": incoming.URL, "enabled": incoming.Enabled}
			if tc.headers != nil {
				form["headers"] = tc.headers
			}
			if tc.fields != nil {
				form["customFields"] = tc.fields
			}
			body, err := json.Marshal(form)
			assert.NoError(t, err)
			rec := httptest.NewRecorder()
			NewNotificationHandlers(nil, monitor).UpdateWebhook(rec, httptest.NewRequest(http.MethodPut,
				"/api/notifications/webhooks/"+stored.ID, bytes.NewReader(body)))
			wantStatus := http.StatusOK
			if tc.invalid {
				wantStatus = http.StatusBadRequest
				manager.AssertNotCalled(t, "UpdateWebhook", mock.Anything, mock.Anything)
				persistence.AssertNotCalled(t, "SaveWebhooks", mock.Anything)
			}
			assert.Equal(t, wantStatus, rec.Code, rec.Body.String())
			assert.NotContains(t, rec.Body.String(), "synthetic-auth")
			assert.NotContains(t, rec.Body.String(), "synthetic-token")
			if !tc.invalid {
				var response notifications.WebhookConfig
				assert.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
				var expectedFields map[string]string
				if len(incoming.CustomFields) > 0 {
					expectedFields = make(map[string]string, len(incoming.CustomFields))
					for key := range incoming.CustomFields {
						expectedFields[key] = maskedWebhookSecret
					}
				}
				assert.Equal(t, expectedFields, response.CustomFields)
			}
			assert.Equal(t, "old", stored.CustomFields["channel"], "edit mutated the previous map")
			assert.Equal(t, "text/plain", stored.Headers["Content-Type"], "edit mutated the previous map")
			manager.AssertExpectations(t)
			persistence.AssertExpectations(t)
		})
	}
}

func TestWebhookEditRejectsUnrecoverableMasks(t *testing.T) {
	for _, stored := range []map[string]string{
		{"Authorization": "***REDACTED***"},
		{"AUTHORIZATION": "synthetic-first", "authorization": "synthetic-second"},
	} {
		manager := new(MockNotificationManager)
		persistence := new(MockNotificationConfigPersistence)
		monitor := new(MockNotificationMonitor)
		monitor.On("GetNotificationManager").Return(manager)
		monitor.On("GetConfigPersistence").Return(persistence)
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "edit", Headers: stored}}).Once()
		rec := httptest.NewRecorder()
		NewNotificationHandlers(nil, monitor).UpdateWebhook(rec, httptest.NewRequest(http.MethodPut,
			"/api/notifications/webhooks/edit", strings.NewReader(`{"headers":{"Authorization":"***REDACTED***"}}`)))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.NotContains(t, rec.Body.String(), "synthetic-first")
		assert.NotContains(t, rec.Body.String(), "synthetic-second")
		manager.AssertNotCalled(t, "UpdateWebhook", mock.Anything, mock.Anything)
		persistence.AssertNotCalled(t, "SaveWebhooks", mock.Anything)
		manager.AssertExpectations(t)
	}
}

type webhookEditMonitor struct {
	manager     *notifications.NotificationManager
	persistence *config.ConfigPersistence
}

func (m *webhookEditMonitor) GetNotificationManager() NotificationManager { return m.manager }
func (m *webhookEditMonitor) GetConfigPersistence() NotificationConfigPersistence {
	return m.persistence
}

// This complements the #2540 single/static-header control: a real masked form
// edit with another saved credential, grouped firing/recovery, disable-grouping
// flush and the persistent autonomous sender. No native Telegram is contacted.
func TestWebhookMaskedEditPersistsAndDelivers(t *testing.T) {
	for _, service := range []string{"telegram", "generic"} {
		for _, mode := range []string{"grouped", "disable-pending-group"} {
			t.Run(service+"/"+mode, func(t *testing.T) {
				type delivery struct {
					header                     http.Header
					text, channel, chat, event string
				}
				var mu sync.Mutex
				var received []delivery
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload struct {
						Text    string `json:"text"`
						Channel string `json:"channel"`
						Chat    string `json:"chat_id"`
						Event   string `json:"event"`
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if r.Header.Get("Content-Type") != "application/json" || payload.Text == "" {
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					mu.Lock()
					received = append(received, delivery{r.Header.Clone(), payload.Text, payload.Channel, payload.Chat, payload.Event})
					mu.Unlock()
					w.WriteHeader(http.StatusOK)
				}))
				defer server.Close()
				dir := t.TempDir()
				persistence := config.NewConfigPersistence(dir)
				if !persistence.IsEncryptionEnabled() {
					t.Fatal("connected control requires encrypted save/load")
				}
				hook := notifications.WebhookConfig{ID: "edit", Name: "edit", Enabled: true, Method: "POST", Service: service,
					URL:          server.URL + "/hook?chat_id=-1001234",
					Headers:      map[string]string{"Authorization": "synthetic-auth", "Content-Type": "text/plain", "X-Route": "old", "X-Remove": "old"},
					CustomFields: map[string]string{"token": "synthetic-token", "channel": "old", "remove": "old"}}
				if service == "generic" {
					hook.Template = `{"text":"{{.Message | jsonString}}","channel":"{{.CustomFields.channel | jsonString}}","event":"{{.Event}}"}`
				}
				if err := persistence.SaveWebhooks([]notifications.WebhookConfig{hook}); err != nil {
					t.Fatal(err)
				}
				open := func() *notifications.NotificationManager {
					t.Helper()
					loaded, err := config.NewConfigPersistence(dir).LoadWebhooks()
					if err != nil || len(loaded) != 1 {
						t.Fatalf("saved destination unavailable: %v", err)
					}
					m := notifications.NewNotificationManagerWithDeferredQueue("", dir)
					if err := m.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
						t.Fatal(err)
					}
					m.AddWebhook(loaded[0])
					m.SetNotifyOnResolve(true)
					m.SetGroupingConfig(true, 1, true, false)
					t.Cleanup(m.Stop)
					return m
				}
				m := open()
				monitor := &webhookEditMonitor{m, persistence}
				handler := NewNotificationHandlers(nil, monitor)
				listed := httptest.NewRecorder()
				handler.GetWebhooks(listed, httptest.NewRequest(http.MethodGet, "/api/notifications/webhooks", nil))
				var editable []notifications.WebhookConfig
				if err := json.Unmarshal(listed.Body.Bytes(), &editable); err != nil || len(editable) != 1 {
					t.Fatal("cannot read masked destination")
				}
				edit := editable[0]
				if edit.Headers["Authorization"] != "***REDACTED***" || edit.CustomFields["token"] != "***REDACTED***" {
					t.Fatal("list disclosed credentials")
				}
				edit.Headers["authorization"] = edit.Headers["Authorization"]
				delete(edit.Headers, "Authorization")
				edit.Headers["Content-Type"], edit.Headers["X-Route"] = "application/json", "new"
				delete(edit.Headers, "X-Remove")
				edit.CustomFields["channel"] = "new"
				delete(edit.CustomFields, "remove")
				body, err := json.Marshal(edit)
				if err != nil {
					t.Fatal(err)
				}
				updated := httptest.NewRecorder()
				handler.UpdateWebhook(updated, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/edit", bytes.NewReader(body)))
				if updated.Code != http.StatusOK {
					t.Fatal("masked update failed")
				}
				if strings.Contains(updated.Body.String(), "synthetic-token") {
					t.Error("masked update exposed a stored credential")
				}
				m.Stop()
				m = open()
				monitor.manager = m
				saved := m.GetWebhooks()[0]
				if saved.Service != service || saved.Template != hook.Template || saved.URL != hook.URL ||
					saved.Headers["authorization"] != "synthetic-auth" || saved.Headers["Content-Type"] != "application/json" ||
					saved.Headers["X-Route"] != "new" || len(saved.Headers) != 3 ||
					saved.CustomFields["token"] != "synthetic-token" || saved.CustomFields["channel"] != "new" || len(saved.CustomFields) != 2 {
					t.Fatal("masked edit did not survive encrypted reload with explicit edits/removals intact")
				}
				encrypted, err := os.ReadFile(filepath.Join(dir, "webhooks.enc"))
				if err != nil || bytes.Contains(encrypted, []byte("synthetic-token")) || bytes.Contains(encrypted, []byte("synthetic-auth")) {
					t.Fatal("saved credentials were not encrypted")
				}
				wait := func(done func() bool) {
					t.Helper()
					deadline := time.Now().Add(15 * time.Second)
					for !done() {
						if time.Now().After(deadline) {
							t.Fatal("connected delivery did not complete")
						}
						time.Sleep(5 * time.Millisecond)
					}
				}
				batch := []*alerts.Alert{
					{ID: "edit-alpha", ResourceName: "edit-alpha", Node: "node", Type: "cpu", Message: "edit-alpha CPU above threshold", Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute)},
					{ID: "edit-bravo", ResourceName: "edit-bravo", Node: "node", Type: "cpu", Message: "edit-bravo CPU above threshold", Level: alerts.AlertLevelWarning, StartTime: time.Now().Add(-time.Minute)},
				}
				for _, a := range batch {
					m.SendAlert(a)
				}
				wantFiring := 1
				if mode == "disable-pending-group" {
					m.SetGroupingConfig(false, 600, true, false)
					wantFiring = 2
				}
				wait(func() bool { stats, err := m.GetQueueStats(); return err == nil && stats["pending"] == wantFiring })
				pending, err := m.GetQueue().GetPending(10)
				if err != nil || len(pending) != wantFiring {
					t.Fatal("ordinary grouping/disable policy produced the wrong number of jobs")
				}
				for _, row := range pending {
					var queued notifications.WebhookConfig
					if err := json.Unmarshal(row.Config, &queued); err != nil || queued.Headers["X-Route"] != "new" || queued.CustomFields["channel"] != "new" {
						t.Fatal("new admission did not snapshot the edited destination")
					}
					if len(row.Alerts) != 3-wantFiring {
						t.Fatal("grouped log did not match actual group membership")
					}
				}
				m.StartQueueProcessing()
				wait(func() bool { stats, err := m.GetQueueStats(); return err == nil && stats["sent"] == wantFiring })
				// Reopen after firing so recovery relies on durable receipts.
				m.Stop()
				m = open()
				if mode == "disable-pending-group" {
					m.SetGroupingConfig(false, 600, true, false)
				}
				for _, a := range batch {
					m.SendResolvedAlert(&alerts.ResolvedAlert{Alert: a, ResolvedTime: time.Now()})
				}
				m.StartQueueProcessing()
				wantTotal := wantFiring * 2
				wait(func() bool { stats, err := m.GetQueueStats(); return err == nil && stats["sent"] == wantTotal })
				wait(func() bool {
					entries, err := m.GetDeliveryLog(time.Now().Add(-time.Hour), 20)
					return err == nil && len(entries) == wantTotal
				})
				entries, err := m.GetDeliveryLog(time.Now().Add(-time.Hour), 20)
				if err != nil {
					t.Fatal(err)
				}
				for _, e := range entries {
					if !e.Success || e.Outcome != notifications.DeliveryOutcomeSent || e.Attempts != 1 || e.AlertCount != 3-wantFiring {
						t.Fatal("delivery audit lost exact attempt or group membership")
					}
				}
				m.Stop()
				mu.Lock()
				got := append([]delivery(nil), received...)
				mu.Unlock()
				if len(got) != wantTotal {
					t.Fatalf("received %d payloads, want %d", len(got), wantTotal)
				}
				for phase := 0; phase < 2; phase++ {
					combined := ""
					for _, d := range got[phase*wantFiring : (phase+1)*wantFiring] {
						if d.header.Get("Authorization") != "synthetic-auth" || d.header.Get("X-Route") != "new" || d.header.Get("X-Remove") != "" {
							t.Fatal("delivery lost an explicit edit or masked credential")
						}
						if service == "telegram" && d.chat != "-1001234" {
							t.Fatal("saved Telegram service/chat did not reach delivery")
						}
						if service == "generic" && d.channel != "new" {
							t.Fatal("custom field edit did not reach the custom payload")
						}
						if phase == 1 && ((service == "telegram" && !strings.Contains(strings.ToLower(d.text), "resolved")) || (service == "generic" && d.event != "resolved")) {
							t.Fatal("recovery rendered as a firing")
						}
						combined += d.text
					}
					for _, a := range batch {
						if !strings.Contains(combined, a.ResourceName) {
							t.Fatal("grouped/individual payload lost an alert")
						}
					}
				}
			})
		}
	}
}

// The webhook editor sends back the masked values GetWebhooks returned for
// every header, custom field and signing secret the user did not retype. Each
// masked value must resolve to its own saved value: never to the mask itself,
// never by discarding values typed in the same edit, and never back out in a
// response.
func TestWebhookMaskedValuesResolvePerKey(t *testing.T) {
	newHandlers := func() (*NotificationHandlers, *MockNotificationManager, *MockNotificationConfigPersistence) {
		monitor := new(MockNotificationMonitor)
		manager := new(MockNotificationManager)
		persistence := new(MockNotificationConfigPersistence)
		monitor.On("GetNotificationManager").Return(manager)
		monitor.On("GetConfigPersistence").Return(persistence)
		return NewNotificationHandlers(nil, monitor), manager, persistence
	}
	saved := notifications.WebhookConfig{
		ID:            "wh-ops",
		Name:          "Ops",
		URL:           "https://hooks.example.com/ops",
		Enabled:       true,
		Service:       "pushover",
		Headers:       map[string]string{"Authorization": "Bearer saved-token", "Content-Type": "application/json"},
		CustomFields:  map[string]string{"token": "saved-app-token", "user": "saved-user-key"},
		SigningSecret: "saved-signing-secret",
	}
	savedSecrets := []string{"saved-token", "saved-app-token", "saved-user-key", "saved-signing-secret"}

	t.Run("update keeps values typed in the same edit", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		var published notifications.WebhookConfig
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{saved}).Once()
		manager.On("ValidateWebhookURL", saved.URL).Return(nil).Once()
		manager.On("UpdateWebhook", "wh-ops", mock.Anything).Run(func(args mock.Arguments) {
			published = args.Get(1).(notifications.WebhookConfig)
		}).Return(nil).Once()
		persistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name": "Ops", "url": saved.URL, "enabled": true, "service": "pushover",
			"headers": map[string]string{
				"Authorization": "***REDACTED***",
				"Content-Type":  "***REDACTED***",
				"X-Team":        "platform",
			},
			"customFields":  map[string]string{"token": "***REDACTED***", "user": "new-user-key"},
			"signingSecret": "***REDACTED***",
		})
		rec := httptest.NewRecorder()
		h.UpdateWebhook(rec, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/wh-ops", bytes.NewReader(body)))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]string{
			"Authorization": "Bearer saved-token",
			"Content-Type":  "application/json",
			"X-Team":        "platform",
		}, published.Headers)
		assert.Equal(t, map[string]string{"token": "saved-app-token", "user": "new-user-key"}, published.CustomFields)
		assert.Equal(t, "saved-signing-secret", published.SigningSecret)
		for _, secret := range append(savedSecrets, "new-user-key", "platform") {
			assert.NotContains(t, rec.Body.String(), secret, "update response must stay masked")
		}
		assert.Contains(t, rec.Body.String(), `"X-Team":"***REDACTED***"`)
	})

	t.Run("update never stores the mask or a cleared header", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		var published notifications.WebhookConfig
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{saved}).Once()
		manager.On("ValidateWebhookURL", saved.URL).Return(nil).Once()
		manager.On("UpdateWebhook", "wh-ops", mock.Anything).Run(func(args mock.Arguments) {
			published = args.Get(1).(notifications.WebhookConfig)
		}).Return(nil).Once()
		persistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name": "Ops", "url": saved.URL, "enabled": true, "service": "pushover",
			// Content-Type cleared in the editor; known masked values stay saved.
			// Unknown masks are rejected by TestWebhookEditPreservesOnlyMaskedValues.
			"headers":      map[string]string{"Authorization": "***REDACTED***", "Content-Type": ""},
			"customFields": map[string]string{"token": "***REDACTED***", "user": "***REDACTED***"},
		})
		rec := httptest.NewRecorder()
		h.UpdateWebhook(rec, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/wh-ops", bytes.NewReader(body)))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]string{"Authorization": "Bearer saved-token"}, published.Headers)
		assert.Equal(t, "", published.SigningSecret, "an omitted signing secret is removed, as before")
	})

	t.Run("create keeps literal values and responds masked", func(t *testing.T) {
		h, manager, persistence := newHandlers()
		var added notifications.WebhookConfig
		manager.On("ValidateWebhookURL", "https://hooks.example.com/new").Return(nil).Once()
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		manager.On("AddWebhook", mock.Anything).Run(func(args mock.Arguments) {
			added = args.Get(0).(notifications.WebhookConfig)
		}).Return().Once()
		persistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(map[string]interface{}{
			"name": "New", "url": "https://hooks.example.com/new", "enabled": true,
			"headers":       map[string]string{"X-Api-Key": "typed-key"},
			"signingSecret": "typed-signing-secret",
		})
		rec := httptest.NewRecorder()
		h.CreateWebhook(rec, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks", bytes.NewReader(body)))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, map[string]string{"X-Api-Key": "typed-key"}, added.Headers)
		assert.Equal(t, "typed-signing-secret", added.SigningSecret)
		assert.NotContains(t, rec.Body.String(), "typed-key")
		assert.NotContains(t, rec.Body.String(), "typed-signing-secret")
		assert.Contains(t, rec.Body.String(), `"signingSecret":"***REDACTED***"`)
	})

	t.Run("form test of a saved webhook sends its saved values", func(t *testing.T) {
		h, manager, _ := newHandlers()
		var tested notifications.EnhancedWebhookConfig
		manager.On("GetWebhooks").Return([]notifications.WebhookConfig{saved}).Once()
		manager.On("TestEnhancedWebhook", mock.Anything).Run(func(args mock.Arguments) {
			tested = args.Get(0).(notifications.EnhancedWebhookConfig)
		}).Return(200, "OK", nil).Once()
		manager.On("IsEnabled").Return(true).Once()

		// The edit form posts its whole state, including the saved id.
		body, _ := json.Marshal(map[string]interface{}{
			"id": "wh-ops", "name": "Ops", "url": saved.URL, "enabled": true, "service": "pushover",
			"headers":       map[string]string{"Authorization": "***REDACTED***", "X-Team": "platform"},
			"customFields":  map[string]string{"token": "***REDACTED***", "user": "***REDACTED***"},
			"signingSecret": "***REDACTED***",
		})
		rec := httptest.NewRecorder()
		h.TestWebhook(rec, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks/test", bytes.NewReader(body)))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "Bearer saved-token", tested.Headers["Authorization"])
		assert.Equal(t, "platform", tested.Headers["X-Team"])
		assert.Equal(t, "saved-app-token", tested.CustomFields["token"])
		assert.Equal(t, "saved-user-key", tested.CustomFields["user"])
		assert.Equal(t, "saved-signing-secret", tested.SigningSecret)
	})

	t.Run("form test of an unsaved webhook rejects the mask before sending", func(t *testing.T) {
		h, manager, _ := newHandlers()
		body, _ := json.Marshal(map[string]interface{}{
			"url":     "https://hooks.example.com/new",
			"headers": map[string]string{"Authorization": maskedWebhookSecret, "X-Team": "platform"},
		})
		rec := httptest.NewRecorder()
		h.TestWebhook(rec, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks/test", bytes.NewReader(body)))
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		manager.AssertNotCalled(t, "TestEnhancedWebhook", mock.Anything)
		manager.AssertNotCalled(t, "GetWebhooks")
	})
}

// A masked form must fail before persistence or a Test send if its saved
// identity is missing or ambiguous. Test and Update must resolve the same map.
func TestWebhookPlaceholderBoundaryAgreement(t *testing.T) {
	for _, route := range []string{"update", "test", "create"} {
		for _, tc := range []struct {
			name                                                       string
			savedHeaders, incomingHeaders, savedFields, incomingFields map[string]string
			savedSigning, incomingSigning                              string
			valid                                                      bool
		}{
			{"case varied saved header", map[string]string{"Authorization": "synthetic-auth"}, map[string]string{"authorization": maskedWebhookSecret}, nil, nil, "", "", true},
			{"unknown header", nil, map[string]string{"X-Missing": maskedWebhookSecret}, nil, nil, "", "", false},
			{"stored header is a mask", map[string]string{"Authorization": maskedWebhookSecret}, map[string]string{"Authorization": maskedWebhookSecret}, nil, nil, "", "", false},
			{"exact match also has conflicting alias", map[string]string{"Authorization": "synthetic-one", "authorization": "synthetic-two"}, map[string]string{"Authorization": maskedWebhookSecret}, nil, nil, "", "", false},
			{"incoming aliases conflict", nil, map[string]string{"X-Route": "one", "x-route": "two"}, nil, nil, "", "", false},
			{"custom fields are case sensitive", nil, nil, map[string]string{"Token": "synthetic-token"}, map[string]string{"token": maskedWebhookSecret}, "", "", false},
			{"missing signing secret", nil, nil, nil, nil, "", maskedWebhookSecret, false},
			{"saved signing secret is a mask", nil, nil, nil, nil, maskedWebhookSecret, maskedWebhookSecret, false},
		} {
			if route == "create" && tc.valid {
				continue
			}
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				manager := new(MockNotificationManager)
				persistence := new(MockNotificationConfigPersistence)
				monitor := new(MockNotificationMonitor)
				monitor.On("GetNotificationManager").Return(manager)
				monitor.On("GetConfigPersistence").Return(persistence)
				saved := notifications.WebhookConfig{ID: "saved", URL: "https://hooks.example.test/alerts", Headers: tc.savedHeaders, CustomFields: tc.savedFields, SigningSecret: tc.savedSigning}
				manager.On("GetWebhooks").Return([]notifications.WebhookConfig{saved}).Maybe()
				manager.On("ValidateWebhookURL", mock.Anything).Return(nil).Maybe()
				manager.On("IsEnabled").Return(true).Maybe()
				manager.On("UpdateWebhook", mock.Anything, mock.Anything).Return(nil).Maybe()
				manager.On("AddWebhook", mock.Anything).Return().Maybe()
				persistence.On("SaveWebhooks", mock.Anything).Return(nil).Maybe()
				var tested notifications.EnhancedWebhookConfig
				manager.On("TestEnhancedWebhook", mock.Anything).Run(func(a mock.Arguments) { tested = a.Get(0).(notifications.EnhancedWebhookConfig) }).Return(200, "OK", nil).Maybe()
				incoming := notifications.WebhookConfig{ID: "saved", URL: saved.URL, Headers: tc.incomingHeaders, CustomFields: tc.incomingFields, SigningSecret: tc.incomingSigning}
				body, err := json.Marshal(incoming)
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				h := NewNotificationHandlers(nil, monitor)
				switch route {
				case "create":
					h.CreateWebhook(rec, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks", bytes.NewReader(body)))
				case "update":
					h.UpdateWebhook(rec, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/saved", bytes.NewReader(body)))
				case "test":
					h.TestWebhook(rec, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks/test", bytes.NewReader(body)))
				}
				if tc.valid {
					if rec.Code != http.StatusOK {
						t.Fatalf("status = %d, want 200", rec.Code)
					}
					if route == "test" && tested.Headers["authorization"] != "synthetic-auth" {
						t.Fatal("Test silently dropped the case-varied saved credential")
					}
				} else {
					if rec.Code != http.StatusBadRequest {
						t.Errorf("status = %d, want 400", rec.Code)
					}
					manager.AssertNotCalled(t, "TestEnhancedWebhook", mock.Anything)
					manager.AssertNotCalled(t, "UpdateWebhook", mock.Anything, mock.Anything)
					manager.AssertNotCalled(t, "AddWebhook", mock.Anything)
					persistence.AssertNotCalled(t, "SaveWebhooks", mock.Anything)
				}
				for _, secret := range []string{"synthetic-auth", "synthetic-one", "synthetic-two", "synthetic-token"} {
					if strings.Contains(rec.Body.String(), secret) {
						t.Error("response disclosed a synthetic saved value")
					}
				}
			})
		}
	}
}

func TestWebhookSavedFormTestMatchesEncryptedEdit(t *testing.T) {
	type observation struct {
		header  http.Header
		payload string
	}
	var mu sync.Mutex
	var observed []observation
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(400)
			return
		}
		mu.Lock()
		observed = append(observed, observation{r.Header.Clone(), string(payload)})
		mu.Unlock()
		fmt.Fprint(w, "OK")
	}))
	defer server.Close()
	dir := t.TempDir()
	persistence := config.NewConfigPersistence(dir)
	saved := notifications.WebhookConfig{ID: "saved-form", Name: "Ops", URL: server.URL, Enabled: true, Service: "generic", Headers: map[string]string{"Authorization": "synthetic-auth", "Content-Type": "text/plain", "X-Delete": "remove"}, CustomFields: map[string]string{"token": "synthetic-token", "channel": "old", "delete": "remove"}, SigningSecret: "synthetic-signing"}
	if err := persistence.SaveWebhooks([]notifications.WebhookConfig{saved}); err != nil {
		t.Fatal(err)
	}
	manager := notifications.NewNotificationManagerWithDeferredQueue("", dir)
	defer manager.Stop()
	if err := manager.UpdateAllowedPrivateCIDRs("127.0.0.1/32,::1/128"); err != nil {
		t.Fatal(err)
	}
	manager.AddWebhook(saved)
	liveBefore := manager.GetWebhooks()[0]
	h := NewNotificationHandlers(nil, &webhookEditMonitor{manager, persistence})
	listed := httptest.NewRecorder()
	h.GetWebhooks(listed, httptest.NewRequest(http.MethodGet, "/api/notifications/webhooks", nil))
	var editable []notifications.WebhookConfig
	if err := json.Unmarshal(listed.Body.Bytes(), &editable); err != nil || len(editable) != 1 {
		t.Fatal("masked saved form unavailable")
	}
	form := editable[0]
	form.Headers["authorization"] = form.Headers["Authorization"]
	delete(form.Headers, "Authorization")
	form.Headers["Content-Type"] = ""
	delete(form.Headers, "X-Delete")
	form.Headers["X-New"] = "new"
	form.CustomFields["channel"] = "new"
	delete(form.CustomFields, "delete")
	body, err := json.Marshal(form)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "webhooks.enc"))
	if err != nil {
		t.Fatal(err)
	}
	tested := httptest.NewRecorder()
	h.TestWebhook(tested, httptest.NewRequest(http.MethodPost, "/api/notifications/webhooks/test", bytes.NewReader(body)))
	if tested.Code != http.StatusOK {
		t.Fatalf("saved form test: %d", tested.Code)
	}
	after, err := os.ReadFile(filepath.Join(dir, "webhooks.enc"))
	if err != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(manager.GetWebhooks()[0], liveBefore) {
		t.Fatal("form Test mutated durable or live configuration")
	}
	mu.Lock()
	got := append([]observation(nil), observed...)
	mu.Unlock()
	if len(got) != 1 || got[0].header.Get("Authorization") != "synthetic-auth" || got[0].header.Get("Content-Type") != "application/json" || got[0].header.Get("X-New") != "new" || got[0].header.Get("X-Delete") != "" || strings.Contains(got[0].payload, maskedWebhookSecret) {
		t.Fatal("real Test did not honour saved identities, explicit edits/removals and JSON default")
	}
	updated := httptest.NewRecorder()
	h.UpdateWebhook(updated, httptest.NewRequest(http.MethodPut, "/api/notifications/webhooks/saved-form", bytes.NewReader(body)))
	if updated.Code != http.StatusOK {
		t.Fatalf("saved form update: %d", updated.Code)
	}
	loaded, err := persistence.LoadWebhooks()
	if err != nil || len(loaded) != 1 {
		t.Fatal("encrypted edit unavailable after reload")
	}
	if loaded[0].Headers["authorization"] != "synthetic-auth" || loaded[0].Headers["X-New"] != "new" || len(loaded[0].Headers) != 2 || loaded[0].CustomFields["token"] != "synthetic-token" || loaded[0].CustomFields["channel"] != "new" || len(loaded[0].CustomFields) != 2 || loaded[0].SigningSecret != "synthetic-signing" {
		t.Fatal("Test and persisted edit resolved different identities")
	}
	for _, response := range []string{listed.Body.String(), updated.Body.String(), tested.Body.String()} {
		for _, secret := range []string{"synthetic-auth", "synthetic-token", "synthetic-signing"} {
			if strings.Contains(response, secret) {
				t.Fatal("configuration response disclosed a synthetic saved secret")
			}
		}
	}
	// The existing ordinary-delivery integration separately covers restart,
	// grouped/un-grouped firing and resolution using the saved destination.
}
