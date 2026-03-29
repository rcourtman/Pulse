package truenas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type apiResponse struct {
	status      int
	body        string
	contentType string
}

type closeTrackingTransport struct {
	closeCalls int
}

func (t *closeTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

func (t *closeTrackingTransport) CloseIdleConnections() {
	t.closeCalls++
}

func TestClientGetters(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	ctx := context.Background()

	system, err := client.GetSystemInfo(ctx)
	if err != nil {
		t.Fatalf("GetSystemInfo() error = %v", err)
	}
	if system.Hostname != "truenas-main" || system.Version != "TrueNAS-SCALE-24.10.2" {
		t.Fatalf("unexpected system info: %+v", system)
	}
	if system.Build != "24.10.2.1" || system.UptimeSeconds != 86400 {
		t.Fatalf("unexpected system fields: %+v", system)
	}
	if !system.Healthy || system.MachineID != "SER123" {
		t.Fatalf("unexpected system health/identity: %+v", system)
	}
	if system.CPUCount != 16 || system.MemoryTotalBytes != 68719476736 {
		t.Fatalf("unexpected system capacity mapping: %+v", system)
	}

	pools, err := client.GetPools(ctx)
	if err != nil {
		t.Fatalf("GetPools() error = %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].ID != "1" || pools[0].Name != "tank" || pools[0].UsedBytes != 400 {
		t.Fatalf("unexpected pool mapping: %+v", pools[0])
	}

	datasets, err := client.GetDatasets(ctx)
	if err != nil {
		t.Fatalf("GetDatasets() error = %v", err)
	}
	if len(datasets) != 1 {
		t.Fatalf("expected 1 dataset, got %d", len(datasets))
	}
	if datasets[0].ID != "tank/apps" || datasets[0].UsedBytes != 12345 || datasets[0].AvailBytes != 555 {
		t.Fatalf("unexpected dataset usage mapping: %+v", datasets[0])
	}
	if !datasets[0].Mounted || datasets[0].ReadOnly {
		t.Fatalf("unexpected dataset mount/readonly mapping: %+v", datasets[0])
	}

	disks, err := client.GetDisks(ctx)
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("expected 2 disks, got %d", len(disks))
	}
	if disks[0].Transport != "sata" || !disks[0].Rotational {
		t.Fatalf("unexpected rotational disk mapping: %+v", disks[0])
	}
	if disks[1].Transport != "nvme" || disks[1].Rotational {
		t.Fatalf("unexpected nvme disk mapping: %+v", disks[1])
	}
	if disks[0].Temperature != 34 || disks[1].Temperature != 49 {
		t.Fatalf("unexpected disk temperatures: %+v", disks)
	}

	alerts, err := client.GetAlerts(ctx)
	if err != nil {
		t.Fatalf("GetAlerts() error = %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].ID != "a1" || alerts[0].Level != "WARNING" {
		t.Fatalf("unexpected alert identity mapping: %+v", alerts[0])
	}
	if alerts[0].Datetime != time.UnixMilli(1707400000000).UTC() {
		t.Fatalf("unexpected alert datetime: %s", alerts[0].Datetime)
	}

	apps, err := client.GetApps(ctx)
	if err != nil {
		t.Fatalf("GetApps() error = %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	if apps[0].ID != "nextcloud" || apps[0].Name != "Nextcloud" {
		t.Fatalf("unexpected app identity mapping: %+v", apps[0])
	}
	if apps[0].ContainerCount != 2 || len(apps[0].Containers) != 2 {
		t.Fatalf("unexpected app container mapping: %+v", apps[0])
	}
	if len(apps[0].UsedPorts) != 1 || apps[0].UsedPorts[0].ContainerPort != 443 {
		t.Fatalf("unexpected app used ports mapping: %+v", apps[0].UsedPorts)
	}
	if len(apps[0].Volumes) != 2 || len(apps[0].Networks) != 1 {
		t.Fatalf("unexpected app volume/network mapping: volumes=%d networks=%d", len(apps[0].Volumes), len(apps[0].Networks))
	}
}

func TestClientAuthHeaderAPIKey(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
	}, func(t *testing.T, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected bearer auth header, got %q", got)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "test-key"})
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestClientAuthHeaderBasic(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
	}, func(t *testing.T, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok {
			t.Fatalf("expected basic auth")
		}
		if username != "admin" || password != "secret" {
			t.Fatalf("unexpected basic auth credentials: %q:%q", username, password)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{Username: "admin", Password: "secret"})
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() error = %v", err)
	}
}

func TestTestConnectionSuccessAndFailure(t *testing.T) {
	successServer := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
	}, nil)
	t.Cleanup(successServer.Close)

	successClient := mustClientForServer(t, successServer.URL, ClientConfig{APIKey: "key"})
	if err := successClient.TestConnection(context.Background()); err != nil {
		t.Fatalf("TestConnection() success error = %v", err)
	}

	failureServer := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {status: http.StatusUnauthorized, body: `{"error":"unauthorized"}`},
	}, nil)
	t.Cleanup(failureServer.Close)

	failureClient := mustClientForServer(t, failureServer.URL, ClientConfig{APIKey: "bad"})
	if err := failureClient.TestConnection(context.Background()); err == nil {
		t.Fatal("expected TestConnection() failure error")
	}
}

