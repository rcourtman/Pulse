package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func newTestMonitor(t *testing.T) *Monitor {
	t.Helper()

	m := &Monitor{
		state:               models.NewState(),
		alertManager:        alerts.NewManager(),
		removedDockerHosts:  make(map[string]time.Time),
		rateTracker:         NewRateTracker(),
		metricsHistory:      NewMetricsHistory(1000, 24*time.Hour),
		dockerTokenBindings: make(map[string]string),
		dockerMetadataStore: config.NewDockerMetadataStore(t.TempDir()),
	}
	t.Cleanup(func() { m.alertManager.Stop() })
	return m
}

func TestApplyDockerReportGeneratesUniqueIDsForCollidingHosts(t *testing.T) {
	monitor := newTestMonitor(t)

	baseTimestamp := time.Now().UTC()
	baseReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "docker-host",
			Name:             "Docker Host",
			MachineID:        "machine-duplicate",
			DockerVersion:    "26.0.0",
			TotalCPU:         4,
			TotalMemoryBytes: 8 << 30,
			UptimeSeconds:    120,
		},
		Containers: []agentsdocker.Container{
			{ID: "container-1", Name: "nginx"},
		},
		Timestamp: baseTimestamp,
	}

	token1 := &config.APITokenRecord{ID: "token-host-1", Name: "Host 1"}
	host1, err := monitor.ApplyDockerReport(baseReport, token1)
	if err != nil {
		t.Fatalf("ApplyDockerReport host1: %v", err)
	}
	if host1.ID == "" {
		t.Fatalf("expected host1 to have an identifier")
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host after first report, got %d", len(hosts))
	}

	secondReport := baseReport
	secondReport.Host.Name = "Docker Host Clone"
	secondReport.Timestamp = baseTimestamp.Add(45 * time.Second)

	token2 := &config.APITokenRecord{ID: "token-host-2", Name: "Host 2"}
	host2, err := monitor.ApplyDockerReport(secondReport, token2)
	if err != nil {
		t.Fatalf("ApplyDockerReport host2: %v", err)
	}
	if host2.ID == "" {
		t.Fatalf("expected host2 to have an identifier")
	}
	if host2.ID == host1.ID {
		t.Fatalf("expected unique identifiers, but both hosts share %q", host2.ID)
	}

	hosts = monitor.state.GetDockerHosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts after second report, got %d", len(hosts))
	}

	secondReport.Timestamp = secondReport.Timestamp.Add(45 * time.Second)
	secondReport.Containers = append(secondReport.Containers, agentsdocker.Container{
		ID:   "container-2",
		Name: "redis",
	})

	updatedHost2, err := monitor.ApplyDockerReport(secondReport, token2)
	if err != nil {
		t.Fatalf("ApplyDockerReport host2 update: %v", err)
	}
	if updatedHost2.ID != host2.ID {
		t.Fatalf("expected host2 to retain identifier %q, got %q", host2.ID, updatedHost2.ID)
	}

	hosts = monitor.state.GetDockerHosts()
	var found models.DockerHost
	for _, h := range hosts {
		if h.ID == host2.ID {
			found = h
			break
		}
	}
	if found.ID == "" {
		t.Fatalf("failed to locate host2 in state after update")
	}
	if len(found.Containers) != 2 {
		t.Fatalf("expected host2 to have 2 containers after update, got %d", len(found.Containers))
	}
}

