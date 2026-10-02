package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Five native result excerpts supplied in #2077 on 1 October 2026. Identifiers
// are anonymised and the SHORT EXCERPT windows made coherent. This is decoder
// evidence, not a complete time window, REST capture or appliance acceptance.
func core13ReportingFixture(t *testing.T) []trueNASReportingGetDataResponse {
	t.Helper()
	raw, err := os.ReadFile("testdata/core13_reporting.json")
	if err != nil {
		t.Fatal(err)
	}
	var replies []trueNASReportingGetDataResponse
	if err := json.Unmarshal(raw, &replies); err != nil {
		t.Fatal(err)
	}
	return replies
}

func assertCOREPoint(t *testing.T, points []TimeSeriesPoint, index int, timestamp int64, value float64) {
	t.Helper()
	if len(points) <= index || points[index].Timestamp.Unix() != timestamp || math.Abs(points[index].Value-value) > 0.000001 {
		t.Fatalf("point %d: %+v, want %d / %v", index, points, timestamp, value)
	}
}

// These five subtests also compile with the predecessor decoder and are the
// predetermined adverse controls; do not substitute compiler failure for them.
func TestCORE13NativeResultControls(t *testing.T) {
	replies := core13ReportingFixture(t)
	start := int64(1789000000)
	t.Run("CPU", func(t *testing.T) {
		history := parseSystemMetricHistory(replies[:1])
		if history == nil {
			t.Fatal("native CPU rows produced no history")
		}
		want := (0.033358538468 + 4.4805217137 + 7.8754519386 + 7.8754519386) / (100 + 0.033358538468 + 4.4805217137 + 7.8754519386 + 7.8754519386) * 100
		assertCOREPoint(t, history.CPUPercent, 0, start, want)
		if len(history.CPUPercent) != 1 {
			t.Fatal("all-zero state buckets became CPU measurements")
		}
	})
	t.Run("memory", func(t *testing.T) {
		history := parseSystemMetricHistory(replies[2:3])
		if history == nil {
			t.Fatal("native memory aliases produced no history")
		}
		assertCOREPoint(t, history.MemoryAvailableBytes, 0, start, 2178945024)
		if len(history.MemoryAvailableBytes) != 1 || len(history.MemoryUsedBytes) != 0 {
			t.Fatal("empty bucket or active pages became total usage")
		}
	})
	t.Run("disk", func(t *testing.T) {
		history := parseSystemMetricHistory(replies[3:4])
		if history == nil {
			t.Fatal("native disk rows produced no history")
		}
		assertCOREPoint(t, history.DiskReadRate, 0, start, 484.54070578)
		assertCOREPoint(t, history.DiskReadRate, 1, start+10, 0)
		assertCOREPoint(t, history.DiskWriteRate, 1, start+10, 1276361.9904)
		if len(history.DiskReadRate) != 2 {
			t.Fatal("null disk bucket became zero")
		}
	})
	t.Run("network", func(t *testing.T) {
		history := parseSystemMetricHistory(replies[4:5])
		if history == nil {
			t.Fatal("native network rows produced no history")
		}
		assertCOREPoint(t, history.NetInRate, 0, start, 9728.2270494)
		assertCOREPoint(t, history.NetOutRate, 0, start, 45551.708899)
		if len(history.NetInRate) != 1 {
			t.Fatal("null or overlap became RX")
		}
	})
	t.Run("temperature request", func(t *testing.T) {
		found := false
		for _, graph := range legacyRESTReportingGraphs() {
			found = found || graph["name"] == "cputemp"
		}
		if !found {
			t.Fatal("legacy graph set never requests CPU temperature")
		}
	})
}

func TestCORE13TemperatureUsesSamplesNotMeans(t *testing.T) {
	replies := core13ReportingFixture(t)[1:2]
	replies[0].Aggregations.Mean = []any{999., 999., 999., 999., 999., 999., 999., 999.}
	temperatures := parseSystemTemperatures(replies)
	if len(temperatures) != 8 || temperatures["cputemp2"] != 30.95 {
		t.Fatalf("temperature samples lost: %+v", temperatures)
	}
	history := parseSystemMetricHistory(replies)
	if history == nil {
		t.Fatal("temperature-only history lost")
	}
	system := systemInfoFromMetricHistory(history)
	if got := maxTrueNASSystemTemperature(*system); got == nil || *got != 30.95 {
		t.Fatalf("current temperature = %v", got)
	}
	points := systemTemperatureHistory(history.TemperatureCelsius)
	assertCOREPoint(t, points, 0, 1789000000, 30.95)
	replies[0].Data[0] = []any{nil, nil, nil, nil, nil, nil, nil, nil}
	if got := parseSystemTemperatures(replies); len(got) != 0 {
		t.Fatalf("all-null rows substituted a mean: %v", got)
	}
}