func TestFetchSnapshot(t *testing.T) {
	server := newMockServer(t, defaultAPIResponses(), nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	snapshot, err := client.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected snapshot")
	}
	if snapshot.CollectedAt.IsZero() {
		t.Fatal("expected non-zero CollectedAt")
	}
	if snapshot.System.Hostname != "truenas-main" {
		t.Fatalf("unexpected snapshot system: %+v", snapshot.System)
	}
	if len(snapshot.Pools) != 1 || len(snapshot.Datasets) != 1 || len(snapshot.Disks) != 2 || len(snapshot.Alerts) != 1 || len(snapshot.Apps) != 1 {
		t.Fatalf("unexpected snapshot counts: pools=%d datasets=%d disks=%d alerts=%d apps=%d",
			len(snapshot.Pools), len(snapshot.Datasets), len(snapshot.Disks), len(snapshot.Alerts), len(snapshot.Apps))
	}
	if snapshot.Disks[0].Temperature != 34 || snapshot.Disks[1].Temperature != 49 {
		t.Fatalf("unexpected snapshot disk temperatures: %+v", snapshot.Disks)
	}
	if snapshot.Apps[0].ID != "nextcloud" || snapshot.Apps[0].ContainerCount != 2 {
		t.Fatalf("unexpected snapshot apps: %+v", snapshot.Apps)
	}
}

