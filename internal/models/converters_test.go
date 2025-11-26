package models

import (
	"strings"
	"testing"
	"time"
)

func TestZeroIfNegative(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected int64
	}{
		{"positive value", 100, 100},
		{"zero", 0, 0},
		{"negative value", -50, 0},
		{"large negative", -999999, 0},
		{"large positive", 999999, 999999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := zeroIfNegative(tc.input)
			if result != tc.expected {
				t.Errorf("zeroIfNegative(%d) = %d, want %d", tc.input, result, tc.expected)
			}
		})
	}
}

func TestCopyStringFloatMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]float64
		nilCheck bool
	}{
		{
			name:     "nil map",
			input:    nil,
			nilCheck: true,
		},
		{
			name:     "empty map",
			input:    map[string]float64{},
			nilCheck: true,
		},
		{
			name: "populated map",
			input: map[string]float64{
				"cpu":  45.0,
				"gpu":  65.0,
				"nvme": 40.0,
			},
			nilCheck: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := copyStringFloatMap(tc.input)
			if tc.nilCheck {
				if result != nil {
					t.Errorf("copyStringFloatMap(%v) = %v, want nil", tc.input, result)
				}
				return
			}

			// Verify copy is separate from original
			if len(result) != len(tc.input) {
				t.Errorf("copyStringFloatMap length = %d, want %d", len(result), len(tc.input))
			}

			for k, v := range tc.input {
				if result[k] != v {
					t.Errorf("result[%s] = %f, want %f", k, result[k], v)
				}
			}

			// Verify it's a copy, not the same reference
			if tc.input != nil && len(tc.input) > 0 {
				for k := range result {
					result[k] = 999.0
					break
				}
				// Original should be unchanged
				for k, v := range tc.input {
					if v == 999.0 && tc.input[k] != 999.0 {
						t.Error("copyStringFloatMap should create a copy, not reference")
					}
					break
				}
			}
		})
	}
}

func TestNodeToFrontend(t *testing.T) {
	now := time.Now()
	node := Node{
		ID:          "node-1",
		Name:        "pve1",
		DisplayName: "Proxmox Node 1",
		Instance:    "default",
		Host:        "192.168.1.100",
		Status:      "online",
		Type:        "node",
		CPU:         0.25,
		Memory: Memory{
			Total: 16000000000,
			Used:  8000000000,
		},
		Disk: Disk{
			Total: 500000000000,
			Used:  250000000000,
		},
		Uptime:           86400,
		LoadAverage:      []float64{0.5, 0.7, 0.9},
		KernelVersion:    "5.15.0",
		PVEVersion:       "7.4-3",
		LastSeen:         now,
		ConnectionHealth: "connected",
		IsClusterMember:  true,
		ClusterName:      "pve-cluster",
	}

	frontend := node.ToFrontend()

	if frontend.ID != node.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, node.ID)
	}
	if frontend.Node != node.Name {
		t.Errorf("Node = %q, want %q", frontend.Node, node.Name)
	}
	if frontend.DisplayName != node.DisplayName {
		t.Errorf("DisplayName = %q, want %q", frontend.DisplayName, node.DisplayName)
	}
	if frontend.Status != node.Status {
		t.Errorf("Status = %q, want %q", frontend.Status, node.Status)
	}
	if frontend.CPU != node.CPU {
		t.Errorf("CPU = %f, want %f", frontend.CPU, node.CPU)
	}
	if frontend.Mem != node.Memory.Used {
		t.Errorf("Mem = %d, want %d", frontend.Mem, node.Memory.Used)
	}
	if frontend.MaxMem != node.Memory.Total {
		t.Errorf("MaxMem = %d, want %d", frontend.MaxMem, node.Memory.Total)
	}
	if frontend.Memory == nil {
		t.Error("Memory should not be nil when Total > 0")
	}
	if frontend.Disk == nil {
		t.Error("Disk should not be nil when Total > 0")
	}
	if frontend.LastSeen != now.Unix()*1000 {
		t.Errorf("LastSeen = %d, want %d", frontend.LastSeen, now.Unix()*1000)
	}
	if len(frontend.LoadAverage) != 3 {
		t.Errorf("LoadAverage length = %d, want 3", len(frontend.LoadAverage))
	}
}

