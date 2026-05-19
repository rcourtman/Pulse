package monitoring

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// mockDockerChecker is a test implementation of DockerChecker
type mockDockerChecker struct {
	results map[int]bool // vmid -> hasDocker
	mu      sync.Mutex
	calls   []int // records vmids that were checked
}

func (m *mockDockerChecker) CheckDockerInContainer(ctx context.Context, node string, vmid int) (bool, error) {
	m.mu.Lock()
	m.calls = append(m.calls, vmid)
	m.mu.Unlock()
	if hasDocker, ok := m.results[vmid]; ok {
		return hasDocker, nil
	}
	return false, nil
}

func TestCheckContainersForDocker_NewContainers(t *testing.T) {
	// Create a monitor with state
	state := models.NewState()
	monitor := &Monitor{
		state: state,
	}

	// Create a mock checker
	checker := &mockDockerChecker{
		results: map[int]bool{
			101: true,
			102: false,
			103: true, // This one is stopped, so it shouldn't be checked
		},
	}
	monitor.SetDockerChecker(checker)

	// Input containers - all new (not in state yet)
	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "container-with-docker", Node: "node1", Status: "running"},
		{ID: "ct-2", VMID: 102, Name: "container-without-docker", Node: "node1", Status: "running"},
		{ID: "ct-3", VMID: 103, Name: "stopped-container", Node: "node1", Status: "stopped"},
	}

	// Run the check
	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Verify results
	var ct1, ct2, ct3 *models.Container
	for i := range result {
		switch result[i].ID {
		case "ct-1":
			ct1 = &result[i]
		case "ct-2":
			ct2 = &result[i]
		case "ct-3":
			ct3 = &result[i]
		}
	}

	if ct1 == nil || !ct1.HasDocker {
		t.Errorf("Container 101 should have HasDocker=true")
	}
	if ct1 != nil && ct1.DockerCheckedAt.IsZero() {
		t.Errorf("Container 101 should have DockerCheckedAt set")
	}

	if ct2 == nil || ct2.HasDocker {
		t.Errorf("Container 102 should have HasDocker=false")
	}
	if ct2 != nil && ct2.DockerCheckedAt.IsZero() {
		t.Errorf("Container 102 should have DockerCheckedAt set")
	}

	// Stopped container should not be checked
	if ct3 != nil && !ct3.DockerCheckedAt.IsZero() {
		t.Errorf("Stopped container 103 should not have been checked")
	}

	// Verify only running containers were checked
	if len(checker.calls) != 2 {
		t.Errorf("Expected 2 Docker checks (running containers only), got %d", len(checker.calls))
	}
}

func TestCheckContainersForDocker_PreservesExistingStatus(t *testing.T) {
	// Create a monitor with state containing previously checked containers
	state := models.NewState()
	checkedTime := time.Now().Add(-30 * time.Minute)
	state.UpdateContainers([]models.Container{
		{
			ID:              "ct-1",
			VMID:            101,
			Name:            "already-checked",
			Node:            "node1",
			Status:          "running",
			HasDocker:       true,
			DockerCheckedAt: checkedTime,
			Uptime:          3600, // 1 hour uptime
		},
	})

	monitor := &Monitor{
		state: state,
	}

	checker := &mockDockerChecker{
		results: map[int]bool{101: false}, // Would return false if checked
	}
	monitor.SetDockerChecker(checker)

	// Same container, still running with same/higher uptime
	containers := []models.Container{
		{
			ID:     "ct-1",
			VMID:   101,
			Name:   "already-checked",
			Node:   "node1",
			Status: "running",
			Uptime: 7200, // 2 hours uptime (no restart)
		},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should not have been re-checked
	if len(checker.calls) != 0 {
		t.Errorf("Expected 0 checks (should preserve existing status), got %d", len(checker.calls))
	}

	// Should preserve the previous Docker status
	if !result[0].HasDocker {
		t.Errorf("Container should still have cached HasDocker=true")
	}
	if result[0].DockerCheckedAt.IsZero() {
		t.Errorf("Container should have preserved DockerCheckedAt")
	}
}

func TestCheckContainersForDocker_RechecksOnRestart(t *testing.T) {
	// Create a monitor with state containing previously checked container
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{
			ID:              "ct-1",
			VMID:            101,
			Name:            "restarted-container",
			Node:            "node1",
			Status:          "running",
			HasDocker:       false,
			DockerCheckedAt: time.Now().Add(-1 * time.Hour),
			Uptime:          3600, // 1 hour uptime
		},
	})

	monitor := &Monitor{
		state: state,
	}

	checker := &mockDockerChecker{
		results: map[int]bool{101: true}, // Now has Docker after restart
	}
	monitor.SetDockerChecker(checker)

	// Same container but with reset uptime (restarted)
	containers := []models.Container{
		{
			ID:     "ct-1",
			VMID:   101,
			Name:   "restarted-container",
			Node:   "node1",
			Status: "running",
			Uptime: 60, // Only 1 minute uptime - it restarted
		},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should have been re-checked because uptime reset
	if len(checker.calls) != 1 {
		t.Errorf("Expected 1 check (container restarted), got %d", len(checker.calls))
	}

	// Should have updated Docker status
	if !result[0].HasDocker {
		t.Errorf("Container should now have HasDocker=true after restart")
	}
}

