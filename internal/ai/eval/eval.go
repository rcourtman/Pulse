// Package eval provides an evaluation framework for testing Pulse Assistant
// behavior end-to-end. It sends prompts to the live API and captures the
// full trace of tool calls, FSM transitions, and responses for verification.
package eval

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config holds eval runner configuration
type Config struct {
	BaseURL  string // e.g., "http://127.0.0.1:7655"
	Username string
	Password string
	Verbose  bool
	// Retry behavior for transient eval failures.
	StepRetries          int
	RetryOnPhantom       bool
	RetryOnExplicitTool  bool
	RetryOnStreamFailure bool
	RetryOnEmptyResponse bool
	RetryOnToolErrors    bool
	// Optional preflight to fail fast when SSE hangs.
	Preflight        bool
	PreflightTimeout time.Duration
	// Optional report output directory (JSON per scenario).
	ReportDir string
}

// DefaultConfig returns a config for local development
func DefaultConfig() Config {
	return Config{
		BaseURL:              "http://127.0.0.1:7655",
		Username:             "admin",
		Password:             "admin",
		Verbose:              true,
		StepRetries:          2,
		RetryOnPhantom:       true,
		RetryOnExplicitTool:  true,
		RetryOnStreamFailure: true,
		RetryOnEmptyResponse: true,
		RetryOnToolErrors:    true,
		Preflight:            false,
		PreflightTimeout:     15 * time.Second,
	}
}

// Runner executes eval scenarios against the Pulse API
type Runner struct {
	config Config
	client *http.Client
}

// NewRunner creates a new eval runner
func NewRunner(config Config) *Runner {
	applyEvalEnvOverrides(&config)
	return &Runner{
		config: config,
		client: &http.Client{
			Timeout: 5 * time.Minute, // Long timeout for AI responses
		},
	}
}

// StepResult captures the result of a single eval step
type StepResult struct {
	StepName   string
	Prompt     string
	SessionID  string
	Success    bool
	Error      error
	Duration   time.Duration
	Retries    int
	RetryNotes []string
	ToolCalls  []ToolCallEvent
	Approvals  []ApprovalEvent
	Content    string
	RawEvents  []SSEEvent
	Assertions []AssertionResult
}

// ToolCallEvent represents a tool call captured during execution
type ToolCallEvent struct {
	ID      string
	Name    string
	Input   string
	Output  string
	Success bool
}

// ApprovalEvent represents an approval request captured during execution
type ApprovalEvent struct {
	ApprovalID  string
	ToolID      string
	ToolName    string
	Command     string
	Risk        string
	Description string
}

// SSEEvent represents a raw SSE event from the stream
type SSEEvent struct {
	Type string
	Data json.RawMessage
}

// AssertionResult captures the result of a single assertion
type AssertionResult struct {
	Name    string
	Passed  bool
	Message string
}

// ScenarioResult captures the result of a full scenario
type ScenarioResult struct {
	ScenarioName string
	Steps        []StepResult
	Passed       bool
	Duration     time.Duration
	ReportPath   string
}

// Step defines a single step in an eval scenario
type Step struct {
	Name             string
	Prompt           string
	Assertions       []Assertion
	ApprovalDecision ApprovalDecision
	ApprovalReason   string
}

// ApprovalDecision controls how eval handles approval requests during a step.
type ApprovalDecision string

const (
	ApprovalNone    ApprovalDecision = ""
	ApprovalApprove ApprovalDecision = "approve"
	ApprovalDeny    ApprovalDecision = "deny"
)

// Assertion defines a check to run after a step
type Assertion func(result *StepResult) AssertionResult

// Scenario defines a multi-step eval scenario
type Scenario struct {
	Name        string
	Description string
	Steps       []Step
}