func TestNodeToFrontend_EmptyDisplayName(t *testing.T) {
	node := Node{
		Name:        "pve1",
		DisplayName: "",
		LastSeen:    time.Now(),
	}

	frontend := node.ToFrontend()

	// Should fall back to Name
	if frontend.DisplayName != node.Name {
		t.Errorf("DisplayName = %q, want %q (should fallback to Name)", frontend.DisplayName, node.Name)
	}
}

func TestVMToFrontend(t *testing.T) {
	now := time.Now()
	lastBackup := now.Add(-24 * time.Hour)

	vm := VM{
		ID:        "vm-100",
		VMID:      100,
		Name:      "test-vm",
		Node:      "pve1",
		Instance:  "default",
		Status:    "running",
		Type:      "qemu",
		CPU:       0.15,
		CPUs:      4,
		Memory:    Memory{Total: 8000000000, Used: 4000000000},
		Disk:      Disk{Total: 100000000000, Used: 50000000000},
		NetworkIn: 1000000,
		NetworkOut: 500000,
		DiskRead:  100000,
		DiskWrite: 50000,
		Uptime:    3600,
		Tags:      []string{"production", "web"},
		LastSeen:  now,
		LastBackup: lastBackup,
		IPAddresses: []string{"192.168.1.50", "10.0.0.50"},
		OSName:    "Ubuntu",
		OSVersion: "22.04",
	}

	frontend := vm.ToFrontend()

	if frontend.ID != vm.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, vm.ID)
	}
	if frontend.VMID != vm.VMID {
		t.Errorf("VMID = %d, want %d", frontend.VMID, vm.VMID)
	}
	if frontend.Name != vm.Name {
		t.Errorf("Name = %q, want %q", frontend.Name, vm.Name)
	}
	if frontend.Status != vm.Status {
		t.Errorf("Status = %q, want %q", frontend.Status, vm.Status)
	}
	if frontend.CPU != vm.CPU {
		t.Errorf("CPU = %f, want %f", frontend.CPU, vm.CPU)
	}
	if frontend.Tags != "production,web" {
		t.Errorf("Tags = %q, want 'production,web'", frontend.Tags)
	}
	if frontend.LastBackup != lastBackup.Unix()*1000 {
		t.Errorf("LastBackup = %d, want %d", frontend.LastBackup, lastBackup.Unix()*1000)
	}
	if len(frontend.IPAddresses) != 2 {
		t.Errorf("IPAddresses length = %d, want 2", len(frontend.IPAddresses))
	}
	if frontend.OSName != "Ubuntu" {
		t.Errorf("OSName = %q, want Ubuntu", frontend.OSName)
	}
	if frontend.Memory == nil {
		t.Error("Memory should not be nil when Total > 0")
	}
}

func TestVMToFrontend_NegativeNetworkValues(t *testing.T) {
	vm := VM{
		NetworkIn:  -1,
		NetworkOut: -1,
		DiskRead:   -1,
		DiskWrite:  -1,
		LastSeen:   time.Now(),
	}

	frontend := vm.ToFrontend()

	if frontend.NetIn != 0 {
		t.Errorf("NetIn = %d, want 0 (should convert negative to 0)", frontend.NetIn)
	}
	if frontend.NetOut != 0 {
		t.Errorf("NetOut = %d, want 0 (should convert negative to 0)", frontend.NetOut)
	}
	if frontend.DiskRead != 0 {
		t.Errorf("DiskRead = %d, want 0 (should convert negative to 0)", frontend.DiskRead)
	}
	if frontend.DiskWrite != 0 {
		t.Errorf("DiskWrite = %d, want 0 (should convert negative to 0)", frontend.DiskWrite)
	}
}

