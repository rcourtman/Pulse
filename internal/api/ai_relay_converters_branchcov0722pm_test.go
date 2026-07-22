package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/approval"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/chat"
	"github.com/rcourtman/pulse-go-rewrite/pkg/aicontracts"
	"github.com/stretchr/testify/assert"
)

// branchcovHandoffActionAllFields builds a HandoffAction whose every string
// field is set to strVal and every bool field is set to boolVal. Used so the
// merge tests can distinguish "canonical won" from "requested won" per field
// via a single struct-equality assertion.
func branchcovHandoffActionAllFields(strVal string, boolVal bool) chat.HandoffAction {
	return chat.HandoffAction{
		FindingID:              strVal,
		RecordID:               strVal,
		ApprovalID:             strVal,
		ApprovalStatus:         strVal,
		ApprovalRequestedAt:    strVal,
		ApprovalExpiresAt:      strVal,
		ApprovalDecidedAt:      strVal,
		ApprovalConsumed:       boolVal,
		ActionID:               strVal,
		ActionState:            strVal,
		ActionUpdatedAt:        strVal,
		ActionRequestedBy:      strVal,
		ActionCapability:       strVal,
		ActionApprovalPolicy:   strVal,
		ActionRequiresApproval: boolVal,
		ActionPlanExpiresAt:    strVal,
		ActionPlanMessage:      strVal,
		ActionPreflight:        strVal,
		ActionDryRunSummary:    strVal,
		ActionResult:           strVal,
		FixID:                  strVal,
		Description:            strVal,
		RiskLevel:              strVal,
		Destructive:            boolVal,
		TargetHost:             strVal,
		TargetResourceID:       strVal,
		TargetResourceName:     strVal,
		TargetResourceType:     strVal,
		TargetNode:             strVal,
	}
}

func TestBranchcov0722PM_ApprovalRequestToInfo(t *testing.T) {
	t.Run("nil request returns nil", func(t *testing.T) {
		assert.Nil(t, approvalRequestToInfo(nil))
	})

	t.Run("populated request maps every field", func(t *testing.T) {
		requestedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		expiresAt := time.Date(2024, 1, 2, 4, 4, 5, 0, time.UTC)
		decidedAt := time.Date(2024, 5, 6, 7, 8, 9, 0, time.UTC)
		preflightGeneratedAt := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)

		req := &approval.ApprovalRequest{
			ID:          "ap-1",
			OrgID:       "org-1",
			ExecutionID: "exec-1",
			ToolID:      "tool-1",
			Command:     "systemctl restart x",
			TargetType:  "agent",
			TargetID:    "agent-1",
			TargetName:  "node-1",
			Context:     "restart context",
			RequestedBy: "alice",
			RiskLevel:   approval.RiskHigh,
			Status:      approval.StatusPending,
			RequestedAt: requestedAt,
			ExpiresAt:   expiresAt,
			DecidedAt:   &decidedAt,
			DecidedBy:   "bob",
			DenyReason:  "not-now",
			CommandHash: "hash-1",
			Consumed:    true,
			ContextConfidence: &approval.ContextConfidence{
				Level:    approval.ContextConfidenceVerified,
				Summary:  "verified summary",
				Evidence: []string{"e1", "e2"},
			},
			Preflight: &approval.ActionPreflight{
				Target:            "preflight-target",
				CurrentState:      "running",
				IntendedChange:    "restart",
				DryRunAvailable:   true,
				DryRunSummary:     "dry ok",
				SafetyChecks:      []string{"s1"},
				VerificationSteps: []string{"v1"},
				GeneratedAt:       preflightGeneratedAt,
			},
		}

		info := approvalRequestToInfo(req)
		if assert.NotNil(t, info) {
			assert.Equal(t, "ap-1", info.ID)
			assert.Equal(t, "org-1", info.OrgID)
			assert.Equal(t, "exec-1", info.ExecutionID)
			assert.Equal(t, "tool-1", info.ToolID)
			assert.Equal(t, "systemctl restart x", info.Command)
			assert.Equal(t, "agent", info.TargetType)
			assert.Equal(t, "agent-1", info.TargetID)
			assert.Equal(t, "node-1", info.TargetName)
			assert.Equal(t, "restart context", info.Context)
			assert.Equal(t, "alice", info.RequestedBy)
			assert.Equal(t, "high", info.RiskLevel)
			assert.Equal(t, "pending", info.Status)
			assert.Equal(t, requestedAt, info.RequestedAt)
			assert.Equal(t, expiresAt, info.ExpiresAt)
			if assert.NotNil(t, info.DecidedAt) {
				assert.Equal(t, decidedAt, *info.DecidedAt)
			}
			assert.Equal(t, "bob", info.DecidedBy)
			assert.Equal(t, "not-now", info.DenyReason)
			assert.Equal(t, "hash-1", info.CommandHash)
			assert.True(t, info.Consumed)
			assert.Nil(t, info.Plan)

			if assert.NotNil(t, info.ContextConfidence) {
				assert.Equal(t, "verified", info.ContextConfidence.Level)
				assert.Equal(t, "verified summary", info.ContextConfidence.Summary)
				assert.Equal(t, []string{"e1", "e2"}, info.ContextConfidence.Evidence)
			}
			if assert.NotNil(t, info.Preflight) {
				assert.Equal(t, "preflight-target", info.Preflight.Target)
				assert.Equal(t, "running", info.Preflight.CurrentState)
				assert.Equal(t, "restart", info.Preflight.IntendedChange)
				assert.True(t, info.Preflight.DryRunAvailable)
				assert.Equal(t, "dry ok", info.Preflight.DryRunSummary)
				assert.Equal(t, []string{"s1"}, info.Preflight.SafetyChecks)
				assert.Equal(t, []string{"v1"}, info.Preflight.VerificationSteps)
				assert.Equal(t, preflightGeneratedAt, info.Preflight.GeneratedAt)
			}
		}
	})

	t.Run("zero-value request still converts", func(t *testing.T) {
		info := approvalRequestToInfo(&approval.ApprovalRequest{})
		if assert.NotNil(t, info) {
			assert.Empty(t, info.ID)
			assert.Equal(t, "", info.RiskLevel)
			assert.Equal(t, "", info.Status)
			assert.Nil(t, info.ContextConfidence)
			assert.Nil(t, info.Preflight)
			assert.Nil(t, info.Plan)
			assert.Nil(t, info.DecidedAt)
		}
	})
}

