package agentcapabilities

import (
	"errors"
	"fmt"
	"strings"
)

// ToolCallParams is the shared request/response tool-call parameter shape used
// by native Pulse Assistant execution and external-agent adapters.
type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

// ToolCallKind classifies a Pulse Intelligence tool call for safety gates and
// workflow transitions. Native Assistant currently uses it for FSM decisions,
// while the vocabulary lives in the shared core so external-agent adapters and
// future surfaces do not grow separate read/write heuristics.
type ToolCallKind int

const (
	// ToolCallKindResolve covers discovery/query tools that establish resource
	// context and also count as safe verification reads.
	ToolCallKindResolve ToolCallKind = iota

	// ToolCallKindRead covers read-only tools such as logs, metrics, status,
	// and config inspection.
	ToolCallKindRead

	// ToolCallKindWrite covers tools that can change Pulse state or target-side
	// infrastructure state.
	ToolCallKindWrite

	// ToolCallKindUserInput covers interactive tools that ask the user for
	// missing information and do not advance infrastructure workflow state.
	ToolCallKindUserInput
)

func (k ToolCallKind) String() string {
	switch k {
	case ToolCallKindResolve:
		return "resolve"
	case ToolCallKindRead:
		return "read"
	case ToolCallKindWrite:
		return "write"
	case ToolCallKindUserInput:
		return "user_input"
	default:
		return "unknown"
	}
}

// NormalizeToolCallParams returns the canonical tool-call parameter shape
// shared by native Assistant execution and external-agent adapters.
func NormalizeToolCallParams(params ToolCallParams) ToolCallParams {
	params.Name = strings.TrimSpace(params.Name)
	params.Arguments = CloneToolArguments(params.Arguments)
	if params.Arguments == nil {
		params.Arguments = map[string]any{}
	}
	return params
}

// ValidateToolCallParams checks the normalized tool-call contract before a
// surface attempts capability lookup or execution.
func ValidateToolCallParams(params ToolCallParams) error {
	if strings.TrimSpace(params.Name) == "" {
		return fmt.Errorf("tool name is required")
	}
	return nil
}

// PrepareToolRegistryExecution normalizes and validates the direct in-process
// registry entrypoint used by native Assistant execution. It returns a
// ready-to-return shared error result when the tool-call params contract is
// invalid, so registry surfaces do not duplicate failure text or envelope
// construction.
func PrepareToolRegistryExecution(name string, args map[string]any) (ToolCallParams, ToolResult, bool) {
	params := NormalizeToolCallParams(ToolCallParams{
		Name:      name,
		Arguments: args,
	})
	if err := ValidateToolCallParams(params); err != nil {
		return params, NewInvalidToolCallParamsResult(err), false
	}
	return params, ToolResult{}, true
}

// ClassifyToolCall classifies a provider/registry tool call for safety gates
// and workflow state transitions. Unknown tools default to write so newly
// introduced tools cannot bypass governed-action checks accidentally.
func ClassifyToolCall(toolName string, args map[string]interface{}) ToolCallKind {
	action, _ := args["action"].(string)
	actionLower := strings.ToLower(action)
	operation, _ := args["operation"].(string)
	operationLower := strings.ToLower(operation)

	switch strings.TrimSpace(toolName) {
	case PulseQuestionToolName:
		return ToolCallKindUserInput

	case PulseQueryToolName, PulseDiscoveryToolName:
		return ToolCallKindResolve

	case PulseMetricsToolName, PulseStorageToolName, PulsePMGToolName, PulseSummarizeToolName:
		return ToolCallKindRead

	case PulseAlertsToolName:
		switch actionLower {
		case "resolve", "dismiss":
			return ToolCallKindWrite
		default:
			return ToolCallKindRead
		}

	case PulseKubernetesToolName:
		switch actionLower {
		case "scale", "restart", "delete_pod", "exec":
			return ToolCallKindWrite
		default:
			return ToolCallKindRead
		}

	case PulseKnowledgeToolName:
		switch actionLower {
		case "remember", "note", "save":
			return ToolCallKindWrite
		default:
			return ToolCallKindRead
		}

	case PulseReadToolName, LegacyAssistantFetchURLToolName:
		return ToolCallKindRead

	case PulseControlToolName:
		return ToolCallKindWrite

	case PulseDockerToolName:
		switch actionLower {
		case "control", "update", "check_updates", "trigger_update":
			return ToolCallKindWrite
		default:
			return ToolCallKindRead
		}

	case PulseFileEditToolName:
		switch actionLower {
		case "read":
			return ToolCallKindRead
		case "write", "append":
			return ToolCallKindWrite
		default:
			return ToolCallKindRead
		}

	case PulseRunCommandToolName, PulseControlGuestToolName, PulseControlDockerToolName,
		LegacyAssistantRunCommandToolName, LegacyAssistantSetResourceURLToolName:
		return ToolCallKindWrite

	case PulseSearchResourcesToolName, PulseGetResourceToolName, PulseGetTopologyToolName,
		PulseListInfrastructureToolName, PulseGetConnectionHealthToolName:
		return ToolCallKindResolve

	case PulseGetDockerLogsToolName, PulseGetPerformanceMetricsToolName,
		PulseGetTemperaturesToolName, PulseGetBaselinesToolName, PulseGetPatternsToolName:
		return ToolCallKindRead

	case PatrolGetFindingsToolName:
		return ToolCallKindRead
	case PatrolReportFindingToolName, PatrolResolveFindingToolName:
		return ToolCallKindWrite
	}

	if toolCallActionIsWrite(actionLower) || toolCallActionIsWrite(operationLower) {
		return ToolCallKindWrite
	}

	if toolCallActionIsRead(actionLower) || toolCallActionIsRead(operationLower) {
		return ToolCallKindRead
	}

	return ToolCallKindWrite
}

func toolCallActionIsWrite(action string) bool {
	switch action {
	case "start", "stop", "restart", "delete",
		"shutdown", "reboot", "write", "append",
		"update", "trigger", "resolve", "dismiss",
		"control":
		return true
	default:
		return false
	}
}

func toolCallActionIsRead(action string) bool {
	switch action {
	case "get", "list", "search", "query",
		"read", "logs", "status", "health",
		"describe", "inspect", "show":
		return true
	default:
		return false
	}
}

// NewInvalidToolCallParamsResult returns the stable shared error result for a
// registry tool-call request that fails the shared params contract.
func NewInvalidToolCallParamsResult(err error) ToolResult {
	if err == nil {
		err = errors.New("tool call params are invalid")
	}
	return NewToolErrorResult(fmt.Errorf("invalid tools/call params: %w", err))
}

// NewUnknownToolResult returns the stable shared error result for a registry
// tool-call request that names no registered tool.
func NewUnknownToolResult(name string) ToolResult {
	return NewToolErrorResult(fmt.Errorf("unknown tool: %s", strings.TrimSpace(name)))
}

// NewControlToolsDisabledToolResult returns the stable shared text result used
// when a direct registry execution attempts a control tool at read-only control
// level.
func NewControlToolsDisabledToolResult() ToolResult {
	return NewToolTextResult(ControlToolsDisabledMessage)
}
