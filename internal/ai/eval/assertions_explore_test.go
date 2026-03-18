package eval

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssertExploreStatusSeen(t *testing.T) {
	passResult := &StepResult{
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"started","message":"running","model":"openai:gpt-4.1"}`),
			},
		},
	}
	failResult := &StepResult{
		RawEvents: []SSEEvent{
			{Type: "content", Data: json.RawMessage(`{"text":"ok"}`)},
		},
	}

	assert.True(t, AssertExploreStatusSeen()(passResult).Passed)
	assert.False(t, AssertExploreStatusSeen()(failResult).Passed)
}

func TestAssertExploreLifecycleValid(t *testing.T) {
	validStartedCompleted := &StepResult{
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"started","message":"running","model":"openai:gpt-4.1"}`),
			},
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"completed","message":"done","model":"openai:gpt-4.1","outcome":"success"}`),
			},
		},
	}
	validSkipped := &StepResult{
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"skipped","message":"no model","outcome":"skipped_no_model"}`),
			},
		},
	}
	invalidMissingModel := &StepResult{
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"started","message":"running"}`),
			},
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"completed","message":"done","outcome":"success"}`),
			},
		},
	}
	invalidNoTerminal := &StepResult{
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"started","message":"running","model":"openai:gpt-4.1"}`),
			},
		},
	}

	assert.True(t, AssertExploreLifecycleValid()(validStartedCompleted).Passed)
	assert.True(t, AssertExploreLifecycleValid()(validSkipped).Passed)
	assert.False(t, AssertExploreLifecycleValid()(invalidMissingModel).Passed)
	assert.False(t, AssertExploreLifecycleValid()(invalidNoTerminal).Passed)
}

func TestAssertExploreFallbackHasContent(t *testing.T) {
	completedNoFallback := &StepResult{
		Content: "summary",
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"started","message":"running","model":"openai:gpt-4.1"}`),
			},
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"completed","message":"done","outcome":"success"}`),
			},
		},
	}
	failedWithContent := &StepResult{
		Content: "main response still produced",
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"failed","message":"failed","outcome":"failed"}`),
			},
		},
	}
	failedEmptyContent := &StepResult{
		RawEvents: []SSEEvent{
			{
				Type: "explore_status",
				Data: json.RawMessage(`{"phase":"failed","message":"failed","outcome":"failed"}`),
			},
		},
	}

	assert.True(t, AssertExploreFallbackHasContent()(completedNoFallback).Passed)
	assert.True(t, AssertExploreFallbackHasContent()(failedWithContent).Passed)
	assert.False(t, AssertExploreFallbackHasContent()(failedEmptyContent).Passed)
}
