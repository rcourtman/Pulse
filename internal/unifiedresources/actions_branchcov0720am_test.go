package unifiedresources

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// Branch-coverage tests for currently-uncovered functions in actions.go:
//   - IsPermanentActionExecutionRefusal  (error classifier; nil, wrapped
//     sentinels, unrelated error)
//   - ValidateHumanActionBinding          (validation; each rejection arm +
//     the valid path)
//
// ActionPolicyAuthorizationDigest is intentionally NOT exercised here: its
// body is straight-line (zero Digest, json.Marshal, sha256.Sum256, Sprintf)
// with no conditional logic to drive, so it has no branches to cover. See
// GLM_REPORT.md for the skip rationale.

// ---------------------------------------------------------------------------
// IsPermanentActionExecutionRefusal
// ---------------------------------------------------------------------------

// TestBranchcov0720am_IsPermanentActionExecutionRefusal drives every arm of
// the underlying permanentActionExecutionRefusalMessage switch via the public
// classifier: each permanent sentinel (direct and wrapped), nil, an unrelated
// error, and the non-permanent ErrActionExecutionRefusal wrapper.
func TestBranchcov0720am_IsPermanentActionExecutionRefusal(t *testing.T) {
	permanentSentinels := []struct {
		name string
		err  error
	}{
		{"plan drift", ErrActionPlanDrift},
		{"plan expired", ErrActionPlanExpired},
		{"dry run only", ErrActionDryRunOnly},
		{"resource remediation locked", ErrResourceRemediationLocked},
		{"policy authorization expired", ErrActionPolicyAuthorizationExpired},
		{"policy authorization invalid", ErrActionPolicyAuthorizationInvalid},
		{"policy authorization revoked", ErrActionPolicyAuthorizationRevoked},
		{"emergency stop", ErrActionEmergencyStop},
		{"replan required", ErrActionReplanRequired},
	}

	for _, s := range permanentSentinels {
		s := s
		t.Run("sentinel/"+s.name, func(t *testing.T) {
			if !IsPermanentActionExecutionRefusal(s.err) {
				t.Errorf("IsPermanentActionExecutionRefusal(%v) = false, want true", s.err)
			}
		})
		t.Run("wrapped/"+s.name, func(t *testing.T) {
			wrapped := fmt.Errorf("transport failure: %w", s.err)
			if !IsPermanentActionExecutionRefusal(wrapped) {
				t.Errorf("IsPermanentActionExecutionRefusal(wrapped %v) = false, want true", s.err)
			}
		})
	}

	t.Run("nil error", func(t *testing.T) {
		if IsPermanentActionExecutionRefusal(nil) {
			t.Error("IsPermanentActionExecutionRefusal(nil) = true, want false")
		}
	})

	t.Run("unrelated error", func(t *testing.T) {
		if IsPermanentActionExecutionRefusal(errors.New("transient network blip")) {
			t.Error("IsPermanentActionExecutionRefusal(unrelated) = true, want false")
		}
	})

	// ErrActionExecutionRefusal is the wrapper RefuseActionExecution returns
	// when the reason is NOT a permanent refusal. It must therefore not
	// classify as permanent itself, or transient errors would be mis-reported.
	t.Run("execution refusal wrapper is not permanent", func(t *testing.T) {
		if IsPermanentActionExecutionRefusal(ErrActionExecutionRefusal) {
			t.Error("ErrActionExecutionRefusal must not be permanent (it is the non-permanent wrapper sentinel)")
		}
	})
}

// ---------------------------------------------------------------------------
// ValidateHumanActionBinding
// ---------------------------------------------------------------------------