func TestApplyDockerReportUsesTokenToDisambiguateAgentIDCollisions(t *testing.T) {
	monitor := newTestMonitor(t)

	baseReport := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "duplicate-agent",
			Version:         "1.0.0",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname:         "docker-one",
			Name:             "Docker One",
			MachineID:        "machine-A",
			DockerVersion:    "26.0.0",
			TotalCPU:         4,
			TotalMemoryBytes: 16 << 30,
			UptimeSeconds:    120,
		},
		Containers: []agentsdocker.Container{
			{ID: "container-a", Name: "api"},
		},
		Timestamp: time.Now().UTC(),
	}

	tokenOne := &config.APITokenRecord{ID: "token-one", Name: "Token One"}
	hostOne, err := monitor.ApplyDockerReport(baseReport, tokenOne)
	if err != nil {
		t.Fatalf("ApplyDockerReport hostOne: %v", err)
	}
	if hostOne.ID == "" {
		t.Fatal("expected hostOne to receive an identifier")
	}

	secondReport := baseReport
	secondReport.Host.Hostname = "docker-two"
	secondReport.Host.Name = "Docker Two"
	secondReport.Host.MachineID = "machine-B"
	secondReport.Containers = []agentsdocker.Container{
		{ID: "container-b", Name: "db"},
	}
	secondReport.Timestamp = baseReport.Timestamp.Add(30 * time.Second)

	tokenTwo := &config.APITokenRecord{ID: "token-two", Name: "Token Two"}
	hostTwo, err := monitor.ApplyDockerReport(secondReport, tokenTwo)
	if err != nil {
		t.Fatalf("ApplyDockerReport hostTwo: %v", err)
	}

	if hostTwo.ID == "" {
		t.Fatal("expected hostTwo to receive an identifier")
	}
	if hostOne.ID == hostTwo.ID {
		t.Fatalf("expected different identifiers for hosts sharing an agent ID, got %q", hostOne.ID)
	}

	hosts := monitor.state.GetDockerHosts()
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts after two reports, got %d", len(hosts))
	}

	updatedReport := baseReport
	updatedReport.Timestamp = baseReport.Timestamp.Add(60 * time.Second)
	updatedReport.Containers = append(updatedReport.Containers, agentsdocker.Container{
		ID:   "container-c",
		Name: "cache",
	})

	updatedHostOne, err := monitor.ApplyDockerReport(updatedReport, tokenOne)
	if err != nil {
		t.Fatalf("ApplyDockerReport hostOne update: %v", err)
	}
	if updatedHostOne.ID != hostOne.ID {
		t.Fatalf("expected hostOne to retain identifier %q, got %q", hostOne.ID, updatedHostOne.ID)
	}
}

func TestApplyDockerReportIncludesContainerDiskDetails(t *testing.T) {
	timestamp := time.Now().UTC()
	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "agent-1",
			Version:         "1.2.3",
			IntervalSeconds: 30,
		},
		Host: agentsdocker.HostInfo{
			Hostname: "disk-host",
		},
		Containers: []agentsdocker.Container{
			{
				ID:                  "ctr-1",
				Name:                "app",
				WritableLayerBytes:  512 * 1024 * 1024,
				RootFilesystemBytes: 2 * 1024 * 1024 * 1024,
				BlockIO: &agentsdocker.ContainerBlockIO{
					ReadBytes:  123456,
					WriteBytes: 654321,
				},
				Mounts: []agentsdocker.ContainerMount{
					{
						Type:        "bind",
						Source:      "/srv/app/config",
						Destination: "/config",
						Mode:        "rw",
						RW:          true,
						Propagation: "rprivate",
						Name:        "",
						Driver:      "",
					},
				},
			},
		},
		Timestamp: timestamp,
	}

	monitor := newTestMonitor(t)
	host, err := monitor.ApplyDockerReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport returned error: %v", err)
	}

	if len(host.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(host.Containers))
	}

	container := host.Containers[0]
	if container.WritableLayerBytes != 512*1024*1024 {
		t.Fatalf("expected writable layer bytes to match, got %d", container.WritableLayerBytes)
	}
	if container.RootFilesystemBytes != 2*1024*1024*1024 {
		t.Fatalf("expected root filesystem bytes to match, got %d", container.RootFilesystemBytes)
	}

	if container.BlockIO == nil {
		t.Fatalf("expected block IO stats to be populated")
	}
	if container.BlockIO.ReadBytes != 123456 || container.BlockIO.WriteBytes != 654321 {
		t.Fatalf("unexpected block IO values: %+v", container.BlockIO)
	}
	if container.BlockIO.ReadRateBytesPerSecond != nil || container.BlockIO.WriteRateBytesPerSecond != nil {
		t.Fatalf("expected block IO rates to be unset on first sample: %+v", container.BlockIO)
	}

	if len(container.Mounts) != 1 {
		t.Fatalf("expected mounts to be preserved, got %d", len(container.Mounts))
	}
	mount := container.Mounts[0]
	if mount.Source != "/srv/app/config" || mount.Destination != "/config" || !mount.RW {
		t.Fatalf("unexpected mount payload: %+v", mount)
	}
}

