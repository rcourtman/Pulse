package api

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Regression coverage for #1437: adaptive polling stretches an instance's
// cadence past the configured interval while data is fresh, so the
// active→stale cutoff has to follow the planned schedule. Before the fix a
// healthy connection on a stretched (e.g. 5-minute) cadence read as stale for
// the back half of every cycle and the platform page dropped to Agent-only.
func TestBuildConnectionsAdaptiveIntervalScalesStaleCutoff(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	lastSuccess := now.Add(-4 * time.Minute)
	base := aggregatorInputs{
		pveInstances: []config.PVEInstance{{Name: "lab", Host: "https://lab:8006"}},
		instanceHealth: map[string]monitoring.InstanceHealth{
			"pve::lab": {PollStatus: monitoring.InstancePollStatus{LastSuccess: &lastSuccess}},
		},
		pvePollingInterval: 30 * time.Second,
		now:                now,
	}

	pveState := func(in aggregatorInputs) ConnectionState {
		t.Helper()
		conns := buildConnections(in)
		if len(conns) != 1 {
			t.Fatalf("connections = %d, want 1", len(conns))
		}
		return conns[0].State
	}

	// No planned interval known: a 4-minute-old poll on a 30s configured
	// cadence is past the floor and correctly reads stale.
	if state := pveState(base); state != ConnectionStateStale {
		t.Fatalf("state without plan = %q, want %q", state, ConnectionStateStale)
	}

	// The scheduler planned a 5-minute cadence, so 4 minutes old is on
	// schedule and must stay active.
	onSchedule := base
	onSchedule.plannedPollIntervals = map[string]time.Duration{"pve::lab": 5 * time.Minute}
	if state := pveState(onSchedule); state != ConnectionStateActive {
		t.Fatalf("state with 5m plan = %q, want %q", state, ConnectionStateActive)
	}

	// A plan tighter than the configured cadence never tightens the cutoff:
	// a genuine outage still trips against the configured interval and floor.
	tightPlan := base
	tightPlan.plannedPollIntervals = map[string]time.Duration{"pve::lab": 5 * time.Second}
	if state := pveState(tightPlan); state != ConnectionStateStale {
		t.Fatalf("state with 5s plan = %q, want %q", state, ConnectionStateStale)
	}
}
