package agentexec

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	approvalGrantVersion    = 1
	approvalGrantKeyPrefix  = "pulse-agent-exec-approval-grant-v1"
	approvalGrantSigPrefix  = "hmac-sha256:"
	DefaultApprovalGrantTTL = 2 * time.Minute
)

const (
	ApprovalGrantRejectionUnknown             = "unknown"
	ApprovalGrantRejectionMissing             = "missing"
	ApprovalGrantRejectionKeyUnavailable      = "key_unavailable"
	ApprovalGrantRejectionVersionMismatch     = "version_mismatch"
	ApprovalGrantRejectionApprovalIDMismatch  = "approval_id_mismatch"
	ApprovalGrantRejectionRequestMismatch     = "request_mismatch"
	ApprovalGrantRejectionAgentMismatch       = "agent_mismatch"
	ApprovalGrantRejectionCommandHashMismatch = "command_hash_mismatch"
	ApprovalGrantRejectionTargetTypeMismatch  = "target_type_mismatch"
	ApprovalGrantRejectionTargetIDMismatch    = "target_id_mismatch"
	ApprovalGrantRejectionExpired             = "expired"
	ApprovalGrantRejectionIssuedAtInvalid     = "issued_at_invalid"
	ApprovalGrantRejectionNonceMissing        = "nonce_missing"
	ApprovalGrantRejectionSignatureInvalid    = "signature_invalid"
)

// ApprovalGrantVerificationError preserves a stable rejection reason for
// metrics and logs while keeping the human-readable verification error.
type ApprovalGrantVerificationError struct {
	Reason string
	Err    error
}

func (e *ApprovalGrantVerificationError) Error() string {
	if e == nil || e.Err == nil {
		return "approval grant verification failed"
	}
	return e.Err.Error()
}

