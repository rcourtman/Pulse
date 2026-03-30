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
	if len(host.TriggeredAlarms) != 1 || host.TriggeredAlarms[0].Name != "Host connection degraded" {
		t.Fatalf("expected resolved host alarm metadata, got %+v", host.TriggeredAlarms)
	}
	if len(host.RecentTasks) != 1 || host.RecentTasks[0].Name != "Reconnect host" {
		t.Fatalf("expected host recent task summary, got %+v", host.RecentTasks)
	}

	vm := snapshot.VMs[0]
	if len(vm.TriggeredAlarms) != 1 || vm.TriggeredAlarms[0].OverallStatus != "red" {
		t.Fatalf("expected VM alarm signals, got %+v", vm.TriggeredAlarms)
	}
	if vm.SnapshotCount != 2 {
		t.Fatalf("vm snapshot count = %d, want 2", vm.SnapshotCount)
	}
	if len(vm.RecentTasks) != 1 || vm.RecentTasks[0].State != "success" {
		t.Fatalf("expected VM recent task info, got %+v", vm.RecentTasks)
	}

	datastore := snapshot.Datastores[0]
	if datastore.OverallStatus != "yellow" {
		t.Fatalf("datastore overall status = %q, want yellow", datastore.OverallStatus)
	}
	if len(datastore.RecentTasks) != 1 || datastore.RecentTasks[0].Name != "Refresh datastore" {
		t.Fatalf("expected datastore recent task info, got %+v", datastore.RecentTasks)
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

	mux.HandleFunc("/sdk/vim25/9.0.0.0/ServiceInstance/ServiceInstance/content", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"sessionManager": map[string]any{"value": "SessionManager"},
		})
	})
	mux.HandleFunc("/sdk/vim25/9.0.0.0/SessionManager/SessionManager/Login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("vmware-api-session-id", "vi-session")
		writeJSON(w, map[string]any{"value": "ok"})
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
