package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRemovedDockerHostToFrontend_DisplayName(t *testing.T) {
	removedAt := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	host := RemovedDockerHost{
		ID:          "host-1",
		Hostname:    "docker-1",
		DisplayName: "Docker One",
		RemovedAt:   removedAt,
	}

	frontend := host.ToFrontend()
	if frontend.ID != host.ID {
		t.Fatalf("ID = %q, want %q", frontend.ID, host.ID)
	}
	if frontend.Hostname != host.Hostname {
		t.Fatalf("Hostname = %q, want %q", frontend.Hostname, host.Hostname)
	}
	if frontend.DisplayName != host.DisplayName {
		t.Fatalf("DisplayName = %q, want %q", frontend.DisplayName, host.DisplayName)
	}
	if frontend.RemovedAt != removedAt.Unix()*1000 {
		t.Fatalf("RemovedAt = %d, want %d", frontend.RemovedAt, removedAt.Unix()*1000)
	}
}

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

func TestRemovedKubernetesClusterToFrontend(t *testing.T) {
	removedAt := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	cluster := RemovedKubernetesCluster{
		ID:          "cluster-3",
		Name:        "old",
		DisplayName: "Old Cluster",
		RemovedAt:   removedAt,
	}

	frontend := cluster.ToFrontend()
	if frontend.ID != cluster.ID {
		t.Fatalf("ID = %q, want %q", frontend.ID, cluster.ID)
	}
	if frontend.Name != cluster.Name {
		t.Fatalf("Name = %q, want %q", frontend.Name, cluster.Name)
	}
	if frontend.DisplayName != cluster.DisplayName {
		t.Fatalf("DisplayName = %q, want %q", frontend.DisplayName, cluster.DisplayName)
	}
	if frontend.RemovedAt != removedAt.Unix()*1000 {
		t.Fatalf("RemovedAt = %d, want %d", frontend.RemovedAt, removedAt.Unix()*1000)
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
