package chat

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

func Test_w0716_agentic_providerRetryStatusMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "nil error uses generic interrupted message", err: nil, want: "Selected route stream interrupted before any output; retrying."},
		{name: "rate limit", err: strErr("HTTP 429: rate limit exceeded"), want: "Selected route is rate limiting the request; retrying."},
		{name: "too many requests", err: strErr("too many requests"), want: "Selected route is rate limiting the request; retrying."},
		{name: "429 token", err: strErr("429 Quota exceeded"), want: "Selected route is rate limiting the request; retrying."},
		{name: "timeout", err: strErr("context timeout"), want: "Selected route timed out before any output; retrying."},
		{name: "timed out", err: strErr("request timed out"), want: "Selected route timed out before any output; retrying."},
		{name: "deadline", err: strErr("context deadline exceeded"), want: "Selected route timed out before any output; retrying."},
		{name: "connection", err: strErr("connection reset by peer"), want: "Selected route connection failed before any output; retrying."},
		{name: "broken pipe", err: strErr("write: broken pipe"), want: "Selected route connection failed before any output; retrying."},
		{name: "eof", err: strErr("unexpected EOF"), want: "Selected route connection failed before any output; retrying."},
		{name: "default falls back to interrupted", err: strErr("internal server error"), want: "Selected route stream interrupted before any output; retrying."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := providerRetryStatusMessage(tt.err); got != tt.want {
				t.Fatalf("providerRetryStatusMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_w0716_agentic_removeToolCallFromResultMessages(t *testing.T) {
	tr := func(id, content string) *ToolResult { return &ToolResult{ToolUseID: id, Content: content} }

	tests := []struct {
		name      string
		messages  []Message
		toolUseID string
		wantLen   int
		check     func(t *testing.T, got []Message)
	}{
		{
			name:      "blank id returns messages unchanged",
			messages:  []Message{{Role: "assistant", Content: "hi"}},
			toolUseID: "   ",
			wantLen:   1,
		},
		{
			name: "no messages carry tool calls",
			messages: []Message{
				{Role: "user", Content: "hello"},
				{Role: "assistant", Content: "hi"},
			},
			toolUseID: "call-1",
			wantLen:   2,
		},
		{
			name: "tool calls present but no id match leaves them",
			messages: []Message{
				{Role: "assistant", Content: "text", ToolCalls: []ToolCall{{ID: "call-1", Name: "t"}}},
			},
			toolUseID: "call-2",
			wantLen:   1,
			check: func(t *testing.T, got []Message) {
				if len(got[0].ToolCalls) != 1 || got[0].ToolCalls[0].ID != "call-1" {
					t.Fatalf("unmatched tool call was removed: %+v", got[0].ToolCalls)
				}
			},
		},
		{
			name: "matching id removed but message keeps remaining content",
			messages: []Message{
				{Role: "assistant", Content: "keep me", ToolCalls: []ToolCall{{ID: "call-1", Name: "t"}, {ID: "call-2", Name: "u"}}},
			},
			toolUseID: "call-1",
			wantLen:   1,
			check: func(t *testing.T, got []Message) {
				if len(got[0].ToolCalls) != 1 || got[0].ToolCalls[0].ID != "call-2" {
					t.Fatalf("remaining tool call not preserved: %+v", got[0].ToolCalls)
				}
				if got[0].Content != "keep me" {
					t.Fatalf("content changed: %q", got[0].Content)
				}
			},
		},
		{
			name: "matching id empties message and removes it",
			messages: []Message{
				{Role: "user", Content: "hello"},
				{Role: "assistant", ToolCalls: []ToolCall{{ID: "call-1", Name: "t"}}},
				{Role: "assistant", Content: "tail"},
			},
			toolUseID: "call-1",
			wantLen:   2,
			check: func(t *testing.T, got []Message) {
				for _, m := range got {
					if len(m.ToolCalls) > 0 {
						t.Fatalf("message with removed tool call was not dropped: %+v", got)
					}
				}
				if got[0].Content != "hello" || got[1].Content != "tail" {
					t.Fatalf("remaining messages reordered: %+v", got)
				}
			},
		},
		{
			name: "matching id leaves message because tool result present",
			messages: []Message{
				{Role: "user", ToolResult: tr("call-1", "ok"), ToolCalls: []ToolCall{{ID: "call-1", Name: "t"}}},
			},
			toolUseID: "call-1",
			wantLen:   1,
			check: func(t *testing.T, got []Message) {
				if len(got[0].ToolCalls) != 0 {
					t.Fatalf("tool call not removed: %+v", got[0].ToolCalls)
				}
				if got[0].ToolResult == nil {
					t.Fatal("tool result should keep message alive")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeToolCallFromResultMessages(tt.messages, tt.toolUseID)
			if len(got) != tt.wantLen {
				t.Fatalf("len = %d, want %d (%+v)", len(got), tt.wantLen, got)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func Test_w0716_agentic_toolStartKey(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		nameArg string
		want    string
	}{
		{name: "non-empty id wins", id: "call-1", nameArg: "pulse_query", want: "call-1"},
		{name: "whitespace id falls back to name", id: "  ", nameArg: "pulse_query", want: "pulse_query"},
		{name: "empty id falls back to trimmed name", id: "", nameArg: "  pulse_read  ", want: "pulse_read"},
		{name: "both empty yields empty", id: "", nameArg: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolStartKey(tt.id, tt.nameArg); got != tt.want {
				t.Fatalf("toolStartKey(%q, %q) = %q, want %q", tt.id, tt.nameArg, got, tt.want)
			}
		})
	}
}

func Test_w0716_agentic_toolExecutionProgressMessage(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]interface{}
		toolKind ToolKind
		want     string
	}{
		{name: "run command", toolName: agentcapabilities.PulseRunCommandToolName, want: "Running command."},
		{name: "legacy run command", toolName: agentcapabilities.LegacyAssistantRunCommandToolName, want: "Running command."},
		{name: "query reads inventory", toolName: agentcapabilities.PulseQueryToolName, want: "Reading inventory."},
		{name: "read reads target", toolName: agentcapabilities.PulseReadToolName, want: "Reading target."},
		{name: "bare read alias reads target", toolName: "read", want: "Reading target."},
		{name: "governed write control", toolName: agentcapabilities.PulseControlToolName, toolKind: ToolKindWrite, want: "Executing governed action."},
		{name: "patrol report finding lifecycle", toolName: agentcapabilities.PatrolReportFindingToolName, toolKind: ToolKindWrite, want: "Executing governed action."},
		{name: "unknown tool generic running", toolName: "some_other_tool", want: "Running."},
		{name: "unknown write tool not in governed set", toolName: "mystery_writer", toolKind: ToolKindWrite, want: "Running."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolExecutionProgressMessage(tt.toolName, tt.input, tt.toolKind); got != tt.want {
				t.Fatalf("toolExecutionProgressMessage(%q) = %q, want %q", tt.toolName, got, tt.want)
			}
		})
	}
}

func Test_w0716_agentic_isKnownGovernedWriteProgress(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    map[string]interface{}
		toolKind ToolKind
		want     bool
	}{
		{name: "read kind never governed", toolName: agentcapabilities.PulseControlToolName, toolKind: ToolKindRead, want: false},
		{name: "control write", toolName: agentcapabilities.PulseControlToolName, toolKind: ToolKindWrite, want: true},
		{name: "control guest write", toolName: agentcapabilities.PulseControlGuestToolName, toolKind: ToolKindWrite, want: true},
		{name: "control docker write", toolName: agentcapabilities.PulseControlDockerToolName, toolKind: ToolKindWrite, want: true},
		{name: "alerts resolve", toolName: agentcapabilities.PulseAlertsToolName, input: map[string]interface{}{"action": "resolve"}, toolKind: ToolKindWrite, want: true},
		{name: "alerts dismiss", toolName: agentcapabilities.PulseAlertsToolName, input: map[string]interface{}{"action": "dismiss"}, toolKind: ToolKindWrite, want: true},
		{name: "alerts list is read-only", toolName: agentcapabilities.PulseAlertsToolName, input: map[string]interface{}{"action": "list"}, toolKind: ToolKindWrite, want: false},
		{name: "alerts uppercase action matches", toolName: agentcapabilities.PulseAlertsToolName, input: map[string]interface{}{"action": "  RESOLVE "}, toolKind: ToolKindWrite, want: true},
		{name: "alerts no action", toolName: agentcapabilities.PulseAlertsToolName, input: nil, toolKind: ToolKindWrite, want: false},
		{name: "docker control", toolName: agentcapabilities.PulseDockerToolName, input: map[string]interface{}{"action": "control"}, toolKind: ToolKindWrite, want: true},
		{name: "docker update", toolName: agentcapabilities.PulseDockerToolName, input: map[string]interface{}{"action": "update"}, toolKind: ToolKindWrite, want: true},
		{name: "docker check_updates", toolName: agentcapabilities.PulseDockerToolName, input: map[string]interface{}{"action": "check_updates"}, toolKind: ToolKindWrite, want: true},
		{name: "docker trigger_update", toolName: agentcapabilities.PulseDockerToolName, input: map[string]interface{}{"action": "trigger_update"}, toolKind: ToolKindWrite, want: true},
		{name: "docker services is read-only", toolName: agentcapabilities.PulseDockerToolName, input: map[string]interface{}{"action": "services"}, toolKind: ToolKindWrite, want: false},
		{name: "kubernetes scale", toolName: agentcapabilities.PulseKubernetesToolName, input: map[string]interface{}{"action": "scale"}, toolKind: ToolKindWrite, want: true},
		{name: "kubernetes restart", toolName: agentcapabilities.PulseKubernetesToolName, input: map[string]interface{}{"action": "restart"}, toolKind: ToolKindWrite, want: true},
		{name: "kubernetes delete_pod", toolName: agentcapabilities.PulseKubernetesToolName, input: map[string]interface{}{"action": "delete_pod"}, toolKind: ToolKindWrite, want: true},
		{name: "kubernetes exec", toolName: agentcapabilities.PulseKubernetesToolName, input: map[string]interface{}{"action": "exec"}, toolKind: ToolKindWrite, want: true},
		{name: "kubernetes pods is read-only", toolName: agentcapabilities.PulseKubernetesToolName, input: map[string]interface{}{"action": "pods"}, toolKind: ToolKindWrite, want: false},
		{name: "file edit write", toolName: agentcapabilities.PulseFileEditToolName, input: map[string]interface{}{"action": "write"}, toolKind: ToolKindWrite, want: true},
		{name: "file edit append", toolName: agentcapabilities.PulseFileEditToolName, input: map[string]interface{}{"action": "append"}, toolKind: ToolKindWrite, want: true},
		{name: "file edit read is not governed", toolName: agentcapabilities.PulseFileEditToolName, input: map[string]interface{}{"action": "read"}, toolKind: ToolKindWrite, want: false},
		{name: "knowledge remember", toolName: agentcapabilities.PulseKnowledgeToolName, input: map[string]interface{}{"action": "remember"}, toolKind: ToolKindWrite, want: true},
		{name: "knowledge note", toolName: agentcapabilities.PulseKnowledgeToolName, input: map[string]interface{}{"action": "note"}, toolKind: ToolKindWrite, want: true},
		{name: "knowledge save", toolName: agentcapabilities.PulseKnowledgeToolName, input: map[string]interface{}{"action": "save"}, toolKind: ToolKindWrite, want: true},
		{name: "knowledge recall is read-only", toolName: agentcapabilities.PulseKnowledgeToolName, input: map[string]interface{}{"action": "recall"}, toolKind: ToolKindWrite, want: false},
		{name: "patrol report finding", toolName: agentcapabilities.PatrolReportFindingToolName, toolKind: ToolKindWrite, want: true},
		{name: "patrol assess finding", toolName: agentcapabilities.PatrolAssessFindingToolName, toolKind: ToolKindWrite, want: true},
		{name: "patrol resolve finding", toolName: agentcapabilities.PatrolResolveFindingToolName, toolKind: ToolKindWrite, want: true},
		{name: "unknown write tool default false", toolName: "mystery_tool", toolKind: ToolKindWrite, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownGovernedWriteProgress(tt.toolName, tt.input, tt.toolKind); got != tt.want {
				t.Fatalf("isKnownGovernedWriteProgress(%q, %+v, %s) = %v, want %v", tt.toolName, tt.input, tt.toolKind, got, tt.want)
			}
		})
	}
}

func Test_w0716_agentic_emitToolStartEvent(t *testing.T) {
	t.Run("nil callback is a no-op", func(t *testing.T) {
		emitToolStartEvent(nil, "id-1", agentcapabilities.PulseQueryToolName, map[string]interface{}{"query": "x"})
	})

	t.Run("emits tool_start with projected input", func(t *testing.T) {
		var got []StreamEvent
		emitToolStartEvent(func(e StreamEvent) { got = append(got, e) }, "id-1", agentcapabilities.PulseControlToolName, map[string]interface{}{"command": "uptime"})
		if len(got) != 1 || got[0].Type != "tool_start" {
			t.Fatalf("expected one tool_start event, got %+v", got)
		}
		var data ToolStartData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.ID != "id-1" || data.Name != agentcapabilities.PulseControlToolName {
			t.Fatalf("unexpected id/name: %+v", data)
		}
		if data.Input != "Running: uptime" {
			t.Fatalf("input not projected through frontend formatter: %q", data.Input)
		}
		if data.Phase != "running" {
			t.Fatalf("phase = %q, want running", data.Phase)
		}
		if !strings.Contains(data.RawInput, "uptime") {
			t.Fatalf("raw input missing command: %q", data.RawInput)
		}
	})

	t.Run("nil input yields empty object input string", func(t *testing.T) {
		var got []StreamEvent
		emitToolStartEvent(func(e StreamEvent) { got = append(got, e) }, "id-2", "custom_tool", nil)
		var data ToolStartData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.Input != "{}" {
			t.Fatalf("nil input should project to {}, got %q", data.Input)
		}
	})
}

func Test_w0716_agentic_emitToolCancelEvent(t *testing.T) {
	t.Run("nil callback is a no-op", func(t *testing.T) {
		emitToolCancelEvent(nil, "id-1", "pulse_query", "  blocked  ")
	})

	t.Run("emits tool_cancel with trimmed reason", func(t *testing.T) {
		var got []StreamEvent
		emitToolCancelEvent(func(e StreamEvent) { got = append(got, e) }, "id-1", "pulse_query", "  current_resource unavailable  ")
		if len(got) != 1 || got[0].Type != "tool_cancel" {
			t.Fatalf("expected one tool_cancel event, got %+v", got)
		}
		var data ToolCancelData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.ID != "id-1" || data.Name != "pulse_query" {
			t.Fatalf("unexpected id/name: %+v", data)
		}
		if data.Reason != "current_resource unavailable" {
			t.Fatalf("reason not trimmed: %q", data.Reason)
		}
	})
}

func Test_w0716_agentic_emitToolEndEvent(t *testing.T) {
	t.Run("nil callback is a no-op", func(t *testing.T) {
		emitToolEndEvent(nil, "id-1", "pulse_query", map[string]interface{}{"query": "x"}, "done", true)
	})

	t.Run("emits tool_end with projected input and output", func(t *testing.T) {
		var got []StreamEvent
		emitToolEndEvent(func(e StreamEvent) { got = append(got, e) }, "id-1", agentcapabilities.PulseControlToolName, map[string]interface{}{"command": "uptime"}, "ok output", true)
		if len(got) != 1 || got[0].Type != "tool_end" {
			t.Fatalf("expected one tool_end event, got %+v", got)
		}
		var data ToolEndData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.ID != "id-1" || data.Name != agentcapabilities.PulseControlToolName {
			t.Fatalf("unexpected id/name: %+v", data)
		}
		if data.Input != "Running: uptime" {
			t.Fatalf("input not projected: %q", data.Input)
		}
		if data.Output != "ok output" || !data.Success {
			t.Fatalf("output/success mismatch: %+v", data)
		}
	})

	t.Run("nil input yields empty input string for end event", func(t *testing.T) {
		var got []StreamEvent
		emitToolEndEvent(func(e StreamEvent) { got = append(got, e) }, "id-2", "custom_tool", nil, "out", false)
		var data ToolEndData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.Input != "" {
			t.Fatalf("nil input should project to empty for end event, got %q", data.Input)
		}
	})
}

func Test_w0716_agentic_emitToolProgressEventWithRawInput(t *testing.T) {
	t.Run("nil callback is a no-op", func(t *testing.T) {
		emitToolProgressEventWithRawInput(nil, "id-1", "pulse_query", map[string]interface{}{"query": "x"}, "OVERRIDE", "running", "msg")
	})

	t.Run("no override uses projected raw input", func(t *testing.T) {
		var got []StreamEvent
		emitToolProgressEventWithRawInput(func(e StreamEvent) { got = append(got, e) }, "id-1", "pulse_query", map[string]interface{}{"query": "containers"}, "", "pending", "Reading inventory.")
		if len(got) != 1 || got[0].Type != "tool_progress" {
			t.Fatalf("expected one tool_progress event, got %+v", got)
		}
		var data ToolProgressData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.ID != "id-1" || data.Name != "pulse_query" {
			t.Fatalf("unexpected id/name: %+v", data)
		}
		if data.Phase != "pending" || data.Message != "Reading inventory." {
			t.Fatalf("phase/message mismatch: %+v", data)
		}
		if !strings.Contains(data.RawInput, "containers") {
			t.Fatalf("raw input should carry projected value: %q", data.RawInput)
		}
	})

	t.Run("override applied for non-restricted tool", func(t *testing.T) {
		var got []StreamEvent
		emitToolProgressEventWithRawInput(func(e StreamEvent) { got = append(got, e) }, "id-1", "pulse_query", map[string]interface{}{"query": "x"}, "PROVIDER_RAW_OVERRIDE", "running", "msg")
		var data ToolProgressData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.RawInput != "PROVIDER_RAW_OVERRIDE" {
			t.Fatalf("non-restricted tool should use override, got %q", data.RawInput)
		}
	})

	t.Run("override suppressed for exposure-restricted patrol_propose_action", func(t *testing.T) {
		var got []StreamEvent
		emitToolProgressEventWithRawInput(
			func(e StreamEvent) { got = append(got, e) },
			"id-1",
			agentcapabilities.PatrolProposeActionToolName,
			map[string]interface{}{"params": "secret-value", "finding_id": "f1"},
			"LEAK_ATTEMPT_OVERRIDE",
			"running",
			"msg",
		)
		var data ToolProgressData
		if err := json.Unmarshal(got[0].Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if data.RawInput == "LEAK_ATTEMPT_OVERRIDE" {
			t.Fatal("restricted-exposure tool must not surface provider raw override")
		}
		if strings.Contains(data.RawInput, "secret-value") {
			t.Fatalf("restricted params leaked into raw input: %q", data.RawInput)
		}
		if !strings.Contains(data.RawInput, "redacted-proposal-params") {
			t.Fatalf("expected redacted marker in raw input: %q", data.RawInput)
		}
	})
}

func Test_w0716_agentic_normalizeToolUseID(t *testing.T) {
	tests := []struct {
		name      string
		toolUseID string
		want      string
	}{
		{name: "empty", toolUseID: "", want: ""},
		{name: "whitespace only", toolUseID: "   ", want: ""},
		{name: "no underscore returned as-is", toolUseID: "abc", want: "abc"},
		{name: "leading underscore returned as-is", toolUseID: "_abc", want: "_abc"},
		{name: "trailing underscore returned as-is", toolUseID: "abc_", want: "abc_"},
		{name: "numeric suffix stripped", toolUseID: "abc_123", want: "abc"},
		{name: "single digit suffix stripped", toolUseID: "x_0", want: "x"},
		{name: "non-numeric suffix kept", toolUseID: "abc_12a", want: "abc_12a"},
		{name: "multi underscore strips only trailing numeric", toolUseID: "a_b_3", want: "a_b"},
		{name: "real pulse call id stripped", toolUseID: "pulse_query_0", want: "pulse_query"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeToolUseID(tt.toolUseID); got != tt.want {
				t.Fatalf("normalizeToolUseID(%q) = %q, want %q", tt.toolUseID, got, tt.want)
			}
		})
	}
}

func Test_w0716_agentic_fallbackToolDisplayName(t *testing.T) {
	tests := []struct {
		name      string
		toolUseID string
		idToName  map[string]string
		want      string
	}{
		{name: "resolved via id map strips pulse prefix", toolUseID: "call_27f0", idToName: map[string]string{"call_27f0": "pulse_query"}, want: "query"},
		{name: "resolved via id map keeps non-pulse name", toolUseID: "x", idToName: map[string]string{"x": "custom_tool"}, want: "custom_tool"},
		{name: "opaque call id suppressed", toolUseID: "call_27f0f389aba4652a1e292dc", idToName: map[string]string{}, want: ""},
		{name: "opaque toolu id suppressed", toolUseID: "toolu_abcDEF123", idToName: map[string]string{}, want: ""},
		{name: "opaque fc id suppressed", toolUseID: "fc_xyz789", idToName: map[string]string{}, want: ""},
		{name: "empty tool use id suppressed", toolUseID: "", idToName: map[string]string{}, want: ""},
		{name: "normalized numeric call id resolves to bare name", toolUseID: "pulse_query_0", idToName: map[string]string{}, want: "query"},
		{name: "whitespace id suppressed", toolUseID: "   ", idToName: map[string]string{}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fallbackToolDisplayName(tt.toolUseID, tt.idToName); got != tt.want {
				t.Fatalf("fallbackToolDisplayName(%q) = %q, want %q", tt.toolUseID, got, tt.want)
			}
		})
	}
}

