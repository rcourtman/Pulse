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

	shares, err := client.GetNetworkShares(ctx)
	if err != nil {
		t.Fatalf("GetNetworkShares() error = %v", err)
	}
	if len(shares) != 2 {
		t.Fatalf("expected 2 network shares, got %d", len(shares))
	}
	if shares[0].Name != "Media" || shares[0].Protocol != "SMB" || shares[0].Dataset != "tank/media" || !shares[0].AuditEnabled {
		t.Fatalf("unexpected SMB share mapping: %+v", shares[0])
	}
	if shares[1].Name != "tank/projects" || shares[1].Protocol != "NFS" || !shares[1].ReadOnly || len(shares[1].Networks) != 1 {
		t.Fatalf("unexpected NFS share mapping: %+v", shares[1])
	}
}

func TestClientAuthHeaderAPIKey(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"TrueNAS-SCALE-24.10.2","buildtime":"b","uptime_seconds":1}`},
	}, func(t *testing.T, request *http.Request) {
		if request.URL.Path == "/api/current" {
			return
		}
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
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"TrueNAS-SCALE-24.10.2","buildtime":"b","uptime_seconds":1}`},
	}, func(t *testing.T, request *http.Request) {
		if request.URL.Path == "/api/current" {
			return
		}
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

func TestGetSystemInfoAcceptsStructuredBuildTime(t *testing.T) {
	var response systemInfoResponse
	if err := json.Unmarshal([]byte(`{"hostname":"nas","version":"TrueNAS-SCALE-25.10.3.1","buildtime":{"$date":"2026-05-14T18:24:01+02:00"},"uptime_seconds":1}`), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	system := systemInfoFromResponse(response)
	if system.Build != "2026-05-14T18:24:01+02:00" {
		t.Fatalf("Build = %q, want structured buildtime date", system.Build)
	}
}

func TestGetSystemInfoAcceptsFractionalUptimeSeconds(t *testing.T) {
	var response systemInfoResponse
	if err := json.Unmarshal([]byte(`{"hostname":"nas","version":"TrueNAS-SCALE-25.10.3.1","buildtime":{"$date":"2026-05-14T18:24:01+02:00"},"uptime_seconds":360144.629139547}`), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	system := systemInfoFromResponse(response)
	if system.UptimeSeconds != 360144 {
		t.Fatalf("UptimeSeconds = %d, want truncated fractional uptime", system.UptimeSeconds)
	}
}

func TestTestConnectionSuccessAndFailure(t *testing.T) {
	successServer := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/system/info": {body: `{"hostname":"nas","version":"TrueNAS-SCALE-24.10.2","buildtime":"b","uptime_seconds":1}`},
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
	if len(snapshot.Pools) != 1 || len(snapshot.Datasets) != 1 || len(snapshot.Disks) != 2 || len(snapshot.Alerts) != 1 || len(snapshot.Services) != 2 || len(snapshot.Apps) != 1 || len(snapshot.VMs) != 1 || len(snapshot.Shares) != 2 {
		t.Fatalf("unexpected snapshot counts: pools=%d datasets=%d disks=%d alerts=%d services=%d apps=%d vms=%d shares=%d",
			len(snapshot.Pools), len(snapshot.Datasets), len(snapshot.Disks), len(snapshot.Alerts), len(snapshot.Services), len(snapshot.Apps), len(snapshot.VMs), len(snapshot.Shares))
	}
	if snapshot.Disks[0].Temperature != 34 || snapshot.Disks[1].Temperature != 49 {
		t.Fatalf("unexpected snapshot disk temperatures: %+v", snapshot.Disks)
	}
	if snapshot.Apps[0].ID != "nextcloud" || snapshot.Apps[0].ContainerCount != 2 {
		t.Fatalf("unexpected snapshot apps: %+v", snapshot.Apps)
	}
	if snapshot.VMs[0].Name != "windows-lab" || snapshot.VMs[0].MemoryBytes != 8*1024*1024*1024 {
		t.Fatalf("unexpected snapshot vms: %+v", snapshot.VMs)
	}
	if snapshot.Services[0].Service != "smb" || !snapshot.Services[0].Enabled || snapshot.Services[0].State != "RUNNING" {
		t.Fatalf("unexpected snapshot services: %+v", snapshot.Services)
	}
}

func TestGetPoolsDatasetsAndAlertsUseNativeQueryShapes(t *testing.T) {
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/pool":         {status: http.StatusInternalServerError, body: `{"error":"legacy pool endpoint should not be used"}`},
		"/api/v2.0/pool/dataset": {status: http.StatusInternalServerError, body: `{"error":"legacy dataset endpoint should not be used"}`},
		"/api/v2.0/alert/list":   {status: http.StatusInternalServerError, body: `{"error":"legacy alert endpoint should not be used"}`},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		request := readRPCRequest(t, conn)
		switch request.Method {
		case "pool.query":
			assertQueryParams(t, request.Params, "pool.query")
			writeRPCResult(t, conn, request.ID, defaultRoutePayloadMaps(t, "/api/v2.0/pool"))
		case "pool.dataset.query":
			assertQueryParams(t, request.Params, "pool.dataset.query")
			writeRPCResult(t, conn, request.ID, defaultRoutePayloadMaps(t, "/api/v2.0/pool/dataset"))
		case "alert.list":
			params, ok := request.Params.([]any)
			if !ok || len(params) != 0 {
				t.Fatalf("expected alert.list to use no params, got %#v", request.Params)
			}
			writeRPCResult(t, conn, request.ID, defaultRoutePayloadMaps(t, "/api/v2.0/alert/list"))
		default:
			t.Fatalf("unexpected rpc method %q", request.Method)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	pools, err := client.GetPools(context.Background())
	if err != nil {
		t.Fatalf("GetPools() error = %v", err)
	}
	if len(pools) != 1 || pools[0].Name != "tank" || pools[0].UsedBytes != 400 {
		t.Fatalf("unexpected native pool mapping: %+v", pools)
	}

	datasets, err := client.GetDatasets(context.Background())
	if err != nil {
		t.Fatalf("GetDatasets() error = %v", err)
	}
	if len(datasets) != 1 || datasets[0].ID != "tank/apps" || datasets[0].Pool != "tank" || datasets[0].UsedBytes != 12345 {
		t.Fatalf("unexpected native dataset mapping: %+v", datasets)
	}

	alerts, err := client.GetAlerts(context.Background())
	if err != nil {
		t.Fatalf("GetAlerts() error = %v", err)
	}
	if len(alerts) != 1 || alerts[0].ID != "a1" || alerts[0].Level != "WARNING" {
		t.Fatalf("unexpected native alert mapping: %+v", alerts)
	}
}

func TestGetServicesUsesNativeServiceQueryShape(t *testing.T) {
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/service": {status: http.StatusInternalServerError, body: `{"error":"legacy service endpoint should not be used"}`},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		queryReq := readRPCRequest(t, conn)
		if queryReq.Method != "service.query" {
			t.Fatalf("expected service.query, got %q", queryReq.Method)
		}
		params, ok := queryReq.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("expected service.query filters/options params, got %#v", queryReq.Params)
		}
		options, ok := params[1].(map[string]any)
		if !ok {
			t.Fatalf("expected service.query options, got %#v", params[1])
		}
		extra, ok := options["extra"].(map[string]any)
		if !ok || extra["include_state"] != true {
			t.Fatalf("expected service.query include_state option, got %#v", options)
		}
		writeRPCResult(t, conn, queryReq.ID, []map[string]any{
			{"id": 1, "service": "smb", "enable": true, "state": "RUNNING", "pids": []int{2418, 2420}},
			{"id": 2, "service": "ssh", "enable": false, "state": "STOPPED", "pids": []any{}},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	services, err := client.GetServices(context.Background())
	if err != nil {
		t.Fatalf("GetServices() error = %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].ID != "1" || services[0].Service != "smb" || !services[0].Enabled || services[0].State != "RUNNING" || len(services[0].PIDs) != 2 {
		t.Fatalf("unexpected smb service mapping: %+v", services[0])
	}
	if services[1].Service != "ssh" || services[1].Enabled || services[1].State != "STOPPED" {
		t.Fatalf("unexpected ssh service mapping: %+v", services[1])
	}
}

func TestGetDisksUsesNativeDiskQueryShape(t *testing.T) {
	connectionCount := 0
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/disk": {status: http.StatusInternalServerError, body: `{"error":"legacy disk endpoint should not be used"}`},
		"/api/v2.0/disk/temperatures": {
			body: `{"sda":34}`,
		},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		connectionCount++
		request := readRPCRequest(t, conn)
		switch connectionCount {
		case 1:
			if request.Method != "disk.query" {
				t.Fatalf("expected disk.query, got %q", request.Method)
			}
			params, ok := request.Params.([]any)
			if !ok || len(params) != 2 {
				t.Fatalf("expected disk.query filters/options params, got %#v", request.Params)
			}
			payload := defaultRoutePayloadMaps(t, "/api/v2.0/disk")
			payload[0]["smart_status"] = "FAILED"
			writeRPCResult(t, conn, request.ID, payload[:1])
		case 2:
			if request.Method != "reporting.get_data" {
				t.Fatalf("expected reporting.get_data, got %q", request.Method)
			}
			writeRPCResult(t, conn, request.ID, []map[string]any{{
				"name":       "disktemp",
				"identifier": "sda",
				"legend":     []string{"time", "temperature"},
				"aggregations": map[string]any{
					"mean": map[string]any{"temperature": 34.0},
				},
			}})
		case 3:
			if request.Method != "disk.temperature_agg" {
				t.Fatalf("expected disk.temperature_agg, got %q", request.Method)
			}
			writeRPCResult(t, conn, request.ID, map[string]any{
				"sda": map[string]any{
					"min":         29.0,
					"avg":         32.8,
					"max":         38.0,
					"window_days": 7,
				},
			})
		default:
			t.Fatalf("unexpected extra websocket connection %d", connectionCount)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	disks, err := client.GetDisks(context.Background())
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
	if disks[0].Name != "sda" || disks[0].Temperature != 34 || disks[0].TemperatureAggregate.MaxCelsius != 38 || disks[0].Health != "FAILED" || !disks[0].HealthStatusPresent {
		t.Fatalf("unexpected native disk mapping: %+v", disks[0])
	}
}

func TestGetDisksParsesRESTSmartStatus(t *testing.T) {
	server := newMockServer(t, map[string]apiResponse{
		"/api/v2.0/disk": {
			body: `[
				{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE","smart_status":null},
				{"identifier":"{disk-2}","name":"sdb","serial":"SER-B","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE","smart_status":""},
				{"identifier":"{disk-3}","name":"sdc","serial":"SER-C","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE","smart_status":"FAILED"},
				{"identifier":"{disk-4}","name":"sdd","serial":"SER-D","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"}
			]`,
		},
		"/api/v2.0/disk/temperatures": {
			body: `{}`,
		},
	}, nil)
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	disks, err := client.GetDisks(context.Background())
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	byName := make(map[string]Disk, len(disks))
	for _, disk := range disks {
		byName[disk.Name] = disk
	}
	tests := []struct {
		name        string
		wantHealth  string
		wantPresent bool
	}{
		{name: "sda", wantHealth: "UNKNOWN", wantPresent: true},
		{name: "sdb", wantHealth: "UNKNOWN", wantPresent: true},
		{name: "sdc", wantHealth: "FAILED", wantPresent: true},
		{name: "sdd", wantHealth: "", wantPresent: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disk, ok := byName[tt.name]
			if !ok {
				t.Fatalf("missing disk %q in %+v", tt.name, disks)
			}
			if disk.Health != tt.wantHealth || disk.HealthStatusPresent != tt.wantPresent {
				t.Fatalf("disk %s health=(%q,%v), want (%q,%v)", tt.name, disk.Health, disk.HealthStatusPresent, tt.wantHealth, tt.wantPresent)
			}
		})
	}
}

func TestGetVMsParsesNativeVMQueryShape(t *testing.T) {
	server := newMockServerWithRPC(t, map[string]apiResponse{}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		queryReq := readRPCRequest(t, conn)
		if queryReq.Method != "vm.query" {
			t.Fatalf("expected vm.query, got %q", queryReq.Method)
		}
		writeRPCResult(t, conn, queryReq.ID, []map[string]any{
			{
				"id":                      42,
				"name":                    "windows-lab",
				"description":             "Build test box",
				"vcpus":                   4,
				"cores":                   2,
				"threads":                 2,
				"memory":                  8192,
				"min_memory":              4096,
				"cpu_mode":                "HOST-PASSTHROUGH",
				"cpu_model":               nil,
				"bootloader":              "UEFI",
				"autostart":               true,
				"suspend_on_snapshot":     true,
				"trusted_platform_module": true,
				"enable_secure_boot":      true,
				"time":                    "UTC",
				"arch_type":               "x86_64",
				"machine_type":            "q35",
				"uuid":                    "vm-uuid-1",
				"display_available":       true,
				"status": map[string]any{
					"state":        "RUNNING",
					"pid":          1234,
					"domain_state": "RUNNING",
				},
				"devices": []map[string]any{
					{"id": 1, "attributes": map[string]any{"dtype": "DISK"}},
					{"id": 2, "attributes": map[string]any{"dtype": "NIC"}},
					{"id": 3, "attributes": map[string]any{"dtype": "DISPLAY"}},
					{"id": 4, "attributes": map[string]any{"dtype": "CDROM"}},
				},
			},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	vms, err := client.GetVMs(context.Background())
	if err != nil {
		t.Fatalf("GetVMs() error = %v", err)
	}
	if len(vms) != 1 {
		t.Fatalf("expected 1 VM, got %d", len(vms))
	}
	vm := vms[0]
	if vm.ID != "42" || vm.Name != "windows-lab" || vm.State != "RUNNING" || vm.DomainState != "RUNNING" {
		t.Fatalf("unexpected VM identity/status: %+v", vm)
	}
	if vm.VCPUs != 4 || vm.Cores != 2 || vm.Threads != 2 {
		t.Fatalf("unexpected VM CPU topology: %+v", vm)
	}
	if vm.MemoryBytes != 8*1024*1024*1024 || vm.MinMemoryBytes != 4*1024*1024*1024 {
		t.Fatalf("unexpected VM memory: %+v", vm)
	}
	if !vm.Autostart || !vm.SuspendOnSnapshot || !vm.TrustedPlatformModule || !vm.SecureBoot {
		t.Fatalf("expected VM boolean flags, got %+v", vm)
	}
	if vm.DeviceCount != 4 || vm.DiskCount != 1 || vm.NICCount != 1 || vm.DisplayCount != 1 || vm.CDROMCount != 1 {
		t.Fatalf("unexpected VM device counts: %+v", vm)
	}
}

func TestGetAppsParsesNativeAppQueryShape(t *testing.T) {
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/app": {status: http.StatusInternalServerError, body: `{"error":"legacy app endpoint should not be used"}`},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		request := readRPCRequest(t, conn)
		switch request.Method {
		case "app.query":
			params, ok := request.Params.([]any)
			if !ok || len(params) != 2 {
				t.Fatalf("expected app.query filters/options params, got %#v", request.Params)
			}
			writeRPCResult(t, conn, request.ID, defaultAppQueryPayload(t))
		case "core.subscribe":
			writeRPCError(t, conn, request.ID, 1, "stats unavailable")
		default:
			t.Fatalf("unexpected rpc method %q", request.Method)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	apps, err := client.GetApps(context.Background())
	if err != nil {
		t.Fatalf("GetApps() error = %v", err)
	}
	if len(apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(apps))
	}
	app := apps[0]
	if app.ID != "nextcloud" || app.Name != "Nextcloud" || app.State != "RUNNING" {
		t.Fatalf("unexpected app identity/status: %+v", app)
	}
	if app.ContainerCount != 2 || len(app.Containers) != 2 {
		t.Fatalf("unexpected app container mapping: %+v", app)
	}
	if len(app.UsedPorts) != 1 || len(app.UsedPorts[0].HostPorts) != 1 || app.UsedPorts[0].HostPorts[0].HostPort != 30443 {
		t.Fatalf("unexpected app port mapping: %+v", app.UsedPorts)
	}
	if len(app.Volumes) != 2 || len(app.Images) != 2 || len(app.Networks) != 1 {
		t.Fatalf("unexpected app runtime shape: volumes=%d images=%d networks=%d", len(app.Volumes), len(app.Images), len(app.Networks))
	}
}

func TestGetAppsEnrichesStatsFromRPC(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		request := readRPCRequest(t, conn)
		switch request.Method {
		case "app.query":
			writeRPCResult(t, conn, request.ID, defaultAppQueryPayload(t))
			return
		case "core.subscribe":
		default:
			t.Fatalf("expected app.query or core.subscribe, got %q", request.Method)
		}
		writeRPCResult(t, conn, request.ID, "sub-1")
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
		expectRPCUnsubscribe(t, conn, "sub-1")
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

func TestGetZFSSnapshotsUsesNativeSnapshotQueryShape(t *testing.T) {
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/zfs/snapshot": {status: http.StatusInternalServerError, body: `{"error":"legacy snapshot endpoint should not be used"}`},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		request := readRPCRequest(t, conn)
		if request.Method != "zfs.resource.snapshot.query" {
			t.Fatalf("expected zfs.resource.snapshot.query, got %q", request.Method)
		}
		params, ok := request.Params.([]any)
		if !ok || len(params) != 1 {
			t.Fatalf("unexpected snapshot query params: %#v", request.Params)
		}
		writeRPCResult(t, conn, request.ID, []map[string]any{{
			"name":          "tank/apps@auto-20260331-0600",
			"dataset":       "tank/apps",
			"snapshot_name": "auto-20260331-0600",
			"properties": map[string]any{
				"creation":   map[string]any{"raw": "1710000000"},
				"used":       map[string]any{"raw": "1024"},
				"referenced": map[string]any{"raw": "2048"},
			},
		}})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	snapshots, err := client.GetZFSSnapshots(context.Background())
	if err != nil {
		t.Fatalf("GetZFSSnapshots() error = %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	snapshot := snapshots[0]
	if snapshot.FullName != "tank/apps@auto-20260331-0600" || snapshot.Dataset != "tank/apps" || snapshot.Name != "auto-20260331-0600" {
		t.Fatalf("unexpected snapshot identity: %+v", snapshot)
	}
	if snapshot.CreatedAt == nil || !snapshot.CreatedAt.Equal(time.Unix(1710000000, 0).UTC()) {
		t.Fatalf("unexpected snapshot creation time: %+v", snapshot.CreatedAt)
	}
	if snapshot.UsedBytes == nil || *snapshot.UsedBytes != 1024 || snapshot.Referenced == nil || *snapshot.Referenced != 2048 {
		t.Fatalf("unexpected snapshot byte properties: %+v", snapshot)
	}
}

func TestGetReplicationTasksUsesNativeReplicationQueryShape(t *testing.T) {
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/replication": {status: http.StatusInternalServerError, body: `{"error":"legacy replication endpoint should not be used"}`},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		request := readRPCRequest(t, conn)
		if request.Method != "replication.query" {
			t.Fatalf("expected replication.query, got %q", request.Method)
		}
		params, ok := request.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("expected replication.query filters/options params, got %#v", request.Params)
		}
		writeRPCResult(t, conn, request.ID, []map[string]any{{
			"id":              7,
			"name":            "Offsite apps",
			"direction":       "PUSH",
			"transport":       "SSH",
			"readonly":        "SET",
			"source_datasets": []string{"tank/apps"},
			"target_dataset":  "backup/apps",
			"ssh_credentials": map[string]any{
				"id":   3,
				"name": "backup-nas",
				"attributes": map[string]any{
					"host": "backup.example.test",
					"port": 22,
				},
			},
			"state": map[string]any{
				"state":         "SUCCESS",
				"datetime":      "2026-03-31T06:30:00Z",
				"last_snapshot": "tank/apps@auto-20260331-0600",
			},
		}})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	tasks, err := client.GetReplicationTasks(context.Background())
	if err != nil {
		t.Fatalf("GetReplicationTasks() error = %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	task := tasks[0]
	if task.ID != "7" || task.Name != "Offsite apps" || task.Direction != "PUSH" {
		t.Fatalf("unexpected replication identity: %+v", task)
	}
	if len(task.SourceDatasets) != 1 || task.SourceDatasets[0] != "tank/apps" || task.TargetDataset != "backup/apps" {
		t.Fatalf("unexpected replication datasets: %+v", task)
	}
	if task.Transport != "SSH" || task.ReadOnlyMode != "SET" || task.TargetHost != "backup.example.test" {
		t.Fatalf("unexpected replication target posture: %+v", task)
	}
	if task.LastState != "SUCCESS" || task.LastSnapshot != "tank/apps@auto-20260331-0600" {
		t.Fatalf("unexpected replication state: %+v", task)
	}
	if task.LastRun == nil || task.LastRun.Format(time.RFC3339) != "2026-03-31T06:30:00Z" {
		t.Fatalf("unexpected replication last run: %+v", task.LastRun)
	}
}

func TestStartAndStopAppUseRPCMethods(t *testing.T) {
	var rpcSessions int
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		rpcSessions++
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		for _, expectedMethod := range []string{"app.start", "app.stop"} {
			actionReq := readRPCRequest(t, conn)
			if actionReq.Method != expectedMethod {
				t.Fatalf("expected %s, got %q", expectedMethod, actionReq.Method)
			}
			writeRPCResult(t, conn, actionReq.ID, true)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	if err := client.StartApp(context.Background(), "nextcloud"); err != nil {
		t.Fatalf("StartApp() error = %v", err)
	}
	if err := client.StopApp(context.Background(), "nextcloud"); err != nil {
		t.Fatalf("StopApp() error = %v", err)
	}
	if rpcSessions != 1 {
		t.Fatalf("expected one persistent RPC app-action session, got %d", rpcSessions)
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
		expectRPCUnsubscribe(t, conn, "sub-logs")
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
		expectRPCUnsubscribe(t, conn, "sub-1")
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

func TestGetSystemMetricHistoryUsesReportingRPC(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		historyReq := readRPCRequest(t, conn)
		if historyReq.Method != "reporting.get_data" {
			t.Fatalf("expected reporting.get_data, got %q", historyReq.Method)
		}
		params, ok := historyReq.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("unexpected history params: %#v", historyReq.Params)
		}
		graphs, ok := params[0].([]any)
		if !ok || len(graphs) != 4 {
			t.Fatalf("unexpected history graphs: %#v", params[0])
		}
		query, ok := params[1].(map[string]any)
		if !ok || query["aggregate"] != false {
			t.Fatalf("expected aggregate=false history query, got %#v", params[1])
		}

		now := time.Now().UTC().Truncate(time.Second)
		writeRPCResult(t, conn, historyReq.ID, []map[string]any{
			{
				"name":       "cpu",
				"identifier": nil,
				"legend":     []string{"usage"},
				"data": []any{
					map[string]any{"timestamp": now.Add(-2 * time.Hour).Unix(), "usage": 21.0},
					map[string]any{"timestamp": now.Unix(), "usage": 34.0},
				},
				"aggregations": map[string]any{},
				"start":        now.Add(-2 * time.Hour).Unix(),
				"end":          now.Unix(),
			},
			{
				"name":       "memory",
				"identifier": nil,
				"legend":     []string{"used", "total"},
				"data": []any{
					map[string]any{"timestamp": now.Add(-2 * time.Hour).Unix(), "used": 8.0 * 1024 * 1024 * 1024, "total": 16.0 * 1024 * 1024 * 1024},
					map[string]any{"timestamp": now.Unix(), "used": 10.0 * 1024 * 1024 * 1024, "total": 16.0 * 1024 * 1024 * 1024},
				},
				"aggregations": map[string]any{},
				"start":        now.Add(-2 * time.Hour).Unix(),
				"end":          now.Unix(),
			},
			{
				"name":       "interface",
				"identifier": nil,
				"legend":     []string{"received", "sent"},
				"data": []any{
					map[string]any{"timestamp": now.Add(-2 * time.Hour).Unix(), "received": 1024.0, "sent": 512.0},
					map[string]any{"timestamp": now.Unix(), "received": 4096.0, "sent": 2048.0},
				},
				"aggregations": map[string]any{},
				"start":        now.Add(-2 * time.Hour).Unix(),
				"end":          now.Unix(),
			},
			{
				"name":       "disk",
				"identifier": nil,
				"legend":     []string{"read", "write"},
				"data": []any{
					map[string]any{"timestamp": now.Add(-2 * time.Hour).Unix(), "read": 2048.0, "write": 1024.0},
					map[string]any{"timestamp": now.Unix(), "read": 8192.0, "write": 4096.0},
				},
				"aggregations": map[string]any{},
				"start":        now.Add(-2 * time.Hour).Unix(),
				"end":          now.Unix(),
			},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	history, err := client.GetSystemMetricHistory(context.Background(), 4*time.Hour)
	if err != nil {
		t.Fatalf("GetSystemMetricHistory() error = %v", err)
	}
	if history == nil {
		t.Fatal("expected system metric history")
	}
	if got := len(history.CPUPercent); got != 2 {
		t.Fatalf("expected cpu history, got %+v", history)
	}
	if got := len(history.MemoryPercent); got != 2 {
		t.Fatalf("expected memory percent history, got %+v", history)
	}
	if got := history.MemoryPercent[1].Value; got <= 0 {
		t.Fatalf("expected non-zero memory percent, got %+v", history.MemoryPercent)
	}
	if got := history.NetInRate[1].Value; got != 4096.0 {
		t.Fatalf("expected network history, got %+v", history.NetInRate)
	}
	if got := history.DiskWriteRate[1].Value; got != 4096.0 {
		t.Fatalf("expected disk history, got %+v", history.DiskWriteRate)
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
		expectRPCUnsubscribe(t, conn, "sub-1")
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

func TestGetDiskTemperaturesFallsBackToReportingRPC(t *testing.T) {
	connectionCount := 0
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/disk": {
			body: `[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"}]`,
		},
		"/api/v2.0/disk/temperatures": {
			status: http.StatusNotFound,
			body:   `{"error":"not found"}`,
		},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		connectionCount++
		temperatureReq := readRPCRequest(t, conn)
		if connectionCount == 1 {
			if temperatureReq.Method != "disk.query" {
				t.Fatalf("expected disk.query, got %q", temperatureReq.Method)
			}
			writeRPCResult(t, conn, temperatureReq.ID, defaultRoutePayloadMaps(t, "/api/v2.0/disk")[:1])
			return
		}
		if temperatureReq.Method != "reporting.get_data" {
			t.Fatalf("expected reporting.get_data, got %q", temperatureReq.Method)
		}
		params, ok := temperatureReq.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("unexpected reporting params: %#v", temperatureReq.Params)
		}
		graphs, ok := params[0].([]any)
		if !ok || len(graphs) != 1 {
			t.Fatalf("unexpected reporting graphs: %#v", params[0])
		}
		graph, ok := graphs[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected reporting graph entry: %#v", graphs[0])
		}
		if got := readStringAny(graph, "name"); got != "disktemp" {
			t.Fatalf("expected disktemp graph, got %q", got)
		}
		if got := readStringAny(graph, "identifier"); got != "sda" {
			t.Fatalf("expected sda identifier, got %q", got)
		}
		writeRPCResult(t, conn, temperatureReq.ID, []map[string]any{{
			"name":       "disktemp",
			"identifier": "sda",
			"legend":     []string{"temperature"},
			"aggregations": map[string]any{
				"mean": map[string]any{
					"temperature": 41.8,
				},
			},
			"data":  []any{},
			"start": time.Now().Add(-5 * time.Minute).Unix(),
			"end":   time.Now().Unix(),
		}})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	temperatures, err := client.GetDiskTemperatures(context.Background())
	if err != nil {
		t.Fatalf("GetDiskTemperatures() error = %v", err)
	}
	if got := temperatures["sda"]; got != 42 {
		t.Fatalf("expected sda reporting fallback temperature 42, got %d", got)
	}
}

func TestGetDisksFallsBackToReportingRPCWhenTemperatureEndpointUnavailable(t *testing.T) {
	connectionCount := 0
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/disk": {
			body: `[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"}]`,
		},
		"/api/v2.0/disk/temperatures": {
			status: http.StatusNotFound,
			body:   `{"error":"not found"}`,
		},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		connectionCount++
		request := readRPCRequest(t, conn)
		switch connectionCount {
		case 1:
			if request.Method != "disk.query" {
				t.Fatalf("expected disk.query, got %q", request.Method)
			}
			writeRPCResult(t, conn, request.ID, defaultRoutePayloadMaps(t, "/api/v2.0/disk")[:1])
		case 2:
			if request.Method != "reporting.get_data" {
				t.Fatalf("expected reporting.get_data, got %q", request.Method)
			}
			writeRPCResult(t, conn, request.ID, []map[string]any{{
				"name":       "disktemp",
				"identifier": "sda",
				"legend":     []string{"temperature"},
				"aggregations": map[string]any{
					"mean": map[string]any{
						"temperature": 43.2,
					},
				},
				"data":  []any{},
				"start": time.Now().Add(-5 * time.Minute).Unix(),
				"end":   time.Now().Unix(),
			}})
		case 3:
			if request.Method != "disk.temperature_agg" {
				t.Fatalf("expected disk.temperature_agg, got %q", request.Method)
			}
			writeRPCResult(t, conn, request.ID, map[string]any{
				"sda": map[string]any{
					"min":         39.0,
					"avg":         41.6,
					"max":         45.0,
					"window_days": 7,
				},
			})
		default:
			t.Fatalf("unexpected extra websocket connection %d", connectionCount)
		}
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	disks, err := client.GetDisks(context.Background())
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
	if got := disks[0].Temperature; got != 43 {
		t.Fatalf("expected reporting fallback temperature 43, got %+v", disks[0])
	}
	if got := disks[0].TemperatureAggregate.MaxCelsius; got != 45.0 {
		t.Fatalf("expected aggregate max 45.0, got %+v", disks[0].TemperatureAggregate)
	}
}

func TestGetDisksIncludesDiskTemperatureAggregatesFromRPC(t *testing.T) {
	connectionCount := 0
	server := newMockServerWithRPC(t, map[string]apiResponse{
		"/api/v2.0/disk": {
			body: `[{"identifier":"{disk-1}","name":"sda","serial":"SER-A","size":1000000,"model":"Seagate","type":"HDD","pool":"tank","bus":"SATA","rotationrate":7200,"status":"ONLINE"}]`,
		},
		"/api/v2.0/disk/temperatures": {
			body: `{"sda":34}`,
		},
	}, nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		connectionCount++
		aggregateReq := readRPCRequest(t, conn)
		if connectionCount == 1 {
			if aggregateReq.Method != "disk.query" {
				t.Fatalf("expected disk.query, got %q", aggregateReq.Method)
			}
			writeRPCResult(t, conn, aggregateReq.ID, defaultRoutePayloadMaps(t, "/api/v2.0/disk")[:1])
			return
		}
		if aggregateReq.Method == "reporting.get_data" {
			// Current-temperature reporting probe: answer empty so the
			// client falls back to the REST POST /disk/temperatures route.
			writeRPCResult(t, conn, aggregateReq.ID, []map[string]any{})
			return
		}
		if aggregateReq.Method != "disk.temperature_agg" {
			t.Fatalf("expected disk.temperature_agg, got %q", aggregateReq.Method)
		}
		params, ok := aggregateReq.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("unexpected aggregate params: %#v", aggregateReq.Params)
		}
		identifiers, ok := params[0].([]any)
		if !ok || len(identifiers) != 1 {
			t.Fatalf("unexpected aggregate identifiers: %#v", params[0])
		}
		if got := strings.TrimSpace(fmt.Sprint(identifiers[0])); got != "sda" {
			t.Fatalf("expected sda identifier, got %q", got)
		}
		if got := int(readFloatAny(map[string]any{"value": params[1]}, "value")); got != defaultDiskTemperatureAggregateWindowDays {
			t.Fatalf("expected window %d, got %#v", defaultDiskTemperatureAggregateWindowDays, params[1])
		}
		writeRPCResult(t, conn, aggregateReq.ID, map[string]any{
			"sda": map[string]any{
				"min":         29.0,
				"avg":         32.8,
				"max":         38.0,
				"window_days": 7,
			},
		})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	disks, err := client.GetDisks(context.Background())
	if err != nil {
		t.Fatalf("GetDisks() error = %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("expected 1 disk, got %d", len(disks))
	}
	if got := disks[0].TemperatureAggregate.WindowDays; got != 7 {
		t.Fatalf("expected aggregate window 7, got %+v", disks[0].TemperatureAggregate)
	}
	if got := disks[0].TemperatureAggregate.MinCelsius; got != 29.0 {
		t.Fatalf("expected aggregate min 29.0, got %+v", disks[0].TemperatureAggregate)
	}
	if got := disks[0].TemperatureAggregate.AvgCelsius; got != 32.8 {
		t.Fatalf("expected aggregate avg 32.8, got %+v", disks[0].TemperatureAggregate)
	}
	if got := disks[0].TemperatureAggregate.MaxCelsius; got != 38.0 {
		t.Fatalf("expected aggregate max 38.0, got %+v", disks[0].TemperatureAggregate)
	}
}

func TestGetDiskTemperatureHistoryUsesReportingRPC(t *testing.T) {
	server := newMockServerWithRPC(t, defaultAPIResponses(), nil, func(t *testing.T, conn *websocket.Conn) {
		authReq := readRPCRequest(t, conn)
		if authReq.Method != "auth.login_with_api_key" {
			t.Fatalf("expected api-key auth method, got %q", authReq.Method)
		}
		writeRPCResult(t, conn, authReq.ID, true)

		historyReq := readRPCRequest(t, conn)
		if historyReq.Method != "reporting.get_data" {
			t.Fatalf("expected reporting.get_data, got %q", historyReq.Method)
		}
		params, ok := historyReq.Params.([]any)
		if !ok || len(params) != 2 {
			t.Fatalf("unexpected history params: %#v", historyReq.Params)
		}
		graphs, ok := params[0].([]any)
		if !ok || len(graphs) != 1 {
			t.Fatalf("unexpected history graphs: %#v", params[0])
		}
		graph, ok := graphs[0].(map[string]any)
		if !ok {
			t.Fatalf("unexpected history graph entry: %#v", graphs[0])
		}
		if got := readStringAny(graph, "name"); got != "disktemp" {
			t.Fatalf("expected disktemp graph, got %q", got)
		}
		if got := readStringAny(graph, "identifier"); got != "sda" {
			t.Fatalf("expected sda identifier, got %q", got)
		}
		query, ok := params[1].(map[string]any)
		if !ok {
			t.Fatalf("unexpected history query: %#v", params[1])
		}
		if aggregate := query["aggregate"]; aggregate != false {
			t.Fatalf("expected aggregate=false for history query, got %#v", aggregate)
		}

		now := time.Now().UTC().Truncate(time.Second)
		writeRPCResult(t, conn, historyReq.ID, []map[string]any{{
			"name":       "disktemp",
			"identifier": "sda",
			"legend":     []string{"temperature"},
			"data": []any{
				[]any{now.Add(-2 * time.Hour).Unix(), 30.0},
				[]any{now.Add(-1 * time.Hour).Unix(), 31.5},
				[]any{now.Unix(), 33.0},
			},
			"aggregations": map[string]any{},
			"start":        now.Add(-2 * time.Hour).Unix(),
			"end":          now.Unix(),
		}})
	})
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{APIKey: "api-key"})
	history, err := client.GetDiskTemperatureHistory(context.Background(), []string{"sda"}, 4*time.Hour)
	if err != nil {
		t.Fatalf("GetDiskTemperatureHistory() error = %v", err)
	}
	points, ok := history["sda"]
	if !ok {
		t.Fatalf("expected history for sda, got %#v", history)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 history points, got %+v", points)
	}
	if points[0].Value != 30.0 || points[2].Value != 33.0 {
		t.Fatalf("unexpected history values: %+v", points)
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
		_, _ = writer.Write([]byte(`{"hostname":"nas","version":"TrueNAS-SCALE-24.10.2","buildtime":"b","uptime_seconds":1}`))
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

func defaultAppQueryPayload(t *testing.T) []map[string]any {
	t.Helper()

	return defaultRoutePayloadMaps(t, "/api/v2.0/app")
}

func defaultRoutePayloadMaps(t *testing.T, route string) []map[string]any {
	t.Helper()

	response, ok := defaultAPIResponses()[route]
	if !ok {
		t.Fatalf("missing default fixture route %q", route)
	}
	var payload []map[string]any
	if err := json.Unmarshal([]byte(response.body), &payload); err != nil {
		t.Fatalf("unmarshal default fixture route %q: %v", route, err)
	}
	return payload
}

func assertQueryParams(t *testing.T, params any, method string) {
	t.Helper()

	values, ok := params.([]any)
	if !ok || len(values) != 2 {
		t.Fatalf("expected %s filters/options params, got %#v", method, params)
	}
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
		"/api/v2.0/service": {
			body: `[{"id":1,"service":"smb","enable":true,"state":"RUNNING","pids":[2418,2420]},{"id":2,"service":"ssh","enable":false,"state":"STOPPED","pids":[]}]`,
		},
		"/api/v2.0/app": {
			body: `[{"id":"nextcloud","name":"Nextcloud","state":"RUNNING","version":"1.0.3","human_version":"29.0.7","upgrade_available":true,"image_updates_available":true,"notes":"Team cloud","active_workloads":{"containers":2,"used_host_ips":["0.0.0.0"],"used_ports":[{"container_port":443,"protocol":"tcp","host_ports":[{"host_port":30443,"host_ip":"0.0.0.0"}]}],"container_details":[{"id":"nextcloud-web-1","service_name":"nextcloud","image":"docker.io/library/nextcloud:29.0.7","state":"running","port_config":[{"container_port":443,"protocol":"tcp","host_ports":[{"host_port":30443,"host_ip":"0.0.0.0"}]}],"volume_mounts":[{"source":"/mnt/tank/apps/nextcloud","destination":"/var/www/html","mode":"rw","type":"bind"}]},{"id":"nextcloud-redis-1","service_name":"redis","image":"docker.io/library/redis:7.2","state":"running","port_config":[],"volume_mounts":[{"source":"ix-nextcloud-redis","destination":"/data","mode":"rw","type":"volume"}]}],"volumes":[{"source":"/mnt/tank/apps/nextcloud","destination":"/var/www/html","mode":"rw","type":"bind"},{"source":"ix-nextcloud-redis","destination":"/data","mode":"rw","type":"volume"}],"images":["docker.io/library/nextcloud:29.0.7","docker.io/library/redis:7.2"],"networks":[{"name":"ix-nextcloud_default","id":"net-1","labels":{"com.docker.compose.project":"nextcloud"}}]}}]`,
		},
		"/api/v2.0/vm": {
			body: `[{"id":42,"name":"windows-lab","vcpus":4,"cores":2,"threads":2,"memory":8192,"bootloader":"UEFI","autostart":true,"status":{"state":"RUNNING","pid":1234,"domain_state":"RUNNING"},"devices":[{"id":1,"attributes":{"dtype":"DISK"}},{"id":2,"attributes":{"dtype":"NIC"}}]}]`,
		},
		"/api/v2.0/sharing/smb": {
			body: `[{"id":1,"name":"Media","path":"/mnt/tank/media","dataset":"tank/media","enabled":true,"comment":"Household media","readonly":false,"browsable":true,"access_based_share_enumeration":true,"locked":false,"audit":{"enable":true}}]`,
		},
		"/api/v2.0/sharing/nfs": {
			body: `[{"id":2,"path":"/mnt/tank/projects","dataset":"tank/projects","comment":"Linux exports","enabled":true,"ro":true,"networks":["10.10.20.0/24"],"hosts":[],"security":["SYS"],"locked":false}]`,
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
			if request.URL.Path == "/api/v2.0/system/info" {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"hostname":"legacy-test","version":"TrueNAS-SCALE-24.10.2","buildtime":"24.10.2","uptime_seconds":1}`))
				return
			}
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

func expectRPCUnsubscribe(t *testing.T, conn *websocket.Conn, subscriptionID string) {
	t.Helper()

	request := readRPCRequest(t, conn)
	if request.Method != "core.unsubscribe" {
		t.Fatalf("expected core.unsubscribe, got %q", request.Method)
	}
	params, ok := request.Params.([]any)
	if !ok || len(params) != 1 || params[0] != subscriptionID {
		t.Fatalf("core.unsubscribe params = %#v, want [%q]", request.Params, subscriptionID)
	}
	writeRPCResult(t, conn, request.ID, nil)
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
	client.allowInsecureRPC = true
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