func TestGetAppsEnrichesStatsFromRPC(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		subscribeReq := readRPCRequest(t, conn)
		if subscribeReq.Method != "core.subscribe" {
			t.Fatalf("expected core.subscribe, got %q", subscribeReq.Method)
		}
		writeRPCResult(t, conn, subscribeReq.ID, "sub-1")
		writeRPCNotification(t, conn, "collection_update", map[string]any{
			"collection": "app.stats:{\"interval\":2}",
			"fields": []map[string]any{
				{
					"app_name":  "nextcloud",
					"cpu_usage": 17,
					"memory":    268435456,
					"networks": []map[string]any{
						{"interface_name": "eth0", "rx_bytes": 2048, "tx_bytes": 1024},
						{"interface_name": "eth1", "rx_bytes": 512, "tx_bytes": 256},
					},
					"blkio": map[string]any{
						"read":  4096,
						"write": 2048,
					},
				},
			},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	apps, err := client.GetApps(context.Background())
	if err != nil {
		t.Fatalf("GetApps() error = %v", err)
	}
	if len(apps) != 1 || apps[0].Stats == nil {
		t.Fatalf("expected one app with stats, got %+v", apps)
	}
	if apps[0].Stats.CPUPercent != 17 || apps[0].Stats.MemoryBytes != 268435456 {
		t.Fatalf("unexpected app stats core fields: %+v", apps[0].Stats)
	}
	if apps[0].Stats.NetInRate != 2560 || apps[0].Stats.NetOutRate != 1280 {
		t.Fatalf("unexpected aggregated app network rates: %+v", apps[0].Stats)
	}
	if apps[0].Stats.BlockReadBytes != 4096 || apps[0].Stats.BlockWriteBytes != 2048 {
		t.Fatalf("unexpected app blkio stats: %+v", apps[0].Stats)
	}
	if len(apps[0].Stats.Interfaces) != 2 {
		t.Fatalf("expected two interface stats, got %+v", apps[0].Stats.Interfaces)
	}
}

func TestStartAndStopAppUseRPCMethods(t *testing.T) {
	var rpcCalls int
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		rpcCalls++
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		actionReq := readRPCRequest(t, conn)
		expectedMethod := "app.start"
		if rpcCalls == 2 {
			expectedMethod = "app.stop"
		}
		if actionReq.Method != expectedMethod {
			t.Fatalf("expected %s, got %q", expectedMethod, actionReq.Method)
		}
		writeRPCResult(t, conn, actionReq.ID, true)
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	if err := client.StartApp(context.Background(), "nextcloud"); err != nil {
		t.Fatalf("StartApp() error = %v", err)
	}
	if err := client.StopApp(context.Background(), "nextcloud"); err != nil {
		t.Fatalf("StopApp() error = %v", err)
	}
	if rpcCalls != 2 {
		t.Fatalf("expected two RPC app-action sessions, got %d", rpcCalls)
	}
}

func TestGetAppLogsUsesRPCSubscription(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		subscribeReq := readRPCRequest(t, conn)
		if subscribeReq.Method != "core.subscribe" {
			t.Fatalf("expected core.subscribe, got %q", subscribeReq.Method)
		}
		params, ok := subscribeReq.Params.([]any)
		if !ok || len(params) != 1 {
			t.Fatalf("expected one subscription param, got %#v", subscribeReq.Params)
		}
		subscriptionName, _ := params[0].(string)
		if !strings.HasPrefix(subscriptionName, "app.container_log_follow:") {
			t.Fatalf("expected app.container_log_follow subscription, got %q", subscriptionName)
		}
		if !strings.Contains(subscriptionName, "\"app_name\":\"nextcloud\"") || !strings.Contains(subscriptionName, "\"container_id\":\"nextcloud-web-1\"") {
			t.Fatalf("expected subscription args for nextcloud-web-1, got %q", subscriptionName)
		}
		writeRPCResult(t, conn, subscribeReq.ID, "sub-logs")
		writeRPCNotification(t, conn, "collection_update", map[string]any{
			"collection": "app.container_log_follow:{\"app_name\":\"nextcloud\",\"container_id\":\"nextcloud-web-1\",\"tail_lines\":2}",
			"fields": map[string]any{
				"data":      "ready",
				"timestamp": "2026-03-29T18:00:00Z",
			},
		})
		writeRPCNotification(t, conn, "collection_update", map[string]any{
			"collection": "app.container_log_follow:{\"app_name\":\"nextcloud\",\"container_id\":\"nextcloud-web-1\",\"tail_lines\":2}",
			"fields": map[string]any{
				"data":      "serving",
				"timestamp": "2026-03-29T18:01:00Z",
			},
		})
		time.Sleep(defaultAppLogIdleWait + 100*time.Millisecond)
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	lines, err := client.GetAppLogs(context.Background(), "nextcloud", "nextcloud-web-1", 2)
	if err != nil {
		t.Fatalf("GetAppLogs() error = %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected two log lines, got %+v", lines)
	}
	if lines[0].Timestamp != "2026-03-29T18:00:00Z" || lines[0].Data != "ready" {
		t.Fatalf("unexpected first log line: %+v", lines[0])
	}
	if lines[1].Data != "serving" {
		t.Fatalf("unexpected second log line: %+v", lines[1])
	}
}

func TestGetSystemTelemetryFromRPC(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		temperatureReq := readRPCRequest(t, conn)
		if temperatureReq.Method != "reporting.get_data" {
			t.Fatalf("expected reporting.get_data, got %q", temperatureReq.Method)
		}
		writeRPCResult(t, conn, temperatureReq.ID, []map[string]any{{
			"name":       "cputemp",
			"identifier": nil,
			"legend":     []string{"cpu_package", "core 0", "core 1"},
			"aggregations": map[string]any{
				"mean": map[string]any{
					"cpu_package": 61.5,
					"core 0":      58.0,
					"core 1":      59.0,
				},
				"min": map[string]any{
					"cpu_package": 60.0,
					"core 0":      57.0,
					"core 1":      58.0,
				},
				"max": map[string]any{
					"cpu_package": 63.0,
					"core 0":      60.0,
					"core 1":      61.0,
				},
			},
			"data":  []any{},
			"start": time.Now().Add(-5 * time.Minute).Unix(),
			"end":   time.Now().Unix(),
		}})

		subscribeReq := readRPCRequest(t, conn)
		if subscribeReq.Method != "core.subscribe" {
			t.Fatalf("expected core.subscribe, got %q", subscribeReq.Method)
		}
		writeRPCResult(t, conn, subscribeReq.ID, "sub-1")
		writeRPCNotification(t, conn, "collection_update", map[string]any{
			"collection": "reporting.realtime:{\"interval\":2}",
			"fields": map[string]any{
				"cpu": map[string]any{
					"usage": 41,
				},
				"memory": map[string]any{
					"physical_memory_total":     68719476736,
					"physical_memory_available": 21474836480,
				},
				"interfaces": map[string]any{
					"enp1s0": map[string]any{"rx_bytes": 4096, "tx_bytes": 2048},
					"enp2s0": map[string]any{"received_bytes": 1024, "sent_bytes": 512},
				},
				"disks": map[string]any{
					"sda":     map[string]any{"read_bytes": 2048, "write_bytes": 1024},
					"nvme0n1": map[string]any{"read_bytes": 4096, "write_bytes": 3072},
				},
			},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	system, err := client.GetSystemTelemetry(context.Background())
	if err != nil {
		t.Fatalf("GetSystemTelemetry() error = %v", err)
	}
	if system == nil {
		t.Fatal("expected system telemetry")
	}
	if system.CPUPercent != 41 {
		t.Fatalf("expected cpu percent 41, got %+v", system)
	}
	if system.MemoryTotalBytes != 68719476736 || system.MemoryAvailableBytes != 21474836480 {
		t.Fatalf("unexpected memory telemetry: %+v", system)
	}
	if system.NetInRate != 5120 || system.NetOutRate != 2560 {
		t.Fatalf("unexpected network telemetry: %+v", system)
	}
	if system.DiskReadRate != 6144 || system.DiskWriteRate != 4096 {
		t.Fatalf("unexpected disk telemetry: %+v", system)
	}
	if got := system.TemperatureCelsius["cpu_package"]; got != 61.5 {
		t.Fatalf("expected cpu_package temperature 61.5, got %+v", system.TemperatureCelsius)
	}
	if got := system.TemperatureCelsius["cpu_core_0"]; got != 58.0 {
		t.Fatalf("expected cpu_core_0 temperature 58.0, got %+v", system.TemperatureCelsius)
	}
	if system.IntervalSeconds != 2 || system.CollectedAt.IsZero() {
		t.Fatalf("expected interval/collectedAt metadata, got %+v", system)
	}
}

func TestGetSystemTelemetryIgnoresUnavailableTemperatureRPC(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		temperatureReq := readRPCRequest(t, conn)
		if temperatureReq.Method != "reporting.get_data" {
			t.Fatalf("expected reporting.get_data, got %q", temperatureReq.Method)
		}
		writeRPCError(t, conn, temperatureReq.ID, -32601, "not found")

		subscribeReq := readRPCRequest(t, conn)
		if subscribeReq.Method != "core.subscribe" {
			t.Fatalf("expected core.subscribe, got %q", subscribeReq.Method)
		}
		writeRPCResult(t, conn, subscribeReq.ID, "sub-1")
		writeRPCNotification(t, conn, "collection_update", map[string]any{
			"collection": "reporting.realtime:{\"interval\":2}",
			"fields": map[string]any{
				"cpu": map[string]any{"usage": 41},
			},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	system, err := client.GetSystemTelemetry(context.Background())
	if err != nil {
		t.Fatalf("GetSystemTelemetry() error = %v", err)
	}
	if system == nil {
		t.Fatal("expected system telemetry")
	}
	if system.CPUPercent != 41 {
		t.Fatalf("expected cpu percent 41, got %+v", system)
	}
	if len(system.TemperatureCelsius) != 0 {
		t.Fatalf("expected unavailable temperature RPC to be ignored, got %+v", system.TemperatureCelsius)
	}
}

func TestGetDiskTemperaturesSupportsArrayShape(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/disk/temperatures": {
			body: `[{"name":"sda","temperature":33},{"identifier":"{disk-2}","temperature_celsius":"48"},{"serial":"SER-C","temperature":{"parsed":52}}]`,
		},
	}, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	temperatures, err := client.GetDiskTemperatures(context.Background())
	if err != nil {
		t.Fatalf("GetDiskTemperatures() error = %v", err)
	}
	if got := temperatures["sda"]; got != 33 {
		t.Fatalf("expected sda temperature 33, got %d", got)
	}
	if got := temperatures["{disk-2}"]; got != 48 {
		t.Fatalf("expected {disk-2} temperature 48, got %d", got)
	}
	if got := temperatures["SER-C"]; got != 52 {
		t.Fatalf("expected SER-C temperature 52, got %d", got)
	}
}

func TestGetDisksToleratesUnavailableTemperatureEndpoint(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/disk": {
			body: `[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"}]`,
		},
		"/api/v2.0/disk/temperatures": {
			status: http.StatusNotFound,
			body:   `{"error":"not found"}`,
		},
	}, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	disks, err := client.GetDisks(context.Background())
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
	if disks[0].Temperature != 0 {
		t.Fatalf("expected unavailable temperature to stay empty, got %+v", disks[0])
	}
}

func TestClientHandlesHTTPAndDecodeErrors(t *testing.T) {
	t.Run("non-2xx response", func(t *testing.T) {
		server := newMockServer(t, map[string]apiResponse{
			"/api/v2.0/pool": {status: http.StatusServiceUnavailable, body: `{"error":"down"}`},
		}, nil)
		t.Cleanup(server.Close)

		client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
		_, err := client.GetPools(context.Background())
		if err == nil {
			t.Fatal("expected error from non-2xx response")
		}
		if !strings.Contains(err.Error(), "status=503") {
			t.Fatalf("expected status code in error, got %v", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		server := newMockServer(t, map[string]apiResponse{
			"/api/v2.0/system/info": {body: `{"hostname":`},
		}, nil)
		t.Cleanup(server.Close)

		client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
		_, err := client.GetSystemInfo(context.Background())
		if err == nil {
			t.Fatal("expected malformed json error")
		}
		if !strings.Contains(err.Error(), "decode truenas response") {
			t.Fatalf("unexpected decode error: %v", err)
		}
	})

	t.Run("connection failure", func(t *testing.T) {
		server := newMockServer(t, map[string]apiResponse{
			"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`},
		}, nil)

		client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
		server.Close()

		_, err := client.GetSystemInfo(context.Background())
		if err == nil {
			t.Fatal("expected connection error")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Fatalf("unexpected connection error: %v", err)
		}
	})
}

func TestClientRejectsOversizedJSONResponses(t *testing.T) {
	oversizedBody := fmt.Sprintf(
		`{"hostname":"%s","version":"v","buildtime":"b","uptime_seconds":1}`,
		strings.Repeat("a", int(maxResponseBodyBytes)),
	)
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: oversizedBody},
	}, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "key"})
	_, err := client.GetSystemInfo(context.Background())
	if err == nil {
		t.Fatal("expected oversized response error")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("response body exceeds %d bytes", maxResponseBodyBytes)) {
		t.Fatalf("expected response size limit error, got %v", err)
	}
}

