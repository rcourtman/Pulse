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
			if len(tc.input) > 0 {
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
		ID:          "vm-100",
		VMID:        100,
		Name:        "test-vm",
		Node:        "pve1",
		Instance:    "default",
		Status:      "running",
		Type:        "qemu",
		CPU:         0.15,
		CPUs:        4,
		Memory:      Memory{Total: 8000000000, Used: 4000000000},
		Disk:        Disk{Total: 100000000000, Used: 50000000000},
		NetworkIn:   1000000,
		NetworkOut:  500000,
		DiskRead:    100000,
		DiskWrite:   50000,
		Uptime:      3600,
		Tags:        []string{"production", "web"},
		LastSeen:    now,
		LastBackup:  lastBackup,
		IPAddresses: []string{"192.168.1.50", "10.0.0.50"},
		OSName:      "Ubuntu",
		OSVersion:   "22.04",
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

	t.Run("lxc", func(t *testing.T) {
		container := Container{
			ID:          "ct-101",
			VMID:        101,
			Name:        "test-ct",
			Node:        "pve1",
			Status:      "running",
			Type:        "lxc",
			CPU:         0.10,
			CPUs:        2,
			Memory:      Memory{Total: 4000000000, Used: 2000000000},
			Disk:        Disk{Total: 50000000000, Used: 25000000000},
			NetworkIn:   500000,
			NetworkOut:  250000,
			Uptime:      7200,
			Tags:        []string{"dev", "database"},
			LastSeen:    now,
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
		if frontend.IsOCI {
			t.Errorf("IsOCI = %v, want false", frontend.IsOCI)
		}
		if frontend.Tags != "dev,database" {
			t.Errorf("Tags = %q, want 'dev,database'", frontend.Tags)
		}
		if frontend.Memory == nil {
			t.Error("Memory should not be nil when Total > 0")
		}
	})

	t.Run("oci", func(t *testing.T) {
		container := Container{
			ID:         "ct-300",
			VMID:       300,
			Name:       "oci-alpine",
			Node:       "pve1",
			Status:     "running",
			Type:       "lxc",
			IsOCI:      true,
			OSTemplate: "oci:docker.io/library/alpine:latest",
			CPU:        0.05,
			CPUs:       2,
			Memory:     Memory{Total: 1000000000, Used: 100000000},
			Disk:       Disk{Total: 10000000000, Used: 1000000000},
			LastSeen:   now,
		}

		frontend := container.ToFrontend()

		if frontend.Type != "oci" {
			t.Errorf("Type = %q, want %q", frontend.Type, "oci")
		}
		if !frontend.IsOCI {
			t.Errorf("IsOCI = %v, want true", frontend.IsOCI)
		}
		if frontend.OSTemplate != container.OSTemplate {
			t.Errorf("OSTemplate = %q, want %q", frontend.OSTemplate, container.OSTemplate)
		}
	})

	t.Run("oci-derived-from-type", func(t *testing.T) {
		container := Container{
			ID:       "ct-301",
			VMID:     301,
			Name:     "oci-from-type",
			Node:     "pve1",
			Status:   "running",
			Type:     "oci",
			IsOCI:    false,
			LastSeen: now,
		}

		frontend := container.ToFrontend()

		if frontend.Type != "oci" {
			t.Errorf("Type = %q, want %q", frontend.Type, "oci")
		}
		if !frontend.IsOCI {
			t.Errorf("IsOCI = %v, want true", frontend.IsOCI)
		}
	})
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

func TestToDockerHostCommandFrontend(t *testing.T) {
	now := time.Now()
	dispatched := now.Add(-time.Minute)
	acknowledged := now.Add(-30 * time.Second)
	completed := now.Add(-10 * time.Second)
	expires := now.Add(time.Hour)

	t.Run("basic fields", func(t *testing.T) {
		cmd := DockerHostCommandStatus{
			ID:        "cmd-123",
			Type:      "update",
			Status:    "pending",
			Message:   "Update requested",
			CreatedAt: now,
			UpdatedAt: now,
		}

		result := toDockerHostCommandFrontend(cmd)

		if result.ID != "cmd-123" {
			t.Errorf("ID = %q, want %q", result.ID, "cmd-123")
		}
		if result.Type != "update" {
			t.Errorf("Type = %q, want %q", result.Type, "update")
		}
		if result.Status != "pending" {
			t.Errorf("Status = %q, want %q", result.Status, "pending")
		}
		if result.Message != "Update requested" {
			t.Errorf("Message = %q, want %q", result.Message, "Update requested")
		}
		if result.CreatedAt != now.Unix()*1000 {
			t.Errorf("CreatedAt = %d, want %d", result.CreatedAt, now.Unix()*1000)
		}
		if result.UpdatedAt != now.Unix()*1000 {
			t.Errorf("UpdatedAt = %d, want %d", result.UpdatedAt, now.Unix()*1000)
		}
	})

	t.Run("optional timestamps nil", func(t *testing.T) {
		cmd := DockerHostCommandStatus{
			ID:        "cmd-456",
			Type:      "restart",
			Status:    "pending",
			CreatedAt: now,
			UpdatedAt: now,
		}

		result := toDockerHostCommandFrontend(cmd)

		if result.DispatchedAt != nil {
			t.Errorf("DispatchedAt = %v, want nil", result.DispatchedAt)
		}
		if result.AcknowledgedAt != nil {
			t.Errorf("AcknowledgedAt = %v, want nil", result.AcknowledgedAt)
		}
		if result.CompletedAt != nil {
			t.Errorf("CompletedAt = %v, want nil", result.CompletedAt)
		}
		if result.FailedAt != nil {
			t.Errorf("FailedAt = %v, want nil", result.FailedAt)
		}
		if result.ExpiresAt != nil {
			t.Errorf("ExpiresAt = %v, want nil", result.ExpiresAt)
		}
	})

	t.Run("all optional timestamps set", func(t *testing.T) {
		failed := now.Add(-5 * time.Second)

		cmd := DockerHostCommandStatus{
			ID:             "cmd-789",
			Type:           "update",
			Status:         "completed",
			CreatedAt:      now,
			UpdatedAt:      now,
			DispatchedAt:   &dispatched,
			AcknowledgedAt: &acknowledged,
			CompletedAt:    &completed,
			FailedAt:       &failed,
			ExpiresAt:      &expires,
		}

		result := toDockerHostCommandFrontend(cmd)

		if result.DispatchedAt == nil || *result.DispatchedAt != dispatched.Unix()*1000 {
			t.Errorf("DispatchedAt = %v, want %d", result.DispatchedAt, dispatched.Unix()*1000)
		}
		if result.AcknowledgedAt == nil || *result.AcknowledgedAt != acknowledged.Unix()*1000 {
			t.Errorf("AcknowledgedAt = %v, want %d", result.AcknowledgedAt, acknowledged.Unix()*1000)
		}
		if result.CompletedAt == nil || *result.CompletedAt != completed.Unix()*1000 {
			t.Errorf("CompletedAt = %v, want %d", result.CompletedAt, completed.Unix()*1000)
		}
		if result.FailedAt == nil || *result.FailedAt != failed.Unix()*1000 {
			t.Errorf("FailedAt = %v, want %d", result.FailedAt, failed.Unix()*1000)
		}
		if result.ExpiresAt == nil || *result.ExpiresAt != expires.Unix()*1000 {
			t.Errorf("ExpiresAt = %v, want %d", result.ExpiresAt, expires.Unix()*1000)
		}
	})

	t.Run("failure reason set", func(t *testing.T) {
		cmd := DockerHostCommandStatus{
			ID:            "cmd-fail",
			Type:          "update",
			Status:        "failed",
			FailureReason: "Connection timeout",
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		result := toDockerHostCommandFrontend(cmd)

		if result.FailureReason != "Connection timeout" {
			t.Errorf("FailureReason = %q, want %q", result.FailureReason, "Connection timeout")
		}
	})

	t.Run("empty failure reason", func(t *testing.T) {
		cmd := DockerHostCommandStatus{
			ID:            "cmd-ok",
			Type:          "update",
			Status:        "completed",
			FailureReason: "",
			CreatedAt:     now,
			UpdatedAt:     now,
		}

		result := toDockerHostCommandFrontend(cmd)

		if result.FailureReason != "" {
			t.Errorf("FailureReason = %q, want empty", result.FailureReason)
		}
	})
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

func TestDockerHostToFrontend(t *testing.T) {
	now := time.Now()
	tokenLastUsed := now.Add(-1 * time.Hour)

	host := DockerHost{
		ID:                "dh-123",
		AgentID:           "agent-456",
		Hostname:          "docker-server",
		DisplayName:       "Docker Server",
		CustomDisplayName: "My Docker Server",
		MachineID:         "machine-789",
		OS:                "linux",
		KernelVersion:     "5.15.0",
		Architecture:      "amd64",
		Runtime:           "docker",
		RuntimeVersion:    "24.0.0",
		DockerVersion:     "24.0.0",
		CPUs:              8,
		TotalMemoryBytes:  16000000000,
		UptimeSeconds:     86400,
		CPUUsage:          25.5,
		Status:            "online",
		LastSeen:          now,
		IntervalSeconds:   30,
		AgentVersion:      "1.0.0",
		TokenID:           "token-123",
		TokenName:         "my-token",
		TokenHint:         "abc***xyz",
		TokenLastUsedAt:   &tokenLastUsed,
		LoadAverage:       []float64{1.0, 0.8, 0.5},
		Memory:            Memory{Total: 16000000000, Used: 8000000000},
		Disks:             []Disk{{Total: 500000000000, Used: 250000000000}},
		NetworkInterfaces: []HostNetworkInterface{{Name: "eth0", Addresses: []string{"192.168.1.100"}}},
		Containers: []DockerContainer{
			{ID: "ct-1", Name: "nginx", State: "running"},
		},
	}

	frontend := host.ToFrontend()

	if frontend.ID != host.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, host.ID)
	}
	if frontend.AgentID != host.AgentID {
		t.Errorf("AgentID = %q, want %q", frontend.AgentID, host.AgentID)
	}
	if frontend.Hostname != host.Hostname {
		t.Errorf("Hostname = %q, want %q", frontend.Hostname, host.Hostname)
	}
	if frontend.DisplayName != host.DisplayName {
		t.Errorf("DisplayName = %q, want %q", frontend.DisplayName, host.DisplayName)
	}
	if frontend.CPUUsagePercent != host.CPUUsage {
		t.Errorf("CPUUsagePercent = %f, want %f", frontend.CPUUsagePercent, host.CPUUsage)
	}
	if frontend.LastSeen != now.Unix()*1000 {
		t.Errorf("LastSeen = %d, want %d", frontend.LastSeen, now.Unix()*1000)
	}
	if frontend.TokenID != host.TokenID {
		t.Errorf("TokenID = %q, want %q", frontend.TokenID, host.TokenID)
	}
	if frontend.TokenLastUsedAt == nil || *frontend.TokenLastUsedAt != tokenLastUsed.Unix()*1000 {
		t.Errorf("TokenLastUsedAt = %v, want %d", frontend.TokenLastUsedAt, tokenLastUsed.Unix()*1000)
	}
	if len(frontend.LoadAverage) != 3 {
		t.Errorf("LoadAverage length = %d, want 3", len(frontend.LoadAverage))
	}
	if frontend.Memory == nil {
		t.Error("Memory should not be nil")
	}
	if len(frontend.Disks) != 1 {
		t.Errorf("Disks length = %d, want 1", len(frontend.Disks))
	}
	if len(frontend.NetworkInterfaces) != 1 {
		t.Errorf("NetworkInterfaces length = %d, want 1", len(frontend.NetworkInterfaces))
	}
	if len(frontend.Containers) != 1 {
		t.Errorf("Containers length = %d, want 1", len(frontend.Containers))
	}
}

func TestDockerHostToFrontend_EmptyDisplayName(t *testing.T) {
	host := DockerHost{
		Hostname:    "docker-host",
		DisplayName: "",
		LastSeen:    time.Now(),
	}

	frontend := host.ToFrontend()

	// Should fall back to Hostname
	if frontend.DisplayName != host.Hostname {
		t.Errorf("DisplayName = %q, want %q (should fallback to Hostname)", frontend.DisplayName, host.Hostname)
	}
}

func TestDockerHostToFrontend_WithSwarm(t *testing.T) {
	host := DockerHost{
		LastSeen: time.Now(),
		Swarm: &DockerSwarmInfo{
			NodeID:   "node-123",
			NodeRole: "manager",
		},
	}

	frontend := host.ToFrontend()

	if frontend.Swarm == nil {
		t.Error("Swarm should not be nil")
	}
	if frontend.Swarm.NodeID != "node-123" {
		t.Errorf("Swarm.NodeID = %q, want %q", frontend.Swarm.NodeID, "node-123")
	}
}

func TestDockerHostToFrontend_WithServices(t *testing.T) {
	host := DockerHost{
		LastSeen: time.Now(),
		Services: []DockerService{
			{ID: "svc-1", Name: "web"},
			{ID: "svc-2", Name: "api"},
		},
	}

	frontend := host.ToFrontend()

	if len(frontend.Services) != 2 {
		t.Errorf("Services length = %d, want 2", len(frontend.Services))
	}
}

func TestDockerHostToFrontend_WithTasks(t *testing.T) {
	host := DockerHost{
		LastSeen: time.Now(),
		Tasks: []DockerTask{
			{ID: "task-1", ServiceName: "web"},
		},
	}

	frontend := host.ToFrontend()

	if len(frontend.Tasks) != 1 {
		t.Errorf("Tasks length = %d, want 1", len(frontend.Tasks))
	}
}

func TestDockerHostToFrontend_WithCommand(t *testing.T) {
	now := time.Now()
	host := DockerHost{
		LastSeen: now,
		Command: &DockerHostCommandStatus{
			ID:        "cmd-123",
			Type:      "update",
			Status:    "pending",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	frontend := host.ToFrontend()

	if frontend.Command == nil {
		t.Error("Command should not be nil")
	}
	if frontend.Command.ID != "cmd-123" {
		t.Errorf("Command.ID = %q, want %q", frontend.Command.ID, "cmd-123")
	}
}

func TestHostToFrontend(t *testing.T) {
	now := time.Now()
	tokenLastUsed := now.Add(-2 * time.Hour)

	host := Host{
		ID:              "host-123",
		Hostname:        "server1",
		DisplayName:     "Server 1",
		Platform:        "linux",
		OSName:          "Ubuntu",
		OSVersion:       "22.04",
		KernelVersion:   "5.15.0-generic",
		Architecture:    "amd64",
		CPUCount:        16,
		CPUUsage:        35.0,
		Status:          "online",
		UptimeSeconds:   172800,
		IntervalSeconds: 60,
		AgentVersion:    "1.2.0",
		TokenID:         "tok-456",
		TokenName:       "host-token",
		TokenHint:       "xyz***",
		TokenLastUsedAt: &tokenLastUsed,
		Tags:            []string{"production", "web"},
		LastSeen:        now,
		LoadAverage:     []float64{2.0, 1.5, 1.0},
		Memory:          Memory{Total: 32000000000, Used: 16000000000},
		Disks:           []Disk{{Total: 1000000000000, Used: 500000000000}},
		NetworkInterfaces: []HostNetworkInterface{
			{Name: "eth0", Addresses: []string{"10.0.0.50"}},
		},
		Sensors: HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu": 55.0},
		},
	}

	frontend := host.ToFrontend()

	if frontend.ID != host.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, host.ID)
	}
	if frontend.Hostname != host.Hostname {
		t.Errorf("Hostname = %q, want %q", frontend.Hostname, host.Hostname)
	}
	if frontend.DisplayName != host.DisplayName {
		t.Errorf("DisplayName = %q, want %q", frontend.DisplayName, host.DisplayName)
	}
	if frontend.Platform != host.Platform {
		t.Errorf("Platform = %q, want %q", frontend.Platform, host.Platform)
	}
	if frontend.CPUCount != host.CPUCount {
		t.Errorf("CPUCount = %d, want %d", frontend.CPUCount, host.CPUCount)
	}
	if frontend.CPUUsage != host.CPUUsage {
		t.Errorf("CPUUsage = %f, want %f", frontend.CPUUsage, host.CPUUsage)
	}
	if frontend.LastSeen != now.Unix()*1000 {
		t.Errorf("LastSeen = %d, want %d", frontend.LastSeen, now.Unix()*1000)
	}
	if len(frontend.Tags) != 2 {
		t.Errorf("Tags length = %d, want 2", len(frontend.Tags))
	}
	if len(frontend.LoadAverage) != 3 {
		t.Errorf("LoadAverage length = %d, want 3", len(frontend.LoadAverage))
	}
	if frontend.Memory == nil {
		t.Error("Memory should not be nil")
	}
	if len(frontend.Disks) != 1 {
		t.Errorf("Disks length = %d, want 1", len(frontend.Disks))
	}
	if len(frontend.NetworkInterfaces) != 1 {
		t.Errorf("NetworkInterfaces length = %d, want 1", len(frontend.NetworkInterfaces))
	}
	if frontend.Sensors == nil {
		t.Error("Sensors should not be nil")
	}
	if frontend.TokenLastUsedAt == nil || *frontend.TokenLastUsedAt != tokenLastUsed.Unix()*1000 {
		t.Errorf("TokenLastUsedAt = %v, want %d", frontend.TokenLastUsedAt, tokenLastUsed.Unix()*1000)
	}
}

func TestHostToFrontend_DisplayNameFallback(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		hostname    string
		expected    string
	}{
		{
			name:        "both empty",
			displayName: "",
			hostname:    "",
			expected:    "",
		},
		{
			name:        "displayName set",
			displayName: "My Server",
			hostname:    "server1",
			expected:    "My Server",
		},
		{
			name:        "only hostname",
			displayName: "",
			hostname:    "server1",
			expected:    "server1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			host := Host{
				DisplayName: tc.displayName,
				Hostname:    tc.hostname,
				LastSeen:    time.Now(),
			}

			frontend := host.ToFrontend()

			if frontend.DisplayName != tc.expected {
				t.Errorf("DisplayName = %q, want %q", frontend.DisplayName, tc.expected)
			}
		})
	}
}

