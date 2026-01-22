package tools

import (
	"encoding/json"
)

// JSON-RPC 2.0 types for MCP protocol

// Request represents a JSON-RPC request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents a JSON-RPC response
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Standard JSON-RPC error codes
const (
	ErrParse          = -32700
	ErrInvalidRequest = -32600
	ErrMethodNotFound = -32601
	ErrInvalidParams  = -32602
	ErrInternal       = -32603
)

// MCP-specific types

// ServerInfo describes the MCP server
type ServerInfo struct {
	Name         string       `json:"name"`
	Version      string       `json:"version"`
	Capabilities Capabilities `json:"capabilities,omitempty"`
}

// Capabilities describes what the server supports
type Capabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability describes tool support
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability describes resource support
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability describes prompt support
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeParams are the params for the initialize method
type InitializeParams struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ClientInfo      ClientInfo   `json:"clientInfo"`
}

// ClientInfo describes the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResult is the result of the initialize method
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

// Tool describes an available tool
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema describes the expected input for a tool
type InputSchema struct {
	Type       string                    `json:"type"` // Always "object"
	Properties map[string]PropertySchema `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

// PropertySchema describes a property in the input schema
type PropertySchema struct {
	Type        string      `json:"type"`
	Description string      `json:"description,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

// ListToolsResult is the result of tools/list
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams are the params for tools/call
type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// CallToolResult is the result of tools/call
type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents content in a tool result
type Content struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"` // base64 for images
	URI      string `json:"uri,omitempty"`  // for resources
}

// Resource describes an available resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ListResourcesResult is the result of resources/list
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// ReadResourceParams are the params for resources/read
type ReadResourceParams struct {
	URI string `json:"uri"`
}

// ReadResourceResult is the result of resources/read
type ReadResourceResult struct {
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent is the content of a resource
type ResourceContent struct {
	URI      string `json:"uri"`
	MimeType string `json:"mimeType,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     string `json:"blob,omitempty"` // base64
}

// Prompt describes an available prompt template
type Prompt struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
}

// PromptArgument describes an argument for a prompt
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ListPromptsResult is the result of prompts/list
type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
}

// GetPromptParams are the params for prompts/get
type GetPromptParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments,omitempty"`
}

// GetPromptResult is the result of prompts/get
type GetPromptResult struct {
	Description string          `json:"description,omitempty"`
	Messages    []PromptMessage `json:"messages"`
}

// PromptMessage is a message in a prompt
type PromptMessage struct {
	Role    string  `json:"role"` // "user" or "assistant"
	Content Content `json:"content"`
}

// Helper functions

// NewTextContent creates a text content object
func NewTextContent(text string) Content {
	return Content{
		Type: "text",
		Text: text,
	}
}

// NewErrorResult creates an error tool result
func NewErrorResult(err error) CallToolResult {
	return CallToolResult{
		Content: []Content{NewTextContent(err.Error())},
		IsError: true,
	}
}

// NewTextResult creates a successful text tool result
func NewTextResult(text string) CallToolResult {
	return CallToolResult{
		Content: []Content{NewTextContent(text)},
		IsError: false,
	}
}

// NewJSONResult creates a successful JSON tool result
// The data is marshaled to JSON and returned as text content
func NewJSONResult(data interface{}) CallToolResult {
	b, err := json.Marshal(data)
	if err != nil {
		return NewErrorResult(err)
	}
	return CallToolResult{
		Content: []Content{NewTextContent(string(b))},
		IsError: false,
	}
}
