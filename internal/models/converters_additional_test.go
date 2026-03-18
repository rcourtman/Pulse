package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestKubernetesClusterToFrontend(t *testing.T) {
	now := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	lastUsed := time.Date(2024, 2, 4, 5, 6, 7, 0, time.UTC)

	cluster := KubernetesCluster{
		ID:               "cluster-1",
		AgentID:          "agent-1",
		Name:             "prod",
		DisplayName:      "",
		Server:           "https://k8s",
		Context:          "ctx",
		Version:          "v1.25.0",
		Status:           "healthy",
		LastSeen:         now,
		IntervalSeconds:  30,
		AgentVersion:     "v1",
		TokenID:          "token-1",
		TokenName:        "token-name",
		TokenHint:        "hint",
		TokenLastUsedAt:  &lastUsed,
		Hidden:           true,
		PendingUninstall: true,
		Nodes: []KubernetesNode{
			{
				UID:   "node-1",
				Name:  "node-a",
				Ready: true,
				Roles: []string{"master"},
			},
		},
		Pods: []KubernetesPod{
			{
				UID:       "pod-1",
				Name:      "pod-a",
				Namespace: "ns",
				Labels:    map[string]string{"app": "svc"},
				Containers: []KubernetesPodContainer{
					{Name: "c1", Ready: true},
				},
			},
		},
		Deployments: []KubernetesDeployment{
			{
				UID:             "dep-1",
				Name:            "deploy-a",
				Namespace:       "ns",
				DesiredReplicas: 2,
				Labels:          map[string]string{"tier": "web"},
			},
		},
	}

	frontend := cluster.ToFrontend()
	if frontend.DisplayName != cluster.Name {
		t.Fatalf("DisplayName = %q, want %q", frontend.DisplayName, cluster.Name)
	}
	if frontend.TokenLastUsedAt == nil || *frontend.TokenLastUsedAt != lastUsed.Unix()*1000 {
		t.Fatalf("TokenLastUsedAt = %v, want %d", frontend.TokenLastUsedAt, lastUsed.Unix()*1000)
	}
	if len(frontend.Nodes) != 1 || frontend.Nodes[0].Name != "node-a" {
		t.Fatalf("Nodes = %#v, want 1 node", frontend.Nodes)
	}
	if len(frontend.Pods) != 1 || frontend.Pods[0].Name != "pod-a" {
		t.Fatalf("Pods = %#v, want 1 pod", frontend.Pods)
	}
	if len(frontend.Deployments) != 1 || frontend.Deployments[0].Name != "deploy-a" {
		t.Fatalf("Deployments = %#v, want 1 deployment", frontend.Deployments)
	}
}

func TestKubernetesClusterToFrontend_DisplayNameFallback(t *testing.T) {
	cluster := KubernetesCluster{
		ID:   "cluster-2",
		Name: "",
	}

	frontend := cluster.ToFrontend()
	if frontend.DisplayName != cluster.ID {
		t.Fatalf("DisplayName = %q, want %q", frontend.DisplayName, cluster.ID)
	}
	if frontend.Nodes == nil || frontend.Pods == nil || frontend.Deployments == nil {
		t.Fatalf("expected empty Kubernetes frontend collections to normalize, got %#v", frontend)
	}
}

func TestKubernetesClusterToFrontend_ZeroTokenLastUsedAtOmitted(t *testing.T) {
	zero := time.Time{}
	cluster := KubernetesCluster{
		ID:              "cluster-3",
		Name:            "cluster",
		LastSeen:        time.Now(),
		TokenLastUsedAt: &zero,
	}

	frontend := cluster.ToFrontend()
	if frontend.TokenLastUsedAt != nil {
		t.Fatalf("TokenLastUsedAt = %v, want nil for zero timestamp", frontend.TokenLastUsedAt)
	}

	payload, err := json.Marshal(frontend)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if _, ok := decoded["tokenLastUsedAt"]; ok {
		t.Fatalf("tokenLastUsedAt should be omitted from JSON for zero timestamp")
	}
}

func TestTimeToUnixMillis(t *testing.T) {
	if got := timeToUnixMillis(time.Time{}); got != 0 {
		t.Fatalf("timeToUnixMillis(zero) = %d, want 0", got)
	}

	now := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if got := timeToUnixMillis(now); got != now.Unix()*1000 {
		t.Fatalf("timeToUnixMillis = %d, want %d", got, now.Unix()*1000)
	}
}

