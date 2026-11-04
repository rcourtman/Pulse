package monitoring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// mockDockerChecker is a test implementation of DockerChecker
type mockDockerChecker struct {
	results map[int]bool  // vmid -> hasDocker
	errs    map[int]error // vmid -> probe error
	mu      sync.Mutex
	calls   []int // records vmids that were checked
}

func (m *mockDockerChecker) CheckDockerInContainer(ctx context.Context, container models.Container) (bool, error) {
	vmid := container.VMID
	m.mu.Lock()
	m.calls = append(m.calls, vmid)
	m.mu.Unlock()
	if err, ok := m.errs[vmid]; ok {
		return false, err
	}
	if hasDocker, ok := m.results[vmid]; ok {
		return hasDocker, nil
	}
	return false, nil
}

func (m *mockDockerChecker) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
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
			stdout:     proxmoxGuestDockerSocketMarker + "\tyes\n",
			exitCode:   0,
			wantDocker: true,
			wantErr:    false,
		},
		{
			name:       "docker socket does not exist",
			stdout:     proxmoxGuestDockerSocketMarker + "\tno\n",
			exitCode:   0,
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
		{
			name:       "host-side pct failure is not cached as no docker",
			stdout:     "no\npct: CT 101 is locked",
			exitCode:   255,
			wantDocker: false,
			wantErr:    true,
		},
		{
			name:       "missing marker is an error",
			stdout:     "no\n",
			exitCode:   0,
			wantDocker: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := NewAgentDockerChecker(func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
				return tt.stdout, tt.exitCode, nil
			})

			hasDocker, err := checker.CheckDockerInContainer(context.Background(), models.Container{Node: "node1", VMID: 101})

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

func TestAgentDockerChecker_ProbeRunsInsideContainer(t *testing.T) {
	var gotCommand string
	checker := NewAgentDockerChecker(func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
		gotCommand = command
		return proxmoxGuestDockerSocketMarker + "\tyes\n", 0, nil
	})

	hasDocker, err := checker.CheckDockerInContainer(context.Background(), models.Container{Node: "node1", VMID: 101})
	if err != nil {
		t.Fatalf("CheckDockerInContainer returned error: %v", err)
	}
	if !hasDocker {
		t.Fatal("expected Docker to be detected")
	}
	if !strings.Contains(gotCommand, "pct exec 101 -- sh -c") {
		t.Fatalf("expected probe to execute inside the LXC, got %q", gotCommand)
	}
	if strings.Contains(gotCommand, "&& echo yes || echo no") {
		t.Fatalf("host-side shell fallback must not mask pct exec failures: %q", gotCommand)
	}
}

func TestAgentDockerInventoryCollector_MinimalCommandAndReport(t *testing.T) {
	var gotCommand string
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
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
	if report.Host.Name != "web-lxc" {
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
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
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
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
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

func TestEnrichGuestDockerReportFromContainer_FillsMissingHostMetrics(t *testing.T) {
	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			Hostname:         "web-lxc",
			TotalCPU:         4,
			TotalMemoryBytes: 4 * 1024 * 1024 * 1024,
		},
	}
	container := models.Container{
		VMID:   141,
		Name:   "homepage-docker",
		CPU:    0.42,
		Uptime: 86400,
		Memory: models.Memory{Total: 4 * 1024 * 1024 * 1024, Used: 1 * 1024 * 1024 * 1024, Free: 3 * 1024 * 1024 * 1024, Usage: 25},
		Disk:   models.Disk{Total: 8 * 1024 * 1024 * 1024, Used: 2 * 1024 * 1024 * 1024, Free: 6 * 1024 * 1024 * 1024, Usage: 25},
	}

	enrichGuestDockerReportFromContainer(&report, container)

	if report.Host.CPUUsagePercent != 42 {
		t.Fatalf("CPUUsagePercent = %v, want 42", report.Host.CPUUsagePercent)
	}
	if report.Host.Memory.UsedBytes != 1*1024*1024*1024 || report.Host.Memory.TotalBytes != 4*1024*1024*1024 || report.Host.Memory.Usage != 25 {
		t.Fatalf("Memory enrichment failed: %#v", report.Host.Memory)
	}
	if len(report.Host.Disks) != 1 || report.Host.Disks[0].Mountpoint != "/" || report.Host.Disks[0].UsedBytes != 2*1024*1024*1024 {
		t.Fatalf("Disks enrichment failed: %#v", report.Host.Disks)
	}
	if report.Host.UptimeSeconds != 86400 {
		t.Fatalf("UptimeSeconds = %d, want 86400", report.Host.UptimeSeconds)
	}
}