func TestCheckContainersForDocker_RechecksWhenStarted(t *testing.T) {
	// Create a monitor with state containing previously stopped container
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{
			ID:              "ct-1",
			VMID:            101,
			Name:            "was-stopped",
			Node:            "node1",
			Status:          "stopped",
			HasDocker:       false,
			DockerCheckedAt: time.Now().Add(-1 * time.Hour),
		},
	})

	monitor := &Monitor{
		state: state,
	}

	checker := &mockDockerChecker{
		results: map[int]bool{101: true},
	}
	monitor.SetDockerChecker(checker)

	// Container is now running
	containers := []models.Container{
		{
			ID:     "ct-1",
			VMID:   101,
			Name:   "was-stopped",
			Node:   "node1",
			Status: "running",
			Uptime: 60,
		},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should have been checked because it just started
	if len(checker.calls) != 1 {
		t.Errorf("Expected 1 check (container started), got %d", len(checker.calls))
	}

	if !result[0].HasDocker {
		t.Errorf("Container should have HasDocker=true after starting")
	}
}

func TestCheckContainersForDocker_NoCheckerConfigured(t *testing.T) {
	state := models.NewState()
	monitor := &Monitor{
		state: state,
	}
	// No checker configured

	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "test", Node: "node1", Status: "running"},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	// Should return containers unchanged
	if len(result) != 1 {
		t.Errorf("Expected 1 container, got %d", len(result))
	}
	if result[0].HasDocker {
		t.Errorf("Container should not have HasDocker set without checker")
	}
}

func TestAgentDockerChecker_ParsesOutput(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		exitCode   int
		wantDocker bool
		wantErr    bool
	}{
		{
			name:       "docker socket exists",
			stdout:     "yes\n",
			exitCode:   0,
			wantDocker: true,
			wantErr:    false,
		},
		{
			name:       "docker socket does not exist",
			stdout:     "no\n",
			exitCode:   1,
			wantDocker: false,
			wantErr:    false,
		},
		{
			name:       "container not accessible",
			stdout:     "error: CT 101 is locked",
			exitCode:   1,
			wantDocker: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewAgentDockerChecker(func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
				return tt.stdout, tt.exitCode, nil
			})

			hasDocker, err := checker.CheckDockerInContainer(context.Background(), "node1", 101)

			if tt.wantErr && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if hasDocker != tt.wantDocker {
				t.Errorf("Expected hasDocker=%v, got %v", tt.wantDocker, hasDocker)
			}
		})
	}
}

