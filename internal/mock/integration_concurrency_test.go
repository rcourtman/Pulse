package mock

import (
	"testing"
	"time"
)

func TestSetEnabledDisableDoesNotDeadlockWhenUpdateIsBlocked(t *testing.T) {
	SetEnabled(false)
	t.Cleanup(func() {
		SetEnabled(false)
	})

	cfg := DefaultConfig
	cfg.RandomMetrics = true
	SetMockConfig(cfg)
	SetEnabled(true)

	dataMu.Lock()
	time.Sleep(updateInterval + 250*time.Millisecond)

	done := make(chan struct{})
	go func() {
		SetEnabled(false)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	dataMu.Unlock()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SetEnabled(false) deadlocked while stopping update loop")
	}

	if IsMockEnabled() {
		t.Fatal("mock mode should be disabled after SetEnabled(false)")
	}
}
