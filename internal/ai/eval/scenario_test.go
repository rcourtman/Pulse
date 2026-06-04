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

func TestRunner_RunScenario_SendsHandoffEnvelope(t *testing.T) {
	var req map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"handoff ok\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"session-1\"}}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Preflight = false
	runner.config.Verbose = false

	autonomous := false
	scenario := Scenario{
		Name: "Handoff envelope",
		Steps: []Step{
			{
				Name:           "step",
				Prompt:         "hello",
				HandoffContext: "model-only context",
				HandoffResources: []StepHandoffResource{{
					ID:   "delly:delly:101",
					Name: "homeassistant",
					Type: "system-container",
					Node: "delly",
				}},
				HandoffMetadata: StepHandoffMetadata{Kind: "resource_context"},
				AutonomousMode:  &autonomous,
				Assertions: []Assertion{
					AssertContentContains("handoff ok"),
				},
			},
		},
	}

	result := runner.RunScenario(scenario)
	assert.True(t, result.Passed)
	assert.Equal(t, "model-only context", req["handoff_context"])
	assert.Equal(t, false, req["autonomous_mode"])

	resources, ok := req["handoff_resources"].([]interface{})
	if assert.True(t, ok, "handoff_resources should be an array") && assert.Len(t, resources, 1) {
		resource, ok := resources[0].(map[string]interface{})
		if assert.True(t, ok, "handoff resource should be an object") {
			assert.Equal(t, "delly:delly:101", resource["id"])
			assert.Equal(t, "homeassistant", resource["name"])
			assert.Equal(t, "system-container", resource["type"])
			assert.Equal(t, "delly", resource["node"])
		}
	}

	metadata, ok := req["handoff_metadata"].(map[string]interface{})
	if assert.True(t, ok, "handoff_metadata should be an object") {
		assert.Equal(t, "resource_context", metadata["kind"])
	}
}

func TestResourceContextHandoffScenarioUsesConfiguredResource(t *testing.T) {
	t.Setenv("EVAL_RESOURCE_CONTEXT_ID", "lab:node:101")
	t.Setenv("EVAL_RESOURCE_CONTEXT_NAME", "homeassistant")
	t.Setenv("EVAL_RESOURCE_CONTEXT_TYPE", "system-container")
	t.Setenv("EVAL_RESOURCE_CONTEXT_NODE", "node")
	t.Setenv("EVAL_RESOURCE_CONTEXT_FORBIDDEN", "/mnt/secret;token-value")

	scenario := ResourceContextHandoffScenario()
	assert.Equal(t, "Resource Context Handoff", scenario.Name)
	if assert.Len(t, scenario.Steps, 4) {
		first := scenario.Steps[0]
		if assert.Len(t, first.HandoffResources, 1) {
			assert.Equal(t, StepHandoffResource{
				ID:   "lab:node:101",
				Name: "homeassistant",
				Type: "system-container",
				Node: "node",
			}, first.HandoffResources[0])
		}
		assert.Equal(t, StepHandoffMetadata{Kind: "resource_context"}, first.HandoffMetadata)
		assert.NotNil(t, first.AutonomousMode)
		assert.False(t, *first.AutonomousMode)
		assert.Empty(t, scenario.Steps[1].HandoffResources, "later steps should exercise persisted session handoff")
	}
}
