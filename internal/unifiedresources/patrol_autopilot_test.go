package unifiedresources

import (
	"testing"
	"time"
)

func patrolAutopilotTestActor(subject, credential, orgID string) ActionActor {
	return ActionActor{SubjectID: subject, Kind: ActionActorUser, CredentialID: credential, OrgID: orgID}
}

func TestPatrolAutopilotContractRegistryOwnsSupportedVersionShapes(t *testing.T) {
	v1, v1Supported := PatrolAutopilotContractForVersion(PatrolAutopilotAcknowledgementVersionV1)
	v2, v2Supported := PatrolAutopilotContractForVersion(PatrolAutopilotAcknowledgementVersionV2)
	if !v1Supported || !v2Supported {
		t.Fatalf("supported contracts missing: v1=%v v2=%v", v1Supported, v2Supported)
	}
	if v1.Version != PatrolAutopilotAcknowledgementVersionV1 || v2.Version != PatrolAutopilotAcknowledgementVersionV2 || len(v1.AcceptedScope) == len(v2.AcceptedScope) || v1.AcceptedLimits == v2.AcceptedLimits {
		t.Fatalf("versioned contracts are not distinct: v1=%#v v2=%#v", v1, v2)
	}
	if _, supported := PatrolAutopilotContractForVersion(99); supported {
		t.Fatal("unsupported V99 contract was accepted")
	}
	if CurrentPatrolAutopilotServerPolicy(time.Now()).CurrentVersion != PatrolAutopilotCurrentAcknowledgementVersion {
		t.Fatal("current version provider diverges from the server-owned current version")
	}
}

func issuedPatrolAutopilotTestEvidence(t *testing.T, id string, actor ActionActor, policy PatrolAutopilotServerPolicy) (PatrolAutopilotAcknowledgement, PatrolAutopilotActivation) {
	t.Helper()
	acknowledgement, created, err := IssuePatrolAutopilotAcknowledgement(nil, id, actor, policy)
	if err != nil || !created {
		t.Fatalf("issue acknowledgement created=%v err=%v", created, err)
	}
	activation, created, err := BindPatrolAutopilotActivation([]PatrolAutopilotAcknowledgement{acknowledgement}, nil, nil, acknowledgement.ID, actor, policy)
	if err != nil || !created {
		t.Fatalf("bind activation created=%v err=%v", created, err)
	}
	return acknowledgement, activation
}

func TestPatrolAutopilotAcknowledgementExactRetryAndConflict(t *testing.T) {
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := CurrentPatrolAutopilotServerPolicy(now)
	actor := patrolAutopilotTestActor("admin-one", "session:one", "org-a")
	acknowledgement, created, err := IssuePatrolAutopilotAcknowledgement(nil, "ack-retry-0001", actor, policy)
	if err != nil || !created {
		t.Fatalf("first issue created=%v err=%v", created, err)
	}
	replayed, created, err := IssuePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, acknowledgement.ID, actor, PatrolAutopilotServerPolicy{CurrentVersion: policy.CurrentVersion, Now: now.Add(time.Minute)})
	if err != nil || created || replayed.Digest != acknowledgement.Digest || !replayed.AcceptedAt.Equal(acknowledgement.AcceptedAt) {
		t.Fatalf("exact replay created=%v err=%v record=%#v", created, err, replayed)
	}
	otherActor := patrolAutopilotTestActor("admin-two", "session:two", "org-a")
	if _, _, err := IssuePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, acknowledgement.ID, otherActor, policy); PatrolAutopilotErrorCode(err) != PatrolAutopilotStatusConflict {
		t.Fatalf("conflicting actor error=%v code=%q", err, PatrolAutopilotErrorCode(err))
	}
	otherOrg := patrolAutopilotTestActor("admin-one", "session:one", "org-b")
	if _, _, err := IssuePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, acknowledgement.ID, otherOrg, policy); PatrolAutopilotErrorCode(err) != PatrolAutopilotStatusConflict {
		t.Fatalf("cross-org reuse error=%v code=%q", err, PatrolAutopilotErrorCode(err))
	}
}

