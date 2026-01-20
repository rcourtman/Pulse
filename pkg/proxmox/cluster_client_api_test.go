package proxmox

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterClient_GetCephStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":{"health":{"status":"HEALTH_OK"}}}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	ctx := context.Background()
	status, err := cc.GetCephStatus(ctx)
	if err != nil {
		t.Fatalf("GetCephStatus failed: %v", err)
	}
	if status.Health.Status != "HEALTH_OK" {
		t.Errorf("expected HEALTH_OK, got %s", status.Health.Status)
	}
}

func TestClusterClient_GetVMSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"name":"snap1","description":"first"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	ctx := context.Background()
	snaps, err := cc.GetVMSnapshots(ctx, "node1", 100)
	if err != nil {
		t.Fatalf("GetVMSnapshots failed: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestClusterClient_GetStorageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"volid":"local:iso/ubuntu.iso","format":"iso","size":1000,"content":"backup"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	ctx := context.Background()
	content, err := cc.GetStorageContent(ctx, "node1", "local")
	if err != nil {
		t.Fatalf("GetStorageContent failed: %v", err)
	}
	if len(content) != 1 {
		t.Errorf("expected 1 item, got %d", len(content))
	}
}

func TestClusterClient_RecoverUnhealthyNodes(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	// Manually mark as unhealthy
	cc.mu.Lock()
	cc.nodeHealth[server.URL] = false
	delete(cc.lastHealthCheck, server.URL) // Ensure no throttle
	cc.mu.Unlock()

	cc.recoverUnhealthyNodes(context.Background())

	// Wait for async recovery
	time.Sleep(100 * time.Millisecond)

	cc.mu.RLock()
	healthy := cc.nodeHealth[server.URL]
	cc.mu.RUnlock()

	if !healthy {
		t.Error("expected node to be recovered and marked healthy")
	}
	if atomic.LoadInt32(&callCount) == 0 {
		t.Error("expected recovery check to be performed")
	}
}

func TestClusterClient_InitialHealthCheck_Failures(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server2.Close()

	cfg := ClientConfig{Host: server1.URL, TokenName: "u@p!t", TokenValue: "v", Timeout: 100 * time.Millisecond}
	// initialHealthCheck happens in NewClusterClient
	cc := NewClusterClient("test", cfg, []string{server1.URL, server2.URL}, nil)

	health := cc.GetHealthStatus()
	// NewClusterClient already calls initialHealthCheck which should mark them unhealthy
	if healthy, ok := health[server1.URL]; !ok || healthy {
		t.Errorf("expected node 1 to be marked unhealthy initially, got healthy=%v", healthy)
	}
	if healthy, ok := health[server2.URL]; !ok || healthy {
		t.Errorf("expected node 2 to be marked unhealthy initially, got healthy=%v", healthy)
	}
}

func TestClusterClient_GetNodeRRDData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"time":12345678,"cpu":0.5}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	data, err := cc.GetNodeRRDData(context.Background(), "node1", "hour", "AVERAGE", []string{"cpu"})
	if err != nil {
		t.Fatalf("GetNodeRRDData failed: %v", err)
	}
	if len(data) != 1 {
		t.Errorf("expected 1 data point, got %d", len(data))
	}
}

func TestClusterClient_GetLXCRRDData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"time":12345678,"cpu":0.5}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	data, err := cc.GetLXCRRDData(context.Background(), "node1", 100, "hour", "AVERAGE", []string{"cpu"})
	if err != nil {
		t.Fatalf("GetLXCRRDData failed: %v", err)
	}
	if len(data) != 1 {
		t.Errorf("expected 1 data point, got %d", len(data))
	}
}

func TestClusterClient_GetVMs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"vmid":100,"name":"vm1","status":"running"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	vms, err := cc.GetVMs(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetVMs failed: %v", err)
	}
	if len(vms) != 1 {
		t.Errorf("expected 1 VM, got %d", len(vms))
	}
}

func TestClusterClient_GetContainers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"vmid":200,"name":"ct1","status":"running"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	cts, err := cc.GetContainers(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetContainers failed: %v", err)
	}
	if len(cts) != 1 {
		t.Errorf("expected 1 container, got %d", len(cts))
	}
}

func TestClusterClient_GetStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"storage":"local","type":"dir"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	storage, err := cc.GetStorage(context.Background(), "node1")
	if err != nil {
		t.Fatalf("GetStorage failed: %v", err)
	}
	if len(storage) != 1 {
		t.Errorf("expected 1 storage, got %d", len(storage))
	}
}

func TestClusterClient_GetBackupTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"upid":"upid:1","type":"vzdump","status":"ok"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	tasks, err := cc.GetBackupTasks(context.Background())
	if err != nil {
		t.Fatalf("GetBackupTasks failed: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}
}

func TestClusterClient_GetContainerSnapshots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"name":"snap1","description":"desc"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	snaps, err := cc.GetContainerSnapshots(context.Background(), "node1", 200)
	if err != nil {
		t.Fatalf("GetContainerSnapshots failed: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
}

func TestClusterClient_GetContainerConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		fmt.Fprint(w, `{"data":{"hostname":"ct1"}}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	config, err := cc.GetContainerConfig(context.Background(), "node1", 200)
	if err != nil {
		t.Fatalf("GetContainerConfig failed: %v", err)
	}
	if config["hostname"] != "ct1" {
		t.Errorf("expected hostname ct1, got %v", config["hostname"])
	}
}
