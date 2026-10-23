package truenas

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

func TestRealtimeSCALEAggregateCPU(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		available    bool
		percent      float64
	}{
		{"reported-idle", `{"cpu":{"cpu":{"usage":0,"temp":36}},"memory":{"physical_memory_total":67374374912,"physical_memory_available":4122996736}}`, true, 0},
		{"reported-active", `{"cpu":{"cpu":{"usage":1,"temp":45}}}`, true, 1},
		{"fractional", `{"cpu":{"cpu":{"usage":12.5}}}`, true, 12.5},
		{"full", `{"cpu":{"cpu":{"usage":100}}}`, true, 100},
		{"not-a-core-average", `{"cpu":{"cpu":{"usage":1},"cpu0":{"usage":99},"cpu31":{"usage":75}}}`, true, 1},
		{"no-cpu", `{}`, false, 0},
		{"cores-only", `{"cpu":{"cpu0":{"usage":99},"cpu31":{"usage":75}}}`, false, 0},
		{"temperature-only", `{"cpu":{"cpu":{"temp":36}}}`, false, 0},
		{"null-aggregate", `{"cpu":{"cpu":null,"cpu0":{"usage":99}}}`, false, 0},
		{"null-usage", `{"cpu":{"cpu":{"usage":null,"temp":36}}}`, false, 0},
		{"invalid-usage", `{"cpu":{"cpu":{"usage":"unavailable"},"cpu0":{"usage":99}}}`, false, 0},
		{"boolean-usage", `{"cpu":{"cpu":{"usage":false}}}`, false, 0},
		{"flat-compatibility", `{"cpu":{"usage":12.5}}`, true, 12.5},
		{"nested-usage-compatibility", `{"cpu":{"usage":{"percent":12.5}}}`, true, 12.5},
		{"nested-total-compatibility", `{"cpu":{"total":{"usage":12.5}}}`, true, 12.5},
		{"flat-zero-precedence", `{"cpu":{"usage":0,"cpu":{"usage":75}}}`, true, 0},
		{"nested-zero-precedence", `{"cpu":{"usage":{"percent":0},"cpu":{"usage":75}}}`, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var fields map[string]any
			if err := json.Unmarshal([]byte(tc.fields), &fields); err != nil {
				t.Fatal(err)
			}
			telemetry := parseSystemTelemetry(fields, 2, time.Unix(1792796400, 0).UTC())
			if telemetry.Telemetry.CPU != tc.available || telemetry.CPUPercent != tc.percent {
				t.Fatalf("CPU available=%v percent=%v, want %v/%v", telemetry.Telemetry.CPU, telemetry.CPUPercent, tc.available, tc.percent)
			}
		})
	}
}

// Use the real TLS websocket subscription, not a direct parser invocation.
// Samples reuse one ordinary RPC session and acknowledge every unsubscribe.
func scaleAggregateTelemetryClient(t *testing.T, samples []string) *Client {
	t.Helper()
	var subscribes, unsubscribes atomic.Int32
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		switch request.Method {
		case "auth.login_ex":
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS"}}
		case "reporting.get_data":
			return protocolFixtureReply{result: []any{}}
		case "core.subscribe":
			collection := request.Params.([]any)[0].(string)
			if !strings.HasPrefix(collection, "reporting.realtime:") {
				t.Errorf("unexpected subscription %s", collection)
				return protocolFixtureReply{close: true}
			}
			index := int(subscribes.Add(1)) - 1
			if index >= len(samples) {
				t.Error("unexpected repeated telemetry read")
				return protocolFixtureReply{close: true}
			}
			var fields map[string]any
			if err := json.Unmarshal([]byte(samples[index]), &fields); err != nil {
				t.Error(err)
				return protocolFixtureReply{close: true}
			}
			return protocolFixtureReply{result: "fixture-realtime", notifications: []protocolFixtureNotification{{
				method: "collection_update", params: map[string]any{"collection": collection, "fields": fields},
			}}}
		case "core.unsubscribe":
			unsubscribes.Add(1)
			return protocolFixtureReply{result: nil}
		default:
			t.Errorf("unexpected RPC method %s", request.Method)
			return protocolFixtureReply{close: true}
		}
	}, nil)
	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "readonly-fixture-key", Username: "readonly-fixture", Timeout: 5 * time.Second})
	t.Cleanup(func() {
		if fixture.restRequests.Load() != 0 || fixture.sessions.Load() != 1 || int(subscribes.Load()) != len(samples) || subscribes.Load() != unsubscribes.Load() {
			t.Errorf("sessions=%d REST=%d subscribe/unsubscribe=%d/%d, want 1/0/%d/%d", fixture.sessions.Load(), fixture.restRequests.Load(), subscribes.Load(), unsubscribes.Load(), len(samples), len(samples))
		}
	})
	return client
}

