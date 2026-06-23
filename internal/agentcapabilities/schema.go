package agentcapabilities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Tool describes a structured tool definition with a JSON object input schema.
// The Assistant registry owns concrete tool handlers, while this shared shape
// keeps registry-to-provider projection aligned with external agent adapters.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

// NormalizeCollections keeps registry tool definitions stable when they cross
// Assistant, provider, and external-agent boundaries.
func (t Tool) NormalizeCollections() Tool {
	t.InputSchema = t.InputSchema.NormalizeCollections()
	return t
}

// ProviderTool is the neutral chat-provider tool declaration projected from the
// Assistant registry. Provider clients map this into their supported upstream
// function schema fields, while Pulse-owned metadata stays available to local
// runtime and contract checks without being blindly forwarded.
type ProviderTool struct {
	Type            string                    `json:"type,omitempty"`
	Name            string                    `json:"name"`
	Description     string                    `json:"description,omitempty"`
	InputSchema     map[string]interface{}    `json:"input_schema"`
	MaxUses         int                       `json:"max_uses,omitempty"`
	BehaviorHints   *ToolBehaviorHints        `json:"behavior_hints,omitempty"`
	PulseGovernance *ToolGovernanceDescriptor `json:"pulse_governance,omitempty"`
}

// AssistantProviderToolOptions controls native Assistant surface-tool
// projection. Registry tools are always projected from the shared governed
// manifest; native interactive tools are opt-in because Patrol/autonomous runs
// cannot wait on in-app user input.
type AssistantProviderToolOptions struct {
	IncludeQuestionTool bool
}

// EmptyProviderTool returns a provider tool with initialized collection fields.
func EmptyProviderTool() ProviderTool {
	return ProviderTool{}.NormalizeCollections()
}

// NormalizeCollections keeps provider tool JSON stable by preserving an empty
// input_schema object instead of marshaling it as null.
func (t ProviderTool) NormalizeCollections() ProviderTool {
	t.InputSchema = CloneProviderInputSchema(t.InputSchema)
	if t.InputSchema == nil {
		t.InputSchema = map[string]interface{}{}
	}
	t.BehaviorHints = CloneToolBehaviorHints(t.BehaviorHints)
	if t.PulseGovernance != nil {
		governance := *t.PulseGovernance
		t.PulseGovernance = &governance
	}
	return t
}

// ProviderToolCall is the provider-facing tool invocation shape returned by
// chat model clients. Keeping it here lets Assistant and external-agent
// adapters share the provider-call to tools/call bridge.
type ProviderToolCall struct {
	ID               string                 `json:"id"`
	Name             string                 `json:"name"`
	Input            map[string]interface{} `json:"input"`
	ThoughtSignature json.RawMessage        `json:"thought_signature,omitempty"`
}

// EmptyProviderToolCall returns a provider tool call with initialized
// collection fields.
func EmptyProviderToolCall() ProviderToolCall {
	return ProviderToolCall{}.NormalizeCollections()
}

// NormalizeCollections keeps provider tool-call JSON stable by preserving an
// empty input object instead of marshaling it as null.
func (t ProviderToolCall) NormalizeCollections() ProviderToolCall {
	t.Input = CloneToolArguments(t.Input)
	t.ThoughtSignature = CloneRawMessage(t.ThoughtSignature)
	if t.Input == nil {
		t.Input = map[string]interface{}{}
	}
	return t
}

// ProviderToolResult is the provider-facing response shape sent back to model
// clients after a tool invocation is executed.
type ProviderToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

// ProviderToolResultContextOptions controls how a provider tool result is
// projected into the model-facing context while preserving the full transcript
// result. A non-positive MaxModelContentChars disables truncation.
type ProviderToolResultContextOptions struct {
	MaxModelContentChars int
}

// ProviderToolResultTruncation describes the model-context truncation applied
// while projecting a provider tool result.
type ProviderToolResultTruncation struct {
	Applied        bool
	OriginalChars  int
	MaxChars       int
	TruncatedChars int
}

// ProviderToolResultContextProjection carries the full transcript result and
// the result sent back to the model for the next provider turn.
type ProviderToolResultContextProjection struct {
	Transcript ProviderToolResult
	Model      ProviderToolResult
	Truncation ProviderToolResultTruncation
}

// NewProviderToolResult builds the provider-facing result shape in one place so
// Assistant turns and external-agent adapters do not duplicate tool-result JSON.
func NewProviderToolResult(toolUseID, content string, isError bool) ProviderToolResult {
	return ProviderToolResult{
		ToolUseID: toolUseID,
		Content:   content,
		IsError:   isError,
	}
}

