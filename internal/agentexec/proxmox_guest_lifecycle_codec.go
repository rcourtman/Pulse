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

const (
	ProxmoxGuestLifecycleOperationVersion = 1
	ProxmoxGuestLifecycleReceiptKind      = "pulse.proxmox_guest_lifecycle_result"
	ProxmoxGuestLifecycleReceiptVersion   = 1
	maxProxmoxGuestLifecycleBytes         = 64 << 10
)

func DecodeProxmoxGuestLifecyclePayload(data []byte) (ProxmoxGuestLifecyclePayload, error) {
	var payload ProxmoxGuestLifecyclePayload
	if err := decodeStrictProxmoxLifecycle(data, &payload); err != nil {
		return payload, err
	}
	return payload, ValidateProxmoxGuestLifecyclePayload(&payload)
}

func DecodeProxmoxGuestLifecycleResultPayload(data []byte) (ProxmoxGuestLifecycleResultPayload, error) {
	var payload ProxmoxGuestLifecycleResultPayload
	if err := decodeStrictProxmoxLifecycle(data, &payload); err != nil {
		return payload, err
	}
	return payload, ValidateProxmoxGuestLifecycleResultPayload(&payload)
}

func decodeStrictProxmoxLifecycle(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 || len(data) > maxProxmoxGuestLifecycleBytes {
		return fmt.Errorf("proxmox guest lifecycle payload is empty or oversized")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("proxmox guest lifecycle payload contains trailing data")
	}
	return nil
}

func BindProxmoxGuestLifecyclePayload(payload *ProxmoxGuestLifecyclePayload) error {
	if payload == nil {
		return fmt.Errorf("proxmox guest lifecycle payload is required")
	}
	payload.OperationVersion = ProxmoxGuestLifecycleOperationVersion
	digest, err := proxmoxGuestLifecycleRequestDigest(*payload)
	if err != nil {
		return err
	}
	payload.RequestDigest = digest
	return nil
}

func proxmoxGuestLifecycleRequestDigest(payload ProxmoxGuestLifecyclePayload) (string, error) {
	return operationreceipt.DigestCanonicalJSON(struct {
		ActionID         string `json:"action_id"`
		Operation        string `json:"operation"`
		OperationVersion int    `json:"operation_version"`
		GuestKind        string `json:"guest_kind"`
		VMID             int    `json:"vmid"`
		ExpectedStatus   string `json:"expected_status"`
	}{strings.TrimSpace(payload.ActionID), strings.ToLower(strings.TrimSpace(payload.Operation)), payload.OperationVersion, strings.ToLower(strings.TrimSpace(payload.GuestKind)), payload.VMID, strings.ToLower(strings.TrimSpace(payload.ExpectedStatus))})
}

func ValidateProxmoxGuestLifecyclePayload(payload *ProxmoxGuestLifecyclePayload) error {
	if payload == nil {
		return fmt.Errorf("proxmox guest lifecycle payload is required")
	}
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.ActionID = strings.TrimSpace(payload.ActionID)
	payload.Operation = strings.ToLower(strings.TrimSpace(payload.Operation))
	payload.GuestKind = strings.ToLower(strings.TrimSpace(payload.GuestKind))
	payload.ExpectedStatus = strings.ToLower(strings.TrimSpace(payload.ExpectedStatus))
	if payload.RequestID == "" || len(payload.RequestID) > maxRequestIDLength || payload.ActionID == "" || len(payload.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid proxmox lifecycle request identity")
	}
	if payload.OperationVersion != ProxmoxGuestLifecycleOperationVersion || !isProxmoxGuestOperation(payload.Operation) {
		return fmt.Errorf("unsupported proxmox guest lifecycle operation or version")
	}
	if payload.GuestKind != "vm" && payload.GuestKind != "ct" {
		return fmt.Errorf("unsupported proxmox guest kind")
	}
	if payload.VMID < 1 || payload.VMID > 999999999 {
		return fmt.Errorf("proxmox vmid must be a bounded positive integer")
	}
	if payload.ExpectedStatus != "running" && payload.ExpectedStatus != "stopped" {
		return fmt.Errorf("expected proxmox guest status must be running or stopped")
	}
	want, err := proxmoxGuestLifecycleRequestDigest(*payload)
	if err != nil || payload.RequestDigest != want {
		return fmt.Errorf("proxmox guest lifecycle request digest mismatch")
	}
	if payload.Timeout < 0 || payload.Timeout > 300 {
		return fmt.Errorf("proxmox guest lifecycle timeout must be between 0 and 300 seconds")
	}
	if payload.Timeout == 0 {
		payload.Timeout = 180
	}
	return nil
}

