package updates

import (
	"context"
	"testing"
)

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func TestInstallShAdapter_PrepareUpdate(t *testing.T) {
	adapter := NewInstallShAdapter()

	plan, err := adapter.PrepareUpdate(context.Background(), UpdateRequest{Version: "v1.2.3"})
	if err != nil {
		t.Fatalf("PrepareUpdate error: %v", err)
	}
	if !plan.CanAutoUpdate || !plan.RequiresRoot || !plan.RollbackSupport {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if len(plan.Instructions) == 0 || len(plan.Prerequisites) == 0 {
		t.Fatalf("expected instructions and prerequisites: %+v", plan)
	}
	if !containsString(plan.Prerequisites, "About 1.2GB free disk space for update staging") {
		t.Fatalf("expected update staging disk prerequisite, got %+v", plan.Prerequisites)
	}
}
