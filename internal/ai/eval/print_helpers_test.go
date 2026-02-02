package eval

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// captureOutput captures stdout during the execution of f
func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	out, _ := io.ReadAll(r)
	return string(out)
}

func TestPrintStepResult(t *testing.T) {
	runner := &Runner{config: DefaultConfig()} // Verbose defaults to true
	runner.config.Verbose = true

	result := &StepResult{
		StepName:     "Test Step",
		Prompt:       "Do something",
		Success:      true,
		Duration:     100 * time.Millisecond,
		InputTokens:  10,
		OutputTokens: 20,
		Content:      "Done.",
		ToolCalls: []ToolCallEvent{
			{Name: "tool1", Input: "arg", Output: "res", Success: true},
		},
		Assertions: []AssertionResult{
			{Name: "check", Passed: true, Message: "ok"},
		},
	}

	output := captureOutput(func() {
		runner.printStepResult(result)
	})

	assert.Contains(t, output, "Duration:")
	assert.Contains(t, output, "tool1")
	assert.Contains(t, output, "[OK]:")  // Tool call status
	assert.Contains(t, output, "[PASS]") // Assertion status
}

func TestPrintStepResult_Failure(t *testing.T) {
	runner := &Runner{config: DefaultConfig()}
	result := &StepResult{
		StepName: "Fail Step",
		Success:  false,
		Error:    fmt.Errorf("oops"),
		Assertions: []AssertionResult{
			{Name: "bad", Passed: false, Message: "failed"},
		},
	}

	output := captureOutput(func() {
		runner.printStepResult(result)
	})

	assert.Contains(t, output, "ERROR: oops")
	assert.Contains(t, output, "[FAIL] bad: failed")
}

func TestPrintPatrolSummary(t *testing.T) {
	runner := &Runner{config: DefaultConfig()}
	runner.config.Verbose = true

	result := PatrolRunResult{
		ScenarioName: "Patrol 1",
		Success:      true,
		Duration:     5 * time.Second,
		Completed:    true,
		ToolCalls: []ToolCallEvent{
			{Name: "scan", Input: "all", Output: "ok", Success: true},
		},
		Findings: []PatrolFinding{
			{Key: "k1", Severity: "high", Title: "issue"},
		},
		Content: "Summary text",
		Assertions: []AssertionResult{
			{Name: "a1", Passed: true, Message: "good"},
		},
		Quality: &PatrolQualityReport{
			CoverageKnown:  true,
			SignalsMatched: 1,
			SignalsTotal:   1,
			SignalCoverage: 1.0,
			Signals: []PatrolSignalResult{
				{SignalType: "s1", Matched: true, ResourceID: "r1"},
			},
		},
	}

	output := captureOutput(func() {
		runner.PrintPatrolSummary(result)
	})

	assert.Contains(t, output, "PATROL SCENARIO: Patrol 1")
	assert.Contains(t, output, "Result: PASSED")
	assert.Contains(t, output, "scan")
	assert.Contains(t, output, "k1")
	assert.Contains(t, output, "Summary text")
	assert.Contains(t, output, "Signal coverage: 1/1 (100%)")
}

func TestPrintPatrolSummary_Failure(t *testing.T) {
	runner := &Runner{config: DefaultConfig()}
	result := PatrolRunResult{
		ScenarioName: "Patrol Fail",
		Success:      false,
		Error:        fmt.Errorf("crash"),
		Assertions: []AssertionResult{
			{Name: "a1", Passed: false, Message: "bad"},
		},
	}

	output := captureOutput(func() {
		runner.PrintPatrolSummary(result)
	})

	assert.Contains(t, output, "ERROR: crash")
	assert.Contains(t, output, "Result: FAILED")
	assert.Contains(t, output, "Assertion 'a1': bad")
}
