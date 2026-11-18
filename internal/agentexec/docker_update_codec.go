package agentexec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

const (
	DockerContainerUpdateOperationVersion = 1
	DockerComposeUpdateOperationVersion   = 2
	DockerContainerUpdateReceiptKind      = "pulse.docker_container_update_result"
	DockerContainerUpdateReceiptVersion   = 1

	maxDockerContainerNameLength = 256
	maxDockerImageDigestLength   = 256
	// Image pulls dominate update time; the module's own overall budget is 15
	// minutes, so the transport bound leaves headroom without being unbounded.
	maxDockerContainerUpdateTimeoutSeconds     = 1800
	defaultDockerContainerUpdateTimeoutSeconds = 900
)

var composeProposalCommit = regexp.MustCompile(`^(?:[a-f0-9]{40}|[a-f0-9]{64})$`)

func DecodeDockerContainerUpdatePayload(data []byte) (DockerContainerUpdatePayload, error) {
	var payload DockerContainerUpdatePayload
	if err := decodeStrictDockerLifecycle(data, &payload); err != nil {
		return DockerContainerUpdatePayload{}, err
	}
	if err := ValidateDockerContainerUpdatePayload(&payload); err != nil {
		return DockerContainerUpdatePayload{}, err
	}
	return payload, nil
}

func DecodeDockerContainerUpdateResultPayload(data []byte) (DockerContainerUpdateResultPayload, error) {
	var payload DockerContainerUpdateResultPayload
	if err := decodeStrictDockerLifecycle(data, &payload); err != nil {
		return DockerContainerUpdateResultPayload{}, err
	}
	if err := ValidateDockerContainerUpdateResultPayload(&payload); err != nil {
		return DockerContainerUpdateResultPayload{}, err
	}
	return payload, nil
}

func BindDockerContainerUpdatePayload(payload *DockerContainerUpdatePayload) error {
	if payload == nil {
		return fmt.Errorf("docker container update payload is required")
	}
	payload.Operation = DockerContainerOperationUpdate
	payload.OperationVersion = DockerContainerUpdateOperationVersion
	if payload.Compose != nil {
		payload.OperationVersion = DockerComposeUpdateOperationVersion
	}
	digest, err := dockerContainerUpdateRequestDigest(*payload)
	if err != nil {
		return err
	}
	payload.RequestDigest = digest
	return nil
}

func dockerContainerUpdateRequestDigest(payload DockerContainerUpdatePayload) (string, error) {
	return operationreceipt.DigestCanonicalJSON(struct {
		ActionID            string                           `json:"action_id"`
		Operation           string                           `json:"operation"`
		OperationVersion    int                              `json:"operation_version"`
		Runtime             string                           `json:"runtime"`
		ContainerID         string                           `json:"container_id"`
		ExpectedImageDigest string                           `json:"expected_image_digest"`
		Compose             *agentsdocker.ComposeImageChange `json:"compose,omitempty"`
	}{
		strings.TrimSpace(payload.ActionID), strings.TrimSpace(payload.Operation), payload.OperationVersion,
		strings.ToLower(strings.TrimSpace(payload.Runtime)), strings.ToLower(strings.TrimSpace(payload.ContainerID)),
		strings.ToLower(strings.TrimSpace(payload.ExpectedImageDigest)), payload.Compose,
	})
}

