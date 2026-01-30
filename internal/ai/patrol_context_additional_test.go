package ai

import (
	"strings"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestPatrolService_buildSeedContext_QuietSummary(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	ps.SetConfig(PatrolConfig{
		Enabled:      true,
		AnalyzeNodes: true,
	})

	state := models.StateSnapshot{
		Nodes: []models.Node{
			{
				ID:     "node-1",
				Name:   "node-1",
				Status: "online",
				CPU:    0.10,
				Memory: models.Memory{Usage: 20.0},
			},
		},
	}

	seed, _ := ps.buildSeedContext(state, nil)
	if !strings.Contains(seed, "# Nodes: All 1 healthy") {
		t.Fatalf("expected quiet node summary, got:\n%s", seed)
	}
}

func TestPatrolService_buildSeedContext_ScopeSection(t *testing.T) {
	ps := NewPatrolService(nil, nil)
	scope := &PatrolScope{
		Reason:        TriggerReasonAlertFired,
		Context:       "CPU alert",
		ResourceIDs:   []string{"node-1"},
		ResourceTypes: []string{"node"},
		AlertID:       "alert-123",
		FindingID:     "finding-456",
		Depth:         PatrolDepthQuick,
	}

	seed, _ := ps.buildSeedContext(models.StateSnapshot{}, scope)
	if !strings.Contains(seed, "# Patrol Scope") {
		t.Fatalf("expected patrol scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Trigger: alert") {
		t.Fatalf("expected trigger in scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Context: CPU alert") {
		t.Fatalf("expected context in scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Focus resources: node-1") {
		t.Fatalf("expected resource IDs in scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Resource types: node") {
		t.Fatalf("expected resource types in scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Alert ID: alert-123") {
		t.Fatalf("expected alert ID in scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Finding ID: finding-456") {
		t.Fatalf("expected finding ID in scope section, got:\n%s", seed)
	}
	if !strings.Contains(seed, "Depth: quick") {
		t.Fatalf("expected depth in scope section, got:\n%s", seed)
	}
}
