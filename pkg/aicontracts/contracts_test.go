package aicontracts

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
)

func TestEmptyOrchestratorMessage_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyOrchestratorMessage())
	if err != nil {
		t.Fatalf("marshal empty orchestrator message: %v", err)
	}
	if !strings.Contains(string(payload), `"tool_calls":[]`) {
		t.Fatalf("expected empty orchestrator message to retain tool_calls array, got %s", payload)
	}
}

func TestEmptyOrchestratorToolCallInfo_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyOrchestratorToolCallInfo())
	if err != nil {
		t.Fatalf("marshal empty orchestrator tool call info: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected empty orchestrator tool call info to retain input object, got %s", payload)
	}

	payload, err = json.Marshal(OrchestratorMessage{
		ToolCalls: []OrchestratorToolCallInfo{{
			ID:   "call-1",
			Name: "diagnose",
		}},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("marshal normalized orchestrator message with tool call: %v", err)
	}
	if !strings.Contains(string(payload), `"input":{}`) {
		t.Fatalf("expected normalized orchestrator tool call to retain input object, got %s", payload)
	}
}

func TestOrchestratorToolCallInfoUsesSharedProviderProjection(t *testing.T) {
	input := map[string]interface{}{
		"resource_id": "vm/100",
		"body": map[string]interface{}{
			"tags": []interface{}{"prod"},
		},
	}
	signature := json.RawMessage(`{"provider":"gemini"}`)

	info := OrchestratorToolCallInfoFromProvider(agentcapabilities.ProviderToolCall{
		ID:               "call-1",
		Name:             "diagnose",
		Input:            input,
		ThoughtSignature: signature,
	})
	if info.ID != "call-1" || info.Name != "diagnose" || info.Input == nil {
		t.Fatalf("orchestrator tool call info = %+v", info)
	}
	info.Input["resource_id"] = "vm/101"
	info.Input["body"].(map[string]interface{})["tags"].([]interface{})[0] = "changed"
	info.ThoughtSignature[0] = '['
	if input["resource_id"] != "vm/100" || input["body"].(map[string]interface{})["tags"].([]interface{})[0] != "prod" {
		t.Fatalf("orchestrator projection must not alias provider input: source=%#v info=%#v", input, info.Input)
	}
	if string(signature) != `{"provider":"gemini"}` {
		t.Fatalf("orchestrator projection must not alias provider thought signature: source=%s info=%s", signature, info.ThoughtSignature)
	}

	orchestratorInput := map[string]interface{}{"resource_id": "vm/300"}
	infoForProvider := OrchestratorToolCallInfo{
		ID:               "call-2",
		Name:             "inspect",
		Input:            orchestratorInput,
		ThoughtSignature: json.RawMessage(`{"provider":"gemini"}`),
	}.NormalizeCollections()
	provider := infoForProvider.ProviderToolCall()
	provider.Input["resource_id"] = "vm/102"
	provider.ThoughtSignature[0] = '['
	if infoForProvider.Input["resource_id"] != "vm/300" || string(infoForProvider.ThoughtSignature) != `{"provider":"gemini"}` {
		t.Fatalf("provider projection must not alias orchestrator tool call info: info=%#v provider=%#v", infoForProvider, provider)
	}
}

func TestOrchestratorToolResultInfoUsesSharedProviderProjection(t *testing.T) {
	info := OrchestratorToolResultInfoFromProvider(agentcapabilities.NewProviderToolResult("call-1", "done", true))
	if info.ToolUseID != "call-1" || info.Content != "done" || !info.IsError {
		t.Fatalf("orchestrator tool result info = %+v", info)
	}

	var shared agentcapabilities.ProviderToolResult = info.ProviderToolResult()
	if shared.ToolUseID != "call-1" || shared.Content != "done" || !shared.IsError {
		t.Fatalf("shared provider tool result = %+v", shared)
	}
}

