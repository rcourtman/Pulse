package dockeragent

import (
	"context"
	"errors"
	"testing"
	"time"

	swarmtypes "github.com/docker/docker/api/types/swarm"
	systemtypes "github.com/docker/docker/api/types/system"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func TestResolvedSwarmScope(t *testing.T) {
	info := systemtypes.Info{
		Swarm: swarmtypes.Info{
			ControlAvailable: true,
		},
	}

	agent := &Agent{cfg: Config{SwarmScope: swarmScopeAuto}}
	if got := agent.resolvedSwarmScope(info); got != swarmScopeCluster {
		t.Fatalf("expected cluster scope, got %q", got)
	}

	agent.cfg.SwarmScope = swarmScopeNode
	if got := agent.resolvedSwarmScope(info); got != swarmScopeNode {
		t.Fatalf("expected node scope, got %q", got)
	}

	info.Swarm.ControlAvailable = false
	agent.cfg.SwarmScope = swarmScopeAuto
	if got := agent.resolvedSwarmScope(info); got != swarmScopeNode {
		t.Fatalf("expected node scope, got %q", got)
	}

	agent.cfg.SwarmScope = "unknown"
	if got := agent.resolvedSwarmScope(info); got != swarmScopeNode {
		t.Fatalf("expected fallback node scope, got %q", got)
	}
}

func TestMapSwarmService(t *testing.T) {
	now := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	updateAt := now.Add(2 * time.Minute)

	svc := &swarmtypes.Service{
		ID: "svc1",
		Spec: swarmtypes.ServiceSpec{
			Annotations: swarmtypes.Annotations{
				Name: "web",
				Labels: map[string]string{
					"com.docker.stack.namespace": "stack",
				},
			},
			Mode: swarmtypes.ServiceMode{Replicated: &swarmtypes.ReplicatedService{}},
			TaskTemplate: swarmtypes.TaskSpec{
				ContainerSpec: &swarmtypes.ContainerSpec{
					Image: "nginx:latest",
				},
			},
		},
		ServiceStatus: &swarmtypes.ServiceStatus{
			DesiredTasks:   3,
			RunningTasks:   2,
			CompletedTasks: 1,
		},
		UpdateStatus: &swarmtypes.UpdateStatus{
			State:       swarmtypes.UpdateStateCompleted,
			Message:     "done",
			CompletedAt: &updateAt,
		},
		Endpoint: swarmtypes.Endpoint{
			Ports: []swarmtypes.PortConfig{
				{Name: "http", Protocol: swarmtypes.PortConfigProtocolTCP, TargetPort: 80, PublishedPort: 8080, PublishMode: swarmtypes.PortConfigPublishModeIngress},
			},
		},
		Meta: swarmtypes.Meta{
			CreatedAt: now,
			UpdatedAt: updateAt,
		},
	}

	got := mapSwarmService(svc)
	if got.ID != "svc1" || got.Name != "web" || got.Mode == "" {
		t.Fatalf("unexpected service mapping: %+v", got)
	}
	if got.Stack != "stack" {
		t.Fatalf("expected stack label to be mapped, got %q", got.Stack)
	}
	if got.Image != "nginx:latest" {
		t.Fatalf("expected image to be mapped, got %q", got.Image)
	}
	if got.DesiredTasks != 3 || got.RunningTasks != 2 || got.CompletedTasks != 1 {
		t.Fatalf("unexpected task counts: %+v", got)
	}
	if got.UpdateStatus == nil || got.UpdateStatus.State != string(swarmtypes.UpdateStateCompleted) {
		t.Fatalf("expected update status to be mapped, got %+v", got.UpdateStatus)
	}
	if got.EndpointPorts == nil || len(got.EndpointPorts) != 1 {
		t.Fatalf("expected endpoint ports to be mapped")
	}
	if got.CreatedAt == nil || got.UpdatedAt == nil {
		t.Fatalf("expected timestamps to be mapped")
	}
}

func TestMapSwarmTask(t *testing.T) {
	now := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	containerStart := now.Add(-2 * time.Minute)
	containerFinish := now.Add(-time.Minute)

	containers := map[string]agentsdocker.Container{
		"container-full": {
			ID:        "container-full",
			Name:      "web.1",
			StartedAt: &containerStart,
			FinishedAt: func() *time.Time {
				val := containerFinish
				return &val
			}(),
		},
	}

	t.Run("running task", func(t *testing.T) {
		task := &swarmtypes.Task{
			ID:        "task1",
			ServiceID: "svc1",
			Slot:      1,
			NodeID:    "node1",
			Status: swarmtypes.TaskStatus{
				State:     swarmtypes.TaskStateRunning,
				Timestamp: now,
				ContainerStatus: &swarmtypes.ContainerStatus{
					ContainerID: "container-full",
				},
			},
			Meta: swarmtypes.Meta{
				CreatedAt: now.Add(-time.Minute),
			},
		}
		svc := &swarmtypes.Service{Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "web"}}}

		got := mapSwarmTask(task, svc, containers)
		if got.ServiceName != "web" {
			t.Fatalf("expected service name, got %q", got.ServiceName)
		}
		if got.ContainerName != "web.1" {
			t.Fatalf("expected container name, got %q", got.ContainerName)
		}
		if got.StartedAt == nil || !got.StartedAt.Equal(now) {
			t.Fatalf("expected started at timestamp from task")
		}
	})

	t.Run("completed task uses container timestamps", func(t *testing.T) {
		task := &swarmtypes.Task{
			ID:        "task2",
			ServiceID: "svc2",
			Status: swarmtypes.TaskStatus{
				State: swarmtypes.TaskStateComplete,
				ContainerStatus: &swarmtypes.ContainerStatus{
					ContainerID: "container-full",
				},
			},
			Meta: swarmtypes.Meta{
				CreatedAt: now.Add(-time.Hour),
			},
		}

		got := mapSwarmTask(task, nil, containers)
		if got.CompletedAt == nil || !got.CompletedAt.Equal(containerFinish) {
			t.Fatalf("expected completed at from container timestamp")
		}
		if got.StartedAt == nil || !got.StartedAt.Equal(containerStart) {
			t.Fatalf("expected started at from container timestamp")
		}
	})

	t.Run("updated at and completed timestamp", func(t *testing.T) {
		updated := now.Add(time.Minute)
		task := &swarmtypes.Task{
			ID:        "task3",
			ServiceID: "svc3",
			Status: swarmtypes.TaskStatus{
				State:     swarmtypes.TaskStateFailed,
				Timestamp: now,
			},
			Meta: swarmtypes.Meta{
				CreatedAt: now.Add(-time.Hour),
				UpdatedAt: updated,
			},
		}
		svc := &swarmtypes.Service{Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "api"}}}

		got := mapSwarmTask(task, svc, nil)
		if got.UpdatedAt == nil || !got.UpdatedAt.Equal(updated) {
			t.Fatalf("expected updated at to be set")
		}
		if got.CompletedAt == nil || !got.CompletedAt.Equal(now) {
			t.Fatalf("expected completed at from task timestamp")
		}
	})
}