func TestBranchcov0722PM_ContextConfidenceRequestToInfo(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, contextConfidenceRequestToInfo(nil))
	})

	t.Run("populated maps fields", func(t *testing.T) {
		conf := &approval.ContextConfidence{
			Level:    approval.ContextConfidencePartial,
			Summary:  "partial summary",
			Evidence: []string{"a", "b"},
		}
		info := contextConfidenceRequestToInfo(conf)
		if assert.NotNil(t, info) {
			assert.Equal(t, "partial", info.Level)
			assert.Equal(t, "partial summary", info.Summary)
			assert.Equal(t, []string{"a", "b"}, info.Evidence)
		}
	})

	t.Run("evidence slice is copied not aliased", func(t *testing.T) {
		conf := &approval.ContextConfidence{
			Level:    approval.ContextConfidenceInferred,
			Evidence: []string{"x"},
		}
		info := contextConfidenceRequestToInfo(conf)
		info.Evidence[0] = "mutated"
		assert.Equal(t, "x", conf.Evidence[0], "mutating the returned slice must not affect the source")
	})

	t.Run("nil evidence yields nil slice", func(t *testing.T) {
		info := contextConfidenceRequestToInfo(&approval.ContextConfidence{Level: approval.ContextConfidenceUnknown})
		if assert.NotNil(t, info) {
			assert.Nil(t, info.Evidence)
			assert.Equal(t, "unknown", info.Level)
		}
	})
}