func TestNewClientRejectsUnsafeURLComponents(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		errContains string
	}{
		{
			name:        "credentials",
			host:        "https://admin:secret@truenas.local",
			errContains: "credentials are not supported",
		},
		{
			name:        "non-root path",
			host:        "https://truenas.local/api/v2.0",
			errContains: "path is not supported",
		},
		{
			name:        "query",
			host:        "https://truenas.local?insecure=1",
			errContains: "query is not supported",
		},
		{
			name:        "fragment",
			host:        "https://truenas.local#section",
			errContains: "fragment is not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClient(ClientConfig{
				Host:   tt.host,
				APIKey: "key",
			})
			if err == nil {
				t.Fatalf("expected error for host %q", tt.host)
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
			}
		})
	}
}

func TestClientTLSFingerprintPinning(t *testing.T) {
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v2.0/system/info" {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"hostname":"nas","version":"v","buildtime":"b","uptime_seconds":1}`))
	})

	tlsServer := httptest.NewTLSServer(handler)
	t.Cleanup(tlsServer.Close)

	cert := tlsServer.Certificate()
	fingerprintRaw := sha256.Sum256(cert.Raw)
	fingerprint := withFingerprintColons(strings.ToUpper(hex.EncodeToString(fingerprintRaw[:])))

	client := mustClientForServer(t, tlsServer.URL, ClientConfig{
		APIKey:             "key",
		InsecureSkipVerify: true,
		Fingerprint:        "SHA256:" + fingerprint,
	})
	if err := client.TestConnection(context.Background()); err != nil {
		t.Fatalf("expected pinning success, got %v", err)
	}

	badClient := mustClientForServer(t, tlsServer.URL, ClientConfig{
		APIKey:             "key",
		InsecureSkipVerify: true,
		Fingerprint:        strings.Repeat("0", 64),
	})
	if err := badClient.TestConnection(context.Background()); err == nil {
		t.Fatal("expected pinning failure")
	}
}

