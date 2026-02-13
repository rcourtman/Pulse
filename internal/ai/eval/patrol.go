package eval

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PatrolScenario defines a patrol eval scenario.
type PatrolScenario struct {
	Name        string
	Description string
	Setup       func(r *Runner) error // optional pre-run setup
	Teardown    func(r *Runner) error // optional post-run cleanup
	Assertions  []PatrolAssertion
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
	Completed    bool // true if patrol reported completion via status API
	Quality      *PatrolQualityReport
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
	Type         string `json:"type"`
	Content      string `json:"content,omitempty"`
	Phase        string `json:"phase,omitempty"`
	Tokens       int    `json:"tokens,omitempty"`
	ToolID       string `json:"tool_id,omitempty"`
	ToolName     string `json:"tool_name,omitempty"`
	ToolInput    string `json:"tool_input,omitempty"`
	ToolRawInput string `json:"tool_raw_input,omitempty"`
	ToolOutput   string `json:"tool_output,omitempty"`
	ToolSuccess  *bool  `json:"tool_success,omitempty"`
}

// RunPatrolScenario executes a patrol scenario and returns the results.
//
// Strategy: trigger the patrol run, then use a dual approach:
//  1. Poll GET /api/ai/patrol/status until Running=false (primary completion signal)
//  2. Attempt to connect to the SSE stream in a goroutine to capture tool events
//
// The SSE stream may or may not connect depending on timing (the server only
// sends HTTP headers once it has data). We treat the stream as best-effort
// for tool-level visibility, and rely on polling for completion.
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

	// Optional patrol model override (e.g., to force a cheaper model for evals)
	restoreModel, overrideErr := r.applyPatrolModelOverride(ctx)
	if overrideErr != nil {
		result.Error = fmt.Errorf("patrol model override failed: %w", overrideErr)
		result.Success = false
		result.Duration = time.Since(startTime)
		return result
	}
	if restoreModel != nil {
		defer restoreModel()
	}

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

	// Trigger patrol run
	if err := r.triggerPatrolRun(); err != nil {
		result.Error = fmt.Errorf("triggering patrol run: %w", err)
		result.Success = false
		result.Duration = time.Since(startTime)
		return result
	}

	if r.config.Verbose {
		fmt.Printf("  Patrol triggered\n")
	}

	// Start SSE stream reader in background goroutine.
	// This captures tool events if the stream connects. It's best-effort.
	var streamMu sync.Mutex
	var streamEvents []PatrolSSEEvent
	var streamToolCalls []ToolCallEvent
	var streamContent strings.Builder
	var streamConnected bool

	streamCtx, streamCancel := context.WithCancel(ctx)
	defer streamCancel()

	streamDone := make(chan struct{})
	go func() {
		defer close(streamDone)
		body, err := r.connectPatrolStream(streamCtx)
		if err != nil {
			// Stream didn't connect — that's OK, we still poll for completion
			if r.config.Verbose && streamCtx.Err() == nil {
				fmt.Printf("  [SSE] Could not connect to stream: %v\n", err)
			}
			return
		}
		defer body.Close()

		streamMu.Lock()
		streamConnected = true
		streamMu.Unlock()

		if r.config.Verbose {
			fmt.Printf("  [SSE] Connected to patrol stream\n")
		}

		events, toolCalls, content, _ := r.parsePatrolSSEStream(streamCtx, body)
		streamMu.Lock()
		streamEvents = events
		streamToolCalls = toolCalls
		streamContent.WriteString(content)
		streamMu.Unlock()
	}()

	// Poll for completion (primary mechanism)
	completed, pollErr := r.waitForPatrolComplete(ctx)
	result.Completed = completed

	// Cancel the stream goroutine and wait for it to finish
	streamCancel()
	<-streamDone

	if pollErr != nil {
		result.Error = pollErr
		result.Success = false
	}

	// Collect stream results
	streamMu.Lock()
	result.RawEvents = streamEvents
	result.ToolCalls = streamToolCalls
	result.Content = streamContent.String()
	if streamConnected && r.config.Verbose {
		fmt.Printf("  [SSE] Captured %d events, %d tool calls\n", len(streamEvents), len(streamToolCalls))
	} else if !streamConnected && r.config.Verbose {
		fmt.Printf("  [SSE] Stream did not connect (tool events not captured)\n")
	}
	streamMu.Unlock()

	// Fetch findings from REST API
	findings, findErr := r.fetchPatrolFindings()
	if findErr != nil {
		if result.Error == nil {
			result.Error = fmt.Errorf("fetching findings: %w", findErr)
		}
	}
	result.Findings = mergeFindingsFromToolCalls(findings, result.ToolCalls)

	if r.config.Verbose && len(findings) > 0 {
		fmt.Printf("  Fetched %d findings from API\n", len(findings))
	}

	// Compute quality metrics (best-effort)
	result.Quality = EvaluatePatrolQuality(&result)

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

