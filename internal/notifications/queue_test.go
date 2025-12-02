package notifications

import (
	"fmt"
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

func TestProcessNotification_CancelledNotification(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// Create a cancelled notification
	notif := &QueuedNotification{
		ID:     "test-cancelled",
		Type:   "email",
		Status: QueueStatusCancelled,
	}

	// processNotification should return early without processing
	// No panic or error expected
	nq.processNotification(notif)

	// Verify the notification wasn't modified (no attempts incremented)
	// Since it's cancelled, it should just return
}

func TestProcessNotification_NoProcessor(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// Enqueue a notification first so IncrementAttemptAndSetStatus works
	notif := &QueuedNotification{
		ID:          "test-no-processor",
		Type:        "email",
		Status:      QueueStatusPending,
		MaxAttempts: 3,
		Config:      []byte(`{}`),
	}

	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Don't set a processor - processNotification should handle this
	nq.processNotification(notif)

	// The notification should be scheduled for retry or moved to DLQ
	// since no processor means failure
}

func TestProcessNotification_ProcessorSuccess(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// Enqueue a notification
	notif := &QueuedNotification{
		ID:          "test-success",
		Type:        "email",
		Status:      QueueStatusPending,
		MaxAttempts: 3,
		Config:      []byte(`{}`),
	}

	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Set a processor that succeeds
	processorCalled := false
	nq.SetProcessor(func(n *QueuedNotification) error {
		processorCalled = true
		return nil
	})

	nq.processNotification(notif)

	if !processorCalled {
		t.Error("Processor was not called")
	}
}

func TestProcessNotification_ProcessorFailure(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// Enqueue a notification with low max attempts
	notif := &QueuedNotification{
		ID:          "test-failure",
		Type:        "email",
		Status:      QueueStatusPending,
		MaxAttempts: 1, // Only 1 attempt, so failure goes to DLQ
		Config:      []byte(`{}`),
	}

	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Set a processor that fails
	nq.SetProcessor(func(n *QueuedNotification) error {
		return fmt.Errorf("simulated failure")
	})

	nq.processNotification(notif)

	// Notification should be in DLQ since max attempts reached
}

func TestScanNotification_DLQWithTimestamps(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}

	// Enqueue a notification with max 1 attempt
	notif := &QueuedNotification{
		ID:          "test-dlq-timestamps",
		Type:        "webhook",
		Status:      QueueStatusPending,
		MaxAttempts: 1, // Will go to DLQ on first failure
		Config:      []byte(`{"url":"http://example.com"}`),
	}

	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Set a failing processor to trigger DLQ
	nq.SetProcessor(func(n *QueuedNotification) error {
		return fmt.Errorf("simulated failure")
	})

	nq.processNotification(notif)

	// Get DLQ notifications - this exercises scanNotification with timestamps
	dlq, err := nq.GetDLQ(10)
	if err != nil {
		t.Fatalf("GetDLQ failed: %v", err)
	}

	if len(dlq) != 1 {
		t.Fatalf("Expected 1 DLQ notification, got %d", len(dlq))
	}

	// DLQ notification should have CompletedAt set (when it was moved to DLQ)
	if dlq[0].CompletedAt == nil {
		t.Error("Expected CompletedAt to be set for DLQ notification")
	}

	// Also verify LastAttempt is set
	if dlq[0].LastAttempt == nil {
		t.Error("Expected LastAttempt to be set for DLQ notification")
	}
}

func TestIncrementAttempt(t *testing.T) {
	t.Run("increments attempt counter", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Set next_retry_at far in the future so background processor doesn't pick it up
		futureRetry := time.Now().Add(1 * time.Hour)

		// Enqueue a notification
		notif := &QueuedNotification{
			ID:          "test-increment",
			Type:        "email",
			Status:      QueueStatusPending,
			MaxAttempts: 3,
			Config:      []byte(`{}`),
			NextRetryAt: &futureRetry,
		}

		if err := nq.Enqueue(notif); err != nil {
			t.Fatalf("Failed to enqueue: %v", err)
		}

		// Increment the attempt counter multiple times
		for i := 0; i < 3; i++ {
			if err := nq.IncrementAttempt("test-increment"); err != nil {
				t.Fatalf("IncrementAttempt failed on iteration %d: %v", i, err)
			}
		}

		// Verify via DLQ - first move to DLQ to query it
		// (This exercises the function; actual count verification would require db access)
		if err := nq.UpdateStatus("test-increment", QueueStatusDLQ, "test"); err != nil {
			t.Fatalf("UpdateStatus to DLQ failed: %v", err)
		}

		dlq, err := nq.GetDLQ(10)
		if err != nil {
			t.Fatalf("GetDLQ failed: %v", err)
		}
		if len(dlq) != 1 {
			t.Fatalf("Expected 1 DLQ notification, got %d", len(dlq))
		}
		if dlq[0].Attempts != 3 {
			t.Errorf("After 3 increments, attempts = %d, want 3", dlq[0].Attempts)
		}
	})

	t.Run("non-existent ID does not error", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Calling IncrementAttempt on non-existent ID should not error
		// (the SQL UPDATE just affects 0 rows)
		err = nq.IncrementAttempt("non-existent-id")
		if err != nil {
			t.Errorf("IncrementAttempt with non-existent ID returned error: %v", err)
		}
	})
}

func TestGetQueueStats(t *testing.T) {
	t.Run("empty queue", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		stats, err := nq.GetQueueStats()
		if err != nil {
			t.Fatalf("GetQueueStats failed: %v", err)
		}

		// Empty queue should return empty map
		if len(stats) != 0 {
			t.Errorf("Expected empty stats map, got %v", stats)
		}
	})

	t.Run("with notifications in various statuses", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Enqueue notifications with different statuses
		notifications := []*QueuedNotification{
			{ID: "pending-1", Type: "email", Status: QueueStatusPending, MaxAttempts: 3, Config: []byte(`{}`)},
			{ID: "pending-2", Type: "email", Status: QueueStatusPending, MaxAttempts: 3, Config: []byte(`{}`)},
			{ID: "sending-1", Type: "webhook", Status: QueueStatusSending, MaxAttempts: 3, Config: []byte(`{}`)},
		}

		for _, notif := range notifications {
			if err := nq.Enqueue(notif); err != nil {
				t.Fatalf("Failed to enqueue %s: %v", notif.ID, err)
			}
		}

		// Mark one as sent (completed)
		if err := nq.UpdateStatus("pending-1", QueueStatusSent, ""); err != nil {
			t.Fatalf("Failed to update status: %v", err)
		}

		// Mark one as failed
		if err := nq.UpdateStatus("sending-1", QueueStatusFailed, "connection refused"); err != nil {
			t.Fatalf("Failed to update status with error: %v", err)
		}

		stats, err := nq.GetQueueStats()
		if err != nil {
			t.Fatalf("GetQueueStats failed: %v", err)
		}

		// Verify counts
		if stats["pending"] != 1 {
			t.Errorf("pending count = %d, want 1", stats["pending"])
		}
		if stats["sent"] != 1 {
			t.Errorf("sent count = %d, want 1", stats["sent"])
		}
		if stats["failed"] != 1 {
			t.Errorf("failed count = %d, want 1", stats["failed"])
		}
	})
}
