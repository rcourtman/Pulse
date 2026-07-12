package agentexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

func decodeStrictAPTPayload(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("APT payload is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("APT payload contains trailing JSON")
		}
		return fmt.Errorf("APT payload contains trailing data: %w", err)
	}
	return nil
}

func DecodeHostUpdatePayload(data []byte) (HostUpdatePayload, error) {
	var payload HostUpdatePayload
	if err := decodeStrictAPTPayload(data, &payload); err != nil {
		return HostUpdatePayload{}, err
	}
	if err := ValidateHostUpdatePayload(&payload); err != nil {
		return HostUpdatePayload{}, err
	}
	return payload, nil
}

func DecodeHostUpdateResultPayload(data []byte) (HostUpdateResultPayload, error) {
	var payload HostUpdateResultPayload
	if err := decodeStrictAPTPayload(data, &payload); err != nil {
		return HostUpdateResultPayload{}, err
	}
	if err := ValidateHostUpdateResultPayload(&payload); err != nil {
		return HostUpdateResultPayload{}, err
	}
	return payload, nil
}

func DecodeHostStorageCleanupPayload(data []byte) (HostStorageCleanupPayload, error) {
	var payload HostStorageCleanupPayload
	if err := decodeStrictAPTPayload(data, &payload); err != nil {
		return HostStorageCleanupPayload{}, err
	}
	if err := ValidateHostStorageCleanupPayload(&payload); err != nil {
		return HostStorageCleanupPayload{}, err
	}
	return payload, nil
}

func DecodeHostStorageCleanupResultPayload(data []byte) (HostStorageCleanupResultPayload, error) {
	var payload HostStorageCleanupResultPayload
	if err := decodeStrictAPTPayload(data, &payload); err != nil {
		return HostStorageCleanupResultPayload{}, err
	}
	if err := ValidateHostStorageCleanupResultPayload(&payload); err != nil {
		return HostStorageCleanupResultPayload{}, err
	}
	return payload, nil
}

func ValidateHostUpdatePayload(payload *HostUpdatePayload) error {
	return validateHostUpdatePayload(payload)
}

func ValidateHostStorageCleanupPayload(payload *HostStorageCleanupPayload) error {
	return validateHostStorageCleanupPayload(payload)
}

const HostAPTOperationVersion = 1

const (
	HostUpdateReceiptKind         = "pulse.host_update_result"
	HostStorageCleanupReceiptKind = "pulse.host_storage_cleanup_result"
	HostAPTReceiptVersion         = 1
)

func ValidateOperationQueryResultForIdentity(result operationreceipt.QueryResult, identity operationreceipt.Identity, receivedAt time.Time) error {
	if result.Status == operationreceipt.QueryNotFound {
		return nil
	}
	if result.Record == nil || result.Record.Identity != identity {
		return fmt.Errorf("operation query result identity mismatch")
	}
	if result.Status != operationreceipt.QueryFoundTerminal {
		return nil
	}
	switch identity.OperationKind {
	case HostUpdateOperationInstall:
		if result.Record.ResultKind != HostUpdateReceiptKind || result.Record.ResultVersion != HostAPTReceiptVersion {
			return fmt.Errorf("host update query result envelope mismatch")
		}
		payload, err := DecodeHostUpdateResultPayload(result.Record.Result)
		if err != nil {
			return err
		}
		req := HostUpdatePayload{RequestID: identity.AttemptID, ActionID: identity.ActionID, Operation: identity.OperationKind, OperationVersion: identity.OperationVersion, RequestDigest: identity.RequestDigest, ExpectedInventoryHash: payload.Before.InventoryHash}
		if err := ValidateHostUpdatePayload(&req); err != nil {
			return err
		}
		return ValidateHostUpdateResultForRequestAt(req, payload, receivedAt)
	case HostStorageCleanupOperationPackageCache:
		if result.Record.ResultKind != HostStorageCleanupReceiptKind || result.Record.ResultVersion != HostAPTReceiptVersion {
			return fmt.Errorf("host cleanup query result envelope mismatch")
		}
		payload, err := DecodeHostStorageCleanupResultPayload(result.Record.Result)
		if err != nil {
			return err
		}
		req := HostStorageCleanupPayload{RequestID: identity.AttemptID, ActionID: identity.ActionID, Operation: identity.OperationKind, OperationVersion: identity.OperationVersion, RequestDigest: identity.RequestDigest, ExpectedFingerprint: payload.Before.Fingerprint}
		if err := ValidateHostStorageCleanupPayload(&req); err != nil {
			return err
		}
		return ValidateHostStorageCleanupResultForRequestAt(req, payload, receivedAt)
	default:
		return fmt.Errorf("unsupported operation query kind %q", identity.OperationKind)
	}
}

