package monitoring

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestMonitoringBroadcastCarriesEveryAvailabilityProjection(t *testing.T) {
	input := monitorResourceToConvertInput(unifiedresources.Resource{
		ID:     "agent-core2026",
		Type:   unifiedresources.ResourceTypeAgent,
		Name:   "core2026",
		Status: unifiedresources.StatusOnline,
		Sources: []unifiedresources.DataSource{
			unifiedresources.SourceAgent,
			unifiedresources.SourceAvailability,
		},
		AvailabilityChecks: []unifiedresources.AvailabilityData{
			{TargetID: "stats-pv", CorrelationState: unifiedresources.AvailabilityCorrelationAttached},
			{TargetID: "grafana", CorrelationState: unifiedresources.AvailabilityCorrelationAttached},
		},
	})

	payload := string(input.AvailabilityChecks)
	if !strings.Contains(payload, `"targetId":"stats-pv"`) ||
		!strings.Contains(payload, `"targetId":"grafana"`) {
		t.Fatalf("AvailabilityChecks = %s, want both projected checks", payload)
	}
	frontend := models.ConvertResourceToFrontend(input)
	if string(frontend.AvailabilityChecks) != payload {
		t.Fatalf("frontend availabilityChecks = %s, want %s", frontend.AvailabilityChecks, payload)
	}
}

func TestNormalizeAgentMemoryCacheAwareFallbacks(t *testing.T) {
	const gib = int64(1024 * 1024 * 1024)

	tests := []struct {
		name            string
		total           int64
		used            int64
		free            int64
		cache           int64
		usage           float64
		swapTotal       int64
		swapUsed        int64
		wantUsed        int64
		wantUsage       float64
		wantUnavailable bool
	}{
		{
			name:      "explicit agent usage remains authoritative",
			total:     8 * gib,
			used:      6 * gib,
			free:      gib / 2,
			cache:     2 * gib,
			wantUsed:  6 * gib,
			wantUsage: 75,
		},
		{
			name:      "explicit bytes correct a conflicting reported percentage",
			total:     8 * gib,
			used:      4 * gib,
			free:      gib,
			usage:     94,
			wantUsed:  4 * gib,
			wantUsage: 50,
		},
		{
			name:      "complete Linux cache evidence derives used without swap",
			total:     8 * gib,
			free:      gib,
			cache:     2 * gib,
			wantUsed:  5 * gib,
			wantUsage: 62.5,
		},
		{
			name:            "total and free alone stay unknown",
			total:           8 * gib,
			free:            gib,
			wantUnavailable: true,
		},
		{
			name:      "reported percentage can establish usage",
			total:     8 * gib,
			free:      gib,
			usage:     76,
			wantUsed:  8 * gib * 76 / 100,
			wantUsage: 76,
		},
		{
			name:      "zero swap does not invalidate cache aware memory",
			total:     8 * gib,
			free:      2 * gib,
			cache:     gib,
			swapTotal: 0,
			swapUsed:  0,
			wantUsed:  5 * gib,
			wantUsage: 62.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeAgentMemory(tt.total, tt.used, tt.free, tt.cache, tt.usage, tt.swapTotal, tt.swapUsed)
			if got.UsageUnavailable != tt.wantUnavailable {
				t.Fatalf("UsageUnavailable = %t, want %t", got.UsageUnavailable, tt.wantUnavailable)
			}
			if tt.wantUnavailable {
				if got.HasKnownUsage() {
					t.Fatalf("HasKnownUsage() = true for unavailable memory: %+v", got)
				}
				return
			}
			if got.Used != tt.wantUsed {
				t.Fatalf("Used = %d, want %d", got.Used, tt.wantUsed)
			}
			if got.Usage != tt.wantUsage {
				t.Fatalf("Usage = %.2f, want %.2f", got.Usage, tt.wantUsage)
			}
		})
	}
}

func TestApplyHostReportOperationReceiptProtocolReplacesCapabilityAuthority(t *testing.T) {
	now := time.Now().UTC()
	baseReport := func(version int) agentshost.Report {
		return agentshost.Report{
			Agent: agentshost.AgentInfo{
				ID:                      "receipt-agent",
				Version:                 "6.0.6",
				Type:                    "unified",
				IntervalSeconds:         30,
				CommandsEnabled:         true,
				OperationReceiptVersion: version,
			},
			Host: agentshost.HostInfo{
				ID:        "receipt-machine",
				Hostname:  "receipt-host",
				Platform:  "linux",
				OSName:    "debian",
				OSVersion: "12",
				PackageUpdates: &agentshost.PackageUpdateStatus{
					Supported: true, Manager: "apt", InventoryHash: "sha256:" + strings.Repeat("a", 64), PendingCount: 2, CheckedAt: now,
				},
				StorageCleanup: &agentshost.StorageCleanupStatus{
					Supported: true, Provider: "apt-package-cache", Fingerprint: "sha256:" + strings.Repeat("b", 64), ReclaimableBytes: 512 << 20, CheckedAt: now,
				},
			},
			Disks:     []agentshost.Disk{{Mountpoint: "/", TotalBytes: 100 << 30, UsedBytes: 95 << 30, FreeBytes: 5 << 30, Usage: 95}},
			Timestamp: now,
		}
	}
	capabilityNames := func(host models.Host) map[string]bool {
		names := map[string]bool{}
		for _, capability := range unifiedresources.HostIngestRecord(host).Resource.Capabilities {
			names[capability.Name] = true
		}
		return names
	}
	t.Run("legacy absent", func(t *testing.T) {
		encoded, err := json.Marshal(baseReport(0))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "operationReceiptVersion") {
			t.Fatalf("legacy fixture unexpectedly contains receipt protocol: %s", encoded)
		}
		var legacy agentshost.Report
		if err := json.Unmarshal(encoded, &legacy); err != nil {
			t.Fatal(err)
		}
		monitor := newTestMonitor(t)
		host, err := monitor.ApplyHostReport(legacy, &config.APITokenRecord{ID: "receipt-token"})
		if err != nil {
			t.Fatalf("ApplyHostReport: %v", err)
		}
		if host.OperationReceiptVersion != 0 || len(capabilityNames(host)) != 0 {
			t.Fatalf("legacy absent protocol gained capability authority: version=%d capabilities=%#v", host.OperationReceiptVersion, capabilityNames(host))
		}
	})

	for _, test := range []struct {
		name    string
		version int
		want    bool
	}{
		{name: "explicit zero unsupported", version: 0, want: false},
		{name: "current supported", version: operationreceipt.ProtocolVersion, want: true},
		{name: "future unsupported", version: operationreceipt.ProtocolVersion + 1, want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			monitor := newTestMonitor(t)
			host, err := monitor.ApplyHostReport(baseReport(test.version), &config.APITokenRecord{ID: "receipt-token"})
			if err != nil {
				t.Fatalf("ApplyHostReport: %v", err)
			}
			if host.OperationReceiptVersion != test.version {
				t.Fatalf("ingested receipt version = %d, want %d", host.OperationReceiptVersion, test.version)
			}
			names := capabilityNames(host)
			got := names["install_os_updates"] && names["clean_package_cache"] && len(names) == 2
			if got != test.want {
				t.Fatalf("derived capabilities = %#v, want both=%v", names, test.want)
			}
		})
	}

	monitor := newTestMonitor(t)
	compatible, err := monitor.ApplyHostReport(baseReport(operationreceipt.ProtocolVersion), &config.APITokenRecord{ID: "receipt-token"})
	if err != nil || len(capabilityNames(compatible)) != 2 {
		t.Fatalf("compatible ingest: capabilities=%#v err=%v", capabilityNames(compatible), err)
	}
	downgradedReport := baseReport(0)
	downgradedReport.Timestamp = now.Add(time.Second)
	downgraded, err := monitor.ApplyHostReport(downgradedReport, &config.APITokenRecord{ID: "receipt-token"})
	if err != nil {
		t.Fatalf("downgrade ApplyHostReport: %v", err)
	}
	if downgraded.OperationReceiptVersion != 0 || len(capabilityNames(downgraded)) != 0 {
		t.Fatalf("downgrade retained capability authority: version=%d capabilities=%#v", downgraded.OperationReceiptVersion, capabilityNames(downgraded))
	}
}

func TestMonitor_GetConfiguredHostIPs_ResolvesOutsideMonitorLock(t *testing.T) {
	originalLookup := lookupConfiguredHostIP
	lookupStarted := make(chan struct{})
	releaseLookup := make(chan struct{})
	lookupConfiguredHostIP = func(host string) ([]net.IP, error) {
		close(lookupStarted)
		<-releaseLookup
		return []net.IP{net.ParseIP("192.0.2.10")}, nil
	}
	defer func() {
		lookupConfiguredHostIP = originalLookup
	}()

	m := &Monitor{
		config: &config.Config{
			PVEInstances: []config.PVEInstance{
				{Host: "pve.example.test"},
			},
		},
	}

	done := make(chan []string, 1)
	go func() {
		done <- m.getConfiguredHostIPs()
	}()

	select {
	case <-lookupStarted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for configured host lookup to start")
	}

	writerAcquired := make(chan struct{})
	go func() {
		m.mu.Lock()
		close(writerAcquired)
		m.mu.Unlock()
	}()

	select {
	case <-writerAcquired:
	case <-time.After(time.Second):
		close(releaseLookup)
		t.Fatal("configured host lookup held the monitor lock while resolving hostnames")
	}

	close(releaseLookup)
	ips := <-done
	if len(ips) != 1 || ips[0] != "192.0.2.10" {
		t.Fatalf("unexpected resolved host IPs: %#v", ips)
	}
}

func TestMonitor_DiscoveryConfigSnapshot_MergesConfiguredHostIPs(t *testing.T) {
	m := &Monitor{
		config: &config.Config{
			Discovery: config.DiscoveryConfig{
				IPBlocklist: []string{"10.0.0.8"},
			},
			PVEInstances: []config.PVEInstance{
				{Host: "https://192.168.1.10:8006"},
			},
			PBSInstances: []config.PBSInstance{
				{Host: "http://192.168.1.20:8007"},
			},
		},
	}

	cfg := m.discoveryConfigSnapshot()
	ipMap := make(map[string]bool, len(cfg.IPBlocklist))
	for _, ip := range cfg.IPBlocklist {
		ipMap[ip] = true
	}

	for _, expected := range []string{"10.0.0.8", "192.168.1.10", "192.168.1.20"} {
		if !ipMap[expected] {
			t.Fatalf("expected discovery IP blocklist to include %s, got %#v", expected, cfg.IPBlocklist)
		}
	}
}