func TestEnrichGuestDockerReportFromContainer_DoesNotOverrideReportedValues(t *testing.T) {
	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			CPUUsagePercent: 7,
			Memory:          agentsdocker.MemoryMetric{TotalBytes: 1, UsedBytes: 1, Usage: 99},
			Disks:           []agentsdocker.Disk{{Mountpoint: "/data", TotalBytes: 1}},
		},
	}
	container := models.Container{
		CPU:    0.5,
		Memory: models.Memory{Total: 1000, Used: 500, Usage: 50},
		Disk:   models.Disk{Total: 1000, Used: 500, Usage: 50},
	}

	enrichGuestDockerReportFromContainer(&report, container)

	if report.Host.CPUUsagePercent != 7 {
		t.Fatalf("CPUUsagePercent overwritten: %v", report.Host.CPUUsagePercent)
	}
	if report.Host.Memory.UsedBytes != 1 || report.Host.Memory.Usage != 99 {
		t.Fatalf("Memory overwritten: %#v", report.Host.Memory)
	}
	if len(report.Host.Disks) != 1 || report.Host.Disks[0].Mountpoint != "/data" {
		t.Fatalf("Disks overwritten: %#v", report.Host.Disks)
	}
}

func TestAgentDockerInventoryCollector_AllowlistSkipsUnlistedVMID(t *testing.T) {
	called := false
	collector := NewAgentDockerInventoryCollector(func(ctx context.Context, container models.Container, command string, timeout int) (string, int, error) {
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
					Name:           "web-lxc",
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

func TestMonitorCollectProxmoxGuestDockerInventory_RefusesEmptyOverwriteOfPopulatedHost(t *testing.T) {
	monitor := newTestMonitor(t)

	// First poll populates the docker host with one container.
	collector := &mockDockerInventoryCollector{
		reports: map[int]agentsdocker.Report{
			101: {
				Agent: agentsdocker.AgentInfo{ID: "proxmox-lxc-docker:pve-a:node-a:101", Type: "unified", IntervalSeconds: 30},
				Host: agentsdocker.HostInfo{
					Hostname:       "ct-web",
					Name:           "web-lxc",
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
	})

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 || len(hosts[0].Containers) != 1 {
		t.Fatalf("setup failed: expected 1 host with 1 container, got %d hosts", len(hosts))
	}

	// Second poll returns the same host but with zero containers — the
	// transient `docker ps` blip we want to ignore so the UI does not
	// flash empty until the next successful poll.
	collector.reports[101] = agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{ID: "proxmox-lxc-docker:pve-a:node-a:101", Type: "unified", IntervalSeconds: 30},
		Host: agentsdocker.HostInfo{
			Hostname:       "ct-web",
			Name:           "web-lxc",
			Runtime:        "docker",
			DockerVersion:  "26.1.4",
			RuntimeVersion: "26.1.4",
		},
		Containers: nil,
		Timestamp:  time.Now().UTC(),
	}

	monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{
		{ID: "pve-a:node-a:101", VMID: 101, Name: "web-lxc", Node: "node-a", Status: "running", HasDocker: true},
	})

	hosts = monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("docker host disappeared after empty report")
	}
	if len(hosts[0].Containers) != 1 || hosts[0].Containers[0].Name != "web" {
		t.Fatalf("empty inventory report wiped the previously-populated container list: %#v", hosts[0].Containers)
	}
}

func TestMonitorCollectProxmoxGuestDockerInventory_AppliesFirstZeroContainerReport(t *testing.T) {
	monitor := newTestMonitor(t)
	collector := &mockDockerInventoryCollector{
		reports: map[int]agentsdocker.Report{
			101: {
				Agent: agentsdocker.AgentInfo{ID: "proxmox-lxc-docker:pve-a:node-a:101", Type: "unified", IntervalSeconds: 30},
				Host: agentsdocker.HostInfo{
					Hostname:      "ct-web",
					Name:          "web-lxc",
					Runtime:       "docker",
					DockerVersion: "26.1.4",
				},
				Containers: nil,
				Timestamp:  time.Now().UTC(),
			},
		},
	}
	monitor.SetDockerInventoryCollector(collector)

	monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{
		{ID: "pve-a:node-a:101", VMID: 101, Name: "web-lxc", Node: "node-a", Status: "running", HasDocker: true},
	})

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected the host to be created from the first (empty) report, got %d hosts", len(hosts))
	}
	if len(hosts[0].Containers) != 0 {
		t.Fatalf("expected zero containers on a freshly-seen empty host, got %d", len(hosts[0].Containers))
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
		name                string
		container           models.Container
		previous            map[string]models.Container
		checkerConfiguredAt time.Time
		wantReason          string
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
			name:      "recheck expired negative docker result",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 7200},
			previous: map[string]models.Container{
				"ct-1": {
					ID:              "ct-1",
					Status:          "running",
					Uptime:          3600,
					HasDocker:       false,
					DockerCheckedAt: time.Now().Add(-proxmoxGuestDockerNegativeRecheckAfter - time.Second),
				},
			},
			wantReason: "negative_cache_expired",
		},
		{
			name:      "recheck negative docker result from previous checker generation",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 7200},
			previous: map[string]models.Container{
				"ct-1": {
					ID:              "ct-1",
					Status:          "running",
					Uptime:          3600,
					HasDocker:       false,
					DockerCheckedAt: time.Now().Add(-time.Minute),
				},
			},
			checkerConfiguredAt: time.Now(),
			wantReason:          "checker_reconfigured",
		},
		{
			name:      "keep fresh negative docker result",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 7200},
			previous: map[string]models.Container{
				"ct-1": {
					ID:              "ct-1",
					Status:          "running",
					Uptime:          3600,
					HasDocker:       false,
					DockerCheckedAt: time.Now().Add(-proxmoxGuestDockerNegativeRecheckAfter + time.Second),
				},
			},
			wantReason: "",
		},
		{
			name:      "keep positive docker result",
			container: models.Container{ID: "ct-1", Status: "running", Uptime: 7200},
			previous: map[string]models.Container{
				"ct-1": {
					ID:              "ct-1",
					Status:          "running",
					Uptime:          3600,
					HasDocker:       true,
					DockerCheckedAt: time.Now().Add(-proxmoxGuestDockerNegativeRecheckAfter - time.Hour),
				},
			},
			wantReason: "",
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
			reason := monitor.containerNeedsDockerCheck(tt.container, tt.previous, tt.checkerConfiguredAt)
			if reason != tt.wantReason {
				t.Errorf("Expected reason %q, got %q", tt.wantReason, reason)
			}
		})
	}
}

