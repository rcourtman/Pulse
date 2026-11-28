package monitoring

import (
	"errors"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/stretchr/testify/require"
)

func TestSchedulerHealthReadsAvailabilityRosterOnce(t *testing.T) {
	local := probeAgentTarget("local", "")
	local.Name = "Local router"
	remote := probeAgentTarget("remote", "edge")
	remote.Name = "Remote branch"
	paused := probeAgentTarget("paused", "")
	paused.Enabled = false
	monitor, writer, catalog := newAvailabilityPlanningFixture(t, []config.AvailabilityTarget{local, remote, paused})
	monitor.SetLicenseChecker(licenseWithExternalProbe(true))

	// A provider that supplies metadata for only part of its roster must
	// still get its other instances from ListInstances.
	require.NoError(t, monitor.RegisterPollProvider(testPollProvider{
		providerType: "fixture",
		instances:    []string{"detailed", "list-only"},
		describeInstances: []PollProviderInstanceInfo{
			{Name: "detailed", DisplayName: "Detailed peer", Connection: "fixture-detailed"},
		},
	}))

	check := func(phase string, targets ...config.AvailabilityTarget) {
		t.Helper()
		before := catalog.reads
		entries := make(map[string]InstanceHealth)
		for _, entry := range monitor.SchedulerHealth().Instances {
			entries[entry.Key] = entry
		}
		reads := catalog.reads - before
		t.Logf("%s: saved availability catalog reads=%d, total health instances=%d", phase, reads, len(entries))
		if reads != 1 {
			t.Errorf("%s: read saved availability catalog %d times, want one", phase, reads)
		}
		for _, target := range targets {
			row, ok := entries["availability::"+target.ID]
			require.True(t, ok, "%s: missing health instance %s", phase, target.ID)
			require.Equal(t, target.DisplayName(), row.DisplayName)
			require.Equal(t, availabilityConnectionKey(target.ID), row.Connection)
			delete(entries, row.Key)
		}
		require.Len(t, entries, 4, "%s: only the non-Availability peers should remain", phase)
		require.Contains(t, entries, "pve::pve-peer")
		require.Contains(t, entries, "pbs::pbs-peer")
		require.Equal(t, "Detailed peer", entries["fixture::detailed"].DisplayName)
		require.Equal(t, "fixture-detailed", entries["fixture::detailed"].Connection)
		require.Equal(t, "list-only", entries["fixture::list-only"].DisplayName)
	}

	check("initial", local, remote)
	remote.Name = "Renamed remote branch"
	require.NoError(t, writer.SaveAvailabilityTargets([]config.AvailabilityTarget{remote, paused}))
	check("edited", remote)
	catalog.err = errors.New("fixture: catalog read unavailable")
	check("unreadable")
}
