package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PatrolScenario defines a patrol eval scenario.
type PatrolScenario struct {
	Name        string
	Description string
	Setup       func(r *Runner) error // optional pre-run setup
	Teardown    func(r *Runner) error // optional post-run cleanup
	Assertions  []PatrolAssertion
	Deep        bool          // trigger deep patrol
	Timeout     time.Duration // default 5m
}

// PatrolRunResult captures complete patrol execution trace.
type PatrolRunResult struct {
	ScenarioName string
	Success      bool
	Error        error
	Duration     time.Duration
	ToolCalls    []ToolCallEvent
	Findings     []PatrolFinding
	Content      string
	RawEvents    []PatrolSSEEvent
	Assertions   []AssertionResult
}

// PatrolFinding mirrors the Finding JSON from the API.
type PatrolFinding struct {
	ID             string `json:"id"`
	Key            string `json:"key"`
	Severity       string `json:"severity"`
	Category       string `json:"category"`
	ResourceID     string `json:"resource_id"`
	ResourceName   string `json:"resource_name"`
	ResourceType   string `json:"resource_type"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Recommendation string `json:"recommendation"`
	Evidence       string `json:"evidence"`
}

// PatrolAssertion checks a PatrolRunResult.
type PatrolAssertion func(result *PatrolRunResult) AssertionResult

// PatrolSSEEvent represents a raw SSE event from the patrol stream.
type PatrolSSEEvent struct {
	Type        string `json:"type"`
	Content     string `json:"content,omitempty"`
	Phase       string `json:"phase,omitempty"`
	Tokens      int    `json:"tokens,omitempty"`
	ToolID      string `json:"tool_id,omitempty"`
	ToolName    string `json:"tool_name,omitempty"`
	ToolInput   string `json:"tool_input,omitempty"`
	ToolOutput  string `json:"tool_output,omitempty"`
	ToolSuccess *bool  `json:"tool_success,omitempty"`
}

// RunPatrolScenario executes a patrol scenario and returns the results.
func (r *Runner) RunPatrolScenario(scenario PatrolScenario) PatrolRunResult {
	startTime := time.Now()
	result := PatrolRunResult{
		ScenarioName: scenario.Name,
		Success:      true,
	}

	timeout := scenario.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Run optional setup
	if scenario.Setup != nil {
		if err := scenario.Setup(r); err != nil {
			result.Error = fmt.Errorf("setup failed: %w", err)
			result.Success = false
			result.Duration = time.Since(startTime)
			return result
		}
	}

	// Run optional teardown on exit
	if scenario.Teardown != nil {
		defer func() {
			if err := scenario.Teardown(r); err != nil {
				fmt.Printf("  [WARN] Teardown error: %v\n", err)
			}
		}()
	}

	// Wait for patrol to be idle before starting
	if err := r.waitForPatrolIdle(ctx); err != nil {
		result.Error = fmt.Errorf("waiting for patrol idle: %w", err)
		result.Success = false
		result.Duration = time.Since(startTime)
		return result
	}

	// Connect to SSE stream before triggering (to catch "start" event)
	streamBody, err := r.connectPatrolStream(ctx)
	if err != nil {
		result.Error = fmt.Errorf("connecting to patrol stream: %w", err)
		result.Success = false
		result.Duration = time.Since(startTime)
		return result
	}
	defer streamBody.Close()

	// Trigger patrol run
	if err := r.triggerPatrolRun(scenario.Deep); err != nil {
		result.Error = fmt.Errorf("triggering patrol run: %w", err)
		result.Success = false
		result.Duration = time.Since(startTime)
		return result
	}

	if r.config.Verbose {
		fmt.Printf("  Patrol triggered (deep=%v), reading SSE stream...\n", scenario.Deep)
	}

	// Parse SSE stream until complete/error/timeout
	rawEvents, toolCalls, content, streamErr := r.parsePatrolSSEStream(ctx, streamBody)
	result.RawEvents = rawEvents
	result.ToolCalls = toolCalls
	result.Content = content
	if streamErr != nil {
		result.Error = streamErr
		result.Success = false
	}

	// Fetch findings from REST API
	findings, findErr := r.fetchPatrolFindings()
	if findErr != nil {
		if result.Error == nil {
			result.Error = fmt.Errorf("fetching findings: %w", findErr)
		}
	}
	result.Findings = findings

	// Run assertions
	for _, assertion := range scenario.Assertions {
		assertResult := assertion(&result)
		result.Assertions = append(result.Assertions, assertResult)
		if !assertResult.Passed {
			result.Success = false
		}
	}

	result.Duration = time.Since(startTime)
	return result
}

// waitForPatrolIdle polls GET /api/ai/patrol/status until Running=false.
func (r *Runner) waitForPatrolIdle(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for patrol idle")
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", r.config.BaseURL+"/api/ai/patrol/status", nil)
		if err != nil {
			return err
		}
		req.SetBasicAuth(r.config.Username, r.config.Password)

		resp, err := r.client.Do(req)
		if err != nil {
			return err
		}

		var status struct {
			Running bool `json:"running"`
		}
		json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()

		if !status.Running {
			return nil
		}

		if r.config.Verbose {
			fmt.Printf("  Patrol is running, waiting...\n")
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for patrol idle")
		case <-time.After(3 * time.Second):
		}
	}
}

// triggerPatrolRun triggers POST /api/ai/patrol/run.
func (r *Runner) triggerPatrolRun(deep bool) error {
	url := r.config.BaseURL + "/api/ai/patrol/run"
	if deep {
		url += "?deep=true"
	}

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(r.config.Username, r.config.Password)

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("patrol run returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// fetchPatrolFindings fetches GET /api/ai/patrol/findings.
func (r *Runner) fetchPatrolFindings() ([]PatrolFinding, error) {
	req, err := http.NewRequest("GET", r.config.BaseURL+"/api/ai/patrol/findings", nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(r.config.Username, r.config.Password)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("findings returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var findings []PatrolFinding
	if err := json.NewDecoder(resp.Body).Decode(&findings); err != nil {
		return nil, fmt.Errorf("decoding findings: %w", err)
	}
	return findings, nil
}

// connectPatrolStream opens GET /api/ai/patrol/stream SSE connection.
func (r *Runner) connectPatrolStream(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.config.BaseURL+"/api/ai/patrol/stream", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.SetBasicAuth(r.config.Username, r.config.Password)

	// Use a client without the default timeout for streaming
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return resp.Body, nil
}

// parsePatrolSSEStream reads patrol SSE events and extracts tool calls + content.
// Patrol SSE events are flat JSON objects (not nested like chat SSE).
func (r *Runner) parsePatrolSSEStream(ctx context.Context, body io.Reader) ([]PatrolSSEEvent, []ToolCallEvent, string, error) {
	var events []PatrolSSEEvent
	var toolCalls []ToolCallEvent
	var contentBuilder strings.Builder

	// Track tool calls in progress (by ID)
	toolCallsInProgress := make(map[string]*ToolCallEvent)

	scanner := bufio.NewScanner(body)
	const maxSSEEventSize = 8 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSEEventSize)

	done := make(chan struct{})
	var scanErr error

	go func() {
		defer close(done)
		for scanner.Scan() {
			line := scanner.Text()

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "" {
				continue
			}

			var event PatrolSSEEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			events = append(events, event)

			if r.config.Verbose {
				switch event.Type {
				case "start":
					fmt.Printf("  [SSE] Patrol started\n")
				case "phase":
					fmt.Printf("  [SSE] Phase: %s\n", event.Phase)
				case "tool_start":
					fmt.Printf("  [SSE] Tool start: %s\n", event.ToolName)
				case "tool_end":
					status := "OK"
					if event.ToolSuccess != nil && !*event.ToolSuccess {
						status = "FAILED"
					}
					fmt.Printf("  [SSE] Tool end: %s [%s]\n", event.ToolName, status)
				case "complete":
					fmt.Printf("  [SSE] Patrol complete\n")
				case "error":
					fmt.Printf("  [SSE] Error: %s\n", event.Content)
				}
			}

			switch event.Type {
			case "content":
				contentBuilder.WriteString(event.Content)

			case "tool_start":
				toolCallsInProgress[event.ToolID] = &ToolCallEvent{
					ID:    event.ToolID,
					Name:  event.ToolName,
					Input: event.ToolInput,
				}

			case "tool_end":
				success := event.ToolSuccess != nil && *event.ToolSuccess
				if tc, ok := toolCallsInProgress[event.ToolID]; ok {
					tc.Output = event.ToolOutput
					tc.Success = success
					toolCalls = append(toolCalls, *tc)
					delete(toolCallsInProgress, event.ToolID)
				} else {
					// tool_end without matching tool_start
					toolCalls = append(toolCalls, ToolCallEvent{
						ID:      event.ToolID,
						Name:    event.ToolName,
						Input:   event.ToolInput,
						Output:  event.ToolOutput,
						Success: success,
					})
				}

			case "complete":
				return

			case "error":
				scanErr = fmt.Errorf("patrol error: %s", event.Content)
				return
			}
		}
		if err := scanner.Err(); err != nil {
			scanErr = err
		}
	}()

	select {
	case <-ctx.Done():
		return events, toolCalls, contentBuilder.String(), fmt.Errorf("timeout reading patrol stream")
	case <-done:
		return events, toolCalls, contentBuilder.String(), scanErr
	}
}

// PrintPatrolSummary prints a summary of the patrol run result.
func (r *Runner) PrintPatrolSummary(result PatrolRunResult) {
	fmt.Printf("\n")
	fmt.Printf("========================================\n")
	fmt.Printf("PATROL SCENARIO: %s\n", result.ScenarioName)
	fmt.Printf("========================================\n")
	fmt.Printf("Duration: %v\n", result.Duration)

	if result.Error != nil {
		fmt.Printf("ERROR: %v\n", result.Error)
	}

	if len(result.ToolCalls) > 0 {
		fmt.Printf("\nTool Calls (%d):\n", len(result.ToolCalls))
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

	if len(result.Findings) > 0 {
		fmt.Printf("\nFindings (%d):\n", len(result.Findings))
		for _, f := range result.Findings {
			fmt.Printf("  - [%s] %s: %s\n", f.Severity, f.Key, f.Title)
		}
	}

	if result.Content != "" && r.config.Verbose {
		fmt.Printf("\nContent:\n%s\n", truncate(result.Content, 500))
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

	if result.Success {
		fmt.Printf("\nResult: PASSED\n")
	} else {
		fmt.Printf("\nResult: FAILED\n")
		if len(result.Assertions) > 0 {
			fmt.Printf("\nFailures:\n")
			for _, a := range result.Assertions {
				if !a.Passed {
					fmt.Printf("  Assertion '%s': %s\n", a.Name, a.Message)
				}
			}
		}
	}
	fmt.Printf("========================================\n")
}
