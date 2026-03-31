package vmware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCollectInventoryEnrichesSignals(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{})
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	snapshot, err := client.CollectInventory(context.Background())
	if err != nil {
		t.Fatalf("CollectInventory: %v", err)
	}
	if snapshot == nil {
		t.Fatal("expected inventory snapshot")
	}
	if len(snapshot.Hosts) != 1 || len(snapshot.VMs) != 1 || len(snapshot.Datastores) != 1 {
		t.Fatalf("unexpected inventory sizes: hosts=%d vms=%d datastores=%d", len(snapshot.Hosts), len(snapshot.VMs), len(snapshot.Datastores))
	}

	host := snapshot.Hosts[0]
	if host.OverallStatus != "yellow" {
		t.Fatalf("host overall status = %q, want yellow", host.OverallStatus)
	}
	if host.ClusterName != "Prod Compute" || host.DatacenterName != "DC1" {
		t.Fatalf("expected host placement enrichment, got cluster=%q datacenter=%q", host.ClusterName, host.DatacenterName)
	}
	if len(host.DatastoreNames) != 1 || host.DatastoreNames[0] != "nvme-primary" {
		t.Fatalf("expected host datastore names, got %+v", host.DatastoreNames)
	}
	if host.Metrics == nil || host.Metrics.CPUPercent == nil || *host.Metrics.CPUPercent != 21.4 {
		t.Fatalf("expected host cpu metrics, got %+v", host.Metrics)
	}
	if len(host.TriggeredAlarms) != 1 || host.TriggeredAlarms[0].Name != "Host connection degraded" {
		t.Fatalf("expected resolved host alarm metadata, got %+v", host.TriggeredAlarms)
	}
	if len(host.RecentTasks) != 1 || host.RecentTasks[0].Name != "Reconnect host" {
		t.Fatalf("expected host recent task summary, got %+v", host.RecentTasks)
	}
	if len(host.RecentEvents) != 1 || host.RecentEvents[0].Type != "HostConnectionStateEvent" {
		t.Fatalf("expected host recent event summary, got %+v", host.RecentEvents)
	}

	vm := snapshot.VMs[0]
	if vm.Metrics == nil || vm.Metrics.MemoryPercent == nil || *vm.Metrics.MemoryPercent != 57.5 {
		t.Fatalf("expected VM memory metrics, got %+v", vm.Metrics)
	}
	if len(vm.TriggeredAlarms) != 1 || vm.TriggeredAlarms[0].OverallStatus != "red" {
		t.Fatalf("expected VM alarm signals, got %+v", vm.TriggeredAlarms)
	}
	if vm.RuntimeHostName != "esxi-01.lab.local" || vm.ResourcePoolName != "Tier 1" {
		t.Fatalf("expected VM placement enrichment, got host=%q pool=%q", vm.RuntimeHostName, vm.ResourcePoolName)
	}
	if vm.InstanceUUID != "vm-instance-201" || vm.BIOSUUID != "vm-bios-201" {
		t.Fatalf("expected VM identity enrichment, got instance=%q bios=%q", vm.InstanceUUID, vm.BIOSUUID)
	}
	if vm.GuestHostname != "app-01.internal" || len(vm.GuestIPAddresses) != 1 || vm.GuestIPAddresses[0] != "10.0.0.21" {
		t.Fatalf("expected VM guest identity enrichment, got host=%q ips=%v", vm.GuestHostname, vm.GuestIPAddresses)
	}
	if vm.SnapshotCount != 2 {
		t.Fatalf("vm snapshot count = %d, want 2", vm.SnapshotCount)
	}
	if len(vm.RecentTasks) != 1 || vm.RecentTasks[0].State != "success" {
		t.Fatalf("expected VM recent task info, got %+v", vm.RecentTasks)
	}
	if len(vm.RecentEvents) != 1 || vm.RecentEvents[0].Type != "VmMessageEvent" {
		t.Fatalf("expected VM recent event info, got %+v", vm.RecentEvents)
	}

	datastore := snapshot.Datastores[0]
	if datastore.OverallStatus != "yellow" {
		t.Fatalf("datastore overall status = %q, want yellow", datastore.OverallStatus)
	}
	if datastore.Accessible == nil || !*datastore.Accessible {
		t.Fatalf("expected datastore accessibility enrichment, got %+v", datastore.Accessible)
	}
	if len(datastore.HostNames) != 1 || datastore.HostNames[0] != "esxi-01.lab.local" {
		t.Fatalf("expected datastore host attachment enrichment, got %+v", datastore.HostNames)
	}
	if len(datastore.VMNames) != 1 || datastore.VMNames[0] != "app-01" {
		t.Fatalf("expected datastore VM attachment enrichment, got %+v", datastore.VMNames)
	}
	if len(datastore.RecentTasks) != 1 || datastore.RecentTasks[0].Name != "Refresh datastore" {
		t.Fatalf("expected datastore recent task info, got %+v", datastore.RecentTasks)
	}
	if len(datastore.RecentEvents) != 1 || datastore.RecentEvents[0].Type != "DatastoreRenamedEvent" {
		t.Fatalf("expected datastore recent event info, got %+v", datastore.RecentEvents)
	}
}

