package unifiedresources

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mockruntime"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/storagehealth"
)

func TestResourceFromProxmoxNodeIncludesTemperature(t *testing.T) {
	node := models.Node{
		ID:     "mock-cluster-pve1",
		Name:   "pve1",
		Status: "online",
		Temperature: &models.Temperature{
			Available:  true,
			HasCPU:     true,
			CPUPackage: 55.2,
			CPUMax:     61.9,
		},
	}

	resource, _ := resourceFromProxmoxNode(node, nil)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if resource.Proxmox.SourceID != node.ID {
		t.Fatalf("SourceID = %q, want %q (must preserve legacy node ID for MetricsHistory)", resource.Proxmox.SourceID, node.ID)
	}
	if resource.Proxmox.Temperature == nil {
		t.Fatal("expected proxmox temperature to be populated")
	}
	if got, want := *resource.Proxmox.Temperature, 61.9; got != want {
		t.Fatalf("temperature = %v, want %v", got, want)
	}
}

func TestResourceFromProxmoxNodeUsesDisplayNameWithoutChangingRawNodeIdentity(t *testing.T) {
	node := models.Node{
		ID:          "pve-cluster-node01",
		Name:        "node01",
		DisplayName: "PVE Cluster (node01)",
		ClusterName: "PVE Cluster",
		Status:      "online",
	}

	resource, identity := resourceFromProxmoxNode(node, nil)
	if resource.Name != "PVE Cluster (node01)" {
		t.Fatalf("resource.Name = %q, want cluster-qualified display label", resource.Name)
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "node01" {
		t.Fatalf("Proxmox.NodeName = %+v, want raw node identity node01", resource.Proxmox)
	}
	foundClusterIdentity := false
	for _, hostname := range identity.Hostnames {
		if hostname == "PVE Cluster:node01" {
			foundClusterIdentity = true
			break
		}
	}
	if !foundClusterIdentity {
		t.Fatalf("identity hostnames = %v, want cluster-qualified raw hostname identity", identity.Hostnames)
	}
}

func TestResourceFromDockerContainerAdvertisesLifecycleCapabilities(t *testing.T) {
	now := time.Now().UTC()
	host := models.DockerHost{
		ID:       "docker-host-1",
		AgentID:  "agent-1",
		Hostname: "dock1",
		Runtime:  "docker",
		LastSeen: now,
	}

	running, _ := resourceFromDockerContainer(models.DockerContainer{
		ID:    "abc123",
		Name:  "web",
		State: "running",
	}, host)
	if running.Docker == nil || running.Docker.AgentID != "agent-1" {
		t.Fatalf("docker agent id was not projected onto container resource: %#v", running.Docker)
	}
	if got := capabilityNames(running.Capabilities); !reflect.DeepEqual(got, []string{"stop", "restart"}) {
		t.Fatalf("running container capabilities = %#v, want stop/restart", got)
	}
	for _, capability := range running.Capabilities {
		if capability.MinimumApprovalLevel != ApprovalAdmin {
			t.Fatalf("capability %q approval = %q, want admin", capability.Name, capability.MinimumApprovalLevel)
		}
		if capability.Platform != "docker" || capability.InternalHandler != "docker.container.lifecycle" {
			t.Fatalf("capability %q platform/handler = %q/%q", capability.Name, capability.Platform, capability.InternalHandler)
		}
	}

	exited, _ := resourceFromDockerContainer(models.DockerContainer{
		ID:    "def456",
		Name:  "worker",
		State: "exited",
	}, host)
	if got := capabilityNames(exited.Capabilities); !reflect.DeepEqual(got, []string{"start"}) {
		t.Fatalf("exited container capabilities = %#v, want start", got)
	}
}

func TestResourceFromDockerContainerLifecycleCapabilitiesFailClosed(t *testing.T) {
	now := time.Now().UTC()
	baseHost := models.DockerHost{
		ID:       "docker-host-1",
		AgentID:  "agent-1",
		Hostname: "dock1",
		Runtime:  "docker",
		LastSeen: now,
	}
	container := models.DockerContainer{ID: "abc123", Name: "web", State: "running"}

	cases := []struct {
		name string
		host models.DockerHost
		ct   models.DockerContainer
	}{
		{
			name: "missing agent",
			host: func() models.DockerHost {
				host := baseHost
				host.AgentID = ""
				return host
			}(),
			ct: container,
		},
		{
			name: "stale inventory",
			host: func() models.DockerHost {
				host := baseHost
				host.LastSeen = now.Add(-10 * time.Minute)
				return host
			}(),
			ct: container,
		},
		{
			name: "daemon mutation blocked",
			host: func() models.DockerHost {
				host := baseHost
				host.Security = &models.DockerHostSecurity{MutatingCommandsBlocked: true, MutatingCommandsBlockedReason: "blocked"}
				return host
			}(),
			ct: container,
		},
		{
			name: "unsupported runtime",
			host: func() models.DockerHost {
				host := baseHost
				host.Runtime = "containerd"
				return host
			}(),
			ct: container,
		},
		{
			name: "non-actionable state",
			host: baseHost,
			ct:   models.DockerContainer{ID: "abc123", Name: "web", State: "restarting"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource, _ := resourceFromDockerContainer(tc.ct, tc.host)
			if len(resource.Capabilities) != 0 {
				t.Fatalf("capabilities = %#v, want none", resource.Capabilities)
			}
		})
	}
}

func TestResourceFromDockerContainerPodmanLifecycleCapabilities(t *testing.T) {
	resource, _ := resourceFromDockerContainer(models.DockerContainer{
		ID:     "podman-1",
		Name:   "api",
		State:  "running",
		Podman: &models.DockerPodmanContainer{PodName: "apps"},
	}, models.DockerHost{
		ID:       "podman-host-1",
		AgentID:  "agent-1",
		Hostname: "podman1",
		LastSeen: time.Now().UTC(),
	})

	if got := capabilityNames(resource.Capabilities); !reflect.DeepEqual(got, []string{"stop", "restart"}) {
		t.Fatalf("podman capabilities = %#v, want stop/restart", got)
	}
	for _, capability := range resource.Capabilities {
		if capability.Platform != "podman" {
			t.Fatalf("capability %q platform = %q, want podman", capability.Name, capability.Platform)
		}
	}
	if resource.Docker == nil || resource.Docker.Runtime != "podman" {
		t.Fatalf("podman runtime not projected: %#v", resource.Docker)
	}
}

func TestResourceFromProxmoxGuestAdvertisesLifecycleCapabilities(t *testing.T) {
	now := time.Now().UTC()

	vm, _ := resourceFromVM(models.VM{
		ID:       "homelab:delly:160",
		VMID:     160,
		Name:     "windows-runner",
		Node:     "delly",
		Instance: "homelab",
		Status:   "running",
		LastSeen: now,
	})
	if got := capabilityNames(vm.Capabilities); !reflect.DeepEqual(got, []string{"shutdown", "reboot", "stop"}) {
		t.Fatalf("running VM capabilities = %#v, want shutdown/reboot/stop", got)
	}
	for _, capability := range vm.Capabilities {
		if capability.MinimumApprovalLevel != ApprovalAdmin {
			t.Fatalf("VM capability %q approval = %q, want admin", capability.Name, capability.MinimumApprovalLevel)
		}
		if capability.Platform != "qemu" || capability.InternalHandler != "proxmox.vm.lifecycle" {
			t.Fatalf("VM capability %q platform/handler = %q/%q", capability.Name, capability.Platform, capability.InternalHandler)
		}
	}

	stoppedCT, _ := resourceFromContainer(models.Container{
		ID:       "homelab:delly:101",
		VMID:     101,
		Name:     "homeassistant",
		Node:     "delly",
		Instance: "homelab",
		Status:   "stopped",
		LastSeen: now,
	})
	if got := capabilityNames(stoppedCT.Capabilities); !reflect.DeepEqual(got, []string{"start"}) {
		t.Fatalf("stopped LXC capabilities = %#v, want start", got)
	}
	if stoppedCT.Capabilities[0].Platform != "lxc" || stoppedCT.Capabilities[0].InternalHandler != "proxmox.ct.lifecycle" {
		t.Fatalf("LXC platform/handler = %q/%q", stoppedCT.Capabilities[0].Platform, stoppedCT.Capabilities[0].InternalHandler)
	}
}

func TestResourceFromProxmoxGuestLifecycleCapabilitiesFailClosed(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name string
		vm   models.VM
	}{
		{
			name: "template",
			vm: models.VM{
				ID:       "homelab:delly:9000",
				VMID:     9000,
				Name:     "template",
				Node:     "delly",
				Instance: "homelab",
				Status:   "stopped",
				Template: true,
				LastSeen: now,
			},
		},
		{
			name: "locked",
			vm: models.VM{
				ID:       "homelab:delly:161",
				VMID:     161,
				Name:     "locked",
				Node:     "delly",
				Instance: "homelab",
				Status:   "running",
				Lock:     "backup",
				LastSeen: now,
			},
		},
		{
			name: "unknown transition state",
			vm: models.VM{
				ID:       "homelab:delly:162",
				VMID:     162,
				Name:     "transitioning",
				Node:     "delly",
				Instance: "homelab",
				Status:   "migrating",
				LastSeen: now,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resource, _ := resourceFromVM(tc.vm)
			if len(resource.Capabilities) != 0 {
				t.Fatalf("capabilities = %#v, want none", resource.Capabilities)
			}
		})
	}
}

