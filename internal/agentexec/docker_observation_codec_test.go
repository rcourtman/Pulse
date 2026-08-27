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
		Snapshot: DockerContainerObservationSnapshot{ContainerID: decoded.ContainerID, State: "running", Running: true, Health: DockerContainerHealthHealthy, ObservedAt: now},
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

func TestDockerContainerObservationRequiresExplicitHealthTruth(t *testing.T) {
	now := time.Now().UTC()
	req := DockerContainerObservationPayload{RequestID: "observe-1", ActionID: "action-1", Runtime: "docker", ContainerID: strings.Repeat("a", 64)}
	if err := BindDockerContainerObservationPayload(&req); err != nil {
		t.Fatal(err)
	}
	base := DockerContainerObservationResultPayload{
		RequestID: req.RequestID, ActionID: req.ActionID, ProtocolVersion: req.ProtocolVersion, RequestDigest: req.RequestDigest, Observed: true,
		Snapshot: DockerContainerObservationSnapshot{ContainerID: req.ContainerID, State: "running", Running: true, ObservedAt: now},
	}
	for _, health := range []string{"", "unknown", "degraded"} {
		result := base
		result.Snapshot.Health = health
		if err := ValidateDockerContainerObservationResultForRequest(req, result, now); err == nil {
			t.Fatalf("health %q was accepted without canonical truth", health)
		}
	}
	for _, health := range []string{DockerContainerHealthNone, DockerContainerHealthStarting, DockerContainerHealthHealthy, DockerContainerHealthUnhealthy} {
		result := base
		result.Snapshot.Health = health
		if err := ValidateDockerContainerObservationResultForRequest(req, result, now); err != nil {
			t.Fatalf("canonical health %q was rejected: %v", health, err)
		}
	}
}

func TestDockerContainerObservationWireCarriesFactsNotActionTruth(t *testing.T) {
	payload, err := json.Marshal(DockerContainerObservationResultPayload{Snapshot: DockerContainerObservationSnapshot{Health: DockerContainerHealthHealthy}})
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(payload)
	for _, forbidden := range []string{`"success"`, `"verification"`, `"evidence"`, `"independent"`, `"command"`, `"operation"`, `"approval"`} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("observation wire contains forbidden action-truth or authority field %s: %s", forbidden, encoded)
		}
	}
	if !strings.Contains(encoded, `"health":"healthy"`) {
		t.Fatalf("observation wire omitted explicit health fact: %s", encoded)
	}
	lifecycle, err := json.Marshal(DockerContainerLifecycleSnapshot{Health: DockerContainerHealthHealthy})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(lifecycle), `"health"`) {
		t.Fatalf("version-1 lifecycle receipt wire changed incompatibly: %s", lifecycle)
	}
}