func TestPatrolAutopilotRevocationRetryIsExact(t *testing.T) {
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := CurrentPatrolAutopilotServerPolicy(now)
	actor := patrolAutopilotTestActor("admin-one", "session:one", "org-a")
	acknowledgement, _ := issuedPatrolAutopilotTestEvidence(t, "ack-revoke-001", actor, policy)
	revocation, created, err := RevokePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, nil, acknowledgement.ID, actor, "operator stop", policy)
	if err != nil || !created {
		t.Fatalf("first revocation created=%v err=%v", created, err)
	}
	replayed, created, err := RevokePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, []PatrolAutopilotRevocation{revocation}, acknowledgement.ID, actor, "operator stop", PatrolAutopilotServerPolicy{CurrentVersion: policy.CurrentVersion, Now: now.Add(time.Minute)})
	if err != nil || created || replayed.Digest != revocation.Digest {
		t.Fatalf("exact revocation replay created=%v err=%v", created, err)
	}
	for _, tc := range []struct {
		name   string
		actor  ActionActor
		reason string
	}{
		{name: "different actor", actor: patrolAutopilotTestActor("admin-two", "session:two", "org-a"), reason: "operator stop"},
		{name: "different credential", actor: patrolAutopilotTestActor("admin-one", "session:changed", "org-a"), reason: "operator stop"},
		{name: "different reason", actor: actor, reason: "different request"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := RevokePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, []PatrolAutopilotRevocation{revocation}, acknowledgement.ID, tc.actor, tc.reason, policy); PatrolAutopilotErrorCode(err) != PatrolAutopilotStatusConflict {
				t.Fatalf("error=%v code=%q", err, PatrolAutopilotErrorCode(err))
			}
		})
	}
}

func TestEvaluatePatrolAutopilotFailsClosedForIneligibleEvidence(t *testing.T) {
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := CurrentPatrolAutopilotServerPolicy(now)
	actor := patrolAutopilotTestActor("admin-one", "session:one", "org-a")
	acknowledgement, activation := issuedPatrolAutopilotTestEvidence(t, "ack-negative-01", actor, policy)

	tests := []struct {
		name         string
		legacy       bool
		acknowledges []PatrolAutopilotAcknowledgement
		revocations  []PatrolAutopilotRevocation
		activation   *PatrolAutopilotActivation
		orgID        string
		policy       PatrolAutopilotServerPolicy
		wantCode     string
	}{
		{name: "no acknowledgement", activation: nil, orgID: "org-a", policy: policy, wantCode: PatrolAutopilotStatusAcknowledgementRequired},
		{name: "legacy boolean", legacy: true, activation: nil, orgID: "org-a", policy: policy, wantCode: PatrolAutopilotStatusLegacyBooleanIgnored},
		{name: "stale version", acknowledges: []PatrolAutopilotAcknowledgement{acknowledgement}, activation: &activation, orgID: "org-a", policy: PatrolAutopilotServerPolicy{CurrentVersion: 2, Now: now}, wantCode: PatrolAutopilotStatusStaleVersion},
		{name: "wrong org", acknowledges: []PatrolAutopilotAcknowledgement{acknowledgement}, activation: &activation, orgID: "org-b", policy: policy, wantCode: PatrolAutopilotStatusWrongOrg},
		{name: "wrong actor binding", acknowledges: []PatrolAutopilotAcknowledgement{acknowledgement}, activation: func() *PatrolAutopilotActivation {
			record := activation
			record.Actor.CredentialID = "session:other"
			record.Digest = PatrolAutopilotActivationDigest(record)
			return &record
		}(), orgID: "org-a", policy: policy, wantCode: PatrolAutopilotStatusWrongActor},
		{name: "tampered acknowledgement digest", acknowledges: []PatrolAutopilotAcknowledgement{func() PatrolAutopilotAcknowledgement {
			record := acknowledgement
			record.AcceptedScope = []string{"tampered"}
			return record
		}()}, activation: &activation, orgID: "org-a", policy: policy, wantCode: PatrolAutopilotStatusStaleVersion},
		{name: "tampered activation digest", acknowledges: []PatrolAutopilotAcknowledgement{acknowledgement}, activation: func() *PatrolAutopilotActivation {
			record := activation
			record.AcknowledgementDigest = "sha256:tampered"
			return &record
		}(), orgID: "org-a", policy: policy, wantCode: PatrolAutopilotStatusActivationDigestInvalid},
	}

	expiringPolicy := PatrolAutopilotServerPolicy{CurrentVersion: 1, Now: now, Lifetime: time.Minute}
	expiringAcknowledgement, expiringActivation := issuedPatrolAutopilotTestEvidence(t, "ack-expired-001", actor, expiringPolicy)
	tests = append(tests, struct {
		name         string
		legacy       bool
		acknowledges []PatrolAutopilotAcknowledgement
		revocations  []PatrolAutopilotRevocation
		activation   *PatrolAutopilotActivation
		orgID        string
		policy       PatrolAutopilotServerPolicy
		wantCode     string
	}{name: "expired", acknowledges: []PatrolAutopilotAcknowledgement{expiringAcknowledgement}, activation: &expiringActivation, orgID: "org-a", policy: PatrolAutopilotServerPolicy{CurrentVersion: 1, Now: now.Add(2 * time.Minute)}, wantCode: PatrolAutopilotStatusExpired})

	revocation, _, err := RevokePatrolAutopilotAcknowledgement([]PatrolAutopilotAcknowledgement{acknowledgement}, nil, acknowledgement.ID, actor, "stop", policy)
	if err != nil {
		t.Fatal(err)
	}
	tests = append(tests, struct {
		name         string
		legacy       bool
		acknowledges []PatrolAutopilotAcknowledgement
		revocations  []PatrolAutopilotRevocation
		activation   *PatrolAutopilotActivation
		orgID        string
		policy       PatrolAutopilotServerPolicy
		wantCode     string
	}{name: "revoked", acknowledges: []PatrolAutopilotAcknowledgement{acknowledgement}, revocations: []PatrolAutopilotRevocation{revocation}, activation: &activation, orgID: "org-a", policy: policy, wantCode: PatrolAutopilotStatusRevoked})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			effective, status := EvaluatePatrolAutopilot("full", "approval", tc.orgID, tc.legacy, tc.acknowledges, tc.revocations, tc.activation, tc.policy)
			if effective != "approval" || status.Active || status.Code != tc.wantCode {
				t.Fatalf("effective=%q status=%#v", effective, status)
			}
		})
	}
}

