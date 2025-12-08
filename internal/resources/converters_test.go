package resources

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestFromNode(t *testing.T) {
	node := models.Node{
		ID:          "pve1/node/node1",
		Name:        "node1",
		DisplayName: "Production Node 1",
		Instance:    "pve1",
		Host:        "https://192.168.1.100:8006",
		Status:      "online",
		CPU:         0.25, // 25%
		Memory: models.Memory{
			Total: 16 * 1024 * 1024 * 1024, // 16GB
			Used:  8 * 1024 * 1024 * 1024,  // 8GB
			Free:  8 * 1024 * 1024 * 1024,
			Usage: 50.0,
		},
		Disk: models.Disk{
			Total: 500 * 1024 * 1024 * 1024, // 500GB
			Used:  200 * 1024 * 1024 * 1024, // 200GB
			Free:  300 * 1024 * 1024 * 1024,
			Usage: 40.0,
		},
		Uptime:          86400,
		KernelVersion:   "6.8.4-2-pve",
		PVEVersion:      "8.2.2",
		IsClusterMember: true,
		ClusterName:     "production",
		LastSeen:        time.Now(),
		LoadAverage:     []float64{1.5, 2.0, 1.8},
	}

	r := FromNode(node)

	// Check basic fields
	if r.ID != node.ID {
		t.Errorf("Expected ID %s, got %s", node.ID, r.ID)
	}
	if r.Type != ResourceTypeNode {
		t.Errorf("Expected type %s, got %s", ResourceTypeNode, r.Type)
	}
	if r.Name != node.Name {
		t.Errorf("Expected name %s, got %s", node.Name, r.Name)
	}
	if r.DisplayName != node.DisplayName {
		t.Errorf("Expected displayName %s, got %s", node.DisplayName, r.DisplayName)
	}
	if r.PlatformType != PlatformProxmoxPVE {
		t.Errorf("Expected platform %s, got %s", PlatformProxmoxPVE, r.PlatformType)
	}
	if r.SourceType != SourceAPI {
		t.Errorf("Expected source %s, got %s", SourceAPI, r.SourceType)
	}
	if r.Status != StatusOnline {
		t.Errorf("Expected status %s, got %s", StatusOnline, r.Status)
	}

	// Check metrics
	if r.CPU == nil {
		t.Fatal("CPU should not be nil")
	}
	if r.CPU.Current != 25.0 { // Converted from 0-1 to percentage
		t.Errorf("Expected CPU 25%%, got %f%%", r.CPU.Current)
	}

	if r.Memory == nil {
		t.Fatal("Memory should not be nil")
	}
	if r.Memory.Current != 50.0 {
		t.Errorf("Expected memory 50%%, got %f%%", r.Memory.Current)
	}

	if r.Disk == nil {
		t.Fatal("Disk should not be nil")
	}
	if r.Disk.Current != 40.0 {
		t.Errorf("Expected disk 40%%, got %f%%", r.Disk.Current)
	}

	if r.Uptime == nil || *r.Uptime != 86400 {
		t.Errorf("Expected uptime 86400, got %v", r.Uptime)
	}

	// Check cluster info
	if r.ClusterID != "pve-cluster/production" {
		t.Errorf("Expected clusterID pve-cluster/production, got %s", r.ClusterID)
	}

	// Check identity
	if r.Identity == nil {
		t.Fatal("Identity should not be nil")
	}
	if r.Identity.Hostname != "node1" {
		t.Errorf("Expected hostname node1, got %s", r.Identity.Hostname)
	}

	// Check platform data
	var pd NodePlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("Failed to get platform data: %v", err)
	}
	if pd.PVEVersion != "8.2.2" {
		t.Errorf("Expected PVE version 8.2.2, got %s", pd.PVEVersion)
	}
	if pd.KernelVersion != "6.8.4-2-pve" {
		t.Errorf("Expected kernel version 6.8.4-2-pve, got %s", pd.KernelVersion)
	}
	if len(pd.LoadAverage) != 3 {
		t.Errorf("Expected 3 load average values, got %d", len(pd.LoadAverage))
	}
}

