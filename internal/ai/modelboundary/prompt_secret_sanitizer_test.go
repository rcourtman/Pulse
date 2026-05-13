package modelboundary

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
)

func TestRequestSanitizerForModelRedactsPromptSecretsWithoutResourcePolicy(t *testing.T) {
	sanitizer := RequestSanitizerForModel("anthropic:claude-3-5-sonnet", nil)
	if sanitizer == nil {
		t.Fatal("expected external model sanitizer without resource provider")
	}

	req := providers.ChatRequest{
		System: "Use this only if needed: password: system-password",
		Messages: []providers.Message{
			{
				Role:             "user",
				Content:          `Operator prompt includes {"api_key":"json-secret-value"}`,
				ReasoningContent: "Authorization: Bearer sk-reasoning-secret",
				ToolCalls: []providers.ToolCall{{
					ID:   "tool-1",
					Name: "pulse_report",
					Input: map[string]interface{}{
						"api_key": "plain-tool-key",
						"credentials": map[string]interface{}{
							"value":    "nested-credential-value",
							"metadata": "safe metadata",
						},
						"safe": []interface{}{"keep-me", "sk-provider-token"},
					},
				}},
			},
			{
				Role:       "tool",
				ToolResult: &providers.ToolResult{ToolUseID: "tool-1", Content: "x-api-key: sk-tool-result-secret"},
			},
		},
		Tools: []providers.Tool{{
			Name:        "pulse_report",
			Description: "Call report narrator with access_token=tool-description-token",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"api_key": map[string]interface{}{
						"type":        "string",
						"description": "Provider API key",
						"default":     "schema-default-secret",
						"examples":    []interface{}{"schema-example-secret"},
					},
				},
			},
		}},
	}

	got := sanitizer(req)
	combined := strings.Join([]string{
		got.System,
		got.Messages[0].Content,
		got.Messages[0].ReasoningContent,
		got.Messages[1].ToolResult.Content,
		got.Tools[0].Description,
	}, "\n")
	for _, forbidden := range []string{
		"system-password",
		"json-secret-value",
		"sk-reasoning-secret",
		"sk-tool-result-secret",
		"tool-description-token",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("sanitized request leaked %q:\n%s", forbidden, combined)
		}
	}

	input := got.Messages[0].ToolCalls[0].Input
	if input["api_key"] != "[REDACTED]" {
		t.Fatalf("tool call api_key = %#v, want redacted marker", input["api_key"])
	}
	credentials := input["credentials"].(map[string]interface{})
	if credentials["value"] != "[REDACTED]" {
		t.Fatalf("nested credential value = %#v, want redacted marker", credentials["value"])
	}
	if credentials["metadata"] != "safe metadata" {
		t.Fatalf("non-value credential metadata was changed: %#v", credentials["metadata"])
	}
	safeValues := input["safe"].([]interface{})
	if safeValues[0] != "keep-me" {
		t.Fatalf("safe non-secret value was changed: %#v", safeValues[0])
	}
	if strings.Contains(safeValues[1].(string), "sk-provider-token") {
		t.Fatalf("provider-shaped token in safe field was not redacted: %#v", safeValues[1])
	}

	properties := got.Tools[0].InputSchema["properties"].(map[string]interface{})
	apiKeyProperty := properties["api_key"].(map[string]interface{})
	if apiKeyProperty["type"] != "string" {
		t.Fatalf("schema type was changed: %#v", apiKeyProperty["type"])
	}
	if apiKeyProperty["description"] != "Provider API key" {
		t.Fatalf("schema description was changed: %#v", apiKeyProperty["description"])
	}
	if apiKeyProperty["default"] != "[REDACTED]" {
		t.Fatalf("schema default = %#v, want redacted marker", apiKeyProperty["default"])
	}
	examples := apiKeyProperty["examples"].([]interface{})
	if examples[0] != "[REDACTED]" {
		t.Fatalf("schema example = %#v, want redacted marker", examples[0])
	}

	if req.Messages[0].ToolCalls[0].Input["api_key"] != "plain-tool-key" {
		t.Fatalf("sanitizer mutated original tool call input: %#v", req.Messages[0].ToolCalls[0].Input["api_key"])
	}
	if req.Tools[0].InputSchema["type"] != "object" {
		t.Fatalf("sanitizer mutated original tool schema: %#v", req.Tools[0].InputSchema["type"])
	}
}
