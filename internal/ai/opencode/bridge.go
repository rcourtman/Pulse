package opencode

import (
	"encoding/json"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai"
)

// Bridge transforms OpenCode events to Pulse StreamEvent format
type Bridge struct{}

// NewBridge creates a new event bridge
func NewBridge() *Bridge {
	return &Bridge{}
}

// TransformEvent converts an OpenCode StreamEvent to a Pulse StreamEvent
func (b *Bridge) TransformEvent(ocEvent StreamEvent) (ai.StreamEvent, error) {
	switch ocEvent.Type {
	case "tool_use", "tool_call":
		return b.transformToolUse(ocEvent)
	case "tool_result", "tool_end":
		return b.transformToolResult(ocEvent)
	case "content", "text":
		return b.transformContent(ocEvent)
	case "thinking":
		return b.transformThinking(ocEvent)
	case "complete", "done":
		return b.transformComplete(ocEvent)
	case "error":
		return b.transformError(ocEvent)
	default:
		// Pass through unknown events as content
		return ai.StreamEvent{
			Type: "content",
			Data: string(ocEvent.Data),
		}, nil
	}
}

func (b *Bridge) transformToolUse(ocEvent StreamEvent) (ai.StreamEvent, error) {
	var toolUse ToolUseEvent
	if err := json.Unmarshal(ocEvent.Data, &toolUse); err != nil {
		return ai.StreamEvent{}, err
	}

	// Format input for display
	inputStr := formatToolInput(toolUse.Name, toolUse.Input)

	return ai.StreamEvent{
		Type: "tool_start",
		Data: ai.ToolStartData{
			Name:  mapToolName(toolUse.Name),
			Input: inputStr,
		},
	}, nil
}

func (b *Bridge) transformToolResult(ocEvent StreamEvent) (ai.StreamEvent, error) {
	var toolResult ToolResultEvent
	if err := json.Unmarshal(ocEvent.Data, &toolResult); err != nil {
		return ai.StreamEvent{}, err
	}

	// Check if this is an approval_needed response
	if isApprovalNeeded(toolResult.Output) {
		return b.handleApprovalNeeded(toolResult)
	}

	return ai.StreamEvent{
		Type: "tool_end",
		Data: ai.ToolEndData{
			Name:    mapToolName(toolResult.Name),
			Input:   "", // We don't have input in the result, frontend should track it
			Output:  toolResult.Output,
			Success: toolResult.Success,
		},
	}, nil
}

func (b *Bridge) transformContent(ocEvent StreamEvent) (ai.StreamEvent, error) {
	var content ContentEvent
	if err := json.Unmarshal(ocEvent.Data, &content); err != nil {
		// If it's not a structured event, treat it as raw content
		return ai.StreamEvent{
			Type: "content",
			Data: string(ocEvent.Data),
		}, nil
	}

	// Use delta if available, otherwise use full content
	text := content.Delta
	if text == "" {
		text = content.Content
	}

	return ai.StreamEvent{
		Type: "content",
		Data: text,
	}, nil
}

func (b *Bridge) transformThinking(ocEvent StreamEvent) (ai.StreamEvent, error) {
	var content ContentEvent
	if err := json.Unmarshal(ocEvent.Data, &content); err != nil {
		return ai.StreamEvent{
			Type: "thinking",
			Data: string(ocEvent.Data),
		}, nil
	}

	return ai.StreamEvent{
		Type: "thinking",
		Data: content.Content,
	}, nil
}

func (b *Bridge) transformComplete(ocEvent StreamEvent) (ai.StreamEvent, error) {
	var complete CompleteEvent
	if err := json.Unmarshal(ocEvent.Data, &complete); err != nil {
		// Return basic done event
		return ai.StreamEvent{
			Type: "done",
			Data: map[string]interface{}{},
		}, nil
	}

	return ai.StreamEvent{
		Type: "done",
		Data: map[string]interface{}{
			"session_id":    complete.SessionID,
			"input_tokens":  complete.Usage.InputTokens,
			"output_tokens": complete.Usage.OutputTokens,
			"model":         complete.Model.ID,
			"provider":      complete.Model.Provider,
		},
	}, nil
}

func (b *Bridge) transformError(ocEvent StreamEvent) (ai.StreamEvent, error) {
	var errorEvent ErrorEvent
	if err := json.Unmarshal(ocEvent.Data, &errorEvent); err != nil {
		return ai.StreamEvent{
			Type: "error",
			Data: string(ocEvent.Data),
		}, nil
	}

	msg := errorEvent.Message
	if msg == "" {
		msg = errorEvent.Error
	}

	return ai.StreamEvent{
		Type: "error",
		Data: msg,
	}, nil
}