// NewProviderToolErrorResult builds a provider-facing synthetic error result.
func NewProviderToolErrorResult(toolUseID, content string) ProviderToolResult {
	return NewProviderToolResult(toolUseID, content, true)
}

// NewProviderToolResultFromToolResult projects a shared tool result into the
// provider context shape sent back to chat models.
func NewProviderToolResultFromToolResult(toolUseID string, result ToolResult) ProviderToolResult {
	interpreted := InterpretToolResult(result)
	return NewProviderToolResult(toolUseID, interpreted.Text, interpreted.IsError)
}

// NewProviderToolResultContextProjection builds the paired transcript/model
// provider results in one shared place. The transcript result retains full
// content; the model result applies the supplied model-context limit.
func NewProviderToolResultContextProjection(toolUseID, content string, isError bool, opts ProviderToolResultContextOptions) ProviderToolResultContextProjection {
	modelContent, truncation := ProviderToolResultModelContent(content, opts)
	return ProviderToolResultContextProjection{
		Transcript: NewProviderToolResult(toolUseID, content, isError),
		Model:      NewProviderToolResult(toolUseID, modelContent, isError),
		Truncation: truncation,
	}
}

// NewProviderToolResultContextProjectionFromToolResult projects a shared tool
// result into the paired transcript/model provider results used by native
// Assistant execution and external adapters with provider context.
func NewProviderToolResultContextProjectionFromToolResult(toolUseID string, result ToolResult, opts ProviderToolResultContextOptions) ProviderToolResultContextProjection {
	interpreted := InterpretToolResult(result)
	return NewProviderToolResultContextProjection(toolUseID, interpreted.Text, interpreted.IsError, opts)
}

// ProviderToolResultModelContent applies the shared provider tool-result
// truncation notice used for model context. The caller supplies the limit so
// each surface keeps owning its model-context budget.
func ProviderToolResultModelContent(content string, opts ProviderToolResultContextOptions) (string, ProviderToolResultTruncation) {
	maxChars := opts.MaxModelContentChars
	if maxChars <= 0 || len(content) <= maxChars {
		return content, ProviderToolResultTruncation{}
	}

	truncatedChars := len(content) - maxChars
	truncated := content[:maxChars]
	return fmt.Sprintf("%s\n\n---\n[TRUNCATED: %d characters cut. The result was too large. If you need specific details that may have been cut, make a more targeted query (e.g., filter by specific resource or type).]", truncated, truncatedChars), ProviderToolResultTruncation{
		Applied:        true,
		OriginalChars:  len(content),
		MaxChars:       maxChars,
		TruncatedChars: truncatedChars,
	}
}

// ProjectProviderToolCallToToolCall projects a model-chosen provider tool call
// into the shared tool-call parameter envelope used by Assistant tool execution
// and external-agent adapters.
func ProjectProviderToolCallToToolCall(tc ProviderToolCall) ToolCallParams {
	return NormalizeToolCallParams(ToolCallParams{
		Name:      tc.Name,
		Arguments: tc.Input,
	})
}

// NormalizeProviderToolCallForExecution returns the provider call shape that
// Assistant should execute after projecting through the shared tools/call
// contract. The provider id and thought signature stay attached for transcript
// and provider-continuation use, while name/arguments are normalized exactly as
// the shared tool-call contract sees them.
func NormalizeProviderToolCallForExecution(tc ProviderToolCall) ProviderToolCall {
	normalized := tc.NormalizeCollections()
	params := ProjectProviderToolCallToToolCall(tc)
	normalized.Name = params.Name
	normalized.Input = params.Arguments
	return normalized
}

// NormalizeProviderToolCallsForExecution preserves provider order while
// normalizing every call through the shared tools/call projection.
func NormalizeProviderToolCallsForExecution(calls []ProviderToolCall) []ProviderToolCall {
	normalized := make([]ProviderToolCall, 0, len(calls))
	for _, call := range calls {
		normalized = append(normalized, NormalizeProviderToolCallForExecution(call))
	}
	return normalized
}

