package opencode

import (
	"encoding/json"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBridge_TransformEvent(t *testing.T) {
	bridge := NewBridge()

	tests := []struct {
		name      string
		input     StreamEvent
		wantType  string
		checkData func(*testing.T, interface{})
		wantErr   bool
	}{
		{
			name: "content event (text)",
			input: StreamEvent{
				Type: "content",
				Data: json.RawMessage(`{"content":"hello world"}`),
			},
			wantType: "content",
			checkData: func(t *testing.T, data interface{}) {
				assert.Equal(t, "hello world", data)
			},
		},
		{
			name: "content event (delta)",
			input: StreamEvent{
				Type: "content",
				Data: json.RawMessage(`{"delta":"hello"}`),
			},
			wantType: "content",
			checkData: func(t *testing.T, data interface{}) {
				assert.Equal(t, "hello", data)
			},
		},
		{
			name: "thinking event",
			input: StreamEvent{
				Type: "thinking",
				Data: json.RawMessage(`{"content":"hmm..."}`),
			},
			wantType: "thinking",
			checkData: func(t *testing.T, data interface{}) {
				assert.Equal(t, "hmm...", data)
			},
		},
		{
			name: "tool use event",
			input: StreamEvent{
				Type: "tool_use",
				Data: json.RawMessage(`{"id":"call_1","name":"pulse_run_command","input":{"command":"ls -la"}}`),
			},
			wantType: "tool_start",
			checkData: func(t *testing.T, data interface{}) {
				d, ok := data.(ai.ToolStartData)
				require.True(t, ok)
				assert.Equal(t, "run_command", d.Name)
				assert.Equal(t, "ls -la", d.Input)
			},
		},
		{
			name: "tool use event (generic)",
			input: StreamEvent{
				Type: "tool_use",
				Data: json.RawMessage(`{"id":"call_2","name":"unknown_tool","input":{"foo":"bar"}}`),
			},
			wantType: "tool_start",
			checkData: func(t *testing.T, data interface{}) {
				d, ok := data.(ai.ToolStartData)
				require.True(t, ok)
				assert.Equal(t, "unknown_tool", d.Name)
				// JSON string check might be flaky due to key ordering, but simple map usually ok
				assert.Contains(t, d.Input, `"foo":"bar"`)
			},
		},
		{
			name: "tool result event (success)",
			input: StreamEvent{
				Type: "tool_result",
				Data: json.RawMessage(`{"id":"call_1","name":"pulse_run_command","output":"file1 file2","success":true}`),
			},
			wantType: "tool_end",
			checkData: func(t *testing.T, data interface{}) {
				d, ok := data.(ai.ToolEndData)
				require.True(t, ok)
				assert.Equal(t, "run_command", d.Name)
				assert.Equal(t, "file1 file2", d.Output)
				assert.True(t, d.Success)
			},
		},
		{
			name: "tool result event (approval needed)",
			input: StreamEvent{
				Type: "tool_result",
				Data: json.RawMessage(`{"id":"call_3","name":"pulse_run_command","output":"APPROVAL_REQUIRED: {\"command\":\"rm -rf /\",\"run_on_host\":true}","success":true}`),
			},
			wantType: "approval_needed",
			checkData: func(t *testing.T, data interface{}) {
				d, ok := data.(ai.ApprovalNeededData)
				require.True(t, ok)
				assert.Equal(t, "run_command", d.ToolName)
				assert.Equal(t, "call_3", d.ToolID)
				assert.Equal(t, "rm -rf /", d.Command)
				assert.True(t, d.RunOnHost)
			},
		},
		{
			name: "complete event",
			input: StreamEvent{
				Type: "complete",
				Data: json.RawMessage(`{"sessionId":"ses-123","usage":{"inputTokens":10,"outputTokens":20},"model":{"id":"gpt-4","provider":"openai"}}`),
			},
			wantType: "done",
			checkData: func(t *testing.T, data interface{}) {
				m, ok := data.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "ses-123", m["session_id"])
				assert.Equal(t, "gpt-4", m["model"])
			},
		},
		{
			name: "error event (structured)",
			input: StreamEvent{
				Type: "error",
				Data: json.RawMessage(`{"error":"bad_request","message":"Something went wrong"}`),
			},
			wantType: "error",
			checkData: func(t *testing.T, data interface{}) {
				assert.Equal(t, "Something went wrong", data)
			},
		},
		{
			name: "error event (simple)",
			input: StreamEvent{
				Type: "error",
				Data: json.RawMessage(`{"error":"Just an error"}`),
			},
			wantType: "error",
			checkData: func(t *testing.T, data interface{}) {
				assert.Equal(t, "Just an error", data)
			},
		},
		{
			name: "unknown event",
			input: StreamEvent{
				Type: "unknown_type",
				Data: json.RawMessage(`"raw data"`),
			},
			wantType: "content",
			checkData: func(t *testing.T, data interface{}) {
				assert.Equal(t, "\"raw data\"", data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := bridge.TransformEvent(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantType, got.Type)
			if tt.checkData != nil {
				tt.checkData(t, got.Data)
			}
		})
	}
}

func TestBridge_StreamEvents(t *testing.T) {
	bridge := NewBridge()
	ocEvents := make(chan StreamEvent, 5)
	ocErrors := make(chan error, 5)

	// Feed some events
	ocEvents <- StreamEvent{
		Type: "content",
		Data: json.RawMessage(`{"delta":"hello"}`),
	}
	ocEvents <- StreamEvent{
		Type: "complete",
		Data: json.RawMessage(`{"sessionId":"ses-1"}`),
	}

	// Dont close ocEvents immediately to avoid race if we want to ensure processing order,
	// but here we want to ensure it handles the events we sent.
	// We MUST close ocEvents eventually or it will block if it drains them all and waits for more.
	// But since we sent "complete" (which maps to "done"), the bridge should exit upon seeing "complete".
	// So we technically don't need to close ocEvents if the bridge logic works.
	// But let's close it to be safe.
	close(ocEvents)
	// Do NOT close ocErrors, to prevent the "return on closed ocErrors" race path.
	// close(ocErrors)

	pulseEvents, pulseErrors := bridge.StreamEvents(ocEvents, ocErrors)

	// helpers to collect
	var events []ai.StreamEvent
	var errs []error

	done := make(chan bool)
	go func() {
		t.Log("Starting to collect events")
		for e := range pulseEvents {
			t.Logf("Got event: %s", e.Type)
			events = append(events, e)
		}
		t.Log("Finished collecting events, collecting errors")
		for e := range pulseErrors {
			t.Logf("Got error: %v", e)
			errs = append(errs, e)
		}
		t.Log("Finished collecting all")
		done <- true
	}()

	<-done

	require.Len(t, events, 2)
	assert.Equal(t, "content", events[0].Type)
	assert.Equal(t, "hello", events[0].Data)
	assert.Equal(t, "done", events[1].Type)
	assert.Empty(t, errs)
}

func TestMapToolName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"pulse_run_command", "run_command"},
		{"pulse_control_guest", "control_guest"},
		{"pulse_list_findings", "list_findings"},
		{"other_tool", "other_tool"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, mapToolName(tt.input))
	}
}
