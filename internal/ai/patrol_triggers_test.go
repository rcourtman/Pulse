package ai

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- TriggerManager construction ---

func TestNewTriggerManager_Defaults(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	tm := NewTriggerManager(cfg)

	if tm == nil {
		t.Fatal("expected non-nil TriggerManager")
	}
	if tm.minResourceInterval != 2*time.Minute {
		t.Errorf("expected minResourceInterval 2m, got %v", tm.minResourceInterval)
	}
	if tm.minGlobalInterval != 30*time.Second {
		t.Errorf("expected minGlobalInterval 30s, got %v", tm.minGlobalInterval)
	}
	if tm.maxPendingTriggers != 10 {
		t.Errorf("expected maxPendingTriggers 10, got %d", tm.maxPendingTriggers)
	}
	if tm.baseInterval != 15*time.Minute {
		t.Errorf("expected baseInterval 15m, got %v", tm.baseInterval)
	}
	if tm.currentInterval != 15*time.Minute {
		t.Errorf("expected currentInterval to equal baseInterval, got %v", tm.currentInterval)
	}
	if tm.busyThreshold != 5 {
		t.Errorf("expected busyThreshold 5, got %d", tm.busyThreshold)
	}
}

func TestNewTriggerManager_ZeroDefaults(t *testing.T) {
	tm := NewTriggerManager(TriggerManagerConfig{})
	if tm.minResourceInterval != 2*time.Minute {
		t.Errorf("expected default minResourceInterval, got %v", tm.minResourceInterval)
	}
	if tm.minGlobalInterval != 30*time.Second {
		t.Errorf("expected default minGlobalInterval, got %v", tm.minGlobalInterval)
	}
	if tm.maxPendingTriggers != 10 {
		t.Errorf("expected default maxPendingTriggers, got %d", tm.maxPendingTriggers)
	}
	if tm.baseInterval != 15*time.Minute {
		t.Errorf("expected default baseInterval, got %v", tm.baseInterval)
	}
	if tm.busyThreshold != 5 {
		t.Errorf("expected default busyThreshold, got %d", tm.busyThreshold)
	}
	if tm.eventWindow != 5*time.Minute {
		t.Errorf("expected default eventWindow, got %v", tm.eventWindow)
	}
}

// --- TriggerPatrol queueing ---

func TestTriggerPatrol_AcceptsAndQueues(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	scope := AlertTriggeredPatrolScope("a1", "res-1", "node", "cpu")

	accepted := tm.TriggerPatrol(scope)
	if !accepted {
		t.Fatal("expected trigger to be accepted")
	}
	if tm.GetPendingCount() != 1 {
		t.Errorf("expected 1 pending trigger, got %d", tm.GetPendingCount())
	}
}

func TestTriggerPatrol_QueueFull_RejectsLowerPriority(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.MaxPendingTriggers = 2
	tm := NewTriggerManager(cfg)

	// Fill queue with high priority
	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"r1"}})
	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"r2"}})

	// Try to add a lower-priority trigger — should be rejected
	accepted := tm.TriggerPatrol(PatrolScope{Priority: 20, Reason: TriggerReasonScheduled})
	if accepted {
		t.Error("expected lower-priority trigger to be rejected when queue is full of higher priority")
	}
	if tm.GetPendingCount() != 2 {
		t.Errorf("expected 2 pending triggers, got %d", tm.GetPendingCount())
	}
}

func TestTriggerPatrol_QueueFull_ReplacesLowestPriority(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.MaxPendingTriggers = 2
	tm := NewTriggerManager(cfg)

	// Fill queue with mixed priority
	tm.TriggerPatrol(PatrolScope{Priority: 20, Reason: TriggerReasonScheduled, ResourceIDs: []string{"r1"}})
	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"r2"}})

	// Add a high-priority trigger — should replace the priority-20 one
	accepted := tm.TriggerPatrol(PatrolScope{Priority: 90, Reason: TriggerReasonManual, ResourceIDs: []string{"r3"}})
	if !accepted {
		t.Error("expected higher-priority trigger to be accepted when it can replace a lower one")
	}
	if tm.GetPendingCount() != 2 {
		t.Errorf("expected 2 pending triggers (replaced lowest), got %d", tm.GetPendingCount())
	}
}

