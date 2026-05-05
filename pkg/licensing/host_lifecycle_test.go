package licensing

import (
	"os"
	"strings"
	"testing"
)

func TestHostLifecycleCapacityTrackerStaysRetired(t *testing.T) {
	source, err := os.ReadFile("host_lifecycle.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	for _, retired := range []string{
		"HostLifecycleTracker",
		"HostStabilizationPeriod",
		"HostInactivityTimeout",
		"StableActiveHosts",
		"RecordHeartbeat",
		"monitored-system capacity",
	} {
		if strings.Contains(string(source), retired) {
			t.Fatalf("host lifecycle capacity tracker must stay retired; found %q", retired)
		}
	}
}
