package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
)

// Regression tests for the provider half of #1582: the scheduler already pins
// InstanceDescriptor.FixedInterval exactly (TestBuildPlan_FixedInterval), but
// the availability provider must actually populate that descriptor from the
// target's configured interval, or the adaptive scheduler takes over again.

func availabilityFixedIntervalTestMonitor(t *testing.T, targets []config.AvailabilityTarget) *Monitor {
	t.Helper()
	persistence := config.NewConfigPersistence(t.TempDir())
	if err := persistence.SaveAvailabilityTargets(targets); err != nil {
		t.Fatalf("SaveAvailabilityTargets() error = %v", err)
	}
	return &Monitor{configPersist: persistence}
}

func TestAvailabilityPollProviderFixedInstanceInterval(t *testing.T) {
	monitor := availabilityFixedIntervalTestMonitor(t, []config.AvailabilityTarget{
		{ID: "plex", Name: "Plex", Address: "plex.local", Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 120},
		{ID: "defaulted", Name: "Defaulted", Address: "defaulted.local", Protocol: config.AvailabilityProbeICMP, Enabled: true},
		{ID: "oversized", Name: "Oversized", Address: "oversized.local", Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 7200},
		{ID: "paused", Name: "Paused", Address: "paused.local", Protocol: config.AvailabilityProbeICMP, Enabled: false, PollIntervalSecs: 120},
	})
	provider := availabilityPollProvider{}

	if got := provider.FixedInstanceInterval(monitor, "plex"); got != 120*time.Second {
		t.Fatalf("FixedInstanceInterval(plex) = %v, want 120s", got)
	}
	wantDefault := time.Duration(config.DefaultAvailabilityPollIntervalSecs) * time.Second
	if got := provider.FixedInstanceInterval(monitor, "defaulted"); got != wantDefault {
		t.Fatalf("FixedInstanceInterval(defaulted) = %v, want %v", got, wantDefault)
	}
	if got := provider.FixedInstanceInterval(monitor, "oversized"); got != time.Hour {
		t.Fatalf("FixedInstanceInterval(oversized) = %v, want clamp to 1h", got)
	}
	if got := provider.FixedInstanceInterval(monitor, "paused"); got != 0 {
		t.Fatalf("FixedInstanceInterval(paused) = %v, want 0 for disabled target", got)
	}
	if got := provider.FixedInstanceInterval(monitor, "missing"); got != 0 {
		t.Fatalf("FixedInstanceInterval(missing) = %v, want 0 for unknown target", got)
	}
}

// End-to-end wiring: the scheduler descriptor for an availability target must
// carry the configured interval as FixedInterval.
func TestDescribeInstancesForSchedulerPinsAvailabilityFixedInterval(t *testing.T) {
	monitor := availabilityFixedIntervalTestMonitor(t, []config.AvailabilityTarget{
		{ID: "plex", Name: "Plex", Address: "plex.local", Protocol: config.AvailabilityProbeICMP, Enabled: true, PollIntervalSecs: 120},
	})

	descriptors := monitor.describeInstancesForScheduler()
	var found *InstanceDescriptor
	for i := range descriptors {
		if descriptors[i].Type == InstanceTypeAvailability && descriptors[i].Name == "plex" {
			found = &descriptors[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no availability descriptor for plex in %+v", descriptors)
	}
	if found.FixedInterval != 120*time.Second {
		t.Fatalf("descriptor FixedInterval = %v, want 120s", found.FixedInterval)
	}
}