func TestAgentDockerInventoryCollector_MinimalCommandAndReport(t *testing.T) {
	var gotCommand string
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
		gotCommand = command
		return strings.Join([]string{
			proxmoxGuestDockerInventoryMarker,
			"HOSTNAME\tct-web",
			"UNAME\tLinux 6.8.12-9-pve x86_64",
			"CPUS\t2",
			"MEMTOTAL\t1073741824",
			"VERSION\t\"26.1.4\"",
			"CONTAINER\t\"abcdef123456\"\t\"web\"\t\"nginx:latest\"\t\"running\"\t\"Up 2 hours\"\t\"0.0.0.0:8080->80/tcp, 443/tcp\"\t\"2 hours ago\"",
			"PS_OK",
			"STAT\t\"abcdef123456\"\t\"web\"\t\"0.50%\"\t\"64MiB / 1GiB\"\t\"6.25%\"\t\"1.2kB / 3.4MB\"\t\"5MB / 6MB\"",
			"",
		}, "\n"), 0, nil
	}, AgentDockerInventoryCollectorOptions{})

	report, ok, err := collector.CollectDockerInventory(context.Background(), models.Container{
		ID:     "pve-a:node-a:101",
		VMID:   101,
		Name:   "web-lxc",
		Node:   "node-a",
		Status: "running",
	})
	if err != nil {
		t.Fatalf("CollectDockerInventory returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected inventory report to be collected")
	}
	if !strings.Contains(gotCommand, "pct exec 101 -- sh -c") {
		t.Fatalf("expected pct exec command for VMID 101, got %q", gotCommand)
	}
	for _, forbidden := range []string{"docker inspect", "docker exec", "docker top", "printenv", ".Labels", ".Mounts", ".Command"} {
		if strings.Contains(gotCommand, forbidden) {
			t.Fatalf("inventory command must not collect %s; command=%q", forbidden, gotCommand)
		}
	}

	if report.Agent.ID != "proxmox-lxc-docker:pve-a:node-a:101" {
		t.Fatalf("Agent.ID = %q", report.Agent.ID)
	}
	if report.Host.Hostname != "ct-web" {
		t.Fatalf("Host.Hostname = %q", report.Host.Hostname)
	}
	if report.Host.Name != "web-lxc (LXC 101)" {
		t.Fatalf("Host.Name = %q", report.Host.Name)
	}
	if report.Host.DockerVersion != "26.1.4" || report.Host.TotalCPU != 2 || report.Host.TotalMemoryBytes != 1073741824 {
		t.Fatalf("unexpected host info: %#v", report.Host)
	}
	if len(report.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(report.Containers))
	}
	ct := report.Containers[0]
	if ct.ID != "abcdef123456" || ct.Name != "web" || ct.Image != "nginx:latest" || ct.State != "running" {
		t.Fatalf("unexpected container payload: %#v", ct)
	}
	if ct.UptimeSeconds != 7200 {
		t.Fatalf("UptimeSeconds = %d, want 7200", ct.UptimeSeconds)
	}
	if ct.CPUPercent != 0.5 || ct.MemoryUsageBytes != 64*1024*1024 || ct.MemoryLimitBytes != 1024*1024*1024 || ct.MemoryPercent != 6.25 {
		t.Fatalf("unexpected stats payload: %#v", ct)
	}
	if ct.NetworkRXBytes != 1200 || ct.NetworkTXBytes != 3400000 {
		t.Fatalf("unexpected network IO: rx=%d tx=%d", ct.NetworkRXBytes, ct.NetworkTXBytes)
	}
	if ct.BlockIO == nil || ct.BlockIO.ReadBytes != 5000000 || ct.BlockIO.WriteBytes != 6000000 {
		t.Fatalf("unexpected block IO: %#v", ct.BlockIO)
	}
	if len(ct.Ports) != 2 || ct.Ports[0].PrivatePort != 80 || ct.Ports[0].PublicPort != 8080 || ct.Ports[1].PrivatePort != 443 {
		t.Fatalf("unexpected ports: %#v", ct.Ports)
	}
}

func TestAgentDockerInventoryCollector_PartialOutputRefusesApply(t *testing.T) {
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
		return strings.Join([]string{
			proxmoxGuestDockerInventoryMarker,
			"HOSTNAME\tct-web",
			"UNAME\tLinux 6.8.12-9-pve x86_64",
			"CPUS\t2",
			"MEMTOTAL\t1073741824",
			"VERSION\t\"26.1.4\"",
			"",
		}, "\n"), 0, nil
	}, AgentDockerInventoryCollectorOptions{})

	_, ok, err := collector.CollectDockerInventory(context.Background(), models.Container{
		ID:     "pve-a:node-a:101",
		VMID:   101,
		Name:   "web-lxc",
		Node:   "node-a",
		Status: "running",
	})
	if err == nil {
		t.Fatal("expected an error when docker ps marker is missing")
	}
	if ok {
		t.Fatal("partial inventory output (no PS_OK) must not produce an applicable report")
	}
}

