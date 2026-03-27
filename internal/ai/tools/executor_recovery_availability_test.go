package tools

import (
	"context"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/recovery"
)

type availabilityRecoveryPointsProvider struct{}

func (p *availabilityRecoveryPointsProvider) ListPoints(ctx context.Context, opts recovery.ListPointsOptions) ([]recovery.RecoveryPoint, int, error) {
	return nil, 0, nil
}

func TestPulseStorageAvailableWithRecoveryPointsProviderOnly(t *testing.T) {
	exec := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider: &availabilityRecoveryPointsProvider{},
	})

	tools := exec.ListTools()
	for _, tool := range tools {
		if tool.Name == "pulse_storage" {
			return
		}
	}

	t.Fatalf("expected pulse_storage to be available when recovery points provider is configured")
}
