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

// ValidateOperationQueryResultForIdentity validates a durable receipt against
// its immutable dispatch identity. Receipt replay can happen long after the
// agent observed the terminal result, so evidence freshness is anchored to the
// durable agent-authored terminal boundary rather than the later query time.
// receivedAt remains the server's transport receipt time and is used only to
// reject an implausibly future agent terminal timestamp.
func ValidateOperationQueryResultForIdentity(result operationreceipt.QueryResult, identity operationreceipt.Identity, receivedAt time.Time) error {
	if result.Version != operationreceipt.ProtocolVersion {
		return fmt.Errorf("unsupported operation query result version %d", result.Version)
	}
	if result.Status == operationreceipt.QueryNotFound {
		if result.Record != nil {
			return fmt.Errorf("not-found operation query result contains a record")
		}
		return nil
	}
	if result.Record == nil || result.Record.Identity != identity {
		return fmt.Errorf("operation query result identity mismatch")
	}
	if err := operationreceipt.ValidateRecord(*result.Record); err != nil {
		return err
	}
	if result.Status != operationreceipt.QueryFoundTerminal {
		if result.Status != operationreceipt.QueryFoundInterrupted || result.Record.State == operationreceipt.StateTerminal {
			return fmt.Errorf("operation query result status and record state mismatch")
		}
		return nil
	}
	if result.Record.State != operationreceipt.StateTerminal {
		return fmt.Errorf("terminal operation query result requires terminal record")
	}
	if receivedAt.IsZero() || result.Record.TerminalAt.After(receivedAt.UTC().Add(5*time.Minute)) {
		return fmt.Errorf("operation query terminal time is invalid or implausibly future")
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
		if err := ValidateHostUpdateReceiptForIdentity(identity, payload); err != nil {
			return err
		}
		return validateDurableAPTObservation(payload.Before.CheckedAt, payload.After.CheckedAt, result.Record.TerminalAt)
	case HostStorageCleanupOperationPackageCache:
		if result.Record.ResultKind != HostStorageCleanupReceiptKind || result.Record.ResultVersion != HostAPTReceiptVersion {
			return fmt.Errorf("host cleanup query result envelope mismatch")
		}
		payload, err := DecodeHostStorageCleanupResultPayload(result.Record.Result)
		if err != nil {
			return err
		}
		if err := ValidateHostStorageCleanupReceiptForIdentity(identity, payload); err != nil {
			return err
		}
		return validateDurableAPTObservation(payload.Before.CheckedAt, payload.After.CheckedAt, result.Record.TerminalAt)
	case DockerContainerOperationStart, DockerContainerOperationStop, DockerContainerOperationRestart:
		if result.Record.ResultKind != DockerContainerLifecycleReceiptKind || result.Record.ResultVersion != DockerContainerLifecycleReceiptVersion {
			return fmt.Errorf("docker container lifecycle query result envelope mismatch")
		}
		payload, err := DecodeDockerContainerLifecycleResultPayload(result.Record.Result)
		if err != nil {
			return err
		}
		if payload.RequestID != identity.AttemptID || payload.ActionID != identity.ActionID || payload.Operation != identity.OperationKind || payload.OperationVersion != identity.OperationVersion || payload.RequestDigest != identity.RequestDigest {
			return operationreceipt.ErrBindingConflict
		}
		if payload.ReadbackRan && (payload.After.ObservedAt.IsZero() || result.Record.TerminalAt.Before(payload.After.ObservedAt)) {
			return fmt.Errorf("docker container lifecycle readback has invalid terminal chronology")
		}
		return nil
	case DockerContainerOperationUpdate:
		if result.Record.ResultKind != DockerContainerUpdateReceiptKind || result.Record.ResultVersion != DockerContainerUpdateReceiptVersion {
			return fmt.Errorf("docker container update query result envelope mismatch")
		}
		payload, err := DecodeDockerContainerUpdateResultPayload(result.Record.Result)
		if err != nil {
			return err
		}
		if payload.RequestID != identity.AttemptID || payload.ActionID != identity.ActionID || payload.Operation != identity.OperationKind || payload.OperationVersion != identity.OperationVersion || payload.RequestDigest != identity.RequestDigest {
			return operationreceipt.ErrBindingConflict
		}
		if payload.ReadbackRan && (payload.After.ObservedAt.IsZero() || result.Record.TerminalAt.Before(payload.After.ObservedAt)) {
			return fmt.Errorf("docker container update readback has invalid terminal chronology")
		}
		return nil
	default:
		return fmt.Errorf("unsupported operation query kind %q", identity.OperationKind)
	}
}

