package api

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

var routeAllocationChild = flag.Bool("test.routeAllocationChild", false, "measure route allocations in an isolated test process")

// AllocsPerRun reads process-wide MemStats, even with GOMAXPROCS=1. Router,
// alert and monitoring fixtures may have background workers in this package;
// their allocations must not be attributed to a constant-return fast path.
// Reuse the actual test binary (including race instrumentation), but select
// only the requested measurement. No allocation floor, retries or skips.
func isolatedRouteAllocationCheck(t *testing.T, measure func()) {
	t.Helper()
	if *routeAllocationChild {
		measure()
		t.Log("ROUTE_ALLOCATION_MEASURED " + t.Name())
		return
	}
	runRouteAllocationTest(t, t.Name())
}

func runRouteAllocationTest(t *testing.T, name string) {
	t.Helper()
	switch name {
	case "TestNormalizeSegment_DoesNotAllocate", "TestNormalizeRoute_RootFastPathDoesNotAllocate":
	default:
		t.Fatalf("unknown route allocation measurement %q", name)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable,
		"-test.run=^"+regexp.QuoteMeta(name)+"$", "-test.count=1", "-test.v",
		"-test.timeout=25s", "-test.routeAllocationChild=true")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated %s failed: %v\n%s", name, err, output)
	}
	if !strings.Contains(string(output), "ROUTE_ALLOCATION_MEASURED "+name) {
		t.Fatalf("isolated %s did not execute its measurement:\n%s", name, output)
	}
}

func TestRouteAllocationMeasurementExcludesBackgroundFixtures(t *testing.T) {
	requests := make(chan struct{})
	acknowledged := make(chan struct{})
	stop := make(chan struct{})
	done := make(chan struct{})
	var sink []byte
	var calls atomic.Uint64
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			case <-requests:
				sink = make([]byte, 256)
				calls.Add(1)
				acknowledged <- struct{}{}
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
		if len(sink) != 256 {
			t.Error("background fixture did not retain its allocation")
		}
	})
	allocateElsewhere := func() {
		requests <- struct{}{}
		<-acknowledged
	}

	// Synchronise an unrelated allocation inside each measured call. The
	// normaliser still returns a literal, but the old process-wide assertion
	// necessarily attributes this other goroutine's allocation to it.
	contaminated := testing.AllocsPerRun(100, func() {
		normalizeRouteSink = normalizeRoute("/")
		allocateElsewhere()
	})
	if contaminated < 1 {
		t.Fatal("control failed to demonstrate process-wide allocation accounting")
	}
	t.Logf("unrelated fixture contaminates root measurement: %v allocations/call", contaminated)

	// Keep the parent fixture active while the same binary's isolated root
	// and segment checks execute. Both must still enforce exactly zero.
	pumpStop := make(chan struct{})
	pumpDone := make(chan struct{})
	go func() {
		defer close(pumpDone)
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-pumpStop:
				return
			case <-ticker.C:
				allocateElsewhere()
			}
		}
	}()
	t.Cleanup(func() { close(pumpStop); <-pumpDone })
	before := calls.Load()
	runRouteAllocationTest(t, "TestNormalizeRoute_RootFastPathDoesNotAllocate")
	runRouteAllocationTest(t, "TestNormalizeSegment_DoesNotAllocate")
	if calls.Load() <= before {
		t.Fatal("background fixture was not active during isolated checks")
	}
}
