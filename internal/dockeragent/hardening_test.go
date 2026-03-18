package dockeragent

import (
	"context"
	"errors"
	"strings"
	"syscall"
	"testing"
	"time"
)

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