func TestConvertResourceToFrontend(t *testing.T) {
	total := int64(100)
	used := int64(60)
	free := int64(40)
	identity := &ResourceIdentityInput{
		Hostname:  "host-a",
		MachineID: "machine-1",
		IPs:       []string{"10.0.0.1"},
	}

	input := ResourceConvertInput{
		ID:           "res-1",
		Type:         "node",
		Name:         "node-a",
		DisplayName:  "Node A",
		Status:       "healthy",
		CPU:          &ResourceMetricInput{Current: 12.5, Total: &total, Used: &used, Free: &free},
		Memory:       &ResourceMetricInput{Current: 50.1},
		Disk:         &ResourceMetricInput{Current: 70.2},
		NetworkRX:    1000,
		NetworkTX:    2000,
		HasNetwork:   true,
		Tags:         []string{"prod"},
		Labels:       map[string]string{"env": "prod"},
		LastSeenUnix: 12345,
		Alerts: []ResourceAlertInput{
			{ID: "a1", Type: "cpu", Level: "warn", Message: "high", Value: 90, Threshold: 80, StartTimeUnix: 111},
		},
		Identity:     identity,
		PlatformData: json.RawMessage(`{"key":"value"}`),
	}

	frontend := ConvertResourceToFrontend(input)
	if frontend.ID != input.ID || frontend.Name != input.Name {
		t.Fatalf("frontend = %#v, want ID %q Name %q", frontend, input.ID, input.Name)
	}
	if frontend.CPU == nil || frontend.CPU.Total == nil || *frontend.CPU.Total != total {
		t.Fatalf("CPU = %#v, want total %d", frontend.CPU, total)
	}
	if frontend.Network == nil || frontend.Network.RXBytes != 1000 || frontend.Network.TXBytes != 2000 {
		t.Fatalf("Network = %#v, want RX/TX set", frontend.Network)
	}
	if len(frontend.Alerts) != 1 || frontend.Alerts[0].ID != "a1" {
		t.Fatalf("Alerts = %#v, want 1 alert", frontend.Alerts)
	}
	if frontend.Identity == nil || frontend.Identity.Hostname != identity.Hostname {
		t.Fatalf("Identity = %#v, want hostname %q", frontend.Identity, identity.Hostname)
	}
}

func TestConvertResourceToFrontend_UsesCanonicalEmptyCollections(t *testing.T) {
	frontend := ConvertResourceToFrontend(ResourceConvertInput{
		ID:           "res-2",
		Type:         "node",
		Name:         "node-b",
		DisplayName:  "Node B",
		Status:       "healthy",
		LastSeenUnix: 123,
		Identity:     &ResourceIdentityInput{},
	})

	if frontend.Tags == nil || frontend.Labels == nil || frontend.Alerts == nil {
		t.Fatalf("expected resource collections to normalize, got %#v", frontend)
	}
	if frontend.Identity == nil || frontend.Identity.IPs == nil {
		t.Fatalf("expected resource identity collections to normalize, got %#v", frontend.Identity)
	}
}

func TestNodeToFrontend_UsesCanonicalEmptyCollections(t *testing.T) {
	node := Node{
		ID:       "node-1",
		Name:     "pve1",
		Instance: "default",
		Status:   "online",
		Type:     "node",
	}

	frontend := node.ToFrontend()
	if frontend.LoadAverage == nil {
		t.Fatalf("expected node loadAverage to normalize, got %#v", frontend)
	}
}

func TestDockerHostToFrontend_UsesCanonicalEmptyCollections(t *testing.T) {
	host := DockerHost{
		ID:       "docker-1",
		AgentID:  "agent-1",
		Hostname: "tower",
		LastSeen: time.Now(),
	}

	frontend := host.ToFrontend()
	if frontend.Containers == nil || frontend.Services == nil || frontend.Tasks == nil {
		t.Fatalf("expected Docker host frontend collections to normalize, got %#v", frontend)
	}
}