func (b *Bridge) handleApprovalNeeded(toolResult ToolResultEvent) (ai.StreamEvent, error) {
	// Parse the approval needed data from tool output
	// Format: APPROVAL_REQUIRED: {"command": "...", "tool_id": "...", ...}
	approvalData := parseApprovalFromOutput(toolResult.Output)

	return ai.StreamEvent{
		Type: "approval_needed",
		Data: ai.ApprovalNeededData{
			Command:    approvalData.Command,
			ToolID:     toolResult.ID,
			ToolName:   mapToolName(toolResult.Name),
			RunOnHost:  approvalData.RunOnHost,
			TargetHost: approvalData.TargetHost,
			ApprovalID: approvalData.ApprovalID,
		},
	}, nil
}

// mapToolName maps OpenCode/MCP tool names to Pulse tool names
func mapToolName(name string) string {
	// Map pulse_ prefixed MCP tools back to display names
	switch name {
	// Control tools
	case "pulse_run_command":
		return "run_command"
	case "pulse_control_guest":
		return "control_guest"
	case "pulse_control_docker":
		return "control_docker"
	// Query tools
	case "pulse_get_capabilities":
		return "get_capabilities"
	case "pulse_get_url_content":
		return "get_url_content"
	case "pulse_list_infrastructure":
		return "list_infrastructure"
	case "pulse_set_resource_url":
		return "set_resource_url"
	case "pulse_get_resource":
		return "get_resource"
	// Patrol tools
	case "pulse_get_metrics":
		return "get_metrics"
	case "pulse_get_baselines":
		return "get_baselines"
	case "pulse_get_patterns":
		return "get_patterns"
	case "pulse_list_alerts":
		return "list_alerts"
	case "pulse_list_findings":
		return "list_findings"
	case "pulse_resolve_finding":
		return "resolve_finding"
	case "pulse_dismiss_finding":
		return "dismiss_finding"
	// Infrastructure tools
	case "pulse_list_backups":
		return "list_backups"
	case "pulse_list_storage":
		return "list_storage"
	case "pulse_get_disk_health":
		return "get_disk_health"
	// Profile tools
	case "pulse_get_agent_scope":
		return "get_agent_scope"
	case "pulse_set_agent_scope":
		return "set_agent_scope"
	default:
		return name
	}
}

// formatToolInput formats tool input for display
func formatToolInput(name string, input map[string]interface{}) string {
	switch name {
	case "pulse_run_command", "run_command":
		if cmd, ok := input["command"].(string); ok {
			return cmd
		}
	case "pulse_fetch_url", "fetch_url":
		if url, ok := input["url"].(string); ok {
			return url
		}
	}

	// Default: return JSON
	b, _ := json.Marshal(input)
	return string(b)
}

// isApprovalNeeded checks if a tool result indicates approval is required
func isApprovalNeeded(output string) bool {
	return len(output) > 18 && output[:18] == "APPROVAL_REQUIRED:"
}

// approvalInfo holds parsed approval data
type approvalInfo struct {
	Command    string
	RunOnHost  bool
	TargetHost string
	ApprovalID string
}

// parseApprovalFromOutput parses approval data from tool output
func parseApprovalFromOutput(output string) approvalInfo {
	if !isApprovalNeeded(output) {
		return approvalInfo{}
	}

	// Try to parse JSON after the prefix
	jsonStr := output[18:]
	var data struct {
		Command    string `json:"command"`
		RunOnHost  bool   `json:"run_on_host"`
		TargetHost string `json:"target_host"`
		ApprovalID string `json:"approval_id"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		// Fallback: treat the rest as the command
		return approvalInfo{Command: jsonStr}
	}

	return approvalInfo{
		Command:    data.Command,
		RunOnHost:  data.RunOnHost,
		TargetHost: data.TargetHost,
		ApprovalID: data.ApprovalID,
	}
}

// StreamEvents takes an OpenCode event channel and transforms to Pulse events
func (b *Bridge) StreamEvents(
	ocEvents <-chan StreamEvent,
	ocErrors <-chan error,
) (<-chan ai.StreamEvent, <-chan error) {
	pulseEvents := make(chan ai.StreamEvent, 100)
	pulseErrors := make(chan error, 1)

	go func() {
		defer close(pulseEvents)
		defer close(pulseErrors)

		for {
			select {
			case event, ok := <-ocEvents:
				if !ok {
					return
				}

				pulseEvent, err := b.TransformEvent(event)
				if err != nil {
					pulseErrors <- err
					return
				}

				pulseEvents <- pulseEvent

				// Check if this is the final event
				if pulseEvent.Type == "done" || pulseEvent.Type == "error" {
					return
				}

			case err, ok := <-ocErrors:
				if !ok {
					return
				}
				if err != nil {
					pulseErrors <- err
					return
				}
			}
		}
	}()

	return pulseEvents, pulseErrors
}