func TestCheckContainersForDocker_VMIDAllowlistGatesProbe(t *testing.T) {
	state := models.NewState()
	monitor := &Monitor{state: state}
	checker := &mockDockerChecker{
		results: map[int]bool{101: true, 102: true},
	}
	monitor.SetDockerChecker(checker)
	monitor.SetDockerCheckerAllowedVMIDs(map[int]struct{}{101: {}})

	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "allowed", Node: "node1", Status: "running"},
		{ID: "ct-2", VMID: 102, Name: "excluded", Node: "node1", Status: "running"},
	}

	result := monitor.CheckContainersForDocker(context.Background(), containers)

	if got := checker.callCount(); got != 1 {
		t.Fatalf("expected only the allowlisted guest to be probed, got %d probes", got)
	}
	if len(checker.calls) != 1 || checker.calls[0] != 101 {
		t.Fatalf("expected probe of VMID 101 only, got %v", checker.calls)
	}
	for i := range result {
		switch result[i].ID {
		case "ct-1":
			if !result[i].HasDocker || result[i].DockerCheckedAt.IsZero() {
				t.Errorf("allowlisted container should have been probed and marked")
			}
		case "ct-2":
			if result[i].HasDocker || !result[i].DockerCheckedAt.IsZero() {
				t.Errorf("excluded container must not be probed or marked")
			}
		}
	}

	// Clearing the allowlist re-enables probing for every guest.
	monitor.SetDockerCheckerAllowedVMIDs(nil)
	monitor.CheckContainersForDocker(context.Background(), containers)
	if got := checker.callCount(); got != 3 {
		t.Fatalf("expected both guests probed after clearing allowlist, got %d total probes", got)
	}
}