func TestFromVM(t *testing.T) {
	vm := models.VM{
		ID:        "pve1/qemu/100",
		VMID:      100,
		Name:      "webserver",
		Node:      "node1",
		Instance:  "pve1",
		Status:    "running",
		Type:      "qemu",
		CPU:       0.15,
		CPUs:      4,
		Memory: models.Memory{
			Total: 8 * 1024 * 1024 * 1024,
			Used:  4 * 1024 * 1024 * 1024,
			Free:  4 * 1024 * 1024 * 1024,
			Usage: 50.0,
		},
		Disk: models.Disk{
			Total: 100 * 1024 * 1024 * 1024,
			Used:  30 * 1024 * 1024 * 1024,
			Free:  70 * 1024 * 1024 * 1024,
			Usage: 30.0,
		},
		NetworkIn:  1000000,
		NetworkOut: 500000,
		DiskRead:   2000000,
		DiskWrite:  1000000,
		Uptime:     3600,
		Tags:       []string{"production", "web"},
		LastSeen:   time.Now(),
	}

	r := FromVM(vm)

	if r.ID != vm.ID {
		t.Errorf("Expected ID %s, got %s", vm.ID, r.ID)
	}
	if r.Type != ResourceTypeVM {
		t.Errorf("Expected type %s, got %s", ResourceTypeVM, r.Type)
	}
	if r.Status != StatusRunning {
		t.Errorf("Expected status %s, got %s", StatusRunning, r.Status)
	}
	if r.ParentID != "pve1-node1" {
		t.Errorf("Expected parent pve1-node1, got %s", r.ParentID)
	}

	// Check CPU is converted to percentage
	if r.CPU.Current != 15.0 {
		t.Errorf("Expected CPU 15%%, got %f%%", r.CPU.Current)
	}

	// Check network
	if r.Network == nil {
		t.Fatal("Network should not be nil")
	}
	if r.Network.RXBytes != 1000000 {
		t.Errorf("Expected RXBytes 1000000, got %d", r.Network.RXBytes)
	}
	if r.Network.TXBytes != 500000 {
		t.Errorf("Expected TXBytes 500000, got %d", r.Network.TXBytes)
	}

	// Check tags
	if len(r.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(r.Tags))
	}

	// Check platform data
	var pd VMPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("Failed to get platform data: %v", err)
	}
	if pd.VMID != 100 {
		t.Errorf("Expected VMID 100, got %d", pd.VMID)
	}
	if pd.CPUs != 4 {
		t.Errorf("Expected 4 CPUs, got %d", pd.CPUs)
	}
}

func TestFromContainer(t *testing.T) {
	ct := models.Container{
		ID:       "pve1/lxc/101",
		VMID:     101,
		Name:     "database",
		Node:     "node1",
		Instance: "pve1",
		Status:   "stopped",
		Type:     "lxc",
		CPU:      0.0,
		CPUs:     2,
		Memory: models.Memory{
			Total: 4 * 1024 * 1024 * 1024,
			Used:  0,
			Free:  4 * 1024 * 1024 * 1024,
			Usage: 0.0,
		},
		Uptime:      0,
		Tags:        []string{"database"},
		IPAddresses: []string{"192.168.1.50"},
		LastSeen:    time.Now(),
	}

	r := FromContainer(ct)

	if r.Type != ResourceTypeContainer {
		t.Errorf("Expected type %s, got %s", ResourceTypeContainer, r.Type)
	}
	if r.Status != StatusStopped {
		t.Errorf("Expected status %s, got %s", StatusStopped, r.Status)
	}
	if r.ParentID != "pve1-node1" {
		t.Errorf("Expected parent pve1-node1, got %s", r.ParentID)
	}

	var pd ContainerPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("Failed to get platform data: %v", err)
	}
	if len(pd.IPAddresses) != 1 || pd.IPAddresses[0] != "192.168.1.50" {
		t.Errorf("Expected IP 192.168.1.50, got %v", pd.IPAddresses)
	}
}

