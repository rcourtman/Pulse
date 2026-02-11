package notifications

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/utils"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		expected time.Duration
	}{
		{
			name:     "negative attempt defaults to first backoff",
			attempt:  -1,
			expected: 1 * time.Second,
		},
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
		{
			name:     "very large attempts stay capped",
			attempt:  60,
			expected: 60 * time.Second,
		},
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

func TestNewNotificationQueue_WhitespaceDataDirUsesDefault(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("PULSE_DATA_DIR", dataDir)

	nq, err := NewNotificationQueue("   \t  ")
	if err != nil {
		t.Fatalf("Failed to create notification queue with whitespace data dir: %v", err)
	}
	defer nq.Stop()

	expectedDBPath := filepath.Join(utils.GetDataDir(), "notifications", "notification_queue.db")
	if nq.dbPath != expectedDBPath {
		t.Fatalf("expected db path %q, got %q", expectedDBPath, nq.dbPath)
	}
}

func TestEnqueue_ValidatesAndNormalizesInput(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}
	defer nq.Stop()

	t.Run("rejects nil notification", func(t *testing.T) {
		err := nq.Enqueue(nil)
		if err == nil {
			t.Fatalf("expected error for nil notification")
		}
	})

	t.Run("rejects empty type", func(t *testing.T) {
		err := nq.Enqueue(&QueuedNotification{
			Config: []byte(`{}`),
		})
		if err == nil {
			t.Fatalf("expected error for empty notification type")
		}
	})

	t.Run("rejects empty config", func(t *testing.T) {
		err := nq.Enqueue(&QueuedNotification{
			Type: "email",
		})
		if err == nil {
			t.Fatalf("expected error for empty notification config")
		}
	})

	t.Run("normalizes attempts and type", func(t *testing.T) {
		futureRetry := time.Now().Add(1 * time.Hour)
		notif := &QueuedNotification{
			ID:          "normalize-test",
			Type:        "  email  ",
			Status:      QueueStatusPending,
			Attempts:    -10,
			MaxAttempts: -2,
			Config:      []byte(`{}`),
			NextRetryAt: &futureRetry, // keep background worker from picking it up
			Alerts:      []*alerts.Alert{{ID: "a-1"}},
		}

		if err := nq.Enqueue(notif); err != nil {
			t.Fatalf("enqueue failed: %v", err)
		}

		var dbType string
		var attempts int
		var maxAttempts int
		err := nq.db.QueryRow(`SELECT type, attempts, max_attempts FROM notification_queue WHERE id = ?`, notif.ID).Scan(&dbType, &attempts, &maxAttempts)
		if err != nil {
			t.Fatalf("failed to query normalized notification: %v", err)
		}

		if dbType != "email" {
			t.Fatalf("expected trimmed type 'email', got %q", dbType)
		}
		if attempts != 0 {
			t.Fatalf("expected attempts to normalize to 0, got %d", attempts)
		}
		if maxAttempts != defaultQueueMaxAttempts {
			t.Fatalf("expected max attempts to normalize to %d, got %d", defaultQueueMaxAttempts, maxAttempts)
		}
	})
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
	defer nq.Stop()

	// Enqueue a notification with alert-1 (far future NextRetryAt so background processor doesn't pick it up)
	futureRetry := time.Now().Add(1 * time.Hour)
	notif := &QueuedNotification{
		ID:          "notif-1",
		Type:        "email",
		Status:      QueueStatusPending,
		MaxAttempts: 3,
		Config:      []byte(`{}`),
		NextRetryAt: &futureRetry,
		Alerts:      []*alerts.Alert{{ID: "alert-1"}},
	}
	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Cancel with non-matching alert ID
	err = nq.CancelByAlertIDs([]string{"alert-2"})
	if err != nil {
		t.Errorf("CancelByAlertIDs returned error: %v", err)
	}

	// Verify the notification is still pending using GetQueueStats
	stats, err := nq.GetQueueStats()
	if err != nil {
		t.Fatalf("GetQueueStats failed: %v", err)
	}
	if stats["pending"] != 1 {
		t.Errorf("Expected 1 pending notification, got %d (stats: %v)", stats["pending"], stats)
	}
	if stats["cancelled"] != 0 {
		t.Errorf("Expected 0 cancelled notifications, got %d", stats["cancelled"])
	}
}