func TestCollectSwarmDataFromManager(t *testing.T) {
	agent := &Agent{
		docker: &fakeDockerClient{
			serviceListFn: func(_ context.Context, _ swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error) {
				return []swarmtypes.Service{
					{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
					{ID: "svc2", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "beta"}}},
				}, nil
			},
			taskListFn: func(_ context.Context, opts swarmtypes.TaskListOptions) ([]swarmtypes.Task, error) {
				if got := opts.Filters.Get("node"); len(got) != 1 || got[0] != "node1" {
					t.Fatalf("expected node filter to include node1, got %v", got)
				}
				return []swarmtypes.Task{
					{ID: "task1", ServiceID: "svc1", Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
				}, nil
			},
		},
	}

	info := systemtypes.Info{
		Swarm: swarmtypes.Info{
			NodeID: "node1",
		},
	}

	services, tasks, err := agent.collectSwarmDataFromManager(context.Background(), info, swarmScopeNode, nil, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if len(services) != 1 {
		t.Fatalf("expected filtered services, got %d", len(services))
	}

	t.Run("task list error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				serviceListFn: func(_ context.Context, _ swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
					}, nil
				},
				taskListFn: func(_ context.Context, _ swarmtypes.TaskListOptions) ([]swarmtypes.Task, error) {
					return nil, errors.New("task failed")
				},
			},
		}

		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID: "node1",
			},
		}

		services, tasks, err := agent.collectSwarmDataFromManager(context.Background(), info, swarmScopeNode, nil, true, true)
		if err == nil {
			t.Fatal("expected error")
		}
		if len(services) != 1 || tasks != nil {
			t.Fatalf("expected services only on task error")
		}
	})
}

