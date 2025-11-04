package monitoring

import (
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

func newTestMonitor(t *testing.T) *Monitor {
	t.Helper()

	return &Monitor{
		state:              models.NewState(),
		alertManager:       alerts.NewManager(),
		removedDockerHosts: make(map[string]time.Time),
		rateTracker:        NewRateTracker(),
	}
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