func TestContainerToFrontend(t *testing.T) {
	now := time.Now()

	container := Container{
		ID:        "ct-101",
		VMID:      101,
		Name:      "test-ct",
		Node:      "pve1",
		Status:    "running",
		Type:      "lxc",
		CPU:       0.10,
		CPUs:      2,
		Memory:    Memory{Total: 4000000000, Used: 2000000000},
		Disk:      Disk{Total: 50000000000, Used: 25000000000},
		NetworkIn: 500000,
		NetworkOut: 250000,
		Uptime:    7200,
		Tags:      []string{"dev", "database"},
		LastSeen:  now,
		IPAddresses: []string{"192.168.1.51"},
	}

	frontend := container.ToFrontend()

	if frontend.ID != container.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, container.ID)
	}
	if frontend.VMID != container.VMID {
		t.Errorf("VMID = %d, want %d", frontend.VMID, container.VMID)
	}
	if frontend.Type != container.Type {
		t.Errorf("Type = %q, want %q", frontend.Type, container.Type)
	}
	if frontend.Tags != "dev,database" {
		t.Errorf("Tags = %q, want 'dev,database'", frontend.Tags)
	}
	if frontend.Memory == nil {
		t.Error("Memory should not be nil when Total > 0")
	}
}

func TestDockerContainerToFrontend(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(-1 * time.Hour)

	container := DockerContainer{
		ID:            "abc123def456",
		Name:          "nginx-web",
		Image:         "nginx:latest",
		State:         "running",
		Status:        "Up 1 hour",
		Health:        "healthy",
		CPUPercent:    15.5,
		MemoryUsage:   512000000,
		MemoryLimit:   1024000000,
		MemoryPercent: 50.0,
		UptimeSeconds: 3600,
		RestartCount:  0,
		CreatedAt:     now.Add(-2 * time.Hour),
		StartedAt:     &startedAt,
		Ports: []DockerContainerPort{
			{PrivatePort: 80, PublicPort: 8080, Protocol: "tcp", IP: "0.0.0.0"},
		},
		Networks: []DockerContainerNetworkLink{
			{Name: "bridge", IPv4: "172.17.0.2"},
		},
		BlockIO: &DockerContainerBlockIO{
			ReadBytes:  1000000,
			WriteBytes: 500000,
		},
		Mounts: []DockerContainerMount{
			{Type: "bind", Source: "/host/data", Destination: "/data", RW: true},
		},
	}

	frontend := container.ToFrontend()

	if frontend.ID != container.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, container.ID)
	}
	if frontend.Name != container.Name {
		t.Errorf("Name = %q, want %q", frontend.Name, container.Name)
	}
	if frontend.State != container.State {
		t.Errorf("State = %q, want %q", frontend.State, container.State)
	}
	if frontend.Health != container.Health {
		t.Errorf("Health = %q, want %q", frontend.Health, container.Health)
	}
	if frontend.CPUPercent != container.CPUPercent {
		t.Errorf("CPUPercent = %f, want %f", frontend.CPUPercent, container.CPUPercent)
	}
	if frontend.MemoryPercent != container.MemoryPercent {
		t.Errorf("MemoryPercent = %f, want %f", frontend.MemoryPercent, container.MemoryPercent)
	}
	if len(frontend.Ports) != 1 {
		t.Errorf("Ports length = %d, want 1", len(frontend.Ports))
	}
	if len(frontend.Networks) != 1 {
		t.Errorf("Networks length = %d, want 1", len(frontend.Networks))
	}
	if frontend.BlockIO == nil {
		t.Error("BlockIO should not be nil")
	}
	if len(frontend.Mounts) != 1 {
		t.Errorf("Mounts length = %d, want 1", len(frontend.Mounts))
	}
	if frontend.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
}

func TestDockerServiceToFrontend(t *testing.T) {
	now := time.Now()
	createdAt := now.Add(-24 * time.Hour)
	updatedAt := now.Add(-1 * time.Hour)

	service := DockerService{
		ID:             "svc123",
		Name:           "web",
		Stack:          "mystack",
		Image:          "nginx:latest",
		Mode:           "replicated",
		DesiredTasks:   3,
		RunningTasks:   3,
		CompletedTasks: 0,
		Labels:         map[string]string{"app": "web"},
		EndpointPorts: []DockerServicePort{
			{Name: "http", Protocol: "tcp", TargetPort: 80, PublishedPort: 8080, PublishMode: "ingress"},
		},
		CreatedAt: &createdAt,
		UpdatedAt: &updatedAt,
	}

	frontend := service.ToFrontend()

	if frontend.ID != service.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, service.ID)
	}
	if frontend.Name != service.Name {
		t.Errorf("Name = %q, want %q", frontend.Name, service.Name)
	}
	if frontend.Mode != service.Mode {
		t.Errorf("Mode = %q, want %q", frontend.Mode, service.Mode)
	}
	if frontend.DesiredTasks != service.DesiredTasks {
		t.Errorf("DesiredTasks = %d, want %d", frontend.DesiredTasks, service.DesiredTasks)
	}
	if len(frontend.EndpointPorts) != 1 {
		t.Errorf("EndpointPorts length = %d, want 1", len(frontend.EndpointPorts))
	}
	if frontend.CreatedAt == nil {
		t.Error("CreatedAt should not be nil")
	}
}

