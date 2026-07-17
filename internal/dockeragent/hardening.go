package dockeragent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
)

const (
	dockerInfoCallTimeout          = 8 * time.Second
	dockerContainerListCallTimeout = 20 * time.Second
	dockerInventoryCallTimeout     = 20 * time.Second
	dockerSwarmListCallTimeout     = 20 * time.Second
	dockerCleanupCallTimeout       = 15 * time.Second
	dockerUpdateCallTimeout        = 2 * time.Minute
	dockerUpdateOverallTimeout     = 15 * time.Minute

	// dockerCollectCycleTimeout bounds one whole collection cycle
	// (buildReport). Every docker call inside the cycle already carries its
	// own per-call timeout, so a healthy cycle finishes in seconds; this
	// ceiling only trips when a call stalls without its deadline aborting it
	// (observed once on 2026-07-17: a DiskUsage roundtrip parked 6+ minutes
	// on a colima daemon with the 20s per-call deadline never firing).
	dockerCollectCycleTimeout = 5 * time.Minute

	// dockerCollectWatchdogGrace is how long past dockerCollectCycleTimeout
	// the watchdog waits before concluding that the cycle context deadline
	// failed to abort the cycle and dumping goroutines for diagnosis.
	dockerCollectWatchdogGrace = 30 * time.Second

	dockerCallRetryAttempts = 2
	dockerRetryBaseDelay    = 200 * time.Millisecond
)

// startCollectCycleWatchdog arms an independent timer that fires only if a
// collection cycle is still running after budget. The cycle context deadline
// should abort the cycle well before then, so the watchdog firing means
// context cancellation was swallowed or a runtime timer was lost; it logs the
// full goroutine stack so a recurrence of the 2026-07-17 stall is diagnosable
// in the field. The returned stop function must be called when the cycle ends.
func startCollectCycleWatchdog(logger zerolog.Logger, budget time.Duration) (stop func()) {
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(budget)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
			buf := make([]byte, 64*1024)
			n := runtime.Stack(buf, true)
			logger.Error().
				Dur("budget", budget).
				Str("goroutines", string(buf[:n])).
				Msg("Docker collect cycle still running past its watchdog budget; the cycle context deadline did not abort it")
		}
	}()
	return func() { close(done) }
}

func dockerCallWithRetry[T any](ctx context.Context, timeout time.Duration, call func(context.Context) (T, error)) (T, error) {
	return dockerCallWithRetryAttempts(ctx, timeout, dockerCallRetryAttempts, call)
}

func dockerCallWithRetryAttempts[T any](ctx context.Context, timeout time.Duration, attempts int, call func(context.Context) (T, error)) (T, error) {
	var zero T

	if attempts < 1 {
		attempts = 1
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		callCtx, cancel := context.WithTimeout(ctx, timeout)
		value, err := call(callCtx)
		cancel()
		if err == nil {
			return value, nil
		}

		lastErr = err
		if !isTransientDockerError(err) || attempt == attempts {
			break
		}

		delay := dockerRetryBaseDelay * time.Duration(attempt)
		if !waitForRetry(ctx, delay) {
			if err := ctx.Err(); err != nil {
				return zero, err
			}
			break
		}
	}

	return zero, lastErr
}

func waitForRetry(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func annotateDockerConnectionError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("docker daemon request timed out; check daemon responsiveness and host load: %w", err)
	}

	if isDockerDaemonUnavailable(err) {
		return fmt.Errorf("docker daemon unavailable; verify daemon is running and socket permissions are correct: %w", err)
	}

	return err
}

func isTransientDockerError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
		type temporary interface {
			Temporary() bool
		}
		if te, ok := any(netErr).(temporary); ok && te.Temporary() {
			return true
		}
	}

	for _, transientErrno := range []error{
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.EPIPE,
		syscall.ENOENT,
		syscall.ENETDOWN,
		syscall.ENETUNREACH,
		syscall.EHOSTUNREACH,
		syscall.ETIMEDOUT,
	} {
		if errors.Is(err, transientErrno) {
			return true
		}
	}

	lower := strings.ToLower(err.Error())
	for _, marker := range []string{
		"cannot connect to the docker daemon",
		"is the docker daemon running",
		"connection refused",
		"connection reset by peer",
		"broken pipe",
		"timeout",
		"timed out",
		"no such file or directory",
		"error during connect",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	return false
}

func isDockerDaemonUnavailable(err error) bool {
	if err == nil {
		return false
	}

	for _, unavailableErrno := range []error{
		syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.EPIPE,
		syscall.ENOENT,
		syscall.ENETDOWN,
		syscall.ENETUNREACH,
		syscall.EHOSTUNREACH,
		syscall.ETIMEDOUT,
	} {
		if errors.Is(err, unavailableErrno) {
			return true
		}
	}

	lower := strings.ToLower(err.Error())
	for _, marker := range []string{
		"cannot connect to the docker daemon",
		"is the docker daemon running",
		"connection refused",
		"connection reset by peer",
		"broken pipe",
		"no such file or directory",
		"error during connect",
		"dial unix",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	return false
}
