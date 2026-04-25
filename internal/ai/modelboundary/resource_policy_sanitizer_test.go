package modelboundary

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type policySanitizerProvider struct {
	resources []unifiedresources.Resource
}

func (p policySanitizerProvider) GetAll() []unifiedresources.Resource {
	return append([]unifiedresources.Resource(nil), p.resources...)
}

func (p policySanitizerProvider) GetByType(t unifiedresources.ResourceType) []unifiedresources.Resource {
	var out []unifiedresources.Resource
	for _, resource := range p.resources {
		if resource.Type == t {
			out = append(out, resource)
		}
	}
	return out
}

func TestRequestSanitizerForModelRedactsExternalModelRequest(t *testing.T) {
	resource := unifiedresources.Resource{
		ID:   "agent/pve-secret",
		Type: unifiedresources.ResourceTypeAgent,
		Name: "pve-secret",
		Tags: []string{"restricted"},
		Identity: unifiedresources.ResourceIdentity{
			Hostnames:   []string{"pve-secret.lan"},
			IPAddresses: []string{"10.0.0.5"},
			ClusterName: "prod-alias",
		},
	}
	sanitizer := RequestSanitizerForModel("openai:gpt-4o", policySanitizerProvider{resources: []unifiedresources.Resource{resource}})
	if sanitizer == nil {
		t.Fatal("expected external model sanitizer")
	}

	req := providers.ChatRequest{
		System: "Investigate pve-secret at 10.0.0.5",
		Messages: []providers.Message{
			{
				Role:    "user",
				Content: "pve-secret has alerts on pve-secret.lan",
				ToolCalls: []providers.ToolCall{{
					ID: "tool-1",
					Input: map[string]interface{}{
						"target": "pve-secret",
						"nested": map[string]interface{}{"host": "pve-secret.lan"},
						"hosts":  []string{"pve-secret.lan"},
					},
				}},
			},
			{
				Role:       "tool",
				ToolResult: &providers.ToolResult{ToolUseID: "tool-1", Content: "agent/pve-secret reports prod-alias"},
			},
		},
		Tools: []providers.Tool{{
			Name:        "pulse_read",
			Description: "Read pve-secret",
			InputSchema: map[string]interface{}{
				"properties": map[string]interface{}{
					"target": map[string]interface{}{
						"enum": []interface{}{"pve-secret", "other"},
					},
				},
			},
		}},
	}

	got := sanitizer(req)
	combined := got.System + "\n" + got.Messages[0].Content + "\n" + got.Messages[1].ToolResult.Content + "\n" + got.Tools[0].Description
	rawValues := []string{"pve-secret", "10.0.0.5", "pve-secret.lan", "agent/pve-secret", "prod-alias"}
	for _, raw := range rawValues {
		if strings.Contains(combined, raw) {
			t.Fatalf("sanitized request still contains %q: %s", raw, combined)
		}
	}
	if gotTarget, _ := got.Messages[0].ToolCalls[0].Input["target"].(string); strings.Contains(gotTarget, "pve-secret") {
		t.Fatalf("tool call input target was not redacted: %q", gotTarget)
	}
	nested := got.Messages[0].ToolCalls[0].Input["nested"].(map[string]interface{})
	if gotHost, _ := nested["host"].(string); strings.Contains(gotHost, "pve-secret.lan") {
		t.Fatalf("nested tool call input host was not redacted: %q", gotHost)
	}
	hosts := got.Messages[0].ToolCalls[0].Input["hosts"].([]string)
	if strings.Contains(hosts[0], "pve-secret.lan") {
		t.Fatalf("tool call input host slice was not redacted: %q", hosts[0])
	}
	toolProperties := got.Tools[0].InputSchema["properties"].(map[string]interface{})
	toolTarget := toolProperties["target"].(map[string]interface{})
	toolEnum := toolTarget["enum"].([]interface{})
	if gotEnum, _ := toolEnum[0].(string); strings.Contains(gotEnum, "pve-secret") {
		t.Fatalf("tool schema enum was not redacted: %q", gotEnum)
	}
	if req.Messages[0].Content != "pve-secret has alerts on pve-secret.lan" {
		t.Fatalf("sanitizer mutated original request content: %q", req.Messages[0].Content)
	}
	if req.Tools[0].Description != "Read pve-secret" {
		t.Fatalf("sanitizer mutated original tool description: %q", req.Tools[0].Description)
	}
}

func TestRequestSanitizerForModelSkipsLocalModel(t *testing.T) {
	resource := unifiedresources.Resource{
		ID:     "agent/pve-secret",
		Type:   unifiedresources.ResourceTypeAgent,
		Name:   "pve-secret",
		Policy: &unifiedresources.ResourcePolicy{Routing: unifiedresources.ResourceRoutingPolicy{Scope: unifiedresources.ResourceRoutingScopeLocalOnly}},
	}
	if sanitizer := RequestSanitizerForModel("ollama:llama3", policySanitizerProvider{resources: []unifiedresources.Resource{resource}}); sanitizer != nil {
		t.Fatal("expected no sanitizer for local Ollama model")
	}
}