// NewPulseQuestionProviderTool returns the native Assistant structured
// clarification tool declaration. It is intentionally provider-facing only:
// pulse_question is not a manifest capability or MCP tool, but its identity and
// schema still belong to the shared Pulse Intelligence provider-tool contract
// so native chat does not hand-roll tool policy locally.
func NewPulseQuestionProviderTool() ProviderTool {
	return ProviderTool{
		Name:        PulseQuestionToolName,
		Description: "Ask the user for missing information using a structured prompt. Use this when you must clarify before proceeding (e.g., choose a target, confirm a risky action, or select among options).",
		InputSchema: PulseQuestionProviderInputSchema(),
	}.NormalizeCollections()
}

const (
	// LegacyAssistantRunCommandToolName is the compatibility alias still used by
	// the older native Assistant service. New registry-backed chat uses
	// pulse_control with type=command, but the alias vocabulary remains shared so
	// provider schemas, execution, and approval handling cannot drift.
	LegacyAssistantRunCommandToolName = "run_command"
	// LegacyAssistantFetchURLToolName is the compatibility alias for the older
	// native Assistant URL fetch helper.
	LegacyAssistantFetchURLToolName = "fetch_url"
	// LegacyAssistantSetResourceURLToolName is the compatibility alias for the
	// older native Assistant resource URL metadata helper.
	LegacyAssistantSetResourceURLToolName = "set_resource_url"

	LegacyAssistantCommandArgumentName      = "command"
	LegacyAssistantRunOnHostArgumentName    = "run_on_host"
	LegacyAssistantTargetHostArgumentName   = "target_host"
	LegacyAssistantURLArgumentName          = "url"
	LegacyAssistantResourceTypeArgumentName = "resource_type"
	LegacyAssistantResourceIDArgumentName   = "resource_id"
)

// LegacyAssistantUtilityProviderTools returns the provider-facing compatibility
// tools still exposed by the older native Assistant service. The definitions
// live in the shared provider-tool contract so legacy Assistant aliases can be
// bridged toward the registry-backed Pulse Intelligence core without local
// schema copies.
func LegacyAssistantUtilityProviderTools() []ProviderTool {
	return ProjectProviderTools([]Tool{
		legacyAssistantRunCommandTool(),
		legacyAssistantFetchURLTool(),
		legacyAssistantSetResourceURLTool(),
	})
}

func legacyAssistantRunCommandTool() Tool {
	return Tool{
		Name:        LegacyAssistantRunCommandToolName,
		Description: "Execute a shell command. By default runs on the current target (container/VM), but set run_on_host=true for Proxmox host commands. IMPORTANT: For targets on different nodes, specify target_host to route to the correct PVE node.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				LegacyAssistantCommandArgumentName: {
					Type:        "string",
					Description: "The shell command to execute (e.g., 'ps aux --sort=-%mem | head -20')",
				},
				LegacyAssistantRunOnHostArgumentName: {
					Type:        "boolean",
					Description: "If true, run on the Proxmox/Docker host instead of inside the container/VM. Use for pct/qm commands like 'pct resize 101 rootfs +10G'. When true, you should also set target_host.",
				},
				LegacyAssistantTargetHostArgumentName: {
					Type:        "string",
					Description: "Optional hostname of the specific host/node to run the command on. Use this to explicitly route pct/qm/docker commands to the correct host node or Docker host. Check the 'node' or 'Host Node' field in the target's context.",
				},
			},
			Required: []string{LegacyAssistantCommandArgumentName},
		},
	}
}

func legacyAssistantFetchURLTool() Tool {
	return Tool{
		Name:        LegacyAssistantFetchURLToolName,
		Description: "Fetch content from a URL. Use this to check if web services are responding, read API endpoints, or fetch documentation. Works with local network URLs and public sites.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				LegacyAssistantURLArgumentName: {
					Type:        "string",
					Description: "The URL to fetch (e.g., 'http://192.0.2.50:8080/api/health' or 'https://example.com/docs')",
				},
			},
			Required: []string{LegacyAssistantURLArgumentName},
		},
	}
}

func legacyAssistantSetResourceURLTool() Tool {
	return Tool{
		Name:        LegacyAssistantSetResourceURLToolName,
		Description: "Set the web URL for a resource in Pulse after discovering a web service. Use this when you've found a web server running on a VM/container/host and want to save it for quick access. The URL will appear as a clickable link in the Pulse dashboard.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertySchema{
				LegacyAssistantResourceTypeArgumentName: {
					Type:        "string",
					Description: "Canonical v6 resource type: 'vm', 'system-container', 'oci-container', 'app-container', 'agent', 'node', or 'docker-host'",
					Enum:        []string{"vm", "system-container", "oci-container", "app-container", "agent", "node", "docker-host"},
				},
				LegacyAssistantResourceIDArgumentName: {
					Type:        "string",
					Description: "The resource ID from context. For VMs/LXC, use the canonical resource ID shown by Pulse (for example 'homelab:pve-node:150'). For app containers, use the container resource ID (for example 'hostid:container:containerid').",
				},
				LegacyAssistantURLArgumentName: {
					Type:        "string",
					Description: "The discovered URL (e.g., 'http://192.0.2.50:8096' for Jellyfin). Use an empty string to remove the URL.",
				},
			},
			Required: []string{LegacyAssistantResourceTypeArgumentName, LegacyAssistantResourceIDArgumentName},
		},
	}
}

