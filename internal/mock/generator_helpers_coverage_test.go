package mock

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestKubernetesDeploymentHealthy(t *testing.T) {
	testCases := []struct {
		name       string
		deployment models.KubernetesDeployment
		want       bool
	}{
		{
			name:       "zero desired replicas is healthy",
			deployment: models.KubernetesDeployment{DesiredReplicas: 0},
			want:       true,
		},
		{
			name:       "ready below desired is unhealthy",
			deployment: models.KubernetesDeployment{DesiredReplicas: 3, ReadyReplicas: 2, AvailableReplicas: 3, UpdatedReplicas: 3},
			want:       false,
		},
		{
			name:       "available below desired is unhealthy",
			deployment: models.KubernetesDeployment{DesiredReplicas: 3, ReadyReplicas: 3, AvailableReplicas: 2, UpdatedReplicas: 3},
			want:       false,
		},
		{
			name:       "updated below desired is unhealthy",
			deployment: models.KubernetesDeployment{DesiredReplicas: 3, ReadyReplicas: 3, AvailableReplicas: 3, UpdatedReplicas: 2},
			want:       false,
		},
		{
			name:       "all replica counters meeting desired is healthy",
			deployment: models.KubernetesDeployment{DesiredReplicas: 3, ReadyReplicas: 3, AvailableReplicas: 3, UpdatedReplicas: 3},
			want:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := kubernetesDeploymentHealthy(tc.deployment); got != tc.want {
				t.Fatalf("kubernetesDeploymentHealthy() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestClusterHasIssuesDetectsNodePodAndDeploymentProblems(t *testing.T) {
	healthyNodes := []models.KubernetesNode{{Name: "node-1", Ready: true, Unschedulable: false}}
	healthyPods := []models.KubernetesPod{{
		Name:     "pod-1",
		Phase:    "Running",
		NodeName: "node-1",
		Containers: []models.KubernetesPodContainer{{
			Name:  "main",
			Ready: true,
			State: "running",
		}},
	}}
	healthyDeployments := []models.KubernetesDeployment{{
		Name:              "dep-1",
		DesiredReplicas:   1,
		ReadyReplicas:     1,
		AvailableReplicas: 1,
		UpdatedReplicas:   1,
	}}

	testCases := []struct {
		name        string
		nodes       []models.KubernetesNode
		pods        []models.KubernetesPod
		deployments []models.KubernetesDeployment
		want        bool
	}{
		{
			name:        "all healthy resources",
			nodes:       healthyNodes,
			pods:        healthyPods,
			deployments: healthyDeployments,
			want:        false,
		},
		{
			name:        "not ready node triggers issue",
			nodes:       []models.KubernetesNode{{Name: "node-1", Ready: false}},
			pods:        healthyPods,
			deployments: healthyDeployments,
			want:        true,
		},
		{
			name:        "unschedulable node triggers issue",
			nodes:       []models.KubernetesNode{{Name: "node-1", Ready: true, Unschedulable: true}},
			pods:        healthyPods,
			deployments: healthyDeployments,
			want:        true,
		},
		{
			name:        "non-running pod triggers issue",
			nodes:       healthyNodes,
			pods:        []models.KubernetesPod{{Name: "pod-1", Phase: "Pending", NodeName: "node-1"}},
			deployments: healthyDeployments,
			want:        true,
		},
		{
			name:  "not-ready pod container triggers issue",
			nodes: healthyNodes,
			pods: []models.KubernetesPod{{
				Name:     "pod-1",
				Phase:    "Running",
				NodeName: "node-1",
				Containers: []models.KubernetesPodContainer{{
					Name:  "main",
					Ready: false,
					State: "running",
				}},
			}},
			deployments: healthyDeployments,
			want:        true,
		},
		{
			name:        "unhealthy deployment triggers issue",
			nodes:       healthyNodes,
			pods:        healthyPods,
			deployments: []models.KubernetesDeployment{{Name: "dep-1", DesiredReplicas: 1, ReadyReplicas: 0, AvailableReplicas: 1, UpdatedReplicas: 1}},
			want:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clusterHasIssues(tc.nodes, tc.pods, tc.deployments); got != tc.want {
				t.Fatalf("clusterHasIssues() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeMockKubernetesNodeCapacityDefaultsAndBounds(t *testing.T) {
	normalizeMockKubernetesNodeCapacity(nil)

	node := &models.KubernetesNode{
		CapacityCPU:         0,
		AllocCPU:            10,
		CapacityMemoryBytes: 0,
		AllocMemoryBytes:    -1,
		CapacityPods:        0,
		AllocPods:           999,
	}

	normalizeMockKubernetesNodeCapacity(node)

	if node.CapacityCPU != 4 || node.AllocCPU != 4 {
		t.Fatalf("expected cpu defaults and clamp to 4, got capacity=%d alloc=%d", node.CapacityCPU, node.AllocCPU)
	}
	wantMem := int64(16) * 1024 * 1024 * 1024
	if node.CapacityMemoryBytes != wantMem || node.AllocMemoryBytes != wantMem {
		t.Fatalf("expected memory defaults and clamp to %d, got capacity=%d alloc=%d", wantMem, node.CapacityMemoryBytes, node.AllocMemoryBytes)
	}
	if node.CapacityPods != 110 || node.AllocPods != 110 {
		t.Fatalf("expected pod defaults and clamp to 110, got capacity=%d alloc=%d", node.CapacityPods, node.AllocPods)
	}
}

func TestMockKubernetesPodIsActiveConditions(t *testing.T) {
	testCases := []struct {
		name string
		pod  *models.KubernetesPod
		node *models.KubernetesNode
		want bool
	}{
		{name: "nil pod", pod: nil, node: nil, want: false},
		{name: "non-running pod", pod: &models.KubernetesPod{Phase: "Pending", NodeName: "node-1"}, want: false},
		{name: "running pod without node assignment", pod: &models.KubernetesPod{Phase: "Running", NodeName: ""}, want: false},
		{name: "node not ready", pod: &models.KubernetesPod{Phase: "Running", NodeName: "node-1"}, node: &models.KubernetesNode{Ready: false}, want: false},
		{name: "running pod on ready node", pod: &models.KubernetesPod{Phase: "Running", NodeName: "node-1"}, node: &models.KubernetesNode{Ready: true}, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mockKubernetesPodIsActive(tc.pod, tc.node); got != tc.want {
				t.Fatalf("mockKubernetesPodIsActive() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestApplyMockKubernetesPodZeroUsageResetsUsageFields(t *testing.T) {
	applyMockKubernetesPodZeroUsage(nil)

	pod := &models.KubernetesPod{
		UsageCPUMilliCores:            250,
		UsageMemoryBytes:              1024,
		UsageCPUPercent:               20,
		UsageMemoryPercent:            10,
		NetInRate:                     12,
		NetOutRate:                    13,
		DiskUsagePercent:              50,
		EphemeralStorageUsedBytes:     2048,
		EphemeralStorageCapacityBytes: 4096,
	}

	applyMockKubernetesPodZeroUsage(pod)

	if pod.UsageCPUMilliCores != 0 || pod.UsageMemoryBytes != 0 || pod.UsageCPUPercent != 0 || pod.UsageMemoryPercent != 0 {
		t.Fatalf("expected cpu/memory usage fields to be zeroed, got %+v", pod)
	}
	if pod.NetInRate != 0 || pod.NetOutRate != 0 || pod.DiskUsagePercent != 0 || pod.EphemeralStorageUsedBytes != 0 {
		t.Fatalf("expected network/disk usage fields to be zeroed, got %+v", pod)
	}
	if pod.EphemeralStorageCapacityBytes != 4096 {
		t.Fatalf("expected capacity field to remain unchanged, got %d", pod.EphemeralStorageCapacityBytes)
	}
}

func TestServiceKeyUsesIDWhenPresent(t *testing.T) {
	if got := serviceKey("svc-id", "svc-name"); got != "svc-id" {
		t.Fatalf("expected service id to be preferred, got %q", got)
	}
	if got := serviceKey("", "svc-name"); got != "svc-name" {
		t.Fatalf("expected service name fallback, got %q", got)
	}
}

func TestRecalculateDockerServiceHealthHandlesStateTransitionsAndServiceAggregation(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	started := now.Add(-time.Minute)
	finished := now.Add(-30 * time.Second)

	runningContainerID := "container-running-1234567890"
	pausedContainerID := "container-paused-1234567890"
	exitedContainerID := "container-exited-1234567890"

	host := &models.DockerHost{
		Status: "online",
		Containers: []models.DockerContainer{
			{ID: runningContainerID, State: "running", StartedAt: &started},
			{ID: pausedContainerID, State: "paused", StartedAt: &started},
			{ID: exitedContainerID, State: "exited", FinishedAt: &finished},
		},
		Tasks: []models.DockerTask{
			{ID: "task-fallback", ServiceName: "svc-fallback", ContainerID: runningContainerID[:12]},
			{ID: "task-paused", ServiceID: "svc-low", ContainerID: pausedContainerID[:12]},
			{ID: "task-exited", ServiceID: "svc-low", ContainerID: exitedContainerID[:12]},
			{ID: "task-recovered", ServiceID: "svc-recovered", ContainerID: runningContainerID[:12]},
		},
		Services: []models.DockerService{
			{ID: "", Name: "svc-fallback", DesiredTasks: 0},
			{ID: "svc-low", Name: "svc-low", DesiredTasks: 2},
			{ID: "svc-recovered", Name: "svc-recovered", DesiredTasks: 1, UpdateStatus: &models.DockerServiceUpdate{State: "updating"}},
		},
	}

	recalculateDockerServiceHealth(host, now)

	if host.Tasks[0].CurrentState != "running" {
		t.Fatalf("expected fallback task to be running, got %q", host.Tasks[0].CurrentState)
	}
	if host.Tasks[0].StartedAt == nil || !host.Tasks[0].StartedAt.Equal(started) {
		t.Fatalf("expected running task to copy started-at, got %+v", host.Tasks[0].StartedAt)
	}
	if host.Tasks[0].CompletedAt != nil {
		t.Fatalf("expected running task to clear completed-at")
	}
	if host.Tasks[1].CurrentState != "paused" {
		t.Fatalf("expected paused task state, got %q", host.Tasks[1].CurrentState)
	}
	if host.Tasks[2].CurrentState != "exited" {
		t.Fatalf("expected exited task state, got %q", host.Tasks[2].CurrentState)
	}
	if host.Tasks[2].CompletedAt == nil || !host.Tasks[2].CompletedAt.Equal(finished) {
		t.Fatalf("expected exited task to copy finished-at, got %+v", host.Tasks[2].CompletedAt)
	}

	fallbackService := host.Services[0]
	if fallbackService.DesiredTasks != 1 || fallbackService.RunningTasks != 1 {
		t.Fatalf("expected desired/running task counts for fallback service, got %+v", fallbackService)
	}
	if fallbackService.UpdateStatus != nil {
		t.Fatalf("expected fallback service without update status, got %+v", fallbackService.UpdateStatus)
	}

	lowService := host.Services[1]
	if lowService.RunningTasks != 0 || lowService.CompletedTasks != 1 {
		t.Fatalf("unexpected svc-low task counts: %+v", lowService)
	}
	if lowService.UpdateStatus == nil || lowService.UpdateStatus.State != "rollback_started" {
		t.Fatalf("expected svc-low to enter rollback update status, got %+v", lowService.UpdateStatus)
	}

	recoveredService := host.Services[2]
	if recoveredService.UpdateStatus != nil {
		t.Fatalf("expected recovered service update status to be cleared, got %+v", recoveredService.UpdateStatus)
	}
}

func TestRecalculateDockerServiceHealthOfflineMissingContainerMarksShutdown(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	host := &models.DockerHost{
		Status: "offline",
		Tasks: []models.DockerTask{{
			ID:          "task-1",
			ServiceID:   "svc-offline",
			ServiceName: "svc-offline",
			ContainerID: "missing-container",
		}},
		Services: []models.DockerService{{
			ID:           "svc-offline",
			Name:         "svc-offline",
			DesiredTasks: 0,
		}},
	}

	recalculateDockerServiceHealth(host, now)

	if host.Tasks[0].CurrentState != "shutdown" {
		t.Fatalf("expected missing offline container to mark shutdown, got %q", host.Tasks[0].CurrentState)
	}
	if host.Tasks[0].CompletedAt == nil {
		t.Fatalf("expected missing offline container task to be completed")
	}
	if host.Services[0].DesiredTasks != 1 || host.Services[0].CompletedTasks != 1 {
		t.Fatalf("expected service desired/completed task counts to reflect shutdown task, got %+v", host.Services[0])
	}
}

func TestEnsureDockerSwarmInfoDefaultsAndManagerScope(t *testing.T) {
	ensureDockerSwarmInfo(nil)

	offlineWorker := &models.DockerHost{ID: "host-offline", Status: "offline"}
	ensureDockerSwarmInfo(offlineWorker)

	if offlineWorker.Swarm == nil {
		t.Fatal("expected swarm info to be initialized")
	}
	if offlineWorker.Swarm.NodeID != "host-offline-node" || offlineWorker.Swarm.NodeRole != "worker" {
		t.Fatalf("unexpected default swarm identity: %+v", offlineWorker.Swarm)
	}
	if offlineWorker.Swarm.LocalState != "inactive" || offlineWorker.Swarm.Scope != "node" || offlineWorker.Swarm.ControlAvailable {
		t.Fatalf("unexpected offline worker swarm fields: %+v", offlineWorker.Swarm)
	}

	manager := &models.DockerHost{
		ID:       "host-manager",
		Status:   "online",
		Services: []models.DockerService{{ID: "svc-1"}},
		Swarm: &models.DockerSwarmInfo{
			NodeRole:         "manager",
			ControlAvailable: true,
		},
	}
	ensureDockerSwarmInfo(manager)
	if manager.Swarm.LocalState != "active" || manager.Swarm.Scope != "cluster" {
		t.Fatalf("expected manager with control and services to use cluster scope, got %+v", manager.Swarm)
	}

	workerWithControl := &models.DockerHost{
		ID:     "host-worker",
		Status: "online",
		Swarm: &models.DockerSwarmInfo{
			NodeRole:         "worker",
			ControlAvailable: true,
		},
	}
	ensureDockerSwarmInfo(workerWithControl)
	if workerWithControl.Swarm.Scope != "node" || workerWithControl.Swarm.ControlAvailable {
		t.Fatalf("expected non-manager to force node scope and disable control, got %+v", workerWithControl.Swarm)
	}
}