func TestCephClusterToFrontend(t *testing.T) {
	now := time.Now()

	cluster := CephCluster{
		ID:             "ceph-123",
		Instance:       "default",
		Name:           "ceph-cluster",
		FSID:           "abc-def-123",
		Health:         "HEALTH_OK",
		HealthMessage:  "Cluster is healthy",
		TotalBytes:     10000000000000,
		UsedBytes:      5000000000000,
		AvailableBytes: 5000000000000,
		UsagePercent:   50.0,
		NumMons:        3,
		NumMgrs:        2,
		NumOSDs:        12,
		NumOSDsUp:      12,
		NumOSDsIn:      12,
		NumPGs:         256,
		LastUpdated:    now,
		Pools: []CephPool{
			{Name: "rbd", StoredBytes: 500000000000, AvailableBytes: 500000000000},
		},
		Services: []CephServiceStatus{
			{Type: "mon", Running: 3, Total: 3},
		},
	}

	frontend := cluster.ToFrontend()

	if frontend.ID != cluster.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, cluster.ID)
	}
	if frontend.Name != cluster.Name {
		t.Errorf("Name = %q, want %q", frontend.Name, cluster.Name)
	}
	if frontend.FSID != cluster.FSID {
		t.Errorf("FSID = %q, want %q", frontend.FSID, cluster.FSID)
	}
	if frontend.Health != cluster.Health {
		t.Errorf("Health = %q, want %q", frontend.Health, cluster.Health)
	}
	if frontend.TotalBytes != cluster.TotalBytes {
		t.Errorf("TotalBytes = %d, want %d", frontend.TotalBytes, cluster.TotalBytes)
	}
	if frontend.UsagePercent != cluster.UsagePercent {
		t.Errorf("UsagePercent = %f, want %f", frontend.UsagePercent, cluster.UsagePercent)
	}
	if frontend.NumOSDs != cluster.NumOSDs {
		t.Errorf("NumOSDs = %d, want %d", frontend.NumOSDs, cluster.NumOSDs)
	}
	if frontend.LastUpdated != now.Unix()*1000 {
		t.Errorf("LastUpdated = %d, want %d", frontend.LastUpdated, now.Unix()*1000)
	}
	if len(frontend.Pools) != 1 {
		t.Errorf("Pools length = %d, want 1", len(frontend.Pools))
	}
	if len(frontend.Services) != 1 {
		t.Errorf("Services length = %d, want 1", len(frontend.Services))
	}
}

