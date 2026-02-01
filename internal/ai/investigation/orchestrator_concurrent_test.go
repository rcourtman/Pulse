package investigation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
)

// concurrentChatService is a chat service stub that supports concurrent usage
// with configurable delays and tracking.
type concurrentChatService struct {
	mu             sync.Mutex
	sessionCounter int
	executionDelay time.Duration
	executeFunc    func(ctx context.Context, cb StreamCallback) error
}

func (s *concurrentChatService) CreateSession(ctx context.Context) (*Session, error) {
	s.mu.Lock()
	s.sessionCounter++
	id := fmt.Sprintf("session-%d", s.sessionCounter)
	s.mu.Unlock()
	return &Session{ID: id}, nil
}

func (s *concurrentChatService) ExecuteStream(ctx context.Context, req ExecuteRequest, callback StreamCallback) error {
	if s.executeFunc != nil {
		return s.executeFunc(ctx, callback)
	}
	if s.executionDelay > 0 {
		select {
		case <-time.After(s.executionDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	payload, _ := json.Marshal(map[string]string{"text": "CANNOT_FIX: simulated"})
	callback(StreamEvent{Type: "content", Data: payload})
	return nil
}

func (s *concurrentChatService) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	return nil, nil
}

func (s *concurrentChatService) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (s *concurrentChatService) ListAvailableTools(ctx context.Context, prompt string) []string {
	return nil
}

func (s *concurrentChatService) SetAutonomousMode(enabled bool) {}

// concurrentFindingsStore is a thread-safe findings store for concurrent tests.
type concurrentFindingsStore struct {
	mu       sync.RWMutex
	findings map[string]*Finding
}

func newConcurrentFindingsStore() *concurrentFindingsStore {
	return &concurrentFindingsStore{
		findings: make(map[string]*Finding),
	}
}

func (s *concurrentFindingsStore) Add(f *Finding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findings[f.ID] = f
}

func (s *concurrentFindingsStore) Get(id string) *Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findings[id]
}

func (s *concurrentFindingsStore) Update(f *Finding) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.findings[f.ID] = f
	return true
}

// concurrentApprovalStore is a thread-safe stub for the ApprovalStore interface
// used in the investigation package (Create only).
type concurrentApprovalStore struct {
	mu        sync.Mutex
	approvals []*Approval
}

func (s *concurrentApprovalStore) Create(appr *Approval) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.approvals = append(s.approvals, appr)
	return nil
}

func (s *concurrentApprovalStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.approvals)
}

// ---------------------------------------------------------------------------
// Test 1: TestConcurrent_MultipleInvestigations
// ---------------------------------------------------------------------------

func TestConcurrent_MultipleInvestigations(t *testing.T) {
	const N = 8
	const maxConcurrent = 3

	var activeCount int64 // tracks currently executing investigations
	var peakCount int64

	chatService := &concurrentChatService{
		executeFunc: func(ctx context.Context, cb StreamCallback) error {
			cur := atomic.AddInt64(&activeCount, 1)
			// Track peak concurrency
			for {
				old := atomic.LoadInt64(&peakCount)
				if cur <= old || atomic.CompareAndSwapInt64(&peakCount, old, cur) {
					break
				}
			}
			defer atomic.AddInt64(&activeCount, -1)

			// Simulate work
			select {
			case <-time.After(50 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}

			payload, _ := json.Marshal(map[string]string{"text": "CANNOT_FIX: simulated investigation"})
			cb(StreamEvent{Type: "content", Data: payload})
			return nil
		},
	}

	store := NewStore("")
	findings := newConcurrentFindingsStore()

	cfg := DefaultConfig()
	cfg.MaxConcurrent = maxConcurrent
	cfg.Timeout = 10 * time.Second

	orchestrator := NewOrchestrator(chatService, store, findings, nil, cfg)

	// Pre-create findings
	for i := 0; i < N; i++ {
		findings.Add(&Finding{
			ID:           fmt.Sprintf("finding-%d", i),
			Title:        fmt.Sprintf("Test Finding %d", i),
			Severity:     "warning",
			Category:     "performance",
			ResourceID:   fmt.Sprintf("vm-%d", i),
			ResourceName: fmt.Sprintf("test-vm-%d", i),
			ResourceType: "vm",
			Description:  "Test finding for concurrency",
		})
	}

	var wg sync.WaitGroup
	errors := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			finding := findings.Get(fmt.Sprintf("finding-%d", idx))
			errors[idx] = orchestrator.InvestigateFinding(context.Background(), finding, "approval")
		}(i)
	}

	wg.Wait()

	// Count successes vs max-concurrent rejections
	succeeded := 0
	rejected := 0
	for _, err := range errors {
		if err == nil {
			succeeded++
		} else {
			rejected++
		}
	}

	t.Logf("Results: %d succeeded, %d rejected, peak concurrency: %d", succeeded, rejected, atomic.LoadInt64(&peakCount))

	// At least maxConcurrent should succeed (the first batch gets through)
	if succeeded < maxConcurrent {
		t.Errorf("expected at least %d investigations to succeed, got %d", maxConcurrent, succeeded)
	}

	// The running count should be back to zero after all complete
	orchestrator.runningMu.Lock()
	finalCount := orchestrator.runningCount
	orchestrator.runningMu.Unlock()
	if finalCount != 0 {
		t.Errorf("expected running count to return to 0, got %d", finalCount)
	}

	// Verify CanStartInvestigation returns true after all complete
	if !orchestrator.CanStartInvestigation() {
		t.Error("expected CanStartInvestigation to return true after all investigations complete")
	}
}

// ---------------------------------------------------------------------------
// Test 2: TestConcurrent_InvestigationDuringShutdown
// ---------------------------------------------------------------------------

