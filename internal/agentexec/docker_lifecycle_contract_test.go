package agentexec

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

func TestDockerContainerLifecycleWireCarriesFactsNotActionTruth(t *testing.T) {
	typeOf := reflect.TypeOf(DockerContainerLifecycleResultPayload{})
	for _, forbidden := range []string{"Success", "Verification", "Evidence", "EvidenceClass", "Compensation", "Independent"} {
		if _, ok := typeOf.FieldByName(forbidden); ok {
			t.Fatalf("Docker lifecycle wire result declares canonical action truth field %q", forbidden)
		}
	}
	payload, err := json.Marshal(DockerContainerLifecycleResultPayload{})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"success"`, `"verification"`, `"evidence"`, `"evidence_class"`, `"compensation"`, `"independent"`} {
		if jsonContains(payload, forbidden) {
			t.Fatalf("Docker lifecycle wire result exposes canonical action truth key %s: %s", forbidden, payload)
		}
	}
}

func TestDockerAgentWireAndRuntimeDeclareNoLocalActionTruthModel(t *testing.T) {
	paths := []string{"docker_lifecycle_codec.go", "../hostagent/docker_lifecycle.go"}
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"DockerContainerVerification", "ActionResultV2", "ActionVerificationTruth", "ActionEvidenceClass", "ActionCompensationTruth", `json:"verification`, `json:"evidence`, `json:"independent`} {
			if strings.Contains(string(source), forbidden) {
				t.Fatalf("%s declares server-owned action truth token %q", path, forbidden)
			}
		}
	}
}