func TestFindLinkedProxmoxEntity_MatchesCanonicalReadStateViews(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve-a", Instance: "pve1"},
	})
	monitor.state.UpdateVMs([]models.VM{
		{ID: "vm-100", Name: "vm-a", Instance: "pve1", VMID: 100},
	})
	monitor.state.UpdateContainers([]models.Container{
		{ID: "ct-200", Name: "ct-a", Instance: "pve1", VMID: 200},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntity("pve-a")
	if nodeID != "node-1" || vmID != "" || ctID != "" {
		t.Fatalf("expected node match only, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}

	nodeID, vmID, ctID = monitor.findLinkedProxmoxEntity("vm-a")
	if nodeID != "" || vmID != "vm-100" || ctID != "" {
		t.Fatalf("expected vm match only, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}

	nodeID, vmID, ctID = monitor.findLinkedProxmoxEntity("ct-a")
	if nodeID != "" || vmID != "" || ctID != "ct-200" {
		t.Fatalf("expected container match only, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestSnapshotBackedUnifiedReadStateUsesConfiguredStaleThreshold(t *testing.T) {
	seen := time.Now().UTC().Add(-90 * time.Second).Truncate(time.Millisecond)
	state := models.NewState()
	state.UpdateVMs([]models.VM{{
		ID:       "cluster-a:pve-a:101",
		Name:     "db",
		Node:     "pve-a",
		Instance: "cluster-a",
		VMID:     101,
		Status:   "running",
		Type:     "qemu",
		LastSeen: seen,
	}})

	monitor := &Monitor{
		config: &config.Config{
			PVEPollingInterval: 5 * time.Minute,
		},
		state: state,
	}

	readState := monitor.snapshotBackedUnifiedReadState()
	if readState == nil {
		t.Fatal("expected snapshot-backed read state")
	}
	vms := readState.VMs()
	if len(vms) != 1 {
		t.Fatalf("VM count = %d, want 1", len(vms))
	}
	if vms[0].Status() != unifiedresources.StatusOnline {
		t.Fatalf("VM status = %q, want online", vms[0].Status())
	}
}

func TestFindLinkedProxmoxEntity_AmbiguousNodeNameReturnsNoLink(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve", Instance: "pve-a"},
		{ID: "node-2", Name: "pve", Instance: "pve-b"},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntity("pve")
	if nodeID != "" || vmID != "" || ctID != "" {
		t.Fatalf("expected ambiguous node name to produce no link, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestFindLinkedProxmoxEntityWithHints_UsesEndpointIPToDisambiguateNodes(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve", Instance: "pve-a", Host: "https://10.0.0.1:8006"},
		{ID: "node-2", Name: "pve", Instance: "pve-b", Host: "https://10.0.0.2:8006"},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntityWithHints("pve", "10.0.0.2", nil)
	if nodeID != "node-2" || vmID != "" || ctID != "" {
		t.Fatalf("expected endpoint IP to disambiguate node-2, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestFindLinkedProxmoxEntityWithHints_UsesExactEndpointHostnameBeforeNameFallback(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	monitor.state.UpdateNodes([]models.Node{
		{ID: "node-1", Name: "pve", Instance: "pve-a", Host: "https://pve-a.lab:8006"},
		{ID: "node-2", Name: "pve", Instance: "pve-b", Host: "https://pve-b.lab:8006"},
	})

	nodeID, vmID, ctID := monitor.findLinkedProxmoxEntityWithHints("pve-b.lab", "", nil)
	if nodeID != "node-2" || vmID != "" || ctID != "" {
		t.Fatalf("expected endpoint hostname to disambiguate node-2, got node=%q vm=%q ct=%q", nodeID, vmID, ctID)
	}
}

func TestHostSensorsFromReadStateViewPreservesSMARTSizeBytes(t *testing.T) {
	sensors := &unifiedresources.HostSensorMeta{
		SMART: []unifiedresources.HostSMARTMeta{
			{
				Device:      "/dev/sda",
				Model:       "CT240BX500SSD1",
				Serial:      "SATA-SERIAL-1",
				Type:        "sata",
				SizeBytes:   240_057_409_536,
				Temperature: 32,
				Health:      "PASSED",
			},
		},
	}

	got := hostSensorsFromReadStateView(sensors)
	if len(got.SMART) != 1 {
		t.Fatalf("SMART row count = %d, want 1", len(got.SMART))
	}
	if got.SMART[0].SizeBytes != 240_057_409_536 {
		t.Fatalf("SMART SizeBytes = %d, want 240057409536", got.SMART[0].SizeBytes)
	}
	if got.SMART[0].Temperature != 32 {
		t.Fatalf("SMART temperature = %d, want 32", got.SMART[0].Temperature)
	}
}

func TestApplyDockerReportNormalizesContainerCPUCapacityAcceptedIngestProof(t *testing.T) {
	monitor := newTestMonitor(t)
	token := &config.APITokenRecord{ID: "token-docker-cpu", Name: "Docker Token"}

	host, err := monitor.ApplyDockerReport(agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "docker-cpu-agent",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "docker-cpu-host",
			MachineID:        "docker-cpu-machine",
			TotalCPU:         4,
			TotalMemoryBytes: 8 << 30,
		},
		Containers: []agentsdocker.Container{{
			ID:         "container-cpu",
			Name:       "api",
			Image:      "nginx:latest",
			State:      "running",
			Status:     "Up 1 minute",
			CPUPercent: 240,
		}},
		Timestamp: time.Now().UTC(),
	}, token)
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}
	if len(host.Containers) != 1 {
		t.Fatalf("expected one reported container, got %d", len(host.Containers))
	}
	if got := host.Containers[0].CPUCapacityPercent; got != 60 {
		t.Fatalf("reported container CPU capacity percent = %v, want 60", got)
	}

	points := monitor.metricsHistory.GetGuestMetrics("docker:"+host.Containers[0].ID, "cpu", time.Hour)
	if len(points) != 1 || points[0].Value != 60 {
		t.Fatalf("metrics history Docker CPU points = %+v, want one normalized value 60", points)
	}
}

func TestApplyDockerReportRefreshesUnifiedReadStateWithoutBroadcast(t *testing.T) {
	monitor := newTestMonitor(t)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	monitor.SetResourceStore(adapter)

	_, err := monitor.ApplyDockerReport(agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "headless-docker-agent", IntervalSeconds: 30},
		Host: agentsdocker.HostInfo{
			Hostname:         "headless-docker-host",
			MachineID:        "headless-docker-machine",
			DockerVersion:    "27.0.0",
			TotalCPU:         4,
			TotalMemoryBytes: 8 << 30,
		},
		Containers: []agentsdocker.Container{{
			ID:    "headless-container-id",
			Name:  "headless-canary-container",
			State: "running",
		}},
		Timestamp: time.Now().UTC(),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}

	var foundHost bool
	for _, host := range adapter.DockerHosts() {
		if host != nil && host.Hostname() == "headless-docker-host" {
			foundHost = true
			break
		}
	}
	if !foundHost {
		t.Fatal("accepted Docker report did not refresh the canonical headless read state")
	}

	var foundContainer bool
	for _, container := range adapter.DockerContainers() {
		if container != nil && container.Name() == "headless-canary-container" {
			foundContainer = true
			break
		}
	}
	if !foundContainer {
		t.Fatal("accepted Docker container report did not refresh the canonical headless read state")
	}
}

func TestApplyHostReportRefreshesUnifiedReadStateWithoutBroadcast(t *testing.T) {
	monitor := newTestMonitor(t)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	monitor.SetResourceStore(adapter)

	_, err := monitor.ApplyHostReport(agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "headless-host-agent", IntervalSeconds: 30},
		Host: agentshost.HostInfo{
			ID:       "headless-host-machine",
			Hostname: "headless-host",
			Platform: "linux",
			OSName:   "debian",
		},
		Timestamp: time.Now().UTC(),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	for _, host := range adapter.Hosts() {
		if host != nil && host.Hostname() == "headless-host" {
			return
		}
	}
	t.Fatal("accepted host report did not refresh the canonical headless read state")
}

func TestUnifiedAgentHostAndDockerReportsShareOneCanonicalMachine(t *testing.T) {
	now := time.Now().UTC()
	hostReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "dual-mode-agent",
			Version:         "6.1.1",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "dual-mode-machine",
			MachineID: "dual-mode-machine",
			Hostname:  "docker-lxc.local",
			Platform:  "linux",
			OSName:    "debian",
		},
		Timestamp: now,
	}
	dockerReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "dual-mode-agent",
			Version:         "6.1.1",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "docker-lxc.local",
			MachineID:        "dual-mode-machine",
			DockerVersion:    "27.0.0",
			TotalCPU:         4,
			TotalMemoryBytes: 8 << 30,
		},
		Containers: []agentsdocker.Container{{
			ID:    "workload-1",
			Name:  "app",
			State: "running",
		}},
		Timestamp: now.Add(time.Second),
	}

	for _, tc := range []struct {
		name        string
		dockerFirst bool
	}{
		{name: "host report first"},
		{name: "Docker report first", dockerFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			monitor := newTestMonitor(t)
			adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
			monitor.SetResourceStore(adapter)
			token := &config.APITokenRecord{ID: "dual-mode-token", Name: "Dual-mode Agent"}

			applyHost := func() {
				t.Helper()
				if _, err := monitor.ApplyHostReport(hostReport, token); err != nil {
					t.Fatalf("ApplyHostReport: %v", err)
				}
			}
			applyDocker := func() {
				t.Helper()
				if _, err := monitor.ApplyDockerReport(dockerReport, token); err != nil {
					t.Fatalf("ApplyDockerReport: %v", err)
				}
			}
			if tc.dockerFirst {
				applyDocker()
				applyHost()
			} else {
				applyHost()
				applyDocker()
			}

			resources := adapter.GetAll()
			if len(resources) != 2 {
				t.Fatalf("canonical resource count = %d, want one machine and one workload: %#v", len(resources), resources)
			}
			var machine *unifiedresources.Resource
			for i := range resources {
				if resources[i].Type == unifiedresources.ResourceTypeAgent {
					machine = &resources[i]
					break
				}
			}
			if machine == nil {
				t.Fatalf("canonical resources did not retain the machine: %#v", resources)
			}
			if machine.Agent == nil || machine.Docker == nil {
				t.Fatalf("machine facets = agent:%v docker:%v, want both", machine.Agent != nil, machine.Docker != nil)
			}
			if got := unifiedresources.ContractResourceType(*machine); got != unifiedresources.ResourceTypeAgent {
				t.Fatalf("machine contract type = %q, want agent", got)
			}

			hostViews := adapter.Hosts()
			dockerViews := adapter.DockerHosts()
			if len(hostViews) != 1 || len(dockerViews) != 1 {
				t.Fatalf("typed views = hosts:%d docker:%d, want one in each", len(hostViews), len(dockerViews))
			}
			if hostViews[0].ID() != dockerViews[0].ID() || hostViews[0].ID() != machine.ID {
				t.Fatalf("typed view identities diverged: host=%q docker=%q machine=%q", hostViews[0].ID(), dockerViews[0].ID(), machine.ID)
			}
		})
	}
}

func TestDockerOnlyAgentRemainsWorkloadOnly(t *testing.T) {
	monitor := newTestMonitor(t)
	adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
	monitor.SetResourceStore(adapter)

	_, err := monitor.ApplyDockerReport(agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "workload-only-agent",
			Version:         "6.1.1",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "runtime-only.local",
			MachineID:        "runtime-only-machine",
			DockerVersion:    "27.0.0",
			TotalCPU:         2,
			TotalMemoryBytes: 4 << 30,
		},
		Timestamp: time.Now().UTC(),
	}, &config.APITokenRecord{ID: "workload-only-token"})
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}

	resources := adapter.GetAll()
	if len(resources) != 1 {
		t.Fatalf("canonical resource count = %d, want one Docker runtime: %#v", len(resources), resources)
	}
	if resources[0].Agent != nil || resources[0].Docker == nil {
		t.Fatalf("workload-only facets = agent:%v docker:%v, want Docker only", resources[0].Agent != nil, resources[0].Docker != nil)
	}
	if got := unifiedresources.ContractResourceType(resources[0]); got != unifiedresources.ResourceType("docker-host") {
		t.Fatalf("workload-only contract type = %q, want docker-host", got)
	}
	if len(adapter.Hosts()) != 0 || len(adapter.DockerHosts()) != 1 {
		t.Fatalf("typed views = hosts:%d docker:%d, want workload-only runtime excluded from Hosts", len(adapter.Hosts()), len(adapter.DockerHosts()))
	}
}

func TestApplyDockerReportMigratesAppContainerURLToStableNameAcceptedIngestProof(t *testing.T) {
	monitor := newTestMonitor(t)
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "docker-url-agent",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-url-host",
			MachineID: "docker-url-machine",
		},
		Containers: []agentsdocker.Container{{
			ID:   "container-runtime-old",
			Name: "/homepage",
		}},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyDockerReport(report, nil)
	if err != nil {
		t.Fatalf("initial ApplyDockerReport: %v", err)
	}

	legacyKey := dockerAppContainerLegacyResourceID(host.ID, "container-runtime-old")
	if err := monitor.guestMetadataStore.Set(legacyKey, &config.GuestMetadata{
		CustomURL: "https://homepage.internal",
	}); err != nil {
		t.Fatalf("seed legacy guest metadata: %v", err)
	}

	report.Timestamp = report.Timestamp.Add(30 * time.Second)
	if _, err := monitor.ApplyDockerReport(report, nil); err != nil {
		t.Fatalf("metadata migration ApplyDockerReport: %v", err)
	}

	stableKey := dockerAppContainerMetadataKey(host.ID, "homepage")
	stableMeta := monitor.guestMetadataStore.Get(stableKey)
	if stableMeta == nil {
		t.Fatalf("expected stable metadata at %q", stableKey)
	}
	if stableMeta.CustomURL != "https://homepage.internal" {
		t.Fatalf("stable CustomURL = %q, want legacy URL", stableMeta.CustomURL)
	}
}

func TestMonitor_HostAgentConfigUpdatePreservesReportedCommandStateInHostState(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		hostMetadataStore: config.NewHostMetadataStore(t.TempDir(), nil),
		config:            &config.Config{},
	}
	monitor.state.UpsertHost(models.Host{
		ID:              "host-command-policy",
		Hostname:        "host-command-policy",
		CommandsEnabled: true,
	})

	desired := false
	if err := monitor.UpdateHostAgentConfig("host-command-policy", &desired); err != nil {
		t.Fatalf("UpdateHostAgentConfig: %v", err)
	}

	hosts := monitor.state.GetHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected one host, got %d", len(hosts))
	}
	if !hosts[0].CommandsEnabled {
		t.Fatalf("reported CommandsEnabled should remain true until the agent applies and reports the desired policy")
	}

	cfg := monitor.GetHostAgentConfig("host-command-policy")
	if cfg.CommandsEnabled == nil || *cfg.CommandsEnabled {
		t.Fatalf("desired CommandsEnabled = %#v, want false", cfg.CommandsEnabled)
	}
	if cfg.DesiredConfig == nil {
		t.Fatal("expected desired config metadata for command-policy update")
	}
}

func TestApplyDockerReport_RecreatedContainerAgentIDKeepsTokenBinding(t *testing.T) {
	monitor := newTestMonitor(t)
	token := &config.APITokenRecord{ID: "token-recreated-container", Name: "Docker Token"}

	firstReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "container-machine-id-a",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-lxc",
			MachineID: "container-machine-id-a",
		},
		Timestamp: time.Now().UTC(),
	}

	host1, err := monitor.ApplyDockerReport(firstReport, token)
	if err != nil {
		t.Fatalf("first ApplyDockerReport failed: %v", err)
	}

	recreatedReport := firstReport
	recreatedReport.Agent.ID = "container-machine-id-b"
	recreatedReport.Host.MachineID = "container-machine-id-b"
	recreatedReport.Timestamp = firstReport.Timestamp.Add(time.Minute)

	host2, err := monitor.ApplyDockerReport(recreatedReport, token)
	if err != nil {
		t.Fatalf("recreated container report should keep the existing token binding: %v", err)
	}

	if host1.ID != host2.ID {
		t.Fatalf("expected recreated container to retain host ID %q, got %q", host1.ID, host2.ID)
	}
	if got := monitor.dockerTokenBindings[token.ID]; got != host1.ID {
		t.Fatalf("token binding = %q, want stable host ID %q", got, host1.ID)
	}
}

func TestApplyDockerReportTokenConflictUsesModuleCopy(t *testing.T) {
	monitor := newTestMonitor(t)
	token := &config.APITokenRecord{ID: "token-conflict", Name: "Docker Token"}

	firstReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "module-a", Version: "1.0.0", IntervalSeconds: 30},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-a",
			MachineID: "machine-a",
		},
		Timestamp: time.Now().UTC(),
	}
	if _, err := monitor.ApplyDockerReport(firstReport, token); err != nil {
		t.Fatalf("first ApplyDockerReport failed: %v", err)
	}

	conflictingReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "module-b", Version: "1.0.0", IntervalSeconds: 30},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-b",
			MachineID: "machine-b",
		},
		Timestamp: firstReport.Timestamp.Add(time.Minute),
	}
	_, err := monitor.ApplyDockerReport(conflictingReport, token)
	if err == nil {
		t.Fatal("expected token conflict")
	}
	message := err.Error()
	if !strings.Contains(message, "Each Docker / Podman module must use a unique API token") {
		t.Fatalf("token conflict must use module copy, got %q", message)
	}
	if strings.Contains(message, "Docker"+" agent") || strings.Contains(message, "Docker / Podman"+" agent") {
		t.Fatalf("token conflict must not describe Docker / Podman as a separate agent product: %q", message)
	}
}

