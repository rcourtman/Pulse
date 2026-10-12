package config

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func configPatrolAutopilotActor(subject, credential, orgID string) unifiedresources.ActionActor {
	return unifiedresources.ActionActor{SubjectID: subject, Kind: unifiedresources.ActionActorUser, CredentialID: credential, OrgID: orgID}
}

func TestAIConfigPatrolAutopilotStoredEvidenceRejectsMaliciousTwoTenantRewrite(t *testing.T) {
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := unifiedresources.CurrentPatrolAutopilotServerPolicy(now)

	tests := []struct {
		name   string
		mutate func(*AIConfig, unifiedresources.ActionActor)
	}{
		{name: "self digesting cross org revocation", mutate: func(cfg *AIConfig, foreignActor unifiedresources.ActionActor) {
			revocation := unifiedresources.PatrolAutopilotRevocation{
				Version:           unifiedresources.PatrolAutopilotRevocationVersion,
				AcknowledgementID: cfg.PatrolAutopilotAcknowledgements[0].ID,
				OrgID:             foreignActor.OrgID,
				Actor:             foreignActor,
				Reason:            "foreign mutation",
				RevokedAt:         now.Add(time.Minute),
			}
			revocation.Digest = unifiedresources.PatrolAutopilotRevocationDigest(revocation)
			cfg.PatrolAutopilotRevocations = append(cfg.PatrolAutopilotRevocations, revocation)
		}},
		{name: "wrong acknowledgement actor org", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements[0].Actor.OrgID = "org-b"
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "unsupported acknowledgement version", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements[0].Version = 99
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "unsupported revocation version", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			acknowledgement := cfg.PatrolAutopilotAcknowledgements[0]
			revocation := unifiedresources.PatrolAutopilotRevocation{
				Version:           99,
				AcknowledgementID: acknowledgement.ID,
				OrgID:             acknowledgement.OrgID,
				Actor:             acknowledgement.Actor,
				Reason:            "unsupported",
				RevokedAt:         now.Add(time.Minute),
			}
			revocation.Digest = unifiedresources.PatrolAutopilotRevocationDigest(revocation)
			cfg.PatrolAutopilotRevocations = append(cfg.PatrolAutopilotRevocations, revocation)
		}},
		{name: "invalid acknowledgement id syntax", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements[0].ID = "bad"
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "altered accepted scope", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements[0].AcceptedScope = []string{"self_authored_scope"}
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "altered accepted limits", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements[0].AcceptedLimits.EmergencyStopHonored = false
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "V1 record carrying V2 limits", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			v2Contract, ok := unifiedresources.PatrolAutopilotContractForVersion(unifiedresources.PatrolAutopilotAcknowledgementVersionV2)
			if !ok {
				t.Fatal("V2 contract is not registered")
			}
			cfg.PatrolAutopilotAcknowledgements[0].AcceptedLimits = v2Contract.AcceptedLimits
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "invalid acknowledgement time", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements[0].ExpiresAt = cfg.PatrolAutopilotAcknowledgements[0].AcceptedAt
			cfg.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "duplicate acknowledgement id", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotAcknowledgements = append(cfg.PatrolAutopilotAcknowledgements, cfg.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "duplicate revocation", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			acknowledgement := cfg.PatrolAutopilotAcknowledgements[0]
			revocation := unifiedresources.PatrolAutopilotRevocation{
				Version:           unifiedresources.PatrolAutopilotRevocationVersion,
				AcknowledgementID: acknowledgement.ID,
				OrgID:             acknowledgement.OrgID,
				Actor:             acknowledgement.Actor,
				Reason:            "duplicate",
				RevokedAt:         now.Add(time.Minute),
			}
			revocation.Digest = unifiedresources.PatrolAutopilotRevocationDigest(revocation)
			cfg.PatrolAutopilotRevocations = append(cfg.PatrolAutopilotRevocations, revocation, revocation)
		}},
		{name: "activation actor mismatch", mutate: func(cfg *AIConfig, _ unifiedresources.ActionActor) {
			cfg.PatrolAutopilotActivation.Actor.CredentialID = "session:other"
			cfg.PatrolAutopilotActivation.Digest = unifiedresources.PatrolAutopilotActivationDigest(*cfg.PatrolAutopilotActivation)
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			persistences := map[string]*ConfigPersistence{}
			before := map[string][]byte{}
			for _, orgID := range []string{"org-a", "org-b"} {
				persistence := NewConfigPersistence(root + "/" + orgID)
				actor := configPatrolAutopilotActor("admin", "session:"+orgID, orgID)
				acknowledgement, activation := configPatrolAutopilotEvidence(t, "ack-shared-malicious", actor, policy)
				cfg := NewDefaultAIConfig()
				cfg.PatrolAutonomyLevel = PatrolAutonomyFull
				cfg.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}
				cfg.PatrolAutopilotActivation = &activation
				if err := persistence.SaveAIConfig(*cfg); err != nil {
					t.Fatal(err)
				}
				persistences[orgID] = persistence
				data, err := os.ReadFile(persistence.aiFile)
				if err != nil {
					t.Fatal(err)
				}
				before[orgID] = data
			}

			malicious, err := persistences["org-a"].LoadAIConfig()
			if err != nil {
				t.Fatal(err)
			}
			tc.mutate(malicious, configPatrolAutopilotActor("foreign-admin", "session:org-b", "org-b"))
			if err := persistences["org-a"].SaveAIConfig(*malicious); err == nil {
				t.Fatal("malicious stored evidence unexpectedly persisted")
			}

			for _, orgID := range []string{"org-a", "org-b"} {
				after, err := os.ReadFile(persistences[orgID].aiFile)
				if err != nil {
					t.Fatal(err)
				}
				if string(after) != string(before[orgID]) {
					t.Fatalf("tenant %s file changed after rejected mutation", orgID)
				}
				reopened, err := NewConfigPersistence(root + "/" + orgID).LoadAIConfig()
				if err != nil {
					t.Fatal(err)
				}
				effective, status := reopened.GetEffectivePatrolAutonomyWithPolicy(orgID, policy)
				if effective != PatrolAutonomyFull || !status.Active || len(reopened.PatrolAutopilotAcknowledgements) != 1 || len(reopened.PatrolAutopilotRevocations) != 0 {
					t.Fatalf("tenant %s authoritative state changed: effective=%q status=%#v config=%#v", orgID, effective, status, reopened)
				}
			}
		})
	}
}

