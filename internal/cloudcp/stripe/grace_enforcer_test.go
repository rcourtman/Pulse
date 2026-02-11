package stripe

import (
	"testing"
	"time"
)

func TestGraceEnforcer_MaxGraceDays(t *testing.T) {
	if maxGraceDays != 14 {
		t.Errorf("expected maxGraceDays=14, got %d", maxGraceDays)
	}
}

func TestGraceEnforcer_CheckInterval(t *testing.T) {
	if graceCheckInterval != 1*time.Hour {
		t.Errorf("expected graceCheckInterval=1h, got %v", graceCheckInterval)
	}
}
