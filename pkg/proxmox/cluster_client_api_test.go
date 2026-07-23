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

func TestClusterClient_GetVMMemoryAvailabilityFromAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
		case "/api2/json/nodes/node1/qemu/1501/agent/file-read":
			fmt.Fprint(w, `{"data":{"content":"MemTotal: 7796964 kB\nMemFree: 444824 kB\nMemAvailable: 1872820 kB\nCached: 1580464 kB\n","truncated":false}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cc := NewClusterClient(
		"test",
		ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"},
		[]string{server.URL},
		nil,
	)
	availability, err := cc.GetVMMemoryAvailabilityFromAgent(context.Background(), "node1", 1501)
	if err != nil {
		t.Fatalf("GetVMMemoryAvailabilityFromAgent() error = %v", err)
	}
	if availability.Source != "meminfo-available" {
		t.Fatalf("source = %q, want meminfo-available", availability.Source)
	}
	if availability.EffectiveAvailable != 1872820*1024 {
		t.Fatalf("available = %d, want %d", availability.EffectiveAvailable, 1872820*1024)
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

func TestClusterClient_ConfiguredAuthorityDoesNotWaitForMemberRecovery(t *testing.T) {
	var authorityDataCalls atomic.Int32
	authority := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api2/json/nodes/node1/qemu/100/snapshot":
			authorityDataCalls.Add(1)
			fmt.Fprint(w, `{"data":[{"name":"snap-authority","description":"cluster snapshot"}]}`)
		case "/api2/json/nodes/node1/storage/local/content":
			authorityDataCalls.Add(1)
			fmt.Fprint(w, `{"data":[{"volid":"local:backup/vzdump-qemu-100.vma.zst","format":"vma.zst","size":1000,"content":"backup"}]}`)
		default:
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
		}
	}))
	defer authority.Close()

	recoveryStarted := make(chan struct{}, 2)
	releaseRecovery := make(chan struct{})
	recoveryReleased := false

	var memberDataCalls atomic.Int32
	newMember := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if r.URL.Path == "/api2/json/nodes" {
				recoveryStarted <- struct{}{}
				<-releaseRecovery
				fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
				return
			}
			memberDataCalls.Add(1)
			fmt.Fprint(w, `{"data":[]}`)
		}))
	}
	memberA := newMember()
	defer memberA.Close()
	memberB := newMember()
	defer memberB.Close()
	defer func() {
		if !recoveryReleased {
			close(releaseRecovery)
		}
	}()

	cfg := ClientConfig{
		Host:       authority.URL,
		TokenName:  "u@p!t",
		TokenValue: "v",
		Timeout:    2 * time.Second,
	}
	authorityClient, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("create configured authority client: %v", err)
	}

	endpoints := []string{authority.URL, memberA.URL, memberB.URL}
	cc := &ClusterClient{
		name:                 "remote-cluster",
		clients:              map[string]*Client{authority.URL: authorityClient},
		endpoints:            endpoints,
		endpointFingerprints: make(map[string]string),
		nodeHealth: map[string]bool{
			authority.URL: true,
			memberA.URL:   false,
			memberB.URL:   false,
		},
		lastHealthCheck: make(map[string]time.Time),
		lastError: map[string]string{
			memberA.URL: "Network unreachable - check network connectivity to Proxmox host",
			memberB.URL: "Network unreachable - check network connectivity to Proxmox host",
		},
		config:         cfg,
		rateLimitUntil: make(map[string]time.Time),
	}

	snapshotResult := make(chan error, 1)
	go func() {
		snapshots, snapshotErr := cc.GetVMSnapshots(context.Background(), "node1", 100)
		if snapshotErr == nil && (len(snapshots) != 1 || snapshots[0].Name != "snap-authority") {
			snapshotErr = fmt.Errorf("unexpected authority snapshots: %+v", snapshots)
		}
		snapshotResult <- snapshotErr
	}()

	select {
	case <-recoveryStarted:
		// Recovery is actively blocked on a member-only address.
	case <-time.After(2 * time.Second):
		t.Fatal("member recovery did not start")
	}

	select {
	case err := <-snapshotResult:
		if err != nil {
			t.Fatalf("snapshot polling through configured authority failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("snapshot polling waited for unreachable member recovery")
	}

	health := cc.GetHealthStatus()
	if !health[authority.URL] || health[memberA.URL] || health[memberB.URL] {
		t.Fatalf("expected healthy authority with degraded member evidence, got %+v", health)
	}

	close(releaseRecovery)
	recoveryReleased = true

	deadline := time.Now().Add(2 * time.Second)
	for {
		health = cc.GetHealthStatus()
		cc.mu.RLock()
		recoveryDone := !cc.recoveryInProgress
		cc.mu.RUnlock()
		if health[memberA.URL] && health[memberB.URL] && recoveryDone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected member refresh to recover both failovers, got %+v", health)
		}
		time.Sleep(10 * time.Millisecond)
	}

	for i := 0; i < 20; i++ {
		content, err := cc.GetStorageContent(context.Background(), "node1", "local")
		if err != nil {
			t.Fatalf("storage content poll %d failed: %v", i+1, err)
		}
		if len(content) != 1 {
			t.Fatalf("storage content poll %d returned %+v", i+1, content)
		}
	}
	if got := memberDataCalls.Load(); got != 0 {
		t.Fatalf("expected recovered members to remain failovers, got %d member data calls", got)
	}
	if got := authorityDataCalls.Load(); got != 21 {
		t.Fatalf("expected all API-only data from configured authority, got %d calls", got)
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

func TestClusterClient_GetVMRRDData(t *testing.T) {
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

	data, err := cc.GetVMRRDData(context.Background(), "node1", 100, "hour", "AVERAGE", []string{"cpu"})
	if err != nil {
		t.Fatalf("GetVMRRDData failed: %v", err)
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
