package truenas

import "testing"

func TestFixtureRecordsIgnoreFeatureFlag(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(false)
	t.Cleanup(func() { SetFeatureEnabled(previous) })

	records := FixtureRecords(DefaultFixtures())
	if len(records) == 0 {
		t.Fatal("expected fixture records even when feature flag is disabled")
	}

	systemID := systemSourceID("truenas-main")
	for _, record := range records {
		if record.SourceID == systemID {
			return
		}
	}

	t.Fatalf("expected fixture records to include system source %q", systemID)
}
