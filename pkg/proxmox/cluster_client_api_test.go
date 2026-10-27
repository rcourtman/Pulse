package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClusterRecoveryRequiresUsableSession(t *testing.T) {
	testClusterRecoveryRequiresUsableSession(t)
}

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
	isolateGuestAgentCapacity(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Guest reads require fresh operation-lock evidence, independently of their payload fixture.
		if strings.HasSuffix(r.URL.Path, "/config") {
			fmt.Fprint(w, `{"data":{}}`)
			return
		}
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
		fmt.Fprint(w, `{"data":[{"name":"snap1","description":"first","snaptime":1700000000}]}`)
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
			fmt.Fprint(w, `{"data":[{"name":"snap-authority","description":"cluster snapshot","snaptime":1700000000}]}`)
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
		fmt.Fprint(w, `{"data":[{"storage":"local","type":"dir","content":"images,backup"}]}`)
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
		fmt.Fprint(w, `{"data":[{"name":"snap1","description":"desc","snaptime":1700000000}]}`)
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

func TestClusterClient_GetTaskLog(t *testing.T) {
	const upid = "UPID:node1:000E9F2C:0AC734B2:68A1B2C3:vzdump::root@pam:"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api2/json/nodes" {
			fmt.Fprint(w, `{"data":[{"node":"node1","status":"online"}]}`)
			return
		}
		if r.URL.Path != "/api2/json/nodes/node1/tasks/"+upid+"/log" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		fmt.Fprint(w, `{"data":[{"n":1,"t":"INFO: Starting Backup of VM 101 (qemu)"},{"n":2,"t":"INFO: Finished Backup of VM 101 (00:01:30)"}]}`)
	}))
	defer server.Close()

	cfg := ClientConfig{Host: server.URL, TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("test", cfg, []string{server.URL}, nil)

	lines, err := cc.GetTaskLog(context.Background(), "node1", upid)
	if err != nil {
		t.Fatalf("GetTaskLog failed: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d", len(lines))
	}
	if lines[0].Text != "INFO: Starting Backup of VM 101 (qemu)" || lines[0].LineNumber != 1 {
		t.Errorf("unexpected first line: %+v", lines[0])
	}
}

// Issue #1664: refreshTOFUFingerprintAndRetry only refreshed endpoints that
// already had a per-endpoint fingerprint entry, so a newly joined member could
// never establish its first-use trust. Known-member rotation is not first use.
// A new member has no entry and fell back to the primary's pinned
// fingerprint forever.
func TestClusterClient_TOFUFirstUse_TrustsNewMemberCert(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api2/json/nodes" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[{"node":"member-b","status":"online"}]}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := ClientConfig{
		Host:        "https://primary.issue1664.invalid",
		TokenName:   "root@pam!pulse",
		TokenValue:  "secret",
		Fingerprint: "DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF:DE:AD:BE:EF",
	}
	// Single endpoint skips the initial health check; the member endpoint has
	// no per-endpoint fingerprint entry, exactly like a freshly joined node.
	cc := NewClusterClient("issue1664", cfg, []string{server.URL}, nil)

	pinnedConfig := cfg
	pinnedConfig.Host = server.URL
	pinned, err := NewClient(pinnedConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pinned.httpClient.CloseIdleConnections()
	_, mismatch := pinned.GetNodes(context.Background())
	if !isFingerprintMismatchError(mismatch) {
		t.Fatalf("expected a real TLS verifier mismatch, got %v", mismatch)
	}
	client, err, refreshed := cc.refreshTOFUFingerprintAndRetry(context.Background(), server.URL, 2*time.Second, mismatch)
	if !refreshed {
		t.Fatal("expected TOFU refresh to run for an endpoint with no stored fingerprint")
	}
	if err != nil {
		t.Fatalf("expected retry with freshly captured fingerprint to succeed, got %v", err)
	}
	if client == nil {
		t.Fatal("expected a working client after first-use trust")
	}

	cc.mu.RLock()
	stored := cc.endpointFingerprints[server.URL]
	cc.mu.RUnlock()
	if stored == "" {
		t.Fatal("expected member fingerprint to be stored for future connections")
	}
}