func TestEmptyInvestigationSession_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyInvestigationSession())
	if err != nil {
		t.Fatalf("marshal empty investigation session: %v", err)
	}
	if !strings.Contains(string(payload), `"tools_available":[]`) {
		t.Fatalf("expected empty investigation session to retain tools_available array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"tools_used":[]`) {
		t.Fatalf("expected empty investigation session to retain tools_used array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"evidence_ids":[]`) {
		t.Fatalf("expected empty investigation session to retain evidence_ids array, got %s", payload)
	}
}

func TestEmptyInvestigationRecord_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyInvestigationRecord())
	if err != nil {
		t.Fatalf("marshal empty investigation record: %v", err)
	}
	if !strings.Contains(string(payload), `"evidence":[]`) {
		t.Fatalf("expected empty investigation record to retain evidence array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"verification":[]`) {
		t.Fatalf("expected empty investigation record to retain verification array, got %s", payload)
	}
	if !strings.Contains(string(payload), `"tools_used":[]`) {
		t.Fatalf("expected empty investigation record to retain tools_used array, got %s", payload)
	}
}

func TestInvestigationOutcomeFixRejected_IsCanonicalWireValue(t *testing.T) {
	payload, err := json.Marshal(struct {
		Outcome InvestigationOutcome `json:"outcome"`
	}{
		Outcome: OutcomeFixRejected,
	})
	if err != nil {
		t.Fatalf("marshal rejected investigation outcome: %v", err)
	}
	if !strings.Contains(string(payload), `"outcome":"fix_rejected"`) {
		t.Fatalf("expected rejected investigation outcome wire value, got %s", payload)
	}
}

func TestEmptyFix_UsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(EmptyFix())
	if err != nil {
		t.Fatalf("marshal empty fix: %v", err)
	}
	if !strings.Contains(string(payload), `"commands":[]`) {
		t.Fatalf("expected empty fix to retain commands array, got %s", payload)
	}
}

func TestActionProposalWireShapeIsTypedAndCommandFree(t *testing.T) {
	payload, err := json.Marshal(ActionProposal{
		ProposalID:      "prop-1",
		FindingID:       "finding-1",
		InvestigationID: "inv-1",
		ResourceID:      "vm:42",
		CapabilityName:  "restart",
		Params:          map[string]any{"mode": "graceful"},
		Reason:          "recover",
	})
	if err != nil {
		t.Fatalf("marshal action proposal: %v", err)
	}
	for _, want := range []string{`"proposal_id":"prop-1"`, `"resource_id":"vm:42"`, `"capability_name":"restart"`} {
		if !strings.Contains(string(payload), want) {
			t.Fatalf("proposal wire shape missing %s: %s", want, payload)
		}
	}
	// The proposal contract must stay command-free: no command, target
	// host, risk, destructive, or approval fields exist for enterprise
	// code to smuggle authority through.
	for _, forbidden := range []string{"command", "target_host", "risk", "destructive", "approval", "autonomy"} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("proposal wire shape must not carry %q: %s", forbidden, payload)
		}
	}
}

func TestActionReferenceIsAdditiveOnInvestigationShapes(t *testing.T) {
	session := EmptyInvestigationSession()
	session.Action = &ActionReference{
		ActionID:       "act-1",
		ResourceID:     "vm:42",
		CapabilityName: "restart",
		State:          "pending_approval",
	}
	payload, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("marshal investigation session with action reference: %v", err)
	}
	if !strings.Contains(string(payload), `"action":{`) || !strings.Contains(string(payload), `"action_id":"act-1"`) {
		t.Fatalf("investigation session must carry the action reference, got %s", payload)
	}

	record := EmptyInvestigationRecord()
	record.Action = &ActionReference{ActionID: "act-1", ResourceID: "vm:42", CapabilityName: "restart", State: "planned"}
	recordPayload, err := json.Marshal(record)
	if err != nil {
		t.Fatalf("marshal investigation record with action reference: %v", err)
	}
	if !strings.Contains(string(recordPayload), `"action_id":"act-1"`) {
		t.Fatalf("investigation record must carry the action reference, got %s", recordPayload)
	}

	// Absent references stay absent: legacy readers must not see a new
	// non-optional field.
	emptyPayload, err := json.Marshal(EmptyInvestigationSession())
	if err != nil {
		t.Fatalf("marshal empty investigation session: %v", err)
	}
	if strings.Contains(string(emptyPayload), `"action"`) {
		t.Fatalf("empty investigation session must omit the action reference, got %s", emptyPayload)
	}
}

