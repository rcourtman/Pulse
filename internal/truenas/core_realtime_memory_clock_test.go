package truenas

import (
	"context"
	"testing"
)

func TestCORE13MemoryFixtureUsesRequestedReportingEnd(t *testing.T) {
	scenario := &coreMemoryScenario{version: "TrueNAS-SCALE-24.10.2", restARC: 8 << 30}
	client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key"})
	// Fixed query windows expose a responder clock outside the request without
	// relying on a sleep or a lucky wall-clock second rollover. A second query
	// also prevents a fixture from reusing the first request's timestamp.
	for _, end := range []int64{1789000060, 1789000120} {
		responses, err := client.getLegacySystemReportingData(context.Background(), reportingRangeQuery(end-60, end))
		if err != nil {
			t.Fatal(err)
		}
		for _, response := range responses {
			if len(response.Data) > 0 && (response.Start != end || response.End != end) {
				t.Errorf("%s fixture range [%d, %d] does not match requested sample time %d", response.Name, response.Start, response.End, end)
			}
		}
		system := systemInfoFromMetricHistory(parseSystemMetricHistory(liveReportingResponses(responses, end)))
		if system == nil {
			t.Error("fixture samples were discarded outside the requested live window")
			continue
		}
		if system.CollectedAt.Unix() != end || system.CPUPercent != 12 || system.MemoryAvailableBytes != 2178945024 || system.ARCSizeBytes != 8<<30 || !system.Telemetry.CPU || !system.Telemetry.Memory {
			t.Errorf("request-bound fixture lost reporting readings: %+v", system)
		}
	}
	if scenario.sockets.Load() != 0 {
		t.Fatal("reporting fixture used the native DDP memory path")
	}
}