// RunScenario executes a scenario and returns the results
func (r *Runner) RunScenario(scenario Scenario) ScenarioResult {
	startTime := time.Now()
	result := ScenarioResult{
		ScenarioName: scenario.Name,
		Passed:       true,
	}

	var sessionID string

	if r.config.Preflight {
		preflight := r.runPreflight()
		result.Steps = append(result.Steps, preflight)
		if !preflight.Success {
			result.Passed = false
			result.Duration = time.Since(startTime)
			if reportPath, err := r.writeReport(result); err == nil {
				result.ReportPath = reportPath
			}
			return result
		}
	}

	for i, step := range scenario.Steps {
		if r.config.Verbose {
			fmt.Printf("\n=== Step %d: %s ===\n", i+1, step.Name)
			fmt.Printf("Prompt: %s\n", step.Prompt)
		}

		stepResult := r.executeStep(step, sessionID)

		// Use session from first step for subsequent steps
		if sessionID == "" && stepResult.SessionID != "" {
			sessionID = stepResult.SessionID
		}
		stepResult.SessionID = sessionID

		// Run assertions
		for _, assertion := range step.Assertions {
			assertResult := assertion(&stepResult)
			stepResult.Assertions = append(stepResult.Assertions, assertResult)
			if !assertResult.Passed {
				stepResult.Success = false
				result.Passed = false
			}
		}

		if stepResult.Error != nil {
			stepResult.Success = false
			result.Passed = false
		}

		if r.config.Verbose {
			r.printStepResult(&stepResult)
		}

		result.Steps = append(result.Steps, stepResult)

		// Stop on failure
		if !stepResult.Success {
			break
		}
	}

	result.Duration = time.Since(startTime)
	if reportPath, err := r.writeReport(result); err == nil {
		result.ReportPath = reportPath
	}
	return result
}

func (r *Runner) executeStep(step Step, sessionID string) StepResult {
	retries := r.config.StepRetries
	if retries < 0 {
		retries = 0
	}
	return r.executeStepWithRetry(step, sessionID, retries)
}

func (r *Runner) executeStepWithRetry(step Step, sessionID string, retries int) StepResult {
	if retries < 0 {
		retries = 0
	}

	var retryNotes []string
	for attempt := 0; attempt <= retries; attempt++ {
		result := r.executeStepOnce(step, sessionID)
		shouldRetry, reason := r.shouldRetryStep(&result, step)
		if !shouldRetry || attempt == retries {
			result.Retries = len(retryNotes)
			result.RetryNotes = retryNotes
			return result
		}
		if reason != "" {
			retryNotes = append(retryNotes, reason)
		}
		if r.config.Verbose {
			fmt.Printf("\n--- Retrying step '%s' (attempt %d/%d) due to transient failure ---\n",
				step.Name, attempt+1, retries)
		}
	}

	return r.executeStepOnce(step, sessionID)
}

func (r *Runner) executeStepOnce(step Step, sessionID string) StepResult {
	return r.executeStepOnceWithClient(step, sessionID, r.client)
}

func (r *Runner) executeStepOnceWithClient(step Step, sessionID string, client *http.Client) StepResult {
	startTime := time.Now()
	result := StepResult{
		StepName:  step.Name,
		Prompt:    step.Prompt,
		SessionID: sessionID,
		Success:   true,
	}

	// Build request
	reqBody := map[string]string{
		"prompt": step.Prompt,
	}
	if sessionID != "" {
		reqBody["session_id"] = sessionID
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", r.config.BaseURL+"/api/ai/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		result.Error = fmt.Errorf("failed to create request: %w", err)
		result.Success = false
		return result
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.SetBasicAuth(r.config.Username, r.config.Password)

	// Execute request
	if client == nil {
		client = r.client
	}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Errorf("request failed: %w", err)
		result.Success = false
		return result
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		result.Error = fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
		result.Success = false
		return result
	}

	// Parse SSE stream
	result.RawEvents, result.ToolCalls, result.Approvals, result.Content, result.SessionID, err = r.parseSSEStream(resp.Body, step.ApprovalDecision, step.ApprovalReason)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse SSE stream: %w", err)
		result.Success = false
		return result
	}

	result.Duration = time.Since(startTime)
	return result
}

func (r *Runner) runPreflight() StepResult {
	step := Step{
		Name:   "Preflight",
		Prompt: "Say hello.",
	}
	client := &http.Client{
		Timeout: r.config.PreflightTimeout,
	}
	result := r.executeStepOnceWithClient(step, "", client)
	result.StepName = "Preflight"
	if result.Error == nil && strings.TrimSpace(result.Content) == "" && len(result.ToolCalls) == 0 {
		result.Error = fmt.Errorf("preflight returned empty response")
		result.Success = false
	}
	return result
}