func TestConcurrent_InvestigationDuringShutdown(t *testing.T) {
	investigationStarted := make(chan struct{})
	investigationBlocked := make(chan struct{})

	chatService := &concurrentChatService{
		executeFunc: func(ctx context.Context, cb StreamCallback) error {
			close(investigationStarted)
			// Block until context is cancelled (simulating a long-running investigation)
			<-investigationBlocked
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				// Fallback so test doesn't hang forever
				payload, _ := json.Marshal(map[string]string{"text": "CANNOT_FIX: timed out"})
				cb(StreamEvent{Type: "content", Data: payload})
				return nil
			}
		},
	}

	store := NewStore("")
	findings := newConcurrentFindingsStore()
	findings.Add(&Finding{
		ID:           "finding-shutdown-1",
		Title:        "Shutdown Test Finding",
		Severity:     "warning",
		Category:     "performance",
		ResourceID:   "vm-1",
		ResourceName: "test-vm",
		ResourceType: "vm",
		Description:  "Finding for shutdown test",
	})

	cfg := DefaultConfig()
	cfg.MaxConcurrent = 5
	cfg.Timeout = 30 * time.Second

	orchestrator := NewOrchestrator(chatService, store, findings, nil, cfg)

	// Start an investigation in the background
	investigationDone := make(chan error, 1)
	go func() {
		finding := findings.Get("finding-shutdown-1")
		investigationDone <- orchestrator.InvestigateFinding(context.Background(), finding, "approval")
	}()

	// Wait for investigation to actually start executing
	select {
	case <-investigationStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for investigation to start")
	}

	// Now trigger shutdown with a reasonable timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()

	// Unblock the investigation so it can observe the context cancellation
	close(investigationBlocked)

	shutdownErr := orchestrator.Shutdown(shutdownCtx)

	// Shutdown should complete (not deadlock)
	// It may return nil (all completed) or timeout error -- both are acceptable
	// as long as we did not deadlock.
	t.Logf("Shutdown returned: %v", shutdownErr)

	// The investigation should also finish
	select {
	case err := <-investigationDone:
		t.Logf("Investigation finished with: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("investigation did not finish after shutdown -- possible deadlock")
	}

	// After shutdown, new investigations should be rejected
	finding := findings.Get("finding-shutdown-1")
	err := orchestrator.InvestigateFinding(context.Background(), finding, "approval")
	if err == nil {
		t.Error("expected error when starting investigation after shutdown")
	}
}

// ---------------------------------------------------------------------------
// Test 3: TestConcurrent_ApprovalStoreConcurrency
// ---------------------------------------------------------------------------

func TestConcurrent_ApprovalStoreConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := approval.NewStore(approval.StoreConfig{
		DataDir:            tmpDir,
		DefaultTimeout:     5 * time.Minute,
		MaxApprovals:       200,
		DisablePersistence: true,
	})
	if err != nil {
		t.Fatalf("failed to create approval store: %v", err)
	}

	const N = 20
	var wg sync.WaitGroup

	// Phase 1: Create N approval requests concurrently
	ids := make([]string, N)
	createErrs := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("approval-%d", idx)
			ids[idx] = id
			createErrs[idx] = store.CreateApproval(&approval.ApprovalRequest{
				ID:         id,
				Command:    fmt.Sprintf("echo test-%d", idx),
				TargetType: "vm",
				TargetID:   fmt.Sprintf("vm-%d", idx),
				TargetName: fmt.Sprintf("test-vm-%d", idx),
				Context:    "concurrent test",
			})
		}(i)
	}
	wg.Wait()

	for i, err := range createErrs {
		if err != nil {
			t.Errorf("create approval %d failed: %v", i, err)
		}
	}

	// Verify all were created
	pending := store.GetPendingApprovals()
	if len(pending) != N {
		t.Errorf("expected %d pending approvals, got %d", N, len(pending))
	}

	// Phase 2: Concurrently approve the first half and deny the second half
	approveErrs := make([]error, N)
	denyErrs := make([]error, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx < N/2 {
				_, approveErrs[idx] = store.Approve(ids[idx], "admin")
			} else {
				_, denyErrs[idx] = store.Deny(ids[idx], "admin", "test denial")
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < N/2; i++ {
		if approveErrs[i] != nil {
			t.Errorf("approve %d failed: %v", i, approveErrs[i])
		}
	}
	for i := N / 2; i < N; i++ {
		if denyErrs[i] != nil {
			t.Errorf("deny %d failed: %v", i, denyErrs[i])
		}
	}

	// Phase 3: Verify final state concurrently
	var verifyWg sync.WaitGroup
	for i := 0; i < N; i++ {
		verifyWg.Add(1)
		go func(idx int) {
			defer verifyWg.Done()
			req, ok := store.GetApproval(ids[idx])
			if !ok {
				t.Errorf("approval %d not found", idx)
				return
			}
			if idx < N/2 {
				if req.Status != approval.StatusApproved {
					t.Errorf("approval %d: expected approved, got %s", idx, req.Status)
				}
			} else {
				if req.Status != approval.StatusDenied {
					t.Errorf("approval %d: expected denied, got %s", idx, req.Status)
				}
			}
		}(i)
	}
	verifyWg.Wait()

	// Verify stats
	stats := store.GetStats()
	if stats["approved"] != N/2 {
		t.Errorf("expected %d approved, got %d", N/2, stats["approved"])
	}
	if stats["denied"] != N-N/2 {
		t.Errorf("expected %d denied, got %d", N-N/2, stats["denied"])
	}
	if stats["pending"] != 0 {
		t.Errorf("expected 0 pending, got %d", stats["pending"])
	}
}