func (e *ApprovalGrantVerificationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func approvalGrantVerificationError(reason, message string, args ...interface{}) error {
	if reason == "" {
		reason = ApprovalGrantRejectionUnknown
	}
	return &ApprovalGrantVerificationError{
		Reason: reason,
		Err:    fmt.Errorf(message, args...),
	}
}

// ApprovalGrantVerificationReason returns the stable metric/log label for a
// verification error. Non-verification errors collapse to "unknown".
func ApprovalGrantVerificationReason(err error) string {
	if err == nil {
		return ""
	}
	var verificationErr *ApprovalGrantVerificationError
	if errors.As(err, &verificationErr) && strings.TrimSpace(verificationErr.Reason) != "" {
		return verificationErr.Reason
	}
	return ApprovalGrantRejectionUnknown
}

// CommandApprovalGrant is a server-issued, token-bound grant that lets an agent
// verify an approval-gated command before executing it.
type CommandApprovalGrant struct {
	Version     int       `json:"version"`
	ApprovalID  string    `json:"approval_id"`
	RequestID   string    `json:"request_id"`
	AgentID     string    `json:"agent_id"`
	CommandHash string    `json:"command_hash"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id,omitempty"`
	IssuedAt    time.Time `json:"issued_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	Nonce       string    `json:"nonce"`
	Signature   string    `json:"signature"`
}

// DeriveApprovalGrantKey returns the in-memory signing key derived from the
// runtime agent token. Callers should keep the raw token out of long-lived state.
func DeriveApprovalGrantKey(token string) []byte {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(approvalGrantKeyPrefix + "\x00" + token))
	return sum[:]
}

// ComputeCommandApprovalHash computes the canonical command+target hash used by
// approval grants and the approval store.
func ComputeCommandApprovalHash(command, targetType, targetID string) string {
	h := sha256.New()
	h.Write([]byte(command))
	h.Write([]byte("|"))
	h.Write([]byte(strings.ToLower(strings.TrimSpace(targetType))))
	h.Write([]byte("|"))
	h.Write([]byte(strings.TrimSpace(targetID)))
	return hex.EncodeToString(h.Sum(nil))
}

func NewCommandApprovalGrant(key []byte, agentID string, cmd ExecuteCommandPayload, now time.Time, ttl time.Duration) (*CommandApprovalGrant, error) {
	if len(key) == 0 {
		return nil, errors.New("approval grant key is unavailable")
	}
	if strings.TrimSpace(cmd.ApprovalID) == "" {
		return nil, errors.New("approval id is required")
	}
	if ttl <= 0 {
		ttl = DefaultApprovalGrantTTL
	}
	nonce, err := newApprovalGrantNonce()
	if err != nil {
		return nil, err
	}
	grant := &CommandApprovalGrant{
		Version:     approvalGrantVersion,
		ApprovalID:  strings.TrimSpace(cmd.ApprovalID),
		RequestID:   strings.TrimSpace(cmd.RequestID),
		AgentID:     strings.TrimSpace(agentID),
		CommandHash: ComputeCommandApprovalHash(cmd.Command, cmd.TargetType, cmd.TargetID),
		TargetType:  strings.ToLower(strings.TrimSpace(cmd.TargetType)),
		TargetID:    strings.TrimSpace(cmd.TargetID),
		IssuedAt:    now.UTC(),
		ExpiresAt:   now.Add(ttl).UTC(),
		Nonce:       nonce,
	}
	grant.Signature = signApprovalGrant(key, grant)
	return grant, nil
}

func VerifyCommandApprovalGrant(token string, agentID string, cmd ExecuteCommandPayload, now time.Time) error {
	if cmd.ApprovalGrant == nil {
		return approvalGrantVerificationError(ApprovalGrantRejectionMissing, "approval grant is required")
	}
	key := DeriveApprovalGrantKey(token)
	return VerifyCommandApprovalGrantWithKey(key, agentID, cmd, now)
}

func VerifyCommandApprovalGrantWithKey(key []byte, agentID string, cmd ExecuteCommandPayload, now time.Time) error {
	grant := cmd.ApprovalGrant
	if grant == nil {
		return approvalGrantVerificationError(ApprovalGrantRejectionMissing, "approval grant is required")
	}
	if len(key) == 0 {
		return approvalGrantVerificationError(ApprovalGrantRejectionKeyUnavailable, "approval grant key is unavailable")
	}
	if grant.Version != approvalGrantVersion {
		return approvalGrantVerificationError(ApprovalGrantRejectionVersionMismatch, "unsupported approval grant version %d", grant.Version)
	}
	if strings.TrimSpace(grant.ApprovalID) == "" || strings.TrimSpace(grant.ApprovalID) != strings.TrimSpace(cmd.ApprovalID) {
		return approvalGrantVerificationError(ApprovalGrantRejectionApprovalIDMismatch, "approval grant id does not match command")
	}
	if strings.TrimSpace(grant.RequestID) == "" || strings.TrimSpace(grant.RequestID) != strings.TrimSpace(cmd.RequestID) {
		return approvalGrantVerificationError(ApprovalGrantRejectionRequestMismatch, "approval grant request does not match command")
	}
	if strings.TrimSpace(grant.AgentID) == "" || strings.TrimSpace(grant.AgentID) != strings.TrimSpace(agentID) {
		return approvalGrantVerificationError(ApprovalGrantRejectionAgentMismatch, "approval grant agent does not match command")
	}
	if strings.TrimSpace(grant.CommandHash) != ComputeCommandApprovalHash(cmd.Command, cmd.TargetType, cmd.TargetID) {
		return approvalGrantVerificationError(ApprovalGrantRejectionCommandHashMismatch, "approval grant command hash does not match command")
	}
	if strings.ToLower(strings.TrimSpace(grant.TargetType)) != strings.ToLower(strings.TrimSpace(cmd.TargetType)) {
		return approvalGrantVerificationError(ApprovalGrantRejectionTargetTypeMismatch, "approval grant target type does not match command")
	}
	if strings.TrimSpace(grant.TargetID) != strings.TrimSpace(cmd.TargetID) {
		return approvalGrantVerificationError(ApprovalGrantRejectionTargetIDMismatch, "approval grant target id does not match command")
	}
	if grant.ExpiresAt.IsZero() || now.UTC().After(grant.ExpiresAt.UTC()) {
		return approvalGrantVerificationError(ApprovalGrantRejectionExpired, "approval grant has expired")
	}
	if grant.IssuedAt.IsZero() || grant.IssuedAt.UTC().After(now.UTC().Add(30*time.Second)) {
		return approvalGrantVerificationError(ApprovalGrantRejectionIssuedAtInvalid, "approval grant issued_at is invalid")
	}
	if strings.TrimSpace(grant.Nonce) == "" {
		return approvalGrantVerificationError(ApprovalGrantRejectionNonceMissing, "approval grant nonce is required")
	}
	if !hmac.Equal([]byte(strings.TrimSpace(grant.Signature)), []byte(signApprovalGrant(key, grant))) {
		return approvalGrantVerificationError(ApprovalGrantRejectionSignatureInvalid, "approval grant signature is invalid")
	}
	return nil
}

func signApprovalGrant(key []byte, grant *CommandApprovalGrant) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(approvalGrantCanonicalString(grant)))
	return approvalGrantSigPrefix + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func approvalGrantCanonicalString(grant *CommandApprovalGrant) string {
	return strings.Join([]string{
		fmt.Sprintf("%d", grant.Version),
		strings.TrimSpace(grant.ApprovalID),
		strings.TrimSpace(grant.RequestID),
		strings.TrimSpace(grant.AgentID),
		strings.TrimSpace(grant.CommandHash),
		strings.ToLower(strings.TrimSpace(grant.TargetType)),
		strings.TrimSpace(grant.TargetID),
		grant.IssuedAt.UTC().Format(time.RFC3339Nano),
		grant.ExpiresAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(grant.Nonce),
	}, "\n")
}

func newApprovalGrantNonce() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate approval grant nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
