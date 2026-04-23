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
	}
}