func TestCORE13TimingAndMissingValueBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		count      int
		last       int64
	}{
		{"external zero", `{"start":1789000000,"end":1789000020,"step":10,"legend":["usage"],"data":[[0],[null],[12]]}`, 2, 1789000020},
		{"missing step", `{"start":1789000000,"end":1789000020,"legend":["usage"],"data":[[42]]}`, 0, 0},
		{"zero step", `{"start":1789000000,"end":1789000020,"step":0,"legend":["usage"],"data":[[42]]}`, 0, 0},
		{"negative step", `{"start":1789000000,"end":1789000020,"step":-10,"legend":["usage"],"data":[[42]]}`, 0, 0},
		{"missing start", `{"end":1789000020,"step":10,"legend":["usage"],"data":[[42]]}`, 0, 0},
		{"reversed range", `{"start":1789000020,"end":1789000000,"step":10,"legend":["usage"],"data":[[42]]}`, 0, 0},
		{"past end", `{"start":1789000000,"end":1789000000,"step":10,"legend":["usage"],"data":[[42],[43]]}`, 1, 1789000000},
		{"overflow", `{"start":9223372036854775797,"end":9223372036854775807,"step":9223372036854775807,"legend":["usage"],"data":[[42],[43]]}`, 1, 9223372036854775797},
		{"timestamped", `{"legend":["usage"],"data":[[1789000000,0],[1789000010000,12]]}`, 2, 1789000010},
		{"object", `{"legend":["usage"],"data":[{"timestamp":"2026-09-10T01:46:40Z","usage":0}]}`, 1, 1789004800},
		{"short row", `{"legend":["rx","tx","overlap"],"data":[[1789000000,12]]}`, 0, 0},
		{"nonfinite", `{"start":1789000000,"end":1789000000,"step":10,"legend":["usage"],"data":[["NaN"],["+Inf"]]}`, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var response trueNASReportingGetDataResponse
			if err := json.Unmarshal([]byte(tc.body), &response); err != nil {
				t.Fatal(err)
			}
			response.Name = "cpu"
			history := parseSystemMetricHistory([]trueNASReportingGetDataResponse{response})
			var points []TimeSeriesPoint
			if history != nil {
				points = history.CPUPercent
			}
			if len(points) != tc.count || (tc.count > 0 && points[len(points)-1].Timestamp.Unix() != tc.last) {
				t.Fatalf("invalid timing/value handling: %+v", points)
			}
		})
	}
}

func TestCORE13CPUAndMemoryEmptyBuckets(t *testing.T) {
	for _, tc := range []struct {
		row   []any
		valid bool
		want  float64
	}{
		{[]any{0., 0., 0., 0., 100.}, true, 0}, {[]any{0., 0., 0., 0., 0.}, false, 0},
		{[]any{nil, 0., 0., 0., 100.}, false, 0}, {[]any{-1., 0., 0., 0., 100.}, false, 0},
		{[]any{10., 10., 10., 10., 160.}, true, 20},
	} {
		response := trueNASReportingGetDataResponse{Name: "cpu", Legend: coreCPUStates, Start: 1789000000, End: 1789000000, Step: 10, Data: []any{tc.row}}
		history := parseSystemMetricHistory([]trueNASReportingGetDataResponse{response})
		if (history != nil) != tc.valid {
			t.Fatalf("CPU row %v became %+v", tc.row, history)
		}
		if tc.valid {
			assertCOREPoint(t, history.CPUPercent, 0, response.Start, tc.want)
		}
	}
	for _, tc := range []struct {
		row   []any
		valid bool
	}{
		{[]any{0., 0., 0., 16., 0.}, true}, {[]any{0., 0., 0., 0., 0.}, false}, {[]any{1., 1., 1., nil, 0.}, false},
	} {
		response := trueNASReportingGetDataResponse{Name: "memory", Legend: coreMemoryClasses, Start: 1789000000, End: 1789000000, Step: 10, Data: []any{tc.row}}
		history := parseSystemMetricHistory([]trueNASReportingGetDataResponse{response})
		if (history != nil) != tc.valid {
			t.Fatalf("memory row %v became %+v", tc.row, history)
		}
		if tc.valid {
			assertCOREPoint(t, history.MemoryAvailableBytes, 0, response.Start, 0)
		}
	}
}

