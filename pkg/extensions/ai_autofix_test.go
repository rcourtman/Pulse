package extensions

import (
	"context"
	"testing"
)

type recordingApprovedAssistantToolExecutor struct {
	calls      int
	command    string
	approvalID string
	output     string
	exitCode   int
	err        error
}

func (e *recordingApprovedAssistantToolExecutor) ExecuteApprovedAssistantTool(_ context.Context, command, approvalID string) (string, int, error) {
	e.calls++
	e.command = command
	e.approvalID = approvalID
	return e.output, e.exitCode, e.err
}

func TestAIAutoFixHandlerDepsResolveApprovedAssistantToolExecutorReturnsNative(t *testing.T) {
	native := &recordingApprovedAssistantToolExecutor{output: "native-ok"}

	executor := (AIAutoFixHandlerDeps{
		AssistantToolExecutor: native,
	}).ResolveApprovedAssistantToolExecutor()
	if executor == nil {
		t.Fatal("expected native approved Assistant executor")
	}

	output, exitCode, err := executor.ExecuteApprovedAssistantTool(context.Background(), "pulse_control_guest()", "approval-1")
	if err != nil {
		t.Fatalf("ExecuteApprovedAssistantTool returned error: %v", err)
	}
	if output != "native-ok" || exitCode != 0 {
		t.Fatalf("ExecuteApprovedAssistantTool = (%q, %d), want native output", output, exitCode)
	}
	if native.calls != 1 || native.command != "pulse_control_guest()" || native.approvalID != "approval-1" {
		t.Fatalf("native executor was not called with approved tool request: %+v", native)
	}
}

func TestAIAutoFixHandlerDepsResolveApprovedAssistantToolExecutorReturnsNilWhenUnwired(t *testing.T) {
	if executor := (AIAutoFixHandlerDeps{}).ResolveApprovedAssistantToolExecutor(); executor != nil {
		t.Fatalf("ResolveApprovedAssistantToolExecutor() = %T, want nil", executor)
	}
}