func capabilityNames(capabilities []ResourceCapability) []string {
	names := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		names = append(names, capability.Name)
	}
	return names
}

func TestResourceFromProxmoxNodeStoresEndpointIPAsIPAddress(t *testing.T) {
	node := models.Node{
		ID:   "mock-cluster-minipc",
		Name: "minipc",
		Host: "https://10.0.0.5:8006",
	}

	_, identity := resourceFromProxmoxNode(node, nil)
	if len(identity.Hostnames) != 1 || identity.Hostnames[0] != "minipc" {
		t.Fatalf("Hostnames = %v, want [minipc]", identity.Hostnames)
	}
	if len(identity.IPAddresses) != 1 || identity.IPAddresses[0] != "10.0.0.5" {
		t.Fatalf("IPAddresses = %v, want [10.0.0.5]", identity.IPAddresses)
	}
}

func TestResourceFromProxmoxNodeInheritsLinkedHostIdentity(t *testing.T) {
	node := models.Node{
		ID:            "mock-cluster-minipc",
		Name:          "minipc",
		Host:          "https://10.0.0.5:8006",
		LinkedAgentID: "host-1",
	}
	host := &models.Host{
		ID:        "host-1",
		Hostname:  "minipc.local",
		MachineID: "machine-1",
		ReportIP:  "10.0.0.5",
		NetworkInterfaces: []models.HostNetworkInterface{
			{Name: "eth0", MAC: "00:11:22:33:44:55", Addresses: []string{"10.0.0.5/24"}},
		},
	}

	_, identity := resourceFromProxmoxNode(node, host)
	if identity.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", identity.MachineID)
	}
	if len(identity.MACAddresses) != 1 || identity.MACAddresses[0] != "00:11:22:33:44:55" {
		t.Fatalf("MACAddresses = %v, want [00:11:22:33:44:55]", identity.MACAddresses)
	}
}

func TestMaxNodeTempHandlesUnavailableData(t *testing.T) {
	if temp := maxNodeTemp(nil); temp != nil {
		t.Fatalf("expected nil temperature for nil input, got %v", *temp)
	}

	input := &models.Temperature{
		Available: false,
		HasCPU:    false,
	}
	if temp := maxNodeTemp(input); temp != nil {
		t.Fatalf("expected nil temperature for unavailable snapshot, got %v", *temp)
	}
}

