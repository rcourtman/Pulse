package hostagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rs/zerolog"
)

func TestAgentBuffering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	var (
		mu              sync.Mutex
		receivedReports []host.Report
		shouldFail      bool
		failedAttempts  int
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		fail := shouldFail
		mu.Unlock()

		if r.URL.Path != "/api/agents/host/report" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if fail {
			mu.Lock()
			failedAttempts++
			mu.Unlock()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var report host.Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		receivedReports = append(receivedReports, report)
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	logger := zerolog.New(zerolog.NewConsoleWriter()).Level(zerolog.WarnLevel)

	cfg := Config{
		PulseURL:         server.URL,
		APIToken:         "test-token",
		Interval:         50 * time.Millisecond,
		HostnameOverride: "test-host",
		Logger:           &logger,
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = agent.Run(ctx)
	}()

	// Wait for condition with polling
	waitFor := func(cond func() bool, timeout time.Duration) bool {
		deadline := time.After(timeout)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if cond() {
				return true
			}
			select {
			case <-ticker.C:
			case <-deadline:
				return false
			}
		}
	}

	// Wait for first successful report
	if !waitFor(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedReports) >= 1
	}, 10*time.Second) {
		t.Fatal("Timed out waiting for initial report")
	}

	mu.Lock()
	initialCount := len(receivedReports)
	mu.Unlock()
	t.Logf("Initial reports: %d", initialCount)

	// Simulate outage
	mu.Lock()
	shouldFail = true
	mu.Unlock()

	// Wait for at least 3 failed attempts (these should be buffered)
	if !waitFor(func() bool {
		mu.Lock()
		defer mu.Unlock()
		return failedAttempts >= 3
	}, 10*time.Second) {
		mu.Lock()
		got := failedAttempts
		mu.Unlock()
		t.Fatalf("Timed out waiting for failed attempts: got %d", got)
	}

	mu.Lock()
	countDuringOutage := len(receivedReports)
	mu.Unlock()

	if countDuringOutage != initialCount {
		t.Errorf("Reports received during outage: expected %d, got %d", initialCount, countDuringOutage)
	}

	// Check buffer has items
	bufferLen := agent.reportBuffer.Len()
	if bufferLen == 0 {
		t.Fatal("Buffer should have items after failed sends")
	}
	t.Logf("Buffer has %d items after outage", bufferLen)

	// Recover
	mu.Lock()
	shouldFail = false
	mu.Unlock()

	// Wait for buffer to empty (flush complete)
	if !waitFor(func() bool {
		return agent.reportBuffer.IsEmpty()
	}, 10*time.Second) {
		t.Fatalf("Timed out waiting for buffer to empty, still has %d items", agent.reportBuffer.Len())
	}

	// Get final count before stopping
	mu.Lock()
	finalCount := len(receivedReports)
	mu.Unlock()

	cancel()
	wg.Wait()

	// Verify we received the buffered reports
	if finalCount < initialCount+bufferLen {
		t.Errorf("Expected at least %d reports (initial %d + buffered %d), got %d",
			initialCount+bufferLen, initialCount, bufferLen, finalCount)
	}

	t.Logf("Initial: %d, Buffered: %d, Final: %d", initialCount, bufferLen, finalCount)
}