// PulseQuestionToolType is the shared value vocabulary for the native
// Assistant structured clarification tool's `type` field.
type PulseQuestionToolType string

const (
	PulseQuestionToolTypeText   PulseQuestionToolType = "text"
	PulseQuestionToolTypeSelect PulseQuestionToolType = "select"
)

// PulseQuestionToolTypeValues returns the provider-schema enum for
// pulse_question question types.
func PulseQuestionToolTypeValues() []string {
	return []string{string(PulseQuestionToolTypeText), string(PulseQuestionToolTypeSelect)}
}

// NormalizePulseQuestionToolType applies the shared pulse_question type
// defaulting and validation rule used by provider schemas and native Assistant
// input parsing.
func NormalizePulseQuestionToolType(rawType string, hasOptions bool) (PulseQuestionToolType, error) {
	normalizedType := strings.TrimSpace(strings.ToLower(rawType))
	if normalizedType == "" {
		if hasOptions {
			normalizedType = string(PulseQuestionToolTypeSelect)
		} else {
			normalizedType = string(PulseQuestionToolTypeText)
		}
	}

	switch PulseQuestionToolType(normalizedType) {
	case PulseQuestionToolTypeText:
		return PulseQuestionToolTypeText, nil
	case PulseQuestionToolTypeSelect:
		if !hasOptions {
			return "", fmt.Errorf("select questions must include options")
		}
		return PulseQuestionToolTypeSelect, nil
	default:
		return "", fmt.Errorf("question.type must be 'text' or 'select'")
	}
}

// PulseQuestionToolQuestion is the normalized in-app Assistant question payload
// parsed from the shared pulse_question provider tool input.
type PulseQuestionToolQuestion struct {
	ID       string
	Type     PulseQuestionToolType
	Header   string
	Question string
	Options  []PulseQuestionToolOption
}

// PulseQuestionToolOption is one normalized option for a select-style
// pulse_question entry.
type PulseQuestionToolOption struct {
	Label       string
	Value       string
	Description string
}

type pulseQuestionToolInputPayload struct {
	Questions json.RawMessage `json:"questions"`
}

type pulseQuestionToolInputQuestion struct {
	ID       string          `json:"id"`
	Type     string          `json:"type,omitempty"`
	Header   string          `json:"header,omitempty"`
	Question string          `json:"question"`
	Options  json.RawMessage `json:"options,omitempty"`
}

type pulseQuestionToolInputOption struct {
	Label       string `json:"label"`
	Value       string `json:"value,omitempty"`
	Description string `json:"description,omitempty"`
}

// ParsePulseQuestionToolInput applies the same validation and normalization as
// the advertised pulse_question provider schema. Native Assistant code should
// adapt the returned values into UI-specific question events rather than owning
// a second parser.
func ParsePulseQuestionToolInput(input map[string]interface{}) ([]PulseQuestionToolQuestion, error) {
	payloadBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("invalid question payload: %w", err)
	}

	var payload pulseQuestionToolInputPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("invalid question payload: %w", err)
	}

	if len(payload.Questions) == 0 {
		return nil, fmt.Errorf("missing required field: questions")
	}

	var rawQuestions []json.RawMessage
	if err := json.Unmarshal(payload.Questions, &rawQuestions); err != nil {
		return nil, fmt.Errorf("questions must be an array")
	}

	if len(rawQuestions) == 0 {
		return nil, fmt.Errorf("questions must not be empty")
	}

	questions := make([]PulseQuestionToolQuestion, 0, len(rawQuestions))
	for _, rawQuestion := range rawQuestions {
		var parsedQuestion pulseQuestionToolInputQuestion
		if err := json.Unmarshal(rawQuestion, &parsedQuestion); err != nil {
			return nil, fmt.Errorf("invalid question entry")
		}

		id := strings.TrimSpace(parsedQuestion.ID)
		questionText := strings.TrimSpace(parsedQuestion.Question)
		header := strings.TrimSpace(parsedQuestion.Header)

		if id == "" {
			return nil, fmt.Errorf("question.id is required")
		}
		if questionText == "" {
			return nil, fmt.Errorf("question.question is required")
		}

		opts, err := parsePulseQuestionToolOptions(parsedQuestion.Options)
		if err != nil {
			return nil, err
		}

		qType, err := NormalizePulseQuestionToolType(parsedQuestion.Type, len(opts) > 0)
		if err != nil {
			return nil, err
		}

		questions = append(questions, PulseQuestionToolQuestion{
			ID:       id,
			Type:     qType,
			Header:   header,
			Question: questionText,
			Options:  opts,
		})
	}
	return questions, nil
}

