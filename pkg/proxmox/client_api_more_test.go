package proxmox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := ClientConfig{
		Host:       server.URL,
		TokenName:  "user@pve!token",
		TokenValue: "secret",
		VerifySSL:  false,
		Timeout:    2 * time.Second,
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	return client
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload interface{}) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}

func TestClientStorageAndTasks(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/storage":
			writeJSON(t, w, map[string]interface{}{
				"data": []Storage{{Storage: "local", Type: "dir"}},
			})
		case "/api2/json/nodes":
			writeJSON(t, w, map[string]interface{}{
				"data": []Node{{Node: "node1", Status: "online"}, {Node: "node2", Status: "offline"}},
			})
		case "/api2/json/nodes/node1/tasks":
			writeJSON(t, w, map[string]interface{}{
				"data": []Task{
					{UPID: "1", Type: "vzdump"},
					{UPID: "2", Type: "other"},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	storage, err := client.GetAllStorage(ctx)
	if err != nil {
		t.Fatalf("GetAllStorage error: %v", err)
	}
	if len(storage) != 1 || storage[0].Storage != "local" {
		t.Fatalf("unexpected storage: %+v", storage)
	}

	tasks, err := client.GetBackupTasks(ctx)
	if err != nil {
		t.Fatalf("GetBackupTasks error: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Type != "vzdump" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestClientSnapshotsAndContent(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/storage/local/content":
			writeJSON(t, w, map[string]interface{}{
				"data": []StorageContent{
					{Volid: "backup1", Content: "backup"},
					{Volid: "iso1", Content: "iso"},
					{Volid: "tmpl1", Content: "vztmpl"},
				},
			})
		case "/api2/json/nodes/node1/qemu/100/snapshot":
			writeJSON(t, w, map[string]interface{}{
				"data": []Snapshot{{Name: "current"}, {Name: "snap1"}},
			})
		case "/api2/json/nodes/node1/lxc/101/snapshot":
			writeJSON(t, w, map[string]interface{}{
				"data": []Snapshot{{Name: "current"}, {Name: "snap2"}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	content, err := client.GetStorageContent(ctx, "node1", "local")
	if err != nil {
		t.Fatalf("GetStorageContent error: %v", err)
	}
	if len(content) != 2 {
		t.Fatalf("expected 2 backup-related items, got %d", len(content))
	}

	snaps, err := client.GetVMSnapshots(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMSnapshots error: %v", err)
	}
	if len(snaps) != 1 || snaps[0].Name != "snap1" || snaps[0].VMID != 100 {
		t.Fatalf("unexpected VM snapshots: %+v", snaps)
	}

	ctSnaps, err := client.GetContainerSnapshots(ctx, "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerSnapshots error: %v", err)
	}
	if len(ctSnaps) != 1 || ctSnaps[0].Name != "snap2" || ctSnaps[0].VMID != 101 {
		t.Fatalf("unexpected container snapshots: %+v", ctSnaps)
	}
}

func TestClientClusterAndAgentInfo(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/cluster/status":
			writeJSON(t, w, map[string]interface{}{
				"data": []ClusterStatus{
					{Type: "cluster", Name: "prod"},
					{Type: "node", Name: "node1"},
				},
			})
		case "/api2/json/nodes/node1/qemu/100/config":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{"name": "vm1"},
			})
		case "/api2/json/nodes/node1/qemu/100/agent/get-osinfo":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{"id": "linux"},
			})
		case "/api2/json/nodes/node1/qemu/100/agent/info":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": map[string]interface{}{
						"version": "1.2.3",
					},
				},
			})
		case "/api2/json/nodes/node1/qemu/100/agent/network-get-interfaces":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{
					"result": []VMNetworkInterface{{Name: "eth0"}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	status, err := client.GetClusterStatus(ctx)
	if err != nil {
		t.Fatalf("GetClusterStatus error: %v", err)
	}
	if len(status) != 2 {
		t.Fatalf("unexpected cluster status: %+v", status)
	}
	member, err := client.IsClusterMember(ctx)
	if err != nil {
		t.Fatalf("IsClusterMember error: %v", err)
	}
	if !member {
		t.Fatal("expected cluster membership")
	}

	config, err := client.GetVMConfig(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMConfig error: %v", err)
	}
	if config["name"] != "vm1" {
		t.Fatalf("unexpected vm config: %+v", config)
	}

	osInfo, err := client.GetVMAgentInfo(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentInfo error: %v", err)
	}
	if osInfo["id"] != "linux" {
		t.Fatalf("unexpected agent info: %+v", osInfo)
	}

	version, err := client.GetVMAgentVersion(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentVersion error: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("unexpected agent version: %q", version)
	}

	ifaces, err := client.GetVMNetworkInterfaces(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMNetworkInterfaces error: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Fatalf("unexpected interfaces: %+v", ifaces)
	}
}

func TestClientStatusAndResources(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/status/current":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{"status": "running"},
			})
		case "/api2/json/nodes/node1/lxc/101/status/current":
			writeJSON(t, w, map[string]interface{}{
				"data": map[string]interface{}{"status": "running", "vmid": 101},
			})
		case "/api2/json/cluster/resources":
			writeJSON(t, w, map[string]interface{}{
				"data": []ClusterResource{{ID: "qemu/100", Type: "vm", VMID: 100}},
			})
		case "/api2/json/nodes/node1/lxc/101/interfaces":
			writeJSON(t, w, map[string]interface{}{
				"data": []ContainerInterface{{Name: "eth0", HWAddr: "aa:bb"}},
			})
		default:
			http.NotFound(w, r)
		}
	})

	ctx := context.Background()
	vmStatus, err := client.GetVMStatus(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMStatus error: %v", err)
	}
	if vmStatus.Status != "running" {
		t.Fatalf("unexpected VM status: %+v", vmStatus)
	}

	ctStatus, err := client.GetContainerStatus(ctx, "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerStatus error: %v", err)
	}
	if ctStatus.Status != "running" || ctStatus.VMID != 101 {
		t.Fatalf("unexpected container status: %+v", ctStatus)
	}

	resources, err := client.GetClusterResources(ctx, "vm")
	if err != nil {
		t.Fatalf("GetClusterResources error: %v", err)
	}
	if len(resources) != 1 || resources[0].VMID != 100 {
		t.Fatalf("unexpected resources: %+v", resources)
	}

	ifaces, err := client.GetContainerInterfaces(ctx, "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerInterfaces error: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Fatalf("unexpected container interfaces: %+v", ifaces)
	}
}