func TestAgentDockerInventoryCollector_PSOKWithZeroContainersApplies(t *testing.T) {
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
		return strings.Join([]string{
			proxmoxGuestDockerInventoryMarker,
			"HOSTNAME\tct-web",
			"UNAME\tLinux 6.8.12-9-pve x86_64",
			"CPUS\t2",
			"MEMTOTAL\t1073741824",
			"VERSION\t\"26.1.4\"",
			"PS_OK",
			"",
		}, "\n"), 0, nil
	}, AgentDockerInventoryCollectorOptions{})

	report, ok, err := collector.CollectDockerInventory(context.Background(), models.Container{
		ID:     "pve-a:node-a:101",
		VMID:   101,
		Name:   "web-lxc",
		Node:   "node-a",
		Status: "running",
	})
	if err != nil {
		t.Fatalf("CollectDockerInventory returned error: %v", err)
	}
	if !ok {
		t.Fatal("zero-container result with PS_OK must be applied (host genuinely has no containers)")
	}
	if len(report.Containers) != 0 {
		t.Fatalf("expected zero containers, got %d", len(report.Containers))
	}
}

func TestAgentDockerInventoryCollector_AllowlistSkipsUnlistedVMID(t *testing.T) {
	called := false
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, hostname string, command string, timeout int) (string, int, error) {
		called = true
		return "", 0, nil
	}, AgentDockerInventoryCollectorOptions{
		AllowedVMIDs: map[int]struct{}{102: {}},
	})

	_, ok, err := collector.CollectDockerInventory(context.Background(), models.Container{VMID: 101, Node: "node-a"})
	if err != nil {
		t.Fatalf("CollectDockerInventory returned error: %v", err)
	}
	if ok {
		t.Fatal("expected unlisted VMID to be skipped")
	}
	if called {
		t.Fatal("collector should not execute a command for an unlisted VMID")
	}
}

type mockDockerInventoryCollector struct {
	mu      sync.Mutex
	calls   []int
	reports map[int]agentsdocker.Report
}

func (m *mockDockerInventoryCollector) CollectDockerInventory(ctx context.Context, container models.Container) (agentsdocker.Report, bool, error) {
	m.mu.Lock()
	m.calls = append(m.calls, container.VMID)
	m.mu.Unlock()
	report, ok := m.reports[container.VMID]
	return report, ok, nil
}

func TestMonitorCollectProxmoxGuestDockerInventory_AppliesDockerReport(t *testing.T) {
	monitor := newTestMonitor(t)
	collector := &mockDockerInventoryCollector{
		reports: map[int]agentsdocker.Report{
			101: {
				Agent: agentsdocker.AgentInfo{ID: "proxmox-lxc-docker:pve-a:node-a:101", Type: "unified", IntervalSeconds: 30},
				Host: agentsdocker.HostInfo{
					Hostname:       "ct-web",
					Name:           "web-lxc (LXC 101)",
					Runtime:        "docker",
					DockerVersion:  "26.1.4",
					RuntimeVersion: "26.1.4",
				},
				Containers: []agentsdocker.Container{{
					ID:     "docker-1",
					Name:   "web",
					Image:  "nginx:latest",
					State:  "running",
					Status: "Up 1 minute",
				}},
				Timestamp: time.Now().UTC(),
			},
		},
	}
	monitor.SetDockerInventoryCollector(collector)

	monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{
		{ID: "pve-a:node-a:101", VMID: 101, Name: "web-lxc", Node: "node-a", Status: "running", HasDocker: true},
		{ID: "pve-a:node-a:102", VMID: 102, Name: "plain-lxc", Node: "node-a", Status: "running", HasDocker: false},
	})

	if len(collector.calls) != 1 || collector.calls[0] != 101 {
		t.Fatalf("collector calls = %#v, want [101]", collector.calls)
	}
	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected 1 Docker host, got %d", len(hosts))
	}
	if hosts[0].ID != "proxmox-lxc-docker:pve-a:node-a:101" {
		t.Fatalf("Docker host ID = %q", hosts[0].ID)
	}
	if len(hosts[0].Containers) != 1 || hosts[0].Containers[0].Name != "web" {
		t.Fatalf("unexpected Docker containers: %#v", hosts[0].Containers)
	}
}