func ValidateProxmoxGuestLifecycleResultPayload(result *ProxmoxGuestLifecycleResultPayload) error {
	if result == nil {
		return fmt.Errorf("proxmox guest lifecycle result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.ActionID = strings.TrimSpace(result.ActionID)
	result.Operation = strings.ToLower(strings.TrimSpace(result.Operation))
	result.GuestKind = strings.ToLower(strings.TrimSpace(result.GuestKind))
	result.RequestDigest = strings.TrimSpace(result.RequestDigest)
	result.ExecutionPhase = strings.TrimSpace(result.ExecutionPhase)
	result.ReasonCode = strings.TrimSpace(result.ReasonCode)
	result.Error = strings.TrimSpace(result.Error)
	if result.RequestID == "" || result.ActionID == "" || !isProxmoxGuestOperation(result.Operation) || result.OperationVersion != ProxmoxGuestLifecycleOperationVersion || (result.GuestKind != "vm" && result.GuestKind != "ct") || result.VMID < 1 || result.VMID > 999999999 || !hostUpdateInventoryHashPattern.MatchString(result.RequestDigest) {
		return fmt.Errorf("invalid proxmox guest lifecycle result binding")
	}
	switch result.ExecutionPhase {
	case ProxmoxGuestPhasePreflight, ProxmoxGuestPhaseMutate, ProxmoxGuestPhaseVerify, ProxmoxGuestPhaseComplete:
	default:
		return fmt.Errorf("unsupported proxmox guest lifecycle execution phase")
	}
	if len(result.Error) > 1024 || (result.ReasonCode != "" && !IsActionRefusalReasonCode(result.ReasonCode)) || (result.ReasonCode != "" && result.MutationStarted) || (result.MutationCompleted && !result.MutationStarted) {
		return fmt.Errorf("invalid proxmox guest lifecycle result state")
	}
	for _, snapshot := range []*ProxmoxGuestLifecycleSnapshot{&result.Before, &result.After} {
		snapshot.Status = strings.ToLower(strings.TrimSpace(snapshot.Status))
		if snapshot.Status != "" && snapshot.Status != "running" && snapshot.Status != "stopped" {
			return fmt.Errorf("invalid proxmox guest status")
		}
		if !snapshot.ObservedAt.IsZero() && snapshot.ObservedAt.Location() != time.UTC {
			return fmt.Errorf("proxmox observation timestamp must be UTC")
		}
	}
	if result.ReadbackRan && result.After.ObservedAt.IsZero() {
		return fmt.Errorf("proxmox readback requires an observation")
	}
	return nil
}

func ValidateProxmoxGuestLifecycleResultForRequest(req ProxmoxGuestLifecyclePayload, result ProxmoxGuestLifecycleResultPayload) error {
	if err := ValidateProxmoxGuestLifecyclePayload(&req); err != nil {
		return err
	}
	if err := ValidateProxmoxGuestLifecycleResultPayload(&result); err != nil {
		return err
	}
	if result.RequestID != req.RequestID || result.ActionID != req.ActionID || result.Operation != req.Operation || result.OperationVersion != req.OperationVersion || result.RequestDigest != req.RequestDigest || result.GuestKind != req.GuestKind || result.VMID != req.VMID {
		return fmt.Errorf("proxmox guest lifecycle result identity mismatch")
	}
	return nil
}

func ProxmoxGuestLifecycleOperationIdentity(agentID string, payload ProxmoxGuestLifecyclePayload) operationreceipt.Identity {
	return operationreceipt.Identity{AttemptID: payload.RequestID, ActionID: payload.ActionID, OperationKind: payload.Operation, OperationVersion: payload.OperationVersion, RequestDigest: payload.RequestDigest, AgentID: strings.TrimSpace(agentID)}
}

func isProxmoxGuestOperation(operation string) bool {
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "start", "stop", "shutdown", "reboot":
		return true
	default:
		return false
	}
}
