package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// This models the response-validation failure supplied in #2346, not a capture
// of the appliance response. Aggregation summaries accompany, not replace, data.
const scale2504ReportingValidationError = "6 validation errors for ReportingGetDataResult\n" +
	"result.0.aggregations.min\n  Field required [type=missing, input_value={}, input_type=dict]\n" +
	"result.0.aggregations.mean\n  Field required [type=missing, input_value={}, input_type=dict]\n" +
	"result.0.aggregations.max\n  Field required [type=missing, input_value={}, input_type=dict]\n" +
	"result.1.aggregations.min\n  Field required [type=missing, input_value={}, input_type=dict]\n" +
	"result.1.aggregations.mean\n  Field required [type=missing, input_value={}, input_type=dict]\n" +
	"result.1.aggregations.max\n  Field required [type=missing, input_value={}, input_type=dict]"

func writeReportingRPCFailure(t *testing.T, conn *websocket.Conn, id int64, code int, name, message string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"errname": name})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(trueNASRPCResponse{
		JSONRPC: "2.0", ID: id,
		Error: &trueNASRPCError{Code: code, Message: message, Data: data},
	}); err != nil {
		t.Fatal(err)
	}
}

func aggregatedReportingFixture(name string, end int64, count int) map[string]any {
	legend := map[string][]string{
		"cpu": {"usage"}, "memory": {"used", "total"}, "arcsize": {"size"},
		"interface": {"received", "sent"}, "disk": {"read", "write"},
		"disktemp": {"temperature"},
	}[name]
	data := make([]any, count)
	for i := range data {
		row := []any{end - int64(count-1) + int64(i)}
		switch name {
		case "cpu":
			row = append(row, 10+i%20)
		case "memory":
			row = append(row, int64(4+i%2)<<30, int64(16)<<30)
		case "arcsize":
			row = append(row, int64(1)<<30)
		case "interface":
			row = append(row, 4096, 2048)
		case "disk":
			row = append(row, 8192, 4096)
		case "disktemp":
			row = append(row, 30+i%5)
		}
		data[i] = row
	}
	return map[string]any{
		"name": name, "legend": legend, "data": data,
		"start": end - int64(count-1), "end": end,
		// Deliberately different from all sample values: summaries must not be
		// substituted for raw samples in History or latest live telemetry.
		"aggregations": map[string]any{
			"min": []int{-999}, "mean": []int{999}, "max": []int{9999},
		},
	}
}

func assertReportingRange(t *testing.T, query map[string]any, seconds int64) int64 {
	t.Helper()
	start, startOK := query["start"].(float64)
	end, endOK := query["end"].(float64)
	if !startOK || !endOK || len(query) != 3 || int64(end-start) != seconds {
		t.Fatalf("reporting query window changed: %#v, want %ds", query, seconds)
	}
	if delta := time.Now().Unix() - int64(end); delta < 0 || delta > 5 {
		t.Fatalf("reporting end is not current: %#v", query)
	}
	return int64(end)
}

func assertRawReportingSeries(t *testing.T, points []TimeSeriesPoint, end int64, value func(int) float64) {
	t.Helper()
	if len(points) != 3600 {
		t.Fatalf("raw series has %d samples, want 3600", len(points))
	}
	for i, point := range points {
		if !point.Timestamp.Equal(time.Unix(end-3599+int64(i), 0)) || point.Value != value(i) {
			t.Fatalf("raw sample %d changed: %+v", i, point)
		}
	}
}

func TestReportingHistorySupportsSCALE2504Aggregations(t *testing.T) {
	for _, test := range []struct {
		name     string
		disk     bool
		duration time.Duration
		seconds  int64
	}{
		{"system", false, time.Hour, 3600},
		{"system default", false, -time.Second, 86400},
		{"disk", true, time.Hour, 3600},
		{"disk default", true, 0, 86400},
	} {
		t.Run(test.name, func(t *testing.T) {
			var responseEnd atomic.Int64
			server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
				auth := readRPCRequest(t, conn)
				writeRPCResult(t, conn, auth.ID, true)
				request := readRPCRequest(t, conn)
				if request.Method != "reporting.get_data" {
					t.Fatalf("unexpected method: %s", request.Method)
				}
				params := request.Params.([]any)
				query := params[1].(map[string]any)
				if query["aggregate"] != true {
					writeReportingRPCFailure(t, conn, request.ID, -32001, "EINVAL", scale2504ReportingValidationError)
					return
				}
				end := assertReportingRange(t, query, test.seconds)
				responseEnd.Store(end)
				graphs := params[0].([]any)
				wantGraphs := 5
				if test.disk {
					wantGraphs = 1
				}
				if len(graphs) != wantGraphs {
					t.Fatalf("graphs = %#v, want %d", graphs, wantGraphs)
				}
				var result []map[string]any
				for _, raw := range graphs {
					graph := raw.(map[string]any)
					entry := aggregatedReportingFixture(graph["name"].(string), end, 3600)
					entry["identifier"] = graph["identifier"]
					result = append(result, entry)
				}
				writeRPCResult(t, conn, request.ID, result)
			})
			t.Cleanup(server.Close)
			client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "fixture-key"})
			if test.disk {
				history, err := client.GetDiskTemperatureHistory(context.Background(), []string{"sda"}, test.duration)
				if err != nil {
					t.Fatal(err)
				}
				assertRawReportingSeries(t, history["sda"], responseEnd.Load(), func(i int) float64 { return float64(30 + i%5) })
				return
			}
			// Use the real API history fetcher and canonical provider projection;
			// cached inventory is synthetic and is not an appliance identity proof.
			fixtures := DefaultFixtures()
			fixtures.System.MemoryTotalBytes = 16 << 30
			provider := NewProvider(fixtures)
			provider.fetcher = &APIFetcher{Client: client}
			resourceID, history, err := provider.SystemMetricHistory(context.Background(), test.duration)
			if err != nil {
				t.Fatal(err)
			}
			if resourceID != "truenas-main" {
				t.Fatalf("canonical metrics target changed: %q", resourceID)
			}
			end := responseEnd.Load()
			assertRawReportingSeries(t, history["cpu"], end, func(i int) float64 { return float64(10 + i%20) })
			assertRawReportingSeries(t, history["memory"], end, func(i int) float64 { return float64(3+i%2) / 16 * 100 })
			for key, value := range map[string]float64{"netin": 4096, "netout": 2048, "diskread": 8192, "diskwrite": 4096} {
				assertRawReportingSeries(t, history[key], end, func(int) float64 { return value })
			}
		})
	}
}