func TestHostToFrontend_UsesCanonicalEmptyCollections(t *testing.T) {
	host := Host{
		ID:       "host-1",
		Hostname: "server1",
		Status:   "online",
		LastSeen: time.Now(),
		Sensors: HostSensorSummary{
			TemperatureCelsius: map[string]float64{"cpu": 55},
		},
	}

	frontend := host.ToFrontend()
	if frontend.LoadAverage == nil || frontend.Disks == nil || frontend.DiskIO == nil || frontend.NetworkInterfaces == nil || frontend.Tags == nil {
		t.Fatalf("expected host frontend collections to normalize, got %#v", frontend)
	}
	if frontend.Sensors == nil || frontend.Sensors.FanRPM == nil || frontend.Sensors.Additional == nil || frontend.Sensors.SMART == nil {
		t.Fatalf("expected host sensor collections to normalize, got %#v", frontend.Sensors)
	}
}

func TestVMAndContainerToFrontend_UsesCanonicalEmptyCollections(t *testing.T) {
	vm := VM{
		ID:       "vm-1",
		VMID:     101,
		Name:     "vm-a",
		Node:     "pve1",
		Instance: "default",
		Status:   "running",
		Type:     "qemu",
	}
	container := Container{
		ID:       "ct-1",
		VMID:     201,
		Name:     "ct-a",
		Node:     "pve1",
		Instance: "default",
		Status:   "running",
		Type:     "lxc",
	}

	vmFrontend := vm.ToFrontend()
	if vmFrontend.Disks == nil || vmFrontend.NetworkInterfaces == nil || vmFrontend.IPAddresses == nil {
		t.Fatalf("expected VM frontend collections to normalize, got %#v", vmFrontend)
	}

	containerFrontend := container.ToFrontend()
	if containerFrontend.Disks == nil || containerFrontend.NetworkInterfaces == nil || containerFrontend.IPAddresses == nil {
		t.Fatalf("expected container frontend collections to normalize, got %#v", containerFrontend)
	}
}

func TestStorageToFrontend_UsesCanonicalEmptyCollections(t *testing.T) {
	frontend := Storage{
		ID:       "storage-1",
		Name:     "local",
		Node:     "pve1",
		Instance: "default",
		Type:     "dir",
		Status:   "available",
	}.ToFrontend()

	if frontend.Nodes == nil || frontend.NodeIDs == nil {
		t.Fatalf("expected storage frontend collections to normalize, got %#v", frontend)
	}
}

func TestDockerHostToFrontend_NormalizesNestedCollections(t *testing.T) {
	host := DockerHost{
		ID:       "docker-2",
		AgentID:  "agent-2",
		Hostname: "tower",
		LastSeen: time.Now(),
		Containers: []DockerContainer{
			{
				ID:        "container-1",
				Name:      "pbs",
				Image:     "pbs:latest",
				State:     "running",
				Status:    "Up",
				CreatedAt: time.Now(),
			},
		},
		Services: []DockerService{
			{
				ID:   "service-1",
				Name: "web",
			},
		},
	}

	frontend := host.ToFrontend()
	if frontend.Containers[0].Ports == nil || frontend.Containers[0].Labels == nil || frontend.Containers[0].Networks == nil || frontend.Containers[0].Mounts == nil {
		t.Fatalf("expected Docker container nested collections to normalize, got %#v", frontend.Containers[0])
	}
	if frontend.Services[0].Labels == nil || frontend.Services[0].EndpointPorts == nil {
		t.Fatalf("expected Docker service nested collections to normalize, got %#v", frontend.Services[0])
	}
}

