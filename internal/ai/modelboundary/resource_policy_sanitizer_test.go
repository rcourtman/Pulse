package modelboundary

import (
	"reflect"
	"strconv"
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

func TestRequestSanitizerForModelKeepsModelBoundResourceIDs(t *testing.T) {
	// Anyone who may create a Secret in a monitored namespace chooses its name.
	// That name must not take the ID of an unrelated host out of provider-bound
	// text, neither as a part of the ID nor as the whole of it. A name that the
	// host shares with a local-only resource stays redacted.
	host := unifiedresources.Resource{
		ID:   "agent-89b9f14dfbe851b7",
		Type: unifiedresources.ResourceTypeAgent,
		Name: "production",
	}
	secret := func(name string) unifiedresources.Resource {
		return unifiedresources.Resource{
			ID:         "k8s-secret-" + name,
			Type:       unifiedresources.ResourceTypeK8sSecret,
			Name:       name,
			Identity:   unifiedresources.ResourceIdentity{Hostnames: []string{name, "team-a/" + name}},
			Kubernetes: &unifiedresources.K8sData{Namespace: "team-a"},
		}
	}
	provider := policySanitizerProvider{resources: []unifiedresources.Resource{
		host, secret("production"), secret("agent"), secret("agent-89b9f14dfbe851b7"), secret("registry-password"),
	}}

	req := providers.ChatRequest{
		System: "Resolved requested identity aliases: agent-89b9f14dfbe851b7, production-1\n| production | 0 | 0 | 0 |",
		Messages: []providers.Message{{
			Role: "assistant",
			ToolCalls: []providers.ToolCall{{
				ID:    "tool-1",
				Input: map[string]interface{}{"host": "agent-89b9f14dfbe851b7"},
			}},
		}, {
			Role:       "tool",
			ToolResult: &providers.ToolResult{ToolUseID: "tool-1", Content: `{"host":"production","secrets":["team-a/production","registry-password"]}`},
		}},
	}

	out := RequestSanitizerForModel("openai:gpt-4o", provider)(req)
	label := unifiedresources.ResourcePolicyRedactedLabel
	if want := "Resolved requested identity aliases: agent-89b9f14dfbe851b7, production-1\n| " + label + " | 0 | 0 | 0 |"; out.System != want {
		t.Fatalf("system prompt = %q, want %q", out.System, want)
	}
	if got := out.Messages[0].ToolCalls[0].Input["host"]; got != "agent-89b9f14dfbe851b7" {
		t.Fatalf("model-bound host ID was redacted in tool input: %q", got)
	}
	want := `{"host":"` + label + `","secrets":["` + label + `","` + label + `"]}`
	if got := out.Messages[1].ToolResult.Content; got != want {
		t.Fatalf("tool result = %q, want %q", got, want)
	}
}

func TestRequestSanitizerForModelRedactsEachIdentifierOnce(t *testing.T) {
	// The redaction label contains "redacted" and "policy". Local-only
	// resources with those names must not keep replacing the words of the
	// labels inserted for the resources before them.
	var resources []unifiedresources.Resource
	for i := 0; i < 24; i++ {
		resources = append(resources, unifiedresources.Resource{
			ID:   "k8s-secret-" + strconv.Itoa(i),
			Type: unifiedresources.ResourceTypeK8sSecret,
			Name: []string{"redacted", "policy"}[i%2],
		})
	}

	req := providers.ChatRequest{System: "Apply the retention policy to k8s-secret-7."}
	out := RequestSanitizerForModel("openai:gpt-4o", policySanitizerProvider{resources: resources})(req)
	want := "Apply the retention " + unifiedresources.ResourcePolicyRedactedLabel + " to " + unifiedresources.ResourcePolicyRedactedLabel + "."
	if out.System != want {
		t.Fatalf("system prompt is %d bytes, want %q", len(out.System), want)
	}
}

func TestRequestSanitizerForModelKeepsToolSchemaKeywords(t *testing.T) {
	var resources []unifiedresources.Resource
	for _, name := range []string{"object", "string", "array", "action", "vault"} {
		resources = append(resources, unifiedresources.Resource{
			ID:   "k8s-secret-" + name,
			Type: unifiedresources.ResourceTypeK8sSecret,
			Name: name,
		})
	}

	req := providers.ChatRequest{Tools: []providers.Tool{{
		Name:        "pulse_docker",
		Description: "Run an action on vault",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{"type": "string", "enum": []string{"services", "vault"}},
				"type":   map[string]interface{}{"type": []interface{}{"string", "null"}, "description": "Type of vault"},
				"hosts":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"filter": map[string]interface{}{"anyOf": []interface{}{map[string]interface{}{"type": "object", "required": []interface{}{"action"}}}},
			},
			"required": []string{"action"},
		},
	}}}

	got := RequestSanitizerForModel("openai:gpt-4o", policySanitizerProvider{resources: resources})(req).Tools[0]
	label := unifiedresources.ResourcePolicyRedactedLabel
	want := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{"type": "string", "enum": []string{"services", label}},
			"type":   map[string]interface{}{"type": []interface{}{"string", "null"}, "description": "Type of " + label},
			"hosts":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			"filter": map[string]interface{}{"anyOf": []interface{}{map[string]interface{}{"type": "object", "required": []interface{}{"action"}}}},
		},
		"required": []string{"action"},
	}
	if !reflect.DeepEqual(got.InputSchema, want) {
		t.Fatalf("tool schema = %#v, want %#v", got.InputSchema, want)
	}
	if want := "Run an " + label + " on " + label; got.Description != want {
		t.Fatalf("tool description = %q, want %q", got.Description, want)
	}
}
