package actionrunner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
)

const (
	ProtocolVersion      = 1
	ReceiptKind          = "pulse.action_runner_result"
	ReceiptVersion       = 1
	MaxRequestBytes      = 64 << 10
	MaxResultBytes       = 32 << 10
	MaxOperationDeadline = 30 * time.Minute
	ActionCapability     = "typed_actions.v1"
	RuntimeRole          = "action-runner"
)

var boundedID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// IsValidBoundedID defines the closed identity vocabulary shared by action
// credential issuance, runner configuration, session admission, and receipts.
func IsValidBoundedID(value string) bool {
	return boundedID.MatchString(strings.TrimSpace(value))
}

var (
	ErrUnauthorized     = errors.New("action runner session is not authorized")
	ErrUnsupported      = errors.New("action runner operation is unsupported")
	ErrReplayInProgress = errors.New("action runner operation is already in progress")
	ErrNotActive        = errors.New("action runner operation is not active")
)

type Target struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type Request struct {
	ProtocolVersion  int             `json:"protocol_version"`
	OrganizationID   string          `json:"organization_id"`
	HostID           string          `json:"host_id"`
	AttemptID        string          `json:"attempt_id"`
	ActionID         string          `json:"action_id"`
	Operation        string          `json:"operation"`
	OperationVersion int             `json:"operation_version"`
	RequestDigest    string          `json:"request_digest"`
	Target           Target          `json:"target"`
	Deadline         time.Time       `json:"deadline"`
	Payload          json.RawMessage `json:"payload"`
}

type Session struct {
	OrganizationID string
	HostID         string
	TokenID        string
	Capabilities   map[string]bool
}

// Registration is the transport-neutral identity emitted when this runtime
// opens an action session. Collector registrations intentionally do not use
// this type, so the server can fail closed on runtime role.
type Registration struct {
	RuntimeRole    string `json:"runtime_role"`
	OrganizationID string `json:"organization_id"`
	HostID         string `json:"host_id"`
}

func (session Session) Registration() (Registration, error) {
	organizationID := strings.TrimSpace(session.OrganizationID)
	hostID := strings.TrimSpace(session.HostID)
	if !IsValidBoundedID(organizationID) || !IsValidBoundedID(hostID) || !IsValidBoundedID(session.TokenID) || !session.Capabilities[ActionCapability] {
		return Registration{}, ErrUnauthorized
	}
	return Registration{RuntimeRole: RuntimeRole, OrganizationID: organizationID, HostID: hostID}, nil
}

type ResultStatus string

const (
	ResultSucceeded ResultStatus = "succeeded"
	ResultRefused   ResultStatus = "refused"
	ResultFailed    ResultStatus = "failed"
	ResultCanceled  ResultStatus = "canceled"
	ResultDeadline  ResultStatus = "deadline_exceeded"
)

type Result struct {
	Status     ResultStatus    `json:"status"`
	ReasonCode string          `json:"reason_code,omitempty"`
	Output     json.RawMessage `json:"output,omitempty"`
}

type TerminalResult struct {
	ProtocolVersion  int             `json:"protocol_version"`
	AttemptID        string          `json:"attempt_id"`
	ActionID         string          `json:"action_id"`
	Operation        string          `json:"operation"`
	OperationVersion int             `json:"operation_version"`
	RequestDigest    string          `json:"request_digest"`
	Target           Target          `json:"target"`
	Status           ResultStatus    `json:"status"`
	ReasonCode       string          `json:"reason_code,omitempty"`
	Output           json.RawMessage `json:"output,omitempty"`
}

func DecodeRequest(data []byte, now time.Time) (Request, error) {
	if len(data) > MaxRequestBytes {
		return Request{}, fmt.Errorf("action request exceeds %d bytes", MaxRequestBytes)
	}
	var request Request
	if err := decodeStrict(data, &request); err != nil {
		return Request{}, err
	}
	if err := ValidateRequest(&request, now); err != nil {
		return Request{}, err
	}
	return request, nil
}