func TestApplyDockerReportPreservesDockerSwarmNodes(t *testing.T) {
	monitor := newTestMonitor(t)
	now := time.Now().UTC()
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "docker-agent-1",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:       "docker-host",
			MachineID:      "machine-docker-1",
			Runtime:        "docker",
			RuntimeVersion: "27.5.1",
			Swarm: &agentsdocker.SwarmInfo{
				NodeID:           "node-manager",
				NodeRole:         "manager",
				LocalState:       "active",
				ControlAvailable: true,
				ClusterID:        "cluster-1",
				ClusterName:      "prod-swarm",
				Scope:            "cluster",
			},
		},
		Nodes: []agentsdocker.Node{{
			ID:                  " node-manager ",
			Hostname:            " manager-1 ",
			Role:                " manager ",
			Availability:        " active ",
			State:               " ready ",
			Address:             " 192.0.2.10 ",
			ManagerReachability: " reachable ",
			ManagerAddress:      " 192.0.2.10:2377 ",
			Leader:              true,
			EngineVersion:       " 27.5.1 ",
			OS:                  " linux ",
			Architecture:        " amd64 ",
			NanoCPUs:            8_000_000_000,
			MemoryBytes:         32 * 1024 * 1024 * 1024,
			Labels:              map[string]string{"zone": "rack-a"},
			EngineLabels:        map[string]string{"engine": "primary"},
			CreatedAt:           now.Add(-time.Hour),
			UpdatedAt:           &now,
		}},
		Timestamp: now,
	}

	host, err := monitor.ApplyDockerReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}
	if len(host.Nodes) != 1 {
		t.Fatalf("expected host node inventory, got %+v", host.Nodes)
	}
	node := host.Nodes[0]
	if node.ID != "node-manager" || node.Hostname != "manager-1" || node.Role != "manager" {
		t.Fatalf("unexpected host node identity: %+v", node)
	}
	if !node.Leader || node.ManagerReachability != "reachable" || node.EngineLabels["engine"] != "primary" {
		t.Fatalf("expected manager metadata to be preserved, got %+v", node)
	}

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.DockerHosts) != 1 || len(snapshot.DockerHosts[0].Nodes) != 1 {
		t.Fatalf("expected state snapshot to preserve nodes, got %+v", snapshot.DockerHosts)
	}
	if got := snapshot.DockerHosts[0].Nodes[0].ID; got != "node-manager" {
		t.Fatalf("snapshot node id = %q, want node-manager", got)
	}
}

func TestEvaluateHostAgentsTriggersOfflineAlert(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-offline"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "offline.local",
		DisplayName:     "Offline Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now().Add(-10 * time.Minute),
	})

	now := time.Now()
	for i := 0; i < 3; i++ {
		monitor.evaluateHostAgents(now.Add(time.Duration(i) * time.Second))
	}

	snapshot := monitor.state.GetSnapshot()
	statusUpdated := false
	for _, host := range snapshot.Hosts {
		if host.ID == hostID {
			statusUpdated = true
			if got := host.Status; got != "offline" {
				t.Fatalf("expected host status offline, got %q", got)
			}
		}
	}
	if !statusUpdated {
		t.Fatalf("host %q not found in state snapshot", hostID)
	}

	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false, got %v (exists=%v)", healthy, ok)
	}

	alerts := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range alerts {
		if alert.Type == "host-offline" && alert.ResourceID == "agent:"+hostID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected host offline alert to remain active")
	}
}

func TestEvaluateHostAgentsClearsAlertWhenHostReturns(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-recover"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recover.local",
		DisplayName:     "Recover Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now().Add(-10 * time.Minute),
	})

	for i := 0; i < 3; i++ {
		monitor.evaluateHostAgents(time.Now().Add(time.Duration(i) * time.Second))
	}

	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recover.local",
		DisplayName:     "Recover Host",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true after recovery, got %v (exists=%v)", healthy, ok)
	}

	for _, alert := range monitor.alertManager.GetActiveAlerts() {
		if alert.ID == "host-offline-"+hostID {
			t.Fatalf("offline alert still active after recovery")
		}
	}
}

func TestCleanupTrackingMapsClearsStalePVEBackupInventoryScope(t *testing.T) {
	now := time.Now()
	stale := now.Add(-25 * time.Hour)
	fresh := now.Add(-time.Hour)
	staleSubject := pveBackupTemplateSubjectKey("pve-stale", "qemu", "node1", 900)
	freshSubject := pveBackupTemplateSubjectKey("pve-fresh", "qemu", "node1", 901)

	monitor := &Monitor{
		lastPVEBackupPoll: map[string]time.Time{
			"pve-stale": stale,
			"pve-fresh": fresh,
		},
		pveBackupInventoryReady: map[string]map[string]bool{
			"pve-stale": {"qemu": true},
			"pve-fresh": {"qemu": true},
		},
		pveBackupTemplateSubjects: map[string]map[string]struct{}{
			"pve-stale": {staleSubject: {}},
			"pve-fresh": {freshSubject: {}},
		},
	}

	monitor.cleanupTrackingMaps(now)

	if _, ok := monitor.lastPVEBackupPoll["pve-stale"]; ok {
		t.Fatalf("expected stale PVE backup poll marker to be removed")
	}
	if _, ok := monitor.pveBackupInventoryReady["pve-stale"]; ok {
		t.Fatalf("expected stale PVE backup inventory readiness to be removed")
	}
	if _, ok := monitor.pveBackupTemplateSubjects["pve-stale"]; ok {
		t.Fatalf("expected stale PVE backup template subjects to be removed")
	}
	if _, ok := monitor.lastPVEBackupPoll["pve-fresh"]; !ok {
		t.Fatalf("expected fresh PVE backup poll marker to remain")
	}
	if !monitor.pveBackupInventoryReady["pve-fresh"]["qemu"] {
		t.Fatalf("expected fresh PVE backup inventory readiness to remain")
	}
	if _, ok := monitor.pveBackupTemplateSubjects["pve-fresh"][freshSubject]; !ok {
		t.Fatalf("expected fresh PVE backup template subject to remain")
	}
}

func TestApplyHostReportAllowsTokenReuseAcrossHosts(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-one",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-one",
			Hostname:  "host-one",
			Platform:  "linux",
			OSName:    "debian",
			OSVersion: "12",
		},
		Timestamp: now,
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 1.0,
		},
	}

	token := &config.APITokenRecord{ID: "token-one", Name: "Token One"}

	hostOne, err := monitor.ApplyHostReport(baseReport, token)
	if err != nil {
		t.Fatalf("ApplyHostReport hostOne: %v", err)
	}
	if hostOne.ID == "" {
		t.Fatalf("expected hostOne to have an identifier")
	}

	secondReport := baseReport
	secondReport.Agent.ID = "agent-two"
	secondReport.Host.ID = "machine-two"
	secondReport.Host.Hostname = "host-two"
	secondReport.Timestamp = now.Add(30 * time.Second)

	hostTwo, err := monitor.ApplyHostReport(secondReport, token)
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo: %v", err)
	}
	if hostTwo.ID == "" {
		t.Fatalf("expected hostTwo to have an identifier")
	}
	if hostTwo.ID == hostOne.ID {
		t.Fatalf("expected different host IDs for different machines, got %q", hostTwo.ID)
	}

	snapshot := monitor.state.GetSnapshot()
	if got := len(snapshot.Hosts); got != 2 {
		t.Fatalf("expected 2 hosts in state, got %d", got)
	}
}

func TestApplyHostReportPreservesThermalState(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	warningLevel := 1
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "mac-agent", Version: "1.0.0", IntervalSeconds: 30},
		Host: agentshost.HostInfo{
			ID:       "mac-machine",
			Hostname: "mac-mini",
			Platform: "macos",
		},
		Timestamp: time.Now().UTC(),
		Sensors: agentshost.Sensors{
			ThermalState: &agentshost.ThermalState{
				Source:              "pmset",
				Pressure:            agentshost.ThermalPressureConstrained,
				ThermalWarningLevel: &warningLevel,
				LimitsPercent:       map[string]int{"cpu_speed_limit": 72},
			},
		},
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.Sensors.ThermalState == nil {
		t.Fatalf("expected thermal state on applied host: %+v", host.Sensors)
	}
	report.Sensors.ThermalState.LimitsPercent["cpu_speed_limit"] = 99
	*report.Sensors.ThermalState.ThermalWarningLevel = 2

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.Hosts) != 1 || snapshot.Hosts[0].Sensors.ThermalState == nil {
		t.Fatalf("expected one host with thermal state, got %+v", snapshot.Hosts)
	}
	state := snapshot.Hosts[0].Sensors.ThermalState
	if state.Pressure != agentshost.ThermalPressureConstrained {
		t.Fatalf("pressure = %q, want constrained", state.Pressure)
	}
	if got := state.LimitsPercent["cpu_speed_limit"]; got != 72 {
		t.Fatalf("thermal limit = %d, want copied value 72", got)
	}
	if state.ThermalWarningLevel == nil || *state.ThermalWarningLevel != 1 {
		t.Fatalf("thermal warning level = %+v, want copied value 1", state.ThermalWarningLevel)
	}
}

func TestApplyHostReportUsesReceiptTimeForSkewedAgentClockLiveness(t *testing.T) {
	monitor := newTestMonitor(t)

	agentClock := time.Now().UTC().Add(-3 * time.Hour)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "clock-skew-agent",
			Version:         "6.0.3",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "clock-skew-machine",
			MachineID: "clock-skew-machine",
			Hostname:  "clock-skew.local",
			Platform:  "linux",
		},
		Timestamp: agentClock,
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 12,
		},
	}

	before := time.Now().UTC()
	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "clock-skew-token"})
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.LastSeen.Before(before) || host.LastSeen.After(after.Add(time.Second)) {
		t.Fatalf("host LastSeen = %s, want server receipt between %s and %s", host.LastSeen, before, after)
	}
	if !host.LastSeen.After(agentClock.Add(2 * time.Hour)) {
		t.Fatalf("host LastSeen followed skewed agent timestamp %s: got %s", agentClock, host.LastSeen)
	}

	readState := monitor.snapshotBackedUnifiedReadState()
	if readState == nil {
		t.Fatal("expected snapshot-backed read state")
	}
	hosts := readState.Hosts()
	if len(hosts) != 1 {
		t.Fatalf("host count = %d, want 1", len(hosts))
	}
	if hosts[0].Status() != unifiedresources.StatusOnline {
		t.Fatalf("canonical host status = %q, want online", hosts[0].Status())
	}
}

func TestApplyHostReportPreservesAgentLifecycleEvidence(t *testing.T) {
	monitor := newTestMonitor(t)
	checkedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:          "lifecycle-agent",
			Version:     "6.0.4",
			UpdatedFrom: "6.0.3",
			AppliedConfig: &agentshost.ConfigFingerprint{
				Version: "pulse-agent-config-v1",
				Hash:    "sha256:applied",
			},
			Update: &agentshost.UpdateStatus{
				State:            "error",
				AutoUpdate:       true,
				AvailableVersion: "6.0.5",
				LastCheckedAt:    &checkedAt,
				LastError:        "signature verification failed",
			},
			Modules: []agentshost.ModuleStatus{{
				Name: "docker", Enabled: true, State: "retrying", LastError: "socket unavailable", UpdatedAt: checkedAt,
			}},
		},
		Host: agentshost.HostInfo{ID: "machine-lifecycle", Hostname: "lifecycle.local", Platform: "linux"},
	}

	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "lifecycle-token"})
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.AppliedConfig == nil || host.AppliedConfig.Version != "pulse-agent-config-v1" || host.AppliedConfig.Hash != "sha256:applied" {
		t.Fatalf("applied config = %+v", host.AppliedConfig)
	}
	if host.AgentUpdate == nil || host.AgentUpdate.State != "error" || host.AgentUpdate.LastCheckedAt == nil || !host.AgentUpdate.LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("agent update = %+v", host.AgentUpdate)
	}
	if host.AgentUpdate.LastError != "signature verification failed" {
		t.Fatalf("last update error = %q", host.AgentUpdate.LastError)
	}
	if len(host.AgentModules) != 1 || host.AgentModules[0].Name != "docker" || host.AgentModules[0].LastError != "socket unavailable" {
		t.Fatalf("agent modules = %+v", host.AgentModules)
	}
	if host.AgentUpdate.UpdatedFrom != "6.0.3" || host.AgentUpdate.LastSuccessAt == nil {
		t.Fatalf("successful restart evidence = %+v", host.AgentUpdate)
	}

	report.Agent.UpdatedFrom = ""
	report.Agent.Update = &agentshost.UpdateStatus{State: "idle", AutoUpdate: true}
	host, err = monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "lifecycle-token"})
	if err != nil {
		t.Fatalf("ApplyHostReport second report: %v", err)
	}
	if host.AgentUpdate == nil || host.AgentUpdate.UpdatedFrom != "6.0.3" || host.AgentUpdate.LastSuccessAt == nil {
		t.Fatalf("second report lost successful restart evidence: %+v", host.AgentUpdate)
	}
}