func TestResourceFromDockerHostIncludesTemperature(t *testing.T) {
	value := 52.7
	host := models.DockerHost{
		ID:          "docker-1",
		Hostname:    "docker-1",
		DisplayName: "Docker 1",
		Status:      "online",
		Temperature: &value,
	}

	resource, _ := resourceFromDockerHost(host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if resource.Docker.Temperature == nil {
		t.Fatal("expected docker temperature to be populated")
	}
	if got, want := *resource.Docker.Temperature, value; got != want {
		t.Fatalf("temperature = %v, want %v", got, want)
	}
}

func TestResourceFromDockerHostOmitsStandaloneInactiveSwarm(t *testing.T) {
	host := models.DockerHost{
		ID:       "docker-1",
		Hostname: "docker-1",
		Status:   "online",
		Swarm: &models.DockerSwarmInfo{
			NodeRole:   "worker",
			LocalState: "inactive",
			Scope:      "node",
		},
	}

	resource, _ := resourceFromDockerHost(host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if resource.Docker.Swarm != nil {
		t.Fatalf("standalone inactive Docker Swarm metadata should be omitted, got %+v", resource.Docker.Swarm)
	}
}

func TestResourceFromDockerContainerIncludesContainerID(t *testing.T) {
	container := models.DockerContainer{
		ID:            "aurora-3-abcdef123456",
		Name:          "web",
		State:         "running",
		Image:         "ghcr.io/example/web:1.0.0",
		ImageDigest:   "sha256:current",
		UptimeSeconds: 1234,
		CPUPercent:    12.5,
	}

	host := models.DockerHost{
		ID:             "docker-1",
		Hostname:       "docker-1",
		Runtime:        "podman",
		RuntimeVersion: "5.2.0",
	}
	resource, _ := resourceFromDockerContainer(container, host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if got, want := resource.Docker.ContainerID, container.ID; got != want {
		t.Fatalf("containerId = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Runtime, host.Runtime; got != want {
		t.Fatalf("runtime = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Hostname, host.Hostname; got != want {
		t.Fatalf("hostname = %q, want %q", got, want)
	}
	if got, want := resource.Docker.RuntimeVersion, host.RuntimeVersion; got != want {
		t.Fatalf("runtimeVersion = %q, want %q", got, want)
	}
	if got, want := resource.Docker.ImageID, container.ImageDigest; got != want {
		t.Fatalf("imageId = %q, want %q", got, want)
	}
}

func TestResourceFromDockerContainerPreservesRuntimeMetadata(t *testing.T) {
	startedAt := time.Date(2026, 6, 11, 8, 15, 30, 0, time.FixedZone("UTC-5", -5*60*60))
	finishedAt := startedAt.Add(45 * time.Minute)
	container := models.DockerContainer{
		ID:         "abcdef123456",
		Name:       "edge-web",
		State:      "exited",
		StartedAt:  &startedAt,
		FinishedAt: &finishedAt,
		BlockIO: &models.DockerContainerBlockIO{
			ReadBytes:  9_876_543,
			WriteBytes: 1_234_567,
		},
		Podman: &models.DockerPodmanContainer{
			PodName:          "edge-pod",
			PodID:            "pod-123",
			Infra:            true,
			ComposeProject:   "orion",
			ComposeService:   "web",
			AutoUpdatePolicy: "registry",
			UserNamespace:    "keep-id",
		},
	}
	host := models.DockerHost{
		ID:       "podman-1",
		Hostname: "podman-1",
	}

	resource, _ := resourceFromDockerContainer(container, host)
	if resource.Docker == nil {
		t.Fatal("expected docker payload")
	}
	if got, want := resource.Docker.Runtime, "podman"; got != want {
		t.Fatalf("runtime = %q, want %q", got, want)
	}
	if resource.Docker.StartedAt == nil {
		t.Fatal("expected startedAt")
	}
	if got, want := *resource.Docker.StartedAt, startedAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("startedAt = %s (%s), want UTC %s", got, got.Location(), want)
	}
	if resource.Docker.FinishedAt == nil {
		t.Fatal("expected finishedAt")
	}
	if got, want := *resource.Docker.FinishedAt, finishedAt.UTC(); !got.Equal(want) || got.Location() != time.UTC {
		t.Fatalf("finishedAt = %s (%s), want UTC %s", got, got.Location(), want)
	}
	if resource.Docker.BlockIO == nil {
		t.Fatal("expected block IO totals")
	}
	if got, want := resource.Docker.BlockIO.ReadBytes, container.BlockIO.ReadBytes; got != want {
		t.Fatalf("blockIo.readBytes = %d, want %d", got, want)
	}
	if got, want := resource.Docker.BlockIO.WriteBytes, container.BlockIO.WriteBytes; got != want {
		t.Fatalf("blockIo.writeBytes = %d, want %d", got, want)
	}
	if resource.Docker.Podman == nil {
		t.Fatal("expected podman metadata")
	}
	if got, want := resource.Docker.Podman.PodName, container.Podman.PodName; got != want {
		t.Fatalf("podman.podName = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Podman.PodID, container.Podman.PodID; got != want {
		t.Fatalf("podman.podId = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Podman.Infra, container.Podman.Infra; got != want {
		t.Fatalf("podman.infra = %t, want %t", got, want)
	}
	if got, want := resource.Docker.Podman.ComposeProject, container.Podman.ComposeProject; got != want {
		t.Fatalf("podman.composeProject = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Podman.ComposeService, container.Podman.ComposeService; got != want {
		t.Fatalf("podman.composeService = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Podman.AutoUpdatePolicy, container.Podman.AutoUpdatePolicy; got != want {
		t.Fatalf("podman.autoUpdatePolicy = %q, want %q", got, want)
	}
	if got, want := resource.Docker.Podman.UserNamespace, container.Podman.UserNamespace; got != want {
		t.Fatalf("podman.userNamespace = %q, want %q", got, want)
	}
}

func TestResourceFromHostProjectsAgentHostProfile(t *testing.T) {
	host := models.Host{
		ID:        "tower-host",
		Hostname:  "tower",
		Platform:  "slackware",
		OSName:    "Unraid",
		OSVersion: "7.1.0",
		Status:    "online",
	}

	resource, _ := resourceFromHost(host)
	if resource.Technology != "linux" {
		t.Fatalf("Technology = %q, want linux runtime platform", resource.Technology)
	}
	if resource.Agent == nil {
		t.Fatal("expected agent payload")
	}
	if resource.Agent.Platform != "linux" {
		t.Fatalf("agent platform = %q, want linux runtime platform", resource.Agent.Platform)
	}
	if resource.Agent.HostProfile != "unraid" {
		t.Fatalf("agent host profile = %q, want unraid", resource.Agent.HostProfile)
	}
}

func TestResourceFromHostProjectsPressureOnlyThermalState(t *testing.T) {
	warningLevel := 1
	host := models.Host{
		ID:       "mac-host",
		Hostname: "mac-mini",
		Platform: "macos",
		Status:   "online",
		Sensors: models.HostSensorSummary{
			ThermalState: &models.HostThermalState{
				Source:              "pmset",
				Pressure:            "constrained",
				ThermalWarningLevel: &warningLevel,
				LimitsPercent:       map[string]int{"cpu_speed_limit": 72},
			},
		},
	}

	resource, _ := resourceFromHost(host)
	if resource.Agent == nil || resource.Agent.Sensors == nil || resource.Agent.Sensors.ThermalState == nil {
		t.Fatalf("expected host thermal state on agent resource, got %+v", resource.Agent)
	}
	state := resource.Agent.Sensors.ThermalState
	if state.Source != "pmset" || state.Pressure != "constrained" {
		t.Fatalf("unexpected thermal state: %+v", state)
	}
	if got := state.LimitsPercent["cpu_speed_limit"]; got != 72 {
		t.Fatalf("thermal limit = %d, want 72", got)
	}
	if resource.Agent.Temperature != nil || len(resource.Agent.Sensors.TemperatureCelsius) != 0 {
		t.Fatalf("pressure-only host must not create Celsius values: agent=%+v sensors=%+v", resource.Agent.Temperature, resource.Agent.Sensors)
	}
}

func TestResourceFromHostProjectsTypedGPUSensors(t *testing.T) {
	temperature := 63.0
	utilization := 18.0
	usedBytes := int64(3 * 1024 * 1024 * 1024)
	totalBytes := int64(24 * 1024 * 1024 * 1024)
	host := models.Host{
		ID:       "gpu-host",
		Hostname: "gpu-node",
		Platform: "linux",
		Status:   "online",
		Sensors: models.HostSensorSummary{
			TemperatureCelsius: map[string]float64{"gpu_nvidia_0": temperature},
			GPU: []models.HostGPUSensor{
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
	}

	resource, _ := resourceFromHost(host)
	if resource.Agent == nil || resource.Agent.Sensors == nil {
		t.Fatalf("expected agent sensor payload, got %+v", resource.Agent)
	}
	if len(resource.Agent.Sensors.GPU) != 1 {
		t.Fatalf("GPU sensors = %+v, want one sensor", resource.Agent.Sensors.GPU)
	}
	gpu := resource.Agent.Sensors.GPU[0]
	if gpu.ID != "0" || gpu.Name != "NVIDIA RTX A6000" {
		t.Fatalf("unexpected GPU identity: %+v", gpu)
	}
	if gpu.UtilizationPercent == nil || *gpu.UtilizationPercent != utilization {
		t.Fatalf("GPU utilization = %#v, want %.1f", gpu.UtilizationPercent, utilization)
	}
	if gpu.MemoryTotalBytes == nil || *gpu.MemoryTotalBytes != totalBytes {
		t.Fatalf("GPU memory total = %#v, want %d", gpu.MemoryTotalBytes, totalBytes)
	}
	if resource.Agent.Sensors.TemperatureCelsius["gpu_nvidia_0"] != temperature {
		t.Fatalf("legacy GPU temperature = %+v, want %.1f", resource.Agent.Sensors.TemperatureCelsius, temperature)
	}
}

func TestResourceFromHostProjectsPowerSensors(t *testing.T) {
	host := models.Host{
		ID:       "power-host",
		Hostname: "power-node",
		Platform: "linux",
		Status:   "online",
		Sensors: models.HostSensorSummary{
			PowerWatts: map[string]float64{
				"cpu_package": 82.4,
				"dram":        13.2,
			},
		},
	}

	resource, _ := resourceFromHost(host)
	if resource.Agent == nil || resource.Agent.Sensors == nil {
		t.Fatalf("expected agent sensor payload, got %+v", resource.Agent)
	}
	if got := resource.Agent.Sensors.PowerWatts["cpu_package"]; got != 82.4 {
		t.Fatalf("power sensor = %.1f, want 82.4", got)
	}
	if resource.Agent.Temperature != nil || len(resource.Agent.Sensors.TemperatureCelsius) != 0 {
		t.Fatalf("power-only sensors must not create Celsius values: agent=%+v sensors=%+v", resource.Agent.Temperature, resource.Agent.Sensors)
	}
}

func TestResourceFromVMPreservesProxmoxPool(t *testing.T) {
	vm := models.VM{
		ID:       "cluster-a:pve-a:101",
		Name:     "app-vm",
		Node:     "pve-a",
		Pool:     "prod-vms",
		Instance: "cluster-a",
		VMID:     101,
		Status:   "running",
	}

	resource, _ := resourceFromVM(vm)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if got, want := resource.Proxmox.Pool, "prod-vms"; got != want {
		t.Fatalf("pool = %q, want %q", got, want)
	}
}

func TestResourceFromContainerPreservesProxmoxPool(t *testing.T) {
	container := models.Container{
		ID:       "cluster-a:pve-a:202",
		Name:     "cache-ct",
		Node:     "pve-a",
		Pool:     "ops-lxc",
		Instance: "cluster-a",
		VMID:     202,
		Status:   "running",
		Type:     "lxc",
	}

	resource, _ := resourceFromContainer(container)
	if resource.Proxmox == nil {
		t.Fatal("expected proxmox payload")
	}
	if got, want := resource.Proxmox.Pool, "ops-lxc"; got != want {
		t.Fatalf("pool = %q, want %q", got, want)
	}
}

func TestResourceFromStorageIncludesStorageMetadata(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-1",
		Name:     "ceph-rbd-pool",
		Node:     "pve-1",
		Instance: "cluster-a",
		Type:     "RBD",
		Pool:     "ceph/rbd-a",
		Content:  "images, rootdir,images",
		Shared:   true,
		Status:   "available",
		Enabled:  true,
		Active:   true,
		Total:    100,
		Used:     20,
		Free:     80,
		Usage:    20,
	}

	resource, _ := resourceFromStorage(storage)
	if resource.Storage == nil {
		t.Fatal("expected storage metadata payload")
	}
	if got, want := resource.Storage.Type, "rbd"; got != want {
		t.Fatalf("storage type = %q, want %q", got, want)
	}
	if got, want := resource.Storage.Content, "images, rootdir,images"; got != want {
		t.Fatalf("storage content = %q, want %q", got, want)
	}
	if got, want := resource.Storage.Pool, "ceph/rbd-a"; got != want {
		t.Fatalf("storage pool = %q, want %q", got, want)
	}
	wantContentTypes := []string{"images", "rootdir"}
	if len(resource.Storage.ContentTypes) != len(wantContentTypes) {
		t.Fatalf("contentTypes length = %d, want %d (%v)", len(resource.Storage.ContentTypes), len(wantContentTypes), resource.Storage.ContentTypes)
	}
	for i, want := range wantContentTypes {
		if got := resource.Storage.ContentTypes[i]; got != want {
			t.Fatalf("contentTypes[%d] = %q, want %q", i, got, want)
		}
	}
	if !resource.Storage.Shared {
		t.Fatalf("expected shared=true")
	}
	if !resource.Storage.IsCeph {
		t.Fatalf("expected isCeph=true")
	}
	if resource.Storage.IsZFS {
		t.Fatalf("expected isZfs=false")
	}
	if resource.Proxmox == nil || resource.Proxmox.NodeName != "pve-1" || resource.Proxmox.Instance != "cluster-a" {
		t.Fatalf("expected existing proxmox mapping to remain populated, got %+v", resource.Proxmox)
	}
}

func TestResourceFromStorageDetectsZFSMetadata(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-zfs",
		Name:     "local-zfs",
		Node:     "pve-2",
		Instance: "cluster-a",
		Type:     "zfspool",
		Content:  "images",
		Shared:   false,
		Status:   "available",
		Enabled:  true,
		Active:   true,
		ZFSPool: &models.ZFSPool{
			Name:  "rpool",
			State: "DEGRADED",
			Scan:  "resilver in progress since Thu Jun 11 09:00:00 2026",
			Devices: []models.ZFSDevice{
				{Name: "sda2", Type: "disk", State: "ONLINE"},
				{Name: "sdb2", Type: "disk", State: "FAULTED", ReadErrors: 3},
			},
		},
	}

	resource, _ := resourceFromStorage(storage)
	if resource.Storage == nil {
		t.Fatal("expected storage metadata payload")
	}
	if !resource.Storage.IsZFS {
		t.Fatalf("expected isZfs=true")
	}
	if resource.Storage.IsCeph {
		t.Fatalf("expected isCeph=false")
	}
	if resource.Storage.Shared {
		t.Fatalf("expected shared=false")
	}
	pool := resource.Storage.ZFSPool
	if pool == nil {
		t.Fatal("expected full ZFS pool payload on storage metadata")
	}
	if pool.Scan != storage.ZFSPool.Scan {
		t.Fatalf("zfs scan = %q, want %q", pool.Scan, storage.ZFSPool.Scan)
	}
	if len(pool.Devices) != 2 {
		t.Fatalf("zfs devices = %d, want 2", len(pool.Devices))
	}
	if pool.Devices[1].State != "FAULTED" || pool.Devices[1].ReadErrors != 3 {
		t.Fatalf("zfs device passthrough mismatch: %+v", pool.Devices[1])
	}
	if pool == storage.ZFSPool {
		t.Fatal("expected ZFS pool to be copied, not aliased to the source pointer")
	}
}

func TestResourceFromStorageDerivesZFSTopologyRisk(t *testing.T) {
	storage := models.Storage{
		ID:       "storage-zfs-risk",
		Name:     "tank",
		Node:     "pve-2",
		Instance: "cluster-a",
		Type:     "zfspool",
		Content:  "images",
		Shared:   false,
		Status:   "available",
		Enabled:  true,
		Active:   true,
		ZFSPool: &models.ZFSPool{
			Name:           "tank",
			State:          "ONLINE",
			ChecksumErrors: 2,
		},
	}

	resource, _ := resourceFromStorage(storage)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.Storage == nil || resource.Storage.Risk == nil {
		t.Fatalf("expected zfs topology risk payload, got %+v", resource.Storage)
	}
	if resource.Storage.Risk.Level != storagehealth.RiskWarning {
		t.Fatalf("risk level = %q, want %q", resource.Storage.Risk.Level, storagehealth.RiskWarning)
	}
}

func TestResourceFromPBSDatastoreDerivesRiskAndIncidents(t *testing.T) {
	now := time.Now().UTC()
	instance := models.PBSInstance{
		ID:       "pbs-1",
		Name:     "pbs-main",
		Host:     "https://pbs-main.local:8007",
		LastSeen: now,
	}
	datastore := models.PBSDatastore{
		Name:   "backup-store",
		Status: "online",
		Total:  100,
		Used:   96,
	}

	resource, _ := resourceFromPBSDatastore(instance, datastore)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.Storage == nil || resource.Storage.Risk == nil {
		t.Fatalf("expected PBS datastore risk payload, got %+v", resource.Storage)
	}
	if resource.Storage.Risk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", resource.Storage.Risk.Level, storagehealth.RiskCritical)
	}
	if len(resource.Incidents) != 1 {
		t.Fatalf("expected 1 datastore incident, got %+v", resource.Incidents)
	}
	if resource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("incident code = %q, want capacity_runway_low", resource.Incidents[0].Code)
	}
	if resource.Incidents[0].Source != "pbs" {
		t.Fatalf("incident source = %q, want pbs", resource.Incidents[0].Source)
	}
}

func TestResourceFromPBSInstanceRollsUpDatastoreRisk(t *testing.T) {
	now := time.Now().UTC()
	instance := models.PBSInstance{
		ID:       "pbs-1",
		Name:     "pbs-main",
		Host:     "https://pbs-main.local:8007",
		Status:   "online",
		LastSeen: now,
		Datastores: []models.PBSDatastore{
			{
				Name:   "backup-store",
				Status: "online",
				Total:  100,
				Used:   96,
			},
		},
	}

	resource, _ := resourceFromPBSInstance(instance)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.PBS == nil || resource.PBS.StorageRisk == nil {
		t.Fatalf("expected PBS storage risk payload, got %+v", resource.PBS)
	}
	if resource.PBS.StorageRisk.Level != storagehealth.RiskCritical {
		t.Fatalf("storage risk level = %q, want %q", resource.PBS.StorageRisk.Level, storagehealth.RiskCritical)
	}
	if len(resource.Incidents) != 1 {
		t.Fatalf("expected 1 rolled-up PBS incident, got %+v", resource.Incidents)
	}
	if resource.Incidents[0].Code != "capacity_runway_low" {
		t.Fatalf("incident code = %q, want capacity_runway_low", resource.Incidents[0].Code)
	}
}

func TestResourceFromHostUnraidStorageIncludesTopologyMetadata(t *testing.T) {
	host := models.Host{
		ID:          "tower-host",
		Hostname:    "tower",
		DisplayName: "Tower",
		Status:      "online",
		LastSeen:    time.Now().UTC(),
		Disks: []models.Disk{
			{Mountpoint: "/mnt/user", Total: 1000, Used: 400, Free: 600, Usage: 40},
		},
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			ArrayState:   "STARTED",
			SyncAction:   "check",
			SyncProgress: 22,
			NumProtected: 1,
			Disks: []models.HostUnraidDisk{
				{Name: "parity", Role: "parity", Status: "online"},
				{Name: "disk1", Role: "data", Status: "online"},
			},
		},
	}

	resource, identity := resourceFromHostUnraidStorage(host)
	if resource.Type != ResourceTypeStorage {
		t.Fatalf("Type = %q, want %q", resource.Type, ResourceTypeStorage)
	}
	if resource.Storage == nil {
		t.Fatal("expected storage metadata payload")
	}
	if got := resource.Storage.Type; got != "unraid-array" {
		t.Fatalf("storage type = %q, want unraid-array", got)
	}
	if got := resource.Storage.Platform; got != "unraid" {
		t.Fatalf("platform = %q, want unraid", got)
	}
	if got := resource.Storage.Protection; got != "single-parity" {
		t.Fatalf("protection = %q, want single-parity", got)
	}
	if resource.Metrics == nil || resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent != 40 {
		t.Fatalf("expected disk metrics from /mnt/user, got %+v", resource.Metrics)
	}
	if got := identity.MachineID; got != "" && !strings.Contains(got, "/storage/unraid-array") {
		t.Fatalf("expected unraid storage identity suffix, got %q", got)
	}
}

func TestResourceFromHostDerivesStorageTopologyRisk(t *testing.T) {
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		Status:   "online",
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{Name: "parity", Role: "parity", Status: "disabled"},
				{Name: "disk1", Role: "data", Status: "online"},
			},
		},
	}

	resource, _ := resourceFromHost(host)
	if resource.Status != StatusWarning {
		t.Fatalf("Status = %q, want %q", resource.Status, StatusWarning)
	}
	if resource.Agent == nil || resource.Agent.StorageRisk == nil {
		t.Fatalf("expected host storage risk payload, got %+v", resource.Agent)
	}
	if resource.Agent.StorageRisk.Level != storagehealth.RiskCritical {
		t.Fatalf("risk level = %q, want %q", resource.Agent.StorageRisk.Level, storagehealth.RiskCritical)
	}
	if resource.Agent.StorageRiskSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("storage risk summary = %q", resource.Agent.StorageRiskSummary)
	}
	if resource.Agent.StoragePostureSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("storage posture summary = %q", resource.Agent.StoragePostureSummary)
	}
	if !resource.Agent.ProtectionReduced || resource.Agent.ProtectionSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("protection semantics = reduced:%v summary:%q", resource.Agent.ProtectionReduced, resource.Agent.ProtectionSummary)
	}
	if resource.Agent.Unraid == nil || resource.Agent.Unraid.Risk == nil {
		t.Fatalf("expected unraid risk payload, got %+v", resource.Agent.Unraid)
	}
	if resource.Agent.Unraid.RiskSummary != "Unraid parity disk parity is DISABLED" || resource.Agent.Unraid.PostureSummary != "Unraid parity disk parity is DISABLED" {
		t.Fatalf("unexpected unraid summaries %+v", resource.Agent.Unraid)
	}
}