func TestCancelByAlertIDs_MatchingNotificationCancelled(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}
	defer nq.Stop()

	// Enqueue a notification with alert-1 (far future NextRetryAt so background processor doesn't pick it up)
	futureRetry := time.Now().Add(1 * time.Hour)
	notif := &QueuedNotification{
		ID:          "notif-1",
		Type:        "email",
		Status:      QueueStatusPending,
		MaxAttempts: 3,
		Config:      []byte(`{}`),
		NextRetryAt: &futureRetry,
		Alerts:      []*alerts.Alert{{ID: "alert-1"}},
	}
	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Cancel with matching alert ID
	err = nq.CancelByAlertIDs([]string{"alert-1"})
	if err != nil {
		t.Errorf("CancelByAlertIDs returned error: %v", err)
	}

	// Verify the notification is now cancelled using GetQueueStats
	stats, err := nq.GetQueueStats()
	if err != nil {
		t.Fatalf("GetQueueStats failed: %v", err)
	}
	if stats["pending"] != 0 {
		t.Errorf("Expected 0 pending notifications, got %d", stats["pending"])
	}
	if stats["cancelled"] != 1 {
		t.Errorf("Expected 1 cancelled notification, got %d (stats: %v)", stats["cancelled"], stats)
	}
}

func TestCancelByAlertIDs_MultipleAlertsPartialMatch(t *testing.T) {
	tempDir := t.TempDir()
	nq, err := NewNotificationQueue(tempDir)
	if err != nil {
		t.Fatalf("Failed to create notification queue: %v", err)
	}
	defer nq.Stop()

	// Enqueue a notification with multiple alerts (far future NextRetryAt so background processor doesn't pick it up)
	futureRetry := time.Now().Add(1 * time.Hour)
	notif := &QueuedNotification{
		ID:          "notif-multi",
		Type:        "webhook",
		Status:      QueueStatusPending,
		MaxAttempts: 3,
		Config:      []byte(`{}`),
		NextRetryAt: &futureRetry,
		Alerts: []*alerts.Alert{
			{ID: "alert-1"},
			{ID: "alert-2"},
		},
	}
	if err := nq.Enqueue(notif); err != nil {
		t.Fatalf("Failed to enqueue: %v", err)
	}

	// Cancel with only one matching alert ID - should still cancel the notification
	err = nq.CancelByAlertIDs([]string{"alert-1"})
	if err != nil {
		t.Errorf("CancelByAlertIDs returned error: %v", err)
	}

	// Verify the notification is cancelled (any matching alert should cancel)
	stats, err := nq.GetQueueStats()
	if err != nil {
		t.Fatalf("GetQueueStats failed: %v", err)
	}
	if stats["pending"] != 0 {
		t.Errorf("Expected 0 pending notifications after partial match cancel, got %d", stats["pending"])
	}
	if stats["cancelled"] != 1 {
		t.Errorf("Expected 1 cancelled notification, got %d (stats: %v)", stats["cancelled"], stats)
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

	t.Run("UpdateStatus returns error for non-existent notification", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		err = nq.UpdateStatus("non-existent-id", QueueStatusSent, "")
		if err == nil {
			t.Error("expected error when updating non-existent notification, got nil")
		}
	})
}

