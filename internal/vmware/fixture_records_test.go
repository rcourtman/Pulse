package vmware

import "testing"

func TestFixtureRecordsIgnoreFeatureFlag(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(false)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	snapshot := DefaultFixtures()
	records := FixtureRecords(snapshot)
	if len(records) == 0 {
		t.Fatal("expected fixture records even when feature flag is disabled")
	}

	hostID := vmwareSourceID(snapshot.ConnectionID, "host", snapshot.Hosts[0].Host)
	for _, record := range records {
		if record.SourceID == hostID {
			return
		}
	}

	t.Fatalf("expected fixture records to include host source %q", hostID)
}