func TestOrchestratorActionBrokerIsProposeOnly(t *testing.T) {
	// The broker seam must never grow decision or execution authority:
	// enterprise investigation code proposes, the canonical core
	// lifecycle decides and executes. Guard the method set so a Decide
	// or Execute method cannot be added without failing this proof.
	brokerType := reflect.TypeOf((*OrchestratorActionBroker)(nil)).Elem()
	if brokerType.NumMethod() != 2 {
		t.Fatalf("OrchestratorActionBroker must stay propose-only (Capabilities, Submit), got %d methods", brokerType.NumMethod())
	}
	for _, name := range []string{"Capabilities", "Submit"} {
		if _, ok := brokerType.MethodByName(name); !ok {
			t.Fatalf("OrchestratorActionBroker missing method %s", name)
		}
	}
	for _, forbidden := range []string{"Decide", "Execute", "Approve", "ExecuteCommand"} {
		if _, ok := brokerType.MethodByName(forbidden); ok {
			t.Fatalf("OrchestratorActionBroker must not expose %s", forbidden)
		}
	}
}

func TestOrchestratorDepsExposesTypedActionBroker(t *testing.T) {
	depsType := reflect.TypeOf(OrchestratorDeps{})
	field, ok := depsType.FieldByName("ActionBroker")
	if !ok {
		t.Fatal("OrchestratorDeps must expose the typed ActionBroker seam")
	}
	if field.Type != reflect.TypeOf((*OrchestratorActionBroker)(nil)).Elem() {
		t.Fatalf("ActionBroker field type = %v, want OrchestratorActionBroker", field.Type)
	}
}

func TestOrchestratorChatServiceIsInvestigationOnly(t *testing.T) {
	svcType := reflect.TypeOf((*OrchestratorChatService)(nil)).Elem()
	// The generic-execution and autonomy surface is gone: no method may
	// carry autonomy, generic execution, or a broad tool listing.
	for _, forbidden := range []string{"ExecuteStream", "SetAutonomousMode", "ListAvailableTools"} {
		if _, ok := svcType.MethodByName(forbidden); ok {
			t.Fatalf("OrchestratorChatService must not expose %s", forbidden)
		}
	}
	for _, required := range []string{"ExecuteInvestigationStream", "ListInvestigationTools"} {
		if _, ok := svcType.MethodByName(required); !ok {
			t.Fatalf("OrchestratorChatService must expose %s", required)
		}
	}
	// The investigation request must carry no autonomy field.
	reqType := reflect.TypeOf(OrchestratorInvestigationRequest{})
	for i := 0; i < reqType.NumField(); i++ {
		name := strings.ToLower(reqType.Field(i).Name)
		if strings.Contains(name, "autonom") {
			t.Fatalf("investigation request must carry no autonomy field, found %s", reqType.Field(i).Name)
		}
	}
}

func TestOrchestratorDepsHasNoCommandOrAutonomyDeps(t *testing.T) {
	depsType := reflect.TypeOf(OrchestratorDeps{})
	for _, forbidden := range []string{"CmdExecutor", "ApprovalStore", "Autonomy", "FixVerifier", "License"} {
		if _, ok := depsType.FieldByName(forbidden); ok {
			t.Fatalf("OrchestratorDeps must not expose %s after the typed-lifecycle migration", forbidden)
		}
	}
}

func TestActionCapabilityParamInfoCarriesPattern(t *testing.T) {
	// Canonical planner validation enforces Pattern; the cross-repo
	// capability projection must carry it or proposal validation parity
	// dies at the boundary.
	if _, ok := reflect.TypeOf(ActionCapabilityParamInfo{}).FieldByName("Pattern"); !ok {
		t.Fatal("ActionCapabilityParamInfo must carry Pattern")
	}
}

func TestInvestigationResultCarriesStructuredProposal(t *testing.T) {
	resultType := reflect.TypeOf(OrchestratorInvestigationResult{})
	field, ok := resultType.FieldByName("Proposal")
	if !ok {
		t.Fatal("investigation result must carry a structured Proposal")
	}
	if field.Type != reflect.TypeOf((*ActionProposal)(nil)) {
		t.Fatalf("Proposal field type = %v, want *ActionProposal", field.Type)
	}
}