func (r *Runner) shouldRetryStep(result *StepResult, step Step) (bool, string) {
	if result == nil {
		return false, ""
	}

	// Retry on known transient errors (stream parse or phantom detection).
	if result.Error != nil && r.config.RetryOnStreamFailure {
		errMsg := result.Error.Error()
		if strings.Contains(errMsg, "token too long") ||
			strings.Contains(errMsg, "failed to parse SSE stream") ||
			strings.Contains(errMsg, "stream error") {
			return true, "stream_error"
		}
	}

	phantomMessage := "I apologize, but I wasn't able to access the infrastructure tools needed to complete that request"
	if r.config.RetryOnPhantom && !hasSuccessfulToolCallRetry(result.ToolCalls) && strings.Contains(result.Content, phantomMessage) {
		return true, "phantom_detection"
	}

	if r.config.RetryOnEmptyResponse && strings.TrimSpace(result.Content) == "" {
		return true, "empty_response"
	}

	// If an explicit tool was requested and no tool calls occurred, retry once.
	if r.config.RetryOnExplicitTool && len(result.ToolCalls) == 0 && requiresExplicitTool(step.Prompt) {
		return true, "no_tool_calls_for_explicit_tool"
	}

	if r.config.RetryOnToolErrors && len(result.ToolCalls) > 0 && !hasSuccessfulToolCallRetry(result.ToolCalls) {
		if hasRetryableToolError(result.ToolCalls) {
			return true, "tool_error"
		}
	}

	return false, ""
}

func requiresExplicitTool(prompt string) bool {
	prompt = strings.ToLower(prompt)
	explicitTools := []string{
		"pulse_read",
		"pulse_control",
		"pulse_query",
		"pulse_discovery",
		"pulse_docker",
		"pulse_kubernetes",
		"pulse_metrics",
		"pulse_storage",
	}
	for _, tool := range explicitTools {
		if strings.Contains(prompt, tool) {
			return true
		}
	}
	if strings.Contains(prompt, "read-only tool") || strings.Contains(prompt, "read only tool") {
		return true
	}
	if strings.Contains(prompt, "control tool") || strings.Contains(prompt, "query tool") {
		return true
	}
	return false
}

func applyEvalEnvOverrides(config *Config) {
	if config == nil {
		return
	}

	if value, ok := envInt("EVAL_STEP_RETRIES"); ok {
		config.StepRetries = value
	} else if config.StepRetries == 0 {
		config.StepRetries = 1
	}

	if value, ok := envBool("EVAL_RETRY_ON_PHANTOM"); ok {
		config.RetryOnPhantom = value
	} else if !config.RetryOnPhantom {
		config.RetryOnPhantom = true
	}

	if value, ok := envBool("EVAL_RETRY_ON_EXPLICIT_TOOL"); ok {
		config.RetryOnExplicitTool = value
	} else if !config.RetryOnExplicitTool {
		config.RetryOnExplicitTool = true
	}

	if value, ok := envBool("EVAL_RETRY_ON_STREAM_FAILURE"); ok {
		config.RetryOnStreamFailure = value
	} else if !config.RetryOnStreamFailure {
		config.RetryOnStreamFailure = true
	}

	if value, ok := envBool("EVAL_RETRY_ON_EMPTY_RESPONSE"); ok {
		config.RetryOnEmptyResponse = value
	} else if !config.RetryOnEmptyResponse {
		config.RetryOnEmptyResponse = true
	}

	if value, ok := envBool("EVAL_RETRY_ON_TOOL_ERRORS"); ok {
		config.RetryOnToolErrors = value
	} else if !config.RetryOnToolErrors {
		config.RetryOnToolErrors = true
	}

	if value, ok := envBool("EVAL_PREFLIGHT"); ok {
		config.Preflight = value
	}

	if value, ok := envInt("EVAL_PREFLIGHT_TIMEOUT"); ok && value > 0 {
		config.PreflightTimeout = time.Duration(value) * time.Second
	} else if config.PreflightTimeout == 0 {
		config.PreflightTimeout = 15 * time.Second
	}

	if dir, ok := envString("EVAL_REPORT_DIR"); ok {
		config.ReportDir = dir
	}
}

func (r *Runner) writeReport(result ScenarioResult) (string, error) {
	if r == nil || r.config.ReportDir == "" {
		return "", nil
	}

	if err := os.MkdirAll(r.config.ReportDir, 0700); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("eval-%s-%s.json", sanitizeFilename(result.ScenarioName), timestamp)
	path := filepath.Join(r.config.ReportDir, filename)

	report := struct {
		GeneratedAt time.Time      `json:"generated_at"`
		BaseURL     string         `json:"base_url"`
		Username    string         `json:"username"`
		Result      ScenarioResult `json:"result"`
	}{
		GeneratedAt: time.Now(),
		BaseURL:     r.config.BaseURL,
		Username:    r.config.Username,
		Result:      result,
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", err
	}

	return path, nil
}

func sanitizeFilename(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	return name
}

func envBool(key string) (bool, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func envInt(key string) (int, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0, false
	}
	var parsed int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err != nil {
		return 0, false
	}
	return parsed, true
}