func TestDockerSwarmInfoToFrontend(t *testing.T) {
	swarm := DockerSwarmInfo{
		NodeID:           "node123",
		NodeRole:         "manager",
		LocalState:       "active",
		ControlAvailable: true,
		ClusterID:        "cluster456",
		ClusterName:      "my-swarm",
		Scope:            "swarm",
	}

	frontend := swarm.ToFrontend()

	if frontend.NodeID != swarm.NodeID {
		t.Errorf("NodeID = %q, want %q", frontend.NodeID, swarm.NodeID)
	}
	if frontend.NodeRole != swarm.NodeRole {
		t.Errorf("NodeRole = %q, want %q", frontend.NodeRole, swarm.NodeRole)
	}
	if frontend.ControlAvailable != swarm.ControlAvailable {
		t.Errorf("ControlAvailable = %v, want %v", frontend.ControlAvailable, swarm.ControlAvailable)
	}
}

func TestStorageToFrontend(t *testing.T) {
	storage := Storage{
		ID:        "storage-1",
		Name:      "local-lvm",
		Node:      "pve1",
		Instance:  "default",
		Nodes:     []string{"pve1", "pve2"},
		NodeIDs:   []string{"pve1", "pve2"},
		NodeCount: 2,
		Type:      "lvmthin",
		Status:    "available",
		Total:     1000000000000,
		Used:      500000000000,
		Free:      500000000000,
		Usage:     50.0,
		Content:   "images,rootdir",
		Shared:    false,
		Enabled:   true,
		Active:    true,
	}

	frontend := storage.ToFrontend()

	if frontend.ID != storage.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, storage.ID)
	}
	if frontend.Storage != storage.Name {
		t.Errorf("Storage = %q, want %q", frontend.Storage, storage.Name)
	}
	if frontend.Type != storage.Type {
		t.Errorf("Type = %q, want %q", frontend.Type, storage.Type)
	}
	if frontend.Total != storage.Total {
		t.Errorf("Total = %d, want %d", frontend.Total, storage.Total)
	}
	if frontend.Used != storage.Used {
		t.Errorf("Used = %d, want %d", frontend.Used, storage.Used)
	}
	if frontend.Free != storage.Free {
		t.Errorf("Free = %d, want %d", frontend.Free, storage.Free)
	}
	if frontend.Avail != storage.Free {
		t.Errorf("Avail = %d, want %d", frontend.Avail, storage.Free)
	}
}

func TestHostSensorSummaryToFrontend(t *testing.T) {
	tests := []struct {
		name     string
		input    HostSensorSummary
		nilCheck bool
	}{
		{
			name:     "empty sensors",
			input:    HostSensorSummary{},
			nilCheck: true,
		},
		{
			name: "with temperatures",
			input: HostSensorSummary{
				TemperatureCelsius: map[string]float64{"cpu": 45.0},
			},
			nilCheck: false,
		},
		{
			name: "with fan RPM",
			input: HostSensorSummary{
				FanRPM: map[string]float64{"cpu_fan": 1200.0},
			},
			nilCheck: false,
		},
		{
			name: "with additional",
			input: HostSensorSummary{
				Additional: map[string]float64{"voltage": 12.0},
			},
			nilCheck: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := hostSensorSummaryToFrontend(tc.input)
			if tc.nilCheck {
				if result != nil {
					t.Errorf("hostSensorSummaryToFrontend should return nil for empty sensors")
				}
				return
			}
			if result == nil {
				t.Error("hostSensorSummaryToFrontend should not return nil for non-empty sensors")
			}
		})
	}
}