func Test_w0716_agentic_ResetTokenCounts(t *testing.T) {
	loop := &AgenticLoop{
		totalInputTokens:  1500,
		totalOutputTokens: 900,
		totalToolCalls:    7,
	}
	loop.ResetTokenCounts()
	if loop.totalInputTokens != 0 || loop.totalOutputTokens != 0 || loop.totalToolCalls != 0 {
		t.Fatalf("ResetTokenCounts left non-zero counters: in=%d out=%d calls=%d",
			loop.totalInputTokens, loop.totalOutputTokens, loop.totalToolCalls)
	}
	if loop.GetTotalInputTokens() != 0 || loop.GetTotalOutputTokens() != 0 || loop.GetTotalToolCalls() != 0 {
		t.Fatal("public token getters must report zero after reset")
	}
}

func Test_w0716_agentic_SetExecutionProfile(t *testing.T) {
	t.Run("patrol detection sets stream idle timeout", func(t *testing.T) {
		loop := &AgenticLoop{}
		loop.SetExecutionProfile(tools.ProfilePatrolDetection)
		if loop.streamIdleTimeout != patrolProviderStreamIdleTimeout {
			t.Fatalf("detection stream idle timeout = %s, want %s", loop.streamIdleTimeout, patrolProviderStreamIdleTimeout)
		}
		if loop.executionProfile != tools.ProfilePatrolDetection {
			t.Fatalf("executionProfile = %d, want %d", loop.executionProfile, tools.ProfilePatrolDetection)
		}
	})

	t.Run("patrol investigation sets stream idle timeout", func(t *testing.T) {
		loop := &AgenticLoop{}
		loop.SetExecutionProfile(tools.ProfilePatrolInvestigation)
		if loop.streamIdleTimeout != patrolProviderStreamIdleTimeout {
			t.Fatalf("investigation stream idle timeout = %s, want %s", loop.streamIdleTimeout, patrolProviderStreamIdleTimeout)
		}
	})

	t.Run("interactive assistant clears stream idle timeout", func(t *testing.T) {
		loop := &AgenticLoop{streamIdleTimeout: patrolProviderStreamIdleTimeout}
		loop.SetExecutionProfile(tools.ProfileInteractiveAssistant)
		if loop.streamIdleTimeout != 0 {
			t.Fatalf("interactive stream idle timeout = %s, want 0", loop.streamIdleTimeout)
		}
	})

	t.Run("invalid profile panics", func(t *testing.T) {
		loop := &AgenticLoop{}
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected SetExecutionProfile to panic on an invalid profile")
			}
		}()
		loop.SetExecutionProfile(tools.ExecutionProfile(99))
	})
}

type strErr string

func (e strErr) Error() string { return string(e) }
