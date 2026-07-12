package agentexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

const (
	DockerContainerLifecycleOperationVersion = 1
	DockerContainerLifecycleReceiptKind      = "pulse.docker_container_lifecycle_result"
	DockerContainerLifecycleReceiptVersion   = 1
)

var dockerContainerIDPattern = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

func decodeStrictDockerLifecycle(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("docker lifecycle payload is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("docker lifecycle payload contains trailing JSON")
		}
		return fmt.Errorf("docker lifecycle payload contains trailing data: %w", err)
	}
	return nil
}

func DecodeDockerContainerLifecyclePayload(data []byte) (DockerContainerLifecyclePayload, error) {
	var payload DockerContainerLifecyclePayload
	if err := decodeStrictDockerLifecycle(data, &payload); err != nil {
		return DockerContainerLifecyclePayload{}, err
	}
	if err := ValidateDockerContainerLifecyclePayload(&payload); err != nil {
		return DockerContainerLifecyclePayload{}, err
	}
	return payload, nil
}

func DecodeDockerContainerLifecycleResultPayload(data []byte) (DockerContainerLifecycleResultPayload, error) {
	var payload DockerContainerLifecycleResultPayload
	if err := decodeStrictDockerLifecycle(data, &payload); err != nil {
		return DockerContainerLifecycleResultPayload{}, err
	}
	if err := ValidateDockerContainerLifecycleResultPayload(&payload); err != nil {
		return DockerContainerLifecycleResultPayload{}, err
	}
	return payload, nil
}

func BindDockerContainerLifecyclePayload(payload *DockerContainerLifecyclePayload) error {
	if payload == nil {
		return fmt.Errorf("docker container lifecycle payload is required")
	}
	payload.OperationVersion = DockerContainerLifecycleOperationVersion
	digest, err := dockerContainerLifecycleRequestDigest(*payload)
	if err != nil {
		return err
	}
	payload.RequestDigest = digest
	return nil
}

func dockerContainerLifecycleRequestDigest(payload DockerContainerLifecyclePayload) (string, error) {
	return operationreceipt.DigestCanonicalJSON(struct {
		ActionID          string    `json:"action_id"`
		Operation         string    `json:"operation"`
		OperationVersion  int       `json:"operation_version"`
		Runtime           string    `json:"runtime"`
		ContainerID       string    `json:"container_id"`
		ExpectedState     string    `json:"expected_state"`
		ExpectedStartedAt time.Time `json:"expected_started_at,omitempty"`
	}{
		strings.TrimSpace(payload.ActionID), strings.TrimSpace(payload.Operation), payload.OperationVersion,
		strings.ToLower(strings.TrimSpace(payload.Runtime)), strings.ToLower(strings.TrimSpace(payload.ContainerID)),
		strings.ToLower(strings.TrimSpace(payload.ExpectedState)), payload.ExpectedStartedAt.UTC(),
	})
}

