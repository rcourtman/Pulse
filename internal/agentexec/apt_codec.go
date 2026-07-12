package agentexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
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
