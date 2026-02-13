package updates

import (
	"testing"
	"time"
)

func TestUpdateQueue_Enqueue(t *testing.T) {
	queue := NewUpdateQueue()

	// Test successful enqueue
	job1, accepted := queue.Enqueue("https://example.com/update1.tar.gz")
	if !accepted {
		t.Fatal("First job should be accepted")
	}
	if job1 == nil {
		t.Fatal("Job should not be nil")
	}
	if job1.State != JobStateQueued {
		t.Errorf("Job state should be queued, got %s", job1.State)
	}

	// Test rejection when another job is running
	queue.MarkRunning(job1.ID)
	job2, accepted := queue.Enqueue("https://example.com/update2.tar.gz")
	if accepted {
		t.Fatal("Second job should be rejected when first is running")
	}
	if job2 != nil {
		t.Error("Rejected job should be nil")
	}
}

func TestUpdateQueue_MarkRunning(t *testing.T) {
	queue := NewUpdateQueue()

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	if job.State != JobStateQueued {
		t.Errorf("Initial state should be queued, got %s", job.State)
	}

	success := queue.MarkRunning(job.ID)
	if !success {
		t.Fatal("MarkRunning should succeed")
	}

	currentJob := queue.GetCurrentJob()
	if currentJob.State != JobStateRunning {
		t.Errorf("State should be running, got %s", currentJob.State)
	}

	// Test marking wrong job ID
	success = queue.MarkRunning("wrong-id")
	if success {
		t.Error("MarkRunning with wrong ID should fail")
	}
}

func TestUpdateQueue_GetCurrentJobReturnsDefensiveCopy(t *testing.T) {
	queue := NewUpdateQueue()
	job, accepted := queue.Enqueue("https://example.com/update.tar.gz")
	if !accepted || job == nil {
		t.Fatal("expected enqueue to succeed")
	}

	snapshot := queue.GetCurrentJob()
	if snapshot == nil {
		t.Fatal("expected current job snapshot")
	}

	snapshot.State = JobStateFailed
	snapshot.DownloadURL = "tampered"

	current := queue.GetCurrentJob()
	if current == nil {
		t.Fatal("expected current job")
	}
	if current.State != JobStateQueued {
		t.Fatalf("expected queued state to remain unchanged, got %s", current.State)
	}
	if current.DownloadURL != "https://example.com/update.tar.gz" {
		t.Fatalf("expected original download URL, got %q", current.DownloadURL)
	}
}

func TestUpdateQueue_MarkCompleted(t *testing.T) {
	queue := NewUpdateQueue()

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	queue.MarkRunning(job.ID)

	// Test successful completion
	queue.MarkCompleted(job.ID, nil)
	currentJob := queue.GetCurrentJob()
	if currentJob.State != JobStateCompleted {
		t.Errorf("State should be completed, got %s", currentJob.State)
	}
	if currentJob.Error != nil {
		t.Error("Error should be nil for successful completion")
	}

	// Check history
	history := queue.GetHistory()
	if len(history) != 1 {
		t.Errorf("History should contain 1 job, got %d", len(history))
	}
}

func TestUpdateQueue_MarkCompletedWithError(t *testing.T) {
	queue := NewUpdateQueue()

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	queue.MarkRunning(job.ID)

	// Test failed completion
	testErr := &testError{"test error"}
	queue.MarkCompleted(job.ID, testErr)

	currentJob := queue.GetCurrentJob()
	if currentJob.State != JobStateFailed {
		t.Errorf("State should be failed, got %s", currentJob.State)
	}
	if currentJob.Error == nil {
		t.Error("Error should not be nil for failed completion")
	}
	if currentJob.Error.Error() != "test error" {
		t.Errorf("Error message should be 'test error', got %s", currentJob.Error.Error())
	}
}

