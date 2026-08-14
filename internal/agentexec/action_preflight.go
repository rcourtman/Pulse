package agentexec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

const ActionPreflightProtocolVersion = 1

// ActionPreflightError lets an in-process typed module preserve a safe refusal
// category without making callers parse provider error text. Err remains local
// and is never serialized by the action preflight protocol.
type ActionPreflightError struct {
	ReasonCode string
	Err        error
}

func (e *ActionPreflightError) Error() string {
	if e == nil || e.Err == nil {
		return "action preflight refused"
	}
	return e.Err.Error()
}

func (e *ActionPreflightError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewActionPreflightError(reasonCode string, err error) error {
	reasonCode = strings.TrimSpace(reasonCode)
	if !IsActionRefusalReasonCode(reasonCode) {
		reasonCode = ActionRefusalTargetPreconditionFailed
	}
	return &ActionPreflightError{ReasonCode: reasonCode, Err: err}
}

func ActionPreflightReasonCode(err error, fallback string) string {
	var refusal *ActionPreflightError
	if errors.As(err, &refusal) && IsActionRefusalReasonCode(refusal.ReasonCode) {
		return refusal.ReasonCode
	}
	if IsActionRefusalReasonCode(fallback) {
		return fallback
	}
	return ActionRefusalTargetPreconditionFailed
}

func DecodeActionPreflightPayload(data []byte) (ActionPreflightPayload, error) {
	var payload ActionPreflightPayload
	if err := decodeStrictActionPreflight(data, &payload); err != nil {
		return ActionPreflightPayload{}, err
	}
	if err := ValidateActionPreflightPayload(&payload); err != nil {
		return ActionPreflightPayload{}, err
	}
	return payload, nil
}

func DecodeActionPreflightResultPayload(data []byte) (ActionPreflightResultPayload, error) {
	var result ActionPreflightResultPayload
	if err := decodeStrictActionPreflight(data, &result); err != nil {
		return ActionPreflightResultPayload{}, err
	}
	if err := ValidateActionPreflightResultPayload(&result); err != nil {
		return ActionPreflightResultPayload{}, err
	}
	return result, nil
}

func decodeStrictActionPreflight(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("action preflight payload is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("action preflight payload contains trailing JSON")
		}
		return fmt.Errorf("action preflight payload contains trailing data: %w", err)
	}
	return nil
}

func ValidateActionPreflightPayload(payload *ActionPreflightPayload) error {
	if payload == nil {
		return fmt.Errorf("action preflight payload is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	if payload.RequestID == "" || len(payload.RequestID) > maxRequestIDLength {
		return fmt.Errorf("invalid action preflight request id")
	}
	if payload.ProtocolVersion != ActionPreflightProtocolVersion {
		return fmt.Errorf("unsupported action preflight protocol version %d", payload.ProtocolVersion)
	}
	count := 0
	typedRequestID := ""
	if payload.HostUpdate != nil {
		count++
		if err := ValidateHostUpdatePayload(payload.HostUpdate); err != nil {
			return err
		}
		typedRequestID = payload.HostUpdate.RequestID
	}
	if payload.StorageCleanup != nil {
		count++
		if err := ValidateHostStorageCleanupPayload(payload.StorageCleanup); err != nil {
			return err
		}
		typedRequestID = payload.StorageCleanup.RequestID
	}
	if payload.DockerLifecycle != nil {
		count++
		if err := ValidateDockerContainerLifecyclePayload(payload.DockerLifecycle); err != nil {
			return err
		}
		typedRequestID = payload.DockerLifecycle.RequestID
	}
	if payload.DockerUpdate != nil {
		count++
		if err := ValidateDockerContainerUpdatePayload(payload.DockerUpdate); err != nil {
			return err
		}
		typedRequestID = payload.DockerUpdate.RequestID
	}
	if count != 1 {
		return fmt.Errorf("action preflight requires exactly one typed operation")
	}
	if payload.RequestID != typedRequestID {
		return fmt.Errorf("action preflight request id does not match typed operation")
	}
	return nil
}

func ActionPreflightBinding(payload ActionPreflightPayload) (operation string, version int, digest string) {
	switch {
	case payload.HostUpdate != nil:
		return payload.HostUpdate.Operation, payload.HostUpdate.OperationVersion, payload.HostUpdate.RequestDigest
	case payload.StorageCleanup != nil:
		return payload.StorageCleanup.Operation, payload.StorageCleanup.OperationVersion, payload.StorageCleanup.RequestDigest
	case payload.DockerLifecycle != nil:
		return payload.DockerLifecycle.Operation, payload.DockerLifecycle.OperationVersion, payload.DockerLifecycle.RequestDigest
	case payload.DockerUpdate != nil:
		return payload.DockerUpdate.Operation, payload.DockerUpdate.OperationVersion, payload.DockerUpdate.RequestDigest
	default:
		return "", 0, ""
	}
}

func ValidateActionPreflightResultPayload(result *ActionPreflightResultPayload) error {
	if result == nil {
		return fmt.Errorf("action preflight result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.Operation = strings.TrimSpace(result.Operation)
	result.RequestDigest = strings.TrimSpace(result.RequestDigest)
	result.ReasonCode = strings.TrimSpace(result.ReasonCode)
	if result.RequestID == "" || len(result.RequestID) > maxRequestIDLength {
		return fmt.Errorf("invalid action preflight result request id")
	}
	if result.ProtocolVersion != ActionPreflightProtocolVersion || result.Operation == "" || result.OperationVersion <= 0 || !hostUpdateInventoryHashPattern.MatchString(result.RequestDigest) {
		return fmt.Errorf("invalid action preflight result binding")
	}
	if result.CheckedAt.IsZero() {
		return fmt.Errorf("action preflight checked_at is required")
	}
	result.CheckedAt = result.CheckedAt.UTC()
	if result.Feasible && result.ReasonCode != "" {
		return fmt.Errorf("feasible action preflight cannot carry a refusal reason")
	}
	if !result.Feasible && !IsActionRefusalReasonCode(result.ReasonCode) {
		return fmt.Errorf("infeasible action preflight requires a valid refusal reason")
	}
	return nil
}

func ValidateActionPreflightResultForRequest(req ActionPreflightPayload, result ActionPreflightResultPayload, receivedAt time.Time) error {
	if err := ValidateActionPreflightPayload(&req); err != nil {
		return err
	}
	if err := ValidateActionPreflightResultPayload(&result); err != nil {
		return err
	}
	operation, version, digest := ActionPreflightBinding(req)
	if result.RequestID != req.RequestID || result.ProtocolVersion != req.ProtocolVersion || result.Operation != operation || result.OperationVersion != version || result.RequestDigest != digest {
		return fmt.Errorf("action preflight result does not match request binding")
	}
	receivedAt = receivedAt.UTC()
	if receivedAt.IsZero() || result.CheckedAt.After(receivedAt.Add(5*time.Minute)) || result.CheckedAt.Before(receivedAt.Add(-5*time.Minute)) {
		return fmt.Errorf("action preflight result is stale or has invalid chronology")
	}
	return nil
}