func ValidateRequest(request *Request, now time.Time) error {
	if request == nil {
		return fmt.Errorf("action request is required")
	}
	request.OrganizationID = strings.TrimSpace(request.OrganizationID)
	request.HostID = strings.TrimSpace(request.HostID)
	request.AttemptID = strings.TrimSpace(request.AttemptID)
	request.ActionID = strings.TrimSpace(request.ActionID)
	request.Operation = strings.ToLower(strings.TrimSpace(request.Operation))
	request.RequestDigest = strings.ToLower(strings.TrimSpace(request.RequestDigest))
	request.Target.Kind = strings.ToLower(strings.TrimSpace(request.Target.Kind))
	request.Target.ID = strings.TrimSpace(request.Target.ID)
	request.Deadline = request.Deadline.UTC()
	if request.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported action protocol version %d", request.ProtocolVersion)
	}
	for label, value := range map[string]string{"organization_id": request.OrganizationID, "host_id": request.HostID, "attempt_id": request.AttemptID, "action_id": request.ActionID, "target.kind": request.Target.Kind, "target.id": request.Target.ID} {
		if !IsValidBoundedID(value) {
			return fmt.Errorf("invalid %s", label)
		}
	}
	if !AllowedOperation(request.Operation, request.OperationVersion) {
		return fmt.Errorf("%w: %s v%d", ErrUnsupported, request.Operation, request.OperationVersion)
	}
	switch {
	case strings.HasPrefix(request.Operation, "host."):
		if request.Target.Kind != "host" || request.Target.ID != request.HostID {
			return fmt.Errorf("host action target does not match the bound host")
		}
	case strings.HasPrefix(request.Operation, "proxmox."):
		if request.Target.Kind != "proxmox-guest" {
			return fmt.Errorf("Proxmox action requires a Proxmox guest target")
		}
	case strings.HasPrefix(request.Operation, "container."):
		if request.Target.Kind != "container" {
			return fmt.Errorf("container action requires a container target")
		}
	}
	if len(request.Payload) == 0 || len(request.Payload) > MaxRequestBytes || !json.Valid(request.Payload) {
		return fmt.Errorf("action payload is empty, invalid, or oversized")
	}
	if request.Deadline.IsZero() || !request.Deadline.After(now.UTC()) || request.Deadline.After(now.UTC().Add(MaxOperationDeadline)) {
		return fmt.Errorf("action deadline must be in the future and at most %s", MaxOperationDeadline)
	}
	digest, err := RequestDigest(*request)
	if err != nil {
		return err
	}
	if request.RequestDigest != digest {
		return fmt.Errorf("action request digest mismatch")
	}
	return nil
}

func RequestDigest(request Request) (string, error) {
	canonical := struct {
		OrganizationID   string          `json:"organization_id"`
		HostID           string          `json:"host_id"`
		ActionID         string          `json:"action_id"`
		Operation        string          `json:"operation"`
		OperationVersion int             `json:"operation_version"`
		Target           Target          `json:"target"`
		Payload          json.RawMessage `json:"payload"`
	}{request.OrganizationID, request.HostID, request.ActionID, request.Operation, request.OperationVersion, request.Target, request.Payload}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func AllowedOperation(operation string, version int) bool {
	if version != 1 {
		return false
	}
	switch operation {
	case "host.update", "host.storage_cleanup",
		"proxmox.guest.start", "proxmox.guest.stop", "proxmox.guest.shutdown", "proxmox.guest.reboot",
		"container.start", "container.stop", "container.restart", "container.update":
		return true
	default:
		return false
	}
}

func ReceiptIdentity(request Request, tokenID string) operationreceipt.Identity {
	return operationreceipt.Identity{AttemptID: request.AttemptID, ActionID: request.ActionID, OperationKind: request.Operation, OperationVersion: request.OperationVersion, RequestDigest: request.RequestDigest, AgentID: strings.TrimSpace(tokenID)}
}

func ValidateTerminal(identity operationreceipt.Identity, data json.RawMessage) error {
	var result TerminalResult
	if err := decodeStrict(data, &result); err != nil {
		return err
	}
	if result.ProtocolVersion != ProtocolVersion || result.AttemptID != identity.AttemptID || result.ActionID != identity.ActionID || result.Operation != identity.OperationKind || result.OperationVersion != identity.OperationVersion || result.RequestDigest != identity.RequestDigest {
		return operationreceipt.ErrBindingConflict
	}
	if !IsValidBoundedID(result.Target.Kind) || !IsValidBoundedID(result.Target.ID) {
		return fmt.Errorf("invalid action result target")
	}
	switch result.Status {
	case ResultSucceeded:
		if result.ReasonCode != "" {
			return fmt.Errorf("successful action cannot include a reason code")
		}
	case ResultRefused, ResultFailed, ResultCanceled, ResultDeadline:
		if !validReasonCode(result.ReasonCode) {
			return fmt.Errorf("non-success action requires a bounded reason code")
		}
	default:
		return fmt.Errorf("unsupported action result status %q", result.Status)
	}
	if len(result.Output) > MaxResultBytes || (len(result.Output) > 0 && !json.Valid(result.Output)) {
		return fmt.Errorf("action result output is invalid or oversized")
	}
	return nil
}

func validReasonCode(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}

func decodeStrict(data []byte, target any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("action payload is empty")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("action payload contains trailing JSON")
		}
		return fmt.Errorf("action payload contains trailing data: %w", err)
	}
	return nil
}
