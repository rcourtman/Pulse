package dockeragent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestReport_AgentKey(t *testing.T) {
	tests := []struct {
		name     string
		report   Report
		expected string
	}{
		{
			name: "uses agent ID first",
			report: Report{
				Agent: AgentInfo{ID: "agent-123"},
				Host:  HostInfo{MachineID: "machine-456", Hostname: "myhost"},
			},
			expected: "agent-123",
		},
		{
			name: "falls back to machine ID",
			report: Report{
				Agent: AgentInfo{ID: ""},
				Host:  HostInfo{MachineID: "machine-456", Hostname: "myhost"},
			},
			expected: "machine-456",
		},
		{
			name: "falls back to hostname",
			report: Report{
				Agent: AgentInfo{ID: ""},
				Host:  HostInfo{MachineID: "", Hostname: "myhost"},
			},
			expected: "myhost",
		},
		{
			name: "empty report returns empty hostname",
			report: Report{
				Agent: AgentInfo{},
				Host:  HostInfo{},
			},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.report.AgentKey()
			if result != tc.expected {
				t.Errorf("AgentKey() = %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestReport_JSONMarshal(t *testing.T) {
	now := time.Now()
	report := Report{
		Agent: AgentInfo{
			ID:              "docker-agent-1",
			Version:         "1.0.0",
			Type:            "docker",
			IntervalSeconds: 30,
		},
		Host: HostInfo{
			Hostname:         "docker-host",
			DockerVersion:    "24.0.0",
			TotalCPU:         8,
			TotalMemoryBytes: 16000000000,
		},
		Containers: []Container{
			{
				ID:    "abc123",
				Name:  "nginx",
				Image: "nginx:latest",
				State: "running",
			},
		},
		Timestamp: now,
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Failed to marshal Report: %v", err)
	}

	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal Report: %v", err)
	}

	if decoded.Agent.ID != report.Agent.ID {
		t.Errorf("Agent.ID = %q, want %q", decoded.Agent.ID, report.Agent.ID)
	}
	if len(decoded.Containers) != 1 {
		t.Errorf("Containers count = %d, want 1", len(decoded.Containers))
	}
}

func TestAgentInfo_Fields(t *testing.T) {
	agent := AgentInfo{
		ID:              "agent-456",
		Version:         "2.0.0",
		Type:            "unified",
		IntervalSeconds: 60,
	}

	if agent.ID != "agent-456" {
		t.Errorf("ID = %q, want agent-456", agent.ID)
	}
	if agent.Version != "2.0.0" {
		t.Errorf("Version = %q, want 2.0.0", agent.Version)
	}
	if agent.Type != "unified" {
		t.Errorf("Type = %q, want unified", agent.Type)
	}
	if agent.IntervalSeconds != 60 {
		t.Errorf("IntervalSeconds = %d, want 60", agent.IntervalSeconds)
	}
}

func TestHostInfo_Fields(t *testing.T) {
	host := HostInfo{
		Hostname:         "docker-server",
		Name:             "Docker Server",
		MachineID:        "machine123",
		OS:               "linux",
		Runtime:          "docker",
		RuntimeVersion:   "24.0.0",
		KernelVersion:    "5.15.0",
		Architecture:     "x86_64",
		DockerVersion:    "24.0.0",
		TotalCPU:         16,
		TotalMemoryBytes: 32000000000,
		UptimeSeconds:    86400,
		CPUUsagePercent:  25.5,
		LoadAverage:      []float64{1.0, 0.8, 0.6},
	}

	if host.Hostname != "docker-server" {
		t.Errorf("Hostname = %q, want docker-server", host.Hostname)
	}
	if host.Runtime != "docker" {
		t.Errorf("Runtime = %q, want docker", host.Runtime)
	}
	if host.TotalCPU != 16 {
		t.Errorf("TotalCPU = %d, want 16", host.TotalCPU)
	}
	if len(host.LoadAverage) != 3 {
		t.Errorf("LoadAverage length = %d, want 3", len(host.LoadAverage))
	}
}

func TestContainer_Fields(t *testing.T) {
	now := time.Now()
	container := Container{
		ID:               "container123",
		Name:             "web-app",
		Image:            "myapp:v1.2.3",
		CreatedAt:        now,
		State:            "running",
		Status:           "Up 2 hours",
		Health:           "healthy",
		CPUPercent:       15.5,
		MemoryUsageBytes: 512000000,
		MemoryLimitBytes: 1024000000,
		MemoryPercent:    50.0,
		UptimeSeconds:    7200,
		RestartCount:     0,
		ExitCode:         0,
	}

	if container.ID != "container123" {
		t.Errorf("ID = %q, want container123", container.ID)
	}
	if container.State != "running" {
		t.Errorf("State = %q, want running", container.State)
	}
	if container.Health != "healthy" {
		t.Errorf("Health = %q, want healthy", container.Health)
	}
	if container.CPUPercent != 15.5 {
		t.Errorf("CPUPercent = %f, want 15.5", container.CPUPercent)
	}
	if container.MemoryPercent != 50.0 {
		t.Errorf("MemoryPercent = %f, want 50.0", container.MemoryPercent)
	}
}

func TestContainerPort_Fields(t *testing.T) {
	port := ContainerPort{
		PrivatePort: 80,
		PublicPort:  8080,
		Protocol:    "tcp",
		IP:          "0.0.0.0",
	}

	if port.PrivatePort != 80 {
		t.Errorf("PrivatePort = %d, want 80", port.PrivatePort)
	}
	if port.PublicPort != 8080 {
		t.Errorf("PublicPort = %d, want 8080", port.PublicPort)
	}
	if port.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want tcp", port.Protocol)
	}
}

func TestContainerNetwork_Fields(t *testing.T) {
	network := ContainerNetwork{
		Name: "bridge",
		IPv4: "172.17.0.2",
		IPv6: "fe80::1",
	}

	if network.Name != "bridge" {
		t.Errorf("Name = %q, want bridge", network.Name)
	}
	if network.IPv4 != "172.17.0.2" {
		t.Errorf("IPv4 = %q, want 172.17.0.2", network.IPv4)
	}
}

func TestContainerBlockIO_Fields(t *testing.T) {
	blockIO := ContainerBlockIO{
		ReadBytes:  1000000,
		WriteBytes: 500000,
	}

	if blockIO.ReadBytes != 1000000 {
		t.Errorf("ReadBytes = %d, want 1000000", blockIO.ReadBytes)
	}
	if blockIO.WriteBytes != 500000 {
		t.Errorf("WriteBytes = %d, want 500000", blockIO.WriteBytes)
	}
}

func TestContainerMount_Fields(t *testing.T) {
	mount := ContainerMount{
		Type:        "bind",
		Source:      "/host/data",
		Destination: "/data",
		Mode:        "rw",
		RW:          true,
		Propagation: "rprivate",
	}

	if mount.Type != "bind" {
		t.Errorf("Type = %q, want bind", mount.Type)
	}
	if mount.Source != "/host/data" {
		t.Errorf("Source = %q, want /host/data", mount.Source)
	}
	if !mount.RW {
		t.Error("RW should be true")
	}
}

func TestPodmanContainer_Fields(t *testing.T) {
	podman := PodmanContainer{
		PodName:           "mypod",
		PodID:             "pod123",
		Infra:             false,
		ComposeProject:    "myproject",
		ComposeService:    "web",
		AutoUpdatePolicy:  "registry",
		AutoUpdateRestart: "always",
	}

	if podman.PodName != "mypod" {
		t.Errorf("PodName = %q, want mypod", podman.PodName)
	}
	if podman.ComposeProject != "myproject" {
		t.Errorf("ComposeProject = %q, want myproject", podman.ComposeProject)
	}
	if podman.AutoUpdatePolicy != "registry" {
		t.Errorf("AutoUpdatePolicy = %q, want registry", podman.AutoUpdatePolicy)
	}
}

func TestSwarmInfo_Fields(t *testing.T) {
	swarm := SwarmInfo{
		NodeID:           "node123",
		NodeRole:         "manager",
		LocalState:       "active",
		ControlAvailable: true,
		ClusterID:        "cluster456",
		ClusterName:      "my-swarm",
		Scope:            "swarm",
	}

	if swarm.NodeID != "node123" {
		t.Errorf("NodeID = %q, want node123", swarm.NodeID)
	}
	if swarm.NodeRole != "manager" {
		t.Errorf("NodeRole = %q, want manager", swarm.NodeRole)
	}
	if !swarm.ControlAvailable {
		t.Error("ControlAvailable should be true")
	}
}

func TestService_Fields(t *testing.T) {
	service := Service{
		ID:             "service123",
		Name:           "web",
		Stack:          "mystack",
		Image:          "nginx:latest",
		Mode:           "replicated",
		DesiredTasks:   3,
		RunningTasks:   3,
		CompletedTasks: 0,
	}

	if service.ID != "service123" {
		t.Errorf("ID = %q, want service123", service.ID)
	}
	if service.Mode != "replicated" {
		t.Errorf("Mode = %q, want replicated", service.Mode)
	}
	if service.DesiredTasks != 3 {
		t.Errorf("DesiredTasks = %d, want 3", service.DesiredTasks)
	}
}

func TestServicePort_Fields(t *testing.T) {
	port := ServicePort{
		Name:          "http",
		Protocol:      "tcp",
		TargetPort:    80,
		PublishedPort: 8080,
		PublishMode:   "ingress",
	}

	if port.Name != "http" {
		t.Errorf("Name = %q, want http", port.Name)
	}
	if port.TargetPort != 80 {
		t.Errorf("TargetPort = %d, want 80", port.TargetPort)
	}
	if port.PublishMode != "ingress" {
		t.Errorf("PublishMode = %q, want ingress", port.PublishMode)
	}
}

func TestServiceUpdate_Fields(t *testing.T) {
	now := time.Now()
	update := ServiceUpdate{
		State:       "completed",
		Message:     "update completed",
		CompletedAt: &now,
	}

	if update.State != "completed" {
		t.Errorf("State = %q, want completed", update.State)
	}
	if update.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestTask_Fields(t *testing.T) {
	now := time.Now()
	task := Task{
		ID:           "task123",
		ServiceID:    "service456",
		ServiceName:  "web",
		Slot:         1,
		NodeID:       "node789",
		NodeName:     "docker-node-1",
		DesiredState: "running",
		CurrentState: "running",
		ContainerID:  "container000",
		CreatedAt:    now,
	}

	if task.ID != "task123" {
		t.Errorf("ID = %q, want task123", task.ID)
	}
	if task.Slot != 1 {
		t.Errorf("Slot = %d, want 1", task.Slot)
	}
	if task.DesiredState != "running" {
		t.Errorf("DesiredState = %q, want running", task.DesiredState)
	}
}

func TestCommand_Fields(t *testing.T) {
	cmd := Command{
		ID:   "cmd-123",
		Type: CommandTypeStop,
		Payload: map[string]any{
			"reason": "shutdown",
		},
	}

	if cmd.ID != "cmd-123" {
		t.Errorf("ID = %q, want cmd-123", cmd.ID)
	}
	if cmd.Type != CommandTypeStop {
		t.Errorf("Type = %q, want %q", cmd.Type, CommandTypeStop)
	}
	if cmd.Payload["reason"] != "shutdown" {
		t.Errorf("Payload[reason] = %v, want shutdown", cmd.Payload["reason"])
	}
}

func TestReportResponse_Fields(t *testing.T) {
	resp := ReportResponse{
		Success: true,
		Commands: []Command{
			{ID: "cmd-1", Type: CommandTypeStop},
		},
	}

	if !resp.Success {
		t.Error("Success should be true")
	}
	if len(resp.Commands) != 1 {
		t.Errorf("Commands count = %d, want 1", len(resp.Commands))
	}
}

func TestCommandAck_Fields(t *testing.T) {
	ack := CommandAck{
		HostID:  "host-123",
		Status:  CommandStatusCompleted,
		Message: "Command executed successfully",
	}

	if ack.HostID != "host-123" {
		t.Errorf("HostID = %q, want host-123", ack.HostID)
	}
	if ack.Status != CommandStatusCompleted {
		t.Errorf("Status = %q, want %q", ack.Status, CommandStatusCompleted)
	}
}

func TestCommandConstants(t *testing.T) {
	// Verify constants are defined
	if CommandTypeStop != "stop" {
		t.Errorf("CommandTypeStop = %q, want stop", CommandTypeStop)
	}
	if CommandStatusAcknowledged != "acknowledged" {
		t.Errorf("CommandStatusAcknowledged = %q, want acknowledged", CommandStatusAcknowledged)
	}
	if CommandStatusCompleted != "completed" {
		t.Errorf("CommandStatusCompleted = %q, want completed", CommandStatusCompleted)
	}
	if CommandStatusFailed != "failed" {
		t.Errorf("CommandStatusFailed = %q, want failed", CommandStatusFailed)
	}
}

func TestReport_WithSwarm(t *testing.T) {
	report := Report{
		Host: HostInfo{
			Hostname: "swarm-manager",
			Swarm: &SwarmInfo{
				NodeID:           "node1",
				NodeRole:         "manager",
				ControlAvailable: true,
			},
		},
		Services: []Service{
			{ID: "svc1", Name: "web", DesiredTasks: 3, RunningTasks: 3},
		},
		Tasks: []Task{
			{ID: "task1", ServiceName: "web", CurrentState: "running"},
		},
	}

	if report.Host.Swarm == nil {
		t.Error("Swarm should not be nil")
	}
	if len(report.Services) != 1 {
		t.Errorf("Services count = %d, want 1", len(report.Services))
	}
	if len(report.Tasks) != 1 {
		t.Errorf("Tasks count = %d, want 1", len(report.Tasks))
	}
}
