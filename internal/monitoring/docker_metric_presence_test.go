package monitoring

import (
	"encoding/json"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	"github.com/rcourtman/pulse-go-rewrite/pkg/metrics"
)

func TestApplyDockerReportPreservesDiskObservationPresence(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	m := newTestMonitor(t)
	store, err := metrics.NewStore(metrics.DefaultConfig(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	m.metricsStore = store
	start := time.Now().Add(-time.Minute)
	report := agentsdocker.Report{
		Agent:      agentsdocker.AgentInfo{ID: "presence-agent", Version: "6.4.2", IntervalSeconds: 30},
		Host:       agentsdocker.HostInfo{Hostname: "presence-host"},
		Containers: []agentsdocker.Container{{ID: "presence-container", Name: "api", WritableLayerBytes: 200, RootFilesystemBytes: 1000}},
	}
	observed := &agentsdocker.ContainerBlockIO{ReadBytes: 5000, WriteBytes: 7000}
	steps := []struct {
		name        string
		io          *agentsdocker.ContainerBlockIO
		wantSamples int
	}{
		{"absent", nil, 0},
		{"first observation", observed, 0},
		{"measured idle", observed, 1},
		{"missing after observation", nil, 1},
		{"idle after gap", observed, 2},
	}
	for i, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			report.Timestamp = start.Add(time.Duration(i) * time.Second)
			report.Containers[0].BlockIO = step.io
			host, err := m.ApplyDockerReport(report, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(host.Containers) != 1 {
				t.Fatalf("containers: %+v", host.Containers)
			}
			ct := host.Containers[0]
			if step.io == nil && ct.BlockIO != nil {
				t.Fatalf("absent BlockIO retained: %+v", ct.BlockIO)
			}
			if i == 1 && (ct.BlockIO.ReadRateBytesPerSecond != nil || ct.BlockIO.WriteRateBytesPerSecond != nil) {
				t.Fatalf("warmup became a rate: %+v", ct.BlockIO)
			}
			if i == 2 || i == 4 {
				if ct.BlockIO.ReadRateBytesPerSecond == nil || *ct.BlockIO.ReadRateBytesPerSecond != 0 || ct.BlockIO.WriteRateBytesPerSecond == nil || *ct.BlockIO.WriteRateBytesPerSecond != 0 {
					t.Fatalf("unchanged counters must remain measured idle across gaps: %+v", ct.BlockIO)
				}
			}
			store.Flush()
			for _, metric := range []string{"diskread", "diskwrite"} {
				points := m.metricsHistory.GetGuestMetrics("docker:"+ct.ID, metric, time.Hour)
				if len(points) != step.wantSamples {
					t.Fatalf("%s history = %d, want %d", metric, len(points), step.wantSamples)
				}
				persisted, err := store.Query("dockerContainer", ct.ID, metric, start, time.Now().Add(time.Minute), 0)
				if err != nil {
					t.Fatal(err)
				}
				if step.wantSamples == 0 && len(persisted) != 0 {
					t.Fatalf("fabricated persisted %s: %+v", metric, persisted)
				}
				if step.wantSamples > 0 && (len(persisted) == 0 || persisted[len(persisted)-1].Value != 0) {
					t.Fatalf("lost persisted idle %s: %+v", metric, persisted)
				}
			}
			if points := m.metricsHistory.GetGuestMetrics("docker:"+ct.ID, "disk", time.Hour); len(points) != 0 {
				t.Fatalf("layer sizes became capacity history: %+v", points)
			}
			points, err := store.Query("dockerContainer", ct.ID, "disk", start, time.Now().Add(time.Minute), 0)
			if err != nil || len(points) != 0 {
				t.Fatalf("layer sizes became persisted capacity: %+v, %v", points, err)
			}
		})
	}
}

func TestApplyDockerReportPreservesIndependentZeroCounters(t *testing.T) {
	previous := mock.IsMockEnabled()
	mustSetMockEnabled(t, false)
	t.Cleanup(func() { mustSetMockEnabled(t, previous) })
	m := newTestMonitor(t)
	present, absent := true, false
	report := agentsdocker.Report{
		Agent:      agentsdocker.AgentInfo{ID: "zero-agent", Version: "6.4.2", IntervalSeconds: 30},
		Host:       agentsdocker.HostInfo{Hostname: "zero-host"},
		Containers: []agentsdocker.Container{{ID: "zero-container", Name: "idle", BlockIO: &agentsdocker.ContainerBlockIO{ReadBytesPresent: &present, WriteBytesPresent: &absent}}},
	}
	for i := 0; i < 2; i++ {
		report.Timestamp = time.Now().Add(time.Duration(i) * time.Second)
		host, err := m.ApplyDockerReport(report, nil)
		if err != nil {
			t.Fatal(err)
		}
		io := host.Containers[0].BlockIO
		if io == nil {
			t.Fatal("explicit zero counter lost")
		}
		if io.WriteRateBytesPerSecond != nil {
			t.Fatalf("missing write direction became measured rate: %+v", io)
		}
		if i == 0 && io.ReadRateBytesPerSecond != nil {
			t.Fatal("first zero counter became a measured rate")
		}
		if i == 1 && (io.ReadRateBytesPerSecond == nil || *io.ReadRateBytesPerSecond != 0) {
			t.Fatalf("measured zero lost: %+v", io)
		}
	}
	if points := m.metricsHistory.GetGuestMetrics("docker:zero-container", "diskwrite", time.Hour); len(points) != 0 {
		t.Fatalf("missing writes recorded: %+v", points)
	}
	if points := m.metricsHistory.GetGuestMetrics("docker:zero-container", "diskread", time.Hour); len(points) != 1 {
		t.Fatalf("idle reading lost: %+v", points)
	}
}

func TestResourceDiskIOWirePreservesAbsentDirection(t *testing.T) {
	zero := &unifiedresources.MetricValue{Value: 0, Unit: "bytes/s", Source: unifiedresources.SourceDocker}
	for _, tc := range []struct {
		name    string
		metrics *unifiedresources.ResourceMetrics
		wire    string
	}{
		{"absent", nil, ``},
		{"read idle", &unifiedresources.ResourceMetrics{DiskRead: zero}, `{"readRate":0}`},
		{"write idle", &unifiedresources.ResourceMetrics{DiskWrite: zero}, `{"writeRate":0}`},
		{"both idle", &unifiedresources.ResourceMetrics{DiskRead: zero, DiskWrite: zero}, `{"readRate":0,"writeRate":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read, write := monitorDiskIOMetricInput(tc.metrics)
			frontend := models.ConvertResourceToFrontend(models.ResourceConvertInput{DiskReadRate: read, DiskWriteRate: write})
			if tc.wire == "" {
				if frontend.DiskIO != nil {
					t.Fatalf("absent IO became a payload: %+v", frontend.DiskIO)
				}
				return
			}
			wire, err := json.Marshal(frontend.DiskIO)
			if err != nil {
				t.Fatal(err)
			}
			if string(wire) != tc.wire {
				t.Fatalf("wire = %s, want %s", wire, tc.wire)
			}
		})
	}
}