func TestApplyDockerReportUsesReceiptTimeForSkewedAgentClockLiveness(t *testing.T) {
	monitor := newTestMonitor(t)

	agentClock := time.Now().UTC().Add(-3 * time.Hour)
	before := time.Now().UTC()
	host, err := monitor.ApplyDockerReport(agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "docker-clock-skew-agent",
			Version:         "6.0.3",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:  "docker-clock-skew.local",
			MachineID: "docker-clock-skew-machine",
		},
		Timestamp: agentClock,
	}, &config.APITokenRecord{ID: "docker-clock-skew-token"})
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}
	if host.LastSeen.Before(before) || host.LastSeen.After(after.Add(time.Second)) {
		t.Fatalf("docker host LastSeen = %s, want server receipt between %s and %s", host.LastSeen, before, after)
	}
	if !host.LastSeen.After(agentClock.Add(2 * time.Hour)) {
		t.Fatalf("docker host LastSeen followed skewed agent timestamp %s: got %s", agentClock, host.LastSeen)
	}
}

func TestApplyDockerReportPreservesNativeRuntimeInventory(t *testing.T) {
	monitor := newTestMonitor(t)
	now := time.Date(2026, 5, 24, 8, 0, 0, 0, time.UTC)
	updatedAt := now.Add(time.Minute)
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "docker-agent-1", Version: "1.0.0", IntervalSeconds: 30},
		Host: agentsdocker.HostInfo{
			Hostname:       "docker-host",
			Name:           "Docker Host",
			Runtime:        "podman",
			RuntimeVersion: "5.0.0",
		},
		Images: []agentsdocker.Image{{
			ID:              "sha256:image1",
			RepoTags:        []string{"repo/app:latest"},
			RepoDigests:     []string{"repo/app@sha256:abc"},
			SizeBytes:       1024,
			SharedSizeBytes: 128,
			Containers:      2,
			CreatedAt:       now,
			Labels:          map[string]string{"tier": "web"},
		}},
		Volumes: []agentsdocker.Volume{{
			Name: "app-data", Driver: "local", Mountpoint: "/var/lib/volumes/app-data", Scope: "local", SizeBytes: 2048, RefCount: 3,
		}},
		Networks: []agentsdocker.Network{{
			ID: "net1", Name: "app-net", Driver: "bridge", Scope: "local", EnableIPv4: true, Attachable: true, Subnets: []agentsdocker.NetworkSubnet{{Subnet: "10.88.0.0/24", Gateway: "10.88.0.1"}},
		}},
		StorageUsage: &agentsdocker.StorageUsage{
			Images:     agentsdocker.StorageUsageBucket{TotalCount: 3, ActiveCount: 2, TotalSizeBytes: 4096, ReclaimableBytes: 512},
			Volumes:    agentsdocker.StorageUsageBucket{TotalCount: 1, ActiveCount: 1, TotalSizeBytes: 2048},
			Containers: agentsdocker.StorageUsageBucket{TotalCount: 2, ActiveCount: 2},
		},
		Secrets: []agentsdocker.Secret{{
			ID: "secret1", Name: "api-token", DriverName: "vault", TemplatingDriver: "", CreatedAt: now, UpdatedAt: &updatedAt, Labels: map[string]string{"stack": "ops"},
		}},
		Configs: []agentsdocker.Config{{
			ID: "config1", Name: "nginx-conf", TemplatingDriver: "golang", CreatedAt: now, UpdatedAt: &updatedAt, Labels: map[string]string{"stack": "frontend"},
		}},
		Timestamp: now,
	}

	host, err := monitor.ApplyDockerReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport: %v", err)
	}
	if len(host.Images) != 1 || host.Images[0].ID != "sha256:image1" || host.Images[0].RepoTags[0] != "repo/app:latest" {
		t.Fatalf("expected image inventory, got %+v", host.Images)
	}
	if len(host.Volumes) != 1 || host.Volumes[0].Name != "app-data" || host.Volumes[0].SizeBytes != 2048 {
		t.Fatalf("expected volume inventory, got %+v", host.Volumes)
	}
	if len(host.Networks) != 1 || host.Networks[0].Name != "app-net" || host.Networks[0].Subnets[0].Gateway != "10.88.0.1" {
		t.Fatalf("expected network inventory, got %+v", host.Networks)
	}
	if host.StorageUsage == nil || host.StorageUsage.Images.ReclaimableBytes != 512 {
		t.Fatalf("expected storage usage inventory, got %+v", host.StorageUsage)
	}
	if len(host.Secrets) != 1 || host.Secrets[0].Name != "api-token" || host.Secrets[0].DriverName != "vault" {
		t.Fatalf("expected secret metadata inventory, got %+v", host.Secrets)
	}
	if len(host.Configs) != 1 || host.Configs[0].Name != "nginx-conf" || host.Configs[0].TemplatingDriver != "golang" {
		t.Fatalf("expected config metadata inventory, got %+v", host.Configs)
	}
}

func TestApplyDockerReportDerivesCanonicalDockerSecurityPosture(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:               models.NewState(),
		alertManager:        alerts.NewManager(),
		config:              &config.Config{},
		rateTracker:         NewRateTracker(),
		dockerMetadataStore: config.NewDockerMetadataStore(t.TempDir(), nil),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	host, err := monitor.ApplyDockerReport(agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "agent-secure",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname: "docker-secure-host",
			Runtime:  "docker",
			Security: &agentsdocker.HostSecurityInfo{
				AuthorizationPlugins: []string{"opa", " audit "},
			},
		},
		Timestamp: time.Now().UTC(),
	}, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport returned error: %v", err)
	}

	if host.Security == nil {
		t.Fatalf("expected security posture on docker host")
	}
	if !host.Security.MutatingCommandsBlocked {
		t.Fatalf("expected mutating commands to be blocked")
	}
	if got := host.Security.AuthorizationPlugins; len(got) != 2 || got[0] != "opa" || got[1] != "audit" {
		t.Fatalf("expected normalized authorization plugins, got %#v", got)
	}
	if !strings.Contains(host.Security.MutatingCommandsBlockedReason, "GO-2026-4887") {
		t.Fatalf("expected advisory reason, got %q", host.Security.MutatingCommandsBlockedReason)
	}
}

func TestHostReportMatchesKnownIdentityUsesPersistedContinuity(t *testing.T) {
	now := time.Now().UTC()
	store := config.NewHostContinuityStore(t.TempDir(), nil)
	if err := store.Upsert(config.HostContinuityEntry{
		HostID:          "host-1",
		ReportHostID:    "machine-1",
		AgentReportedID: "agent-1",
		Hostname:        "host-1.local",
		MachineID:       "machine-1",
		TokenID:         "token-1",
		LastSeen:        now,
	}); err != nil {
		t.Fatalf("Upsert continuity: %v", err)
	}

	monitor := &Monitor{
		state:               models.NewState(),
		hostContinuityStore: store,
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:      "agent-1",
			Version: "6.0.0-rc.1",
		},
		Host: agentshost.HostInfo{
			ID:        "machine-1",
			MachineID: "machine-1",
			Hostname:  "host-1.local",
			Platform:  "linux",
		},
		Timestamp: now.Add(time.Minute),
	}

	if !monitor.HostReportMatchesKnownIdentity(report, &config.APITokenRecord{ID: "token-1"}) {
		t.Fatal("expected persisted continuity to count as a known host identity")
	}
}

func TestApplyHostReportReusesPersistedContinuityAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	newMonitor := func() *Monitor {
		monitor := &Monitor{
			state:               models.NewState(),
			alertManager:        alerts.NewManager(),
			hostTokenBindings:   make(map[string]string),
			config:              &config.Config{DataPath: dir},
			rateTracker:         NewRateTracker(),
			hostContinuityStore: config.NewHostContinuityStore(dir, nil),
		}
		t.Cleanup(func() { monitor.alertManager.Stop() })
		return monitor
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-1",
			Version:         "6.0.0-rc.1",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-1",
			MachineID: "machine-1",
			Hostname:  "host-1.local",
			Platform:  "linux",
		},
		Timestamp: now,
	}
	token := &config.APITokenRecord{ID: "token-1", Name: "Token One"}

	firstMonitor := newMonitor()
	firstHost, err := firstMonitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("first ApplyHostReport: %v", err)
	}

	restartedMonitor := newMonitor()
	restartReport := report
	restartReport.Timestamp = now.Add(30 * time.Second)
	restartedHost, err := restartedMonitor.ApplyHostReport(restartReport, token)
	if err != nil {
		t.Fatalf("restarted ApplyHostReport: %v", err)
	}

	if restartedHost.ID != firstHost.ID {
		t.Fatalf("expected persisted continuity to preserve host ID %q, got %q", firstHost.ID, restartedHost.ID)
	}
}

func TestApplyHostReportReusesTokenBindingAcrossShortFQDNAfterReload(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	token := &config.APITokenRecord{ID: "token-host-continuity", Name: "Host Token"}
	monitor.state.UpsertHost(models.Host{
		ID:              "host-v5",
		Hostname:        "docker-lxc.lab",
		DisplayName:     "docker-lxc",
		TokenID:         token.ID,
		Status:          "online",
		AgentVersion:    "5.1.30",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-time.Minute),
	})
	monitor.hostTokenBindings[token.ID+":docker-lxc.lab"] = "host-v5"

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			Version:         "6.0.0-rc.4",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			Hostname: "docker-lxc",
			Platform: "linux",
			OSName:   "debian",
		},
		Timestamp: now,
	}

	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.ID != "host-v5" {
		t.Fatalf("expected equivalent hostname token binding to preserve host ID host-v5, got %q", host.ID)
	}

	snapshot := monitor.state.GetSnapshot()
	if got := len(snapshot.Hosts); got != 1 {
		t.Fatalf("expected host report to update existing host instead of creating duplicate, got %d hosts", got)
	}
	if got := monitor.hostTokenBindings[token.ID+":docker-lxc"]; got != "host-v5" {
		t.Fatalf("expected current hostname alias to bind to host-v5, got %q", got)
	}
}

func TestApplyHostReportDisambiguatesCollidingIdentifiersAcrossTokens(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-one",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "colliding-machine-id",
			Hostname:  "nas-one",
			Platform:  "linux",
			OSName:    "synology",
			OSVersion: "7.0",
		},
		Timestamp: now,
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 1.0,
		},
	}

	hostOne, err := monitor.ApplyHostReport(baseReport, &config.APITokenRecord{ID: "token-one"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostOne: %v", err)
	}
	if hostOne.ID == "" {
		t.Fatalf("expected hostOne to have an identifier")
	}

	secondReport := baseReport
	secondReport.Agent.ID = "agent-two"
	secondReport.Host.Hostname = "nas-two"
	secondReport.Timestamp = now.Add(30 * time.Second)

	hostTwo, err := monitor.ApplyHostReport(secondReport, &config.APITokenRecord{ID: "token-two"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo: %v", err)
	}
	if hostTwo.ID == "" {
		t.Fatalf("expected hostTwo to have an identifier")
	}
	if hostTwo.ID == hostOne.ID {
		t.Fatalf("expected disambiguated host IDs, got %q", hostTwo.ID)
	}

	hostTwoRepeat, err := monitor.ApplyHostReport(secondReport, &config.APITokenRecord{ID: "token-two"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo repeat: %v", err)
	}
	if hostTwoRepeat.ID != hostTwo.ID {
		t.Fatalf("expected stable host ID for repeated reports, got %q want %q", hostTwoRepeat.ID, hostTwo.ID)
	}

	// Removing the first host should not cause the second host to change identity.
	if _, err := monitor.RemoveHostAgent(hostOne.ID); err != nil {
		t.Fatalf("RemoveHostAgent hostOne: %v", err)
	}

	hostTwoAfterRemoval, err := monitor.ApplyHostReport(secondReport, &config.APITokenRecord{ID: "token-two"})
	if err != nil {
		t.Fatalf("ApplyHostReport hostTwo after removal: %v", err)
	}
	if hostTwoAfterRemoval.ID != hostTwo.ID {
		t.Fatalf("expected stable host ID after removal, got %q want %q", hostTwoAfterRemoval.ID, hostTwo.ID)
	}

	snapshot := monitor.state.GetSnapshot()
	if got := len(snapshot.Hosts); got != 1 {
		t.Fatalf("expected 1 host in state after removal, got %d", got)
	}
}

func TestRemoveHostAgent_DoesNotBlockDistinctLiveHostWithSameHostname(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	staleReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "old-agent",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "shared-machine",
			MachineID: "shared-machine",
			Hostname:  "pve-node",
			Platform:  "linux",
		},
		Timestamp: now,
	}
	liveReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "new-agent",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "shared-machine",
			MachineID: "shared-machine",
			Hostname:  "pve-node",
			Platform:  "linux",
		},
		Timestamp: now.Add(30 * time.Second),
	}

	staleHost, err := monitor.ApplyHostReport(staleReport, &config.APITokenRecord{ID: "old-token"})
	if err != nil {
		t.Fatalf("ApplyHostReport stale host: %v", err)
	}
	liveHost, err := monitor.ApplyHostReport(liveReport, &config.APITokenRecord{ID: "new-token"})
	if err != nil {
		t.Fatalf("ApplyHostReport live host: %v", err)
	}
	if liveHost.ID == staleHost.ID {
		t.Fatalf("expected distinct duplicate host IDs, got %q", liveHost.ID)
	}

	if _, err := monitor.RemoveHostAgent(staleHost.ID); err != nil {
		t.Fatalf("RemoveHostAgent stale host: %v", err)
	}

	liveReport.Timestamp = now.Add(time.Minute)
	liveHostAfterRemoval, err := monitor.ApplyHostReport(liveReport, &config.APITokenRecord{ID: "new-token"})
	if err != nil {
		t.Fatalf("live host with same hostname should not be blocked by stale removal: %v", err)
	}
	if liveHostAfterRemoval.ID != liveHost.ID {
		t.Fatalf("expected live host ID %q to remain stable, got %q", liveHost.ID, liveHostAfterRemoval.ID)
	}
}

