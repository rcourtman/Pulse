package qualification

import (
	"strings"
	"testing"
)

func TestReplaySessionRequiresExactOrderedCanonicalCalls(t *testing.T) {
	report := RunReport{
		RunID: "q-test-replay", Manifest: validTestManifest(),
		Environment: Environment{Model: "provider:model"},
		PatrolRun: PatrolRun{ToolCalls: []ToolCall{
			{ID: "one", ToolName: "get_resource", Input: `{"b":2,"a":1}`, Output: `{"status":"ok"}`, Success: true},
			{ID: "two", ToolName: "get_logs", Input: `{"resource_id":"r1"}`, Output: `[]`, Success: true},
		}},
	}
	bundle, err := BuildReplayBundle(report)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Exchanges[0].CanonicalInput != `{"a":1,"b":2}` {
		t.Fatalf("canonical input = %s", bundle.Exchanges[0].CanonicalInput)
	}
	session, err := NewReplaySession(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Call("get_resource", `{ "a": 1, "b": 2 }`); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Call("wrong_tool", `{"resource_id":"r1"}`); err == nil {
		t.Fatal("expected reordered or renamed tool call to fail")
	}
	if _, err := session.Call("get_logs", `{"resource_id":"r1"}`); err != nil {
		t.Fatal(err)
	}
	if err := session.Complete(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildReplayBundleMarksMalformedCapturedInputNonReplayable(t *testing.T) {
	report := RunReport{
		RunID: "q-truncated-input", Manifest: validTestManifest(),
		PatrolRun: PatrolRun{ToolCalls: []ToolCall{{
			ID: "truncated", ToolName: "patrol_report_finding", Input: `{"description":"cut...`, Success: true,
		}}},
	}
	bundle, err := BuildReplayBundle(report)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Replayable || len(bundle.ReplayIssues) != 1 {
		t.Fatalf("malformed capture must be explicit and non-replayable: %+v", bundle)
	}
	if !strings.Contains(bundle.Exchanges[0].CanonicalInput, "captured_raw_input") {
		t.Fatalf("opaque captured bytes not preserved: %+v", bundle.Exchanges[0])
	}
	if _, err := NewReplaySession(bundle); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected deterministic replay rejection, got %v", err)
	}
}

func TestReplaySessionRejectsMissingCall(t *testing.T) {
	session, err := NewReplaySession(ReplayBundle{SchemaVersion: ReplaySchemaVersion, Exchanges: []ToolExchange{{Sequence: 1, ToolName: "get_resource", CanonicalInput: `{}`}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := session.Complete(); err == nil {
		t.Fatal("expected incomplete transcript to fail")
	}
}