func TestCollectSwarmData(t *testing.T) {
	t.Run("unsupported swarm returns nils", func(t *testing.T) {
		agent := &Agent{supportsSwarm: false}
		services, tasks, info := agent.collectSwarmData(context.Background(), systemtypes.Info{}, nil)
		if services != nil || tasks != nil || info != nil {
			t.Fatal("expected nil outputs when swarm unsupported")
		}
	})

	t.Run("empty swarm info returns nil", func(t *testing.T) {
		agent := &Agent{supportsSwarm: true}
		services, tasks, info := agent.collectSwarmData(context.Background(), systemtypes.Info{}, nil)
		if services != nil || tasks != nil || info != nil {
			t.Fatal("expected nil outputs for empty swarm info")
		}
	})

	t.Run("inactive swarm returns info only", func(t *testing.T) {
		agent := &Agent{supportsSwarm: true, cfg: Config{SwarmScope: swarmScopeNode}}
		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID:         "node1",
				LocalNodeState: swarmtypes.LocalNodeStatePending,
			},
		}

		services, tasks, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if services != nil || tasks != nil {
			t.Fatal("expected nil services/tasks for inactive swarm")
		}
		if swarmInfo == nil || swarmInfo.NodeID != "node1" {
			t.Fatal("expected swarm info to be returned")
		}
	})

	t.Run("manager error falls back to containers", func(t *testing.T) {
		agent := &Agent{
			supportsSwarm: true,
			cfg: Config{
				IncludeServices: true,
				IncludeTasks:    true,
				SwarmScope:      swarmScopeAuto,
			},
			docker: &fakeDockerClient{
				serviceListFn: func(context.Context, swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error) {
					return nil, errors.New("boom")
				},
			},
		}

		containers := []agentsdocker.Container{
			{
				ID:    "container1",
				Name:  "web.1",
				Image: "nginx:latest",
				State: "running",
				Labels: map[string]string{
					"com.docker.swarm.service.id":    "svc1",
					"com.docker.swarm.service.name":  "web",
					"com.docker.swarm.task.id":       "task1",
					"com.docker.swarm.task.slot":     "1",
					"com.docker.swarm.task.message":  "ok",
					"com.docker.swarm.task.error":    "",
					"com.docker.stack.namespace":     "stack",
					"com.docker.swarm.node.id":       "node1",
					"com.docker.swarm.node.name":     "node1",
					"com.docker.swarm.task.desired-state": "running",
				},
			},
		}

		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID:           "node1",
				ControlAvailable: true,
				LocalNodeState:   swarmtypes.LocalNodeStateActive,
			},
		}

		services, tasks, swarmInfo := agent.collectSwarmData(context.Background(), info, containers)
		if len(tasks) != 1 || len(services) != 1 {
			t.Fatalf("expected derived tasks/services, got %d/%d", len(tasks), len(services))
		}
		if swarmInfo == nil || swarmInfo.Scope != swarmScopeNode {
			t.Fatalf("expected effective scope node, got %+v", swarmInfo)
		}
	})

	t.Run("manager success uses manager data", func(t *testing.T) {
		agent := &Agent{
			supportsSwarm: true,
			cfg: Config{
				IncludeServices: true,
				IncludeTasks:    true,
				SwarmScope:      swarmScopeCluster,
			},
			docker: &fakeDockerClient{
				serviceListFn: func(context.Context, swarmtypes.ServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "zeta"}}},
						{ID: "svc2", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
					}, nil
				},
				taskListFn: func(context.Context, swarmtypes.TaskListOptions) ([]swarmtypes.Task, error) {
					return []swarmtypes.Task{
						{ID: "task2", ServiceID: "svc2", Slot: 2},
						{ID: "task1", ServiceID: "svc2", Slot: 1},
					}, nil
				},
			},
		}

		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID:           "node1",
				ControlAvailable: true,
				LocalNodeState:   swarmtypes.LocalNodeStateActive,
			},
		}

		services, tasks, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if len(tasks) != 2 || len(services) != 2 {
			t.Fatalf("expected manager tasks/services, got %d/%d", len(tasks), len(services))
		}
		if swarmInfo == nil || swarmInfo.Scope != swarmScopeCluster {
			t.Fatalf("unexpected swarm info: %+v", swarmInfo)
		}
	})

	t.Run("include flags prune outputs", func(t *testing.T) {
		agent := &Agent{
			supportsSwarm: true,
			cfg: Config{
				IncludeServices: false,
				IncludeTasks:    true,
				SwarmScope:      swarmScopeAuto,
			},
		}

		containers := []agentsdocker.Container{
			{
				ID:    "container1",
				Name:  "web.1",
				Image: "nginx:latest",
				State: "running",
				Labels: map[string]string{
					"com.docker.swarm.service.id":   "svc1",
					"com.docker.swarm.service.name": "web",
					"com.docker.swarm.task.id":      "task1",
				},
			},
		}

		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID:           "node1",
				ControlAvailable: false,
				LocalNodeState:   swarmtypes.LocalNodeStateActive,
			},
		}

		services, tasks, swarmInfo := agent.collectSwarmData(context.Background(), info, containers)
		if services != nil {
			t.Fatal("expected services to be nil when disabled")
		}
		if len(tasks) != 1 {
			t.Fatalf("expected tasks to be returned")
		}
		if swarmInfo == nil {
			t.Fatalf("expected swarm info")
		}
	})
}