func TestClientTestConnectionRequiresSignalFloor(t *testing.T) {
	server := newVMwareTestServer(t, vmwareTestServerConfig{
		omitHostOverallStatus: true,
	})
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.TestConnection(context.Background())
	if err == nil {
		t.Fatal("expected signal-floor validation error")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "not_found" {
		t.Fatalf("connection error category = %q, want not_found", connectionErr.Category)
	}
}

func TestClientResolveVIJSONReleaseFallsBackToSupportedProbeFloor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/8.0.3/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"sessionManager": map[string]any{"value": "SessionManager"},
			"perfManager":    map[string]any{"value": "PerformanceManager"},
			"eventManager":   map[string]any{"value": "EventManager"},
		}); err != nil {
			t.Fatalf("encode service content: %v", err)
		}
	})
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	release, refs, err := client.resolveVIJSONRelease(context.Background())
	if err != nil {
		t.Fatalf("resolveVIJSONRelease: %v", err)
	}
	if release != "8.0.3" {
		t.Fatalf("release = %q, want 8.0.3", release)
	}
	if refs.SessionManagerMoID != "SessionManager" || refs.PerfManagerMoID != "PerformanceManager" || refs.EventManagerMoID != "EventManager" {
		t.Fatalf("unexpected refs: %+v", refs)
	}
}

func TestClientResolveVIJSONReleaseClassifiesUnsupportedVersionFloor(t *testing.T) {
	server := httptest.NewTLSServer(http.NewServeMux())
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, _, err = client.resolveVIJSONRelease(context.Background())
	if err == nil {
		t.Fatal("expected unsupported version error")
	}
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T", err)
	}
	if connectionErr.Category != "unsupported_version" {
		t.Fatalf("connection error category = %q, want unsupported_version", connectionErr.Category)
	}
	if !strings.Contains(connectionErr.Message, "9.0.0.0") || !strings.Contains(connectionErr.Message, "8.0.1.0") {
		t.Fatalf("expected message to mention implemented probe floor, got %q", connectionErr.Message)
	}
}

type vmwareTestServerConfig struct {
	omitHostOverallStatus bool
}

