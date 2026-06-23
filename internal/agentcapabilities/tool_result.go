package agentcapabilities

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// ToolContent is one content block in a shared tool result.
type ToolContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// ToolResult is the shared result envelope used by the in-app Assistant tool
// executor and external-agent adapters.
type ToolResult struct {
	Content           []ToolContent  `json:"content"`
	StructuredContent map[string]any `json:"structuredContent,omitempty"`
	IsError           bool           `json:"isError,omitempty"`
}

// EmptyToolResult returns a result with initialized collection fields.
func EmptyToolResult() ToolResult {
	return ToolResult{}.NormalizeCollections()
}

// NormalizeCollections returns an independent tool result with stable empty
// collections.
func (r ToolResult) NormalizeCollections() ToolResult {
	r.Content = append([]ToolContent(nil), r.Content...)
	if r.Content == nil {
		r.Content = []ToolContent{}
	}
	r.StructuredContent = CloneToolStructuredContent(r.StructuredContent)
	return r
}

// NewToolTextContent creates a text content block.
func NewToolTextContent(text string) ToolContent {
	return ToolContent{Type: "text", Text: text}
}

// NewToolTextResult creates a successful text tool result.
func NewToolTextResult(text string) ToolResult {
	return NewToolTextResultWithIsError(text, false)
}

// NewToolTextResultWithIsError creates a text result with explicit error state.
func NewToolTextResultWithIsError(text string, isError bool) ToolResult {
	return ToolResult{
		Content: []ToolContent{NewToolTextContent(text)},
		IsError: isError,
	}.NormalizeCollections()
}

// NewToolErrorResult creates an error tool result from a Go error.
func NewToolErrorResult(err error) ToolResult {
	if err == nil {
		return NewToolTextResultWithIsError("", true)
	}
	return NewToolTextResultWithIsError(err.Error(), true)
}

// NewToolJSONResult creates a successful JSON text result.
func NewToolJSONResult(data any) ToolResult {
	return NewToolJSONResultWithIsError(data, false)
}

// NewToolJSONResultWithIsError creates a JSON text result with explicit error
// state.
func NewToolJSONResultWithIsError(data any, isError bool) ToolResult {
	body, err := json.Marshal(data)
	if err != nil {
		return NewToolErrorResult(err)
	}
	result := NewToolTextResultWithIsError(string(body), isError)
	if structured, ok := ToolStructuredContentFromJSON(body); ok {
		result.StructuredContent = structured
	}
	return result.NormalizeCollections()
}

// NewToolHTTPTextResult wraps an upstream Pulse HTTP response body as the tool
// content result while preserving non-2xx status as isError=true.
func NewToolHTTPTextResult(body []byte, statusCode int) ToolResult {
	return newToolTextResultFromBody(body, statusCode < 200 || statusCode >= 300)
}

// NewCapabilityHTTPToolResult wraps a manifest capability HTTP response as a
// tool result. The full HTTPCallResponse stays in this shared package so
// adapters do not duplicate the status-to-isError interpretation.
func NewCapabilityHTTPToolResult(resp HTTPCallResponse) ToolResult {
	return newToolTextResultFromBody(resp.Body, !resp.OK())
}

func newToolTextResultFromBody(body []byte, isError bool) ToolResult {
	result := NewToolTextResultWithIsError(string(body), isError)
	if structured, ok := ToolStructuredContentFromJSON(body); ok {
		result.StructuredContent = structured
	}
	return result.NormalizeCollections()
}

// ToolStructuredContentFromJSON returns an MCP-compatible structured content
// object for JSON object payloads. JSON array payloads are wrapped as
// {"items":[...],"count":n} because MCP structuredContent is object-shaped,
// while Pulse's list endpoints intentionally keep their raw array text body for
// legacy clients.
func ToolStructuredContentFromJSON(raw []byte) (map[string]any, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, false
	}

	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	switch trimmed[0] {
	case '{':
		var structured map[string]any
		if err := dec.Decode(&structured); err != nil || structured == nil {
			return nil, false
		}
		var trailing any
		if err := dec.Decode(&trailing); err != io.EOF {
			return nil, false
		}
		return CloneToolStructuredContent(structured), true
	case '[':
		var items []any
		if err := dec.Decode(&items); err != nil || items == nil {
			return nil, false
		}
		var trailing any
		if err := dec.Decode(&trailing); err != io.EOF {
			return nil, false
		}
		return CloneToolStructuredContent(map[string]any{
			"items": items,
			"count": len(items),
		}), true
	default:
		return nil, false
	}
}

// CloneToolStructuredContent returns an independent JSON-like object for shared
// tool results crossing Assistant, MCP, and API boundaries.
func CloneToolStructuredContent(content map[string]any) map[string]any {
	if content == nil {
		return nil
	}
	cloned := make(map[string]any, len(content))
	for key, value := range content {
		cloned[key] = cloneSchemaValue(value)
	}
	return cloned
}

// ToolResultText joins text content blocks from a shared tool result for
// provider context and display surfaces. Non-text content blocks stay available
// on the structured envelope but are intentionally not flattened here.
func ToolResultText(result ToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}

	var text strings.Builder
	for _, c := range result.Content {
		if c.Type != "text" || c.Text == "" {
			continue
		}
		if text.Len() > 0 {
			text.WriteByte('\n')
		}
		text.WriteString(c.Text)
	}
	return text.String()
}

// ToolResultInterpretation is the shared flattened view of a tool result used
// by native Assistant execution paths and external-agent adapters that need to
// branch on the legacy-compatible tool marker vocabulary.
type ToolResultInterpretation struct {
	Text             string
	IsError          bool
	ApprovalRequired bool
	PolicyBlocked    bool
}

// InterpretToolResult flattens text content and detects the shared
// approval-required and policy-blocked markers in one place so callers do not
// duplicate marker prefix checks around ToolResultText.
func InterpretToolResult(result ToolResult) ToolResultInterpretation {
	text := ToolResultText(result)
	return ToolResultInterpretation{
		Text:             text,
		IsError:          result.IsError,
		ApprovalRequired: HasApprovalRequiredToolMarker(text),
		PolicyBlocked:    HasPolicyBlockedToolMarker(text),
	}
}
