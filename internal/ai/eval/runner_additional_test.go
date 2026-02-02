package eval

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunner_ParseSSEStream_Comprehensive tests the parsing of various SSE events and scenarios
func TestRunner_ParseSSEStream_Comprehensive(t *testing.T) {
	runner := &Runner{
		config: DefaultConfig(),
	}

	tests := []struct {
		name              string
		inputBody         string
		expectedContent   string
		expectedTools     int
		expectedApprovals int
		expectError       bool
		errorContains     string
	}{
		{
			name: "Standard flow with content and done",
			inputBody: `data: {"type":"content","data":{"text":"Hello"}}
data: {"type":"content","data":{"text":" World"}}
data: {"type":"done","data":{"session_id":"sess-1","input_tokens":10,"output_tokens":5}}
`,
			expectedContent: "Hello World",
		},
		{
			name: "Tool call flow",
			inputBody: `data: {"type":"tool_start","data":{"id":"call-1","name":"test_tool","input":"{\"arg\":\"val\"}"}}
data: {"type":"tool_end","data":{"id":"call-1","name":"test_tool","output":"result","success":true}}
data: {"type":"done","data":{}}
`,
			expectedTools: 1,
		},
		{
			name: "Interleaved content and tools",
			inputBody: `data: {"type":"content","data":{"text":"Using tool..."}}
data: {"type":"tool_start","data":{"id":"call-1","name":"t1","input":""}}
data: {"type":"tool_end","data":{"id":"call-1","name":"t1","output":"ok","success":true}}
data: {"type":"content","data":{"text":" Done."}}
data: {"type":"done","data":{}}
`,
			expectedContent: "Using tool... Done.",
			expectedTools:   1,
		},
		{
			name: "Approval needed event",
			inputBody: `data: {"type":"approval_needed","data":{"approval_id":"app-1","tool_id":"call-2","tool_name":"dangerous_tool","risk":"high"}}
data: {"type":"done","data":{}}
`,
			expectedApprovals: 1,
		},
		{
			name: "Stream error event",
			inputBody: `data: {"type":"error","data":{"message":"something went wrong"}}
`,
			expectError:   true,
			errorContains: "something went wrong",
		},
		{
			name: "Raw string error event",
			inputBody: `data: {"type":"error","data":"raw error message"}
`,
			expectError:   true,
			errorContains: "raw error message",
		},
		{
			name: "Malformed JSON (should be ignored/handled gracefully)",
			inputBody: `data: {invalid-json}
data: {"type":"content","data":{"text":"Still working"}}
data: {"type":"done","data":{}}
`,
			expectedContent: "Still working",
		},
		{
			name: "Multiple tool calls",
			inputBody: `data: {"type":"tool_start","data":{"id":"1","name":"t1","input":"i1"}}
data: {"type":"tool_start","data":{"id":"2","name":"t2","input":"i2"}}
data: {"type":"tool_end","data":{"id":"1","output":"o1","success":true}}
data: {"type":"tool_end","data":{"id":"2","output":"o2","success":false}}
data: {"type":"done","data":{}}
`,
			expectedTools: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, tools, approvals, content, _, _, _, _, err := runner.parseSSEStream(
				strings.NewReader(tc.inputBody),
				ApprovalNone,
				"",
			)

			if tc.expectError {
				require.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedContent, content)
				assert.Len(t, tools, tc.expectedTools)
				assert.Len(t, approvals, tc.expectedApprovals)
			}
		})
	}
}

func TestRunner_HandleApprovalDecision(t *testing.T) {
	// Setup a mock server to handle approval requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Basic auth check
		u, p, ok := r.BasicAuth()
		if !ok || u != "admin" || p != "admin" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if strings.HasSuffix(r.URL.Path, "/approve") {
			assert.Equal(t, http.MethodPost, r.Method)
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/deny") {
			assert.Equal(t, http.MethodPost, r.Method)
			// Check if reason payload is present
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
				if reason, ok := payload["reason"]; ok && reason == "unsafe" {
					w.WriteHeader(http.StatusOK)
					return
				}
			}
			// If we expected a reason but didn't get one or got wrong one
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	runner := NewRunner(cfg)

	// Test Approve
	err := runner.handleApprovalDecision(ApprovalApprove, "app-123", "")
	require.NoError(t, err)

	// Test Deny with Reason
	err = runner.handleApprovalDecision(ApprovalDeny, "app-456", "unsafe")
	require.NoError(t, err)

	// Test Ignore (None)
	err = runner.handleApprovalDecision(ApprovalNone, "app-789", "")
	require.NoError(t, err)

	// Test Server Error Handling (setup failure server)
	failServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("oops"))
	}))
	defer failServer.Close()

	failRunner := NewRunner(DefaultConfig())
	failRunner.config.BaseURL = failServer.URL

	err = failRunner.handleApprovalDecision(ApprovalApprove, "app-fail", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
}

