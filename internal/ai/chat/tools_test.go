package chat

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/tools"
	"github.com/stretchr/testify/assert"
)

func TestToolsUtils(t *testing.T) {
	t.Run("ConvertMCPToolsToProvider", func(t *testing.T) {
		mcpTools := []tools.Tool{
			{
				Name:        "test_tool",
				Description: "A test tool",
				InputSchema: tools.InputSchema{
					Type: "object",
					Properties: map[string]tools.PropertySchema{
						"arg1": {
							Type:        "string",
							Description: "First argument",
							Enum:        []string{"a", "b"},
						},
						"arg2": {
							Type:    "number",
							Default: 10,
						},
					},
					Required: []string{"arg1"},
				},
			},
		}

		converted := ConvertMCPToolsToProvider(mcpTools)
		require := assert.New(t)
		require.Len(converted, 1)
		require.Equal("test_tool", converted[0].Name)
		require.Equal("A test tool", converted[0].Description)

		schema := converted[0].InputSchema
		require.Equal("object", schema["type"])
		props := schema["properties"].(map[string]interface{})
		arg1 := props["arg1"].(map[string]interface{})
		require.Equal("string", arg1["type"])
		require.Equal("First argument", arg1["description"])
		require.Equal([]string{"a", "b"}, arg1["enum"])

		required := schema["required"].([]string)
		require.Contains(required, "arg1")
	})

	t.Run("ConvertProviderToolCallToMCP", func(t *testing.T) {
		tc := providers.ToolCall{
			ID:    "call1",
			Name:  "test_tool",
			Input: map[string]interface{}{"arg1": "val1"},
		}
		mcpTC := ConvertProviderToolCallToMCP(tc)
		assert.Equal(t, "test_tool", mcpTC.Name)
		assert.Equal(t, "val1", mcpTC.Arguments["arg1"])
	})

	t.Run("FormatToolResult", func(t *testing.T) {
		result := tools.CallToolResult{
			Content: []tools.Content{
				{Type: "text", Text: "part 1"},
				{Type: "text", Text: "part 2"},
			},
		}
		formatted := FormatToolResult(result)
		assert.Equal(t, "part 1\npart 2", formatted)

		assert.Equal(t, "", FormatToolResult(tools.CallToolResult{}))
	})
}
