package agentexec

import (
	"fmt"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

const DockerContainerObservationProtocolVersion = 2

const dockerContainerObservationMaxClockSkew = 5 * time.Minute

func DecodeDockerContainerObservationPayload(data []byte) (DockerContainerObservationPayload, error) {
	var payload DockerContainerObservationPayload
	if err := decodeStrictDockerLifecycle(data, &payload); err != nil {
		return DockerContainerObservationPayload{}, err
	}
	if err := ValidateDockerContainerObservationPayload(&payload); err != nil {
		return DockerContainerObservationPayload{}, err
	}
	return payload, nil
}

func DecodeDockerContainerObservationResultPayload(data []byte) (DockerContainerObservationResultPayload, error) {
	var result DockerContainerObservationResultPayload
	if err := decodeStrictDockerLifecycle(data, &result); err != nil {
		return DockerContainerObservationResultPayload{}, err
	}
	if err := ValidateDockerContainerObservationResultPayload(&result); err != nil {
		return DockerContainerObservationResultPayload{}, err
	}
	return result, nil
}

func BindDockerContainerObservationPayload(payload *DockerContainerObservationPayload) error {
	if payload == nil {
		return fmt.Errorf("docker container observation payload is required")
	}
	payload.ProtocolVersion = DockerContainerObservationProtocolVersion
	digest, err := dockerContainerObservationRequestDigest(*payload)
	if err != nil {
		return err
	}
	payload.RequestDigest = digest
	return nil
}

func dockerContainerObservationRequestDigest(payload DockerContainerObservationPayload) (string, error) {
	return operationreceipt.DigestCanonicalJSON(struct {
		ActionID        string `json:"action_id"`
		ProtocolVersion int    `json:"protocol_version"`
		Runtime         string `json:"runtime"`
		ContainerID     string `json:"container_id"`
	}{strings.TrimSpace(payload.ActionID), payload.ProtocolVersion, strings.ToLower(strings.TrimSpace(payload.Runtime)), strings.ToLower(strings.TrimSpace(payload.ContainerID))})
}

func ValidateDockerContainerObservationPayload(payload *DockerContainerObservationPayload) error {
	if payload == nil {
		return fmt.Errorf("docker container observation payload is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.ActionID = strings.TrimSpace(payload.ActionID)
	payload.RequestDigest = strings.TrimSpace(payload.RequestDigest)
	payload.Runtime = strings.ToLower(strings.TrimSpace(payload.Runtime))
	payload.ContainerID = strings.ToLower(strings.TrimSpace(payload.ContainerID))
	if payload.RequestID == "" || len(payload.RequestID) > maxRequestIDLength || payload.ActionID == "" || len(payload.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid docker observation request identity")
	}
	if payload.ProtocolVersion != DockerContainerObservationProtocolVersion {
		return fmt.Errorf("unsupported docker observation protocol version %d", payload.ProtocolVersion)
	}
	if payload.Runtime != "docker" && payload.Runtime != "podman" {
		return fmt.Errorf("unsupported container runtime %q", payload.Runtime)
	}
	if !dockerContainerIDPattern.MatchString(payload.ContainerID) {
		return fmt.Errorf("container id must be an immutable hexadecimal id")
	}
	digest, err := dockerContainerObservationRequestDigest(*payload)
	if err != nil {
		return err
	}
	if payload.RequestDigest != digest {
		return fmt.Errorf("docker observation request digest mismatch")
	}
	return nil
}

func ValidateDockerContainerObservationResultPayload(result *DockerContainerObservationResultPayload) error {
	if result == nil {
		return fmt.Errorf("docker container observation result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.ActionID = strings.TrimSpace(result.ActionID)
	result.RequestDigest = strings.TrimSpace(result.RequestDigest)
	result.ReasonCode = strings.TrimSpace(result.ReasonCode)
	result.Snapshot.ContainerID = strings.ToLower(strings.TrimSpace(result.Snapshot.ContainerID))
	result.Snapshot.State = strings.ToLower(strings.TrimSpace(result.Snapshot.State))
	result.Snapshot.Health = strings.ToLower(strings.TrimSpace(result.Snapshot.Health))
	result.Snapshot.ObservedAt = result.Snapshot.ObservedAt.UTC()
	result.Snapshot.StartedAt = result.Snapshot.StartedAt.UTC()
	if result.RequestID == "" || len(result.RequestID) > maxRequestIDLength || result.ActionID == "" || len(result.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid docker observation result identity")
	}
	if result.ProtocolVersion != DockerContainerObservationProtocolVersion || !hostUpdateInventoryHashPattern.MatchString(result.RequestDigest) {
		return fmt.Errorf("invalid docker observation result binding")
	}
	if !result.Observed {
		if !IsActionRefusalReasonCode(result.ReasonCode) || result.Snapshot.ContainerID != "" || result.Snapshot.State != "" || result.Snapshot.Health != "" || result.Snapshot.Running || !result.Snapshot.StartedAt.IsZero() || result.Snapshot.RestartCount != 0 || !result.Snapshot.ObservedAt.IsZero() {
			return fmt.Errorf("inconclusive docker observation requires a bounded reason and no snapshot")
		}
		return nil
	}
	if result.ReasonCode != "" || !dockerContainerIDPattern.MatchString(result.Snapshot.ContainerID) || result.Snapshot.State == "" || len(result.Snapshot.State) > 32 || !IsDockerContainerHealth(result.Snapshot.Health) || result.Snapshot.RestartCount < 0 || result.Snapshot.ObservedAt.IsZero() {
		return fmt.Errorf("invalid docker observation snapshot")
	}
	return nil
}

func ValidateDockerContainerObservationResultForRequest(req DockerContainerObservationPayload, result DockerContainerObservationResultPayload, receivedAt time.Time) error {
	if err := ValidateDockerContainerObservationPayload(&req); err != nil {
		return err
	}
	if err := ValidateDockerContainerObservationResultPayload(&result); err != nil {
		return err
	}
	if result.RequestID != req.RequestID || result.ActionID != req.ActionID || result.ProtocolVersion != req.ProtocolVersion || result.RequestDigest != req.RequestDigest || (result.Observed && result.Snapshot.ContainerID != req.ContainerID) {
		return fmt.Errorf("docker observation result identity mismatch")
	}
	if !result.Observed {
		return nil
	}
	if receivedAt.IsZero() || result.Snapshot.ObservedAt.After(receivedAt.UTC().Add(dockerContainerObservationMaxClockSkew)) || result.Snapshot.ObservedAt.Before(receivedAt.UTC().Add(-15*time.Minute)) {
		return fmt.Errorf("docker observation result is stale or clock-skewed")
	}
	return nil
}
