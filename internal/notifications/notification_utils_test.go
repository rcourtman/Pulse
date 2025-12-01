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
		name     string
		alerts   []*alerts.Alert
		checkFn  func(*testing.T, time.Time)
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