func TestDockerContainerAndServiceFrontend_JSONUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(DockerHostFrontend{
		ID:         "docker-1",
		AgentID:    "agent-1",
		Hostname:   "tower",
		Status:     "healthy",
		LastSeen:   1,
		Containers: []DockerContainerFrontend{{ID: "container-1", Name: "pbs", Image: "pbs:latest", State: "running", Status: "Up", CreatedAt: 1}},
		Services:   []DockerServiceFrontend{{ID: "service-1", Name: "web"}},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	containers, ok := decoded["containers"].([]any)
	if !ok || len(containers) != 1 {
		t.Fatalf("containers = %#v, want one container", decoded["containers"])
	}
	container, ok := containers[0].(map[string]any)
	if !ok {
		t.Fatalf("container = %#v, want object", containers[0])
	}
	for _, key := range []string{"ports", "labels", "networks", "mounts"} {
		if _, ok := container[key]; !ok {
			t.Fatalf("container JSON missing %q in canonical empty shape: %#v", key, container)
		}
	}

	services, ok := decoded["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("services = %#v, want one service", decoded["services"])
	}
	service, ok := services[0].(map[string]any)
	if !ok {
		t.Fatalf("service = %#v, want object", services[0])
	}
	for _, key := range []string{"labels", "endpointPorts"} {
		if _, ok := service[key]; !ok {
			t.Fatalf("service JSON missing %q in canonical empty shape: %#v", key, service)
		}
	}
}

func TestKubernetesClusterToFrontend_NormalizesNestedCollections(t *testing.T) {
	cluster := KubernetesCluster{
		ID: "cluster-4",
		Nodes: []KubernetesNode{
			{UID: "node-1", Name: "node-a"},
		},
		Pods: []KubernetesPod{
			{UID: "pod-1", Name: "pod-a", Namespace: "default"},
		},
		Deployments: []KubernetesDeployment{
			{UID: "deploy-1", Name: "deploy-a", Namespace: "default"},
		},
	}

	frontend := cluster.ToFrontend()
	if frontend.Nodes[0].Roles == nil {
		t.Fatalf("expected node roles to normalize, got %#v", frontend.Nodes[0])
	}
	if frontend.Pods[0].Labels == nil || frontend.Pods[0].Containers == nil {
		t.Fatalf("expected pod nested collections to normalize, got %#v", frontend.Pods[0])
	}
	if frontend.Deployments[0].Labels == nil {
		t.Fatalf("expected deployment labels to normalize, got %#v", frontend.Deployments[0])
	}
}

func TestKubernetesClusterFrontend_JSONUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(KubernetesClusterFrontend{
		ID:      "cluster-1",
		AgentID: "agent-1",
		Status:  "healthy",
		Nodes: []KubernetesNodeFrontend{
			{UID: "node-1", Name: "node-a"},
		},
		Pods: []KubernetesPodFrontend{
			{UID: "pod-1", Name: "pod-a", Namespace: "default"},
		},
		Deployments: []KubernetesDeploymentFrontend{
			{UID: "deploy-1", Name: "deploy-a", Namespace: "default"},
		},
	}.NormalizeCollections())
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	nodes, ok := decoded["nodes"].([]any)
	if !ok || len(nodes) != 1 {
		t.Fatalf("nodes = %#v, want one node", decoded["nodes"])
	}
	node, ok := nodes[0].(map[string]any)
	if !ok {
		t.Fatalf("node = %#v, want object", nodes[0])
	}
	if _, ok := node["roles"]; !ok {
		t.Fatalf("node JSON missing roles in canonical empty shape: %#v", node)
	}

	pods, ok := decoded["pods"].([]any)
	if !ok || len(pods) != 1 {
		t.Fatalf("pods = %#v, want one pod", decoded["pods"])
	}
	pod, ok := pods[0].(map[string]any)
	if !ok {
		t.Fatalf("pod = %#v, want object", pods[0])
	}
	for _, key := range []string{"labels", "containers"} {
		if _, ok := pod[key]; !ok {
			t.Fatalf("pod JSON missing %q in canonical empty shape: %#v", key, pod)
		}
	}

	deployments, ok := decoded["deployments"].([]any)
	if !ok || len(deployments) != 1 {
		t.Fatalf("deployments = %#v, want one deployment", decoded["deployments"])
	}
	deployment, ok := deployments[0].(map[string]any)
	if !ok {
		t.Fatalf("deployment = %#v, want object", deployments[0])
	}
	if _, ok := deployment["labels"]; !ok {
		t.Fatalf("deployment JSON missing labels in canonical empty shape: %#v", deployment)
	}
}

