package notifications

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
)

func TestAnnotateResolvedMetadata(t *testing.T) {
	tests := []struct {
		name       string
		alert      *alerts.Alert
		resolvedAt time.Time
		checkFn    func(*testing.T, *alerts.Alert)
	}{
		{
			name:       "nil alert",
			alert:      nil,
			resolvedAt: time.Now(),
			checkFn: func(t *testing.T, a *alerts.Alert) {
				// Should not panic, nothing to check
			},
		},
		{
			name:       "alert with nil metadata",
			alert:      &alerts.Alert{ID: "test-1"},
			resolvedAt: time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			checkFn: func(t *testing.T, a *alerts.Alert) {
				if a.Metadata == nil {
					t.Error("Metadata should be initialized")
					return
				}
				raw, ok := a.Metadata[metadataResolvedAt]
				if !ok {
					t.Error("resolvedAt key should be set")
					return
				}
				ts, ok := raw.(string)
				if !ok {
					t.Errorf("resolvedAt should be string, got %T", raw)
					return
				}
				expected := "2025-01-15T10:30:00Z"
				if ts != expected {
					t.Errorf("resolvedAt = %q, want %q", ts, expected)
				}
			},
		},
		{
			name: "alert with existing metadata",
			alert: &alerts.Alert{
				ID: "test-2",
				Metadata: map[string]interface{}{
					"existingKey": "existingValue",
				},
			},
			resolvedAt: time.Date(2025, 6, 20, 15, 45, 30, 0, time.UTC),
			checkFn: func(t *testing.T, a *alerts.Alert) {
				// Should preserve existing metadata
				if v, ok := a.Metadata["existingKey"]; !ok || v != "existingValue" {
					t.Error("existing metadata should be preserved")
				}
				// Should add resolvedAt
				raw, ok := a.Metadata[metadataResolvedAt]
				if !ok {
					t.Error("resolvedAt key should be set")
					return
				}
				ts, ok := raw.(string)
				if !ok {
					t.Errorf("resolvedAt should be string, got %T", raw)
					return
				}
				expected := "2025-06-20T15:45:30Z"
				if ts != expected {
					t.Errorf("resolvedAt = %q, want %q", ts, expected)
				}
			},
		},
		{
			name: "overwrites existing resolvedAt",
			alert: &alerts.Alert{
				ID: "test-3",
				Metadata: map[string]interface{}{
					metadataResolvedAt: "old-value",
				},
			},
			resolvedAt: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			checkFn: func(t *testing.T, a *alerts.Alert) {
				raw := a.Metadata[metadataResolvedAt]
				ts, ok := raw.(string)
				if !ok {
					t.Errorf("resolvedAt should be string, got %T", raw)
					return
				}
				expected := "2025-12-01T00:00:00Z"
				if ts != expected {
					t.Errorf("resolvedAt = %q, want %q (should overwrite old value)", ts, expected)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			annotateResolvedMetadata(tc.alert, tc.resolvedAt)
			tc.checkFn(t, tc.alert)
		})
	}
}

func TestResolveAppriseNotificationType(t *testing.T) {
	tests := []struct {
		name     string
		alerts   []*alerts.Alert
		expected string
	}{
		{
			name:     "nil slice",
			alerts:   nil,
			expected: "info",
		},
		{
			name:     "empty slice",
			alerts:   []*alerts.Alert{},
			expected: "info",
		},
		{
			name:     "slice with nil alert",
			alerts:   []*alerts.Alert{nil},
			expected: "info",
		},
		{
			name:     "slice with multiple nil alerts",
			alerts:   []*alerts.Alert{nil, nil, nil},
			expected: "info",
		},
		{
			name: "single info-level alert (no level set)",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: ""},
			},
			expected: "info",
		},
		{
			name: "single warning alert",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: alerts.AlertLevelWarning},
			},
			expected: "warning",
		},
		{
			name: "single critical alert",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: alerts.AlertLevelCritical},
			},
			expected: "failure",
		},
		{
			name: "multiple warnings",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: alerts.AlertLevelWarning},
				{ID: "test-2", Level: alerts.AlertLevelWarning},
			},
			expected: "warning",
		},
		{
			name: "warning and critical - returns failure (critical takes priority)",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: alerts.AlertLevelWarning},
				{ID: "test-2", Level: alerts.AlertLevelCritical},
			},
			expected: "failure",
		},
		{
			name: "critical first - returns failure immediately",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: alerts.AlertLevelCritical},
				{ID: "test-2", Level: alerts.AlertLevelWarning},
			},
			expected: "failure",
		},
		{
			name: "mixed with nil - critical takes priority",
			alerts: []*alerts.Alert{
				nil,
				{ID: "test-1", Level: alerts.AlertLevelWarning},
				nil,
				{ID: "test-2", Level: alerts.AlertLevelCritical},
			},
			expected: "failure",
		},
		{
			name: "info and warning - warning takes priority",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: ""},
				{ID: "test-2", Level: alerts.AlertLevelWarning},
				{ID: "test-3", Level: ""},
			},
			expected: "warning",
		},
		{
			name: "unknown level treated as info",
			alerts: []*alerts.Alert{
				{ID: "test-1", Level: "unknown"},
			},
			expected: "info",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveAppriseNotificationType(tc.alerts)
			if result != tc.expected {
				t.Errorf("resolveAppriseNotificationType() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestNormalizeQueueType(t *testing.T) {
	tests := []struct {
		name          string
		notifType     string
		expectedType  string
		expectedEvent notificationEvent
	}{
		{
			name:          "email type",
			notifType:     "email",
			expectedType:  "email",
			expectedEvent: eventAlert,
		},
		{
			name:          "webhook type",
			notifType:     "webhook",
			expectedType:  "webhook",
			expectedEvent: eventAlert,
		},
		{
			name:          "apprise type",
			notifType:     "apprise",
			expectedType:  "apprise",
			expectedEvent: eventAlert,
		},
		{
			name:          "email_resolved type",
			notifType:     "email_resolved",
			expectedType:  "email",
			expectedEvent: eventResolved,
		},
		{
			name:          "webhook_resolved type",
			notifType:     "webhook_resolved",
			expectedType:  "webhook",
			expectedEvent: eventResolved,
		},
		{
			name:          "apprise_resolved type",
			notifType:     "apprise_resolved",
			expectedType:  "apprise",
			expectedEvent: eventResolved,
		},
		{
			name:          "empty type",
			notifType:     "",
			expectedType:  "",
			expectedEvent: eventAlert,
		},
		{
			name:          "unknown type",
			notifType:     "unknown",
			expectedType:  "unknown",
			expectedEvent: eventAlert,
		},
		{
			name:          "unknown_resolved type",
			notifType:     "unknown_resolved",
			expectedType:  "unknown",
			expectedEvent: eventResolved,
		},
		{
			name:          "type with _resolved in middle - not stripped",
			notifType:     "_resolved_email",
			expectedType:  "_resolved_email",
			expectedEvent: eventAlert,
		},
		{
			name:          "just _resolved suffix",
			notifType:     "_resolved",
			expectedType:  "",
			expectedEvent: eventResolved,
		},
		{
			name:          "email_escalation type",
			notifType:     "email_escalation",
			expectedType:  "email",
			expectedEvent: eventEscalation,
		},
		{
			name:          "webhook_escalation type",
			notifType:     "webhook_escalation",
			expectedType:  "webhook",
			expectedEvent: eventEscalation,
		},
		{
			name:          "apprise_escalation type",
			notifType:     "apprise_escalation",
			expectedType:  "apprise",
			expectedEvent: eventEscalation,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotEvent := normalizeQueueType(tc.notifType)
			if gotType != tc.expectedType {
				t.Errorf("normalizeQueueType() type = %q, want %q", gotType, tc.expectedType)
			}
			if gotEvent != tc.expectedEvent {
				t.Errorf("normalizeQueueType() event = %q, want %q", gotEvent, tc.expectedEvent)
			}
		})
	}
}

func TestResolvedTimeFromAlerts(t *testing.T) {
	fixedTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	fixedTimeStr := fixedTime.Format(time.RFC3339)

	tests := []struct {
		name    string
		alerts  []*alerts.Alert
		checkFn func(*testing.T, time.Time)
	}{
		{
			name:   "nil slice - returns current time",
			alerts: nil,
			checkFn: func(t *testing.T, result time.Time) {
				// Should return a time close to now
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for nil slice")
				}
			},
		},
		{
			name:   "empty slice - returns current time",
			alerts: []*alerts.Alert{},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for empty slice")
				}
			},
		},
		{
			name:   "slice with nil alert - returns current time",
			alerts: []*alerts.Alert{nil},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for nil alert")
				}
			},
		},
		{
			name: "alert with nil metadata - returns current time",
			alerts: []*alerts.Alert{
				{ID: "test-1", Metadata: nil},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for nil metadata")
				}
			},
		},
		{
			name: "alert without resolvedAt key - returns current time",
			alerts: []*alerts.Alert{
				{ID: "test-1", Metadata: map[string]interface{}{"other": "value"}},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for missing resolvedAt")
				}
			},
		},
		{
			name: "alert with string resolvedAt (RFC3339)",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: fixedTimeStr,
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if !result.Equal(fixedTime) {
					t.Errorf("got %v, want %v", result, fixedTime)
				}
			},
		},
		{
			name: "alert with float64 resolvedAt (Unix timestamp)",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: float64(fixedTime.Unix()),
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				// Unix timestamp loses nanoseconds
				expected := time.Unix(fixedTime.Unix(), 0)
				if !result.Equal(expected) {
					t.Errorf("got %v, want %v", result, expected)
				}
			},
		},
		{
			name: "alert with zero float64 - returns current time",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: float64(0),
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for zero timestamp")
				}
			},
		},
		{
			name: "alert with negative float64 - returns current time",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: float64(-1000),
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for negative timestamp")
				}
			},
		},
		{
			name: "alert with invalid string format - returns current time",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: "not-a-timestamp",
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for invalid string")
				}
			},
		},
		{
			name: "alert with unsupported type - returns current time",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: 12345, // int, not float64
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if time.Since(result) > time.Second {
					t.Error("expected time close to now for unsupported type")
				}
			},
		},
		{
			name: "multiple alerts - returns first valid resolvedAt",
			alerts: []*alerts.Alert{
				nil,
				{ID: "test-1", Metadata: nil},
				{ID: "test-2", Metadata: map[string]interface{}{}},
				{
					ID: "test-3",
					Metadata: map[string]interface{}{
						metadataResolvedAt: fixedTimeStr,
					},
				},
				{
					ID: "test-4",
					Metadata: map[string]interface{}{
						metadataResolvedAt: "2024-01-01T00:00:00Z", // should not be reached
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if !result.Equal(fixedTime) {
					t.Errorf("got %v, want %v (first valid)", result, fixedTime)
				}
			},
		},
		{
			name: "first alert has valid resolvedAt",
			alerts: []*alerts.Alert{
				{
					ID: "test-1",
					Metadata: map[string]interface{}{
						metadataResolvedAt: fixedTimeStr,
					},
				},
			},
			checkFn: func(t *testing.T, result time.Time) {
				if !result.Equal(fixedTime) {
					t.Errorf("got %v, want %v", result, fixedTime)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolvedTimeFromAlerts(tc.alerts)
			tc.checkFn(t, result)
		})
	}
}

// Test the event constants
func TestNotificationEventConstants(t *testing.T) {
	if eventAlert != "alert" {
		t.Errorf("eventAlert = %q, want %q", eventAlert, "alert")
	}
	if eventResolved != "resolved" {
		t.Errorf("eventResolved = %q, want %q", eventResolved, "resolved")
	}
}

// Test the queue type suffix constant
func TestQueueTypeSuffixConstant(t *testing.T) {
	if queueTypeSuffixResolved != "_resolved" {
		t.Errorf("queueTypeSuffixResolved = %q, want %q", queueTypeSuffixResolved, "_resolved")
	}
}

// Test the metadata key constant
func TestMetadataKeyConstant(t *testing.T) {
	if metadataResolvedAt != "resolvedAt" {
		t.Errorf("metadataResolvedAt = %q, want %q", metadataResolvedAt, "resolvedAt")
	}
}

func TestCopyEmailConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  EmailConfig
	}{
		{
			name: "empty config",
			cfg:  EmailConfig{},
		},
		{
			name: "config with empty To slice",
			cfg: EmailConfig{
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				SMTPPort: 587,
				From:     "sender@example.com",
				To:       []string{},
			},
		},
		{
			name: "config with single recipient",
			cfg: EmailConfig{
				Enabled:  true,
				SMTPHost: "smtp.example.com",
				SMTPPort: 465,
				Username: "user",
				Password: "pass",
				From:     "sender@example.com",
				To:       []string{"recipient@example.com"},
			},
		},
		{
			name: "config with multiple recipients",
			cfg: EmailConfig{
				Enabled:  true,
				SMTPHost: "mail.company.org",
				SMTPPort: 25,
				From:     "alerts@company.org",
				To:       []string{"admin@company.org", "ops@company.org", "devops@company.org"},
				TLS:      true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			copied := copyEmailConfig(tc.cfg)

			// Verify fields are equal
			if copied.Enabled != tc.cfg.Enabled {
				t.Errorf("Enabled = %v, want %v", copied.Enabled, tc.cfg.Enabled)
			}
			if copied.SMTPHost != tc.cfg.SMTPHost {
				t.Errorf("SMTPHost = %q, want %q", copied.SMTPHost, tc.cfg.SMTPHost)
			}
			if copied.SMTPPort != tc.cfg.SMTPPort {
				t.Errorf("SMTPPort = %d, want %d", copied.SMTPPort, tc.cfg.SMTPPort)
			}
			if copied.From != tc.cfg.From {
				t.Errorf("From = %q, want %q", copied.From, tc.cfg.From)
			}
			if len(copied.To) != len(tc.cfg.To) {
				t.Errorf("To length = %d, want %d", len(copied.To), len(tc.cfg.To))
			}

			// Verify slice independence (if original has elements)
			if len(tc.cfg.To) > 0 {
				originalTo := tc.cfg.To[0]
				copied.To[0] = "modified@example.com"
				if tc.cfg.To[0] != originalTo {
					t.Error("Modifying copied.To should not affect original")
				}
			}
		})
	}
}