// Non-mismatch errors must not trigger TOFU capture.
func TestClusterClient_TOFURefresh_IgnoresNonTLSMismatch(t *testing.T) {
	cfg := ClientConfig{Host: "https://127.0.0.1:1", TokenName: "u@p!t", TokenValue: "v"}
	cc := NewClusterClient("issue1664b", cfg, []string{"https://127.0.0.1:1"}, nil)

	plainErr := errors.New("connection refused")
	_, err, refreshed := cc.refreshTOFUFingerprintAndRetry(context.Background(), "https://127.0.0.1:1", time.Second, plainErr)
	if refreshed {
		t.Fatal("expected no TOFU refresh for a non-mismatch error")
	}
	if !errors.Is(err, plainErr) {
		t.Fatalf("expected original error back, got %v", err)
	}
}

// Keep the trust-boundary proof alongside cluster runtime qualification.
func TestClusterClientFingerprintTrustContract(t *testing.T) {
	testClusterClientFingerprintTrust(t)
}

func TestClusterClientEndpointOwnershipContract(t *testing.T) {
	t.Run("routing-and-first-use-trust", testClusterClientEndpointOwnershipRoutingAndTrust)
	t.Run("shared-guest-admission", testClusterClientEndpointOwnershipGuestAdmission)
	t.Run("input-shape", testClusterClientEndpointOwnershipShape)
	t.Run("concurrent-ordinary-reads", testClusterClientEndpointOwnershipConcurrentReads)
}

// Guest admission equivalence must not broaden the existing certificate trust scope.
func TestGuestAgentIPv4MappedTrustScope(t *testing.T) {
	const ipv4 = "https://192.0.2.10:8006/tenant"
	const mapped = "https://[::ffff:192.0.2.10]:8006/tenant"
	for _, pair := range [][2]string{{ipv4, mapped}, {mapped, ipv4}} {
		cc := &ClusterClient{
			config:               ClientConfig{Host: pair[0], Fingerprint: "primary-pin"},
			endpoints:            []string{pair[0], pair[1]},
			endpointFingerprints: map[string]string{pair[0]: "saved-pin"},
		}
		a, okA := clusterEndpointKey(pair[0])
		b, okB := clusterEndpointKey(pair[1])
		if !okA || !okB || a == b {
			t.Error("admission equivalence broadened the existing TLS trust identity")
		}
		if _, known, trustErr := cc.knownEndpointFingerprintLocked(pair[1]); trustErr != nil || known || checkedClusterFingerprint(t, cc, pair[1]) != "primary-pin" {
			t.Error("coordination shared another configured authority's saved pin")
		}
		if !cc.canTrustMemberOnFirstUseLocked(pair[1]) {
			t.Error("unchanged declared-member first-use policy lost its distinct origin")
		}
		cc.endpointFingerprints[pair[1]] = "other-saved-pin"
		if checkedClusterFingerprint(t, cc, pair[0]) != "saved-pin" || checkedClusterFingerprint(t, cc, pair[1]) != "other-saved-pin" || cc.canTrustMemberOnFirstUseLocked(pair[1]) {
			t.Error("distinct established pins or no-replacement policy changed")
		}
	}
}

