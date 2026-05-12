package alerts

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestFlappingDetectedCallback verifies that:
//  1. The callback fires exactly once on the transition into the flapping
//     state for a given trackingKey.
//  2. Subsequent calls inside the cooldown window (alert still flapping)
//     are silent -- the one-shot semantics hold.
//  3. The callback receives the canonical tracking key plus the cloned
//     alert so the consumer can read identity / context fields directly.
//  4. The callback runs without the alerts manager lock held: it is safe
//     to take the manager's own mutex from inside the callback.
func TestFlappingDetectedCallback(t *testing.T) {
	m := newTestManager(t)

	var (
		mu       sync.Mutex
		calls    int32
		recvKey  string
		recvAlrt *Alert
	)
	done := make(chan struct{}, 8)

	m.SetFlappingDetectedCallback(func(a *Alert, trackingKey string) {
		// Re-entering the manager from inside the callback must not deadlock:
		// if the manager lock were still held when this fires, RLock() here
		// would block forever and the test would time out.
		m.mu.RLock()
		_ = m.flappingActive[trackingKey]
		m.mu.RUnlock()

		mu.Lock()
		recvKey = trackingKey
		recvAlrt = a
		mu.Unlock()
		atomic.AddInt32(&calls, 1)
		done <- struct{}{}
	})

	m.mu.Lock()
	m.config.FlappingEnabled = true
	m.config.FlappingThreshold = 3
	m.config.FlappingWindowSeconds = 300
	m.config.FlappingCooldownMinutes = 15
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()

	// Subscribe an inert alert callback so dispatchAlert reaches the flapping
	// check (it short-circuits when no alert callbacks are registered).
	m.SetAlertCallback(func(*Alert) {})

	alert := &Alert{
		ID:            "vm-100-cpu",
		Type:          "cpu",
		ResourceID:    "vm-100",
		ResourceName:  "testvm",
		CanonicalKind: "vm",
		Level:         AlertLevelWarning,
	}

	// First two dispatches: below threshold, no transition, no callback.
	for i := 0; i < 2; i++ {
		m.mu.Lock()
		m.dispatchAlert(alert, false)
		m.mu.Unlock()
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected 0 callback invocations before threshold, got %d", got)
	}

	// Third dispatch crosses the threshold -- this is the transition.
	m.mu.Lock()
	m.dispatchAlert(alert, false)
	m.mu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("flapping-detected callback did not fire after threshold crossing")
	}

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 callback invocation on transition, got %d", got)
	}

	mu.Lock()
	if recvKey == "" {
		t.Errorf("expected non-empty trackingKey in callback")
	}
	if recvAlrt == nil {
		t.Fatal("expected non-nil alert in callback")
	}
	if recvAlrt.ResourceID != "vm-100" {
		t.Errorf("expected ResourceID=vm-100, got %q", recvAlrt.ResourceID)
	}
	if recvAlrt.CanonicalKind != "vm" {
		t.Errorf("expected CanonicalKind=vm, got %q", recvAlrt.CanonicalKind)
	}
	if recvAlrt.Type != "cpu" {
		t.Errorf("expected Type=cpu, got %q", recvAlrt.Type)
	}
	if recvAlrt.ResourceName != "testvm" {
		t.Errorf("expected ResourceName=testvm, got %q", recvAlrt.ResourceName)
	}
	mu.Unlock()

	// Subsequent dispatches inside the cooldown window: alert remains
	// flapping, callback must not refire.
	for i := 0; i < 4; i++ {
		m.mu.Lock()
		m.dispatchAlert(alert, false)
		m.mu.Unlock()
	}

	// Give any stray goroutines a chance to run.
	time.Sleep(50 * time.Millisecond)

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected callback to fire EXACTLY once on transition; got %d invocations", got)
	}
}

// TestFlappingDetectedCallbackNotInvokedWhenDisabled confirms the callback is
// not fired when flapping detection is disabled in config -- the early return
// in checkFlappingLocked must not surface a phantom transition.
func TestFlappingDetectedCallbackNotInvokedWhenDisabled(t *testing.T) {
	m := newTestManager(t)

	var calls int32
	m.SetFlappingDetectedCallback(func(*Alert, string) {
		atomic.AddInt32(&calls, 1)
	})

	m.mu.Lock()
	m.config.FlappingEnabled = false
	m.config.FlappingThreshold = 1
	m.config.ActivationState = ActivationActive
	m.mu.Unlock()
	m.SetAlertCallback(func(*Alert) {})

	alert := &Alert{
		ID:            "vm-101-mem",
		Type:          "memory",
		ResourceID:    "vm-101",
		ResourceName:  "testvm",
		CanonicalKind: "vm",
		Level:         AlertLevelWarning,
	}
	for i := 0; i < 10; i++ {
		m.mu.Lock()
		m.dispatchAlert(alert, false)
		m.mu.Unlock()
	}

	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("expected 0 callback invocations when flapping detection disabled, got %d", got)
	}
}