func TestCheckContainersForDocker_FailedProbeBacksOff(t *testing.T) {
	state := models.NewState()
	monitor := &Monitor{state: state}
	checker := &mockDockerChecker{
		errs: map[int]error{101: errors.New("write_id_mapping: Operation not permitted")},
	}
	monitor.SetDockerChecker(checker)

	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "failing", Node: "node1", Status: "running"},
	}

	// First poll: probe runs and fails.
	monitor.CheckContainersForDocker(context.Background(), containers)
	if got := checker.callCount(); got != 1 {
		t.Fatalf("expected 1 probe on first poll, got %d", got)
	}

	// Immediate next polls: still inside the backoff window, no re-probe.
	monitor.CheckContainersForDocker(context.Background(), containers)
	monitor.CheckContainersForDocker(context.Background(), containers)
	if got := checker.callCount(); got != 1 {
		t.Fatalf("expected failed probe to back off, got %d probes", got)
	}

	// Rewind the failure timestamp past the first backoff window: retries.
	monitor.dockerProbeFailureMu.Lock()
	monitor.dockerProbeFailures["ct-1"].lastAt = time.Now().Add(-2 * proxmoxGuestDockerProbeFailureBackoffBase)
	monitor.dockerProbeFailureMu.Unlock()
	monitor.CheckContainersForDocker(context.Background(), containers)
	if got := checker.callCount(); got != 2 {
		t.Fatalf("expected retry after backoff expiry, got %d probes", got)
	}
	monitor.dockerProbeFailureMu.Lock()
	failures := monitor.dockerProbeFailures["ct-1"].failures
	monitor.dockerProbeFailureMu.Unlock()
	if failures != 2 {
		t.Fatalf("expected 2 consecutive failures recorded, got %d", failures)
	}

	// Probe starts succeeding: streak clears and result applies.
	checker.errs = nil
	checker.results = map[int]bool{101: true}
	monitor.dockerProbeFailureMu.Lock()
	monitor.dockerProbeFailures["ct-1"].lastAt = time.Now().Add(-time.Hour)
	monitor.dockerProbeFailureMu.Unlock()
	result := monitor.CheckContainersForDocker(context.Background(), containers)
	if !result[0].HasDocker {
		t.Fatalf("expected successful probe to mark HasDocker")
	}
	monitor.dockerProbeFailureMu.Lock()
	_, stillTracked := monitor.dockerProbeFailures["ct-1"]
	monitor.dockerProbeFailureMu.Unlock()
	if stillTracked {
		t.Fatalf("expected failure streak to clear after success")
	}
}

func TestSetDockerCheckerResetsProbeFailureBackoff(t *testing.T) {
	state := models.NewState()
	monitor := &Monitor{state: state}
	checker := &mockDockerChecker{
		errs: map[int]error{101: errors.New("agent token not authorized for command execution")},
	}
	monitor.SetDockerChecker(checker)

	containers := []models.Container{
		{ID: "ct-1", VMID: 101, Name: "failing", Node: "node1", Status: "running"},
	}
	monitor.CheckContainersForDocker(context.Background(), containers)
	monitor.CheckContainersForDocker(context.Background(), containers)
	if got := checker.callCount(); got != 1 {
		t.Fatalf("expected backoff after first failure, got %d probes", got)
	}

	// Reconfiguring the checker (e.g. commands newly enabled) retries at once.
	monitor.SetDockerChecker(checker)
	monitor.CheckContainersForDocker(context.Background(), containers)
	if got := checker.callCount(); got != 2 {
		t.Fatalf("expected immediate retry after checker reconfiguration, got %d probes", got)
	}
}

func TestDockerProbeFailureBackoffSchedule(t *testing.T) {
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, 0},
		{1, proxmoxGuestDockerProbeFailureBackoffBase},
		{2, 2 * proxmoxGuestDockerProbeFailureBackoffBase},
		{5, 16 * proxmoxGuestDockerProbeFailureBackoffBase},
		{6, proxmoxGuestDockerProbeFailureBackoffMax},
		{100, proxmoxGuestDockerProbeFailureBackoffMax},
	}
	for _, tc := range cases {
		if got := dockerProbeFailureBackoff(tc.failures); got != tc.want {
			t.Errorf("backoff(%d) = %v, want %v", tc.failures, got, tc.want)
		}
	}
}

// Routing regressions for #2681: a Proxmox node name is not an address. Two
// connections can each have a node called "pve", and pct exec on the wrong one
// reads a different machine's guest under the same VMID.
func dockerRoutingMonitor(t *testing.T, nodes []models.Node, hosts []models.Host) *Monitor {
	t.Helper()
	monitor := readStateMonitor(nodes, hosts)
	instances := map[string]config.PVEInstance{}
	for _, node := range nodes {
		instances[node.Instance] = config.PVEInstance{Name: node.Instance}
	}
	cfg := &config.Config{}
	for _, instance := range instances {
		cfg.PVEInstances = append(cfg.PVEInstances, instance)
	}
	monitor.config = cfg
	// Monitoring's own state holds the link; the read state only finds the node.
	monitor.state.Nodes = nodes
	monitor.state.Hosts = hosts
	return monitor
}

