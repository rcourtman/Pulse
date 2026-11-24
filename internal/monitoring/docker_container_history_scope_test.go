package monitoring

import (
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/alerts"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
)

// A container ID is text from the reporting host's own report. Two Docker
// hosts that report the same ID (a cloned VM, or a host naming another host's
// container) must each keep their own history, under the key the container's
// metrics target hands out, and a container's CPU alert window must read only
// what its own host reported.
func TestApplyDockerReportScopesContainerHistoryToReportingHost(t *testing.T) {
	useRealMetricWindowMode(t)
	m := newMetricWindowRegistryMonitor(t, true)
	m.SetResourceStore(newMetricWindowAdapter())
	start := time.Now().Add(-time.Minute)

	containerID := strings.Repeat("7b", 32)
	report := func(name string, cpuPercent float64) models.DockerHost {
		t.Helper()
		host, err := m.ApplyDockerReport(agentsdocker.Report{
			Agent:      agentsdocker.AgentInfo{ID: name + "-agent", Version: "test", IntervalSeconds: 30},
			Host:       agentsdocker.HostInfo{Hostname: name, MachineID: name + "-machine", TotalCPU: 4},
			Containers: []agentsdocker.Container{{ID: containerID, Name: metricWindowContainerName, State: "running", CPUPercent: cpuPercent}},
			Timestamp:  time.Now().UTC(),
		}, nil)
		if err != nil {
			t.Fatalf("ApplyDockerReport %s: %v", name, err)
		}
		if len(host.Containers) != 1 || host.Containers[0].ID != containerID {
			t.Fatalf("report of %s did not record the container: %+v", name, host.Containers)
		}
		return host
	}
	victim := report("web-b", 380)
	other := report("web-a", 4)
	if victim.ID == other.ID {
		t.Fatalf("reports resolved to one Docker host %q", victim.ID)
	}
	m.metricsStore.Flush()

	for _, host := range []models.DockerHost{victim, other} {
		want := models.DockerContainerCPUCapacityPercent(host.Containers[0], host.CPUs)
		metricID := unifiedresources.DockerContainerMetricID(host.ID, containerID)
		points := m.metricsHistory.GetGuestMetrics("docker:"+metricID, "cpu", time.Hour)
		if len(points) != 1 || points[0].Value != want {
			t.Fatalf("host %s CPU history = %+v, want one point %v", host.ID, points, want)
		}
		persisted, err := m.metricsStore.Query("dockerContainer", metricID, "cpu", start, time.Now().Add(time.Minute), 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(persisted) != 1 || persisted[0].Value != want {
			t.Fatalf("host %s persisted CPU = %+v, want one point %v", host.ID, persisted, want)
		}

		reference := alerts.DockerContainerResourceID(host.ID, containerID, metricWindowContainerName)
		window, _ := readCPUWindow(t, m, reference, "app-container")
		if len(window) == 0 {
			t.Fatalf("host %s CPU alert window is empty", host.ID)
		}
		for _, point := range window {
			if point.Value != want {
				t.Fatalf("host %s CPU alert window = %+v, want only its own %v", host.ID, window, want)
			}
		}
	}
	if victimCPU, otherCPU := models.DockerContainerCPUCapacityPercent(victim.Containers[0], victim.CPUs),
		models.DockerContainerCPUCapacityPercent(other.Containers[0], other.CPUs); victimCPU == otherCPU {
		t.Fatalf("both hosts recorded %v, the test cannot tell their series apart", victimCPU)
	}

	if points := m.metricsHistory.GetGuestMetrics("docker:"+containerID, "cpu", time.Hour); len(points) != 0 {
		t.Fatalf("history recorded under the bare container ID: %+v", points)
	}
	if persisted, err := m.metricsStore.Query("dockerContainer", containerID, "cpu", start, time.Now().Add(time.Minute), 0); err != nil || len(persisted) != 0 {
		t.Fatalf("history persisted under the bare container ID: %+v, %v", persisted, err)
	}
}