func parsePulseQuestionToolOptions(rawOptions json.RawMessage) ([]PulseQuestionToolOption, error) {
	if len(rawOptions) == 0 {
		return nil, nil
	}

	trimmed := bytes.TrimSpace(rawOptions)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, fmt.Errorf("question.options must be an array")
	}

	var decodedOptions []pulseQuestionToolInputOption
	if err := json.Unmarshal(rawOptions, &decodedOptions); err != nil {
		return nil, fmt.Errorf("question.options must be an array")
	}

	options := make([]PulseQuestionToolOption, 0, len(decodedOptions))
	for _, decodedOption := range decodedOptions {
		label := strings.TrimSpace(decodedOption.Label)
		value := strings.TrimSpace(decodedOption.Value)
		desc := strings.TrimSpace(decodedOption.Description)

		if label == "" {
			return nil, fmt.Errorf("option.label is required")
		}
		if value == "" {
			value = label
		}

		options = append(options, PulseQuestionToolOption{
			Label:       label,
			Value:       value,
			Description: desc,
		})
	}

	return options, nil
}

// PulseQuestionProviderInputSchema returns the provider-facing JSON Schema for
// the native Assistant structured clarification tool.
func PulseQuestionProviderInputSchema() map[string]interface{} {
	return CloneProviderInputSchema(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"questions": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type":        "string",
							"description": "Stable identifier for this question (used in the answer payload).",
						},
						"type": map[string]interface{}{
							"type":        "string",
							"description": "Question type. Use 'text' for free-form input or 'select' for predefined options.",
							"enum":        PulseQuestionToolTypeValues(),
						},
						"header": map[string]interface{}{
							"type":        "string",
							"description": "Optional short context shown above the question.",
						},
						"question": map[string]interface{}{
							"type":        "string",
							"description": "The question shown to the user.",
						},
						"options": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"label": map[string]interface{}{"type": "string"},
									"value": map[string]interface{}{"type": "string"},
									"description": map[string]interface{}{
										"type":        "string",
										"description": "Optional detail shown under the label.",
									},
								},
								"required": []string{"label"},
							},
						},
					},
					"required": []string{"id", "question"},
				},
			},
		},
		"required": []string{"questions"},
	})
}

// ParseProviderToolInput parses a provider-emitted tool input JSON object. It
// returns ok=false for incomplete streamed JSON, malformed JSON, empty input, or
// non-object values so stream progress surfaces can keep showing raw input while
// the provider is still emitting arguments.
func ParseProviderToolInput(raw string) (map[string]interface{}, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, false
	}
	var input map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &input); err != nil || input == nil {
		return nil, false
	}
	return input, true
}

// ProviderToolInputOrRaw returns a parsed provider tool input object, or the
// legacy-compatible raw fallback used when a provider emits malformed,
// incomplete, or empty arguments at final tool-call assembly time.
func ProviderToolInputOrRaw(raw string) map[string]interface{} {
	if input, ok := ParseProviderToolInput(raw); ok {
		return input
	}
	return map[string]interface{}{"raw": raw}
}

