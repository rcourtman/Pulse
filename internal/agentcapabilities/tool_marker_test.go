package agentcapabilities

import (
	"strings"
	"testing"
)

func TestApprovalRequiredToolMarkerFormatsSharedPayload(t *testing.T) {
	marker := ApprovalRequiredToolMarker("qm start 101", "tool-1", "approval required", "approval-1", "Click approve.")
	if !strings.HasPrefix(marker, ToolMarkerApprovalRequiredPrefix+" ") {
		t.Fatalf("marker prefix = %q", marker)
	}
	payload, ok := ParseApprovalRequiredToolMarkerPayload(marker)
	if !ok {
		t.Fatalf("ParseApprovalRequiredToolMarkerPayload failed for %s", marker)
	}
	for key, want := range map[string]any{
		"type":           ToolMarkerApprovalRequiredType,
		"command":        "qm start 101",
		"tool_id":        "tool-1",
		"reason":         "approval required",
		"approval_id":    "approval-1",
		"how_to_approve": "Click approve.",
		"do_not_retry":   true,
	} {
		if got := payload[key]; got != want {
			t.Fatalf("payload[%s] = %#v, want %#v in %#v", key, got, want, payload)
		}
	}
}

func TestFormatApprovalRequiredToolMarkerPreservesCallerFields(t *testing.T) {
	marker := FormatApprovalRequiredToolMarker(map[string]any{
		"approval_id": "approval-2",
		"command":     "docker restart web",
		"risk":        "high",
		"plan": map[string]any{
			"action_id": "act-1",
		},
	})
	payload, ok := ParseApprovalRequiredToolMarkerPayload(marker)
	if !ok {
		t.Fatalf("ParseApprovalRequiredToolMarkerPayload failed for %s", marker)
	}
	if payload["risk"] != "high" {
		t.Fatalf("risk = %#v, want high", payload["risk"])
	}
	plan, ok := payload["plan"].(map[string]any)
	if !ok || plan["action_id"] != "act-1" {
		t.Fatalf("plan = %#v, want action_id", payload["plan"])
	}
	if payload["type"] != ToolMarkerApprovalRequiredType || payload["do_not_retry"] != true {
		t.Fatalf("shared fields missing: %#v", payload)
	}
}

func TestParseApprovalRequiredToolMarkerDataUsesSharedTypedPayload(t *testing.T) {
	marker := FormatApprovalRequiredToolMarker(map[string]any{
		"approval_id":    "approval-3",
		"command":        "kubectl rollout restart deployment/api",
		"risk":           "medium",
		"reason":         "restart requested",
		"cluster":        "prod-cluster",
		"target_type":    "kubernetes-deployment",
		"target_id":      "prod/api",
		"audit_id":       "act-1",
		"do_not_retry":   false,
		"how_to_approve": "Approve in chat.",
		"plan": map[string]any{
			"action_id":          "act-1",
			"request_id":         "req-1",
			"summary":            "Restart api",
			"requires_approval":  true,
			"approval_policy":    "operator",
			"blast_radius":       "api pods",
			"rollback_available": true,
			"plan_hash":          "hash-1",
			"expires_at":         "2026-06-18T06:30:00Z",
		},
		"context_confidence": map[string]any{
			"level":    "high",
			"summary":  "resource explicitly selected",
			"evidence": []string{"@api"},
		},
		"preflight": map[string]any{
			"target":             "deployment/api",
			"current_state":      "available",
			"intended_change":    "restart pods",
			"dry_run_available":  true,
			"dry_run_summary":    "server-side dry run passed",
			"safety_checks":      []string{"within maintenance window"},
			"verification_steps": []string{"watch rollout"},
			"generated_at":       "2026-06-18T06:20:00Z",
		},
	})

	payload, ok := ParseApprovalRequiredToolMarkerData(marker)
	if !ok {
		t.Fatalf("ParseApprovalRequiredToolMarkerData failed for %s", marker)
	}
	if payload.Type != ToolMarkerApprovalRequiredType || !payload.DoNotRetry {
		t.Fatalf("shared marker fields = type:%q do_not_retry:%v", payload.Type, payload.DoNotRetry)
	}
	if payload.ApprovalID != "approval-3" || payload.Command != "kubectl rollout restart deployment/api" {
		t.Fatalf("payload identity fields = %+v", payload)
	}
	if payload.TargetHint() != "prod-cluster" {
		t.Fatalf("TargetHint = %q, want prod-cluster", payload.TargetHint())
	}
	if payload.DescriptionText() != "restart requested" {
		t.Fatalf("DescriptionText = %q, want reason fallback", payload.DescriptionText())
	}
	if payload.Plan == nil || payload.Plan.ActionID != "act-1" || !payload.Plan.RollbackAvailable {
		t.Fatalf("plan payload = %+v", payload.Plan)
	}
	if payload.ContextConfidence == nil || payload.ContextConfidence.Level != "high" || len(payload.ContextConfidence.Evidence) != 1 {
		t.Fatalf("context confidence payload = %+v", payload.ContextConfidence)
	}
	if payload.Preflight == nil || payload.Preflight.Target != "deployment/api" || len(payload.Preflight.VerificationSteps) != 1 {
		t.Fatalf("preflight payload = %+v", payload.Preflight)
	}
}

func TestPolicyBlockedToolMarkerFormatsSharedPayload(t *testing.T) {
	marker := PolicyBlockedToolMarker("rm -rf /", "blocked by policy")
	if !HasPolicyBlockedToolMarker(marker) {
		t.Fatalf("HasPolicyBlockedToolMarker(%q) = false", marker)
	}
	payload, ok := ParsePolicyBlockedToolMarkerPayload(marker)
	if !ok {
		t.Fatalf("ParsePolicyBlockedToolMarkerPayload failed for %s", marker)
	}
	for key, want := range map[string]any{
		"type":         ToolMarkerPolicyBlockedType,
		"command":      "rm -rf /",
		"reason":       "blocked by policy",
		"do_not_retry": true,
	} {
		if got := payload[key]; got != want {
			t.Fatalf("payload[%s] = %#v, want %#v in %#v", key, got, want, payload)
		}
	}
}

func TestToolMarkerPayloadJSONAcceptsLegacyNoSpaceForm(t *testing.T) {
	raw, ok := ApprovalRequiredToolMarkerPayloadJSON(`APPROVAL_REQUIRED:{"type":"approval_required","command":"uptime"}`)
	if !ok {
		t.Fatal("ApprovalRequiredToolMarkerPayloadJSON rejected no-space marker")
	}
	if string(raw) != `{"type":"approval_required","command":"uptime"}` {
		t.Fatalf("raw = %s", raw)
	}
}

func TestParseToolMarkerPayloadRejectsWrongShape(t *testing.T) {
	if _, ok := ParseApprovalRequiredToolMarkerPayload("not a marker"); ok {
		t.Fatal("parsed non-marker")
	}
	if _, ok := ParseApprovalRequiredToolMarkerPayload("APPROVAL_REQUIRED: not-json"); ok {
		t.Fatal("parsed non-JSON marker")
	}
	if _, ok := ParseApprovalRequiredToolMarkerPayload(`APPROVAL_REQUIRED: {"type":"policy_blocked"}`); ok {
		t.Fatal("parsed marker with wrong type")
	}
	if _, ok := ParsePolicyBlockedToolMarkerPayload(`APPROVAL_REQUIRED: {"type":"approval_required"}`); ok {
		t.Fatal("parsed wrong marker prefix")
	}
}