func TestCORE13MultipleDevicesAndFreshness(t *testing.T) {
	base := trueNASReportingGetDataResponse{Name: "interface", Identifier: "nic-a", Legend: []string{"rx", "tx", "overlap"}, Start: 1789000000, End: 1789000020, Step: 10, Data: []any{[]any{10., 20., 10.}, []any{0., 5., 0.}, []any{nil, nil, nil}}}
	second := base
	second.Identifier = "nic-b"
	second.Data = []any{[]any{100., 200., 100.}, []any{0., nil, 0.}, []any{4., 8., 4.}}
	for _, replies := range [][]trueNASReportingGetDataResponse{{base, second, base}, {second, base, base}} {
		history := parseSystemMetricHistory(replies)
		assertCOREPoint(t, history.NetInRate, 0, base.Start, 110)
		assertCOREPoint(t, history.NetInRate, 1, base.Start+10, 0)
		assertCOREPoint(t, history.NetOutRate, 0, base.Start, 220)
		if len(history.NetInRate) != 2 || len(history.NetOutRate) != 1 {
			t.Fatalf("missing device reading treated as zero: %+v", history)
		}
	}
	if got := parseSystemMetricHistory(liveReportingResponses([]trueNASReportingGetDataResponse{base}, base.End+31)); got != nil {
		t.Fatalf("stale sample became current: %+v", got)
	}
	fresh := parseSystemMetricHistory(liveReportingResponses([]trueNASReportingGetDataResponse{base}, base.End))
	assertCOREPoint(t, fresh.NetInRate, 1, base.Start+10, 0)
	if at := systemInfoFromMetricHistory(fresh).CollectedAt.Unix(); at != base.Start+10 {
		t.Fatalf("sample falsely stamped as current: %d", at)
	}
	if !reflect.DeepEqual(base.Data[0], []any{10., 20., 10.}) {
		t.Fatal("live filtering mutated native History")
	}
}

// A request-validating REST bridge models catalogue -> per-device requests ->
// reporting rows -> complete snapshot and canonical host/history. Only the five
// excerpts are native evidence; catalogue, ARC, HTTP and timing are controls.
type core13ReportingTransport struct {
	routes                         alertArgsTransport
	replies                        []trueNASReportingGetDataResponse
	requested                      []map[string]any
	catalogueCalls, reportingCalls int
}

