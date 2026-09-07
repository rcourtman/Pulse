package actionlifecycle

import (
	"context"
	"errors"
	"testing"
	"time"

	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestPlanRequestReplayRetainsAcceptedPlanWithoutLiveRegistry(t *testing.T) {
	store := unified.NewMemoryStore()
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
	actor := testActionActor("requester", "default")
	request := restartRequest()
	plan, err := service.Plan(context.Background(), "default", request, actor)
	if err != nil {
		t.Fatal(err)
	}
	service.Registry = func(string) (*unified.ResourceRegistry, error) {
		t.Fatal("replay consulted live registry")
		return nil, nil
	}
	stronger := unified.ApprovalRequirementForFloor(unified.ApprovalMultiFactor)
	replay, err := service.PlanWithOptions(context.Background(), "default", request, PlanOptions{Actor: actor, ApprovalRequirement: &stronger})
	if err != nil || replay.ActionID != plan.ActionID || replay.PlanHash != plan.PlanHash || !replay.ExpiresAt.Equal(plan.ExpiresAt) {
		t.Fatalf("replay changed accepted plan: %#v %v", replay, err)
	}
}

func TestPlanRequestConflictCannotCreateAnotherAction(t *testing.T) {
	store := unified.NewMemoryStore()
	service := serviceForStore(t, store, testResource(time.Now().UTC(), unified.ApprovalAdmin), &stubExecutor{})
	actor := testActionActor("requester", "default")
	request := restartRequest()
	plan, err := service.Plan(context.Background(), "default", request, actor)
	if err != nil {
		t.Fatal(err)
	}
	registry := service.Registry
	service.Registry = func(string) (*unified.ResourceRegistry, error) {
		t.Fatal("conflicting request consulted live registry")
		return nil, nil
	}
	request.Reason = "different intent on same request identity"
	if _, err := service.Plan(context.Background(), "default", request, actor); !errors.Is(err, unified.ErrActionIdentityConflict) {
		t.Fatalf("conflicting replay: %v", err)
	}
	record, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || record.Request.Reason == request.Reason {
		t.Fatalf("original record changed: %#v %v", record, err)
	}
	service.Registry = registry
	request.RequestID += "-new"
	replacement, err := service.Plan(context.Background(), "default", request, actor)
	if err != nil || replacement.ActionID == plan.ActionID {
		t.Fatalf("explicit new request failed: %#v %v", replacement, err)
	}
}