func TestProxmoxNodeAgentForContainer(t *testing.T) {
	guest := func(instance, node string) models.Container {
		return models.Container{ID: instance + ":" + node + ":101", VMID: 101, Instance: instance, Node: node, Status: "running"}
	}
	sites := func() []models.Node {
		return []models.Node{readStateTestNode("site-a", "pve"), readStateTestNode("site-b", "pve")}
	}

	t.Run("each same-named node has only its own agent", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, sites(), []models.Host{
			{ID: "agent-a", Hostname: "pve.a.lan", TokenID: "token-a", LinkedNodeID: "site-a-pve"},
			{ID: "agent-b", Hostname: "pve.b.lan", TokenID: "token-b", LinkedNodeID: "site-b-pve"},
		})
		for instance, want := range map[string]ProxmoxNodeAgent{
			"site-a": {AgentID: "agent-a", Hostname: "pve.a.lan", TokenID: "token-a"},
			"site-b": {AgentID: "agent-b", Hostname: "pve.b.lan", TokenID: "token-b"},
		} {
			if got, ok := monitor.ProxmoxNodeAgentForContainer(guest(instance, "pve")); !ok || got != want {
				t.Errorf("%s: got %+v (proven=%v), want %+v", instance, got, ok, want)
			}
		}
	})

	t.Run("an agent linked to the other site's node proves nothing here", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, sites(), []models.Host{
			{ID: "agent-a", Hostname: "pve", TokenID: "token-a", LinkedNodeID: "site-a-pve"},
		})
		if got, ok := monitor.ProxmoxNodeAgentForContainer(guest("site-b", "pve")); ok {
			t.Fatalf("site-b's guest took site-a's agent: %+v", got)
		}
	})

	t.Run("a matching hostname alone proves nothing", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, []models.Node{readStateTestNode("home", "pve")}, []models.Host{
			{ID: "agent-x", Hostname: "pve", TokenID: "token-x"},
		})
		if got, ok := monitor.ProxmoxNodeAgentForContainer(guest("home", "pve")); ok {
			t.Fatalf("an unlinked agent was taken on its hostname: %+v", got)
		}
	})

	t.Run("two agents claiming one node prove neither", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, []models.Node{readStateTestNode("site-a", "pve")}, []models.Host{
			{ID: "agent-a", Hostname: "pve", LinkedNodeID: "site-a-pve"},
			{ID: "agent-b", Hostname: "pve", LinkedNodeID: "site-a-pve"},
		})
		if got, ok := monitor.ProxmoxNodeAgentForContainer(guest("site-a", "pve")); ok {
			t.Fatalf("one of two claimants was taken: %+v", got)
		}
	})

	t.Run("a link only the agent holds proves nothing", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, []models.Node{readStateTestNode("site-a", "pve")}, []models.Host{
			{ID: "agent-a", Hostname: "pve", LinkedNodeID: "site-a-pve"},
		})
		monitor.state.Nodes[0].LinkedAgentID = ""
		if got, ok := monitor.ProxmoxNodeAgentForContainer(guest("site-a", "pve")); ok {
			t.Fatalf("a one-sided link was taken: %+v", got)
		}
	})

	t.Run("a link only the node holds proves nothing", func(t *testing.T) {
		node := readStateTestNode("site-a", "pve")
		node.LinkedAgentID = "agent-a"
		monitor := dockerRoutingMonitor(t, []models.Node{node}, []models.Host{{ID: "agent-a", Hostname: "pve"}})
		if got, ok := monitor.ProxmoxNodeAgentForContainer(guest("site-a", "pve")); ok {
			t.Fatalf("a one-sided link was taken: %+v", got)
		}
	})

	t.Run("an operator's split ends the proof before the next node poll", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, []models.Node{readStateTestNode("site-a", "pve")}, []models.Host{
			{ID: "agent-a", Hostname: "pve", LinkedNodeID: "site-a-pve"},
		})
		if _, ok := monitor.ProxmoxNodeAgentForContainer(guest("site-a", "pve")); !ok {
			t.Fatal("expected the linked agent before the split")
		}
		// A real split reaches the links only when the state next reconciles, so
		// install a decider directly and restore the link it holds back.
		monitor.state.SetNodeAgentSplitDecider(dockerRoutingSplitDecider{nodeID: "site-a-pve", hostID: "agent-a"})
		monitor.state.Nodes[0].LinkedAgentID = "agent-a"
		monitor.state.Hosts[0].LinkedNodeID = "site-a-pve"
		if got, ok := monitor.ProxmoxNodeAgentForContainer(guest("site-a", "pve")); ok {
			t.Fatalf("a split agent was still used: %+v", got)
		}
	})

	t.Run("a guest without a configured connection proves nothing", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, sites(), []models.Host{{ID: "agent-a", Hostname: "pve", LinkedNodeID: "site-a-pve"}})
		for _, ct := range []models.Container{guest("elsewhere", "pve"), guest("", "pve"), guest("site-a", "")} {
			if got, ok := monitor.ProxmoxNodeAgentForContainer(ct); ok {
				t.Errorf("%+v took %+v", ct, got)
			}
		}
	})

	t.Run("one cluster added through two connections keeps its agent on both", func(t *testing.T) {
		var instances []config.PVEInstance
		for _, name := range []string{"enacon-a", "enacon-b"} {
			instances = append(instances, config.PVEInstance{
				Name: name, IsCluster: true, ClusterName: "enacon",
				ClusterEndpoints: []config.ClusterEndpoint{{NodeName: "pve01", Host: "https://192.168.1.11:8006", Fingerprint: "AA:AA"}},
			})
		}
		adapter := unifiedresources.NewMonitorAdapter(unifiedresources.NewRegistry(nil))
		monitor := &Monitor{config: &config.Config{PVEInstances: instances}, state: models.NewState(), resourceStore: adapter}
		view := func(instanceCfg *config.PVEInstance) models.Node {
			return models.Node{
				ID: monitor.pveNodeID(instanceCfg.Name, instanceCfg, "pve01"), NodeIdentity: monitor.pveNodeID(instanceCfg.Name, instanceCfg, "pve01"),
				Name: "pve01", Instance: instanceCfg.Name, ClusterName: "enacon", IsClusterMember: true,
				Host: "https://192.168.1.11:8006", TLSFingerprint: pveNodeTLSFingerprint(instanceCfg, "pve01"), Status: "online", LastSeen: time.Now(),
			}
		}
		first := view(&instances[0])
		monitor.state.UpdateNodesForInstance("enacon-a", []models.Node{first})
		monitor.state.UpsertHost(models.Host{ID: "agent-pve01", Hostname: "pve01", Status: "online", LastSeen: time.Now()})
		if err := monitor.state.LinkHostAgentToNode("agent-pve01", first.ID); err != nil {
			t.Fatalf("LinkHostAgentToNode: %v", err)
		}
		monitor.state.UpdateNodesForInstance("enacon-b", []models.Node{view(&instances[1])})
		adapter.PopulateFromSnapshot(monitor.state.GetSnapshot())

		for _, instance := range []string{"enacon-a", "enacon-b"} {
			if got, ok := monitor.ProxmoxNodeAgentForContainer(guest(instance, "pve01")); !ok || got.AgentID != "agent-pve01" {
				t.Errorf("%s: got %+v (proven=%v), want agent-pve01", instance, got, ok)
			}
		}
	})
}