// A local TLS barrier holds only the unauthenticated first-use capture. A
// caller cancellation must release the probe without waiting for that peer;
// returning later without a pin is not enough to bound polling shutdown.
func TestClusterClientFirstUseCaptureCancellation(t *testing.T) {
	for _, mode := range []string{"cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			f := newClusterTrustFixture(t)
			primaryPin := clusterTrustPin(clusterTrustCertificate(t))
			cc := NewClusterClient("cancelled-capture", clusterTrustConfig("https://primary.trust.invalid", primaryPin), []string{f.server.URL}, nil)
			lastErr := clusterTrustMismatch(t, f, primaryPin)
			ctx, cancel := context.WithCancel(context.Background())
			if mode == "deadline" {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
			}
			defer cancel()
			type result struct {
				client  *Client
				err     error
				retried bool
			}
			done := make(chan result, 1)
			f.capture.Store(true)
			released := false
			release := func() {
				if !released {
					close(f.resume)
					released = true
				}
			}
			defer release()
			go func() {
				client, err, retried := cc.refreshTOFUFingerprintAndRetry(ctx, f.server.URL, time.Second, lastErr)
				done <- result{client, err, retried}
			}()
			select {
			case <-f.entered:
			case got := <-done:
				t.Fatalf("probe did not reach the owned TLS barrier: %v", got.err)
			case <-time.After(10 * time.Second):
				t.Fatal("probe did not reach the owned TLS barrier")
			}
			if mode == "cancel" {
				cancel()
			}
			<-ctx.Done()
			var got result
			select {
			case got = <-done:
			case <-time.After(time.Second):
				t.Error("cancelled capture still waits for the peer's TLS handshake")
				release() // Join even the adverse parent without leaving a blocked peer.
				got = <-done
			}
			if got.client != nil {
				got.client.httpClient.CloseIdleConnections()
			}
			if got.client != nil || got.retried || !errors.Is(got.err, lastErr) || len(cc.endpointFingerprints) != 0 || f.requests.Load() != 0 {
				t.Errorf("cancelled capture changed trust or sent authentication: retried=%t pins=%d requests=%d err=%v", got.retried, len(cc.endpointFingerprints), f.requests.Load(), got.err)
			}
			release()
			// Ending this caller is not a revocation of ordinary declared-member
			// first use. A fresh caller must still establish the same owned leaf
			// and make exactly one pin-verified authenticated request.
			client, err, retried := cc.refreshTOFUFingerprintAndRetry(context.Background(), f.server.URL, time.Second, lastErr)
			if client != nil {
				defer client.httpClient.CloseIdleConnections()
			}
			if err != nil || !retried || client == nil || checkedClusterFingerprint(t, cc, f.server.URL) != clusterTrustPin(f.active.Load()) || f.requests.Load() != 1 {
				t.Errorf("ordinary first use after cancelled probe failed: retried=%t requests=%d err=%v", retried, f.requests.Load(), err)
			}
		})
	}
}

// Startup, ordinary recovery and first selection share the same declared-member
// boundary for password and token authentication, not just post-construction I/O.
func TestClusterFirstUseSessionAdmission(t *testing.T) {
	testClusterFirstUseSessionAdmission(t)
}

// A nodes/login failure is not a per-guest failure, even when the provider's
// body mentions QGA. Exercise the real constructors/probes in all three phases;
// none of these handlers can enqueue a guest command or perform a backup.
func TestClusterNodeProbeRequiresSuccess(t *testing.T) {
	for _, phase := range []string{"startup", "recovery", "selection"} {
		for _, mode := range []string{"token", "json", "form"} {
			for _, stage := range []string{"nodes", "login"} {
				if stage == "login" && mode == "token" {
					continue
				}
				for i, message := range []string{"QEMU guest agent is not running", "VM 105 qmp command 'guest-get-fsinfo' failed", "proxy: guest-get-osinfo unavailable"} {
					t.Run(fmt.Sprintf("%s/%s/%s/body-%d", phase, mode, stage, i), func(t *testing.T) {
						f := newNodeProbeFixture(t, mode, stage, message)
						cfg := f.config()
						saved := map[string]string{f.server.URL: cfg.Fingerprint}
						f.reject.Store(true)
						var cc *ClusterClient
						if phase == "startup" {
							sibling := newNodeProbeFixture(t, mode, "nodes", "")
							saved[sibling.server.URL] = sibling.config().Fingerprint
							cc = NewClusterClient("node-probe", cfg, []string{f.server.URL, sibling.server.URL}, saved)
							if !cc.GetHealthStatus()[sibling.server.URL] {
								t.Fatal("failed peer blocked accepted sibling")
							}
						} else {
							cc = NewClusterClient("node-probe", cfg, []string{f.server.URL}, saved)
							if phase == "recovery" {
								cc.nodeHealth[f.server.URL] = false
								cc.lastError[f.server.URL] = "previous failed observation"
								cc.recoverUnhealthyNodes(context.Background())
							} else {
								c, err := cc.getHealthyClient(context.Background())
								if c != nil || err == nil {
									t.Error("failed nodes probe installed a polling client")
								}
							}
						}
						t.Cleanup(func() {
							cc.mu.RLock()
							defer cc.mu.RUnlock()
							for _, c := range cc.clients {
								if c != nil {
									c.httpClient.CloseIdleConnections()
								}
							}
						})
						health := cc.GetHealthStatusWithErrors()[f.server.URL]
						if health.Healthy || health.LastError == "" || health.LastCheck.IsZero() {
							t.Errorf("failed probe published healthy or lost its reason/time: %+v", health)
						}
						if cc.clients[f.server.URL] != nil {
							t.Error("failed probe installed a full polling client")
						}
						wantNodes := int32(1)
						if stage == "login" {
							wantNodes = 0
						}
						if f.logins.Load() != f.loginAttempts() || f.nodes.Load() != wantNodes {
							t.Errorf("failed probe was retried/advanced: logins=%d want=%d nodes=%d want=%d", f.logins.Load(), f.loginAttempts(), f.nodes.Load(), wantNodes)
						}
						if checkedClusterFingerprint(t, cc, f.server.URL) != cfg.Fingerprint || saved[f.server.URL] != cfg.Fingerprint || cc.config.VerifySSL != true {
							t.Error("failed probe changed saved trust")
						}
						// A distinct later recovery uses this fixture's normal accepted reply.
						// Only endpoint recovery eligibility advances, never a guest fence.
						f.reject.Store(false)
						cc.mu.Lock()
						delete(cc.lastHealthCheck, f.server.URL)
						cc.mu.Unlock()
						cc.recoverUnhealthyNodes(context.Background())
						if c := cc.clients[f.server.URL]; c == nil || c.config.Timeout != cfg.Timeout {
							t.Fatal("accepted later recovery has no full-timeout polling client")
						}
						cc.mu.Lock()
						for ep := range cc.nodeHealth {
							if ep != f.server.URL {
								cc.nodeHealth[ep] = false
								cc.lastHealthCheck[ep] = time.Now()
							}
						}
						cc.mu.Unlock()
						nodes, err := cc.GetNodes(context.Background())
						health = cc.GetHealthStatusWithErrors()[f.server.URL]
						if err != nil || len(nodes) != 1 || nodes[0].CPU != 0.25 || !health.Healthy || health.LastError != "" || f.nodes.Load() != wantNodes+2 || f.logins.Load() != 3*f.loginAttempts() {
							t.Errorf("separate ordinary recovery did not restore coherent readings: err=%v health=%+v logins=%d nodes=%d", err, health, f.logins.Load(), f.nodes.Load())
						}
					})
				}
			}
		}
	}
}

