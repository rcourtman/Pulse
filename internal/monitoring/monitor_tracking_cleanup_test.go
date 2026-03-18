package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pmg"
)

func TestCleanupTrackingMaps_PreservesActiveTypedCircuitBreakers(t *testing.T) {
	now := time.Now()
	stale := now.Add(-25 * time.Hour)

	activeKey := schedulerKey(InstanceTypePVE, "pve-active")
	inactiveKey := schedulerKey(InstanceTypePVE, "pve-removed")

	activeBreaker := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)
	inactiveBreaker := newCircuitBreaker(3, time.Second, time.Minute, 30*time.Second)

	activeBreaker.lastTransition = stale
	inactiveBreaker.lastTransition = stale

	m := &Monitor{
		pveClients: map[string]PVEClientInterface{
			"pve-active": nil,
		},
		pbsClients:      make(map[string]*pbs.Client),
		pmgClients:      make(map[string]*pmg.Client),
		circuitBreakers: map[string]*circuitBreaker{activeKey: activeBreaker, inactiveKey: inactiveBreaker},
		failureCounts:   map[string]int{activeKey: 2, inactiveKey: 3},
		lastOutcome: map[string]taskOutcome{
			activeKey:   {recordedAt: stale},
			inactiveKey: {recordedAt: stale},
		},
	}

	m.cleanupTrackingMaps(now)

	if _, ok := m.circuitBreakers[activeKey]; !ok {
		t.Fatalf("active circuit breaker %q was incorrectly removed", activeKey)
	}
	if _, ok := m.failureCounts[activeKey]; !ok {
		t.Fatalf("active failure count %q was incorrectly removed", activeKey)
	}
	if _, ok := m.lastOutcome[activeKey]; !ok {
		t.Fatalf("active lastOutcome %q was incorrectly removed", activeKey)
	}

	if _, ok := m.circuitBreakers[inactiveKey]; ok {
		t.Fatalf("inactive stale circuit breaker %q was not removed", inactiveKey)
	}
	if _, ok := m.failureCounts[inactiveKey]; ok {
		t.Fatalf("inactive stale failure count %q was not removed", inactiveKey)
	}
	if _, ok := m.lastOutcome[inactiveKey]; ok {
		t.Fatalf("inactive stale lastOutcome %q was not removed", inactiveKey)
	}
}