func TestCopyWebhookConfigs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		webhooks []WebhookConfig
		wantNil  bool
	}{
		{
			name:     "nil input",
			webhooks: nil,
			wantNil:  true,
		},
		{
			name:     "empty slice",
			webhooks: []WebhookConfig{},
			wantNil:  true,
		},
		{
			name: "single webhook without maps",
			webhooks: []WebhookConfig{
				{
					Enabled: true,
					URL:     "https://hooks.example.com/webhook",
					Method:  "POST",
				},
			},
		},
		{
			name: "webhook with headers",
			webhooks: []WebhookConfig{
				{
					Enabled: true,
					URL:     "https://api.example.com/alerts",
					Method:  "POST",
					Headers: map[string]string{
						"Authorization": "Bearer token123",
						"Content-Type":  "application/json",
					},
				},
			},
		},
		{
			name: "webhook with custom fields",
			webhooks: []WebhookConfig{
				{
					Enabled: true,
					URL:     "https://pushover.net/api",
					CustomFields: map[string]string{
						"priority": "1",
						"sound":    "alarm",
					},
				},
			},
		},
		{
			name: "multiple webhooks with all fields",
			webhooks: []WebhookConfig{
				{
					Enabled:      true,
					URL:          "https://discord.com/api/webhooks/123",
					Headers:      map[string]string{"X-Custom": "value"},
					CustomFields: map[string]string{"key": "val"},
				},
				{
					Enabled: false,
					URL:     "https://slack.com/api/post",
					Method:  "POST",
					Headers: map[string]string{"Authorization": "xoxb-token"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			copied := copyWebhookConfigs(tc.webhooks)

			if tc.wantNil {
				if copied != nil {
					t.Errorf("Expected nil, got %v", copied)
				}
				return
			}

			if len(copied) != len(tc.webhooks) {
				t.Errorf("Length = %d, want %d", len(copied), len(tc.webhooks))
				return
			}

			// Verify each webhook
			for i := range tc.webhooks {
				if copied[i].URL != tc.webhooks[i].URL {
					t.Errorf("[%d] URL = %q, want %q", i, copied[i].URL, tc.webhooks[i].URL)
				}
				if copied[i].Enabled != tc.webhooks[i].Enabled {
					t.Errorf("[%d] Enabled = %v, want %v", i, copied[i].Enabled, tc.webhooks[i].Enabled)
				}

				// Verify headers independence
				if len(tc.webhooks[i].Headers) > 0 {
					for k := range tc.webhooks[i].Headers {
						originalVal := tc.webhooks[i].Headers[k]
						copied[i].Headers[k] = "modified"
						if tc.webhooks[i].Headers[k] != originalVal {
							t.Errorf("[%d] Modifying Headers should not affect original", i)
						}
						break // Test one key is enough
					}
				}

				// Verify custom fields independence
				if len(tc.webhooks[i].CustomFields) > 0 {
					for k := range tc.webhooks[i].CustomFields {
						originalVal := tc.webhooks[i].CustomFields[k]
						copied[i].CustomFields[k] = "modified"
						if tc.webhooks[i].CustomFields[k] != originalVal {
							t.Errorf("[%d] Modifying CustomFields should not affect original", i)
						}
						break
					}
				}
			}
		})
	}
}

func TestCopyAppriseConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  AppriseConfig
	}{
		{
			name: "empty config",
			cfg:  AppriseConfig{},
		},
		{
			name: "config with empty targets",
			cfg: AppriseConfig{
				Enabled: true,
				Targets: []string{},
			},
		},
		{
			name: "config with single target",
			cfg: AppriseConfig{
				Enabled: true,
				Targets: []string{"discord://webhook/id/token"},
			},
		},
		{
			name: "config with multiple targets",
			cfg: AppriseConfig{
				Enabled: true,
				Targets: []string{
					"slack://token/channel",
					"telegram://bot_token/chat_id",
					"email://user:pass@smtp.example.com",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			copied := copyAppriseConfig(tc.cfg)

			if copied.Enabled != tc.cfg.Enabled {
				t.Errorf("Enabled = %v, want %v", copied.Enabled, tc.cfg.Enabled)
			}
			if len(copied.Targets) != len(tc.cfg.Targets) {
				t.Errorf("Targets length = %d, want %d", len(copied.Targets), len(tc.cfg.Targets))
			}

			// Verify slice independence
			if len(tc.cfg.Targets) > 0 {
				originalTarget := tc.cfg.Targets[0]
				copied.Targets[0] = "modified://target"
				if tc.cfg.Targets[0] != originalTarget {
					t.Error("Modifying copied.Targets should not affect original")
				}
			}
		})
	}
}