func TestCephClusterToFrontend_EmptyPoolsAndServices(t *testing.T) {
	cluster := CephCluster{
		ID:          "ceph-empty",
		LastUpdated: time.Now(),
	}

	frontend := cluster.ToFrontend()

	if frontend.Pools != nil {
		t.Errorf("Pools = %v, want nil for empty pools", frontend.Pools)
	}
	if frontend.Services != nil {
		t.Errorf("Services = %v, want nil for empty services", frontend.Services)
	}
}

func TestReplicationJobToFrontend(t *testing.T) {
	now := time.Now()
	lastSync := now.Add(-1 * time.Hour)
	nextSync := now.Add(1 * time.Hour)

	job := ReplicationJob{
		ID:                      "rep-123",
		Instance:                "default",
		JobID:                   "job-456",
		JobNumber:               1,
		Guest:                   "vm/100",
		GuestID:                 100,
		GuestName:               "test-vm",
		GuestType:               "qemu",
		GuestNode:               "pve1",
		SourceNode:              "pve1",
		SourceStorage:           "local-lvm",
		TargetNode:              "pve2",
		TargetStorage:           "local-lvm",
		Schedule:                "*/15 * * * *",
		Type:                    "local",
		Enabled:                 true,
		State:                   "active",
		Status:                  "OK",
		LastSyncStatus:          "success",
		LastSyncUnix:            lastSync.Unix(),
		LastSyncDurationSeconds: 120,
		LastSyncDurationHuman:   "2m",
		NextSyncUnix:            nextSync.Unix(),
		DurationSeconds:         120,
		DurationHuman:           "2m",
		FailCount:               0,
		Comment:                 "Daily replication",
		LastSyncTime:            &lastSync,
		NextSyncTime:            &nextSync,
		LastPolled:              now,
	}

	frontend := job.ToFrontend()

	if frontend.ID != job.ID {
		t.Errorf("ID = %q, want %q", frontend.ID, job.ID)
	}
	if frontend.JobID != job.JobID {
		t.Errorf("JobID = %q, want %q", frontend.JobID, job.JobID)
	}
	if frontend.GuestID != job.GuestID {
		t.Errorf("GuestID = %d, want %d", frontend.GuestID, job.GuestID)
	}
	if frontend.GuestName != job.GuestName {
		t.Errorf("GuestName = %q, want %q", frontend.GuestName, job.GuestName)
	}
	if frontend.SourceNode != job.SourceNode {
		t.Errorf("SourceNode = %q, want %q", frontend.SourceNode, job.SourceNode)
	}
	if frontend.TargetNode != job.TargetNode {
		t.Errorf("TargetNode = %q, want %q", frontend.TargetNode, job.TargetNode)
	}
	if frontend.Enabled != job.Enabled {
		t.Errorf("Enabled = %v, want %v", frontend.Enabled, job.Enabled)
	}
	if frontend.LastSyncStatus != job.LastSyncStatus {
		t.Errorf("LastSyncStatus = %q, want %q", frontend.LastSyncStatus, job.LastSyncStatus)
	}
	if frontend.LastSyncTime != lastSync.UnixMilli() {
		t.Errorf("LastSyncTime = %d, want %d", frontend.LastSyncTime, lastSync.UnixMilli())
	}
	if frontend.NextSyncTime != nextSync.UnixMilli() {
		t.Errorf("NextSyncTime = %d, want %d", frontend.NextSyncTime, nextSync.UnixMilli())
	}
	if frontend.PolledAt != now.UnixMilli() {
		t.Errorf("PolledAt = %d, want %d", frontend.PolledAt, now.UnixMilli())
	}
}

