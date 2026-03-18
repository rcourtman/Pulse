package notifications

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnhancedWebhook(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

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

func TestParseRetryAfterBackoff(t *testing.T) {
	now := time.Date(2026, time.March, 13, 19, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		retryAfter string
		want       time.Duration
		ok         bool
	}{
		{
			name:       "delta seconds",
			retryAfter: "3",
			want:       3 * time.Second,
			ok:         true,
		},
		{
			name:       "delta seconds with whitespace",
			retryAfter: "  3  ",
			want:       3 * time.Second,
			ok:         true,
		},
		{
			name:       "negative delta treated as immediate",
			retryAfter: "-5",
			want:       0,
			ok:         true,
		},
		{
			name:       "http date",
			retryAfter: "Fri, 13 Mar 2026 19:00:02 GMT",
			want:       2 * time.Second,
			ok:         true,
		},
		{
			name:       "http date with whitespace",
			retryAfter: "  Fri, 13 Mar 2026 19:00:02 GMT  ",
			want:       2 * time.Second,
			ok:         true,
		},
		{
			name:       "delta seconds capped at max backoff",
			retryAfter: "999",
			want:       WebhookMaxBackoff,
			ok:         true,
		},
		{
			name:       "http date capped at max backoff",
			retryAfter: "Fri, 13 Mar 2026 19:05:00 GMT",
			want:       WebhookMaxBackoff,
			ok:         true,
		},
		{
			name:       "past http date treated as immediate",
			retryAfter: "Fri, 13 Mar 2026 18:59:58 GMT",
			want:       0,
			ok:         true,
		},
		{
			name:       "invalid value",
			retryAfter: "not-a-number",
			want:       0,
			ok:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseRetryAfterBackoff(tt.retryAfter, now)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSendWebhookWithRetry_429RetryAfter(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

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

func TestSendWebhookWithRetry_InvalidRetryAfterFallsBackToExponentialBackoff(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	attemptTimes := make([]time.Time, 0, 2)
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		attemptTimes = append(attemptTimes, time.Now())
		if attempts == 1 {
			w.Header().Set("Retry-After", "not-a-number")
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
		RetryCount:   1,
	}

	start := time.Now()
	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	elapsed := time.Since(start)
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
	if assert.Len(t, attemptTimes, 2) {
		assert.GreaterOrEqual(t, attemptTimes[1].Sub(attemptTimes[0]), 900*time.Millisecond)
	}
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond)
}

func TestSendWebhookWithRetry_RetriesRequestTimeoutResponses(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Timeout Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   1,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestSendWebhookWithRetry_RetriesMisdirectedRequestResponses(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusMisdirectedRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Misdirected Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   1,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestSendWebhookWithRetry_RetriesLockedResponses(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusLocked)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Locked Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   1,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestSendWebhookWithRetry_RetriesTooEarlyResponses(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooEarly)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Too Early Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   1,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.NoError(t, err)
	assert.Equal(t, 2, attempts)
}

func TestSendWebhookWithRetry_StopsOnNonRetryableErrorAfterRetryable(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	attempts := 0
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		switch attempts {
		case 1:
			// Keep test fast by making the retry delay zero.
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Test Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   5,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")
	assert.Contains(t, err.Error(), "after 2 attempts")
	assert.Equal(t, 2, attempts)
}

func TestSendWebhookWithRetry_PreservesSuccessfulStatusCodeInHistory(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Accepted Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   1,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.NoError(t, err)

	history := nm.GetWebhookHistory()
	if assert.Len(t, history, 1) {
		assert.Equal(t, http.StatusAccepted, history[0].StatusCode)
		assert.True(t, history[0].Success)
		assert.Equal(t, "Accepted Webhook", history[0].WebhookName)
	}
}

func TestSendWebhookWithRetry_PreservesFailedStatusCodeInHistory(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Rejected Webhook",
			URL:  server.URL,
		},
		RetryEnabled: true,
		RetryCount:   0,
	}

	err := nm.sendWebhookWithRetry(webhook, []byte(`{"test":true}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status 400")

	history := nm.GetWebhookHistory()
	if assert.Len(t, history, 1) {
		assert.Equal(t, http.StatusBadRequest, history[0].StatusCode)
		assert.False(t, history[0].Success)
		assert.Equal(t, "Rejected Webhook", history[0].WebhookName)
	}
}

func TestSendEnhancedWebhook_TemplateRendering(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

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
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

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
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

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

func TestPrepareEnhancedWebhookExecution_UsesCanonicalDeliveryContext(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Telegram Test",
			URL:  "https://example.com/hook?chat_id=12345",
		},
		Service:         "telegram",
		PayloadTemplate: `{"chat_id":"{{.ChatID}}","alert":"{{.ID}}"}`,
	}

	alert := &alerts.Alert{
		ID:   "alert-123",
		Node: "node-a",
	}

	prepared, payload, err := nm.prepareEnhancedWebhookExecution(webhook, alert)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/hook?chat_id=12345", prepared.URL)
	assert.JSONEq(t, `{"chat_id":"12345","alert":"alert-123"}`, string(payload))
}

func TestPrepareEnhancedWebhookExecution_NtfyPreservesPlainTextPayload(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "ntfy Test",
			URL:  "https://example.com/topic",
		},
		Service:         "ntfy",
		PayloadTemplate: "Alert for {{.ResourceName}} on {{.Node}}",
	}

	alert := &alerts.Alert{
		ResourceName: "vm-101",
		Node:         "pve-01",
	}

	prepared, payload, err := nm.prepareEnhancedWebhookExecution(webhook, alert)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/topic", prepared.URL)
	assert.Equal(t, "Alert for vm-101 on pve-01", string(payload))
}

func TestBuildEnhancedWebhookTestConfig_UsesServiceTemplateOwnership(t *testing.T) {
	basic := WebhookConfig{
		Name: "Discord test",
		URL:  "https://example.com/hook",
		Headers: map[string]string{
			"X-Custom": "value",
		},
	}

	webhook := BuildEnhancedWebhookTestConfig(basic, "discord")

	assert.Equal(t, "discord", webhook.Service)
	assert.NotEmpty(t, webhook.PayloadTemplate)
	assert.Equal(t, "value", webhook.Headers["X-Custom"])
	assert.Equal(t, "application/json", webhook.Headers["Content-Type"])
}

func TestBuildEnhancedWebhookTestConfig_FallsBackToGenericTemplate(t *testing.T) {
	basic := WebhookConfig{
		Name: "Unknown service test",
		URL:  "https://example.com/hook",
	}

	webhook := BuildEnhancedWebhookTestConfig(basic, "unknown-service")

	assert.Equal(t, "unknown-service", webhook.Service)
	assert.Contains(t, webhook.PayloadTemplate, `"source": "pulse-monitoring"`)
	assert.Contains(t, webhook.PayloadTemplate, `"resourceName": "{{.ResourceName}}"`)
}

func TestEnhancedWebhook_UsesCanonicalTransportSanitization(t *testing.T) {
	nm := NewNotificationManager("http://pulse.local")
	_ = nm.UpdateAllowedPrivateCIDRs("127.0.0.1")

	var gotQuery string
	var gotBody []byte
	server := newIPv4HTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = body
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	webhook := EnhancedWebhookConfig{
		WebhookConfig: WebhookConfig{
			Name: "Telegram Test",
			URL:  server.URL + "/hook?chat_id=12345",
		},
		Service:         "telegram",
		PayloadTemplate: `{"chat_id":"{{.ChatID}}","alert":"{{.ID}}"}`,
	}

	alert := &alerts.Alert{
		ID:   "alert-telegram-1",
		Node: "node-a",
	}

	err := nm.SendEnhancedWebhook(webhook, alert)
	require.NoError(t, err)
	assert.Empty(t, gotQuery)
	assert.JSONEq(t, `{"chat_id":"12345","alert":"alert-telegram-1"}`, string(gotBody))
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