func TestRemoveHostAgent_BlocksRemovedTokenMachineAfterIdentifierChurn(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	hostID := "agent-id-before-reinstall"
	monitor.state.UpsertHost(models.Host{
		ID:        hostID,
		Hostname:  "blocked-node",
		MachineID: "machine-stable",
		TokenID:   "host-token",
		Status:    "offline",
		LastSeen:  now.Add(-time.Minute),
	})

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-id-after-reinstall",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-stable",
			MachineID: "machine-stable",
			Hostname:  "blocked-node",
			Platform:  "linux",
		},
		Timestamp: now,
	}

	if _, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "host-token"}); err == nil {
		t.Fatal("expected stable token and machine identity to remain blocked after host ID changed")
	}
}

func TestRemoveHostAgentUnbindsToken(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-to-remove"
	tokenID := "token-remove"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "remove.me",
		TokenID:  tokenID,
	})
	monitor.hostTokenBindings[tokenID+":remove.me"] = hostID
	monitor.hostTokenBindings[tokenID+":remove.me.local"] = hostID
	monitor.hostTokenBindings[tokenID] = hostID

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	if _, exists := monitor.hostTokenBindings[tokenID+":remove.me"]; exists {
		t.Fatalf("expected token binding to be cleared after host removal")
	}
	if _, exists := monitor.hostTokenBindings[tokenID]; exists {
		t.Fatalf("expected legacy token binding to be cleared after host removal")
	}
	if _, exists := monitor.hostTokenBindings[tokenID+":remove.me.local"]; exists {
		t.Fatalf("expected equivalent hostname token binding to be cleared after host removal")
	}
}

func TestRemoveHostAgent_PreservesLinkedGuestIdentityInRemovedState(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-linked-guest"
	monitor.state.UpsertHost(models.Host{
		ID:                hostID,
		Hostname:          "guest-host.local",
		DisplayName:       "guest-host",
		LinkedVMID:        "101",
		LinkedContainerID: "102",
	})

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	removedHosts := monitor.state.GetRemovedHostAgents()
	if len(removedHosts) != 1 {
		t.Fatalf("expected one removed host entry, got %d", len(removedHosts))
	}
	if removedHosts[0].LinkedVMID != "101" {
		t.Fatalf("expected linked VM id to persist, got %q", removedHosts[0].LinkedVMID)
	}
	if removedHosts[0].LinkedContainerID != "102" {
		t.Fatalf(
			"expected linked container id to persist, got %q",
			removedHosts[0].LinkedContainerID,
		)
	}
}

func TestRemoveHostAgent_KeepsSharedTokenUsedByDockerRuntime(t *testing.T) {
	t.Helper()

	tokenID := "shared-token"
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config: &config.Config{
			APITokens: []config.APITokenRecord{
				{ID: tokenID, Name: "Shared Token"},
			},
		},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-shared"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "shared-host.local",
		TokenID:  tokenID,
	})
	monitor.state.UpsertDockerHost(models.DockerHost{
		ID:       "docker-shared",
		Hostname: "docker-shared.local",
		TokenID:  tokenID,
		Status:   "online",
	})
	monitor.hostTokenBindings[tokenID+":shared-host.local"] = hostID

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	if got := len(monitor.config.APITokens); got != 1 {
		t.Fatalf("expected shared API token to remain, got %d tokens", got)
	}
	if monitor.config.APITokens[0].ID != tokenID {
		t.Fatalf("expected shared token %q to remain, got %q", tokenID, monitor.config.APITokens[0].ID)
	}
}

func TestApplyHostReport_PreservesPreviousTokenMetadata(t *testing.T) {
	t.Helper()

	lastUsed := time.Now().UTC().Add(-5 * time.Minute)
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	monitor.state.UpsertHost(models.Host{
		ID:              "host-prev",
		Hostname:        "preserve.local",
		TokenID:         "token-prev",
		TokenName:       "Previous Token",
		TokenHint:       "prev_1234",
		TokenLastUsedAt: &lastUsed,
	})

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-prev",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:       "host-prev",
			Hostname: "preserve.local",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if host.TokenID != "token-prev" || host.TokenName != "Previous Token" || host.TokenHint != "prev_1234" {
		t.Fatalf("expected previous token metadata to be preserved, got id=%q name=%q hint=%q", host.TokenID, host.TokenName, host.TokenHint)
	}
	if host.TokenLastUsedAt == nil || !host.TokenLastUsedAt.Equal(lastUsed) {
		t.Fatalf("expected TokenLastUsedAt %v, got %v", lastUsed, host.TokenLastUsedAt)
	}
}

func TestApplyHostReportStoresUnraidTopology(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower",
			Hostname:  "tower",
			MachineID: "machine-tower",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 55,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Device: "/dev/sdb", Role: "parity", Status: "online", RawStatus: "DISK_OK", Serial: "SERIAL-PARITY"},
				{
					Name:        "disk1",
					Device:      "/dev/sdc",
					Role:        "data",
					Status:      "online",
					RawStatus:   "DISK_OK",
					Model:       "WDC WD60EFRX",
					Serial:      "SERIAL-DATA",
					Filesystem:  "xfs",
					Transport:   "sata",
					SizeBytes:   6_000_000_000_000,
					UsedBytes:   4_000,
					FreeBytes:   2_000,
					Temperature: 31,
					SpunDown:    true,
					ReadCount:   11,
					WriteCount:  12,
					ErrorCount:  16,
					Slot:        1,
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if host.Unraid == nil {
		t.Fatal("expected unraid topology on host")
	}
	if !host.Unraid.ArrayStarted || host.Unraid.SyncAction != "check" {
		t.Fatalf("unexpected unraid summary %+v", host.Unraid)
	}
	if len(host.Unraid.Disks) != 2 || host.Unraid.Disks[0].Role != "parity" {
		t.Fatalf("unexpected unraid disks %+v", host.Unraid.Disks)
	}
	disk := host.Unraid.Disks[1]
	if disk.Model != "WDC WD60EFRX" || disk.Transport != "sata" || disk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("expected enriched unraid disk metadata, got %+v", disk)
	}
	if disk.UsedBytes != 4_000 || disk.FreeBytes != 2_000 || disk.Temperature != 31 || !disk.SpunDown {
		t.Fatalf("expected native unraid capacity and state fields, got %+v", disk)
	}
	if disk.ReadCount != 11 || disk.WriteCount != 12 || disk.ErrorCount != 16 {
		t.Fatalf("expected native unraid counters, got %+v", disk)
	}
}

func TestApplyHostReportNormalizesLegacyUnraidRawStatuses(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower-legacy",
			Version:         "5.1.27",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower-legacy",
			Hostname:  "tower",
			MachineID: "machine-tower-legacy",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 55,
			NumDisabled:  1,
			NumInvalid:   1,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Device: "/dev/sdb", Role: "parity", RawStatus: "DISK_OK", Serial: "SERIAL-PARITY"},
				{Name: "disk1", Device: "/dev/sdc", Role: "data", RawStatus: "DISK_OK", Serial: "SERIAL-DATA"},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if host.Unraid == nil {
		t.Fatal("expected unraid topology on host")
	}
	for _, disk := range host.Unraid.Disks {
		if disk.Status != "online" {
			t.Fatalf("expected normalized online status from rawStatus, got %+v", host.Unraid.Disks)
		}
	}

	assessment := storagehealth.AssessUnraidStorage(*host.Unraid)
	if assessment.Level != storagehealth.RiskWarning {
		t.Fatalf("assessment level = %q, want %q", assessment.Level, storagehealth.RiskWarning)
	}
	for _, reason := range assessment.Reasons {
		if reason.Code == "unraid_disabled_disks" || reason.Code == "unraid_invalid_disks" {
			t.Fatalf("unexpected aggregate-count reason after raw-status normalization: %+v", assessment.Reasons)
		}
	}
}

func TestApplyHostReportFiltersLegacyUnraidEmptySlots(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower-empty-slots",
			Version:         "5.1.27",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower-empty-slots",
			Hostname:  "tower",
			MachineID: "machine-tower-empty-slots",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Unraid: &agentshost.UnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			NumDisabled:  2,
			NumInvalid:   2,
			Disks: []agentshost.UnraidDisk{
				{Name: "parity", Role: "parity", RawStatus: "DISK_NP_DSBL"},
				{Name: "md1p1", Device: "/dev/sde", RawStatus: "DISK_OK", SizeBytes: 5860522532},
				{RawStatus: "DISK_NP", Slot: 5},
				{RawStatus: "DISK_NP_DSBL", Slot: 29},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if host.Unraid == nil {
		t.Fatal("expected unraid topology on host")
	}
	if len(host.Unraid.Disks) != 1 {
		t.Fatalf("unraid disk count = %d, want only assigned disks: %+v", len(host.Unraid.Disks), host.Unraid.Disks)
	}
	if got := host.Unraid.Disks[0]; got.Device != "/dev/sde" || got.Status != "online" {
		t.Fatalf("unexpected assigned disk: %+v", got)
	}
}

func TestApplyHostReportFiltersVendorManagedSystemRAIDArrays(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	testCases := []struct {
		name        string
		hostInfo    agentshost.HostInfo
		reportRAID  []agentshost.RAIDArray
		wantDevices []string
	}{
		{
			name: "synology suppresses md0 and md1",
			hostInfo: agentshost.HostInfo{
				ID:       "machine-synology",
				Hostname: "synology.local",
				Platform: "linux",
				OSName:   "Synology DSM",
			},
			reportRAID: []agentshost.RAIDArray{
				{Device: "/dev/md0", Level: "raid1", State: "degraded", FailedDevices: 1},
				{Device: "/dev/md1", Level: "raid1", State: "resyncing", RebuildPercent: 42},
				{Device: "/dev/md2", Level: "raid5", State: "clean"},
			},
			wantDevices: []string{"/dev/md2"},
		},
		{
			name: "qnap suppresses md9 and md13",
			hostInfo: agentshost.HostInfo{
				ID:       "machine-qnap",
				Hostname: "qnap.local",
				Platform: "linux",
				OSName:   "QNAP QTS",
			},
			reportRAID: []agentshost.RAIDArray{
				{Device: "/dev/md9", Level: "raid1", State: "degraded", FailedDevices: 1},
				{Device: "/dev/md13", Level: "raid1", State: "clean"},
				{Device: "/dev/md2", Level: "raid5", State: "clean"},
			},
			wantDevices: []string{"/dev/md2"},
		},
		{
			name: "generic hosts keep md0",
			hostInfo: agentshost.HostInfo{
				ID:       "machine-generic",
				Hostname: "generic.local",
				Platform: "linux",
				OSName:   "Ubuntu",
			},
			reportRAID: []agentshost.RAIDArray{
				{Device: "/dev/md0", Level: "raid1", State: "clean"},
				{Device: "/dev/md2", Level: "raid5", State: "clean"},
			},
			wantDevices: []string{"/dev/md0", "/dev/md2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			report := agentshost.Report{
				Agent: agentshost.AgentInfo{
					ID:              "agent-" + tc.hostInfo.ID,
					Version:         "1.0.0",
					IntervalSeconds: 30,
				},
				Host:      tc.hostInfo,
				Timestamp: now,
				Metrics: agentshost.Metrics{
					CPUUsagePercent: 5.5,
				},
				RAID: tc.reportRAID,
			}

			host, err := monitor.ApplyHostReport(report, nil)
			if err != nil {
				t.Fatalf("ApplyHostReport: %v", err)
			}

			if len(host.RAID) != len(tc.wantDevices) {
				t.Fatalf("host RAID count = %d, want %d", len(host.RAID), len(tc.wantDevices))
			}
			for i, device := range tc.wantDevices {
				if host.RAID[i].Device != device {
					t.Fatalf("host RAID[%d] = %q, want %q", i, host.RAID[i].Device, device)
				}
			}

			snapshot := monitor.state.GetSnapshot()
			var stored *models.Host
			for i := range snapshot.Hosts {
				if snapshot.Hosts[i].ID == host.ID {
					stored = &snapshot.Hosts[i]
					break
				}
			}
			if stored == nil {
				t.Fatalf("stored host %q not found", host.ID)
			}
			if len(stored.RAID) != len(tc.wantDevices) {
				t.Fatalf("stored RAID count = %d, want %d", len(stored.RAID), len(tc.wantDevices))
			}
			for i, device := range tc.wantDevices {
				if stored.RAID[i].Device != device {
					t.Fatalf("stored RAID[%d] = %q, want %q", i, stored.RAID[i].Device, device)
				}
			}
		})
	}
}

func TestApplyHostReportKeepsLocalMergerFSMounts(t *testing.T) {
	t.Helper()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-mergerfs-host",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-mergerfs-host",
			Hostname:  "mergerfs-host",
			MachineID: "machine-mergerfs-host",
		},
		Disks: []agentshost.Disk{
			{
				Device:     "mergerfs",
				Mountpoint: "/mnt/storage",
				Type:       "fuse.mergerfs",
				TotalBytes: 10_000,
				UsedBytes:  4_000,
				FreeBytes:  6_000,
				Usage:      40,
			},
			{
				Device:     "sshfs",
				Mountpoint: "/mnt/remote",
				Type:       "fuse.sshfs",
				TotalBytes: 10_000,
				UsedBytes:  4_000,
				FreeBytes:  6_000,
				Usage:      40,
			},
		},
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	if len(host.Disks) != 1 {
		t.Fatalf("host disk count = %d, want 1 (%+v)", len(host.Disks), host.Disks)
	}
	if host.Disks[0].Type != "fuse.mergerfs" || host.Disks[0].Mountpoint != "/mnt/storage" {
		t.Fatalf("unexpected retained disk %+v", host.Disks[0])
	}

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.Hosts) != 1 {
		t.Fatalf("snapshot host count = %d, want 1", len(snapshot.Hosts))
	}
	if len(snapshot.Hosts[0].Disks) != 1 {
		t.Fatalf("stored host disk count = %d, want 1 (%+v)", len(snapshot.Hosts[0].Disks), snapshot.Hosts[0].Disks)
	}
	if snapshot.Hosts[0].Disks[0].Type != "fuse.mergerfs" || snapshot.Hosts[0].Disks[0].Mountpoint != "/mnt/storage" {
		t.Fatalf("unexpected stored retained disk %+v", snapshot.Hosts[0].Disks[0])
	}
}