func TestRemovedDockerHostToFrontend(t *testing.T) {
	now := time.Now()

	removed := RemovedDockerHost{
		ID:          "host-123",
		Hostname:    "docker-host",
		DisplayName: "Docker Host",
		RemovedAt:   now,
	}

	frontend := removed.ToFrontend()

	if frontend.ID != removed.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, removed.ID)
	}
	if frontend.Hostname != removed.Hostname {
		t.Errorf("Hostname = %q, want %q", frontend.Hostname, removed.Hostname)
	}
	if frontend.RemovedAt != now.Unix()*1000 {
		t.Errorf("RemovedAt = %d, want %d", frontend.RemovedAt, now.Unix()*1000)
	}
}

func TestVMToFrontend_EmptyTags(t *testing.T) {
	vm := VM{
		Tags:     []string{},
		LastSeen: time.Now(),
	}

	frontend := vm.ToFrontend()

	if frontend.Tags != "" {
		t.Errorf("Tags = %q, want empty string", frontend.Tags)
	}
}

func TestVMToFrontend_ZeroLastBackup(t *testing.T) {
	vm := VM{
		LastBackup: time.Time{}, // zero value
		LastSeen:   time.Now(),
	}

	frontend := vm.ToFrontend()

	if frontend.LastBackup != 0 {
		t.Errorf("LastBackup = %d, want 0 for zero time", frontend.LastBackup)
	}
}

func TestDockerServicePortToFrontend(t *testing.T) {
	port := DockerServicePort{
		Name:          "http",
		Protocol:      "tcp",
		TargetPort:    80,
		PublishedPort: 8080,
		PublishMode:   "ingress",
	}

	frontend := port.ToFrontend()

	if frontend.Name != port.Name {
		t.Errorf("Name = %q, want %q", frontend.Name, port.Name)
	}
	if frontend.Protocol != port.Protocol {
		t.Errorf("Protocol = %q, want %q", frontend.Protocol, port.Protocol)
	}
	if frontend.TargetPort != port.TargetPort {
		t.Errorf("TargetPort = %d, want %d", frontend.TargetPort, port.TargetPort)
	}
	if frontend.PublishedPort != port.PublishedPort {
		t.Errorf("PublishedPort = %d, want %d", frontend.PublishedPort, port.PublishedPort)
	}
	if frontend.PublishMode != port.PublishMode {
		t.Errorf("PublishMode = %q, want %q", frontend.PublishMode, port.PublishMode)
	}
}

func TestDockerTaskToFrontend(t *testing.T) {
	now := time.Now()
	updatedAt := now.Add(-5 * time.Minute)
	startedAt := now.Add(-10 * time.Minute)

	task := DockerTask{
		ID:           "task123",
		ServiceID:    "svc456",
		ServiceName:  "web",
		Slot:         1,
		NodeID:       "node789",
		NodeName:     "docker-node-1",
		DesiredState: "running",
		CurrentState: "running",
		ContainerID:  "container000",
		CreatedAt:    now.Add(-15 * time.Minute),
		UpdatedAt:    &updatedAt,
		StartedAt:    &startedAt,
	}

	frontend := task.ToFrontend()

	if frontend.ID != task.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, task.ID)
	}
	if frontend.ServiceName != task.ServiceName {
		t.Errorf("ServiceName = %q, want %q", frontend.ServiceName, task.ServiceName)
	}
	if frontend.Slot != task.Slot {
		t.Errorf("Slot = %d, want %d", frontend.Slot, task.Slot)
	}
	if frontend.DesiredState != task.DesiredState {
		t.Errorf("DesiredState = %q, want %q", frontend.DesiredState, task.DesiredState)
	}
	if frontend.CreatedAt == nil {
		t.Error("CreatedAt should not be nil")
	}
	if frontend.UpdatedAt == nil {
		t.Error("UpdatedAt should not be nil")
	}
	if frontend.StartedAt == nil {
		t.Error("StartedAt should not be nil")
	}
}

func TestVMToFrontend_TagsJoinedCorrectly(t *testing.T) {
	vm := VM{
		Tags:     []string{"tag1", "tag2", "tag3"},
		LastSeen: time.Now(),
	}

	frontend := vm.ToFrontend()

	// Tags should be comma-separated
	if !strings.Contains(frontend.Tags, ",") {
		t.Error("Tags should be comma-separated")
	}
	if frontend.Tags != "tag1,tag2,tag3" {
		t.Errorf("Tags = %q, want 'tag1,tag2,tag3'", frontend.Tags)
	}
}