func TestClientCloseClosesIdleConnections(t *testing.T) {
	transport := &closeTrackingTransport{}
	client := &Client{
		httpClient: &http.Client{
			Transport: transport,
		},
	}

	client.Close()
	if transport.closeCalls != 1 {
		t.Fatalf("expected CloseIdleConnections to be called once, got %d", transport.closeCalls)
	}
}

func TestClientCloseNilSafe(t *testing.T) {
	var nilClient *Client
	nilClient.Close()

	(&Client{}).Close()
	(&Client{httpClient: &http.Client{}}).Close()
}

func defaultAPIResponses() map[string]apiResponse {
	return map[string]apiResponse{
		"/api/v2.0/system/info": {
			body: `{"hostname":"truenas-main","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2.1","uptime_seconds":86400,"system_serial":"SER123","system_manufacturer":"iXsystems","physical_cores":16,"physmem":68719476736}`,
		},
		"/api/v2.0/pool": {
			body: `[{"id":1,"name":"tank","status":"ONLINE","size":1000,"allocated":400,"free":600}]`,
		},
		"/api/v2.0/pool/dataset": {
			body: `[{"id":"tank/apps","name":"tank/apps","pool":"tank","used":{"rawvalue":"12345","parsed":12345},"available":{"rawvalue":"555","parsed":555},"mountpoint":"/mnt/tank/apps","readonly":{"rawvalue":"off","parsed":false},"mounted":true}]`,
		},
		"/api/v2.0/disk": {
			body: `[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"},{"identifier":"{disk-2}","name":"nvme0n1","serial":"SER-B","size":2000000,"model":"Samsung","type":"SSD","pool":"tank","bus":"NVMe","rotationrate":0,"status":"ONLINE"}]`,
		},
		"/api/v2.0/disk/temperatures": {
			body: `{"sda":34,"nvme0n1":"49","SER-B":51}`,
		},
		"/api/v2.0/alert/list": {
			body: `[{"id":"a1","level":"WARNING","formatted":"Disk temp high","source":"DiskService","dismissed":false,"datetime":{"$date":1707400000000}}]`,
		},
		"/api/v2.0/app": {
			body: `[{"id":"nextcloud","name":"Nextcloud","state":"RUNNING","version":"1.0.3","human_version":"29.0.7","upgrade_available":true,"image_updates_available":true,"notes":"Team cloud","active_workloads":{"containers":2,"used_host_ips":["0.0.0.0"],"used_ports":[{"container_port":443,"protocol":"tcp","host_ports":[{"host_port":30443,"host_ip":"0.0.0.0"}]}],"container_details":[{"id":"nextcloud-web-1","service_name":"nextcloud","image":"docker.io/library/nextcloud:29.0.7","state":"running","port_config":[{"container_port":443,"protocol":"tcp","host_ports":[{"host_port":30443,"host_ip":"0.0.0.0"}]}],"volume_mounts":[{"source":"/mnt/tank/apps/nextcloud","destination":"/var/www/html","mode":"rw","type":"bind"}]},{"id":"nextcloud-redis-1","service_name":"redis","image":"docker.io/library/redis:7.2","state":"running","port_config":[],"volume_mounts":[{"source":"ix-nextcloud-redis","destination":"/data","mode":"rw","type":"volume"}]}],"volumes":[{"source":"/mnt/tank/apps/nextcloud","destination":"/var/www/html","mode":"rw","type":"bind"},{"source":"ix-nextcloud-redis","destination":"/data","mode":"rw","type":"volume"}],"images":["docker.io/library/nextcloud:29.0.7","docker.io/library/redis:7.2"],"networks":[{"name":"ix-nextcloud_default","id":"net-1","labels":{"com.docker.compose.project":"nextcloud"}}]}}]`,
		},
	}
}

