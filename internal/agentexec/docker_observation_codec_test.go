package agentexec

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDockerContainerObservationContractBindsRequestAndFreshResult(t *testing.T) {
	now := time.Now().UTC()
	req := DockerContainerObservationPayload{
		RequestID: "observe-1", ActionID: "action-1", Runtime: "docker", ContainerID: strings.Repeat("a", 64),
	}
	if err := BindDockerContainerObservationPayload(&req); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeDockerContainerObservationPayload(encoded)
	if err != nil {
		t.Fatal(err)
	}
	result := DockerContainerObservationResultPayload{
		RequestID: decoded.RequestID, ActionID: decoded.ActionID, ProtocolVersion: decoded.ProtocolVersion, RequestDigest: decoded.RequestDigest,
		Observed: true,
		Snapshot: DockerContainerLifecycleSnapshot{ContainerID: decoded.ContainerID, State: "running", Running: true, ObservedAt: now},
	}
	if err := ValidateDockerContainerObservationResultForRequest(decoded, result, now); err != nil {
		t.Fatalf("fresh bound result rejected: %v", err)
	}

	tampered := result
	tampered.ActionID = "different-action"
	if err := ValidateDockerContainerObservationResultForRequest(decoded, tampered, now); err == nil {
		t.Fatal("cross-action observation result was accepted")
	}
	stale := result
	stale.Snapshot.ObservedAt = now.Add(-16 * time.Minute)
	if err := ValidateDockerContainerObservationResultForRequest(decoded, stale, now); err == nil {
		t.Fatal("stale observation result was accepted")
	}
}

func TestDockerContainerObservationWireCarriesFactsNotActionTruth(t *testing.T) {
	payload, err := json.Marshal(DockerContainerObservationResultPayload{})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{`"success"`, `"verification"`, `"evidence"`, `"independent"`, `"command"`, `"operation"`, `"approval"`} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("observation wire contains forbidden action-truth or authority field %s: %s", forbidden, encoded)
		}
	}
}
