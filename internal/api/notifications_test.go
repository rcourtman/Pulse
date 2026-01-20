package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func (m *MockNotificationMonitor) GetState() models.StateSnapshot {
	args := m.Called()
	return args.Get(0).(models.StateSnapshot)
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

	h := NewNotificationHandlers(mockMonitor)

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
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "smtp.example.com", resp.SMTPHost)
		assert.Empty(t, resp.Password) // Should be redacted
	})

	t.Run("UpdateEmailConfig", func(t *testing.T) {
		cfg := notifications.EmailConfig{
			Enabled:  true,
			SMTPHost: "smtp.example.com",
			Password: "newpassword",
		}
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

	t.Run("UpdateEmailConfig_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader([]byte("{invalid}")))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("UpdateEmailConfig_SaveError", func(t *testing.T) {
		cfg := notifications.EmailConfig{Enabled: true}
		mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
		mockManager.On("SetEmailConfig", mock.Anything).Return().Once()
		mockPersistence.On("SaveEmailConfig", mock.Anything).Return(fmt.Errorf("save error")).Once()

		body, _ := json.Marshal(cfg)
		req := httptest.NewRequest("PUT", "/api/notifications/email", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateEmailConfig(w, req)
		assert.Equal(t, 200, w.Code) // Matches implementation: logs error but returns success
	})

	t.Run("GetWebhooks", func(t *testing.T) {
		webhooks := []notifications.WebhookConfig{
			{
				ID:      "wh1",
				Name:    "Test Webhook",
				URL:     "https://example.com",
				Headers: map[string]string{"Authorization": "Bearer token"},
			},
		}
		mockManager.On("GetWebhooks").Return(webhooks).Once()

		req := httptest.NewRequest("GET", "/api/notifications/webhooks", nil)
		w := httptest.NewRecorder()
		h.GetWebhooks(w, req)

		assert.Equal(t, 200, w.Code)
		var resp []map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, 1, len(resp))
		assert.Equal(t, "wh1", resp[0]["id"])
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
		mockManager.On("GetEmailConfig").Return(notifications.EmailConfig{}).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		mockPersistence.On("IsEncryptionEnabled").Return(true).Once()

		req := httptest.NewRequest("GET", "/api/notifications/health", nil)
		w := httptest.NewRecorder()
		h.GetNotificationHealth(w, req)

		assert.Equal(t, 200, w.Code)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		queue := resp["queue"].(map[string]interface{})
		assert.Equal(t, float64(1), queue["pending"])
		assert.Equal(t, true, queue["healthy"])
	})

	t.Run("GetAppriseConfig", func(t *testing.T) {
		cfg := notifications.AppriseConfig{Enabled: true}
		mockManager.On("GetAppriseConfig").Return(cfg).Once()

		req := httptest.NewRequest("GET", "/api/notifications/apprise", nil)
		w := httptest.NewRecorder()
		h.GetAppriseConfig(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("UpdateAppriseConfig", func(t *testing.T) {
		cfg := notifications.AppriseConfig{Enabled: true}
		mockManager.On("SetAppriseConfig", mock.Anything).Return().Once()
		mockPersistence.On("SaveAppriseConfig", mock.Anything).Return(nil).Once()
		mockManager.On("GetAppriseConfig").Return(cfg).Once()

		body, _ := json.Marshal(cfg)
		req := httptest.NewRequest("PUT", "/api/notifications/apprise", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateAppriseConfig(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("UpdateWebhook", func(t *testing.T) {
		webhook := notifications.WebhookConfig{ID: "wh1", Name: "Updated"}
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
		mockManager.On("ValidateWebhookURL", mock.Anything).Return(nil).Once()
		mockManager.On("UpdateWebhook", "wh1", mock.Anything).Return(nil).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{webhook}).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(webhook)
		req := httptest.NewRequest("PUT", "/api/notifications/webhooks/wh1", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("DeleteWebhook", func(t *testing.T) {
		mockManager.On("DeleteWebhook", "wh1").Return(nil).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
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
				mockManager.On("GetAppriseConfig").Return(notifications.AppriseConfig{}).Once()
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
			}},
			{"PUT", "/api/notifications/webhooks/wh1", func() {
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
				mockManager.On("ValidateWebhookURL", mock.Anything).Return(nil).Once()
				mockManager.On("UpdateWebhook", "wh1", mock.Anything).Return(nil).Once()
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
				mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()
			}},
			{"DELETE", "/api/notifications/webhooks/wh1", func() {
				mockManager.On("DeleteWebhook", "wh1").Return(nil).Once()
				mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
				mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()
			}},
			{"GET", "/api/notifications/webhook-templates", func() {}},
			{"GET", "/api/notifications/webhook-history", func() { mockManager.On("GetWebhookHistory").Return([]notifications.WebhookDelivery{}).Once() }},
			{"GET", "/api/notifications/email-providers", func() {}},
			{"GET", "/api/notifications/health", func() {
				mockManager.On("GetQueueStats").Return(map[string]int{}, nil).Once()
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
		mockMonitor.On("GetState").Return(models.StateSnapshot{}).Once()
		mockManager.On("SendTestNotification", "email").Return(nil).Once()
		body, _ := json.Marshal(map[string]string{"method": "email"})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_Webhook", func(t *testing.T) {
		mockMonitor.On("GetState").Return(models.StateSnapshot{}).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{{ID: "wh1"}}).Once()
		mockManager.On("SendTestWebhook", mock.Anything).Return(nil).Once()
		body, _ := json.Marshal(map[string]string{"method": "webhook", "webhookId": "wh1"})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestWebhook", func(t *testing.T) {
		mockManager.On("TestEnhancedWebhook", mock.Anything).Return(200, "OK", nil).Once()
		body, _ := json.Marshal(map[string]string{"url": "https://example.com/test", "service": "ntfy"})
		req := httptest.NewRequest("POST", "/api/notifications/webhooks/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestWebhook(w, req)
		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_EmailWithConfig", func(t *testing.T) {
		mockMonitor.On("GetState").Return(models.StateSnapshot{}).Once()
		mockManager.On("SendTestNotificationWithConfig", "email", mock.Anything, mock.Anything).Return(nil).Once()
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
		mockMonitor.On("GetState").Return(models.StateSnapshot{}).Once()
		mockManager.On("SendTestAppriseWithConfig", mock.Anything).Return(nil).Once()
		body, _ := json.Marshal(map[string]interface{}{
			"method": "apprise",
			"config": notifications.AppriseConfig{Enabled: true, APIKey: "test"},
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
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{updated}).Once()
		mockPersistence.On("SaveWebhooks", mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(updated)
		req := httptest.NewRequest("PUT", "/api/notifications/webhooks/wh1", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.UpdateWebhook(w, req)

		assert.Equal(t, 200, w.Code)
	})

	t.Run("TestNotification_InvalidJSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader([]byte("{invalid}")))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 400, w.Code)
	})

	t.Run("TestNotification_WebhookNotFound", func(t *testing.T) {
		mockMonitor.On("GetState").Return(models.StateSnapshot{}).Once()
		mockManager.On("GetWebhooks").Return([]notifications.WebhookConfig{}).Once()
		body, _ := json.Marshal(map[string]string{"method": "webhook", "webhookId": "nonexistent"})
		req := httptest.NewRequest("POST", "/api/notifications/test", bytes.NewReader(body))
		w := httptest.NewRecorder()
		h.TestNotification(w, req)
		assert.Equal(t, 404, w.Code)
	})
}