func envString(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

func (r *Runner) handleApprovalDecision(decision ApprovalDecision, approvalID, reason string) error {
	if r == nil || approvalID == "" {
		return nil
	}

	path := ""
	switch decision {
	case ApprovalApprove:
		path = "/api/ai/approvals/" + approvalID + "/approve"
	case ApprovalDeny:
		path = "/api/ai/approvals/" + approvalID + "/deny"
	default:
		return nil
	}

	var body io.Reader
	if decision == ApprovalDeny && reason != "" {
		payload := map[string]string{"reason": reason}
		if encoded, err := json.Marshal(payload); err == nil {
			body = bytes.NewReader(encoded)
		}
	}

	req, err := http.NewRequest("POST", r.config.BaseURL+path, body)
	if err != nil {
		return fmt.Errorf("failed to create approval request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(r.config.Username, r.config.Password)

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("approval %s request failed: %w", decision, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approval %s returned status %d: %s", decision, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	return nil
}

func hasSuccessfulToolCallRetry(toolCalls []ToolCallEvent) bool {
	for _, tc := range toolCalls {
		if tc.Success {
			return true
		}
	}
	return false
}

func hasRetryableToolError(toolCalls []ToolCallEvent) bool {
	retryableIndicators := []string{
		"timeout",
		"timed out",
		"context deadline exceeded",
		"connection refused",
		"connection reset",
		"network is unreachable",
		"no such host",
		"i/o timeout",
		"server error",
		"502",
		"503",
		"504",
		"eof",
		"dial tcp",
		"temporarily",
		"query is required",
	}

	nonRetryableIndicators := []string{
		"read_only_violation",
		"strict_resolution",
		"routing_mismatch",
	}

	for _, tc := range toolCalls {
		if tc.Success {
			continue
		}
		lower := strings.ToLower(tc.Output)
		if lower == "" {
			continue
		}
		for _, indicator := range nonRetryableIndicators {
			if strings.Contains(lower, indicator) {
				goto next
			}
		}
		for _, indicator := range retryableIndicators {
			if strings.Contains(lower, indicator) {
				return true
			}
		}
	next:
	}

	return false
}

func (r *Runner) parseSSEStream(body io.Reader, approvalDecision ApprovalDecision, approvalReason string) ([]SSEEvent, []ToolCallEvent, []ApprovalEvent, string, string, error) {
	var events []SSEEvent
	var toolCalls []ToolCallEvent
	var approvals []ApprovalEvent
	var contentBuilder strings.Builder
	var sessionID string
	handledApprovals := make(map[string]struct{})

	// Track tool calls in progress
	toolCallsInProgress := make(map[string]*ToolCallEvent)

	scanner := bufio.NewScanner(body)
	// Allow large SSE events (tool results can be big).
	const maxSSEEventSize = 8 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "" {
			continue
		}

		// Parse the event
		var event struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			// Try parsing as raw event data
			continue
		}

		events = append(events, SSEEvent{
			Type: event.Type,
			Data: event.Data,
		})

		switch event.Type {
		case "session":
			var sessionData struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(event.Data, &sessionData); err == nil {
				sessionID = sessionData.ID
			}
		case "done":
			var doneData struct {
				SessionID string `json:"session_id"`
			}
			if err := json.Unmarshal(event.Data, &doneData); err == nil {
				if doneData.SessionID != "" {
					sessionID = doneData.SessionID
				}
			}

		case "content":
			var contentData struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal(event.Data, &contentData); err == nil {
				contentBuilder.WriteString(contentData.Text)
			}

		case "tool_start":
			var toolData struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Input string `json:"input"`
			}
			if err := json.Unmarshal(event.Data, &toolData); err == nil {
				toolCallsInProgress[toolData.ID] = &ToolCallEvent{
					ID:    toolData.ID,
					Name:  toolData.Name,
					Input: toolData.Input,
				}
			}

		case "tool_end":
			var toolData struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Output  string `json:"output"`
				Success bool   `json:"success"`
			}
			if err := json.Unmarshal(event.Data, &toolData); err == nil {
				if tc, ok := toolCallsInProgress[toolData.ID]; ok {
					tc.Output = toolData.Output
					tc.Success = toolData.Success
					toolCalls = append(toolCalls, *tc)
					delete(toolCallsInProgress, toolData.ID)
				} else {
					// Tool end without start
					toolCalls = append(toolCalls, ToolCallEvent{
						ID:      toolData.ID,
						Name:    toolData.Name,
						Output:  toolData.Output,
						Success: toolData.Success,
					})
				}
			}

		case "approval_needed":
			var approvalData struct {
				ApprovalID  string `json:"approval_id"`
				ToolID      string `json:"tool_id"`
				ToolName    string `json:"tool_name"`
				Command     string `json:"command"`
				Risk        string `json:"risk"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal(event.Data, &approvalData); err == nil {
				approvals = append(approvals, ApprovalEvent{
					ApprovalID:  approvalData.ApprovalID,
					ToolID:      approvalData.ToolID,
					ToolName:    approvalData.ToolName,
					Command:     approvalData.Command,
					Risk:        approvalData.Risk,
					Description: approvalData.Description,
				})
				if approvalDecision != ApprovalNone && approvalData.ApprovalID != "" {
					if _, ok := handledApprovals[approvalData.ApprovalID]; !ok {
						handledApprovals[approvalData.ApprovalID] = struct{}{}
						if err := r.handleApprovalDecision(approvalDecision, approvalData.ApprovalID, approvalReason); err != nil {
							return events, toolCalls, approvals, contentBuilder.String(), sessionID, err
						}
					}
				}
			}

		case "error":
			var errorData struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &errorData); err == nil {
				return events, toolCalls, approvals, contentBuilder.String(), sessionID, fmt.Errorf("stream error: %s", errorData.Message)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return events, toolCalls, approvals, contentBuilder.String(), sessionID, err
	}

	return events, toolCalls, approvals, contentBuilder.String(), sessionID, nil
}

func (r *Runner) printStepResult(result *StepResult) {
	fmt.Printf("\n--- Result ---\n")
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Session: %s\n", result.SessionID)
	if result.Retries > 0 {
		fmt.Printf("Retries: %d", result.Retries)
		if len(result.RetryNotes) > 0 {
			fmt.Printf(" (%s)", strings.Join(result.RetryNotes, ", "))
		}
		fmt.Printf("\n")
	}
	if len(result.Approvals) > 0 {
		fmt.Printf("Approvals: %d\n", len(result.Approvals))
	}

	if result.Error != nil {
		fmt.Printf("ERROR: %v\n", result.Error)
	}

	if len(result.ToolCalls) > 0 {
		fmt.Printf("\nTool Calls:\n")
		for _, tc := range result.ToolCalls {
			status := "OK"
			if !tc.Success {
				status = "FAILED"
			}
			fmt.Printf("  - %s [%s]: %s\n", tc.Name, status, truncate(tc.Input, 80))
			if !tc.Success || r.config.Verbose {
				fmt.Printf("    Output: %s\n", truncate(tc.Output, 200))
			}
		}
	}

	if result.Content != "" {
		fmt.Printf("\nAssistant Response:\n%s\n", truncate(result.Content, 500))
	}

	if len(result.Assertions) > 0 {
		fmt.Printf("\nAssertions:\n")
		for _, a := range result.Assertions {
			status := "PASS"
			if !a.Passed {
				status = "FAIL"
			}
			fmt.Printf("  [%s] %s: %s\n", status, a.Name, a.Message)
		}
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// PrintSummary prints a summary of the scenario result
func (r *Runner) PrintSummary(result ScenarioResult) {
	fmt.Printf("\n")
	fmt.Printf("========================================\n")
	fmt.Printf("SCENARIO: %s\n", result.ScenarioName)
	fmt.Printf("========================================\n")
	fmt.Printf("Duration: %v\n", result.Duration)

	passedSteps := 0
	totalRetries := 0
	for _, step := range result.Steps {
		if step.Success {
			passedSteps++
		}
		totalRetries += step.Retries
	}

	fmt.Printf("Steps: %d/%d passed\n", passedSteps, len(result.Steps))
	if totalRetries > 0 {
		fmt.Printf("Retries: %d\n", totalRetries)
		for _, step := range result.Steps {
			if step.Retries > 0 {
				note := ""
				if len(step.RetryNotes) > 0 {
					note = fmt.Sprintf(" (%s)", strings.Join(step.RetryNotes, ", "))
				}
				fmt.Printf("  - %s: %d%s\n", step.StepName, step.Retries, note)
			}
		}
	}
	if result.ReportPath != "" {
		fmt.Printf("Report: %s\n", result.ReportPath)
	}

	if result.Passed {
		fmt.Printf("Result: PASSED\n")
	} else {
		fmt.Printf("Result: FAILED\n")
		fmt.Printf("\nFailures:\n")
		for _, step := range result.Steps {
			if !step.Success {
				fmt.Printf("  - %s\n", step.StepName)
				if step.Error != nil {
					fmt.Printf("    Error: %v\n", step.Error)
				}
				for _, a := range step.Assertions {
					if !a.Passed {
						fmt.Printf("    Assertion '%s': %s\n", a.Name, a.Message)
					}
				}
			}
		}
	}
	fmt.Printf("========================================\n")
}