func TestReportingAggregationsPreserveRPCFailures(t *testing.T) {
	for _, test := range []struct {
		name, errname, message string
		code                   int
	}{
		{"validation", "EINVAL", scale2504ReportingValidationError, -32001},
		{"access", "EACCES", "permission denied", -32001},
		{"unrelated validation", "EINVAL", "unknown graph", -32001},
		{"missing method", "ENOSYS", "method not found", -32601},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
				auth := readRPCRequest(t, conn)
				writeRPCResult(t, conn, auth.ID, true)
				request := readRPCRequest(t, conn)
				calls.Add(1)
				writeReportingRPCFailure(t, conn, request.ID, test.code, test.errname, test.message)
			})
			t.Cleanup(server.Close)
			client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "fixture-key"})
			history, err := client.GetSystemMetricHistory(context.Background(), time.Hour)
			var rpcErr *RPCError
			if history != nil || !errors.As(err, &rpcErr) || rpcErr.Code != test.code || rpcErr.Errname != test.errname || rpcErr.Message != test.message {
				t.Fatalf("reporting error lost: history=%+v, err=%v", history, err)
			}
			if calls.Load() != 1 {
				t.Fatalf("unexpected reporting retry: %d calls", calls.Load())
			}
		})
	}
}

func TestRESTReportingDefaultAggregationsPreserveSamples(t *testing.T) {
	for _, live := range []bool{false, true} {
		name := "history"
		seconds := int64(3600)
		if live {
			name, seconds = "live telemetry", legacyRESTTelemetryWindowSeconds
		}
		t.Run(name, func(t *testing.T) {
			var end int64
			var calls int
			client := newLegacyRESTReportingClient(t, nil)
			client.httpClient.Transport = reportingAggregationRESTFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/api/v2.0/reporting/graphs" {
					return (alertArgsTransport{}).RoundTrip(r)
				}
				calls++
				var request struct {
					Graphs []map[string]any `json:"graphs"`
					Query  map[string]any   `json:"query"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					return nil, err
				}
				if r.Method != http.MethodPost || request.Query["aggregate"] != true {
					t.Fatalf("unexpected reporting request: %s %#v", r.Method, request.Query)
				}
				end = assertReportingRange(t, request.Query, seconds)
				var result []map[string]any
				for _, graph := range request.Graphs {
					if _, present := graph["identifier"]; present {
						t.Fatalf("legacy graph shape changed: %#v", graph)
					}
					result = append(result, aggregatedReportingFixture(graph["name"].(string), end, int(seconds)))
				}
				body, err := json.Marshal(result)
				if err != nil {
					return nil, err
				}
				return (alertArgsTransport{r.URL.Path: apiResponse{body: string(body)}}).RoundTrip(r)
			})
			if live {
				system, err := client.GetSystemTelemetry(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				if system.CPUPercent != 29 || system.MemoryTotalBytes != 16<<30 || system.ARCSizeBytes != 1<<30 || system.NetInRate != 4096 || system.DiskWriteRate != 4096 {
					t.Fatalf("latest raw telemetry changed: %+v", system)
				}
			} else {
				history, err := client.GetSystemMetricHistory(context.Background(), time.Hour)
				if err != nil {
					t.Fatal(err)
				}
				assertRawReportingSeries(t, history.CPUPercent, end, func(i int) float64 { return float64(10 + i%20) })
				assertRawReportingSeries(t, systemMemoryPercentHistory(history, 16<<30), end, func(i int) float64 { return float64(3+i%2) / 16 * 100 })
			}
			if calls != 1 {
				t.Fatalf("successful REST batch issued %d requests, want 1", calls)
			}
		})
	}
}

type reportingAggregationRESTFunc func(*http.Request) (*http.Response, error)

func (f reportingAggregationRESTFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