func TestTriggerPatrol_DeduplicatesMergesPriority(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())

	// Queue a trigger
	tm.TriggerPatrol(PatrolScope{
		Priority:    40,
		Reason:      TriggerReasonAlertCleared,
		ResourceIDs: []string{"res-1"},
	})
	// Queue the same trigger with higher priority
	accepted := tm.TriggerPatrol(PatrolScope{
		Priority:    80,
		Reason:      TriggerReasonAlertCleared,
		ResourceIDs: []string{"res-1"},
	})
	if !accepted {
		t.Error("expected duplicate trigger to be accepted (merged)")
	}
	if tm.GetPendingCount() != 1 {
		t.Errorf("expected 1 pending (deduplicated), got %d", tm.GetPendingCount())
	}

	// Priority should have been upgraded
	tm.mu.Lock()
	if tm.pendingTriggers[0].Priority != 80 {
		t.Errorf("expected priority to be upgraded to 80, got %d", tm.pendingTriggers[0].Priority)
	}
	tm.mu.Unlock()
}

// --- processPendingTriggers ---

func TestProcessPendingTriggers_PriorityOrder(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.MinGlobalInterval = 0 // disable global rate limit for this test
	tm := NewTriggerManager(cfg)

	var executed []string
	var mu sync.Mutex
	tm.SetOnTrigger(func(_ context.Context, scope PatrolScope) {
		mu.Lock()
		executed = append(executed, string(scope.Reason))
		mu.Unlock()
	})

	// Queue triggers with different priorities
	tm.TriggerPatrol(PatrolScope{Priority: 20, Reason: TriggerReasonScheduled, ResourceIDs: []string{"r1"}})
	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"r2"}})
	tm.TriggerPatrol(PatrolScope{Priority: 100, Reason: TriggerReasonManual, ResourceIDs: []string{"r3"}})

	// Process — should pick highest priority first
	ctx := context.Background()
	tm.processPendingTriggers(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(executed) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(executed))
	}
	if executed[0] != string(TriggerReasonManual) {
		t.Errorf("expected manual (priority 100) to be executed first, got %s", executed[0])
	}
	if tm.GetPendingCount() != 2 {
		t.Errorf("expected 2 remaining triggers, got %d", tm.GetPendingCount())
	}
}

func TestProcessPendingTriggers_EmptyQueue(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	var called int32
	tm.SetOnTrigger(func(_ context.Context, scope PatrolScope) {
		atomic.AddInt32(&called, 1)
	})

	ctx := context.Background()
	tm.processPendingTriggers(ctx)

	if atomic.LoadInt32(&called) != 0 {
		t.Error("expected no callback when queue is empty")
	}
}

func TestProcessPendingTriggers_NilCallback(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	tm.TriggerPatrol(PatrolScope{Priority: 50, Reason: TriggerReasonScheduled})

	// No callback set — should not panic
	ctx := context.Background()
	tm.processPendingTriggers(ctx)

	if tm.GetPendingCount() != 1 {
		t.Errorf("expected trigger to remain in queue when callback is nil, got %d", tm.GetPendingCount())
	}
}

func TestProcessPendingTriggers_GlobalRateLimit(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.MinGlobalInterval = 1 * time.Hour // very long rate limit
	tm := NewTriggerManager(cfg)

	var callCount int32
	tm.SetOnTrigger(func(_ context.Context, scope PatrolScope) {
		atomic.AddInt32(&callCount, 1)
	})

	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"r1"}})

	ctx := context.Background()

	// First call should execute (lastGlobalPatrol is zero)
	tm.processPendingTriggers(ctx)
	if atomic.LoadInt32(&callCount) != 1 {
		t.Fatal("expected first call to execute")
	}

	// Add another trigger
	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"r2"}})

	// Second call should be rate-limited
	tm.processPendingTriggers(ctx)
	if atomic.LoadInt32(&callCount) != 1 {
		t.Error("expected second call to be rate-limited")
	}
	if tm.GetPendingCount() != 1 {
		t.Errorf("expected trigger to remain in queue, got %d", tm.GetPendingCount())
	}
}