func TestApplyHostReportPersistsSMARTMetricsForAgentDisks(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	powerOnHours := int64(1234)
	reallocated := int64(2)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower",
			Hostname:  "tower",
			MachineID: "machine-tower",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/sda",
					Model:       "IronWolf",
					Serial:      "SERIAL-TOWER-1",
					Temperature: 41,
					Attributes: &agentshost.SMARTAttributes{
						PowerOnHours:       &powerOnHours,
						ReallocatedSectors: &reallocated,
					},
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	points := waitForStoredDiskMetric(t, store, "SERIAL-TOWER-1", "smart_temp")
	if len(points) == 0 {
		t.Fatal("expected SMART temperature metric for agent disk")
	}

	points = waitForStoredDiskMetric(t, store, "SERIAL-TOWER-1", "smart_power_on_hours")
	if len(points) == 0 || points[len(points)-1].Value != float64(powerOnHours) {
		t.Fatalf("expected power-on-hours metric %.0f, got %+v", float64(powerOnHours), points)
	}

	points = waitForStoredDiskMetric(t, store, "SERIAL-TOWER-1", "smart_reallocated_sectors")
	if len(points) == 0 || points[len(points)-1].Value != float64(reallocated) {
		t.Fatalf("expected reallocated-sectors metric %.0f, got %+v", float64(reallocated), points)
	}
}

func TestApplyHostReportPersistsAgentTemperatureMetric(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(10, time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-thermal",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-thermal",
			Hostname:  "thermal-node",
			MachineID: "machine-thermal",
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 12.5,
			Memory:          agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			TemperatureCelsius: map[string]float64{
				"cpu_core_0":  59,
				"cpu_package": 62.5,
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	storePoints := waitForStoredMetric(t, store, "agent", host.ID, "temperature")
	if got := storePoints[len(storePoints)-1].Value; got != 62.5 {
		t.Fatalf("expected persisted temperature 62.5, got %f", got)
	}

	historyPoints := monitor.GetGuestMetrics("agent:"+host.ID, time.Hour)["temperature"]
	if len(historyPoints) == 0 || historyPoints[len(historyPoints)-1].Value != 62.5 {
		t.Fatalf("expected in-memory temperature 62.5, got %+v", historyPoints)
	}
}

func TestApplyHostReportPreservesGPUSensorSummary(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	temperature := 63.0
	utilization := 7.0
	usedBytes := int64(2 * 1024 * 1024 * 1024)
	totalBytes := int64(48 * 1024 * 1024 * 1024)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-gpu",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-gpu",
			Hostname:  "gpu-node",
			MachineID: "machine-gpu",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			TemperatureCelsius: map[string]float64{"gpu_nvidia_0": temperature},
			GPU: []agentshost.GPUSensor{
				{
					ID:                 "0",
					Name:               "NVIDIA RTX A6000",
					TemperatureCelsius: &temperature,
					UtilizationPercent: &utilization,
					MemoryUsedBytes:    &usedBytes,
					MemoryTotalBytes:   &totalBytes,
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if len(host.Sensors.GPU) != 1 {
		t.Fatalf("host GPU sensors = %+v, want one sensor", host.Sensors.GPU)
	}
	gpu := host.Sensors.GPU[0]
	if gpu.ID != "0" || gpu.Name != "NVIDIA RTX A6000" {
		t.Fatalf("unexpected GPU identity: %+v", gpu)
	}
	if gpu.TemperatureCelsius == nil || *gpu.TemperatureCelsius != temperature {
		t.Fatalf("GPU temperature = %#v, want %.1f", gpu.TemperatureCelsius, temperature)
	}
	if gpu.UtilizationPercent == nil || *gpu.UtilizationPercent != utilization {
		t.Fatalf("GPU utilization = %#v, want %.1f", gpu.UtilizationPercent, utilization)
	}
	if gpu.MemoryUsedBytes == nil || *gpu.MemoryUsedBytes != usedBytes {
		t.Fatalf("GPU used memory = %#v, want %d", gpu.MemoryUsedBytes, usedBytes)
	}
	if host.Sensors.TemperatureCelsius["gpu_nvidia_0"] != temperature {
		t.Fatalf("legacy GPU temperature compatibility = %+v, want %.1f", host.Sensors.TemperatureCelsius, temperature)
	}

	projected := hostSensorsFromReadStateView(&unifiedresources.HostSensorMeta{
		GPU: []unifiedresources.HostGPUSensor{
			{
				ID:                 "0",
				Name:               "NVIDIA RTX A6000",
				TemperatureCelsius: &temperature,
				UtilizationPercent: &utilization,
				MemoryUsedBytes:    &usedBytes,
				MemoryTotalBytes:   &totalBytes,
			},
		},
	})
	if len(projected.GPU) != 1 || projected.GPU[0].MemoryTotalBytes == nil || *projected.GPU[0].MemoryTotalBytes != totalBytes {
		t.Fatalf("read-state GPU projection = %+v, want total VRAM %d", projected.GPU, totalBytes)
	}
}

func TestApplyHostReportPreservesPowerSensorSummary(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-power",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-power",
			Hostname:  "power-node",
			MachineID: "machine-power",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			PowerWatts: map[string]float64{"cpu_package": 82.4, "dram": 13.2},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if got := host.Sensors.PowerWatts["cpu_package"]; got != 82.4 {
		t.Fatalf("host power sensor = %.1f, want 82.4", got)
	}
	report.Sensors.PowerWatts["cpu_package"] = 1
	if got := host.Sensors.PowerWatts["cpu_package"]; got != 82.4 {
		t.Fatalf("host power sensors share report map, got %.1f want 82.4", got)
	}

	projected := hostSensorsFromReadStateView(&unifiedresources.HostSensorMeta{
		PowerWatts: map[string]float64{"cpu_package": 82.4},
	})
	if got := projected.PowerWatts["cpu_package"]; got != 82.4 {
		t.Fatalf("read-state power projection = %.1f, want 82.4", got)
	}
}

func TestApplyHostReportPersistsSMARTMetricsForAgentDisksWithFallbackID(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	mediaErrors := int64(7)
	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-tower-fallback",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-tower",
			Hostname:  "tower",
			MachineID: "machine-tower",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/nvme0n1",
					Model:       "CacheDisk",
					Temperature: 39,
					Attributes: &agentshost.SMARTAttributes{
						MediaErrors: &mediaErrors,
					},
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	resourceID := "machine-tower:nvme0n1"
	points := waitForStoredDiskMetric(t, store, resourceID, "smart_temp")
	if len(points) == 0 {
		t.Fatal("expected SMART temperature metric for fallback-id agent disk")
	}

	points = waitForStoredDiskMetric(t, store, resourceID, "smart_media_errors")
	if len(points) == 0 || points[len(points)-1].Value != float64(mediaErrors) {
		t.Fatalf("expected media-errors metric %.0f, got %+v", float64(mediaErrors), points)
	}
}

func TestApplyHostReportPersistsPhysicalDiskIOMetricsForAgentDisks(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(1000, 24*time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-pve2",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "host-pve2",
			Hostname:  "pve2",
			MachineID: "machine-pve2",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/nvme2",
					Model:       "Samsung 980 PRO 2TB",
					Serial:      "SERIAL884006359727",
					Temperature: 46,
				},
			},
		},
		DiskIO: []agentshost.DiskIO{
			{
				Device:     "nvme2",
				ReadBytes:  1_000_000,
				WriteBytes: 2_000_000,
				ReadOps:    100,
				WriteOps:   200,
				IOTime:     10_000,
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(baseReport, nil); err != nil {
		t.Fatalf("ApplyHostReport initial: %v", err)
	}

	nextReport := baseReport
	nextReport.Timestamp = baseReport.Timestamp.Add(30 * time.Second)
	nextReport.DiskIO = []agentshost.DiskIO{
		{
			Device:     "nvme2",
			ReadBytes:  4_000_000,
			WriteBytes: 5_000_000,
			ReadOps:    250,
			WriteOps:   350,
			IOTime:     22_000,
		},
	}

	if _, err := monitor.ApplyHostReport(nextReport, nil); err != nil {
		t.Fatalf("ApplyHostReport second: %v", err)
	}
	store.Flush()

	readPoints := waitForStoredDiskMetric(t, store, "SERIAL884006359727", "diskread")
	writePoints := waitForStoredDiskMetric(t, store, "SERIAL884006359727", "diskwrite")
	busyPoints := waitForStoredDiskMetric(t, store, "SERIAL884006359727", "disk")

	if got := readPoints[len(readPoints)-1].Value; got <= 0 {
		t.Fatalf("expected persisted diskread rate > 0, got %+v", readPoints)
	}
	if got := writePoints[len(writePoints)-1].Value; got <= 0 {
		t.Fatalf("expected persisted diskwrite rate > 0, got %+v", writePoints)
	}
	if got := busyPoints[len(busyPoints)-1].Value; got <= 0 || got > 100 {
		t.Fatalf("expected persisted disk busy percent within (0,100], got %+v", busyPoints)
	}

	if got := monitor.metricsHistory.GetDiskMetrics("SERIAL884006359727", "diskread", time.Hour); len(got) == 0 {
		t.Fatal("expected in-memory diskread history for physical disk")
	}
	if got := monitor.metricsHistory.GetDiskMetrics("SERIAL884006359727", "disk", time.Hour); len(got) == 0 {
		t.Fatal("expected in-memory busy history for physical disk")
	}
}

func TestApplyHostReportPersistsDiskIOMetricsWithoutSMART(t *testing.T) {
	t.Helper()

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(1000, 24*time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	baseReport := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-pve3",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "host-pve3",
			Hostname:  "pve3",
			MachineID: "machine-pve3",
		},
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		DiskIO: []agentshost.DiskIO{
			{
				Device:     "sda",
				ReadBytes:  1_000_000,
				WriteBytes: 2_000_000,
				ReadOps:    100,
				WriteOps:   200,
				IOTime:     10_000,
			},
		},
		Timestamp: time.Now().UTC(),
	}

	if _, err := monitor.ApplyHostReport(baseReport, nil); err != nil {
		t.Fatalf("ApplyHostReport initial: %v", err)
	}

	nextReport := baseReport
	nextReport.Timestamp = baseReport.Timestamp.Add(30 * time.Second)
	nextReport.DiskIO = []agentshost.DiskIO{
		{
			Device:     "sda",
			ReadBytes:  4_000_000,
			WriteBytes: 5_000_000,
			ReadOps:    250,
			WriteOps:   350,
			IOTime:     22_000,
		},
	}

	if _, err := monitor.ApplyHostReport(nextReport, nil); err != nil {
		t.Fatalf("ApplyHostReport second: %v", err)
	}
	store.Flush()

	fallbackID := "host-pve3:sda"

	readPoints := waitForStoredDiskMetric(t, store, fallbackID, "diskread")
	writePoints := waitForStoredDiskMetric(t, store, fallbackID, "diskwrite")
	busyPoints := waitForStoredDiskMetric(t, store, fallbackID, "disk")

	if got := readPoints[len(readPoints)-1].Value; got <= 0 {
		t.Fatalf("expected persisted diskread rate > 0 without SMART, got %+v", readPoints)
	}
	if got := writePoints[len(writePoints)-1].Value; got <= 0 {
		t.Fatalf("expected persisted diskwrite rate > 0 without SMART, got %+v", writePoints)
	}
	if got := busyPoints[len(busyPoints)-1].Value; got <= 0 || got > 100 {
		t.Fatalf("expected persisted disk busy percent within (0,100] without SMART, got %+v", busyPoints)
	}
}

func TestHostDiskIOMetricResourceIDFallbacks(t *testing.T) {
	host := models.Host{
		ID: "myhost",
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "/dev/nvme0n1", Serial: "NVME-SERIAL-123"},
			},
		},
	}

	io := models.DiskIO{Device: "nvme0n1"}

	got := hostDiskIOMetricResourceID(host, io, nil)
	if got != "NVME-SERIAL-123" {
		t.Fatalf("SMART match: expected NVME-SERIAL-123, got %q", got)
	}

	ioNoSMART := models.DiskIO{Device: "sda"}
	got = hostDiskIOMetricResourceID(host, ioNoSMART, nil)
	if got != "myhost:sda" {
		t.Fatalf("no SMART, no proxmox: expected myhost:sda, got %q", got)
	}

	proxmoxDisks := []proxmoxDiskMatch{
		{device: "sda", metricID: "SATA-SERIAL-456"},
	}
	got = hostDiskIOMetricResourceID(host, ioNoSMART, proxmoxDisks)
	if got != "SATA-SERIAL-456" {
		t.Fatalf("proxmox fallback: expected SATA-SERIAL-456, got %q", got)
	}
}

