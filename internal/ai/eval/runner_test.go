package eval

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, "http://127.0.0.1:7655", cfg.BaseURL)
	assert.Equal(t, "admin", cfg.Username)
	assert.Equal(t, 2, cfg.StepRetries)
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Test Scenario", "test-scenario"},
		{"Test/Scenario", "test-scenario"},
		{"Test:Scenario", "test-scenario"},
		{"  Test  ", "test"},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, sanitizeFilename(tc.input))
	}
}

func TestRequiresExplicitTool(t *testing.T) {
	tests := []struct {
		prompt   string
		expected bool
	}{
		{"use pulse_read please", true},
		{"check the system", false},
		{"use a read-only tool", true},
		{"use a control tool", true},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, requiresExplicitTool(tc.prompt), "Prompt: %s", tc.prompt)
	}
}

func TestApplyEvalEnvOverrides(t *testing.T) {
	os.Setenv("EVAL_STEP_RETRIES", "5")
	os.Setenv("EVAL_RETRY_ON_PHANTOM", "false")
	defer os.Unsetenv("EVAL_STEP_RETRIES")
	defer os.Unsetenv("EVAL_RETRY_ON_PHANTOM")

	cfg := DefaultConfig()
	applyEvalEnvOverrides(&cfg)

	assert.Equal(t, 5, cfg.StepRetries)
	assert.False(t, cfg.RetryOnPhantom)
}

func TestRunner_RunScenario(t *testing.T) {
	// Mock server validating the request and returning a fake SSE stream
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/ai/chat", r.URL.Path)
		assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

		// Check basic auth
		u, p, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "admin", u)
		assert.Equal(t, "admin", p)

		w.Header().Set("Content-Type", "text/event-stream")

		// Send some events
		// 1. Tool call
		// Pulse internal protocol expects data to be a JSON object with "type" and "data" fields
		fmt.Fprintf(w, "data: {\"type\":\"tool_start\",\"data\":{\"id\":\"call_1\",\"name\":\"pulse_read\",\"input\":\"\"}}\n\n")

		// 2. Tool output
		fmt.Fprintf(w, "data: {\"type\":\"tool_end\",\"data\":{\"id\":\"call_1\",\"name\":\"pulse_read\",\"output\":\"output\",\"success\":true}}\n\n")

		// 3. Content
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Hello world\"}}\n\n")

		// 4. Done
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{}}\n\n")
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.Verbose = false
	runner := NewRunner(cfg)

	scenario := Scenario{
		Name: "Test Scenario",
		Steps: []Step{
			{Name: "Step 1", Prompt: "Hello"},
		},
	}

	result := runner.RunScenario(scenario)

	assert.True(t, result.Passed)
	require.Len(t, result.Steps, 1)
	step := result.Steps[0]
	assert.Equal(t, "Hello world", step.Content)
	require.Len(t, step.ToolCalls, 1)
	assert.Equal(t, "pulse_read", step.ToolCalls[0].Name)
}

func TestRunner_ShouldRetry(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RetryOnPhantom = true
	runner := NewRunner(cfg)

	// Case 1: Phantom detection
	res := &StepResult{
		Content:   "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request",
		ToolCalls: []ToolCallEvent{},
	}
	retry, reason := runner.shouldRetryStep(res, Step{})
	assert.True(t, retry)
	assert.Equal(t, "phantom_detection", reason)

	// Case 2: Success
	res = &StepResult{
		Content: "OK",
	}
	retry, _ = runner.shouldRetryStep(res, Step{})
	assert.False(t, retry)
}

func TestRunner_UpdateAISettings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"patrol_model": "old-model"}`))
			return
		}
		if r.Method == http.MethodPut {
			w.Write([]byte("{}"))
			return
		}
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	runner := NewRunner(cfg)

	// Test Get
	settings, err := runner.getAISettings(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "old-model", settings.PatrolModel)

	// Test Update
	update := "new-model"
	err = runner.updateAISettings(context.Background(), aiSettingsUpdateRequest{PatrolModel: &update})
	require.NoError(t, err)
}

func TestNormalizeModelString(t *testing.T) {
	// ParseModelString likely defaults to openai provider if missing
	assert.Equal(t, "", normalizeModelString("  "))
	assert.Equal(t, "openai:gpt-4", normalizeModelString("gpt-4"))
}
