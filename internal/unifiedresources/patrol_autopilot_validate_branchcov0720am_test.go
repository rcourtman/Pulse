package unifiedresources

import (
	"errors"
	"testing"
	"time"
)

// branchcov0720amValidAck mints one structurally-valid acknowledgement that
// passes every arm of ValidatePatrolAutopilotStoredEvidence's acknowledgement
// loop. Each call produces an independent record so subtests cannot contaminate
// one another.
func branchcov0720amValidAck(t *testing.T, id string) PatrolAutopilotAcknowledgement {
	t.Helper()
	policy := CurrentPatrolAutopilotServerPolicy(time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC))
	actor := ActionActor{SubjectID: "admin-one", Kind: ActionActorUser, CredentialID: "session:one", OrgID: "org-a"}
	record, created, err := IssuePatrolAutopilotAcknowledgement(nil, id, actor, policy)
	if err != nil || !created {
		t.Fatalf("mint ack %q: created=%v err=%v", id, created, err)
	}
	return record
}

// branchcov0720amValidActivation mints an activation bound to ack whose
// ActivatedAt sits strictly after ack.AcceptedAt (avoids the Before edge).
func branchcov0720amValidActivation(t *testing.T, ack PatrolAutopilotAcknowledgement) PatrolAutopilotActivation {
	t.Helper()
	policy := CurrentPatrolAutopilotServerPolicy(ack.AcceptedAt.Add(30 * time.Second))
	record, created, err := BindPatrolAutopilotActivation([]PatrolAutopilotAcknowledgement{ack}, nil, nil, ack.ID, ack.Actor, policy)
	if err != nil || !created {
		t.Fatalf("mint activation: created=%v err=%v", created, err)
	}
	return record
}

// branchcov0720amValidRevocation mints a revocation bound to ack whose
// RevokedAt sits strictly after ack.AcceptedAt.
func branchcov0720amValidRevocation(t *testing.T, ack PatrolAutopilotAcknowledgement) PatrolAutopilotRevocation {
	t.Helper()
	policy := CurrentPatrolAutopilotServerPolicy(ack.AcceptedAt.Add(time.Minute))
	record, created, err := RevokePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{ack}, nil, ack.ID, ack.Actor, "operator stop", policy)
	if err != nil || !created {
		t.Fatalf("mint revocation: created=%v err=%v", created, err)
	}
	return record
}