func configPatrolAutopilotEvidence(t *testing.T, id string, actor unifiedresources.ActionActor, policy unifiedresources.PatrolAutopilotServerPolicy) (unifiedresources.PatrolAutopilotAcknowledgement, unifiedresources.PatrolAutopilotActivation) {
	t.Helper()
	acknowledgement, _, err := unifiedresources.IssuePatrolAutopilotAcknowledgement(nil, id, actor, policy)
	if err != nil {
		t.Fatal(err)
	}
	activation, _, err := unifiedresources.BindPatrolAutopilotActivation([]unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}, nil, nil, id, actor, policy)
	if err != nil {
		t.Fatal(err)
	}
	return acknowledgement, activation
}

func TestAIConfigPatrolAutopilotHistoryIsImmutableAcrossRewriteAndReopen(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := unifiedresources.CurrentPatrolAutopilotServerPolicy(now)
	actor := configPatrolAutopilotActor("admin-one", "session:one", "org-a")
	acknowledgement, _ := configPatrolAutopilotEvidence(t, "ack-history-001", actor, policy)

	cfg := NewDefaultAIConfig()
	cfg.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}
	if err := persistence.SaveAIConfig(*cfg); err != nil {
		t.Fatal(err)
	}

	reopened := NewConfigPersistence(dir)
	loaded, err := reopened.LoadAIConfig()
	if err != nil || len(loaded.PatrolAutopilotAcknowledgements) != 1 || loaded.PatrolAutopilotAcknowledgements[0].Digest != acknowledgement.Digest {
		t.Fatalf("reopen err=%v config=%#v", err, loaded)
	}
	secondActor := configPatrolAutopilotActor("admin-two", "session:two", "org-a")
	second, _, err := unifiedresources.IssuePatrolAutopilotAcknowledgement(loaded.PatrolAutopilotAcknowledgements, "ack-history-002", secondActor, unifiedresources.CurrentPatrolAutopilotServerPolicy(now.Add(time.Minute)))
	if err != nil {
		t.Fatal(err)
	}
	loaded.PatrolAutopilotAcknowledgements = append(loaded.PatrolAutopilotAcknowledgements, second)
	loaded.PatrolInvestigationBudget = 20
	if err := reopened.SaveAIConfig(*loaded); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		mutate func(*AIConfig)
	}{
		{name: "removed", mutate: func(record *AIConfig) {
			record.PatrolAutopilotAcknowledgements = record.PatrolAutopilotAcknowledgements[1:]
		}},
		{name: "reordered", mutate: func(record *AIConfig) {
			record.PatrolAutopilotAcknowledgements[0], record.PatrolAutopilotAcknowledgements[1] = record.PatrolAutopilotAcknowledgements[1], record.PatrolAutopilotAcknowledgements[0]
		}},
		{name: "edited", mutate: func(record *AIConfig) {
			record.PatrolAutopilotAcknowledgements[0].AcceptedAt = record.PatrolAutopilotAcknowledgements[0].AcceptedAt.Add(time.Second)
			record.PatrolAutopilotAcknowledgements[0].Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(record.PatrolAutopilotAcknowledgements[0])
		}},
		{name: "duplicate", mutate: func(record *AIConfig) {
			record.PatrolAutopilotAcknowledgements = append(record.PatrolAutopilotAcknowledgements, record.PatrolAutopilotAcknowledgements[0])
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current, loadErr := reopened.LoadAIConfig()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			tc.mutate(current)
			if err := reopened.SaveAIConfig(*current); err == nil {
				t.Fatal("malicious history rewrite unexpectedly persisted")
			}
			authoritative, loadErr := NewConfigPersistence(dir).LoadAIConfig()
			if loadErr != nil || len(authoritative.PatrolAutopilotAcknowledgements) != 2 || authoritative.PatrolAutopilotAcknowledgements[0].Digest != acknowledgement.Digest || authoritative.PatrolAutopilotAcknowledgements[1].Digest != second.Digest {
				t.Fatalf("authoritative history changed err=%v config=%#v", loadErr, authoritative)
			}
		})
	}
}