func BindHostUpdatePayload(payload *HostUpdatePayload) error {
	if payload == nil {
		return fmt.Errorf("host update payload is required")
	}
	payload.OperationVersion = HostAPTOperationVersion
	digest, err := hostUpdateRequestDigest(*payload)
	if err != nil {
		return err
	}
	payload.RequestDigest = digest
	return nil
}

func hostUpdateRequestDigest(payload HostUpdatePayload) (string, error) {
	return operationreceipt.DigestCanonicalJSON(struct {
		ActionID              string `json:"action_id"`
		Operation             string `json:"operation"`
		OperationVersion      int    `json:"operation_version"`
		ExpectedInventoryHash string `json:"expected_inventory_hash"`
	}{strings.TrimSpace(payload.ActionID), strings.TrimSpace(payload.Operation), payload.OperationVersion, strings.TrimSpace(payload.ExpectedInventoryHash)})
}

func BindHostStorageCleanupPayload(payload *HostStorageCleanupPayload) error {
	if payload == nil {
		return fmt.Errorf("host storage cleanup payload is required")
	}
	payload.OperationVersion = HostAPTOperationVersion
	digest, err := hostStorageCleanupRequestDigest(*payload)
	if err != nil {
		return err
	}
	payload.RequestDigest = digest
	return nil
}

func hostStorageCleanupRequestDigest(payload HostStorageCleanupPayload) (string, error) {
	return operationreceipt.DigestCanonicalJSON(struct {
		ActionID            string `json:"action_id"`
		Operation           string `json:"operation"`
		OperationVersion    int    `json:"operation_version"`
		ExpectedFingerprint string `json:"expected_fingerprint"`
	}{strings.TrimSpace(payload.ActionID), strings.TrimSpace(payload.Operation), payload.OperationVersion, strings.TrimSpace(payload.ExpectedFingerprint)})
}

func HostUpdateOperationIdentity(agentID string, payload HostUpdatePayload) operationreceipt.Identity {
	return operationreceipt.Identity{AttemptID: payload.RequestID, ActionID: payload.ActionID, OperationKind: payload.Operation, OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, AgentID: agentID}
}

func HostStorageCleanupOperationIdentity(agentID string, payload HostStorageCleanupPayload) operationreceipt.Identity {
	return operationreceipt.Identity{AttemptID: payload.RequestID, ActionID: payload.ActionID, OperationKind: payload.Operation, OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, AgentID: agentID}
}

func ValidateHostUpdateResultPayload(result *HostUpdateResultPayload) error {
	if err := validateHostUpdateResultPayload(result); err != nil {
		return err
	}
	result.ActionID = strings.TrimSpace(result.ActionID)
	result.ExecutionPhase = strings.TrimSpace(result.ExecutionPhase)
	if result.ActionID == "" || len(result.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid action id")
	}
	switch result.ExecutionPhase {
	case HostUpdatePhasePreflight, HostUpdatePhaseRefresh, HostUpdatePhaseInstall, HostUpdatePhaseVerify, HostUpdatePhaseComplete:
	default:
		return fmt.Errorf("unsupported host update execution phase %q", result.ExecutionPhase)
	}
	if result.Success && result.ExecutionPhase != HostUpdatePhaseComplete {
		return fmt.Errorf("successful host update must be complete")
	}
	if (result.Verification == HostUpdateVerificationVerified || result.Verification == HostUpdateVerificationFailed) && !validAPTResultObservationChronology(result.Before.CheckedAt, result.After.CheckedAt) {
		return fmt.Errorf("evidence-bearing host update observation timestamps are invalid")
	}
	if result.MutationStarted && result.ExecutionPhase != HostUpdatePhaseRefresh && result.ExecutionPhase != HostUpdatePhaseInstall && result.ExecutionPhase != HostUpdatePhaseVerify && result.ExecutionPhase != HostUpdatePhaseComplete {
		return fmt.Errorf("host update mutation state conflicts with execution phase")
	}
	return nil
}