func TestDeriveSwarmTasksFromContainers(t *testing.T) {
	started := time.Date(2024, 1, 1, 1, 1, 1, 0, time.UTC)
	finished := started.Add(time.Minute)
	containers := []agentsdocker.Container{
		{ID: "no-labels"},
		{
			ID:     "no-service",
			Labels: map[string]string{"com.docker.swarm.task.id": "task1"},
		},
		{
			ID:        "container1",
			Name:      "web.1",
			State:     "running",
			CreatedAt: started,
			Labels: map[string]string{
				"com.docker.swarm.service.id":          "svc1",
				"com.docker.swarm.service.name":        "web",
				"com.docker.swarm.task.slot":           "notint",
				"com.docker.swarm.task.message":        "ok",
				"com.docker.swarm.task.error":          "err",
				"com.docker.swarm.task.desired-state":  "running",
				"com.docker.swarm.task.id":             "",
				"com.docker.swarm.node.name":           "nodeA",
				"com.docker.swarm.node.id":             "",
			},
			StartedAt:  &started,
			FinishedAt: &finished,
		},
		{
			ID:    "container2",
			Name:  "web.2",
			State: "running",
			Labels: map[string]string{
				"com.docker.swarm.service.id":   "svc1",
				"com.docker.swarm.service.name": "web",
				"com.docker.swarm.task.id":      "task2",
				"com.docker.swarm.task.slot":    "2",
			},
		},
	}

	info := systemtypes.Info{
		Swarm: swarmtypes.Info{
			NodeID: "node1",
		},
	}

	tasks := deriveSwarmTasksFromContainers(containers, info)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}

	if tasks[0].NodeID == "" {
		t.Fatalf("expected node id fallback to be set")
	}
	if tasks[0].Slot != 0 {
		t.Fatalf("expected slot to remain 0 for invalid slot")
	}
	if tasks[0].StartedAt == nil || tasks[0].CompletedAt == nil {
		t.Fatalf("expected timestamps to be set from container")
	}
}

func TestDeriveSwarmServicesFromData(t *testing.T) {
	tasks := []agentsdocker.Task{
		{
			ID:           "task1",
			ServiceID:    "svc1",
			ServiceName:  "web",
			CurrentState: "running",
		},
		{
			ID:           "task2",
			ServiceID:    "svc1",
			ServiceName:  "web",
			CurrentState: "complete",
		},
		{
			ID:           "task3",
			ServiceID:    "",
			ServiceName:  "",
			CurrentState: "running",
		},
	}

	containers := []agentsdocker.Container{
		{
			ID:    "container1",
			Image: "nginx:latest",
			Labels: map[string]string{
				"com.docker.swarm.service.id":   "svc1",
				"com.docker.stack.namespace":    "stack",
				"com.docker.swarm.service.name": "web",
			},
		},
	}

	services := deriveSwarmServicesFromData(tasks, containers)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Image != "nginx:latest" {
		t.Fatalf("expected image to be populated")
	}
	if services[0].Stack != "stack" {
		t.Fatalf("expected stack to be populated")
	}

	if got := deriveSwarmServicesFromData(nil, containers); got != nil {
		t.Fatal("expected nil services for empty tasks")
	}
}
