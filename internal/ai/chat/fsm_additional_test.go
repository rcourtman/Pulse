package chat

import (
	"strings"
	"testing"
)

func TestFSMBlockedErrorFormatting(t *testing.T) {
	err := &FSMBlockedError{
		State:    StateWriting,
		ToolName: "pulse_control",
		ToolKind: ToolKindWrite,
		Reason:   "requires approval",
	}
	msg := err.Error()
	if !strings.Contains(msg, "pulse_control") {
		t.Fatalf("expected tool name in error message")
	}
	if err.Code() != "FSM_BLOCKED" {
		t.Fatalf("expected FSM_BLOCKED code")
	}

	err = &FSMBlockedError{State: StateReading, Reason: "test"}
	if !strings.Contains(err.Error(), string(StateReading)) {
		t.Fatalf("expected state in error message")
	}
}

func TestToolKindStringUnknown(t *testing.T) {
	var k ToolKind = 99
	if k.String() != "unknown" {
		t.Fatalf("expected unknown for invalid tool kind")
	}
}