func TestFromHost(t *testing.T) {
	host := models.Host{
		ID:            "host-abc123",
		Hostname:      "standalone-server",
		DisplayName:   "Standalone Server",
		Platform:      "linux",
		OSName:        "Ubuntu",
		OSVersion:     "22.04",
		KernelVersion: "5.15.0-generic",
		Architecture:  "amd64",
		CPUCount:      8,
		CPUUsage:      45.5,
		Memory: models.Memory{
			Total: 32 * 1024 * 1024 * 1024,
			Used:  16 * 1024 * 1024 * 1024,
			Free:  16 * 1024 * 1024 * 1024,
			Usage: 50.0,
		},
		LoadAverage: []float64{2.5, 2.0, 1.5},
		Disks: []models.Disk{
			{
				Mountpoint: "/",
				Total:      500 * 1024 * 1024 * 1024,
				Used:       200 * 1024 * 1024 * 1024,
				Free:       300 * 1024 * 1024 * 1024,
				Usage:      40.0,
			},
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{
				Name:      "eth0",
				MAC:       "aa:bb:cc:dd:ee:ff",
				Addresses: []string{"192.168.1.100", "fe80::1"},
				RXBytes:   10000000,
				TXBytes:   5000000,
			},
		},
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{
				"cpu_temp": 55.0,
			},
		},
		Status:        "online",
		UptimeSeconds: 86400 * 30, // 30 days
		AgentVersion:  "1.2.3",
		Tags:          []string{"production"},
		LastSeen:      time.Now(),
	}

	r := FromHost(host)

	if r.Type != ResourceTypeHost {
		t.Errorf("Expected type %s, got %s", ResourceTypeHost, r.Type)
	}
	if r.PlatformType != PlatformHostAgent {
		t.Errorf("Expected platform %s, got %s", PlatformHostAgent, r.PlatformType)
	}
	if r.SourceType != SourceAgent {
		t.Errorf("Expected source %s, got %s", SourceAgent, r.SourceType)
	}

	// CPU is already a percentage for hosts
	if r.CPU.Current != 45.5 {
		t.Errorf("Expected CPU 45.5%%, got %f%%", r.CPU.Current)
	}

	// Check temperature
	if r.Temperature == nil || *r.Temperature != 55.0 {
		t.Errorf("Expected temperature 55.0, got %v", r.Temperature)
	}

	// Check identity includes IP
	if r.Identity == nil {
		t.Fatal("Identity should not be nil")
	}
	if r.Identity.Hostname != "standalone-server" {
		t.Errorf("Expected hostname standalone-server, got %s", r.Identity.Hostname)
	}
	if len(r.Identity.IPs) < 1 {
		t.Error("Expected at least 1 IP in identity")
	}

	// Check platform data
	var pd HostPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("Failed to get platform data: %v", err)
	}
	if pd.CPUCount != 8 {
		t.Errorf("Expected 8 CPUs, got %d", pd.CPUCount)
	}
	if pd.AgentVersion != "1.2.3" {
		t.Errorf("Expected agent version 1.2.3, got %s", pd.AgentVersion)
	}
}

func TestFromDockerHost(t *testing.T) {
	dh := models.DockerHost{
		ID:           "docker-host-1",
		AgentID:      "agent-xyz",
		Hostname:     "docker-server",
		DisplayName:  "Docker Server",
		MachineID:    "machine-id-123",
		OS:           "linux",
		Architecture: "amd64",
		Runtime:      "docker",
		DockerVersion: "24.0.5",
		CPUs:         16,
		CPUUsage:     35.0,
		Memory: models.Memory{
			Total: 64 * 1024 * 1024 * 1024,
			Used:  32 * 1024 * 1024 * 1024,
			Free:  32 * 1024 * 1024 * 1024,
			Usage: 50.0,
		},
		UptimeSeconds: 86400,
		Status:        "online",
		LastSeen:      time.Now(),
		Swarm: &models.DockerSwarmInfo{
			NodeID:      "swarm-node-1",
			NodeRole:    "manager",
			ClusterID:   "swarm-cluster-1",
			ClusterName: "production-swarm",
		},
		NetworkInterfaces: []models.HostNetworkInterface{
			{
				Name:      "docker0",
				Addresses: []string{"172.17.0.1"},
			},
		},
	}

	r := FromDockerHost(dh)

	if r.Type != ResourceTypeDockerHost {
		t.Errorf("Expected type %s, got %s", ResourceTypeDockerHost, r.Type)
	}
	if r.PlatformType != PlatformDocker {
		t.Errorf("Expected platform %s, got %s", PlatformDocker, r.PlatformType)
	}

	// Check cluster ID from swarm
	if r.ClusterID != "docker-swarm/swarm-cluster-1" {
		t.Errorf("Expected clusterID docker-swarm/swarm-cluster-1, got %s", r.ClusterID)
	}

	// Check identity includes machine ID
	if r.Identity == nil {
		t.Fatal("Identity should not be nil")
	}
	if r.Identity.MachineID != "machine-id-123" {
		t.Errorf("Expected machineID machine-id-123, got %s", r.Identity.MachineID)
	}

	var pd DockerHostPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("Failed to get platform data: %v", err)
	}
	if pd.DockerVersion != "24.0.5" {
		t.Errorf("Expected Docker version 24.0.5, got %s", pd.DockerVersion)
	}
	if pd.Swarm == nil || pd.Swarm.NodeRole != "manager" {
		t.Error("Expected swarm info with manager role")
	}
}

