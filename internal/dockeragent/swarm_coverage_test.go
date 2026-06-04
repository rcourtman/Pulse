package dockeragent

import (
	"context"
	"errors"
	"testing"
	"time"

	networktypes "github.com/moby/moby/api/types/network"
	swarmtypes "github.com/moby/moby/api/types/swarm"
	systemtypes "github.com/moby/moby/api/types/system"
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
				{Name: "http", Protocol: networktypes.TCP, TargetPort: 80, PublishedPort: 8080, PublishMode: swarmtypes.PortConfigPublishModeIngress},
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
			serviceListFn: func(_ context.Context, _ dockerServiceListOptions) ([]swarmtypes.Service, error) {
				return []swarmtypes.Service{
					{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
					{ID: "svc2", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "beta"}}},
				}, nil
			},
			taskListFn: func(_ context.Context, opts dockerTaskListOptions) ([]swarmtypes.Task, error) {
				if got := opts.Filters.Get("node"); len(got) != 1 || got[0] != "node1" {
					t.Fatalf("expected node filter to include node1, got %v", got)
				}
				if got := opts.Filters.Get("desired-state"); len(got) != 1 || got[0] != string(swarmtypes.TaskStateRunning) {
					t.Fatalf("expected desired-state filter to include running, got %v", got)
				}
				return []swarmtypes.Task{
					{ID: "task1", ServiceID: "svc1", DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
					{ID: "task-old", ServiceID: "svc2", DesiredState: swarmtypes.TaskStateShutdown, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateComplete}},
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
	if tasks[0].ID != "task1" {
		t.Fatalf("expected running task only, got %#v", tasks)
	}
	if len(services) != 1 {
		t.Fatalf("expected filtered services, got %d", len(services))
	}

	t.Run("task list error", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				serviceListFn: func(_ context.Context, _ dockerServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
					}, nil
				},
				taskListFn: func(_ context.Context, _ dockerTaskListOptions) ([]swarmtypes.Task, error) {
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

	t.Run("node scope uses local task counts for services", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				serviceListFn: func(_ context.Context, _ dockerServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{
							ID:   "svc1",
							Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}},
							ServiceStatus: &swarmtypes.ServiceStatus{
								DesiredTasks:   4,
								RunningTasks:   3,
								CompletedTasks: 1,
							},
						},
						{
							ID:   "svc2",
							Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "beta"}},
							ServiceStatus: &swarmtypes.ServiceStatus{
								DesiredTasks: 2,
								RunningTasks: 2,
							},
						},
					}, nil
				},
				taskListFn: func(_ context.Context, _ dockerTaskListOptions) ([]swarmtypes.Task, error) {
					return []swarmtypes.Task{
						{
							ID:           "task1",
							ServiceID:    "svc1",
							DesiredState: swarmtypes.TaskStateRunning,
							Status:       swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning},
						},
						{
							ID:           "task2",
							ServiceID:    "svc1",
							DesiredState: swarmtypes.TaskStateRunning,
							Status:       swarmtypes.TaskStatus{State: swarmtypes.TaskStatePreparing},
						},
					}, nil
				},
			},
		}

		info := systemtypes.Info{Swarm: swarmtypes.Info{NodeID: "node1"}}

		services, tasks, err := agent.collectSwarmDataFromManager(context.Background(), info, swarmScopeNode, nil, true, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tasks) != 2 {
			t.Fatalf("expected 2 local tasks, got %d", len(tasks))
		}
		if len(services) != 1 {
			t.Fatalf("expected only local services, got %d", len(services))
		}
		if services[0].ID != "svc1" {
			t.Fatalf("expected local service svc1, got %+v", services[0])
		}
		if services[0].DesiredTasks != 2 || services[0].RunningTasks != 1 || services[0].CompletedTasks != 0 {
			t.Fatalf("expected node-local task counts, got %+v", services[0])
		}
	})

	t.Run("node scope drops services when no local runtime tasks remain", func(t *testing.T) {
		agent := &Agent{
			docker: &fakeDockerClient{
				serviceListFn: func(_ context.Context, _ dockerServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
						{ID: "svc2", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "beta"}}},
					}, nil
				},
				taskListFn: func(_ context.Context, _ dockerTaskListOptions) ([]swarmtypes.Task, error) {
					return []swarmtypes.Task{}, nil
				},
			},
		}

		info := systemtypes.Info{Swarm: swarmtypes.Info{NodeID: "node1"}}

		services, tasks, err := agent.collectSwarmDataFromManager(context.Background(), info, swarmScopeNode, nil, true, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tasks) != 0 {
			t.Fatalf("expected no local tasks, got %d", len(tasks))
		}
		if len(services) != 0 {
			t.Fatalf("expected no node-scoped services without local tasks, got %d", len(services))
		}
	})
}