func TestUpdateQueue_Cancel(t *testing.T) {
	queue := NewUpdateQueue()

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	queue.MarkRunning(job.ID)

	success := queue.Cancel(job.ID)
	if !success {
		t.Fatal("Cancel should succeed")
	}

	currentJob := queue.GetCurrentJob()
	if currentJob.State != JobStateCancelled {
		t.Errorf("State should be cancelled, got %s", currentJob.State)
	}

	// Verify context was cancelled
	select {
	case <-currentJob.Context.Done():
		// Context was cancelled as expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}
}

func TestUpdateQueue_IsRunning(t *testing.T) {
	queue := NewUpdateQueue()

	if queue.IsRunning() {
		t.Error("Queue should not be running initially")
	}

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	if !queue.IsRunning() {
		t.Error("Queue should be running after enqueue")
	}

	queue.MarkRunning(job.ID)
	if !queue.IsRunning() {
		t.Error("Queue should be running after marking as running")
	}

	queue.MarkCompleted(job.ID, nil)
	// Note: IsRunning will still return true for a short period after completion
	// This is by design to allow status polling
}

func TestUpdateQueue_MarkCompletedClearsAfterDelay(t *testing.T) {
	queue := NewUpdateQueue()
	queue.clearDelay = 20 * time.Millisecond

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	queue.MarkRunning(job.ID)
	queue.MarkCompleted(job.ID, nil)

	if queue.GetCurrentJob() == nil {
		t.Fatal("current job should be visible immediately after completion")
	}

	time.Sleep(50 * time.Millisecond)
	if queue.GetCurrentJob() != nil {
		t.Fatal("current job should be cleared after delay")
	}
}

func TestUpdateQueue_CancelAddsHistoryAndClearsAfterDelay(t *testing.T) {
	queue := NewUpdateQueue()
	queue.clearDelay = 20 * time.Millisecond

	job, _ := queue.Enqueue("https://example.com/update.tar.gz")
	queue.MarkRunning(job.ID)

	if ok := queue.Cancel(job.ID); !ok {
		t.Fatal("Cancel should succeed")
	}

	history := queue.GetHistory()
	if len(history) != 1 {
		t.Fatalf("History should contain 1 cancelled job, got %d", len(history))
	}
	if history[0].State != JobStateCancelled {
		t.Fatalf("history state = %s, want %s", history[0].State, JobStateCancelled)
	}

	time.Sleep(50 * time.Millisecond)
	if queue.GetCurrentJob() != nil {
		t.Fatal("current job should be cleared after cancellation delay")
	}
}

func TestUpdateQueue_History(t *testing.T) {
	queue := NewUpdateQueue()

	// Add multiple jobs
	for i := 0; i < 5; i++ {
		job, _ := queue.Enqueue("https://example.com/update.tar.gz")
		queue.MarkRunning(job.ID)
		queue.MarkCompleted(job.ID, nil)

		// Wait for the job to be cleared from current
		time.Sleep(50 * time.Millisecond)
	}

	history := queue.GetHistory()
	if len(history) != 5 {
		t.Errorf("History should contain 5 jobs, got %d", len(history))
	}

	// Verify history ordering (should be chronological)
	for i := 1; i < len(history); i++ {
		if history[i].StartedAt.Before(history[i-1].StartedAt) {
			t.Error("History should be in chronological order")
		}
	}
}

func TestUpdateQueue_MaxHistory(t *testing.T) {
	queue := NewUpdateQueue()
	queue.maxHistory = 3

	// Add more jobs than maxHistory
	for i := 0; i < 5; i++ {
		job, _ := queue.Enqueue("https://example.com/update.tar.gz")
		queue.MarkRunning(job.ID)
		queue.MarkCompleted(job.ID, nil)

		// Wait for the job to be cleared
		time.Sleep(50 * time.Millisecond)
	}

	history := queue.GetHistory()
	if len(history) > queue.maxHistory {
		t.Errorf("History should be limited to %d jobs, got %d", queue.maxHistory, len(history))
	}
}

func TestUpdateQueue_GetHistoryReturnsDefensiveCopies(t *testing.T) {
	queue := NewUpdateQueue()

	job, accepted := queue.Enqueue("https://example.com/update.tar.gz")
	if !accepted || job == nil {
		t.Fatal("expected enqueue to succeed")
	}
	queue.MarkRunning(job.ID)
	queue.MarkCompleted(job.ID, nil)

	history := queue.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected one history entry, got %d", len(history))
	}

	history[0].State = JobStateFailed
	history[0].DownloadURL = "tampered"

	historyAgain := queue.GetHistory()
	if len(historyAgain) != 1 {
		t.Fatalf("expected one history entry, got %d", len(historyAgain))
	}
	if historyAgain[0].State != JobStateCompleted {
		t.Fatalf("expected completed state to remain unchanged, got %s", historyAgain[0].State)
	}
	if historyAgain[0].DownloadURL != "https://example.com/update.tar.gz" {
		t.Fatalf("expected original download URL, got %q", historyAgain[0].DownloadURL)
	}
}

// Helper type for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
