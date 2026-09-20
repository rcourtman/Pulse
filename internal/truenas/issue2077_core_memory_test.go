package truenas

import (
	"context"
	"testing"
	"time"
)

// #2077: on the legacy REST transport (TrueNAS CORE 13) there is no
// reporting.realtime JSON-RPC subscription, so the available-memory reading is
// never populated and the ARC-aware used figure collapses to total (100% used).
// These tests pin the REST reporting fallback and the "usage unavailable"
// guard that replaced the fabricated reading.

func legacyRESTReportingMemoryBody() string {
	return `[
		{
			"name": "memory",
			"identifier": null,
			"legend": ["used", "free", "available", "total"],
			"data": [
				{"timestamp": 1789000000, "used": 12000000000, "free": 2000000000, "available": 20000000000, "total": 34359738368},
				{"timestamp": 1789000060, "used": 13000000000, "free": 3000000000, "available": 21000000000, "total": 34359738368}
			],
			"start": 1789000000,
			"end": 1789000060
		},
		{
			"name": "arcsize",
			"identifier": null,
			"legend": ["arc_size"],
			"data": [
				{"timestamp": 1789000060, "arc_size": 8000000000}
			],
			"start": 1789000000,
			"end": 1789000060
		}
	]`
}

func newLegacyRESTReportingClient(t *testing.T, routes alertArgsTransport) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{Host: "http://truenas.invalid", APIKey: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Transport = routes
	client.mode = TransportLegacyREST
	return client
}

func TestRESTSystemTelemetryReadsReportingMemory(t *testing.T) {
	routes := alertArgsTransport(defaultAPIResponses())
	routes["/api/v2.0/system/info"] = apiResponse{body: `{"hostname":"core","version":"TrueNAS-13.0-U6.1","physmem":34359738368}`}
	routes["/api/v2.0/reporting/get_data"] = apiResponse{body: legacyRESTReportingMemoryBody()}

	client := newLegacyRESTReportingClient(t, routes)
	telemetry, err := client.GetSystemTelemetry(context.Background())
	if err != nil {
		t.Fatalf("GetSystemTelemetry() error = %v", err)
	}
	if telemetry == nil {
		t.Fatal("expected telemetry")
	}
	if telemetry.MemoryTotalBytes != 34359738368 {
		t.Fatalf("MemoryTotalBytes = %d, want 34359738368", telemetry.MemoryTotalBytes)
	}
	if telemetry.MemoryAvailableBytes != 21000000000 {
		t.Fatalf("MemoryAvailableBytes = %d, want latest sample 21000000000", telemetry.MemoryAvailableBytes)
	}
	if telemetry.ARCSizeBytes != 8000000000 {
		t.Fatalf("ARCSizeBytes = %d, want 8000000000", telemetry.ARCSizeBytes)
	}

	metrics := metricsFromTrueNASSystem(*telemetry, 0, 0)
	if metrics.Memory == nil || metrics.Memory.Used == nil {
		t.Fatalf("expected derived memory usage, got %+v", metrics.Memory)
	}
	wantUsed := int64(34359738368 - 21000000000 - 8000000000)
	if *metrics.Memory.Used != wantUsed {
		t.Fatalf("used = %d, want %d", *metrics.Memory.Used, wantUsed)
	}
	if metrics.Memory.Percent >= 99 {
		t.Fatalf("memory percent = %v, want a real reading below 99", metrics.Memory.Percent)
	}

	agent := agentDataFromTrueNASSystem("conn-core", *telemetry, nil, nil, false, "", false, "")
	if agent.Memory == nil || agent.Memory.UsageUnavailable {
		t.Fatalf("expected known agent memory, got %+v", agent.Memory)
	}
	if agent.Memory.Used != wantUsed || agent.Memory.Cache != 8000000000 {
		t.Fatalf("unexpected agent memory: %+v", agent.Memory)
	}
}

func TestRESTSystemMetricHistoryUsesReporting(t *testing.T) {
	routes := alertArgsTransport(defaultAPIResponses())
	routes["/api/v2.0/reporting/get_data"] = apiResponse{body: legacyRESTReportingMemoryBody()}

	client := newLegacyRESTReportingClient(t, routes)
	history, err := client.GetSystemMetricHistory(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("GetSystemMetricHistory() error = %v", err)
	}
	if history == nil {
		t.Fatal("expected history")
	}
	if len(history.MemoryAvailableBytes) != 2 {
		t.Fatalf("MemoryAvailableBytes points = %d, want 2", len(history.MemoryAvailableBytes))
	}
	if len(history.ARCSizeBytes) != 1 {
		t.Fatalf("ARCSizeBytes points = %d, want 1", len(history.ARCSizeBytes))
	}
}

func TestTrueNASMemoryWithoutAvailableIsNotReportedAsUsed(t *testing.T) {
	system := SystemInfo{Hostname: "core", MemoryTotalBytes: 34359738368}

	metrics := metricsFromTrueNASSystem(system, 0, 0)
	if metrics.Memory != nil {
		t.Fatalf("fabricated memory metric without an available reading: %+v", metrics.Memory)
	}

	agent := agentDataFromTrueNASSystem("conn-core", system, nil, nil, false, "", false, "")
	if agent.Memory == nil {
		t.Fatal("expected agent memory capacity")
	}
	if !agent.Memory.UsageUnavailable {
		t.Fatalf("UsageUnavailable = false, want true: %+v", agent.Memory)
	}
	if agent.Memory.Total != 34359738368 || agent.Memory.Used != 0 {
		t.Fatalf("unexpected agent memory: %+v", agent.Memory)
	}
}
