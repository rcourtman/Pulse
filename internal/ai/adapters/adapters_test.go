package adapters

import (
	"context"
	"testing"
	"time"
)

func TestForecastDataAdapter_NilHistory(t *testing.T) {
	adapter := NewForecastDataAdapter(nil)
	if adapter != nil {
		t.Error("Expected nil adapter for nil history")
	}
}

func TestCommandExecutorAdapter_Disabled(t *testing.T) {
	adapter := NewCommandExecutorAdapter()

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	output, err := adapter.Execute(ctx, "pve1", "echo test")
	if err == nil {
		t.Error("Expected error for disabled command execution")
	}
	if output != "" {
		t.Errorf("Expected empty output, got '%s'", output)
	}

	// Verify error type
	_, ok := err.(*CommandExecutionDisabledError)
	if !ok {
		t.Errorf("Expected CommandExecutionDisabledError, got %T", err)
	}
}

func TestCommandExecutionDisabledError_Message(t *testing.T) {
	err := &CommandExecutionDisabledError{
		Target:  "pve1",
		Command: "test command",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected non-empty error message")
	}
	if msg != "command execution is disabled - commands must be run manually" {
		t.Errorf("Unexpected error message: %s", msg)
	}
}