func TestProcessPendingTriggers_ResourceRateLimit(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.MinGlobalInterval = 0               // disable global rate limit
	cfg.MinResourceInterval = 1 * time.Hour // very long resource rate limit
	tm := NewTriggerManager(cfg)

	var executed []string
	var mu sync.Mutex
	tm.SetOnTrigger(func(_ context.Context, scope PatrolScope) {
		mu.Lock()
		executed = append(executed, scope.ResourceIDs[0])
		mu.Unlock()
	})

	tm.TriggerPatrol(PatrolScope{Priority: 80, Reason: TriggerReasonAlertFired, ResourceIDs: []string{"res-1"}})

	ctx := context.Background()
	tm.processPendingTriggers(ctx)

	// First execution should succeed
	mu.Lock()
	if len(executed) != 1 || executed[0] != "res-1" {
		t.Fatalf("expected first execution on res-1")
	}
	mu.Unlock()

	// Queue another trigger for the same resource
	tm.TriggerPatrol(PatrolScope{Priority: 90, Reason: TriggerReasonManual, ResourceIDs: []string{"res-1"}})

	// Should be skipped due to resource rate limit
	tm.processPendingTriggers(ctx)
	mu.Lock()
	if len(executed) != 1 {
		t.Error("expected second call on same resource to be rate-limited")
	}
	mu.Unlock()
}

func TestProcessPendingTriggers_RetryAfterBackoff(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.MinGlobalInterval = 0
	tm := NewTriggerManager(cfg)

	var callCount int32
	tm.SetOnTrigger(func(_ context.Context, scope PatrolScope) {
		atomic.AddInt32(&callCount, 1)
	})

	// Queue a trigger with RetryAfter in the future
	tm.TriggerPatrol(PatrolScope{
		Priority:    80,
		Reason:      TriggerReasonAlertFired,
		ResourceIDs: []string{"r1"},
	})
	// Manually set RetryAfter on the queued trigger
	tm.mu.Lock()
	tm.pendingTriggers[0].RetryAfter = time.Now().Add(1 * time.Hour)
	tm.mu.Unlock()

	ctx := context.Background()
	tm.processPendingTriggers(ctx)

	if atomic.LoadInt32(&callCount) != 0 {
		t.Error("expected trigger with future RetryAfter to be skipped")
	}
	if tm.GetPendingCount() != 1 {
		t.Errorf("expected trigger to remain in queue, got %d", tm.GetPendingCount())
	}
}

// --- updateAdaptiveInterval ---

func TestUpdateAdaptiveInterval_BusyMode(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.BusyThreshold = 3
	tm := NewTriggerManager(cfg)

	// Add enough events to trigger busy mode
	tm.mu.Lock()
	for i := 0; i < 3; i++ {
		tm.recentEvents = append(tm.recentEvents, time.Now())
	}
	tm.updateAdaptiveInterval()
	interval := tm.currentInterval
	tm.mu.Unlock()

	if interval != 5*time.Minute {
		t.Errorf("expected busy interval 5m, got %v", interval)
	}
}

func TestUpdateAdaptiveInterval_QuietMode(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())

	tm.mu.Lock()
	tm.recentEvents = nil // no events
	tm.updateAdaptiveInterval()
	interval := tm.currentInterval
	tm.mu.Unlock()

	if interval != 30*time.Minute {
		t.Errorf("expected quiet interval 30m, got %v", interval)
	}
}