func TestAIConfigPatrolAutopilotSupportedVersionRotationPreservesHistory(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	actor := configPatrolAutopilotActor("admin-one", "session:one", "org-a")
	v1Policy := unifiedresources.PatrolAutopilotServerPolicy{CurrentVersion: unifiedresources.PatrolAutopilotAcknowledgementVersion, Now: now}
	v1, _, err := unifiedresources.IssuePatrolAutopilotAcknowledgement(nil, "ack-rotation-v1", actor, v1Policy)
	if err != nil {
		t.Fatal(err)
	}
	v1Bytes, err := json.Marshal(v1)
	if err != nil {
		t.Fatal(err)
	}
	initial := NewDefaultAIConfig()
	initial.PatrolAutonomyLevel = PatrolAutonomyApproval
	initial.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{v1}
	if err := persistence.SaveAIConfig(*initial); err != nil {
		t.Fatal(err)
	}

	v2Policy := unifiedresources.PatrolAutopilotServerPolicy{CurrentVersion: unifiedresources.PatrolAutopilotAcknowledgementVersionV2, Now: now.Add(time.Hour)}
	reopenedV1, err := NewConfigPersistence(dir).LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	// A server version rotation must not make unrelated config writes reject
	// valid immutable history from an older supported contract.
	reopenedV1.PatrolInvestigationBudget = 23
	if err := persistence.SaveAIConfig(*reopenedV1); err != nil {
		t.Fatalf("unrelated save with valid V1 history failed: %v", err)
	}

	beforeConflict, err := os.ReadFile(persistence.aiFile)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := unifiedresources.IssuePatrolAutopilotAcknowledgement(reopenedV1.PatrolAutopilotAcknowledgements, v1.ID, actor, v2Policy); unifiedresources.PatrolAutopilotErrorCode(err) != unifiedresources.PatrolAutopilotStatusConflict {
		t.Fatalf("V1 id reuse under V2 error=%v code=%q", err, unifiedresources.PatrolAutopilotErrorCode(err))
	}
	afterConflict, err := os.ReadFile(persistence.aiFile)
	if err != nil || string(afterConflict) != string(beforeConflict) {
		t.Fatalf("conflicting V1 id reuse changed persisted config: err=%v", err)
	}
	if _, _, err := unifiedresources.BindPatrolAutopilotActivation(reopenedV1.PatrolAutopilotAcknowledgements, nil, nil, v1.ID, actor, v2Policy); unifiedresources.PatrolAutopilotErrorCode(err) != unifiedresources.PatrolAutopilotStatusStaleVersion {
		t.Fatalf("stale V1 activation error=%v code=%q", err, unifiedresources.PatrolAutopilotErrorCode(err))
	}

	v2, created, err := unifiedresources.IssuePatrolAutopilotAcknowledgement(reopenedV1.PatrolAutopilotAcknowledgements, "ack-rotation-v2", actor, v2Policy)
	if err != nil || !created {
		t.Fatalf("issue V2 created=%v err=%v", created, err)
	}
	if v2.Version != unifiedresources.PatrolAutopilotAcknowledgementVersionV2 || len(v2.AcceptedScope) == len(v1.AcceptedScope) || v2.AcceptedLimits == v1.AcceptedLimits {
		t.Fatalf("V2 did not use its registered contract: v1=%#v v2=%#v", v1, v2)
	}
	reopenedV1.PatrolAutopilotAcknowledgements = append(reopenedV1.PatrolAutopilotAcknowledgements, v2)
	if err := persistence.SaveAIConfig(*reopenedV1); err != nil {
		t.Fatalf("persist V2 alongside V1: %v", err)
	}
	activation, created, err := unifiedresources.BindPatrolAutopilotActivation(reopenedV1.PatrolAutopilotAcknowledgements, nil, nil, v2.ID, actor, v2Policy)
	if err != nil || !created {
		t.Fatalf("activate V2 created=%v err=%v", created, err)
	}
	reopenedV1.PatrolAutonomyLevel = PatrolAutonomyFull
	reopenedV1.PatrolAutopilotActivation = &activation
	if err := persistence.SaveAIConfig(*reopenedV1); err != nil {
		t.Fatalf("persist V2 activation: %v", err)
	}

	finalConfig, err := NewConfigPersistence(dir).LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(finalConfig.PatrolAutopilotAcknowledgements) != 2 {
		t.Fatalf("rotation history=%#v", finalConfig.PatrolAutopilotAcknowledgements)
	}
	finalV1Bytes, err := json.Marshal(finalConfig.PatrolAutopilotAcknowledgements[0])
	if err != nil || string(finalV1Bytes) != string(v1Bytes) {
		t.Fatalf("V1 history changed during rotation: err=%v before=%s after=%s", err, v1Bytes, finalV1Bytes)
	}
	effective, status := finalConfig.GetEffectivePatrolAutonomyWithPolicy("org-a", unifiedresources.PatrolAutopilotServerPolicy{CurrentVersion: unifiedresources.PatrolAutopilotAcknowledgementVersionV2, Now: now.Add(2 * time.Hour)})
	if effective != PatrolAutonomyFull || !status.Active || status.CurrentVersion != unifiedresources.PatrolAutopilotAcknowledgementVersionV2 || status.AcknowledgementVersion != unifiedresources.PatrolAutopilotAcknowledgementVersionV2 || status.AcknowledgementID != v2.ID || !slices.Equal(status.AcceptedScope, v2.AcceptedScope) || status.AcceptedLimits != v2.AcceptedLimits {
		t.Fatalf("V2 activation did not survive reopen: effective=%q status=%#v", effective, status)
	}
}