// A guest with no proven agent is a routing decision, not a failure: nothing is
// dispatched, the prior answer stands, no backoff or breaker builds up, and an
// agent linked later is used on the next poll.
func TestCheckContainersForDocker_NoProvenNodeAgentIsNotAFailure(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}
	checker := &mockDockerChecker{
		errs: map[int]error{101: fmt.Errorf("command execution failed: %w", ErrNoProvenProxmoxNodeAgent)},
	}
	monitor.SetDockerChecker(checker)
	containers := func() []models.Container {
		return []models.Container{{ID: "ct-1", VMID: 101, Name: "guest", Instance: "site-b", Node: "pve", Status: "running"}}
	}

	for cycle := 0; cycle < proxmoxGuestDockerNodeFailureThreshold+1; cycle++ {
		result := monitor.CheckContainersForDocker(context.Background(), containers())
		if !result[0].DockerCheckedAt.IsZero() || result[0].HasDocker {
			t.Fatalf("cycle %d: guest marked as checked: %+v", cycle, result[0])
		}
		monitor.state.UpdateContainers(result)
	}
	if got := checker.callCount(); got != proxmoxGuestDockerNodeFailureThreshold+1 {
		t.Fatalf("expected a fresh decision every poll, got %d", got)
	}

	monitor.dockerProbeFailureMu.Lock()
	guestFailures := len(monitor.dockerProbeFailures)
	nodeFailures := len(monitor.dockerNodeProbeFailures)
	inFlight := len(monitor.dockerProbesInFlight)
	_, noted := monitor.dockerNodesWithoutProvenAgent[dockerNodeProbeKey(containers()[0])]
	monitor.dockerProbeFailureMu.Unlock()
	if guestFailures != 0 || nodeFailures != 0 || inFlight != 0 {
		t.Fatalf("skip left state behind: guest=%d node=%d inFlight=%d", guestFailures, nodeFailures, inFlight)
	}
	if !noted {
		t.Fatal("expected the node to be noted as having no proven agent")
	}

	// Linking the agent later takes effect on the next poll.
	checker.mu.Lock()
	delete(checker.errs, 101)
	checker.results = map[int]bool{101: true}
	checker.mu.Unlock()
	if result := monitor.CheckContainersForDocker(context.Background(), containers()); !result[0].HasDocker {
		t.Fatalf("guest not probed after its agent was linked: %+v", result[0])
	}
}