func validateDurableAPTObservation(before, after, terminalAt time.Time) error {
	if !validAPTResultObservationChronology(before, after) || terminalAt.IsZero() || terminalAt.Before(after) || after.Before(terminalAt.Add(-hostAPTResultFreshness)) {
		return fmt.Errorf("durable APT result observation is stale or has invalid terminal chronology")
	}
	return nil
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
	if result.Success && result.ExecutionPhase != HostUpdatePhaseVerify && result.ExecutionPhase != HostUpdatePhaseComplete {
		return fmt.Errorf("successful host update mutation must be in verify or complete phase")
	}
	if (result.Verification == HostUpdateVerificationVerified || result.Verification == HostUpdateVerificationFailed) && !validAPTResultObservationChronology(result.Before.CheckedAt, result.After.CheckedAt) {
		return fmt.Errorf("evidence-bearing host update observation timestamps are invalid")
	}
	if result.MutationStarted && result.ExecutionPhase != HostUpdatePhaseInstall && result.ExecutionPhase != HostUpdatePhaseVerify && result.ExecutionPhase != HostUpdatePhaseComplete {
		return fmt.Errorf("host update mutation state conflicts with execution phase")
	}
	if result.RecoveryRequired && !result.MutationStarted {
		return fmt.Errorf("host update recovery requirement conflicts with mutation state")
	}
	if result.PackageManagerHealthy && !result.HealthChecked {
		return fmt.Errorf("healthy package manager claim requires a completed health check")
	}
	if result.HealthChecked && !result.PackageManagerHealthy && result.MutationStarted && !result.RecoveryRequired {
		return fmt.Errorf("unhealthy package manager after mutation requires recovery")
	}
	if result.ExecutionPhase == HostUpdatePhaseInstall && result.MutationStarted && !result.Success && !result.RecoveryRequired {
		return fmt.Errorf("partial host update install requires recovery")
	}
	if result.Success && result.ExecutionPhase == HostUpdatePhaseComplete && result.RecoveryRequired {
		return fmt.Errorf("successful host update completion cannot require recovery")
	}
	if result.Verification == HostUpdateVerificationVerified && result.ExecutionPhase != HostUpdatePhaseComplete {
		return fmt.Errorf("verified host update must be complete")
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
	if result.Success && result.ExecutionPhase != HostStorageCleanupPhaseVerify && result.ExecutionPhase != HostStorageCleanupPhaseComplete {
		return fmt.Errorf("successful host storage cleanup mutation must be in verify or complete phase")
	}
	if result.Verification == HostStorageCleanupVerificationVerified && result.ExecutionPhase != HostStorageCleanupPhaseComplete {
		return fmt.Errorf("verified host storage cleanup must be complete")
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

// ValidateHostUpdateReceiptForIdentity validates a terminal result against the
// immutable identity admitted by the generic operation-receipt store. An
// inconclusive drift result intentionally reports the newly observed inventory
// in Before, so that observation must not be substituted for the originally
// authorized inventory when checking the admitted request digest. The store
// itself binds Complete and Query to the exact admitted identity. Evidence-
// bearing results additionally prove the original before-state by deriving the
// digest from their matching observation.
func ValidateHostUpdateReceiptForIdentity(identity operationreceipt.Identity, result HostUpdateResultPayload) error {
	return validateHostAPTReceiptForIdentity(
		identity, HostUpdateOperationInstall, result.RequestID, result.ActionID,
		func() error { return ValidateHostUpdateResultPayload(&result) },
		result.Verification == HostUpdateVerificationVerified || result.Verification == HostUpdateVerificationFailed,
		func() error {
			req := HostUpdatePayload{RequestID: identity.AttemptID, ActionID: identity.ActionID, Operation: identity.OperationKind, OperationVersion: identity.OperationVersion, RequestDigest: identity.RequestDigest, ExpectedInventoryHash: result.Before.InventoryHash}
			if err := ValidateHostUpdatePayload(&req); err != nil {
				return err
			}
			return ValidateHostUpdateResultForRequest(req, result)
		},
	)
}

// validateHostAPTReceiptForIdentity is the shared receipt-binding core of the
// per-operation ReceiptForIdentity validators: bind the terminal result to the
// exact admitted identity, validate the result payload, and — only for
// evidence-bearing verifications — re-validate against a request reconstructed
// from the admitted identity.
func validateHostAPTReceiptForIdentity(
	identity operationreceipt.Identity,
	operationKind string,
	resultRequestID, resultActionID string,
	validateResultPayload func() error,
	evidenceBearing bool,
	validateEvidence func() error,
) error {
	normalized, err := operationreceipt.NormalizeIdentity(identity)
	if err != nil || normalized != identity {
		return operationreceipt.ErrBindingConflict
	}
	if identity.OperationKind != operationKind || identity.OperationVersion != HostAPTOperationVersion || resultRequestID != identity.AttemptID || resultActionID != identity.ActionID {
		return operationreceipt.ErrBindingConflict
	}
	if err := validateResultPayload(); err != nil {
		return err
	}
	if !evidenceBearing {
		return nil
	}
	return validateEvidence()
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

// ValidateHostStorageCleanupReceiptForIdentity is the cleanup counterpart to
// ValidateHostUpdateReceiptForIdentity. Fingerprint drift is terminal,
// inconclusive evidence and may report a different observed Before value while
// the generic store continues to bind the receipt to the admitted digest.
func ValidateHostStorageCleanupReceiptForIdentity(identity operationreceipt.Identity, result HostStorageCleanupResultPayload) error {
	return validateHostAPTReceiptForIdentity(
		identity, HostStorageCleanupOperationPackageCache, result.RequestID, result.ActionID,
		func() error { return ValidateHostStorageCleanupResultPayload(&result) },
		result.Verification == HostStorageCleanupVerificationVerified || result.Verification == HostStorageCleanupVerificationFailed,
		func() error {
			req := HostStorageCleanupPayload{RequestID: identity.AttemptID, ActionID: identity.ActionID, Operation: identity.OperationKind, OperationVersion: identity.OperationVersion, RequestDigest: identity.RequestDigest, ExpectedFingerprint: result.Before.Fingerprint}
			if err := ValidateHostStorageCleanupPayload(&req); err != nil {
				return err
			}
			return ValidateHostStorageCleanupResultForRequest(req, result)
		},
	)
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
