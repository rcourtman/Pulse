package truenas

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// These are protocol fixtures for #2519's two distinct reporting methods, not
// captures of the reporter's appliance. Epochs/Celsius are actual row samples;
// aggregation means must never be projected as historical observations.
func diskHistoryFixture(identifier string, end int64) map[string]any {
	return map[string]any{
		"name": "disktemp", "identifier": identifier, "legend": []string{"temperature"},
		"data":         [][]any{{end - 3540, 31.25}, {end - 1800, 33.5}, {end - 60, 32.75}},
		"aggregations": map[string]any{"mean": []float64{999}},
	}
}

func TestDiskHistoryFillsOnlyMissingSeriesFromNetdata(t *testing.T) {
	var calls atomic.Int32
	var end int64
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		auth := readRPCRequest(t, conn)
		writeRPCResult(t, conn, auth.ID, true)
		primary := readRPCRequest(t, conn)
		calls.Add(1)
		if primary.Method != "reporting.get_data" {
			t.Fatalf("primary method %q", primary.Method)
		}
		params := primary.Params.([]any)
		query := params[1].(map[string]any)
		end = assertReportingRange(t, query, 3600)
		if !reflect.DeepEqual(params[0], []any{map[string]any{"name": "disktemp", "identifier": "sda"}, map[string]any{"name": "disktemp", "identifier": "sdb"}}) {
			t.Fatalf("primary graphs: %#v", params[0])
		}
		writeRPCResult(t, conn, primary.ID, []any{diskHistoryFixture("sda", end)})
		native := readRPCRequest(t, conn)
		calls.Add(1)
		if native.Method != "reporting.netdata_get_data" {
			t.Fatalf("native method %q", native.Method)
		}
		nativeParams := native.Params.([]any)
		if !reflect.DeepEqual(nativeParams[1], query) || !reflect.DeepEqual(nativeParams[0], []any{map[string]any{"name": "disktemp", "identifier": "sdb"}}) {
			t.Fatalf("native read widened window/identities: %#v", nativeParams)
		}
		result := diskHistoryFixture("sdb", end)
		result["data"] = [][]any{{end - 3601, 10}, {end - 3540, 31.25}, {end - 1800, 33.5}, {end - 60, 32.75}, {end + 1, 50}, {end - 50, nil}, {end - 40, 0}, {end - 30, -1}}
		// A response for an unrequested disk must not overwrite the successful
		// primary series or escape the fixed missing-identifier query.
		wrong := diskHistoryFixture("sda", end)
		wrong["data"] = [][]any{{end - 60, 999}}
		writeRPCResult(t, conn, native.ID, []any{result, wrong, diskHistoryFixture("foreign", end)})
	})
	t.Cleanup(server.Close)
	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "fixture-key"})
	got, err := client.GetDiskTemperatureHistory(context.Background(), []string{"sda", "sdb", "sdb"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || calls.Load() != 2 {
		t.Fatalf("history/calls: %+v / %d", got, calls.Load())
	}
	want := []TimeSeriesPoint{{Timestamp: time.Unix(end-3540, 0).UTC(), Value: 31.25}, {Timestamp: time.Unix(end-1800, 0).UTC(), Value: 33.5}, {Timestamp: time.Unix(end-60, 0).UTC(), Value: 32.75}}
	for _, identifier := range []string{"sda", "sdb"} {
		if !reflect.DeepEqual(got[identifier], want) {
			t.Fatalf("%s timestamps/Celsius/window changed: %+v", identifier, got[identifier])
		}
	}
}

func TestDiskHistoryPrimaryFailureDoesNotTryNetdata(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
	}{{"EACCES", -32001}, {"EINVAL", -32001}, {"ENOSYS", -32601}} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
				auth := readRPCRequest(t, conn)
				writeRPCResult(t, conn, auth.ID, true)
				req := readRPCRequest(t, conn)
				calls.Add(1)
				writeReportingRPCFailure(t, conn, req.ID, test.code, test.name, "fixture failure")
			})
			t.Cleanup(server.Close)
			client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "fixture-key"})
			got, err := client.GetDiskTemperatureHistory(context.Background(), []string{"sda"}, time.Hour)
			var rpcErr *RPCError
			if len(got) != 0 || !errors.As(err, &rpcErr) || rpcErr.Method != "reporting.get_data" || rpcErr.Code != test.code || rpcErr.Errname != test.name || calls.Load() != 1 {
				t.Fatalf("failure/fallback changed: history %+v err %v calls %d", got, err, calls.Load())
			}
		})
	}
}

func TestDiskHistoryNetdataFailureKeepsIndependentSeries(t *testing.T) {
	for _, test := range []struct {
		name string
		code int
	}{{"EACCES", -32001}, {"ENOSYS", -32601}, {"EINVAL", -32001}} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
				auth := readRPCRequest(t, conn)
				writeRPCResult(t, conn, auth.ID, true)
				req := readRPCRequest(t, conn)
				calls.Add(1)
				end := int64(req.Params.([]any)[1].(map[string]any)["end"].(float64))
				writeRPCResult(t, conn, req.ID, []any{diskHistoryFixture("sda", end)})
				req = readRPCRequest(t, conn)
				calls.Add(1)
				writeReportingRPCFailure(t, conn, req.ID, test.code, test.name, "fixture failure")
			})
			t.Cleanup(server.Close)
			client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "fixture-key"})
			got, err := client.GetDiskTemperatureHistory(context.Background(), []string{"sda", "sdb"}, time.Hour)
			var rpcErr *RPCError
			if len(got["sda"]) != 3 || len(got) != 1 || !errors.As(err, &rpcErr) || rpcErr.Method != "reporting.netdata_get_data" || rpcErr.Errname != test.name || calls.Load() != 2 {
				t.Fatalf("partial/failure lost: history %+v err %v calls %d", got, err, calls.Load())
			}
		})
	}
}

func TestDiskHistoryEmptyNetdataDoesNotInventSamples(t *testing.T) {
	var calls atomic.Int32
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		auth := readRPCRequest(t, conn)
		writeRPCResult(t, conn, auth.ID, true)
		for _, method := range []string{"reporting.get_data", "reporting.netdata_get_data"} {
			req := readRPCRequest(t, conn)
			calls.Add(1)
			if req.Method != method {
				t.Fatalf("method %q want %q", req.Method, method)
			}
			writeRPCResult(t, conn, req.ID, []any{})
		}
	})
	t.Cleanup(server.Close)
	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "fixture-key"})
	got, err := client.GetDiskTemperatureHistory(context.Background(), []string{"sda"}, time.Hour)
	if err != nil || len(got) != 0 || calls.Load() != 2 {
		t.Fatalf("empty result changed: history %+v err %v calls %d", got, err, calls.Load())
	}
}
