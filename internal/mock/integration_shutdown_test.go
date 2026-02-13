package mock

import (
	"testing"
	"time"
)

func TestSetEnabledDisableDoesNotDeadlockWhenLoopNeedsStateLock(t *testing.T) {
	SetEnabled(false)
	t.Cleanup(func() {
		SetEnabled(false)
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
		SetEnabled(false)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetEnabled(false) timed out waiting for update loop shutdown")
	}
}