func TestPendingDockerOperationBindsFullIdentityAndRejectsCrossTypeReuse(t *testing.T) {
	server := NewServer(func(string, string, string) bool { return true })
	identity := operationreceipt.Identity{AttemptID: "action.dispatch.1", ActionID: "action", OperationKind: DockerContainerOperationRestart, OperationVersion: DockerContainerLifecycleOperationVersion, RequestDigest: "sha256:" + strings.Repeat("a", 64), AgentID: "agent-1"}
	key, err := server.claimPendingDockerOperation(identity, "0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	defer server.releasePendingHostOperation(key)
	if _, err := server.claimPendingHostOperation(identity.AgentID, identity.AttemptID, identity.ActionID, HostUpdateOperationInstall); err == nil {
		t.Fatal("same attempt identity was reused across operation kinds")
	}
	exact := DockerContainerLifecycleResultPayload{RequestID: identity.AttemptID, ActionID: identity.ActionID, Operation: identity.OperationKind, OperationVersion: identity.OperationVersion, RequestDigest: identity.RequestDigest, ContainerID: "0123456789ab"}
	if !server.matchesPendingDockerOperation(identity.AgentID, exact) {
		t.Fatal("exact durable identity did not match")
	}
	exact.RequestDigest = "sha256:" + strings.Repeat("b", 64)
	if server.matchesPendingDockerOperation(identity.AgentID, exact) {
		t.Fatal("request digest drift reached the waiter")
	}
}

func jsonContains(payload []byte, value string) bool {
	for i := 0; i+len(value) <= len(payload); i++ {
		if string(payload[i:i+len(value)]) == value {
			return true
		}
	}
	return false
}

func validDockerUpdatePayload(t *testing.T) DockerContainerUpdatePayload {
	t.Helper()
	payload := DockerContainerUpdatePayload{
		RequestID:           "attempt-1",
		ActionID:            "action-1",
		Runtime:             "docker",
		ContainerID:         strings.Repeat("a", 12),
		ExpectedImageDigest: "sha256:" + strings.Repeat("9", 64),
		Timeout:             600,
	}
	if err := BindDockerContainerUpdatePayload(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func validDockerUpdateResult(t *testing.T, req DockerContainerUpdatePayload) DockerContainerUpdateResultPayload {
	t.Helper()
	observed := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	return DockerContainerUpdateResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, Operation: req.Operation,
		OperationVersion: req.OperationVersion, RequestDigest: req.RequestDigest, ContainerID: req.ContainerID,
		ExecutionPhase: DockerContainerPhaseComplete, MutationStarted: true, MutationCompleted: true, ReadbackRan: true,
		NewContainerID: strings.Repeat("b", 12), ContainerName: "app",
		OldImageDigest: "sha256:" + strings.Repeat("1", 64), NewImageDigest: "sha256:" + strings.Repeat("2", 64),
		BackupCreated: true, BackupContainer: "app_pulse_backup_20260714_120000",
		After: DockerContainerLifecycleSnapshot{ContainerID: strings.Repeat("b", 12), State: "running", Running: true, ObservedAt: observed},
	}
}

func TestDockerContainerUpdatePayloadBindAndValidate(t *testing.T) {
	payload := validDockerUpdatePayload(t)
	if payload.Operation != DockerContainerOperationUpdate || payload.OperationVersion != DockerContainerUpdateOperationVersion {
		t.Fatalf("bind did not stamp operation identity: %+v", payload)
	}
	if err := ValidateDockerContainerUpdatePayload(&payload); err != nil {
		t.Fatal(err)
	}

	tampered := payload
	tampered.ExpectedImageDigest = "sha256:" + strings.Repeat("8", 64)
	if err := ValidateDockerContainerUpdatePayload(&tampered); err == nil {
		t.Fatal("digest mismatch after expected-image tamper was accepted")
	}

	badDigest := payload
	badDigest.ExpectedImageDigest = "busybox:latest"
	if err := BindDockerContainerUpdatePayload(&badDigest); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDockerContainerUpdatePayload(&badDigest); err == nil {
		t.Fatal("non-digest expected image was accepted")
	}
}

func TestDockerContainerUpdateResultContract(t *testing.T) {
	req := validDockerUpdatePayload(t)
	result := validDockerUpdateResult(t, req)
	if err := ValidateDockerContainerUpdateResultForRequest(req, result); err != nil {
		t.Fatal(err)
	}

	completeWithoutReplacement := result
	completeWithoutReplacement.NewContainerID = ""
	if err := ValidateDockerContainerUpdateResultPayload(&completeWithoutReplacement); err == nil {
		t.Fatal("complete phase without replacement container was accepted")
	}

	rollbackWithoutAttempt := result
	rollbackWithoutAttempt.ExecutionPhase = DockerContainerPhaseMutate
	rollbackWithoutAttempt.MutationCompleted = false
	rollbackWithoutAttempt.Error = "create failed"
	rollbackWithoutAttempt.RolledBack = true
	rollbackWithoutAttempt.RollbackAttempted = false
	if err := ValidateDockerContainerUpdateResultPayload(&rollbackWithoutAttempt); err == nil {
		t.Fatal("rollback success without attempt was accepted")
	}

	wrongContainer := result
	wrongContainer.ContainerID = strings.Repeat("c", 12)
	if err := ValidateDockerContainerUpdateResultForRequest(req, wrongContainer); err == nil {
		t.Fatal("result for a different container was accepted")
	}
}

func TestDockerContainerUpdateWireCarriesFactsNotActionTruth(t *testing.T) {
	typeOf := reflect.TypeOf(DockerContainerUpdateResultPayload{})
	for _, forbidden := range []string{"Success", "Verification", "Evidence", "EvidenceClass", "Compensation", "Independent"} {
		if _, ok := typeOf.FieldByName(forbidden); ok {
			t.Fatalf("Docker update wire result declares canonical action truth field %q", forbidden)
		}
	}
	payload, err := json.Marshal(DockerContainerUpdateResultPayload{})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"success"`, `"verification"`, `"evidence"`, `"evidence_class"`, `"compensation"`, `"independent"`} {
		if jsonContains(payload, forbidden) {
			t.Fatalf("Docker update wire result exposes canonical action truth key %s: %s", forbidden, payload)
		}
	}
}

func TestDockerContainerUpdateOperationQueryEnvelope(t *testing.T) {
	req := validDockerUpdatePayload(t)
	result := validDockerUpdateResult(t, req)
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	identity := DockerContainerUpdateOperationIdentity("agent-1", req)
	terminalAt := result.After.ObservedAt.Add(time.Second)
	query := operationreceipt.QueryResult{
		Version: operationreceipt.ProtocolVersion,
		Status:  operationreceipt.QueryFoundTerminal,
		Record: &operationreceipt.Record{
			Identity: identity, State: operationreceipt.StateTerminal,
			AcceptedAt: result.After.ObservedAt.Add(-time.Minute), StartedAt: result.After.ObservedAt.Add(-30 * time.Second), TerminalAt: terminalAt,
			ResultKind: DockerContainerUpdateReceiptKind, ResultVersion: DockerContainerUpdateReceiptVersion, Result: encoded,
		},
	}
	if err := ValidateOperationQueryResultForIdentity(query, identity, terminalAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}

	query.Record.ResultKind = DockerContainerLifecycleReceiptKind
	if err := ValidateOperationQueryResultForIdentity(query, identity, terminalAt.Add(time.Minute)); err == nil {
		t.Fatal("lifecycle envelope was accepted for an update identity")
	}
}