func TestFromDockerContainer(t *testing.T) {
	dc := models.DockerContainer{
		ID:            "container-123",
		Name:          "nginx",
		Image:         "nginx:latest",
		State:         "running",
		Status:        "Up 2 hours",
		Health:        "healthy",
		CPUPercent:    5.5,
		MemoryUsage:   256 * 1024 * 1024, // 256MB
		MemoryLimit:   512 * 1024 * 1024, // 512MB
		MemoryPercent: 50.0,
		UptimeSeconds: 7200,
		RestartCount:  0,
		CreatedAt:     time.Now().Add(-24 * time.Hour),
		Labels: map[string]string{
			"app": "web",
		},
		Ports: []models.DockerContainerPort{
			{PrivatePort: 80, PublicPort: 8080, Protocol: "tcp"},
		},
	}

	r := FromDockerContainer(dc, "docker-host-1", "docker-server")

	if r.Type != ResourceTypeDockerContainer {
		t.Errorf("Expected type %s, got %s", ResourceTypeDockerContainer, r.Type)
	}
	if r.ParentID != "docker-host-1" {
		t.Errorf("Expected parent docker-host-1, got %s", r.ParentID)
	}
	if r.Status != StatusRunning {
		t.Errorf("Expected status %s, got %s", StatusRunning, r.Status)
	}

	if r.CPU.Current != 5.5 {
		t.Errorf("Expected CPU 5.5%%, got %f%%", r.CPU.Current)
	}
	if r.Memory == nil || r.Memory.Current != 50.0 {
		t.Errorf("Expected memory 50%%, got %v", r.Memory)
	}

	if r.Labels["app"] != "web" {
		t.Error("Expected label app=web")
	}

	var pd DockerContainerPlatformData
	if err := r.GetPlatformData(&pd); err != nil {
		t.Fatalf("Failed to get platform data: %v", err)
	}
	if pd.Image != "nginx:latest" {
		t.Errorf("Expected image nginx:latest, got %s", pd.Image)
	}
	if pd.Health != "healthy" {
		t.Errorf("Expected health healthy, got %s", pd.Health)
	}
	if len(pd.Ports) != 1 || pd.Ports[0].PublicPort != 8080 {
		t.Error("Expected port 8080 mapping")
	}
}

func TestResourceMethods(t *testing.T) {
	// Test IsInfrastructure
	nodeResource := Resource{Type: ResourceTypeNode}
	if !nodeResource.IsInfrastructure() {
		t.Error("Node should be infrastructure")
	}
	if nodeResource.IsWorkload() {
		t.Error("Node should not be workload")
	}

	vmResource := Resource{Type: ResourceTypeVM}
	if vmResource.IsInfrastructure() {
		t.Error("VM should not be infrastructure")
	}
	if !vmResource.IsWorkload() {
		t.Error("VM should be workload")
	}

	// Test EffectiveDisplayName
	r := Resource{Name: "name", DisplayName: ""}
	if r.EffectiveDisplayName() != "name" {
		t.Error("Should return Name when DisplayName is empty")
	}
	r.DisplayName = "custom"
	if r.EffectiveDisplayName() != "custom" {
		t.Error("Should return DisplayName when set")
	}

	// Test CPUPercent
	r = Resource{}
	if r.CPUPercent() != 0 {
		t.Error("CPUPercent should be 0 when CPU is nil")
	}
	r.CPU = &MetricValue{Current: 75.5}
	if r.CPUPercent() != 75.5 {
		t.Errorf("Expected 75.5, got %f", r.CPUPercent())
	}

	// Test MemoryPercent
	r = Resource{}
	if r.MemoryPercent() != 0 {
		t.Error("MemoryPercent should be 0 when Memory is nil")
	}
	r.Memory = &MetricValue{Current: 60.0}
	if r.MemoryPercent() != 60.0 {
		t.Errorf("Expected 60.0, got %f", r.MemoryPercent())
	}
}

func TestStatusMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected ResourceStatus
		mapper   func(string) ResourceStatus
	}{
		{"online", StatusOnline, mapNodeStatus},
		{"offline", StatusOffline, mapNodeStatus},
		{"unknown", StatusUnknown, mapNodeStatus},
		{"running", StatusRunning, mapGuestStatus},
		{"stopped", StatusStopped, mapGuestStatus},
		{"paused", StatusPaused, mapGuestStatus},
		{"online", StatusOnline, mapHostStatus},
		{"degraded", StatusDegraded, mapHostStatus},
		{"running", StatusRunning, mapDockerContainerStatus},
		{"exited", StatusStopped, mapDockerContainerStatus},
		{"dead", StatusStopped, mapDockerContainerStatus},
		{"paused", StatusPaused, mapDockerContainerStatus},
	}

	for _, test := range tests {
		result := test.mapper(test.input)
		if result != test.expected {
			t.Errorf("Mapping %s: expected %s, got %s", test.input, test.expected, result)
		}
	}
}
