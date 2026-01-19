package chat

import (
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
)

// ConvertMCPToolsToProvider converts MCP tool definitions to provider tool format
func ConvertMCPToolsToProvider(mcpTools []tools.Tool) []providers.Tool {
	result := make([]providers.Tool, 0, len(mcpTools))

	for _, t := range mcpTools {
		// Build the input schema in the format expected by providers
		inputSchema := make(map[string]interface{})
		inputSchema["type"] = "object"

		// Convert properties
		if len(t.InputSchema.Properties) > 0 {
			props := make(map[string]interface{})
			for name, prop := range t.InputSchema.Properties {
				propDef := map[string]interface{}{
					"type": prop.Type,
				}
				if prop.Description != "" {
					propDef["description"] = prop.Description
				}
				if len(prop.Enum) > 0 {
					propDef["enum"] = prop.Enum
				}
				if prop.Default != nil {
					propDef["default"] = prop.Default
				}
				props[name] = propDef
			}
			inputSchema["properties"] = props
		}

		// Add required fields
		if len(t.InputSchema.Required) > 0 {
			inputSchema["required"] = t.InputSchema.Required
		}

		result = append(result, providers.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: inputSchema,
		})
	}

	return result
}

// ConvertProviderToolCallToMCP converts a provider tool call to MCP format
func ConvertProviderToolCallToMCP(tc providers.ToolCall) tools.CallToolParams {
	return tools.CallToolParams{
		Name:      tc.Name,
		Arguments: tc.Input,
	}
}

// FormatToolResult formats a tool result for display
func FormatToolResult(result tools.CallToolResult) string {
	if len(result.Content) == 0 {
		return ""
	}

	// Combine all text content
	var text string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			if text != "" {
				text += "\n"
			}
			text += c.Text
		}
	}

	return text
}