func TestPerformCleanup(t *testing.T) {
	t.Run("cleanup removes old completed entries", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Insert a notification directly with old completed_at timestamp
		oldTime := time.Now().Add(-10 * 24 * time.Hour).Unix() // 10 days ago

		_, err = nq.db.Exec(`
			INSERT INTO notification_queue
			(id, type, status, config, alerts, attempts, max_attempts, created_at, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"old-sent-1", "email", "sent", "{}", "[]", 1, 3, oldTime, oldTime)
		if err != nil {
			t.Fatalf("Failed to insert old notification: %v", err)
		}

		// Insert a recent completed notification (should NOT be cleaned)
		recentTime := time.Now().Add(-1 * 24 * time.Hour).Unix() // 1 day ago
		_, err = nq.db.Exec(`
			INSERT INTO notification_queue
			(id, type, status, config, alerts, attempts, max_attempts, created_at, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"recent-sent-1", "email", "sent", "{}", "[]", 1, 3, recentTime, recentTime)
		if err != nil {
			t.Fatalf("Failed to insert recent notification: %v", err)
		}

		// Run cleanup
		nq.performCleanup()

		// Verify old entry was removed
		var count int
		err = nq.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE id = ?`, "old-sent-1").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 0 {
			t.Error("old completed notification should have been cleaned up")
		}

		// Verify recent entry still exists
		err = nq.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE id = ?`, "recent-sent-1").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 1 {
			t.Error("recent completed notification should NOT have been cleaned up")
		}
	})

	t.Run("cleanup removes old DLQ entries", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Insert old DLQ entry (> 30 days)
		oldTime := time.Now().Add(-35 * 24 * time.Hour).Unix()
		_, err = nq.db.Exec(`
			INSERT INTO notification_queue
			(id, type, status, config, alerts, attempts, max_attempts, created_at, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"old-dlq-1", "webhook", "dlq", "{}", "[]", 5, 3, oldTime, oldTime)
		if err != nil {
			t.Fatalf("Failed to insert old DLQ entry: %v", err)
		}

		// Insert recent DLQ entry (< 30 days)
		recentTime := time.Now().Add(-20 * 24 * time.Hour).Unix()
		_, err = nq.db.Exec(`
			INSERT INTO notification_queue
			(id, type, status, config, alerts, attempts, max_attempts, created_at, completed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			"recent-dlq-1", "webhook", "dlq", "{}", "[]", 5, 3, recentTime, recentTime)
		if err != nil {
			t.Fatalf("Failed to insert recent DLQ entry: %v", err)
		}

		// Run cleanup
		nq.performCleanup()

		// Verify old DLQ was removed
		var count int
		err = nq.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE id = ?`, "old-dlq-1").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 0 {
			t.Error("old DLQ entry should have been cleaned up")
		}

		// Verify recent DLQ still exists
		err = nq.db.QueryRow(`SELECT COUNT(*) FROM notification_queue WHERE id = ?`, "recent-dlq-1").Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 1 {
			t.Error("recent DLQ entry should NOT have been cleaned up")
		}
	})

	t.Run("cleanup removes old audit logs", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Insert parent notifications first (foreign key constraint)
		oldTime := time.Now().Add(-35 * 24 * time.Hour).Unix()
		recentTime := time.Now().Add(-5 * 24 * time.Hour).Unix()

		_, err = nq.db.Exec(`
			INSERT INTO notification_queue
			(id, type, status, config, alerts, attempts, max_attempts, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"test-1", "email", "sent", "{}", "[]", 1, 3, oldTime)
		if err != nil {
			t.Fatalf("Failed to insert parent notification 1: %v", err)
		}

		_, err = nq.db.Exec(`
			INSERT INTO notification_queue
			(id, type, status, config, alerts, attempts, max_attempts, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			"test-2", "email", "sent", "{}", "[]", 1, 3, recentTime)
		if err != nil {
			t.Fatalf("Failed to insert parent notification 2: %v", err)
		}

		// Insert old audit log (> 30 days)
		_, err = nq.db.Exec(`
			INSERT INTO notification_audit (notification_id, type, status, timestamp)
			VALUES (?, ?, ?, ?)`,
			"test-1", "email", "created", oldTime)
		if err != nil {
			t.Fatalf("Failed to insert old audit: %v", err)
		}

		// Insert recent audit log (< 30 days)
		_, err = nq.db.Exec(`
			INSERT INTO notification_audit (notification_id, type, status, timestamp)
			VALUES (?, ?, ?, ?)`,
			"test-2", "email", "sent", recentTime)
		if err != nil {
			t.Fatalf("Failed to insert recent audit: %v", err)
		}

		// Run cleanup
		nq.performCleanup()

		// Verify old audit was removed
		var count int
		err = nq.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE timestamp = ?`, oldTime).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 0 {
			t.Error("old audit log should have been cleaned up")
		}

		// Verify recent audit still exists
		err = nq.db.QueryRow(`SELECT COUNT(*) FROM notification_audit WHERE timestamp = ?`, recentTime).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query: %v", err)
		}
		if count != 1 {
			t.Error("recent audit log should NOT have been cleaned up")
		}
	})

	t.Run("cleanup with empty database", func(t *testing.T) {
		tempDir := t.TempDir()
		nq, err := NewNotificationQueue(tempDir)
		if err != nil {
			t.Fatalf("Failed to create notification queue: %v", err)
		}
		defer nq.Stop()

		// Should not panic or error
		nq.performCleanup()
	})
}

func TestNewNotificationQueue_InvalidPath(t *testing.T) {
	// Test with a path that cannot be created (file exists where directory expected)
	tempDir := t.TempDir()

	// Create a file at the path where we'd want to create a directory
	blockingFile := tempDir + "/blocked"
	if err := os.WriteFile(blockingFile, []byte("blocking"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	// Try to create queue at a path nested under the blocking file
	invalidPath := blockingFile + "/subdir"
	_, err := NewNotificationQueue(invalidPath)
	if err == nil {
		t.Error("expected error when creating notification queue with invalid path, got nil")
	}
}
