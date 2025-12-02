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

type testWriter struct {
	t *testing.T
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.t.Log(string(p))
	return len(p), nil
}

func TestAgentBuffering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup Mock Server
	var (
		mu              sync.Mutex
		receivedReports []host.Report
		shouldFail      bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		t.Logf("Server received request: %s %s", r.Method, r.URL.Path)

		if shouldFail {
			t.Log("Server simulating failure (500)")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if r.URL.Path != "/api/agents/host/report" {
			t.Logf("Server 404 for path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var report host.Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			t.Logf("Server failed to decode body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		t.Logf("Server accepted report from %s", report.Host.Hostname)
		receivedReports = append(receivedReports, report)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// 2. Configure Agent
	// Use testWriter to capture logs in test output
	logger := zerolog.New(zerolog.ConsoleWriter{Out: &testWriter{t}}).Level(zerolog.DebugLevel)

	cfg := Config{
		PulseURL:         server.URL,
		APIToken:         "test-token",
		Interval:         250 * time.Millisecond,
		HostnameOverride: "test-host",
		Logger:           &logger,
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// 3. Run Agent in background
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		t.Log("Starting agent...")
		if err := agent.Run(ctx); err != nil && err != context.Canceled {
			t.Errorf("Agent run failed: %v", err)
		}
		t.Log("Agent stopped")
	}()

	// 4. Wait for initial successful report
	t.Log("Waiting for initial report...")
	time.Sleep(2 * time.Second)

	mu.Lock()
	initialCount := len(receivedReports)
	mu.Unlock()

	t.Logf("Initial reports received: %d", initialCount)
	if initialCount == 0 {
		t.Fatal("Expected at least one initial report")
	}

	// 5. Simulate Outage
	t.Log("Simulating outage...")
	mu.Lock()
	shouldFail = true
	mu.Unlock()

	// Wait for a few cycles (should buffer)
	time.Sleep(3 * time.Second)

	mu.Lock()
	// Should not have received any new reports during outage
	if len(receivedReports) > initialCount {
		t.Errorf("Received reports during outage! Expected %d, got %d", initialCount, len(receivedReports))
	}
	mu.Unlock()

	// 6. Recover
	t.Log("Recovering server...")
	mu.Lock()
	shouldFail = false
	mu.Unlock()

	// Wait for flush (flush happens after next successful report)
	time.Sleep(3 * time.Second)

	// Stop agent
	cancel()
	wg.Wait()

	mu.Lock()
	finalCount := len(receivedReports)
	mu.Unlock()

	t.Logf("Final reports received: %d", finalCount)

	// We expect: initial + (buffered during outage) + (new ones after recovery)
	// If collection is slow (e.g. 1s), we might get 1-2 buffered.
	// We just want to ensure we got MORE than just the initial ones.
	// And ideally more than just initial + 1 (which would be just the next report).
	if finalCount <= initialCount+1 {
		t.Errorf("Buffered reports were not flushed? Initial: %d, Final: %d", initialCount, finalCount)
	}

	t.Logf("Test passed. Initial: %d, Final: %d", initialCount, finalCount)
}