func TestApplyHostReportSkipsMetricsAndSMARTWritesInMockMode(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, true)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })

	storeCfg := metrics.DefaultConfig(t.TempDir())
	storeCfg.WriteBufferSize = 1
	store, err := metrics.NewStore(storeCfg)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
		metricsHistory:    NewMetricsHistory(1000, 24*time.Hour),
		metricsStore:      store,
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-demo",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:        "machine-demo",
			Hostname:  "demo-host",
			MachineID: "machine-demo",
		},
		Metrics: agentshost.Metrics{
			CPUUsagePercent: 44,
			Memory:          agentshost.MemoryMetric{TotalBytes: 1024, UsedBytes: 512, FreeBytes: 512, Usage: 50},
		},
		Sensors: agentshost.Sensors{
			SMART: []agentshost.DiskSMART{
				{
					Device:      "/dev/sda",
					Serial:      "SERIAL-DEMO-1",
					Temperature: 39,
				},
			},
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	store.Flush()

	if got := monitor.metricsHistory.GetGuestMetrics("agent:"+host.ID, "cpu", time.Hour); len(got) != 0 {
		t.Fatalf("expected mock mode to skip host-agent metrics history, got %+v", got)
	}

	now := time.Now().UTC()
	points, err := store.Query("disk", "SERIAL-DEMO-1", "smart_temp", now.Add(-time.Hour), now.Add(time.Hour), 60)
	if err != nil {
		t.Fatalf("Query smart_temp: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("expected mock mode to skip SMART metric persistence, got %+v", points)
	}
}

func waitForStoredDiskMetric(t *testing.T, store *metrics.Store, resourceID, metric string) []metrics.MetricPoint {
	t.Helper()

	return waitForStoredMetric(t, store, "disk", resourceID, metric)
}

func waitForStoredMetric(t *testing.T, store *metrics.Store, resourceType, resourceID, metric string) []metrics.MetricPoint {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for {
		now := time.Now().UTC()
		points, err := store.Query(resourceType, resourceID, metric, now.Add(-time.Hour), now.Add(time.Hour), 60)
		if err != nil {
			t.Fatalf("Query %s: %v", metric, err)
		}
		if len(points) > 0 {
			return points
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for disk metric %s for %s", metric, resourceID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestEvaluateHostAgentsEmptyHostsList(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// No hosts in state - should complete without error or state changes
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.Hosts) != 0 {
		t.Errorf("expected 0 hosts, got %d", len(snapshot.Hosts))
	}
	if len(snapshot.ConnectionHealth) != 0 {
		t.Errorf("expected 0 connection health entries, got %d", len(snapshot.ConnectionHealth))
	}
}

func TestEvaluateHostAgentsZeroIntervalUsesDefault(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-zero-interval"
	// IntervalSeconds = 0, LastSeen = now, should use default interval (60s)
	// Default window = 60s * 6 = 360s, but minimum is 60s, so window = 60s
	// With LastSeen = now, the host should be healthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "zero-interval.local",
		Status:          "unknown",
		IntervalSeconds: 0, // Zero interval - should use default
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for zero-interval host with recent LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNegativeIntervalUsesDefault(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-negative-interval"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "negative-interval.local",
		Status:          "unknown",
		IntervalSeconds: -10, // Negative interval - should use default
		LastSeen:        time.Now(),
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for negative-interval host with recent LastSeen, got %v (exists=%v)", healthy, ok)
	}
}

func TestEvaluateHostAgentsWindowClampedToMinimum(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-min-window"
	// IntervalSeconds = 1, so window = 1s * 6 = 6s, but minimum is 60s
	// Host last seen 55s ago should still be healthy (within 60s minimum window)
	now := time.Now()
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "min-window.local",
		Status:          "unknown",
		IntervalSeconds: 1, // Very small interval
		LastSeen:        now.Add(-55 * time.Second),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true (window clamped to minimum 60s), got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsWindowClampedToMaximum(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-max-window"
	// IntervalSeconds = 300 (5 min), so window = 300s * 6 = 1800s (30 min)
	// But maximum is 10 min = 600s
	// Host last seen 11 minutes ago should be unhealthy (outside 10 min max window)
	now := time.Now()
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "max-window.local",
		Status:          "online",
		IntervalSeconds: 300, // 5 minute interval
		LastSeen:        now.Add(-11 * time.Minute),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false (window clamped to maximum 10m), got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsRecentLastSeenIsHealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-recent"
	now := time.Now()
	// IntervalSeconds = 30, window = 30s * 6 = 180s (clamped to min 60s is not needed)
	// LastSeen = 10s ago, should be healthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "recent.local",
		Status:          "unknown",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-10 * time.Second),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true for recent LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsZeroLastSeenIsUnhealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-zero-lastseen"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "zero-lastseen.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Time{}, // Zero time
	})

	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false for zero LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline for zero LastSeen, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsOldLastSeenIsUnhealthy(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-old-lastseen"
	now := time.Now()
	// IntervalSeconds = 30, window = 30s * 6 = 180s
	// LastSeen = 5 minutes ago, should be unhealthy
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "old-lastseen.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        now.Add(-5 * time.Minute),
	})

	monitor.evaluateHostAgents(now)

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false for old LastSeen, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline for old LastSeen, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNilAlertManagerOnline(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: nil, // No alert manager
		config:       &config.Config{},
	}

	hostID := "host-nil-am-online"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "nil-am-online.local",
		Status:          "unknown",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	// Should not panic with nil alertManager
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || !healthy {
		t.Fatalf("expected connection health true, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "online" {
			t.Errorf("expected host status online, got %q", host.Status)
		}
	}
}

func TestEvaluateHostAgentsNilAlertManagerOffline(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: nil, // No alert manager
		config:       &config.Config{},
	}

	hostID := "host-nil-am-offline"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "nil-am-offline.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Time{}, // Zero time - unhealthy
	})

	// Should not panic with nil alertManager
	monitor.evaluateHostAgents(time.Now())

	snapshot := monitor.state.GetSnapshot()
	connKey := hostConnectionPrefix + hostID
	if healthy, ok := snapshot.ConnectionHealth[connKey]; !ok || healthy {
		t.Fatalf("expected connection health false, got %v (exists=%v)", healthy, ok)
	}

	for _, host := range snapshot.Hosts {
		if host.ID == hostID && host.Status != "offline" {
			t.Errorf("expected host status offline, got %q", host.Status)
		}
	}
}

func TestRemoveHostAgent_ClearsConnectionHealth(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-connhealth"
	monitor.state.UpsertHost(models.Host{
		ID:              hostID,
		Hostname:        "connhealth.local",
		Status:          "online",
		IntervalSeconds: 30,
		LastSeen:        time.Now(),
	})

	// Seed connection health for this host (as evaluateHostAgents would)
	monitor.state.SetConnectionHealth(hostConnectionPrefix+hostID, true)

	// Verify it's present before removal
	snapshot := monitor.state.GetSnapshot()
	if _, ok := snapshot.ConnectionHealth[hostConnectionPrefix+hostID]; !ok {
		t.Fatalf("expected connection health entry to exist before removal")
	}

	// Remove the host
	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("RemoveHostAgent: %v", err)
	}

	// Verify connection health entry is gone
	snapshot = monitor.state.GetSnapshot()
	if _, ok := snapshot.ConnectionHealth[hostConnectionPrefix+hostID]; ok {
		t.Fatalf("expected connection health entry to be removed after RemoveHostAgent")
	}
}

func TestRemoveHostAgent_EmptyHostID(t *testing.T) {
	monitor := &Monitor{
		state:        models.NewState(),
		alertManager: alerts.NewManager(),
		config:       &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Empty hostID should return an error
	_, err := monitor.RemoveHostAgent("")
	if err == nil {
		t.Error("expected error for empty hostID")
	}
	if err != nil && err.Error() != "host id is required" {
		t.Errorf("expected 'host id is required' error, got: %v", err)
	}

	// Whitespace-only hostID should also return an error
	_, err = monitor.RemoveHostAgent("   ")
	if err == nil {
		t.Error("expected error for whitespace-only hostID")
	}
}

func TestRemoveHostAgent_NotFound(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Host does not exist in state - should return synthetic host without error
	host, err := monitor.RemoveHostAgent("nonexistent-host")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return a synthetic host with ID/Hostname matching the requested ID
	if host.ID != "nonexistent-host" {
		t.Errorf("expected host.ID = 'nonexistent-host', got %q", host.ID)
	}
	if host.Hostname != "nonexistent-host" {
		t.Errorf("expected host.Hostname = 'nonexistent-host', got %q", host.Hostname)
	}
}

func TestRemoveHostAgent_NoTokenBinding(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-no-binding"
	tokenID := "token-no-binding"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "no-binding.local",
		TokenID:  tokenID,
	})
	// Intentionally NOT adding to hostTokenBindings

	host, err := monitor.RemoveHostAgent(hostID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if host.ID != hostID {
		t.Errorf("expected host.ID = %q, got %q", hostID, host.ID)
	}
}

func TestRemoveHostAgent_NilAlertManager(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      nil, // No alert manager
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}

	hostID := "host-nil-am-remove"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "nil-am-remove.local",
	})

	// Should not panic with nil alertManager
	host, err := monitor.RemoveHostAgent(hostID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if host.ID != hostID {
		t.Errorf("expected host.ID = %q, got %q", hostID, host.ID)
	}
}

func TestRemoveHostAgent_BlocksFutureReportsUntilAllowed(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		removedHostAgents: make(map[string]time.Time),
		rateTracker:       NewRateTracker(),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-blocked"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "blocked.local",
	})

	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("remove host agent: %v", err)
	}

	report := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:       hostID,
			Hostname: "blocked.local",
		},
		Agent:     agentshost.AgentInfo{ID: hostID},
		Timestamp: time.Now(),
	}

	if _, err := monitor.ApplyHostReport(report, nil); err == nil {
		t.Fatal("expected removed host agent report to be rejected")
	}

	if err := monitor.AllowHostAgentReenroll(hostID); err != nil {
		t.Fatalf("allow host reenroll: %v", err)
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("expected host report after allow reenroll, got %v", err)
	}
}

// TestApplyHostReport_FreshTokenClearsRemovalBlock pins the #1581 re-enroll
// path: a report presenting a token created after the removal is explicit
// re-add intent and clears the block, while a pre-removal token stays blocked.
func TestApplyHostReport_FreshTokenClearsRemovalBlock(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		removedHostAgents: make(map[string]time.Time),
		rateTracker:       NewRateTracker(),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-fresh-token"
	monitor.state.UpsertHost(models.Host{
		ID:       hostID,
		Hostname: "fresh-token.local",
	})
	if _, err := monitor.RemoveHostAgent(hostID); err != nil {
		t.Fatalf("remove host agent: %v", err)
	}

	report := agentshost.Report{
		Host: agentshost.HostInfo{
			ID:       hostID,
			Hostname: "fresh-token.local",
		},
		Agent:     agentshost.AgentInfo{ID: hostID},
		Timestamp: time.Now(),
	}

	staleToken := &config.APITokenRecord{
		ID:        "token-old",
		CreatedAt: time.Now().Add(-time.Hour),
	}
	if _, err := monitor.ApplyHostReport(report, staleToken); err == nil {
		t.Fatal("expected report with pre-removal token to stay blocked")
	}

	freshToken := &config.APITokenRecord{
		ID:        "token-new",
		CreatedAt: time.Now().Add(time.Minute),
	}
	if _, err := monitor.ApplyHostReport(report, freshToken); err != nil {
		t.Fatalf("expected report with post-removal token to clear the block, got %v", err)
	}

	if len(monitor.state.GetRemovedHostAgents()) != 0 {
		t.Fatal("expected state removal block to be cleared")
	}
}

// TestAllowHostAgentReenroll_ClearsStateMirrorWithoutMemoryEntry preserves
// compatibility with callers that only populated the models.State mirror.
// The production restart path is covered by TestHostAgentRemovalLifecycleSurvivesMonitorReconstruction.
func TestAllowHostAgentReenroll_ClearsStateMirrorWithoutMemoryEntry(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		removedHostAgents: make(map[string]time.Time),
		rateTracker:       NewRateTracker(),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	hostID := "host-restart-block"
	monitor.state.AddRemovedHostAgent(models.RemovedHostAgent{
		ID:        hostID,
		Hostname:  "restart-block.local",
		RemovedAt: time.Now().Add(-time.Hour),
	})

	if err := monitor.AllowHostAgentReenroll(hostID); err != nil {
		t.Fatalf("allow host reenroll: %v", err)
	}
	if len(monitor.state.GetRemovedHostAgents()) != 0 {
		t.Fatal("expected state removal block to be cleared without an in-memory entry")
	}
}

// TestCleanupRemovedHostAgents_ExpiresStateMirrorEntries pins expiry for
// models.State-only compatibility entries when the in-memory map lacks them.
func TestCleanupRemovedHostAgents_ExpiresStateMirrorEntries(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		hostTokenBindings: make(map[string]string),
		removedHostAgents: make(map[string]time.Time),
		config:            &config.Config{},
	}

	monitor.state.AddRemovedHostAgent(models.RemovedHostAgent{
		ID:        "host-expired",
		Hostname:  "expired.local",
		RemovedAt: time.Now().Add(-25 * time.Hour),
	})
	monitor.state.AddRemovedHostAgent(models.RemovedHostAgent{
		ID:        "host-recent",
		Hostname:  "recent.local",
		RemovedAt: time.Now().Add(-time.Hour),
	})

	monitor.cleanupRemovedHostAgents(time.Now())

	remaining := monitor.state.GetRemovedHostAgents()
	if len(remaining) != 1 {
		t.Fatalf("expected exactly one persisted block to survive, got %d", len(remaining))
	}
	if remaining[0].ID != "host-recent" {
		t.Fatalf("expected host-recent to survive, got %q", remaining[0].ID)
	}
}

