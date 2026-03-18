package dockeragent

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

const (
	dockerInfoCallTimeout          = 8 * time.Second
	dockerContainerListCallTimeout = 20 * time.Second
	dockerSwarmListCallTimeout     = 20 * time.Second
	dockerCleanupCallTimeout       = 15 * time.Second
	dockerUpdateCallTimeout        = 2 * time.Minute
	dockerUpdateOverallTimeout     = 15 * time.Minute

	dockerCallRetryAttempts = 2
	dockerRetryBaseDelay    = 200 * time.Millisecond
)

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
