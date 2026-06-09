package mock

import (
	"testing"
	"time"
)

func TestSetEnabledDisableDoesNotDeadlockWhenLoopNeedsStateLock(t *testing.T) {
	mustSetEnabled(t, false)
	t.Cleanup(func() {
		mustSetEnabled(t, false)
	})

	dataMu.Lock()
	enabled.Store(true)
	stopUpdatesCh = make(chan struct{})
	updateTicker = time.NewTicker(time.Hour)
	updateLoopWg.Add(1)
	go func(stop <-chan struct{}) {
		defer updateLoopWg.Done()
		<-stop
		dataMu.Lock()
		dataMu.Unlock()
	}(stopUpdatesCh)
	dataMu.Unlock()

	done := make(chan struct{})
	go func() {
		if err := SetEnabled(false); err != nil {
			t.Errorf("SetEnabled(false): %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetEnabled(false) timed out waiting for update loop shutdown")
	}
}