func TestResourceFromHostSMARTDiskCarriesUnraidRole(t *testing.T) {
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		LastSeen: time.Now().UTC(),
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{
					Name:       "parity",
					Device:     "/dev/sdb",
					Role:       "parity",
					Status:     "online",
					Model:      "Seagate",
					Serial:     "PARITY-1",
					SizeBytes:  6_000_000_000_000,
					SpunDown:   true,
					ReadCount:  11,
					WriteCount: 12,
					ErrorCount: 16,
				},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "sdb [sat]", Serial: "PARITY-1"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.StorageRole != "parity" {
		t.Fatalf("storageRole = %q, want parity", resource.PhysicalDisk.StorageRole)
	}
	if resource.PhysicalDisk.StorageGroup != "unraid-array" {
		t.Fatalf("storageGroup = %q, want unraid-array", resource.PhysicalDisk.StorageGroup)
	}
	if resource.PhysicalDisk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("sizeBytes = %d, want unraid disk size", resource.PhysicalDisk.SizeBytes)
	}
	if resource.PhysicalDisk.Health != "PASSED" {
		t.Fatalf("health = %q, want PASSED from Unraid online state", resource.PhysicalDisk.Health)
	}
	if !resource.PhysicalDisk.SpunDown || resource.PhysicalDisk.ReadCount != 11 || resource.PhysicalDisk.WriteCount != 12 || resource.PhysicalDisk.ErrorCount != 16 {
		t.Fatalf("expected native Unraid disk counters, got %+v", resource.PhysicalDisk)
	}
}

