package notifications

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "attempt 0 (first retry)",
			attempt:  0,
			expected: 1 * time.Second,
		},
		{
			name:     "attempt 1",
			attempt:  1,
			expected: 2 * time.Second,
		},
		{
			name:     "attempt 2",
			attempt:  2,
			expected: 4 * time.Second,
		},
		{
			name:     "attempt 3",
			attempt:  3,
			expected: 8 * time.Second,
		},
		{
			name:     "attempt 4",
			attempt:  4,
			expected: 16 * time.Second,
		},
		{
			name:     "attempt 5",
			attempt:  5,
			expected: 32 * time.Second,
		},
		{
			name:     "attempt 6 (capped at 60s)",
			attempt:  6,
			expected: 60 * time.Second,
		},
		{
			name:     "attempt 7 (stays at cap)",
			attempt:  7,
			expected: 60 * time.Second,
		},
		{
			name:     "attempt 10 (stays at cap)",
			attempt:  10,
			expected: 60 * time.Second,
		},
		// Note: For very large attempt numbers (>= 60 on 64-bit), bit shift
		// overflows causing duration to be 0. In practice this never happens
		// as max_attempts is typically 3-10.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := calculateBackoff(tc.attempt)
			if result != tc.expected {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tc.attempt, result, tc.expected)
			}
		})
	}
}

func TestCalculateBackoff_ExponentialGrowth(t *testing.T) {
	// Verify backoff grows exponentially until cap
	prev := calculateBackoff(0)
	for attempt := 1; attempt <= 5; attempt++ {
		curr := calculateBackoff(attempt)
		if curr != prev*2 {
			t.Errorf("calculateBackoff(%d) = %v, expected %v (2x previous)", attempt, curr, prev*2)
		}
		prev = curr
	}
}

func TestCalculateBackoff_NeverExceedsCap(t *testing.T) {
	cap := 60 * time.Second
	// Test a range of practical attempt values (0-20 is realistic range)
	for attempt := 0; attempt <= 20; attempt++ {
		result := calculateBackoff(attempt)
		if result > cap {
			t.Errorf("calculateBackoff(%d) = %v, exceeds cap of %v", attempt, result, cap)
		}
	}
}

func TestNotificationQueueStatus_Values(t *testing.T) {
	// Verify status constants have expected string values
	tests := []struct {
		status   NotificationQueueStatus
		expected string
	}{
		{QueueStatusPending, "pending"},
		{QueueStatusSending, "sending"},
		{QueueStatusSent, "sent"},
		{QueueStatusFailed, "failed"},
		{QueueStatusDLQ, "dlq"},
		{QueueStatusCancelled, "cancelled"},
	}

	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			if string(tc.status) != tc.expected {
				t.Errorf("status = %q, want %q", tc.status, tc.expected)
			}
		})
	}
}

func TestQueuedNotification_Fields(t *testing.T) {
	now := time.Now()
	lastAttempt := now.Add(-1 * time.Minute)
	nextRetry := now.Add(5 * time.Minute)
	errorMsg := "connection refused"

	notif := QueuedNotification{
		ID:          "test-123",
		Type:        "email",
		Method:      "smtp",
		Status:      QueueStatusPending,
		Alerts:      nil,
		Config:      []byte(`{"host":"smtp.example.com"}`),
		Attempts:    2,
		MaxAttempts: 5,
		LastAttempt: &lastAttempt,
		LastError:   &errorMsg,
		CreatedAt:   now,
		NextRetryAt: &nextRetry,
	}

	if notif.ID != "test-123" {
		t.Errorf("ID = %q, want 'test-123'", notif.ID)
	}
	if notif.Type != "email" {
		t.Errorf("Type = %q, want 'email'", notif.Type)
	}
	if notif.Method != "smtp" {
		t.Errorf("Method = %q, want 'smtp'", notif.Method)
	}
	if notif.Status != QueueStatusPending {
		t.Errorf("Status = %q, want 'pending'", notif.Status)
	}
	if notif.Attempts != 2 {
		t.Errorf("Attempts = %d, want 2", notif.Attempts)
	}
	if notif.MaxAttempts != 5 {
		t.Errorf("MaxAttempts = %d, want 5", notif.MaxAttempts)
	}
	if notif.LastAttempt == nil {
		t.Error("LastAttempt should not be nil")
	}
	if notif.LastError == nil || *notif.LastError != "connection refused" {
		t.Errorf("LastError = %v, want 'connection refused'", notif.LastError)
	}
	if notif.NextRetryAt == nil {
		t.Error("NextRetryAt should not be nil")
	}
}

func TestQueuedNotification_ZeroValues(t *testing.T) {
	notif := QueuedNotification{}

	if notif.ID != "" {
		t.Error("ID should be empty by default")
	}
	if notif.Type != "" {
		t.Error("Type should be empty by default")
	}
	if notif.Status != "" {
		t.Error("Status should be empty by default")
	}
	if notif.Attempts != 0 {
		t.Error("Attempts should be 0 by default")
	}
	if notif.MaxAttempts != 0 {
		t.Error("MaxAttempts should be 0 by default")
	}
	if notif.LastAttempt != nil {
		t.Error("LastAttempt should be nil by default")
	}
	if notif.LastError != nil {
		t.Error("LastError should be nil by default")
	}
	if !notif.CreatedAt.IsZero() {
		t.Error("CreatedAt should be zero by default")
	}
	if notif.NextRetryAt != nil {
		t.Error("NextRetryAt should be nil by default")
	}
	if notif.CompletedAt != nil {
		t.Error("CompletedAt should be nil by default")
	}
}

func TestCancelByAlertIDs_EmptyInput(t *testing.T) {
	// Create a temporary queue for testing
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// Empty slice should return nil without error
	err = nq.CancelByAlertIDs([]string{})
	if err != nil {
		t.Errorf("CancelByAlertIDs with empty slice returned error: %v", err)
	}

	// Nil slice should also return nil without error
	err = nq.CancelByAlertIDs(nil)
	if err != nil {
		t.Errorf("CancelByAlertIDs with nil slice returned error: %v", err)
	}
}

func TestCancelByAlertIDs_NoMatchingNotifications(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// With an empty queue, no notifications should match
	err = nq.CancelByAlertIDs([]string{"alert-1", "alert-2"})
	if err != nil {
		t.Errorf("CancelByAlertIDs with no matching notifications returned error: %v", err)
	}
}
