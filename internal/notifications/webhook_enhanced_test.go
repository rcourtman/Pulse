package notifications

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
)

func TestEnhancedWebhook(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Test Webhook",
			URL:  server.URL,
		},
		Service:         "discord",
		PayloadTemplate: "{}",
	}

	status, resp, err := nm.TestEnhancedWebhook(webhook)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "ok", resp)
}

func TestShouldSendWebhook(t *testing.T) {
	nm := &NotificationManager{}

	tests := []struct {
		name     string
		webhook  EnhancedWebhookConfig
		alert    *alerts.Alert
		expected bool
	}{
		{
			name: "Match Level",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					Levels: []string{"critical"},
				},
			},
			alert: &alerts.Alert{
				Level: alerts.AlertLevelCritical,
			},
			expected: true,
		},
		{
			name: "No Match Level",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					Levels: []string{"critical"},
				},
			},
			alert: &alerts.Alert{
				Level: alerts.AlertLevelWarning,
			},
			expected: false,
		},
		{
			name: "Match Type",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					Types: []string{"cpu"},
				},
			},
			alert: &alerts.Alert{
				Type: "cpu",
			},
			expected: true,
		},
		{
			name: "Match Node",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					Nodes: []string{"pve-01"},
				},
			},
			alert: &alerts.Alert{
				Node: "pve-01",
			},
			expected: true,
		},
		{
			name: "Match ResourceType",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					ResourceTypes: []string{"vm"},
				},
			},
			alert: &alerts.Alert{
				Metadata: map[string]interface{}{
					"resourceType": "vm",
				},
			},
			expected: true,
		},
		{
			name: "Empty Rules Match Everything",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{},
			},
			alert: &alerts.Alert{
				Level: alerts.AlertLevelWarning,
			},
			expected: true,
		},
		{
			name: "No Match Node",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					Nodes: []string{"pve-01"},
				},
			},
			alert: &alerts.Alert{
				Node: "pve-02",
			},
			expected: false,
		},
		{
			name: "No Match ResourceType",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					ResourceTypes: []string{"vm"},
				},
			},
			alert: &alerts.Alert{
				Metadata: map[string]interface{}{
					"resourceType": "container",
				},
			},
			expected: false,
		},
		{
			name: "Missing ResourceType Metadata",
			webhook: EnhancedWebhookConfig{
				FilterRules: WebhookFilterRules{
					ResourceTypes: []string{"vm"},
				},
			},
			alert: &alerts.Alert{
				Metadata: map[string]interface{}{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nm.shouldSendWebhook(tt.webhook, tt.alert)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSendWebhookWithRetry_429RetryAfter(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Test Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   2,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestSendEnhancedWebhook_TemplateRendering(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	receivedPayload := ""
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedPayload = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Templated Webhook",
			URL:  server.URL,
		},
		PayloadTemplate: `{"alert":"{{.ID}}", "node":"{{.Node}}"}`,
	}

	alert := &alerts.Alert{
		ID:   "ALERT-123",
		Node: "pve-node",
	}

	err := nm.SendEnhancedWebhook(webhook, alert)
	assert.NoError(t, err)
	assert.Contains(t, receivedPayload, "ALERT-123")
	assert.Contains(t, receivedPayload, "pve-node")
}

func TestSendEnhancedWebhook_SpecialServices(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	t.Run("Telegram ChatID", func(t *testing.T) {
		server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		webhook := EnhancedWebhookConfig{
			WebhookConfig: WebhookConfig{
				URL: server.URL + "?chat_id=12345",
			},
			Service:         "telegram",
			PayloadTemplate: "{}",
		}
		err := nm.SendEnhancedWebhook(webhook, &alerts.Alert{})
		assert.NoError(t, err)
	})

	t.Run("PagerDuty RoutingKey", func(t *testing.T) {
		server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		webhook := EnhancedWebhookConfig{
			WebhookConfig: WebhookConfig{
				URL:     server.URL,
				Headers: map[string]string{"routing_key": "abc"},
			},
			Service:         "pagerduty",
			PayloadTemplate: "{}",
		}
		err := nm.SendEnhancedWebhook(webhook, &alerts.Alert{})
		assert.NoError(t, err)
	})
}

func TestEnhancedWebhook_ntfy(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "text/plain", r.Header.Get("Content-Type"))
		assert.NotEmpty(t, r.Header.Get("Title"))
		assert.NotEmpty(t, r.Header.Get("Priority"))
		assert.NotEmpty(t, r.Header.Get("Tags"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			URL: server.URL,
		},
		Service: "ntfy",
		// ntfy doesn't need valid JSON
		PayloadTemplate: "Test Alert",
	}

	status, _, err := nm.TestEnhancedWebhook(webhook)
	assert.NoError(t, err)
	assert.Equal(t, 200, status)
}

func TestIsRetryableWebhookErrorEnhanced(t *testing.T) {
	tests := []struct {
		err      string
		expected bool
	}{
		{"timeout", true},
		{"connection refused", true},
		{"status 429", true},
		{"status 502", true},
		{"status 404", false},
		{"status 400", false},
	}

	for _, tt := range tests {
		t.Run(tt.err, func(t *testing.T) {
			result := isRetryableWebhookError(fmt.Errorf("%s", tt.err))
			assert.Equal(t, tt.expected, result)
		})
	}
}
