package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/circuit"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/knowledge"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/remediation"
	"github.com/rcourtman/pulse-go-rewrite/internal/servicediscovery"
)

type mockLearningProvider struct{}

func (m mockLearningProvider) FormatForContext() string { return "prefs" }

type mockProxmoxEventProvider struct{}

func (m mockProxmoxEventProvider) FormatForPatrol(duration time.Duration) string { return "events" }

type mockForecastProvider struct{}

func (m mockForecastProvider) FormatKeyForecasts() string { return "forecast" }

type stubInvestigationOrchestrator struct{}

func (s *stubInvestigationOrchestrator) InvestigateFinding(ctx context.Context, finding *InvestigationFinding, autonomyLevel string) error {
	return nil
}

func (s *stubInvestigationOrchestrator) GetInvestigationByFinding(findingID string) *InvestigationSession {
	return nil
}

func (s *stubInvestigationOrchestrator) GetRunningCount() int { return 0 }

func (s *stubInvestigationOrchestrator) GetFixedCount() int { return 0 }

func (s *stubInvestigationOrchestrator) CanStartInvestigation() bool { return true }

func (s *stubInvestigationOrchestrator) ReinvestigateFinding(ctx context.Context, findingID, autonomyLevel string) error {
	return nil
}

func (s *stubInvestigationOrchestrator) Shutdown(ctx context.Context) error { return nil }

func TestPatrolService_SettersAndScopeHints(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	ks := &knowledge.Store{}
	ps.SetKnowledgeStore(ks)
	if ps.GetKnowledgeStore() != ks {
		t.Fatalf("expected knowledge store to be set")
	}

	discoveryStore, err := servicediscovery.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create discovery store: %v", err)
	}
	ps.SetDiscoveryStore(discoveryStore)
	if ps.GetDiscoveryStore() == nil {
		t.Fatalf("expected discovery store to be set")
	}

	lp := mockLearningProvider{}
	ps.SetLearningProvider(lp)
	if ps.learningProvider == nil {
		t.Fatalf("expected learning provider to be set")
	}

	pep := mockProxmoxEventProvider{}
	ps.SetProxmoxEventProvider(pep)
	if ps.proxmoxEventProvider == nil {
		t.Fatalf("expected proxmox event provider to be set")
	}

	fp := mockForecastProvider{}
	ps.SetForecastProvider(fp)
	if ps.forecastProvider == nil {
		t.Fatalf("expected forecast provider to be set")
	}

	tm := NewTriggerManager(TriggerManagerConfig{MaxPendingTriggers: 1})
	ps.SetTriggerManager(tm)
	if ps.GetTriggerManager() == nil {
		t.Fatalf("expected trigger manager to be set")
	}

	discovery := &servicediscovery.ResourceDiscovery{
		ID:           "vm:node1:101",
		ResourceType: servicediscovery.ResourceTypeVM,
		ResourceID:   "101",
		HostID:       "node1",
		Hostname:     "node1",
		ServiceName:  "nginx",
		ServiceType:  "nginx",
	}
	if err := discoveryStore.Save(discovery); err != nil {
		t.Fatalf("failed to save discovery: %v", err)
	}

	scope := PatrolScope{ResourceIDs: []string{"101"}}
	updated := ps.addDiscoveryScopeHint(scope)
	if updated.Context == "" || !strings.Contains(updated.Context, "Discovery:") {
		t.Fatalf("expected discovery hint in scope context")
	}
}

func TestTruncateScopeContext(t *testing.T) {
	if truncateScopeContext("short", 10) != "short" {
		t.Fatalf("expected short string to remain unchanged")
	}
	if truncateScopeContext("long-value", 3) != "lon" {
		t.Fatalf("expected hard truncation for small max")
	}
	if !strings.HasSuffix(truncateScopeContext("long-value", 6), "...") {
		t.Fatalf("expected ellipsis for truncated context")
	}
}

func TestPatrolService_AdditionalSetters(t *testing.T) {
	ps := NewPatrolService(nil, nil)

	breaker := circuit.NewBreaker("test", circuit.Config{})
	ps.SetCircuitBreaker(breaker)
	if ps.circuitBreaker != breaker {
		t.Fatalf("expected circuit breaker to be set")
	}

	engine := remediation.NewEngine(remediation.DefaultEngineConfig())
	ps.SetRemediationEngine(engine)
	if ps.GetRemediationEngine() != engine {
		t.Fatalf("expected remediation engine to be set")
	}

	orchestrator := &stubInvestigationOrchestrator{}
	ps.SetInvestigationOrchestrator(orchestrator)
	if ps.GetInvestigationOrchestrator() != orchestrator {
		t.Fatalf("expected investigation orchestrator to be set")
	}

	callbackCalled := false
	ps.SetUnifiedFindingCallback(func(f *Finding) bool {
		callbackCalled = true
		return true
	})
	if ps.unifiedFindingCallback == nil {
		t.Fatalf("expected unified finding callback to be set")
	}
	ps.unifiedFindingCallback(&Finding{})
	if !callbackCalled {
		t.Fatalf("expected unified finding callback to be invoked")
	}

	var resolvedID string
	ps.SetUnifiedFindingResolver(func(id string) {
		resolvedID = id
	})
	if ps.unifiedFindingResolver == nil {
		t.Fatalf("expected unified finding resolver to be set")
	}
	ps.unifiedFindingResolver("finding-1")
	if resolvedID != "finding-1" {
		t.Fatalf("expected unified finding resolver to capture id, got %q", resolvedID)
	}
}