func TestReplicationJobToFrontend_NilTimes(t *testing.T) {
	job := ReplicationJob{
		ID:           "rep-nil",
		LastSyncTime: nil,
		NextSyncTime: nil,
		LastPolled:   time.Time{}, // zero value
	}

	frontend := job.ToFrontend()

	if frontend.LastSyncTime != 0 {
		t.Errorf("LastSyncTime = %d, want 0 for nil", frontend.LastSyncTime)
	}
	if frontend.NextSyncTime != 0 {
		t.Errorf("NextSyncTime = %d, want 0 for nil", frontend.NextSyncTime)
	}
	// PolledAt defaults to now if LastPolled is zero
	if frontend.PolledAt == 0 {
		t.Error("PolledAt should not be 0 (defaults to now)")
	}
}

func TestDockerServiceUpdateToFrontend(t *testing.T) {
	completedAt := time.Now().Add(-5 * time.Minute)

	update := DockerServiceUpdate{
		State:       "completed",
		Message:     "Update completed successfully",
		CompletedAt: &completedAt,
	}

	frontend := update.ToFrontend()

	if frontend.State != update.State {
		t.Errorf("State = %q, want %q", frontend.State, update.State)
	}
	if frontend.Message != update.Message {
		t.Errorf("Message = %q, want %q", frontend.Message, update.Message)
	}
	if frontend.CompletedAt == nil || *frontend.CompletedAt != completedAt.Unix()*1000 {
		t.Errorf("CompletedAt = %v, want %d", frontend.CompletedAt, completedAt.Unix()*1000)
	}
}