func mergeFindingsFromToolCalls(findings []PatrolFinding, toolCalls []ToolCallEvent) []PatrolFinding {
	if len(toolCalls) == 0 {
		return findings
	}

	byID := make(map[string]PatrolFinding, len(findings))
	for _, f := range findings {
		if f.ID == "" {
			continue
		}
		byID[f.ID] = f
	}

	for _, tc := range toolCalls {
		if tc.Name != "patrol_get_findings" || strings.TrimSpace(tc.Output) == "" {
			continue
		}

		var payload struct {
			Findings []struct {
				ID             string `json:"id"`
				Key            string `json:"key"`
				Severity       string `json:"severity"`
				Category       string `json:"category"`
				ResourceID     string `json:"resource_id"`
				ResourceName   string `json:"resource_name"`
				ResourceType   string `json:"resource_type"`
				Title          string `json:"title"`
				Description    string `json:"description"`
				Recommendation string `json:"recommendation,omitempty"`
				Evidence       string `json:"evidence,omitempty"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(tc.Output), &payload); err != nil {
			continue
		}

		for _, info := range payload.Findings {
			if info.ID == "" {
				continue
			}
			if _, exists := byID[info.ID]; exists {
				continue
			}
			byID[info.ID] = PatrolFinding{
				ID:             info.ID,
				Key:            info.Key,
				Severity:       info.Severity,
				Category:       info.Category,
				ResourceID:     info.ResourceID,
				ResourceName:   info.ResourceName,
				ResourceType:   info.ResourceType,
				Title:          info.Title,
				Description:    info.Description,
				Recommendation: info.Recommendation,
				Evidence:       info.Evidence,
			}
		}
	}

	if len(byID) == len(findings) {
		return findings
	}

	merged := make([]PatrolFinding, 0, len(byID))
	for _, f := range byID {
		merged = append(merged, f)
	}
	return merged
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
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			resp.Body.Close()
			return fmt.Errorf("decode patrol status: %w", err)
		}
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

// waitForPatrolComplete polls status until patrol finishes (Running transitions
// from true back to false). Returns true if patrol completed, false on timeout.
func (r *Runner) waitForPatrolComplete(ctx context.Context) (bool, error) {
	// First, wait briefly for patrol to actually start (Running=true)
	sawRunning := false
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for patrol to start")
		case <-time.After(1 * time.Second):
		}

		running, healthy, err := r.getPatrolStatus(ctx)
		if err != nil {
			continue
		}
		if running {
			sawRunning = true
			if r.config.Verbose {
				fmt.Printf("  Patrol is running...\n")
			}
			break
		}
		// If not running and we see a recent completion, maybe it finished instantly
		if !running && i > 2 {
			if r.config.Verbose {
				fmt.Printf("  Patrol not running (may have completed quickly), healthy=%v\n", healthy)
			}
			return true, nil
		}
	}

	if !sawRunning {
		// May have completed extremely fast, check findings to verify it ran
		if r.config.Verbose {
			fmt.Printf("  Never saw patrol running state (may have completed instantly)\n")
		}
		return true, nil
	}

	// Now poll until Running=false (patrol completed)
	for {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout waiting for patrol to complete")
		case <-time.After(3 * time.Second):
		}

		running, healthy, err := r.getPatrolStatus(ctx)
		if err != nil {
			if r.config.Verbose {
				fmt.Printf("  Status poll error: %v\n", err)
			}
			continue
		}

		if !running {
			if r.config.Verbose {
				fmt.Printf("  Patrol completed (healthy=%v)\n", healthy)
			}
			return true, nil
		}

		if r.config.Verbose {
			fmt.Printf("  Still running...\n")
		}
	}
}

// getPatrolStatus returns (running, healthy, error) from the status endpoint.
func (r *Runner) getPatrolStatus(ctx context.Context) (bool, bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.config.BaseURL+"/api/ai/patrol/status", nil)
	if err != nil {
		return false, false, err
	}
	req.SetBasicAuth(r.config.Username, r.config.Password)

	resp, err := r.client.Do(req)
	if err != nil {
		return false, false, err
	}
	defer resp.Body.Close()

	var status struct {
		Running bool `json:"running"`
		Healthy bool `json:"healthy"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return false, false, err
	}
	return status.Running, status.Healthy, nil
}

// triggerPatrolRun triggers POST /api/ai/patrol/run.
func (r *Runner) triggerPatrolRun() error {
	patrolRunURL := r.config.BaseURL + "/api/ai/patrol/run"

	req, err := http.NewRequest("POST", patrolRunURL, nil)
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
				input := event.ToolInput
				if event.ToolRawInput != "" {
					input = event.ToolRawInput
				}
				toolCallsInProgress[event.ToolID] = &ToolCallEvent{
					ID:    event.ToolID,
					Name:  event.ToolName,
					Input: input,
				}

			case "tool_end":
				success := event.ToolSuccess != nil && *event.ToolSuccess
				if tc, ok := toolCallsInProgress[event.ToolID]; ok {
					input := event.ToolInput
					if event.ToolRawInput != "" {
						input = event.ToolRawInput
					}
					if tc.Input == "" && input != "" {
						tc.Input = input
					}
					tc.Output = event.ToolOutput
					tc.Success = success
					toolCalls = append(toolCalls, *tc)
					delete(toolCallsInProgress, event.ToolID)
				} else {
					input := event.ToolInput
					if event.ToolRawInput != "" {
						input = event.ToolRawInput
					}
					// tool_end without matching tool_start
					toolCalls = append(toolCalls, ToolCallEvent{
						ID:      event.ToolID,
						Name:    event.ToolName,
						Input:   input,
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
		return events, toolCalls, contentBuilder.String(), nil // context cancel is expected
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
	fmt.Printf("Completed: %v\n", result.Completed)

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

	if result.Quality != nil {
		q := result.Quality
		fmt.Printf("\nQuality:\n")
		if q.CoverageKnown {
			if q.SignalsTotal > 0 {
				fmt.Printf("  Signal coverage: %d/%d (%.0f%%)\n", q.SignalsMatched, q.SignalsTotal, q.SignalCoverage*100)
			} else {
				fmt.Printf("  Signal coverage: no signals detected\n")
			}
		} else {
			fmt.Printf("  Signal coverage: unknown (no tool calls captured)\n")
		}
		if r.config.Verbose && len(q.Signals) > 0 {
			fmt.Printf("  Signals:\n")
			for _, s := range q.Signals {
				status := "MISS"
				if s.Matched {
					status = "MATCH"
				}
				fmt.Printf("    - [%s] %s on %s (%s)\n", status, s.SignalType, s.ResourceID, s.Category)
			}
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