// InputSchema describes the expected object input for a tool.
type InputSchema struct {
	Type       string                    `json:"type"` // Always "object"
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// NormalizeCollections returns an independent JSON-object schema with stable
// empty collections.
func (s InputSchema) NormalizeCollections() InputSchema {
	if s.Type == "" {
		s.Type = "object"
	}
	s.Properties = ClonePropertySchemas(s.Properties)
	if s.Properties == nil {
		s.Properties = map[string]PropertySchema{}
	}
	s.Required = append([]string(nil), s.Required...)
	return s
}

// PropertySchema describes a property in a structured tool input schema.
type PropertySchema struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// NormalizeCollections returns an independent property schema for use in
// cross-surface tool declarations.
func (p PropertySchema) NormalizeCollections() PropertySchema {
	p.Enum = append([]string(nil), p.Enum...)
	p.Default = cloneSchemaValue(p.Default)
	return p
}

// ClonePropertySchemas returns an independent copy of structured tool property
// schemas.
func ClonePropertySchemas(properties map[string]PropertySchema) map[string]PropertySchema {
	if properties == nil {
		return nil
	}
	cloned := make(map[string]PropertySchema, len(properties))
	for name, property := range properties {
		cloned[name] = property.NormalizeCollections()
	}
	return cloned
}

// CloneProviderInputSchema returns an independent copy of a provider JSON
// Schema map.
func CloneProviderInputSchema(schema map[string]interface{}) map[string]interface{} {
	if schema == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(schema))
	for name, value := range schema {
		cloned[name] = cloneSchemaValue(value)
	}
	return cloned
}

func cloneSchemaValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			cloned[key] = cloneSchemaValue(child)
		}
		return cloned
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i, child := range typed {
			cloned[i] = cloneSchemaValue(child)
		}
		return cloned
	case []string:
		return append([]string(nil), typed...)
	default:
		return value
	}
}

// ObjectInputSchemaMap builds the common JSON Schema object envelope used by
// Pulse Intelligence capability manifests and adapters. Strict manifest-owned
// schemas should pass additionalProperties=false; adapter fallback schemas may
// pass true when the underlying endpoint contract is only partially known.
func ObjectInputSchemaMap(required []string, properties map[string]any, additionalProperties bool) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": additionalProperties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// StrictObjectInputSchemaMap builds a closed object schema for manifest-owned
// tool arguments.
func StrictObjectInputSchemaMap(required []string, properties map[string]any) map[string]any {
	return ObjectInputSchemaMap(required, properties, false)
}

// RawInputSchema marshals a hand-authored JSON Schema map into the raw manifest
// field served by the API and forwarded by adapters.
func RawInputSchema(schema map[string]any) json.RawMessage {
	raw, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return raw
}

