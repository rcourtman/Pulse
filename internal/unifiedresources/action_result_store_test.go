package unifiedresources

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func actionResultStoreRecord(id string) ActionAuditRecord {
	record := atomicLifecycleTestRecord(id, ActionStatePlanned)
	record.Plan.RequiresApproval = false
	record.Plan.ApprovalPolicy = ApprovalNone
	record.Plan.ApprovalRequirement = ApprovalRequirementForFloor(ApprovalNone)
	return record
}

func persistedActionResultV2(t *testing.T, store ResourceStore, id string) ActionResultV2 {
	canonical := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionSucceeded, Summary: "mutation returned success"},
		Verification: ActionVerificationTruth{Status: ActionVerificationContradicted, EvidenceClass: ActionEvidenceAgentAttested, Evidence: actionResultTestEvidence(ActionEvidenceAgentAttested)},
		Compensation: ActionCompensationTruth{Support: ActionCompensationDeclared, Strategy: "restore_backup", Trigger: "verification_contradicted", Status: ActionCompensationNotAttempted},
	}
	return persistedActionResultV2WithTruth(t, store, id, canonical)
}

func persistedActionResultV2WithTruth(t *testing.T, store ResourceStore, id string, canonical ActionResultV2) ActionResultV2 {
	t.Helper()
	record := actionResultStoreRecord(id)
	if _, created, err := store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil || !created {
		t.Fatalf("CreateActionAudit created=%v err=%v", created, err)
	}
	started, startEvent, err := BeginActionExecution(record, "operator", record.CreatedAt.Add(time.Minute))
	if err != nil || store.RecordActionExecutionStart(started, startEvent) != nil {
		t.Fatalf("start action: %v", err)
	}
	result, _, err := ApplyActionResultV2(&ExecutionResult{Output: "mutation returned success"}, canonical)
	if err != nil {
		t.Fatal(err)
	}
	completed, doneEvent, err := CompleteActionExecution(started, result, "operator", record.CreatedAt.Add(2*time.Minute))
	if err != nil || store.RecordActionExecutionResult(completed, doneEvent) != nil {
		t.Fatalf("complete action: %v", err)
	}
	got, found, err := store.GetActionAudit(id)
	if err != nil || !found {
		t.Fatalf("GetActionAudit found=%v err=%v", found, err)
	}
	return CanonicalActionResultV2(got)
}

func TestCompensationFailureSurvivesSQLiteReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	truth := ActionResultV2{
		Version:      ActionResultV2Version,
		Execution:    ActionExecutionTruth{Status: ActionExecutionSucceeded},
		Verification: ActionVerificationTruth{Status: ActionVerificationContradicted, EvidenceClass: ActionEvidenceAgentAttested, Evidence: actionResultTestEvidence(ActionEvidenceAgentAttested)},
		Compensation: ActionCompensationTruth{
			Support: ActionCompensationDeclared, Strategy: "restore_backup", Trigger: "verification_contradicted", Status: ActionCompensationFailed,
			AttemptID: "comp-1", StepID: "restore", StartedAt: actionResultTestTime(time.Now().UTC().Add(-time.Minute)), CompletedAt: actionResultTestTime(time.Now().UTC()),
			Evidence:   actionResultTestEvidence(ActionEvidenceAgentAttested),
			ReasonCode: "restore_failed", Execution: &ActionExecutionTruth{Status: ActionExecutionFailed, ReasonCode: "provider_error"},
		},
	}
	want := persistedActionResultV2WithTruth(t, store, "act-compensation-failed", truth)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record, found, err := reopened.GetActionAudit("act-compensation-failed")
	if err != nil || !found {
		t.Fatalf("reopen found=%v err=%v", found, err)
	}
	got := CanonicalActionResultV2(record)
	if !reflect.DeepEqual(got, want) || got.Compensation.Status != ActionCompensationFailed || got.Execution.Status != ActionExecutionSucceeded {
		t.Fatalf("reopened compensation truth\n got: %#v\nwant: %#v", got, want)
	}
}

func TestCompensationSucceededRestoredStateSurvivesSQLiteReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	digest := "sha256:" + strings.Repeat("c", 64)
	verification := ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceIndependent, Evidence: actionResultTestEvidence(ActionEvidenceIndependent)}
	truth := ActionResultV2{
		Version: ActionResultV2Version, Execution: ActionExecutionTruth{Status: ActionExecutionSucceeded},
		Verification: ActionVerificationTruth{Status: ActionVerificationContradicted, EvidenceClass: ActionEvidenceAgentAttested, Evidence: actionResultTestEvidence(ActionEvidenceAgentAttested)},
		Compensation: ActionCompensationTruth{
			Support: ActionCompensationDeclared, Strategy: "restore_backup", Trigger: "verification_contradicted", Status: ActionCompensationSucceeded,
			AttemptID: "comp-2", StepID: "restore", StartedAt: actionResultTestTime(now.Add(-time.Minute)), CompletedAt: actionResultTestTime(now),
			Evidence:  actionResultTestEvidence(ActionEvidenceAgentAttested),
			Execution: &ActionExecutionTruth{Status: ActionExecutionSucceeded}, Verification: &verification,
			RestoredState: &ActionRestoredStateTruth{SubjectID: "vm:42", ExpectedDigest: digest, ObservedDigest: digest, ObservedAt: now},
		},
	}
	want := persistedActionResultV2WithTruth(t, store, "act-compensation-succeeded", truth)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record, found, err := reopened.GetActionAudit("act-compensation-succeeded")
	if err != nil || !found {
		t.Fatalf("reopen found=%v err=%v", found, err)
	}
	got := CanonicalActionResultV2(record)
	if !reflect.DeepEqual(got, want) || len(got.Compensation.Evidence) != 1 || got.Compensation.RestoredState == nil || got.Compensation.RestoredState.ExpectedDigest != digest {
		t.Fatalf("reopened restored state\n got: %#v\nwant: %#v", got, want)
	}
}