func TestUpdateAdaptiveInterval_NormalMode(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.BusyThreshold = 5
	tm := NewTriggerManager(cfg)

	// Add a few events (less than busy threshold, more than zero)
	tm.mu.Lock()
	tm.recentEvents = []time.Time{time.Now(), time.Now()}
	tm.updateAdaptiveInterval()
	interval := tm.currentInterval
	tm.mu.Unlock()

	if interval != cfg.BaseInterval {
		t.Errorf("expected normal interval %v, got %v", cfg.BaseInterval, interval)
	}
}

// --- GetCurrentInterval / GetStatus ---

func TestGetCurrentInterval(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())

	interval := tm.GetCurrentInterval()
	if interval != 15*time.Minute {
		t.Errorf("expected default interval 15m, got %v", interval)
	}
}

func TestGetStatus(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())
	tm.TriggerPatrol(PatrolScope{Priority: 50, Reason: TriggerReasonScheduled})

	status := tm.GetStatus()
	if status.Running {
		t.Error("expected not running before Start()")
	}
	if status.PendingTriggers != 1 {
		t.Errorf("expected 1 pending trigger, got %d", status.PendingTriggers)
	}
	if status.CurrentInterval != 15*time.Minute {
		t.Errorf("expected current interval 15m, got %v", status.CurrentInterval)
	}
}

// --- Start / Stop ---

func TestStartStop(t *testing.T) {
	tm := NewTriggerManager(DefaultTriggerManagerConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tm.Start(ctx)

	status := tm.GetStatus()
	if !status.Running {
		t.Error("expected running after Start()")
	}

	// Double start should be a no-op
	tm.Start(ctx)

	tm.Stop()

	status = tm.GetStatus()
	if status.Running {
		t.Error("expected not running after Stop()")
	}

	// Double stop should be a no-op
	tm.Stop()
}

// --- Scope factory functions ---

func TestAlertTriggeredPatrolScope(t *testing.T) {
	scope := AlertTriggeredPatrolScope("alert-1", "res-1", "node", "cpu")

	if scope.Reason != TriggerReasonAlertFired {
		t.Errorf("expected reason alert_fired, got %s", scope.Reason)
	}
	if scope.Priority != triggerPriorityAlertFired {
		t.Errorf("expected priority %d, got %d", triggerPriorityAlertFired, scope.Priority)
	}
	if len(scope.ResourceIDs) != 1 || scope.ResourceIDs[0] != "res-1" {
		t.Errorf("expected resource res-1, got %v", scope.ResourceIDs)
	}
	if scope.AlertID != "alert-1" {
		t.Errorf("expected alertID alert-1, got %s", scope.AlertID)
	}
	if scope.Depth != PatrolDepthQuick {
		t.Errorf("expected quick depth, got %v", scope.Depth)
	}
}

func TestAlertClearedPatrolScope(t *testing.T) {
	scope := AlertClearedPatrolScope("alert-2", "res-2", "vm")

	if scope.Reason != TriggerReasonAlertCleared {
		t.Errorf("expected reason alert_cleared, got %s", scope.Reason)
	}
	if scope.Priority != triggerPriorityAlertCleared {
		t.Errorf("expected priority %d, got %d", triggerPriorityAlertCleared, scope.Priority)
	}
	if scope.AlertID != "alert-2" {
		t.Errorf("expected alertID alert-2, got %s", scope.AlertID)
	}
}

func TestAnomalyDetectedPatrolScope(t *testing.T) {
	scope := AnomalyDetectedPatrolScope("res-3", "node", "cpu", 95.0, 50.0)

	if scope.Reason != TriggerReasonAnomalyDetected {
		t.Errorf("expected reason anomaly, got %s", scope.Reason)
	}
	if scope.Priority != triggerPriorityAnomaly {
		t.Errorf("expected priority %d, got %d", triggerPriorityAnomaly, scope.Priority)
	}
	if scope.Depth != PatrolDepthNormal {
		t.Errorf("expected normal depth, got %v", scope.Depth)
	}
}

func TestAnomalyTriggeredPatrolScope_SeverityPriority(t *testing.T) {
	tests := []struct {
		severity string
		expected int
	}{
		{"critical", 85},
		{"high", 70},
		{"medium", 60},
		{"low", 60},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			scope := AnomalyTriggeredPatrolScope("r1", "node", "cpu", tt.severity)
			if scope.Priority != tt.expected {
				t.Errorf("severity %s: expected priority %d, got %d", tt.severity, tt.expected, scope.Priority)
			}
		})
	}
}