func TestMonitorCollectProxmoxGuestDockerInventory_SkipsLinkedGuestLocalAgent(t *testing.T) {
	monitor := newTestMonitor(t)
	monitor.state.UpsertHost(models.Host{
		ID:                "host-agent-101",
		Hostname:          "web-lxc",
		Status:            "online",
		LinkedContainerID: "pve-a:node-a:101",
	})
	collector := &mockDockerInventoryCollector{reports: map[int]agentsdocker.Report{}}
	monitor.SetDockerInventoryCollector(collector)

	monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{
		{ID: "pve-a:node-a:101", VMID: 101, Name: "web-lxc", Node: "node-a", Status: "running", HasDocker: true},
	})

	if len(collector.calls) != 0 {
		t.Fatalf("collector should not run for a container with a linked online host agent, got calls %#v", collector.calls)
	}
}

func TestParseProxmoxGuestDockerInventoryVMIDs(t *testing.T) {
	allowed, invalid := ParseProxmoxGuestDockerInventoryVMIDs("101, 102, nope, -1, 101")
	if len(invalid) != 2 || invalid[0] != "nope" || invalid[1] != "-1" {
		t.Fatalf("invalid entries = %#v", invalid)
	}
	if _, ok := allowed[101]; !ok {
		t.Fatal("expected VMID 101 in allowlist")
	}
	if _, ok := allowed[102]; !ok {
		t.Fatal("expected VMID 102 in allowlist")
	}
	if len(allowed) != 2 {
		t.Fatalf("allowed = %#v", allowed)
	}
}

func TestState_GetContainers(t *testing.T) {
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{ID: "ct-1", VMID: 101, Name: "test1"},
		{ID: "ct-2", VMID: 102, Name: "test2"},
	})

	containers := state.GetContainers()

	if len(containers) != 2 {
		t.Errorf("Expected 2 containers, got %d", len(containers))
	}

	// Verify it's a copy (modifying shouldn't affect original)
	containers[0].Name = "modified"
	originalContainers := state.GetContainers()
	if originalContainers[0].Name == "modified" {
		t.Error("GetContainers should return a copy, not the original slice")
	}
}

func TestState_UpdateContainerDockerStatus(t *testing.T) {
	state := models.NewState()
	state.UpdateContainers([]models.Container{
		{ID: "ct-1", VMID: 101, Name: "test1"},
	})

	now := time.Now()
	updated := state.UpdateContainerDockerStatus("ct-1", true, now)

	if !updated {
		t.Error("UpdateContainerDockerStatus should return true for existing container")
	}

	containers := state.GetContainers()
	if !containers[0].HasDocker {
		t.Error("Container should have HasDocker=true")
	}
	if containers[0].DockerCheckedAt.IsZero() {
		t.Error("Container should have DockerCheckedAt set")
	}

	// Test non-existent container
	updated = state.UpdateContainerDockerStatus("non-existent", true, now)
	if updated {
		t.Error("UpdateContainerDockerStatus should return false for non-existent container")
	}
}

func TestContainerNeedsDockerCheck(t *testing.T) {
	monitor := &Monitor{
		state: models.NewState(),
	}

	tests := []struct {
		name       string
		container  models.Container
		previous   map[string]models.Container
		wantReason string
	}{
		{
			name:       "new container",
			container:  models.Container{ID: "ct-1", Status: "running"},
			previous:   map[string]models.Container{},
			wantReason: "new",
		},
		{
			name:      "first check - never checked before",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 3600},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "running", Uptime: 3600},
			},
			wantReason: "first_check",
		},
		{
			name:      "restarted - uptime lower",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 60},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "running", Uptime: 3600, DockerCheckedAt: time.Now()},
			},
			wantReason: "restarted",
		},
		{
			name:      "started - was stopped",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 60},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "stopped", DockerCheckedAt: time.Now()},
			},
			wantReason: "started",
		},
		{
			name:      "no check needed - same state",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 7200},
			previous: map[string]models.Container{
				"ct-1": {ID: "ct-1", Status: "running", Uptime: 3600, DockerCheckedAt: time.Now()},
			},
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason := monitor.containerNeedsDockerCheck(tt.container, tt.previous)
			if reason != tt.wantReason {
				t.Errorf("Expected reason %q, got %q", tt.wantReason, reason)
			}
		})
	}
}
