package chat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestResolvedResource_GettersAndAliases(t *testing.T) {
	res := &ResolvedResource{
		ResourceID:     "vm:node1:101",
		ResourceType:   "vm",
		TargetHost:     "alpha",
		AgentID:        "agent-1",
		Adapter:        "qm",
		VMID:           101,
		Node:           "node1",
		AllowedActions: []string{"start"},
		ProviderUID:    "101",
		Kind:           "vm",
		Aliases:        []string{"alpha", "Alpha-VM"},
		Name:           "alpha",
	}

	if res.GetResourceID() != "vm:node1:101" || res.GetResourceType() != "vm" {
		t.Fatalf("expected getters to return structured fields")
	}
	if res.GetTargetHost() != "alpha" || res.GetAgentID() != "agent-1" || res.GetAdapter() != "qm" {
		t.Fatalf("expected routing getters to return values")
	}
	if res.GetVMID() != 101 || res.GetNode() != "node1" {
		t.Fatalf("expected VMID/node getters to return values")
	}
	if len(res.GetAllowedActions()) != 1 || res.GetAllowedActions()[0] != "start" {
		t.Fatalf("expected allowed actions getter to return actions")
	}
	if len(res.GetAliases()) != 2 {
		t.Fatalf("expected aliases getter to return values")
	}
	if !res.HasAlias("alpha-vm") {
		t.Fatalf("expected HasAlias to be case-insensitive")
	}
}

func TestToolCallProviderProjectionUsesSharedAgentCapabilitiesShape(t *testing.T) {
	success := true
	runtimeCall := ToolCall{
		ID:               "call-1",
		Name:             "diagnose",
		Output:           "in-app output",
		Success:          &success,
		ThoughtSignature: json.RawMessage(`{"provider":"gemini"}`),
	}

	projected := runtimeCall.ProviderToolCall()
	var shared agentcapabilities.ProviderToolCall = projected
	if shared.ID != "call-1" || shared.Name != "diagnose" {
		t.Fatalf("shared provider call = %+v", shared)
	}
	if shared.Input == nil {
		t.Fatal("shared provider call input must normalize to an empty object")
	}

	payload, err := json.Marshal(projected)
	if err != nil {
		t.Fatalf("marshal provider projection: %v", err)
	}
	text := string(payload)
	if !strings.Contains(text, `"input":{}`) {
		t.Fatalf("provider projection must retain empty input object, got %s", text)
	}
	if !strings.Contains(text, `"thought_signature":{"provider":"gemini"}`) {
		t.Fatalf("provider projection must retain provider continuation signature, got %s", text)
	}
	if strings.Contains(text, `"output"`) || strings.Contains(text, `"success"`) {
		t.Fatalf("provider projection leaked in-app transcript fields: %s", text)
	}

	roundTrip := ToolCallFromProvider(shared)
	if roundTrip.ID != "call-1" || roundTrip.Name != "diagnose" || roundTrip.Input == nil {
		t.Fatalf("stored runtime call from provider shape = %+v", roundTrip)
	}
}

func TestToolCallProviderProjectionDoesNotAliasInputs(t *testing.T) {
	input := map[string]interface{}{"resource_id": "vm/100"}
	runtimeCall := ToolCall{
		ID:    "call-1",
		Name:  "diagnose",
		Input: input,
	}
	projected := runtimeCall.ProviderToolCall()
	projected.Input["resource_id"] = "vm/101"
	if input["resource_id"] != "vm/100" {
		t.Fatalf("provider projection aliased transcript input: source=%#v projected=%#v", input, projected.Input)
	}

	providerInput := map[string]interface{}{"resource_id": "vm/200"}
	roundTrip := ToolCallFromProvider(agentcapabilities.ProviderToolCall{
		ID:    "call-2",
		Name:  "diagnose",
		Input: providerInput,
	})
	roundTrip.Input["resource_id"] = "vm/201"
	if providerInput["resource_id"] != "vm/200" {
		t.Fatalf("provider-to-transcript projection aliased provider input: source=%#v roundTrip=%#v", providerInput, roundTrip.Input)
	}
}