func TestUserActionPatrolScope(t *testing.T) {
	scope := UserActionPatrolScope("f1", "res-4", "dismiss")

	if scope.Reason != TriggerReasonUserAction {
		t.Errorf("expected reason user_action, got %s", scope.Reason)
	}
	if scope.Priority != triggerPriorityUserAction {
		t.Errorf("expected priority %d, got %d", triggerPriorityUserAction, scope.Priority)
	}
	if scope.FindingID != "f1" {
		t.Errorf("expected findingID f1, got %s", scope.FindingID)
	}
}

func TestManualPatrolScope(t *testing.T) {
	scope := ManualPatrolScope([]string{"r1", "r2"}, PatrolDepthNormal)

	if scope.Reason != TriggerReasonManual {
		t.Errorf("expected reason manual, got %s", scope.Reason)
	}
	if scope.Priority != triggerPriorityManual {
		t.Errorf("expected priority %d, got %d", triggerPriorityManual, scope.Priority)
	}
	if len(scope.ResourceIDs) != 2 {
		t.Errorf("expected 2 resource IDs, got %d", len(scope.ResourceIDs))
	}
	if scope.Depth != PatrolDepthNormal {
		t.Errorf("expected normal depth, got %v", scope.Depth)
	}
}

func TestScheduledPatrolScope(t *testing.T) {
	scope := ScheduledPatrolScope()

	if scope.Reason != TriggerReasonScheduled {
		t.Errorf("expected reason scheduled, got %s", scope.Reason)
	}
	if scope.Priority != triggerPriorityScheduled {
		t.Errorf("expected priority %d, got %d", triggerPriorityScheduled, scope.Priority)
	}
	if scope.Depth != PatrolDepthNormal {
		t.Errorf("expected normal depth, got %v", scope.Depth)
	}
}

// --- slicesEqual ---

func TestSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []string
		expected bool
	}{
		{"both empty", nil, nil, true},
		{"both empty slices", []string{}, []string{}, true},
		{"different lengths", []string{"a"}, []string{"a", "b"}, false},
		{"same single element", []string{"a"}, []string{"a"}, true},
		{"different single element", []string{"a"}, []string{"b"}, false},
		{"same elements same order", []string{"a", "b"}, []string{"a", "b"}, true},
		{"same elements different order", []string{"b", "a"}, []string{"a", "b"}, true},
		{"different elements", []string{"a", "b"}, []string{"a", "c"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slicesEqual(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("slicesEqual(%v, %v) = %v, want %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

// --- PatrolDepth.String ---

func TestPatrolDepth_String(t *testing.T) {
	tests := []struct {
		depth    PatrolDepth
		expected string
	}{
		{PatrolDepthQuick, "quick"},
		{PatrolDepthNormal, "normal"},
		{PatrolDepth(99), "unknown"},
	}

	for _, tt := range tests {
		result := tt.depth.String()
		if result != tt.expected {
			t.Errorf("PatrolDepth(%d).String() = %q, want %q", tt.depth, result, tt.expected)
		}
	}
}

// --- cleanupOldEvents ---

func TestCleanupOldEvents(t *testing.T) {
	cfg := DefaultTriggerManagerConfig()
	cfg.EventWindow = 1 * time.Minute
	tm := NewTriggerManager(cfg)

	tm.mu.Lock()
	tm.recentEvents = []time.Time{
		time.Now().Add(-2 * time.Minute),  // old, should be removed
		time.Now().Add(-30 * time.Second), // recent, should be kept
		time.Now(),                        // current, should be kept
	}
	tm.cleanupOldEvents()
	count := len(tm.recentEvents)
	tm.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 events after cleanup, got %d", count)
	}
}