func TestEvaluatePatrolAutopilotRejectsForeignMalformedRevocationWithoutVictimAttribution(t *testing.T) {
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := CurrentPatrolAutopilotServerPolicy(now)
	actor := patrolAutopilotTestActor("admin-one", "session:one", "org-a")
	acknowledgement, activation := issuedPatrolAutopilotTestEvidence(t, "ack-foreign-revoke", actor, policy)
	foreign := PatrolAutopilotRevocation{
		Version:           PatrolAutopilotRevocationVersion,
		AcknowledgementID: acknowledgement.ID,
		OrgID:             "org-b",
		Actor:             patrolAutopilotTestActor("admin-two", "session:two", "org-b"),
		Reason:            "foreign mutation",
		RevokedAt:         now.Add(time.Minute),
	}
	foreign.Digest = PatrolAutopilotRevocationDigest(foreign)

	effective, status := EvaluatePatrolAutopilot("full", "approval", "org-a", false, []PatrolAutopilotAcknowledgement{acknowledgement}, []PatrolAutopilotRevocation{foreign}, &activation, PatrolAutopilotServerPolicy{CurrentVersion: 1, Now: now.Add(2 * time.Minute)})
	if effective != "approval" || status.Active || status.Code != PatrolAutopilotStatusWrongOrg {
		t.Fatalf("foreign revocation was accepted or attributed to victim: effective=%q status=%#v", effective, status)
	}
}

func TestPatrolAutopilotRequiresHumanActorAndHonestOutcomeLimits(t *testing.T) {
	policy := CurrentPatrolAutopilotServerPolicy(time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC))
	tokenActor := ActionActor{SubjectID: "admin-one", Kind: ActionActorAPIToken, CredentialID: "api-token:one", OrgID: "org-a"}
	if _, _, err := IssuePatrolAutopilotAcknowledgement(nil, "ack-token-0001", tokenActor, policy); PatrolAutopilotErrorCode(err) != PatrolAutopilotStatusUserRequired {
		t.Fatalf("token acknowledgement error=%v code=%q", err, PatrolAutopilotErrorCode(err))
	}
	limits := CurrentPatrolAutopilotAcceptedLimits()
	if !limits.VerificationReconciledWhenSupported || !limits.EvidenceClassDisclosed || !limits.InconclusiveOutcomeAllowed || !limits.ExecutionSuccessIsNotOutcomeTruth {
		t.Fatalf("accepted limits overstate or omit outcome truth: %#v", limits)
	}
	for _, scope := range CurrentPatrolAutopilotAcceptedScope() {
		if scope == "verified_outcomes" {
			t.Fatal("accepted scope must not promise universal verified outcomes")
		}
	}
}