func ValidateHostStorageCleanupResultPayload(result *HostStorageCleanupResultPayload) error {
	if err := validateHostStorageCleanupResultPayload(result); err != nil {
		return err
	}
	result.ActionID = strings.TrimSpace(result.ActionID)
	result.ExecutionPhase = strings.TrimSpace(result.ExecutionPhase)
	if result.ActionID == "" || len(result.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid action id")
	}
	switch result.ExecutionPhase {
	case HostStorageCleanupPhasePreflight, HostStorageCleanupPhaseClean, HostStorageCleanupPhaseVerify, HostStorageCleanupPhaseComplete:
	default:
		return fmt.Errorf("unsupported host storage cleanup execution phase %q", result.ExecutionPhase)
	}
	if result.Success && result.ExecutionPhase != HostStorageCleanupPhaseComplete {
		return fmt.Errorf("successful host storage cleanup must be complete")
	}
	if (result.Verification == HostStorageCleanupVerificationVerified || result.Verification == HostStorageCleanupVerificationFailed) && !validAPTResultObservationChronology(result.Before.CheckedAt, result.After.CheckedAt) {
		return fmt.Errorf("evidence-bearing host storage cleanup observation timestamps are invalid")
	}
	if result.MutationStarted && result.ExecutionPhase != HostStorageCleanupPhaseClean && result.ExecutionPhase != HostStorageCleanupPhaseVerify && result.ExecutionPhase != HostStorageCleanupPhaseComplete {
		return fmt.Errorf("host storage cleanup mutation state conflicts with execution phase")
	}
	return nil
}

func validAPTResultObservationChronology(before, after time.Time) bool {
	return !before.IsZero() && !after.IsZero() && !after.Before(before)
}

func ValidateHostUpdateResultForRequest(req HostUpdatePayload, result HostUpdateResultPayload) error {
	if err := ValidateHostUpdateResultPayload(&result); err != nil {
		return err
	}
	if result.RequestID != req.RequestID || result.ActionID != req.ActionID {
		return fmt.Errorf("host update result identity does not match request")
	}
	if result.Verification == HostUpdateVerificationVerified || result.Verification == HostUpdateVerificationFailed {
		if result.Before.InventoryHash != req.ExpectedInventoryHash {
			return fmt.Errorf("evidence-bearing host update before-state does not match request")
		}
	}
	return nil
}

func ValidateHostUpdateResultForRequestAt(req HostUpdatePayload, result HostUpdateResultPayload, receivedAt time.Time) error {
	if err := ValidateHostUpdateResultForRequest(req, result); err != nil {
		return err
	}
	if (result.Verification == HostUpdateVerificationVerified || result.Verification == HostUpdateVerificationFailed) && !validAPTResultObservationAt(result.Before.CheckedAt, result.After.CheckedAt, receivedAt) {
		return fmt.Errorf("host update result observation is stale or skewed")
	}
	return nil
}

func ValidateHostStorageCleanupResultForRequest(req HostStorageCleanupPayload, result HostStorageCleanupResultPayload) error {
	if err := ValidateHostStorageCleanupResultPayload(&result); err != nil {
		return err
	}
	if result.RequestID != req.RequestID || result.ActionID != req.ActionID {
		return fmt.Errorf("host storage cleanup result identity does not match request")
	}
	if result.Verification == HostStorageCleanupVerificationVerified || result.Verification == HostStorageCleanupVerificationFailed {
		if result.Before.Fingerprint != req.ExpectedFingerprint {
			return fmt.Errorf("evidence-bearing host storage cleanup before-state does not match request")
		}
	}
	return nil
}

func ValidateHostStorageCleanupResultForRequestAt(req HostStorageCleanupPayload, result HostStorageCleanupResultPayload, receivedAt time.Time) error {
	if err := ValidateHostStorageCleanupResultForRequest(req, result); err != nil {
		return err
	}
	if (result.Verification == HostStorageCleanupVerificationVerified || result.Verification == HostStorageCleanupVerificationFailed) && !validAPTResultObservationAt(result.Before.CheckedAt, result.After.CheckedAt, receivedAt) {
		return fmt.Errorf("host storage cleanup result observation is stale or skewed")
	}
	return nil
}

const hostAPTResultFreshness = 15 * time.Minute

func validAPTResultObservationAt(before, after, receivedAt time.Time) bool {
	if receivedAt.IsZero() || !validAPTResultObservationChronology(before, after) {
		return false
	}
	receivedAt = receivedAt.UTC()
	after = after.UTC()
	return !after.Before(receivedAt.Add(-hostAPTResultFreshness)) && !after.After(receivedAt.Add(5*time.Minute))
}