// Keep the existing single-endpoint last-resort read, but do not publish
// recovery simply for selecting its cached client. A failed read is still a
// failure; only a later successful ordinary read clears the observed error.
func TestClusterSingleEndpointFallbackKeepsFailedHealth(t *testing.T) {
	f := newNodeProbeFixture(t, "token", "nodes", "QEMU guest agent is not running")
	cc := NewClusterClient("single-probe", f.config(), []string{f.server.URL}, nil)
	if _, err := cc.GetNodes(context.Background()); err != nil {
		t.Fatal(err)
	}
	original := cc.clients[f.server.URL]
	defer original.httpClient.CloseIdleConnections()
	f.reject.Store(true)
	cc.markUnhealthyWithError(f.server.URL, "previous connection failure")
	c, err := cc.getHealthyClient(context.Background())
	if err != nil || c != original {
		t.Fatal("last-resort selection lost the existing client")
	}
	if h := cc.GetHealthStatusWithErrors()[f.server.URL]; h.Healthy || h.LastError == "" {
		t.Errorf("selection invented endpoint recovery: %+v", h)
	}
	before := f.nodes.Load()
	if _, err := cc.GetNodes(context.Background()); err == nil || !strings.Contains(err.Error(), "API error 500") {
		t.Errorf("failed ordinary read was hidden: %v", err)
	}
	if h := cc.GetHealthStatusWithErrors()[f.server.URL]; h.Healthy || h.LastError == "" || f.nodes.Load() != before+1 {
		t.Errorf("failed fallback read published recovery or replayed: %+v reads=%d", h, f.nodes.Load())
	}
	f.reject.Store(false)
	if nodes, err := cc.GetNodes(context.Background()); err != nil || len(nodes) != 1 {
		t.Fatalf("accepted later read did not recover: %+v %v", nodes, err)
	}
	if h := cc.GetHealthStatusWithErrors()[f.server.URL]; !h.Healthy || h.LastError != "" || f.nodes.Load() != before+2 || cc.clients[f.server.URL] != original {
		t.Errorf("accepted fallback did not restore health with the same client: %+v", h)
	}
}

