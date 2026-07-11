package actionlifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func actorApprovalService(t *testing.T, floor unified.ActionApprovalLevel) (*Service, unified.ResourceStore) {
	t.Helper()
	store := unified.NewMemoryStore()
	service := serviceForStore(t, store, testResource(time.Now().UTC(), floor), &stubExecutor{result: &unified.ExecutionResult{Success: true}})
	return service, store
}

func planForApprovalTest(t *testing.T, service *Service, requirement *unified.ApprovalRequirement) unified.ActionPlan {
	t.Helper()
	options := PlanOptions{Actor: testActionActor("requester", "default"), ApprovalRequirement: requirement}
	plan, err := service.PlanWithOptions(context.Background(), "default", restartRequest(), options)
	if err != nil {
		t.Fatalf("PlanWithOptions: %v", err)
	}
	return plan
}

func decisionForApprovalTest(plan unified.ActionPlan, actor unified.ActionActor, outcome unified.ApprovalOutcome, reason string, method unified.ApprovalMethod, challenge string, now time.Time) unified.ActionDecision {
	return unified.ActionDecision{
		Actor: actor, Outcome: outcome, Reason: reason,
		Evidence: unified.ApprovalEvidence{
			Version: 1, Method: method, Actor: actor, OrgID: actor.OrgID, ActionID: plan.ActionID,
			PlanHash: plan.PlanHash, Outcome: outcome, ChallengeID: challenge, IssuedAt: now,
			ExpiresAt: now.Add(time.Minute),
		},
	}
}

func TestDecideRejectsMFAPolicyWithAPIMethodOnly(t *testing.T) {
	service, _ := actorApprovalService(t, unified.ApprovalMultiFactor)
	plan := planForApprovalTest(t, service, nil)
	decision := decisionForApprovalTest(plan, testActionActor("admin", "default"), unified.OutcomeApproved, "approve", unified.MethodAPIToken, "", time.Now().UTC())
	decision.Actor.Kind = unified.ActionActorAPIToken
	decision.Evidence.Actor = decision.Actor
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, decision); !errors.Is(err, ErrApprovalStepUpUnavailable) {
		t.Fatalf("Decide error=%v, want step-up unavailable", err)
	}
}

func TestDecideRejectsUnsignedOrTamperedStepUpEvidence(t *testing.T) {
	service, _ := actorApprovalService(t, unified.ApprovalMultiFactor)
	plan := planForApprovalTest(t, service, nil)
	actor := testActionActor("admin", "default")
	unsigned := decisionForApprovalTest(plan, actor, unified.OutcomeApproved, "approve", unified.MethodWebAuthnUV, "", time.Now().UTC())
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, unsigned); !errors.Is(err, ErrApprovalEvidenceInvalid) {
		t.Fatalf("unsigned error=%v", err)
	}
	tampered := decisionForApprovalTest(plan, actor, unified.OutcomeApproved, "approve", unified.MethodWebAuthnUV, "challenge-1", time.Now().UTC())
	tampered.Evidence.PlanHash = "sha256:tampered"
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, tampered); !errors.Is(err, ErrApprovalEvidenceInvalid) {
		t.Fatalf("tampered error=%v", err)
	}
}

