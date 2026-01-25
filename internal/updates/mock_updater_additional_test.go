package updates

import (
	"context"
	"testing"
)

func TestMockUpdaterRollback(t *testing.T) {
	updater := NewMockUpdater()
	if err := updater.Rollback(context.Background(), "event-1"); err != nil {
		t.Fatalf("Rollback returned error: %v", err)
	}
}