func TestRunner_RetryLogic_Advanced(t *testing.T) {
	runner := &Runner{config: DefaultConfig()}

	// Enable all retries for testing
	runner.config.RetryOnRateLimit = true
	runner.config.RetryOnPhantom = true
	runner.config.RetryOnToolErrors = true
	runner.config.RetryOnExplicitTool = true

	tests := []struct {
		name        string
		result      *StepResult
		step        Step
		shouldRetry bool
		retryReason string
	}{
		{
			name: "Rate Limit Error",
			result: &StepResult{
				Error: fmt.Errorf("HTTP 429 Too Many Requests"),
			},
			shouldRetry: true,
			retryReason: "rate_limit",
		},
		{
			name: "Stream Parse Error",
			result: &StepResult{
				Error: fmt.Errorf("failed to parse SSE stream: unexpected EOF"),
			},
			shouldRetry: true,
			retryReason: "stream_error",
		},
		{
			name: "Phantom Detection (Phantom content, no successful tools)",
			result: &StepResult{
				Content: "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request",
				ToolCalls: []ToolCallEvent{
					{Success: false},
				},
			},
			shouldRetry: true,
			retryReason: "phantom_detection",
		},
		{
			name: "Phantom Detection (Phantom content, BUT has successful tools - No Retry)",
			result: &StepResult{
				Content: "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request",
				ToolCalls: []ToolCallEvent{
					{Success: true},
				},
			},
			shouldRetry: false,
		},
		{
			name: "Explicit Tool Requested but Missing",
			step: Step{Prompt: "Please use pulse_read to check files"},
			result: &StepResult{
				Content:   "some content",
				ToolCalls: []ToolCallEvent{},
			},
			shouldRetry: true,
			retryReason: "no_tool_calls_for_explicit_tool",
		},
		{
			name: "Tool Error (Retryable: timeout)",
			result: &StepResult{
				Content: "some content",
				ToolCalls: []ToolCallEvent{
					{Success: false, Output: "context deadline exceeded"},
				},
			},
			shouldRetry: true,
			retryReason: "tool_error",
		},
		{
			name: "Tool Error (Non-Retryable: routing mismatch)",
			result: &StepResult{
				Content: "some content",
				ToolCalls: []ToolCallEvent{
					{Success: false, Output: "routing_mismatch detected"},
				},
			},
			shouldRetry: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			retry, reason := runner.shouldRetryStep(tc.result, tc.step)
			assert.Equal(t, tc.shouldRetry, retry)
			if tc.shouldRetry {
				assert.Equal(t, tc.retryReason, reason)
			}
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	assert.True(t, isRateLimitError("429 Too Many Requests"))
	assert.True(t, isRateLimitError("Quota exceeded"))
	assert.True(t, isRateLimitError("Retry-After: 30"))
	assert.False(t, isRateLimitError("Generic error"))
}

func TestHasRetryableToolError(t *testing.T) {
	// Retryable
	assert.True(t, hasRetryableToolError([]ToolCallEvent{
		{Success: false, Output: "connection refused"},
	}))
	assert.True(t, hasRetryableToolError([]ToolCallEvent{
		{Success: false, Output: "502 Bad Gateway"},
	}))

	// Non-retryable has precedence
	assert.False(t, hasRetryableToolError([]ToolCallEvent{
		{Success: false, Output: "read_only_violation"},
	}))

	// Success means no retry needed for that tool
	assert.False(t, hasRetryableToolError([]ToolCallEvent{
		{Success: true, Output: "success"},
	}))
}

func TestRunner_ExecuteStep_FullFlow(t *testing.T) {
	// Mock a successful flow
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Step done\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{\"session_id\":\"s-1\"}}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.Verbose = true // Cover verbose logging paths

	step := Step{Name: "Test Step", Prompt: "Run"}
	result := runner.executeStep(step, "")
	assert.True(t, result.Success)
	assert.Equal(t, "Step done", result.Content)
	assert.Equal(t, "s-1", result.SessionID)
}

func TestRunner_ExecuteStep_RetryFlow(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			// First attempt fails with rate limit
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Rate limit exceeded"))
			return
		}
		// Second attempt succeeds
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Success\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{}}\n\n")
	}))
	defer server.Close()

	runner := NewRunner(DefaultConfig())
	runner.config.BaseURL = server.URL
	runner.config.StepRetries = 2
	runner.config.RetryOnRateLimit = true
	runner.config.RateLimitCooldown = 1 * time.Millisecond // Fast retry

	result := runner.executeStep(Step{Name: "Retry Step", Prompt: "Go"}, "")

	// Should pass eventually
	assert.True(t, result.Success)
	// Should have retries recorded
	assert.True(t, result.Retries > 0)
	assert.Contains(t, result.RetryNotes, "rate_limit")
}