func TestDecideRejectsEvidenceForDifferentActorOrgActionPlanOrOutcome(t *testing.T) {
	service, _ := actorApprovalService(t, unified.ApprovalAdmin)
	plan := planForApprovalTest(t, service, nil)
	baseActor := testActionActor("admin", "default")
	base := decisionForApprovalTest(plan, baseActor, unified.OutcomeApproved, "approve", unified.MethodSession, "", time.Now().UTC())
	cases := map[string]func(*unified.ActionDecision){
		"actor":   func(d *unified.ActionDecision) { d.Evidence.Actor.CredentialID = "session:other" },
		"org":     func(d *unified.ActionDecision) { d.Evidence.OrgID = "other" },
		"action":  func(d *unified.ActionDecision) { d.Evidence.ActionID = "act_other" },
		"plan":    func(d *unified.ActionDecision) { d.Evidence.PlanHash = "sha256:other" },
		"outcome": func(d *unified.ActionDecision) { d.Evidence.Outcome = unified.OutcomeRejected },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			decision := base
			mutate(&decision)
			if _, err := service.Decide(context.Background(), "default", plan.ActionID, decision); !errors.Is(err, ErrApprovalEvidenceInvalid) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestDecideRejectsExpiredOrReplayedChallenge(t *testing.T) {
	service, store := actorApprovalService(t, unified.ApprovalMultiFactor)
	plan := planForApprovalTest(t, service, nil)
	actor := testActionActor("admin", "default")
	expired := decisionForApprovalTest(plan, actor, unified.OutcomeApproved, "approve", unified.MethodWebAuthnUV, "expired", time.Now().UTC().Add(-2*time.Minute))
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, expired); !errors.Is(err, ErrApprovalEvidenceInvalid) {
		t.Fatalf("expired error=%v", err)
	}
	var verifierCalls int
	service.StepUpVerifier = StepUpVerifierFunc(func(context.Context, unified.ActionAuditRecord, unified.ActionDecision) error {
		verifierCalls++
		return nil
	})
	now := time.Now().UTC()
	decision := decisionForApprovalTest(plan, actor, unified.OutcomeApproved, "approve", unified.MethodWebAuthnUV, "challenge-1", now)
	first, err := service.Decide(context.Background(), "default", plan.ActionID, decision)
	if err != nil {
		t.Fatal(err)
	}
	retry := decision
	retry.Evidence.IssuedAt = now.Add(10 * time.Second)
	retry.Evidence.ExpiresAt = now.Add(70 * time.Second)
	second, err := service.Decide(context.Background(), "default", plan.ActionID, retry)
	if err != nil || second.State != first.State || verifierCalls != 1 {
		t.Fatalf("retry=%#v err=%v verifierCalls=%d", second, err, verifierCalls)
	}
	events, _ := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 20)
	if len(second.Approvals) != 1 || len(events) != 4 {
		t.Fatalf("approvals=%d events=%d", len(second.Approvals), len(events))
	}
	conflict := retry
	conflict.Reason = "changed"
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, conflict); !errors.Is(err, ErrDecisionReplayConflict) {
		t.Fatalf("conflict error=%v", err)
	}
}

func TestDecideRejectsRequesterWhenSeparationOfDutiesEnabled(t *testing.T) {
	service, _ := actorApprovalService(t, unified.ApprovalAdmin)
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	requirement.DisallowRequester = true
	plan := planForApprovalTest(t, service, &requirement)
	decision := decisionForApprovalTest(plan, testActionActor("requester", "default"), unified.OutcomeApproved, "approve", unified.MethodSession, "", time.Now().UTC())
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, decision); !errors.Is(err, ErrApprovalSeparationRequired) {
		t.Fatalf("error=%v", err)
	}
}

func TestRejectedDecisionPersistsDecisionAndRejectedTransitionAtomically(t *testing.T) {
	service, store := actorApprovalService(t, unified.ApprovalAdmin)
	plan := planForApprovalTest(t, service, nil)
	decision := decisionForApprovalTest(plan, testActionActor("admin", "default"), unified.OutcomeRejected, "reject", unified.MethodSession, "", time.Now().UTC())
	record, err := service.Decide(context.Background(), "default", plan.ActionID, decision)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != unified.ActionStateRejected || record.DecisionRevision != 1 {
		t.Fatalf("record state=%q revision=%d", record.State, record.DecisionRevision)
	}
	events, err := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	decisions, rejectedTransitions := 0, 0
	for _, event := range events {
		if event.Kind == unified.ActionLifecycleEventDecision {
			decisions++
		}
		if event.Kind == unified.ActionLifecycleEventTransition && event.State == unified.ActionStateRejected {
			rejectedTransitions++
		}
	}
	if len(events) != 4 || decisions != 1 || rejectedTransitions != 1 {
		t.Fatalf("events=%#v, want one decision and one rejected transition", events)
	}
}

func TestDecideKeepsPendingUntilDistinctActorQuorumReached(t *testing.T) {
	service, store := actorApprovalService(t, unified.ApprovalAdmin)
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	requirement.Quorum = 2
	plan := planForApprovalTest(t, service, &requirement)
	now := time.Now().UTC()
	firstDecision := decisionForApprovalTest(plan, testActionActor("admin-one", "default"), unified.OutcomeApproved, "approve", unified.MethodSession, "", now)
	first, err := service.Decide(context.Background(), "default", plan.ActionID, firstDecision)
	if err != nil || first.State != unified.ActionStatePending {
		t.Fatalf("first=%#v err=%v", first, err)
	}
	retry := firstDecision
	retry.Evidence.IssuedAt = now.Add(time.Second)
	replayed, err := service.Decide(context.Background(), "default", plan.ActionID, retry)
	if err != nil || replayed.State != unified.ActionStatePending || len(replayed.Approvals) != 1 {
		t.Fatalf("replay=%#v err=%v", replayed, err)
	}
	secondDecision := decisionForApprovalTest(plan, testActionActor("admin-two", "default"), unified.OutcomeApproved, "approve", unified.MethodSession, "", now)
	approved, err := service.Decide(context.Background(), "default", plan.ActionID, secondDecision)
	if err != nil || approved.State != unified.ActionStateApproved || len(approved.Approvals) != 2 {
		t.Fatalf("approved=%#v err=%v", approved, err)
	}
	events, _ := store.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 20)
	if len(events) != 5 {
		t.Fatalf("events=%d, want planned+pending+two decisions+approved transition", len(events))
	}
	if approved.DecisionRevision != 2 {
		t.Fatalf("decision revision=%d, want 2", approved.DecisionRevision)
	}
}