func newMockServer(t *testing.T, responses map[string]apiResponse, assertRequest func(*testing.T, *http.Request)) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if assertRequest != nil {
			assertRequest(t, request)
		}

		response, ok := responses[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}

		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		contentType := response.contentType
		if contentType == "" {
			contentType = "application/json"
		}

		writer.Header().Set("Content-Type", contentType)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(response.body))
	}))
}

func newMockServerWithRPC(t *testing.T, responses map[string]apiResponse, assertRequest func(*testing.T, *http.Request), handleRPC func(*testing.T, *websocket.Conn)) *httptest.Server {
	t.Helper()

	upgrader := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/current" && websocket.IsWebSocketUpgrade(request) {
			conn, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				t.Fatalf("upgrade websocket: %v", err)
			}
			defer func() { _ = conn.Close() }()
			handleRPC(t, conn)
			return
		}

		if assertRequest != nil {
			assertRequest(t, request)
		}

		response, ok := responses[request.URL.Path]
		if !ok {
			http.NotFound(writer, request)
			return
		}

		status := response.status
		if status == 0 {
			status = http.StatusOK
		}
		contentType := response.contentType
		if contentType == "" {
			contentType = "application/json"
		}

		writer.Header().Set("Content-Type", contentType)
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(response.body))
	}))
}