// A guest that restarts while no agent is proven must not lose that recheck:
// the restart is no longer visible once the poll is published.
func TestCheckContainersForDocker_SkippedRestartIsAskedAgain(t *testing.T) {
	state := models.NewState()
	state.UpdateContainers([]models.Container{{
		ID: "ct-1", VMID: 101, Instance: "site-b", Node: "pve", Status: "running",
		HasDocker: true, DockerCheckedAt: time.Now().Add(-time.Hour), Uptime: 3600,
	}})
	monitor := &Monitor{state: state}
	checker := &mockDockerChecker{errs: map[int]error{101: ErrNoProvenProxmoxNodeAgent}}
	monitor.SetDockerChecker(checker)

	restarted := []models.Container{{ID: "ct-1", VMID: 101, Instance: "site-b", Node: "pve", Status: "running", Uptime: 60}}
	result := monitor.CheckContainersForDocker(context.Background(), restarted)
	if !result[0].HasDocker {
		t.Fatalf("skip dropped the prior answer: %+v", result[0])
	}
	if !result[0].DockerCheckedAt.IsZero() {
		t.Fatalf("skip kept the old check time, so the guest would not be asked again: %+v", result[0])
	}
	state.UpdateContainers(result)

	// The agent is proven now, and Docker went away during the restart.
	checker.mu.Lock()
	checker.errs = nil
	checker.results = map[int]bool{101: false}
	checker.mu.Unlock()
	result = monitor.CheckContainersForDocker(context.Background(), []models.Container{
		{ID: "ct-1", VMID: 101, Instance: "site-b", Node: "pve", Status: "running", Uptime: 120},
	})
	if result[0].HasDocker || result[0].DockerCheckedAt.IsZero() {
		t.Fatalf("guest was not asked again once an agent was proven: %+v", result[0])
	}
}

// A shared token proves nothing about which machine a session belongs to. Live
// state decides, because a same-token agent can hold a session before the read
// state lists it.
func TestProxmoxNodeAgentOwnsTokenSession(t *testing.T) {
	agent := ProxmoxNodeAgent{AgentID: "agent-b", Hostname: "pve", TokenID: "shared"}
	nodes := []models.Node{readStateTestNode("site-b", "pve")}
	target := models.Host{ID: "agent-b", Hostname: "pve", TokenID: "shared", LinkedNodeID: "site-b-pve"}

	t.Run("only this agent holds the token", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, nodes, []models.Host{target})
		if !monitor.ProxmoxNodeAgentOwnsTokenSession(agent, "earlier-id") {
			t.Fatal("a session under an unknown earlier ID should be this agent's")
		}
	})

	t.Run("another agent under an equivalent hostname holds it", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, nodes, []models.Host{target, {ID: "agent-a", Hostname: "pve.lan", TokenID: "shared"}})
		if monitor.ProxmoxNodeAgentOwnsTokenSession(agent, "earlier-id") {
			t.Fatal("a token two same-named agents hold identified one of them")
		}
	})

	t.Run("the other agent is in live state but not yet in the read state", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, nodes, []models.Host{target})
		monitor.state.Hosts = append(monitor.state.Hosts, models.Host{ID: "agent-a", Hostname: "pve", TokenID: "shared"})
		if monitor.ProxmoxNodeAgentOwnsTokenSession(agent, "agent-a") {
			t.Fatal("the read state's stale host list decided the session's owner")
		}
	})

	t.Run("the session belongs to another known agent", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, nodes, []models.Host{target, {ID: "agent-a", Hostname: "pve", TokenID: "rotated"}})
		if monitor.ProxmoxNodeAgentOwnsTokenSession(agent, "agent-a") {
			t.Fatal("another agent's session was taken as an earlier ID")
		}
	})

	t.Run("a token shared under another hostname does not stop it", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, nodes, []models.Host{target, {ID: "agent-c", Hostname: "backup", TokenID: "shared"}})
		if !monitor.ProxmoxNodeAgentOwnsTokenSession(agent, "earlier-id") {
			t.Fatal("a differently named machine on the token blocked the lookup")
		}
	})

	t.Run("no token proves nothing", func(t *testing.T) {
		monitor := dockerRoutingMonitor(t, nodes, []models.Host{target})
		if monitor.ProxmoxNodeAgentOwnsTokenSession(ProxmoxNodeAgent{AgentID: "agent-b", Hostname: "pve"}, "earlier-id") {
			t.Fatal("a session was accepted with no token")
		}
	})
}