type decisionCASBarrier struct {
	mu      sync.Mutex
	calls   int
	arrived chan struct{}
	release chan struct{}
}

func newDecisionCASBarrier() *decisionCASBarrier {
	return &decisionCASBarrier{arrived: make(chan struct{}, 2), release: make(chan struct{})}
}

func (b *decisionCASBarrier) wait() {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	if call <= 2 {
		b.arrived <- struct{}{}
		<-b.release
	}
}

type barrierDecisionStore struct {
	unified.ResourceStore
	barrier *decisionCASBarrier
}

func (s *barrierDecisionStore) RecordActionDecision(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) error {
	s.barrier.wait()
	return s.ResourceStore.RecordActionDecision(record, event)
}

func runConcurrentDistinctApprovalCAS(t *testing.T, firstStore, secondStore unified.ResourceStore) {
	t.Helper()
	barrier := newDecisionCASBarrier()
	first := serviceForStore(t, &barrierDecisionStore{ResourceStore: firstStore, barrier: barrier}, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
	second := serviceForStore(t, &barrierDecisionStore{ResourceStore: secondStore, barrier: barrier}, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	requirement.Quorum = 2
	plan := planForApprovalTest(t, first, &requirement)
	now := time.Now().UTC()
	decisions := []unified.ActionDecision{
		decisionForApprovalTest(plan, testActionActor("admin-one", "default"), unified.OutcomeApproved, "approve one", unified.MethodSession, "", now),
		decisionForApprovalTest(plan, testActionActor("admin-two", "default"), unified.OutcomeApproved, "approve two", unified.MethodSession, "", now),
	}
	results := make(chan error, 2)
	go func() {
		_, err := first.Decide(context.Background(), "default", plan.ActionID, decisions[0])
		results <- err
	}()
	go func() {
		_, err := second.Decide(context.Background(), "default", plan.ActionID, decisions[1])
		results <- err
	}()
	<-barrier.arrived
	<-barrier.arrived
	close(barrier.release)
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("concurrent decision: %v", err)
		}
	}
	record, found, err := firstStore.GetActionAudit(plan.ActionID)
	if err != nil || !found {
		t.Fatalf("authoritative record found=%v err=%v", found, err)
	}
	if record.State != unified.ActionStateApproved || record.DecisionRevision != 2 || len(record.Approvals) != 2 {
		t.Fatalf("authoritative record state=%q revision=%d approvals=%#v", record.State, record.DecisionRevision, record.Approvals)
	}
	events, err := firstStore.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	approvedTransitions := 0
	decisionEvents := 0
	for _, event := range events {
		if event.Kind == unified.ActionLifecycleEventDecision {
			decisionEvents++
		}
		if event.Kind == unified.ActionLifecycleEventTransition && event.State == unified.ActionStateApproved {
			approvedTransitions++
		}
	}
	if len(events) != 5 || decisionEvents != 2 || approvedTransitions != 1 {
		t.Fatalf("events=%#v, want two decisions and one approved transition", events)
	}
	beforeRevision := record.DecisionRevision
	beforeEvents := len(events)
	if _, err := first.Decide(context.Background(), "default", plan.ActionID, decisions[0]); err != nil {
		t.Fatalf("exact replay: %v", err)
	}
	replayed, _, _ := firstStore.GetActionAudit(plan.ActionID)
	events, _ = firstStore.GetActionLifecycleEvents(plan.ActionID, time.Time{}, 20)
	if replayed.DecisionRevision != beforeRevision || len(events) != beforeEvents {
		t.Fatalf("exact replay revision/events=%d/%d, want %d/%d", replayed.DecisionRevision, len(events), beforeRevision, beforeEvents)
	}
	conflict := decisions[0]
	conflict.Reason = "changed reason"
	if _, err := first.Decide(context.Background(), "default", plan.ActionID, conflict); !errors.Is(err, ErrDecisionReplayConflict) {
		t.Fatalf("conflicting replay error=%v", err)
	}
}

func TestConcurrentDistinctApprovalsRetainQuorumMemoryStore(t *testing.T) {
	store := unified.NewMemoryStore()
	runConcurrentDistinctApprovalCAS(t, store, store)
}