// CloneRawMessage returns an independent copy of a raw JSON payload.
func CloneRawMessage(raw json.RawMessage) json.RawMessage {
	if raw == nil {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

// ObjectInputSchema builds a raw JSON Schema object envelope.
func ObjectInputSchema(required []string, properties map[string]any, additionalProperties bool) json.RawMessage {
	return RawInputSchema(ObjectInputSchemaMap(required, properties, additionalProperties))
}

// StrictObjectInputSchema builds a closed raw JSON Schema object envelope for
// manifest-owned tool arguments.
func StrictObjectInputSchema(required []string, properties map[string]any) json.RawMessage {
	return ObjectInputSchema(required, properties, false)
}

// ProviderInputSchema projects a structured registry input schema into the
// generic JSON Schema map expected by chat providers.
func ProviderInputSchema(schema InputSchema) map[string]interface{} {
	schema = schema.NormalizeCollections()
	projected := map[string]interface{}{
		"type":       "object",
		"properties": ProviderPropertySchemas(schema.Properties),
	}

	if len(schema.Required) > 0 {
		projected["required"] = append([]string(nil), schema.Required...)
	}

	return projected
}

// ProviderInputSchemaFromRaw projects a manifest-authored JSON Schema into the
// provider-facing schema map used by Assistant provider tools. Manifest schemas
// are already JSON Schema object envelopes; this helper keeps legacy Assistant
// callers from copying required-field lists by hand.
func ProviderInputSchemaFromRaw(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return ProviderInputSchema(InputSchema{})
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil || schema == nil {
		return ProviderInputSchema(InputSchema{})
	}

	projected := CloneProviderInputSchema(schema)
	if _, ok := projected["type"]; !ok {
		projected["type"] = "object"
	}
	if _, ok := projected["properties"]; !ok {
		projected["properties"] = map[string]interface{}{}
	}
	if required, ok := projected["required"].([]interface{}); ok {
		normalized := make([]string, 0, len(required))
		allStrings := true
		for _, value := range required {
			item, ok := value.(string)
			if !ok {
				allStrings = false
				break
			}
			normalized = append(normalized, item)
		}
		if allStrings {
			projected["required"] = normalized
		}
	}

	return projected
}

// ProjectProviderTool projects one Assistant registry tool into the generic
// provider tool shape used by model clients.
func ProjectProviderTool(tool Tool) ProviderTool {
	tool = tool.NormalizeCollections()
	return ProviderTool{
		Name:        tool.Name,
		Description: tool.Description,
		InputSchema: ProviderInputSchema(tool.InputSchema),
	}.NormalizeCollections()
}

// ProjectProviderToolWithGovernance projects one Assistant registry tool into
// the provider-facing shape and appends the shared governance posture to the
// tool description when a matching descriptor is available.
func ProjectProviderToolWithGovernance(tool Tool, governance ToolGovernanceDescriptor) ProviderTool {
	projected := ProjectProviderTool(tool)
	if !providerToolGovernanceMatches(projected.Name, governance.Name) {
		return projected
	}
	governance = NormalizeToolGovernanceDescriptor(governance)
	projected.Description = ProviderToolDescriptionWithGovernance(projected.Description, governance)
	projected.BehaviorHints = ToolGovernanceBehaviorHints(governance)
	projected.PulseGovernance = &governance
	return projected.NormalizeCollections()
}

// ProjectProviderTools projects Assistant registry tools into provider-facing
// tool declarations while preserving order.
func ProjectProviderTools(tools []Tool) []ProviderTool {
	projected := make([]ProviderTool, 0, len(tools))
	for _, tool := range tools {
		projected = append(projected, ProjectProviderTool(tool))
	}
	return projected
}

// ProjectProviderToolsWithGovernance projects Assistant registry tools into
// provider-facing declarations while attaching the matching registry-owned
// governance descriptor to each description.
func ProjectProviderToolsWithGovernance(tools []Tool, governance []ToolGovernanceDescriptor) []ProviderTool {
	governanceByName := make(map[string]ToolGovernanceDescriptor, len(governance))
	for _, descriptor := range governance {
		name := strings.TrimSpace(descriptor.Name)
		if name == "" {
			continue
		}
		governanceByName[name] = descriptor
	}

	projected := make([]ProviderTool, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		descriptor, ok := governanceByName[name]
		if !ok {
			projected = append(projected, ProjectProviderTool(tool))
			continue
		}
		projected = append(projected, ProjectProviderToolWithGovernance(tool, descriptor))
	}
	return projected
}

// ProjectAssistantProviderTools projects the native Assistant provider tool
// surface from shared Pulse Intelligence contracts. Chat chooses whether the
// current turn is interactive; the shared layer owns how Assistant registry
// tools and Assistant-native interaction tools are composed.
func ProjectAssistantProviderTools(tools []Tool, governance []ToolGovernanceDescriptor, opts AssistantProviderToolOptions) []ProviderTool {
	projected := ProjectProviderToolsWithGovernance(tools, governance)
	if opts.IncludeQuestionTool {
		projected = append(projected, AssistantNativeProviderTools()...)
	}
	return projected
}

// ProjectPulseAssistantProviderTools projects native Assistant provider tools
// through the manifest-owned Pulse Assistant surface affordances. The runtime
// registry remains the source of available handlers; the manifest decides
// whether the Assistant surface may advertise tools and interactive questions.
func ProjectPulseAssistantProviderTools(manifest Manifest, tools []Tool, governance []ToolGovernanceDescriptor, opts AssistantProviderToolOptions) []ProviderTool {
	affordances, _ := ManifestSurfaceAffordances(manifest, SurfaceIDPulseAssistant)
	if !affordances.Tools {
		return []ProviderTool{}
	}
	if !affordances.InteractiveQuestions {
		opts.IncludeQuestionTool = false
	}
	return ProjectAssistantProviderTools(tools, governance, opts)
}

// ProviderToolGovernanceDescriptors extracts registry-owned governance from an
// offered Assistant provider-tool list. A nil tool slice means the caller has no
// concrete offered-tool list, so no complete metadata projection can be proven.
// An empty non-nil slice is a valid "no tools offered" manifest.
func ProviderToolGovernanceDescriptors(tools []ProviderTool) ([]ToolGovernanceDescriptor, bool) {
	if tools == nil {
		return nil, false
	}

	descriptors := make([]ToolGovernanceDescriptor, 0, len(tools))
	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if name == PulseQuestionToolName {
			continue
		}
		if tool.PulseGovernance == nil {
			return nil, false
		}
		descriptor := NormalizeToolGovernanceDescriptor(*tool.PulseGovernance)
		if !providerToolGovernanceMatches(name, descriptor.Name) {
			return nil, false
		}
		descriptors = append(descriptors, descriptor)
	}
	return descriptors, true
}

// AssistantNativeProviderTools returns provider-facing tools owned by the
// native in-app Assistant surface rather than the registry-backed execution
// tools. These names still belong to the shared provider-tool contract because
// model clients, prompt governance, and leak sanitizers all need the same
// surface vocabulary.
func AssistantNativeProviderTools() []ProviderTool {
	return []ProviderTool{NewPulseQuestionProviderTool()}
}

// AssistantNativeProviderToolNames returns the canonical names for native
// Assistant provider tools.
func AssistantNativeProviderToolNames() []string {
	return ProviderToolNames(AssistantNativeProviderTools())
}

// ProviderToolNames returns the stable offered-tool name sequence for a
// provider tool list. A nil tool slice remains nil so callers can preserve the
// "all manifest tools" meaning used by prompt projections; an empty non-nil
// slice means no tools are offered.
func ProviderToolNames(tools []ProviderTool) []string {
	if tools == nil {
		return nil
	}
	names := make([]string, 0, len(tools))
	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

// ProviderToolNameCatalog is a reusable exact/prefix lookup over provider-tool
// names. It keeps Assistant stream sanitizers and adapter leak guards on the
// same name normalization rules as provider tool projection.
type ProviderToolNameCatalog struct {
	names []string
	set   map[string]struct{}
}

// NewProviderToolNameCatalog builds a detached tool-name catalog from one or
// more ordered name groups. Empty and duplicate names are removed while the
// first-seen order is preserved.
func NewProviderToolNameCatalog(nameGroups ...[]string) ProviderToolNameCatalog {
	total := 0
	for _, names := range nameGroups {
		total += len(names)
	}

	catalog := ProviderToolNameCatalog{
		names: make([]string, 0, total),
		set:   make(map[string]struct{}, total),
	}
	for _, names := range nameGroups {
		for _, name := range names {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			if _, ok := catalog.set[trimmed]; ok {
				continue
			}
			catalog.set[trimmed] = struct{}{}
			catalog.names = append(catalog.names, trimmed)
		}
	}
	return catalog
}

// NewAssistantProviderToolNameCatalog adds native Assistant provider tools to
// the registered runtime tool names so leak guards recognize the complete
// Assistant provider surface.
func NewAssistantProviderToolNameCatalog(registryToolNames []string) ProviderToolNameCatalog {
	return NewProviderToolNameCatalog(registryToolNames, AssistantNativeProviderToolNames())
}

// Names returns a detached ordered copy of the catalog names.
func (c ProviderToolNameCatalog) Names() []string {
	return append([]string(nil), c.names...)
}

// Has reports whether name is in the catalog.
func (c ProviderToolNameCatalog) Has(name string) bool {
	if name == "" {
		return false
	}
	_, ok := c.set[name]
	return ok
}

// HasPrefix reports whether prefix can still become a catalogued tool name.
func (c ProviderToolNameCatalog) HasPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for _, name := range c.names {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// ProviderToolDescriptionWithGovernance returns a provider-facing description
// that keeps the tool's functional description and the canonical governance
// posture adjacent in the same declaration the model sees.
func ProviderToolDescriptionWithGovernance(description string, governance ToolGovernanceDescriptor) string {
	governanceDescription := strings.TrimSpace(ToolGovernancePromptDescription(governance))
	description = strings.TrimSpace(description)
	if governanceDescription == "" {
		return description
	}
	line := "Pulse governance: " + governanceDescription
	if description == "" {
		return line
	}
	return description + "\n\n" + line
}

func providerToolGovernanceMatches(toolName, governanceName string) bool {
	return strings.TrimSpace(toolName) != "" && strings.TrimSpace(toolName) == strings.TrimSpace(governanceName)
}

// ProviderPropertySchemas projects structured registry properties into provider
// JSON Schema property definitions.
func ProviderPropertySchemas(properties map[string]PropertySchema) map[string]interface{} {
	projected := make(map[string]interface{}, len(properties))
	for name, property := range properties {
		projected[name] = ProviderPropertySchema(property)
	}
	return projected
}

// ProviderPropertySchema projects one structured registry property into a
// provider JSON Schema property definition.
func ProviderPropertySchema(property PropertySchema) map[string]interface{} {
	property = property.NormalizeCollections()
	projected := map[string]interface{}{
		"type": property.Type,
	}
	if property.Description != "" {
		projected["description"] = property.Description
	}
	if len(property.Enum) > 0 {
		projected["enum"] = append([]string(nil), property.Enum...)
	}
	if property.Default != nil {
		projected["default"] = property.Default
	}
	return projected
}