func ValidateDockerContainerLifecyclePayload(payload *DockerContainerLifecyclePayload) error {
	if payload == nil {
		return fmt.Errorf("docker container lifecycle payload is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.ActionID = strings.TrimSpace(payload.ActionID)
	payload.Operation = strings.TrimSpace(payload.Operation)
	payload.Runtime = strings.ToLower(strings.TrimSpace(payload.Runtime))
	payload.ContainerID = strings.ToLower(strings.TrimSpace(payload.ContainerID))
	payload.ExpectedState = strings.ToLower(strings.TrimSpace(payload.ExpectedState))
	payload.ExpectedStartedAt = payload.ExpectedStartedAt.UTC()
	if payload.RequestID == "" || len(payload.RequestID) > maxRequestIDLength || payload.ActionID == "" || len(payload.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid docker lifecycle request or action id")
	}
	if payload.Operation != DockerContainerOperationStart && payload.Operation != DockerContainerOperationStop && payload.Operation != DockerContainerOperationRestart {
		return fmt.Errorf("unsupported docker container lifecycle operation %q", payload.Operation)
	}
	if payload.OperationVersion != DockerContainerLifecycleOperationVersion {
		return fmt.Errorf("unsupported docker container lifecycle operation version %d", payload.OperationVersion)
	}
	if payload.Runtime != "docker" && payload.Runtime != "podman" {
		return fmt.Errorf("unsupported container runtime %q", payload.Runtime)
	}
	if !dockerContainerIDPattern.MatchString(payload.ContainerID) {
		return fmt.Errorf("container id must be an immutable hexadecimal id")
	}
	if payload.ExpectedState == "" || len(payload.ExpectedState) > 32 {
		return fmt.Errorf("expected container state is required")
	}
	expectedDigest, err := dockerContainerLifecycleRequestDigest(*payload)
	if err != nil {
		return err
	}
	if payload.RequestDigest != expectedDigest {
		return fmt.Errorf("docker container lifecycle request digest mismatch")
	}
	if payload.Timeout < 0 || payload.Timeout > 300 {
		return fmt.Errorf("docker container lifecycle timeout must be between 0 and 300 seconds")
	}
	if payload.Timeout == 0 {
		payload.Timeout = 120
	}
	return nil
}

func ValidateDockerContainerLifecycleResultPayload(result *DockerContainerLifecycleResultPayload) error {
	if result == nil {
		return fmt.Errorf("docker container lifecycle result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.ActionID = strings.TrimSpace(result.ActionID)
	result.Operation = strings.TrimSpace(result.Operation)
	result.RequestDigest = strings.TrimSpace(result.RequestDigest)
	result.ContainerID = strings.ToLower(strings.TrimSpace(result.ContainerID))
	result.ExecutionPhase = strings.TrimSpace(result.ExecutionPhase)
	result.Error = strings.TrimSpace(result.Error)
	if result.RequestID == "" || len(result.RequestID) > maxRequestIDLength || result.ActionID == "" || len(result.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid docker lifecycle result identity")
	}
	if result.Operation != DockerContainerOperationStart && result.Operation != DockerContainerOperationStop && result.Operation != DockerContainerOperationRestart {
		return fmt.Errorf("unsupported docker lifecycle result operation %q", result.Operation)
	}
	if result.OperationVersion != DockerContainerLifecycleOperationVersion || !dockerContainerIDPattern.MatchString(result.ContainerID) || !hostUpdateInventoryHashPattern.MatchString(result.RequestDigest) {
		return fmt.Errorf("invalid docker lifecycle result binding")
	}
	if result.ExecutionPhase != DockerContainerPhasePreflight && result.ExecutionPhase != DockerContainerPhaseMutate && result.ExecutionPhase != DockerContainerPhaseVerify && result.ExecutionPhase != DockerContainerPhaseComplete {
		return fmt.Errorf("unsupported docker lifecycle execution phase %q", result.ExecutionPhase)
	}
	if len(result.Error) > 1024 || result.Before.RestartCount < 0 || result.After.RestartCount < 0 {
		return fmt.Errorf("docker lifecycle result exceeds bounded contract")
	}
	for _, snapshot := range []DockerContainerLifecycleSnapshot{result.Before, result.After} {
		if snapshot.ContainerID != "" && !dockerContainerIDPattern.MatchString(strings.ToLower(strings.TrimSpace(snapshot.ContainerID))) {
			return fmt.Errorf("docker lifecycle result has invalid container id")
		}
		if !snapshot.ObservedAt.IsZero() && snapshot.ObservedAt.Location() != time.UTC {
			return fmt.Errorf("docker lifecycle observation timestamp must be UTC")
		}
	}
	if result.MutationCompleted && !result.MutationStarted {
		return fmt.Errorf("completed docker lifecycle mutation requires mutation start")
	}
	if result.ReadbackRan && result.After.ObservedAt.IsZero() {
		return fmt.Errorf("docker lifecycle readback requires an observation")
	}
	return nil
}

func DockerContainerLifecycleOperationIdentity(agentID string, payload DockerContainerLifecyclePayload) operationreceipt.Identity {
	return operationreceipt.Identity{AttemptID: payload.RequestID, ActionID: payload.ActionID, OperationKind: payload.Operation, OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, AgentID: strings.TrimSpace(agentID)}
}

func ValidateDockerContainerLifecycleResultForRequest(req DockerContainerLifecyclePayload, result DockerContainerLifecycleResultPayload) error {
	if err := ValidateDockerContainerLifecyclePayload(&req); err != nil {
		return err
	}
	if err := ValidateDockerContainerLifecycleResultPayload(&result); err != nil {
		return err
	}
	if result.RequestID != req.RequestID || result.ActionID != req.ActionID || result.Operation != req.Operation || result.OperationVersion != req.OperationVersion || result.RequestDigest != req.RequestDigest || result.ContainerID != req.ContainerID {
		return fmt.Errorf("docker lifecycle result identity mismatch")
	}
	if result.Before.ContainerID != "" && !strings.EqualFold(result.Before.ContainerID, req.ContainerID) {
		return fmt.Errorf("docker lifecycle before-state container mismatch")
	}
	if result.After.ContainerID != "" && !strings.EqualFold(result.After.ContainerID, req.ContainerID) {
		return fmt.Errorf("docker lifecycle after-state container mismatch")
	}
	return nil
}