func TestHostResourceAndStorageFrontend_JSONUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"host": HostFrontend{
			ID:          "host-1",
			Hostname:    "server1",
			DisplayName: "Server 1",
			Status:      "online",
			LastSeen:    1,
			Sensors:     &HostSensorSummaryFrontend{TemperatureCelsius: map[string]float64{"cpu": 50}},
		}.NormalizeCollections(),
		"storage": StorageFrontend{
			ID:       "storage-1",
			Storage:  "local",
			Name:     "local",
			Node:     "pve1",
			Instance: "default",
			Type:     "dir",
			Status:   "available",
		}.NormalizeCollections(),
		"resource": ResourceFrontend{
			ID:           "res-1",
			Type:         "node",
			Name:         "node-a",
			DisplayName:  "Node A",
			PlatformID:   "platform-1",
			PlatformType: "proxmox",
			SourceType:   "agent",
			Status:       "healthy",
			LastSeen:     1,
			Identity:     &ResourceIdentityFrontend{},
		}.NormalizeCollections(),
	})
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	host := decoded["host"].(map[string]any)
	for _, key := range []string{"loadAverage", "disks", "diskIO", "networkInterfaces", "tags"} {
		if _, ok := host[key]; !ok {
			t.Fatalf("host JSON missing %q in canonical empty shape: %#v", key, host)
		}
	}
	sensors := host["sensors"].(map[string]any)
	for _, key := range []string{"temperatureCelsius", "fanRpm", "additional", "smart"} {
		if _, ok := sensors[key]; !ok {
			t.Fatalf("host sensors JSON missing %q in canonical empty shape: %#v", key, sensors)
		}
	}

	storage := decoded["storage"].(map[string]any)
	for _, key := range []string{"nodes", "nodeIds"} {
		if _, ok := storage[key]; !ok {
			t.Fatalf("storage JSON missing %q in canonical empty shape: %#v", key, storage)
		}
	}

	resource := decoded["resource"].(map[string]any)
	for _, key := range []string{"tags", "labels", "alerts"} {
		if _, ok := resource[key]; !ok {
			t.Fatalf("resource JSON missing %q in canonical empty shape: %#v", key, resource)
		}
	}
	identity := resource["identity"].(map[string]any)
	if _, ok := identity["ips"]; !ok {
		t.Fatalf("resource identity JSON missing ips in canonical empty shape: %#v", identity)
	}
}

func TestGuestAndDockerHostFrontend_JSONUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"vm": VMFrontend{
			ID:       "vm-1",
			VMID:     101,
			Name:     "vm-a",
			Node:     "pve1",
			Instance: "default",
			Status:   "running",
			Type:     "qemu",
		}.NormalizeCollections(),
		"container": ContainerFrontend{
			ID:       "ct-1",
			VMID:     201,
			Name:     "ct-a",
			Node:     "pve1",
			Instance: "default",
			Status:   "running",
			Type:     "lxc",
		}.NormalizeCollections(),
		"dockerHost": DockerHostFrontend{
			ID:          "docker-1",
			AgentID:     "agent-1",
			Hostname:    "tower",
			DisplayName: "Tower",
			Runtime:     "docker",
			Status:      "healthy",
			LastSeen:    1,
		}.NormalizeCollections(),
	})
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	vm := decoded["vm"].(map[string]any)
	for _, key := range []string{"disks", "networkInterfaces", "ipAddresses"} {
		if _, ok := vm[key]; !ok {
			t.Fatalf("vm JSON missing %q in canonical empty shape: %#v", key, vm)
		}
	}

	container := decoded["container"].(map[string]any)
	for _, key := range []string{"disks", "networkInterfaces", "ipAddresses"} {
		if _, ok := container[key]; !ok {
			t.Fatalf("container JSON missing %q in canonical empty shape: %#v", key, container)
		}
	}

	dockerHost := decoded["dockerHost"].(map[string]any)
	for _, key := range []string{"loadAverage", "disks", "networkInterfaces", "containers", "services", "tasks"} {
		if _, ok := dockerHost[key]; !ok {
			t.Fatalf("docker host JSON missing %q in canonical empty shape: %#v", key, dockerHost)
		}
	}
}

func TestNodeAndConnectedInfrastructureFrontend_JSONUsesCanonicalEmptyCollections(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"node": NodeFrontend{
			ID:          "node-1",
			Node:        "pve1",
			Name:        "pve1",
			DisplayName: "PVE 1",
			Instance:    "default",
			Status:      "online",
			Type:        "node",
			CPUInfo:     CPUInfo{},
		}.NormalizeCollections(),
		"state": StateFrontend{
			ConnectedInfrastructure: []ConnectedInfrastructureItemFrontend{{
				ID:     "infra-1",
				Name:   "tower",
				Status: "online",
			}},
		}.NormalizeCollections(),
	})
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	node := decoded["node"].(map[string]any)
	if _, ok := node["loadAverage"]; !ok {
		t.Fatalf("node JSON missing loadAverage in canonical empty shape: %#v", node)
	}

	state := decoded["state"].(map[string]any)
	connected := state["connectedInfrastructure"].([]any)
	if len(connected) != 1 {
		t.Fatalf("connectedInfrastructure = %#v, want one item", connected)
	}
	item := connected[0].(map[string]any)
	if _, ok := item["surfaces"]; !ok {
		t.Fatalf("connected infrastructure JSON missing surfaces in canonical empty shape: %#v", item)
	}
}
