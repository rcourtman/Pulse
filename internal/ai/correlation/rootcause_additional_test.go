package correlation

import (
	"strings"
	"testing"
	"time"
)

type stubTopologyProvider struct {
	relationships []ResourceRelationship
	types         map[string]string
	names         map[string]string
}

func (s *stubTopologyProvider) GetRelationships(resourceID string) []ResourceRelationship {
	return s.relationships
}

func (s *stubTopologyProvider) GetResourceType(resourceID string) string {
	return s.types[resourceID]
}

func (s *stubTopologyProvider) GetResourceName(resourceID string) string {
	return s.names[resourceID]
}

type stubEventProvider struct {
	events map[string][]RelatedEvent
}

func (s *stubEventProvider) GetRecentEvents(resourceID string, window time.Duration) []RelatedEvent {
	return s.events[resourceID]
}

func TestRootCauseEngine_DefaultsAndNilProviders(t *testing.T) {
	engine := NewRootCauseEngine(RootCauseEngineConfig{})
	if engine.config.CorrelationWindow == 0 || engine.config.MaxChainLength == 0 {
		t.Fatalf("expected defaults to be applied")
	}
	if engine.Analyze(RelatedEvent{}) != nil {
		t.Fatalf("expected nil analysis without providers")
	}
}

func TestRootCauseEngine_AnalyzeAndFormat(t *testing.T) {
	triggerTime := time.Now()
	topology := &stubTopologyProvider{
		relationships: []ResourceRelationship{
			{SourceID: "node-1", TargetID: "vm-1", Relationship: RelationshipRunsOn},
		},
		types: map[string]string{
			"node-1": "node",
			"vm-1":   "vm",
		},
		names: map[string]string{
			"node-1": "Node 1",
			"vm-1":   "VM 1",
		},
	}
	events := &stubEventProvider{
		events: map[string][]RelatedEvent{
			"node-1": {
				{
					ResourceID:   "node-1",
					ResourceType: "node",
					EventType:    "alert",
					Metric:       "cpu",
					Value:        95,
					Timestamp:    triggerTime.Add(-30 * time.Second),
					Description:  "CPU spike",
				},
			},
		},
	}

	engine := NewRootCauseEngine(DefaultRootCauseEngineConfig())
	engine.config.MinConfidence = 0.1
	engine.SetTopologyProvider(topology)
	engine.SetEventProvider(events)

	trigger := RelatedEvent{
		ResourceID:   "vm-1",
		ResourceName: "VM 1",
		ResourceType: "vm",
		EventType:    "alert",
		Metric:       "cpu",
		Timestamp:    triggerTime,
		Description:  "VM alert",
	}

	analysis := engine.Analyze(trigger)
	if analysis == nil || analysis.RootCause == nil {
		t.Fatalf("expected root cause analysis")
	}
	if analysis.Explanation == "" {
		t.Fatalf("expected explanation")
	}
	if analysis.Confidence <= 0 {
		t.Fatalf("expected confidence")
	}

	context := engine.FormatForContext("vm-1")
	if !strings.Contains(context, "confidence") {
		t.Fatalf("expected context output")
	}

	patrol := engine.FormatAnalysisForPatrol()
	if patrol == "" {
		t.Fatalf("expected patrol output")
	}
}

func TestRootCauseEngine_ScoringAndHelpers(t *testing.T) {
	engine := NewRootCauseEngine(DefaultRootCauseEngineConfig())

	trigger := RelatedEvent{ResourceID: "vm-1", ResourceType: "vm", Metric: "cpu", Timestamp: time.Now()}
	candidate := RelatedEvent{ResourceID: "node-1", ResourceType: "node", Metric: "cpu", Timestamp: time.Now().Add(-30 * time.Second)}
	relationships := []ResourceRelationship{{SourceID: "node-1", TargetID: "vm-1", Relationship: RelationshipRunsOn}}

	score := engine.scoreAsRootCause(&candidate, trigger, relationships)
	if score <= 0 {
		t.Fatalf("expected score > 0")
	}

	root := engine.identifyRootCause(trigger, []RelatedEvent{candidate}, relationships)
	if root == nil {
		t.Fatalf("expected root cause")
	}

	chain := engine.buildCausalChain(root, trigger, []RelatedEvent{candidate}, relationships)
	if len(chain) < 2 {
		t.Fatalf("expected causal chain")
	}

	confidence := engine.calculateConfidence(&RootCauseAnalysis{RootCause: root, RelatedEvents: []RelatedEvent{candidate}, CausalChain: chain, TriggerEvent: trigger})
	if confidence <= 0 {
		t.Fatalf("expected confidence")
	}

	if formatEventForChain(&RelatedEvent{ResourceID: "r1", Description: "desc"}) == "" {
		t.Fatalf("expected formatted chain event")
	}
	if !isRelatedMetric("cpu", "load") || isRelatedMetric("cpu", "disk") {
		t.Fatalf("unexpected metric relation result")
	}
	if minFloat(1, 2) != 1 {
		t.Fatalf("expected min float")
	}
}

func TestRootCauseEngine_AnalysisQueries(t *testing.T) {
	engine := NewRootCauseEngine(DefaultRootCauseEngineConfig())
	engine.recentAnalyses = []RootCauseAnalysis{
		{
			ID:           "a1",
			TriggerEvent: RelatedEvent{ResourceID: "vm-1"},
		},
	}

	if len(engine.GetRecentAnalyses(1)) != 1 {
		t.Fatalf("expected recent analysis")
	}
	if len(engine.GetAnalysisForResource("vm-1")) != 1 {
		t.Fatalf("expected analysis for resource")
	}
}
