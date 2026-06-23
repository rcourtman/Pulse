package agentcapabilities

import (
	"fmt"
	"strings"
)

const ApprovalArgumentKey = "_approval_id"

const CurrentResourceHandle = "current_resource"

var currentResourceReferenceAliases = map[string]struct{}{
	CurrentResourceHandle: {},
	"attached_resource":   {},
	"selected_resource":   {},
	"this_resource":       {},
	"redacted by policy":  {},
}

// IsTextToolInvocation reports whether command looks like Pulse's legacy text
// projection for a tool-call request.
func IsTextToolInvocation(command string) bool {
	return strings.HasPrefix(command, "pulse_") ||
		strings.HasPrefix(command, "default_api:") ||
		strings.Contains(command, PulseControlGuestToolName) ||
		strings.Contains(command, PulseRunCommandToolName) ||
		strings.Contains(command, PulseGetResourceToolName)
}

// ParseTextToolInvocation converts Pulse's legacy text projection for a
// tool-call request into the shared tool-call params shape.
func ParseTextToolInvocation(command string) (ToolCallParams, error) {
	command = strings.TrimPrefix(command, "default_api:")
	openParen := strings.Index(command, "(")
	if openParen == -1 {
		return ToolCallParams{}, fmt.Errorf("no opening parenthesis in tool call")
	}
	toolName := strings.TrimSpace(command[:openParen])
	closeParen := strings.LastIndex(command, ")")
	if closeParen == -1 || closeParen <= openParen {
		return ToolCallParams{}, fmt.Errorf("no closing parenthesis in tool call")
	}
	argsStr := command[openParen+1 : closeParen]
	args := make(map[string]any)
	if strings.TrimSpace(argsStr) != "" {
		for _, pair := range splitTextToolArguments(argsStr) {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.Trim(value, "'\"")
			args[key] = value
		}
	}
	params := NormalizeToolCallParams(ToolCallParams{Name: toolName, Arguments: args})
	if err := ValidateToolCallParams(params); err != nil {
		return ToolCallParams{}, err
	}
	return params, nil
}

// IsCurrentResourceReference reports whether a tool argument is one of the
// session-scoped placeholders that must resolve through attached resource
// context before execution.
func IsCurrentResourceReference(value string) bool {
	_, ok := currentResourceReferenceAliases[strings.ToLower(strings.TrimSpace(value))]
	return ok
}

// ToolInputContainsCurrentResourceReference recursively scans JSON-like tool
// input values for the shared current_resource handle and its governed legacy
// aliases. Provider streaming, native Assistant execution, and external-agent
// adapters all branch on this shared vocabulary.
func ToolInputContainsCurrentResourceReference(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return IsCurrentResourceReference(v)
	case map[string]any:
		for _, child := range v {
			if ToolInputContainsCurrentResourceReference(child) {
				return true
			}
		}
	case []any:
		for _, child := range v {
			if ToolInputContainsCurrentResourceReference(child) {
				return true
			}
		}
	case []string:
		for _, child := range v {
			if IsCurrentResourceReference(child) {
				return true
			}
		}
	}
	return false
}

// ApprovalArgument returns the trimmed approval id carried on an approved
// action replay. The key is intentionally internal to the shared Pulse
// Intelligence tool-call contract rather than a user-facing argument.
func ApprovalArgument(args map[string]any) string {
	approvalID, _ := args[ApprovalArgumentKey].(string)
	return strings.TrimSpace(approvalID)
}

// IsInternalToolArgument reports whether an argument is Pulse runtime metadata
// that must not be projected into public manifest-backed HTTP request bodies.
func IsInternalToolArgument(name string) bool {
	return name == ApprovalArgumentKey
}

// CloneToolArguments returns an independent copy of tool-call arguments.
func CloneToolArguments(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	cloned := make(map[string]any, len(args))
	for k, v := range args {
		cloned[k] = cloneSchemaValue(v)
	}
	return cloned
}

// PublicToolArguments returns a copy of args without Pulse runtime metadata.
// Nested JSON-like values are detached so public HTTP projection and native
// handler execution cannot mutate caller-owned tool-call state.
func PublicToolArguments(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	public := make(map[string]any, len(args))
	for k, v := range args {
		if IsInternalToolArgument(k) {
			continue
		}
		public[k] = cloneSchemaValue(v)
	}
	return public
}

// WithApprovalArgument returns args with approvalID attached when non-empty.
// Callers pass the returned map because a non-empty approval id initializes a
// nil input map and the returned map is independent from the input.
func WithApprovalArgument(args map[string]any, approvalID string) map[string]any {
	args = CloneToolArguments(args)
	approvalID = strings.TrimSpace(approvalID)
	if approvalID == "" {
		return args
	}
	if args == nil {
		args = map[string]any{}
	}
	args[ApprovalArgumentKey] = approvalID
	return args
}

func splitTextToolArguments(argsStr string) []string {
	var result []string
	var current strings.Builder
	var inQuote rune
	var escaped bool
	for _, r := range argsStr {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			current.WriteRune(r)
			continue
		}
		if inQuote != 0 {
			current.WriteRune(r)
			if r == inQuote {
				inQuote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			inQuote = r
			current.WriteRune(r)
			continue
		}
		if r == ',' {
			if s := strings.TrimSpace(current.String()); s != "" {
				result = append(result, s)
			}
			current.Reset()
			continue
		}
		current.WriteRune(r)
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		result = append(result, s)
	}
	return result
}
