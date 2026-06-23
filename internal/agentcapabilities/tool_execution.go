package agentcapabilities

import (
	"context"
	"errors"
	"fmt"
)

// ExecuteCapabilityToolHTTP projects one shared tool-call request onto the
// canonical agent capability manifest and wraps the upstream Pulse response in
// the shared tool result envelope.
func ExecuteCapabilityToolHTTP(ctx context.Context, client HTTPDoer, baseURL, token string, capabilities []Capability, params ToolCallParams) (ToolResult, error) {
	params = NormalizeToolCallParams(params)
	if err := ValidateToolCallParams(params); err != nil {
		return ToolResult{}, err
	}
	resp, err := CallRequestResponseCapabilityHTTPByName(ctx, client, baseURL, token, capabilities, params.Name, params.Arguments)
	if err != nil {
		var lookupErr CapabilityLookupError
		if errors.As(err, &lookupErr) {
			return ToolResult{}, fmt.Errorf("unknown tool: %s", lookupErr.Name)
		}
		return ToolResult{}, fmt.Errorf("call Pulse: %w", err)
	}
	return NewCapabilityHTTPToolResult(resp), nil
}

// DirectToolExecutionOptions names the surface-specific error messages used
// when native Pulse code executes a shared tool result directly instead of
// feeding it back into a model turn.
type DirectToolExecutionOptions struct {
	FailurePrefix           string
	ApprovalRequiredMessage string
	PolicyBlockedMessage    string
}

// DirectToolExecutionOutcome is the shared direct-execution projection of a
// tool result. OutputText is the text callers should expose as execution
// output; Interpretation keeps the full marker/result view available for
// diagnostics and future policy callers.
type DirectToolExecutionOutcome struct {
	OutputText     string
	Interpretation ToolResultInterpretation
}

// InterpretDirectToolExecution maps a shared tool result into the output and
// error contract used by native Assistant direct execution paths.
// This keeps marker detection, isError handling, and direct-execution failure
// messages out of individual chat or adapter surfaces.
func InterpretDirectToolExecution(result ToolResult, opts DirectToolExecutionOptions) (DirectToolExecutionOutcome, error) {
	interpreted := InterpretToolResult(result)
	outcome := DirectToolExecutionOutcome{
		OutputText:     interpreted.Text,
		Interpretation: interpreted,
	}

	if interpreted.IsError {
		prefix := firstNonEmptyString(opts.FailurePrefix, "tool execution failed")
		return outcome, fmt.Errorf("%s: %s", prefix, interpreted.Text)
	}

	if interpreted.ApprovalRequired {
		outcome.OutputText = ""
		message := firstNonEmptyString(opts.ApprovalRequiredMessage, "tool requires approval")
		return outcome, errors.New(message)
	}

	if interpreted.PolicyBlocked {
		outcome.OutputText = ""
		message := firstNonEmptyString(opts.PolicyBlockedMessage, "tool blocked by security policy")
		return outcome, errors.New(message)
	}

	return outcome, nil
}