func TestMessageNormalizeCollectionsDoesNotAliasToolCallState(t *testing.T) {
	success := true
	sourceInput := map[string]interface{}{
		"body": map[string]interface{}{
			"resource_id": "vm/100",
		},
		"items": []interface{}{
			map[string]interface{}{"name": "alpha"},
		},
	}
	sourceSignature := json.RawMessage(`{"provider":"gemini"}`)
	sourceResult := agentcapabilities.NewProviderToolResult("call-1", "ok", false)
	source := Message{
		ToolCalls: []ToolCall{{
			ID:               "call-1",
			Name:             "diagnose",
			Input:            sourceInput,
			Success:          &success,
			ThoughtSignature: sourceSignature,
		}},
		ToolResult: &sourceResult,
	}

	normalized := source.NormalizeCollections()
	normalized.ToolCalls[0].Name = "changed"
	normalized.ToolCalls[0].Input["body"].(map[string]interface{})["resource_id"] = "vm/101"
	normalized.ToolCalls[0].Input["items"].([]interface{})[0].(map[string]interface{})["name"] = "bravo"
	normalized.ToolCalls[0].ThoughtSignature[0] = '['
	*normalized.ToolCalls[0].Success = false
	normalized.ToolResult.Content = "changed"

	if source.ToolCalls[0].Name != "diagnose" {
		t.Fatalf("message normalization aliased tool-call slice: source=%#v normalized=%#v", source.ToolCalls, normalized.ToolCalls)
	}
	if got := sourceInput["body"].(map[string]interface{})["resource_id"]; got != "vm/100" {
		t.Fatalf("message normalization aliased nested tool input: got %v", got)
	}
	if got := sourceInput["items"].([]interface{})[0].(map[string]interface{})["name"]; got != "alpha" {
		t.Fatalf("message normalization aliased nested tool input slice: got %v", got)
	}
	if string(sourceSignature) != `{"provider":"gemini"}` {
		t.Fatalf("message normalization aliased thought signature: %s", sourceSignature)
	}
	if !success {
		t.Fatalf("message normalization aliased success pointer")
	}
	if sourceResult.Content != "ok" {
		t.Fatalf("message normalization aliased tool result pointer: %+v", sourceResult)
	}
}

func TestResolvedResource_GetBestExecutor(t *testing.T) {
	res := &ResolvedResource{
		ReachableVia: []ExecutorPath{
			{ExecutorID: "a", Actions: []string{"restart"}, Priority: 1},
			{ExecutorID: "b", Actions: []string{"*"}, Priority: 0},
		},
	}

	best := res.GetBestExecutor("restart")
	if best == nil || best.ExecutorID != "a" {
		t.Fatalf("expected highest priority executor for action")
	}

	best = res.GetBestExecutor("stop")
	if best == nil || best.ExecutorID != "b" {
		t.Fatalf("expected wildcard executor for action")
	}
}

func TestResolvedContext_AccessAndValidation(t *testing.T) {
	ctx := NewResolvedContext("session-1")
	if ctx.HasAnyResources() {
		t.Fatalf("expected empty context")
	}

	res := &ResolvedResource{
		ResourceID:     "vm:node1:101",
		ResourceType:   "vm",
		Name:           "alpha",
		AllowedActions: []string{"start"},
	}
	ctx.AddResource("alpha", res)
	if !ctx.HasAnyResources() {
		t.Fatalf("expected resources to exist")
	}

	if _, ok := ctx.GetResourceByID(res.ResourceID); !ok {
		t.Fatalf("expected GetResourceByID to find resource")
	}

	if _, err := ctx.ValidateResourceID("missing"); err == nil {
		t.Fatalf("expected resource not resolved error")
	}

	if err := ctx.ValidateAction(res.ResourceID, "start"); err != nil {
		t.Fatalf("expected action allowed")
	}
	if err := ctx.ValidateAction(res.ResourceID, "stop"); err == nil {
		t.Fatalf("expected action not allowed")
	}

	if _, err := ctx.ValidateResourceForAction(res.ResourceID, "start"); err != nil {
		t.Fatalf("expected ValidateResourceForAction to succeed: %v", err)
	}
}

func TestResolvedContext_ExplicitAccess(t *testing.T) {
	ctx := NewResolvedContext("session-1")
	res := &ResolvedResource{ResourceID: "node:node1", Name: "node1"}
	ctx.AddResourceWithExplicitAccess(res.Name, res)

	recent := ctx.GetRecentlyAccessedResourcesSorted(5*time.Minute, 10)
	if len(recent) != 1 || recent[0] != res.ResourceID {
		t.Fatalf("expected recently accessed resource to be tracked")
	}
}

func TestResolvedContextErrors(t *testing.T) {
	resErr := (&ResourceNotResolvedError{ResourceID: "missing"}).Error()
	if !strings.Contains(resErr, "missing") {
		t.Fatalf("expected resource error to include ID")
	}

	actionErr := (&ActionNotAllowedError{ResourceID: "vm:1", Action: "stop"}).Error()
	if !strings.Contains(actionErr, "stop") {
		t.Fatalf("expected action error to include action")
	}
}