func TestResourceFromHostSMARTDiskInfersUnraidArrayRole(t *testing.T) {
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		LastSeen: time.Now().UTC(),
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{
					Name:      "md1p1",
					Device:    "/dev/sde",
					Status:    "online",
					SizeBytes: 6_000_000_000_000,
					Slot:      1,
				},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "sde [sat]", Health: "UNKNOWN"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.StorageRole != "data" {
		t.Fatalf("storageRole = %q, want data", resource.PhysicalDisk.StorageRole)
	}
	if resource.PhysicalDisk.StorageGroup != "unraid-array" {
		t.Fatalf("storageGroup = %q, want unraid-array", resource.PhysicalDisk.StorageGroup)
	}
	if resource.PhysicalDisk.SizeBytes != 6_000_000_000_000 {
		t.Fatalf("sizeBytes = %d, want unraid disk size", resource.PhysicalDisk.SizeBytes)
	}
	if resource.PhysicalDisk.Health != "PASSED" {
		t.Fatalf("health = %q, want PASSED from Unraid online state", resource.PhysicalDisk.Health)
	}
}

func TestResourceFromHostSMARTDiskNormalizesLegacyUnraidKiBSize(t *testing.T) {
	const legacyDiskKiB = int64(5_860_522_532)
	const normalizedDiskBytes = legacyDiskKiB * 1024
	host := models.Host{
		ID:       "tower-host",
		Hostname: "tower",
		LastSeen: time.Now().UTC(),
		Disks: []models.Disk{
			{
				Mountpoint: "/mnt/user",
				Total:      normalizedDiskBytes * 4,
				Used:       normalizedDiskBytes * 3,
				Free:       normalizedDiskBytes,
				Usage:      75,
			},
		},
		Unraid: &models.HostUnraidStorage{
			ArrayStarted: true,
			Disks: []models.HostUnraidDisk{
				{Name: "md1p1", Device: "/dev/sde", Status: "online", SizeBytes: legacyDiskKiB, Slot: 1},
				{Name: "md2p1", Device: "/dev/sdd", Status: "online", SizeBytes: legacyDiskKiB, Slot: 2},
				{Name: "md3p1", Device: "/dev/sdb", Status: "online", SizeBytes: legacyDiskKiB, Slot: 3},
				{Name: "md4p1", Device: "/dev/sdc", Status: "online", SizeBytes: legacyDiskKiB, Slot: 4},
			},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "sde [sat]", Health: "UNKNOWN"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.SizeBytes != normalizedDiskBytes {
		t.Fatalf("sizeBytes = %d, want normalized Unraid bytes %d", resource.PhysicalDisk.SizeBytes, normalizedDiskBytes)
	}

	agentResource, _ := resourceFromHost(host)
	if agentResource.Agent == nil || agentResource.Agent.Unraid == nil || len(agentResource.Agent.Unraid.Disks) == 0 {
		t.Fatalf("expected agent Unraid metadata, got %+v", agentResource.Agent)
	}
	if got := agentResource.Agent.Unraid.Disks[0].SizeBytes; got != normalizedDiskBytes {
		t.Fatalf("agent Unraid disk sizeBytes = %d, want %d", got, normalizedDiskBytes)
	}
}

func TestResourceFromHostSMARTDiskPrefersReportedSize(t *testing.T) {
	const diskBytes = int64(2000398934016)
	host := models.Host{
		ID:       "pve-host",
		Hostname: "pve",
		LastSeen: time.Now().UTC(),
		// A filesystem-usage entry whose Total is NOT the whole-disk size. The
		// adapter must not pick this up over the agent-reported capacity.
		Disks: []models.Disk{
			{Device: "/dev/nvme0n1p2", Mountpoint: "/", Total: 100_000_000_000},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				{Device: "nvme0n1", Serial: "50026B72FB69", Type: "nvme", Health: "PASSED", SizeBytes: diskBytes},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.SizeBytes != diskBytes {
		t.Fatalf("sizeBytes = %d, want reported capacity %d", resource.PhysicalDisk.SizeBytes, diskBytes)
	}
}

func TestResourceFromHostSMARTDiskFallsBackToFilesystemSize(t *testing.T) {
	const fsBytes = int64(240057409536)
	host := models.Host{
		ID:       "pve-host",
		Hostname: "pve",
		LastSeen: time.Now().UTC(),
		Disks: []models.Disk{
			{Device: "/dev/sda", Mountpoint: "/data", Total: fsBytes},
		},
		Sensors: models.HostSensorSummary{
			SMART: []models.HostDiskSMART{
				// Legacy agent with no SizeBytes: fall back to the filesystem match.
				{Device: "sda", Serial: "INTEL-1", Type: "sata", Health: "PASSED"},
			},
		},
	}

	resource, _ := resourceFromHostSMARTDisk(host, host.Sensors.SMART[0])
	if resource.PhysicalDisk == nil {
		t.Fatal("expected physical disk metadata")
	}
	if resource.PhysicalDisk.SizeBytes != fsBytes {
		t.Fatalf("sizeBytes = %d, want filesystem fallback %d", resource.PhysicalDisk.SizeBytes, fsBytes)
	}
}

func TestResourceFromKubernetesNodeProjectsClusterAgentVersion(t *testing.T) {
	cluster := models.KubernetesCluster{
		ID:           "cluster-1",
		AgentID:      "agent-k8s-1",
		Name:         "prod",
		DisplayName:  "Production",
		LastSeen:     time.Now().UTC(),
		AgentVersion: "5.1.34",
	}
	node := models.KubernetesNode{
		UID:   "node-1",
		Name:  "worker-1",
		Ready: true,
	}

	resource, _ := resourceFromKubernetesNode(cluster, node, nil, nil)
	if resource.Type != ResourceTypeK8sNode {
		t.Fatalf("resource type = %q, want %q", resource.Type, ResourceTypeK8sNode)
	}
	if resource.Agent != nil {
		t.Fatalf("expected pure k8s-node adapter row without agent facet, got %+v", resource.Agent)
	}
	if resource.Kubernetes == nil {
		t.Fatalf("expected kubernetes facet")
	}
	if resource.Kubernetes.AgentID != "agent-k8s-1" {
		t.Fatalf("agent id = %q, want agent-k8s-1", resource.Kubernetes.AgentID)
	}
	if resource.Kubernetes.AgentVersion != "5.1.34" {
		t.Fatalf("agent version = %q, want 5.1.34", resource.Kubernetes.AgentVersion)
	}
}

func TestResourceFromKubernetesDeployment_PopulatesMetricsUnderMockMode(t *testing.T) {
	mockruntime.SetEnabled(true)
	t.Cleanup(func() { mockruntime.SetEnabled(false) })

	cluster := models.KubernetesCluster{
		ID:   "cluster-1",
		Name: "production",
	}
	deployment := models.KubernetesDeployment{
		UID:                "dep-uid-1",
		Name:               "checkout-api",
		Namespace:          "services",
		CreatedAt:          time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC),
		DesiredReplicas:    3,
		UpdatedReplicas:    3,
		ReadyReplicas:      2,
		AvailableReplicas:  2,
		ObservedGeneration: 12,
	}

	resource, _ := resourceFromKubernetesDeployment(cluster, deployment, nil)
	if resource.Kubernetes == nil {
		t.Fatal("expected kubernetes payload")
	}
	if resource.Kubernetes.ResourceKind != "Deployment" || resource.Kubernetes.ResourceUID != "dep-uid-1" {
		t.Fatalf("deployment resource identity fields = kind %q uid %q", resource.Kubernetes.ResourceKind, resource.Kubernetes.ResourceUID)
	}
	if resource.Kubernetes.ObservedGeneration != 12 {
		t.Fatalf("observedGeneration = %d, want 12", resource.Kubernetes.ObservedGeneration)
	}
	if resource.Kubernetes.CreatedAt == nil || !resource.Kubernetes.CreatedAt.Equal(deployment.CreatedAt) {
		t.Fatalf("createdAt = %+v, want %s", resource.Kubernetes.CreatedAt, deployment.CreatedAt)
	}
	if resource.Metrics == nil {
		t.Fatal("expected mock-mode deployment to carry synthetic Metrics, got nil")
	}
	if resource.Metrics.CPU == nil || resource.Metrics.CPU.Percent <= 0 {
		t.Fatalf("expected non-zero CPU percent, got %+v", resource.Metrics.CPU)
	}
	if resource.Metrics.Memory == nil || resource.Metrics.Memory.Percent <= 0 {
		t.Fatalf("expected non-zero Memory percent, got %+v", resource.Metrics.Memory)
	}
	if resource.Metrics.Disk == nil || resource.Metrics.Disk.Percent < 0 {
		t.Fatalf("expected Disk metric, got %+v", resource.Metrics.Disk)
	}
}

func TestResourceFromKubernetesDeployment_NilMetricsOutsideMockMode(t *testing.T) {
	mockruntime.SetEnabled(false)
	cluster := models.KubernetesCluster{ID: "cluster-2", Name: "live"}
	deployment := models.KubernetesDeployment{
		Name: "frontend", Namespace: "web", DesiredReplicas: 2, ReadyReplicas: 2, AvailableReplicas: 2,
	}
	resource, _ := resourceFromKubernetesDeployment(cluster, deployment, nil)
	if resource.Metrics != nil {
		t.Fatalf("expected nil Metrics outside mock mode (no canonical aggregation yet), got %+v", resource.Metrics)
	}
}

func TestNamespacedKubernetesResourceScaffold(t *testing.T) {
	lastSeen := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	cluster := models.KubernetesCluster{ID: "cluster-3", Name: "prod", LastSeen: lastSeen}
	labels := map[string]string{"app": "ingest"}
	data := baseKubernetesData(cluster, "prod", "Job", nil)
	data.Namespace = "pipelines"

	resource, identity := namespacedKubernetesResource(cluster, "prod", "pipelines", "nightly", ResourceTypeK8sJob, StatusOnline, data, labels)

	if resource.Type != ResourceTypeK8sJob {
		t.Fatalf("type = %q, want %q", resource.Type, ResourceTypeK8sJob)
	}
	if resource.Technology != "kubernetes" {
		t.Fatalf("technology = %q, want kubernetes", resource.Technology)
	}
	if resource.Name != "nightly" {
		t.Fatalf("name = %q, want nightly", resource.Name)
	}
	if resource.Status != StatusOnline {
		t.Fatalf("status = %q, want %q", resource.Status, StatusOnline)
	}
	if !resource.LastSeen.Equal(lastSeen) {
		t.Fatalf("lastSeen = %s, want cluster lastSeen %s", resource.LastSeen, lastSeen)
	}
	if resource.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be stamped")
	}
	if resource.Kubernetes == nil || resource.Kubernetes.Namespace != "pipelines" {
		t.Fatalf("expected Kubernetes facet with namespace, got %+v", resource.Kubernetes)
	}
	if len(resource.Tags) == 0 {
		t.Fatal("expected label-derived tags on the scaffold")
	}
	wantHostnames := []string{"nightly", "pipelines/nightly", "prod:nightly"}
	if len(identity.Hostnames) != len(wantHostnames) {
		t.Fatalf("identity hostnames = %v, want %v", identity.Hostnames, wantHostnames)
	}
	for i, want := range wantHostnames {
		if identity.Hostnames[i] != want {
			t.Fatalf("identity hostnames[%d] = %q, want %q", i, identity.Hostnames[i], want)
		}
	}
	if identity.ClusterName != "prod" {
		t.Fatalf("identity clusterName = %q, want prod", identity.ClusterName)
	}
}

// Storage models carry the timestamp of the poll that delivered them. The
// adapter must pass that sighting through — and preserve zero for never-seen
// entries — instead of fabricating a fresh one at conversion time, because
// the registry is rebuilt from the retained snapshot every cycle and a
// conversion-time stamp would report a dead source as freshly delivering.
func TestResourceFromStorageUsesModelSighting(t *testing.T) {
	seen := time.Now().UTC().Add(-45 * time.Second).Truncate(time.Millisecond)
	storage := models.Storage{
		ID:       "cluster-a-pve-1-local",
		Name:     "local",
		Node:     "pve-1",
		Instance: "cluster-a",
		Type:     "dir",
		Status:   "available",
		LastSeen: seen,
	}

	resource, _ := resourceFromStorage(storage)
	if !resource.LastSeen.Equal(seen) {
		t.Fatalf("resource.LastSeen = %s, want the model sighting %s", resource.LastSeen, seen)
	}

	storage.LastSeen = time.Time{}
	resource, _ = resourceFromStorage(storage)
	if !resource.LastSeen.IsZero() {
		t.Fatalf("resource.LastSeen = %s, want zero for a never-seen storage entry", resource.LastSeen)
	}
}

// Docker containers are delivered wholesale with each host report, so the
// host's report timestamp is the container's sighting. Stamping conversion
// time instead would keep a container "fresh" forever after its host agent
// stopped reporting.
func TestResourceFromDockerContainerUsesHostSighting(t *testing.T) {
	seen := time.Now().UTC().Add(-45 * time.Second).Truncate(time.Millisecond)
	container := models.DockerContainer{
		ID:    "abcdef123456",
		Name:  "web",
		State: "running",
	}
	host := models.DockerHost{
		ID:       "docker-1",
		Hostname: "docker-1",
		LastSeen: seen,
	}

	resource, _ := resourceFromDockerContainer(container, host)
	if !resource.LastSeen.Equal(seen) {
		t.Fatalf("resource.LastSeen = %s, want the host report sighting %s", resource.LastSeen, seen)
	}

	host.LastSeen = time.Time{}
	resource, _ = resourceFromDockerContainer(container, host)
	if !resource.LastSeen.IsZero() {
		t.Fatalf("resource.LastSeen = %s, want zero when the host has never reported", resource.LastSeen)
	}
}

func TestResourceFromHostCarriesReclaimableMemoryCache(t *testing.T) {
	host := models.Host{
		ID:       "cache-host",
		Hostname: "cache-host",
		Platform: "linux",
		Status:   "online",
		Memory: models.Memory{
			Total: 16 << 30,
			Used:  6 << 30,
			Free:  4 << 30,
			Cache: 6 << 30,
			Usage: 37.5,
		},
	}

	resource, _ := resourceFromHost(host)
	if resource.Agent == nil || resource.Agent.Memory == nil {
		t.Fatal("expected agent memory payload")
	}
	if got, want := resource.Agent.Memory.Cache, int64(6<<30); got != want {
		t.Fatalf("agent memory cache = %d, want %d", got, want)
	}
	if sum := resource.Agent.Memory.Used + resource.Agent.Memory.Cache + resource.Agent.Memory.Free; sum > resource.Agent.Memory.Total {
		t.Fatalf("agent memory used+cache+free %d exceeds total %d", sum, resource.Agent.Memory.Total)
	}
}