// TestMatchHostConfigContinuity pins the #1570 config-fetch fallback: after a
// monitor reload wipes live state, a report-scoped token still resolves its
// bound host from the persisted continuity store, other tokens do not, and a
// manage-scoped (empty token) lookup resolves by host ID.
func TestMatchHostConfigContinuity(t *testing.T) {
	dir := t.TempDir()
	monitor := &Monitor{
		hostContinuityStore: config.NewHostContinuityStore(dir, nil),
	}

	entry := config.HostContinuityEntry{
		HostID:    "host-cont-1",
		Hostname:  "cont.local",
		MachineID: "machine-cont-1",
		TokenID:   "token-cont-1",
		LastSeen:  time.Now().UTC(),
	}
	if err := monitor.hostContinuityStore.Upsert(entry); err != nil {
		t.Fatalf("upsert continuity entry: %v", err)
	}

	host, ok := monitor.MatchHostConfigContinuity("host-cont-1", "token-cont-1")
	if !ok || host.ID != "host-cont-1" {
		t.Fatalf("expected token-bound continuity match, got ok=%v host=%q", ok, host.ID)
	}

	if _, ok := monitor.MatchHostConfigContinuity("host-cont-1", "token-other"); ok {
		t.Fatal("expected no match for a token the host is not bound to")
	}

	host, ok = monitor.MatchHostConfigContinuity("host-cont-1", "")
	if !ok || host.ID != "host-cont-1" {
		t.Fatalf("expected ID-based continuity match for manage scope, got ok=%v host=%q", ok, host.ID)
	}

	if _, ok := monitor.MatchHostConfigContinuity("host-unknown", ""); ok {
		t.Fatal("expected no ID-based match for an unknown host")
	}
}

func TestApplyHostReport_MissingHostname(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Report with empty hostname should fail
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "", // Missing hostname
			ID:       "machine-id",
		},
		Agent: agentshost.AgentInfo{
			ID:      "agent-id",
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyHostReport(report, nil)
	if err == nil {
		t.Error("expected error for missing hostname")
	}
	if err != nil && err.Error() != "host report missing hostname" {
		t.Errorf("expected 'host report missing hostname' error, got: %v", err)
	}
}

func TestApplyHostReport_WhitespaceHostname(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Report with whitespace-only hostname should fail
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "   ", // Whitespace only
			ID:       "machine-id",
		},
		Agent: agentshost.AgentInfo{
			ID:      "agent-id",
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyHostReport(report, nil)
	if err == nil {
		t.Error("expected error for whitespace-only hostname")
	}
}

func TestApplyHostReport_NilTokenBindingsMap(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: nil, // Nil map
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "test-host",
			ID:       "machine-id",
		},
		Agent: agentshost.AgentInfo{
			ID:      "agent-id",
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	token := &config.APITokenRecord{ID: "token-id", Name: "Test Token"}

	// Should not panic with nil hostTokenBindings - map should be initialized
	host, err := monitor.ApplyHostReport(report, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host.Hostname != "test-host" {
		t.Errorf("expected hostname 'test-host', got %q", host.Hostname)
	}
}

func TestApplyHostReport_FallbackIdentifier(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	// Report with no ID fields - should generate fallback identifier
	report := agentshost.Report{
		Host: agentshost.HostInfo{
			Hostname: "fallback-host",
			// No ID, MachineID
		},
		Agent: agentshost.AgentInfo{
			// No ID
			Version: "1.0.0",
		},
		Timestamp: time.Now(),
	}

	host, err := monitor.ApplyHostReport(report, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use hostname as fallback identifier
	if host.ID == "" {
		t.Error("expected host to have an identifier")
	}
	if host.Hostname != "fallback-host" {
		t.Errorf("expected hostname 'fallback-host', got %q", host.Hostname)
	}
}

// Regression for #1341: when Ceph is reported by a Pulse host-agent, pool
// alert evaluation must run even when there is no Proxmox API Ceph source.
// Existing per-pool overrides keyed against the older agent-prefixed pool ID
// still have to resolve after the pool's canonical ID drops the agent routing
// prefix.
func TestApplyHostReportFiresCephPoolAlertsForAgentSourcedCluster(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	overrideTrigger := alerts.HysteresisThreshold{Trigger: 50, Clear: 45}
	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 95, Clear: 90}
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {Usage: &overrideTrigger},
	}
	monitor.alertManager.UpdateConfig(cfg)

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-ceph",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:       "pve5-host",
			Hostname: "pve5",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
		Ceph: &agentshost.CephCluster{
			FSID:   "ceph-fsid-1341",
			Health: agentshost.CephHealth{Status: "HEALTH_OK"},
			Pools: []agentshost.CephPool{
				{
					ID:             2,
					Name:           "data_replication",
					BytesUsed:      611,
					BytesAvailable: 389,
					PercentUsed:    61.1,
				},
			},
		},
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	active := monitor.alertManager.GetActiveAlerts()
	found := false
	for _, alert := range active {
		if alert.ResourceID == "pve5-ceph-pool-data_replication" && alert.Type == "usage" {
			found = true
			if alert.Threshold != 50 {
				t.Fatalf("Ceph pool alert fired but threshold = %.1f, want 50 (override)", alert.Threshold)
			}
			break
		}
	}
	if !found {
		ids := make([]string, 0, len(active))
		for _, a := range active {
			ids = append(ids, a.ID)
		}
		t.Fatalf("expected agent-sourced Ceph pool alert to fire under legacy override at 61.1%% usage; got %d active alerts: %v", len(active), ids)
	}
}

func TestApplyHostReportMergesAgentCephWithProxmoxAPICluster(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	overrideTrigger := alerts.HysteresisThreshold{Trigger: 50, Clear: 45}
	cfg := monitor.alertManager.GetConfig()
	cfg.MinimumDelta = 0
	if cfg.TimeThresholds == nil {
		cfg.TimeThresholds = make(map[string]int)
	}
	cfg.TimeThresholds["storage"] = 0
	cfg.StorageDefault = alerts.HysteresisThreshold{Trigger: 95, Clear: 90}
	cfg.Overrides = map[string]alerts.ThresholdConfig{
		"agent:pve5-ceph-pool-data_replication": {Usage: &overrideTrigger},
	}
	monitor.alertManager.UpdateConfig(cfg)

	monitor.state.UpdateCephClustersForInstance("prod", []models.CephCluster{{
		ID:       "prod-ceph-fsid-1341",
		Instance: "prod",
		Source:   models.CephClusterSourceProxmoxAPI,
		Name:     "Ceph",
		FSID:     "ceph-fsid-1341",
		Health:   "HEALTH_OK",
	}})

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{
			ID:              "agent-ceph",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentshost.HostInfo{
			ID:       "pve5-host",
			Hostname: "pve5",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
		Ceph: &agentshost.CephCluster{
			FSID:   "ceph-fsid-1341",
			Health: agentshost.CephHealth{Status: "HEALTH_OK"},
			Pools: []agentshost.CephPool{
				{
					ID:             2,
					Name:           "data_replication",
					BytesUsed:      611,
					BytesAvailable: 389,
					PercentUsed:    61.1,
				},
			},
		},
	}

	if _, err := monitor.ApplyHostReport(report, nil); err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}

	snapshot := monitor.state.GetSnapshot()
	if len(snapshot.CephClusters) != 1 {
		t.Fatalf("expected API and agent Ceph reports to reconcile to one cluster, got %#v", snapshot.CephClusters)
	}
	cluster := snapshot.CephClusters[0]
	if cluster.Source != models.CephClusterSourceProxmoxAPI || cluster.Instance != "prod" {
		t.Fatalf("expected API cluster to remain canonical, got %+v", cluster)
	}
	foundAlias := false
	for _, alias := range cluster.InstanceAliases {
		if alias == "pve5" {
			foundAlias = true
			break
		}
	}
	if !foundAlias {
		t.Fatalf("expected agent hostname to be preserved as an alias, got %#v", cluster.InstanceAliases)
	}

	active := monitor.alertManager.GetActiveAlerts()
	for _, alert := range active {
		if alert.ResourceID == "prod-ceph-pool-data_replication" && alert.Type == "usage" {
			if alert.Threshold != 50 {
				t.Fatalf("Ceph pool alert threshold = %.1f, want legacy agent override 50", alert.Threshold)
			}
			return
		}
	}
	t.Fatalf("expected canonical API Ceph pool alert using legacy agent override, got active alerts: %#v", active)
}

func TestApplyHostReportMapsReclaimableMemoryCache(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	report := agentshost.Report{
		Agent: agentshost.AgentInfo{ID: "agent-cache", Version: "1.0.0", IntervalSeconds: 30},
		Host: agentshost.HostInfo{
			ID:       "machine-cache",
			Hostname: "cache-host",
			Platform: "linux",
		},
		Timestamp: time.Now().UTC(),
		Metrics: agentshost.Metrics{
			Memory: agentshost.MemoryMetric{
				TotalBytes: 16 << 30,
				UsedBytes:  6 << 30,
				FreeBytes:  4 << 30,
				CacheBytes: 6 << 30,
				Usage:      37.5,
			},
		},
	}

	host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "token-cache", Name: "Token"})
	if err != nil {
		t.Fatalf("ApplyHostReport: %v", err)
	}
	if got, want := host.Memory.Cache, int64(6<<30); got != want {
		t.Fatalf("memory cache = %d, want %d", got, want)
	}

	// An inconsistent report (cache pushing past total) is clamped so the
	// used | cache | free split can never exceed the bar.
	report.Metrics.Memory.CacheBytes = 12 << 30
	host, err = monitor.ApplyHostReport(report, &config.APITokenRecord{ID: "token-cache", Name: "Token"})
	if err != nil {
		t.Fatalf("ApplyHostReport clamp: %v", err)
	}
	if got, want := host.Memory.Cache, int64(10<<30); got != want {
		t.Fatalf("clamped memory cache = %d, want %d", got, want)
	}
}

func TestApplyHostReportNormalizesLegacyAgentPlatformAcceptedIngestProof(t *testing.T) {
	monitor := &Monitor{
		state:             models.NewState(),
		alertManager:      alerts.NewManager(),
		hostTokenBindings: make(map[string]string),
		config:            &config.Config{},
		rateTracker:       NewRateTracker(),
	}
	t.Cleanup(func() { monitor.alertManager.Stop() })

	now := time.Now().UTC()
	cases := []struct {
		name     string
		platform string
		want     string
	}{
		// Legacy v5 agents report gopsutil's host.Info().Platform verbatim,
		// e.g. "microsoft windows 11 pro" on Windows (refs #1555).
		{"legacy windows caption", "microsoft windows 11 pro", "windows"},
		{"darwin", "darwin", "macos"},
		{"linux distro preserved", "Ubuntu", "ubuntu"},
	}
	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := agentshost.Report{
				Agent: agentshost.AgentInfo{
					ID:              fmt.Sprintf("agent-platform-%d", index),
					Version:         "5.1.36",
					IntervalSeconds: 30,
				},
				Host: agentshost.HostInfo{
					ID:       fmt.Sprintf("machine-platform-%d", index),
					Hostname: fmt.Sprintf("host-platform-%d", index),
					Platform: tc.platform,
				},
				Timestamp: now,
			}

			host, err := monitor.ApplyHostReport(report, &config.APITokenRecord{ID: fmt.Sprintf("token-platform-%d", index), Name: "Platform Token"})
			if err != nil {
				t.Fatalf("ApplyHostReport: %v", err)
			}
			if host.Platform != tc.want {
				t.Fatalf("expected ingested platform %q for reported %q, got %q", tc.want, tc.platform, host.Platform)
			}
		})
	}
}

// hostFromReadStateView must carry the fabric's integration-source
// discriminator onto models.Host so ledger consumers (connections aggregator,
// update readiness) can tell real Pulse Agents from integration-monitored
// machines, while agent-sourced hosts stay unmarked.
func TestHostFromReadStateViewMapsIntegrationSource(t *testing.T) {
	t.Parallel()

	vmwareResource := unifiedresources.Resource{
		ID:      "vc-1:host:host-101",
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    "esxi-01.lab.local",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceVMware},
		Agent:   &unifiedresources.AgentData{AgentID: "vc-1:host:host-101", Platform: "vmware-vsphere"},
	}
	vmwareView := unifiedresources.NewHostView(&vmwareResource)
	if got := hostFromReadStateView(&vmwareView).IntegrationSource; got != "vmware" {
		t.Fatalf("vSphere-only host must map IntegrationSource %q, got %q", "vmware", got)
	}

	agentResource := unifiedresources.Resource{
		ID:      "host-linux-1",
		Type:    unifiedresources.ResourceTypeAgent,
		Name:    "apollo-114",
		Sources: []unifiedresources.DataSource{unifiedresources.SourceAgent},
		Agent:   &unifiedresources.AgentData{AgentID: "host-linux-1", AgentVersion: "6.1.0"},
	}
	agentView := unifiedresources.NewHostView(&agentResource)
	if got := hostFromReadStateView(&agentView).IntegrationSource; got != "" {
		t.Fatalf("agent-sourced host must map empty IntegrationSource, got %q", got)
	}
}

// Removal must retain the agent's last-known platform on the removed record so
// the fleet doctor can still scope host-side cleanup commands after the live
// host record is gone.
func TestRemoveHostAgentRetainsLastKnownPlatform(t *testing.T) {
	monitor := &Monitor{
		state:  models.NewState(),
		config: &config.Config{},
	}

	now := time.Now().UTC()
	monitor.state.UpsertHost(models.Host{
		ID:       "removed-platform-host",
		Hostname: "win-node",
		Platform: "Microsoft Windows Server 2022",
		Status:   "online",
		LastSeen: now.Add(-time.Minute),
	})
	monitor.state.UpsertHost(models.Host{
		ID:       "removed-osname-host",
		Hostname: "debian-node",
		OSName:   "Debian GNU/Linux",
		Status:   "online",
		LastSeen: now.Add(-time.Minute),
	})

	for _, hostID := range []string{"removed-platform-host", "removed-osname-host"} {
		if _, err := monitor.RemoveHostAgent(hostID); err != nil {
			t.Fatalf("RemoveHostAgent(%s): %v", hostID, err)
		}
	}

	got := make(map[string]string)
	for _, entry := range monitor.state.GetRemovedHostAgents() {
		got[entry.ID] = entry.Platform
	}
	if got["removed-platform-host"] != "Microsoft Windows Server 2022" {
		t.Fatalf("removed record platform = %q, want last-known reported platform", got["removed-platform-host"])
	}
	if got["removed-osname-host"] != "Debian GNU/Linux" {
		t.Fatalf("removed record platform = %q, want OS-name fallback when Platform was never reported", got["removed-osname-host"])
	}
}