func (s *core13ReportingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/api/v2.0/reporting/graphs" {
		s.catalogueCalls++
		return (alertArgsTransport{r.URL.Path: {body: `[{"name":"disk","identifiers":["disk-a","disk-a",""]},{"name":"interface","identifiers":["nic-a"]}]`}}).RoundTrip(r)
	}
	if r.URL.Path != "/api/v2.0/reporting/get_data" {
		return s.routes.RoundTrip(r)
	}
	s.reportingCalls++
	var request struct {
		Graphs []map[string]any `json:"graphs"`
		Query  map[string]any   `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return nil, err
	}
	s.requested = append(s.requested, request.Graphs...)
	var responses []trueNASReportingGetDataResponse
	for _, graph := range request.Graphs {
		name := readStringAny(graph, "name")
		identifier := readStringAny(graph, "identifier")
		if (name == "disk" && identifier != "disk-a") || (name == "interface" && identifier != "nic-a") || (name != "disk" && name != "interface" && graph["identifier"] != nil) {
			return (alertArgsTransport{r.URL.Path: {status: 422, body: `{"error":"native graph needs the correct device identifier"}`}}).RoundTrip(r)
		}
		for _, reply := range s.replies {
			if reply.Name != name {
				continue
			}
			reply.End = int64(request.Query["end"].(float64))
			reply.Start = reply.End - int64(len(reply.Data)-1)*reply.Step
			responses = append(responses, reply)
		}
		if name == "arcsize" {
			end := int64(request.Query["end"].(float64))
			responses = append(responses, trueNASReportingGetDataResponse{Name: name, Start: end - 10, End: end, Step: 10, Legend: []string{"arcsize_value"}, Data: []any{[]any{float64(8 << 30)}, []any{nil}}})
		}
	}
	raw, err := json.Marshal(responses)
	if err != nil {
		return nil, err
	}
	return (alertArgsTransport{r.URL.Path: {body: string(raw)}}).RoundTrip(r)
}

func TestCORE13SnapshotAndCanonicalHistoryJourney(t *testing.T) {
	previous := IsFeatureEnabled()
	SetFeatureEnabled(true)
	t.Cleanup(func() { SetFeatureEnabled(previous) })
	routes := alertArgsTransport(defaultAPIResponses())
	routes["/api/v2.0/system/info"] = apiResponse{body: `{"hostname":"core-fixture","version":"TrueNAS-13.0-U6.1","physmem":34359738368,"cores":8}`}
	transport := &core13ReportingTransport{routes: routes, replies: core13ReportingFixture(t)}
	client := newLegacyRESTReportingClient(t, routes)
	client.httpClient.Transport = transport
	t.Cleanup(client.Close)
	provider := NewLiveProviderForConnection(&APIFetcher{Client: client}, "core-fixture-connection")
	for cycle := 0; cycle < 2; cycle++ {
		if err := provider.Refresh(context.Background()); err != nil {
			t.Fatal(err)
		}
		snapshot := provider.Snapshot()
		if snapshot.System.CPUCount != 8 || snapshot.System.MemoryTotalBytes != 34359738368 || len(snapshot.Disks) != 2 || len(snapshot.Pools) != 1 {
			t.Fatal("native telemetry lost inventory/capacity")
		}
		var host *unifiedresources.Resource
		for _, record := range provider.Records() {
			if record.Resource.Type == unifiedresources.ResourceTypeAgent {
				copy := record.Resource
				host = &copy
			}
		}
		if host == nil || host.Metrics.CPU == nil || host.Metrics.Memory == nil || host.Temperature == nil || host.Agent.Memory == nil {
			t.Fatalf("collapsed/expanded row lost native readings: %+v", host)
		}
		wantUsed := int64(34359738368-2178945024) - int64(8<<30)
		if *host.Metrics.Memory.Used != wantUsed || host.Agent.Memory.Used != wantUsed || *host.Temperature != 30.95 || host.Agent.Memory.Cache != 8<<30 {
			t.Fatalf("ARC-aware current values wrong: %+v %+v", host.Metrics, host.Agent)
		}
		for key, metric := range map[string]*unifiedresources.MetricValue{"netin": host.Metrics.NetIn, "netout": host.Metrics.NetOut, "diskread": host.Metrics.DiskRead, "diskwrite": host.Metrics.DiskWrite} {
			want := map[string]float64{"netin": 9728.2270494, "netout": 45551.708899, "diskread": 0, "diskwrite": 1276361.9904}[key]
			if metric == nil || metric.Unit != "bytes/s" || math.Abs(metric.Value-want) > 0.000001 {
				t.Fatalf("%s current rate: %+v", key, metric)
			}
		}
		id, history, err := provider.SystemMetricHistory(context.Background(), time.Hour)
		if err != nil {
			t.Fatal(err)
		}
		if id != "core-fixture-connection" || len(history) != 7 {
			t.Fatalf("canonical History panels: %q %+v", id, history)
		}
		if len(history["cpu"]) != 1 || len(history["memory"]) != 1 || len(history["temperature"]) != 1 || len(history["diskread"]) != 2 {
			t.Fatal("empty buckets filled with means or zeros")
		}
		if math.Abs(history["memory"][0].Value-host.Metrics.Memory.Percent) > 0.000001 {
			t.Fatal("live/History disagree on free RAM/ARC")
		}
	}
	if transport.catalogueCalls != 4 || transport.reportingCalls != 4 {
		t.Fatalf("successful catalogue/batch unexpectedly retried: %+v", transport)
	}
	for _, graph := range transport.requested {
		if name := readStringAny(graph, "name"); (name == "disk" || name == "interface") && readStringAny(graph, "identifier") == "" {
			t.Fatal("device graph queried without identifier")
		}
	}
}

func TestCORE13CatalogueAndResponseSafety(t *testing.T) {
	for _, tc := range []struct {
		status   int
		body     string
		fallback bool
	}{
		{404, `{}`, true}, {405, `{}`, true}, {401, `{}`, false}, {403, `{}`, false}, {429, `{}`, false}, {500, `{}`, false}, {200, `{`, false},
	} {
		routes := alertArgsTransport{"/api/v2.0/reporting/graphs": {status: tc.status, body: tc.body}}
		client := newLegacyRESTReportingClient(t, routes)
		graphs, err := client.legacyReportingGraphs(context.Background())
		if tc.fallback {
			if err != nil || len(graphs) != 6 {
				t.Fatalf("missing catalogue fallback: %v %v", graphs, err)
			}
		} else if err == nil {
			t.Fatalf("catalogue failure concealed: %d", tc.status)
		}
		client.Close()
	}
	client := newLegacyRESTReportingClient(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.GetSystemTelemetry(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled catalogue: %v", err)
	}
	replies := core13ReportingFixture(t)
	missing := bindLegacyReportingResponses([]map[string]any{reportingGraph("interface", "nic-a"), reportingGraph("interface", "nic-b")}, replies)
	if got := parseSystemMetricHistory(missing); got != nil {
		t.Fatalf("omitted device became a complete host rate: %+v", got)
	}
	oversized := `[{"name":"disk","identifiers":[` + strings.Repeat(`"disk-a",`, 300) + `"disk-b"]}]`
	// Duplicate catalogue entries do not consume the graph bound.
	client = newLegacyRESTReportingClient(t, alertArgsTransport{"/api/v2.0/reporting/graphs": {body: oversized}})
	if graphs, err := client.legacyReportingGraphs(context.Background()); err != nil || len(graphs) != 6 {
		t.Fatalf("catalogue deduplication: %v %v", graphs, err)
	}
	client.Close()
}

func TestCORE13CatalogueRejectsUniqueOverflow(t *testing.T) {
	identifiers := make([]string, 253)
	for i := range identifiers {
		identifiers[i] = fmt.Sprintf("disk-%d", i)
	}
	raw, err := json.Marshal([]map[string]any{{"name": "disk", "identifiers": identifiers}})
	if err != nil {
		t.Fatal(err)
	}
	client := newLegacyRESTReportingClient(t, alertArgsTransport{"/api/v2.0/reporting/graphs": {body: string(raw)}})
	defer client.Close()
	if graphs, err := client.legacyReportingGraphs(context.Background()); err == nil || graphs != nil {
		t.Fatal("oversized unique catalogue became a partial host total")
	}
}

func TestCORE13ARCAlignedWithFreeMemory(t *testing.T) {
	replies := core13ReportingFixture(t)
	history := parseSystemMetricHistory(replies)
	history.ARCSizeBytes = []TimeSeriesPoint{{Timestamp: time.Unix(1789000000-10, 0), Value: 8 << 30}}
	system := systemInfoFromMetricHistory(history)
	if system.ARCSizeBytes != 0 {
		t.Fatal("old ARC subtracted from newer memory")
	}
	history.ARCSizeBytes = append(history.ARCSizeBytes, TimeSeriesPoint{Timestamp: time.Unix(1789000000, 0), Value: 8 << 30})
	system = systemInfoFromMetricHistory(history)
	system.MemoryTotalBytes = 32 << 30
	if system.ARCSizeBytes != 8<<30 {
		t.Fatal("same-bucket ARC lost")
	}
	points := systemMemoryPercentHistory(history, system.MemoryTotalBytes)
	want := metricsFromTrueNASSystem(*system, 0, 0).Memory.Percent
	assertCOREPoint(t, points, 0, 1789000000, want)
	history.ARCSizeBytes[1].Value = 64 << 30
	points = systemMemoryPercentHistory(history, system.MemoryTotalBytes)
	assertCOREPoint(t, points, 0, 1789000000, 0)
}

func TestCORE13CoarseStepDoesNotMakeOldRowsCurrent(t *testing.T) {
	response := trueNASReportingGetDataResponse{Name: "cpu", Legend: []string{"usage"}, Start: 1789000000, End: 1789000000, Step: 3600, Data: []any{[]any{12.}}}
	if history := parseSystemMetricHistory(liveReportingResponses([]trueNASReportingGetDataResponse{response}, 1789000400)); history != nil {
		t.Fatal("coarse step made an old CPU sample current")
	}
	if history := parseSystemMetricHistory([]trueNASReportingGetDataResponse{response}); history == nil {
		t.Fatal("live age filtering destroyed native History")
	}
}

func TestCORE13ARCOnlyDoesNotInventMemoryUsage(t *testing.T) {
	history := &SystemMetricHistory{ARCSizeBytes: []TimeSeriesPoint{{Timestamp: time.Unix(1789000000, 0), Value: 8 << 30}}}
	system := systemInfoFromMetricHistory(history)
	if system.ARCSizeBytes != 8<<30 || system.Telemetry.Memory {
		t.Fatal("ARC-only telemetry lost or claimed free RAM")
	}
	system.MemoryTotalBytes = 32 << 30
	if metricsFromTrueNASSystem(*system, 0, 0).Memory != nil {
		t.Fatal("ARC-only reading invented usage")
	}
}

func TestCORE13DiskTemperatureHistoryExternalTiming(t *testing.T) {
	response := trueNASReportingGetDataResponse{Name: "disktemp", Identifier: "disk-a", Legend: []string{"temperature"}, Start: 1789000000, End: 1789000020, Step: 10, Data: []any{[]any{40.}, []any{nil}, []any{42.}}}
	points := parseReportingDiskTemperatureHistory([]trueNASReportingGetDataResponse{response})["disk-a"]
	assertCOREPoint(t, points, 0, response.Start, 40)
	assertCOREPoint(t, points, 1, response.Start+20, 42)
}