// TestBranchcov0720am_ValidatePatrolAutopilotStoredEvidence drives every
// distinct return arm of the captured-evidence validator: the happy paths
// (empty input, nil activation, valid ack/revocation/activation, multi-record),
// each acknowledgement-loop rejection arm, each revocation-loop rejection arm,
// and both activation rejection arms.
func TestBranchcov0720am_ValidatePatrolAutopilotStoredEvidence(t *testing.T) {
	ack := branchcov0720amValidAck(t, "ack-validate-0001")
	ack2 := branchcov0720amValidAck(t, "ack-validate-0002")
	act := branchcov0720amValidActivation(t, ack)
	rev := branchcov0720amValidRevocation(t, ack)

	cases := []struct {
		name    string
		build   func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation)
		wantErr bool
	}{
		// --- happy paths ---
		{name: "empty_inputs", build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return nil, nil, nil
		}},
		{name: "ack_only_nil_activation", build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack}, nil, nil
		}},
		{name: "two_acknowledgements", build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack, ack2}, nil, nil
		}},
		{name: "ack_and_revocation", build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack}, []PatrolAutopilotRevocation{rev}, nil
		}},
		{name: "ack_and_activation", build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack}, nil, &act
		}},
		{name: "ack_revocation_and_activation", build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack}, []PatrolAutopilotRevocation{rev}, &act
		}},

		// --- acknowledgement-loop arms ---
		{name: "ack_id_untrimmed", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.ID = ack.ID + " "
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_id_pattern_too_short", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.ID = "short"
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_duplicate_id", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack, ack}, nil, nil
		}},
		{name: "ack_unsupported_version", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.Version = 99
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_actor_not_user", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.Actor.Kind = ActionActorAPIToken
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_actor_org_mismatch", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.Actor.OrgID = "org-b"
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_scope_invalid", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.AcceptedScope = []string{"tampered"}
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_limits_invalid", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.AcceptedLimits = PatrolAutopilotAcceptedLimits{}
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_validity_accepted_at_zero", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.AcceptedAt = time.Time{}
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_validity_expires_not_after_accepted", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.ExpiresAt = ack.AcceptedAt
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_digest_empty", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.Digest = ""
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},
		{name: "ack_digest_tampered", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := ack
			bad.Digest = "sha256:deadbeef"
			return []PatrolAutopilotAcknowledgement{bad}, nil, nil
		}},

		// --- revocation-loop arms ---
		{name: "revocation_unknown_acknowledgement", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			orphan := rev
			orphan.AcknowledgementID = "does-not-exist"
			return []PatrolAutopilotAcknowledgement{ack}, []PatrolAutopilotRevocation{orphan}, nil
		}},
		{name: "revocation_duplicate", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			return []PatrolAutopilotAcknowledgement{ack}, []PatrolAutopilotRevocation{rev, rev}, nil
		}},
		{name: "revocation_binding_invalid", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := rev
			bad.Digest = "sha256:revoked-bad"
			return []PatrolAutopilotAcknowledgement{ack}, []PatrolAutopilotRevocation{bad}, nil
		}},

		// --- activation arms ---
		{name: "activation_unknown_acknowledgement", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			orphan := act
			orphan.AcknowledgementID = "does-not-exist"
			return []PatrolAutopilotAcknowledgement{ack}, nil, &orphan
		}},
		{name: "activation_binding_invalid", wantErr: true, build: func() ([]PatrolAutopilotAcknowledgement, []PatrolAutopilotRevocation, *PatrolAutopilotActivation) {
			bad := act
			bad.Digest = "sha256:activation-bad"
			return []PatrolAutopilotAcknowledgement{ack}, nil, &bad
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			acks, revocations, activation := tc.build()
			err := ValidatePatrolAutopilotStoredEvidence(acks, revocations, activation)
			switch {
			case tc.wantErr && err == nil:
				t.Fatalf("expected rejection error, got nil")
			case !tc.wantErr && err != nil:
				t.Fatalf("expected acceptance (nil error), got %v", err)
			}
		})
	}

	// Behavioral confirmation that the multi-record happy path actually built
	// the lookup map (a revocation against the second ack is accepted, proving
	// both acknowledgements were indexed rather than silently dropped).
	t.Run("two_acknowledgements_both_indexed", func(t *testing.T) {
		revForSecond := branchcov0720amValidRevocation(t, ack2)
		if err := ValidatePatrolAutopilotStoredEvidence([]PatrolAutopilotAcknowledgement{ack, ack2}, []PatrolAutopilotRevocation{revForSecond}, nil); err != nil {
			t.Fatalf("revocation against second ack should be accepted once indexed: %v", err)
		}
	})
}

// TestBranchcov0720am_ContractErrorError covers the three branches of
// (*PatrolAutopilotContractError).Error(): nil receiver, nil wrapped Err, and a
// non-nil wrapped Err. The Code markers are authored by the test so the
// assertions exercise real formatting logic rather than re-stating source
// constants.
func TestBranchcov0720am_ContractErrorError(t *testing.T) {
	t.Run("nil_receiver_returns_empty", func(t *testing.T) {
		var nilErr *PatrolAutopilotContractError
		if got := nilErr.Error(); got != "" {
			t.Fatalf("nil receiver: expected empty string, got %q", got)
		}
	})

	t.Run("nil_wrapped_err_returns_code_only", func(t *testing.T) {
		codeOnly := &PatrolAutopilotContractError{Code: "branchcov-code-only", Err: nil}
		if got, want := codeOnly.Error(), "branchcov-code-only"; got != want {
			t.Fatalf("nil Err: expected code-only %q, got %q", want, got)
		}
	})

	t.Run("wrapped_err_joins_code_and_inner", func(t *testing.T) {
		inner := errors.New("inner-detail")
		wrapped := &PatrolAutopilotContractError{Code: "branchcov-wrapped", Err: inner}
		want := "branchcov-wrapped" + ": " + inner.Error()
		if got := wrapped.Error(); got != want {
			t.Fatalf("wrapped Err: expected %q, got %q", want, got)
		}
		// errors.Is must traverse to the wrapped error via Unwrap.
		if !errors.Is(wrapped, inner) {
			t.Fatal("errors.Is failed to reach the wrapped inner error")
		}
	})
}