func newVMwareTestServer(t *testing.T, cfg vmwareTestServerConfig) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	writeJSON := func(w http.ResponseWriter, payload any) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(payload); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}

	mux.HandleFunc("/api/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, "automation-session")
	})

	mux.HandleFunc("/api/vcenter/host", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryHost{{
			Host:            "host-101",
			Name:            "esxi-01.lab.local",
			ConnectionState: "CONNECTED",
			PowerState:      "POWERED_ON",
			HostUUID:        "uuid-host-1",
		}})
	})
	mux.HandleFunc("/api/vcenter/vm", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryVM{{
			VM:            "vm-201",
			Name:          "app-01",
			PowerState:    "POWERED_ON",
			CPUCount:      4,
			MemorySizeMiB: 8192,
		}})
	})
	mux.HandleFunc("/api/vcenter/datastore", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, []InventoryDatastore{{
			Datastore: "datastore-11",
			Name:      "nvme-primary",
			Type:      "VMFS",
			FreeSpace: 40,
			Capacity:  100,
		}})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, map[string]any{
			"identity": map[string]any{
				"bios_uuid":     "vm-bios-201",
				"instance_uuid": "vm-instance-201",
			},
		})
	})
	mux.HandleFunc("/api/vcenter/vm/vm-201/guest/identity", func(w http.ResponseWriter, r *http.Request) {
		requireAutomationSession(t, r)
		writeJSON(w, map[string]any{
			"family":     "LINUX",
			"host_name":  "app-01.internal",
			"ip_address": "10.0.0.21",
		})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"sessionManager": map[string]any{"value": "SessionManager"},
			"perfManager":    map[string]any{"value": "PerformanceManager"},
			"eventManager":   map[string]any{"value": "EventManager"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/SessionManager/SessionManager/Login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("vmware-api-session-id", "vi-session")
		writeJSON(w, map[string]any{"value": "ok"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/perfCounter", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{
			{"key": 1, "groupInfo": map[string]any{"key": "cpu"}, "nameInfo": map[string]any{"key": "usage"}, "rollupType": "average"},
			{"key": 2, "groupInfo": map[string]any{"key": "mem"}, "nameInfo": map[string]any{"key": "usage"}, "rollupType": "average"},
			{"key": 3, "groupInfo": map[string]any{"key": "mem"}, "nameInfo": map[string]any{"key": "totalCapacity"}, "rollupType": "average"},
			{"key": 4, "groupInfo": map[string]any{"key": "net"}, "nameInfo": map[string]any{"key": "bytesRx"}, "rollupType": "average"},
			{"key": 5, "groupInfo": map[string]any{"key": "net"}, "nameInfo": map[string]any{"key": "bytesTx"}, "rollupType": "average"},
			{"key": 6, "groupInfo": map[string]any{"key": "disk"}, "nameInfo": map[string]any{"key": "read"}, "rollupType": "average"},
			{"key": 7, "groupInfo": map[string]any{"key": "disk"}, "nameInfo": map[string]any{"key": "write"}, "rollupType": "average"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryPerfProviderSummary", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"currentSupported": true,
			"refreshRate":      20,
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryAvailablePerfMetric", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		var request struct {
			Entity struct {
				Type string `json:"type"`
			} `json:"entity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode available perf request: %v", err)
		}
		switch request.Entity.Type {
		case "HostSystem":
			writeJSON(w, []map[string]any{
				{"counterId": 1, "instance": ""},
				{"counterId": 2, "instance": ""},
				{"counterId": 3, "instance": ""},
				{"counterId": 4, "instance": "vmnic0"},
				{"counterId": 5, "instance": "vmnic0"},
				{"counterId": 6, "instance": "naa.1"},
				{"counterId": 7, "instance": "naa.1"},
			})
		case "VirtualMachine":
			writeJSON(w, []map[string]any{
				{"counterId": 1, "instance": ""},
				{"counterId": 2, "instance": ""},
				{"counterId": 4, "instance": "4000"},
				{"counterId": 5, "instance": "4000"},
				{"counterId": 6, "instance": "2000"},
				{"counterId": 7, "instance": "2000"},
			})
		default:
			writeJSON(w, []map[string]any{})
		}
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryPerf", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		var request struct {
			QuerySpec []struct {
				Entity struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"entity"`
			} `json:"querySpec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode query perf request: %v", err)
		}
		if len(request.QuerySpec) != 1 {
			t.Fatalf("expected single query spec, got %d", len(request.QuerySpec))
		}
		switch request.QuerySpec[0].Entity.Type {
		case "HostSystem":
			writeJSON(w, []map[string]any{{
				"entity": map[string]any{"type": "HostSystem", "value": "host-101"},
				"value": []map[string]any{
					{"id": map[string]any{"counterId": 1, "instance": ""}, "value": []int64{2140}},
					{"id": map[string]any{"counterId": 2, "instance": ""}, "value": []int64{6320}},
					{"id": map[string]any{"counterId": 3, "instance": ""}, "value": []int64{40960}},
					{"id": map[string]any{"counterId": 4, "instance": "vmnic0"}, "value": []int64{1}},
					{"id": map[string]any{"counterId": 5, "instance": "vmnic0"}, "value": []int64{2}},
					{"id": map[string]any{"counterId": 6, "instance": "naa.1"}, "value": []int64{4}},
					{"id": map[string]any{"counterId": 7, "instance": "naa.1"}, "value": []int64{8}},
				},
			}})
		case "VirtualMachine":
			writeJSON(w, []map[string]any{{
				"entity": map[string]any{"type": "VirtualMachine", "value": "vm-201"},
				"value": []map[string]any{
					{"id": map[string]any{"counterId": 1, "instance": ""}, "value": []int64{3810}},
					{"id": map[string]any{"counterId": 2, "instance": ""}, "value": []int64{5750}},
					{"id": map[string]any{"counterId": 4, "instance": "4000"}, "value": []int64{3}},
					{"id": map[string]any{"counterId": 5, "instance": "4000"}, "value": []int64{4}},
					{"id": map[string]any{"counterId": 6, "instance": "2000"}, "value": []int64{5}},
					{"id": map[string]any{"counterId": 7, "instance": "2000"}, "value": []int64{6}},
				},
			}})
		default:
			writeJSON(w, []map[string]any{})
		}
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/EventManager/EventManager/QueryEvents", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		var request struct {
			Filter struct {
				Entity *struct {
					Entity struct {
						Type  string `json:"type"`
						Value string `json:"value"`
					} `json:"entity"`
					Recursion string `json:"recursion"`
				} `json:"entity"`
				MaxCount int `json:"maxCount"`
			} `json:"filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode query events request: %v", err)
		}
		if request.Filter.Entity == nil {
			t.Fatal("expected event filter entity")
		}
		if request.Filter.Entity.Recursion != "self" {
			t.Fatalf("event recursion = %q, want self", request.Filter.Entity.Recursion)
		}
		if request.Filter.MaxCount != 3 {
			t.Fatalf("event maxCount = %d, want 3", request.Filter.MaxCount)
		}
		switch request.Filter.Entity.Entity.Type {
		case "HostSystem":
			writeJSON(w, []map[string]any{{
				"key":                  101,
				"userName":             "administrator@vsphere.local",
				"createdTime":          "2026-03-30T18:15:00Z",
				"fullFormattedMessage": "Host entered maintenance evaluation",
				"eventTypeId":          "HostConnectionStateEvent",
			}})
		case "VirtualMachine":
			writeJSON(w, []map[string]any{{
				"key":                  201,
				"userName":             "vpxuser",
				"createdTime":          "2026-03-30T18:12:00Z",
				"fullFormattedMessage": "Snapshot completed successfully",
				"eventTypeId":          "VmMessageEvent",
			}})
		case "Datastore":
			writeJSON(w, []map[string]any{{
				"key":                  301,
				"userName":             "storage-admin",
				"createdTime":          "2026-03-30T18:09:00Z",
				"fullFormattedMessage": "Datastore metadata refreshed",
				"eventTypeId":          "DatastoreRenamedEvent",
			}})
		default:
			writeJSON(w, []map[string]any{})
		}
	})

	if !cfg.omitHostOverallStatus {
		mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/overallStatus", func(w http.ResponseWriter, r *http.Request) {
			requireVISession(t, r)
			writeJSON(w, "yellow")
		})
	}
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{
			"alarm":         map[string]any{"type": "Alarm", "value": "alarm-11"},
			"overallStatus": "yellow",
			"acknowledged":  false,
			"time":          "2026-03-30T18:12:00Z",
		}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Task", "value": "task-11"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "esxi-01.lab.local")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "ClusterComputeResource", "value": "domain-c101"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/HostSystem/host-101/datastore", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Datastore", "value": "datastore-11"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/overallStatus", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "green")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{
			"alarm":         map[string]any{"type": "Alarm", "value": "alarm-21"},
			"overallStatus": "red",
			"acknowledged":  false,
			"time":          "2026-03-30T18:13:00Z",
		}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Task", "value": "task-21"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/snapshot", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"rootSnapshotList": []map[string]any{{
				"childSnapshotList": []map[string]any{{"childSnapshotList": []map[string]any{}}},
			}},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-v7"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/runtime", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"host": map[string]any{"type": "HostSystem", "value": "host-101"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/resourcePool", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "ResourcePool", "value": "resgroup-22"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/VirtualMachine/vm-201/datastore", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Datastore", "value": "datastore-11"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/overallStatus", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "yellow")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/triggeredAlarmState", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/recentTask", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "Task", "value": "task-31"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "nvme-primary")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-s4"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/summary", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"accessible":         true,
			"multipleHostAccess": true,
			"maintenanceMode":    "normal",
			"url":                "ds:///vmfs/volumes/datastore-11/",
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/host", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "HostSystem", "value": "host-101"}})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datastore/datastore-11/vm", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, []map[string]any{{"type": "VirtualMachine", "value": "vm-201"}})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ClusterComputeResource/domain-c101/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Prod Compute")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/ClusterComputeResource/domain-c101/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Folder", "value": "group-h4"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-h4/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Prod Hosts")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-h4/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-v7/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Production VMs")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-v7/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-s4/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Shared Datastores")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Folder/group-s4/parent", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "Datacenter", "value": "datacenter-1"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Datacenter/datacenter-1/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "DC1")
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ResourcePool/resgroup-22/name", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, "Tier 1")
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/ResourcePool/resgroup-22/owner", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"type": "ClusterComputeResource", "value": "domain-c101"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Alarm/alarm-11/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"name": "Host connection degraded"})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Alarm/alarm-21/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{"name": "VM replication fault"})
	})

	mux.HandleFunc("/sdk/vim25/9.0.0.0/Task/task-11/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"name":      map[string]any{"localizedMessage": "Reconnect host"},
			"state":     "running",
			"startTime": "2026-03-30T18:14:00Z",
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Task/task-21/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"name":          map[string]any{"localizedMessage": "Create snapshot"},
			"state":         "success",
			"descriptionId": "VirtualMachine.createSnapshot",
			"startTime":     "2026-03-30T18:10:00Z",
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/Task/task-31/info", func(w http.ResponseWriter, r *http.Request) {
		requireVISession(t, r)
		writeJSON(w, map[string]any{
			"name":      map[string]any{"localizedMessage": "Refresh datastore"},
			"state":     "queued",
			"startTime": "2026-03-30T18:11:00Z",
		})
	})

	return httptest.NewTLSServer(mux)
}

func requireAutomationSession(t *testing.T, r *http.Request) {
	t.Helper()
	if got := strings.TrimSpace(r.Header.Get("vmware-api-session-id")); got != "automation-session" {
		t.Fatalf("automation session header = %q, want automation-session", got)
	}
}

func requireVISession(t *testing.T, r *http.Request) {
	t.Helper()
	if got := strings.TrimSpace(r.Header.Get("vmware-api-session-id")); got != "vi-session" {
		t.Fatalf("vi-json session header = %q, want vi-session", got)
	}
}