func readRPCRequest(t *testing.T, conn *websocket.Conn) trueNASRPCRequest {
	t.Helper()

	var request trueNASRPCRequest
	if err := conn.ReadJSON(&request); err != nil {
		t.Fatalf("ReadJSON() rpc request error = %v", err)
	}
	return request
}

func writeRPCResult(t *testing.T, conn *websocket.Conn, id int64, result any) {
	t.Helper()

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal() rpc result error = %v", err)
	}
	if err := conn.WriteJSON(trueNASRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  raw,
	}); err != nil {
		t.Fatalf("WriteJSON() rpc result error = %v", err)
	}
}

func writeRPCError(t *testing.T, conn *websocket.Conn, id int64, code int, message string) {
	t.Helper()

	if err := conn.WriteJSON(trueNASRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &trueNASRPCError{
			Code:    code,
			Message: message,
		},
	}); err != nil {
		t.Fatalf("WriteJSON() rpc error response = %v", err)
	}
}

func writeRPCNotification(t *testing.T, conn *websocket.Conn, method string, params any) {
	t.Helper()

	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal() rpc notification params error = %v", err)
	}
	if err := conn.WriteJSON(trueNASRPCResponse{
		JSONRPC: "2.0",
		Method:  method,
		Params:  raw,
	}); err != nil {
		t.Fatalf("WriteJSON() rpc notification error = %v", err)
	}
}

func mustClientForServer(t *testing.T, serverURL string, config ClientConfig) *Client {
	t.Helper()

	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatalf("failed to parse server URL %q: %v", serverURL, err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("failed to parse server port from %q: %v", serverURL, err)
	}

	config.Host = parsed.Hostname()
	config.Port = port
	config.UseHTTPS = parsed.Scheme == "https"

	client, err := NewClient(config)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func withFingerprintColons(value string) string {
	if len(value)%2 != 0 {
		return value
	}
	parts := make([]string, 0, len(value)/2)
	for i := 0; i < len(value); i += 2 {
		parts = append(parts, value[i:i+2])
	}
	return strings.Join(parts, ":")
}