func TestBranchcov0722PM_ContextConfidenceInfoToRequest(t *testing.T) {
	t.Run("nil returns nil", func(t *testing.T) {
		assert.Nil(t, contextConfidenceInfoToRequest(nil))
	})

	t.Run("populated maps fields", func(t *testing.T) {
		info := &aicontracts.ContextConfidenceInfo{
			Level:    "verified",
			Summary:  "s",
			Evidence: []string{"a", "b"},
		}
		req := contextConfidenceInfoToRequest(info)
		if assert.NotNil(t, req) {
			assert.Equal(t, approval.ContextConfidenceVerified, req.Level)
			assert.Equal(t, "s", req.Summary)
			assert.Equal(t, []string{"a", "b"}, req.Evidence)
		}
	})

	t.Run("evidence slice is copied not aliased", func(t *testing.T) {
		info := &aicontracts.ContextConfidenceInfo{
			Level:    "inferred",
			Evidence: []string{"x"},
		}
		req := contextConfidenceInfoToRequest(info)
		req.Evidence[0] = "mutated"
		assert.Equal(t, "x", info.Evidence[0], "mutating the returned slice must not affect the source")
	})
}

func TestBranchcov0722PM_ContextConfidenceRoundTrip(t *testing.T) {
	original := &aicontracts.ContextConfidenceInfo{
		Level:    "partial",
		Summary:  "round-trip summary",
		Evidence: []string{"e1", "e2", "e3"},
	}
	roundTripped := contextConfidenceRequestToInfo(contextConfidenceInfoToRequest(original))
	assert.Equal(t, original, roundTripped, "Info->Request->Info must preserve every field")
}

func TestBranchcov0722PM_SameNonEmptyHandoffActionID(t *testing.T) {
	cases := []struct {
		name        string
		left, right string
		want        bool
	}{
		{"both empty", "", "", false},
		{"left empty right set", "", "id-1", false},
		{"left set right empty", "id-1", "", false},
		{"equal exact non-empty", "abc", "abc", true},
		{"equal case-insensitive", "ABC", "abc", true},
		{"differing non-empty", "abc", "abd", false},
		{"whitespace-only both treated as empty", "   ", "  ", false},
		{"whitespace-only left treated as empty", "   ", "abc", false},
		{"trimmed surrounding whitespace still equal", "  abc ", "abc", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, sameNonEmptyHandoffActionID(tc.left, tc.right))
		})
	}
}

func TestBranchcov0722PM_MergeHandoffActionSafeFields(t *testing.T) {
	t.Run("canonical wins when already populated", func(t *testing.T) {
		canonical := branchcovHandoffActionAllFields("canon", true)
		requested := branchcovHandoffActionAllFields("req", false)
		// Every canonical field is non-empty / truthy, so requested must be
		// ignored entirely: the merged result equals canonical.
		assert.Equal(t, canonical, mergeHandoffActionSafeFields(canonical, requested))
	})

	t.Run("requested fills when canonical is empty", func(t *testing.T) {
		requested := branchcovHandoffActionAllFields("req", true)
		// A zero-value canonical has every string empty and every bool false,
		// so every field is taken from requested.
		assert.Equal(t, requested, mergeHandoffActionSafeFields(chat.HandoffAction{}, requested))
	})

	t.Run("whitespace-only canonical string is treated as empty", func(t *testing.T) {
		canonical := chat.HandoffAction{FindingID: "   "}
		requested := chat.HandoffAction{FindingID: "req-id"}
		got := mergeHandoffActionSafeFields(canonical, requested)
		assert.Equal(t, "req-id", got.FindingID)
	})

	t.Run("bool stays true from canonical despite requested false", func(t *testing.T) {
		canonical := chat.HandoffAction{Destructive: true, ActionRequiresApproval: true, ApprovalConsumed: true}
		requested := chat.HandoffAction{Destructive: false, ActionRequiresApproval: false, ApprovalConsumed: false}
		got := mergeHandoffActionSafeFields(canonical, requested)
		assert.True(t, got.Destructive)
		assert.True(t, got.ActionRequiresApproval)
		assert.True(t, got.ApprovalConsumed)
	})

	t.Run("bool taken from requested when canonical false", func(t *testing.T) {
		canonical := chat.HandoffAction{}
		requested := chat.HandoffAction{Destructive: true, ActionRequiresApproval: true, ApprovalConsumed: true}
		got := mergeHandoffActionSafeFields(canonical, requested)
		assert.True(t, got.Destructive)
		assert.True(t, got.ActionRequiresApproval)
		assert.True(t, got.ApprovalConsumed)
	})
}
