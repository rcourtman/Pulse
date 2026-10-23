package ai

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

type concurrencyRecordingOrchestrator struct {
	stubInvestigationOrchestrator
	maxConcurrent int
}

func (s *concurrencyRecordingOrchestrator) SetInvestigationConcurrencyLimit(maxConcurrent int) {
	s.maxConcurrent = maxConcurrent
}

func TestLoadConfigPublishesProviderAwareInvestigationLimits(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	saved := config.NewDefaultAIConfig()
	saved.PatrolModel = config.AIProviderCodexSubscription + ":gpt-5.6-luna"
	if err := persistence.SaveAIConfig(*saved); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}

	orchestrator := &concurrencyRecordingOrchestrator{}
	service := &Service{
		persistence:   persistence,
		patrolService: &PatrolService{investigationOrchestrator: orchestrator},
	}
	if err := service.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if orchestrator.maxConcurrent != 1 {
		t.Fatalf("concurrency = %d, want 1 for a subscription agent that serves one call at a time", orchestrator.maxConcurrent)
	}
	if want := time.Duration(config.DefaultPatrolSubscriptionInvestigationTimeoutSec) * time.Second; orchestrator.timeout != want {
		t.Fatalf("timeout = %s, want %s", orchestrator.timeout, want)
	}

	// Switching Patrol to an API-key provider restores the normal limits on
	// the next config load, without rebuilding the orchestrator.
	saved.PatrolModel = config.AIProviderAnthropic + ":claude-sonnet-4-5"
	if err := persistence.SaveAIConfig(*saved); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := service.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if orchestrator.maxConcurrent != config.MaxConcurrentInvestigations {
		t.Fatalf("concurrency = %d, want %d", orchestrator.maxConcurrent, config.MaxConcurrentInvestigations)
	}
	if want := time.Duration(config.DefaultPatrolInvestigationTimeoutSec) * time.Second; orchestrator.timeout != want {
		t.Fatalf("timeout = %s, want %s", orchestrator.timeout, want)
	}
}
