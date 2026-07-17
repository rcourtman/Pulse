package dockeragent

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

// syncBuffer is a goroutine-safe bytes.Buffer for capturing watchdog log
// output written from another goroutine.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestDockerCallWithRetryAttempts_RetriesTransientErrors(t *testing.T) {
	t.Parallel()

	var attempts int
	value, err := dockerCallWithRetryAttempts(context.Background(), time.Second, 2, func(context.Context) (int, error) {
		attempts++
		if attempts == 1 {
			return 0, syscall.ECONNREFUSED
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if value != 42 {
		t.Fatalf("expected value 42, got %d", value)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDockerCallWithRetryAttempts_DoesNotRetryPermanentErrors(t *testing.T) {
	t.Parallel()

	permanentErr := errors.New("permanent")
	var attempts int
	_, err := dockerCallWithRetryAttempts(context.Background(), time.Second, 3, func(context.Context) (struct{}, error) {
		attempts++
		return struct{}{}, permanentErr
	})
	if !errors.Is(err, permanentErr) {
		t.Fatalf("expected permanent error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt for permanent error, got %d", attempts)
	}
}

func TestDockerCallWithRetryAttempts_SetsDeadline(t *testing.T) {
	t.Parallel()

	_, err := dockerCallWithRetryAttempts(context.Background(), 250*time.Millisecond, 1, func(ctx context.Context) (struct{}, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("expected context deadline")
		}
		if time.Until(deadline) <= 0 {
			t.Fatal("expected deadline in the future")
		}
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAnnotateDockerConnectionError(t *testing.T) {
	t.Parallel()

	unavailable := errors.New("Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?")
	err := annotateDockerConnectionError(unavailable)
	if !strings.Contains(strings.ToLower(err.Error()), "docker daemon unavailable") {
		t.Fatalf("expected unavailable hint, got %q", err.Error())
	}

	timeoutErr := annotateDockerConnectionError(context.DeadlineExceeded)
	if !strings.Contains(strings.ToLower(timeoutErr.Error()), "timed out") {
		t.Fatalf("expected timeout hint, got %q", timeoutErr.Error())
	}
}

func TestCollectCycleWatchdogFiresWhenNotStopped(t *testing.T) {
	var out syncBuffer
	logger := zerolog.New(&out)

	stop := startCollectCycleWatchdog(logger, 20*time.Millisecond)
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(out.String(), "watchdog budget") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	logged := out.String()
	if !strings.Contains(logged, "watchdog budget") {
		t.Fatalf("watchdog did not log after budget elapsed: %q", logged)
	}
	if !strings.Contains(logged, "goroutine") {
		t.Fatalf("watchdog log does not include a goroutine dump: %q", logged)
	}
}

func TestCollectCycleWatchdogSilentWhenStopped(t *testing.T) {
	var out syncBuffer
	logger := zerolog.New(&out)

	stop := startCollectCycleWatchdog(logger, 30*time.Millisecond)
	stop()

	time.Sleep(100 * time.Millisecond)
	if logged := out.String(); logged != "" {
		t.Fatalf("watchdog logged after being stopped: %q", logged)
	}
}
