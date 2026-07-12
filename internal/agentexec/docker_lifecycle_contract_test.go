package agentexec

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

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
