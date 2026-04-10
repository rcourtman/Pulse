package mockmode

import (
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
)

func TestIsEnabledReflectsRuntimeState(t *testing.T) {
	original := mockruntime.IsEnabled()
	t.Cleanup(func() { mockruntime.SetEnabled(original) })

	for _, enabled := range []bool{false, true} {
		mockruntime.SetEnabled(enabled)
		if got := IsEnabled(); got != enabled {
			t.Fatalf("IsEnabled() = %v, want %v", got, enabled)
		}
	}
}