func TestBuildNotificationTestAlert(t *testing.T) {
	t.Parallel()

	alert := buildNotificationTestAlert()

	// Verify required fields are set
	if alert.ID != "test-alert" {
		t.Errorf("ID = %q, want %q", alert.ID, "test-alert")
	}
	if alert.Type != "cpu" {
		t.Errorf("Type = %q, want %q", alert.Type, "cpu")
	}
	if alert.Level != "warning" {
		t.Errorf("Level = %q, want %q", alert.Level, "warning")
	}
	if alert.ResourceID != "test-resource" {
		t.Errorf("ResourceID = %q, want %q", alert.ResourceID, "test-resource")
	}
	if alert.ResourceName != "Test Resource" {
		t.Errorf("ResourceName = %q, want %q", alert.ResourceName, "Test Resource")
	}
	if alert.Node == "" {
		t.Error("Node should not be empty")
	}
	if alert.Instance == "" {
		t.Error("Instance should not be empty")
	}
	if alert.Message == "" {
		t.Error("Message should not be empty")
	}
	if alert.Value == 0 {
		t.Error("Value should not be zero")
	}
	if alert.Threshold == 0 {
		t.Error("Threshold should not be zero")
	}
	if alert.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
	if alert.LastSeen.IsZero() {
		t.Error("LastSeen should not be zero")
	}
	if alert.Metadata == nil {
		t.Error("Metadata should not be nil")
	}

	// Verify StartTime is in the past (shows alert has been active)
	if !alert.StartTime.Before(time.Now()) {
		t.Error("StartTime should be in the past")
	}

	// Verify metadata contains resourceType
	if rt, ok := alert.Metadata["resourceType"]; !ok || rt != "vm" {
		t.Errorf("Metadata[resourceType] = %v, want %q", rt, "vm")
	}
}
