package monitoring

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rcourtman/pulse-go-rewrite/pkg/agents/filesystem"
)

func TestDockerFilesystemEvidenceSurvivesIngestionAndReplacesStaleUsage(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	m := newTestMonitor(t)
	registry := unifiedresources.NewRegistry(nil)
	now := time.Now().UTC()
	report := agentsdocker.Report{Timestamp: now, Agent: agentsdocker.AgentInfo{ID: "fs-agent", IntervalSeconds: 30}, Host: agentsdocker.HostInfo{Hostname: "fs-host"}, Containers: []agentsdocker.Container{{ID: "fs-container", Name: "worker", State: "running", WritableLayerBytes: 999, RootFilesystemBytes: 9999}}}
	observations := [][]filesystem.Observation{
		{{Mountpoint: "/cache", Source: "linux-proc-root-statfs", ObservedAt: now, Type: "tmpfs", Usage: &filesystem.Usage{CapacityBytes: 8 << 20, Inodes: &filesystem.InodeUsage{Capacity: 100, Free: 50}}}},
		{{Mountpoint: "/cache", Source: "linux-proc-root-statfs", ObservedAt: now.Add(time.Second), Error: "namespace unavailable"}},
		nil,
	}
	for i, rows := range observations {
		report.Timestamp = now.Add(time.Duration(i) * time.Second)
		report.Containers[0].Filesystems = rows
		host, err := m.ApplyDockerReport(report, nil)
		if err != nil {
			t.Fatal(err)
		}
		registry.IngestSnapshot(models.StateSnapshot{DockerHosts: []models.DockerHost{host}})
		resources := registry.ListByType(unifiedresources.ResourceTypeAppContainer)
		if len(resources) != 1 {
			t.Fatalf("resources: %+v", resources)
		}
		got := resources[0].Docker.Filesystems
		if len(got) != len(rows) {
			t.Fatalf("old evidence retained at step %d: %+v", i, got)
		}
		if resources[0].Metrics != nil && resources[0].Metrics.Disk != nil {
			t.Fatal("mount capacity was promoted into a container-wide disk percentage")
		}
		if i == 0 {
			if got[0].Usage == nil || got[0].Usage.CapacityBytes != 8<<20 || got[0].Usage.AvailableBytes != 0 {
				t.Fatalf("measured full filesystem lost: %+v", got)
			}
			got[0].Usage.CapacityBytes = 1
			got[0].Usage.Inodes.Free = 1
			report.Containers[0].Filesystems[0].Usage.CapacityBytes = 2
			fresh := registry.ListByType(unifiedresources.ResourceTypeAppContainer)[0].Docker.Filesystems[0]
			if fresh.Usage.CapacityBytes != 8<<20 || fresh.Usage.Inodes.Free != 50 {
				t.Fatal("resource snapshot aliases external observation storage")
			}
		}
		if i == 1 && (got[0].Usage != nil || got[0].Error == "") {
			t.Fatalf("failed read kept old numeric usage: %+v", got)
		}
		wire, err := json.Marshal(resources)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("FILESYSTEM_RESOURCE_STEP_%d %s", i, wire)
	}
}