// TestBranchcov0720am_ValidateHumanActionBinding drives each rejection arm of
// the binding validator (first-guard disjuncts, inner-block continue arms,
// quorum-not-met) plus the happy paths that return nil.
func TestBranchcov0720am_ValidateHumanActionBinding(t *testing.T) {
	now := time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC)
	const validOrg = "default"

	validActor := func(orgID string) ActionActor {
		return ActionActor{SubjectID: "agent:helper", Kind: ActionActorService, CredentialID: "service:test", OrgID: orgID}
	}

	// baseRecord builds a record whose Request.Actor and Plan are coherent
	// under ValidateHumanActionBinding's first guard. Callers mutate the
	// specific field needed to drive a particular arm.
	baseRecord := func(state ActionState, requiresApproval bool, policy ActionApprovalLevel) ActionAuditRecord {
		return ActionAuditRecord{
			ID:    "action-1",
			State: state,
			Request: ActionRequest{
				Actor: validActor(validOrg),
			},
			Plan: ActionPlan{
				PlanHash:            "sha256:test",
				RequiresApproval:    requiresApproval,
				ApprovalPolicy:      policy,
				ApprovalRequirement: ApprovalRequirementForFloor(policy),
			},
		}
	}

	// boundApproval builds an approval whose ActorBinding and Evidence are
	// mutually coherent and coherent with the record, so it counts toward
	// quorum by default. Callers mutate fields to drive negative arms.
	boundApproval := func(record ActionAuditRecord, subject string, kind ActionActorKind, method ApprovalMethod) ActionApprovalRecord {
		binding := ActionActor{SubjectID: subject, Kind: kind, CredentialID: string(kind) + ":test", OrgID: validOrg}
		evidence := ApprovalEvidence{
			Version:  1,
			Method:   method,
			Actor:    binding,
			OrgID:    validOrg,
			ActionID: record.ID,
			PlanHash: record.Plan.PlanHash,
			Outcome:  OutcomeApproved,
			IssuedAt: now,
		}
		return ActionApprovalRecord{
			Actor:        subject,
			ActorBinding: binding,
			Method:       method,
			Timestamp:    now,
			Outcome:      OutcomeApproved,
			Evidence:     &evidence,
		}
	}

	cases := []struct {
		name    string
		record  ActionAuditRecord
		orgID   string
		wantErr error // nil expects no error; otherwise must satisfy errors.Is
	}{
		// --- First guard: each disjunct independently returns ErrActionReplanRequired ---
		{
			name: "first guard rejects invalid request actor (missing subject)",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				r.Request.Actor = ActionActor{SubjectID: "", Kind: ActionActorUser, CredentialID: "x", OrgID: validOrg}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "first guard rejects org id mismatch",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				r.Request.Actor = validActor("other-org")
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "first guard rejects requirement version mismatch",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				r.Plan.ApprovalRequirement.Version = 2 // unsupported version
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "first guard rejects requirement floor diverging from policy",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				// Non-empty Floor different from ApprovalPolicy; NormalizeApprovalRequirement
				// will not backfill it from the policy, so the divergence survives.
				r.Plan.ApprovalRequirement.Floor = ApprovalNone
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},

		// --- Inner binding block is skipped: returns nil ---
		{
			name:    "no approval required skips binding check",
			record:  baseRecord(ActionStatePlanned, false, ApprovalNone),
			orgID:   validOrg,
			wantErr: nil,
		},
		{
			name:    "approval required but state is Pending skips binding check",
			record:  baseRecord(ActionStatePending, true, ApprovalAdmin),
			orgID:   validOrg,
			wantErr: nil,
		},

		// --- Inner block entered; quorum not met -> ErrActionReplanRequired ---
		{
			name:    "approved but zero approvals collected",
			record:  baseRecord(ActionStateApproved, true, ApprovalAdmin),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "rejected approval outcome is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Outcome = OutcomeRejected
				ap.Evidence.Outcome = OutcomeRejected
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "approved outcome with nil evidence is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence = nil
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "invalid actor binding (missing subject) is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.ActorBinding.SubjectID = ""
				ap.Evidence.Actor.SubjectID = ""
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "actor binding/evidence actor mismatch is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence.Actor = ActionActor{SubjectID: "someone-else", Kind: ActionActorUser, CredentialID: "x", OrgID: validOrg}
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "evidence version not 1 is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence.Version = 2
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "evidence org id mismatch is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence.OrgID = "other-org"
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "evidence action id mismatch is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence.ActionID = "other-action"
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "evidence plan hash mismatch is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence.PlanHash = "sha256:different"
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "evidence outcome not approved is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				// approval.Outcome stays Approved so the first if proceeds; only
				// the evidence outcome disjunct in the second if triggers.
				ap.Evidence.Outcome = OutcomeRejected
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "evidence issued-at zero is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				ap.Evidence.IssuedAt = time.Time{}
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "non-MFA floor with non-session/token method is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				// MethodWebAuthnUV is neither Session nor APIToken, so the
				// non-MFA else-branch's continue fires.
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodWebAuthnUV)
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},
		{
			name: "MFA floor with non-WebAuthn/DeviceKey method is ignored",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalMultiFactor)
				// MethodSession is neither WebAuthnUV nor DeviceKeyUV, so the
				// MFA if-branch's continue fires.
				ap := boundApproval(r, "op@example.com", ActionActorUser, MethodSession)
				r.Approvals = []ActionApprovalRecord{ap}
				return r
			}(),
			orgID:   validOrg,
			wantErr: ErrActionReplanRequired,
		},

		// --- Happy paths inside the inner block: returns nil ---
		{
			name: "non-MFA valid session approval passes",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				r.Approvals = []ActionApprovalRecord{
					boundApproval(r, "op@example.com", ActionActorUser, MethodSession),
				}
				return r
			}(),
			orgID:   validOrg,
			wantErr: nil,
		},
		{
			name: "non-MFA valid API token approval passes",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalAdmin)
				r.Approvals = []ActionApprovalRecord{
					boundApproval(r, "auto-bot", ActionActorAPIToken, MethodAPIToken),
				}
				return r
			}(),
			orgID:   validOrg,
			wantErr: nil,
		},
		{
			name: "MFA valid WebAuthn approval passes",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateApproved, true, ApprovalMultiFactor)
				r.Approvals = []ActionApprovalRecord{
					boundApproval(r, "op@example.com", ActionActorUser, MethodWebAuthnUV),
				}
				return r
			}(),
			orgID:   validOrg,
			wantErr: nil,
		},
		{
			name: "MFA valid device-key approval passes and exercises Executing state",
			record: func() ActionAuditRecord {
				r := baseRecord(ActionStateExecuting, true, ApprovalMultiFactor)
				r.Approvals = []ActionApprovalRecord{
					boundApproval(r, "op@example.com", ActionActorUser, MethodDeviceKeyUV),
				}
				return r
			}(),
			orgID:   validOrg,
			wantErr: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateHumanActionBinding(tc.record, tc.orgID)
			switch {
			case tc.wantErr == nil:
				if err != nil {
					t.Errorf("ValidateHumanActionBinding() unexpected error: %v", err)
				}
			case err == nil:
				t.Errorf("ValidateHumanActionBinding() returned nil, want error matching %v", tc.wantErr)
			case !errors.Is(err, tc.wantErr):
				t.Errorf("ValidateHumanActionBinding() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
