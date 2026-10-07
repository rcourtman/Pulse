package monitoring

import (
	"testing"
	"time"
)

func TestMockModeFenceRefusesCallsFromAnEndedEpoch(t *testing.T) {
	var fence mockModeFence
	stale := fence.begin()
	fence.advance()

	if stale.current() {
		t.Fatal("a scope from an ended epoch still reads as current")
	}
	if stale.run(func() { t.Fatal("a call from an ended epoch ran") }) {
		t.Fatal("run reported a refused call as admitted")
	}
	ran := false
	if !fence.begin().run(func() { ran = true }) || !ran {
		t.Fatal("a scope taken after the epoch ended was refused")
	}
}

func TestMockModeFenceAdvanceWaitsForAdmittedCalls(t *testing.T) {
	var fence mockModeFence
	scope := fence.begin()
	entered, release := make(chan struct{}), make(chan struct{})
	nestedAdmitted := make(chan bool, 1)
	callDone := make(chan struct{})
	go func() {
		defer close(callDone)
		scope.run(func() {
			close(entered)
			<-release
			// A call made from inside an admitted one, after the epoch
			// ended, is refused rather than deadlocking the waiting advance.
			nestedAdmitted <- scope.run(func() {})
		})
	}()
	waitForSignal(t, entered, 10*time.Second, "admitted call never started")

	advanced := make(chan struct{})
	go func() {
		defer close(advanced)
		fence.advance()
	}()
	waitForCondition(t, 10*time.Second, func() bool { return !scope.current() }, "advance never ended the epoch")
	select {
	case <-advanced:
		t.Fatal("advance returned while a call admitted under the ended epoch was still running")
	default:
	}
	if scope.run(func() { t.Error("a call started after the epoch ended ran") }) {
		t.Fatal("run admitted a call after the epoch ended")
	}

	close(release)
	waitForSignal(t, callDone, 10*time.Second, "admitted call did not finish")
	waitForSignal(t, advanced, 10*time.Second, "advance did not return once the admitted call finished")
	if <-nestedAdmitted {
		t.Fatal("a nested call made after the epoch ended was admitted")
	}
}

func TestMockModeFenceFlagsReadsOverlappedByAnEndedEpochPublish(t *testing.T) {
	var fence mockModeFence
	if !fence.registryCurrent() {
		t.Fatal("a monitor that never switched modes distrusts its registry")
	}
	stale := fence.begin()
	entered, release := make(chan struct{}), make(chan struct{})
	staleDone := make(chan struct{})
	go func() {
		defer close(staleDone)
		stale.publish(func() bool {
			close(entered)
			<-release
			return true
		})
	}()
	waitForSignal(t, entered, 10*time.Second, "publish never started")

	// advance does not wait for publishes.
	advanced := make(chan struct{})
	go func() {
		defer close(advanced)
		fence.advance()
	}()
	waitForSignal(t, advanced, 10*time.Second, "advance waited for a publish")
	if fence.registryCurrent() {
		t.Fatal("the registry counted as current right after a switch")
	}
	if _, ok := stale.publish(func() bool { t.Error("a publish from an ended epoch ran"); return true }); ok {
		t.Fatal("publish admitted an ended epoch")
	}

	live := fence.begin()
	mark, ok := live.publish(func() bool { return true })
	if !ok {
		t.Fatal("publish refused the current epoch")
	}
	if live.undisturbedSince(mark) || fence.registryCurrent() {
		t.Fatal("a publish overlapped by a running ended-epoch publish counted as undisturbed")
	}
	close(release)
	waitForSignal(t, staleDone, 10*time.Second, "ended-epoch publish did not finish")
	if live.undisturbedSince(mark) || fence.registryCurrent() {
		t.Fatal("a publish overlapped by a finished ended-epoch publish counted as undisturbed")
	}

	// A refresh that skipped its rebuild replaced nothing.
	if _, ok := live.publish(func() bool { return false }); !ok || fence.registryCurrent() {
		t.Fatal("a publish that replaced nothing made the registry current")
	}
	next, ok := live.publish(func() bool { return true })
	if !ok || !live.undisturbedSince(next) || !fence.registryCurrent() {
		t.Fatal("a rebuild after the ended-epoch publish finished did not make the registry current")
	}
}