func TestApplyDockerReportPodmanRuntimeMetadata(t *testing.T) {
	monitor := newTestMonitor(t)

	report := agentsdocker.Report{
		Agent: agentsdocker.AgentInfo{
			ID:              "agent-podman",
			Version:         "2.0.0",
			IntervalSeconds: 60,
		},
		Host: agentsdocker.HostInfo{
			Hostname:       "podman-host",
			Runtime:        "podman",
			RuntimeVersion: "4.9.3",
			DockerVersion:  "",
		},
		Timestamp: time.Now().UTC(),
	}

	host, err := monitor.ApplyDockerReport(report, nil)
	if err != nil {
		t.Fatalf("ApplyDockerReport returned error: %v", err)
	}

	if host.Runtime != "podman" {
		t.Fatalf("expected runtime podman, got %q", host.Runtime)
	}
	if host.RuntimeVersion != "4.9.3" {
		t.Fatalf("expected runtime version 4.9.3, got %q", host.RuntimeVersion)
	}
	if host.DockerVersion != "4.9.3" {
		t.Fatalf("expected docker version fallback to runtime version, got %q", host.DockerVersion)
	}
}

func TestConvertDockerServices(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		t.Parallel()
		result := convertDockerServices(nil)
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("empty slice returns nil", func(t *testing.T) {
		t.Parallel()
		result := convertDockerServices([]agentsdocker.Service{})
		if result != nil {
			t.Fatalf("expected nil, got %v", result)
		}
	})

	t.Run("basic fields are copied", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:             "svc-123",
			Name:           "web",
			Stack:          "mystack",
			Image:          "nginx:latest",
			Mode:           "replicated",
			DesiredTasks:   3,
			RunningTasks:   2,
			CompletedTasks: 1,
		}}

		result := convertDockerServices(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 service, got %d", len(result))
		}

		svc := result[0]
		if svc.ID != "svc-123" {
			t.Errorf("ID = %q, want svc-123", svc.ID)
		}
		if svc.Name != "web" {
			t.Errorf("Name = %q, want web", svc.Name)
		}
		if svc.Stack != "mystack" {
			t.Errorf("Stack = %q, want mystack", svc.Stack)
		}
		if svc.Image != "nginx:latest" {
			t.Errorf("Image = %q, want nginx:latest", svc.Image)
		}
		if svc.Mode != "replicated" {
			t.Errorf("Mode = %q, want replicated", svc.Mode)
		}
		if svc.DesiredTasks != 3 {
			t.Errorf("DesiredTasks = %d, want 3", svc.DesiredTasks)
		}
		if svc.RunningTasks != 2 {
			t.Errorf("RunningTasks = %d, want 2", svc.RunningTasks)
		}
		if svc.CompletedTasks != 1 {
			t.Errorf("CompletedTasks = %d, want 1", svc.CompletedTasks)
		}
	})

	t.Run("labels are cloned when present", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:   "svc-1",
			Name: "web",
			Labels: map[string]string{
				"env":     "prod",
				"version": "1.0",
			},
		}}

		result := convertDockerServices(input)
		if result[0].Labels == nil {
			t.Fatal("expected labels to be present")
		}
		if result[0].Labels["env"] != "prod" {
			t.Errorf("Labels[env] = %q, want prod", result[0].Labels["env"])
		}
		if result[0].Labels["version"] != "1.0" {
			t.Errorf("Labels[version] = %q, want 1.0", result[0].Labels["version"])
		}

		// Verify it's a clone, not the same map
		input[0].Labels["env"] = "modified"
		if result[0].Labels["env"] == "modified" {
			t.Error("labels should be cloned, not shared")
		}
	})

	t.Run("empty labels are not copied", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:     "svc-1",
			Name:   "web",
			Labels: map[string]string{},
		}}

		result := convertDockerServices(input)
		if result[0].Labels != nil {
			t.Errorf("expected nil labels for empty map, got %v", result[0].Labels)
		}
	})

	t.Run("nil labels stay nil", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:     "svc-1",
			Name:   "web",
			Labels: nil,
		}}

		result := convertDockerServices(input)
		if result[0].Labels != nil {
			t.Errorf("expected nil labels, got %v", result[0].Labels)
		}
	})

	t.Run("endpoint ports are converted when present", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:   "svc-1",
			Name: "web",
			EndpointPorts: []agentsdocker.ServicePort{
				{
					Name:          "http",
					Protocol:      "tcp",
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   "ingress",
				},
				{
					Name:          "https",
					Protocol:      "tcp",
					TargetPort:    443,
					PublishedPort: 8443,
					PublishMode:   "host",
				},
			},
		}}

		result := convertDockerServices(input)
		if len(result[0].EndpointPorts) != 2 {
			t.Fatalf("expected 2 ports, got %d", len(result[0].EndpointPorts))
		}

		port1 := result[0].EndpointPorts[0]
		if port1.Name != "http" {
			t.Errorf("port[0].Name = %q, want http", port1.Name)
		}
		if port1.Protocol != "tcp" {
			t.Errorf("port[0].Protocol = %q, want tcp", port1.Protocol)
		}
		if port1.TargetPort != 80 {
			t.Errorf("port[0].TargetPort = %d, want 80", port1.TargetPort)
		}
		if port1.PublishedPort != 8080 {
			t.Errorf("port[0].PublishedPort = %d, want 8080", port1.PublishedPort)
		}
		if port1.PublishMode != "ingress" {
			t.Errorf("port[0].PublishMode = %q, want ingress", port1.PublishMode)
		}

		port2 := result[0].EndpointPorts[1]
		if port2.PublishMode != "host" {
			t.Errorf("port[1].PublishMode = %q, want host", port2.PublishMode)
		}
	})

	t.Run("empty endpoint ports are not copied", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:            "svc-1",
			Name:          "web",
			EndpointPorts: []agentsdocker.ServicePort{},
		}}

		result := convertDockerServices(input)
		if result[0].EndpointPorts != nil {
			t.Errorf("expected nil endpoint ports for empty slice, got %v", result[0].EndpointPorts)
		}
	})

	t.Run("nil endpoint ports stay nil", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:            "svc-1",
			Name:          "web",
			EndpointPorts: nil,
		}}

		result := convertDockerServices(input)
		if result[0].EndpointPorts != nil {
			t.Errorf("expected nil endpoint ports, got %v", result[0].EndpointPorts)
		}
	})

	t.Run("update status is converted when present", func(t *testing.T) {
		t.Parallel()
		completedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
		input := []agentsdocker.Service{{
			ID:   "svc-1",
			Name: "web",
			UpdateStatus: &agentsdocker.ServiceUpdate{
				State:       "completed",
				Message:     "update succeeded",
				CompletedAt: &completedAt,
			},
		}}

		result := convertDockerServices(input)
		if result[0].UpdateStatus == nil {
			t.Fatal("expected update status to be present")
		}
		if result[0].UpdateStatus.State != "completed" {
			t.Errorf("UpdateStatus.State = %q, want completed", result[0].UpdateStatus.State)
		}
		if result[0].UpdateStatus.Message != "update succeeded" {
			t.Errorf("UpdateStatus.Message = %q, want update succeeded", result[0].UpdateStatus.Message)
		}
		if result[0].UpdateStatus.CompletedAt == nil {
			t.Fatal("expected CompletedAt to be set")
		}
		if !result[0].UpdateStatus.CompletedAt.Equal(completedAt) {
			t.Errorf("UpdateStatus.CompletedAt = %v, want %v", result[0].UpdateStatus.CompletedAt, completedAt)
		}
	})

	t.Run("update status with nil CompletedAt", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:   "svc-1",
			Name: "web",
			UpdateStatus: &agentsdocker.ServiceUpdate{
				State:       "updating",
				Message:     "in progress",
				CompletedAt: nil,
			},
		}}

		result := convertDockerServices(input)
		if result[0].UpdateStatus == nil {
			t.Fatal("expected update status to be present")
		}
		if result[0].UpdateStatus.CompletedAt != nil {
			t.Errorf("expected nil CompletedAt, got %v", result[0].UpdateStatus.CompletedAt)
		}
	})

	t.Run("update status with zero CompletedAt", func(t *testing.T) {
		t.Parallel()
		zeroTime := time.Time{}
		input := []agentsdocker.Service{{
			ID:   "svc-1",
			Name: "web",
			UpdateStatus: &agentsdocker.ServiceUpdate{
				State:       "updating",
				CompletedAt: &zeroTime,
			},
		}}

		result := convertDockerServices(input)
		if result[0].UpdateStatus.CompletedAt != nil {
			t.Errorf("expected nil CompletedAt for zero time, got %v", result[0].UpdateStatus.CompletedAt)
		}
	})

	t.Run("nil update status stays nil", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:           "svc-1",
			Name:         "web",
			UpdateStatus: nil,
		}}

		result := convertDockerServices(input)
		if result[0].UpdateStatus != nil {
			t.Errorf("expected nil update status, got %v", result[0].UpdateStatus)
		}
	})

	t.Run("CreatedAt is copied when valid", func(t *testing.T) {
		t.Parallel()
		created := time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)
		input := []agentsdocker.Service{{
			ID:        "svc-1",
			Name:      "web",
			CreatedAt: &created,
		}}

		result := convertDockerServices(input)
		if result[0].CreatedAt == nil {
			t.Fatal("expected CreatedAt to be set")
		}
		if !result[0].CreatedAt.Equal(created) {
			t.Errorf("CreatedAt = %v, want %v", result[0].CreatedAt, created)
		}
	})

	t.Run("nil CreatedAt stays nil", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:        "svc-1",
			Name:      "web",
			CreatedAt: nil,
		}}

		result := convertDockerServices(input)
		if result[0].CreatedAt != nil {
			t.Errorf("expected nil CreatedAt, got %v", result[0].CreatedAt)
		}
	})

	t.Run("zero CreatedAt is not copied", func(t *testing.T) {
		t.Parallel()
		zeroTime := time.Time{}
		input := []agentsdocker.Service{{
			ID:        "svc-1",
			Name:      "web",
			CreatedAt: &zeroTime,
		}}

		result := convertDockerServices(input)
		if result[0].CreatedAt != nil {
			t.Errorf("expected nil CreatedAt for zero time, got %v", result[0].CreatedAt)
		}
	})

	t.Run("UpdatedAt is copied when valid", func(t *testing.T) {
		t.Parallel()
		updated := time.Date(2025, 1, 12, 14, 30, 0, 0, time.UTC)
		input := []agentsdocker.Service{{
			ID:        "svc-1",
			Name:      "web",
			UpdatedAt: &updated,
		}}

		result := convertDockerServices(input)
		if result[0].UpdatedAt == nil {
			t.Fatal("expected UpdatedAt to be set")
		}
		if !result[0].UpdatedAt.Equal(updated) {
			t.Errorf("UpdatedAt = %v, want %v", result[0].UpdatedAt, updated)
		}
	})

	t.Run("nil UpdatedAt stays nil", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{{
			ID:        "svc-1",
			Name:      "web",
			UpdatedAt: nil,
		}}

		result := convertDockerServices(input)
		if result[0].UpdatedAt != nil {
			t.Errorf("expected nil UpdatedAt, got %v", result[0].UpdatedAt)
		}
	})

	t.Run("zero UpdatedAt is not copied", func(t *testing.T) {
		t.Parallel()
		zeroTime := time.Time{}
		input := []agentsdocker.Service{{
			ID:        "svc-1",
			Name:      "web",
			UpdatedAt: &zeroTime,
		}}

		result := convertDockerServices(input)
		if result[0].UpdatedAt != nil {
			t.Errorf("expected nil UpdatedAt for zero time, got %v", result[0].UpdatedAt)
		}
	})

	t.Run("multiple services are converted", func(t *testing.T) {
		t.Parallel()
		input := []agentsdocker.Service{
			{ID: "svc-1", Name: "web"},
			{ID: "svc-2", Name: "api"},
			{ID: "svc-3", Name: "worker"},
		}

		result := convertDockerServices(input)
		if len(result) != 3 {
			t.Fatalf("expected 3 services, got %d", len(result))
		}
		if result[0].ID != "svc-1" {
			t.Errorf("result[0].ID = %q, want svc-1", result[0].ID)
		}
		if result[1].ID != "svc-2" {
			t.Errorf("result[1].ID = %q, want svc-2", result[1].ID)
		}
		if result[2].ID != "svc-3" {
			t.Errorf("result[2].ID = %q, want svc-3", result[2].ID)
		}
	})

	t.Run("full service with all fields", func(t *testing.T) {
		t.Parallel()
		created := time.Date(2025, 1, 10, 8, 0, 0, 0, time.UTC)
		updated := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
		completedAt := time.Date(2025, 1, 15, 11, 30, 0, 0, time.UTC)

		input := []agentsdocker.Service{{
			ID:             "svc-full",
			Name:           "fullservice",
			Stack:          "production",
			Image:          "myapp:v2.0",
			Mode:           "global",
			DesiredTasks:   5,
			RunningTasks:   5,
			CompletedTasks: 0,
			Labels: map[string]string{
				"com.docker.stack.namespace": "production",
			},
			EndpointPorts: []agentsdocker.ServicePort{
				{Name: "web", Protocol: "tcp", TargetPort: 8080, PublishedPort: 80, PublishMode: "ingress"},
			},
			UpdateStatus: &agentsdocker.ServiceUpdate{
				State:       "completed",
				Message:     "rollout complete",
				CompletedAt: &completedAt,
			},
			CreatedAt: &created,
			UpdatedAt: &updated,
		}}

		result := convertDockerServices(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 service, got %d", len(result))
		}

		svc := result[0]
		if svc.ID != "svc-full" {
			t.Errorf("ID mismatch")
		}
		if svc.Mode != "global" {
			t.Errorf("Mode = %q, want global", svc.Mode)
		}
		if svc.Labels["com.docker.stack.namespace"] != "production" {
			t.Errorf("Labels mismatch")
		}
		if len(svc.EndpointPorts) != 1 || svc.EndpointPorts[0].PublishedPort != 80 {
			t.Errorf("EndpointPorts mismatch")
		}
		if svc.UpdateStatus == nil || svc.UpdateStatus.State != "completed" {
			t.Errorf("UpdateStatus mismatch")
		}
		if svc.CreatedAt == nil || !svc.CreatedAt.Equal(created) {
			t.Errorf("CreatedAt mismatch")
		}
		if svc.UpdatedAt == nil || !svc.UpdatedAt.Equal(updated) {
			t.Errorf("UpdatedAt mismatch")
		}
	})
}

