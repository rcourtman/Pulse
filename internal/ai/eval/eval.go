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
	"strings"
	"time"
)

// Config holds eval runner configuration
type Config struct {
	BaseURL  string // e.g., "http://127.0.0.1:7655"
	Username string
	Password string
	Verbose  bool
}

// DefaultConfig returns a config for local development
func DefaultConfig() Config {
	return Config{
		BaseURL:  "http://127.0.0.1:7655",
		Username: "admin",
		Password: "admin",
		Verbose:  true,
	}
}

// Runner executes eval scenarios against the Pulse API
type Runner struct {
	config Config
	client *http.Client
}

// NewRunner creates a new eval runner
func NewRunner(config Config) *Runner {
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
	ToolCalls  []ToolCallEvent
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
}

// Step defines a single step in an eval scenario
type Step struct {
	Name       string
	Prompt     string
	Assertions []Assertion
}

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
	return result
}

func (r *Runner) executeStep(step Step, sessionID string) StepResult {
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
	resp, err := r.client.Do(req)
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
	result.RawEvents, result.ToolCalls, result.Content, result.SessionID, err = r.parseSSEStream(resp.Body)
	if err != nil {
		result.Error = fmt.Errorf("failed to parse SSE stream: %w", err)
		result.Success = false
		return result
	}

	result.Duration = time.Since(startTime)
	return result
}

func (r *Runner) parseSSEStream(body io.Reader) ([]SSEEvent, []ToolCallEvent, string, string, error) {
	var events []SSEEvent
	var toolCalls []ToolCallEvent
	var contentBuilder strings.Builder
	var sessionID string

	// Track tool calls in progress
	toolCallsInProgress := make(map[string]*ToolCallEvent)

	scanner := bufio.NewScanner(body)
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

		case "error":
			var errorData struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(event.Data, &errorData); err == nil {
				return events, toolCalls, contentBuilder.String(), sessionID, fmt.Errorf("stream error: %s", errorData.Message)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return events, toolCalls, contentBuilder.String(), sessionID, err
	}

	return events, toolCalls, contentBuilder.String(), sessionID, nil
}

func (r *Runner) printStepResult(result *StepResult) {
	fmt.Printf("\n--- Result ---\n")
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Session: %s\n", result.SessionID)

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
	for _, step := range result.Steps {
		if step.Success {
			passedSteps++
		}
	}

	fmt.Printf("Steps: %d/%d passed\n", passedSteps, len(result.Steps))

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