func TestMalformedActionResultV2PersistsFailClosed(t *testing.T) {
	stores := []struct {
		name  string
		store ResourceStore
	}{
		{"memory", NewMemoryStore()},
	}
	sqliteStore, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer sqliteStore.Close()
	stores = append(stores, struct {
		name  string
		store ResourceStore
	}{"sqlite", sqliteStore})
	for _, test := range stores {
		t.Run(test.name, func(t *testing.T) {
			record := actionResultStoreRecord("act-malformed-" + test.name)
			if _, created, err := test.store.CreateActionAudit(record, atomicLifecycleInitialEvents(record)); err != nil || !created {
				t.Fatalf("create created=%v err=%v", created, err)
			}
			started, startEvent, err := BeginActionExecution(record, "operator", record.CreatedAt.Add(time.Minute))
			if err != nil || test.store.RecordActionExecutionStart(started, startEvent) != nil {
				t.Fatalf("start: %v", err)
			}
			malicious := ActionResultV2{Version: 99, Execution: ActionExecutionTruth{Status: ActionExecutionSucceeded}, Verification: ActionVerificationTruth{Status: ActionVerificationConfirmed, EvidenceClass: ActionEvidenceIndependent, Evidence: actionResultTestEvidence(ActionEvidenceIndependent)}, Compensation: actionResultTestCompensation()}
			completed, event, err := CompleteActionExecution(started, &ExecutionResult{Success: true, ActionResultV2: &malicious}, "operator", record.CreatedAt.Add(2*time.Minute))
			if err != nil || test.store.RecordActionExecutionResult(completed, event) != nil {
				t.Fatalf("complete: %v", err)
			}
			stored, found, err := test.store.GetActionAudit(record.ID)
			truth := CanonicalActionResultV2(stored)
			if err != nil || !found || truth.Execution.Status != ActionExecutionInconclusive || truth.Verification.Status != ActionVerificationInconclusive || stored.Result.Success {
				t.Fatalf("stored=%#v truth=%#v found=%v err=%v", stored, truth, found, err)
			}
		})
	}
}

func TestActionResultV2MemoryStoreRoundTrip(t *testing.T) {
	truth := persistedActionResultV2(t, NewMemoryStore(), "act-result-memory")
	if truth.Execution.Status != ActionExecutionSucceeded || truth.Verification.Status != ActionVerificationContradicted || truth.Compensation.Status != ActionCompensationNotAttempted {
		t.Fatalf("round-trip truth=%#v", truth)
	}
}

func TestActionResultV2SQLiteRoundTrip(t *testing.T) {
	store, err := NewSQLiteResourceStore(t.TempDir(), "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	truth := persistedActionResultV2(t, store, "act-result-sqlite")
	if truth.Execution.Status != ActionExecutionSucceeded || truth.Verification.Status != ActionVerificationContradicted || truth.Verification.EvidenceClass != ActionEvidenceAgentAttested {
		t.Fatalf("round-trip truth=%#v", truth)
	}
}

func TestActionResultV2SurvivesSQLiteReopen(t *testing.T) {
	dir := t.TempDir()
	store, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	want := persistedActionResultV2(t, store, "act-result-reopen")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	record, found, err := reopened.GetActionAudit("act-result-reopen")
	if err != nil || !found {
		t.Fatalf("reopen found=%v err=%v", found, err)
	}
	got := CanonicalActionResultV2(record)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reopened truth\n got: %#v\nwant: %#v", got, want)
	}
}

func TestLegacyCompletedNilResultMigratesInconclusive(t *testing.T) {
	record := actionResultStoreRecord("act-legacy-nil")
	record.State = ActionStateCompleted
	record.UpdatedAt = record.CreatedAt.Add(time.Minute)
	truth := CanonicalActionResultV2(record)
	if truth.Execution.Status != ActionExecutionInconclusive || truth.Execution.ReasonCode != "legacy_missing_result" {
		t.Fatalf("legacy nil truth=%#v", truth)
	}
}

func TestLegacyMissingVerificationMigratesNotAttempted(t *testing.T) {
	record := actionResultStoreRecord("act-legacy-no-verification")
	record.State = ActionStateCompleted
	record.Result = &ExecutionResult{Success: true, Output: "done"}
	truth := CanonicalActionResultV2(record)
	if truth.Execution.Status != ActionExecutionSucceeded || truth.Verification.Status != ActionVerificationNotAttempted || truth.Verification.EvidenceClass != ActionEvidenceNone {
		t.Fatalf("legacy missing verification truth=%#v", truth)
	}
}
