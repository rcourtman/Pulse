package updates

import (
	"context"
	"testing"
)

func TestMockUpdaterBasics(t *testing.T) {
	updater := NewMockUpdater()
	if updater == nil {
		t.Fatal("expected updater")
	}
	if !updater.SupportsApply() {
		t.Fatal("expected SupportsApply true")
	}
	if updater.GetDeploymentType() != "mock" {
		t.Fatalf("unexpected deployment type: %s", updater.GetDeploymentType())
	}

	plan, err := updater.PrepareUpdate(context.Background(), UpdateRequest{Version: "1.2.3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan == nil || !plan.CanAutoUpdate || len(plan.Instructions) == 0 {
		t.Fatal("unexpected plan result")
	}
}

func TestMockUpdaterExecute(t *testing.T) {
	updater := NewMockUpdater()
	var stages []UpdateProgress
	err := updater.Execute(context.Background(), UpdateRequest{}, func(stage UpdateProgress) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stages) == 0 || !stages[len(stages)-1].IsComplete {
		t.Fatalf("unexpected stages: %+v", stages)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = updater.Execute(ctx, UpdateRequest{}, func(UpdateProgress) {})
	if err == nil {
		t.Fatal("expected context error")
	}
}
