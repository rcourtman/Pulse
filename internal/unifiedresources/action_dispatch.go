package unifiedresources

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ActionDispatchState records transport authority only. It deliberately does
// not describe execution success, verification, evidence, or compensation.
type ActionDispatchState string

const (
	ActionDispatchQueued          ActionDispatchState = "queued"
	ActionDispatchClaimed         ActionDispatchState = "claimed"
	ActionDispatchReceiptPending  ActionDispatchState = "receipt_pending"
	ActionDispatchReceiptRecorded ActionDispatchState = "receipt_recorded"
)

// ActionDispatchAttempt is the create-once durable identity for sending one
// canonical action to its mutation transport.
type ActionDispatchAttempt struct {
	ID               string              `json:"id"`
	ActionID         string              `json:"actionId"`
	State            ActionDispatchState `json:"state"`
	CreatedAt        time.Time           `json:"createdAt"`
	UpdatedAt        time.Time           `json:"updatedAt"`
	LeaseOwner       string              `json:"leaseOwner,omitempty"`
	LeaseExpiresAt   time.Time           `json:"leaseExpiresAt,omitempty"`
	DispatchCount    int                 `json:"dispatchCount"`
	OperationKind    string              `json:"operationKind,omitempty"`
	OperationVersion int                 `json:"operationVersion,omitempty"`
	RequestDigest    string              `json:"requestDigest,omitempty"`
	AgentID          string              `json:"agentId,omitempty"`
}

type ActionDispatchBinding struct {
	OperationKind    string
	OperationVersion int
	RequestDigest    string
	AgentID          string
}

func (a ActionDispatchAttempt) HasOperationBinding() bool {
	return strings.TrimSpace(a.OperationKind) != "" && a.OperationVersion > 0 && strings.TrimSpace(a.RequestDigest) != "" && strings.TrimSpace(a.AgentID) != ""
}

// ActionDispatchReceipt proves only that the transport answered the committed
// attempt. It intentionally contains no result or verification truth.
type ActionDispatchReceipt struct {
	AttemptID          string    `json:"attemptId"`
	ActionID           string    `json:"actionId"`
	TransportRequestID string    `json:"transportRequestId"`
	ReceivedAt         time.Time `json:"receivedAt"`
}

var (
	ErrActionDispatchNotFound        = errors.New("action dispatch attempt not found")
	ErrActionDispatchNotClaimable    = errors.New("action dispatch attempt is not claimable")
	ErrActionDispatchLeaseMismatch   = errors.New("action dispatch lease owner mismatch")
	ErrActionDispatchReceiptConflict = errors.New("action dispatch receipt conflicts with persisted correlation")
)

func ActionDispatchAttemptID(actionID string) string {
	actionID = strings.TrimSpace(actionID)
	if actionID == "" {
		return ""
	}
	return actionID + ".dispatch.1"
}

func NewActionDispatchAttempt(actionID string, now time.Time) (ActionDispatchAttempt, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	return NormalizeActionDispatchAttempt(ActionDispatchAttempt{
		ID: ActionDispatchAttemptID(actionID), ActionID: strings.TrimSpace(actionID),
		State: ActionDispatchQueued, CreatedAt: now, UpdatedAt: now,
	})
}

func NormalizeActionDispatchAttempt(attempt ActionDispatchAttempt) (ActionDispatchAttempt, error) {
	attempt.ID = strings.TrimSpace(attempt.ID)
	attempt.ActionID = strings.TrimSpace(attempt.ActionID)
	attempt.LeaseOwner = strings.TrimSpace(attempt.LeaseOwner)
	attempt.OperationKind = strings.TrimSpace(attempt.OperationKind)
	attempt.RequestDigest = strings.TrimSpace(attempt.RequestDigest)
	attempt.AgentID = strings.TrimSpace(attempt.AgentID)
	if attempt.ActionID == "" {
		return ActionDispatchAttempt{}, fmt.Errorf("action dispatch action id required")
	}
	if attempt.ID == "" {
		attempt.ID = ActionDispatchAttemptID(attempt.ActionID)
	}
	if attempt.ID != ActionDispatchAttemptID(attempt.ActionID) {
		return ActionDispatchAttempt{}, fmt.Errorf("action dispatch attempt id %q does not match action %q", attempt.ID, attempt.ActionID)
	}
	switch attempt.State {
	case ActionDispatchQueued, ActionDispatchClaimed, ActionDispatchReceiptPending, ActionDispatchReceiptRecorded:
	default:
		return ActionDispatchAttempt{}, fmt.Errorf("unsupported action dispatch state %q", attempt.State)
	}
	if attempt.CreatedAt.IsZero() {
		return ActionDispatchAttempt{}, fmt.Errorf("action dispatch createdAt required")
	}
	attempt.CreatedAt = attempt.CreatedAt.UTC()
	if attempt.UpdatedAt.IsZero() {
		attempt.UpdatedAt = attempt.CreatedAt
	} else {
		attempt.UpdatedAt = attempt.UpdatedAt.UTC()
	}
	if !attempt.LeaseExpiresAt.IsZero() {
		attempt.LeaseExpiresAt = attempt.LeaseExpiresAt.UTC()
	}
	if attempt.State != ActionDispatchClaimed {
		attempt.LeaseOwner = ""
		attempt.LeaseExpiresAt = time.Time{}
	}
	if attempt.DispatchCount < 0 {
		return ActionDispatchAttempt{}, fmt.Errorf("action dispatch count cannot be negative")
	}
	bound := attempt.OperationKind != "" || attempt.OperationVersion != 0 || attempt.RequestDigest != "" || attempt.AgentID != ""
	if bound && (attempt.OperationKind == "" || attempt.OperationVersion <= 0 || attempt.RequestDigest == "" || attempt.AgentID == "") {
		return ActionDispatchAttempt{}, fmt.Errorf("action dispatch operation binding is incomplete")
	}
	return attempt, nil
}

func BindActionDispatchAttempt(attempt ActionDispatchAttempt, binding ActionDispatchBinding) (ActionDispatchAttempt, error) {
	attempt.OperationKind = binding.OperationKind
	attempt.OperationVersion = binding.OperationVersion
	attempt.RequestDigest = binding.RequestDigest
	attempt.AgentID = binding.AgentID
	return NormalizeActionDispatchAttempt(attempt)
}

func NormalizeActionDispatchReceipt(receipt ActionDispatchReceipt) (ActionDispatchReceipt, error) {
	receipt.AttemptID = strings.TrimSpace(receipt.AttemptID)
	receipt.ActionID = strings.TrimSpace(receipt.ActionID)
	receipt.TransportRequestID = strings.TrimSpace(receipt.TransportRequestID)
	if receipt.ActionID == "" || receipt.AttemptID != ActionDispatchAttemptID(receipt.ActionID) {
		return ActionDispatchReceipt{}, fmt.Errorf("action dispatch receipt identity is invalid")
	}
	if receipt.TransportRequestID == "" {
		receipt.TransportRequestID = receipt.AttemptID
	}
	if receipt.ReceivedAt.IsZero() {
		receipt.ReceivedAt = time.Now().UTC()
	} else {
		receipt.ReceivedAt = receipt.ReceivedAt.UTC()
	}
	return receipt, nil
}