func TestDockerServiceUpdateToFrontend_NilCompletedAt(t *testing.T) {
	update := DockerServiceUpdate{
		State:       "updating",
		CompletedAt: nil,
	}

	frontend := update.ToFrontend()

	if frontend.CompletedAt != nil {
		t.Errorf("CompletedAt = %v, want nil", frontend.CompletedAt)
	}
}

func TestDockerContainerToFrontend_PodmanInfo(t *testing.T) {
	container := DockerContainer{
		ID:        "podman-ct",
		Name:      "nginx",
		CreatedAt: time.Now(),
		Podman: &DockerPodmanContainer{
			PodName:           "web-pod",
			PodID:             "pod-123",
			Infra:             false,
			ComposeProject:    "myapp",
			ComposeService:    "web",
			ComposeWorkdir:    "/home/user/myapp",
			ComposeConfigHash: "abc123",
			AutoUpdatePolicy:  "registry",
			AutoUpdateRestart: "always",
			UserNamespace:     "keep-id:uid=1000,gid=1000",
		},
	}

	frontend := container.ToFrontend()

	if frontend.Podman == nil {
		t.Fatal("Podman should not be nil")
	}
	if frontend.Podman.PodName != "web-pod" {
		t.Errorf("Podman.PodName = %q, want %q", frontend.Podman.PodName, "web-pod")
	}
	if frontend.Podman.ComposeProject != "myapp" {
		t.Errorf("Podman.ComposeProject = %q, want %q", frontend.Podman.ComposeProject, "myapp")
	}
	if frontend.Podman.AutoUpdatePolicy != "registry" {
		t.Errorf("Podman.AutoUpdatePolicy = %q, want %q", frontend.Podman.AutoUpdatePolicy, "registry")
	}
}
