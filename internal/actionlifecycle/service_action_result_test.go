package actionlifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestExecutorTupleNilNilPersistsInconclusiveContractViolation(t *testing.T) {
	store := unified.NewMemoryStore()
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalNone), &stubExecutor{})
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "nil tuple")
	if err != nil {
		t.Fatal(err)
	}
	truth := unified.CanonicalActionResultV2(record)
	if record.State != unified.ActionStateFailed || truth.Execution.Status != unified.ActionExecutionInconclusive || truth.Execution.ReasonCode != "executor_nil_result" {
		t.Fatalf("record=%#v truth=%#v", record, truth)
	}
	stored, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || unified.CanonicalActionResultV2(stored).Execution.Status != unified.ActionExecutionInconclusive {
		t.Fatalf("stored=%#v found=%v err=%v", stored, found, err)
	}
}

func TestExecutorTupleResultAndErrorIsContractViolation(t *testing.T) {
	store := unified.NewMemoryStore()
	executor := &stubExecutor{result: &unified.ExecutionResult{Success: true}, err: context.DeadlineExceeded}
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "invalid tuple")
	if !errors.Is(err, unified.ErrExecutorResultContract) || record.State != unified.ActionStateExecuting {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	attempt, found, getErr := store.GetActionDispatchAttempt(plan.ActionID)
	if getErr != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("attempt=%#v found=%v err=%v", attempt, found, getErr)
	}
}

func TestExecutorTuplePostDispatchErrorPreservesReceiptPending(t *testing.T) {
	store := unified.NewMemoryStore()
	executor := &stubExecutor{err: context.DeadlineExceeded}
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "timeout")
	if !errors.Is(err, context.DeadlineExceeded) || record.State != unified.ActionStateExecuting {
		t.Fatalf("record=%#v err=%v", record, err)
	}
	attempt, found, getErr := store.GetActionDispatchAttempt(plan.ActionID)
	if getErr != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("attempt=%#v found=%v err=%v", attempt, found, getErr)
	}
}