func TestAIConfigPatrolAutopilotRejectsMalformedEvidenceOnInitialSave(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	actor := configPatrolAutopilotActor("admin-one", "session:one", "org-a")
	acknowledgement, _ := configPatrolAutopilotEvidence(t, "ack-initial-bad", actor, unifiedresources.CurrentPatrolAutopilotServerPolicy(now))
	acknowledgement.AcceptedLimits.PolicyAllowlistRequired = false
	acknowledgement.Digest = unifiedresources.PatrolAutopilotAcknowledgementDigest(acknowledgement)
	cfg := NewDefaultAIConfig()
	cfg.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}
	if err := persistence.SaveAIConfig(*cfg); err == nil {
		t.Fatal("malformed initial authority evidence unexpectedly persisted")
	}
	if _, err := os.Stat(persistence.aiFile); !os.IsNotExist(err) {
		t.Fatalf("rejected initial save created authority file: %v", err)
	}
}

func TestAIConfigPatrolAutopilotActivationAndRequestedModeCommitAtomically(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := unifiedresources.CurrentPatrolAutopilotServerPolicy(now)
	actor := configPatrolAutopilotActor("admin-one", "session:one", "org-a")
	acknowledgement, activation := configPatrolAutopilotEvidence(t, "ack-atomic-0001", actor, policy)

	baseline := NewDefaultAIConfig()
	baseline.PatrolAutonomyLevel = PatrolAutonomyApproval
	baseline.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}
	if err := persistence.SaveAIConfig(*baseline); err != nil {
		t.Fatal(err)
	}

	desired := *baseline
	desired.PatrolAutonomyLevel = PatrolAutonomyFull
	desired.PatrolFullModeUnlocked = true
	desired.PatrolAutopilotActivation = &activation
	persistence.fs = &mockFSError{FileSystem: defaultFileSystem{}, writeError: errors.New("write failed")}
	if err := persistence.SaveAIConfig(desired); err == nil {
		t.Fatal("write failure unexpectedly succeeded")
	}
	persistence.fs = defaultFileSystem{}
	afterFailure, err := NewConfigPersistence(dir).LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if effective, status := afterFailure.GetEffectivePatrolAutonomyWithPolicy("org-a", policy); effective != PatrolAutonomyApproval || status.Active || afterFailure.PatrolAutopilotActivation != nil {
		t.Fatalf("failed atomic write changed effective mode: effective=%q status=%#v config=%#v", effective, status, afterFailure)
	}

	if err := persistence.SaveAIConfig(desired); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewConfigPersistence(dir).LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	if effective, status := reopened.GetEffectivePatrolAutonomyWithPolicy("org-a", policy); effective != PatrolAutonomyFull || !status.Active || reopened.PatrolAutopilotActivation == nil {
		t.Fatalf("atomic activation did not reopen: effective=%q status=%#v config=%#v", effective, status, reopened)
	}
}

