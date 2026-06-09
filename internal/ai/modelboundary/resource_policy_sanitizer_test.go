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

func TestRequestSanitizerForModelAllowsPulseGeneratedInventoryExportOnly(t *testing.T) {
	resource := unifiedresources.Resource{
		ID:   "vm-100",
		Type: unifiedresources.ResourceTypeVM,
		Name: "vm1",
		Tags: []string{"secret"}, // Restricted -> local-only -> identity redacted in free text
		Identity: unifiedresources.ResourceIdentity{
			Hostnames: []string{"vm1"},
		},
		Proxmox: &unifiedresources.ProxmoxData{VMID: 100, NodeName: "node1"},
	}
	allowedInventoryContext := `Pulse-generated inventory context:
{"answer_label":"VM 100 vm1","name":"vm1","status":"running"}`
	sanitizer := RequestSanitizerForModel(
		"openai:gpt-4o",
		policySanitizerProvider{resources: []unifiedresources.Resource{resource}},
		AllowResourcePolicyText(allowedInventoryContext),
	)
	if sanitizer == nil {
		t.Fatal("expected external model sanitizer")
	}

	got := sanitizer(providers.ChatRequest{
		Messages: []providers.Message{{
			Role:    "user",
			Content: allowedInventoryContext + "\n\n---\nUser message: vm1 should still be governed in free text",
		}},
	})
	content := got.Messages[0].Content
	if !strings.Contains(content, `"answer_label":"VM 100 vm1"`) {
		t.Fatalf("Pulse-generated inventory label was redacted: %q", content)
	}
	if strings.Contains(content, "User message: vm1 should still be governed") {
		t.Fatalf("user-authored resource identity bypassed sanitizer: %q", content)
	}
	if !strings.Contains(content, "User message: redacted by policy should still be governed") {
		t.Fatalf("user-authored resource identity was not redacted: %q", content)
	}
}

func TestRequestSanitizerForModel_FloorsLocalOnlyAndStripsSecrets(t *testing.T) {
	// Pulse shares real infrastructure detail with cloud models, with two always-on
	// invariants: a Sensitive (local-first) resource's identifiers flow, a Restricted
	// (local-only) resource stays redacted as the hard floor, and credentials are
	// always stripped.
	sensitiveVM := unifiedresources.Resource{
		ID:   "vm-200",
		Type: unifiedresources.ResourceTypeVM,
		Name: "finance-vm",
		Tags: []string{"sensitive"}, // -> Sensitive -> local-first (flows)
		Identity: unifiedresources.ResourceIdentity{
			Hostnames:   []string{"finance-vm.lan"},
			IPAddresses: []string{"10.0.0.7"},
		},
		Proxmox: &unifiedresources.ProxmoxData{VMID: 200, NodeName: "node1"},
	}
	restrictedAgent := unifiedresources.Resource{
		ID:   "agent/vault",
		Type: unifiedresources.ResourceTypeAgent,
		Name: "vault",
		Tags: []string{"secret"}, // -> Restricted -> local-only (hard floor)
		Identity: unifiedresources.ResourceIdentity{
			Hostnames:   []string{"vault.lan"},
			IPAddresses: []string{"10.0.0.9"},
		},
	}
	provider := policySanitizerProvider{resources: []unifiedresources.Resource{sensitiveVM, restrictedAgent}}

	req := providers.ChatRequest{
		System: "finance-vm.lan is 10.0.0.7; vault.lan is 10.0.0.9. Authorization: Bearer sk-leaked-secret-token",
	}

	out := RequestSanitizerForModel("openai:gpt-4o", provider)(req)
	// The Sensitive resource's identifiers flow.
	for _, identifier := range []string{"finance-vm.lan", "10.0.0.7"} {
		if !strings.Contains(out.System, identifier) {
			t.Fatalf("sanitizer must preserve the sensitive (non-local-only) identifier %q, got: %s", identifier, out.System)
		}
	}
	// The Restricted (local-only) resource stays redacted as the hard floor.
	for _, identifier := range []string{"vault.lan", "10.0.0.9"} {
		if strings.Contains(out.System, identifier) {
			t.Fatalf("sanitizer must keep the local-only floor for %q, got: %s", identifier, out.System)
		}
	}
	// The credential is always redacted.
	if strings.Contains(out.System, "sk-leaked-secret-token") {
		t.Fatalf("sanitizer must redact the bearer token, got: %s", out.System)
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
