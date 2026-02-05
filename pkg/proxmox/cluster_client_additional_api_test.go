package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClusterClient_GetNodeStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/status" {
			fmt.Fprint(w, `{"data":{"cpu":0.25,"uptime":123}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	status, err := cc.GetNodeStatus(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetNodeStatus failed: %v", err)
	}
	if status == nil || status.Uptime != 123 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestClusterClient_GetAllStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/storage" {
			fmt.Fprint(w, `{"data":[{"storage":"local"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	storage, err := cc.GetAllStorage(context.Background())
	if err != nil {
		t.Fatalf("GetAllStorage failed: %v", err)
	}
	if len(storage) != 1 || storage[0].Storage != "local" {
		t.Fatalf("unexpected storage result: %+v", storage)
	}
}

func TestClusterClient_GetVMStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/status/current" {
			fmt.Fprint(w, `{"data":{"status":"running","cpu":0.5,"uptime":10,"agent":1}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	status, err := cc.GetVMStatus(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMStatus failed: %v", err)
	}
	if status == nil || status.Status != "running" {
		t.Fatalf("unexpected VM status: %+v", status)
	}
}

func TestClusterClient_GetContainerStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/lxc/101/status/current" {
			fmt.Fprint(w, `{"data":{"status":"running"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	status, err := cc.GetContainerStatus(context.Background(), "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerStatus failed: %v", err)
	}
	if status == nil || status.Status != "running" {
		t.Fatalf("unexpected container status: %+v", status)
	}
}

func TestClusterClient_GetVMConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/config" {
			fmt.Fprint(w, `{"data":{"name":"vm-100"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	config, err := cc.GetVMConfig(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMConfig failed: %v", err)
	}
	if config["name"] != "vm-100" {
		t.Fatalf("unexpected VM config: %+v", config)
	}
}

func TestClusterClient_GetVMAgentInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/agent/get-osinfo" {
			fmt.Fprint(w, `{"data":{"name":"debian"}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	info, err := cc.GetVMAgentInfo(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentInfo failed: %v", err)
	}
	if info["name"] != "debian" {
		t.Fatalf("unexpected agent info: %+v", info)
	}
}

func TestClusterClient_GetVMAgentVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/agent/info" {
			fmt.Fprint(w, `{"data":{"result":{"version":"1.2.3"}}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	version, err := cc.GetVMAgentVersion(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMAgentVersion failed: %v", err)
	}
	if version != "1.2.3" {
		t.Fatalf("unexpected version: %s", version)
	}
}

func TestClusterClient_GetVMNetworkInterfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/qemu/100/agent/network-get-interfaces" {
			fmt.Fprint(w, `{"data":{"result":[{"name":"eth0"}]}}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	ifaces, err := cc.GetVMNetworkInterfaces(context.Background(), "node1", 100)
	if err != nil {
		t.Fatalf("GetVMNetworkInterfaces failed: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Fatalf("unexpected interfaces: %+v", ifaces)
	}
}

func TestClusterClient_GetContainerInterfaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/lxc/101/interfaces" {
			fmt.Fprint(w, `{"data":[{"name":"eth0","ip":"10.0.0.2"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	ifaces, err := cc.GetContainerInterfaces(context.Background(), "node1", 101)
	if err != nil {
		t.Fatalf("GetContainerInterfaces failed: %v", err)
	}
	if len(ifaces) != 1 || ifaces[0].Name != "eth0" {
		t.Fatalf("unexpected interfaces: %+v", ifaces)
	}
}

func TestClusterClient_GetClusterResources(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api2/json/cluster/resources") {
			fmt.Fprint(w, `{"data":[{"id":"qemu/100","type":"qemu","node":"node1"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	resources, err := cc.GetClusterResources(context.Background(), "qemu")
	if err != nil {
		t.Fatalf("GetClusterResources failed: %v", err)
	}
	if len(resources) != 1 || resources[0].ID != "qemu/100" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}

func TestClusterClient_GetClusterResources_AllTypes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/resources" {
			fmt.Fprint(w, `{"data":[{"id":"lxc/200","type":"lxc","node":"node1"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	resources, err := cc.GetClusterResources(context.Background(), "")
	if err != nil {
		t.Fatalf("GetClusterResources failed: %v", err)
	}
	if len(resources) != 1 || resources[0].ID != "lxc/200" {
		t.Fatalf("unexpected resources: %+v", resources)
	}
}

func TestClusterClient_GetClusterStatusAndIsClusterMember(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/cluster/status" {
			fmt.Fprint(w, `{"data":[{"type":"cluster","name":"pve"},{"type":"node","name":"node1"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	status, err := cc.GetClusterStatus(context.Background())
	if err != nil {
		t.Fatalf("GetClusterStatus failed: %v", err)
	}
	if len(status) != 2 {
		t.Fatalf("unexpected cluster status: %+v", status)
	}

	member, err := cc.IsClusterMember(context.Background())
	if err != nil {
		t.Fatalf("IsClusterMember failed: %v", err)
	}
	if !member {
		t.Fatal("expected cluster membership to be true")
	}
}

func TestClusterClient_GetZFSPoolStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/disks/zfs" {
			fmt.Fprint(w, `{"data":[{"name":"rpool","health":"ONLINE"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	pools, err := cc.GetZFSPoolStatus(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetZFSPoolStatus failed: %v", err)
	}
	if len(pools) != 1 || pools[0].Name != "rpool" {
		t.Fatalf("unexpected pools: %+v", pools)
	}
}

func TestClusterClient_GetDisks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/disks/list" {
			fmt.Fprint(w, `{"data":[{"devpath":"/dev/sda","model":"Samsung","wearout":"N/A"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	disks, err := cc.GetDisks(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetDisks failed: %v", err)
	}
	if len(disks) != 1 || disks[0].DevPath != "/dev/sda" {
		t.Fatalf("unexpected disks: %+v", disks)
	}
	if disks[0].Wearout != wearoutUnknown {
		t.Fatalf("expected wearout unknown, got %d", disks[0].Wearout)
	}
}

func TestClusterClient_GetNodePendingUpdates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path == "/api2/json/nodes/node1/apt/update" {
			fmt.Fprint(w, `{"data":[{"Package":"vim","Version":"2"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	updates, err := cc.GetNodePendingUpdates(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetNodePendingUpdates failed: %v", err)
	}
	if len(updates) != 1 || updates[0].Package != "vim" {
		t.Fatalf("unexpected updates: %+v", updates)
	}
}
