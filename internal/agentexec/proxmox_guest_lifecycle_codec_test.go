package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

func boundProxmoxGuestLifecycle(t *testing.T) ProxmoxGuestLifecyclePayload {
	t.Helper()
	payload := ProxmoxGuestLifecyclePayload{RequestID: "attempt-1", ActionID: "action-1", Operation: "reboot", GuestKind: "vm", VMID: 101, ExpectedStatus: "running", Timeout: 30}
	if err := BindProxmoxGuestLifecyclePayload(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestProxmoxGuestLifecycleCodecRejectsArgumentInjectionUnknownFieldsAndVersions(t *testing.T) {
	base := boundProxmoxGuestLifecycle(t)
	encoded, _ := json.Marshal(base)
	var object map[string]any
	_ = json.Unmarshal(encoded, &object)
	object["args"] = []string{"--", "sh", "-c", "id"}
	hostile, _ := json.Marshal(object)
	if _, err := DecodeProxmoxGuestLifecyclePayload(hostile); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("argument injection error = %v", err)
	}
	for name, mutate := range map[string]func(*ProxmoxGuestLifecyclePayload){
		"kind":      func(p *ProxmoxGuestLifecyclePayload) { p.GuestKind = "vm;id" },
		"operation": func(p *ProxmoxGuestLifecyclePayload) { p.Operation = "start --skiplock" },
		"vmid":      func(p *ProxmoxGuestLifecyclePayload) { p.VMID = -1 },
		"version":   func(p *ProxmoxGuestLifecyclePayload) { p.OperationVersion = 2 },
	} {
		t.Run(name, func(t *testing.T) {
			payload := base
			mutate(&payload)
			if err := ValidateProxmoxGuestLifecyclePayload(&payload); err == nil {
				t.Fatal("hostile payload was accepted")
			}
		})
	}
	if _, err := DecodeProxmoxGuestLifecyclePayload(append(encoded, []byte(` {}`)...)); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing JSON error = %v", err)
	}
}

func TestProxmoxGuestLifecycleResultIsRequestAndReceiptBound(t *testing.T) {
	req := boundProxmoxGuestLifecycle(t)
	result := ProxmoxGuestLifecycleResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation,
		OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest,
		GuestKind: req.GuestKind, VMID: req.VMID, ExecutionPhase: ProxmoxGuestPhaseComplete,
		MutationStarted: true, MutationCompleted: true,
	}
	if err := ValidateProxmoxGuestLifecycleResultForRequest(req, result); err != nil {
		t.Fatal(err)
	}
	result.VMID++
	if err := ValidateProxmoxGuestLifecycleResultForRequest(req, result); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatalf("mismatched receipt error = %v", err)
	}
}

func TestProxmoxGuestLifecycleTerminalReceiptQuerySupportsEveryClosedVerb(t *testing.T) {
	now := time.Now().UTC()
	for _, operation := range []string{"start", "stop", "shutdown", "reboot"} {
		t.Run(operation, func(t *testing.T) {
			req := ProxmoxGuestLifecyclePayload{
				RequestID: "attempt-" + operation, ActionID: "action-" + operation, Operation: operation,
				GuestKind: "vm", VMID: 101, ExpectedStatus: "running", Timeout: 30,
			}
			if operation == "start" {
				req.ExpectedStatus = "stopped"
			}
			if err := BindProxmoxGuestLifecyclePayload(&req); err != nil {
				t.Fatal(err)
			}
			identity := ProxmoxGuestLifecycleOperationIdentity("agent-pve", req)
			result := ProxmoxGuestLifecycleResultPayload{
				RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation,
				OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest,
				GuestKind: req.GuestKind, VMID: req.VMID, ExecutionPhase: ProxmoxGuestPhaseMutate,
				MutationStarted: true, Before: ProxmoxGuestLifecycleSnapshot{Status: req.ExpectedStatus, ObservedAt: now.Add(-time.Second)},
				Error: "Proxmox guest lifecycle mutation was interrupted after dispatch; target state requires recovery inspection",
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			record := operationreceipt.Record{
				Identity: identity, State: operationreceipt.StateTerminal,
				AcceptedAt: now.Add(-2 * time.Second), StartedAt: now.Add(-time.Second), TerminalAt: now,
				ResultKind: ProxmoxGuestLifecycleReceiptKind, ResultVersion: ProxmoxGuestLifecycleReceiptVersion, Result: encoded,
			}
			query := operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryFoundTerminal, Record: &record}
			if err := ValidateOperationQueryResultForIdentity(query, identity, now.Add(time.Second)); err != nil {
				t.Fatalf("valid Proxmox terminal receipt query rejected: %v", err)
			}

			query.Record.ResultKind = DockerContainerLifecycleReceiptKind
			if err := ValidateOperationQueryResultForIdentity(query, identity, now.Add(time.Second)); err == nil {
				t.Fatal("wrong Proxmox receipt envelope was accepted")
			}

			query.Record.ResultKind = ProxmoxGuestLifecycleReceiptKind
			result.Before.ObservedAt = time.Time{}
			query.Record.Result, err = json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if err := ValidateOperationQueryResultForIdentity(query, identity, now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "preflight observation") {
				t.Fatalf("missing durable preflight observation error = %v", err)
			}
		})
	}
}