func TestMapSwarmNode(t *testing.T) {
	createdAt := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	updatedAt := createdAt.Add(5 * time.Minute)
	node := &swarmtypes.Node{
		ID: "node-1",
		Spec: swarmtypes.NodeSpec{
			Annotations: swarmtypes.Annotations{
				Labels: map[string]string{"zone": "rack-a"},
			},
			Role:         swarmtypes.NodeRoleManager,
			Availability: swarmtypes.NodeAvailabilityActive,
		},
		Description: swarmtypes.NodeDescription{
			Hostname: "manager-1",
			Platform: swarmtypes.Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
			Resources: swarmtypes.Resources{
				NanoCPUs:    4_000_000_000,
				MemoryBytes: 16 * 1024 * 1024 * 1024,
			},
			Engine: swarmtypes.EngineDescription{
				EngineVersion: "27.5.1",
				Labels:        map[string]string{"engine": "primary"},
			},
		},
		Status: swarmtypes.NodeStatus{
			State:   swarmtypes.NodeStateReady,
			Message: "ready",
			Addr:    "192.0.2.10",
		},
		ManagerStatus: &swarmtypes.ManagerStatus{
			Leader:       true,
			Reachability: swarmtypes.ReachabilityReachable,
			Addr:         "192.0.2.10:2377",
		},
		Meta: swarmtypes.Meta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	got := mapSwarmNode(node)
	if got.ID != "node-1" || got.Hostname != "manager-1" || got.Role != string(swarmtypes.NodeRoleManager) {
		t.Fatalf("unexpected node identity: %+v", got)
	}
	if got.Availability != string(swarmtypes.NodeAvailabilityActive) || got.State != string(swarmtypes.NodeStateReady) {
		t.Fatalf("unexpected node state: %+v", got)
	}
	if got.ManagerReachability != string(swarmtypes.ReachabilityReachable) || !got.Leader {
		t.Fatalf("expected manager reachability and leader flag, got %+v", got)
	}
	if got.EngineVersion != "27.5.1" || got.NanoCPUs != 4_000_000_000 || got.MemoryBytes == 0 {
		t.Fatalf("expected engine resources, got %+v", got)
	}
	if got.Labels["zone"] != "rack-a" || got.EngineLabels["engine"] != "primary" {
		t.Fatalf("expected node labels to be mapped, got labels=%+v engine=%+v", got.Labels, got.EngineLabels)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt == nil || got.UpdatedAt.IsZero() {
		t.Fatalf("expected node timestamps, got %+v", got)
	}
}

func TestMapSwarmSecretAndConfigOmitPayloadData(t *testing.T) {
	createdAt := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	updatedAt := createdAt.Add(5 * time.Minute)

	secret := mapSwarmSecret(&swarmtypes.Secret{
		ID: "secret-1",
		Meta: swarmtypes.Meta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		Spec: swarmtypes.SecretSpec{
			Annotations: swarmtypes.Annotations{
				Name:   "db-password",
				Labels: map[string]string{"stack": "backend"},
			},
			Data:       []byte("must-not-be-reported"),
			Driver:     &swarmtypes.Driver{Name: "vault", Options: map[string]string{"path": "secret/db"}},
			Templating: &swarmtypes.Driver{Name: "golang"},
		},
	})
	if secret.ID != "secret-1" || secret.Name != "db-password" || secret.DriverName != "vault" || secret.TemplatingDriver != "golang" {
		t.Fatalf("unexpected mapped secret metadata: %+v", secret)
	}
	if secret.Labels["stack"] != "backend" || secret.CreatedAt.IsZero() || secret.UpdatedAt == nil {
		t.Fatalf("expected secret labels and timestamps, got %+v", secret)
	}

	config := mapSwarmConfig(&swarmtypes.Config{
		ID: "config-1",
		Meta: swarmtypes.Meta{
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		Spec: swarmtypes.ConfigSpec{
			Annotations: swarmtypes.Annotations{
				Name:   "nginx-conf",
				Labels: map[string]string{"stack": "frontend"},
			},
			Data:       []byte("must-not-be-reported"),
			Templating: &swarmtypes.Driver{Name: "golang"},
		},
	})
	if config.ID != "config-1" || config.Name != "nginx-conf" || config.TemplatingDriver != "golang" {
		t.Fatalf("unexpected mapped config metadata: %+v", config)
	}
	if config.Labels["stack"] != "frontend" || config.CreatedAt.IsZero() || config.UpdatedAt == nil {
		t.Fatalf("expected config labels and timestamps, got %+v", config)
	}
}

func TestCollectSwarmData(t *testing.T) {
	t.Run("unsupported swarm returns nils", func(t *testing.T) {
		agent := &Agent{supportsSwarm: false}
		services, tasks, nodes, secrets, configs, info := agent.collectSwarmData(context.Background(), systemtypes.Info{}, nil)
		if services != nil || tasks != nil || nodes != nil || secrets != nil || configs != nil || info != nil {
			t.Fatal("expected nil outputs when swarm unsupported")
		}
	})

	t.Run("empty swarm info returns nil", func(t *testing.T) {
		agent := &Agent{supportsSwarm: true}
		services, tasks, nodes, secrets, configs, info := agent.collectSwarmData(context.Background(), systemtypes.Info{}, nil)
		if services != nil || tasks != nil || nodes != nil || secrets != nil || configs != nil || info != nil {
			t.Fatal("expected nil outputs for empty swarm info")
		}
	})

	t.Run("standalone inactive swarm returns nil", func(t *testing.T) {
		agent := &Agent{supportsSwarm: true, cfg: Config{SwarmScope: swarmScopeNode}}
		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				LocalNodeState: swarmtypes.LocalNodeStateInactive,
			},
		}

		services, tasks, nodes, secrets, configs, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if services != nil || tasks != nil || nodes != nil || secrets != nil || configs != nil || swarmInfo != nil {
			t.Fatal("expected standalone inactive swarm state to be ignored")
		}
	})

	t.Run("pending swarm returns info only", func(t *testing.T) {
		agent := &Agent{supportsSwarm: true, cfg: Config{SwarmScope: swarmScopeNode}}
		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID:         "node1",
				LocalNodeState: swarmtypes.LocalNodeStatePending,
			},
		}

		services, tasks, nodes, secrets, configs, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if services != nil || tasks != nil || nodes != nil || secrets != nil || configs != nil {
			t.Fatal("expected nil services/tasks for pending swarm")
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
				serviceListFn: func(context.Context, dockerServiceListOptions) ([]swarmtypes.Service, error) {
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
					"com.docker.swarm.service.id":         "svc1",
					"com.docker.swarm.service.name":       "web",
					"com.docker.swarm.task.id":            "task1",
					"com.docker.swarm.task.slot":          "1",
					"com.docker.swarm.task.message":       "ok",
					"com.docker.swarm.task.error":         "",
					"com.docker.stack.namespace":          "stack",
					"com.docker.swarm.node.id":            "node1",
					"com.docker.swarm.node.name":          "node1",
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

		services, tasks, nodes, secrets, configs, swarmInfo := agent.collectSwarmData(context.Background(), info, containers)
		if len(tasks) != 1 || len(services) != 1 {
			t.Fatalf("expected derived tasks/services, got %d/%d", len(tasks), len(services))
		}
		if len(secrets) != 0 || len(configs) != 0 {
			t.Fatalf("expected no secrets/configs after manager collection error, got %+v/%+v", secrets, configs)
		}
		if len(nodes) != 1 || nodes[0].ID != "node1" {
			t.Fatalf("expected derived local node, got %+v", nodes)
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
				nodeListFn: func(context.Context, dockerNodeListOptions) ([]swarmtypes.Node, error) {
					return []swarmtypes.Node{{
						ID: "node-manager",
						Spec: swarmtypes.NodeSpec{
							Role:         swarmtypes.NodeRoleManager,
							Availability: swarmtypes.NodeAvailabilityActive,
						},
						Description: swarmtypes.NodeDescription{
							Hostname: "manager-1",
							Engine: swarmtypes.EngineDescription{
								EngineVersion: "27.5.1",
							},
						},
						Status: swarmtypes.NodeStatus{State: swarmtypes.NodeStateReady},
						ManagerStatus: &swarmtypes.ManagerStatus{
							Leader:       true,
							Reachability: swarmtypes.ReachabilityReachable,
						},
					}}, nil
				},
				serviceListFn: func(context.Context, dockerServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{ID: "svc1", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "zeta"}}},
						{ID: "svc2", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "alpha"}}},
					}, nil
				},
				taskListFn: func(context.Context, dockerTaskListOptions) ([]swarmtypes.Task, error) {
					return []swarmtypes.Task{
						{ID: "task2", ServiceID: "svc2", Slot: 2, DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
						{ID: "task1", ServiceID: "svc2", Slot: 1, DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
					}, nil
				},
				secretListFn: func(context.Context, dockerSecretListOptions) ([]swarmtypes.Secret, error) {
					return []swarmtypes.Secret{
						{ID: "secret-z", Spec: swarmtypes.SecretSpec{Annotations: swarmtypes.Annotations{Name: "zeta-secret"}}},
						{ID: "secret-a", Spec: swarmtypes.SecretSpec{Annotations: swarmtypes.Annotations{Name: "alpha-secret"}}},
					}, nil
				},
				configListFn: func(context.Context, dockerConfigListOptions) ([]swarmtypes.Config, error) {
					return []swarmtypes.Config{
						{ID: "config-z", Spec: swarmtypes.ConfigSpec{Annotations: swarmtypes.Annotations{Name: "zeta-config"}}},
						{ID: "config-a", Spec: swarmtypes.ConfigSpec{Annotations: swarmtypes.Annotations{Name: "alpha-config"}}},
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

		services, tasks, nodes, secrets, configs, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if len(tasks) != 2 || len(services) != 2 {
			t.Fatalf("expected manager tasks/services, got %d/%d", len(tasks), len(services))
		}
		if len(secrets) != 2 || secrets[0].Name != "alpha-secret" {
			t.Fatalf("expected sorted manager secret inventory, got %+v", secrets)
		}
		if len(configs) != 2 || configs[0].Name != "alpha-config" {
			t.Fatalf("expected sorted manager config inventory, got %+v", configs)
		}
		if len(nodes) != 1 || nodes[0].ID != "node-manager" || nodes[0].Hostname != "manager-1" {
			t.Fatalf("expected manager node inventory, got %+v", nodes)
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

		services, tasks, nodes, _, _, swarmInfo := agent.collectSwarmData(context.Background(), info, containers)
		if services != nil {
			t.Fatal("expected services to be nil when disabled")
		}
		if len(tasks) != 1 {
			t.Fatalf("expected tasks to be returned")
		}
		if len(nodes) != 1 {
			t.Fatalf("expected local swarm node to be returned")
		}
		if swarmInfo == nil {
			t.Fatalf("expected swarm info")
		}
	})

	t.Run("cluster info populated", func(t *testing.T) {
		agent := &Agent{
			supportsSwarm: true,
			cfg: Config{
				IncludeServices: false,
				IncludeTasks:    false,
				SwarmScope:      swarmScopeAuto,
			},
		}

		info := systemtypes.Info{
			Swarm: swarmtypes.Info{
				NodeID:           "node1",
				ControlAvailable: true,
				LocalNodeState:   swarmtypes.LocalNodeStateActive,
				Cluster: &swarmtypes.ClusterInfo{
					ID: "cluster1",
					Spec: swarmtypes.Spec{
						Annotations: swarmtypes.Annotations{Name: "prod"},
					},
				},
			},
		}

		services, tasks, nodes, _, _, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if services != nil || tasks != nil {
			t.Fatal("expected nil services/tasks when disabled")
		}
		if len(nodes) != 1 {
			t.Fatalf("expected local swarm node to be returned")
		}
		if swarmInfo == nil || swarmInfo.ClusterID != "cluster1" || swarmInfo.ClusterName != "prod" {
			t.Fatalf("expected cluster info to be populated")
		}
	})

	t.Run("sorts tasks and services", func(t *testing.T) {
		agent := &Agent{
			supportsSwarm: true,
			cfg: Config{
				IncludeServices: true,
				IncludeTasks:    true,
				SwarmScope:      swarmScopeCluster,
			},
			docker: &fakeDockerClient{
				serviceListFn: func(context.Context, dockerServiceListOptions) ([]swarmtypes.Service, error) {
					return []swarmtypes.Service{
						{ID: "b", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "web"}}},
						{ID: "a", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "web"}}},
						{ID: "c", Spec: swarmtypes.ServiceSpec{Annotations: swarmtypes.Annotations{Name: "api"}}},
					}, nil
				},
				taskListFn: func(context.Context, dockerTaskListOptions) ([]swarmtypes.Task, error) {
					return []swarmtypes.Task{
						{ID: "b", ServiceID: "a", Slot: 1, DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
						{ID: "a", ServiceID: "a", Slot: 1, DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
						{ID: "c", ServiceID: "a", Slot: 2, DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
						{ID: "d", ServiceID: "c", Slot: 1, DesiredState: swarmtypes.TaskStateRunning, Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning}},
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

		services, tasks, nodes, _, _, swarmInfo := agent.collectSwarmData(context.Background(), info, nil)
		if swarmInfo == nil {
			t.Fatalf("expected swarm info")
		}
		if len(nodes) != 1 {
			t.Fatalf("expected local swarm node to be returned")
		}
		if len(tasks) != 4 || len(services) != 3 {
			t.Fatalf("expected tasks and services")
		}
		if tasks[0].ServiceName == "" || services[0].Name == "" {
			t.Fatalf("expected sorted outputs to be populated")
		}
	})
}

func TestIsRuntimeSwarmTask(t *testing.T) {
	t.Run("accepts desired running task", func(t *testing.T) {
		task := &swarmtypes.Task{
			DesiredState: swarmtypes.TaskStateRunning,
			Status:       swarmtypes.TaskStatus{State: swarmtypes.TaskStateRunning},
		}
		if !isRuntimeSwarmTask(task) {
			t.Fatal("expected running task to be retained")
		}
	})

	t.Run("accepts empty desired state active task as fallback", func(t *testing.T) {
		task := &swarmtypes.Task{
			Status: swarmtypes.TaskStatus{State: swarmtypes.TaskStatePreparing},
		}
		if !isRuntimeSwarmTask(task) {
			t.Fatal("expected active task with empty desired state to be retained")
		}
	})

	t.Run("rejects shutdown historical task", func(t *testing.T) {
		task := &swarmtypes.Task{
			DesiredState: swarmtypes.TaskStateShutdown,
			Status:       swarmtypes.TaskStatus{State: swarmtypes.TaskStateComplete},
		}
		if isRuntimeSwarmTask(task) {
			t.Fatal("expected historical task to be excluded")
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
				"com.docker.swarm.service.id":         "svc1",
				"com.docker.swarm.service.name":       "web",
				"com.docker.swarm.task.slot":          "notint",
				"com.docker.swarm.task.message":       "ok",
				"com.docker.swarm.task.error":         "err",
				"com.docker.swarm.task.desired-state": "running",
				"com.docker.swarm.task.id":            "",
				"com.docker.swarm.node.name":          "nodeA",
				"com.docker.swarm.node.id":            "",
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

func TestDeriveSwarmTasksFromContainersEmpty(t *testing.T) {
	if tasks := deriveSwarmTasksFromContainers(nil, systemtypes.Info{}); tasks != nil {
		t.Fatalf("expected nil tasks for empty input")
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

func TestDeriveSwarmServicesFromDataNameFallback(t *testing.T) {
	tasks := []agentsdocker.Task{
		{
			ID:           "task1",
			ServiceID:    "",
			ServiceName:  "api",
			CurrentState: "running",
		},
	}

	containers := []agentsdocker.Container{
		{
			ID:    "container1",
			Image: "api:latest",
			Labels: map[string]string{
				"com.docker.swarm.service.name": "api",
			},
		},
	}

	services := deriveSwarmServicesFromData(tasks, containers)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].ID != "api" {
		t.Fatalf("expected service id to fall back to name")
	}
	if services[0].Labels != nil {
		t.Fatalf("expected nil labels when none set")
	}
}

func TestDeriveSwarmServicesFromDataEmptyAggregates(t *testing.T) {
	tasks := []agentsdocker.Task{
		{ID: "task1", ServiceID: "", ServiceName: ""},
	}
	containers := []agentsdocker.Container{
		{ID: "container1"},
		{
			ID:     "container2",
			Labels: map[string]string{"unrelated": "true"},
		},
		{
			ID:     "container3",
			Labels: map[string]string{"com.docker.swarm.service.id": "other"},
		},
	}

	if services := deriveSwarmServicesFromData(tasks, containers); services != nil {
		t.Fatalf("expected nil services for empty aggregates")
	}
}

func TestDeriveSwarmServicesFromDataContainerSkips(t *testing.T) {
	tasks := []agentsdocker.Task{
		{ID: "task1", ServiceID: "svc1", ServiceName: "web", CurrentState: "running"},
	}

	containers := []agentsdocker.Container{
		{ID: "container1"},
		{ID: "container2", Labels: map[string]string{"foo": "bar"}},
		{ID: "container3", Labels: map[string]string{"com.docker.swarm.service.id": "other"}},
	}

	services := deriveSwarmServicesFromData(tasks, containers)
	if len(services) != 1 {
		t.Fatalf("expected service aggregate to remain")
	}
}
