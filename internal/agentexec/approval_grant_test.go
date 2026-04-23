package agentexec

import (
	"strings"
	"testing"
	"time"
)

func TestCommandApprovalGrantVerifiesExactCommandAndTarget(t *testing.T) {
	key := DeriveApprovalGrantKey("runtime-token")
	cmd := ExecuteCommandPayload{
		RequestID:  "req-1",
		Command:    "systemctl restart nginx",
		ApprovalID: "approval-1",
		TargetType: "agent",
	}
	now := time.Now()
	grant, err := NewCommandApprovalGrant(key, "agent-1", cmd, now, time.Minute)
	if err != nil {
		t.Fatalf("NewCommandApprovalGrant() error = %v", err)
	}
	cmd.ApprovalGrant = grant

	if err := VerifyCommandApprovalGrant("runtime-token", "agent-1", cmd, now.Add(10*time.Second)); err != nil {
		t.Fatalf("VerifyCommandApprovalGrant() error = %v", err)
	}

	tampered := cmd
	tampered.Command = "systemctl restart postgres"
	if err := VerifyCommandApprovalGrant("runtime-token", "agent-1", tampered, now.Add(10*time.Second)); err == nil || !strings.Contains(err.Error(), "command hash") {
		t.Fatalf("expected command hash rejection, got %v", err)
	} else if got := ApprovalGrantVerificationReason(err); got != ApprovalGrantRejectionCommandHashMismatch {
		t.Fatalf("rejection reason = %q, want %q", got, ApprovalGrantRejectionCommandHashMismatch)
	}
}

func TestCommandApprovalGrantExpires(t *testing.T) {
	key := DeriveApprovalGrantKey("runtime-token")
	cmd := ExecuteCommandPayload{
		RequestID:  "req-1",
		Command:    "systemctl restart nginx",
		ApprovalID: "approval-1",
		TargetType: "agent",
	}
	now := time.Now()
	grant, err := NewCommandApprovalGrant(key, "agent-1", cmd, now, time.Second)
	if err != nil {
		t.Fatalf("NewCommandApprovalGrant() error = %v", err)
	}
	cmd.ApprovalGrant = grant

	if err := VerifyCommandApprovalGrant("runtime-token", "agent-1", cmd, now.Add(2*time.Second)); err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry rejection, got %v", err)
	} else if got := ApprovalGrantVerificationReason(err); got != ApprovalGrantRejectionExpired {
		t.Fatalf("rejection reason = %q, want %q", got, ApprovalGrantRejectionExpired)
	}
}

func TestCommandApprovalGrantClassifiesMissingAndSignatureRejections(t *testing.T) {
	cmd := ExecuteCommandPayload{
		RequestID:  "req-1",
		Command:    "systemctl restart nginx",
		ApprovalID: "approval-1",
		TargetType: "agent",
	}
	now := time.Now()

	if err := VerifyCommandApprovalGrant("runtime-token", "agent-1", cmd, now); err == nil {
		t.Fatal("expected missing approval grant rejection")
	} else if got := ApprovalGrantVerificationReason(err); got != ApprovalGrantRejectionMissing {
		t.Fatalf("missing rejection reason = %q, want %q", got, ApprovalGrantRejectionMissing)
	}

	key := DeriveApprovalGrantKey("runtime-token")
	grant, err := NewCommandApprovalGrant(key, "agent-1", cmd, now, time.Minute)
	if err != nil {
		t.Fatalf("NewCommandApprovalGrant() error = %v", err)
	}
	grant.Signature = "hmac-sha256:invalid"
	cmd.ApprovalGrant = grant

	if err := VerifyCommandApprovalGrant("runtime-token", "agent-1", cmd, now.Add(10*time.Second)); err == nil {
		t.Fatal("expected signature rejection")
	} else if got := ApprovalGrantVerificationReason(err); got != ApprovalGrantRejectionSignatureInvalid {
		t.Fatalf("signature rejection reason = %q, want %q", got, ApprovalGrantRejectionSignatureInvalid)
	}
}