func TestAIConfigPatrolAutopilotTenantFilesDoNotCollide(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	policy := unifiedresources.CurrentPatrolAutopilotServerPolicy(now)
	for _, orgID := range []string{"org-a", "org-b"} {
		persistence := NewConfigPersistence(root + "/" + orgID)
		actor := configPatrolAutopilotActor("admin", "session:"+orgID, orgID)
		acknowledgement, _ := configPatrolAutopilotEvidence(t, "ack-shared-id", actor, policy)
		cfg := NewDefaultAIConfig()
		cfg.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}
		if err := persistence.SaveAIConfig(*cfg); err != nil {
			t.Fatal(err)
		}
	}
	for _, orgID := range []string{"org-a", "org-b"} {
		loaded, err := NewConfigPersistence(root + "/" + orgID).LoadAIConfig()
		if err != nil || len(loaded.PatrolAutopilotAcknowledgements) != 1 || loaded.PatrolAutopilotAcknowledgements[0].OrgID != orgID || loaded.PatrolAutopilotAcknowledgements[0].Actor.CredentialID != "session:"+orgID {
			t.Fatalf("tenant %s collision err=%v config=%#v", orgID, err, loaded)
		}
	}
}

func TestAIConfigLegacyFullUnlockAndStaleVersionFailClosedAfterRestart(t *testing.T) {
	dir := t.TempDir()
	persistence := NewConfigPersistence(dir)
	now := time.Date(2026, 7, 11, 20, 0, 0, 0, time.UTC)
	actor := configPatrolAutopilotActor("admin-one", "session:one", "org-a")
	oldPolicy := unifiedresources.PatrolAutopilotServerPolicy{CurrentVersion: 1, Now: now}
	acknowledgement, activation := configPatrolAutopilotEvidence(t, "ack-stale-0001", actor, oldPolicy)
	cfg := NewDefaultAIConfig()
	cfg.PatrolAutonomyLevel = PatrolAutonomyFull
	cfg.PatrolFullModeUnlocked = true
	cfg.PatrolAutopilotAcknowledgements = []unifiedresources.PatrolAutopilotAcknowledgement{acknowledgement}
	cfg.PatrolAutopilotActivation = &activation
	if err := persistence.SaveAIConfig(*cfg); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewConfigPersistence(dir).LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	effective, status := reopened.GetEffectivePatrolAutonomyWithPolicy("org-a", unifiedresources.PatrolAutopilotServerPolicy{CurrentVersion: 2, Now: now.Add(time.Hour)})
	if effective != PatrolAutonomyApproval || status.Active || status.Code != unifiedresources.PatrolAutopilotStatusStaleVersion {
		t.Fatalf("stale restart effective=%q status=%#v", effective, status)
	}

	legacy := NewDefaultAIConfig()
	legacy.PatrolAutonomyLevel = PatrolAutonomyFull
	legacy.PatrolFullModeUnlocked = true
	legacyDir := t.TempDir()
	if err := NewConfigPersistence(legacyDir).SaveAIConfig(*legacy); err != nil {
		t.Fatal(err)
	}
	legacyReopened, err := NewConfigPersistence(legacyDir).LoadAIConfig()
	if err != nil {
		t.Fatal(err)
	}
	effective, status = legacyReopened.GetEffectivePatrolAutonomyWithPolicy("org-a", oldPolicy)
	if effective != PatrolAutonomyApproval || status.Active || status.Code != unifiedresources.PatrolAutopilotStatusLegacyBooleanIgnored {
		t.Fatalf("legacy restart effective=%q status=%#v", effective, status)
	}
}
