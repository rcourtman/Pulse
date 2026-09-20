package server

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

func TestStartupPhaseMarkAndCurrent(t *testing.T) {
	var nilPhase *startupPhase
	if got := nilPhase.current(); got != "unknown" {
		t.Fatalf("nil phase current = %q, want unknown", got)
	}
	nilPhase.mark("ignored") // must not panic

	phase := &startupPhase{}
	if got := phase.current(); got != "unknown" {
		t.Fatalf("empty phase current = %q, want unknown", got)
	}
	phase.mark("API router initialized")
	if got := phase.current(); got != "API router initialized" {
		t.Fatalf("phase current = %q, want last marked phase", got)
	}
}

func TestStartupWatchdogFiresAndNamesLastPhase(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	phase := &startupPhase{}
	phase.mark("AI services started")

	stop := startStartupWatchdog(logger, phase, 20*time.Millisecond)
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(buf.String(), "startup is stalled") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	out := buf.String()
	if !strings.Contains(out, "startup is stalled") {
		t.Fatalf("watchdog did not fire; log=%q", out)
	}
	if !strings.Contains(out, "AI services started") {
		t.Fatalf("watchdog did not name the last startup phase; log=%q", out)
	}
}

func TestStartupWatchdogStopIsIdempotentAndPreventsFire(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	stop := startStartupWatchdog(logger, &startupPhase{}, 15*time.Millisecond)
	stop()
	stop() // must not panic

	time.Sleep(80 * time.Millisecond)
	if strings.Contains(buf.String(), "startup is stalled") {
		t.Fatalf("watchdog fired after stop; log=%q", buf.String())
	}
}