func ValidateDockerContainerUpdatePayload(payload *DockerContainerUpdatePayload) error {
	if payload == nil {
		return fmt.Errorf("docker container update payload is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.ActionID = strings.TrimSpace(payload.ActionID)
	payload.Operation = strings.TrimSpace(payload.Operation)
	payload.Runtime = strings.ToLower(strings.TrimSpace(payload.Runtime))
	payload.ContainerID = strings.ToLower(strings.TrimSpace(payload.ContainerID))
	payload.ExpectedImageDigest = strings.ToLower(strings.TrimSpace(payload.ExpectedImageDigest))
	if payload.RequestID == "" || len(payload.RequestID) > maxRequestIDLength || payload.ActionID == "" || len(payload.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid docker update request or action id")
	}
	if payload.Operation != DockerContainerOperationUpdate {
		return fmt.Errorf("unsupported docker container update operation %q", payload.Operation)
	}
	if !(payload.OperationVersion == DockerContainerUpdateOperationVersion && payload.Compose == nil || payload.OperationVersion == DockerComposeUpdateOperationVersion && payload.Compose.Valid() && payload.Runtime == "docker") {
		return fmt.Errorf("unsupported docker container update operation version %d", payload.OperationVersion)
	}
	if payload.Runtime != "docker" && payload.Runtime != "podman" {
		return fmt.Errorf("unsupported container runtime %q", payload.Runtime)
	}
	if !dockerContainerIDPattern.MatchString(payload.ContainerID) {
		return fmt.Errorf("container id must be an immutable hexadecimal id")
	}
	if !hostUpdateInventoryHashPattern.MatchString(payload.ExpectedImageDigest) {
		return fmt.Errorf("invalid docker update expected image digest")
	}
	expectedDigest, err := dockerContainerUpdateRequestDigest(*payload)
	if err != nil {
		return err
	}
	if payload.RequestDigest != expectedDigest {
		return fmt.Errorf("docker container update request digest mismatch")
	}
	if payload.Timeout < 0 || payload.Timeout > maxDockerContainerUpdateTimeoutSeconds {
		return fmt.Errorf("docker container update timeout must be between 0 and %d seconds", maxDockerContainerUpdateTimeoutSeconds)
	}
	if payload.Timeout == 0 {
		payload.Timeout = defaultDockerContainerUpdateTimeoutSeconds
	}
	return nil
}

func ValidateDockerContainerUpdateResultPayload(result *DockerContainerUpdateResultPayload) error {
	if result == nil {
		return fmt.Errorf("docker container update result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.ActionID = strings.TrimSpace(result.ActionID)
	result.Operation = strings.TrimSpace(result.Operation)
	result.RequestDigest = strings.TrimSpace(result.RequestDigest)
	result.ContainerID = strings.ToLower(strings.TrimSpace(result.ContainerID))
	result.NewContainerID = strings.ToLower(strings.TrimSpace(result.NewContainerID))
	result.ContainerName = strings.TrimSpace(result.ContainerName)
	result.OldImageDigest = strings.TrimSpace(result.OldImageDigest)
	result.NewImageDigest = strings.TrimSpace(result.NewImageDigest)
	result.BackupContainer = strings.TrimSpace(result.BackupContainer)
	result.ExecutionPhase = strings.TrimSpace(result.ExecutionPhase)
	result.ReasonCode = strings.TrimSpace(result.ReasonCode)
	result.Error = strings.TrimSpace(result.Error)
	if result.RequestID == "" || len(result.RequestID) > maxRequestIDLength || result.ActionID == "" || len(result.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid docker update result identity")
	}
	if result.Operation != DockerContainerOperationUpdate {
		return fmt.Errorf("unsupported docker update result operation %q", result.Operation)
	}
	if (result.OperationVersion != DockerContainerUpdateOperationVersion && result.OperationVersion != DockerComposeUpdateOperationVersion) || !dockerContainerIDPattern.MatchString(result.ContainerID) || !hostUpdateInventoryHashPattern.MatchString(result.RequestDigest) {
		return fmt.Errorf("invalid docker update result binding")
	}
	if result.ExecutionPhase != DockerContainerPhasePreflight && result.ExecutionPhase != DockerContainerPhaseMutate && result.ExecutionPhase != DockerContainerPhaseVerify && result.ExecutionPhase != DockerContainerPhaseComplete {
		return fmt.Errorf("unsupported docker update execution phase %q", result.ExecutionPhase)
	}
	if result.NewContainerID != "" && !dockerContainerIDPattern.MatchString(result.NewContainerID) {
		return fmt.Errorf("docker update result has invalid replacement container id")
	}
	if len(result.Error) > 1024 || len(result.ContainerName) > maxDockerContainerNameLength || len(result.BackupContainer) > maxDockerContainerNameLength {
		return fmt.Errorf("docker update result exceeds bounded contract")
	}
	if result.ReasonCode != "" && !IsActionRefusalReasonCode(result.ReasonCode) {
		return fmt.Errorf("invalid docker update refusal reason code")
	}
	if result.ReasonCode != "" && result.MutationStarted {
		return fmt.Errorf("docker update refusal reason conflicts with mutation state")
	}
	if len(result.OldImageDigest) > maxDockerImageDigestLength || len(result.NewImageDigest) > maxDockerImageDigestLength {
		return fmt.Errorf("docker update result digest exceeds bounded contract")
	}
	if result.MutationCompleted && !result.MutationStarted {
		return fmt.Errorf("completed docker update mutation requires mutation start")
	}
	if result.RolledBack && !result.RollbackAttempted {
		return fmt.Errorf("docker update rollback success requires a rollback attempt")
	}
	if result.RollbackAttempted && !result.MutationStarted {
		return fmt.Errorf("docker update rollback requires mutation start")
	}
	if result.ReadbackRan && result.After.ObservedAt.IsZero() {
		return fmt.Errorf("docker update readback requires an observation")
	}
	if result.OperationVersion == DockerContainerUpdateOperationVersion && (result.SourceStage != "" || result.ProposalCommit != "" || result.ProposalBranch != "") {
		return fmt.Errorf("unexpected source result for container operation")
	}
	if result.OperationVersion == DockerComposeUpdateOperationVersion {
		switch result.SourceStage {
		case "", "source_review", "image_pull", "source_write", "service_recreate", "service_readback", "git_prepare", "git_publish", "git_proposed":
		default:
			return fmt.Errorf("invalid Compose update stage")
		}
		if result.ProposalBranch != "" || result.ProposalCommit != "" || result.SourceStage == "git_proposed" {
			if result.SourceStage != "git_proposed" || result.ProposalBranch != "pulse/service-update/"+result.ActionID || !composeProposalCommit.MatchString(result.ProposalCommit) {
				return fmt.Errorf("invalid Git proposal result")
			}
		}
	}
	if result.ExecutionPhase == DockerContainerPhaseComplete {
		if result.Error != "" || !result.MutationCompleted || (result.NewContainerID == "" && result.SourceStage != "git_proposed") {
			return fmt.Errorf("complete docker update requires a replacement container and no error")
		}
	}
	return nil
}

func DockerContainerUpdateOperationIdentity(agentID string, payload DockerContainerUpdatePayload) operationreceipt.Identity {
	return operationreceipt.Identity{AttemptID: payload.RequestID, ActionID: payload.ActionID, OperationKind: payload.Operation, OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, AgentID: strings.TrimSpace(agentID)}
}

func ValidateDockerContainerUpdateResultForRequest(req DockerContainerUpdatePayload, result DockerContainerUpdateResultPayload) error {
	if err := ValidateDockerContainerUpdatePayload(&req); err != nil {
		return err
	}
	if err := ValidateDockerContainerUpdateResultPayload(&result); err != nil {
		return err
	}
	if result.RequestID != req.RequestID || result.ActionID != req.ActionID || result.Operation != req.Operation || result.OperationVersion != req.OperationVersion || result.RequestDigest != req.RequestDigest {
		return fmt.Errorf("docker update result identity mismatch")
	}
	if req.Compose != nil {
		gitStage := strings.HasPrefix(result.SourceStage, "git_")
		if (gitStage && req.Compose.Mode != "git") || (result.MutationCompleted && req.Compose.Mode == "git" && result.SourceStage != "git_proposed") || (result.MutationCompleted && req.Compose.Mode == "local" && result.SourceStage != "service_readback") || result.BackupCreated || result.RollbackAttempted {
			return fmt.Errorf("result disagrees with the reviewed Compose operation")
		}
		if result.MutationCompleted && req.Compose.Mode == "local" {
			_, want, _ := strings.Cut(req.Compose.AfterImage, "@")
			if result.NewImageDigest != want {
				return fmt.Errorf("replacement manifest differs from approved digest")
			}
		}
	}
	if result.ContainerID != req.ContainerID {
		return fmt.Errorf("docker update result container mismatch")
	}
	if result.After.ContainerID != "" && result.NewContainerID != "" && !strings.EqualFold(result.After.ContainerID, result.NewContainerID) {
		return fmt.Errorf("docker update after-state container mismatch")
	}
	return nil
}
