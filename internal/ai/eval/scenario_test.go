package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunner_RunScenario_Success(t *testing.T) {
	// Mock server that returns different responses based on request count (or prompt)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")

		if calls == 1 {
			// Step 1 response
			fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Step 1 done\"}}\n\n")
			fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"session-1\"}}\n\n")
			return
		}

		if calls == 2 {
			// Step 2 response (uses same session)
			// Check prompt or headers if needed, but simple counter is enough for flow test
			fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Step 2 done\"}}\n\n")
			fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"session-1\"}}\n\n")
			return
		}

		// Should not reach here
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Verbose = false
	runner.config.Preflight = false // Disable preflight for this specific test

	scenario := Scenario{
		Name: "Multi-Step Success",
		Steps: []Step{
			{
				Name:   "Step 1",
				Prompt: "First",
				Assertions: []Assertion{
					AssertContentContains("Step 1"),
				},
			},
			{
				Name:   "Step 2",
				Prompt: "Second",
				Assertions: []Assertion{
					AssertContentContains("Step 2"),
				},
			},
		},
	}

	result := runner.RunScenario(scenario)

	assert.True(t, result.Passed)
	assert.Len(t, result.Steps, 2)
	assert.Equal(t, "Step 1 done", result.Steps[0].Content)
	assert.Equal(t, "Step 2 done", result.Steps[1].Content)
	assert.Equal(t, "session-1", result.Steps[1].SessionID) // Should propagate
}

func TestRunner_RunScenario_AssertionFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Step done\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"session-1\"}}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL

	scenario := Scenario{
		Name: "Assertion Failure",
		Steps: []Step{
			{
				Name:   "Step 1",
				Prompt: "Run",
				Assertions: []Assertion{
					func(result *StepResult) AssertionResult {
						return AssertionResult{Name: "FailAlways", Passed: false, Message: "Boom"}
					},
				},
			},
			{
				Name:   "Step 2",
				Prompt: "Should not run",
			},
		},
	}

	result := runner.RunScenario(scenario)

	assert.False(t, result.Passed)
	assert.Len(t, result.Steps, 1) // Should stop after first step
	assert.False(t, result.Steps[0].Success)
}

func TestRunner_RunScenario_WithPreflight(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "text/event-stream")
		// Preflight and Step 1 return same simple response
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Hello\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"session-1\"}}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Preflight = true

	scenario := Scenario{
		Name: "Preflight Scenario",
		Steps: []Step{
			{Name: "Step 1", Prompt: "Hi"},
		},
	}

	result := runner.RunScenario(scenario)

	assert.True(t, result.Passed)
	// Steps should include Preflight + Step 1 = 2 steps
	assert.Len(t, result.Steps, 2)
	assert.Equal(t, "Preflight", result.Steps[0].StepName)
	assert.Equal(t, "Step 1", result.Steps[1].StepName)
}

func TestRunner_RunScenario_PreflightFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Fail connection
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Preflight = true
	runner.config.StepRetries = 0 // Fail fast

	scenario := Scenario{
		Name: "Preflight Fail",
		Steps: []Step{
			{Name: "Step 1", Prompt: "Hi"},
		},
	}

	result := runner.RunScenario(scenario)

	assert.False(t, result.Passed)
	assert.Len(t, result.Steps, 1) // Only preflight step captured
	assert.Equal(t, "Preflight", result.Steps[0].StepName)
	assert.False(t, result.Steps[0].Success)
}

func TestRunner_RunScenario_SendsAutonomousOverride(t *testing.T) {
	var sawAutonomousOverride *bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if raw, ok := req["autonomous_mode"]; ok {
				if v, ok := raw.(bool); ok {
					sawAutonomousOverride = &v
				}
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"ok\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"session-1\"}}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Preflight = false
	runner.config.Verbose = false

	autonomous := false
	scenario := Scenario{
		Name: "Autonomous override",
		Steps: []Step{
			{
				Name:           "step",
				Prompt:         "hello",
				AutonomousMode: &autonomous,
				Assertions: []Assertion{
					AssertContentContains("ok"),
				},
			},
		},
	}

	result := runner.RunScenario(scenario)
	assert.True(t, result.Passed)
	if assert.NotNil(t, sawAutonomousOverride) {
		assert.False(t, *sawAutonomousOverride)
	}
}