func TestRunner_WriteReport(t *testing.T) {
	tempDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.ReportDir = tempDir
	cfg.Model = "test-model"
	runner := NewRunner(cfg)

	result := ScenarioResult{
		ScenarioName: "Test Report Scenario",
		Passed:       true,
		Steps: []StepResult{
			{StepName: "Step 1", Success: true},
		},
	}

	path, err := runner.writeReport(result)
	require.NoError(t, err)
	assert.Contains(t, path, tempDir)
	assert.Contains(t, path, "eval-test-report-scenario-test-model")
	assert.FileExists(t, path)

	// Verify content
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "Test Report Scenario")
}

func TestEnvHelpers(t *testing.T) {
	// Bool
	os.Setenv("TEST_BOOL_TRUE", "true")
	os.Setenv("TEST_BOOL_1", "1")
	os.Setenv("TEST_BOOL_FALSE", "false")
	defer os.Unsetenv("TEST_BOOL_TRUE")
	defer os.Unsetenv("TEST_BOOL_1")
	defer os.Unsetenv("TEST_BOOL_FALSE")

	val, ok := envBool("TEST_BOOL_TRUE")
	assert.True(t, ok)
	assert.True(t, val)

	val, ok = envBool("TEST_BOOL_1")
	assert.True(t, ok)
	assert.True(t, val)

	val, ok = envBool("TEST_BOOL_FALSE")
	assert.True(t, ok)
	assert.False(t, val)

	_, ok = envBool("NON_EXISTENT")
	assert.False(t, ok)

	// Int
	os.Setenv("TEST_INT", "123")
	os.Setenv("TEST_INT_BAD", "abc")
	defer os.Unsetenv("TEST_INT")
	defer os.Unsetenv("TEST_INT_BAD")

	iVal, ok := envInt("TEST_INT")
	assert.True(t, ok)
	assert.Equal(t, 123, iVal)

	_, ok = envInt("TEST_INT_BAD")
	assert.False(t, ok)

	// Float
	os.Setenv("TEST_FLOAT", "123.456")
	defer os.Unsetenv("TEST_FLOAT")

	fVal, ok := envFloat("TEST_FLOAT")
	assert.True(t, ok)
	assert.InDelta(t, 123.456, fVal, 0.001)

	// String
	os.Setenv("TEST_STRING", "val")
	defer os.Unsetenv("TEST_STRING")
	sVal, ok := envString("TEST_STRING")
	assert.True(t, ok)
	assert.Equal(t, "val", sVal)
}

func TestApplyEvalEnvOverrides_Comprehensive(t *testing.T) {
	// Set all env vars
	envVars := map[string]string{
		"EVAL_HTTP_TIMEOUT":            "60",
		"EVAL_STEP_RETRIES":            "3",
		"EVAL_RETRY_ON_PHANTOM":        "true",
		"EVAL_RETRY_ON_EXPLICIT_TOOL":  "true",
		"EVAL_RETRY_ON_STREAM_FAILURE": "false",
		"EVAL_RETRY_ON_EMPTY_RESPONSE": "false",
		"EVAL_RETRY_ON_TOOL_ERRORS":    "true",
		"EVAL_RETRY_ON_RATE_LIMIT":     "true",
		"EVAL_RATE_LIMIT_COOLDOWN":     "5",
		"EVAL_PREFLIGHT":               "true",
		"EVAL_PREFLIGHT_TIMEOUT":       "10",
		"EVAL_MODEL":                   "gpt-4",
		"EVAL_REPORT_DIR":              "/tmp/reports",
	}

	for k, v := range envVars {
		os.Setenv(k, v)
		defer os.Unsetenv(k)
	}

	cfg := Config{} // Empty config
	applyEvalEnvOverrides(&cfg)

	assert.Equal(t, 60*time.Second, cfg.RequestTimeout)
	assert.Equal(t, 3, cfg.StepRetries)
	assert.True(t, cfg.RetryOnPhantom)
	assert.True(t, cfg.RetryOnExplicitTool)
	assert.False(t, cfg.RetryOnStreamFailure)
	assert.False(t, cfg.RetryOnEmptyResponse)
	assert.True(t, cfg.RetryOnToolErrors)
	assert.True(t, cfg.RetryOnRateLimit)
	assert.Equal(t, 5*time.Second, cfg.RateLimitCooldown)
	assert.True(t, cfg.Preflight)
	assert.Equal(t, 10*time.Second, cfg.PreflightTimeout)
	assert.Equal(t, "gpt-4", cfg.Model)
	assert.Equal(t, "/tmp/reports", cfg.ReportDir)
}

func TestRunner_RunPreflight(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"content\",\"data\":{\"text\":\"Hello\"}}\n\n")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{}}\n\n")
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.PreflightTimeout = 1 * time.Second
	runner := NewRunner(cfg)

	result := runner.runPreflight()
	assert.True(t, result.Success)
	assert.Equal(t, "Preflight", result.StepName)
	assert.NotEmpty(t, result.Content)
}

func TestRunner_RunPreflight_Fail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return empty successful response (which preflight considers failure)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"done\",\"data\":{}}\n\n")
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	runner := NewRunner(cfg)

	result := runner.runPreflight()
	assert.False(t, result.Success)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "preflight returned empty response")
}