// A genuine temporary nodes rate limit retains the established lazy-selection
// route: the actual ordinary read, not the failed probe, supplies its readings.
func TestClusterNodeProbeRateLimitKeepsOrdinaryRead(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/nodes" {
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "fixture busy")
			return
		}
		fmt.Fprint(w, `{"data":[{"node":"member","status":"online","cpu":0.25}]}`)
	}))
	defer server.Close()
	cc := NewClusterClient("probe-rate-limit", ClientConfig{Host: server.URL, TokenName: "fixture@pam!pulse", TokenValue: "fixture-token"}, []string{server.URL}, nil)
	nodes, err := cc.GetNodes(context.Background())
	if c := cc.clients[server.URL]; c != nil {
		defer c.httpClient.CloseIdleConnections()
	}
	if err != nil || len(nodes) != 1 || nodes[0].CPU != 0.25 || calls.Load() != 2 || !cc.GetHealthStatus()[server.URL] {
		t.Fatalf("temporary probe rate limit blocked ordinary read: %+v err=%v calls=%d", nodes, err, calls.Load())
	}
}

// Existing non-conflicting trust controls explicitly require resolution to
// succeed. A resolution failure must never fall back to an unpinned client.
func checkedClusterFingerprint(t *testing.T, cc *ClusterClient, endpoint string) string {
	t.Helper()
	fingerprint, err := cc.getEndpointFingerprint(endpoint)
	if err != nil {
		t.Fatalf("unexpected saved-trust ambiguity: %v", err)
	}
	return fingerprint
}

func TestClusterSavedPinResolutionRejectsConflicts(t *testing.T) {
	pairs := [][2]string{
		{"https://MEMBER.example.invalid:0443/tenant/", "https://member.example.invalid/tenant"},
		{"https://[2001:db8:0:0::1]:8006/tenant/", "https://[2001:db8::1]:08006/tenant"},
		{"member.example.invalid:8006/tenant/./", "https://member.example.invalid:8006/tenant"},
	}
	for _, pair := range pairs {
		pins := map[string]string{pair[0]: "saved-pin", pair[1]: "other-pin"}
		if err := CheckEndpointFingerprintConflicts(pins); !errors.Is(err, errConflictingEndpointFingerprints) {
			t.Errorf("refresh check lost contradictory saved trust: %v", err)
		}
		cc := NewClusterClient("resolution", ClientConfig{Host: "https://primary.example.invalid", Fingerprint: "base-pin"}, nil, pins)
		for _, endpoint := range pair {
			fingerprint, err := cc.getEndpointFingerprint(endpoint)
			if fingerprint != "" || !errors.Is(err, errConflictingEndpointFingerprints) {
				t.Errorf("exact spelling selected contradictory trust: pin=%q err=%v", fingerprint, err)
			}
			cc.mu.RLock()
			_, known, trustErr := cc.knownEndpointFingerprintLocked(endpoint)
			mayCapture := cc.canTrustMemberOnFirstUseLocked(endpoint)
			cc.mu.RUnlock()
			if !known || !errors.Is(trustErr, errConflictingEndpointFingerprints) || mayCapture {
				t.Error("conflicting saved trust became unknown first-use trust")
			}
		}
	}
	// Invalid nonempty pin presentation cannot turn into unpinned CA/opt-out
	// admission, even when every saved spelling agrees after normalization.
	for _, pin := range []string{":", "::", " ", "not-a-pin"} {
		pins := map[string]string{"https://member.example.invalid:443/": pin, "https://MEMBER.example.invalid": pin}
		if err := CheckEndpointFingerprintConflicts(pins); err != nil {
			t.Errorf("refresh check refused agreeing nonempty pins: %v", err)
		}
		cc := NewClusterClient("invalid-pin", ClientConfig{Host: "https://primary.example.invalid", Fingerprint: "base-pin", VerifySSL: false}, nil, pins)
		got, err := cc.getEndpointFingerprint("https://member.example.invalid")
		if err != nil || got != pin || got == "" {
			t.Errorf("invalid saved pin became absent trust: %q %v", got, err)
		}
	}
}