func TestApplyDockerReport_MissingIdentifier(t *testing.T) {
	monitor := newTestMonitor(t)

	// Report with no agent ID and no hostname - should fail
	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			Hostname: "", // Empty hostname
		},
		Agent: agentsdocker.AgentInfo{
			ID: "", // Empty agent ID
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyDockerReport(report, nil)
	if err == nil {
		t.Error("expected error for missing identifier")
	}
	if err != nil && !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected 'missing' in error message, got: %v", err)
	}
}

func TestApplyDockerReport_RemovedHostRejection(t *testing.T) {
	monitor := newTestMonitor(t)

	// Mark host as removed
	hostID := "removed-docker-host"
	removedAt := time.Now().Add(-1 * time.Hour)
	monitor.mu.Lock()
	monitor.removedDockerHosts[hostID] = removedAt
	monitor.mu.Unlock()

	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			Hostname: hostID,
		},
		Agent: agentsdocker.AgentInfo{
			ID: hostID,
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyDockerReport(report, nil)
	if err == nil {
		t.Error("expected error for removed host")
	}
	if err != nil && !strings.Contains(err.Error(), "was removed") {
		t.Errorf("expected 'was removed' in error message, got: %v", err)
	}
}

func TestApplyDockerReport_TokenBoundToDifferentAgent(t *testing.T) {
	monitor := newTestMonitor(t)

	tokenID := "shared-token"
	firstAgentID := "agent-first"
	secondAgentID := "agent-second"

	// Pre-bind token to first agent
	monitor.mu.Lock()
	monitor.dockerTokenBindings[tokenID] = firstAgentID
	monitor.mu.Unlock()

	// Report from second agent using same token
	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			Hostname: "second-host",
		},
		Agent: agentsdocker.AgentInfo{
			ID: secondAgentID,
		},
		Timestamp: time.Now(),
	}

	token := &config.APITokenRecord{ID: tokenID, Name: "TestToken"}

	_, err := monitor.ApplyDockerReport(report, token)
	if err == nil {
		t.Error("expected error for token bound to different agent")
	}
	if err != nil && !strings.Contains(err.Error(), "already in use by agent") {
		t.Errorf("expected 'already in use by agent' in error message, got: %v", err)
	}
}

func TestApplyDockerReport_MissingHostname(t *testing.T) {
	monitor := newTestMonitor(t)

	// Report with agent ID but no hostname
	report := agentsdocker.Report{
		Host: agentsdocker.HostInfo{
			Hostname: "", // Missing hostname
		},
		Agent: agentsdocker.AgentInfo{
			ID: "agent-with-id",
		},
		Timestamp: time.Now(),
	}

	_, err := monitor.ApplyDockerReport(report, nil)
	if err == nil {
		t.Error("expected error for missing hostname")
	}
	if err != nil && !strings.Contains(err.Error(), "missing hostname") {
		t.Errorf("expected 'missing hostname' in error message, got: %v", err)
	}
}
