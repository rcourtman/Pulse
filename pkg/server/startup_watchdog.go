package server

import (
	"runtime"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// startupWatchdogBudget bounds how long Run may spend between binding the
// UI/API listener and actually serving it.
//
// The listener is bound early so a port conflict fails fast, but nothing
// accepts connections until srv.Serve starts. Every synchronous initialization
// step in between (monitor start, router construction, optional AI/relay/
// telemetry/config-watcher startup) therefore runs with a live listening
// socket whose accept queue is filling and no reader. A normal startup
// completes in seconds; this budget only trips when one of those steps stalls,
// which is otherwise invisible because no log line is emitted while it runs.
const startupWatchdogBudget = 2 * time.Minute

// startupWatchdogStackBytes is the buffer reserved for the goroutine dump. The
// default 64KiB used elsewhere truncates under a large goroutine set; a stalled
// startup is exactly when the full stack matters.
const startupWatchdogStackBytes = 256 * 1024

// startupPhase records the last completed initialization step so a watchdog
// dump names where Run was when it stalled instead of only showing the stacks.
type startupPhase struct {
	mu    sync.Mutex
	phase string
}

func (s *startupPhase) mark(phase string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.phase = phase
	s.mu.Unlock()
}

func (s *startupPhase) current() string {
	if s == nil {
		return "unknown"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase == "" {
		return "unknown"
	}
	return s.phase
}

// startStartupWatchdog arms an independent timer that fires only if the server
// has not started serving within budget. It logs the last completed startup
// phase and every goroutine stack so a recurrence of the reported
// bound-but-not-serving startup stall (issue #2129) is diagnosable from the
// field log alone, without waiting for an operator to send SIGQUIT. The
// returned stop function must be called once serving begins or Run exits; it is
// safe to call more than once.
func startStartupWatchdog(logger zerolog.Logger, phase *startupPhase, budget time.Duration) (stop func()) {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		timer := time.NewTimer(budget)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			buf := make([]byte, startupWatchdogStackBytes)
			n := runtime.Stack(buf, true)
			logger.Error().
				Dur("budget", budget).
				Str("last_phase", phase.current()).
				Str("goroutines", string(buf[:n])).
				Msg("Pulse has not started serving its UI/API listener within the startup watchdog budget; startup is stalled")
		}
	}()
	return func() {
		once.Do(func() { close(done) })
	}
}