func TestConcurrentDistinctApprovalsRetainQuorumAcrossSQLiteHandles(t *testing.T) {
	dir := t.TempDir()
	first, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	runConcurrentDistinctApprovalCAS(t, first, second)
}

func persistTwoPendingDecisionRevisions(t *testing.T, store unified.ResourceStore) string {
	t.Helper()
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	requirement.Quorum = 3
	requirement.DisallowRequester = true
	plan := planForApprovalTest(t, service, &requirement)
	now := time.Now().UTC()
	for index, subject := range []string{"admin-one", "admin-two"} {
		decision := decisionForApprovalTest(plan, testActionActor(subject, "default"), unified.OutcomeApproved, "approve "+subject, unified.MethodSession, "", now)
		record, err := service.Decide(context.Background(), "default", plan.ActionID, decision)
		if err != nil {
			t.Fatalf("decision %d: %v", index+1, err)
		}
		if record.State != unified.ActionStatePending || record.DecisionRevision != uint64(index+1) {
			t.Fatalf("decision %d record state=%q revision=%d", index+1, record.State, record.DecisionRevision)
		}
	}
	return plan.ActionID
}

func ledgerValidationRecord(id string, floor unified.ActionApprovalLevel, quorum int, disallowRequester bool, now time.Time) unified.ActionAuditRecord {
	requester := unified.ActionActor{SubjectID: "requester", Kind: unified.ActionActorService, CredentialID: "service:test", OrgID: "default"}
	requirement := unified.ApprovalRequirementForFloor(floor)
	requirement.Quorum = quorum
	requirement.DisallowRequester = disallowRequester
	return unified.ActionAuditRecord{
		ID: id, CreatedAt: now, UpdatedAt: now, State: unified.ActionStatePending,
		Request: unified.ActionRequest{RequestID: "req-" + id, ResourceID: "vm:42", CapabilityName: "restart", Reason: "ledger validation", RequestedBy: requester.SubjectID, Actor: requester},
		Plan:    unified.ActionPlan{ActionID: id, RequestID: "req-" + id, Allowed: true, RequiresApproval: true, ApprovalPolicy: floor, ApprovalRequirement: requirement, PlannedAt: now, ExpiresAt: now.Add(time.Hour), ResourceVersion: "resource:test", PolicyVersion: "policy:test", PlanHash: "sha256:" + id},
	}
}

func ledgerValidationApproval(record unified.ActionAuditRecord, actor unified.ActionActor, method unified.ApprovalMethod, now time.Time) unified.ActionApprovalRecord {
	evidence := unified.ApprovalEvidence{Version: 1, Method: method, Actor: actor, OrgID: record.Request.Actor.OrgID, ActionID: record.ID, PlanHash: record.Plan.PlanHash, Outcome: unified.OutcomeApproved, IssuedAt: now}
	if method == unified.MethodWebAuthnUV || method == unified.MethodDeviceKeyUV {
		evidence.ChallengeID = "challenge-" + record.ID
		evidence.ExpiresAt = now.Add(time.Minute)
	}
	return unified.ActionApprovalRecord{Actor: actor.SubjectID, ActorBinding: actor, Method: method, Timestamp: now, Outcome: unified.OutcomeApproved, Reason: "approve", Evidence: &evidence}
}

func assertMalformedLedgerAppendRejected(t *testing.T, store unified.ResourceStore, record unified.ActionAuditRecord, approval unified.ActionApprovalRecord, mutate func(*unified.ActionAuditRecord, *unified.ActionLifecycleEvent)) {
	t.Helper()
	initial := []unified.ActionLifecycleEvent{
		{ActionID: record.ID, Timestamp: record.CreatedAt, State: unified.ActionStatePlanned, Actor: record.Request.Actor.SubjectID, Message: "Action plan created."},
		{ActionID: record.ID, Timestamp: record.CreatedAt, State: unified.ActionStatePending, Actor: record.Request.Actor.SubjectID, Message: "Action is waiting for approval before execution."},
	}
	if _, created, err := store.CreateActionAudit(record, initial); err != nil || !created {
		t.Fatalf("create audit created=%v err=%v", created, err)
	}
	proposed, event, err := unified.ApplyActionDecision(record, approval, approval.Timestamp)
	if err != nil {
		t.Fatalf("apply malformed fixture: %v", err)
	}
	if mutate != nil {
		mutate(&proposed, &event)
	}
	beforeEvents, _ := store.GetActionLifecycleEvents(record.ID, time.Time{}, 20)
	if err := store.RecordActionDecision(proposed, event); err == nil {
		t.Fatal("malformed ledger append unexpectedly persisted")
	}
	current, found, err := store.GetActionAudit(record.ID)
	if err != nil || !found || current.DecisionRevision != 0 || len(current.Approvals) != 0 || current.State != unified.ActionStatePending {
		t.Fatalf("malformed append mutated record: found=%v err=%v record=%#v", found, err, current)
	}
	afterEvents, _ := store.GetActionLifecycleEvents(record.ID, time.Time{}, 20)
	if len(afterEvents) != len(beforeEvents) {
		t.Fatalf("malformed append mutated events: before=%#v after=%#v", beforeEvents, afterEvents)
	}
}