func TestClusterFirstUseCaptureDoesNotReconcileConflictingPins(t *testing.T) {
	f := newClusterTrustFixture(t)
	cc := NewClusterClient("capture-conflict", clusterTrustConfig("https://primary.capture.invalid", clusterTrustPin(clusterTrustCertificate(t))), []string{f.server.URL}, nil)
	lastErr := clusterTrustMismatch(t, f, cc.config.Fingerprint)
	f.capture.Store(true)
	done := make(chan error, 1)
	go func() {
		client, err, retried := cc.refreshTOFUFingerprintAndRetry(context.Background(), f.server.URL, time.Second, lastErr)
		if client != nil {
			client.httpClient.CloseIdleConnections()
			err = fmt.Errorf("conflicting capture constructed an authenticated client")
		}
		if retried {
			err = fmt.Errorf("conflicting capture attempted a pinned retry")
		}
		done <- err
	}()
	select {
	case <-f.entered:
	case <-time.After(5 * time.Second):
		close(f.resume)
		t.Fatal("capture did not reach fixture handshake")
	}
	cc.mu.Lock()
	cc.endpointFingerprints[f.server.URL] = clusterTrustPin(f.active.Load())
	cc.endpointFingerprints[f.server.URL+"/./"] = strings.Repeat("0", 64)
	cc.mu.Unlock()
	close(f.resume)
	if err := <-done; !errors.Is(err, errConflictingEndpointFingerprints) {
		t.Errorf("completed capture chose a conflicting pin: %v", err)
	}
	if f.requests.Load() != 0 || len(cc.endpointFingerprints) != 2 {
		t.Error("capture authenticated or reconciled contradictory saved trust")
	}
}

func TestClusterProducerPinConflictsAreOwnedAndScoped(t *testing.T) {
	const original = "https://member.example.invalid:443/tenant/"
	input := []string{original}
	if err := CheckEndpointFingerprintConflicts(map[string]string{original: "saved-pin"}, input...); !errors.Is(err, errConflictingEndpointFingerprints) {
		t.Errorf("refresh check discarded producer's duplicate-row conflict: %v", err)
	}
	cc := NewClusterClient("producer-conflict", ClientConfig{Host: "https://primary.example.invalid", Fingerprint: "primary-pin"}, nil, map[string]string{original: "saved-pin"}, input...)
	input[0] = "https://other.example.invalid/tenant"
	for _, endpoint := range []string{original, "https://MEMBER.example.invalid/tenant"} {
		fingerprint, err := cc.getEndpointFingerprint(endpoint)
		if fingerprint != "" || !errors.Is(err, errConflictingEndpointFingerprints) {
			t.Errorf("producer's copied conflict was lost: %q %v", fingerprint, err)
		}
	}
	for _, endpoint := range []string{"https://other.example.invalid/tenant", "https://member.example.invalid/other", "http://member.example.invalid/tenant", "https://member.example.invalid:8006/tenant"} {
		fingerprint, err := cc.getEndpointFingerprint(endpoint)
		if err != nil || fingerprint != "primary-pin" {
			t.Errorf("producer conflict refused an independent authority/path: %q %v", fingerprint, err)
		}
	}
}

func TestClusterExplicitEmptyEndpointTrust(t *testing.T) {
	primary := "https://primary.test:8006"
	member := "https://member.test:8006"
	cc := &ClusterClient{config: ClientConfig{Host: primary, Fingerprint: "primary-pin"}, endpoints: []string{primary, member}, endpointFingerprints: map[string]string{member: ""}}
	fingerprint, err := cc.getEndpointFingerprint(member)
	if err != nil || fingerprint != "" {
		t.Fatalf("explicit CA policy inherited primary pin: %q %v", fingerprint, err)
	}
	if cc.canTrustMemberOnFirstUseLocked(member) {
		t.Fatal("explicit CA policy allowed automatic trust capture")
	}
	cc.endpointFingerprints["https://MEMBER.test:8006/"] = "different-pin"
	if _, err := cc.getEndpointFingerprint(member); !errors.Is(err, errConflictingEndpointFingerprints) {
		t.Fatalf("explicit CA/pin ambiguity did not fail closed: %v", err)
	}
	for _, endpoint := range []string{"https://alias.test:8006", "https://member.test:8007", "http://member.test:8006", "https://member.test:8006/other"} {
		if _, known, err := ResolveEndpointFingerprint(endpoint, map[string]string{member: ""}); known || err != nil {
			t.Fatalf("explicit trust escaped authority/base path: %s", endpoint)
		}
	}
}
