package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunner_RunPatrolScenario_Success(t *testing.T) {
	// Mock state
	running := false
	done := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic Auth Check
		u, p, ok := r.BasicAuth()
		if !ok || u != "admin" || p != "admin" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/api/ai/patrol/status":
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			// Status logic:
			// Initially not running.
			// After trigger, running=true.
			// After stream connected/finished, running=false (simulated here)
			// For simplicity, let's say after 2 status checks while running, it finishes.

			// If already done, return not running
			if done {
				json.NewEncoder(w).Encode(map[string]bool{"running": false, "healthy": true})
				return
			}

			if running {
				// Simulate finishing eventually
				// The real runner polls. Let's make it finish quickly.
				// We'll let the stream handler flip the 'done' bit?
				// Or just flip it here after a check?
				// Let's rely on the runner logic: it waits for idle first, then triggers.

				// For "waitForPatrolComplete", it polls.
				// We return true until stream is consumed?
				// Let's keep it simple: always return running=true here, until we flip it externally?
				// Or just return false immediately if done.
			}

			// Logic:
			// 1. Runner checks idle -> returns false
			// 2. Runner triggers -> sets running=true (internal mock state)
			// 3. Runner polls completion -> returns true for a bit, then false

			json.NewEncoder(w).Encode(map[string]bool{"running": running, "healthy": true})

		case "/api/ai/patrol/run":
			if r.Method != "POST" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			running = true
			done = false
			w.WriteHeader(http.StatusOK)

		case "/api/ai/patrol/stream":
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")

			// Send some events
			fmt.Fprintf(w, "data: {\"type\":\"start\"}\n\n")
			fmt.Fprintf(w, "data: {\"type\":\"tool_start\",\"tool_id\":\"t1\",\"tool_name\":\"scan\"}\n\n")
			time.Sleep(10 * time.Millisecond)
			fmt.Fprintf(w, "data: {\"type\":\"tool_end\",\"tool_id\":\"t1\",\"tool_output\":\"ok\",\"tool_success\":true}\n\n")
			fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")

			// Signal that we are done running after stream closes/finishes
			running = false
			done = true

		case "/api/ai/patrol/findings":
			if r.Method != "GET" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			findings := []PatrolFinding{
				{ID: "f1", Key: "issue.1", Severity: "high", Title: "Issue"},
			}
			json.NewEncoder(w).Encode(findings)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Verbose = true

	scenario := PatrolScenario{
		Name:    "Patrol Success",
		Timeout: 5 * time.Second,
		Assertions: []PatrolAssertion{
			PatrolAssertCompleted(),
			PatrolAssertHasFindings(),
			PatrolAssertToolUsed("scan"),
			PatrolAssertNoToolErrors(),
		},
	}

	result := runner.RunPatrolScenario(scenario)

	assert.True(t, result.Success)
	assert.True(t, result.Completed)
	assert.Len(t, result.Findings, 1)
	assert.Len(t, result.ToolCalls, 1)

	// Check assertions passed
	for _, a := range result.Assertions {
		assert.True(t, a.Passed, "Assertion failed: %s", a.Name)
	}
}

func TestRunner_RunPatrolScenario_SetupTeardown(t *testing.T) {
	setupCalled := false
	teardownCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simple mock allowing everything
		if r.URL.Path == "/api/ai/patrol/status" {
			json.NewEncoder(w).Encode(map[string]bool{"running": false})
			return
		}
		if r.URL.Path == "/api/ai/patrol/run" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/ai/patrol/stream" {
			// quick stream
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"type\":\"complete\"}\n\n")
			return
		}
		if r.URL.Path == "/api/ai/patrol/findings" {
			json.NewEncoder(w).Encode([]PatrolFinding{})
			return
		}
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL

	scenario := PatrolScenario{
		Name: "Setup Teardown",
		Setup: func(r *Runner) error {
			setupCalled = true
			return nil
		},
		Teardown: func(r *Runner) error {
			teardownCalled = true
			return nil
		},
	}

	result := runner.RunPatrolScenario(scenario)
	assert.True(t, result.Success)
	assert.True(t, setupCalled)
	assert.True(t, teardownCalled)
}

func TestRunner_RunPatrolScenario_SetupFail(t *testing.T) {
	runner := NewRunner(DefaultConfig())

	scenario := PatrolScenario{
		Name: "Setup Fail",
		Setup: func(r *Runner) error {
			return fmt.Errorf("setup failed")
		},
	}

	result := runner.RunPatrolScenario(scenario)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error.Error(), "setup failed")
}

func TestRunner_RunPatrolScenario_TriggerFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ai/patrol/status" {
			json.NewEncoder(w).Encode(map[string]bool{"running": false})
			return
		}
		if r.URL.Path == "/api/ai/patrol/run" {
			w.WriteHeader(http.StatusInternalServerError) // Trigger fails
			return
		}
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL

	scenario := PatrolScenario{Name: "Trigger Fail"}
	result := runner.RunPatrolScenario(scenario)

	assert.False(t, result.Success)
	assert.Contains(t, result.Error.Error(), "status 500")
}