func scaleAggregateResource(t *testing.T, telemetry *SystemInfo) unifiedresources.Resource {
	t.Helper()
	system := systemInfoFromResponse(systemInfoResponse{Hostname: "scale-cpu-fixture", Version: "TrueNAS-SCALE-25.10.7", SystemSerial: "synthetic-3158", Cores: 32, Physmem: 67374374912})
	mergeSystemTelemetry(system, telemetry)
	snapshot := FixtureSnapshot{System: *system, CollectedAt: telemetry.CollectedAt}
	provider := NewProvider(snapshot)
	records := provider.RecordsFromSnapshot(&snapshot)
	registry := unifiedresources.NewRegistry(unifiedresources.NewMemoryStore())
	registry.IngestRecords(unifiedresources.SourceTrueNAS, records)
	resources := registry.List()
	if len(resources) != 1 || resources[0].Type != unifiedresources.ResourceTypeAgent {
		t.Fatalf("system resource projection lost identity: %+v", resources)
	}
	return resources[0]
}

func TestJSONRPCSCALEAggregateCPUProjection(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })
	// Only the first aggregate/memory fragment and second aggregate values
	// come from #3158. Other frames and per-core conflicts are synthetic controls.
	samples := []string{
		`{"cpu":{"cpu":{"usage":0,"temp":36}},"memory":{"physical_memory_total":67374374912,"physical_memory_available":4122996736}}`,
		`{"cpu":{"cpu":{"usage":1,"temp":45},"cpu0":{"usage":99}},"memory":{"physical_memory_total":67374374912,"physical_memory_available":4122996736}}`,
		`{"cpu":{"cpu":{"temp":36},"cpu0":{"usage":99}},"memory":{"physical_memory_total":67374374912,"physical_memory_available":4122996736}}`,
		`{"cpu":{"cpu":{"usage":null},"cpu0":{"usage":99}},"memory":{"physical_memory_total":67374374912,"physical_memory_available":4122996736}}`,
		`{"cpu":{"cpu":{"usage":0}},"memory":{"physical_memory_total":67374374912,"physical_memory_available":4122996736}}`,
	}
	client := scaleAggregateTelemetryClient(t, samples)
	for index, want := range []struct {
		available bool
		percent   float64
	}{{true, 0}, {true, 1}, {false, 0}, {false, 0}, {true, 0}} {
		telemetry, err := client.GetSystemTelemetry(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		resource := scaleAggregateResource(t, telemetry)
		if resource.Metrics == nil || (resource.Metrics.CPU != nil) != want.available || resource.Metrics.Memory == nil {
			t.Fatalf("frame %d invented or lost CPU/memory: %+v", index, resource.Metrics)
		}
		if cpu := resource.Metrics.CPU; cpu != nil && (cpu.Value != want.percent || cpu.Percent != want.percent || cpu.Source != unifiedresources.SourceTrueNAS) {
			t.Fatalf("frame %d lost the actual aggregate: %+v", index, cpu)
		}
		body, err := json.Marshal(resource)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			Metrics struct {
				CPU *struct {
					Value float64 `json:"value"`
				} `json:"cpu"`
			} `json:"metrics"`
		}
		if err := json.Unmarshal(body, &wire); err != nil || (wire.Metrics.CPU != nil) != want.available || wire.Metrics.CPU != nil && wire.Metrics.CPU.Value != want.percent {
			t.Fatalf("frame %d wire availability/value changed: %s, %v", index, body, err)
		}
	}
}
