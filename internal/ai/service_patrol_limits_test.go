package ai

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// A config load reads the stored record and then builds the provider, which
// can wait on the provider's model catalog. An autonomy change persisted and
// published during that wait must survive the load's own publication.
func TestLoadConfigKeepsPatrolAutonomyAppliedWhileProviderWasResolving(t *testing.T) {
	persistence := config.NewConfigPersistence(t.TempDir())
	service := &Service{persistence: persistence}

	var catalogCalls atomic.Int32
	catalog := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if catalogCalls.Add(1) == 1 {
			next := config.NewDefaultAIConfig()
			next.PatrolAutonomyLevel = config.PatrolAutonomyMonitor
			next.PatrolInvestigationBudget = 20
			if err := service.ApplyPatrolAutonomyConfig(next); err != nil {
				t.Errorf("ApplyPatrolAutonomyConfig: %v", err)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest","model":"llama3:latest"}]}`))
	}))
	defer catalog.Close()

	saved := config.NewDefaultAIConfig()
	saved.PatrolAutonomyLevel = config.PatrolAutonomyApproval
	saved.PatrolInvestigationBudget = 10
	if err := persistence.SaveAIConfig(*saved); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := service.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	// Only now does the load need the catalog: enabled, Ollama configured, no model chosen.
	saved.Enabled = true
	saved.OllamaBaseURL = catalog.URL
	if err := persistence.SaveAIConfig(*saved); err != nil {
		t.Fatalf("SaveAIConfig: %v", err)
	}
	if err := service.LoadConfig(); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if catalogCalls.Load() == 0 {
		t.Fatal("provider catalog was not consulted, so the load never waited")
	}
	cfg := service.GetConfig()
	if !cfg.Enabled || cfg.OllamaBaseURL != catalog.URL {
		t.Fatalf("load did not publish the stored settings: enabled=%t ollama=%q", cfg.Enabled, cfg.OllamaBaseURL)
	}
	if cfg.PatrolAutonomyLevel != config.PatrolAutonomyMonitor || cfg.PatrolInvestigationBudget != 20 {
		t.Fatalf("autonomy = %q budget = %d, want the mode applied during the load (monitor, 20)", cfg.PatrolAutonomyLevel, cfg.PatrolInvestigationBudget)
	}
}