type ledgerValidationExpectation struct {
	actionID         string
	decisionRevision uint64
	approvalCount    int
}

func assertStoreLedgerRejectsMalformedAuthority(t *testing.T, store unified.ResourceStore) []ledgerValidationExpectation {
	t.Helper()
	now := time.Now().UTC()
	expectations := make([]ledgerValidationExpectation, 0, 13)
	user := func(subject string) unified.ActionActor {
		return unified.ActionActor{SubjectID: subject, Kind: unified.ActionActorUser, CredentialID: "session:" + subject, OrgID: "default"}
	}
	apiToken := unified.ActionActor{SubjectID: "token-owner", Kind: unified.ActionActorAPIToken, CredentialID: "api-token:test", OrgID: "default"}
	tests := []struct {
		name     string
		floor    unified.ActionApprovalLevel
		separate bool
		actor    unified.ActionActor
		method   unified.ApprovalMethod
		mutate   func(*unified.ActionAuditRecord, *unified.ActionLifecycleEvent)
	}{
		{name: "malformed actor", floor: unified.ApprovalAdmin, actor: unified.ActionActor{SubjectID: "bad", Kind: unified.ActionActorUser, OrgID: "default"}, method: unified.MethodSession},
		{name: "requester under separation", floor: unified.ApprovalAdmin, separate: true, actor: user("requester"), method: unified.MethodSession},
		{name: "wrong evidence version", floor: unified.ApprovalAdmin, actor: user("version"), method: unified.MethodSession, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.Version = 2
			event.Decision = &record.Approvals[0]
		}},
		{name: "zero issued time", floor: unified.ApprovalAdmin, actor: user("zero-time"), method: unified.MethodSession, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.IssuedAt = time.Time{}
			event.Decision = &record.Approvals[0]
		}},
		{name: "bad expiry", floor: unified.ApprovalAdmin, actor: user("expiry"), method: unified.MethodSession, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.ExpiresAt = record.Approvals[0].Evidence.IssuedAt.Add(-time.Second)
			event.Decision = &record.Approvals[0]
		}},
		{name: "future issued evidence", floor: unified.ApprovalAdmin, actor: user("future-issued"), method: unified.MethodSession, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.IssuedAt = record.Approvals[0].Timestamp.Add(time.Second)
			event.Decision = &record.Approvals[0]
		}},
		{name: "expired at decision", floor: unified.ApprovalAdmin, actor: user("expired-decision"), method: unified.MethodSession, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.IssuedAt = record.Approvals[0].Timestamp.Add(-2 * time.Minute)
			record.Approvals[0].Evidence.ExpiresAt = record.Approvals[0].Timestamp.Add(-time.Minute)
			event.Decision = &record.Approvals[0]
		}},
		{name: "session challenge identity", floor: unified.ApprovalAdmin, actor: user("session-challenge"), method: unified.MethodSession, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.ChallengeID = "unexpected-challenge"
			event.Decision = &record.Approvals[0]
		}},
		{name: "missing crypto challenge", floor: unified.ApprovalMultiFactor, actor: user("missing-challenge"), method: unified.MethodWebAuthnUV, mutate: func(record *unified.ActionAuditRecord, event *unified.ActionLifecycleEvent) {
			record.Approvals[0].Evidence.ChallengeID = ""
			event.Decision = &record.Approvals[0]
		}},
		{name: "weak method at mfa", floor: unified.ApprovalMultiFactor, actor: user("weak-mfa"), method: unified.MethodSession},
		{name: "api token at mfa", floor: unified.ApprovalMultiFactor, actor: apiToken, method: unified.MethodAPIToken},
		{name: "approved dry run", floor: unified.ApprovalDryRun, actor: user("dry-run"), method: unified.MethodSession},
	}
	for index, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record := ledgerValidationRecord(fmt.Sprintf("act_ledger_%d", index), tc.floor, 1, tc.separate, now.Add(time.Duration(index)*time.Second))
			approval := ledgerValidationApproval(record, tc.actor, tc.method, record.CreatedAt.Add(time.Minute))
			assertMalformedLedgerAppendRejected(t, store, record, approval, tc.mutate)
			expectations = append(expectations, ledgerValidationExpectation{actionID: record.ID})
		})
	}

	duplicateRecord := ledgerValidationRecord("act_ledger_duplicate", unified.ApprovalAdmin, 2, false, now.Add(20*time.Second))
	firstApproval := ledgerValidationApproval(duplicateRecord, user("duplicate"), unified.MethodSession, duplicateRecord.CreatedAt.Add(time.Minute))
	initial := []unified.ActionLifecycleEvent{{ActionID: duplicateRecord.ID, Timestamp: duplicateRecord.CreatedAt, State: unified.ActionStatePlanned}, {ActionID: duplicateRecord.ID, Timestamp: duplicateRecord.CreatedAt, State: unified.ActionStatePending}}
	if _, created, err := store.CreateActionAudit(duplicateRecord, initial); err != nil || !created {
		t.Fatalf("create duplicate audit created=%v err=%v", created, err)
	}
	first, firstEvent, _ := unified.ApplyActionDecision(duplicateRecord, firstApproval, firstApproval.Timestamp)
	if err := store.RecordActionDecision(first, firstEvent); err != nil {
		t.Fatal(err)
	}
	duplicateApproval := ledgerValidationApproval(first, user("duplicate"), unified.MethodSession, firstApproval.Timestamp.Add(time.Second))
	proposed := first
	proposed.DecisionRevision++
	proposed.Approvals = append(append([]unified.ActionApprovalRecord(nil), first.Approvals...), duplicateApproval)
	duplicateEvent := unified.ActionLifecycleEvent{ActionID: proposed.ID, Timestamp: duplicateApproval.Timestamp, State: unified.ActionStatePending, Kind: unified.ActionLifecycleEventDecision, DecisionRevision: proposed.DecisionRevision, Decision: &proposed.Approvals[1], Actor: duplicateApproval.Actor, Message: "Approval recorded; 1 of 2 distinct approvals collected."}
	beforeEvents, _ := store.GetActionLifecycleEvents(proposed.ID, time.Time{}, 20)
	if err := store.RecordActionDecision(proposed, duplicateEvent); err == nil {
		t.Fatal("duplicate actor append unexpectedly persisted")
	}
	authoritative, _, _ := store.GetActionAudit(proposed.ID)
	afterEvents, _ := store.GetActionLifecycleEvents(proposed.ID, time.Time{}, 20)
	if authoritative.DecisionRevision != 1 || len(authoritative.Approvals) != 1 || len(afterEvents) != len(beforeEvents) {
		t.Fatalf("duplicate actor mutated ledger: record=%#v events=%#v", authoritative, afterEvents)
	}
	expectations = append(expectations, ledgerValidationExpectation{actionID: proposed.ID, decisionRevision: 1, approvalCount: 1})
	return expectations
}