type dockerRoutingSplitDecider struct{ nodeID, hostID string }

func (d dockerRoutingSplitDecider) ProxmoxNodeAgentSplit(node models.Node, host models.Host) bool {
	return node.ID == d.nodeID && host.ID == d.hostID
}

type routedInventoryCollector struct {
	err error
}

func (c *routedInventoryCollector) CollectDockerInventory(context.Context, models.Container) (agentsdocker.Report, bool, error) {
	return agentsdocker.Report{}, false, c.err
}

func TestCollectProxmoxGuestDockerInventory_NoProvenNodeAgentIsNotAFailure(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}
	monitor.SetDockerInventoryCollector(&routedInventoryCollector{err: fmt.Errorf("command execution failed: %w", ErrNoProvenProxmoxNodeAgent)})
	container := models.Container{ID: "ct-1", VMID: 101, Name: "guest", Instance: "site-b", Node: "pve", Status: "running", HasDocker: true}

	for cycle := 0; cycle < proxmoxGuestDockerNodeFailureThreshold+1; cycle++ {
		monitor.CollectProxmoxGuestDockerInventory(context.Background(), []models.Container{container})
	}

	monitor.dockerProbeFailureMu.Lock()
	defer monitor.dockerProbeFailureMu.Unlock()
	if len(monitor.dockerNodeProbeFailures) != 0 || len(monitor.dockerProbesInFlight) != 0 {
		t.Fatalf("skip left state behind: node=%d inFlight=%d", len(monitor.dockerNodeProbeFailures), len(monitor.dockerProbesInFlight))
	}
}

// A same-named node on another connection is another machine, so its failing
// commands must not stop this node's probes, and this node's success must not
// close that node's breaker.
func TestCheckContainersForDocker_NodeCircuitBreakerScopedByConnection(t *testing.T) {
	monitor := &Monitor{state: models.NewState()}
	checker := &mockDockerChecker{
		errs:    map[int]error{101: errors.New("pct exec hung"), 102: errors.New("pct exec hung"), 103: errors.New("pct exec hung")},
		results: map[int]bool{201: true},
	}
	monitor.SetDockerChecker(checker)
	failing := models.Container{Instance: "site-a", Node: "pve"}
	healthy := models.Container{Instance: "site-b", Node: "pve"}

	monitor.CheckContainersForDocker(context.Background(), []models.Container{
		{ID: "a-1", VMID: 101, Instance: "site-a", Node: "pve", Status: "running"},
		{ID: "a-2", VMID: 102, Instance: "site-a", Node: "pve", Status: "running"},
		{ID: "a-3", VMID: 103, Instance: "site-a", Node: "pve", Status: "running"},
	})
	if !monitor.dockerNodeProbeBreakerOpen(failing) {
		t.Fatal("expected site-a's breaker to open")
	}
	if monitor.dockerNodeProbeBreakerOpen(healthy) {
		t.Fatal("site-a's failures opened site-b's breaker")
	}

	result := monitor.CheckContainersForDocker(context.Background(), []models.Container{
		{ID: "b-1", VMID: 201, Instance: "site-b", Node: "pve", Status: "running"},
	})
	if !result[0].HasDocker {
		t.Fatalf("site-b's guest was not probed while site-a's breaker was open: %+v", result[0])
	}
	if !monitor.dockerNodeProbeBreakerOpen(failing) {
		t.Fatal("site-b's success closed site-a's breaker")
	}
}
