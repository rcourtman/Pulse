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