func assertDecisionEventIdentityAndTransitionUniqueness(t *testing.T, store unified.ResourceStore) {
	t.Helper()
	actionID := persistTwoPendingDecisionRevisions(t, store)
	events, err := store.GetActionLifecycleEvents(actionID, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	decisionEvents := map[uint64]unified.ActionLifecycleEvent{}
	for _, event := range events {
		if event.Kind == unified.ActionLifecycleEventDecision {
			decisionEvents[event.DecisionRevision] = event
		}
	}
	if len(decisionEvents) != 2 || decisionEvents[1].Decision == nil || decisionEvents[2].Decision == nil {
		t.Fatalf("decision events=%#v, want durable revisions one and two", decisionEvents)
	}
	if err := store.RecordActionLifecycleEvent(decisionEvents[1]); err == nil {
		t.Fatal("duplicate decision revision should be rejected")
	}
	current, found, err := store.GetActionAudit(actionID)
	if err != nil || !found {
		t.Fatalf("current audit found=%v err=%v", found, err)
	}
	thirdActor := testActionActor("admin-three", "default")
	thirdEvidence := unified.ApprovalEvidence{Version: 1, Method: unified.MethodSession, Actor: thirdActor, OrgID: "default", ActionID: actionID, PlanHash: current.Plan.PlanHash, Outcome: unified.OutcomeApproved, IssuedAt: time.Now().UTC()}
	thirdApproval := unified.ActionApprovalRecord{Actor: thirdActor.SubjectID, ActorBinding: thirdActor, Method: unified.MethodSession, Timestamp: thirdEvidence.IssuedAt, Outcome: unified.OutcomeApproved, Reason: "approve admin-three", Evidence: &thirdEvidence}
	desired, desiredEvent, err := unified.ApplyActionDecision(current, thirdApproval, thirdApproval.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	cloneAppend := func(record unified.ActionAuditRecord, event unified.ActionLifecycleEvent) (unified.ActionAuditRecord, unified.ActionLifecycleEvent) {
		record.Approvals = append([]unified.ActionApprovalRecord(nil), record.Approvals...)
		for index := range record.Approvals {
			if record.Approvals[index].Evidence != nil {
				evidence := *record.Approvals[index].Evidence
				record.Approvals[index].Evidence = &evidence
			}
		}
		event.Decision = &record.Approvals[len(record.Approvals)-1]
		return record, event
	}
	type maliciousAppend struct {
		name   string
		record unified.ActionAuditRecord
		event  unified.ActionLifecycleEvent
	}
	var attempts []maliciousAppend
	replaced, replacedEvent := cloneAppend(desired, desiredEvent)
	replaced.Approvals[0] = replaced.Approvals[1]
	attempts = append(attempts, maliciousAppend{"replaced prior approval", replaced, replacedEvent})
	reordered, reorderedEvent := cloneAppend(desired, desiredEvent)
	reordered.Approvals[0], reordered.Approvals[1] = reordered.Approvals[1], reordered.Approvals[0]
	attempts = append(attempts, maliciousAppend{"reordered prior approvals", reordered, reorderedEvent})
	removed, removedEvent := cloneAppend(desired, desiredEvent)
	removed.Approvals = removed.Approvals[1:]
	removedEvent.Decision = &removed.Approvals[len(removed.Approvals)-1]
	attempts = append(attempts, maliciousAppend{"removed prior approval", removed, removedEvent})
	edited, editedEvent := cloneAppend(desired, desiredEvent)
	edited.Approvals[0].Reason = "edited prior reason"
	attempts = append(attempts, maliciousAppend{"edited prior approval", edited, editedEvent})
	stateMismatch, stateMismatchEvent := cloneAppend(desired, desiredEvent)
	stateMismatchEvent.State = unified.ActionStatePending
	attempts = append(attempts, maliciousAppend{"suppressed approved transition", stateMismatch, stateMismatchEvent})
	for _, field := range []string{"org", "plan", "action"} {
		wrongEvidence, wrongEvidenceEvent := cloneAppend(desired, desiredEvent)
		switch field {
		case "org":
			wrongEvidence.Approvals[2].Evidence.OrgID = "other"
		case "plan":
			wrongEvidence.Approvals[2].Evidence.PlanHash = "sha256:other"
		case "action":
			wrongEvidence.Approvals[2].Evidence.ActionID = "act_other"
		}
		wrongEvidenceEvent.Decision = &wrongEvidence.Approvals[2]
		attempts = append(attempts, maliciousAppend{"wrong evidence " + field, wrongEvidence, wrongEvidenceEvent})
	}
	rejectActor := testActionActor("rejector", "default")
	rejectEvidence := unified.ApprovalEvidence{Version: 1, Method: unified.MethodSession, Actor: rejectActor, OrgID: "default", ActionID: actionID, PlanHash: current.Plan.PlanHash, Outcome: unified.OutcomeRejected, IssuedAt: thirdApproval.Timestamp}
	rejectApproval := unified.ActionApprovalRecord{Actor: rejectActor.SubjectID, ActorBinding: rejectActor, Method: unified.MethodSession, Timestamp: thirdApproval.Timestamp, Outcome: unified.OutcomeRejected, Reason: "reject", Evidence: &rejectEvidence}
	forgedRejected, forgedRejectedEvent, err := unified.ApplyActionDecision(current, rejectApproval, rejectApproval.Timestamp)
	if err != nil {
		t.Fatal(err)
	}
	forgedRejected.State = unified.ActionStateApproved
	forgedRejectedEvent.State = unified.ActionStateApproved
	forgedRejectedEvent.Message = "Action approved. Execution remains pending a separate execution contract."
	attempts = append(attempts, maliciousAppend{"forged rejected transition", forgedRejected, forgedRejectedEvent})
	for _, attempt := range attempts {
		if err := store.RecordActionDecision(attempt.record, attempt.event); err == nil {
			t.Fatalf("%s error=%v", attempt.name, err)
		}
	}
	afterConflict, _, _ := store.GetActionAudit(actionID)
	if afterConflict.DecisionRevision != current.DecisionRevision || len(afterConflict.Approvals) != len(current.Approvals) || afterConflict.State != current.State {
		t.Fatalf("conflicting rewrite mutated audit: before=%#v after=%#v", current, afterConflict)
	}
	for _, state := range []unified.ActionState{unified.ActionStatePlanned, unified.ActionStateExecuting, unified.ActionStateCompleted} {
		event := unified.ActionLifecycleEvent{ActionID: actionID, Timestamp: time.Now().UTC(), State: state, Actor: "test"}
		if state != unified.ActionStatePlanned {
			if err := store.RecordActionLifecycleEvent(event); err != nil {
				t.Fatalf("first %s transition: %v", state, err)
			}
		}
		if err := store.RecordActionLifecycleEvent(event); err == nil {
			t.Fatalf("duplicate %s transition should be rejected", state)
		}
	}
}

func TestDecisionEventIdentityAndTransitionUniquenessMemoryStore(t *testing.T) {
	store := unified.NewMemoryStore()
	assertDecisionEventIdentityAndTransitionUniqueness(t, store)
	_ = assertStoreLedgerRejectsMalformedAuthority(t, store)
}

func TestDecisionEventIdentityAndTransitionUniquenessSQLiteStore(t *testing.T) {
	dir := t.TempDir()
	store, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	assertDecisionEventIdentityAndTransitionUniqueness(t, store)
	expectations := assertStoreLedgerRejectsMalformedAuthority(t, store)
	reopened, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	for _, expectation := range expectations {
		authoritative, found, err := reopened.GetActionAudit(expectation.actionID)
		if err != nil || !found || authoritative.DecisionRevision != expectation.decisionRevision || len(authoritative.Approvals) != expectation.approvalCount {
			t.Fatalf("reopen changed rejected ledger chain for %s: found=%v err=%v record=%#v", expectation.actionID, found, err, authoritative)
		}
	}
}

func TestSQLiteReopenPreservesDecisionEventRevisionOrdering(t *testing.T) {
	dir := t.TempDir()
	store, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	actionID := persistTwoPendingDecisionRevisions(t, store)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := unified.NewSQLiteResourceStore(dir, "default")
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	events, err := reopened.GetActionLifecycleEvents(actionID, time.Time{}, 20)
	if err != nil {
		t.Fatal(err)
	}
	var revisions []uint64
	for _, event := range events {
		if event.Kind == unified.ActionLifecycleEventDecision {
			revisions = append(revisions, event.DecisionRevision)
			if event.Decision == nil || event.Decision.Evidence == nil {
				t.Fatalf("reopened decision event lost binding: %#v", event)
			}
		}
	}
	if len(revisions) != 2 || revisions[0] != 2 || revisions[1] != 1 {
		t.Fatalf("reopened decision revisions=%v, want [2 1]", revisions)
	}
}

func TestDecideRejectsDuplicateActorTowardQuorum(t *testing.T) {
	service, _ := actorApprovalService(t, unified.ApprovalAdmin)
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	requirement.Quorum = 2
	plan := planForApprovalTest(t, service, &requirement)
	actor := testActionActor("admin-one", "default")
	first := decisionForApprovalTest(plan, actor, unified.OutcomeApproved, "approve", unified.MethodSession, "", time.Now().UTC())
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, first); err != nil {
		t.Fatal(err)
	}
	conflict := first
	conflict.Evidence.Method = unified.MethodAPIToken
	if _, err := service.Decide(context.Background(), "default", plan.ActionID, conflict); !errors.Is(err, ErrDecisionReplayConflict) {
		t.Fatalf("error=%v", err)
	}
}

func TestExecuteRejectsLegacyUnboundApproval(t *testing.T) {
	service, store := actorApprovalService(t, unified.ApprovalAdmin)
	now := time.Now().UTC()
	record := unified.ActionAuditRecord{
		ID: "act_legacy", CreatedAt: now, UpdatedAt: now, State: unified.ActionStateApproved,
		Request:   unified.ActionRequest{RequestID: "legacy", ResourceID: "vm:42", CapabilityName: "restart", Reason: "legacy", RequestedBy: "legacy"},
		Plan:      unified.ActionPlan{ActionID: "act_legacy", RequestID: "legacy", Allowed: true, RequiresApproval: true, ApprovalPolicy: unified.ApprovalAdmin, PlannedAt: now, ExpiresAt: now.Add(time.Minute), PlanHash: "sha256:legacy"},
		Approvals: []unified.ActionApprovalRecord{{Actor: "legacy-admin", Method: unified.MethodAPI, Outcome: unified.OutcomeApproved, Timestamp: now}},
	}
	if err := store.RecordActionAudit(record); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", record.ID, testActionActor("admin", "default"), ""); !errors.Is(err, unified.ErrActionReplanRequired) {
		t.Fatalf("error=%v", err)
	}
}

func TestExecuteRejectsDryRunOnlyRegardlessOfTenantOverride(t *testing.T) {
	service, _ := actorApprovalService(t, unified.ApprovalDryRun)
	requirement := unified.ApprovalRequirementForFloor(unified.ApprovalAdmin)
	if _, err := service.PlanWithOptions(context.Background(), "default", restartRequest(), PlanOptions{Actor: testActionActor("requester", "default"), ApprovalRequirement: &requirement}); err == nil {
		t.Fatal("expected dry-run capability floor lowering to fail")
	}
}
